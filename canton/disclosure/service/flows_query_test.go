package main

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callCountingACS counts the calls that reach inner.
type callCountingACS struct {
	inner                acsClient
	ledgerEndCalls       atomic.Int64
	activeContractsCalls atomic.Int64
	contractByIDCalls    atomic.Int64
	amountTotalsCalls    atomic.Int64
}

func (counter *callCountingACS) LedgerEnd(ctx context.Context) (int64, error) {
	counter.ledgerEndCalls.Add(1)
	return counter.inner.LedgerEnd(ctx)
}

func (counter *callCountingACS) ActiveContracts(ctx context.Context, query activeContractsQuery) ([]acsEntry, error) {
	counter.activeContractsCalls.Add(1)
	return counter.inner.ActiveContracts(ctx, query)
}

func (counter *callCountingACS) ActiveContractByID(ctx context.Context, readerParties []partyID, wantContractID contractID) (acsEntry, bool, error) {
	counter.contractByIDCalls.Add(1)
	return counter.inner.ActiveContractByID(ctx, readerParties, wantContractID)
}

func (counter *callCountingACS) ActiveContractAmountTotals(ctx context.Context, query amountTotalsQuery) (amountTotalsByGroup, error) {
	counter.amountTotalsCalls.Add(1)
	return counter.inner.ActiveContractAmountTotals(ctx, query)
}

func (counter *callCountingACS) totalCalls() int64 {
	return counter.ledgerEndCalls.Load() + counter.activeContractsCalls.Load() + counter.contractByIDCalls.Load() + counter.amountTotalsCalls.Load()
}

// newCountedFlowTestServer is newFlowTestServer with a call counter in front of the upstream client.
func newCountedFlowTestServer(t *testing.T, fixtures flowFixtures) (*server, *callCountingACS) {
	t.Helper()
	srv := newFlowTestServer(t, fixtures)
	counter := &callCountingACS{inner: srv.acs}
	srv.acs = counter
	return srv, counter
}

func TestParseFlowRequestRejectsBeforeUpstream(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	validManagerDigest := "manager=" + b.managerAddr + "&digest=" + b.digest
	cases := []struct {
		name       string
		flow       string
		query      string
		wantStatus int
	}{
		{"repeated manager", "release", validManagerDigest + "&manager=" + b.managerAddr, http.StatusBadRequest},
		{"9000 repeated manager", "release", strings.Repeat("manager="+b.managerAddr+"&", 9000), http.StatusRequestURITooLong},
		{"query one byte over the limit", "transfer", "manager=" + b.managerAddr + "&x=" + strings.Repeat("a", maxFlowQueryBytes-len("manager="+b.managerAddr+"&x=")+1), http.StatusRequestURITooLong},
		{"unknown param template", "release", validManagerDigest + "&template=Token.CIP0056.Coin:Coin", http.StatusBadRequest},
		{"param of another flow", "transfer", validManagerDigest, http.StatusBadRequest},
		{"unknown flow", "not-a-flow", "manager=" + b.managerAddr, http.StatusNotFound},
		{"bad hex manager", "release", "manager=" + strings.Repeat("zz", 32) + "&digest=" + b.digest, http.StatusBadRequest},
		{"short digest", "release", "manager=" + b.managerAddr + "&digest=abcd", http.StatusBadRequest},
		{"missing manager", "transfer", "", http.StatusBadRequest},
		{"empty recipient on mint", "mint", validManagerDigest + "&recipient=", http.StatusBadRequest},
		{"missing recipient on mint", "mint", validManagerDigest, http.StatusBadRequest},
		{"empty recipient on release", "release", validManagerDigest + "&recipient=", http.StatusBadRequest},
		{"recipient without a fingerprint on mint", "mint", validManagerDigest + "&recipient=alice::1", http.StatusBadRequest},
		{"recipient with a line feed on mint", "mint", validManagerDigest + "&recipient=" + testRecipient + "%0A", http.StatusBadRequest},
		{"recipient with an uppercase fingerprint on release", "release", validManagerDigest + "&recipient=" + strings.ToUpper(testRecipient), http.StatusBadRequest},
		{"gg without a fingerprint", "register", "gg=gg::1", http.StatusBadRequest},
		{"by-vaa not a bool", "register", "by-vaa=yes", http.StatusBadRequest},
		{"by-vaa true without gg", "register", "by-vaa=true", http.StatusBadRequest},
		{"empty gg", "register", "gg=", http.StatusBadRequest},
		{"malformed query", "transfer", "manager=" + b.managerAddr + "&%zz", http.StatusBadRequest},
		{"semicolon separator", "transfer", "manager=" + b.managerAddr + ";digest=" + b.digest, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, counter := newCountedFlowTestServer(t, flowFixtures{})
			recorder := requestFlow(t, srv, tc.flow, tc.query)
			assert.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.Zero(t, counter.totalCalls(), "a rejected query reaches no upstream")
		})
	}

	t.Run("valid release", func(t *testing.T) {
		fixtures := flowFixtures{
			tailNttManager:   jsonArray(b.managerJSON()),
			tailCoreState:    jsonArray(b.coreStateJSON()),
			tailReplayNode:   jsonArray(b.replayNodeJSON("")),
			tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
			tailCoinFactory:  jsonArray(b.factoryJSON()),
		}
		srv, counter := newCountedFlowTestServer(t, fixtures)
		recorder := requestFlow(t, srv, "release", validManagerDigest)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, int64(1), counter.ledgerEndCalls.Load())
	})
}

func TestCheckFlowQuery(t *testing.T) {
	cases := []struct {
		name       string
		flow       string
		rawQuery   string
		want       map[string]string
		wantStatus int
	}{
		{"single values", "release", "manager=a&digest=b", map[string]string{"manager": "a", "digest": "b"}, 0},
		{"empty query", "register", "", map[string]string{}, 0},
		{"repeated name", "release", "manager=a&manager=a", nil, http.StatusBadRequest},
		{"unknown name", "transfer", "manager=a&template=x", nil, http.StatusBadRequest},
		{"unknown flow", "nope", "", nil, http.StatusNotFound},
		{"exactly the byte limit", "transfer", "manager=" + strings.Repeat("a", maxFlowQueryBytes-len("manager=")), map[string]string{"manager": strings.Repeat("a", maxFlowQueryBytes-len("manager="))}, 0},
		{"one byte over the limit", "transfer", "manager=" + strings.Repeat("a", maxFlowQueryBytes-len("manager=")+1), nil, http.StatusRequestURITooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := checkFlowQuery(tc.flow, tc.rawQuery)
			if tc.wantStatus != 0 {
				requireFlowErrorStatus(t, err, tc.wantStatus)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseFlowRequestTypes(t *testing.T) {
	managerHex := strings.Repeat("ab", 32)
	digestHex := strings.Repeat("cd", 32)
	cases := []struct {
		name     string
		flow     string
		rawQuery string
		want     flowRequest
	}{
		{"release without recipient", "release", "manager=" + strings.ToUpper(managerHex) + "&digest=" + digestHex,
			releaseRequest{manager: managerAddressHex{hex: managerHex}, digest: vaaDigestHex{hex: digestHex}}},
		{"release with recipient", "release", "manager=" + managerHex + "&digest=" + digestHex + "&recipient=" + testRecipient,
			releaseRequest{manager: managerAddressHex{hex: managerHex}, digest: vaaDigestHex{hex: digestHex}, recipient: testRecipient}},
		{"mint", "mint", "manager=" + managerHex + "&digest=" + digestHex + "&recipient=" + testRecipient,
			mintRequest{manager: managerAddressHex{hex: managerHex}, digest: vaaDigestHex{hex: digestHex}, recipient: testRecipient}},
		{"set-peer", "set-peer", "manager=" + managerHex + "&digest=" + digestHex,
			setPeerRequest{manager: managerAddressHex{hex: managerHex}, digest: vaaDigestHex{hex: digestHex}}},
		{"accept-admin", "accept-admin", "manager=" + managerHex + "&digest=" + digestHex,
			acceptAdminRequest{manager: managerAddressHex{hex: managerHex}, digest: vaaDigestHex{hex: digestHex}}},
		{"transfer", "transfer", "manager=" + managerHex, transferRequest{manager: managerAddressHex{hex: managerHex}}},
		{"consolidate", "consolidate", "manager=" + managerHex, consolidateRequest{manager: managerAddressHex{hex: managerHex}}},
		{"register defaults", "register", "", registerRequest{}},
		{"register by vaa", "register", "gg=" + testGuardianGovernanceParty + "&by-vaa=true", registerRequest{guardianGovernance: testGuardianGovernanceParty, byVAA: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseFlowRequest(tc.flow, tc.rawQuery)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.flow, got.flowName())
		})
	}
}
