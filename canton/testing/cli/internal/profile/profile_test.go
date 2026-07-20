package profile

import (
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
