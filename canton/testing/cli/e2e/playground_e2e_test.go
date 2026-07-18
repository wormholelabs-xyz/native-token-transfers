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
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

var (
	cliBinPath        string
	cantonDir         string
	playgroundProfile string
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
// one test, so every step in a subtest chain shares state.
type harness struct {
	t         *testing.T
	workDir   string
	stateFile string
}

func newHarness(t *testing.T) *harness {
	workDir := t.TempDir()
	h := &harness{t: t, workDir: workDir, stateFile: filepath.Join(workDir, "playground.state.json")}
	// Safety net: if a subtest fails before reaching the explicit "network down" step, still
	// tear down the network we started -- otherwise a leaked `dpm sandbox` holds its port
	// across test runs and makes the NEXT run's "network up" fail confusingly (a real
	// failure mode hit once during development).
	t.Cleanup(func() {
		out, err := h.run("network", "down")
		if err != nil {
			t.Logf("cleanup: network down failed (may be harmless if the network was never up): %v\n%s", err, out)
		}
	})
	return h
}

func (h *harness) run(args ...string) (string, error) {
	h.t.Helper()
	// --verbose is always on so every step's sub-step narration (stderr, "[v] " prefix)
	// lands in the combined output mustRun logs -- `go test -v` then shows the full story.
	full := append([]string{
		"--state-file", h.stateFile,
		"--canton-dir", cantonDir,
		"--profile", playgroundProfile,
		"--run-dir", filepath.Join(h.workDir, ".run"),
		"--verbose",
	}, args...)
	cmd := exec.Command(cliBinPath, full...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (h *harness) mustRun(args ...string) string {
	h.t.Helper()
	out, err := h.run(args...)
	require.NoErrorf(h.t, err, "ntt-playground %v failed:\n%s", args, out)
	h.t.Logf("ntt-playground %v:\n%s", args, out)
	return out
}

func (h *harness) loadState() state.State {
	h.t.Helper()
	s, err := state.Load(h.stateFile)
	require.NoError(h.t, err)
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

func TestPlaygroundE2E(t *testing.T) {
	if playgroundProfile == "localnet" && os.Getenv("LOCALNET_DIR") == "" {
		t.Skip("NTT_PLAYGROUND_PROFILE=localnet requires LOCALNET_DIR (see the CLI README's LocalNet section)")
	}

	h := newHarness(t)

	t.Run("build and network up", func(t *testing.T) {
		dpm, err := ledger.FindDpm()
		require.NoError(t, err, "dpm not found (PATH or ~/.dpm/bin)")
		build := exec.Command(dpm, "build", "--all")
		build.Dir = cantonDir
		out, err := build.CombinedOutput()
		require.NoErrorf(t, err, "dpm build --all failed:\n%s", out)

		h.mustRun("network", "up")
	})

	var guardianKeyHex string
	t.Run("init with a fresh 1/1 guardian", func(t *testing.T) {
		priv, err := crypto.GenerateKey()
		require.NoError(t, err)
		guardianKeyHex = hex.EncodeToString(crypto.FromECDSA(priv))

		out := h.mustRun("init", "--guardian-key", guardianKeyHex)

		// Stable verbose markers only (messages may evolve): a party allocation and the
		// genesis script invocation must both have been narrated.
		require.Contains(t, out, "[v] allocated party")
		require.Contains(t, out, "[v] script Playground.Init:initPlayground")

		s := h.loadState()
		require.Equal(t, guardianKeyHex, s.Guardian.PrivateKeyHex)
		require.NotEmpty(t, s.Operator)
		require.NotEmpty(t, s.GuardianGovernance)
		require.Contains(t, s.Operator, "::", "operator should be a full party id")
	})

	t.Run("deploy burn-mint", func(t *testing.T) {
		out := h.mustRun("deploy", "--config", testdataPath("deploy-burnmint.json"))

		// Stable verbose marker: the config's pre-set peer must have been narrated.
		require.Contains(t, out, "[v] set peer chain=2")

		s := h.loadState()
		d, ok := s.Deployment("burnmint")
		require.True(t, ok)
		require.NotEmpty(t, d.ManagerAddress)
		require.NotEmpty(t, d.TransceiverAddress)
		_, hasPeer := d.Peer(2)
		require.True(t, hasPeer, "peer chain 2 should have been pre-set from the config file")
	})

	var inboundVaaHex, inboundPubKeyHex string
	t.Run("inbound transfer mints to the recipient", func(t *testing.T) {
		out := h.mustRun("guardian", "sign-transfer",
			"--deployment", "burnmint", "--to-recipient", "Alice", "--amount", "1000000", "--source-chain", "2")
		inboundVaaHex = extractField(t, out, "vaa")
		inboundPubKeyHex = extractField(t, out, "pubkey")
		require.NotEmpty(t, inboundVaaHex)

		out = h.mustRun("receive", "--deployment", "burnmint",
			"--vaa", inboundVaaHex, "--recipient", "Alice", "--pubkey", inboundPubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=1000000")

		out = h.mustRun("balance", "--party", "Alice", "--deployment", "burnmint")
		require.Contains(t, out, "mockHoldingTotal=1000000")
	})

	t.Run("replay is rejected", func(t *testing.T) {
		_, err := h.run("receive", "--deployment", "burnmint",
			"--vaa", inboundVaaHex, "--recipient", "Alice", "--pubkey", inboundPubKeyHex)
		require.Error(t, err, "relaying the same VAA twice must be rejected by the replay trie")
	})

	t.Run("outbound transfer recomputes the message and signs it", func(t *testing.T) {
		out := h.mustRun("transfer", "--deployment", "burnmint",
			"--user", "Bob", "--chain", "2",
			"--recipient-address", "00000000000000000000000000000000000000000000000000000000000000ee",
			"--amount", "500000", "--sign")

		payloadHex := extractField(t, out, "payload")
		require.True(t, strings.HasPrefix(payloadHex, "9945ff10"), "recomputed payload should carry the transceiver prefix")
		require.Equal(t, "72", extractField(t, out, "emitterChain"), "outbound messages are published from Canton (chain 72)")

		s := h.loadState()
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
		out = h.mustRun("guardian", "verify-vaa", "--deployment", "burnmint", "--vaa", outboundVaaHex)
		require.Contains(t, out, "emitterChain=72")
		require.Contains(t, out, "guardianSetIndex=0")
	})

	t.Run("lock-unlock deployment: abbreviated receive + transfer", func(t *testing.T) {
		out := h.mustRun("deploy", "--config", testdataPath("deploy-lockunlock.json"))

		out = h.mustRun("guardian", "sign-transfer",
			"--deployment", "lockunlock", "--to-recipient", "Carol", "--amount", "250000", "--source-chain", "2")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")

		out = h.mustRun("receive", "--deployment", "lockunlock",
			"--vaa", vaaHex, "--recipient", "Carol", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")

		out = h.mustRun("transfer", "--deployment", "lockunlock",
			"--user", "Dave", "--chain", "2",
			"--recipient-address", "00000000000000000000000000000000000000000000000000000000000000ee",
			"--amount", "100000", "--sign")
		require.Contains(t, out, "emitterChain=72")
	})

	// The only subtest driving a real production NttToken impl end to end: the CIP-56
	// burn/mint seam (Cip56BurnMintToken.MintOrUnlock on inbound, LockOrBurn on outbound)
	// over the local mock registry, exercising the auto-funding path, the Daml Decimal
	// JSON boundary, and -- via step f -- the suite's only value-conservation assertion.
	var cip56Total string
	t.Run("cip56 burn-mint deployment: funded transfer round-trip", func(t *testing.T) {
		out := h.mustRun("deploy", "--config", testdataPath("deploy-cip56-burnmint.json"))
		require.Contains(t, out, "[v] set peer chain=2")

		s := h.loadState()
		d, ok := s.Deployment("cip56bm")
		require.True(t, ok)
		require.Equal(t, "cip56-burn-mint-mock", d.TokenKind, "the deployment must use the real CIP-56 burn/mint impl")

		// Inbound: a guardian-signed transfer mints a real owner-signed CIP-56 holding to
		// Frank through Cip56BurnMintToken.MintOrUnlock (recipient authority threaded from
		// Manager.Receive; factory disclosed by receiveVaa).
		out = h.mustRun("guardian", "sign-transfer",
			"--deployment", "cip56bm", "--to-recipient", "Frank", "--amount", "750000", "--source-chain", "2")
		vaaHex := extractField(t, out, "vaa")
		pubKeyHex := extractField(t, out, "pubkey")
		require.NotEmpty(t, vaaHex)

		out = h.mustRun("receive", "--deployment", "cip56bm",
			"--vaa", vaaHex, "--recipient", "Frank", "--pubkey", pubKeyHex)
		require.Contains(t, out, "recipientChain=72")
		require.Contains(t, out, "amount=750000")

		// The mint landed: 750000 wire units at 8 decimals = 0.0075 CIP-56 units, rendered by
		// dpm script as a full-scale Daml Numeric 10 literal (pinned to the observed form).
		out = h.mustRun("balance", "--party", "Frank", "--deployment", "cip56bm")
		cip56Total = extractField(t, out, "cip56HoldingTotal")
		require.Equal(t, "0.0075000000", cip56Total, "the receive should have minted 750000 units (0.0075 at 8 decimals)")

		// Outbound: the CLI auto-funds Frank a fresh 0.003 holding, then LockOrBurn burns it
		// in full (exact cover, no change). Frank's receive-minted 0.0075 is a separate
		// holding and is untouched.
		out = h.mustRun("transfer", "--deployment", "cip56bm",
			"--user", "Frank", "--chain", "2",
			"--recipient-address", "00000000000000000000000000000000000000000000000000000000000000ee",
			"--amount", "300000", "--sign")
		require.Equal(t, "72", extractField(t, out, "emitterChain"), "outbound messages are published from Canton (chain 72)")
		require.True(t, strings.HasPrefix(extractField(t, out, "payload"), "9945ff10"),
			"recomputed payload should carry the transceiver prefix")

		// Value conservation: the auto-funded 0.003 was burned in full, so Frank's balance is
		// exactly what the inbound mint left -- unchanged from the pre-transfer reading.
		out = h.mustRun("balance", "--party", "Frank", "--deployment", "cip56bm")
		require.Equal(t, cip56Total, extractField(t, out, "cip56HoldingTotal"),
			"the funded holding must have been consumed in full by the burn, leaving the minted balance intact")
	})

	t.Run("party list shows allocated parties", func(t *testing.T) {
		s := h.loadState()
		require.NotEmpty(t, s.Users["Alice"], "Alice should have been allocated by the inbound transfer step")

		out := h.mustRun("party", "list")
		require.Contains(t, out, s.Operator, "operator's party id should be listed")
		require.Contains(t, out, s.Users["Alice"], "Alice's party id should be listed")

		out = h.mustRun("party", "allocate", "--hint", "Eve")
		eve := extractField(t, out, "party")
		require.NotEmpty(t, eve)
		s = h.loadState()
		require.Equal(t, eve, s.Users["Eve"], "allocate should persist the hint in state")

		out = h.mustRun("party", "allocate", "--hint", "Eve")
		require.Equal(t, eve, extractField(t, out, "party"), "re-allocating the same hint must return the same party")
	})

	var lastOracleSeq string
	t.Run("standalone emitter publishes a verifiable message", func(t *testing.T) {
		out := h.mustRun("emitter", "register", "--name", "oracle", "--owner", "oracle-admin")
		require.NotEmpty(t, extractField(t, out, "emitterAddress"))

		out = h.mustRun("publish", "--emitter", "oracle", "--payload", "deadbeef", "--sign")
		require.Equal(t, "72", extractField(t, out, "emitterChain"), "standalone messages are published from Canton (chain 72)")
		require.Equal(t, "deadbeef", extractField(t, out, "payload"), "the choice-result payload should round-trip verbatim")
		firstSeq, err := strconv.Atoi(extractField(t, out, "sequence"))
		require.NoError(t, err)
		vaaHex := extractField(t, out, "vaa")
		require.NotEmpty(t, vaaHex)

		// On-ledger verification of the CLI-signed VAA (operator as default verifier).
		out = h.mustRun("guardian", "verify-vaa", "--vaa", vaaHex)
		require.Contains(t, out, "emitterChain=72")

		out = h.mustRun("publish", "--emitter", "oracle", "--payload", "deadbeef", "--sign")
		lastOracleSeq = extractField(t, out, "sequence")
		require.Equal(t, strconv.Itoa(firstSeq+1), lastOracleSeq,
			"the emitter's sequence must increment on every publish")
	})

	t.Run("governance raises the fee and fee-0 publishes start failing", func(t *testing.T) {
		// SubmitGovernanceVAA / fee enforcement have no other e2e coverage -- every other
		// publish in this suite rides the fee-0 path, so Wormhole.Core.Fees:chargeFee's
		// fee-due branch is otherwise unreachable. Reuses the "oracle" emitter registered
		// above.
		out := h.mustRun("guardian", "sign-governance", "set-fee", "--fee", "1000", "--apply")
		require.Contains(t, out, "applied: messageFee=1000")

		// On-ledger read-back through an independent query path (Playground.Query:listContracts).
		out = h.mustRun("contracts", "list")
		var listed struct {
			MessageFee int `json:"messageFee"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(stripVerbose(out)), &listed), "contracts list should print JSON:\n%s", out)
		require.Equal(t, 1000, listed.MessageFee)

		// The CLI's publish path always sends feeAllocation=None (the fee-0 assumption noted
		// in publish.go); with the fee raised, chargeFee's None branch must abort the choice.
		out, err := h.run("publish", "--emitter", "oracle", "--payload", "cafe", "--sign")
		require.Error(t, err, "a fee-0 publish must be rejected once the message fee is raised above zero")
		require.Contains(t, out, "fee required: attach a fee allocation")

		// Restoring fee 0 also implicitly exercises the CLI's governance-sequence
		// auto-increment: a reused sequence would be rejected by the consumed-governance
		// replay guard.
		out = h.mustRun("guardian", "sign-governance", "set-fee", "--fee", "0", "--apply")
		require.Contains(t, out, "applied: messageFee=0")

		lastSeq, err := strconv.Atoi(lastOracleSeq)
		require.NoError(t, err)
		out = h.mustRun("publish", "--emitter", "oracle", "--payload", "cafe", "--sign")
		require.Equal(t, strconv.Itoa(lastSeq+1), extractField(t, out, "sequence"),
			"the rejected fee-raised publish must not have consumed a sequence number")
	})

	t.Run("contracts list reflects deployments", func(t *testing.T) {
		out := h.mustRun("contracts", "list")
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
		// One manager per deploy subtest: "deploy burn-mint", "lock-unlock deployment", and
		// "cip56 burn-mint deployment". Bump these counts whenever a deploy subtest is added.
		require.Len(t, listed.Managers, 3, "burnmint + lockunlock + cip56bm managers")
		require.GreaterOrEqual(t, len(listed.Emitters), 4, "three transceiver emitters + the standalone one")
	})

	t.Run("network status reports running localnet services", func(t *testing.T) {
		if playgroundProfile != "localnet" {
			t.Skip("network status service listing is localnet-only (the sandbox path is a pid check)")
		}
		out := h.mustRun("network", "status")
		require.Contains(t, out, "localnet: service=", "status should report the running compose services")
	})

	t.Run("network down", func(t *testing.T) {
		h.mustRun("network", "down")
	})
}
