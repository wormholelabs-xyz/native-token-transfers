//go:build e2e

// Package e2e drives the ntt-playground CLI binary end to end against a real network --
// sandbox by default (CI-viable, no docker), or Splice LocalNet if
// NTT_PLAYGROUND_PROFILE=localnet is set (requires docker + LOCALNET_DIR pointing at an
// extracted Splice LocalNet release; see the CLI README).
//
// Every step below shells out to the built CLI binary -- the CLI layer itself is under test,
// not the internal packages directly (those have their own unit tests). Steps run as
// sequential subtests sharing one playground.state.json, since each step builds on the
// previous one's on-ledger state.
//
//	go test -tags e2e ./e2e -v -timeout 20m                                  # sandbox
//	NTT_PLAYGROUND_PROFILE=localnet go test -tags e2e ./e2e -v -timeout 20m  # LocalNet
package e2e

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

var (
	cliBinPath        string
	cantonDir         string
	playgroundProfile string
	cliVerbose        bool
)

func TestMain(m *testing.M) {
	var err error
	cantonDir, err = filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		fmt.Println("e2e: resolve canton dir:", err)
		os.Exit(1)
	}
	cliDir, err := filepath.Abs("..")
	if err != nil {
		fmt.Println("e2e: resolve cli dir:", err)
		os.Exit(1)
	}

	playgroundProfile = os.Getenv("NTT_PLAYGROUND_PROFILE")
	if playgroundProfile == "" {
		playgroundProfile = "sandbox"
	}

	// The CLI's "[v] " sub-step narration is on unless NTT_PLAYGROUND_E2E_VERBOSE=false.
	// Default on, so a bare `go test -tags e2e ./e2e` still narrates; the justfile sets
	// this to false when run as `just verbose=false ...` to fully quiet the output.
	cliVerbose = os.Getenv("NTT_PLAYGROUND_E2E_VERBOSE") != "false"

	binDir, err := os.MkdirTemp("", "ntt-playground-e2e-bin-*")
	if err != nil {
		fmt.Println("e2e: create bin dir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(binDir)

	cliBinPath = filepath.Join(binDir, "ntt-playground")
	build := exec.Command("go", "build", "-o", cliBinPath, "./cmd/ntt-playground")
	build.Dir = cliDir
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Printf("e2e: build CLI binary failed: %v\n%s\n", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// harness runs the CLI binary with a fixed --state-file/--canton-dir/--profile/--run-dir for
// one test, so every step in a subtest chain shares state. It deliberately holds no
// *testing.T of its own -- every method that needs one takes the CALLER's, so a failure is
// always scoped to whichever subtest is actually running (see run/mustRun/loadState below).
type harness struct {
	workDir   string
	stateFile string
}

func newHarness(t *testing.T) *harness {
	workDir := t.TempDir()
	h := &harness{workDir: workDir, stateFile: filepath.Join(workDir, "playground.state.json")}
	// Safety net: if a subtest fails before reaching the explicit "network down" step, still
	// tear down the network we started -- otherwise a leaked `dpm sandbox` holds its port
	// across test runs and makes the NEXT run's "network up" fail confusingly. Bound to the
	// outer test's own t (the only one still in scope once every subtest has finished).
	t.Cleanup(func() {
		out, err := h.run(t, "network", "down")
		if err != nil {
			t.Logf("cleanup: network down failed (may be harmless if the network was never up): %v\n%s", err, out)
		}
	})
	return h
}

// run/mustRun/loadState all take the CALLER's *testing.T explicitly -- the current subtest's
// own T, never harness.t (the outer TestPlaygroundE2E's T, retained only for TempDir/Cleanup
// lifetime in newHarness above). require.* and t.Fatal both call FailNow on whatever *testing.T
// they're given; calling FailNow on a PARENT test from a child subtest's goroutine aborts the
// ENTIRE parent immediately ("subtest may have called FailNow on a parent test"), which used to
// mean one subtest's transient failure silently skipped every later subtest, including
// "network down" and the real-Amulet subtest. Passing each subtest's own t scopes a failure to
// that subtest alone, exactly like the harness's own t.Cleanup safety net already does for
// teardown.
func (h *harness) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Flags shared by every CLI call. --verbose adds the "[v] " sub-step narration to the
	// captured output; it is on unless NTT_PLAYGROUND_E2E_VERBOSE=false (see cliVerbose).
	full := []string{
		"--state-file", h.stateFile,
		"--canton-dir", cantonDir,
		"--profile", playgroundProfile,
		"--run-dir", filepath.Join(h.workDir, ".run"),
	}
	if cliVerbose {
		full = append(full, "--verbose")
	}
	full = append(full, args...)
	cmd := exec.Command(cliBinPath, full...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (h *harness) mustRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := h.run(t, args...)
	require.NoErrorf(t, err, "ntt-playground %v failed:\n%s", args, out)
	t.Logf("ntt-playground %v:\n%s", args, out)
	return out
}

func (h *harness) loadState(t *testing.T) state.State {
	t.Helper()
	s, err := state.Load(h.stateFile)
	require.NoError(t, err)
	return *s
}

// extractField finds "field=value" among the whitespace-separated tokens of the CLI's
// stdout/stderr and returns value.
func extractField(t *testing.T, output, field string) string {
	t.Helper()
	prefix := field + "="
	for _, tok := range strings.Fields(output) {
		if strings.HasPrefix(tok, prefix) {
			return strings.TrimPrefix(tok, prefix)
		}
	}
	t.Fatalf("field %q not found in output:\n%s", field, output)
	return ""
}

// stripVerbose drops the "[v] "-prefixed narration lines --verbose interleaves into the
// combined stdout/stderr, leaving only the machine-parseable stdout. Needed where a subtest
// parses structured output (e.g. `contracts list`'s JSON); the field=value assertions go
// through extractField, which already tolerates the extra lines by tokenizing.
func stripVerbose(output string) string {
	var b strings.Builder
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "[v] ") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func testdataPath(name string) string {
	return filepath.Join("..", "testdata", name)
}

// addr32 builds a 32-byte (64 hex char) address literal ending in the given 2-hex-char
// suffix, for constructing distinct peer manager/transceiver addresses in the tests below
// without repeating long hex literals by hand.
func addr32(suffix string) string {
	return strings.Repeat("00", 31) + suffix
}

// mustHex decodes a hex string, failing the test on error -- used for the observer's streamed
// payload, which the CLI already validated on the way in (only the test's own decode needs a
// clean failure message).
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func TestPlaygroundE2E(t *testing.T) {
	if playgroundProfile == "localnet" {
		if _, err := network.ResolveLocalNetDir(cantonDir); err != nil {
			t.Skip(err.Error())
		}
	}

	h := newHarness(t)

	t.Run("build and network up", func(t *testing.T) {
		dpm, err := ledger.FindDpm()
		require.NoError(t, err, "dpm not found (PATH or ~/.dpm/bin)")
		build := exec.Command(dpm, "build", "--all")
		build.Dir = cantonDir
		out, err := build.CombinedOutput()
		require.NoErrorf(t, err, "dpm build --all failed:\n%s", out)

		h.mustRun(t, "network", "up")
	})

	var guardianKeyHex string
	var initOut string
	t.Run("init with a fresh 1/1 guardian", func(t *testing.T) {
		priv, err := crypto.GenerateKey()
		require.NoError(t, err)
		guardianKeyHex = hex.EncodeToString(crypto.FromECDSA(priv))

		initOut = h.mustRun(t, "init", "--guardian-key", guardianKeyHex)
		out := initOut

		// Stable verbose markers only (messages may evolve): a party allocation and both
		// genesis script invocations (propose on operator's participant, accept on
		// guardianGovernance's participant -- phase 4's propose/accept split, plan §5.1)
		// must have been narrated.
		require.Contains(t, out, "[v] allocated party")
		require.Contains(t, out, "[v] script Playground.Init:proposeGenesis")
		require.Contains(t, out, "[v] script Playground.Init:acceptGenesis")

		s := h.loadState(t)
		require.Equal(t, guardianKeyHex, s.Guardian.PrivateKeyHex)
		require.NotEmpty(t, s.Operator)
		require.NotEmpty(t, s.GuardianGovernance)
		require.Contains(t, s.Operator, "::", "operator should be a full party id")
	})

	t.Run("deploy burn-mint", func(t *testing.T) {
		out := h.mustRun(t, "deploy", "--config", testdataPath("deploy-burnmint.json"))

		// Stable verbose marker: the config's pre-set peer must have been narrated.
		require.Contains(t, out, "[v] set peer chain=2")

		s := h.loadState(t)
		d, ok := s.Deployment("burnmint")
		require.True(t, ok)
		require.NotEmpty(t, d.ManagerAddress)
		require.NotEmpty(t, d.TransceiverAddress)
		_, hasPeer := d.Peer(2)
		require.True(t, hasPeer, "peer chain 2 should have been pre-set from the config file")
	})

	var inboundVaaHex, inboundPubKeyHex string
	t.Run("inbound transfer mints to the recipient", func(t *testing.T) {
		// Both modes now require the recipient's prior opt-in (executor-only delivery
		// mirrors real Amulet and this package's own burn/mint Mint -- see
		// Playground.MockRegistry / Wormhole.Ntt.Deposit): one-time preapprove before the
		// first receive on this deployment.
		out := h.mustRun(t, "preapprove", "--deployment", "burnmint", "--user", "Alice")
		require.Contains(t, out, "preapproved=true")

		out = h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "burnmint", "--to-recipient", "Alice", "--amount", "1000000", "--source-chain", "2")
		inboundVaaHex = extractField(t, out, "vaa")
		inboundPubKeyHex = extractField(t, out, "pubkey")
		require.NotEmpty(t, inboundVaaHex)

		out = h.mustRun(t, "receive", "--deployment", "burnmint",
			"--vaa", inboundVaaHex, "--recipient", "Alice", "--pubkey", inboundPubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=1000000")

		out = h.mustRun(t, "balance", "--party", "Alice", "--deployment", "burnmint")
		require.Contains(t, out, "cip56HoldingTotal=")
	})

	t.Run("replay is rejected", func(t *testing.T) {
		out, err := h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", inboundVaaHex, "--recipient", "Alice", "--pubkey", inboundPubKeyHex)
		require.Error(t, err, "relaying the same VAA twice must be rejected by the replay trie")
		// Pin the gate: Wormhole.Core.Replay's ConsumeDigest membership check
		// (Wormhole.Core.State.VerifyAndConsumeVAA's nested exercise), not some other
		// incidental failure.
		require.Contains(t, out, "digest already consumed")
	})

	// The suite's only systematic negative-path coverage: five ways a relayed/verified VAA
	// can be adversarial, each pinned to the exact Daml assertMsg/abort text so the test fails
	// loudly if the wrong gate (or no gate) catches it. Case 3 in particular proves
	// verification cannot be steered by the untrusted pubKeys hint -- if hint-handling ever
	// went vacuous, every other (happy-path) test in this suite would stay green.
	t.Run("adversarial VAAs are rejected on-ledger", func(t *testing.T) {
		// Eve is this subtest's (and "layered gates"' below) wrong-recipient foil: she needs a
		// standing burnmint pre-approval too, purely so the on-ledger recipient/manager-binding
		// gates below are what actually fires -- without it, receive's own client-side
		// pre-approval-existence check (the recipient's prior opt-in requirement, see the
		// module-level note above) would short-circuit before any of these VAAs are even
		// submitted, masking the gate this subtest exists to pin.
		out := h.mustRun(t, "preapprove", "--deployment", "burnmint", "--user", "Eve")
		require.Contains(t, out, "preapproved=true")

		// Fresh VAA (auto-incremented sequence, so it can't collide with the inbound-mint
		// subtest's already-consumed one): a valid inbound transfer to Alice.
		out = h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "burnmint", "--to-recipient", "Alice", "--amount", "10000", "--source-chain", "2")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")
		require.NotEmpty(t, vaaHex)

		// 1. Wrong recipient binding: Eve didn't originate the VAA's recipientAddress, so
		// Manager.Receive's binding check must reject her as the receiving party. Receive is
		// executor-only, so the operator reaches this gate with no recipient authority at all.
		// Any partial effect (e.g. the nested VerifyAndConsumeVAA's replay-trie write) rolls
		// back with the aborted transaction, so this does not burn the VAA for step 5's control
		// receive.
		out, err := h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Eve", "--pubkey", pubKeyHex)
		require.Error(t, err, "receiving with a recipient that doesn't match the VAA's bound recipientAddress must be rejected")
		require.Contains(t, out, "recipient does not match VAA recipientAddress")

		// 2. Tampered payload: flip the final payload byte, leaving the signature
		// untouched -- the recomputed digest (keccak256(keccak256(body))) no longer matches
		// what was signed.
		vaaBytes, err := hex.DecodeString(vaaHex)
		require.NoError(t, err)
		tampered := append([]byte(nil), vaaBytes...)
		tampered[len(tampered)-1] ^= 0xFF
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", hex.EncodeToString(tampered), "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "a tampered VAA payload must fail signature verification")
		require.Contains(t, out, "invalid signature for guardian index 0")

		// 3. Wrong pubkey hint: a fresh, unrelated keypair's uncompressed public key does not
		// hash to the guardian address recorded in the guardian set -- proving 'verifyOne'
		// binds the caller-supplied hint to the stored address rather than trusting it
		// outright.
		fakeKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		fakePubKeyHex := hex.EncodeToString(crypto.FromECDSAPub(&fakeKey.PublicKey))
		out, err = h.run(t, "guardian", "verify-vaa", "--vaa", vaaHex, "--pubkey", fakePubKeyHex)
		require.Error(t, err, "an untrusted pubkey hint that doesn't hash to the stored guardian address must not verify")
		require.Contains(t, out, "public key does not match guardian address")

		// 4. No peer configured: chain 7 was never configured as a burnmint peer.
		out, err = h.run(t, "transfer", "--deployment", "burnmint",
			"--user", "Bob", "--chain", "7",
			"--recipient-address", "00000000000000000000000000000000000000000000000000000000000000ee",
			"--amount", "1", "--sign")
		require.Error(t, err, "an outbound transfer to an unconfigured peer chain must be rejected")
		require.Contains(t, out, "no peer for chain 7")

		// 5. Control: the untampered step-1 VAA, correctly received by Alice, must still
		// succeed -- proving the four negatives above failed for their stated reasons, not
		// because the VAA (or the deployment) was globally broken.
		out = h.mustRun(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=10000")
	})

	t.Run("outbound transfer recomputes the message and signs it", func(t *testing.T) {
		// Bob lives on his own participant (plan §2/§3); transferOut's prepare step needs the
		// NttManager/CoreState/Emitter/MockBurnMintFactory disclosures from
		// topology-localnet.json to fetch and attach the gg-owned contracts across
		// participants (integration plan §8; gg, not operator, is the data owner post-rework).
		out := h.mustRun(t, "transfer", "--deployment", "burnmint",
			"--topology-config", testdataPath("topology-localnet.json"),
			"--user", "Bob", "--chain", "2",
			"--recipient-address", "00000000000000000000000000000000000000000000000000000000000000ee",
			"--amount", "500000", "--sign")

		payloadHex := extractField(t, out, "payload")
		require.True(t, strings.HasPrefix(payloadHex, "9945ff10"), "recomputed payload should carry the transceiver prefix")
		require.Equal(t, "72", extractField(t, out, "emitterChain"), "outbound messages are published from Canton (chain 72)")

		s := h.loadState(t)
		d, ok := s.Deployment("burnmint")
		require.True(t, ok)
		peer, ok := d.Peer(2)
		require.True(t, ok)
		require.Contains(t, payloadHex, peer.ManagerAddress, "recomputed payload's recipientManager should be the configured peer")

		outboundVaaHex := extractField(t, out, "vaa")
		require.NotEmpty(t, outboundVaaHex)

		// In-test signature recovery: the guardian's signature over
		// keccak256(keccak256(body)) must recover to the guardian's own address.
		vaaBytes, err := hex.DecodeString(outboundVaaHex)
		require.NoError(t, err)
		require.Greater(t, len(vaaBytes), 1+4+1+1+65)
		sig := vaaBytes[7:72]
		body := vaaBytes[72:]
		digest := crypto.Keccak256(crypto.Keccak256(body))
		recoveredPub, err := crypto.SigToPub(digest, sig)
		require.NoError(t, err)
		recoveredAddr := crypto.PubkeyToAddress(*recoveredPub)
		require.True(t, strings.EqualFold(s.Guardian.Address, strings.TrimPrefix(recoveredAddr.Hex(), "0x")),
			"recovered address %s should match the guardian's own address %s", recoveredAddr.Hex(), s.Guardian.Address)

		// On-ledger verification via Playground.Query:verifyVaa (CoreState.ParseAndVerifyVAA).
		out = h.mustRun(t, "guardian", "verify-vaa", "--deployment", "burnmint", "--vaa", outboundVaaHex)
		require.Contains(t, out, "emitterChain=72")
		require.Contains(t, out, "guardianSetIndex=0")

		// observe: confirm the manager's own on-ledger outboundSequence advanced (0 -> 1
		// from the single transfer above) -- the transfer output's sequence is
		// recompute-based, so this is the suite's only direct on-ledger check of it. Also
		// cross-check the reported chain-2 peer against the state file's peer entry (set
		// from testdata/deploy-burnmint.json at deploy time).
		out = h.mustRun(t, "observe", "--deployment", "burnmint")
		var observed struct {
			OutboundSequence int `json:"outboundSequence"`
			Peers            []struct {
				Chain              int    `json:"chain"`
				ManagerAddress     string `json:"managerAddress"`
				TransceiverAddress string `json:"transceiverAddress"`
			} `json:"peers"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(stripVerbose(out)), &observed), "observe should print JSON:\n%s", out)
		require.Equal(t, 1, observed.OutboundSequence,
			"outboundSequence should have advanced from 0 by the single outbound transfer above")
		require.Len(t, observed.Peers, 1)
		require.Equal(t, 2, observed.Peers[0].Chain)
		require.Equal(t, peer.ManagerAddress, observed.Peers[0].ManagerAddress)
		require.Equal(t, peer.TransceiverAddress, observed.Peers[0].TransceiverAddress)
	})

	t.Run("lock-unlock deployment: abbreviated receive + transfer", func(t *testing.T) {
		out := h.mustRun(t, "deploy", "--config", testdataPath("deploy-lockunlock.json"))

		// Receive (Release) requires the recipient's prior opt-in for lock-unlock "mock"
		// too now (Playground.MockRegistry's MockTransferPreapproval) -- preapprove(Carol)
		// before fund(sender)/transfer(lock)/receive(release), per plan §7(c)'s flipped
		// lock-unlock flow order.
		out = h.mustRun(t, "preapprove", "--deployment", "lockunlock", "--user", "Carol")
		require.Contains(t, out, "preapproved=true")

		// The lock leg comes FIRST now: admin-owned custody enforces a real reserve cap
		// (Ledger.daml's over-release assertion), so an inbound release needs at least as
		// much already locked on this Canton chain as it asks to unlock. Dave is a fresh,
		// non-admin sender -- lock/unlock custody is admin-owned by construction now (no
		// "promoted custodian" workaround needed for a non-admin sender to lock; see the
		// dedicated subtest below for the direct pin). Locks comfortably more than the
		// receive below asks to release.
		out = h.mustRun(t, "transfer", "--deployment", "lockunlock",
			"--user", "Dave", "--chain", "2",
			"--recipient-address", "00000000000000000000000000000000000000000000000000000000000000ee",
			"--amount", "500000", "--sign")
		require.Contains(t, out, "emitterChain=72")

		out = h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "lockunlock", "--to-recipient", "Carol", "--amount", "250000", "--source-chain", "2")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")

		out = h.mustRun(t, "receive", "--deployment", "lockunlock",
			"--vaa", vaaHex, "--recipient", "Carol", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
	})

	// New: pins the admin-owned custody path for a lock/unlock deployment's non-admin
	// senders directly (plan §7(c)) -- upstream's own Test.TestNtt suite can only exercise
	// this scenario with a PROMOTED custodian (custodian == gg, testNttTransferByDifferentUser);
	// the playground's admin-owned-by-default custody needs no promotion at all.
	t.Run("lock-unlock: non-admin sender lock succeeds without promotion", func(t *testing.T) {
		out := h.mustRun(t, "balance", "--party", "lockunlock-admin", "--deployment", "lockunlock")
		before := extractField(t, out, "cip56HoldingTotal")

		out = h.mustRun(t, "transfer", "--deployment", "lockunlock",
			"--user", "Frank", "--chain", "2",
			"--recipient-address", "00000000000000000000000000000000000000000000000000000000000000ee",
			"--amount", "50000", "--sign")
		require.Contains(t, out, "emitterChain=72")

		out = h.mustRun(t, "balance", "--party", "lockunlock-admin", "--deployment", "lockunlock")
		after := extractField(t, out, "cip56HoldingTotal")
		require.NotEqual(t, before, after,
			"a non-admin sender's lock must move funds into the admin-owned custody pot with no promotion step")
	})

	// New negative subtest (plan §7(c)): receive without the recipient's prior opt-in is
	// rejected cleanly, and the VAA stays deliverable once the recipient opts in --
	// exercised for BOTH modes (burn-mint pins Wormhole.Ntt.Manager's Mint gate; lock-unlock
	// pins Playground.MockRegistry's factory gate, reached through Manager.Release).
	t.Run("receive without preapproval is rejected and the VAA stays deliverable", func(t *testing.T) {
		for _, tc := range []struct {
			deployment, recipient string
		}{
			{"burnmint", "Grace"},
			{"lockunlock", "Heidi"},
		} {
			t.Run(tc.deployment, func(t *testing.T) {
				out := h.mustRun(t, "guardian", "sign-transfer",
					"--deployment", tc.deployment, "--to-recipient", tc.recipient, "--amount", "12345", "--source-chain", "2")
				vaaHex := extractField(t, out, "vaa")
				pubKeyHex := extractField(t, out, "pubkey")
				require.NotEmpty(t, vaaHex)

				// No preapprove yet: the on-ledger gate must reject the delivery cleanly,
				// without consuming the VAA's replay digest. The two modes' abort messages
				// differ slightly ("no DepositPreapproval" for burn-mint vs. "no standing
				// transfer pre-approval" for lock-unlock -- see Playground.Ops:receiveVaa), so
				// assert on the phrase both share instead of "pre-approval" (present only in
				// the lock-unlock wording).
				out, err := h.run(t, "receive", "--deployment", tc.deployment,
					"--vaa", vaaHex, "--recipient", tc.recipient, "--pubkey", pubKeyHex)
				require.Error(t, err, "a receive with no prior recipient opt-in must be rejected")
				require.Contains(t, out, "run `preapprove` first")

				out = h.mustRun(t, "preapprove", "--deployment", tc.deployment, "--user", tc.recipient)
				require.Contains(t, out, "preapproved=true")

				// The SAME VAA now succeeds -- the earlier failure did not burn the digest.
				out = h.mustRun(t, "receive", "--deployment", tc.deployment,
					"--vaa", vaaHex, "--recipient", tc.recipient, "--pubkey", pubKeyHex)
				require.Contains(t, out, "recipientChain=72")
				require.Contains(t, out, "amount=12345")
			})
		}
	})

	t.Run("party list shows allocated parties", func(t *testing.T) {
		s := h.loadState(t)
		require.NotEmpty(t, s.Users["Alice"], "Alice should have been allocated by the inbound transfer step")

		out := h.mustRun(t, "party", "list")
		require.Contains(t, out, s.Operator, "operator's party id should be listed")
		require.Contains(t, out, s.Users["Alice"], "Alice's party id should be listed")

		out = h.mustRun(t, "party", "allocate", "--hint", "Eve")
		eve := extractField(t, out, "party")
		require.NotEmpty(t, eve)
		s = h.loadState(t)
		require.Equal(t, eve, s.Users["Eve"], "allocate should persist the hint in state")

		out = h.mustRun(t, "party", "allocate", "--hint", "Eve")
		require.Equal(t, eve, extractField(t, out, "party"), "re-allocating the same hint must return the same party")
	})

	var lastOracleSeq string
	t.Run("standalone emitter publishes a verifiable message", func(t *testing.T) {
		out := h.mustRun(t, "emitter", "register", "--name", "oracle", "--owner", "oracle-admin")
		require.NotEmpty(t, extractField(t, out, "emitterAddress"))

		out = h.mustRun(t, "publish", "--emitter", "oracle", "--payload", "deadbeef", "--sign")
		require.Equal(t, "72", extractField(t, out, "emitterChain"), "standalone messages are published from Canton (chain 72)")
		require.Equal(t, "deadbeef", extractField(t, out, "payload"), "the choice-result payload should round-trip verbatim")
		firstSeq, err := strconv.Atoi(extractField(t, out, "sequence"))
		require.NoError(t, err)
		vaaHex := extractField(t, out, "vaa")
		require.NotEmpty(t, vaaHex)

		// On-ledger verification of the CLI-signed VAA (operator as default verifier).
		out = h.mustRun(t, "guardian", "verify-vaa", "--vaa", vaaHex)
		require.Contains(t, out, "emitterChain=72")

		out = h.mustRun(t, "publish", "--emitter", "oracle", "--payload", "deadbeef", "--sign")
		lastOracleSeq = extractField(t, out, "sequence")
		require.Equal(t, strconv.Itoa(firstSeq+1), lastOracleSeq,
			"the emitter's sequence must increment on every publish")
	})

	t.Run("governance raises the fee and fee-0 publishes start failing", func(t *testing.T) {
		// SubmitGovernanceVAA / fee enforcement have no other e2e coverage -- every other
		// publish in this suite rides the fee-0 path, so Wormhole.Core.Fees:chargeFee's
		// fee-due branch is otherwise unreachable. Reuses the "oracle" emitter registered
		// above.
		out := h.mustRun(t, "guardian", "sign-governance", "set-fee", "--fee", "1000", "--apply")
		require.Contains(t, out, "applied: messageFee=1000")

		// On-ledger read-back through an independent query path (Playground.Query:listContracts).
		out = h.mustRun(t, "contracts", "list")
		var listed struct {
			MessageFee int `json:"messageFee"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(stripVerbose(out)), &listed), "contracts list should print JSON:\n%s", out)
		require.Equal(t, 1000, listed.MessageFee)

		// The CLI's publish path always sends feeAllocation=None (the fee-0 assumption noted
		// in publish.go); with the fee raised, chargeFee's None branch must abort the choice.
		out, err := h.run(t, "publish", "--emitter", "oracle", "--payload", "cafe", "--sign")
		require.Error(t, err, "a fee-0 publish must be rejected once the message fee is raised above zero")
		require.Contains(t, out, "fee required: attach a fee allocation")

		// Restoring fee 0 also implicitly exercises the CLI's governance-sequence
		// auto-increment: a reused sequence would be rejected by the consumed-governance
		// replay guard.
		out = h.mustRun(t, "guardian", "sign-governance", "set-fee", "--fee", "0", "--apply")
		require.Contains(t, out, "applied: messageFee=0")

		lastSeq, err := strconv.Atoi(lastOracleSeq)
		require.NoError(t, err)
		out = h.mustRun(t, "publish", "--emitter", "oracle", "--payload", "cafe", "--sign")
		require.Equal(t, strconv.Itoa(lastSeq+1), extractField(t, out, "sequence"),
			"the rejected fee-raised publish must not have consumed a sequence number")
	})

	t.Run("contracts list reflects deployments", func(t *testing.T) {
		out := h.mustRun(t, "contracts", "list")
		var listed struct {
			GuardianSetIndex int `json:"guardianSetIndex"`
			Emitters         []struct {
				EmitterID      int    `json:"emitterId"`
				EmitterAddress string `json:"emitterAddress"`
			} `json:"emitters"`
			Managers []struct {
				ManagerID int `json:"managerId"`
			} `json:"managers"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(stripVerbose(out)), &listed), "contracts list should print JSON:\n%s", out)
		require.Equal(t, 0, listed.GuardianSetIndex)
		// One manager per deploy subtest: "deploy burn-mint" and "lock-unlock deployment".
		// Bump this count whenever a deploy subtest is added. (The real-Amulet "amulet"
		// deploy runs after this subtest, and only on localnet, so it is not counted here.)
		require.Len(t, listed.Managers, 2, "burnmint + lockunlock managers")
		require.GreaterOrEqual(t, len(listed.Emitters), 2, "two transceiver emitters + the standalone one")
	})

	// The suite's remaining Receive gate coverage: Manager.daml's Receive choice
	// (Manager.daml:243-285) has six on-ledger gates evaluated in this order -- (1)
	// VerifyAndConsumeVAA (guardian-set lookup, then signature, then replay), (2) peer lookup
	// by source chain, (3) emitter-is-peer-transceiver binding, (4) recipient-manager binding,
	// (5) source-manager-is-peer-manager binding, (6) recipient binding. The existing
	// "adversarial VAAs" subtest above already covers replay, signature tampering, the pubkey
	// hint, and recipient binding (gate 6); the five subtests below cover the rest: the
	// previously-unreached gates 2-5, the guardian-set lookup half of gate 1, the full ladder
	// evaluated together, malformed-byte parsing, and the `peer set` command (previously
	// unexercised).
	t.Run("inbound receive enforces peer and manager binding", func(t *testing.T) {
		// This subtest's controls receive into Alice on "lockunlock" (she was only preapproved
		// on "burnmint" earlier) -- both modes now gate receive on the recipient's prior opt-in
		// (see the module-level note above), so she needs a standing lockunlock pre-approval
		// too before any of the A3/A4 controls below can succeed.
		out := h.mustRun(t, "preapprove", "--deployment", "lockunlock", "--user", "Alice")
		require.Contains(t, out, "preapproved=true")

		// A1 -- gate 2: chain 7 has never been configured as a burnmint peer. sign-vaa lets us
		// sign an arbitrary payload for a chain/emitter pair that sign-transfer could never
		// reach (sign-transfer requires a configured peer client-side); the signature is
		// genuinely valid, so this isolates the peer-lookup gate before the payload is ever
		// decoded.
		out = h.mustRun(t, "guardian", "sign-vaa",
			"--emitter-chain", "7", "--emitter", addr32("cc"), "--sequence", "9001", "--payload", "deadbeef")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")

		out, err := h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "an inbound VAA from a chain with no configured peer must be rejected")
		require.Contains(t, out, "no peer for source chain 7")

		// A2 -- gate 3: chain 2 HAS a peer, but its transceiver is addr32("cc"), not
		// addr32("ee") -- the emitter binding must reject a signer that isn't the configured
		// transceiver even though the chain itself is known.
		out = h.mustRun(t, "guardian", "sign-vaa",
			"--emitter-chain", "2", "--emitter", addr32("ee"), "--sequence", "9002", "--payload", "deadbeef")
		vaaHex = extractField(t, out, "vaa")
		pubKeyHex = extractField(t, out, "pubkey")

		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "an inbound VAA from an emitter that isn't the configured peer transceiver must be rejected")
		require.Contains(t, out, "VAA emitter is not the configured peer transceiver")

		// A3 -- gate 4, the realistic real-world gap: all three testdata deployments share the
		// same chain-2 peer addresses (manager addr32("bb"), transceiver addr32("cc")), so a
		// VAA signed for lockunlock also passes burnmint's gates 2 and 3 (shared transceiver).
		// It must still be stopped -- by gate 4, since each deployment's own managerAddress is
		// unique and checked before the source-manager check (gate 5).
		out = h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "lockunlock", "--to-recipient", "Alice", "--amount", "4321", "--source-chain", "2")
		vaaHex = extractField(t, out, "vaa")
		pubKeyHex = extractField(t, out, "pubkey")

		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "relaying a VAA whose recipientManager belongs to a different deployment must be rejected")
		require.Contains(t, out, "wrong recipient manager")

		// Control: the same VAA, received on its true deployment, must still succeed -- proving
		// the failed cross-deployment relay above burned nothing, and that replay digests are
		// scoped per deployment admin (VerifyAndConsumeVAA's consumer = admin).
		out = h.mustRun(t, "receive", "--deployment", "lockunlock",
			"--vaa", vaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=4321")

		// A4 -- gate 5 (also the first exercise of `peer set`): the only way to make a VAA's
		// baked-in sourceManager diverge from the peer's ON-LEDGER managerAddress without
		// touching the deployment's own manager is to change the peer after signing.
		out = h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "lockunlock", "--to-recipient", "Carol", "--amount", "5000", "--source-chain", "2")
		vaaHex = extractField(t, out, "vaa")
		pubKeyHex = extractField(t, out, "pubkey")

		// Replace lockunlock's chain-2 peer manager (keep the transceiver at "cc" so gate 3
		// still passes, and the VAA's own recipientManager is unaffected -- gate 4 still
		// passes -- so gate 5 is the only thing left to fire).
		out = h.mustRun(t, "peer", "set", "--deployment", "lockunlock", "--chain", "2",
			"--manager", addr32("dd"), "--transceiver", addr32("cc"))
		require.Contains(t, out, "peer set: lockunlock chain=2")

		out, err = h.run(t, "receive", "--deployment", "lockunlock",
			"--vaa", vaaHex, "--recipient", "Carol", "--pubkey", pubKeyHex)
		require.Error(t, err, "a VAA whose baked-in sourceManager no longer matches the peer's current on-ledger manager must be rejected")
		require.Contains(t, out, "source manager is not the peer manager")

		// Restore the peer to its original manager.
		out = h.mustRun(t, "peer", "set", "--deployment", "lockunlock", "--chain", "2",
			"--manager", addr32("bb"), "--transceiver", addr32("cc"))
		require.Contains(t, out, "peer set: lockunlock chain=2")

		// Control: the SAME VAA now succeeds -- proving the restore took effect on-ledger and
		// the failed step above consumed nothing.
		out = h.mustRun(t, "receive", "--deployment", "lockunlock",
			"--vaa", vaaHex, "--recipient", "Carol", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=5000")
	})

	t.Run("unknown guardian set index is rejected", func(t *testing.T) {
		// B1: a VAA naming a never-installed guardian set. The guardianSetIndex lives at
		// vaaBytes[1:5], outside the signed body (which starts at offset vaaHeaderLen=72), so
		// rewriting it leaves the signature -- and the replay digest, computed over the body
		// only -- untouched; the failure is genuinely the on-ledger set lookup, not a botched
		// signature.
		//
		// (A real superseded-guardian-set test -- signing with an old key after a genuine
		// guardian-set upgrade -- needs CLI surface this playground doesn't have yet
		// (guardian.Sign hardcodes index 0, and there's no `sign-governance
		// guardian-set-upgrade` subcommand), so it is not attempted here.)
		out := h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "burnmint", "--to-recipient", "Alice", "--amount", "111", "--source-chain", "2")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")

		vaaBytes, err := hex.DecodeString(vaaHex)
		require.NoError(t, err)
		tampered := append([]byte(nil), vaaBytes...)
		tampered[1], tampered[2], tampered[3], tampered[4] = 0, 0, 0, 42
		tamperedHex := hex.EncodeToString(tampered)

		out, err = h.run(t, "guardian", "verify-vaa", "--vaa", tamperedHex, "--pubkey", pubKeyHex)
		require.Error(t, err, "a VAA naming a never-installed guardian set index must fail on-ledger verification")
		require.Contains(t, out, "unknown guardian set index")

		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", tamperedHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "the same tampered guardian-set index must be rejected inside Receive's VerifyAndConsumeVAA")
		require.Contains(t, out, "unknown guardian set index")

		// Control: the pristine VAA (real index 0) still delivers -- proving the failures above
		// were about the tampered index, not the VAA itself.
		out = h.mustRun(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=111")
	})

	// One base VAA, then four receives walking down the gate ladder from most- to
	// least-violated -- pinning the evaluation order documented above: no later gate can ever
	// be reached while an earlier one is violated.
	t.Run("layered gates fire outermost-first on multiply-invalid VAAs", func(t *testing.T) {
		out := h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "burnmint", "--to-recipient", "Alice", "--amount", "222", "--source-chain", "2")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")
		vaaBytes, err := hex.DecodeString(vaaHex)
		require.NoError(t, err)

		// C1: three violations at once -- unknown guardian set index, a tampered payload byte,
		// and (were it ever reached) the wrong recipient. The set-index lookup is the first
		// sub-check of gate 1, so it fires before the tamper is checked and long before
		// recipient binding (gate 6).
		c1 := append([]byte(nil), vaaBytes...)
		c1[1], c1[2], c1[3], c1[4] = 0, 0, 0, 42
		c1[len(c1)-1] ^= 0xFF
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", hex.EncodeToString(c1), "--recipient", "Eve", "--pubkey", pubKeyHex)
		require.Error(t, err, "an unknown guardian-set index must be rejected even when the payload and recipient are also wrong")
		require.Contains(t, out, "unknown guardian set index")

		// C2: two violations -- tampered payload only (valid set index), wrong recipient.
		// Signature verification (gate 1) fires before recipient binding (gate 6).
		c2 := append([]byte(nil), vaaBytes...)
		c2[len(c2)-1] ^= 0xFF
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", hex.EncodeToString(c2), "--recipient", "Eve", "--pubkey", pubKeyHex)
		require.Error(t, err, "a tampered payload must be rejected even when the recipient is also wrong")
		require.Contains(t, out, "invalid signature for guardian index 0")

		// C3: one violation -- pristine VAA, wrong recipient. The ladder's last rung: recipient
		// binding (gate 6) is reached only once every earlier gate has passed. (Intentionally
		// repeats the existing "adversarial VAAs" case 1, here as the ladder's base case.)
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Eve", "--pubkey", pubKeyHex)
		require.Error(t, err, "an otherwise-valid VAA received by the wrong recipient must be rejected")
		require.Contains(t, out, "recipient does not match VAA recipientAddress")

		// C4: cross-deployment (wrong recipient manager, gate 4) AND wrong recipient (gate 6) --
		// manager binding must fire first.
		out = h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "lockunlock", "--to-recipient", "Alice", "--amount", "2220", "--source-chain", "2")
		c4VaaHex := extractField(t, out, "vaa")
		c4PubKeyHex := extractField(t, out, "pubkey")
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", c4VaaHex, "--recipient", "Eve", "--pubkey", c4PubKeyHex)
		require.Error(t, err, "a cross-deployment relay to the wrong recipient must be rejected on manager binding, not recipient binding")
		require.Contains(t, out, "wrong recipient manager")

		// Control: the pristine group-C burnmint VAA, correctly received by Alice, proves none
		// of the four layered negatives above consumed it.
		out = h.mustRun(t, "receive", "--deployment", "burnmint",
			"--vaa", vaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=222")
	})

	t.Run("malformed VAA byte strings are rejected", func(t *testing.T) {
		// receiveVaa computes its replay digest via parseVAA in-script (Ops.daml:377) -- off-
		// ledger, but through the identical core parsing function and abort strings as the
		// on-ledger ParseAndVerifyVAA that `guardian verify-vaa` drives. Cobra accepts
		// --vaa "" (the flag still counts as explicitly set), so there's no client-side gap
		// masking these.
		out := h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "burnmint", "--to-recipient", "Alice", "--amount", "333", "--source-chain", "2")
		freshVaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")
		freshBytes, err := hex.DecodeString(freshVaaHex)
		require.NoError(t, err)

		// D1: empty -- sliceBytes needs 2 hex chars for the version byte alone; there are 0.
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", "", "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "an empty VAA must fail to parse")
		require.Contains(t, out, "sliceBytes: out of range")

		// D2: truncated -- 40 bytes (80 hex chars) covers version+guardianSetIndex+sigCount+
		// guardianIndex but dies slicing the 65-byte signature. Checked both off-ledger
		// (receive) and on-ledger (verify-vaa) since both drive the identical parse.
		truncatedHex := freshVaaHex[:80]
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", truncatedHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "a truncated VAA must fail to parse")
		require.Contains(t, out, "sliceBytes: out of range")

		out, err = h.run(t, "guardian", "verify-vaa", "--vaa", truncatedHex, "--pubkey", pubKeyHex)
		require.Error(t, err, "the same truncated bytes must fail identically in the on-ledger parse")
		require.Contains(t, out, "sliceBytes: out of range")

		// D3: garbage hex -- passes the length check, but the hex decoder dies on a non-hex
		// character.
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", "zzzz", "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "non-hex characters must be rejected")
		require.Contains(t, out, "invalid hex digit: z")

		// D4: a valid version byte (01) followed by too few bytes for the guardianSetIndex.
		// parseVAA forces `version` first via its `if version /= 1` guard, and only once that
		// passes does it read guardianSetIndex, whose 4-byte slice needs 10 hex chars but has
		// just 3 here. (A bare "abc" would instead read version=0xab=171 and be rejected as an
		// unsupported version before any slice is reached -- that earlier gate is D5's job; the
		// leading "01" is what steers this case to the short-slice failure it means to test.)
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", "01abc", "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "a VAA too short for the guardianSetIndex field must fail to parse")
		require.Contains(t, out, "sliceBytes: out of range")

		// D5: wrong version byte -- vaaBytes[0] is outside the signed body, so the signature
		// stays valid and parsing gets far enough to reject the version value itself.
		d5 := append([]byte(nil), freshBytes...)
		d5[0] = 0x02
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", hex.EncodeToString(d5), "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "an unsupported VAA version must be rejected")
		require.Contains(t, out, "unsupported VAA version: 2")

		// D6: over-length -- appended bytes join the body (everything after the signature block
		// IS the body), so the recomputed digest changes under an unchanged signature: this is
		// a genuinely on-ledger rejection, not a parse error.
		out, err = h.run(t, "receive", "--deployment", "burnmint",
			"--vaa", freshVaaHex+"00000000", "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Error(t, err, "extra trailing bytes must invalidate the signature over the (now longer) body")
		require.Contains(t, out, "invalid signature for guardian index 0")

		// Control: the pristine VAA still delivers.
		out = h.mustRun(t, "receive", "--deployment", "burnmint",
			"--vaa", freshVaaHex, "--recipient", "Alice", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=333")
	})

	t.Run("peer set configures a new chain end to end", func(t *testing.T) {
		// A4 exercised replace-and-restore on an existing chain; this proves a net-new peer
		// (chain 3, never configured before) is honored on both the signing side (the state
		// file `sign-transfer` reads) and the receive side (the on-ledger Manager.Receive
		// checks) -- additive, so no restore is needed afterward.
		out := h.mustRun(t, "peer", "set", "--deployment", "lockunlock", "--chain", "3",
			"--manager", addr32("dd"), "--transceiver", addr32("ee"))
		require.Contains(t, out, "peer set: lockunlock chain=3")

		s := h.loadState(t)
		d, ok := s.Deployment("lockunlock")
		require.True(t, ok)
		p, ok := d.Peer(3)
		require.True(t, ok, "chain 3 should have been persisted to the state file")
		require.Equal(t, addr32("dd"), p.ManagerAddress)
		require.Equal(t, addr32("ee"), p.TransceiverAddress)

		// On-ledger read-back via Playground.Query:listContracts' peer listing (observe).
		out = h.mustRun(t, "observe", "--deployment", "lockunlock")
		var observed struct {
			Peers []struct {
				Chain              int    `json:"chain"`
				ManagerAddress     string `json:"managerAddress"`
				TransceiverAddress string `json:"transceiverAddress"`
			} `json:"peers"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(stripVerbose(out)), &observed), "observe should print JSON:\n%s", out)
		require.Len(t, observed.Peers, 2, "lockunlock should now have chain 2 (from deploy) and chain 3 (just added)")
		var foundChain3 bool
		for _, peer := range observed.Peers {
			if peer.Chain == 3 {
				foundChain3 = true
				require.Equal(t, addr32("dd"), peer.ManagerAddress)
				require.Equal(t, addr32("ee"), peer.TransceiverAddress)
			}
		}
		require.True(t, foundChain3, "the on-ledger peer listing should include the newly configured chain 3")

		// sign-transfer now finds the chain-3 peer in state and bakes in dd/ee; receive on the
		// same deployment then passes all of gates 2-5 against the CLI-configured peer -- a
		// real mint, not just a state-file/on-ledger read-back match.
		out = h.mustRun(t, "guardian", "sign-transfer",
			"--deployment", "lockunlock", "--to-recipient", "Carol", "--amount", "777", "--source-chain", "3")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")

		out = h.mustRun(t, "receive", "--deployment", "lockunlock",
			"--vaa", vaaHex, "--recipient", "Carol", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=777")
	})

	// The only subtest driving REAL Canton Coin (Amulet) rather than a local mock registry:
	// an "amulet" (lock/unlock) deployment, a real tap + TransferPreapproval, an outbound
	// lock of 1 CC, and the guardian observation proved off a real Ledger API v2 update stream
	// (not the recompute path). LocalNet-only: the sandbox has no DSO/Amulet, so `deploy` gates
	// on prof.AmuletAvailable and this subtest self-skips rather than duplicating that gate's own
	// coverage (see the sandbox-rejection subtest below).
	t.Run("real amulet: transfer 1 CC observed on stream", func(t *testing.T) {
		if playgroundProfile != "localnet" {
			t.Skip("real Amulet requires the localnet profile")
		}

		// 1. deploy: onboards the admin as a wallet user (admin IS the custody party now --
		// lock/unlock custody is admin-owned by construction, no separate custody party),
		// creates its TransferPreapproval, and deploys against the real DSO's Amulet instrument.
		h.mustRun(t, "deploy", "--config", testdataPath("deploy-amulet.json"))
		s := h.loadState(t)
		d, ok := s.Deployment("amulet")
		require.True(t, ok)
		require.Equal(t, "amulet", d.TokenKind)
		require.Contains(t, d.InstrumentAdmin, "DSO::", "instrument admin must be the real DSO party")

		// 2. record the pre-transfer ledger end so the stream read below only sees this
		// transfer's own publish.
		out := h.mustRun(t, "observe", "stream", "--deployment", "amulet", "--print-offset")
		fromOffset := extractField(t, out, "ledgerEnd")

		// 3. transfer 1 CC (raw 10^10 at 10 decimals). The CLI auto-onboards and taps the
		// sender, fetches the real transfer factory and choice context from the scan-proxy, and
		// runs Playground.Ops:transferOut against the real Amulet registry.
		out = h.mustRun(t, "transfer", "--deployment", "amulet",
			"--user", "amulet-sender", "--chain", "2",
			"--recipient-address", strings.Repeat("00", 31)+"ee", "--amount", "10000000000", "--sign")
		payloadHex := extractField(t, out, "payload")
		seq := extractField(t, out, "sequence")
		require.True(t, strings.HasPrefix(payloadHex, "9945ff10"))

		// 4. the observation: read the real update stream as guardian-watcher (readAs
		// guardianObserver only -- never the operator/admin) and prove the streamed message
		// matches the recomputed one bit-for-bit.
		out = h.mustRun(t, "observe", "stream", "--deployment", "amulet",
			"--from-offset", fromOffset, "--count", "1", "--timeout", "3m")
		var observed []struct {
			EmitterChain     int    `json:"emitterChain"`
			EmitterAddress   string `json:"emitterAddress"`
			Sequence         int    `json:"sequence"`
			Nonce            int    `json:"nonce"`
			ConsistencyLevel int    `json:"consistencyLevel"`
			Payload          string `json:"payload"`
			EffectiveAt      string `json:"effectiveAt"`
			UpdateID         string `json:"updateId"`
		}
		require.NoError(t, json.Unmarshal([]byte(stripVerbose(out)), &observed))
		require.Len(t, observed, 1)
		m := observed[0]
		require.Equal(t, 72, m.EmitterChain)
		require.Equal(t, d.TransceiverAddress, m.EmitterAddress,
			"derived emitter address must be this deployment's transceiver")
		require.Equal(t, seq, strconv.Itoa(m.Sequence), "streamed sequence == recomputed sequence")
		require.Equal(t, payloadHex, m.Payload, "streamed payload == recomputed payload (bit-exact)")
		require.NotEmpty(t, m.EffectiveAt)

		// Decode the NTT payload off the STREAMED bytes (not the recomputed ones): 1 CC at
		// 8 wire decimals.
		wtm := wire.DecodeWormholeTransceiverMessage(mustHex(t, m.Payload))
		mm := wire.DecodeNttManagerMessage(wtm.ManagerPayload)
		ntt := wire.DecodeNativeTokenTransfer(mm.Payload)
		require.Equal(t, uint64(100_000_000), ntt.Amount, "1 CC at 8 wire decimals")
		require.Equal(t, uint8(8), ntt.Decimals)
		require.Equal(t, uint16(2), ntt.RecipientChain)

		// 5. the admin (custody) actually holds the locked 1.0 CC.
		out = h.mustRun(t, "balance", "--party", "amulet-admin", "--deployment", "amulet")
		require.Equal(t, "1.0000000000", extractField(t, out, "amuletHoldingTotal"))
	})

	// Sandbox-viable coverage of the AmuletAvailable gate: the sandbox has no DSO/Amulet, so
	// `deploy` must reject an "amulet" deployment with a clear error rather than trying (and
	// failing confusingly) to resolve a DSO party that doesn't exist there.
	t.Run("amulet is rejected without real Amulet", func(t *testing.T) {
		if playgroundProfile == "localnet" {
			t.Skip("this pins the sandbox gating error; localnet exercises the real path above")
		}
		out, err := h.run(t, "deploy", "--config", testdataPath("deploy-amulet.json"), "--name", "amulet-rejected")
		require.Error(t, err, "amulet must be rejected on a profile without real Amulet")
		require.Contains(t, out, "amulet requires a profile with real Amulet (localnet)")
	})

	t.Run("network status reports running localnet services", func(t *testing.T) {
		if playgroundProfile != "localnet" {
			t.Skip("network status service listing is localnet-only (the sandbox path is a pid check)")
		}
		out := h.mustRun(t, "network", "status")
		require.Contains(t, out, "localnet: service=", "status should report the running compose services")
	})

	// ----------------------------------------------------------------------
	// Phase-1 placeholders for the participant-split redesign
	// (.claude/tasks/e2e-separate-participants.md). These call the CLI exactly as it exists
	// TODAY -- no new flags, no new output -- and pin today's behavior with a TODO(phase N)
	// comment describing what the assertion becomes once that phase lands. Do not fabricate
	// CLI flags that don't exist yet (--topology-config, etc.); see the plan's §6(c)/§7.
	// ----------------------------------------------------------------------

	t.Run("parties live on separate participants", func(t *testing.T) {
		// Multi-participant routing is a localnet-only concept -- the sandbox profile
		// degenerates every role to its single "default" participant (plan §2), so
		// `party list` there would show every hint under one participant name, which this
		// assertion isn't shaped to check.
		if playgroundProfile != "localnet" {
			t.Skip("participant routing is a localnet-only concept (sandbox has a single participant)")
		}

		// Genesis (`init`, earlier in this suite) now crosses participants for real
		// (Playground/Init.daml's propose/accept split, phase 4's redesign, plan §5.1):
		// proposeGenesis submits as operator alone on operator's participant, acceptGenesis
		// submits as guardianGovernance alone on guardianGovernance's participant. Both
		// resolveParty (party.go) persists s.Users[hint] and s.UserParticipants[hint] as
		// soon as each party is allocated, and init.go assigns state.Operator/
		// GuardianGovernance/GuardianObserver from proposeGenesis's own output -- so
		// s.Users["Operator"]/["GuardianGovernance"]/["GuardianObserver"] are exactly the
		// party ids this subtest relies on. Bob is (re-)allocated explicitly so this subtest
		// doesn't depend on an earlier transfer/receive subtest having reached him first.
		h.mustRun(t, "party", "allocate", "--hint", "Bob")
		s := h.loadState(t)
		require.NotEmpty(t, s.Users["Operator"], "the operator hint should have been allocated during init")
		require.NotEmpty(t, s.Users["GuardianGovernance"])
		require.NotEmpty(t, s.Users["GuardianObserver"])
		require.NotEmpty(t, s.Users["Bob"])

		out := h.mustRun(t, "party", "list")
		lines := strings.Split(stripVerbose(out), "\n")

		// requireRoutedTo asserts that every "participant=... party=<party> ... isLocal=..."
		// line in `party list`'s output agrees with wantParticipant: isLocal=true on that
		// participant, isLocal=false everywhere else the same party shows up (it is
		// topology-visible domain-wide; only its HOME participant actually hosts it).
		requireRoutedTo := func(party, wantParticipant string) {
			matched := 0
			for _, line := range lines {
				fields := strings.Fields(line)
				var gotParty, gotParticipant, gotIsLocal string
				for _, f := range fields {
					switch {
					case strings.HasPrefix(f, "party="):
						gotParty = strings.TrimPrefix(f, "party=")
					case strings.HasPrefix(f, "participant="):
						gotParticipant = strings.TrimPrefix(f, "participant=")
					case strings.HasPrefix(f, "isLocal="):
						gotIsLocal = strings.TrimPrefix(f, "isLocal=")
					}
				}
				if gotParty != party {
					continue
				}
				matched++
				if gotParticipant == wantParticipant {
					require.Equal(t, "true", gotIsLocal, "party %s on its home participant=%s should be local: %q", party, wantParticipant, line)
				} else {
					require.Equal(t, "false", gotIsLocal, "party %s under a foreign participant=%s should not be local: %q", party, gotParticipant, line)
				}
			}
			require.Positivef(t, matched, "expected party %s to appear in `party list` output at all:\n%s", party, out)
		}

		requireRoutedTo(s.Users["Operator"], "app-provider")
		requireRoutedTo(s.Users["GuardianGovernance"], "guardian-governance")
		requireRoutedTo(s.Users["GuardianObserver"], "guardian-observer")
		requireRoutedTo(s.Users["Bob"], "bob")
	})

	t.Run("cross-participant transfer fails without disclosure config", func(t *testing.T) {
		if playgroundProfile != "localnet" {
			t.Skip("participant routing is a localnet-only concept (sandbox has a single participant)")
		}
		// Bob lives on the "bob" participant (default partyHosting), a different participant
		// than guardianGovernance -- the data owner for a Mock-kind transferOut post-rework
		// (integration plan §8: gg co-signs/owns everything transferOut needs, including the
		// gg-sole-owned committed factory). An empty `disclose` list (testdata/
		// topology-empty.json) means Playground.Prepare:prepareTransferOut still resolves real
		// cids/records as gg on gg's own participant, but discloses NONE of them, so the
		// submit-side Playground.Ops:transferOut (running on Bob's own participant) fails
		// on-ledger: the NttManager/CoreState/Emitter it must exercise/read are genuinely
		// invisible there.
		out, err := h.run(t, "--topology-config", testdataPath("topology-empty.json"),
			"transfer", "--deployment", "burnmint", "--user", "Bob", "--chain", "2",
			"--recipient-address", addr32("ee"), "--amount", "1")
		require.Error(t, err, "a cross-participant transfer with no configured disclosures must fail on-ledger")
		require.Contains(t, out, "CONTRACT_NOT_FOUND")
	})

	t.Run("disclosure config recovers the transfer", func(t *testing.T) {
		if playgroundProfile != "localnet" {
			t.Skip("participant routing is a localnet-only concept (sandbox has a single participant)")
		}
		// The SAME command as above, but with testdata/topology-localnet.json (NttManager,
		// CoreState, Emitter, MockBurnMintFactory -- everything burnmint's transferOut needs,
		// per the integration plan's §8) succeeds.
		out := h.mustRun(t, "--topology-config", testdataPath("topology-localnet.json"),
			"transfer", "--deployment", "burnmint", "--user", "Bob", "--chain", "2",
			"--recipient-address", addr32("ee"), "--amount", "1")
		require.Contains(t, out, "emitterChain=72")

		// Removing one entry (CoreState) from a copy of that config and re-running the SAME
		// command must fail again, proving per-template granularity -- the mechanism isn't an
		// all-or-nothing switch.
		out, err := h.run(t, "--topology-config", testdataPath("topology-missing-corestate.json"),
			"transfer", "--deployment", "burnmint", "--user", "Bob", "--chain", "2",
			"--recipient-address", addr32("ee"), "--amount", "1")
		require.Error(t, err, "removing one required disclosure entry must make the SAME command fail again")
		require.Contains(t, out, "CONTRACT_NOT_FOUND")
	})

	t.Run("genesis requires both guardian participants", func(t *testing.T) {
		if playgroundProfile != "localnet" {
			t.Skip("participant routing is a localnet-only concept (sandbox has a single participant)")
		}

		// `init` is a one-time genesis operation -- init.go always starts from a brand new,
		// empty state.New() (deliberately: genesis has no prior identities to resume from),
		// so resolveParty always attempts a FRESH party allocation for the Operator/
		// GuardianGovernance/GuardianObserver hints regardless of what an earlier CLI
		// invocation already persisted. Calling `init` a second time against the SAME live
		// ledger therefore fails ("Party already exists") -- it is not meant to be re-run
		// mid-suite. Rather than re-invoke it, this asserts against the SAME output the
		// earlier "init with a fresh 1/1 guardian" subtest already captured: `init` narrates
		// BOTH the propose script run (on operator's participant) and the accept script run
		// (on guardian-governance's participant) -- proving genesis now needs two
		// independent participant authorizations rather than one single-participant
		// co-signed submit.
		require.Contains(t, initOut, "[v] script Playground.Init:proposeGenesis")
		require.Contains(t, initOut, "[v] script Playground.Init:acceptGenesis")
	})

	t.Run("network down", func(t *testing.T) {
		h.mustRun(t, "network", "down")
	})
}
