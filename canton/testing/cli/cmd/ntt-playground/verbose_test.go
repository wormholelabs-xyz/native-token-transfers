package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// execPlayground runs the root command in-process with args, returning captured
// stdout/stderr. Exercises the CLI layer only -- `network status` against an empty run-dir
// reads a missing PID file and never spawns a process, so no dpm or ledger is needed.
func execPlayground(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	return out.String(), errOut.String()
}

// TestVerboseNarratesToStderr pins the --verbose contract: narration goes to stderr, every
// line carries the "[v] " prefix, and stdout stays byte-identical to a non-verbose run.
func TestVerboseNarratesToStderr(t *testing.T) {
	runDir := t.TempDir()
	base := []string{
		"--state-file", filepath.Join(runDir, "playground.state.json"),
		"--run-dir", runDir,
		"--dpm-path", "dpm-not-invoked", // status never executes dpm; keeps the test hermetic
		"network", "status",
	}

	quietOut, quietErr := execPlayground(t, base...)
	require.Contains(t, quietOut, "sandbox: not running")
	require.Empty(t, quietErr, "without --verbose, stderr must stay silent")

	verboseOut, verboseErr := execPlayground(t, append([]string{"--verbose"}, base...)...)
	require.Equal(t, quietOut, verboseOut, "--verbose must not change stdout")
	require.NotEmpty(t, verboseErr, "--verbose must narrate on stderr")
	for _, line := range strings.Split(strings.TrimRight(verboseErr, "\n"), "\n") {
		require.True(t, strings.HasPrefix(line, "[v] "), "verbose line %q must carry the [v] prefix", line)
	}
}
