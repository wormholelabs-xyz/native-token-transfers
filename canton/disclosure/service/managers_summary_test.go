package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testPeerManagerAddress     = strings.Repeat("0a", 32)
	testPeerTransceiverAddress = strings.Repeat("0b", 32)
)

// peerJSON is one Daml Peer record as the JSON Ledger API encodes it.
func peerJSON(managerAddress, transceiverAddress, decimals string) string {
	return `{"managerAddress":{"tag":"NttManagerAddress","value":"` + managerAddress + `"},"transceiverAddress":{"tag":"EmitterAddress","value":"` + transceiverAddress + `"},"decimals":` + decimals + `}`
}

// managerArgWithSummaryFields adds tokenDecimals and peers to an NttManager createArgument.
func managerArgWithSummaryFields(managerArg, tokenDecimals, peers string) string {
	return strings.TrimSuffix(managerArg, "}") + `,"tokenDecimals":` + tokenDecimals + `,"peers":` + peers + `}`
}

func requestManagers(t *testing.T, srv *server) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/managers", nil))
	return rr
}

func TestHandleManagers_SummaryFields(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	baseArg := func(managerAddress string) string {
		return nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, managerAddress, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid)
	}
	// The JSON Ledger API encoding of a Daml Map: [key, value] pairs, the Int key as a string.
	pairsPeers := `[["2",` + peerJSON(testPeerManagerAddress, testPeerTransceiverAddress, `"18"`) + `],["10002",` + peerJSON(strings.Repeat("0c", 32), strings.Repeat("0d", 32), `"6"`) + `]]`
	// The object form, with a numeric decimals value.
	objectPeers := `{"4":` + peerJSON(testPeerManagerAddress, testPeerTransceiverAddress, `8`) + `}`
	managers := []string{
		acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-pairs", "bWdy", "sync1", managerArgWithSummaryFields(baseArg(strings.Repeat("aa", 32)), `"6"`, pairsPeers)),
		acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-object", "bWdy", "sync1", managerArgWithSummaryFields(baseArg(strings.Repeat("bb", 32)), `8`, objectPeers)),
		acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-no-peers", "bWdy", "sync1", managerArgWithSummaryFields(baseArg(strings.Repeat("cc", 32)), `"10"`, `[]`)),
		// No tokenDecimals and no peers field: the summary omits both.
		acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-older", "bWdy", "sync1", baseArg(strings.Repeat("dd", 32))),
	}
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(managers...)})
	rr := requestManagers(t, srv)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var got struct {
		Managers []map[string]json.RawMessage `json:"managers"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Len(t, got.Managers, 4)

	wantPeers := []string{
		`{"10002":{"managerAddress":"` + strings.Repeat("0c", 32) + `","transceiverAddress":"` + strings.Repeat("0d", 32) + `","decimals":6},"2":{"managerAddress":"` + testPeerManagerAddress + `","transceiverAddress":"` + testPeerTransceiverAddress + `","decimals":18}}`,
		`{"4":{"managerAddress":"` + testPeerManagerAddress + `","transceiverAddress":"` + testPeerTransceiverAddress + `","decimals":8}}`,
		`{}`,
	}
	wantDecimals := []string{"6", "8", "10"}
	for index, manager := range got.Managers {
		assert.JSONEq(t, `"`+b.admin+`"`, string(manager["admin"]), "manager %d admin", index)
		if index == 3 {
			_, hasPeers := manager["peers"]
			_, hasDecimals := manager["tokenDecimals"]
			assert.False(t, hasPeers, "a payload with no peers field gives no peers")
			assert.False(t, hasDecimals, "a payload with no tokenDecimals field gives no tokenDecimals")
			continue
		}
		assert.JSONEq(t, wantDecimals[index], string(manager["tokenDecimals"]), "manager %d tokenDecimals", index)
		assert.JSONEq(t, wantPeers[index], string(manager["peers"]), "manager %d peers", index)
	}
	// The existing fields are unchanged.
	assert.JSONEq(t, `"`+strings.Repeat("aa", 32)+`"`, string(got.Managers[0]["managerAddress"]))
	assert.JSONEq(t, `"mgr-pairs"`, string(got.Managers[0]["contractId"]))
	assert.JSONEq(t, `"`+b.reserveCustodian()+`"`, string(got.Managers[0]["reserveCustodian"]))
}

// TestHandleManagers_BadPeersFailOnlyTheSummary pins that the flows do not decode peers: a peers
// value this service cannot read fails /v1/managers with a 500 and leaves the flows working.
func TestHandleManagers_BadPeersFailOnlyTheSummary(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	badPeers := `[["2",` + peerJSON(strings.ToUpper(testPeerManagerAddress[:2])+testPeerManagerAddress[2:], testPeerTransceiverAddress, `"18"`) + `]]`
	managerArg := managerArgWithSummaryFields(
		nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, b.managerAddr, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid),
		`"6"`, badPeers)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-1", "bWdy", "sync1", managerArg)),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	logged := captureServerLog(srv)
	requirePublicErrorWithLoggedDetail(t, requestManagers(t, srv), logged, http.StatusInternalServerError, "decode NttManager peers")

	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestNttPeerMapUnmarshal(t *testing.T) {
	peer := peerJSON(testPeerManagerAddress, testPeerTransceiverAddress, `"18"`)
	wantPeer := managerPeer{ManagerAddress: testPeerManagerAddress, TransceiverAddress: testPeerTransceiverAddress, Decimals: "18"}
	cases := []struct {
		name    string
		input   string
		want    nttPeerMap
		wantErr string
	}{
		{"pairs with string keys", `[["2",` + peer + `]]`, nttPeerMap{"2": wantPeer}, ""},
		{"pairs with number keys", `[[2,` + peer + `]]`, nttPeerMap{"2": wantPeer}, ""},
		{"empty pairs", `[]`, nttPeerMap{}, ""},
		{"object", `{"2":` + peer + `}`, nttPeerMap{"2": wantPeer}, ""},
		{"empty object", `{}`, nttPeerMap{}, ""},
		{"key with a leading zero", `[["02",` + peer + `]]`, nil, "chain id"},
		{"duplicate chain id", `[["2",` + peer + `],[2,` + peer + `]]`, nil, "appears twice"},
		{"object keys equal after parsing", `{"0":` + peer + `,"-0":` + peer + `}`, nil, "appears twice"},
		{"pair of three", `[["2",` + peer + `,1]]`, nil, "not a [chainId, peer] pair"},
		{"pair of one", `[["2"]]`, nil, "not a [chainId, peer] pair"},
		{"key that is not an integer", `[["two",` + peer + `]]`, nil, "chain id"},
		{"fractional key", `[["2.5",` + peer + `]]`, nil, "not an integer"},
		{"string", `"peers"`, nil, "not an array"},
		{"null element", `[null]`, nil, "not a [chainId, peer] pair"},
		{"wrong manager tag", `[["2",` + strings.Replace(peer, "NttManagerAddress", "EmitterAddress", 1) + `]]`, nil, "NttManagerAddress"},
		{"wrong transceiver tag", `[["2",` + strings.Replace(peer, `"tag":"EmitterAddress"`, `"tag":"UniversalAddress"`, 1) + `]]`, nil, "EmitterAddress"},
		{"short transceiver address", `[["2",` + peerJSON(testPeerManagerAddress, "0b", `"18"`) + `]]`, nil, "EmitterAddress"},
		{"no transceiver address", `[["2",{"managerAddress":{"tag":"NttManagerAddress","value":"` + testPeerManagerAddress + `"},"decimals":"18"}]]`, nil, "no transceiverAddress"},
		{"decimals not an integer", `[["2",` + peerJSON(testPeerManagerAddress, testPeerTransceiverAddress, `"1.5"`) + `]]`, nil, "decimals"},
		{"no decimals", `[["2",{"managerAddress":{"tag":"NttManagerAddress","value":"` + testPeerManagerAddress + `"},"transceiverAddress":{"tag":"EmitterAddress","value":"` + testPeerTransceiverAddress + `"}}]]`, nil, "decimals"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got nttPeerMap
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEmitterAddressValue(t *testing.T) {
	var address emitterAddressValue
	require.NoError(t, json.Unmarshal([]byte(`{"tag":"EmitterAddress","value":"`+testPeerTransceiverAddress+`"}`), &address))
	assert.Equal(t, testPeerTransceiverAddress, address.hex)
	for _, bad := range []string{
		`{"tag":"EmitterAddress","value":"` + strings.ToUpper(testPeerTransceiverAddress) + `"}`,
		`{"tag":"EmitterAddress","value":"` + testPeerTransceiverAddress + `00"}`,
		`{"tag":"NttManagerAddress","value":"` + testPeerTransceiverAddress + `"}`,
		`"` + testPeerTransceiverAddress + `"`,
	} {
		assert.Error(t, json.Unmarshal([]byte(bad), &address), bad)
	}
}
