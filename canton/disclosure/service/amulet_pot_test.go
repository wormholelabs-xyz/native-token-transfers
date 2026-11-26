package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmuletServedOnlyAsCustodyPot(t *testing.T) {
	lockUnlock := newBaseFixtures(tokenConfigLockUnlock)
	potContractID := "amulet-pot-cid"
	amuletPot := acsContractJSONWithArg("pkg:"+tailAmulet, potContractID, "YW11bGV0", "sync1", "{}")
	strangerAmulets := strangerEntries(tailAmulet, 1001, func(int) string { return "{}" })
	fixturesWithPot := func(potContractID *string, amulets ...string) flowFixtures {
		return flowFixtures{
			tailNttManager:          jsonArray(lockUnlock.managerJSON()),
			tailEmitter:             jsonArray(lockUnlock.emitterJSON()),
			tailCoreState:           jsonArray(lockUnlock.coreStateJSON()),
			tailLockedLedger:        jsonArray(lockUnlock.ledgerJSON(potContractID)),
			tailCoinFactory:         jsonArray(lockUnlock.factoryJSON()),
			tailTransferPreapproval: jsonArray(lockUnlock.transferPreapprovalJSON(lockUnlock.reserveCustodian())),
			tailAmulet:              jsonArray(amulets...),
		}
	}
	cases := []struct {
		name          string
		fixtures      flowFixtures
		wantStatus    int
		wantPotServed bool
	}{
		{"pot is an Amulet among 1001 other Amulets", fixturesWithPot(&potContractID, append(append([]string{}, strangerAmulets...), amuletPot)...), http.StatusOK, true},
		{"pot is an Amulet", fixturesWithPot(&potContractID, amuletPot), http.StatusOK, true},
		{"no pot, 1001 visible Amulets", fixturesWithPot(nil, strangerAmulets...), http.StatusOK, false},
		{"pot is an Amulet of another package", fixturesWithPot(&potContractID, strings.Replace(amuletPot, `"packageName":"splice-amulet"`, `"packageName":"splice-amulet-evil"`, 1)), http.StatusBadGateway, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, queriedTemplates := newRecordingFlowTestServer(t, tc.fixtures)
			srv.logger, _ = newCapturedLogger()
			recorder := requestFlow(t, srv, "transfer", "manager="+lockUnlock.managerAddr)
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.NotContains(t, queriedTemplates.tails(), tailAmulet, "the service never scans the Amulet ACS")
			if tc.wantStatus != http.StatusOK {
				return
			}
			response := decodeFlowResponse(t, recorder)
			var servedAmulets []flowDisclosureEntry
			for _, disclosure := range response.Disclosures {
				if templateTail(disclosure.TemplateID) == tailAmulet {
					servedAmulets = append(servedAmulets, disclosure)
				}
			}
			if !tc.wantPotServed {
				assert.Empty(t, servedAmulets)
				return
			}
			require.Len(t, servedAmulets, 1)
			assert.Equal(t, roleCustodyPotHolding, servedAmulets[0].Role)
			assert.Equal(t, potContractID, servedAmulets[0].ContractID)
		})
	}

	t.Run("raw template route is gone", func(t *testing.T) {
		srv, queriedTemplates := newRecordingFlowTestServer(t, fixturesWithPot(&potContractID, amuletPot))
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/disclosures?template=Splice.Amulet:Amulet", nil))
		assert.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Empty(t, queriedTemplates.tails())
	})
}
