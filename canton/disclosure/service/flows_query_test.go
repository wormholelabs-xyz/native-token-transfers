package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func (counter *callCountingACS) ActiveContractAmountTotals(ctx context.Context, query amountTotalsQuery) (amountTotalsRead, error) {
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
	validRegisterClaim := testRegisterClaimQuery()
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
		{"gg without a fingerprint", "register", validRegisterClaim + "&gg=gg::1", http.StatusBadRequest},
		{"by-vaa not a bool", "register", validRegisterClaim + "&by-vaa=yes", http.StatusBadRequest},
		{"by-vaa true without gg", "register", validRegisterClaim + "&by-vaa=true", http.StatusBadRequest},
		{"empty gg", "register", validRegisterClaim + "&gg=", http.StatusBadRequest},
		{"register without parameters", "register", "", http.StatusBadRequest},
		{"register without admin", "register", "instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=" + testRegisterInstrumentID, http.StatusBadRequest},
		{"register without instrument-admin", "register", "admin=" + testRecipient + "&instrument-id=" + testRegisterInstrumentID, http.StatusBadRequest},
		{"register without instrument-id", "register", "admin=" + testRecipient + "&instrument-admin=" + testGuardianGovernanceParty, http.StatusBadRequest},
		{"empty admin", "register", "admin=&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=" + testRegisterInstrumentID, http.StatusBadRequest},
		{"empty instrument-admin", "register", "admin=" + testRecipient + "&instrument-admin=&instrument-id=" + testRegisterInstrumentID, http.StatusBadRequest},
		{"empty instrument-id", "register", "admin=" + testRecipient + "&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=", http.StatusBadRequest},
		{"admin without a fingerprint", "register", "admin=admin::1&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=" + testRegisterInstrumentID, http.StatusBadRequest},
		{"instrument-admin with an uppercase fingerprint", "register", "admin=" + testRecipient + "&instrument-admin=" + strings.ToUpper(testGuardianGovernanceParty) + "&instrument-id=" + testRegisterInstrumentID, http.StatusBadRequest},
		{"instrument-id not UTF-8", "register", "admin=" + testRecipient + "&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=id%FF", http.StatusBadRequest},
		{"instrument-id over the byte limit", "register", "admin=" + testRecipient + "&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=" + strings.Repeat("i", instrumentIDMaxBytes+1), http.StatusBadRequest},
		{"namespace parameter removed", "register", validRegisterClaim + "&namespace=" + url.QueryEscape("ntt:ab"), http.StatusBadRequest},
		{"by-vaa true with an instrument-admin other than gg", "register", "admin=" + testRecipient + "&instrument-admin=" + testRecipient + "&instrument-id=" + testRegisterInstrumentID + "&gg=" + testGuardianGovernanceParty + "&by-vaa=true", http.StatusBadRequest},
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
		{"empty query", "transfer", "", map[string]string{}, 0},
		{"repeated name", "release", "manager=a&manager=a", nil, http.StatusBadRequest},
		{"unknown name", "transfer", "manager=a&template=x", nil, http.StatusBadRequest},
		{"unknown flow", "nope", "", nil, http.StatusNotFound},
		{"exactly the byte limit", "transfer", "manager=" + strings.Repeat("a", maxFlowQueryBytes-len("manager=")), map[string]string{"manager": strings.Repeat("a", maxFlowQueryBytes-len("manager="))}, 0},
		{"one byte over the limit", "transfer", "manager=" + strings.Repeat("a", maxFlowQueryBytes-len("manager=")+1), nil, http.StatusRequestURITooLong},
		{"register one byte over the default limit", "register", "admin=" + strings.Repeat("a", maxFlowQueryBytes-len("admin=")+1), map[string]string{"admin": strings.Repeat("a", maxFlowQueryBytes-len("admin=")+1)}, 0},
		{"register at its limit", "register", "admin=" + strings.Repeat("a", testRegisterQueryCapBytes-len("admin=")), map[string]string{"admin": strings.Repeat("a", testRegisterQueryCapBytes-len("admin="))}, 0},
		{"register one byte over its limit", "register", "admin=" + strings.Repeat("a", testRegisterQueryCapBytes-len("admin=")+1), nil, http.StatusRequestURITooLong},
		{"unknown flow over the default limit", "nope", strings.Repeat("a", maxFlowQueryBytes+1), nil, http.StatusRequestURITooLong},
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
			releaseRequest{manager: managerAddressHex{managerAddressHexText: managerHex}, digest: vaaDigestHex{vaaDigestHexText: digestHex}}},
		{"release with recipient", "release", "manager=" + managerHex + "&digest=" + digestHex + "&recipient=" + testRecipient,
			releaseRequest{manager: managerAddressHex{managerAddressHexText: managerHex}, digest: vaaDigestHex{vaaDigestHexText: digestHex}, recipient: testRecipient}},
		{"mint", "mint", "manager=" + managerHex + "&digest=" + digestHex + "&recipient=" + testRecipient,
			mintRequest{manager: managerAddressHex{managerAddressHexText: managerHex}, digest: vaaDigestHex{vaaDigestHexText: digestHex}, recipient: testRecipient}},
		{"set-peer", "set-peer", "manager=" + managerHex + "&digest=" + digestHex,
			setPeerRequest{manager: managerAddressHex{managerAddressHexText: managerHex}, digest: vaaDigestHex{vaaDigestHexText: digestHex}}},
		{"accept-admin", "accept-admin", "manager=" + managerHex + "&digest=" + digestHex,
			acceptAdminRequest{manager: managerAddressHex{managerAddressHexText: managerHex}, digest: vaaDigestHex{vaaDigestHexText: digestHex}}},
		{"transfer", "transfer", "manager=" + managerHex, transferRequest{manager: managerAddressHex{managerAddressHexText: managerHex}}},
		{"consolidate", "consolidate", "manager=" + managerHex, consolidateRequest{manager: managerAddressHex{managerAddressHexText: managerHex}}},
		{"register defaults", "register", testRegisterClaimQuery(), testRegisterRequest()},
		{"register by vaa", "register", testRegisterClaimQuery() + "&gg=" + testGuardianGovernanceParty + "&by-vaa=true",
			registerRequest{guardianGovernance: testGuardianGovernanceParty, byVAA: true, admin: testRecipient, instrumentAdmin: testGuardianGovernanceParty, instrumentID: instrumentIDText{instrumentIDUTF8Text: testRegisterInstrumentID}}},
		{"register by vaa with gg as instrument-admin", "register", "admin=" + testRecipient + "&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=" + testRegisterInstrumentID + "&gg=" + testGuardianGovernanceParty + "&by-vaa=true",
			registerRequest{guardianGovernance: testGuardianGovernanceParty, byVAA: true, admin: testRecipient, instrumentAdmin: testGuardianGovernanceParty, instrumentID: instrumentIDText{instrumentIDUTF8Text: testRegisterInstrumentID}}},
		{"register not by vaa with an instrument-admin other than gg", "register", "admin=" + testRecipient + "&instrument-admin=" + testRecipient + "&instrument-id=" + testRegisterInstrumentID + "&gg=" + testGuardianGovernanceParty + "&by-vaa=false",
			registerRequest{guardianGovernance: testGuardianGovernanceParty, admin: testRecipient, instrumentAdmin: testRecipient, instrumentID: instrumentIDText{instrumentIDUTF8Text: testRegisterInstrumentID}}},
		{"register with a multi-byte instrument id", "register", "admin=" + testRecipient + "&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=" + url.QueryEscape("é€😀"),
			registerRequest{admin: testRecipient, instrumentAdmin: testGuardianGovernanceParty, instrumentID: instrumentIDText{instrumentIDUTF8Text: "é€😀"}}},
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

const testRegisterInstrumentID = "wormhole-ntt:instrument-1"

// testRegisterClaimQuery is the three required register parameters.
func testRegisterClaimQuery() string {
	return "admin=" + testRecipient + "&instrument-admin=" + testGuardianGovernanceParty + "&instrument-id=" + url.QueryEscape(testRegisterInstrumentID)
}

func testRegisterRequest() registerRequest {
	return registerRequest{admin: testRecipient, instrumentAdmin: testGuardianGovernanceParty, instrumentID: instrumentIDText{instrumentIDUTF8Text: testRegisterInstrumentID}}
}

// testRegisterQueryCapBytes is the register query with every parameter at its maximum length,
// fully percent-encoded.
const testRegisterQueryCapBytes = 3897

// percentEncodeEveryByte writes each byte of raw as %XX.
func percentEncodeEveryByte(raw string) string {
	var encoded strings.Builder
	for index := 0; index < len(raw); index++ {
		fmt.Fprintf(&encoded, "%%%02X", raw[index])
	}
	return encoded.String()
}

// maximalTestParty is a party id of exactly maxPartyIDBytes bytes with a hint of hintLetter.
func maximalTestParty(hintLetter string) string {
	party := strings.Repeat(hintLetter, maxPartyIDBytes-len(testPartyFingerprintSuffix)) + testPartyFingerprintSuffix
	if len(party) != maxPartyIDBytes {
		panic("maximalTestParty is not maxPartyIDBytes bytes")
	}
	return party
}

// maximalRegisterQuery is a valid register query of exactly testRegisterQueryCapBytes bytes.
func maximalRegisterQuery(t *testing.T) (string, registerRequest) {
	t.Helper()
	guardianGovernance := maximalTestParty("g")
	admin := maximalTestParty("a")
	instrumentAdmin := maximalTestParty("i")
	instrumentID := strings.Repeat("n", instrumentIDMaxBytes)
	query := "gg=" + percentEncodeEveryByte(guardianGovernance) +
		"&by-vaa=" + percentEncodeEveryByte("false") +
		"&admin=" + percentEncodeEveryByte(admin) +
		"&instrument-admin=" + percentEncodeEveryByte(instrumentAdmin) +
		"&instrument-id=" + percentEncodeEveryByte(instrumentID)
	require.Len(t, query, testRegisterQueryCapBytes)
	want := registerRequest{
		guardianGovernance: partyID(guardianGovernance),
		admin:              partyID(admin),
		instrumentAdmin:    partyID(instrumentAdmin),
		instrumentID:       instrumentIDText{instrumentIDUTF8Text: instrumentID},
	}
	return query, want
}

func TestParseFlowRequest_RegisterQueryCap(t *testing.T) {
	maximalQuery, maximalRequest := maximalRegisterQuery(t)
	cases := []struct {
		name       string
		flow       string
		rawQuery   string
		wantStatus int
	}{
		{"register at its limit", "register", maximalQuery, 0},
		{"register one byte over its limit", "register", maximalQuery + "&", http.StatusRequestURITooLong},
		{"transfer one byte over the default limit", "transfer", "manager=" + strings.Repeat("a", maxFlowQueryBytes-len("manager=")+1), http.StatusRequestURITooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseFlowRequest(tc.flow, tc.rawQuery)
			if tc.wantStatus != 0 {
				requireFlowErrorStatus(t, err, tc.wantStatus)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, maximalRequest, got)
		})
	}
}

func TestHandleFlows_RegisterQueryCapOverHTTP(t *testing.T) {
	maximalQuery, maximalRequest := maximalRegisterQuery(t)
	guardianGovernance := string(maximalRequest.guardianGovernance)
	flowServer := newFlowTestServer(t, registerFlowFixtures(guardianGovernance, testRegisterNextID, claimedRootsTreeRootJSON(guardianGovernance)))
	flowServer.opts.readerParties.guardianGovernance = maximalRequest.guardianGovernance
	registerCapServer := httptest.NewUnstartedServer(flowServer)
	registerCapServer.Config = newHTTPServer(flowServer)
	registerCapServer.Start()
	t.Cleanup(registerCapServer.Close)
	cases := []struct {
		name       string
		rawQuery   string
		wantStatus int
	}{
		{"at the register limit", maximalQuery, http.StatusOK},
		{"one byte over the register limit", maximalQuery + "&", http.StatusRequestURITooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := registerCapServer.Client().Get(registerCapServer.URL + "/v1/flows/register?" + tc.rawQuery)
			require.NoError(t, err)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, response.StatusCode, string(body))
		})
	}
}
