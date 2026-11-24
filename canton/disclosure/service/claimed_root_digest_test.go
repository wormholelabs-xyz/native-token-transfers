package main

import (
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// patternedBytes returns byteCount bytes of (index*multiplier + offset) mod 256.
func patternedBytes(byteCount, multiplier, offset int) []byte {
	pattern := make([]byte, byteCount)
	for index := range pattern {
		pattern[index] = byte((index*multiplier + offset) % 256)
	}
	return pattern
}

// Expected values computed with `cast keccak`. The lengths cross the 136-byte rate.
func TestKeccak256DigestVectors(t *testing.T) {
	cases := []struct {
		name           string
		input          []byte
		expectedDigest string
	}{
		{"empty", nil, "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
		{"abc", []byte("abc"), "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"},
		{"rate minus one", patternedBytes(135, 1, 0), "cbdfd9dee5faad3818d6b06f95a219fd290b0e1706f6a82e5a595b9ce9faca62"},
		{"rate", patternedBytes(136, 1, 0), "7ce759f1ab7f9ce437719970c26b0a66ff11fe3e38e17df89cf5d29c7d7f807e"},
		{"rate plus one", patternedBytes(137, 7, 0), "a595973359b39ba1fec5cf8f40710c5a213e76281dd4f174f1ea83106ee7759b"},
		{"two blocks plus", patternedBytes(300, 13, 5), "63a7d381328d6b94142df0280facdac854b61eef4a17e80d0054873547121f2c"},
		{"two rates minus one", patternedBytes(271, 3, 1), "a0a8191ad31bb1b46380c071c80d0e96b79c8def412ab0222a0ffa56faacf270"},
		{"two rates", patternedBytes(272, 5, 2), "9e542bb28c090926d47131d7eba9fc1e9c166206d61302b1b7a7b518750e06ce"},
		{"two rates plus one", patternedBytes(273, 11, 3), "327f943497cdbc00972eb0b5e996b07b276401d48f688be564f63d34de752b80"},
		{"many blocks", patternedBytes(1000, 17, 9), "f98fab6131aac28811eed906c9537ca085cafc5191d06a48a205bc44d459d6c9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			digest := keccak256Digest(tc.input)
			assert.Equal(t, tc.expectedDigest, hex.EncodeToString(digest[:]))
		})
	}
}

// The same vectors as Test.TestNttVectors.claimedRootDigestVectors and the core golden value.
func TestClaimedRootDigestVectors(t *testing.T) {
	cases := []struct {
		consumer       partyID
		namespace      string
		expectedDigest string
	}{
		{"GoldenConsumer", "golden-namespace", "58d4dc29f0852dca5f0f626ac31c93ace1028ca0a41ba3ef972c0d681a841b4b"},
		{
			"GuardianGovernance::1220d8f2a6b7c1e4f00112233445566778899aabbccddeeff00112233445566778899",
			"ntt:6e0a3c51f1c0b3cb5f0f5f4c7a3c0a36b6f0ba0ef8b25f4ab2b2d1c6e0e8f4a1",
			"f763c5a8862e48c783303def423d496debd8fe1b4b4ac865eb106c95dee28d4c",
		},
	}
	for _, tc := range cases {
		t.Run(tc.namespace, func(t *testing.T) {
			namespace, err := parseReplayNamespace(tc.namespace)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedDigest, claimedRootDigest(tc.consumer, namespace).claimedRootDigestHexText)
		})
	}
}

func TestParseReplayNamespace(t *testing.T) {
	cases := []struct {
		name          string
		rawNamespace  string
		expectedError string
	}{
		{"one character", "a", ""},
		{"128 characters", strings.Repeat("a", 128), ""},
		{"128 two-byte characters", strings.Repeat("é", 128), ""},
		{"ntt namespace", "ntt:6e0a3c51f1c0b3cb5f0f5f4c7a3c0a36b6f0ba0ef8b25f4ab2b2d1c6e0e8f4a1", ""},
		{"empty", "", "namespace has 0 characters, expected 1 to 128"},
		{"129 characters", strings.Repeat("a", 129), "namespace has 129 characters, expected 1 to 128"},
		{"129 two-byte characters", strings.Repeat("é", 129), "namespace has 129 characters, expected 1 to 128"},
		{"invalid UTF-8", "ntt:\xff", "namespace is not valid UTF-8"},
		{"reserved for the core", claimedRootsNamespace, "namespace is reserved for the core"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			namespace, err := parseReplayNamespace(tc.rawNamespace)
			if tc.expectedError == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.rawNamespace, namespace.replayNamespaceText)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
			var flowErr *flowError
			require.ErrorAs(t, err, &flowErr)
			assert.Equal(t, http.StatusBadRequest, flowErr.status)
		})
	}
}
