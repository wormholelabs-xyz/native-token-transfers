package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Expected values from Daml Wormhole.Ntt.Manager.nttNamespaceFromText (ntt 0.6.2). All four rows
// are pinned in Test.TestNtt.testNamespaceVector.
func TestNttNamespaceVectors(t *testing.T) {
	const (
		vectorInstrumentAdmin = "vector-instrument-admin::1220deadbeef"
		vectorAdmin           = "vector-admin::1220cafebabe"
		issuerParty           = "Issuer::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		adminParty            = "Admin::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)
	cases := []struct {
		name              string
		instrumentAdmin   partyID
		instrumentID      string
		admin             partyID
		managerID         json.Number
		expectedNamespace string
	}{
		{"managerId 0", vectorInstrumentAdmin, "VECTOR", vectorAdmin, "0", "ntt:ab792801446bfcbef62e6559bd83cae570df69488931273fd99820325a7b8af0"},
		{"managerId 1", vectorInstrumentAdmin, "VECTOR", vectorAdmin, "1", "ntt:a44d5b55fdec9b03b477c21b889233da8c500f8f04cf614dcbeeaf93192d4731"},
		{"managerId 2^63-1", vectorInstrumentAdmin, "VECTOR", vectorAdmin, "9223372036854775807", "ntt:937f5dafd928db0ec0c7f10242629b30f8780dc6890444093111a125dfcb9f80"},
		{"multi-byte instrument id, managerId 256", issuerParty, "é€😀", adminParty, "256", "ntt:0c47bf36a16b89849a9e423a11675fe9ca416e0ffb7c6584abb25807759ff3cd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			instrumentID, err := parseInstrumentIDText(tc.instrumentID)
			require.NoError(t, err)
			managerID, err := parseNttManagerID(tc.managerID, "NttGovernance nextId")
			require.NoError(t, err)
			namespace := nttNamespace(tc.instrumentAdmin, instrumentID, tc.admin, managerID)
			assert.Equal(t, tc.expectedNamespace, namespace.replayNamespaceText)
			reparsed, err := parseReplayNamespace(namespace.replayNamespaceText)
			require.NoError(t, err)
			assert.Equal(t, namespace, reparsed)
		})
	}
}

func TestNttNamespaceSeparatesInputs(t *testing.T) {
	instrumentID, err := parseInstrumentIDText("VECTOR")
	require.NoError(t, err)
	managerID, err := parseNttManagerID("0", "NttGovernance nextId")
	require.NoError(t, err)
	baseline := nttNamespace("a::1220", instrumentID, "b::1220", managerID)
	swappedParties := nttNamespace("b::1220", instrumentID, "a::1220", managerID)
	assert.NotEqual(t, baseline, swappedParties)
}

func TestParseInstrumentIDText(t *testing.T) {
	cases := []struct {
		name          string
		rawID         string
		expectedError string
	}{
		{"one byte", "x", ""},
		{"token standard style", "wormhole-ntt:instrument-1", ""},
		{"multi-byte characters", "é€😀", ""},
		{"exactly the byte limit", strings.Repeat("i", instrumentIDMaxBytes), ""},
		{"empty", "", "instrument-id has 0 bytes, expected 1 to 512"},
		{"one byte over the limit", strings.Repeat("i", instrumentIDMaxBytes+1), "instrument-id has 513 bytes, expected 1 to 512"},
		{"multi-byte characters over the limit", strings.Repeat("é", instrumentIDMaxBytes/2+1), "instrument-id has 514 bytes, expected 1 to 512"},
		{"invalid UTF-8", "id\xff", "instrument-id is not valid UTF-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			instrumentID, err := parseInstrumentIDText(tc.rawID)
			if tc.expectedError == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.rawID, instrumentID.instrumentIDUTF8Text)
				return
			}
			requireFlowErrorStatus(t, err, http.StatusBadRequest)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestParseNttManagerID(t *testing.T) {
	cases := []struct {
		name             string
		rawNextID        json.Number
		expectedDecimal  string
		expectedErrorMsg string
	}{
		{"zero", "0", "0", ""},
		{"one", "1", "1", ""},
		{"2^63-1", "9223372036854775807", "9223372036854775807", ""},
		{"2^63", "9223372036854775808", "", "not a decimal integer from 0 to 9223372036854775807"},
		{"negative", "-1", "", "not a decimal integer from 0 to 9223372036854775807"},
		{"leading zero", "01", "", "not a decimal integer from 0 to 9223372036854775807"},
		{"plus sign", "+1", "", "not a decimal integer from 0 to 9223372036854775807"},
		{"decimal point", "1.0", "", "not a decimal integer from 0 to 9223372036854775807"},
		{"exponent", "1e3", "", "not a decimal integer from 0 to 9223372036854775807"},
		{"empty", "", "", "not a decimal integer from 0 to 9223372036854775807"},
		{"text", "abc", "", "not a decimal integer from 0 to 9223372036854775807"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, fieldLabel := range []string{"NttGovernance nextId", "NttManager managerId"} {
				managerID, err := parseNttManagerID(tc.rawNextID, fieldLabel)
				if tc.expectedErrorMsg == "" {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedDecimal, managerID.String())
					continue
				}
				requireFlowErrorStatus(t, err, http.StatusInternalServerError)
				assert.Contains(t, err.Error(), fieldLabel+" ")
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			}
		})
	}
}

func TestNttManagerIDMarshalsAsDecimalString(t *testing.T) {
	cases := []struct {
		name                 string
		rawManagerID         json.Number
		wantManagerIDDecimal string
	}{
		{"zero", "0", "0"},
		{"2^63-1", "9223372036854775807", "9223372036854775807"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			managerID, err := parseNttManagerID(tc.rawManagerID, "NttGovernance nextId")
			require.NoError(t, err)
			registerJSON, err := json.Marshal(flowResponse{
				Flow:        "register",
				Disclosures: []flowDisclosureEntry{},
				Missing:     []flowMissingEntry{},
				ManagerID:   &managerID,
			})
			require.NoError(t, err)
			assert.Equal(t,
				`{"flow":"register","disclosures":[],"missing":[],"managerId":"`+tc.wantManagerIDDecimal+`"}`,
				string(registerJSON))
			managerJSON, err := json.Marshal(managerEntry{ManagerID: managerID})
			require.NoError(t, err)
			assert.Equal(t,
				`{"managerAddress":"","contractId":"","chainId":0,"managerId":"`+tc.wantManagerIDDecimal+
					`","tokenConfig":"","instrumentId":{"admin":"","id":""},"reserveCustodian":"","paused":false,"admin":""}`,
				string(managerJSON))
		})
	}
}

func TestNttManagerIDMarshalPanicsAbove2Pow63Minus1(t *testing.T) {
	cases := []struct {
		name     string
		idNumber uint64
	}{
		{"2^63", 1 << 63},
		{"2^64-1", 1<<64 - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			managerID := nttManagerID{idNumber: tc.idNumber}
			assert.Panics(t, func() { _, _ = managerID.MarshalText() })
			assert.Panics(t, func() { _, _ = json.Marshal(managerEntry{ManagerID: managerID}) })
		})
	}
}
