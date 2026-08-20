// Command disclosure-service is a production-runnable HTTP endpoint. It serves createdEventBlobs
// for a hardcoded allow-list of Daml templates. It reads the templates from one stakeholder
// participant's ACS over the JSON Ledger API v2. This binary is the transport half. The
// canonical per-flow disclosure sets live in the Daml library Wormhole.Ntt.Disclosure. This
// binary serves the flat union of templates that library names.
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
	"strings"
	"syscall"
	"time"
)

const (
	defaultListen                  = "127.0.0.1:7599"
	defaultMaxContractsPerTemplate = 1000
	httpClientTimeout              = 30 * time.Second
	shutdownTimeout                = 5 * time.Second
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
	disclosingParties       []string
	accessTokenFile         string
	allowListPath           string
	maxContractsPerTemplate int
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
	disclosingParty := fs.String("disclosing-party", "", "comma-separated disclosing parties: the parties as which the service reads the ledger; every served disclosure is a contract at least one of them sees; the access token must grant readAs for each (required)")
	tokenFile := fs.String("access-token-file", "", "path to a file holding a bearer token forwarded upstream; re-read on every upstream request")
	allowList := fs.String("allow-list", "", "path to a JSON array of package-qualified \"#name:Module:Entity\" template names, overriding the built-in allow-list")
	maxContracts := fs.Int("max-contracts-per-template", defaultMaxContractsPerTemplate, "cap on active contracts returned per template; a template that exceeds it is an error")
	verbose := fs.Bool("verbose", false, "verbose logging")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("disclosure-service: %w", err)
	}
	if *jsonAPI == "" {
		return nil, fmt.Errorf("disclosure-service: --json-api is required (base URL of the JSON Ledger API v2, e.g. http://localhost:6975)")
	}
	if *disclosingParty == "" {
		return nil, fmt.Errorf("disclosure-service: --disclosing-party is required (the party or comma-separated parties as which the service reads the ledger)")
	}
	parties := make([]string, 0, 4)
	seenParty := make(map[string]bool, 4)
	for _, p := range strings.Split(*disclosingParty, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("disclosure-service: --disclosing-party has an empty entry")
		}
		if seenParty[p] {
			return nil, fmt.Errorf("disclosure-service: --disclosing-party lists %s twice", p)
		}
		seenParty[p] = true
		parties = append(parties, p)
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
		disclosingParties:       parties,
		accessTokenFile:         *tokenFile,
		allowListPath:           *allowList,
		maxContractsPerTemplate: *maxContracts,
		verbose:                 *verbose,
	}, nil
}

// ---------------------------------------------------------------------------
// run / main
// ---------------------------------------------------------------------------

// run builds and serves the service until ctx is canceled, then shuts down gracefully. It
// returns nil on a clean shutdown, including http.ErrServerClosed. Any other error is real.
func run(ctx context.Context, opts *options, stderr io.Writer) error {
	allowList := defaultAllowList()
	if opts.allowListPath != "" {
		var err error
		allowList, err = loadAllowList(opts.allowListPath)
		if err != nil {
			return err
		}
	}

	// Fail fast at startup when the token file is missing or unreadable. httpACSClient keeps
	// only the path; see readAccessToken's call in authHeader.
	if opts.accessTokenFile != "" {
		if _, err := readAccessToken(opts.accessTokenFile); err != nil {
			return err
		}
	}

	acs := newHTTPACSClient(opts.jsonAPIBaseURL, opts.accessTokenFile)
	srv := newServer(opts, allowList, acs)

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("disclosure-service: listen on %s: %w", opts.listen, err)
	}

	fmt.Fprintf(stderr, "disclosure-service: disclosing parties %s, %d allow-listed template(s)\n", strings.Join(opts.disclosingParties, ", "), len(allowList))
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
		// The no-param default queries every allow-listed template plus one ledger-end call, each
		// bounded by httpClientTimeout, so the write timeout scales with the allow-list.
		WriteTimeout: time.Duration(len(allowList)+1) * httpClientTimeout,
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
