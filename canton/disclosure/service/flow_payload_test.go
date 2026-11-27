package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawFlowEntries decodes the disclosures of a flow response as raw JSON objects, so a test sees
// which fields are present and their exact bytes.
func rawFlowEntries(t *testing.T, body []byte) []map[string]json.RawMessage {
	t.Helper()
	var response struct {
		Disclosures []map[string]json.RawMessage `json:"disclosures"`
	}
	require.NoError(t, json.Unmarshal(body, &response), string(body))
	return response.Disclosures
}

// verbatimManagerArg is an NttManager createArgument with fields that a decode and re-encode would
// change: unread fields, a Map as [key, value] pairs, a decimal with a trailing zero, HTML
// characters, and non-ASCII text.
func verbatimManagerArg(b baseFixtures) string {
	manager := nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, b.managerAddr, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid)
	return strings.TrimSuffix(manager, "}") +
		`,"tokenDecimals":"6","peers":[["2",{"managerAddress":{"tag":"NttManagerAddress","value":"` + strings.Repeat("0a", 32) + `"},"transceiverAddress":{"tag":"EmitterAddress","value":"` + strings.Repeat("0b", 32) + `"},"decimals":"18"}]],` +
		`"note":"<a&b> é€😀  ","limit":"1.50","nested":{"empty":[],"none":null}}`
}

func TestFlowEntriesCarryCreateArgumentVerbatim(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	potCid := "pot-cid-1"
	managerArg := verbatimManagerArg(b)
	require.True(t, json.Valid([]byte(managerArg)), managerArg)
	coreStateArg := `{"guardianGovernance":"` + b.gg + `","messageFee":"0.0000000000","guardianSetIndex":"0"}`
	preapprovalPayload := preapprovalArg(testRecipient, b.instrumentAdmin, b.instrumentID)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-1", "bWdy", "sync1", managerArg)),
		tailCoreState:    jsonArray(acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-1", "Y3M=", "sync1", coreStateArg)),
		tailReplayNode:   jsonArray(b.replayNodeJSON("")),
		tailLockedLedger: jsonArray(b.ledgerJSON(&potCid)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
		// The pot is read by contract id. This event has no createArgument.
		tailCoin:                jsonArray(acsContractJSON("pkg:"+tailCoin, potCid, "Y29pbg==", "sync1")),
		tailTransferPreapproval: jsonArray(acsContractJSONWithArg("pkg:"+tailTransferPreapproval, "pre-cid-1", "cHJl", "sync1", preapprovalPayload)),
	}
	srv := newFlowTestServer(t, fixtures)

	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	wantPayloadByRole := map[string]string{
		roleNttManager:                   managerArg,
		roleCoreState:                    coreStateArg,
		roleCoveringReplayNode:           replayNodeArg(b.gg, b.namespace, ""),
		roleLockedLedger:                 ledgerArg(b.managerAddr, &potCid),
		roleCommittedTransferFactory:     coinFactoryArg(b.gg),
		roleRecipientTransferPreapproval: preapprovalPayload,
	}
	entries := rawFlowEntries(t, rr.Body.Bytes())
	require.Len(t, entries, 7)
	for _, entry := range entries {
		var role string
		require.NoError(t, json.Unmarshal(entry["role"], &role))
		if role == roleCustodyPotHolding {
			_, present := entry["createArgument"]
			assert.False(t, present, "an entry with no upstream createArgument omits the field: %v", entry)
			continue
		}
		want, known := wantPayloadByRole[role]
		require.True(t, known, "unexpected role %s", role)
		assert.Equal(t, want, string(entry["createArgument"]), "role %s", role)
	}
	assert.Contains(t, rr.Body.String(), `"note":"<a&b> é€😀  "`, "HTML characters and non-ASCII text keep their bytes")

	// The typed decode sees the same payload.
	decoded := decodeFlowResponse(t, rr)
	assert.JSONEq(t, managerArg, string(decoded.Disclosures[0].CreateArgument))
}

func TestFlowEntryCreateArgumentAbsentOrNull(t *testing.T) {
	entry := acsEntry{TemplateID: "pkg:" + tailCoin, ContractID: "c1", CreatedEventBlob: "YmxvYg==", SynchronizerID: "sync1"}
	for _, createArgument := range []json.RawMessage{nil, json.RawMessage(""), json.RawMessage("null"), json.RawMessage(" null ")} {
		entry.CreateArgument = createArgument
		disclosure := discloseAs(roleCustodyPotHolding, entry)
		assert.Nil(t, disclosure.CreateArgument)
		encoded, err := json.Marshal(disclosure)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "createArgument")
	}
	entry.CreateArgument = json.RawMessage(`{"owner":"alice"}`)
	assert.Equal(t, `{"owner":"alice"}`, string(discloseAs(roleCustodyPotHolding, entry).CreateArgument))
}

func TestFlowResponseInvariantsRejectANonObjectCreateArgument(t *testing.T) {
	cases := []struct {
		name           string
		createArgument json.RawMessage
		wantErr        bool
	}{
		{"absent", nil, false},
		{"empty record", json.RawMessage(`{}`), false},
		{"record", json.RawMessage(`{"a":[1,"2"]}`), false},
		{"array", json.RawMessage(`[1]`), true},
		{"string", json.RawMessage(`"x"`), true},
		{"truncated", json.RawMessage(`{"a":`), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disclosure := validDisclosure("a", "c1", "sync1")
			disclosure.CreateArgument = tc.createArgument
			err := checkFlowResponseInvariants(&flowResponse{Flow: "consolidate", Disclosures: []flowDisclosureEntry{disclosure}, Missing: []flowMissingEntry{}})
			if tc.wantErr {
				assert.ErrorContains(t, err, "createArgument")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFlowResponsePayloadBound pins the createArgument budget: 4 MiB of payload bytes, apart from
// the 4 MiB of blob and id bytes. Past it, the flow is a 502 and no set is sent.
func TestFlowResponsePayloadBound(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	managerWithPadding := func(padding string) string {
		manager := nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, b.managerAddr, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid)
		return strings.TrimSuffix(manager, "}") + `,"pad":"` + padding + `"}`
	}
	otherPayloadBytes := len(managerWithPadding("")) + len(ledgerArg(b.managerAddr, nil)) + len(coinFactoryArg(b.gg))
	paddingForTotal := func(totalBytes int) string { return strings.Repeat("A", totalBytes-otherPayloadBytes) }
	cases := []struct {
		name       string
		padding    string
		wantStatus int
	}{
		{"4 MiB minus 1 KiB", paddingForTotal(maxFlowResponsePayloadBytes - 1024), http.StatusOK},
		{"exactly 4 MiB", paddingForTotal(maxFlowResponsePayloadBytes), http.StatusOK},
		{"4 MiB plus 1", paddingForTotal(maxFlowResponsePayloadBytes + 1), http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := flowFixtures{
				tailNttManager:   jsonArray(acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-1", "bWdy", "sync1", managerWithPadding(tc.padding))),
				tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
				tailCoinFactory:  jsonArray(b.factoryJSON()),
			}
			srv := newFlowTestServer(t, fixtures)
			logged := captureServerLog(srv)
			recorder := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
			if tc.wantStatus != http.StatusOK {
				requirePublicErrorWithLoggedDetail(t, recorder, logged, tc.wantStatus, "createArgument payloads")
				assert.NotContains(t, recorder.Body.String(), "disclosures")
				return
			}
			require.Equal(t, http.StatusOK, recorder.Code)
			response := decodeFlowResponse(t, recorder)
			assert.Len(t, response.Disclosures, 3)
			assert.Equal(t, len(tc.padding)+otherPayloadBytes, countFlowResponsePayloadBytes(&flowResponse{Disclosures: response.Disclosures}))
		})
	}
}

// TestActiveContractsRequestIsVerbose pins verbose:true on the active-contracts read. Without it the
// participant returns createArgument with no field labels. contract_by_id_test.go pins the same
// for the read by contract id.
func TestActiveContractsRequestIsVerbose(t *testing.T) {
	for _, includeCreatedEventBlob := range []bool{true, false} {
		body, err := marshalActiveContractsRequest(testQuery("#ntt:Wormhole.Ntt.Manager:NttManager", 42), includeCreatedEventBlob)
		require.NoError(t, err)
		var request activeContractsRequest
		require.NoError(t, json.Unmarshal(body, &request))
		assert.True(t, request.EventFormat.Verbose)
	}
}
