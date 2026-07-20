// Package ledger runs Daml Scripts via `dpm script`, marshaling Go structs to/from the JSON
// files the daml-script CLI reads/writes. This is deliberately the CLI's only way to touch
// the ledger (no Go gRPC client, per the design note in the playground plan): disclosures,
// multi-party actAs, and interface exercising are already solved in Daml Script, and the
// pattern is proven by the existing canton/testing/go harness. Sandbox and LocalNet differ
// only in host/port/auth/upload flags, all carried by profile.Profile.
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

// Runner invokes `dpm script` for one DAR against one profile.
type Runner struct {
	DpmPath string
	DarPath string
	Profile profile.Profile

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

// NewRunner constructs a Runner, resolving dpm from PATH/~/.dpm/bin if dpmPath is empty.
func NewRunner(dpmPath, darPath string, p profile.Profile, accessTokenFile, workDir string) (*Runner, error) {
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
		"--ledger-host", r.Profile.LedgerHost,
		"--ledger-port", strconv.Itoa(r.Profile.LedgerPort),
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
	if r.Profile.UserID != "" {
		args = append(args, "--user-id", r.Profile.UserID)
	}

	if r.Logf != nil {
		auth, user := "none", "-"
		if r.Profile.RequiresAuth {
			auth = "bearer"
		}
		if r.Profile.UserID != "" {
			user = r.Profile.UserID
		}
		r.logf("script %s → dpm script @ %s:%d (upload-dar=%t auth=%s user=%s)",
			scriptName, r.Profile.LedgerHost, r.Profile.LedgerPort, r.Profile.UploadDAR, auth, user)
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
			return fmt.Errorf("ledger: dpm script %s failed: %w\n%s", scriptName, runErr, out)
		}
		r.logf("script %s: transient ledger connectivity error (attempt %d/%d), retrying in %s",
			scriptName, attempt+1, r.retryAttempts(), r.retryInterval())
		select {
		case <-time.After(r.retryInterval()):
		case <-ctx.Done():
			return fmt.Errorf("ledger: dpm script %s failed: %w\n%s", scriptName, runErr, out)
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
