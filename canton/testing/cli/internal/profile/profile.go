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
	// Cip56Custody against real Amulet never applies there.
	AmuletAvailable bool

	// UserID is the daml-script --user-id to run as (LocalNet's admin/ledger-api user,
	// "ledger-api-user"); empty for the sandbox, which needs no explicit user id.
	UserID string

	// JSONAPIBaseURL is the JSON Ledger API v2 base (user-rights grants, DAR upload, and --
	// for "amulet" -- the observer's update stream). Empty for the sandbox, which has no
	// auth and needs neither.
	JSONAPIBaseURL string

	// ValidatorBaseURL is the Splice validator API base (tap, TransferPreapproval, the
	// transfer-instruction registry's scan-proxy, DSO party lookup) -- internal/amulet's
	// target. Empty for the sandbox, which has no validator/DSO at all.
	ValidatorBaseURL string

	// Participants maps a participant role (e.g. "app-provider", "app-user", "bob",
	// "guardian-governance", "guardian-observer", "alice-solo") to that participant's
	// connection details. LocalNet has all six (see the plan's §2 port table); the sandbox --
	// a single in-process participant, per internal/network/sandbox.go -- has exactly one,
	// keyed "default".
	//
	// TODO(phase 3): nothing routes through this map yet. The existing top-level
	// LedgerHost/LedgerPort/JSONAPIBaseURL/ValidatorBaseURL/UserID fields above remain the
	// single source of truth every other package reads today (internal/ledger, party.go,
	// runner.go, ...); they always mirror Participants[DefaultParticipant]. Rewiring callers
	// to go through Endpoint(role) instead is phase 3's job.
	Participants map[string]Endpoint

	// DefaultParticipant is the Participants key the top-level fields above mirror --
	// "app-provider" for LocalNet, "default" for the sandbox.
	DefaultParticipant string
}

// Endpoint is one participant's connection details: everything internal/ledger and
// internal/network need to submit `dpm script` calls and JSON-API/validator-API requests
// against that specific participant.
type Endpoint struct {
	LedgerHost string
	LedgerPort int

	// JSONAPIBaseURL is empty for the sandbox (no auth, no JSON API needed there).
	JSONAPIBaseURL string

	// ValidatorBaseURL is empty for the sandbox (no validator/DSO at all).
	ValidatorBaseURL string

	// UserID is the daml-script --user-id to run as against this participant. "ledger-api-
	// user" on every LocalNet participant (see the plan's §2 provenance column); empty for
	// the sandbox.
	UserID string
}

// Endpoint returns the connection details for role, falling back to the profile's default
// participant when role is "" or does not name a configured participant (e.g. an unmapped
// party hint on a --topology-config whose partyHosting map doesn't cover it). It never
// errors: the error return exists only so a future stricter mode (e.g. an explicit
// --require-known-participant flag) can be added without a signature change.
func (p Profile) Endpoint(role string) (Endpoint, error) {
	if role == "" {
		role = p.DefaultParticipant
	}
	if ep, ok := p.Participants[role]; ok {
		return ep, nil
	}
	// Unknown role: fall back to the default endpoint rather than erroring. The plan's §3
	// sketch is ambiguous between erroring and falling back here; falling back is what makes
	// DefaultParticipant meaningful as a genuine fallback target (matching how
	// disclosure.Config's "*" wildcard degrades to app-provider), and keeps an unrecognized
	// hint from hard-failing a command outright.
	return p.Participants[p.DefaultParticipant], nil
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
		ep := Endpoint{
			LedgerHost: "localhost",
			LedgerPort: SandboxPort(),
		}
		return Profile{
			Name:               Sandbox,
			LedgerHost:         ep.LedgerHost,
			LedgerPort:         ep.LedgerPort,
			RequiresAuth:       false,
			UploadDAR:          true,
			AmuletAvailable:    false,
			Participants:       map[string]Endpoint{"default": ep},
			DefaultParticipant: "default",
		}, nil
	case LocalNet:
		host := envOr("LOCALNET_LEDGER_HOST", "localhost")
		port := 3901
		if v := os.Getenv("LOCALNET_LEDGER_PORT"); v != "" {
			_, _ = fmt.Sscanf(v, "%d", &port)
		}
		jsonAPI := envOr("LOCALNET_JSON_API_URL", "http://localhost:3975")
		validatorAPI := envOr("LOCALNET_VALIDATOR_URL", "http://localhost:3903")
		const userID = "ledger-api-user"

		// app-provider is the only endpoint the LOCALNET_* env overrides ever touch (see
		// TestGet_LocalNet_EnvOverrides_HitDefaultOnly) -- it is both the top-level
		// LedgerHost/... fields above AND Participants["app-provider"]. The other five
		// participants (see the plan's §2 port table) are localnet-only, bundle-fixed
		// ports with no env override of their own yet.
		appProvider := Endpoint{
			LedgerHost:       host,
			LedgerPort:       port,
			JSONAPIBaseURL:   jsonAPI,
			ValidatorBaseURL: validatorAPI,
			UserID:           userID,
		}
		localEndpoint := func(ledgerPort, jsonAPIPort, validatorPort int) Endpoint {
			return Endpoint{
				LedgerHost:       "localhost",
				LedgerPort:       ledgerPort,
				JSONAPIBaseURL:   fmt.Sprintf("http://localhost:%d", jsonAPIPort),
				ValidatorBaseURL: fmt.Sprintf("http://localhost:%d", validatorPort),
				UserID:           userID,
			}
		}

		return Profile{
			Name:             LocalNet,
			LedgerHost:       host,
			LedgerPort:       port,
			RequiresAuth:     true,
			UploadDAR:        true,
			AmuletAvailable:  true,
			UserID:           userID,
			JSONAPIBaseURL:   jsonAPI,
			ValidatorBaseURL: validatorAPI,
			Participants: map[string]Endpoint{
				"app-provider":        appProvider,
				"app-user":            localEndpoint(2901, 2975, 2903),
				"bob":                 localEndpoint(5901, 5975, 5903),
				"guardian-governance": localEndpoint(6901, 6975, 6903),
				"guardian-observer":   localEndpoint(7901, 7975, 7903),
				"alice-solo":          localEndpoint(8901, 8975, 8903),
			},
			DefaultParticipant: "app-provider",
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
