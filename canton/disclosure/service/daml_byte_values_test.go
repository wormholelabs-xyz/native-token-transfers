package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNttManagerAddressValueDecodesTheVariantOnly(t *testing.T) {
	address := strings.Repeat("ab", 32)
	cases := []struct {
		name      string
		json      string
		wantHex   string
		wantError string
	}{
		{"variant", `{"tag":"NttManagerAddress","value":"` + address + `"}`, address, ""},
		{"plain string", `"` + address + `"`, "", "NttManagerAddress: not a {tag, value} variant"},
		{"another tag", `{"tag":"UniversalAddress","value":"` + address + `"}`, "", `NttManagerAddress: variant tag is "UniversalAddress"`},
		{"no tag", `{"value":"` + address + `"}`, "", "NttManagerAddress: variant has no tag or no value"},
		{"no value", `{"tag":"NttManagerAddress"}`, "", "NttManagerAddress: variant has no tag or no value"},
		{"null value", `{"tag":"NttManagerAddress","value":null}`, "", "NttManagerAddress: variant has no tag or no value"},
		{"uppercase", `{"tag":"NttManagerAddress","value":"` + strings.ToUpper(address) + `"}`, "", "NttManagerAddress: value is not lowercase hex"},
		{"0x prefix", `{"tag":"NttManagerAddress","value":"0x` + address[2:] + `"}`, "", "NttManagerAddress: value is not lowercase hex"},
		{"31 bytes", `{"tag":"NttManagerAddress","value":"` + address[2:] + `"}`, "", "NttManagerAddress: value has 62 hex characters, expected 64 to 64"},
		{"33 bytes", `{"tag":"NttManagerAddress","value":"` + address + `ab"}`, "", "NttManagerAddress: value has 66 hex characters, expected 64 to 64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var decoded nttManagerAddressValue
			err := json.Unmarshal([]byte(tc.json), &decoded)
			if tc.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantHex, decoded.hex)
		})
	}
}

func TestDigestNibblePrefixValueDecodesZeroToSixtyThreeNibbles(t *testing.T) {
	cases := []struct {
		name      string
		json      string
		wantHex   string
		wantError string
	}{
		{"root prefix", `{"tag":"DigestNibblePrefix","value":""}`, "", ""},
		{"one nibble", `{"tag":"DigestNibblePrefix","value":"a"}`, "a", ""},
		{"63 nibbles", `{"tag":"DigestNibblePrefix","value":"` + strings.Repeat("f", 63) + `"}`, strings.Repeat("f", 63), ""},
		{"64 nibbles", `{"tag":"DigestNibblePrefix","value":"` + strings.Repeat("f", 64) + `"}`, "", "DigestNibblePrefix: value has 64 hex characters, expected 0 to 63"},
		{"uppercase", `{"tag":"DigestNibblePrefix","value":"A"}`, "", "DigestNibblePrefix: value is not lowercase hex"},
		{"plain string", `"a"`, "", "DigestNibblePrefix: not a {tag, value} variant"},
		{"another tag", `{"tag":"DigestNibbleSuffix","value":"a"}`, "", `DigestNibblePrefix: variant tag is "DigestNibbleSuffix"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var decoded digestNibblePrefixValue
			err := json.Unmarshal([]byte(tc.json), &decoded)
			if tc.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantHex, decoded.hex)
		})
	}
}
