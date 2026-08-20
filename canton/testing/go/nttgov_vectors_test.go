// Vector tooling for the VAA-gated NTT governance actions
// (RegisterManagerByVaa, SetFactoryByVaa): encode/sign helpers and pinned
// payload fixtures matching the Daml hex in Test.TestNtt. Plain `go test`
// (no build tag); never touches a sandbox.
//
// Wire format (Wormhole.Ntt.Payload):
//
//	governance packet: module(32) "Ntt" left-padded ‖ action(1) ‖ chain(2) ‖ <action-specific>
//	  action 1 (accept-admin):        managerAddress(32) ‖ factoryEpoch(8)        -- 75B payload
//	  action 2 (register-burn-mint):  registrationBinding(32) ‖ tokenDecimals(1)  -- 68B payload
//	  action 3 (rotate-factory):      managerAddress(32) ‖ factoryEpoch(8)        -- 75B payload
//
// VAA body: timestamp 1700000000, nonce 0, consistencyLevel 0, guardian set
// index 0, single devnet-guardian signature at index 0.
package canton

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// devnetGuardianKeyHex is the devnet guardian's secp256k1 private key.
const devnetGuardianKeyHex = "cfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0"

// govEmitterChain/govEmitterAddress: the standard governance emitter (chain
// 1, address 0x00..04) every fixture in this file targets.
const govEmitterChain = 1

func govEmitterAddress() [32]byte { return b32Tail(0x04) }

// b32Tail returns a 32-byte address with only the last byte set.
func b32Tail(last byte) [32]byte {
	var b [32]byte
	b[31] = last
	return b
}

// nttGovernanceModuleBytes is "Ntt" (0x4e7474) left-padded to 32 bytes --
// Wormhole.Ntt.Payload.nttGovernanceModule.
func nttGovernanceModuleBytes() []byte {
	m := make([]byte, 32)
	copy(m[29:], []byte{0x4e, 0x74, 0x74})
	return m
}

func be16(v uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return b
}

func be64(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

// encodeAcceptAdminPayload mirrors Wormhole.Ntt.Payload.encodeNttGovernance's
// AcceptAdminToGovernance branch: module ‖ action(1)=1 ‖ chain(2) ‖
// managerAddress(32) ‖ factoryEpoch(8) -- 75 bytes.
func encodeAcceptAdminPayload(chain uint16, managerAddress [32]byte, factoryEpoch uint64) []byte {
	p := nttGovernanceModuleBytes()
	p = append(p, 0x01)
	p = append(p, be16(chain)...)
	p = append(p, managerAddress[:]...)
	p = append(p, be64(factoryEpoch)...)
	return p
}

// encodeRegisterBurnMintManagerPayload mirrors the RegisterBurnMintManager
// branch: module ‖ action(1)=2 ‖ chain(2) ‖ registrationBinding(32) ‖
// tokenDecimals(1) -- 68 bytes.
func encodeRegisterBurnMintManagerPayload(chain uint16, registrationBinding [32]byte, tokenDecimals uint8) []byte {
	p := nttGovernanceModuleBytes()
	p = append(p, 0x02)
	p = append(p, be16(chain)...)
	p = append(p, registrationBinding[:]...)
	p = append(p, tokenDecimals)
	return p
}

// encodeRotateToCanonicalFactoryPayload mirrors the RotateToCanonicalFactory
// branch: identical layout to accept-admin's, under action(1)=3 -- 75 bytes.
func encodeRotateToCanonicalFactoryPayload(chain uint16, managerAddress [32]byte, factoryEpoch uint64) []byte {
	p := nttGovernanceModuleBytes()
	p = append(p, 0x03)
	p = append(p, be16(chain)...)
	p = append(p, managerAddress[:]...)
	p = append(p, be64(factoryEpoch)...)
	return p
}

// buildGovernanceVAABody assembles the VAA body (timestamp ‖ nonce ‖
// emitterChain ‖ emitterAddress ‖ sequence ‖ consistencyLevel ‖ payload),
// matching signNttTransferVAA's fixed timestamp/nonce/consistencyLevel.
func buildGovernanceVAABody(emitterChain uint16, emitterAddress [32]byte, sequence uint64, payload []byte) []byte {
	body := make([]byte, 0, 4+4+2+32+8+1+len(payload))
	body = append(body, be32(1700000000)...) // timestamp
	body = append(body, 0, 0, 0, 0)          // nonce 0
	body = append(body, be16(emitterChain)...)
	body = append(body, emitterAddress[:]...)
	body = append(body, be64(sequence)...)
	body = append(body, 0x00) // consistencyLevel 0
	body = append(body, payload...)
	return body
}

func be32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// signGovernanceVAA signs `body` with the devnet guardian key (set 0, guardian
// index 0) and assembles the full VAA wire encoding: version ‖
// guardianSetIndex(4)=0 ‖ sigCount(1)=1 ‖ guardianIndex(1)=0 ‖ signature(65) ‖
// body. go-ethereum's crypto.Sign is deterministic (RFC6979), so re-signing the
// same body always reproduces the same bytes -- what makes the pinned-payload
// tests below meaningful.
func signGovernanceVAA(t *testing.T, body []byte) []byte {
	t.Helper()
	priv, err := crypto.HexToECDSA(devnetGuardianKeyHex)
	require.NoError(t, err)
	digest := crypto.Keccak256(crypto.Keccak256(body))
	sig, err := crypto.Sign(digest, priv)
	require.NoError(t, err)

	vaa := []byte{0x01}           // version
	vaa = append(vaa, 0, 0, 0, 0) // guardianSetIndex 0
	vaa = append(vaa, 0x01)       // sig count 1
	vaa = append(vaa, 0x00)       // guardian index 0
	vaa = append(vaa, sig...)
	vaa = append(vaa, body...)
	return vaa
}

// ---------------------------------------------------------------------------
// Registration binding: a domain-separated hash preimage (Wormhole.Core.Bytes),
// using a 4-byte length prefix -- NOT the wire codec's 2-byte prefix.
//
// Exported so ntt_register_by_vaa_integration_test.go can compute the same
// binding against real, allocated party texts.
// ---------------------------------------------------------------------------

// nttRegistrationBindingTag is Wormhole.Ntt.Manager.nttRegistrationBindingTag.
const nttRegistrationBindingTag = "wormhole:ntt-registration:v1"

// lenPrefix4 prepends a 4-byte big-endian length to b -- Wormhole.Core.Bytes.lenPrefixed
// (NOT the 2-byte wire-codec prefix lenPrefixedMatch builds).
func lenPrefix4(b []byte) []byte {
	out := make([]byte, 4, 4+len(b))
	binary.BigEndian.PutUint32(out, uint32(len(b))) //nolint:gosec // party/text fields are small
	return append(out, b...)
}

// nttRegistrationBindingFromText mirrors Wormhole.Ntt.Manager.nttRegistrationBindingFromText:
//
//	keccak256(tag ‖ lp4(utf8(operatorText)) ‖ lp4(utf8(adminText)) ‖ uint64be(nonce))
func nttRegistrationBindingFromText(operatorText, adminText string, nonce uint64) [32]byte {
	pre := []byte(nttRegistrationBindingTag)
	pre = append(pre, lenPrefix4([]byte(operatorText))...)
	pre = append(pre, lenPrefix4([]byte(adminText))...)
	pre = append(pre, be64(nonce)...)
	var out [32]byte
	copy(out[:], crypto.Keccak256(pre))
	return out
}

// TestNttRegistrationBindingFromTextPinned pins nttRegistrationBindingFromText
// against Test.TestNtt's fixed vector. If it fails, this encoder has drifted
// from the Daml side -- fix here, not the Daml constant.
func TestNttRegistrationBindingFromTextPinned(t *testing.T) {
	a0 := nttRegistrationBindingFromText("vector-operator::1220deadbeef", "vector-admin::1220cafebabe", 0)
	a1 := nttRegistrationBindingFromText("vector-operator::1220deadbeef", "vector-admin::1220cafebabe", 1)
	b0 := nttRegistrationBindingFromText("vector-operator::1220aaaaaaaa", "vector-admin::1220cafebabe", 0)
	c0 := nttRegistrationBindingFromText("vector-operator::1220deadbeef", "vector-admin::1220babababa", 0)
	require.Equal(t, "ade4a39770a697a4d00f9b97591986dedb27e8c1f51b12f1cc51b97a07f1e159", hex.EncodeToString(a0[:]))
	require.NotEqual(t, a0, a1)
	require.NotEqual(t, a0, b0)
	require.NotEqual(t, a0, c0)
}

// ---------------------------------------------------------------------------
// Ground truth: reproduce the pinned Daml hex byte-for-byte.
// ---------------------------------------------------------------------------

// TestReproduceAcceptAdminHappyVAA regenerates Test.TestNtt's
// acceptAdminHappyVAA byte-for-byte. If it fails, this encoder has drifted
// from the wire format -- fix here, not the Daml constant.
func TestReproduceAcceptAdminHappyVAA(t *testing.T) {
	payload := encodeAcceptAdminPayload(72, b32Tail(0xaa), 0)
	body := buildGovernanceVAABody(govEmitterChain, govEmitterAddress(), 1, payload)
	got := hex.EncodeToString(signGovernanceVAA(t, body))
	want := "010000000001009391f78297b7bd3fd25a52651e2b18216b625228226059245ef77cf342f971d11beca86f1b1de69ee726c35616edb0f04fcaf69ec0dfbcc9f2be29e0c62fcdb1016553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000010000000000000000000000000000000000000000000000000000000000004e747401004800000000000000000000000000000000000000000000000000000000000000aa0000000000000000"
	require.Equal(t, want, got, "encoder/signer no longer reproduces the pinned acceptAdminHappyVAA fixture")
}

// coreModuleBytes is "Core" (0x436f7265) left-padded to 32 bytes --
// Wormhole.Core.Governance.coreModule.
func coreModuleBytes() []byte {
	m := make([]byte, 32)
	copy(m[28:], []byte{0x43, 0x6f, 0x72, 0x65})
	return m
}

// encodeSetMessageFeePayload mirrors Wormhole.Core.Governance's SetMessageFee
// wire format: module ‖ action(1)=3 ‖ chain(2) ‖ fee(32) -- 67 bytes.
func encodeSetMessageFeePayload(chain uint16, fee uint64) []byte {
	p := coreModuleBytes()
	p = append(p, 0x03)
	p = append(p, be16(chain)...)
	feeBytes := make([]byte, 32)
	binary.BigEndian.PutUint64(feeBytes[24:], fee)
	return append(p, feeBytes...)
}

// TestReproduceGovSetFeeVAA regenerates Test.TestCore's govSetFeeVAA
// byte-for-byte: SetMessageFee(chain=75, fee=1000) for wormhole-core 0.5.0's
// cantonChainId. If it fails, this encoder has drifted from the wire format
// -- fix here, not the Daml constant.
func TestReproduceGovSetFeeVAA(t *testing.T) {
	payload := encodeSetMessageFeePayload(75, 1000)
	body := buildGovernanceVAABody(govEmitterChain, govEmitterAddress(), 1, payload)
	got := hex.EncodeToString(signGovernanceVAA(t, body))
	want := "01000000000100797d5065f8fef799fdfb8e1042eccc143a77b35a4d9d2689474cdbf8500bffe46678c27b114d015c45f175fb5bd687cff92c4238d31035bacb65ddf8ae99b6f8006553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000010000000000000000000000000000000000000000000000000000000000436f726503004b00000000000000000000000000000000000000000000000000000000000003e8"
	require.Equal(t, want, got, "encoder/signer no longer reproduces the pinned govSetFeeVAA fixture")
}

// TestPinnedRegisterBurnMintManagerPayload pins the action-2 payload encoding
// -- ground truth for Test.TestNtt's codec round-trip and registerHappyVAA below.
func TestPinnedRegisterBurnMintManagerPayload(t *testing.T) {
	payload := encodeRegisterBurnMintManagerPayload(72, b32Tail(0xaa), 8)
	require.Equal(t,
		"00000000000000000000000000000000000000000000000000000000004e747402004800000000000000000000000000000000000000000000000000000000000000aa08",
		hex.EncodeToString(payload))
}

// TestPinnedRotateToCanonicalFactoryPayload pins the action-3 payload encoding
// -- ground truth for Test.TestNtt's codec round-trip and rotateHappyVAA below.
func TestPinnedRotateToCanonicalFactoryPayload(t *testing.T) {
	payload := encodeRotateToCanonicalFactoryPayload(72, b32Tail(0xaa), 0)
	require.Equal(t,
		"00000000000000000000000000000000000000000000000000000000004e747403004800000000000000000000000000000000000000000000000000000000000000aa0000000000000000",
		hex.EncodeToString(payload))
}

// nttGovVector names one fixture the Daml tests pin, and how to build it.
type nttGovVector struct {
	name    string
	payload []byte
	// emitterChain/emitterAddress/sequence override the standard governance
	// emitter (chain 1, 0x00..04) when set to zero-value they default there.
	emitterChain   uint16
	emitterAddress [32]byte
	sequence       uint64
}

// nttGovVectors is every RegisterManagerByVaa/SetFactoryByVaa Daml fixture
// this package's tests need, keyed by the Daml constant name they back.
// Sequence numbers only need to be distinct per fixture; 101+/201+ avoid the
// acceptAdmin* range (1-8) in Test.TestNtt.
func nttGovVectors(t *testing.T) []nttGovVector {
	t.Helper()
	std := govEmitterAddress()
	return []nttGovVector{
		{
			name:           "registerHappyVAA",
			payload:        encodeRegisterBurnMintManagerPayload(72, b32Tail(0xaa), 8),
			emitterChain:   govEmitterChain,
			emitterAddress: std,
			sequence:       101,
		},
		{
			name:           "registerWrongEmitterChainVAA",
			payload:        encodeRegisterBurnMintManagerPayload(72, b32Tail(0xaa), 8),
			emitterChain:   2,
			emitterAddress: std,
			sequence:       102,
		},
		{
			name:           "registerWrongEmitterAddressVAA",
			payload:        encodeRegisterBurnMintManagerPayload(72, b32Tail(0xaa), 8),
			emitterChain:   govEmitterChain,
			emitterAddress: b32Tail(0x05),
			sequence:       103,
		},
		{
			name:           "registerChainZeroVAA",
			payload:        encodeRegisterBurnMintManagerPayload(0, b32Tail(0xaa), 8),
			emitterChain:   govEmitterChain,
			emitterAddress: std,
			sequence:       104,
		},
		{
			name:           "registerDecimalsOutOfRangeVAA",
			payload:        encodeRegisterBurnMintManagerPayload(72, b32Tail(0xaa), 11),
			emitterChain:   govEmitterChain,
			emitterAddress: std,
			sequence:       105,
		},
		{
			name:           "rotateHappyVAA",
			payload:        encodeRotateToCanonicalFactoryPayload(72, b32Tail(0xaa), 0),
			emitterChain:   govEmitterChain,
			emitterAddress: std,
			sequence:       201,
		},
		{
			name:           "rotateWrongEpochVAA",
			payload:        encodeRotateToCanonicalFactoryPayload(72, b32Tail(0xaa), 1),
			emitterChain:   govEmitterChain,
			emitterAddress: std,
			sequence:       202,
		},
	}
}

// TestNttGovVectorsPinned pins every fixture above against its known-good hex
// -- ground truth for the identical constants in Test.TestNtt.
func TestNttGovVectorsPinned(t *testing.T) {
	want := map[string]string{
		"registerHappyVAA":               "01000000000100a67474776130521854fa617c73ef5960cc49ba787dd9b7306945fec0c6563c8917cf1e7eb31cafa39609ee63f6036539c48c77ba973ebb7c5ec2c79bf9911ff1006553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000650000000000000000000000000000000000000000000000000000000000004e747402004800000000000000000000000000000000000000000000000000000000000000aa08",
		"registerWrongEmitterChainVAA":   "01000000000100b96c02e6b0b016f779a3f6070fc1ad24f196e3e0b3bd16b1d07d4c024c5457462ee65b528221b1820b0b2b1255166194510d98591b393ed17a97d062d328bfd8006553f100000000000002000000000000000000000000000000000000000000000000000000000000000400000000000000660000000000000000000000000000000000000000000000000000000000004e747402004800000000000000000000000000000000000000000000000000000000000000aa08",
		"registerWrongEmitterAddressVAA": "010000000001005826de69e735575d94655fe4a01a68ce3695a2e4efafd3fa83becb310297859020b846bf0b73d41a6c9ba5eb603b42a63ff79723bff27fe451b2f20a40803091016553f100000000000001000000000000000000000000000000000000000000000000000000000000000500000000000000670000000000000000000000000000000000000000000000000000000000004e747402004800000000000000000000000000000000000000000000000000000000000000aa08",
		"registerChainZeroVAA":           "0100000000010063363e0d4398a2d72dbf429c4bf3d5d54ff08032ed20759839450ca8439f3e6050b59f703895cff0c92907763d7eaaebf833f4a7afe64addbc904e5d4ab2d1c6006553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000680000000000000000000000000000000000000000000000000000000000004e747402000000000000000000000000000000000000000000000000000000000000000000aa08",
		"registerDecimalsOutOfRangeVAA":  "01000000000100d83ef9c4c90ebdc32d319d4884c34be30817b5cabe225b9e4dc2e10dbd6abdab0b26085fe9b1db23fd0763b9a113a9971c4fd4587bb5a56bb7da1ee8472ce14b016553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000690000000000000000000000000000000000000000000000000000000000004e747402004800000000000000000000000000000000000000000000000000000000000000aa0b",
		"rotateHappyVAA":                 "0100000000010088d597b295742eac595a92f786ca1e54aeab7d4f46b4eb6b0027e93a8e3a532e4afa355bd84afb6af7b7a413288697d88783c239f51b5a66c389fd62fb4ca810016553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000c90000000000000000000000000000000000000000000000000000000000004e747403004800000000000000000000000000000000000000000000000000000000000000aa0000000000000000",
		"rotateWrongEpochVAA":            "01000000000100e50b6e1c58de444c05edbfed65204f385ef1f6414aebd2eb85f06a4492efc5d20051b05a16a72012467f3daacf00dde5183dc1066ec6519fa99ee5adfe4aad0f016553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000ca0000000000000000000000000000000000000000000000000000000000004e747403004800000000000000000000000000000000000000000000000000000000000000aa0000000000000001",
	}
	for _, v := range nttGovVectors(t) {
		v := v
		t.Run(v.name, func(t *testing.T) {
			body := buildGovernanceVAABody(v.emitterChain, v.emitterAddress, v.sequence, v.payload)
			got := hex.EncodeToString(signGovernanceVAA(t, body))
			require.Equal(t, want[v.name], got)
		})
	}
}

// TestPrintNttGovVectors (re)generates every fixture's `name = "hex"` line
// for pasting into Test.TestNtt. Skipped unless NTTGOV_PRINT=1.
func TestPrintNttGovVectors(t *testing.T) {
	if os.Getenv("NTTGOV_PRINT") != "1" {
		t.Skip("set NTTGOV_PRINT=1 to print the fixture hex")
	}
	for _, v := range nttGovVectors(t) {
		body := buildGovernanceVAABody(v.emitterChain, v.emitterAddress, v.sequence, v.payload)
		t.Logf("%s = %q", v.name, hex.EncodeToString(signGovernanceVAA(t, body)))
	}
}
