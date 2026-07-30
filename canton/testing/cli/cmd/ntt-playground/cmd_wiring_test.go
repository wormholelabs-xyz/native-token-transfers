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
		"peer", "set", "--deployment", "missing", "--chain", "2", "--manager", "aa", "--transceiver", "bb", "--decimals", "8")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

// TestCmd_PeerSet_MissingDecimalsFlag pins --decimals as required: every OTHER peer-set flag
// present but --decimals absent must still fail as a required-flag error, before any script
// call reaches the runner (the fakeRunner records zero calls).
func TestCmd_PeerSet_MissingDecimalsFlag(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.Deployments["lockunlock"] = state.Deployment{Name: "lockunlock", ManagerID: 1, Admin: "admin::abc", Peers: map[int]state.Peer{}}
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"peer", "set", "--deployment", "lockunlock", "--chain", "2", "--manager", "aa", "--transceiver", "bb")...)
	if err == nil {
		t.Fatalf("expected a required-flag error for missing --decimals")
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected no script call when --decimals is missing, got %v", r.scriptNames())
	}
}

// TestCmd_PeerSet_WiresDecimalsIntoSetPeerInput pins that --decimals flows through
// setPeerOnLedger into Playground.Ops:setPeer's own input, not just persisted state.
func TestCmd_PeerSet_WiresDecimalsIntoSetPeerInput(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.Deployments["burnmint"] = state.Deployment{Name: "burnmint", ManagerID: 1, Admin: "admin::abc", Peers: map[int]state.Peer{}}
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"peer", "set", "--deployment", "burnmint", "--chain", "9",
		"--manager", "aa", "--transceiver", "bb", "--decimals", "3")...)
	if err != nil {
		t.Fatalf("peer set: %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected exactly one script call, got %d: %v", len(r.calls), r.scriptNames())
	}
	input, ok := r.calls[0].Input.(setPeerInput)
	if !ok {
		t.Fatalf("setPeer input type mismatch: %T", r.calls[0].Input)
	}
	if input.Decimals != 3 {
		t.Fatalf("expected Decimals=3 in setPeerInput, got %+v", input)
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
// network status / up / down: localnet profile with no LOCALNET_DIR and no
// discoverable candidate (HOME and --canton-dir both point at empty temp dirs, so
// network.ResolveLocalNetDir has nothing to find).
// ----------------------------------------------------------------------

// noLocalNetDiscoveryFlags isolates a test from the real filesystem's discovery candidates:
// HOME is repointed at an empty temp dir (no ~/.cache/ntt-playground or ~/splice-node), and
// --canton-dir at a separate empty temp dir (no testing/.localnet either).
func noLocalNetDiscoveryFlags(t *testing.T, stateFile string) []string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return append(baseFlags(stateFile), "--profile", "localnet", "--canton-dir", t.TempDir())
}

func TestCmd_NetworkStatus_LocalNetMissingComposeDir(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(noLocalNetDiscoveryFlags(t, stateFile), "network", "status")...)
	if err == nil || !contains(err.Error(), "no Splice LocalNet dir found") {
		t.Fatalf("expected a no-LocalNet-dir-found error, got %v", err)
	}
}

func TestCmd_NetworkUp_LocalNetMissingComposeDir(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(noLocalNetDiscoveryFlags(t, stateFile), "network", "up")...)
	if err == nil || !contains(err.Error(), "no Splice LocalNet dir found") {
		t.Fatalf("expected a no-LocalNet-dir-found error, got %v", err)
	}
}

func TestCmd_NetworkDown_LocalNetMissingComposeDir(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(noLocalNetDiscoveryFlags(t, stateFile), "network", "down")...)
	if err == nil || !contains(err.Error(), "no Splice LocalNet dir found") {
		t.Fatalf("expected a no-LocalNet-dir-found error, got %v", err)
	}
}

// TestCmd_NetworkStatus_DefaultsToStateFileProfileWhenFlagNotSet proves the profile-resolution
// fix end to end: a state file recording "profile":"localnet" (as `init` writes when bootstrapped
// under --profile localnet, e.g. via `just localnet-cli`) makes a bare `network status` (no
// --profile) behave as if --profile localnet had been passed -- it hits the localnet code path
// (no compose dir found) instead of silently defaulting to sandbox.
func TestCmd_NetworkStatus_DefaultsToStateFileProfileWhenFlagNotSet(t *testing.T) {
	t.Setenv("LOCALNET_DIR", "")
	t.Setenv("HOME", t.TempDir())
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Profile = "localnet"
	seedStateFile(t, stateFile, s)

	args := append(baseFlags(stateFile), "--canton-dir", t.TempDir(), "network", "status")
	_, _, err := runPlayground(t, args...)
	if err == nil || !contains(err.Error(), "no Splice LocalNet dir found") {
		t.Fatalf("expected the localnet code path (proving the recorded profile won), got %v", err)
	}
}

// TestCmd_NetworkStatus_ExplicitFlagOverridesStateFile proves an explicit --profile still wins
// over a state file recording a different profile.
func TestCmd_NetworkStatus_ExplicitFlagOverridesStateFile(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Profile = "localnet"
	seedStateFile(t, stateFile, s)

	stdout, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "sandbox"), "network", "status")...)
	if err != nil {
		t.Fatalf("network status: %v", err)
	}
	if !contains(stdout, "sandbox: not running") {
		t.Fatalf("expected the explicit --profile sandbox to win, got %q", stdout)
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
// observe credentials
// ----------------------------------------------------------------------

// TestCmd_ObserveCredentials_RegisteredWithFlags pins the wiring: `observe credentials` is
// registered under the `observe` parent command with --json and --token-ttl flags.
func TestCmd_ObserveCredentials_RegisteredWithFlags(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"observe", "credentials"})
	if err != nil {
		t.Fatalf("expected `observe credentials` to be registered: %v", err)
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Fatalf("expected a --json flag")
	}
	tokenTTL := cmd.Flags().Lookup("token-ttl")
	if tokenTTL == nil {
		t.Fatalf("expected a --token-ttl flag")
	}
	if tokenTTL.DefValue != "2h0m0s" {
		t.Fatalf("expected --token-ttl to default to 2h, got %q", tokenTTL.DefValue)
	}
}

func TestCmd_ObserveCredentials_RequiresLocalNetProfile(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "observe", "credentials")...)
	if err == nil || !contains(err.Error(), "requires the localnet profile") {
		t.Fatalf("expected a requires-localnet-profile error, got %v", err)
	}
}

func TestCmd_ObserveCredentials_MissingState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"),
		"observe", "credentials")...)
	if err == nil || !contains(err.Error(), "run `init` first") {
		t.Fatalf("expected a missing-state error, got %v", err)
	}
}

func TestCmd_ObserveCredentials_NoGuardianObserverInState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(append(baseFlags(stateFile), "--profile", "localnet"),
		"observe", "credentials")...)
	if err == nil || !contains(err.Error(), "no guardianObserver party in state") {
		t.Fatalf("expected a no-guardianObserver error, got %v", err)
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
// receive: unknown deployment / amulet out of scope / missing pubkey
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

// TestCmd_Receive_AmuletOutOfScope pins plan §6's receive.go rejection: "amulet" is rejected
// outright, before any script runs (real receive against Amulet is out of scope for the
// playground, unchanged posture from the pre-rework "cip56-custody" kind).
func TestCmd_Receive_AmuletOutOfScope(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{TokenKind: "amulet"}
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"receive", "--deployment", "ntt1", "--vaa", "aa", "--recipient", "Alice", "--pubkey", "bb")...)
	if err == nil || !contains(err.Error(), "out of scope for real Amulet") {
		t.Fatalf("expected an amulet-out-of-scope error, got %v", err)
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

// ----------------------------------------------------------------------
// Wiring tests (fakeRunner): call SEQUENCE and routing, not real Daml behavior.
//
// Playground.Deploy/Ops's script bodies are real (Phase 2 landed them), but these tests
// still swap in a fakeRunner double (fake_runner_test.go) rather than a real dpm/ledger
// round-trip: they assert on the CLI's own plumbing -- which script names get invoked, in
// what order, with what routing -- fast and without a live sandbox. Real end-to-end Daml
// behavior (the actual RegisterManager/Transfer/Release/Mint semantics) is the e2e suite's
// job (canton/testing/cli/e2e), not this package's.
// ----------------------------------------------------------------------

// TestCmd_Deploy_CallsDeployRegistryAsGgThenDeployNttAsAdmin pins plan §5/§9 Phase 2's
// intended deploy sequence: `deploy` must call Playground.Deploy:deployRegistry (routed with
// the state file's guardianGovernance) BEFORE Playground.Deploy:deployNtt (routed with the
// resolved deployment admin) -- gg stands up the mock registry factory, then admin resolves
// it and registers the manager.
func TestCmd_Deploy_CallsDeployRegistryAsGgThenDeployNttAsAdmin(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.GuardianGovernance = "gg::abc"
	s.Guardian.PrivateKeyHex = "cfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0"
	seedStateFile(t, stateFile, s)

	cfgPath := filepath.Join(t.TempDir(), "deploy.json")
	if err := os.WriteFile(cfgPath, []byte(`{"name":"wiringdeploy","mode":"burn-mint","tokenKind":"mock","decimals":8,`+
		`"peers":[{"chain":9,"manager":"aa","transceiver":"bb","decimals":8}]}`), 0o600); err != nil {
		t.Fatalf("write deploy config: %v", err)
	}

	r := newFakeRunner()
	r.outputs["Playground.Ops:allocatePlaygroundParty"] = map[string]any{"party": "wiringdeploy-admin::abc"}
	r.outputs["Playground.Deploy:deployRegistry"] = map[string]any{"registered": true}
	r.outputs["Playground.Deploy:deployNtt"] = map[string]any{
		"managerId": 0, "managerAddress": "aa", "transceiverAddress": "bb",
		"admin": "wiringdeploy-admin::abc", "instrumentAdmin": "gg::abc", "instrumentId": "wormhole-ntt:xyz",
	}
	r.outputs["Playground.Ops:setPeer"] = map[string]any{"managerId": 0}

	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile), "deploy", "--config", cfgPath)...)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}

	names := r.scriptNames()
	regIdx, nttIdx := -1, -1
	for i, n := range names {
		switch n {
		case "Playground.Deploy:deployRegistry":
			regIdx = i
		case "Playground.Deploy:deployNtt":
			nttIdx = i
		}
	}
	if regIdx == -1 || nttIdx == -1 {
		t.Fatalf("expected both deployRegistry and deployNtt to be called, got %v", names)
	}
	if regIdx >= nttIdx {
		t.Fatalf("expected deployRegistry BEFORE deployNtt, got call sequence %v", names)
	}

	regInput, ok := r.calls[regIdx].Input.(deployRegistryInput)
	if !ok {
		t.Fatalf("deployRegistry input type mismatch: %T", r.calls[regIdx].Input)
	}
	if regInput.GuardianGovernance != "gg::abc" {
		t.Fatalf("deployRegistry should be gg-routed, got %+v", regInput)
	}

	nttInput, ok := r.calls[nttIdx].Input.(deployInput)
	if !ok {
		t.Fatalf("deployNtt input type mismatch: %T", r.calls[nttIdx].Input)
	}
	if nttInput.Admin != "wiringdeploy-admin::abc" {
		t.Fatalf("deployNtt should be admin-routed, got %+v", nttInput)
	}

	// The config's peer (chain 9, decimals 8) must be forwarded to Playground.Ops:setPeer at
	// deploy time -- not just persisted to the state file.
	peerIdx := -1
	for i, n := range names {
		if n == "Playground.Ops:setPeer" {
			peerIdx = i
		}
	}
	if peerIdx == -1 {
		t.Fatalf("expected deploy to call Playground.Ops:setPeer for the config's peer, got %v", names)
	}
	peerInput, ok := r.calls[peerIdx].Input.(setPeerInput)
	if !ok {
		t.Fatalf("setPeer input type mismatch: %T", r.calls[peerIdx].Input)
	}
	if peerInput.Decimals != 8 {
		t.Fatalf("expected the config's peer decimals (8) forwarded to setPeer, got %+v", peerInput)
	}
}

// TestCmd_Transfer_MockKindRunsPreapproveFundThenTransferOut pins plan §6's transfer.go
// redesign: every "mock" transfer runs an ensure-preapproval + fund + transfer sequence
// (Playground.Ops:preapprove, then Playground.Ops:fundUser, then Playground.Ops:transferOut,
// in that order), with fundUser's minted holding cid feeding transferOut's inputHoldingCids.
func TestCmd_Transfer_MockKindRunsPreapproveFundThenTransferOut(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.GuardianGovernance = "gg::abc"
	s.Deployments["ntt1"] = state.Deployment{
		Name: "ntt1", ManagerID: 0, Admin: "ntt1-admin::abc",
		Mode: "burn-mint", TokenKind: "mock", TokenDecimals: 8,
		Peers: map[int]state.Peer{2: {ManagerAddress: strings.Repeat("00", 31) + "bb", TransceiverAddress: strings.Repeat("00", 31) + "cc"}},
	}
	s.Users["Alice"] = "alice::abc" // cached, so resolveParty needs no ledger round-trip
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	r.outputs["Playground.Ops:preapprove"] = map[string]any{"preapproved": true}
	r.outputs["Playground.Ops:fundUser"] = map[string]any{"holdingCid": "holding-cid-1"}
	r.outputs["Playground.Ops:transferOut"] = map[string]any{
		"outboundSequence": 0, "emitterChain": 72, "emitterAddress": strings.Repeat("00", 31) + "dd",
		"nonce": 0, "consistencyLevel": 0, "payload": "9945ff10",
	}

	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"transfer", "--deployment", "ntt1", "--user", "Alice", "--chain", "2",
		"--recipient-address", strings.Repeat("00", 31)+"ee", "--amount", "100")...)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	names := r.scriptNames()
	want := []string{"Playground.Ops:preapprove", "Playground.Ops:fundUser", "Playground.Ops:transferOut"}
	if len(names) != len(want) {
		t.Fatalf("expected exactly %v, got %v", want, names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Fatalf("expected call %d to be %q, got %q (full sequence %v)", i, n, names[i], names)
		}
	}

	transferInput, ok := r.calls[2].Input.(transferOutInput)
	if !ok {
		t.Fatalf("transferOut input type mismatch: %T", r.calls[2].Input)
	}
	if len(transferInput.InputHoldingCids) != 1 || transferInput.InputHoldingCids[0] != "holding-cid-1" {
		t.Fatalf("expected transferOut to use fundUser's minted holding cid, got %+v", transferInput.InputHoldingCids)
	}
}

// TestCmd_Receive_AmuletRejectedBeforeAnyScriptRuns re-pins TestCmd_Receive_AmuletOutOfScope
// at the wiring level: "amulet" must be rejected client-side, before any dpm script runs at
// all (not just before receiveVaa specifically).
func TestCmd_Receive_AmuletRejectedBeforeAnyScriptRuns(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{TokenKind: "amulet"}
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"receive", "--deployment", "ntt1", "--vaa", "aa", "--recipient", "Alice", "--pubkey", "bb")...)
	if err == nil || !contains(err.Error(), "out of scope for real Amulet") {
		t.Fatalf("expected an amulet-out-of-scope error, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected no script calls at all for a rejected amulet receive, got %v", r.scriptNames())
	}
}

// ----------------------------------------------------------------------
// pause / unpause
// ----------------------------------------------------------------------

// TestCmd_Pause_RunsSetPausedAsAdmin pins plan §5's pause wiring: `pause --deployment X`
// sends exactly one Playground.Ops:setPaused call carrying the deployment's ManagerID/Admin
// and Paused: true.
func TestCmd_Pause_RunsSetPausedAsAdmin(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.Deployments["burnmint"] = state.Deployment{Name: "burnmint", ManagerID: 3, Admin: "admin::abc"}
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	r.outputs["Playground.Ops:setPaused"] = map[string]any{"managerId": 3, "paused": true}

	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"pause", "--deployment", "burnmint")...)
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected exactly one script call, got %d: %v", len(r.calls), r.scriptNames())
	}
	if r.calls[0].Script != "Playground.Ops:setPaused" {
		t.Fatalf("expected Playground.Ops:setPaused, got %q", r.calls[0].Script)
	}
	input, ok := r.calls[0].Input.(setPausedInput)
	if !ok {
		t.Fatalf("setPaused input type mismatch: %T", r.calls[0].Input)
	}
	if input.ManagerID != 3 || input.Admin != "admin::abc" || !input.Paused {
		t.Fatalf("expected ManagerID=3 Admin=admin::abc Paused=true, got %+v", input)
	}
}

// TestCmd_Unpause_RunsSetPausedAsAdmin mirrors the above for `unpause`: same call, Paused:
// false.
func TestCmd_Unpause_RunsSetPausedAsAdmin(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.Deployments["burnmint"] = state.Deployment{Name: "burnmint", ManagerID: 3, Admin: "admin::abc"}
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	r.outputs["Playground.Ops:setPaused"] = map[string]any{"managerId": 3, "paused": false}

	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"unpause", "--deployment", "burnmint")...)
	if err != nil {
		t.Fatalf("unpause: %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected exactly one script call, got %d: %v", len(r.calls), r.scriptNames())
	}
	input, ok := r.calls[0].Input.(setPausedInput)
	if !ok {
		t.Fatalf("setPaused input type mismatch: %T", r.calls[0].Input)
	}
	if input.ManagerID != 3 || input.Admin != "admin::abc" || input.Paused {
		t.Fatalf("expected ManagerID=3 Admin=admin::abc Paused=false, got %+v", input)
	}
}

// TestCmd_Pause_UnknownDeployment errors before any script call -- same shape as
// TestCmd_PeerSet_UnknownDeployment.
func TestCmd_Pause_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())

	r := newFakeRunner()
	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"pause", "--deployment", "missing")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected no script call for an unknown deployment, got %v", r.scriptNames())
	}
}

// TestCmd_Unpause_UnknownDeployment mirrors the above for `unpause`.
func TestCmd_Unpause_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())

	r := newFakeRunner()
	_, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"unpause", "--deployment", "missing")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected no script call for an unknown deployment, got %v", r.scriptNames())
	}
}

// TestCmd_PauseUnpause_MissingRequiredFlags pins --deployment as required for both commands.
func TestCmd_PauseUnpause_MissingRequiredFlags(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "pause")...)
	if err == nil {
		t.Fatalf("expected a required-flag error for `pause` with no --deployment")
	}
	_, _, err = runPlayground(t, append(baseFlags(stateFile), "unpause")...)
	if err == nil {
		t.Fatalf("expected a required-flag error for `unpause` with no --deployment")
	}
}

// ----------------------------------------------------------------------
// guardian sign-governance accept-admin: required flags / unknown deployment / no guardian key
// ----------------------------------------------------------------------

func TestCmd_GuardianSignGovernanceAcceptAdmin_MissingRequiredFlags(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "guardian", "sign-governance", "accept-admin")...)
	if err == nil {
		t.Fatalf("expected a required-flag error")
	}
}

func TestCmd_GuardianSignGovernanceAcceptAdmin_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"guardian", "sign-governance", "accept-admin", "--deployment", "missing")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

func TestCmd_GuardianSignGovernanceAcceptAdmin_NoGuardianKey(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{ManagerID: 1, ManagerAddress: strings.Repeat("00", 31) + "aa"}
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"guardian", "sign-governance", "accept-admin", "--deployment", "ntt1")...)
	if err == nil || !contains(err.Error(), "no guardian key in state") {
		t.Fatalf("expected a no-guardian-key error, got %v", err)
	}
}

// TestCmd_GuardianSignGovernanceAcceptAdmin_SignsAgainstDeploymentManagerAddress pins the
// happy path: given a guardian key and a deployment's managerAddress, the command signs
// without error and prints a vaa= line (the payload's exact bytes are pinned by
// internal/guardian's own unit test against the Daml fixtures, not re-checked here).
func TestCmd_GuardianSignGovernanceAcceptAdmin_SignsAgainstDeploymentManagerAddress(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Guardian.PrivateKeyHex = "cfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0"
	s.Deployments["ntt1"] = state.Deployment{ManagerID: 1, ManagerAddress: strings.Repeat("00", 31) + "aa"}
	seedStateFile(t, stateFile, s)
	stdout, _, err := runPlayground(t, append(baseFlags(stateFile),
		"guardian", "sign-governance", "accept-admin", "--deployment", "ntt1", "--factory-epoch", "0")...)
	if err != nil {
		t.Fatalf("guardian sign-governance accept-admin: %v", err)
	}
	if !contains(stdout, "vaa=") || !contains(stdout, "pubkey=") {
		t.Fatalf("expected vaa=/pubkey= output, got %q", stdout)
	}
}

// ----------------------------------------------------------------------
// admin propose-gg / admin accept-gg-vaa
// ----------------------------------------------------------------------

func TestCmd_AdminProposeGg_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"admin", "propose-gg", "--deployment", "missing")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

// TestCmd_AdminProposeGg_CallsProposeAdminTransferToGg pins the script name + routing:
// Playground.Ops:proposeAdminTransferToGg, called with the deployment's managerId.
func TestCmd_AdminProposeGg_CallsProposeAdminTransferToGg(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.GuardianGovernance = "gg::abc"
	s.Deployments["ntt1"] = state.Deployment{Name: "ntt1", ManagerID: 3, Admin: "ntt1-admin::abc"}
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	r.outputs["Playground.Ops:proposeAdminTransferToGg"] = map[string]any{
		"managerAddress": strings.Repeat("00", 31) + "aa", "factoryEpoch": 0,
	}

	stdout, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"admin", "propose-gg", "--deployment", "ntt1")...)
	if err != nil {
		t.Fatalf("admin propose-gg: %v", err)
	}
	if len(r.calls) != 1 || r.calls[0].Script != "Playground.Ops:proposeAdminTransferToGg" {
		t.Fatalf("expected exactly one Playground.Ops:proposeAdminTransferToGg call, got %v", r.scriptNames())
	}
	in, ok := r.calls[0].Input.(proposeAdminTransferToGgInput)
	if !ok {
		t.Fatalf("proposeAdminTransferToGg input type mismatch: %T", r.calls[0].Input)
	}
	if in.Operator != "operator::abc" || in.ManagerID != 3 {
		t.Fatalf("unexpected proposeAdminTransferToGg input: %+v", in)
	}
	if !contains(stdout, "factoryEpoch=0") {
		t.Fatalf("expected factoryEpoch=0 in output, got %q", stdout)
	}
}

func TestCmd_AdminAcceptGgVaa_MissingRequiredFlags(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	_, _, err := runPlayground(t, append(baseFlags(stateFile), "admin", "accept-gg-vaa")...)
	if err == nil {
		t.Fatalf("expected a required-flag error")
	}
}

func TestCmd_AdminAcceptGgVaa_UnknownDeployment(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	seedStateFile(t, stateFile, state.New())
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"admin", "accept-gg-vaa", "--deployment", "missing", "--vaa", "aa", "--pubkey", "bb")...)
	if err == nil || !contains(err.Error(), `unknown deployment "missing"`) {
		t.Fatalf("expected an unknown-deployment error, got %v", err)
	}
}

func TestCmd_AdminAcceptGgVaa_MissingPubkey(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Deployments["ntt1"] = state.Deployment{ManagerID: 1}
	seedStateFile(t, stateFile, s)
	_, _, err := runPlayground(t, append(baseFlags(stateFile),
		"admin", "accept-gg-vaa", "--deployment", "ntt1", "--vaa", "aa", "--pubkey", "")...)
	if err == nil || !contains(err.Error(), "--pubkey is required") {
		t.Fatalf("expected a missing-pubkey error, got %v", err)
	}
}

// TestCmd_AdminAcceptGgVaa_UpdatesDeploymentAdminOnSuccess pins the CLI-side effect the plan
// calls for: on a successful relay, the state file's Deployment.CurrentAdmin is updated to the
// script's reported successor admin (guardianGovernance, in practice) so later commands
// (status, a subsequent `receive`) see the new admin without a fresh ledger round-trip --
// while Deployment.Admin (the REGISTERING admin, hash-bound into the instrument id and
// manager/transceiver addresses forever) stays untouched. See state.Deployment's doc comment.
func TestCmd_AdminAcceptGgVaa_UpdatesDeploymentAdminOnSuccess(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "playground.state.json")
	s := state.New()
	s.Operator = "operator::abc"
	s.GuardianGovernance = "gg::abc"
	s.Deployments["ntt1"] = state.Deployment{Name: "ntt1", ManagerID: 1, Admin: "ntt1-admin::abc", CurrentAdmin: "ntt1-admin::abc"}
	seedStateFile(t, stateFile, s)

	r := newFakeRunner()
	r.outputs["Playground.Ops:acceptAdminTransferByVaa"] = map[string]any{
		"managerAddress": strings.Repeat("00", 31) + "aa", "admin": "gg::abc",
	}

	stdout, _, err := runPlaygroundWithRunner(t, r, append(baseFlags(stateFile),
		"admin", "accept-gg-vaa", "--deployment", "ntt1", "--vaa", "aabbcc", "--pubkey", "dd")...)
	if err != nil {
		t.Fatalf("admin accept-gg-vaa: %v", err)
	}
	if len(r.calls) != 1 || r.calls[0].Script != "Playground.Ops:acceptAdminTransferByVaa" {
		t.Fatalf("expected exactly one Playground.Ops:acceptAdminTransferByVaa call, got %v", r.scriptNames())
	}
	in, ok := r.calls[0].Input.(acceptAdminTransferByVaaInput)
	if !ok {
		t.Fatalf("acceptAdminTransferByVaa input type mismatch: %T", r.calls[0].Input)
	}
	if in.Executor != "operator::abc" {
		t.Fatalf("expected the default executor to be operator, got %+v", in)
	}
	if !contains(stdout, "admin=gg::abc") {
		t.Fatalf("expected admin=gg::abc in output, got %q", stdout)
	}

	reloaded, err := state.Load(stateFile)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if reloaded.Deployments["ntt1"].CurrentAdmin != "gg::abc" {
		t.Fatalf("expected Deployment.CurrentAdmin to be updated to gg::abc, got %+v", reloaded.Deployments["ntt1"])
	}
	if reloaded.Deployments["ntt1"].Admin != "ntt1-admin::abc" {
		t.Fatalf("expected Deployment.Admin (the registering admin) to stay untouched, got %+v", reloaded.Deployments["ntt1"])
	}
}
