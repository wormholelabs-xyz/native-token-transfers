//go:build integration

// Integration test for RegisterManagerByVaa/SetFactoryByVaa's happy path:
// the registration binding commits to the genuinely-allocated (operator,
// admin) Party fingerprints, so a VAA signed ahead of time against a static
// fixture can never predict them -- exercising it needs a live sandbox and a
// VAA signed at runtime.
//
//	go test -tags integration -run TestCantonNttRegisterByVaaIntegration . -v
//
// Requires `dpm` (PATH or ~/.dpm/bin) + a JDK; skipped otherwise.
package canton

import (
	"context"
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

	"github.com/stretchr/testify/require"
)

// registerByVaaChainId is the placeholder Wormhole chain id this deployment
// registers under.
const registerByVaaChainId = 72

// registerByVaaTokenDecimals is the tokenDecimals this registration's
// action-2 VAA carries.
const registerByVaaTokenDecimals = 8

// registerByVaaNonce is the instrumentNonce bound into the registration.
const registerByVaaNonce = 0

// registerByVaaSetup mirrors Test.TestNtt:RegisterByVaaSetup's JSON shape.
type registerByVaaSetup struct {
	Operator     string `json:"operator"`
	Admin        string `json:"admin"`
	Gg           string `json:"gg"`
	GovCid       string `json:"govCid"`
	EmReg        string `json:"emReg"`
	RrReg        string `json:"rrReg"`
	CoreStateCid string `json:"coreStateCid"`
	FactoryCid   string `json:"factoryCid"`
}

// registerByVaaResult mirrors Test.TestNtt:RegisterByVaaResult's JSON shape.
type registerByVaaResult struct {
	MgrCid         string `json:"mgrCid"`
	ManagerAddress string `json:"managerAddress"`
	FactoryEpoch   int    `json:"factoryEpoch"`
}

func TestCantonNttRegisterByVaaIntegration(t *testing.T) {
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

	// Step 1: allocate the admin and everything else registration needs,
	// getting real on-ledger party texts.
	setupFile := filepath.Join(sandboxDir, "register-setup.json")
	out, err := exec.Command(dpm, "script", "--dar", dar, "--upload-dar", "yes",
		"--script-name", "Test.TestNtt:integrationRegisterByVaaSetup",
		"--ledger-host", "localhost", "--ledger-port", port,
		"--output-file", setupFile).CombinedOutput()
	require.NoErrorf(t, err, "register-setup dpm script failed: %s", out)

	rawSetup, err := os.ReadFile(setupFile)
	require.NoError(t, err)
	var setup registerByVaaSetup
	require.NoError(t, json.Unmarshal(rawSetup, &setup))
	require.Contains(t, setup.Operator, "::", "operator should be a full party id")
	require.Contains(t, setup.Admin, "::", "admin should be a full party id")
	t.Logf("step 1: operator=%s admin=%s", setup.Operator, setup.Admin)

	// Step 2: compute the registration binding against the real party texts
	// and sign a fresh action-2 VAA -- a static VAA can't predict these
	// fingerprints.
	binding := nttRegistrationBindingFromText(setup.Operator, setup.Admin, registerByVaaNonce)
	registerPayload := encodeRegisterBurnMintManagerPayload(registerByVaaChainId, binding, registerByVaaTokenDecimals)
	registerBody := buildGovernanceVAABody(govEmitterChain, govEmitterAddress(), 301, registerPayload)
	registerVAA := signGovernanceVAA(t, registerBody)
	t.Logf("step 2: signed a fresh action-2 VAA (%d bytes) against registrationBinding=%s", len(registerVAA), hex.EncodeToString(binding[:]))

	registerInputPayload := fmt.Sprintf(
		`{"operator":%q,"admin":%q,"gg":%q,"govCid":%q,"emReg":%q,"rrReg":%q,"coreStateCid":%q,"factoryCid":%q,"nonce":%d,"vaaBytes":%q}`,
		setup.Operator, setup.Admin, setup.Gg, setup.GovCid, setup.EmReg, setup.RrReg, setup.CoreStateCid, setup.FactoryCid,
		registerByVaaNonce, hex.EncodeToString(registerVAA),
	)
	registerInputFile := filepath.Join(sandboxDir, "register-input.json")
	require.NoError(t, os.WriteFile(registerInputFile, []byte(registerInputPayload), 0o600))

	registerResultFile := filepath.Join(sandboxDir, "register-result.json")
	out, err = exec.Command(dpm, "script", "--dar", dar,
		"--script-name", "Test.TestNtt:integrationRegisterByVaa",
		"--ledger-host", "localhost", "--ledger-port", port,
		"--input-file", registerInputFile,
		"--output-file", registerResultFile).CombinedOutput()
	require.NoErrorf(t, err, "register-by-vaa dpm script failed: %s", out)
	t.Logf("step 2: RegisterManagerByVaa happy path + replay rejection both verified live: %s", out)

	rawResult, err := os.ReadFile(registerResultFile)
	require.NoError(t, err)
	var result registerByVaaResult
	require.NoError(t, json.Unmarshal(rawResult, &result))
	require.Equal(t, 0, result.FactoryEpoch, "a freshly registered deployment must start at factoryEpoch 0")

	managerAddrBytes, err := hex.DecodeString(strings.TrimPrefix(result.ManagerAddress, "0x"))
	require.NoError(t, err)
	require.Len(t, managerAddrBytes, 32, "nttManagerAddressFor must be 32 bytes")
	var managerAddr32 [32]byte
	copy(managerAddr32[:], managerAddrBytes)
	t.Logf("step 2: registered mgrCid=%s managerAddress=%s", result.MgrCid, result.ManagerAddress)

	// Step 3: sign a fresh action-3 VAA against the deployment's REAL
	// managerAddress and relay it through SetFactoryByVaa.
	rotatePayload := encodeRotateToCanonicalFactoryPayload(registerByVaaChainId, managerAddr32, uint64(result.FactoryEpoch))
	rotateBody := buildGovernanceVAABody(govEmitterChain, govEmitterAddress(), 302, rotatePayload)
	rotateVAA := signGovernanceVAA(t, rotateBody)
	t.Logf("step 3: signed a fresh action-3 VAA (%d bytes) against managerAddress=%s", len(rotateVAA), result.ManagerAddress)

	rotateInputPayload := fmt.Sprintf(
		`{"operator":%q,"admin":%q,"gg":%q,"mgrCid":%q,"coreStateCid":%q,"factoryCid":%q,"vaaBytes":%q}`,
		setup.Operator, setup.Admin, setup.Gg, result.MgrCid, setup.CoreStateCid, setup.FactoryCid,
		hex.EncodeToString(rotateVAA),
	)
	rotateInputFile := filepath.Join(sandboxDir, "rotate-input.json")
	require.NoError(t, os.WriteFile(rotateInputFile, []byte(rotateInputPayload), 0o600))

	out, err = exec.Command(dpm, "script", "--dar", dar,
		"--script-name", "Test.TestNtt:integrationSetFactoryByVaa",
		"--ledger-host", "localhost", "--ledger-port", port,
		"--input-file", rotateInputFile).CombinedOutput()
	require.NoErrorf(t, err, "set-factory-by-vaa dpm script failed: %s", out)
	t.Logf("step 3: SetFactoryByVaa happy path + replay rejection both verified live: %s", out)
}
