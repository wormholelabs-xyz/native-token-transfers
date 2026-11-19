package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRequestPartyID(t *testing.T) {
	fingerprint := strings.Repeat("ab", 32)
	validParty := "alice::1220" + fingerprint
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid", validParty, false},
		{"hint with dash, underscore, digit", "a-b_c9::1220" + fingerprint, false},
		{"hint with a space", "alice bob::1220" + fingerprint, false},
		{"hint with a colon", "a:b::1220" + fingerprint, false},
		{"hint that starts with a colon", ":a::1220" + fingerprint, false},
		{"hint that starts with a space", " alice::1220" + fingerprint, false},
		{"hint that ends with a space", "alice ::1220" + fingerprint, false},
		{"hint of 1 character", "a::1220" + fingerprint, false},
		{"hint of 185 characters", strings.Repeat("h", 185) + "::1220" + fingerprint, false},
		{"hint of 185 characters of the full set", strings.Repeat("aZ0 :-_", 26) + "aZ0::1220" + fingerprint, false},
		{"hint of 186 characters", strings.Repeat("h", 186) + "::1220" + fingerprint, true},
		{"empty hint", "::1220" + fingerprint, true},
		{"hint that ends with a colon", "a:::1220" + fingerprint, true},
		{"hint with a dot", "a.b::1220" + fingerprint, true},
		{"hint with a slash", "a/b::1220" + fingerprint, true},
		{"hint with a number sign", "a#b::1220" + fingerprint, true},
		{"hint with an at sign", "a@b::1220" + fingerprint, true},
		{"hint with a tab", "a\tb::1220" + fingerprint, true},
		{"hint with a line feed", "a\nb::1220" + fingerprint, true},
		{"hint with a non-ASCII letter", "é::1220" + fingerprint, true},
		{"no separator", "alice1220" + fingerprint, true},
		{"two separators", "alice::bob::1220" + fingerprint, true},
		{"fingerprint without the 1220 prefix", "alice::" + fingerprint + "abcd", true},
		{"fingerprint prefix 1221", "alice::1221" + fingerprint, true},
		{"fingerprint of 63 hex characters", "alice::1220" + fingerprint[:63], true},
		{"fingerprint of 65 hex characters", "alice::1220" + fingerprint + "a", true},
		{"uppercase fingerprint", "alice::1220" + strings.ToUpper(fingerprint), true},
		{"non-hex fingerprint", "alice::1220" + strings.Repeat("g", 64), true},
		{"trailing line feed", validParty + "\n", true},
		{"trailing space", validParty + " ", true},
		{"empty", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			party, err := parseRequestPartyID(tc.raw, "recipient")
			if !tc.wantErr {
				require.NoError(t, err)
				assert.Equal(t, partyID(tc.raw), party)
				return
			}
			require.Error(t, err)
			assert.Empty(t, party)
			var flowErr *flowError
			require.True(t, errors.As(err, &flowErr))
			assert.Equal(t, http.StatusBadRequest, flowErr.status)
			assert.Contains(t, err.Error(), "recipient")
			assert.NotContains(t, err.Error(), "\n")
		})
	}
}

func TestPartyIDLengthBound(t *testing.T) {
	longestParty := strings.Repeat("h", maxPartyHintBytes) + "::1220" + strings.Repeat("ab", 32)
	assert.Equal(t, maxPartyIDBytes, len(longestParty))
}
