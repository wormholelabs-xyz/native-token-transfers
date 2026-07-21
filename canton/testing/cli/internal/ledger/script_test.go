package ledger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
)

// ----------------------------------------------------------------------
// FindDpm
// ----------------------------------------------------------------------

func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestFindDpm_OnPATH(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "dpm")
	t.Setenv("PATH", dir)

	got, err := FindDpm()
	if err != nil {
		t.Fatalf("FindDpm: %v", err)
	}
	// exec.LookPath resolves to an absolute path; just confirm it's inside our fixture dir.
	if !strings.HasPrefix(got, dir) {
		t.Fatalf("expected dpm resolved from PATH (%s), got %q", dir, got)
	}
}

func TestFindDpm_FallsBackToDpmHomeBin(t *testing.T) {
	emptyPathDir := t.TempDir() // deliberately has no "dpm" binary
	t.Setenv("PATH", emptyPathDir)

	home := t.TempDir()
	t.Setenv("HOME", home)
	dpmBinDir := filepath.Join(home, ".dpm", "bin")
	if err := os.MkdirAll(dpmBinDir, 0o755); err != nil {
		t.Fatalf("mkdir ~/.dpm/bin: %v", err)
	}
	want := writeExecutable(t, dpmBinDir, "dpm")

	got, err := FindDpm()
	if err != nil {
		t.Fatalf("FindDpm: %v", err)
	}
	if got != want {
		t.Fatalf("FindDpm: got %q, want %q", got, want)
	}
}

func TestFindDpm_NotFoundAnywhere(t *testing.T) {
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)
	home := t.TempDir() // no .dpm/bin/dpm under here
	t.Setenv("HOME", home)

	_, err := FindDpm()
	if err == nil || !strings.Contains(err.Error(), "dpm not found") {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// isTransientLedgerError
// ----------------------------------------------------------------------

func TestIsTransientLedgerError(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"UNAVAILABLE io exception", "io.grpc.StatusRuntimeException: UNAVAILABLE: io exception\n...", true},
		{"socket reset", "Caused by: java.net.SocketException: Connection reset\n...", true},
		{"CoordinatedShutdown alone is not evidence of a transient failure", "Coordinated Shutdown ran successfully", false},
		{"unrelated failure", "script raised an exception: DivideByZero", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTransientLedgerError([]byte(c.out)); got != c.want {
				t.Fatalf("isTransientLedgerError(%q) = %v, want %v", c.out, got, c.want)
			}
		})
	}
}

// ----------------------------------------------------------------------
// isLedgerUnreachableError / scriptFailureErr
// ----------------------------------------------------------------------

func TestIsLedgerUnreachableError(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"connection refused", "Caused by: java.net.ConnectException: Connection refused (Connection refused)", true},
		{"UNAVAILABLE io exception", "io.grpc.StatusRuntimeException: UNAVAILABLE: io exception\n...", true},
		{"unrelated dpm failure", "script raised an exception: DivideByZero", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isLedgerUnreachableError([]byte(c.out)); got != c.want {
				t.Fatalf("isLedgerUnreachableError(%q) = %v, want %v", c.out, got, c.want)
			}
		})
	}
}

// TestRun_LedgerUnreachableProducesFriendlyError exercises the classifier end to end: a fake
// dpm binary that always fails with the "Connection refused: localhost:6865" signature should
// make Run return the short, actionable message (naming the profile and host:port) instead of
// the raw dpm output, after exhausting the retry budget.
func TestRun_LedgerUnreachableProducesFriendlyError(t *testing.T) {
	dpm := fakeDpmScript(t, `echo "io.grpc.StatusRuntimeException: UNAVAILABLE: io exception" >&2
echo "Caused by: java.net.ConnectException: Connection refused: localhost:6865" >&2
exit 1
`)
	r := &Runner{
		DpmPath:       dpm,
		DarPath:       "unused.dar",
		WorkDir:       t.TempDir(),
		RetryAttempts: 1,
		RetryInterval: time.Millisecond,
		Profile:       profile.Profile{Name: profile.LocalNet, LedgerHost: "localhost", LedgerPort: 6865},
	}
	err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
	want := "cannot reach the localnet ledger at localhost:6865 -- is it running? " +
		"start it with 'just localnet-cli' (or 'ntt-playground --profile localnet network up'), or check --profile"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected the friendly ledger-unreachable message, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "io.grpc.StatusRuntimeException") {
		t.Fatalf("expected the raw dpm stack to be suppressed from the default error, got %q", err.Error())
	}
}

// TestRun_LedgerUnreachableNarratesRawOutputUnderVerbose confirms the raw dpm output stays
// available for debugging when Logf is set (the app's --verbose path), even though the
// returned error itself remains the clean one-liner.
func TestRun_LedgerUnreachableNarratesRawOutputUnderVerbose(t *testing.T) {
	dpm := fakeDpmScript(t, `echo "Connection refused: localhost:6865" >&2
exit 1
`)
	var lines []string
	r := &Runner{
		DpmPath:       dpm,
		DarPath:       "unused.dar",
		WorkDir:       t.TempDir(),
		RetryAttempts: 0,
		RetryInterval: time.Millisecond,
		Profile:       profile.Profile{Name: profile.Sandbox, LedgerHost: "localhost", LedgerPort: 6865},
		Logf:          func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) },
	}
	err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot reach the sandbox ledger") {
		t.Fatalf("expected the friendly message, got %v", err)
	}
	found := false
	for _, l := range lines {
		if strings.Contains(l, "Connection refused") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the raw dpm output to be narrated via Logf under --verbose, got lines: %v", lines)
	}
}

// TestRun_NonConnectivityFailureIsUnaffected confirms an unrelated dpm failure still surfaces
// its real message (unchanged from before the ledger-unreachable classifier was added).
func TestRun_NonConnectivityFailureIsUnaffected(t *testing.T) {
	dpm := fakeDpmScript(t, `echo "script raised an unrelated exception: DivideByZero" >&2
exit 1
`)
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir(), RetryInterval: time.Millisecond}
	err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "DivideByZero") {
		t.Fatalf("expected the real dpm error to pass through, got %v", err)
	}
	if strings.Contains(err.Error(), "cannot reach the") {
		t.Fatalf("did not expect the friendly ledger-unreachable message here, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Runner: retry/timeout defaults
// ----------------------------------------------------------------------

func TestRunner_RetryDefaults(t *testing.T) {
	var r Runner
	if got := r.retryAttempts(); got != 3 {
		t.Fatalf("default retryAttempts: got %d, want 3", got)
	}
	if got := r.retryInterval(); got != 5*time.Second {
		t.Fatalf("default retryInterval: got %s, want 5s", got)
	}

	r.RetryAttempts = 7
	r.RetryInterval = 250 * time.Millisecond
	if got := r.retryAttempts(); got != 7 {
		t.Fatalf("overridden retryAttempts: got %d", got)
	}
	if got := r.retryInterval(); got != 250*time.Millisecond {
		t.Fatalf("overridden retryInterval: got %s", got)
	}
}

// ----------------------------------------------------------------------
// NewRunner
// ----------------------------------------------------------------------

func TestNewRunner_ExplicitDpmPath(t *testing.T) {
	r, err := NewRunner("/explicit/dpm", "/some.dar", profile.Profile{}, "", t.TempDir())
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if r.DpmPath != "/explicit/dpm" {
		t.Fatalf("DpmPath mismatch: %q", r.DpmPath)
	}
}

func TestNewRunner_ResolvesDpmFromPATH(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "dpm")
	t.Setenv("PATH", dir)

	r, err := NewRunner("", "/some.dar", profile.Profile{}, "", t.TempDir())
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if !strings.HasPrefix(r.DpmPath, dir) {
		t.Fatalf("expected DpmPath resolved from PATH, got %q", r.DpmPath)
	}
}

func TestNewRunner_DpmNotFound(t *testing.T) {
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)
	t.Setenv("HOME", t.TempDir())

	_, err := NewRunner("", "/some.dar", profile.Profile{}, "", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "dpm not found") {
		t.Fatalf("expected a dpm-not-found error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Run: request-construction failures that never invoke dpm
// ----------------------------------------------------------------------

func TestRun_MarshalInputError(t *testing.T) {
	r := &Runner{DpmPath: "unused", DarPath: "unused.dar", WorkDir: t.TempDir()}
	// A channel can never be marshaled to JSON.
	badInput := struct{ Ch chan int }{Ch: make(chan int)}
	err := r.Run(context.Background(), "Some:script", badInput, nil)
	if err == nil || !strings.Contains(err.Error(), "marshal input") {
		t.Fatalf("expected a marshal-input error, got %v", err)
	}
}

func TestRun_MissingAccessTokenFileForAuthRequiredProfile(t *testing.T) {
	dpm := writeExecutable(t, t.TempDir(), "dpm")
	r := &Runner{
		DpmPath: dpm,
		DarPath: "unused.dar",
		WorkDir: t.TempDir(),
		Profile: profile.Profile{Name: profile.LocalNet, RequiresAuth: true},
	}
	err := r.Run(context.Background(), "Some:script", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "requires --access-token-file") {
		t.Fatalf("expected a missing-access-token-file error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Run: exercising the real subprocess path via a fake dpm script.
// ----------------------------------------------------------------------

// fakeDpmScript writes an executable POSIX shell script that pulls --output-file from its
// argv and behaves per body (a shell fragment with $out already bound to that path).
func fakeDpmScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "dpm")
	script := `#!/bin/sh
out=""
while [ $# -gt 0 ]; do
  case "$1" in
    --output-file) out="$2"; shift 2;;
    *) shift;;
  esac
done
` + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake dpm script: %v", err)
	}
	return path
}

func TestRun_Success(t *testing.T) {
	dpm := fakeDpmScript(t, `echo '{"managerId": 42}' > "$out"
exit 0
`)
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir(), Profile: profile.Profile{LedgerHost: "localhost", LedgerPort: 6865}}

	var out struct {
		ManagerID int `json:"managerId"`
	}
	if err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{"x": 1}, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.ManagerID != 42 {
		t.Fatalf("output decode mismatch: %+v", out)
	}
}

func TestRun_FailureIsNotRetriedWhenNotTransient(t *testing.T) {
	dpm := fakeDpmScript(t, `echo "script raised an unrelated exception" >&2
exit 1
`)
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir(), RetryInterval: time.Millisecond}
	err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "dpm script") {
		t.Fatalf("expected a dpm-script-failed error, got %v", err)
	}
}

func TestRun_RetriesTransientErrorThenSucceeds(t *testing.T) {
	counterFile := filepath.Join(t.TempDir(), "count")
	dpm := fakeDpmScript(t, `
count=0
if [ -f "`+counterFile+`" ]; then count=$(cat "`+counterFile+`"); fi
count=$((count+1))
echo "$count" > "`+counterFile+`"
if [ "$count" -lt 2 ]; then
  echo "io.grpc.StatusRuntimeException: UNAVAILABLE: io exception" >&2
  exit 1
fi
echo '{"managerId": 7}' > "$out"
exit 0
`)
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir(), RetryInterval: time.Millisecond}

	var out struct {
		ManagerID int `json:"managerId"`
	}
	if err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.ManagerID != 7 {
		t.Fatalf("expected the retry's success output, got %+v", out)
	}
	raw, _ := os.ReadFile(counterFile)
	if strings.TrimSpace(string(raw)) != "2" {
		t.Fatalf("expected exactly 2 attempts, counter file says %q", raw)
	}
}

func TestRun_RetriesExhaustedReturnsError(t *testing.T) {
	// "SocketException: Connection reset" is transient (retry-worthy, per isTransientLedgerError)
	// but is deliberately not one of isLedgerUnreachableError's signatures, so this keeps testing
	// retry-exhaustion producing the generic dpm-script error, distinct from the
	// ledger-unreachable friendly-message tests above.
	dpm := fakeDpmScript(t, `echo "java.net.SocketException: Connection reset" >&2
exit 1
`)
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir(), RetryAttempts: 1, RetryInterval: time.Millisecond}
	err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "dpm script") {
		t.Fatalf("expected a dpm-script-failed error after exhausting retries, got %v", err)
	}
}

func TestRun_CtxCancelledDuringRetryWait(t *testing.T) {
	// See TestRun_RetriesExhaustedReturnsError: "SocketException: Connection reset" is
	// transient/retry-worthy but not one of isLedgerUnreachableError's signatures.
	dpm := fakeDpmScript(t, `echo "java.net.SocketException: Connection reset" >&2
exit 1
`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir(), RetryInterval: time.Minute}
	err := r.Run(ctx, "Playground.Ops:setPeer", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "dpm script") {
		t.Fatalf("expected a dpm-script-failed error once ctx is cancelled mid-retry-wait, got %v", err)
	}
}

func TestRun_MissingOutputFile(t *testing.T) {
	dpm := fakeDpmScript(t, `exit 0
`) // succeeds but never writes $out
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir()}
	err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, nil)
	if err == nil || !strings.Contains(err.Error(), "read output file for") {
		t.Fatalf("expected a read-output-file error, got %v", err)
	}
}

func TestRun_MalformedOutputJSON(t *testing.T) {
	dpm := fakeDpmScript(t, `echo 'not json' > "$out"
exit 0
`)
	r := &Runner{DpmPath: dpm, DarPath: "unused.dar", WorkDir: t.TempDir()}
	var out map[string]any
	err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, &out)
	if err == nil || !strings.Contains(err.Error(), "parse output for") {
		t.Fatalf("expected a parse-output error, got %v", err)
	}
}

func TestRun_FullFlagWiringForAuthedUploadDARProfile(t *testing.T) {
	var gotArgs []string
	dpm := fakeDpmScript(t, `echo '{}' > "$out"
exit 0
`)
	// Capture argv by wrapping: fakeDpmScript already consumes --output-file; append a
	// second script that also dumps argv to a file for inspection.
	argsFile := filepath.Join(t.TempDir(), "args")
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "dpm")
	script := `#!/bin/sh
echo "$@" > "` + argsFile + `"
` + strings.TrimPrefix(mustReadFile(t, dpm), "#!/bin/sh\n")
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}

	tokenFile := filepath.Join(t.TempDir(), "token.jwt")
	if err := os.WriteFile(tokenFile, []byte("tok"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	r := &Runner{
		DpmPath:         wrapper,
		DarPath:         "unused.dar",
		WorkDir:         t.TempDir(),
		AccessTokenFile: tokenFile,
		Profile: profile.Profile{
			Name:         profile.LocalNet,
			LedgerHost:   "localhost",
			LedgerPort:   3901,
			RequiresAuth: true,
			UploadDAR:    true,
			UserID:       "ledger-api-user",
		},
	}
	var out map[string]any
	if err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read captured args: %v", err)
	}
	gotArgs = strings.Fields(string(raw))
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"--upload-dar true", "--access-token-file " + tokenFile, "--user-id ledger-api-user", "--ledger-host localhost", "--ledger-port 3901"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected args to contain %q, got %q", want, joined)
		}
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// TestRun_LogfNarratesWithoutPanicking exercises the Logf-set path (input/duration
// narration), which is otherwise silent (nil-safe) in every other test above.
func TestRun_LogfNarratesWithoutPanicking(t *testing.T) {
	dpm := fakeDpmScript(t, `echo '{}' > "$out"
exit 0
`)
	var lines []string
	r := &Runner{
		DpmPath: dpm,
		DarPath: "unused.dar",
		WorkDir: t.TempDir(),
		Profile: profile.Profile{LedgerHost: "localhost", LedgerPort: 6865, UserID: "u"},
		Logf:    func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) },
	}
	var out map[string]any
	if err := r.Run(context.Background(), "Playground.Ops:setPeer", map[string]any{}, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(lines) == 0 {
		t.Fatalf("expected Logf to be called at least once")
	}
}
