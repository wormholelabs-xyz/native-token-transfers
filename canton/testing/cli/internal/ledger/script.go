// Package ledger runs Daml Scripts via `dpm script`, marshaling Go structs to and from the
// JSON files the daml-script CLI reads and writes. This is the CLI's only way to touch the
// ledger; there is no Go gRPC client. Daml Script already handles disclosures, multi-party
// actAs, and interface exercising, and the existing canton/testing/go harness proves the
// pattern. The sandbox and LocalNet profiles differ only in host, port, auth, and upload
// flags, all carried by profile.Profile.
package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
)

// FindDpm locates the dpm toolchain on PATH or in ~/.dpm/bin, matching
// canton/testing/go/helpers_test.go:findDpm.
func FindDpm() (string, error) {
	if p, err := exec.LookPath("dpm"); err == nil {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".dpm", "bin", "dpm")
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("ledger: dpm not found (PATH or ~/.dpm/bin)")
}

// Runner invokes `dpm script` for one DAR against one participant endpoint.
type Runner struct {
	DpmPath string
	DarPath string

	// Profile carries the profile-wide settings that don't vary per participant:
	// RequiresAuth, UploadDAR, Name (used only for narration/error messages). Host/port/
	// user-id come from Endpoint below, not from this profile's own top-level fields --
	// see the plan's §3 ("thread an Endpoint, not the whole Profile, through to wherever
	// host/port/user-id are consumed").
	Profile profile.Profile

	// Endpoint is the specific participant this Runner submits `dpm script` calls against
	// -- resolved via Profile.Endpoint(role) by the caller (cmd/ntt-playground/runner.go's
	// newScriptRunnerFor). Every LocalNet participant shares the same RequiresAuth/UploadDAR/
	// Name from Profile above; only LedgerHost/LedgerPort/UserID (and, for callers outside
	// this package, JSONAPIBaseURL/ValidatorBaseURL) vary per participant.
	Endpoint profile.Endpoint

	// AccessTokenFile is a path to a file containing a bearer JWT, used when
	// Profile.RequiresAuth is true (LocalNet). Ignored for the sandbox.
	AccessTokenFile string

	// WorkDir holds the per-call input/output JSON files (a temp dir the caller owns).
	WorkDir string

	// Logf, when non-nil, narrates each script invocation (target, input, duration). The
	// caller-supplied closure owns any prefix; nil (the default) disables narration.
	Logf func(format string, args ...any)

	// RetryAttempts is how many EXTRA attempts (beyond the first) to make when `dpm script`
	// fails with a transient-looking ledger connectivity error (see isTransientLedgerError):
	// a brief participant gRPC blip, observed live under host resource pressure, not a
	// script/logic failure. Zero uses the default (3).
	RetryAttempts int

	// RetryInterval is the pause between retry attempts. Zero uses the default (5s).
	RetryInterval time.Duration

	callCount int
}

// logf forwards to Logf when set -- nil-safe, so instrumentation never needs a guard.
func (r *Runner) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}

func (r *Runner) retryAttempts() int {
	if r.RetryAttempts > 0 {
		return r.RetryAttempts
	}
	return 3
}

func (r *Runner) retryInterval() time.Duration {
	if r.RetryInterval > 0 {
		return r.RetryInterval
	}
	return 5 * time.Second
}

// isTransientLedgerError reports whether out (a FAILED `dpm script` invocation's combined
// stdout/stderr) looks like a transient connectivity blip to the participant's gRPC port
// rather than a genuine script/logic failure. Pinned to the exact signatures observed live
// (a host under heavy memory pressure caused the participant to drop an in-flight gRPC
// connection mid-suite): `io.grpc.StatusRuntimeException: UNAVAILABLE` and the underlying
// `java.net.SocketException: Connection reset`. Never matches on "CoordinatedShutdown" alone
// -- that line is printed by every daml-script JVM exit, success or failure, and is not by
// itself evidence of anything.
func isTransientLedgerError(out []byte) bool {
	s := string(out)
	for _, sig := range []string{
		"UNAVAILABLE: io exception",
		"SocketException: Connection reset",
	} {
		if strings.Contains(s, sig) {
			return true
		}
	}
	return false
}

// isLedgerUnreachableError reports whether out (a FAILED `dpm script` invocation's combined
// stdout/stderr) looks like the target ledger simply isn't listening on its host:port -- a
// fresh TCP refusal, or the gRPC UNAVAILABLE/io-exception dpm-script raises for the same
// condition. Deliberately shares signatures with isTransientLedgerError: that function decides
// whether to retry, this one decides how to report the failure once the retry budget (if any)
// is exhausted. At that point there is no user-facing difference between "the ledger dropped
// the connection and never came back" and "the ledger was never up at all" -- both mean it
// isn't reachable right now, e.g. because --profile doesn't match the ledger that's running.
func isLedgerUnreachableError(out []byte) bool {
	s := string(out)
	for _, sig := range []string{
		"Connection refused",
		"UNAVAILABLE: io exception",
	} {
		if strings.Contains(s, sig) {
			return true
		}
	}
	return false
}

// scriptFailureErr builds the error Run returns for a FAILED dpm script invocation. When out
// matches isLedgerUnreachableError, it returns a short, actionable message naming the profile
// and host:port instead of dumping the raw Java/gRPC stack -- that stack is still narrated to
// stderr under --verbose (via Logf) so debugging stays possible. Any other failure keeps
// surfacing its real message plus the raw dpm output, unchanged.
func (r *Runner) scriptFailureErr(scriptName string, runErr error, out []byte) error {
	if isLedgerUnreachableError(out) {
		if r.Logf != nil {
			r.logf("script %s failed: raw dpm output:\n%s", scriptName, out)
		}
		return fmt.Errorf(
			"ledger: cannot reach the %s ledger at %s:%d -- is it running? "+
				"start it with 'just localnet-cli' (or 'ntt-playground --profile %s network up'), or check --profile",
			r.Profile.Name, r.Endpoint.LedgerHost, r.Endpoint.LedgerPort, r.Profile.Name)
	}
	return fmt.Errorf("ledger: dpm script %s failed: %w\n%s", scriptName, runErr, out)
}

// NewRunner constructs a Runner, resolving dpm from PATH/~/.dpm/bin if dpmPath is empty. ep
// is the specific participant endpoint this Runner submits against (see Runner.Endpoint) --
// callers resolve it via p.Endpoint(role) before calling NewRunner.
func NewRunner(dpmPath, darPath string, p profile.Profile, ep profile.Endpoint, accessTokenFile, workDir string) (*Runner, error) {
	if dpmPath == "" {
		var err error
		dpmPath, err = FindDpm()
		if err != nil {
			return nil, err
		}
	}
	return &Runner{
		DpmPath:         dpmPath,
		DarPath:         darPath,
		Profile:         p,
		Endpoint:        ep,
		AccessTokenFile: accessTokenFile,
		WorkDir:         workDir,
	}, nil
}

// Run invokes scriptName (a fully qualified Module:name) with input marshaled to JSON,
// unmarshaling the script's JSON output into output. Every call re-uploads the DAR
// (--upload-dar true) -- idempotent against a ledger that already vetted it, and the only
// robust option across separate CLI process invocations that share no in-memory state.
func (r *Runner) Run(ctx context.Context, scriptName string, input, output any) error {
	r.callCount++
	inputPath := filepath.Join(r.WorkDir, fmt.Sprintf("in-%d.json", r.callCount))
	outputPath := filepath.Join(r.WorkDir, fmt.Sprintf("out-%d.json", r.callCount))

	inputRaw, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("ledger: marshal input for %s: %w", scriptName, err)
	}
	if err := os.WriteFile(inputPath, inputRaw, 0o600); err != nil {
		return fmt.Errorf("ledger: write input file: %w", err)
	}

	args := []string{
		"script",
		"--dar", r.DarPath,
		"--script-name", scriptName,
		"--ledger-host", r.Endpoint.LedgerHost,
		"--ledger-port", strconv.Itoa(r.Endpoint.LedgerPort),
		"--input-file", inputPath,
		"--output-file", outputPath,
	}
	if r.Profile.UploadDAR {
		args = append(args, "--upload-dar", "true")
	}
	if r.Profile.RequiresAuth {
		if r.AccessTokenFile == "" {
			return fmt.Errorf("ledger: profile %s requires --access-token-file but none was set", r.Profile.Name)
		}
		args = append(args, "--access-token-file", r.AccessTokenFile)
	}
	if r.Endpoint.UserID != "" {
		args = append(args, "--user-id", r.Endpoint.UserID)
	}

	if r.Logf != nil {
		auth, user := "none", "-"
		if r.Profile.RequiresAuth {
			auth = "bearer"
		}
		if r.Endpoint.UserID != "" {
			user = r.Endpoint.UserID
		}
		r.logf("script %s → dpm script @ %s:%d (upload-dar=%t auth=%s user=%s)",
			scriptName, r.Endpoint.LedgerHost, r.Endpoint.LedgerPort, r.Profile.UploadDAR, auth, user)
		// Inputs are parties/addresses/VAA hex -- never the guardian private key, which is
		// signed off-ledger and never crosses this boundary.
		r.logf("script %s input: %s", scriptName, inputRaw)
	}

	start := time.Now()
	var out []byte
	var runErr error
	for attempt := 0; ; attempt++ {
		cmd := exec.CommandContext(ctx, r.DpmPath, args...)
		out, runErr = cmd.CombinedOutput()
		if runErr == nil {
			break
		}
		if attempt >= r.retryAttempts() || !isTransientLedgerError(out) {
			return r.scriptFailureErr(scriptName, runErr, out)
		}
		r.logf("script %s: transient ledger connectivity error (attempt %d/%d), retrying in %s",
			scriptName, attempt+1, r.retryAttempts(), r.retryInterval())
		select {
		case <-time.After(r.retryInterval()):
		case <-ctx.Done():
			return r.scriptFailureErr(scriptName, runErr, out)
		}
	}
	r.logf("script %s ok (%s)", scriptName, time.Since(start).Round(time.Millisecond))

	outputRaw, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("ledger: read output file for %s: %w\n%s", scriptName, err, out)
	}
	if output != nil {
		if err := json.Unmarshal(outputRaw, output); err != nil {
			return fmt.Errorf("ledger: parse output for %s: %w\nraw: %s", scriptName, err, outputRaw)
		}
	}
	return nil
}
