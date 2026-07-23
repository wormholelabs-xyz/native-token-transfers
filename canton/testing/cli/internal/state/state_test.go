package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	s := New()
	if s.Deployments == nil || s.Emitters == nil || s.GuardianSequences == nil || s.Users == nil {
		t.Fatalf("New must initialize every map, got %+v", s)
	}
	if len(s.Deployments) != 0 || len(s.Emitters) != 0 {
		t.Fatalf("New must return empty maps, got %+v", s)
	}
}

// ----------------------------------------------------------------------
// Load
// ----------------------------------------------------------------------

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil || !strings.Contains(err.Error(), "state: read") {
		t.Fatalf("expected a read error, got %v", err)
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "state: parse") {
		t.Fatalf("expected a parse error, got %v", err)
	}
}

func TestLoad_NilMapsAreNormalized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sparse.json")
	// A state file that omits every map field entirely.
	if err := os.WriteFile(path, []byte(`{"profile":"sandbox","operator":"op::abc"}`), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Deployments == nil || s.Emitters == nil || s.GuardianSequences == nil || s.Users == nil {
		t.Fatalf("Load must normalize nil maps, got %+v", s)
	}
	if s.Operator != "op::abc" {
		t.Fatalf("operator mismatch: %q", s.Operator)
	}
}

func TestLoad_PopulatedFieldsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "full.json")
	raw := `{
		"profile": "localnet",
		"operator": "op::abc",
		"guardianGovernance": "gov::abc",
		"guardianObserver": "obs::abc",
		"guardian": {"privateKeyHex": "aa", "address": "bb"},
		"deployments": {
			"ntt1": {
				"name": "ntt1",
				"managerId": 1,
				"managerAddress": "mgr",
				"transceiverAddress": "xcv",
				"admin": "admin::abc",
				"mode": "burn-mint",
				"tokenKind": "mock",
				"tokenDecimals": 6,
				"peers": {"2": {"managerAddress": "pm", "transceiverAddress": "pt"}}
			}
		},
		"emitters": {"e1": {"emitterId": 5, "address": "addr", "owner": "owner::abc"}},
		"users": {"Alice": "alice::abc"},
		"guardianSequences": {"transfer:2": 3}
	}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Profile != "localnet" || s.GuardianGovernance != "gov::abc" || s.GuardianObserver != "obs::abc" {
		t.Fatalf("top-level field mismatch: %+v", s)
	}
	if s.Guardian.PrivateKeyHex != "aa" || s.Guardian.Address != "bb" {
		t.Fatalf("guardian mismatch: %+v", s.Guardian)
	}
	d, ok := s.Deployment("ntt1")
	if !ok || d.ManagerID != 1 || d.Mode != "burn-mint" {
		t.Fatalf("deployment mismatch: ok=%v %+v", ok, d)
	}
	p, ok := d.Peer(2)
	if !ok || p.ManagerAddress != "pm" || p.TransceiverAddress != "pt" {
		t.Fatalf("peer mismatch: ok=%v %+v", ok, p)
	}
	e, ok := s.Emitter("e1")
	if !ok || e.EmitterID != 5 || e.Owner != "owner::abc" {
		t.Fatalf("emitter mismatch: ok=%v %+v", ok, e)
	}
	if s.Users["Alice"] != "alice::abc" {
		t.Fatalf("users mismatch: %+v", s.Users)
	}
	if s.GuardianSequences["transfer:2"] != 3 {
		t.Fatalf("guardianSequences mismatch: %+v", s.GuardianSequences)
	}
}

func TestLoad_RoundTripsInstrumentFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instrument.json")
	raw := `{
		"deployments": {
			"ntt1": {
				"name": "ntt1",
				"mode": "lock-unlock",
				"tokenKind": "mock",
				"instrumentAdmin": "gg::abc",
				"instrumentId": "wormhole-ntt:deadbeef"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	d, ok := s.Deployment("ntt1")
	if !ok || d.InstrumentAdmin != "gg::abc" || d.InstrumentID != "wormhole-ntt:deadbeef" {
		t.Fatalf("instrument field round-trip mismatch: ok=%v %+v", ok, d)
	}
}

func TestLoad_RejectsLegacyCustodyPartyField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-custody.json")
	raw := `{
		"deployments": {
			"cc-custody": {
				"name": "cc-custody",
				"tokenKind": "cip56-custody",
				"custodyParty": "custody::abc",
				"custodyUser": "cc-custody-custody"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected a rejection error for a legacy custodyParty-bearing state file")
	}
	if !strings.Contains(err.Error(), "custodyParty") || !strings.Contains(err.Error(), "pre-CIP-56-rework") {
		t.Fatalf("expected a migration-hinting error naming custodyParty, got: %v", err)
	}
}

func TestLoad_RejectsLegacyTokenKind(t *testing.T) {
	for _, kind := range []string{"mock-admin-signed", "cip56-burn-mint-mock", "cip56-custody-mock", "cip56-custody"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "legacy-kind.json")
			raw := `{"deployments": {"ntt1": {"name": "ntt1", "tokenKind": "` + kind + `"}}}`
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatalf("seed file: %v", err)
			}
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected a rejection error for legacy tokenKind %q", kind)
			}
			if !strings.Contains(err.Error(), kind) || !strings.Contains(err.Error(), "mock") {
				t.Fatalf("expected a migration-hinting error naming %q and the new kinds, got: %v", kind, err)
			}
		})
	}
}

func TestLoad_AcceptsCurrentTokenKinds(t *testing.T) {
	for _, kind := range []string{"mock", "amulet"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "current-kind.json")
			raw := `{"deployments": {"ntt1": {"name": "ntt1", "tokenKind": "` + kind + `"}}}`
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatalf("seed file: %v", err)
			}
			if _, err := Load(path); err != nil {
				t.Fatalf("Load should accept current tokenKind %q, got: %v", kind, err)
			}
		})
	}
}

// ----------------------------------------------------------------------
// Save
// ----------------------------------------------------------------------

func TestSave_WritesPrettyPrintedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	s := New()
	s.Operator = "op::abc"
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(raw), "\n  \"operator\"") {
		t.Fatalf("expected pretty-printed (indented) JSON, got %s", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm mismatch: got %v", info.Mode().Perm())
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load round-trip: %v", err)
	}
	if loaded.Operator != "op::abc" {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

func TestSave_UnwritablePathReturnsError(t *testing.T) {
	// The parent directory doesn't exist, so os.WriteFile must fail.
	path := filepath.Join(t.TempDir(), "missing-subdir", "out.json")
	s := New()
	err := s.Save(path)
	if err == nil || !strings.Contains(err.Error(), "state: write") {
		t.Fatalf("expected a write error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// NextGuardianSequence / lookups
// ----------------------------------------------------------------------

func TestNextGuardianSequence(t *testing.T) {
	s := New()
	if got := s.NextGuardianSequence("transfer", 2); got != 1 {
		t.Fatalf("first sequence: got %d, want 1", got)
	}
	if got := s.NextGuardianSequence("transfer", 2); got != 2 {
		t.Fatalf("second sequence: got %d, want 2", got)
	}
	// A different (kind, chain) key must not share the counter.
	if got := s.NextGuardianSequence("transfer", 3); got != 1 {
		t.Fatalf("distinct chain sequence: got %d, want 1", got)
	}
	if got := s.NextGuardianSequence("governance", 2); got != 1 {
		t.Fatalf("distinct kind sequence: got %d, want 1", got)
	}
}

func TestDeployment_NotFound(t *testing.T) {
	s := New()
	_, ok := s.Deployment("missing")
	if ok {
		t.Fatalf("expected ok=false for a missing deployment")
	}
}

func TestEmitter_NotFound(t *testing.T) {
	s := New()
	_, ok := s.Emitter("missing")
	if ok {
		t.Fatalf("expected ok=false for a missing emitter")
	}
}

func TestDeployment_Peer_NotFound(t *testing.T) {
	d := Deployment{Peers: map[int]Peer{}}
	_, ok := d.Peer(99)
	if ok {
		t.Fatalf("expected ok=false for a missing peer")
	}
}
