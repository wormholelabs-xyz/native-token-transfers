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

// instrumentIDText is the id of a CIP-56 InstrumentId, valid UTF-8 of 1 to instrumentIDMaxBytes
// bytes (the service bound maxRegistryIdentifierLength).
// Only parseInstrumentIDText builds one.
type instrumentIDText struct{ instrumentIDUTF8Text string }

func parseInstrumentIDText(raw string) (instrumentIDText, error) {
	if !utf8.ValidString(raw) {
		return instrumentIDText{}, newFlowError(http.StatusBadRequest, "instrument-id is not valid UTF-8")
	}
	if len(raw) < instrumentIDMinBytes || len(raw) > instrumentIDMaxBytes {
		return instrumentIDText{}, newFlowError(http.StatusBadRequest, "instrument-id has %d bytes, expected %d to %d",
			len(raw), instrumentIDMinBytes, instrumentIDMaxBytes)
	}
	return instrumentIDText{instrumentIDUTF8Text: raw}, nil
}

// nttManagerID is a Daml Int managerId: 0 to 2^63-1. Only parseNttManagerID builds one.
type nttManagerID struct{ idNumber uint64 }

func (managerID nttManagerID) String() string { return strconv.FormatUint(managerID.idNumber, 10) }

// parseNttManagerID reads a ledger managerId, for example NttGovernance.nextId. It accepts only
// the canonical decimal form. A ledger value outside 0 to 2^63-1 is a 500 that names fieldLabel.
func parseNttManagerID(rawManagerID json.Number, fieldLabel string) (nttManagerID, error) {
	if fieldLabel == "" {
		panic("disclosure-service: parseNttManagerID without a field label")
	}
	parsed, err := strconv.ParseInt(string(rawManagerID), 10, 64)
	if err != nil || parsed < 0 || strconv.FormatInt(parsed, 10) != string(rawManagerID) {
		return nttManagerID{}, newFlowError(http.StatusInternalServerError,
			"%s %q is not a decimal integer from 0 to %s", fieldLabel, truncateForError(string(rawManagerID)), nttManagerIDMaxDecimal)
	}
	managerID := nttManagerID{idNumber: uint64(parsed)}
	if managerID.String() != string(rawManagerID) {
		panic("disclosure-service: parseNttManagerID does not round-trip")
	}
	return managerID, nil
}

// nttNamespace is Wormhole.Ntt.Manager.nttNamespaceFromText:
// "ntt:" ‖ hex(keccak256(utf8(tag) ‖ lp(instrumentAdmin) ‖ lp(instrumentID) ‖ lp(admin) ‖ be64(managerID))).
func nttNamespace(instrumentAdmin partyID, instrumentID instrumentIDText, admin partyID, managerID nttManagerID) replayNamespace {
	// SECURITY: intToBytesN managerId 8 aborts outside 0 to 2^64-1; Daml Int caps it at 2^63-1.
	if managerID.idNumber > uint64(1<<63-1) {
		panic("disclosure-service: nttNamespace managerId exceeds 2^63-1")
	}
	if instrumentID.instrumentIDUTF8Text == "" {
		panic("disclosure-service: nttNamespace of an unparsed instrument id")
	}
	preimage := []byte(nttNamespaceDomainTag)
	preimage = append(preimage, coreLengthPrefixed([]byte(instrumentAdmin))...)
	preimage = append(preimage, coreLengthPrefixed([]byte(instrumentID.instrumentIDUTF8Text))...)
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
