package disclosure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// exampleConfigJSON mirrors the plan's §4 example shape verbatim (.claude/tasks/
// e2e-separate-participants.md), including the six templates the e2e suite's disclosure
// subtests eventually gate on.
const exampleConfigJSON = `{
  "partyHosting": { "Alice": "app-user", "Bob": "bob",
                    "GuardianGovernance": "guardian-governance",
                    "GuardianObserver": "guardian-observer", "*": "app-provider" },
  "disclose": [
    { "template": "Wormhole.Ntt.Manager:NttManager",      "fetchAs": "Operator" },
    { "template": "Wormhole.Core.State:CoreState",        "fetchAs": "Operator" },
    { "template": "Wormhole.Core.State:Emitter",           "fetchAs": "Operator" },
    { "template": "Playground.MockToken:MockToken",       "fetchAs": "admin" },
    { "template": "Wormhole.Ntt.TokenCip56:Cip56BurnMintToken", "fetchAs": "admin" },
    { "template": "Test.TestNtt:MockBurnMintFactory",     "fetchAs": "admin" }
  ]
}`

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "topology.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

// ----------------------------------------------------------------------
// Load: valid config
// ----------------------------------------------------------------------

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, exampleConfigJSON)
	c, err := Load(path)
	require.NoError(t, err)

	require.Equal(t, map[string]string{
		"Alice":              "app-user",
		"Bob":                "bob",
		"GuardianGovernance": "guardian-governance",
		"GuardianObserver":   "guardian-observer",
		"*":                  "app-provider",
	}, c.PartyHosting)

	require.Len(t, c.Disclose, 6)
	require.Equal(t, Entry{Template: "Wormhole.Ntt.Manager:NttManager", FetchAs: "Operator"}, c.Disclose[0])
	require.Equal(t, Entry{Template: "Test.TestNtt:MockBurnMintFactory", FetchAs: "admin"}, c.Disclose[5])
}

// ----------------------------------------------------------------------
// Load: missing file
// ----------------------------------------------------------------------

// TestLoad_MissingFile pins the documented decision in Load's doc comment: a nonexistent
// config path is NOT an error -- it resolves to the built-in defaults (empty Disclose,
// DefaultPartyHosting), matching the plan's "absent file ⇒ built-in localnet defaults" wording
// so callers can pass the default --topology-config path unconditionally.
func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	c, err := Load(path)
	require.NoError(t, err)
	require.Empty(t, c.Disclose)
	require.Equal(t, DefaultPartyHosting(), c.PartyHosting)
}

// TestLoad_ConfigOmittingPartyHosting proves a present-but-partial config (only "disclose"
// set) still gets the built-in default hosting map rather than an empty one.
func TestLoad_ConfigOmittingPartyHosting(t *testing.T) {
	path := writeConfig(t, `{"disclose": [{"template": "A:B", "fetchAs": "Operator"}]}`)
	c, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, DefaultPartyHosting(), c.PartyHosting)
	require.Len(t, c.Disclose, 1)
}

// ----------------------------------------------------------------------
// Load: malformed JSON / unknown top-level key
// ----------------------------------------------------------------------

func TestLoad_MalformedJSON(t *testing.T) {
	path := writeConfig(t, `{not valid json`)
	_, err := Load(path)
	require.Error(t, err)
}

// TestLoad_UnknownTopLevelKeyRejected pins the decision (documented in Load) to reject an
// unknown top-level key via json.Decoder.DisallowUnknownFields, rather than silently ignoring
// a typo like "partyHostng".
func TestLoad_UnknownTopLevelKeyRejected(t *testing.T) {
	path := writeConfig(t, `{"partyHostng": {"*": "app-provider"}, "disclose": []}`)
	_, err := Load(path)
	require.Error(t, err)
}

// ----------------------------------------------------------------------
// Entry: hit / miss
// ----------------------------------------------------------------------

func TestEntry_Hit(t *testing.T) {
	c, err := Load(writeConfig(t, exampleConfigJSON))
	require.NoError(t, err)
	e, ok := c.Entry("Wormhole.Core.State:CoreState")
	require.True(t, ok)
	require.Equal(t, "Operator", e.FetchAs)
}

func TestEntry_Miss(t *testing.T) {
	c, err := Load(writeConfig(t, exampleConfigJSON))
	require.NoError(t, err)
	_, ok := c.Entry("Wormhole.Ntt.TokenCip56:Cip56CustodyToken")
	require.False(t, ok)
}

func TestEntry_MissOnEmptyConfig(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	require.NoError(t, err)
	_, ok := c.Entry("anything:AtAll")
	require.False(t, ok)
}
