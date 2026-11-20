package main

import (
	"encoding/json"
	"fmt"
)

// damlByteValue is the Ledger API JSON of a Daml constrained byte value: a
// variant {"tag": "<type name>", "value": "<lowercase hex>"} (wormhole-core
// Wormhole.Core.Bytes, ntt Wormhole.Ntt.Bytes).
type damlByteValue struct {
	Tag   *string `json:"tag"`
	Value *string `json:"value"`
}

// decodeDamlByteValue requires the variant of typeName whose value is lowercase hex of
// minimumHexLength to maximumHexLength characters.
func decodeDamlByteValue(data []byte, typeName string, minimumHexLength, maximumHexLength int) (string, error) {
	var byteValue damlByteValue
	if err := json.Unmarshal(data, &byteValue); err != nil {
		return "", fmt.Errorf("%s: not a {tag, value} variant: %w", typeName, err)
	}
	if byteValue.Tag == nil || byteValue.Value == nil {
		return "", fmt.Errorf("%s: variant has no tag or no value", typeName)
	}
	if *byteValue.Tag != typeName {
		return "", fmt.Errorf("%s: variant tag is %q", typeName, *byteValue.Tag)
	}
	hexText := *byteValue.Value
	if len(hexText) < minimumHexLength || len(hexText) > maximumHexLength {
		return "", fmt.Errorf("%s: value has %d hex characters, expected %d to %d", typeName, len(hexText), minimumHexLength, maximumHexLength)
	}
	if !isLowercaseHex(hexText) {
		return "", fmt.Errorf("%s: value is not lowercase hex", typeName)
	}
	return hexText, nil
}

func isLowercaseHex(text string) bool {
	for _, character := range text {
		isDigit := character >= '0' && character <= '9'
		isLowercaseLetter := character >= 'a' && character <= 'f'
		if !isDigit && !isLowercaseLetter {
			return false
		}
	}
	return true
}

// nttManagerAddressValue is an NttManagerAddress: 64 lowercase hex characters.
type nttManagerAddressValue struct{ hex string }

func (address *nttManagerAddressValue) UnmarshalJSON(data []byte) error {
	hexText, err := decodeDamlByteValue(data, "NttManagerAddress", 64, 64)
	if err != nil {
		return err
	}
	address.hex = hexText
	return nil
}

func (address nttManagerAddressValue) String() string { return address.hex }

// digestNibblePrefixValue is a DigestNibblePrefix: 0 to 63 lowercase hex characters.
type digestNibblePrefixValue struct{ hex string }

func (prefix *digestNibblePrefixValue) UnmarshalJSON(data []byte) error {
	hexText, err := decodeDamlByteValue(data, "DigestNibblePrefix", 0, 63)
	if err != nil {
		return err
	}
	prefix.hex = hexText
	return nil
}
