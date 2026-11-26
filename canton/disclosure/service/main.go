// Command disclosure-service is a production-runnable HTTP endpoint. /v1/flows/{flow} assembles
// one flow's disclosure set from one stakeholder participant's ACS over the JSON Ledger API v2.
// Each set comes from its choice body in canton/ntt/daml/Wormhole/Ntt/Manager.daml (see flows.go).
//
// Posture: the flow routes are public and serve each client. An edge proxy in front terminates
// TLS and applies a rate limit and a WAF. A non-loopback --listen needs --behind-edge-proxy.
// The service's own upstream credential is a bearer token file (--access-token-file) or an
// OAuth2 client-credentials grant (--oauth-*).
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
	// serverReadTimeout bounds the read of headers and body, so a slow body holds an inbound slot
	// for at most this long.
	serverReadTimeout     = 10 * time.Second
	serverIdleTimeout     = 2 * time.Minute
	maxInboundHeaderBytes = 8 << 10

	maxUpstreamConcurrencyLimit = 64
	maxCacheTTL                 = 60 * time.Second
	// minRegistryCacheTTL is the cache floor with the registry API: each instruments request
	// without the cache is a full Coin read.
	minRegistryCacheTTL          = 1 * time.Second
	maxContractsPerTemplateLimit = 10_000
)

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

// options is parseFlags' pure result. I/O happens later, when run() consumes it.
type options struct {
	listen                  string
	jsonAPIBaseURL          string
	readerParties           readerPartiesByRole
	upstreamAuth            upstreamAuthMode
	upstreamTLS             upstreamTLSFiles
	maxContractsPerTemplate int
	maxUpstreamConcurrency  int
	cacheTTL                time.Duration
	behindEdgeProxy         bool
	registryInstrumentsPath string // empty when the registry API is off
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
	tokenFile := fs.String("access-token-file", "", "path to a file holding a bearer token forwarded upstream; re-read on every upstream request")
	maxContracts := fs.Int("max-contracts-per-template", defaultMaxContractsPerTemplate, "cap on NttManager contracts in /v1/managers and in the registry instrument list, 1 to 10000; more is an error")
	maxUpstream := fs.Int("max-upstream-concurrency", defaultMaxUpstreamConcurrency, "cap on in-flight upstream ledger queries, 1 to 64; excess requests wait briefly, then get a 503")
	cacheTTL := fs.Duration("cache-ttl", defaultCacheTTL, "serve repeat ledger reads from memory for this long, 0 to 60s; 0 turns the cache off")
	behindEdgeProxy := fs.Bool("behind-edge-proxy", false, "state that an edge proxy terminates TLS and applies a rate limit and a WAF; required for a non-loopback --listen")
	caFile := fs.String("json-api-ca-file", "", "PEM CA bundle that verifies the JSON Ledger API server certificate; replaces the system roots (https only)")
	clientCertFile := fs.String("json-api-client-cert-file", "", "PEM client certificate for mutual TLS to the JSON Ledger API; needs --json-api-client-key-file (https only)")
	clientKeyFile := fs.String("json-api-client-key-file", "", "PEM private key for --json-api-client-cert-file (https only)")
	oauthTokenURL := fs.String("oauth-token-url", "", "OAuth2 token endpoint for a client-credentials grant; https, or http on loopback only; exclusive with --access-token-file")
	oauthClientID := fs.String("oauth-client-id", "", "OAuth2 client id")
	oauthSecretFile := fs.String("oauth-client-secret-file", "", "path to the OAuth2 client secret; re-read on every token refresh")
	oauthAudience := fs.String("oauth-audience", "", "OAuth2 audience of the JSON Ledger API")
	oauthScope := fs.String("oauth-scope", "", "OAuth2 scope (optional)")
	oauthCAFile := fs.String("oauth-ca-file", "", "PEM CA bundle that verifies the OAuth2 token endpoint certificate; replaces the system roots (https only)")
	registryInstrumentsFile := fs.String("registry-instruments-file", "", "serve the CIP-0056 registry API (v1) for the instruments that --guardian-governance-party administers; JSON object keyed by instrument id, each value {\"name\": ..., \"symbol\": ...}")
	verbose := fs.Bool("verbose", false, "verbose logging")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("disclosure-service: %w", err)
	}
	if *jsonAPI == "" {
		return nil, fmt.Errorf("disclosure-service: --json-api is required (base URL of the JSON Ledger API v2, e.g. http://127.0.0.1:6975)")
	}
	upstreamTLS := upstreamTLSFiles{caFile: *caFile, clientCertFile: *clientCertFile, clientKeyFile: *clientKeyFile}
	upstreamAuth, err := parseUpstreamAuthMode(*tokenFile, oauthClientFlags{tokenURL: *oauthTokenURL, clientID: *oauthClientID, clientSecretFile: *oauthSecretFile, audience: *oauthAudience, scope: *oauthScope, caFile: *oauthCAFile})
	if err != nil {
		return nil, err
	}
	if err := checkJSONAPIURL(*jsonAPI, upstreamAuth.sendsBearerToken(), upstreamTLS); err != nil {
		return nil, err
	}
	readerParties, err := parseReaderParties(*guardianGovernanceParty, *nttCustodyParty)
	if err != nil {
		return nil, err
	}
	if *listen == "" {
		return nil, fmt.Errorf("disclosure-service: --listen must not be empty")
	}
	if err := checkListenAddress(*listen, *behindEdgeProxy); err != nil {
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
	if *registryInstrumentsFile != "" && *cacheTTL < minRegistryCacheTTL {
		return nil, fmt.Errorf("disclosure-service: --cache-ttl must be at least %s with --registry-instruments-file, got %s", minRegistryCacheTTL, *cacheTTL)
	}
	return &options{
		listen:                  *listen,
		jsonAPIBaseURL:          *jsonAPI,
		readerParties:           readerParties,
		upstreamAuth:            upstreamAuth,
		upstreamTLS:             upstreamTLS,
		maxContractsPerTemplate: *maxContracts,
		maxUpstreamConcurrency:  *maxUpstream,
		cacheTTL:                *cacheTTL,
		behindEdgeProxy:         *behindEdgeProxy,
		registryInstrumentsPath: *registryInstrumentsFile,
		verbose:                 *verbose,
	}, nil
}

// checkListenAddress refuses a non-loopback bind unless the operator states that an edge proxy
// is in front.
func checkListenAddress(listenAddress string, behindEdgeProxy bool) error {
	host, _, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return fmt.Errorf("disclosure-service: --listen %q: %w", listenAddress, err)
	}
	if isLoopbackIPAddress(host) {
		return nil
	}
	if !behindEdgeProxy {
		return fmt.Errorf("disclosure-service: --listen %q is not a loopback IP address; set --behind-edge-proxy only when an edge proxy (TLS, rate limit, WAF) is in front", listenAddress)
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
	if err := checkListenAddress(opts.listen, opts.behindEdgeProxy); err != nil {
		return err
	}

	var registry *registryConfig
	if opts.registryInstrumentsPath != "" {
		loadedRegistry, err := loadRegistryConfig(opts)
		if err != nil {
			return err
		}
		registry = loadedRegistry
	}

	// Fail fast at startup on an unreadable or invalid TLS file.
	tlsConfig, err := loadUpstreamTLSConfig(opts.upstreamTLS, time.Now())
	if err != nil {
		return err
	}
	upstreamHTTPClient := newUpstreamHTTPClient(tlsConfig)

	tokens, err := newBearerTokenSource(opts.upstreamAuth)
	if err != nil {
		return err
	}

	if opts.maxUpstreamConcurrency < 1 || opts.maxUpstreamConcurrency > maxUpstreamConcurrencyLimit {
		return fmt.Errorf("disclosure-service: upstream concurrency %d is out of bounds", opts.maxUpstreamConcurrency)
	}
	var acs acsClient = newBoundedACS(newHTTPACSClient(opts.jsonAPIBaseURL, upstreamHTTPClient, tokens), opts.maxUpstreamConcurrency)
	if opts.cacheTTL > 0 {
		acs = newCachingACS(acs, opts.cacheTTL)
	}
	srv := newServer(opts, acs)
	srv.registry = registry

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("disclosure-service: listen on %s: %w", opts.listen, err)
	}
	//SECURITY: second boundary: check the bound address.
	if err := checkListenAddress(ln.Addr().String(), opts.behindEdgeProxy); err != nil {
		_ = ln.Close()
		return err
	}

	fmt.Fprintf(stderr, "disclosure-service: reader parties guardianGovernance=%s nttCustody=%s, %d flow template(s)\n",
		opts.readerParties.guardianGovernance, opts.readerParties.nttCustody, len(flowTemplateByTail))
	fmt.Fprintf(stderr, "disclosure-service: upstream auth: %s; upstream TLS: %s\n", describeUpstreamAuth(opts.upstreamAuth), describeUpstreamTLS(opts.upstreamTLS))
	if registry != nil {
		fmt.Fprintf(stderr, "disclosure-service: CIP-0056 registry API on for admin %s, %d configured instrument(s)\n", registry.adminParty, len(registry.displayByInstrumentID))
	}
	fmt.Fprintln(stderr, "disclosure-service: public flow routes; put an edge proxy (TLS, rate limit, WAF) in front")
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

func describeUpstreamTLS(files upstreamTLSFiles) string {
	roots := "system roots"
	if files.caFile != "" {
		roots = "custom CA"
	}
	if files.clientCertFile != "" {
		return roots + ", mutual TLS"
	}
	return roots
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
