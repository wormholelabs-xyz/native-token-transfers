// Command disclosure-service is a production-runnable HTTP endpoint. /v1/flows/{flow} assembles
// one flow's disclosure set from one stakeholder participant's ACS over the JSON Ledger API v2.
// Each set comes from its choice body in canton/ntt/daml/Wormhole/Ntt/Manager.daml (see flows.go).
//
// Posture: the service has no inbound authentication. An authenticating TLS proxy in front is
// mandatory. A non-loopback --listen needs --behind-authenticating-proxy.
// --access-token-file is the service's own credential for the upstream JSON API.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultListen                  = "127.0.0.1:7599"
	defaultMaxContractsPerTemplate = 1000
	// Bounds this service's load on the shared participant independently of inbound volume.
	defaultMaxUpstreamConcurrency = 8
	// Short enough that a caller rarely sees an archived contract, long enough to collapse
	// a burst of identical requests into one upstream read.
	defaultCacheTTL   = 5 * time.Second
	httpClientTimeout = 30 * time.Second
	shutdownTimeout   = 5 * time.Second
	// maxUpstreamResponseBytes caps every upstream response body. The per-template contract cap
	// fires only after parsing, so this byte ceiling is the earlier bound.
	maxUpstreamResponseBytes = 64 << 20
	serverReadHeaderTimeout  = 10 * time.Second
	serverReadTimeout        = 30 * time.Second
	serverIdleTimeout        = 2 * time.Minute
	maxInboundHeaderBytes    = 8 << 10

	maxUpstreamConcurrencyLimit  = 64
	maxCacheTTL                  = 60 * time.Second
	maxContractsPerTemplateLimit = 10_000
)

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

// options is parseFlags' pure result. I/O happens later, when run() consumes it.
type options struct {
	listen                    string
	jsonAPIBaseURL            string
	readerParties             readerPartiesByRole
	accessTokenFile           string
	maxContractsPerTemplate   int
	maxUpstreamConcurrency    int
	cacheTTL                  time.Duration
	behindAuthenticatingProxy bool
	verbose                   bool
}

// parseFlags parses args (normally os.Args[1:]) into options. It checks the required flags.
// run() reads the files that flags name. parseFlags returns an actionable error that names the
// missing flag.
func parseFlags(args []string) (*options, error) {
	fs := flag.NewFlagSet("disclosure-service", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller reports the error; this skips flag's own usage dump

	listen := fs.String("listen", defaultListen, "address to listen on")
	jsonAPI := fs.String("json-api", "", "base URL of the JSON Ledger API v2 (required)")
	guardianGovernanceParty := fs.String("guardian-governance-party", "", "gg: reads the bridge state (required)")
	nttCustodyParty := fs.String("ntt-custody-party", "", "nttCustody: reads the custody pots and its own TransferPreapprovals (required)")
	operatorParty := fs.String("operator-party", "", "operator: reads ReplayRootRegistry only; enables the register flow (optional)")
	tokenFile := fs.String("access-token-file", "", "path to a file holding a bearer token forwarded upstream; re-read on every upstream request")
	maxContracts := fs.Int("max-contracts-per-template", defaultMaxContractsPerTemplate, "cap on NttManager contracts in /v1/managers, 1 to 10000; more is an error")
	maxUpstream := fs.Int("max-upstream-concurrency", defaultMaxUpstreamConcurrency, "cap on in-flight upstream ledger queries, 1 to 64; excess requests wait briefly, then get a 503")
	cacheTTL := fs.Duration("cache-ttl", defaultCacheTTL, "serve repeat ledger reads from memory for this long, 0 to 60s; 0 turns the cache off")
	behindAuthenticatingProxy := fs.Bool("behind-authenticating-proxy", false, "state that an authenticating TLS proxy is in front; required for a non-loopback --listen")
	verbose := fs.Bool("verbose", false, "verbose logging")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("disclosure-service: %w", err)
	}
	if *jsonAPI == "" {
		return nil, fmt.Errorf("disclosure-service: --json-api is required (base URL of the JSON Ledger API v2, e.g. http://127.0.0.1:6975)")
	}
	if err := checkJSONAPIURL(*jsonAPI, *tokenFile != ""); err != nil {
		return nil, err
	}
	readerParties, err := parseReaderParties(*guardianGovernanceParty, *nttCustodyParty, *operatorParty)
	if err != nil {
		return nil, err
	}
	if *listen == "" {
		return nil, fmt.Errorf("disclosure-service: --listen must not be empty")
	}
	if err := checkListenAddress(*listen, *behindAuthenticatingProxy); err != nil {
		return nil, err
	}
	if *maxContracts < 1 || *maxContracts > maxContractsPerTemplateLimit {
		return nil, fmt.Errorf("disclosure-service: --max-contracts-per-template must be 1 to %d, got %d", maxContractsPerTemplateLimit, *maxContracts)
	}
	if *maxUpstream < 1 || *maxUpstream > maxUpstreamConcurrencyLimit {
		return nil, fmt.Errorf("disclosure-service: --max-upstream-concurrency must be 1 to %d, got %d", maxUpstreamConcurrencyLimit, *maxUpstream)
	}
	if *cacheTTL < 0 || *cacheTTL > maxCacheTTL {
		return nil, fmt.Errorf("disclosure-service: --cache-ttl must be 0 to %s, got %s", maxCacheTTL, *cacheTTL)
	}
	return &options{
		listen:                    *listen,
		jsonAPIBaseURL:            *jsonAPI,
		readerParties:             readerParties,
		accessTokenFile:           *tokenFile,
		maxContractsPerTemplate:   *maxContracts,
		maxUpstreamConcurrency:    *maxUpstream,
		cacheTTL:                  *cacheTTL,
		behindAuthenticatingProxy: *behindAuthenticatingProxy,
		verbose:                   *verbose,
	}, nil
}

// checkListenAddress refuses a non-loopback bind unless the operator states that an
// authenticating proxy is in front.
func checkListenAddress(listenAddress string, behindAuthenticatingProxy bool) error {
	host, _, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return fmt.Errorf("disclosure-service: --listen %q: %w", listenAddress, err)
	}
	if isLoopbackIPAddress(host) {
		return nil
	}
	if !behindAuthenticatingProxy {
		return fmt.Errorf("disclosure-service: --listen %q is not a loopback IP address; set --behind-authenticating-proxy only when an authenticating proxy is in front", listenAddress)
	}
	return nil
}

// newHTTPServer bounds the inbound headers and the time to write each response.
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      inboundRequestDeadline + 5*time.Second,
		IdleTimeout:       serverIdleTimeout,
		MaxHeaderBytes:    maxInboundHeaderBytes,
	}
}

// ---------------------------------------------------------------------------
// run / main
// ---------------------------------------------------------------------------

// run builds and serves the service until ctx is canceled, then shuts down gracefully. It
// returns nil on a clean shutdown, including http.ErrServerClosed. Any other error is real.
func run(ctx context.Context, opts *options, stderr io.Writer) error {
	if err := checkListenAddress(opts.listen, opts.behindAuthenticatingProxy); err != nil {
		return err
	}
	// Fail fast at startup when the token file is missing or unreadable. httpACSClient keeps
	// only the path; see readAccessToken's call in authHeader.
	if opts.accessTokenFile != "" {
		if _, err := readAccessToken(opts.accessTokenFile); err != nil {
			return err
		}
	}

	if opts.maxUpstreamConcurrency < 1 || opts.maxUpstreamConcurrency > maxUpstreamConcurrencyLimit {
		return fmt.Errorf("disclosure-service: upstream concurrency %d is out of bounds", opts.maxUpstreamConcurrency)
	}
	var acs acsClient = newBoundedACS(newHTTPACSClient(opts.jsonAPIBaseURL, opts.accessTokenFile), opts.maxUpstreamConcurrency)
	if opts.cacheTTL > 0 {
		acs = newCachingACS(acs, opts.cacheTTL)
	}
	srv := newServer(opts, acs)

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("disclosure-service: listen on %s: %w", opts.listen, err)
	}
	//SECURITY: second boundary; the bound address, not the flag text.
	if err := checkListenAddress(ln.Addr().String(), opts.behindAuthenticatingProxy); err != nil {
		_ = ln.Close()
		return err
	}

	fmt.Fprintf(stderr, "disclosure-service: reader parties guardianGovernance=%s nttCustody=%s operator=%q, %d flow template(s)\n",
		opts.readerParties.guardianGovernance, opts.readerParties.nttCustody, opts.readerParties.operator, len(flowTemplateByTail))
	fmt.Fprintln(stderr, "disclosure-service: no inbound authentication; an authenticating proxy in front is mandatory")
	fmt.Fprintf(stderr, "disclosure-service: listening on http://%s\n", ln.Addr().String())

	httpSrv := newHTTPServer(srv)
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("disclosure-service: shutdown: %w", err)
		}
		<-serveErr // Serve always returns once Shutdown completes
		return nil
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("disclosure-service: serve: %w", err)
		}
		return nil
	}
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, opts, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
