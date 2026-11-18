package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowResponseSizeBound(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	otherFieldBytes := func(managerBlob string) int {
		response := &flowResponse{Flow: "consolidate", Disclosures: []flowDisclosureEntry{
			{TemplateID: "pkg:" + tailNttManager, ContractID: "mgr-cid-1", CreatedEventBlob: managerBlob},
			{TemplateID: "pkg:" + tailLockedLedger, ContractID: "ledger-cid-1", CreatedEventBlob: "bGVkZ2Vy"},
			{TemplateID: "pkg:" + tailCoinFactory, ContractID: b.factoryCid, CreatedEventBlob: "ZmFjdA=="},
		}}
		return countFlowResponseBytes(response) - len(managerBlob)
	}
	blobOfTotal := func(totalBytes int) string {
		return strings.Repeat("A", totalBytes-otherFieldBytes(""))
	}
	cases := []struct {
		name       string
		blob       string
		wantStatus int
	}{
		{"4 MiB minus 1 KiB", blobOfTotal(maxFlowResponseBytes - 1024), http.StatusOK},
		{"exactly 4 MiB", blobOfTotal(maxFlowResponseBytes), http.StatusOK},
		{"4 MiB plus 1", blobOfTotal(maxFlowResponseBytes + 1), http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-1", tc.blob, "sync1",
				nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, b.managerAddr, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid))
			fixtures := flowFixtures{
				tailNttManager:   jsonArray(manager),
				tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
				tailCoinFactory:  jsonArray(b.factoryJSON()),
			}
			srv := newFlowTestServer(t, fixtures)
			logged := captureServerLog(srv)
			recorder := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
			if tc.wantStatus != http.StatusOK {
				requirePublicErrorWithLoggedDetail(t, recorder, logged, tc.wantStatus, "exceeds")
				assert.NotContains(t, recorder.Body.String(), "disclosures")
				return
			}
			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Len(t, decodeFlowResponse(t, recorder).Disclosures, 3)
		})
	}
}

func TestHandleManagersStreamsSortedEntries(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	managerAt := func(contractID, managerAddress string) string {
		return acsContractJSONWithArg("pkg:"+tailNttManager, contractID, base64.StdEncoding.EncodeToString([]byte(contractID)), "sync1",
			nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, managerAddress, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid))
	}
	addresses := []string{strings.Repeat("cc", 32), strings.Repeat("aa", 32), strings.Repeat("bb", 32)}
	managers := make([]string, 0, len(addresses))
	for index, address := range addresses {
		managers = append(managers, managerAt(fmt.Sprintf("mgr-%d", index), address))
	}
	cases := []struct {
		name          string
		maxContracts  int
		wantStatus    int
		wantAddresses []string
	}{
		{"three managers, sorted", 3, http.StatusOK, []string{addresses[1], addresses[2], addresses[0]}},
		{"over the cap", 2, http.StatusBadGateway, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(managers...)})
			srv.maxContracts = tc.maxContracts
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/managers", nil))
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			if tc.wantStatus != http.StatusOK {
				assert.NotContains(t, recorder.Body.String(), "managers")
				return
			}
			assert.True(t, json.Valid(recorder.Body.Bytes()), recorder.Body.String())
			var got struct {
				Managers []managerEntry `json:"managers"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
			gotAddresses := make([]string, 0, len(got.Managers))
			for _, manager := range got.Managers {
				gotAddresses = append(gotAddresses, manager.ManagerAddress)
			}
			assert.Equal(t, tc.wantAddresses, gotAddresses)
		})
	}
	t.Run("no managers is an empty list", func(t *testing.T) {
		srv := newFlowTestServer(t, flowFixtures{})
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/managers", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"managers":[]}`, recorder.Body.String())
	})
}
