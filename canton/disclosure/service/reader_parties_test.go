package main

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReaderPartiesPerTemplate(t *testing.T) {
	lockUnlock := newBaseFixtures(tokenConfigLockUnlock)
	burnMint := newBaseFixtures(tokenConfigBurnMint)
	potContractID := "pot-cid-readers"
	lockUnlockFixtures := flowFixtures{
		tailNttManager:            jsonArray(lockUnlock.managerJSON()),
		tailCoreState:             jsonArray(lockUnlock.coreStateJSON()),
		tailReplayNode:            jsonArray(lockUnlock.replayNodeJSON("")),
		tailLockedLedger:          jsonArray(lockUnlock.ledgerJSON(&potContractID)),
		tailCoinFactory:           jsonArray(lockUnlock.factoryJSON()),
		tailCoin:                  jsonArray(acsContractJSONWithArg("pkg:"+tailCoin, potContractID, "cG90", "sync1", "{}")),
		tailEmitter:               jsonArray(lockUnlock.emitterJSON()),
		tailTransferPreapproval:   jsonArray(lockUnlock.transferPreapprovalJSON(lockUnlock.reserveCustodian()), lockUnlock.transferPreapprovalJSON(lockUnlock.nttCustody)),
		tailAdminTransferProposal: jsonArray(acsContractJSONWithArg("pkg:"+tailAdminTransferProposal, "prop-1", "cHJvcA==", "sync1", adminTransferProposalArg(lockUnlock.admin, lockUnlock.gg, lockUnlock.managerAddr))),
	}
	burnMintFixtures := flowFixtures{
		tailNttManager:         jsonArray(burnMint.managerJSON()),
		tailCoreState:          jsonArray(burnMint.coreStateJSON()),
		tailReplayNode:         jsonArray(burnMint.replayNodeJSON("")),
		tailCoinFactory:        jsonArray(burnMint.factoryJSON()),
		tailDepositPreapproval: jsonArray(acsContractJSONWithArg("pkg:"+tailDepositPreapproval, "dep-1", "ZGVw", "sync1", preapprovalArg(testRecipient, burnMint.instrumentAdmin, burnMint.instrumentID))),
	}
	registerFixtures := flowFixtures{
		tailNttGovernance:      jsonArray(acsContractJSONWithArg("pkg:"+tailNttGovernance, "gov-1", "Z292", "sync1", guardianAnchorArg(testGuardianGovernanceParty))),
		tailEmitterRegistry:    jsonArray(acsContractJSONWithArg("pkg:"+tailEmitterRegistry, "emreg-1", "ZW1yZWc=", "sync1", guardianAnchorArg(testGuardianGovernanceParty))),
		tailReplayRootRegistry: jsonArray(acsContractJSONWithArg("pkg:"+tailReplayRootRegistry, "rrreg-1", "cnJyZWc=", "sync1", guardianAnchorArg(testGuardianGovernanceParty))),
		tailCoreState:          jsonArray(acsContractJSONWithArg("pkg:"+tailCoreState, "cs-1", "Y3M=", "sync1", guardianAnchorArg(testGuardianGovernanceParty))),
		tailCoinFactory:        jsonArray(acsContractJSONWithArg("pkg:"+tailCoinFactory, "canon-1", "ZmFjdA==", "sync1", coinFactoryArg(testGuardianGovernanceParty))),
	}
	flowRuns := []struct {
		fixtures    flowFixtures
		flow, query string
	}{
		{lockUnlockFixtures, "release", "manager=" + lockUnlock.managerAddr + "&digest=" + lockUnlock.digest + "&recipient=" + lockUnlock.reserveCustodian()},
		{lockUnlockFixtures, "accept-admin", "manager=" + lockUnlock.managerAddr + "&digest=" + lockUnlock.digest},
		{lockUnlockFixtures, "transfer", "manager=" + lockUnlock.managerAddr},
		{lockUnlockFixtures, "set-peer", "manager=" + lockUnlock.managerAddr + "&digest=" + lockUnlock.digest},
		{lockUnlockFixtures, "consolidate", "manager=" + lockUnlock.managerAddr},
		{burnMintFixtures, "mint", "manager=" + burnMint.managerAddr + "&digest=" + burnMint.digest + "&recipient=" + testRecipient},
		{registerFixtures, "register", "gg=" + testGuardianGovernanceParty + "&by-vaa=true"},
	}

	readersByTail := map[string]map[partyID]bool{}
	for _, run := range flowRuns {
		srv, recorder := newRecordingFlowTestServer(t, run.fixtures)
		response := requestFlow(t, srv, run.flow, run.query)
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", run.flow, response.Body.String())
		for tail, parties := range recorder.readerPartiesByTail() {
			if readersByTail[tail] == nil {
				readersByTail[tail] = map[partyID]bool{}
			}
			for _, party := range parties {
				readersByTail[tail][party] = true
			}
		}
	}

	expectedReaders := []struct {
		tail    string
		readers []partyID
	}{
		{tailNttManager, []partyID{testGuardianGovernanceParty}},
		{tailLockedLedger, []partyID{testGuardianGovernanceParty}},
		{tailCoreState, []partyID{testGuardianGovernanceParty}},
		{tailEmitter, []partyID{testGuardianGovernanceParty}},
		{tailReplayNode, []partyID{testGuardianGovernanceParty}},
		{tailNttGovernance, []partyID{testGuardianGovernanceParty}},
		{tailEmitterRegistry, []partyID{testGuardianGovernanceParty}},
		{tailCoinFactory, []partyID{testGuardianGovernanceParty}},
		{tailDepositPreapproval, []partyID{testGuardianGovernanceParty}},
		{tailAdminTransferProposal, []partyID{testGuardianGovernanceParty}},
		{tailTransferPreapproval, []partyID{testGuardianGovernanceParty, testNttCustodyParty}},
		{tailCoin, []partyID{testNttCustodyParty}},
		{tailReplayRootRegistry, []partyID{testOperatorParty}},
	}
	require.Len(t, expectedReaders, len(flowTemplateByTail))
	for _, expected := range expectedReaders {
		t.Run(expected.tail, func(t *testing.T) {
			require.Contains(t, readersByTail, expected.tail, "no flow read %s", expected.tail)
			assert.Equal(t, sortedParties(expected.readers), sortedPartySet(readersByTail[expected.tail]))
			if expected.tail != tailReplayRootRegistry {
				assert.NotContains(t, readersByTail[expected.tail], partyID(testOperatorParty), "the operator reads ReplayRootRegistry only")
			}
		})
	}
}

func sortedParties(parties []partyID) []partyID {
	sorted := append([]partyID(nil), parties...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	return sorted
}

func sortedPartySet(partySet map[partyID]bool) []partyID {
	parties := make([]partyID, 0, len(partySet))
	for party := range partySet {
		parties = append(parties, party)
	}
	return sortedParties(parties)
}

func TestRegisterFlowNeedsOperatorParty(t *testing.T) {
	cases := []struct {
		name          string
		operatorParty partyID
		query         string
		wantStatus    int
	}{
		{"no operator party", "", "", http.StatusNotFound},
		{"no operator party, with gg", "", "gg=" + testGuardianGovernanceParty, http.StatusNotFound},
		{"gg param differs from the configured gg", testOperatorParty, "gg=other-gg::1", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, counter := newCountedFlowTestServer(t, flowFixtures{})
			srv.opts.readerParties.operator = tc.operatorParty
			recorder := requestFlow(t, srv, "register", tc.query)
			assert.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.Zero(t, counter.totalCalls())
		})
	}
}

func TestParseFlagsReaderParties(t *testing.T) {
	const jsonAPI = "http://127.0.0.1:6975"
	cases := []struct {
		name        string
		args        []string
		wantParties readerPartiesByRole
		wantErr     string
	}{
		{"gg and nttCustody", []string{"--guardian-governance-party", "gg::1", "--ntt-custody-party", "custody::1"},
			readerPartiesByRole{guardianGovernance: "gg::1", nttCustody: "custody::1"}, ""},
		{"all three roles", []string{"--guardian-governance-party", "gg::1", "--ntt-custody-party", "custody::1", "--operator-party", "operator::1"},
			readerPartiesByRole{guardianGovernance: "gg::1", nttCustody: "custody::1", operator: "operator::1"}, ""},
		{"gg missing", []string{"--ntt-custody-party", "custody::1"}, readerPartiesByRole{}, "--guardian-governance-party"},
		{"nttCustody missing", []string{"--guardian-governance-party", "gg::1"}, readerPartiesByRole{}, "--ntt-custody-party"},
		{"gg equals nttCustody", []string{"--guardian-governance-party", "gg::1", "--ntt-custody-party", "gg::1"}, readerPartiesByRole{}, "distinct"},
		{"operator equals gg", []string{"--guardian-governance-party", "gg::1", "--ntt-custody-party", "custody::1", "--operator-party", "gg::1"}, readerPartiesByRole{}, "distinct"},
		{"operator equals nttCustody", []string{"--guardian-governance-party", "gg::1", "--ntt-custody-party", "custody::1", "--operator-party", "custody::1"}, readerPartiesByRole{}, "distinct"},
		{"party with surrounding space", []string{"--guardian-governance-party", " gg::1", "--ntt-custody-party", "custody::1"}, readerPartiesByRole{}, "--guardian-governance-party"},
		{"party with a comma", []string{"--guardian-governance-party", "gg::1,custody::1", "--ntt-custody-party", "custody::1"}, readerPartiesByRole{}, "--guardian-governance-party"},
		{"party at the length limit", []string{"--guardian-governance-party", strings.Repeat("g", maxPartyIDBytes), "--ntt-custody-party", "custody::1"},
			readerPartiesByRole{guardianGovernance: partyID(strings.Repeat("g", maxPartyIDBytes)), nttCustody: "custody::1"}, ""},
		{"party over the length limit", []string{"--guardian-governance-party", strings.Repeat("g", maxPartyIDBytes+1), "--ntt-custody-party", "custody::1"}, readerPartiesByRole{}, "--guardian-governance-party"},
		{"removed --disclosing-party flag", []string{"--disclosing-party", "gg::1", "--guardian-governance-party", "gg::1", "--ntt-custody-party", "custody::1"}, readerPartiesByRole{}, "disclosing-party"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseFlags(append([]string{"--json-api", jsonAPI}, tc.args...))
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantParties, opts.readerParties)
		})
	}
}

func TestReaderRolesCoverEveryFlowTemplate(t *testing.T) {
	for tail := range flowTemplateByTail {
		roles, present := readerRolesByTail[tail]
		assert.True(t, present, "no reader role for %s", tail)
		assert.NotEmpty(t, roles, "no reader role for %s", tail)
	}
	assert.Len(t, readerRolesByTail, len(flowTemplateByTail))
}
