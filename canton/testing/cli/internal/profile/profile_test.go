package profile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSandboxPort_Default pins the fallback when CANTON_SANDBOX_PORT is unset.
func TestSandboxPort_Default(t *testing.T) {
	t.Setenv("CANTON_SANDBOX_PORT", "")
	require.Equal(t, 6865, SandboxPort())
}

// TestSandboxPort_EnvOverride pins CANTON_SANDBOX_PORT taking precedence when set to a
// valid positive integer.
func TestSandboxPort_EnvOverride(t *testing.T) {
	t.Setenv("CANTON_SANDBOX_PORT", "7001")
	require.Equal(t, 7001, SandboxPort())
}

// TestSandboxPort_InvalidEnvFallsBack proves a malformed or non-positive
// CANTON_SANDBOX_PORT is ignored rather than propagated as a bad port.
func TestSandboxPort_InvalidEnvFallsBack(t *testing.T) {
	for _, v := range []string{"not-a-port", "0", "-5"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("CANTON_SANDBOX_PORT", v)
			require.Equal(t, 6865, SandboxPort())
		})
	}
}

// TestGet_Sandbox pins the sandbox profile's fixed shape, including the "" alias.
func TestGet_Sandbox(t *testing.T) {
	t.Setenv("CANTON_SANDBOX_PORT", "")
	for _, name := range []Name{Sandbox, ""} {
		p, err := Get(name)
		require.NoError(t, err)
		require.Equal(t, Sandbox, p.Name)
		require.Equal(t, "localhost", p.LedgerHost)
		require.Equal(t, 6865, p.LedgerPort)
		require.False(t, p.RequiresAuth)
		require.True(t, p.UploadDAR)
		require.False(t, p.AmuletAvailable)
	}
}

// TestGet_LocalNet_Defaults pins localnet's defaults when no env overrides are set.
func TestGet_LocalNet_Defaults(t *testing.T) {
	for _, k := range []string{"LOCALNET_LEDGER_HOST", "LOCALNET_LEDGER_PORT", "LOCALNET_JSON_API_URL", "LOCALNET_VALIDATOR_URL"} {
		t.Setenv(k, "")
	}
	p, err := Get(LocalNet)
	require.NoError(t, err)
	require.Equal(t, LocalNet, p.Name)
	require.Equal(t, "localhost", p.LedgerHost)
	require.Equal(t, 3901, p.LedgerPort)
	require.True(t, p.RequiresAuth)
	require.True(t, p.UploadDAR)
	require.True(t, p.AmuletAvailable)
	require.Equal(t, "ledger-api-user", p.UserID)
	require.Equal(t, "http://localhost:3975", p.JSONAPIBaseURL)
	require.Equal(t, "http://localhost:3903", p.ValidatorBaseURL)
}

// TestGet_LocalNet_EnvOverrides pins every localnet env-var override branch, including a
// malformed LOCALNET_LEDGER_PORT being silently ignored (Sscanf's error is swallowed, per
// Get's `_, _ = fmt.Sscanf(...)`).
func TestGet_LocalNet_EnvOverrides(t *testing.T) {
	t.Setenv("LOCALNET_LEDGER_HOST", "canton.example")
	t.Setenv("LOCALNET_LEDGER_PORT", "4001")
	t.Setenv("LOCALNET_JSON_API_URL", "http://json.example")
	t.Setenv("LOCALNET_VALIDATOR_URL", "http://validator.example")

	p, err := Get(LocalNet)
	require.NoError(t, err)
	require.Equal(t, "canton.example", p.LedgerHost)
	require.Equal(t, 4001, p.LedgerPort)
	require.Equal(t, "http://json.example", p.JSONAPIBaseURL)
	require.Equal(t, "http://validator.example", p.ValidatorBaseURL)
}

// TestGet_LocalNet_MalformedPortIgnored proves a non-numeric LOCALNET_LEDGER_PORT leaves
// the port at its 3901 default rather than erroring (Get discards Sscanf's error).
func TestGet_LocalNet_MalformedPortIgnored(t *testing.T) {
	t.Setenv("LOCALNET_LEDGER_HOST", "")
	t.Setenv("LOCALNET_LEDGER_PORT", "not-a-port")
	t.Setenv("LOCALNET_JSON_API_URL", "")
	t.Setenv("LOCALNET_VALIDATOR_URL", "")

	p, err := Get(LocalNet)
	require.NoError(t, err)
	require.Equal(t, 3901, p.LedgerPort)
}

// TestGet_UnknownProfile pins the error path for an unrecognized profile name.
func TestGet_UnknownProfile(t *testing.T) {
	_, err := Get(Name("bogus"))
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown profile "bogus"`)
}

// ----------------------------------------------------------------------
// Multi-participant topology (.claude/tasks/e2e-separate-participants.md §2/§3)
// ----------------------------------------------------------------------

// TestGet_LocalNet_Participants pins the exact six-participant port table from the plan's §2
// (extended in §6.1 with alice-solo): app-provider is unchanged (3901/3975/3903); app-user/
// bob/guardian-governance/guardian-observer/alice-solo are the new participants at
// 2*/5*/6*/7*/8* respectively.
func TestGet_LocalNet_Participants(t *testing.T) {
	for _, k := range []string{"LOCALNET_LEDGER_HOST", "LOCALNET_LEDGER_PORT", "LOCALNET_JSON_API_URL", "LOCALNET_VALIDATOR_URL"} {
		t.Setenv(k, "")
	}
	p, err := Get(LocalNet)
	require.NoError(t, err)
	require.Len(t, p.Participants, 6)
	require.Equal(t, "app-provider", p.DefaultParticipant)

	cases := []struct {
		role                                   string
		ledgerPort, jsonAPIPort, validatorPort int
	}{
		{"app-provider", 3901, 3975, 3903},
		{"app-user", 2901, 2975, 2903},
		{"bob", 5901, 5975, 5903},
		{"guardian-governance", 6901, 6975, 6903},
		{"guardian-observer", 7901, 7975, 7903},
		{"alice-solo", 8901, 8975, 8903},
	}
	for _, c := range cases {
		ep, ok := p.Participants[c.role]
		require.Truef(t, ok, "missing participant %q", c.role)
		require.Equal(t, c.ledgerPort, ep.LedgerPort, "role %q ledger port", c.role)
		require.Equal(t, fmt.Sprintf("http://localhost:%d", c.jsonAPIPort), ep.JSONAPIBaseURL, "role %q JSON API", c.role)
		require.Equal(t, fmt.Sprintf("http://localhost:%d", c.validatorPort), ep.ValidatorBaseURL, "role %q validator API", c.role)
		require.Equal(t, "ledger-api-user", ep.UserID, "role %q user id", c.role)
		require.Equal(t, "localhost", ep.LedgerHost, "role %q ledger host", c.role)
	}
}

// TestEndpoint_DefaultRole proves both "" and the default role name resolve to the SAME
// endpoint.
func TestEndpoint_DefaultRole(t *testing.T) {
	p, err := Get(LocalNet)
	require.NoError(t, err)

	byEmpty, err := p.Endpoint("")
	require.NoError(t, err)
	byName, err := p.Endpoint("app-provider")
	require.NoError(t, err)
	require.Equal(t, byName, byEmpty)
	require.Equal(t, 3901, byEmpty.LedgerPort)
}

// TestEndpoint_UnknownRole proves an unrecognized role string falls back to the default
// endpoint rather than erroring.
//
// NOTE (documented per the phase-1 plan): the plan's §3 sketch (empty string or unknown role -> default)
// is ambiguous about whether an unknown (non-empty) role should error or silently fall back;
// this test pins the fallback choice, since that is what makes DefaultParticipant useful as a
// genuine fallback target (mirroring disclosure.Config's "*" wildcard) rather than requiring
// every caller to special-case an unknown-role error.
func TestEndpoint_UnknownRole(t *testing.T) {
	p, err := Get(LocalNet)
	require.NoError(t, err)

	got, err := p.Endpoint("some-role-nobody-configured")
	require.NoError(t, err)
	def, err := p.Endpoint("")
	require.NoError(t, err)
	require.Equal(t, def, got)
}

// TestGet_Sandbox_SingleEndpoint proves the sandbox profile degenerates to a single
// participant (multi-participant routing is LocalNet-only per the plan's §2).
func TestGet_Sandbox_SingleEndpoint(t *testing.T) {
	p, err := Get(Sandbox)
	require.NoError(t, err)
	require.Len(t, p.Participants, 1)

	ep, err := p.Endpoint("")
	require.NoError(t, err)
	require.Equal(t, 6865, ep.LedgerPort)
}

// TestGet_LocalNet_EnvOverrides_HitDefaultOnly proves the existing LOCALNET_LEDGER_HOST/PORT/
// JSON_API_URL/VALIDATOR_URL overrides only affect the default (app-provider) endpoint, never
// the other five participants (including alice-solo, added in §6.1) -- those are still
// bundle-fixed ports (phase 2's job to make configurable, if ever needed).
func TestGet_LocalNet_EnvOverrides_HitDefaultOnly(t *testing.T) {
	t.Setenv("LOCALNET_LEDGER_HOST", "canton.example")
	t.Setenv("LOCALNET_LEDGER_PORT", "4001")
	t.Setenv("LOCALNET_JSON_API_URL", "http://json.example")
	t.Setenv("LOCALNET_VALIDATOR_URL", "http://validator.example")

	p, err := Get(LocalNet)
	require.NoError(t, err)

	appProvider := p.Participants["app-provider"]
	require.Equal(t, "canton.example", appProvider.LedgerHost)
	require.Equal(t, 4001, appProvider.LedgerPort)
	require.Equal(t, "http://json.example", appProvider.JSONAPIBaseURL)
	require.Equal(t, "http://validator.example", appProvider.ValidatorBaseURL)

	for _, role := range []string{"app-user", "bob", "guardian-governance", "guardian-observer", "alice-solo"} {
		ep := p.Participants[role]
		require.Equal(t, "localhost", ep.LedgerHost, "role %q must not pick up LOCALNET_LEDGER_HOST", role)
		require.NotEqual(t, "http://json.example", ep.JSONAPIBaseURL, "role %q must not pick up LOCALNET_JSON_API_URL", role)
		require.NotEqual(t, "http://validator.example", ep.ValidatorBaseURL, "role %q must not pick up LOCALNET_VALIDATOR_URL", role)
	}
}
