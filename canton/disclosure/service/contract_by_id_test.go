package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eventsByContractIDJSON renders a JsGetEventsByContractIdResponse with a created event and,
// when archived is set, an archived event.
func eventsByContractIDJSON(templateID, contractID, blob, synchronizerID, createArgument string, archived bool) string {
	created := fmt.Sprintf(`"created":{"createdEvent":{"contractId":%q,"templateId":%q,"createdEventBlob":%q,"createArgument":%s},"synchronizerId":%q}`,
		contractID, templateID, blob, createArgument, synchronizerID)
	if !archived {
		return "{" + created + "}"
	}
	return "{" + created + fmt.Sprintf(`,"archived":{"archivedEvent":{"contractId":%q,"templateId":%q},"synchronizerId":%q}}`, contractID, templateID, synchronizerID)
}

func TestActiveContractByID(t *testing.T) {
	const potContractID = "pot-cid-1"
	cases := []struct {
		name          string
		status        int
		body          string
		wantFound     bool
		wantErr       error
		wantAnyErr    bool
		wantBlob      string
		wantSyncID    string
		wantRequestOK bool
	}{
		{"active contract", http.StatusOK, eventsByContractIDJSON("pkg:"+tailCoin, potContractID, "cG90", "sync1", "{}", false), true, nil, false, "cG90", "sync1", true},
		{"not visible is a 404", http.StatusNotFound, `{"code":"CONTRACT_EVENTS_NOT_FOUND"}`, false, nil, false, "", "", true},
		{"created and archived", http.StatusOK, eventsByContractIDJSON("pkg:"+tailCoin, potContractID, "cG90", "sync1", "{}", true), false, nil, false, "", "", true},
		{"no created event", http.StatusOK, `{}`, false, nil, false, "", "", true},
		{"other contract id", http.StatusOK, eventsByContractIDJSON("pkg:"+tailCoin, "other-cid", "cG90", "sync1", "{}", false), false, errUpstreamResponseMalformed, true, "", "", true},
		{"malformed body", http.StatusOK, `{"created":`, false, errUpstreamResponseMalformed, true, "", "", true},
		{"upstream 500", http.StatusInternalServerError, `internal`, false, nil, true, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotRequest eventsByContractIDRequest
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/events/events-by-contract-id", r.URL.Path)
				assert.Equal(t, http.MethodPost, r.Method)
				body, _ := io.ReadAll(r.Body)
				assert.NoError(t, json.Unmarshal(body, &gotRequest))
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(upstream.Close)

			entry, found, err := newHTTPACSClient(upstream.URL, "").ActiveContractByID(context.Background(), []partyID{"nttCustody::1"}, potContractID)
			assert.Equal(t, potContractID, gotRequest.ContractID)
			require.Contains(t, gotRequest.EventFormat.FiltersByParty, "nttCustody::1")
			assert.True(t, gotRequest.EventFormat.Verbose)
			if tc.wantAnyErr {
				require.Error(t, err)
				if tc.wantErr != nil {
					assert.ErrorIs(t, err, tc.wantErr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantFound, found)
			if tc.wantFound {
				assert.Equal(t, potContractID, entry.ContractID)
				assert.Equal(t, tc.wantBlob, entry.CreatedEventBlob)
				assert.Equal(t, tc.wantSyncID, entry.SynchronizerID)
			}
		})
	}
}

func TestFetchPotHoldingByContractID(t *testing.T) {
	lockUnlock := newBaseFixtures(tokenConfigLockUnlock)
	potContractID := "pot-cid-7"
	baseFixtures := func() flowFixtures {
		return flowFixtures{
			tailNttManager:          jsonArray(lockUnlock.managerJSON()),
			tailEmitter:             jsonArray(lockUnlock.emitterJSON()),
			tailCoreState:           jsonArray(lockUnlock.coreStateJSON()),
			tailLockedLedger:        jsonArray(lockUnlock.ledgerJSON(&potContractID)),
			tailCoinFactory:         jsonArray(lockUnlock.factoryJSON()),
			tailTransferPreapproval: jsonArray(lockUnlock.transferPreapprovalJSON(lockUnlock.reserveCustodian())),
		}
	}
	activePot := acsContractJSONWithArg("pkg:"+tailCoin, potContractID, "cG90", "sync1", "{}")
	strangerCoins := strangerEntries(tailCoin, 1001, func(int) string { return "{}" })

	cases := []struct {
		name        string
		mutate      func(flowFixtures)
		wantStatus  int
		wantServed  bool
		wantMissing bool
	}{
		{"active pot served", func(fixtures flowFixtures) { fixtures[tailCoin] = jsonArray(activePot) }, http.StatusOK, true, false},
		{"pot not visible is missing", func(flowFixtures) {}, http.StatusOK, false, true},
		{"created and archived pot is missing", func(fixtures flowFixtures) {
			fixtures[tailCoin] = jsonArray(activePot)
			fixtures[archivedFixtureKey(potContractID)] = ""
		}, http.StatusOK, false, true},
		{"pot contract id names a TransferPreapproval", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = jsonArray(
				lockUnlock.transferPreapprovalJSON(lockUnlock.reserveCustodian()),
				acsContractJSONWithArg("pkg:"+tailTransferPreapproval, potContractID, "cHJl", "sync1", preapprovalArg("x::1", "y::1", "z")))
		}, http.StatusBadGateway, false, false},
		{"1001 other Coin contracts and the pot", func(fixtures flowFixtures) {
			fixtures[tailCoin] = jsonArray(append(append([]string{}, strangerCoins...), activePot)...)
		}, http.StatusOK, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := baseFixtures()
			tc.mutate(fixtures)
			srv, queriedTemplates := newRecordingFlowTestServer(t, fixtures)
			recorder := requestFlow(t, srv, "transfer", "manager="+lockUnlock.managerAddr)
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.NotContains(t, queriedTemplates.tails(), tailCoin, "the pot is read by contract id, not by an ACS query")
			if tc.wantStatus != http.StatusOK {
				return
			}
			response := decodeFlowResponse(t, recorder)
			if tc.wantServed {
				assert.Contains(t, rolesOf(response.Disclosures), roleCustodyPotHolding)
				for _, disclosure := range response.Disclosures {
					assert.NotContains(t, disclosure.ContractID, "stranger")
				}
			}
			if tc.wantMissing {
				assert.Contains(t, missingRolesOf(response.Missing), roleCustodyPotHolding)
			}
		})
	}
}

func TestCommittedFactoryByContractID(t *testing.T) {
	lockUnlock := newBaseFixtures(tokenConfigLockUnlock)
	strangerFactories := strangerEntries(tailCoinFactory, 1001, func(int) string { return coinFactoryArg(lockUnlock.gg) })
	cases := []struct {
		name       string
		factories  string
		wantStatus int
	}{
		{"committed factory among 1001 other factories", jsonArray(append(append([]string{}, strangerFactories...), lockUnlock.factoryJSON())...), http.StatusOK},
		{"committed factory not visible", "[]", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := flowFixtures{
				tailNttManager:   jsonArray(lockUnlock.managerJSON()),
				tailLockedLedger: jsonArray(lockUnlock.ledgerJSON(nil)),
				tailCoinFactory:  tc.factories,
			}
			srv, queriedTemplates := newRecordingFlowTestServer(t, fixtures)
			recorder := requestFlow(t, srv, "consolidate", "manager="+lockUnlock.managerAddr)
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.NotContains(t, queriedTemplates.tails(), tailCoinFactory)
		})
	}
}
