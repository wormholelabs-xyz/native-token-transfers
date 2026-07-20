package wire

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// b32 returns a 32-byte array whose last byte is last -- matching the 0x00..XX
// peer/manager addresses used throughout the Daml fixtures.
func b32(last byte) [32]byte {
	var b [32]byte
	b[31] = last
	return b
}

// TestNttCodecRoundTrip mirrors Test.TestNtt:testNttCodec exactly (same field values),
// checking the Go codec round-trips the same as the Daml one.
func TestNttCodecRoundTrip(t *testing.T) {
	ntt := NativeTokenTransfer{
		Decimals:         8,
		Amount:           1000000,
		SourceToken:      b32(0xdd),
		RecipientAddress: b32(0xee),
		RecipientChain:   72,
	}
	require.Equal(t, ntt, DecodeNativeTokenTransfer(EncodeNativeTokenTransfer(ntt)))

	mm := NttManagerMessage{
		ID:      b32(0x01),
		Sender:  b32(0xbb),
		Payload: EncodeNativeTokenTransfer(ntt),
	}
	require.Equal(t, mm, DecodeNttManagerMessage(EncodeNttManagerMessage(mm)))

	wm := WormholeTransceiverMessage{
		SourceManager:      b32(0xbb),
		RecipientManager:   b32(0xaa),
		ManagerPayload:     EncodeNttManagerMessage(mm),
		TransceiverPayload: nil,
	}
	got := DecodeWormholeTransceiverMessage(EncodeWormholeTransceiverMessage(wm))
	require.Equal(t, wm.SourceManager, got.SourceManager)
	require.Equal(t, wm.RecipientManager, got.RecipientManager)
	require.Equal(t, wm.ManagerPayload, got.ManagerPayload)
	require.Empty(t, got.TransceiverPayload)
}

// TestNttTransferVAAPayloadMatchesFixture decodes the exact wire bytes embedded in
// Test.TestNtt:nttTransferVAA (canton/test/daml/Test/TestNtt.daml:560) -- source chain 2,
// peer transceiver 0x..cc, peer manager 0x..bb, our manager 0x..aa, transfer of 1_000_000
// @ 8 decimals of token 0x..dd to 0x..ee on chain 72 -- proving the Go decoder reads the
// same VAA payload bytes the Daml `Receive` choice consumes.
func TestNttTransferVAAPayloadMatchesFixture(t *testing.T) {
	const nttTransferVAA = "010000000001009e180f3b0c9a9b4d77e9be4c25e67db813e222af53f3fff09219c2b1c43570ea003de6b1c3f91ad6a80a7c8c5b32ee349dd2e8778a77f1d2638414a18821de72016553f10000000000000200000000000000000000000000000000000000000000000000000000000000cc0000000000000001009945ff1000000000000000000000000000000000000000000000000000000000000000bb00000000000000000000000000000000000000000000000000000000000000aa0091000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000bb004f994e54540800000000000f424000000000000000000000000000000000000000000000000000000000000000dd00000000000000000000000000000000000000000000000000000000000000ee00480000"

	raw, err := hex.DecodeString(nttTransferVAA)
	require.NoError(t, err)

	// VAA envelope: version(1) gsIndex(4) sigCount(1) [gIndex(1) sig(65)] body.
	require.Equal(t, byte(1), raw[0])
	sigCount := raw[5]
	require.Equal(t, byte(1), sigCount)
	body := raw[6+1+65:]

	// Body: timestamp(4) nonce(4) emitterChain(2) emitterAddress(32) sequence(8) consistency(1) payload.
	emitterChain := int(body[8])<<8 | int(body[9])
	require.Equal(t, 2, emitterChain)
	var emitterAddr [32]byte
	copy(emitterAddr[:], body[10:42])
	require.Equal(t, b32(0xcc), emitterAddr)
	payload := body[51:]

	wm := DecodeWormholeTransceiverMessage(payload)
	require.Equal(t, b32(0xbb), wm.SourceManager)
	require.Equal(t, b32(0xaa), wm.RecipientManager)

	mm := DecodeNttManagerMessage(wm.ManagerPayload)
	require.Equal(t, b32(0xbb), mm.Sender)

	ntt := DecodeNativeTokenTransfer(mm.Payload)
	require.EqualValues(t, 8, ntt.Decimals)
	require.EqualValues(t, 1000000, ntt.Amount)
	require.Equal(t, b32(0xdd), ntt.SourceToken)
	require.Equal(t, b32(0xee), ntt.RecipientAddress)
	require.EqualValues(t, 72, ntt.RecipientChain)

	// Re-encoding must reproduce the exact fixture payload bytes.
	reEncoded := EncodeWormholeTransceiverMessage(WormholeTransceiverMessage{
		SourceManager:    wm.SourceManager,
		RecipientManager: wm.RecipientManager,
		ManagerPayload: EncodeNttManagerMessage(NttManagerMessage{
			ID:      mm.ID,
			Sender:  mm.Sender,
			Payload: EncodeNativeTokenTransfer(ntt),
		}),
	})
	require.Equal(t, payload, reEncoded)
}

// TestRecipientAddressForVector pins RecipientAddressFor against
// Test.TestNtt:testRecipientAddressForVector (TestNtt.daml:570-575) -- the one static
// vector for a formula whose real inputs (live Party ids) are otherwise nondeterministic.
func TestRecipientAddressForVector(t *testing.T) {
	got := RecipientAddressFor("vector-recipient::1220deadbeef")
	require.Equal(t, "dbd2d1b09ea5715be3e45d0e38e5c5c0f6fcd0f181d45a9620dfb20f8ae3a4d9", hex.EncodeToString(got[:]))

	other := RecipientAddressFor("vector-recipient::1220cafebabe")
	require.NotEqual(t, got, other)
}

// TestDerivedAddressVectors pins DerivedAddress against Test.TestCore:testAddressVector
// (emitter) and Test.TestNtt:testNttDeployment's manager-address vector (TestNtt.daml:489),
// confirming the Go and Daml encoders agree and that the emitter/manager domain tags
// separate identical (registrar, owner, id) inputs.
func TestDerivedAddressVectors(t *testing.T) {
	registrar := "vector-operator::1220deadbeef"
	owner := "vector-owner::1220cafebabe"

	emitter := DerivedAddress(EmitterAddressTag, registrar, owner, 7)
	require.Equal(t, "45ae8886d1c165b071d3b1202fe81ebedc39f61a6a91a1d13140eb64857f3f2b", hex.EncodeToString(emitter[:]))

	manager := DerivedAddress(NttManagerAddressTag, registrar, owner, 7)
	require.Equal(t, "27d9c172814411dbdacf4842ae5de98cf78ce21e77def6f76f0f75387a28c93b", hex.EncodeToString(manager[:]))

	require.NotEqual(t, emitter, manager)
}
