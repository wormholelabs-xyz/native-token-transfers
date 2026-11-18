// Command disclosure-service is a production-runnable HTTP endpoint. /v1/flows/{flow} assembles
// one flow's disclosure set from one stakeholder participant's ACS over the JSON Ledger API v2.
// Each set comes from its choice body in canton/ntt/daml/Wormhole/Ntt/Manager.daml (see flows.go).
//
// Posture: the service is unauthenticated and harness/ops-grade. It listens on loopback by
// default. The --access-token-file flag adds a single shared bearer token. The service forwards
// this token to the upstream JSON API.
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
)

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

// options is parseFlags' pure result. I/O happens later, when run() consumes it.
type options struct {
	listen                  string
	jsonAPIBaseURL          string
	readerParties           readerPartiesByRole
	accessTokenFile         string
	maxContractsPerTemplate int
	maxUpstreamConcurrency  int
	cacheTTL                time.Duration
	verbose                 bool
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
	maxContracts := fs.Int("max-contracts-per-template", defaultMaxContractsPerTemplate, "cap on active contracts returned per template; a template that exceeds it is an error")
	maxUpstream := fs.Int("max-upstream-concurrency", defaultMaxUpstreamConcurrency, "cap on in-flight upstream ledger queries; excess requests wait briefly, then get a 503")
	cacheTTL := fs.Duration("cache-ttl", defaultCacheTTL, "serve repeat ledger reads from memory for this long; 0 disables the cache")
	verbose := fs.Bool("verbose", false, "verbose logging")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("disclosure-service: %w", err)
	}
	if *jsonAPI == "" {
		return nil, fmt.Errorf("disclosure-service: --json-api is required (base URL of the JSON Ledger API v2, e.g. http://127.0.0.1:6975)")
	}
	readerParties, err := parseReaderParties(*guardianGovernanceParty, *nttCustodyParty, *operatorParty)
	if err != nil {
		return nil, err
	}
	if *listen == "" {
		return nil, fmt.Errorf("disclosure-service: --listen must not be empty")
	}
	if *maxContracts <= 0 {
		return nil, fmt.Errorf("disclosure-service: --max-contracts-per-template must be positive, got %d", *maxContracts)
	}
	return &options{
		listen:                  *listen,
		jsonAPIBaseURL:          *jsonAPI,
		readerParties:           readerParties,
		accessTokenFile:         *tokenFile,
		maxContractsPerTemplate: *maxContracts,
		maxUpstreamConcurrency:  *maxUpstream,
		cacheTTL:                *cacheTTL,
		verbose:                 *verbose,
	}, nil
}

// ---------------------------------------------------------------------------
// run / main
// ---------------------------------------------------------------------------

// run builds and serves the service until ctx is canceled, then shuts down gracefully. It
// returns nil on a clean shutdown, including http.ErrServerClosed. Any other error is real.
func run(ctx context.Context, opts *options, stderr io.Writer) error {
	// Fail fast at startup when the token file is missing or unreadable. httpACSClient keeps
	// only the path; see readAccessToken's call in authHeader.
	if opts.accessTokenFile != "" {
		if _, err := readAccessToken(opts.accessTokenFile); err != nil {
			return err
		}
	}

	var acs acsClient = newHTTPACSClient(opts.jsonAPIBaseURL, opts.accessTokenFile)
	if opts.maxUpstreamConcurrency > 0 {
		acs = newBoundedACS(acs, opts.maxUpstreamConcurrency)
	}
	if opts.cacheTTL > 0 {
		acs = newCachingACS(acs, opts.cacheTTL)
	}
	srv := newServer(opts, acs)

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("disclosure-service: listen on %s: %w", opts.listen, err)
	}

	fmt.Fprintf(stderr, "disclosure-service: reader parties guardianGovernance=%s nttCustody=%s operator=%q, %d flow template(s)\n",
		opts.readerParties.guardianGovernance, opts.readerParties.nttCustody, opts.readerParties.operator, len(flowTemplateByTail))
	fmt.Fprintln(stderr, "disclosure-service: WARNING unauthenticated harness/ops-grade service -- do not expose beyond a trusted network")
	if host, _, splitErr := net.SplitHostPort(ln.Addr().String()); splitErr == nil {
		if host != "127.0.0.1" && host != "::1" && host != "localhost" {
			fmt.Fprintf(stderr, "disclosure-service: WARNING listening on non-loopback address %s\n", ln.Addr().String())
		}
	}
	fmt.Fprintf(stderr, "disclosure-service: listening on http://%s\n", ln.Addr().String())

	httpSrv := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		// A flow reads at most every flow template plus one ledger-end call.
		WriteTimeout: time.Duration(len(flowTemplateByTail)+1) * httpClientTimeout,
		IdleTimeout:  serverIdleTimeout,
	}
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
