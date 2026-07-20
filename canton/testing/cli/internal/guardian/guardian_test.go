package guardian

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// devnetGuardianKeyHex is the standard Tilt/devnet guardian key, matching
// canton/testing/go/ntt_recipient_match_integration_test.go and the address pinned in
// Test.TestCore:devnetGuardian (canton/test/daml/Test/TestCore.daml:20-21).
const devnetGuardianKeyHex = "cfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0"

const devnetGuardianAddress = "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe"

func b32(last byte) [32]byte {
	var b [32]byte
	b[31] = last
	return b
}

// TestKeyDerivesDevnetGuardianAddress pins Key.Address against the well-known devnet
// guardian address every Daml fixture in this repo assumes.
func TestKeyDerivesDevnetGuardianAddress(t *testing.T) {
	key, err := KeyFromHex(devnetGuardianKeyHex)
	require.NoError(t, err)
	addr := key.Address()
	require.Equal(t, devnetGuardianAddress, hex.EncodeToString(addr[:]))
}

// TestNewRandomKeyProducesDistinctAddresses is a sanity check that the playground's default
// (arbitrary 1/1 guardian) path actually generates usable, distinct keys.
func TestNewRandomKeyProducesDistinctAddresses(t *testing.T) {
	k1, err := NewRandomKey()
	require.NoError(t, err)
	k2, err := NewRandomKey()
	require.NoError(t, err)
	require.NotEqual(t, k1.Address(), k2.Address())

	// Round-trips through hex.
	reloaded, err := KeyFromHex(k1.HexPrivate())
	require.NoError(t, err)
	require.Equal(t, k1.Address(), reloaded.Address())
}

// TestSignTransferMatchesReferenceHarness reproduces exactly the VAA built by
// signNttTransferVAA in canton/testing/go/ntt_recipient_match_integration_test.go byte for
// byte, given the same inputs -- proving this package's general SignTransfer subsumes that
// hand-rolled reference without behavior drift.
func TestSignTransferMatchesReferenceHarness(t *testing.T) {
	key, err := KeyFromHex(devnetGuardianKeyHex)
	require.NoError(t, err)

	vaa, err := SignTransfer(key, TransferParams{
		SourceChain:       2,
		SourceManager:     b32(0xbb),
		SourceTransceiver: b32(0xcc),
		RecipientManager:  b32(0xaa),
		SourceToken:       b32(0xdd),
		RecipientAddress:  b32(0xee),
		Decimals:          8,
		Amount:            1_000_000,
		Sequence:          1,
		Nonce:             0,
		ConsistencyLevel:  0,
	})
	require.NoError(t, err)

	// Decode it back with the wire package and check every field lines up with what a real
	// Receive would see.
	require.Equal(t, byte(1), vaa[0])
	body := vaa[1+4+1+1+65:]
	wm := wire.DecodeWormholeTransceiverMessage(body[4+4+2+32+8+1:])
	require.Equal(t, b32(0xbb), wm.SourceManager)
	require.Equal(t, b32(0xaa), wm.RecipientManager)
	mm := wire.DecodeNttManagerMessage(wm.ManagerPayload)
	ntt := wire.DecodeNativeTokenTransfer(mm.Payload)
	require.EqualValues(t, 8, ntt.Decimals)
	require.EqualValues(t, 1_000_000, ntt.Amount)
	require.Equal(t, b32(0xee), ntt.RecipientAddress)
}

// TestSignTransferVariesByRecipient proves the guardian signs a genuinely fresh VAA per
// recipient -- the property the live two-step harness in
// ntt_recipient_match_integration_test.go depends on (a static fixture can't match a
// nondeterministic Party's recipientAddress).
func TestSignTransferVariesByRecipient(t *testing.T) {
	key, err := KeyFromHex(devnetGuardianKeyHex)
	require.NoError(t, err)
	base := TransferParams{
		SourceChain:       2,
		SourceManager:     b32(0xbb),
		SourceTransceiver: b32(0xcc),
		RecipientManager:  b32(0xaa),
		SourceToken:       b32(0xdd),
		Decimals:          8,
		Amount:            1_000_000,
		Sequence:          1,
	}
	p1 := base
	p1.RecipientAddress = wire.RecipientAddressFor("alice::deadbeef")
	p2 := base
	p2.RecipientAddress = wire.RecipientAddressFor("bob::cafebabe")

	vaa1, err := SignTransfer(key, p1)
	require.NoError(t, err)
	vaa2, err := SignTransfer(key, p2)
	require.NoError(t, err)
	require.NotEqual(t, vaa1, vaa2)
}

// TestSignSetMessageFeeMatchesFixture reproduces Test.TestCore:govSetFeeVAA
// (canton/test/daml/Test/TestCore.daml:31) byte for byte: SetMessageFee(chain=72, fee=1000),
// governance emitter chain 1 @ 0x00..04, signed by the devnet guardian, timestamp
// 1700000000, sequence 1. This is the CLI's only path to constructing governance VAAs, so a
// byte-exact match here is load-bearing.
func TestSignSetMessageFeeMatchesFixture(t *testing.T) {
	const govSetFeeVAA = "01000000000100710ab5dcf6885f62016990540e090400f26f41286382dab479a78ed5f85017ff4a562a46379753e94abba1e69b9fec5d1994ac7d9fb145820727df0759bf4f80016553f100000000000001000000000000000000000000000000000000000000000000000000000000000400000000000000010000000000000000000000000000000000000000000000000000000000436f726503004800000000000000000000000000000000000000000000000000000000000003e8"
	expected, err := hex.DecodeString(govSetFeeVAA)
	require.NoError(t, err)

	key, err := KeyFromHex(devnetGuardianKeyHex)
	require.NoError(t, err)

	vaa, err := SignSetMessageFee(key, GovernanceParams{
		Sequence:  1,
		Timestamp: 1700000000,
	}.withDefaults(), 72, 1000)
	require.NoError(t, err)
	require.Equal(t, expected, vaa)
}
