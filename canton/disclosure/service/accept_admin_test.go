package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleFlows_AcceptAdmin_StrangerProposalIgnored(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	proposalJSON := func(contractID, admin, newAdmin, managerAddress string) string {
		return acsContractJSONWithArg("pkg:"+tailAdminTransferProposal, contractID, "cHJvcA==", "sync1", adminTransferProposalArg(admin, newAdmin, managerAddress))
	}
	spoofedAdmin := proposalJSON("prop-spoofed-admin", "stranger::1", b.gg, b.managerAddr)
	spoofedNewAdmin := proposalJSON("prop-spoofed-new-admin", b.admin, "stranger::1", b.managerAddr)
	realProposal := proposalJSON("prop-real", b.admin, b.gg, b.managerAddr)
	cases := []struct {
		name            string
		proposals       []string
		wantContractID  string
		wantMissingRole bool
	}{
		{"spoofed admin only", []string{spoofedAdmin}, "", true},
		{"spoofed newAdmin only", []string{spoofedNewAdmin}, "", true},
		{"spoofed proposals and the real one", []string{spoofedAdmin, spoofedNewAdmin, realProposal}, "prop-real", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := flowFixtures{
				tailNttManager:            jsonArray(b.managerJSON()),
				tailCoreState:             jsonArray(b.coreStateJSON()),
				tailReplayNode:            jsonArray(b.replayNodeJSON("")),
				tailAdminTransferProposal: jsonArray(tc.proposals...),
			}
			srv := newFlowTestServer(t, fixtures)
			recorder := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			response := decodeFlowResponse(t, recorder)
			if tc.wantMissingRole {
				assert.Contains(t, missingRolesOf(response.Missing), roleAdminTransferProposal)
				assert.NotContains(t, rolesOf(response.Disclosures), roleAdminTransferProposal)
				return
			}
			for _, disclosure := range response.Disclosures {
				if disclosure.Role == roleAdminTransferProposal {
					assert.Equal(t, tc.wantContractID, disclosure.ContractID)
				}
			}
			assert.Contains(t, rolesOf(response.Disclosures), roleAdminTransferProposal)
		})
	}
}

func TestHandleFlows_AcceptAdmin(t *testing.T) {
	cases := []struct {
		name                 string
		admin                string
		wantStatus           int
		wantProposalQueried  bool
		wantMessageSubstring string
	}{
		{"admin is already guardianGovernance", "gg::1", http.StatusConflict, false, "already guardianGovernance"},
		{"admin is a third party", testRecipient, http.StatusOK, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBaseFixturesAdminedBy(tokenConfigBurnMint, tc.admin)
			fixtures := flowFixtures{
				tailNttManager:            jsonArray(b.managerJSON()),
				tailCoreState:             jsonArray(b.coreStateJSON()),
				tailReplayNode:            jsonArray(b.replayNodeJSON("")),
				tailAdminTransferProposal: jsonArray(acsContractJSONWithArg("pkg:"+tailAdminTransferProposal, "prop-1", "cHJvcA==", "sync1", adminTransferProposalArg(b.admin, b.gg, b.managerAddr))),
			}
			srv, queriedTemplates := newRecordingFlowTestServer(t, fixtures)
			recorder := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), tc.wantMessageSubstring)
			if tc.wantProposalQueried {
				assert.Contains(t, queriedTemplates.tails(), tailAdminTransferProposal)
			} else {
				assert.NotContains(t, queriedTemplates.tails(), tailAdminTransferProposal)
			}
		})
	}
}
