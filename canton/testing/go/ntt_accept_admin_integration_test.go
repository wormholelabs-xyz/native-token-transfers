//go:build integration

// Integration test for the governance-VAA-gated admin acceptance
// (Wormhole.Ntt.Manager:AcceptAdminTransferByVaa) against a REGISTERED deployment,
// whose managerAddress is derived from a freshly allocated admin Party
// (nttManagerAddressFor) and so cannot be predicted by a static Daml-script fixture
// (Test.TestNtt's acceptAdminHappyVAA etc. target setupFixedManager's fixed 0x00..aa
// address instead). The live two-step harness:
//
//  1. integrationAcceptAdminSetup registers a BurnMint deployment, has its admin stand
//     an AdminTransferProposal offering the role to gg, and returns the on-ledger
//     managerAddress + factoryEpoch.
//  2. Sign a fresh AcceptAdminToGovernance governance VAA here, off-chain, against that
//     ACTUAL managerAddress -- exactly what the guardians do in production.
//  3. integrationAcceptAdminByVaa relays it against the same sandbox; its assertions
//     (successor admin == gg; a second relay fails) fail the script.
//
//     go test -tags integration -run TestCantonNttAcceptAdminIntegration . -v
//
// Requires `dpm` (PATH or ~/.dpm/bin) + a JDK; skipped otherwise. Slow, so excluded
// from the default build. findDpm/runCmd are in helpers_test.go.
//
// This test hand-rolls its own AcceptAdminToGovernance VAA signer (signAcceptAdminVAA
// below), the same way ntt_recipient_match_integration_test.go hand-rolls
// signNttTransferVAA, rather than importing
// canton/testing/cli/internal/guardian.SignAcceptAdmin: this module
// (canton/testing/go) and the CLI (canton/testing/cli) are separate, independently
// versioned Go modules, and this repo's dependency-safety policy forbids adding a new
// module dependency (even a local/monorepo one) just to reuse ~20 lines of encoding.
// internal/guardian/guardian_test.go:TestSignAcceptAdminMatchesPinnedPayload pins the
// identical payload encoding on that side.
package canton

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// nttGovernanceModuleMatch is "Ntt" left-padded to 32 bytes -- the NTT governance
// module identifier, matching Wormhole.Ntt.Payload:nttGovernanceModule.
func nttGovernanceModuleMatch() []byte {
	m := make([]byte, 32)
	copy(m[32-3:], []byte("Ntt"))
	return m
}

// signAcceptAdminVAA builds and signs an AcceptAdminToGovernance governance VAA --
// module(32="Ntt") || action(1=1) || chain(2=72) || managerAddress(32) ||
// factoryEpoch(8) -- targeting the caller-supplied (on-ledger-computed)
// managerAddress/factoryEpoch, using the devnet guardian key and the standard
// governance emitter (chain 1, 0x00..04, matching Test.TestCore:govSetFeeVAA).
func signAcceptAdminVAA(managerAddress [32]byte, factoryEpoch uint64, sequence uint64) ([]byte, error) {
	priv, err := crypto.HexToECDSA("cfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0")
	if err != nil {
		return nil, err
	}

	const cantonChain = 72
	payload := make([]byte, 0, 32+1+2+32+8)
	payload = append(payload, nttGovernanceModuleMatch()...)
	payload = append(payload, 0x01) // AcceptAdminTransferToGovernance
	payload = append(payload, 0x00, cantonChain)
	payload = append(payload, managerAddress[:]...)
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, factoryEpoch)
	payload = append(payload, epochBytes...)

	body := make([]byte, 0)
	ts := make([]byte, 4)
	binary.BigEndian.PutUint32(ts, 1700000000)
	body = append(body, ts...)      // timestamp
	body = append(body, 0, 0, 0, 0) // nonce 0
	body = append(body, 0x00, 0x01) // emitterChain 1 (Solana, the standard governance chain)
	body = append(body, b32Match(0x04)...)
	seqBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBytes, sequence)
	body = append(body, seqBytes...) // sequence
	body = append(body, 0x00)        // consistencyLevel 0
	body = append(body, payload...)  // governance payload

	digest := crypto.Keccak256(crypto.Keccak256(body))
	sig, err := crypto.Sign(digest, priv)
	if err != nil {
		return nil, err
	}

	vaa := []byte{0x01}           // version
	vaa = append(vaa, 0, 0, 0, 0) // guardianSetIndex 0
	vaa = append(vaa, 0x01)       // sig count
	vaa = append(vaa, 0x00)       // guardian index 0
	vaa = append(vaa, sig...)
	vaa = append(vaa, body...)
	return vaa, nil
}

// acceptAdminSetup mirrors Test.TestNtt:AcceptAdminSetup's JSON shape.
type acceptAdminSetup struct {
	MgrId          string `json:"mgrId"`
	Operator       string `json:"operator"`
	Admin          string `json:"admin"`
	Gg             string `json:"gg"`
	CoreStateCid   string `json:"coreStateCid"`
	ReplayNodeCid  string `json:"replayNodeCid"`
	ProposalCid    string `json:"proposalCid"`
	ManagerAddress string `json:"managerAddress"`
	FactoryEpoch   int64  `json:"factoryEpoch"`
}

func TestCantonNttAcceptAdminIntegration(t *testing.T) {
	dpm := findDpm(t)

	cantonDir, err := filepath.Abs("../..")
	require.NoError(t, err)
	dar := filepath.Join(cantonDir, "test", ".daml", "dist", "ntt-test-0.1.0.dar")

	port := os.Getenv("CANTON_SANDBOX_PORT")
	if port == "" {
		port = "6865"
	}
	addr := "localhost:" + port

	runCmd(t, cantonDir, dpm, "build", "--all")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sandboxDir := t.TempDir()
	logPath := filepath.Join(sandboxDir, "sandbox.log")
	logFile, err := os.Create(logPath)
	require.NoError(t, err)
	defer logFile.Close()

	sandbox := exec.CommandContext(ctx, dpm, "sandbox", "--no-tty")
	sandbox.Dir = sandboxDir
	sandbox.Stdout = logFile
	sandbox.Stderr = logFile
	sandbox.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, sandbox.Start())
	t.Cleanup(func() {
		if sandbox.Process != nil {
			_ = syscall.Kill(-sandbox.Process.Pid, syscall.SIGKILL)
		}
	})

	require.Eventually(t, func() bool {
		b, _ := os.ReadFile(logPath)
		return strings.Contains(string(b), "Canton sandbox is ready")
	}, 180*time.Second, 2*time.Second, "sandbox never became ready")
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	require.NoError(t, err)
	_ = conn.Close()

	// Step 1: register a deployment (and its standing AdminTransferProposal) and get
	// its ACTUAL, on-ledger-derived managerAddress/factoryEpoch.
	setupFile := filepath.Join(sandboxDir, "setup.json")
	out, err := exec.Command(dpm, "script", "--dar", dar, "--upload-dar", "yes",
		"--script-name", "Test.TestNtt:integrationAcceptAdminSetup",
		"--ledger-host", "localhost", "--ledger-port", port,
		"--output-file", setupFile).CombinedOutput()
	require.NoErrorf(t, err, "setup dpm script failed: %s", out)

	rawSetup, err := os.ReadFile(setupFile)
	require.NoError(t, err)
	var setup acceptAdminSetup
	require.NoError(t, json.Unmarshal(rawSetup, &setup))
	require.NotEmpty(t, setup.Admin)
	require.Contains(t, setup.Admin, "::", "admin should be a full party id")
	t.Logf("step 1: admin=%s managerAddress=%s factoryEpoch=%d", setup.Admin, setup.ManagerAddress, setup.FactoryEpoch)

	managerAddrBytes, err := hex.DecodeString(strings.TrimPrefix(setup.ManagerAddress, "0x"))
	require.NoError(t, err)
	require.Len(t, managerAddrBytes, 32, "managerAddress must be 32 bytes")
	var managerAddr32 [32]byte
	copy(managerAddr32[:], managerAddrBytes)

	// Step 2: sign a FRESH governance VAA, off-chain, against that ACTUAL
	// managerAddress/factoryEpoch -- exactly what the guardians do in production.
	vaaBytes, err := signAcceptAdminVAA(managerAddr32, uint64(setup.FactoryEpoch), 1)
	require.NoError(t, err)
	t.Logf("step 2: signed a fresh accept-admin VAA (%d bytes) for managerAddress=%s", len(vaaBytes), setup.ManagerAddress)

	// Step 3: relay it against the SAME sandbox/manager/proposal from step 1. All
	// internal assertions (the successor's admin is gg; a second relay fails) live
	// inside the script; a non-zero exit means one failed.
	inputPayload := fmt.Sprintf(
		`{"mgrId":%q,"operator":%q,"admin":%q,"gg":%q,"coreStateCid":%q,"replayNodeCid":%q,"proposalCid":%q,"vaaBytes":%q}`,
		setup.MgrId, setup.Operator, setup.Admin, setup.Gg, setup.CoreStateCid, setup.ReplayNodeCid,
		setup.ProposalCid, hex.EncodeToString(vaaBytes),
	)
	inputFile := filepath.Join(sandboxDir, "input.json")
	require.NoError(t, os.WriteFile(inputFile, []byte(inputPayload), 0o600))

	out, err = exec.Command(dpm, "script", "--dar", dar,
		"--script-name", "Test.TestNtt:integrationAcceptAdminByVaa",
		"--ledger-host", "localhost", "--ledger-port", port,
		"--input-file", inputFile).CombinedOutput()
	require.NoErrorf(t, err, "accept-admin-by-vaa dpm script failed: %s", out)
	t.Logf("step 3: accept-admin-by-VAA + replay rejection both verified live: %s", out)
}
