//go:build integration

package canton

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// findDpm locates the dpm toolchain (PATH or ~/.dpm/bin), skipping the test if
// unavailable. Copied from wormhole's watcher_integration_test.go (this repo
// doesn't depend on that module).
func findDpm(t *testing.T) string {
	if p, err := exec.LookPath("dpm"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".dpm", "bin", "dpm")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	t.Skip("dpm not found (PATH or ~/.dpm/bin); skipping Canton integration test")
	return ""
}

// runCmd runs a command in dir and fails the test on non-zero exit.
func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
