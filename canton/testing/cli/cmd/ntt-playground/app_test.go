package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// ----------------------------------------------------------------------
// resolvedCantonDir
// ----------------------------------------------------------------------

func TestResolvedCantonDir_ExplicitFlagWins(t *testing.T) {
	a := &app{cantonDir: "/explicit/canton"}
	got, err := a.resolvedCantonDir()
	if err != nil {
		t.Fatalf("resolvedCantonDir: %v", err)
	}
	if got != "/explicit/canton" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvedCantonDir_DiscoveredByWalkingUp(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "multi-package.yaml"), []byte("---\n"), 0o600); err != nil {
		t.Fatalf("seed marker file: %v", err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	t.Chdir(nested)

	a := &app{}
	got, err := a.resolvedCantonDir()
	if err != nil {
		t.Fatalf("resolvedCantonDir: %v", err)
	}
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(root)
	if gotResolved != wantResolved {
		t.Fatalf("resolvedCantonDir: got %q, want %q", got, root)
	}
}

func TestResolvedCantonDir_NotFound(t *testing.T) {
	// A deep-enough temp dir with no multi-package.yaml anywhere above it within 8 hops.
	dir := t.TempDir()
	nested := dir
	for i := 0; i < 8; i++ {
		nested = filepath.Join(nested, "d")
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(nested)

	a := &app{}
	_, err := a.resolvedCantonDir()
	if err == nil || !strings.Contains(err.Error(), "could not find multi-package.yaml") {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// darPath
// ----------------------------------------------------------------------

func TestDarPath(t *testing.T) {
	a := &app{cantonDir: "/canton-root"}
	got, err := a.darPath()
	if err != nil {
		t.Fatalf("darPath: %v", err)
	}
	want := filepath.Join("/canton-root", "test", ".daml", "dist", "ntt-test-0.1.0.dar")
	if got != want {
		t.Fatalf("darPath: got %q, want %q", got, want)
	}
}

func TestDarPath_PropagatesCantonDirError(t *testing.T) {
	dir := t.TempDir()
	nested := dir
	for i := 0; i < 8; i++ {
		nested = filepath.Join(nested, "d")
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(nested)

	a := &app{}
	_, err := a.darPath()
	if err == nil {
		t.Fatalf("expected darPath to propagate resolvedCantonDir's error")
	}
}

// ----------------------------------------------------------------------
// resolvedRunDir
// ----------------------------------------------------------------------

func TestResolvedRunDir_ExplicitFlagWins(t *testing.T) {
	a := &app{runDir: "/explicit/run"}
	if got := a.resolvedRunDir(); got != "/explicit/run" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvedRunDir_DefaultDerivedFromStateFile(t *testing.T) {
	a := &app{stateFile: "/some/dir/playground.state.json"}
	want := filepath.Join("/some/dir", ".ntt-playground-run")
	if got := a.resolvedRunDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolvedRunDir_StateFileWithNoDirComponent(t *testing.T) {
	a := &app{stateFile: "playground.state.json"}
	want := filepath.Join(".", ".ntt-playground-run")
	if got := a.resolvedRunDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// ----------------------------------------------------------------------
// loadState / saveState
// ----------------------------------------------------------------------

func TestLoadState_MissingFile(t *testing.T) {
	a := &app{stateFile: filepath.Join(t.TempDir(), "missing.json")}
	_, err := a.loadState()
	if err == nil || !strings.Contains(err.Error(), "run `init` first") {
		t.Fatalf("expected a run-init-first error, got %v", err)
	}
}

func TestSaveState_ThenLoadState_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playground.state.json")
	a := &app{stateFile: path}
	s := state.New()
	s.Operator = "op::abc"
	if err := a.saveState(s); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	loaded, err := a.loadState()
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if loaded.Operator != "op::abc" {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

// ----------------------------------------------------------------------
// resolvedProfile
// ----------------------------------------------------------------------

func TestResolvedProfile_Sandbox(t *testing.T) {
	a := &app{profile: profile.Sandbox}
	p, err := a.resolvedProfile()
	if err != nil {
		t.Fatalf("resolvedProfile: %v", err)
	}
	if p.Name != profile.Sandbox {
		t.Fatalf("got %+v", p)
	}
}

func TestResolvedProfile_Unknown(t *testing.T) {
	a := &app{profile: profile.Name("bogus")}
	_, err := a.resolvedProfile()
	if err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("expected an unknown-profile error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// resolveProfileFromState
// ----------------------------------------------------------------------

func TestResolveProfileFromState_StateFileWinsWhenFlagNotSet(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Profile = "localnet"
	if err := s.Save(stateFile); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	a := &app{stateFile: stateFile, profile: profile.Sandbox}
	root := newRootCmd()
	root.SetArgs([]string{"--state-file", stateFile, "status"})
	// Parse flags without running the command, so cmd.Flags().Changed reflects only what was
	// passed on argv (here, nothing touches --profile).
	if err := root.ParseFlags([]string{"--state-file", stateFile, "status"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	a.resolveProfileFromState(root)
	if a.profile != profile.LocalNet {
		t.Fatalf("expected the state file's profile to win, got %q", a.profile)
	}
}

func TestResolveProfileFromState_ExplicitFlagWins(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Profile = "localnet"
	if err := s.Save(stateFile); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	a := &app{stateFile: stateFile, profile: profile.Sandbox}
	root := newRootCmd()
	if err := root.ParseFlags([]string{"--state-file", stateFile, "--profile", "sandbox", "status"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	a.resolveProfileFromState(root)
	if a.profile != profile.Sandbox {
		t.Fatalf("expected the explicit --profile to win, got %q", a.profile)
	}
}

func TestResolveProfileFromState_NoStateFileKeepsDefault(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "missing.json")
	a := &app{stateFile: stateFile, profile: profile.Sandbox}
	root := newRootCmd()
	if err := root.ParseFlags([]string{"--state-file", stateFile, "status"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	a.resolveProfileFromState(root)
	if a.profile != profile.Sandbox {
		t.Fatalf("expected the sandbox default to survive a missing state file, got %q", a.profile)
	}
}

func TestResolveProfileFromState_EmptyRecordedProfileKeepsDefault(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	if err := state.New().Save(stateFile); err != nil { // Profile field left at its zero value
		t.Fatalf("seed state: %v", err)
	}
	a := &app{stateFile: stateFile, profile: profile.Sandbox}
	root := newRootCmd()
	if err := root.ParseFlags([]string{"--state-file", stateFile, "status"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	a.resolveProfileFromState(root)
	if a.profile != profile.Sandbox {
		t.Fatalf("expected the sandbox default to survive an empty recorded profile, got %q", a.profile)
	}
}

// ----------------------------------------------------------------------
// vlogf / verboseLogf
// ----------------------------------------------------------------------

func TestVerboseLogf_NilWhenNotVerbose(t *testing.T) {
	a := &app{verbose: false}
	if a.verboseLogf() != nil {
		t.Fatalf("expected a nil logger when --verbose is off")
	}
}

func TestVerboseLogf_WritesToStderrWhenSet(t *testing.T) {
	var buf strings.Builder
	a := &app{verbose: true, stderr: &buf}
	logf := a.verboseLogf()
	if logf == nil {
		t.Fatalf("expected a non-nil logger when --verbose is on")
	}
	logf("hello %d", 1)
	if !strings.Contains(buf.String(), "[v] hello 1") {
		t.Fatalf("expected the [v]-prefixed line, got %q", buf.String())
	}
}

// ----------------------------------------------------------------------
// resolveParty: cache-hit branch (no ledger round-trip)
// ----------------------------------------------------------------------

func TestResolveParty_CacheHit(t *testing.T) {
	a := &app{}
	root := newRootCmd()
	root.SetContext(context.Background())

	s := state.New()
	s.Users["Alice"] = "alice::abc"

	party, err := resolveParty(root, a, s, "Alice")
	if err != nil {
		t.Fatalf("resolveParty: %v", err)
	}
	if party != "alice::abc" {
		t.Fatalf("expected the cached party, got %q", party)
	}
}

func TestResolveParty_EmptyHint(t *testing.T) {
	a := &app{}
	root := newRootCmd()
	root.SetContext(context.Background())
	_, err := resolveParty(root, a, state.New(), "")
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected an empty-hint error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// grantActAs: no-op on a profile without auth (sandbox)
// ----------------------------------------------------------------------

func TestGrantActAs_NoAuthProfileIsNoOp(t *testing.T) {
	a := &app{profile: profile.Sandbox}
	root := newRootCmd()
	root.SetContext(context.Background())
	if err := grantActAs(root, a, "some::party"); err != nil {
		t.Fatalf("grantActAs on the sandbox profile must be a no-op, got %v", err)
	}
}

