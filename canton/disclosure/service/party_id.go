package main

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	partyFingerprintSeparator = "::"
	// partyFingerprintPrefix is the multihash prefix of a SHA-256 namespace fingerprint.
	partyFingerprintPrefix    = "1220"
	partyFingerprintHexLength = 64
	maxPartyHintBytes         = maxPartyIDBytes - len(partyFingerprintSeparator) - len(partyFingerprintPrefix) - partyFingerprintHexLength
)

// Compile-time: maxPartyHintBytes == 185.
const (
	_ = uint(maxPartyHintBytes - 185)
	_ = uint(185 - maxPartyHintBytes)
)

// parseRequestPartyID accepts only "<hint>::1220<64 lowercase hex>". The hint is 1 to 185
// characters of [A-Za-z0-9._-]. A malformed value is a 400 with a fixed reason.
func parseRequestPartyID(rawParty, paramName string) (partyID, error) {
	hint, fingerprint, hasSeparator := strings.Cut(rawParty, partyFingerprintSeparator)
	if !hasSeparator {
		return "", newPartyIDError(paramName, "no \"::\" separator")
	}
	if len(hint) < 1 || len(hint) > maxPartyHintBytes {
		return "", newPartyIDError(paramName, fmt.Sprintf("the hint is %d bytes, it must be 1 to %d", len(hint), maxPartyHintBytes))
	}
	for index := range len(hint) {
		if !isPartyHintCharacter(hint[index]) {
			return "", newPartyIDError(paramName, "the hint has a character outside [A-Za-z0-9._-]")
		}
	}
	if len(fingerprint) != len(partyFingerprintPrefix)+partyFingerprintHexLength {
		return "", newPartyIDError(paramName, fmt.Sprintf("the fingerprint is %d characters, it must be %d", len(fingerprint), len(partyFingerprintPrefix)+partyFingerprintHexLength))
	}
	if !strings.HasPrefix(fingerprint, partyFingerprintPrefix) {
		return "", newPartyIDError(paramName, "the fingerprint does not start with "+partyFingerprintPrefix)
	}
	for index := range len(fingerprint) {
		if !isLowercaseHexCharacter(fingerprint[index]) {
			return "", newPartyIDError(paramName, "the fingerprint is not lowercase hex")
		}
	}
	if len(rawParty) > maxPartyIDBytes {
		panic("disclosure-service: parsed party id exceeds maxPartyIDBytes")
	}
	return partyID(rawParty), nil
}

func newPartyIDError(paramName, reason string) error {
	return newFlowError(http.StatusBadRequest, "%s must be a party id \"<hint>::1220<64 lowercase hex>\": %s", paramName, reason)
}

func isPartyHintCharacter(character byte) bool {
	switch {
	case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
		return true
	default:
		return character == '.' || character == '_' || character == '-'
	}
}

func isLowercaseHexCharacter(character byte) bool {
	return (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')
}
