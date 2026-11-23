package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func validDisclosure(role, contractID, synchronizerID string) flowDisclosureEntry {
	return flowDisclosureEntry{Role: role, TemplateID: "pkg:Module:Entity", ContractID: contractID, CreatedEventBlob: "YmxvYg==", SynchronizerID: synchronizerID}
}

func TestCheckOneSynchronizer(t *testing.T) {
	cases := []struct {
		name        string
		disclosures []flowDisclosureEntry
		wantErr     bool
	}{
		{"all sync1", []flowDisclosureEntry{validDisclosure("a", "c1", "sync1"), validDisclosure("b", "c2", "sync1")}, false},
		{"single entry", []flowDisclosureEntry{validDisclosure("a", "c1", "sync1")}, false},
		{"one empty", []flowDisclosureEntry{validDisclosure("a", "c1", "sync1"), validDisclosure("b", "c2", "")}, true},
		{"only entry empty", []flowDisclosureEntry{validDisclosure("a", "c1", "")}, true},
		{"sync1 and sync2", []flowDisclosureEntry{validDisclosure("a", "c1", "sync1"), validDisclosure("b", "c2", "sync2")}, true},
		{"no entries", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkOneSynchronizer(tc.disclosures)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestCheckFlowResponseInvariants(t *testing.T) {
	valid := func() *flowResponse {
		return &flowResponse{
			Flow:        "transfer",
			Disclosures: []flowDisclosureEntry{validDisclosure("a", "c1", "sync1"), validDisclosure("b", "c2", "sync1")},
			Missing:     []flowMissingEntry{missingEntry("c")},
		}
	}
	cases := []struct {
		name    string
		mutate  func(*flowResponse)
		wantErr string
	}{
		{"valid", func(*flowResponse) {}, ""},
		{"valid with no missing entries", func(response *flowResponse) { response.Missing = []flowMissingEntry{} }, ""},
		{"no disclosures", func(response *flowResponse) { response.Disclosures = nil }, "no disclosures"},
		{"contract id twice", func(response *flowResponse) { response.Disclosures[1].ContractID = "c1" }, "contract c1 twice"},
		{"role twice", func(response *flowResponse) { response.Disclosures[1].Role = "a" }, "role a twice"},
		{"role in disclosures and missing", func(response *flowResponse) { response.Missing[0].Role = "a" }, "both disclosed and missing"},
		{"missing role twice", func(response *flowResponse) {
			response.Missing = append(response.Missing, missingEntry("c"))
		}, "missing role c twice"},
		{"empty contract id", func(response *flowResponse) { response.Disclosures[0].ContractID = "" }, "no contract id"},
		{"empty template id", func(response *flowResponse) { response.Disclosures[0].TemplateID = "" }, "no template id"},
		{"empty blob", func(response *flowResponse) { response.Disclosures[0].CreatedEventBlob = "" }, "no createdEventBlob"},
		{"empty flow name", func(response *flowResponse) { response.Flow = "" }, "no flow name"},
		{"nil missing list", func(response *flowResponse) { response.Missing = nil }, "missing list is nil"},
		{"valid register", makeValidRegisterResponse, ""},
		{"register with a missing entry", func(response *flowResponse) {
			makeValidRegisterResponse(response)
			response.Missing = []flowMissingEntry{missingEntry(roleCoveringClaimedRootsNode)}
		}, "register response has 1 missing entries"},
		{"register without claimNamespace", func(response *flowResponse) {
			makeValidRegisterResponse(response)
			response.ClaimNamespace = ""
		}, "register response has no claimNamespace"},
		{"register without managerId", func(response *flowResponse) {
			makeValidRegisterResponse(response)
			response.ManagerID = nil
		}, "register response has no managerId"},
		{"transfer with claimNamespace", func(response *flowResponse) { response.ClaimNamespace = "ntt:ab" }, "transfer response has a claimNamespace"},
		{"transfer with managerId", func(response *flowResponse) {
			managerID := json.Number("0")
			response.ManagerID = &managerID
		}, "transfer response has a managerId"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := valid()
			tc.mutate(response)
			err := checkFlowResponseInvariants(response)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func makeValidRegisterResponse(response *flowResponse) {
	managerID := json.Number("0")
	response.Flow = "register"
	response.Missing = []flowMissingEntry{}
	response.ClaimNamespace = "ntt:ab"
	response.ManagerID = &managerID
}

func TestHandleFlows_TwoSynchronizers_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(strings.Replace(b.managerJSON(), `"synchronizerId":"sync1"`, `"synchronizerId":"sync2"`, 1)),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	logged := captureServerLog(srv)
	recorder := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	requirePublicErrorWithLoggedDetail(t, recorder, logged, http.StatusInternalServerError, "sync2")
}
