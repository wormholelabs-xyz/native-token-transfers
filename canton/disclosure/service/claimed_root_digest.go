package main

import (
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"unicode/utf8"
)

// Core Wormhole.Core.Replay constants.
const (
	claimedRootsNamespace     = "wormhole-core:replay-roots"
	claimedRootDomainTag      = "wormhole-core:replay-root-claim"
	replayNamespaceMinLength  = 1
	replayNamespaceMaxLength  = 128
	coreLengthPrefixByteCount = 4
)

// replayNamespace is a replay namespace that ClaimReplayRoot accepts: valid UTF-8 of 1 to 128
// characters, other than claimedRootsNamespace. Only parseReplayNamespace builds one. The zero
// value is an absent namespace.
type replayNamespace struct{ text string }

func (namespace replayNamespace) isPresent() bool { return namespace.text != "" }

// parseReplayNamespace counts characters as Daml T.length does: Unicode code points.
func parseReplayNamespace(raw string) (replayNamespace, error) {
	if !utf8.ValidString(raw) {
		return replayNamespace{}, newFlowError(http.StatusBadRequest, "namespace is not valid UTF-8")
	}
	characterCount := utf8.RuneCountInString(raw)
	if characterCount < replayNamespaceMinLength || characterCount > replayNamespaceMaxLength {
		return replayNamespace{}, newFlowError(http.StatusBadRequest, "namespace has %d characters, expected %d to %d",
			characterCount, replayNamespaceMinLength, replayNamespaceMaxLength)
	}
	if raw == claimedRootsNamespace {
		return replayNamespace{}, newFlowError(http.StatusBadRequest, "namespace is reserved for the core")
	}
	return replayNamespace{text: raw}, nil
}

// coreLengthPrefixed is Daml lenPrefixed: a 4-byte big-endian byte count, then the bytes.
func coreLengthPrefixed(payload []byte) []byte {
	if uint64(len(payload)) > 0xffffffff {
		panic("disclosure-service: length-prefixed payload exceeds uint32")
	}
	prefixed := make([]byte, coreLengthPrefixByteCount, coreLengthPrefixByteCount+len(payload))
	binary.BigEndian.PutUint32(prefixed, uint32(len(payload)))
	return append(prefixed, payload...)
}

// claimedRootDigest is core claimedRootDigest:
// keccak256(utf8(tag) ‖ lp(utf8(consumer)) ‖ lp(utf8(namespace))).
func claimedRootDigest(consumer partyID, namespace replayNamespace) vaaDigestHex {
	if !namespace.isPresent() {
		panic("disclosure-service: claimedRootDigest of an absent namespace")
	}
	preimage := []byte(claimedRootDomainTag)
	preimage = append(preimage, coreLengthPrefixed([]byte(consumer))...)
	preimage = append(preimage, coreLengthPrefixed([]byte(namespace.text))...)
	digest := keccak256Digest(preimage)
	digestHex := hex.EncodeToString(digest[:])
	if len(digestHex) != 64 {
		panic("disclosure-service: claimedRootDigest is not 64 hex characters")
	}
	return vaaDigestHex{hex: digestHex}
}
