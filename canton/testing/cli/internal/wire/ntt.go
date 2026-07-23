// Package wire is the Go mirror of Wormhole.Ntt.Payload (canton/ntt/daml/Wormhole/Ntt/Payload.daml)
// plus the address-derivation helpers from Wormhole.Ntt.Manager and Wormhole.Core.State.
// It is the CLI's only source of truth for NTT wire encoding: the guardian signer builds
// VAA payloads with it, and the playground's outbound-observation path (Playground.Ops:transferOut)
// re-derives the same bytes on the Daml side so the two must never drift.
//
// Layouts mirror the Daml doc comment in Payload.daml exactly (widths in bytes, big-endian):
//
//	NativeTokenTransfer:
//	  prefix(4) = 0x994E5454 || decimals(1) || amount(8) || sourceToken(32)
//	  || recipientAddress(32) || recipientChain(2) [|| additionalPayloadLen(2) || additionalPayload]
//
//	NttManagerMessage:
//	  id(32) || sender(32) || payloadLen(2) || payload
//
//	WormholeTransceiverMessage (the VAA payload):
//	  prefix(4) = 0x9945FF10 || sourceManager(32) || recipientManager(32)
//	  || managerPayloadLen(2) || managerPayload || transceiverPayloadLen(2) || transceiverPayload
package wire

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// NativeTokenTransferPrefix is 0x994E5454 -- 0x99 'N' 'T' 'T'.
var NativeTokenTransferPrefix = [4]byte{0x99, 0x4e, 0x54, 0x54}

// WormholeTransceiverPrefix is the Wormhole transceiver message prefix, 0x9945FF10.
var WormholeTransceiverPrefix = [4]byte{0x99, 0x45, 0xff, 0x10}

// MaxTrimmedDecimals is the NTT wire's maximum amount scale (Wormhole.Ntt.Amount:maxTrimmedDecimals).
const MaxTrimmedDecimals = 8

// TrimDecimals mirrors Wormhole.Ntt.Amount:trimAmount's decimals clamp: the wire scale for a
// token of tokenDecimals decimals is min(tokenDecimals, 8).
func TrimDecimals(tokenDecimals int) int {
	if tokenDecimals <= MaxTrimmedDecimals {
		return tokenDecimals
	}
	return MaxTrimmedDecimals
}

// NativeTokenTransfer is the innermost NTT payload.
type NativeTokenTransfer struct {
	Decimals          uint8
	Amount            uint64
	SourceToken       [32]byte
	RecipientAddress  [32]byte
	RecipientChain    uint16
	AdditionalPayload []byte // nil/empty when absent
}

// EncodeNativeTokenTransfer mirrors Wormhole.Ntt.Payload:encodeNativeTokenTransfer.
func EncodeNativeTokenTransfer(n NativeTokenTransfer) []byte {
	out := make([]byte, 0, 4+1+8+32+32+2)
	out = append(out, NativeTokenTransferPrefix[:]...)
	out = append(out, n.Decimals)
	out = appendUint64(out, n.Amount)
	out = append(out, n.SourceToken[:]...)
	out = append(out, n.RecipientAddress[:]...)
	out = appendUint16(out, n.RecipientChain)
	if len(n.AdditionalPayload) > 0 {
		out = appendLengthPrefixed(out, n.AdditionalPayload)
	}
	return out
}

// DecodeNativeTokenTransfer mirrors Wormhole.Ntt.Payload:decodeNativeTokenTransfer. Panics
// (like the Daml `error`) on malformed input -- callers dealing with untrusted input should
// recover, but every caller in this CLI decodes bytes it just built or that a script already
// validated.
func DecodeNativeTokenTransfer(b []byte) NativeTokenTransfer {
	if len(b) < 79 {
		panic(fmt.Sprintf("ntt: NativeTokenTransfer too short: %d bytes", len(b)))
	}
	var prefix [4]byte
	copy(prefix[:], b[0:4])
	if prefix != NativeTokenTransferPrefix {
		panic(fmt.Sprintf("ntt: bad NativeTokenTransfer prefix: %x", prefix))
	}
	n := NativeTokenTransfer{
		Decimals:       b[4],
		Amount:         binary.BigEndian.Uint64(b[5:13]),
		RecipientChain: binary.BigEndian.Uint16(b[77:79]),
	}
	copy(n.SourceToken[:], b[13:45])
	copy(n.RecipientAddress[:], b[45:77])
	if len(b) > 79 {
		rest := b[79:]
		alen := int(binary.BigEndian.Uint16(rest[0:2]))
		n.AdditionalPayload = append([]byte{}, rest[2:2+alen]...)
	}
	return n
}

// NttManagerMessage wraps a NativeTokenTransfer with a message id and sending manager.
type NttManagerMessage struct {
	ID      [32]byte
	Sender  [32]byte
	Payload []byte
}

// EncodeNttManagerMessage mirrors Wormhole.Ntt.Payload:encodeNttManagerMessage.
func EncodeNttManagerMessage(m NttManagerMessage) []byte {
	out := make([]byte, 0, 32+32+2+len(m.Payload))
	out = append(out, m.ID[:]...)
	out = append(out, m.Sender[:]...)
	out = appendLengthPrefixed(out, m.Payload)
	return out
}

// DecodeNttManagerMessage mirrors Wormhole.Ntt.Payload:decodeNttManagerMessage.
func DecodeNttManagerMessage(b []byte) NttManagerMessage {
	if len(b) < 66 {
		panic(fmt.Sprintf("ntt: NttManagerMessage too short: %d bytes", len(b)))
	}
	var m NttManagerMessage
	copy(m.ID[:], b[0:32])
	copy(m.Sender[:], b[32:64])
	plen := int(binary.BigEndian.Uint16(b[64:66]))
	m.Payload = append([]byte{}, b[66:66+plen]...)
	return m
}

// WormholeTransceiverMessage is the Wormhole VAA payload for an NTT transfer.
type WormholeTransceiverMessage struct {
	SourceManager      [32]byte
	RecipientManager   [32]byte
	ManagerPayload     []byte
	TransceiverPayload []byte // "" for the Wormhole transceiver
}

// EncodeWormholeTransceiverMessage mirrors Wormhole.Ntt.Payload:encodeWormholeTransceiverMessage.
func EncodeWormholeTransceiverMessage(w WormholeTransceiverMessage) []byte {
	out := make([]byte, 0, 4+32+32+2+len(w.ManagerPayload)+2+len(w.TransceiverPayload))
	out = append(out, WormholeTransceiverPrefix[:]...)
	out = append(out, w.SourceManager[:]...)
	out = append(out, w.RecipientManager[:]...)
	out = appendLengthPrefixed(out, w.ManagerPayload)
	out = appendLengthPrefixed(out, w.TransceiverPayload)
	return out
}

// DecodeWormholeTransceiverMessage mirrors Wormhole.Ntt.Payload:decodeWormholeTransceiverMessage.
func DecodeWormholeTransceiverMessage(b []byte) WormholeTransceiverMessage {
	if len(b) < 4+32+32+2+2 {
		panic(fmt.Sprintf("ntt: WormholeTransceiverMessage too short: %d bytes", len(b)))
	}
	var prefix [4]byte
	copy(prefix[:], b[0:4])
	if prefix != WormholeTransceiverPrefix {
		panic(fmt.Sprintf("ntt: bad WormholeTransceiver prefix: %x", prefix))
	}
	var w WormholeTransceiverMessage
	copy(w.SourceManager[:], b[4:36])
	copy(w.RecipientManager[:], b[36:68])
	mlen := int(binary.BigEndian.Uint16(b[68:70]))
	w.ManagerPayload = append([]byte{}, b[70:70+mlen]...)
	tOffset := 70 + mlen
	tlen := int(binary.BigEndian.Uint16(b[tOffset : tOffset+2]))
	w.TransceiverPayload = append([]byte{}, b[tOffset+2:tOffset+2+tlen]...)
	return w
}

// ----------------------------------------------------------------------
// Address derivation
// ----------------------------------------------------------------------

// lenPrefixed4 prepends a 4-byte big-endian length to b -- the `lp` combinator used by
// Wormhole.Core.Bytes' derivedAddress family (distinct from the NTT wire codec's 2-byte
// length prefix above).
func lenPrefixed4(b []byte) []byte {
	out := make([]byte, 4, 4+len(b))
	binary.BigEndian.PutUint32(out, uint32(len(b)))
	return append(out, b...)
}

// DerivedAddress mirrors Wormhole.Core.Bytes:derivedAddressFromText, the domain-separated
// address scheme every identity address in this system (emitter, NTT manager) uses:
//
//	keccak256(tag || lp(registrarText) || lp(ownerText) || be8(registryId))
//
// registrarText/ownerText are full party ids ("displayName::fingerprint"), matching the
// Daml side's `partyToText`. Pinned against Wormhole.Core.State:testAddressVector and
// Wormhole.Ntt.Manager (nttManagerAddressFor vector in Test.TestNtt:testNttDeployment).
func DerivedAddress(tag, registrarText, ownerText string, registryID uint64) [32]byte {
	data := make([]byte, 0, len(tag)+4+len(registrarText)+4+len(ownerText)+8)
	data = append(data, []byte(tag)...)
	data = append(data, lenPrefixed4([]byte(registrarText))...)
	data = append(data, lenPrefixed4([]byte(ownerText))...)
	idBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(idBytes, registryID)
	data = append(data, idBytes...)
	var out [32]byte
	copy(out[:], crypto.Keccak256(data))
	return out
}

// EmitterAddressTag is Wormhole.Core.State:emitterAddressTag.
const EmitterAddressTag = "wormhole:emitter:v1"

// NttManagerAddressTag is Wormhole.Ntt.Manager:nttManagerAddressTag.
const NttManagerAddressTag = "wormhole:ntt-manager:v1"

// RecipientAddressTag is Wormhole.Ntt.Manager:recipientAddressTag.
const RecipientAddressTag = "wormhole:ntt-recipient:v1"

// RecipientAddressFor mirrors Wormhole.Ntt.Manager:recipientAddressFor /
// recipientAddressFromText: keccak256(tag || lp(utf8(recipientPartyText))). recipientPartyText
// is the recipient's full party id, exactly as `partyToText` renders it on the Daml side.
// Pinned against Test.TestNtt:testRecipientAddressForVector.
func RecipientAddressFor(recipientPartyText string) [32]byte {
	data := append([]byte(RecipientAddressTag), lenPrefixed4([]byte(recipientPartyText))...)
	var out [32]byte
	copy(out[:], crypto.Keccak256(data))
	return out
}

// ----------------------------------------------------------------------
// Small big-endian helpers
// ----------------------------------------------------------------------

func appendUint16(b []byte, v uint16) []byte {
	tmp := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp, v)
	return append(b, tmp...)
}

func appendUint64(b []byte, v uint64) []byte {
	tmp := make([]byte, 8)
	binary.BigEndian.PutUint64(tmp, v)
	return append(b, tmp...)
}

// appendLengthPrefixed appends a 2-byte big-endian length followed by payload -- the NTT wire
// codec's length prefix (encodeLengthPrefixed in Payload.daml), distinct from lenPrefixed4 above.
func appendLengthPrefixed(b []byte, payload []byte) []byte {
	b = appendUint16(b, uint16(len(payload))) //nolint:gosec // NTT payloads are bounded well under 64KiB
	return append(b, payload...)
}
