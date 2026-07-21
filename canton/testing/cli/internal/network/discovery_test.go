package network

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeComposeYAML creates dir (and parents) and drops an empty compose.yaml inside it, so
// isLocalNetDir/ResolveLocalNetDir see it as a valid LocalNet bundle.
func writeComposeYAML(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose.yaml in %s: %v", dir, err)
	}
}

func TestResolveLocalNetDir_ExplicitEnvResolves(t *testing.T) {
	dir := t.TempDir()
	writeComposeYAML(t, dir)
	t.Setenv("LOCALNET_DIR", dir)

	got, err := ResolveLocalNetDir("")
	if err != nil {
		t.Fatalf("ResolveLocalNetDir: %v", err)
	}
	if got != dir {
		t.Fatalf("ResolveLocalNetDir: got %q, want %q", got, dir)
	}
}

func TestResolveLocalNetDir_ExplicitEnvMissingComposeErrors(t *testing.T) {
	dir := t.TempDir() // no compose.yaml written
	t.Setenv("LOCALNET_DIR", dir)

	_, err := ResolveLocalNetDir("")
	if err == nil {
		t.Fatal("ResolveLocalNetDir: expected an error for a LOCALNET_DIR with no compose.yaml")
	}
	if !strings.Contains(err.Error(), dir) || !strings.Contains(err.Error(), "compose.yaml") {
		t.Fatalf("ResolveLocalNetDir: error %q should name the bad dir and mention compose.yaml", err.Error())
	}
}

func TestResolveLocalNetDir_DiscoversHomeCandidateWhenEnvUnset(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")

	home := t.TempDir()
	t.Setenv("HOME", home)

	localnetDir := filepath.Join(home, "splice-node", "docker-compose", "localnet")
	writeComposeYAML(t, localnetDir)

	got, err := ResolveLocalNetDir("")
	if err != nil {
		t.Fatalf("ResolveLocalNetDir: %v", err)
	}
	if got != localnetDir {
		t.Fatalf("ResolveLocalNetDir: got %q, want %q", got, localnetDir)
	}
}

func TestResolveLocalNetDir_DiscoversCantonDirCacheWhenEnvUnset(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")

	// Point HOME somewhere with no candidates of its own, so only the cantonDir-relative
	// candidate can succeed -- isolates that candidate from the two HOME-based ones.
	home := t.TempDir()
	t.Setenv("HOME", home)

	cantonDir := t.TempDir()
	localnetDir := filepath.Join(cantonDir, "testing", ".localnet", "splice-node", "docker-compose", "localnet")
	writeComposeYAML(t, localnetDir)

	got, err := ResolveLocalNetDir(cantonDir)
	if err != nil {
		t.Fatalf("ResolveLocalNetDir: %v", err)
	}
	if got != localnetDir {
		t.Fatalf("ResolveLocalNetDir: got %q, want %q", got, localnetDir)
	}
}

func TestResolveLocalNetDir_NothingFoundListsCandidates(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")

	home := t.TempDir() // empty -- no splice-node under it, no .cache/ntt-playground either
	t.Setenv("HOME", home)

	cantonDir := t.TempDir() // empty -- no testing/.localnet under it either

	_, err := ResolveLocalNetDir(cantonDir)
	if err == nil {
		t.Fatal("ResolveLocalNetDir: expected an error when no candidate exists")
	}
	for _, want := range localNetDirCandidates(cantonDir) {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ResolveLocalNetDir: error %q should list candidate %q", err.Error(), want)
		}
	}
}
