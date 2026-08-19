// Command disclosure-service is a standalone host for internal/disclosure.Service: an
// unauthenticated, loopback-by-default HTTP sidecar fronting one participant's allow-listed
// contract disclosures and prepare-seam scripts. Unlike cmd/ntt-playground's `disclosure serve`
// (canton/playground-merged), this binary carries no playground state file, cobra, or party
// bookkeeping -- every input is a flag, and --access-token-file is the only auth material it
// ever reads (no JWT minting).
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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
)

const (
	defaultListen      = "127.0.0.1:7599"
	defaultParticipant = "guardian-governance"
	defaultProfile     = "sandbox"
)

// options is every flag this binary takes, as a plain struct so parseFlags/buildService/run are
// each independently unit-testable without a live dpm/ledger.
type options struct {
	listen          string
	participant     string
	profileName     string
	topologyConfig  string
	dar             string
	dpmPath         string
	accessTokenFile string
	acsParty        string
	verbose         bool
}

func parseFlags(args []string) (*options, error) {
	fs := flag.NewFlagSet("disclosure-service", flag.ContinueOnError)
	opts := &options{}
	fs.StringVar(&opts.listen, "listen", defaultListen, "address to listen on (host:port)")
	fs.StringVar(&opts.participant, "participant", defaultParticipant, "participant role this service fronts (internal/profile.Profile.Participants key)")
	fs.StringVar(&opts.profileName, "profile", defaultProfile, "network profile: sandbox|localnet")
	fs.StringVar(&opts.topologyConfig, "topology-config", "", "path to the topology/disclosure config file (default: built-in localnet defaults; a missing file is not an error)")
	fs.StringVar(&opts.dar, "dar", "", "path to the compiled test DAR (default: discovered by walking up from the working directory for multi-package.yaml)")
	fs.StringVar(&opts.dpmPath, "dpm-path", "", "path to the dpm binary (default: PATH or ~/.dpm/bin)")
	fs.StringVar(&opts.accessTokenFile, "access-token-file", "", "path to a file containing a bearer JWT; the only auth input this binary accepts, required when --profile requires auth")
	fs.StringVar(&opts.acsParty, "acs-party", "", "reading party for GET /v1/disclosures; ACS stays disabled without this")
	fs.BoolVar(&opts.verbose, "verbose", false, "narrate every dpm script invocation on stderr")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return opts, nil
}

// verboseLogf returns a "[v] "-prefixed logger writing to w, or nil when verbose is off --
// mirrors cmd/ntt-playground/main.go's verboseLogf shape.
func verboseLogf(w io.Writer, verbose bool) func(string, ...any) {
	if !verbose {
		return nil
	}
	return func(format string, args ...any) {
		fmt.Fprintf(w, "[v] "+format+"\n", args...)
	}
}

// resolvedCantonDir discovers the canton/ directory by walking up from the working directory
// for multi-package.yaml (the canton/ root's marker file). Local copy of cmd/ntt-playground/
// main.go's resolvedCantonDir (playground-merged) -- this binary has no --canton-dir flag, only
// --dar, so there is no override to check first.
func resolvedCantonDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("disclosure-service: canton-dir: getwd: %w", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "multi-package.yaml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("disclosure-service: canton-dir: could not find multi-package.yaml by walking up from the working directory; pass --dar explicitly")
}

// resolvedDarPath returns dar if non-empty, else the default path under a discovered canton
// directory, then stat-checks whichever path resulted -- same remediation text as
// cmd/ntt-playground/runner.go's newScriptRunnerFor. Called lazily (per seam invocation, not at
// startup), matching the source's own timing.
func resolvedDarPath(dar string) (string, error) {
	if dar == "" {
		dir, err := resolvedCantonDir()
		if err != nil {
			return "", err
		}
		dar = filepath.Join(dir, "test", ".daml", "dist", "ntt-test-0.1.0.dar")
	}
	if _, err := os.Stat(dar); err != nil {
		return "", fmt.Errorf("disclosure-service: DAR not found at %s -- run `dpm build --all` in canton/ first: %w", dar, err)
	}
	return dar, nil
}

// buildService wires a disclosure.Service from opts: loads the topology config, resolves the
// profile/endpoint, and -- only when both an ACS backend is reachable (ep.JSONAPIBaseURL) and a
// reading party is given (--acs-party) -- attaches an ActiveContracts backend for
// GET /v1/disclosures. --access-token-file is read once, trimmed, and used both as the ACS
// bearer token and as the file `dpm script` reads for POST /v1/seam/* -- the only auth material
// this binary ever handles; it mints nothing.
func buildService(opts *options) (*disclosure.Service, profile.Profile, error) {
	cfg, err := disclosure.Load(opts.topologyConfig)
	if err != nil {
		return nil, profile.Profile{}, fmt.Errorf("disclosure-service: load topology config: %w", err)
	}
	prof, err := profile.Get(profile.Name(opts.profileName))
	if err != nil {
		return nil, profile.Profile{}, fmt.Errorf("disclosure-service: %w", err)
	}
	// Fail fast: a profile requiring auth needs --access-token-file regardless of whether ACS
	// is enabled, since POST /v1/seam/* also passes it straight to `dpm script`.
	if prof.RequiresAuth && opts.accessTokenFile == "" {
		return nil, profile.Profile{}, fmt.Errorf("disclosure-service: profile %s requires --access-token-file (used for dpm script auth and, when --acs-party is set, ACS reads) but none was given", prof.Name)
	}
	ep, err := prof.Endpoint(opts.participant)
	if err != nil {
		return nil, profile.Profile{}, fmt.Errorf("disclosure-service: %w", err)
	}

	logf := verboseLogf(os.Stderr, opts.verbose)

	var acs *disclosure.ActiveContracts
	if ep.JSONAPIBaseURL != "" && opts.acsParty != "" {
		var token string
		if prof.RequiresAuth {
			raw, err := os.ReadFile(opts.accessTokenFile)
			if err != nil {
				return nil, profile.Profile{}, fmt.Errorf("disclosure-service: read --access-token-file: %w", err)
			}
			token = strings.TrimSpace(string(raw))
		}
		acs = &disclosure.ActiveContracts{
			BaseURL: ep.JSONAPIBaseURL,
			Token:   token,
			Party:   opts.acsParty,
			Logf:    logf,
		}
	}

	svc := &disclosure.Service{
		Cfg:         cfg,
		Participant: opts.participant,
		ACS:         acs,
		NewRunner: func(ctx context.Context) (disclosure.ScriptRunner, func(), error) {
			darPath, err := resolvedDarPath(opts.dar)
			if err != nil {
				return nil, nil, err
			}
			workDir, err := os.MkdirTemp("", "disclosure-service-script-*")
			if err != nil {
				return nil, nil, fmt.Errorf("disclosure-service: create script work dir: %w", err)
			}
			cleanup := func() { _ = os.RemoveAll(workDir) }
			r, err := ledger.NewRunner(opts.dpmPath, darPath, prof, ep, opts.accessTokenFile, workDir)
			if err != nil {
				cleanup()
				return nil, nil, err
			}
			r.Logf = logf
			return r, cleanup, nil
		},
		Logf: logf,
	}
	return svc, prof, nil
}

// isLoopbackListenAddr reports whether addr's host part names a loopback interface --
// "127.0.0.1", "localhost", or "::1". Any other host, including an empty one (net.Listen binds
// every interface), is treated as non-loopback. Ported verbatim from cmd/ntt-playground/
// disclosure.go (playground-merged).
func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

// run listens on opts.listen, prints the startup banner to stderr, serves svc.Handler() until
// ctx is done, then shuts down gracefully (5s timeout). The banner's "listening on http://"
// line is not gated on --verbose: it is the address an e2e harness parses after an ephemeral
// ":0" port resolves, and the security posture lines are load-bearing information, not
// narration -- both mirror cmd/ntt-playground/disclosure.go's RunE exactly.
func run(ctx context.Context, opts *options, stderr io.Writer) error {
	svc, _, err := buildService(opts)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("disclosure serve: listen on %s: %w", opts.listen, err)
	}

	fmt.Fprintf(stderr, "disclosure serve: fronting participant=%s templates=%d\n", opts.participant, len(svc.Cfg.Templates()))
	fmt.Fprintln(stderr, "disclosure serve: unauthenticated harness-grade service; keep it loopback-bound unless you know why")
	if !isLoopbackListenAddr(opts.listen) {
		fmt.Fprintln(stderr, "disclosure serve: WARNING: --listen is not loopback-bound -- this exposes allow-listed contract payloads, unauthenticated, to anything that can reach this address")
	}
	fmt.Fprintf(stderr, "disclosure serve: listening on http://%s\n", ln.Addr().String())

	srv := &http.Server{Handler: svc.Handler()}
	// Shut down once ctx is done (SIGINT/SIGTERM via main, or a test cancellation).
	// srv.Serve then returns http.ErrServerClosed, which is not itself an error.
	context.AfterFunc(ctx, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	})

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("disclosure serve: %w", err)
	}
	return nil
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		os.Exit(1) // flag.ContinueOnError already printed usage/error to stderr
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, opts, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
