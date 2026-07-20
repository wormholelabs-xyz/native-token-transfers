package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// runPlayground runs the root command in-process with args, returning the RunE error (unlike
// execPlayground in verbose_test.go, which asserts success via require.NoError). Used for every
// negative-path/flag-validation test below, where the command is expected to fail.
func runPlayground(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

// seedStateFile writes s as JSON to path, for tests that need a pre-existing playground
// state file without exercising `init` (which touches the ledger).
func seedStateFile(t *testing.T, path string, s *state.State) {
	t.Helper()
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal seed state: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write seed state: %v", err)
	}
}

func baseFlags(stateFile string) []string {
	return []string{"--state-file", stateFile, "--run-dir", filepath.Dir(stateFile)}
}

// ----------------------------------------------------------------------
// newStatusCmd
// ----------------------------------------------------------------------

func TestCmd_Status_MissingState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "status")...)
	if err == nil {
		t.Fatalf("expected an error when no state file exists")
	}
	if !contains(err.Error(), "run `init` first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ----------------------------------------------------------------------
// peer set: required flags + unknown deployment
// ----------------------------------------------------------------------

func TestCmd_PeerSet_MissingRequiredFlags(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "peer", "set")...)
	if err == nil {
		t.Fatalf("expected a required-flag error")
	}
}

func TestCmd_PeerSet_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"peer", "set", "--deployment", "missing", "--chain", "2", "--manager", "aa", "--transceiver", "bb")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// guardian sign-transfer: unknown deployment / no peer / no guardian key
// ----------------------------------------------------------------------

func TestCmd_GuardianSignTransfer_MissingRequiredFlags(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "guardian", "sign-transfer")...)
	if err == nil {
		t.Fatalf("expected a required-flag error")
	}
}

func TestCmd_GuardianSignTransfer_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"guardian", "sign-transfer", "--deployment", "missing", "--to-recipient", "Alice", "--amount", "1")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

func TestCmd_GuardianSignTransfer_NoPeerConfigured(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{ManagerID: 1}
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"guardian", "sign-transfer", "--deployment", "ntt1", "--to-recipient", "Alice", "--amount", "1", "--source-chain", "2")...)
	if err == nil || !contains(err.Error(), "no peer configured for chain 2") {
		t.Fatalf("expected a no-peer-configured error, got %v", err)
	}
}

func TestCmd_GuardianSignTransfer_NoGuardianKey(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{
		ManagerID: 1,
		Peers:     map[int]state.Peer{2: {ManagerAddress: "aa", TransceiverAddress: "bb"}},
	}
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"guardian", "sign-transfer", "--deployment", "ntt1", "--to-recipient", "Alice", "--amount", "1", "--source-chain", "2")...)
	if err == nil || !contains(err.Error(), "no guardian key in state") {
		t.Fatalf("expected a no-guardian-key error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// network status / up / down: localnet profile without LOCALNET_DIR
// ----------------------------------------------------------------------

func TestCmd_NetworkStatus_LocalNetMissingComposeDir(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"), "network", "status")...)
	if err == nil || !contains(err.Error(), "LOCALNET_DIR must point at") {
		t.Fatalf("expected a missing-LOCALNET_DIR error, got %v", err)
	}
}

func TestCmd_NetworkUp_LocalNetMissingComposeDir(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"), "network", "up")...)
	if err == nil || !contains(err.Error(), "LOCALNET_DIR must point at") {
		t.Fatalf("expected a missing-LOCALNET_DIR error, got %v", err)
	}
}

func TestCmd_NetworkDown_LocalNetMissingComposeDir(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"), "network", "down")...)
	if err == nil || !contains(err.Error(), "LOCALNET_DIR must point at") {
		t.Fatalf("expected a missing-LOCALNET_DIR error, got %v", err)
	}
}

func TestCmd_NetworkStatus_SandboxNotRunning(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	stdout, _, err := runPlayground(t, append(baseFlags(stateFile), "network", "status")...)
	if err != nil {
		t.Fatalf("network status: %v", err)
	}
	if !contains(stdout, "sandbox: not running") {
		t.Fatalf("expected the not-running message, got %q", stdout)
	}
}

// ----------------------------------------------------------------------
// observe / observe stream
// ----------------------------------------------------------------------

func TestCmd_Observe_MissingState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "observe", "--deployment", "ntt1")...)
	if err == nil || !contains(err.Error(), "run `init` first") {
		t.Fatalf("expected a missing-state error, got %v", err)
	}
}

func TestCmd_ObserveStream_RequiresLocalNetProfile(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "observe", "stream", "--deployment", "ntt1")...)
	if err == nil || !contains(err.Error(), "requires the localnet profile") {
		t.Fatalf("expected a requires-localnet-profile error, got %v", err)
	}
}

func TestCmd_ObserveStream_MissingState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"),
		"observe", "stream", "--deployment", "ntt1")...)
	if err == nil || !contains(err.Error(), "run `init` first") {
		t.Fatalf("expected a missing-state error, got %v", err)
	}
}

func TestCmd_ObserveStream_NoGuardianObserverInState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"),
		"observe", "stream", "--deployment", "ntt1")...)
	if err == nil || !contains(err.Error(), "no guardianObserver party in state") {
		t.Fatalf("expected a no-guardianObserver error, got %v", err)
	}
}

func TestCmd_ObserveStream_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.GuardianObserver = "obs::abc"
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"),
		"observe", "stream", "--deployment", "missing")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// balance: required flags + unknown deployment
// ----------------------------------------------------------------------

func TestCmd_Balance_MissingRequiredFlags(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "balance")...)
	if err == nil {
		t.Fatalf("expected a required-flag error")
	}
}

func TestCmd_Balance_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"balance", "--party", "Alice", "--deployment", "missing")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// preapprove / preapprove revoke: unknown deployment
// ----------------------------------------------------------------------

func TestCmd_Preapprove_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"preapprove", "--deployment", "missing", "--user", "Alice")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

func TestCmd_PreapproveRevoke_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"preapprove", "revoke", "--deployment", "missing", "--user", "Alice")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// receive: unknown deployment / cip56-custody out of scope / missing pubkey
// ----------------------------------------------------------------------

func TestCmd_Receive_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"receive", "--deployment", "missing", "--vaa", "aa", "--recipient", "Alice", "--pubkey", "bb")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

func TestCmd_Receive_Cip56CustodyOutOfScope(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{TokenKind: "cip56-custody"}
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"receive", "--deployment", "ntt1", "--vaa", "aa", "--recipient", "Alice", "--pubkey", "bb")...)
	if err == nil || !contains(err.Error(), "out of scope for real Amulet") {
		t.Fatalf("expected a cip56-custody-out-of-scope error, got %v", err)
	}
}

func TestCmd_Receive_MissingPubkey(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{TokenKind: "mock"}
	s.Users["Alice"] = "alice::abc" // cached, so resolveParty needs no ledger round-trip
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"receive", "--deployment", "ntt1", "--vaa", "aa", "--recipient", "Alice", "--pubkey", "")...)
	if err == nil || !contains(err.Error(), "--pubkey is required") {
		t.Fatalf("expected a missing-pubkey error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// publish: unknown emitter
// ----------------------------------------------------------------------

func TestCmd_Publish_UnknownEmitter(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"publish", "--emitter", "missing", "--payload", "aa")...)
	if err == nil || !contains(err.Error(), `unknown emitter "missing"`) {
		t.Fatalf("expected an unknown-emitter error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// party allocate: required flag
// ----------------------------------------------------------------------

func TestCmd_PartyAllocate_MissingRequiredFlag(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "party", "allocate")...)
	if err == nil {
		t.Fatalf("expected a required-flag error")
	}
}

// ----------------------------------------------------------------------
// contracts list / party list: missing state
// ----------------------------------------------------------------------

func TestCmd_ContractsList_MissingState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "contracts", "list")...)
	if err == nil || !contains(err.Error(), "run `init` first") {
		t.Fatalf("expected a missing-state error, got %v", err)
	}
}

func TestCmd_PartyList_MissingState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "party", "list")...)
	if err == nil || !contains(err.Error(), "run `init` first") {
		t.Fatalf("expected a missing-state error, got %v", err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
