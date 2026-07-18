// Package profile describes the two ledgers the playground CLI can target: a bare `dpm
// sandbox` (fast, CI-viable, no auth) and a Splice LocalNet participant (real DSO +
// Canton Coin, unsafe shared-secret JWT auth). Every other package (ledger, network) takes a
// Profile rather than hardcoding host/port/auth assumptions, so adding a real-validator
// profile later (the CLI README's MainNet path) is a matter of adding a case here.
package profile

import (
	"fmt"
	"os"
)

// Name identifies a profile by its CLI flag value (--profile sandbox|localnet).
type Name string

const (
	Sandbox  Name = "sandbox"
	LocalNet Name = "localnet"
)

// Profile is everything the ledger/network layers need to talk to a given target.
type Profile struct {
	Name Name

	// LedgerHost/LedgerPort are the gRPC Ledger API `dpm script` connects to.
	LedgerHost string
	LedgerPort int

	// RequiresAuth is true when `dpm script` needs --access-token-file (LocalNet's unsafe
	// shared-secret JWTs); false for the unauthenticated sandbox.
	RequiresAuth bool

	// UploadDAR is true when the first script call against this profile should pass
	// --upload-dar true (sandbox auto-vets on the fly the same way; LocalNet needs it, or an
	// explicit prior POST to :3975/v2/packages -- the CLI always uses the --upload-dar flag
	// for simplicity on both profiles).
	UploadDAR bool

	// AmuletAvailable is true only for LocalNet -- the sandbox has no DSO/Amulet, so
	// Cip56Custody-against-real-Amulet (task 9, out of scope here) never applies there.
	AmuletAvailable bool

	// UserID is the daml-script --user-id to run as (LocalNet's admin/ledger-api user,
	// "ledger-api-user"); empty for the sandbox, which needs no explicit user id.
	UserID string

	// JSONAPIBaseURL is the JSON Ledger API v2 base (user-rights grants, DAR upload).
	// Empty for the sandbox, which has no auth and needs neither.
	JSONAPIBaseURL string
}

// SandboxPort returns the sandbox gRPC port: CANTON_SANDBOX_PORT if set (matching the
// existing canton/testing/go harness convention), else the dpm sandbox default 6865.
func SandboxPort() int {
	if v := os.Getenv("CANTON_SANDBOX_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil && port > 0 {
			return port
		}
	}
	return 6865
}

// Get returns the Profile for name, applying environment overrides (CANTON_SANDBOX_PORT for
// sandbox; LOCALNET_LEDGER_HOST/LOCALNET_LEDGER_PORT for localnet, matching the app-provider
// participant's gRPC Ledger API on port 3901 by default).
func Get(name Name) (Profile, error) {
	switch name {
	case Sandbox, "":
		return Profile{
			Name:            Sandbox,
			LedgerHost:      "localhost",
			LedgerPort:      SandboxPort(),
			RequiresAuth:    false,
			UploadDAR:       true,
			AmuletAvailable: false,
		}, nil
	case LocalNet:
		host := envOr("LOCALNET_LEDGER_HOST", "localhost")
		port := 3901
		if v := os.Getenv("LOCALNET_LEDGER_PORT"); v != "" {
			_, _ = fmt.Sscanf(v, "%d", &port)
		}
		return Profile{
			Name:            LocalNet,
			LedgerHost:      host,
			LedgerPort:      port,
			RequiresAuth:    true,
			UploadDAR:       true,
			AmuletAvailable: true,
			UserID:          "ledger-api-user",
			JSONAPIBaseURL:  envOr("LOCALNET_JSON_API_URL", "http://localhost:3975"),
		}, nil
	default:
		return Profile{}, fmt.Errorf("profile: unknown profile %q (want sandbox|localnet)", name)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
