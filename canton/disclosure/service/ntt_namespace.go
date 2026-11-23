package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"unicode/utf8"
)

// NTT Wormhole.Ntt.Manager.nttNamespaceFromText constants.
const (
	nttNamespaceDomainTag  = "wormhole:ntt-namespace:v1"
	nttNamespacePrefix     = "ntt:"
	nttNamespaceCharacters = len(nttNamespacePrefix) + 2*keccak256DigestByteCount
	instrumentIDMinBytes   = 1
	instrumentIDMaxBytes   = maxRegistryIdentifierLength
	nttManagerIDMaxDecimal = "9223372036854775807"
)

// Compile-time: the derived namespace fits the core replay-namespace bounds.
const (
	_ = uint(replayNamespaceMaxLength - nttNamespaceCharacters)
	_ = uint(nttNamespaceCharacters - replayNamespaceMinLength)
)

// instrumentIDText is a CIP-56 InstrumentId.id: valid UTF-8 of 1 to instrumentIDMaxBytes bytes.
// Only parseInstrumentIDText builds one.
type instrumentIDText struct{ text string }

func parseInstrumentIDText(raw string) (instrumentIDText, error) {
	if !utf8.ValidString(raw) {
		return instrumentIDText{}, newFlowError(http.StatusBadRequest, "instrument-id is not valid UTF-8")
	}
	if len(raw) < instrumentIDMinBytes || len(raw) > instrumentIDMaxBytes {
		return instrumentIDText{}, newFlowError(http.StatusBadRequest, "instrument-id has %d bytes, expected %d to %d",
			len(raw), instrumentIDMinBytes, instrumentIDMaxBytes)
	}
	return instrumentIDText{text: raw}, nil
}

// nttManagerID is a Daml Int managerId: 0 to 2^63-1. Only parseNttManagerID builds one.
type nttManagerID struct{ idNumber uint64 }

func (managerID nttManagerID) String() string { return strconv.FormatUint(managerID.idNumber, 10) }

// parseNttManagerID reads NttGovernance.nextId. It accepts only the canonical decimal form. A
// ledger value outside 0 to 2^63-1 is a 500.
func parseNttManagerID(rawNextID json.Number) (nttManagerID, error) {
	parsed, err := strconv.ParseInt(string(rawNextID), 10, 64)
	if err != nil || parsed < 0 || strconv.FormatInt(parsed, 10) != string(rawNextID) {
		return nttManagerID{}, newFlowError(http.StatusInternalServerError,
			"NttGovernance nextId %q is not a decimal integer from 0 to %s", truncateForError(string(rawNextID)), nttManagerIDMaxDecimal)
	}
	return nttManagerID{idNumber: uint64(parsed)}, nil
}

// nttNamespace is Wormhole.Ntt.Manager.nttNamespaceFromText:
// "ntt:" ‖ hex(keccak256(utf8(tag) ‖ lp(instrumentAdmin) ‖ lp(instrumentID) ‖ lp(admin) ‖ be64(managerID))).
func nttNamespace(instrumentAdmin partyID, instrumentID instrumentIDText, admin partyID, managerID nttManagerID) replayNamespace {
	// SECURITY: intToBytesN managerId 8 aborts outside 0 to 2^64-1; Daml Int caps it at 2^63-1.
	if managerID.idNumber > uint64(1<<63-1) {
		panic("disclosure-service: nttNamespace managerId exceeds 2^63-1")
	}
	if instrumentID.text == "" {
		panic("disclosure-service: nttNamespace of an unparsed instrument id")
	}
	preimage := []byte(nttNamespaceDomainTag)
	preimage = append(preimage, coreLengthPrefixed([]byte(instrumentAdmin))...)
	preimage = append(preimage, coreLengthPrefixed([]byte(instrumentID.text))...)
	preimage = append(preimage, coreLengthPrefixed([]byte(admin))...)
	preimage = binary.BigEndian.AppendUint64(preimage, managerID.idNumber)
	digest := keccak256Digest(preimage)
	namespaceText := nttNamespacePrefix + hex.EncodeToString(digest[:])
	if len(namespaceText) != nttNamespaceCharacters {
		panic("disclosure-service: nttNamespace is not 68 characters")
	}
	namespace, err := parseReplayNamespace(namespaceText)
	if err != nil {
		panic("disclosure-service: nttNamespace is not a valid replay namespace: " + err.Error())
	}
	return namespace
}
