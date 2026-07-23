package main

import (
	"strings"
	"testing"
)

// ----------------------------------------------------------------------
// parseTokenKind
// ----------------------------------------------------------------------

// TestTokenKindParse_MockAndAmuletOnly pins the CIP-56 rework's tokenKind migration: only
// "mock"/"amulet" are valid, and each of the pre-rework 4-value scheme's strings is rejected
// with a clear hint at the new kind that replaces it, rather than a generic "unknown" error.
func TestTokenKindParse_MockAndAmuletOnly(t *testing.T) {
	t.Run("current kinds accepted", func(t *testing.T) {
		for _, kind := range []string{"mock", "amulet"} {
			got, err := parseTokenKind(kind)
			if err != nil {
				t.Fatalf("parseTokenKind(%q): unexpected error: %v", kind, err)
			}
			if got != kind {
				t.Fatalf("parseTokenKind(%q) = %q, want %q", kind, got, kind)
			}
		}
	})

	t.Run("legacy kinds rejected with a migration hint", func(t *testing.T) {
		cases := map[string]string{
			"mock-admin-signed":    "mock",
			"cip56-burn-mint-mock": "mock",
			"cip56-custody-mock":   "mock",
			"cip56-custody":        "amulet",
		}
		for legacy, wantHint := range cases {
			_, err := parseTokenKind(legacy)
			if err == nil {
				t.Fatalf("parseTokenKind(%q): expected an error, got none", legacy)
			}
			if !strings.Contains(err.Error(), legacy) {
				t.Fatalf("parseTokenKind(%q): error should name the legacy kind, got: %v", legacy, err)
			}
			if !strings.Contains(err.Error(), wantHint) {
				t.Fatalf("parseTokenKind(%q): error should hint at %q, got: %v", legacy, wantHint, err)
			}
		}
	})

	t.Run("unknown kind rejected", func(t *testing.T) {
		_, err := parseTokenKind("bogus")
		if err == nil || !strings.Contains(err.Error(), "unknown tokenKind") {
			t.Fatalf("parseTokenKind(\"bogus\"): expected an unknown-tokenKind error, got: %v", err)
		}
	})
}

// ----------------------------------------------------------------------
// validateDeployConfig
// ----------------------------------------------------------------------

// TestDeployConfig_AmuletRequiresLockUnlock pins that an "amulet" deployment must be
// lock-unlock (Splice's Amulet registry has no BurnMintFactory to bridge burn/mint against);
// "mock" deployments are unconstrained by mode.
func TestDeployConfig_AmuletRequiresLockUnlock(t *testing.T) {
	cases := []struct {
		name    string
		cfg     deployConfig
		wantErr bool
	}{
		{"amulet + lock-unlock ok", deployConfig{TokenKind: "amulet", Mode: "lock-unlock"}, false},
		{"amulet + burn-mint rejected", deployConfig{TokenKind: "amulet", Mode: "burn-mint"}, true},
		{"mock + burn-mint ok", deployConfig{TokenKind: "mock", Mode: "burn-mint"}, false},
		{"mock + lock-unlock ok", deployConfig{TokenKind: "mock", Mode: "lock-unlock"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateDeployConfig(c.cfg)
			if c.wantErr && err == nil {
				t.Fatalf("validateDeployConfig(%+v): expected an error, got none", c.cfg)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("validateDeployConfig(%+v): unexpected error: %v", c.cfg, err)
			}
			if c.wantErr && !strings.Contains(err.Error(), "lock-unlock") {
				t.Fatalf("validateDeployConfig(%+v): error should mention lock-unlock, got: %v", c.cfg, err)
			}
		})
	}
}
