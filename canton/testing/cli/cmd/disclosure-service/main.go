// Command disclosure-service is a standalone host for internal/disclosure.Service. It is an
// unauthenticated HTTP sidecar that fronts one participant's allow-listed contract disclosures
// and prepare-seam scripts. It binds to loopback by default. It differs from cmd/ntt-playground's
// `disclosure serve` (canton/playground-merged): it has no playground state file, no cobra, and
// no party bookkeeping. Every input is a flag. The only auth material it reads is
// --access-token-file; it mints no tokens.
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

// options holds every flag as a plain struct. This makes parseFlags, buildService, and run
// testable without a live dpm or ledger.
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

// verboseLogf returns a logger that writes "[v] "-prefixed lines to w. It returns nil when
// verbose is off. Same shape as cmd/ntt-playground/main.go's verboseLogf.
func verboseLogf(w io.Writer, verbose bool) func(string, ...any) {
	if !verbose {
		return nil
	}
	return func(format string, args ...any) {
		fmt.Fprintf(w, "[v] "+format+"\n", args...)
	}
}

// resolvedCantonDir finds the canton/ directory. It walks up from the working directory until
// it finds multi-package.yaml, the canton/ root's marker file. Local copy of
// cmd/ntt-playground/main.go's resolvedCantonDir (playground-merged). This binary has only
// --dar, no --canton-dir flag, so there is no override to check first.
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

// resolvedDarPath returns dar when it is non-empty. Otherwise it returns the default path under
// the found canton directory. It then stat-checks the result, with the same remediation text as
// cmd/ntt-playground/runner.go's newScriptRunnerFor. It runs per seam invocation, not at
// startup, to match the source's timing.
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

// buildService wires a disclosure.Service from opts. It loads the topology config and resolves
// the profile and endpoint. It attaches an ActiveContracts backend for GET /v1/disclosures only
// when the endpoint has a JSON API URL and --acs-party is set. The service reads
// --access-token-file once and trims it. The same file is the ACS bearer token and the token
// file that `dpm script` reads for POST /v1/seam/*. It is the only auth material this binary
// handles; the binary mints nothing.
func buildService(opts *options) (*disclosure.Service, profile.Profile, error) {
	cfg, err := disclosure.Load(opts.topologyConfig)
	if err != nil {
		return nil, profile.Profile{}, fmt.Errorf("disclosure-service: load topology config: %w", err)
	}
	prof, err := profile.Get(profile.Name(opts.profileName))
	if err != nil {
		return nil, profile.Profile{}, fmt.Errorf("disclosure-service: %w", err)
	}
	// Fail fast: a profile with auth needs --access-token-file even when the ACS backend is
	// off, because POST /v1/seam/* passes the file straight to `dpm script`.
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

// isLoopbackListenAddr reports whether addr's host part names a loopback interface:
// "127.0.0.1", "localhost", or "::1". Every other host counts as non-loopback. An empty host
// also counts as non-loopback, because net.Listen then binds every interface. Ported verbatim
// from cmd/ntt-playground/disclosure.go (playground-merged).
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

// run listens on opts.listen and prints the startup banner to stderr. It serves svc.Handler()
// until ctx is done, then shuts down with a 5-second timeout. The banner does not depend on
// --verbose: an e2e harness parses the "listening on http://" line to learn the resolved ":0"
// port, and the posture lines carry information, not narration. Both mirror
// cmd/ntt-playground/disclosure.go's RunE exactly.
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
	// Shut down when ctx is done (SIGINT/SIGTERM via main, or a test cancels).
	// srv.Serve then returns http.ErrServerClosed, which is not an error here.
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
