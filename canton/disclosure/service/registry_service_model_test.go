package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests of the registry routes under the service model of the flow routes: typed query parsing
// before the upstream, the fixed 5xx error policy with a request id, reads as gg only, exact
// filters before the cap, reads by contract id, package and template checks, and response bounds.

// newCountedRegistryTestServer is newRegistryTestServer with a call counter in front of the upstream.
func newCountedRegistryTestServer(t *testing.T, fixtures flowFixtures) (*server, *callCountingACS) {
	t.Helper()
	srv := newRegistryTestServer(t, fixtures)
	counter := &callCountingACS{inner: srv.acs}
	srv.acs = counter
	return srv, counter
}

// newRecordingRegistryTestServer is newRegistryTestServer with the upstream read recorder.
func newRecordingRegistryTestServer(t *testing.T, fixtures flowFixtures) (*server, *queriedTemplateRecorder) {
	t.Helper()
	srv, recorder := newRecordingFlowTestServer(t, fixtures)
	useRegistryTestParties(srv)
	srv.registry = newRegistryConfig(registryTestGG, registryTestDisplay())
	useRegistryTestReadClock(srv)
	return srv, recorder
}

// requireRegistryErrorBody requires the ErrorResponse shape with exactly one field, and a request
// id header. It returns the error text.
func requireRegistryErrorBody(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.Regexp(t, requestIDPattern, recorder.Header().Get("X-Request-Id"))
	var fields map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &fields), recorder.Body.String())
	require.Len(t, fields, 1, recorder.Body.String())
	errorText, isString := fields["error"].(string)
	require.True(t, isString, recorder.Body.String())
	require.NotEmpty(t, errorText)
	return errorText
}

func registryContractJSONWithPackage(tail, contractID, packageName, synchronizerID, createArgument string) string {
	return fmt.Sprintf(
		`{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":%q,"templateId":%q,"createdEventBlob":"YmxvYg==","createArgument":%s,"createdAt":%q,"packageName":%q},"synchronizerId":%q}}}`,
		contractID, "pkgid:"+tail, createArgument, registryTestCreatedAt, packageName, synchronizerID,
	)
}

func TestParseListInstrumentsRequest(t *testing.T) {
	cases := []struct {
		name          string
		rawQuery      string
		wantPageSize  int
		wantPageToken string
		wantStatus    int // 0: parse succeeds
	}{
		{name: "empty", rawQuery: "", wantPageSize: defaultInstrumentPageSize},
		{name: "page size 1", rawQuery: "pageSize=1", wantPageSize: 1},
		{name: "page size at the max", rawQuery: "pageSize=100", wantPageSize: 100},
		{name: "page size above the max", rawQuery: "pageSize=101", wantPageSize: 100},
		{name: "page size zero", rawQuery: "pageSize=0", wantPageSize: 1},
		{name: "page size negative", rawQuery: "pageSize=-1", wantPageSize: 1},
		{name: "page token", rawQuery: "pageToken=" + instrumentPageToken("abc"), wantPageSize: defaultInstrumentPageSize, wantPageToken: "abc"},
		{name: "page token at the length limit", rawQuery: "pageToken=" + instrumentPageToken(strings.Repeat("a", maxRegistryIdentifierLength)), wantPageSize: defaultInstrumentPageSize, wantPageToken: strings.Repeat("a", maxRegistryIdentifierLength)},
		{name: "page size not an integer", rawQuery: "pageSize=ten", wantStatus: http.StatusBadRequest},
		{name: "page size overflows int", rawQuery: "pageSize=9999999999999999999999", wantStatus: http.StatusBadRequest},
		{name: "page size empty", rawQuery: "pageSize=", wantStatus: http.StatusBadRequest},
		{name: "repeated page size", rawQuery: "pageSize=1&pageSize=2", wantStatus: http.StatusBadRequest},
		{name: "repeated page token", rawQuery: "pageToken=a&pageToken=b", wantStatus: http.StatusBadRequest},
		{name: "unknown parameter", rawQuery: "limit=5", wantStatus: http.StatusBadRequest},
		{name: "empty page token", rawQuery: "pageToken=", wantStatus: http.StatusBadRequest},
		{name: "page token above the length limit", rawQuery: "pageToken=" + instrumentPageToken(strings.Repeat("a", maxRegistryIdentifierLength+1)), wantStatus: http.StatusBadRequest},
		{name: "malformed escape", rawQuery: "pageToken=%zz", wantStatus: http.StatusBadRequest},
		{name: "query at the byte limit", rawQuery: "pageToken=" + strings.Repeat("a", maxFlowQueryBytes-len("pageToken=")), wantStatus: http.StatusBadRequest},
		{name: "query above the byte limit", rawQuery: "pageToken=" + strings.Repeat("a", maxFlowQueryBytes), wantStatus: http.StatusRequestURITooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := parseListInstrumentsRequest(tc.rawQuery)
			if tc.wantStatus != 0 {
				var flowErr *flowError
				require.ErrorAs(t, err, &flowErr)
				assert.Equal(t, tc.wantStatus, flowErr.status)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPageSize, request.pageSize)
			assert.Equal(t, tc.wantPageToken, request.afterInstrumentID)
		})
	}
}

func TestRegistryRejectsBeforeUpstream(t *testing.T) {
	validTransfer := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false)
	cases := []struct {
		name       string
		method     string
		target     string
		body       string
		wantStatus int
	}{
		{"info with a parameter", http.MethodGet, pathRegistryInfo + "?x=1", "", http.StatusBadRequest},
		{"instruments with a repeated pageSize", http.MethodGet, pathInstruments + "?pageSize=1&pageSize=2", "", http.StatusBadRequest},
		{"instruments with an unknown parameter", http.MethodGet, pathInstruments + "?template=Coin", "", http.StatusBadRequest},
		{"instruments with a 1025-byte query", http.MethodGet, pathInstruments + "?pageToken=" + strings.Repeat("a", maxFlowQueryBytes), "", http.StatusRequestURITooLong},
		{"instrument with a parameter", http.MethodGet, pathInstruments + "/" + url.PathEscape(instrumentA) + "?x=1", "", http.StatusBadRequest},
		{"instrument without display config", http.MethodGet, pathInstruments + "/" + url.PathEscape(instrumentUnconfigured), "", http.StatusNotFound},
		{"instrument id above the length limit", http.MethodGet, pathInstruments + "/" + strings.Repeat("i", maxRegistryIdentifierLength+1), "", http.StatusNotFound},
		{"transfer factory with a parameter", http.MethodPost, pathTransferFactory + "?debug=1", validTransfer, http.StatusBadRequest},
		{"transfer factory with an empty object", http.MethodPost, pathTransferFactory, `{}`, http.StatusBadRequest},
		{"transfer factory with a mismatched expectedAdmin", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestIssuer, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false), http.StatusBadRequest},
		{"transfer factory for another admin", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestIssuer, registryTestAlice, registryTestBob, "1", registryTestIssuer, instrumentThirdParty), false), http.StatusNotFound},
		{"transfer factory with a body above 64 KiB", http.MethodPost, pathTransferFactory, `{"choiceArguments":{},"pad":"` + strings.Repeat("x", maxRegistryRequestBodyBytes) + `"}`, http.StatusBadRequest},
		{"accept with a non-hex id", http.MethodPost, "/registry/transfer-instruction/v1/XYZ/choice-contexts/accept", `{}`, http.StatusBadRequest},
		{"unknown transfer instruction choice", http.MethodPost, "/registry/transfer-instruction/v1/" + cidInstructionA + "/choice-contexts/approve", `{}`, http.StatusNotFound},
		{"accept with a body that is an array", http.MethodPost, "/registry/transfer-instruction/v1/" + cidInstructionA + "/choice-contexts/accept", `[]`, http.StatusBadRequest},
		{"cancel with a parameter", http.MethodPost, "/registry/allocations/v1/" + cidAllocationA + "/choice-contexts/cancel?x=1", `{}`, http.StatusBadRequest},
		{"allocation factory with an empty object", http.MethodPost, pathAllocationFactory, `{}`, http.StatusBadRequest},
		{"transfer factory with a short-form sender", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestGG, "alice::1220a1", registryTestBob, "1", registryTestGG, instrumentA), false), http.StatusBadRequest},
		{"transfer factory with a receiver without a fingerprint", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, "bob", "1", registryTestGG, instrumentA), false), http.StatusBadRequest},
		{"transfer factory with an uppercase-hex expectedAdmin", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(strings.ToUpper(registryTestGG), registryTestAlice, registryTestBob, "1", strings.ToUpper(registryTestGG), instrumentA), false), http.StatusBadRequest},
		{"allocation factory with a short-form expectedAdmin", http.MethodPost, pathAllocationFactory, allocationFactoryTestBody(allocationChoiceArguments("gg::1220aa", registryTestAlice, "gg::1220aa", instrumentA)), http.StatusBadRequest},
		{"allocation factory with a short-form sender", http.MethodPost, pathAllocationFactory, allocationFactoryTestBody(allocationChoiceArguments(registryTestGG, "alice::1220a1", registryTestGG, instrumentA)), http.StatusBadRequest},
		{"allocation factory with a receiver hint above 185 bytes", http.MethodPost, pathAllocationFactory, allocationFactoryTestBody(strings.Replace(allocationChoiceArguments(registryTestGG, registryTestAlice, registryTestGG, instrumentA), `"receiver":"`+registryTestBob+`"`, `"receiver":"`+strings.Repeat("b", maxPartyHintBytes+1)+testPartyFingerprintSuffix+`"`, 1)), http.StatusBadRequest},
		{"unknown registry path", http.MethodGet, "/registry/metadata/v2/info", "", http.StatusNotFound},
		{"info by POST", http.MethodPost, pathRegistryInfo, `{}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, counter := newCountedRegistryTestServer(t, registryTestFixtures())
			recorder := serveRegistry(t, srv, tc.method, tc.target, tc.body)
			requireStatus(t, recorder, tc.wantStatus)
			requireRegistryErrorBody(t, recorder)
			requireNoStoreNoSniffHeaders(t, recorder)
			assert.Zero(t, counter.totalCalls(), "upstream calls before the request checks passed")
		})
	}

	t.Run("info answers without an upstream call", func(t *testing.T) {
		srv, counter := newCountedRegistryTestServer(t, registryTestFixtures())
		requireStatus(t, serveRegistry(t, srv, http.MethodGet, pathRegistryInfo, ""), http.StatusOK)
		assert.Zero(t, counter.totalCalls())
	})
	t.Run("registry off answers without an upstream call", func(t *testing.T) {
		srv, counter := newCountedFlowTestServer(t, registryTestFixtures())
		recorder := serveRegistry(t, srv, http.MethodGet, pathInstruments, "")
		requireStatus(t, recorder, http.StatusNotFound)
		requireRegistryErrorBody(t, recorder)
		assert.Zero(t, counter.totalCalls())
	})
	t.Run("a valid list reaches the upstream", func(t *testing.T) {
		srv, counter := newCountedRegistryTestServer(t, registryTestFixtures())
		requireStatus(t, serveRegistry(t, srv, http.MethodGet, pathInstruments+"?pageSize=2", ""), http.StatusOK)
		assert.Equal(t, int64(1), counter.ledgerEndCalls.Load())
		assert.Equal(t, int64(1), counter.activeContractsCalls.Load())
		assert.Equal(t, int64(1), counter.amountTotalsCalls.Load())
	})
}

// failingACS fails every upstream call with err.
type failingACS struct{ err error }

func (failing failingACS) LedgerEnd(context.Context) (int64, error) { return 0, failing.err }
func (failing failingACS) ActiveContracts(context.Context, activeContractsQuery) ([]acsEntry, error) {
	return nil, failing.err
}
func (failing failingACS) ActiveContractByID(context.Context, []partyID, contractID) (acsEntry, bool, error) {
	return acsEntry{}, false, failing.err
}
func (failing failingACS) ActiveContractAmountTotals(context.Context, amountTotalsQuery) (amountTotalsRead, error) {
	return amountTotalsRead{}, failing.err
}

func TestRegistryErrorPolicy(t *testing.T) {
	const upstreamSecret = "dial tcp 10.9.8.7:443 token=s3cr3t /etc/disclosure/gateway.jwt"
	conflictingDecimals := registryTestFixtures()
	conflictingDecimals[tailNttManager] = jsonArray(
		registryContractJSON(tailNttManager, registryTestCid("mgr-A1"), "bWdy", registryManagerArg(registryTestGG, instrumentA, tokenConfigBurnMint, 6, factoryTagBurnMint, cidCanonicalFactory)),
		registryContractJSON(tailNttManager, registryTestCid("mgr-A2"), "bWdy", registryManagerArg(registryTestGG, instrumentA, tokenConfigBurnMint, 8, factoryTagBurnMint, cidCanonicalFactory)),
	)
	validTransfer := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false)
	cases := []struct {
		name           string
		upstreamErr    error        // nil: the fixtures server
		fixtures       flowFixtures // used when upstreamErr is nil
		method, target string       // request
		body           string       // request body
		wantStatus     int          // reply status
		wantPublicText string       // fixed 5xx text; "" for a 4xx
		hiddenTexts    []string     // detail that must stay in the log
	}{
		{name: "upstream fault on the list", upstreamErr: errors.New(upstreamSecret), method: http.MethodGet, target: pathInstruments, wantStatus: http.StatusBadGateway, wantPublicText: "upstream ledger error", hiddenTexts: []string{"10.9.8.7", "s3cr3t", "gateway.jwt"}},
		{name: "upstream fault on the transfer factory", upstreamErr: errors.New(upstreamSecret), method: http.MethodPost, target: pathTransferFactory, body: validTransfer, wantStatus: http.StatusBadGateway, wantPublicText: "upstream ledger error", hiddenTexts: []string{"10.9.8.7", "s3cr3t"}},
		{name: "upstream fault on a read by id", upstreamErr: errors.New(upstreamSecret), method: http.MethodPost, target: "/registry/allocations/v1/" + cidAllocationA + "/choice-contexts/cancel", body: `{}`, wantStatus: http.StatusBadGateway, wantPublicText: "upstream ledger error", hiddenTexts: []string{"10.9.8.7"}},
		{name: "upstream busy", upstreamErr: errUpstreamBusy, method: http.MethodGet, target: pathInstruments + "/" + url.PathEscape(instrumentA), wantStatus: http.StatusServiceUnavailable, wantPublicText: "service busy, retry"},
		{name: "upstream timeout", upstreamErr: context.DeadlineExceeded, method: http.MethodGet, target: pathInstruments, wantStatus: http.StatusGatewayTimeout, wantPublicText: "upstream timeout"},
		{name: "ledger invariant", fixtures: conflictingDecimals, method: http.MethodGet, target: pathInstruments, wantStatus: http.StatusInternalServerError, wantPublicText: "internal error", hiddenTexts: []string{"disagree", "decimals:8"}},
		{name: "client error keeps its message", fixtures: registryTestFixtures(), method: http.MethodGet, target: pathInstruments + "/nope", wantStatus: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var srv *server
			if tc.upstreamErr != nil {
				srv = newTestServerWithClient(t, failingACS{err: tc.upstreamErr})
				useRegistryTestParties(srv)
				srv.registry = newRegistryConfig(registryTestGG, registryTestDisplay())
			} else {
				srv = newRegistryTestServer(t, tc.fixtures)
			}
			var logged strings.Builder
			srv.logger.SetOutput(&logged)
			recorder := serveRegistry(t, srv, tc.method, tc.target, tc.body)
			requireStatus(t, recorder, tc.wantStatus)
			errorText := requireRegistryErrorBody(t, recorder)
			requireNoStoreNoSniffHeaders(t, recorder)
			requestID := recorder.Header().Get("X-Request-Id")
			if tc.wantPublicText == "" {
				assert.NotContains(t, errorText, "request id")
				return
			}
			assert.Equal(t, "disclosure-service: "+tc.wantPublicText+"; request id "+requestID, errorText)
			assert.Contains(t, logged.String(), requestID)
			for _, hiddenText := range tc.hiddenTexts {
				assert.NotContains(t, recorder.Body.String(), hiddenText)
				assert.Contains(t, logged.String(), hiddenText)
			}
		})
	}
}

func requireNoStoreNoSniffHeaders(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
}

func allocationFactoryTestBody(choiceArguments string) string {
	return fmt.Sprintf(`{"choiceArguments":%s,"excludeDebugFields":false}`, choiceArguments)
}

func TestRegistryUpstreamListLimitIsAFixedPublicError(t *testing.T) {
	const listLimitBody = `{"code":"LIST_LIMIT","cause":"matching elements 1025 exceed the limit 1024"}`
	cases := []struct {
		name         string
		newServer    func(t *testing.T) *server
		wantTemplate string
	}{
		{"NttManager list above the participant limit", func(t *testing.T) *server {
			upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, listLimitBody, http.StatusRequestEntityTooLarge)
			})
			srv := newTestServerWithClient(t, newCachingACS(newTestHTTPACSClient(upstream.URL, nil), 5*time.Second))
			useRegistryTestParties(srv)
			srv.registry = newRegistryConfig(registryTestGG, registryTestDisplay())
			return srv
		}, registryQualifiedTemplate(tailNttManager)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := tc.newServer(t)
			logged := captureServerLog(srv)
			recorder := serveRegistry(t, srv, http.MethodGet, pathInstruments, "")
			requireStatus(t, recorder, http.StatusBadGateway)
			errorText := requireRegistryErrorBody(t, recorder)
			requireNoStoreNoSniffHeaders(t, recorder)
			assert.Equal(t, "disclosure-service: "+publicMessageUpstreamListLimit+"; request id "+recorder.Header().Get("X-Request-Id"), errorText)
			assert.NotContains(t, recorder.Body.String(), "1024")
			assert.Contains(t, logged.String(), tc.wantTemplate)
			assert.Contains(t, logged.String(), "http-list-max-elements-limit")
		})
	}
}

func TestHTTPACSClientAmountTotalsMapsUpstream413ToListLimit(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"code":"LIST_LIMIT"}`, http.StatusRequestEntityTooLarge)
	})
	query := amountTotalsQuery{
		contracts: activeContractsQuery{
			readerParties:  []partyID{registryTestGG},
			template:       registryQualifiedTemplate(tailCoin),
			activeAtOffset: 42,
			maxMatches:     maxMatchingContractsPerQuery,
			fieldMatches:   []createArgumentFieldMatch{partyFieldMatch(registryTestGG, "instrumentId", "admin")},
		},
		groupFieldPath:  []string{"instrumentId", "id"},
		amountFieldPath: []string{"amount"},
		maxGroups:       maxRegistryInstruments,
	}
	_, err := newTestHTTPACSClient(upstream.URL, nil).ActiveContractAmountTotals(context.Background(), query)
	require.ErrorIs(t, err, errUpstreamListLimitExceeded)
	assert.Contains(t, err.Error(), registryQualifiedTemplate(tailCoin))
	var httpError *upstreamHTTPError
	require.ErrorAs(t, err, &httpError)
	assert.Equal(t, http.StatusRequestEntityTooLarge, httpError.statusCode)
}

func TestRegistryErrorLogQuotesThePathAndTheError(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	logged := captureServerLog(srv)
	recorder := httptest.NewRecorder()
	srv.writeRegistryError(recorder, httptest.NewRequest(http.MethodGet, "/registry/x%0Aforged-line%0D%0Aanother", nil), errors.New("first\nsecond"))
	requireStatus(t, recorder, http.StatusInternalServerError)
	assert.Equal(t, 1, strings.Count(logged.String(), "\n"), logged.String())
	assert.Contains(t, logged.String(), `"/registry/x\nforged-line\r\nanother"`)
	assert.Contains(t, logged.String(), `"first\nsecond"`)
}

func TestRegistryReadsAsGuardianGovernanceOnly(t *testing.T) {
	directTransfer := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false)
	allocation := transferFactoryBody(allocationChoiceArguments(registryTestGG, registryTestAlice, registryTestGG, instrumentA), false)
	cases := []struct {
		name           string
		method, target string
		body           string
		wantReadTails  []string // every template the request reads
		wantScanTails  []string // the templates it reads by an active-contracts scan
	}{
		{"list", http.MethodGet, pathInstruments, "", []string{tailNttManager, tailCoin}, []string{tailNttManager, tailCoin}},
		{"instrument", http.MethodGet, pathInstruments + "/" + url.PathEscape(instrumentA), "", []string{tailNttManager, tailCoin}, []string{tailNttManager, tailCoin}},
		{"direct transfer", http.MethodPost, pathTransferFactory, directTransfer, []string{tailNttManager, tailCoinFactory, tailTransferPreapproval}, []string{tailNttManager, tailTransferPreapproval}},
		{"allocation factory", http.MethodPost, pathAllocationFactory, allocation, []string{tailNttManager, tailCoinFactory}, []string{tailNttManager}},
		{"accept", http.MethodPost, "/registry/transfer-instruction/v1/" + cidInstructionA + "/choice-contexts/accept", `{}`, []string{tailCoinTransferInstruction}, nil},
		{"execute transfer", http.MethodPost, "/registry/allocations/v1/" + cidAllocationA + "/choice-contexts/execute-transfer", `{}`, []string{tailCoinAllocation}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, recorder := newRecordingRegistryTestServer(t, registryTestFixtures())
			response := serveRegistry(t, srv, tc.method, tc.target, tc.body)
			requireStatus(t, response, http.StatusOK)
			readersByTail := recorder.readerPartiesByTail()
			readTails := make([]string, 0, len(readersByTail))
			for tail, readers := range readersByTail {
				readTails = append(readTails, tail)
				require.NotEmpty(t, readers)
				for _, reader := range readers {
					assert.Equal(t, partyID(registryTestGG), reader, "template %s read as %s", tail, reader)
				}
			}
			assert.ElementsMatch(t, tc.wantReadTails, readTails)
			scanTails := recorder.tails()
			slices.Sort(scanTails)
			scanTails = slices.Compact(scanTails)
			assert.ElementsMatch(t, tc.wantScanTails, scanTails)
		})
	}
}

func TestRegistryExactFiltersBeforeCap(t *testing.T) {
	strangerPreapprovals := func(count int, owner, instrumentID string) []string {
		contracts := make([]string, 0, count)
		for index := range count {
			contracts = append(contracts, registryContractJSON(tailTransferPreapproval, registryTestCid(fmt.Sprintf("pre-%s-%s-%d", owner, instrumentID, index)), "cHJl", preapprovalArg(owner, registryTestGG, instrumentID)))
		}
		return contracts
	}
	strangerManagers := func(count int, tokenConfig, guardianGovernance string) []string {
		contracts := make([]string, 0, count)
		for index := range count {
			managerArg := registryManagerArg(registryTestGG, fmt.Sprintf("other:%d", index), tokenConfig, 6, factoryTagBurnMint, cidCanonicalFactory)
			managerArg = strings.Replace(managerArg, `"guardianGovernance":"`+registryTestGG+`"`, `"guardianGovernance":"`+guardianGovernance+`"`, 1)
			contracts = append(contracts, registryContractJSON(tailNttManager, registryTestCid(fmt.Sprintf("mgr-%s-%s-%d", tokenConfig, guardianGovernance, index)), "bWdy", managerArg))
		}
		return contracts
	}
	baseManagers := func() []string {
		var managers []any
		require.NoError(t, json.Unmarshal([]byte(registryTestFixtures()[tailNttManager]), &managers))
		contracts := make([]string, 0, len(managers))
		for _, manager := range managers {
			encoded, err := json.Marshal(manager)
			require.NoError(t, err)
			contracts = append(contracts, string(encoded))
		}
		return contracts
	}
	transferToBob := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false)
	cases := []struct {
		name         string
		mutate       func(flowFixtures)
		method       string
		target, body string
		wantStatus   int
		wantKind     string
	}{
		{"1001 TransferPreapprovals of other owners, under the participant list limit", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = jsonArray(append(strangerPreapprovals(1001, registryTestAlice, instrumentA), registryContractJSON(tailTransferPreapproval, cidBobPreapprovalA, "cHJlQQ==", preapprovalArg(registryTestBob, registryTestGG, instrumentA)))...)
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusOK, transferKindDirect},
		{"1001 TransferPreapprovals of the receiver for another instrument, under the participant list limit", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = jsonArray(strangerPreapprovals(1001, registryTestBob, instrumentB)...)
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusOK, transferKindOffer},
		{"8 TransferPreapprovals of the receiver", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = jsonArray(strangerPreapprovals(8, registryTestBob, instrumentA)...)
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusOK, transferKindDirect},
		{"9 TransferPreapprovals of the receiver exceed the match cap", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = jsonArray(strangerPreapprovals(9, registryTestBob, instrumentA)...)
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusOK, transferKindOffer},
		{"1001 LockUnlock NttManagers of gg", func(fixtures flowFixtures) {
			fixtures[tailNttManager] = jsonArray(append(baseManagers(), strangerManagers(1001, tokenConfigLockUnlock, registryTestGG)...)...)
		}, http.MethodGet, pathInstruments, "", http.StatusOK, ""},
		{"1001 BurnMint NttManagers of another guardianGovernance", func(fixtures flowFixtures) {
			fixtures[tailNttManager] = jsonArray(append(baseManagers(), strangerManagers(1001, tokenConfigBurnMint, registryTestIssuer)...)...)
		}, http.MethodGet, pathInstruments, "", http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := registryTestFixtures()
			tc.mutate(fixtures)
			srv := newRegistryTestServer(t, fixtures)
			recorder := serveRegistry(t, srv, tc.method, tc.target, tc.body)
			requireStatus(t, recorder, tc.wantStatus)
			if tc.wantStatus != http.StatusOK {
				requireRegistryErrorBody(t, recorder)
				return
			}
			if tc.wantKind != "" {
				got := decodeRecorded[transferFactoryWithChoiceContext](t, recorder)
				assert.Equal(t, tc.wantKind, got.TransferKind)
				return
			}
			got := decodeRecorded[listInstrumentsResponse](t, recorder)
			assert.Len(t, got.Instruments, 3)
		})
	}
}

func TestRegistryReadsByContractIDWithTemplateChecks(t *testing.T) {
	cidArchivedInstruction := registryTestCid("instruction-archived")
	cidForeignInstruction := registryTestCid("instruction-foreign-package")
	cidArchivedAllocation := registryTestCid("allocation-archived")
	cidForeignAllocation := registryTestCid("allocation-foreign-package")
	fixtures := registryTestFixtures()
	fixtures[tailCoinTransferInstruction] = jsonArray(
		registryContractJSON(tailCoinTransferInstruction, cidInstructionA, "aW5zdEE=", registryInstructionArg(registryTestGG, instrumentA)),
		registryContractJSON(tailCoinTransferInstruction, cidInstructionIssuer, "aW5zdFQ=", registryInstructionArg(registryTestIssuer, instrumentThirdParty)),
		registryContractJSON(tailCoinTransferInstruction, cidArchivedInstruction, "aW5zdEE=", registryInstructionArg(registryTestGG, instrumentA)),
	)
	fixtures[tailCoinAllocation] = jsonArray(
		registryContractJSON(tailCoinAllocation, cidAllocationA, "YWxsb2NB", registryAllocationArg(registryTestGG, instrumentA)),
		registryContractJSON(tailCoinAllocation, cidAllocationIssuer, "YWxsb2NU", registryAllocationArg(registryTestIssuer, instrumentThirdParty)),
		registryContractJSON(tailCoinAllocation, cidArchivedAllocation, "YWxsb2NB", registryAllocationArg(registryTestGG, instrumentA)),
	)
	fixtures[archivedFixtureKey(cidArchivedInstruction)] = "archived"
	fixtures[archivedFixtureKey(cidArchivedAllocation)] = "archived"
	fixtures["foreign:by-id-only"] = jsonArray(
		registryContractJSONWithPackage(tailCoinTransferInstruction, cidForeignInstruction, "other-token", registryTestSynchronizerID, registryInstructionArg(registryTestGG, instrumentA)),
		registryContractJSONWithPackage(tailCoinAllocation, cidForeignAllocation, "other-token", registryTestSynchronizerID, registryAllocationArg(registryTestGG, instrumentA)),
	)
	instructionPath := func(contractID string) string {
		return "/registry/transfer-instruction/v1/" + contractID + "/choice-contexts/reject"
	}
	allocationPath := func(contractID string) string {
		return "/registry/allocations/v1/" + contractID + "/choice-contexts/withdraw"
	}
	cases := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{"active instruction", instructionPath(cidInstructionA), http.StatusOK},
		{"archived instruction", instructionPath(cidArchivedInstruction), http.StatusNotFound},
		{"instruction of another package", instructionPath(cidForeignInstruction), http.StatusNotFound},
		{"instruction id names an allocation", instructionPath(cidAllocationA), http.StatusNotFound},
		{"instruction id names the CoinFactory", instructionPath(cidCanonicalFactory), http.StatusNotFound},
		{"instruction of another admin", instructionPath(cidInstructionIssuer), http.StatusNotFound},
		{"instruction not visible", instructionPath(registryTestCid("absent")), http.StatusNotFound},
		{"active allocation", allocationPath(cidAllocationA), http.StatusOK},
		{"archived allocation", allocationPath(cidArchivedAllocation), http.StatusNotFound},
		{"allocation of another package", allocationPath(cidForeignAllocation), http.StatusNotFound},
		{"allocation id names an instruction", allocationPath(cidInstructionA), http.StatusNotFound},
		{"allocation of another admin", allocationPath(cidAllocationIssuer), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, counter := newCountedRegistryTestServer(t, fixtures)
			recorder := serveRegistry(t, srv, http.MethodPost, tc.target, `{}`)
			requireStatus(t, recorder, tc.wantStatus)
			assert.Equal(t, int64(1), counter.contractByIDCalls.Load())
			assert.Zero(t, counter.activeContractsCalls.Load())
			assert.Zero(t, counter.ledgerEndCalls.Load())
			if tc.wantStatus != http.StatusOK {
				requireRegistryErrorBody(t, recorder)
				return
			}
			assert.JSONEq(t, `{"choiceContextData":{"values":{}},"disclosedContracts":[]}`, recorder.Body.String())
		})
	}
}

func TestRegistryUpstreamTemplateChecks(t *testing.T) {
	transferToBob := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false)
	cases := []struct {
		name       string
		mutate     func(flowFixtures)
		method     string
		target     string
		body       string
		wantStatus int
	}{
		{"NttManager of another package", func(fixtures flowFixtures) {
			fixtures[tailNttManager] = jsonArray(registryContractJSONWithPackage(tailNttManager, registryTestCid("mgr-evil"), "evil-ntt", registryTestSynchronizerID, registryManagerArg(registryTestGG, instrumentA, tokenConfigBurnMint, 6, factoryTagBurnMint, cidCanonicalFactory)))
		}, http.MethodGet, pathInstruments, "", http.StatusBadGateway},
		{"Coin of another package", func(fixtures flowFixtures) {
			fixtures[tailCoin] = jsonArray(registryContractJSONWithPackage(tailCoin, registryTestCid("coin-evil"), "evil-token", registryTestSynchronizerID, registryCoinArg(registryTestAlice, registryTestGG, instrumentA, "1.0")))
		}, http.MethodGet, pathInstruments, "", http.StatusBadGateway},
		{"TransferPreapproval of another package", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = jsonArray(registryContractJSONWithPackage(tailTransferPreapproval, cidBobPreapprovalA, "evil-token", registryTestSynchronizerID, preapprovalArg(registryTestBob, registryTestGG, instrumentA)))
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusBadGateway},
		{"committed CoinFactory of another package", func(fixtures flowFixtures) {
			fixtures[tailCoinFactory] = jsonArray(registryContractJSONWithPackage(tailCoinFactory, cidCanonicalFactory, "evil-token", registryTestSynchronizerID, coinFactoryArg(registryTestGG)))
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusInternalServerError},
		{"committed CoinFactory archived", func(fixtures flowFixtures) {
			fixtures[archivedFixtureKey(cidCanonicalFactory)] = "archived"
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusInternalServerError},
		{"committed CoinFactory of another admin", func(fixtures flowFixtures) {
			fixtures[tailCoinFactory] = jsonArray(registryContractJSON(tailCoinFactory, cidCanonicalFactory, "ZmFjdG9yeQ==", coinFactoryArg(registryTestIssuer)))
		}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusInternalServerError},
		{"Coin with a non-positive amount", func(fixtures flowFixtures) {
			fixtures[tailCoin] = jsonArray(registryContractJSON(tailCoin, registryTestCid("coin-zero"), "Y29pbg==", registryCoinArg(registryTestAlice, registryTestGG, instrumentA, "0.0")))
		}, http.MethodGet, pathInstruments, "", http.StatusBadGateway},
		{"unmodified fixtures", func(flowFixtures) {}, http.MethodPost, pathTransferFactory, transferToBob, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := registryTestFixtures()
			tc.mutate(fixtures)
			srv := newRegistryTestServer(t, fixtures)
			recorder := serveRegistry(t, srv, tc.method, tc.target, tc.body)
			requireStatus(t, recorder, tc.wantStatus)
			if tc.wantStatus != http.StatusOK {
				errorText := requireRegistryErrorBody(t, recorder)
				assert.Contains(t, errorText, "; request id ")
			}
		})
	}
}

func TestCheckRegistryChoiceContext(t *testing.T) {
	disclosed := func(contractID, synchronizerID string, blobBytes int) registryDisclosedContract {
		return registryDisclosedContract{TemplateID: "pkg:" + tailCoinFactory, ContractID: contractID, CreatedEventBlob: strings.Repeat("b", blobBytes), SynchronizerID: synchronizerID}
	}
	fixedBytes := len("cid-1") + len("pkg:"+tailCoinFactory) + len("sync-1")
	cases := []struct {
		name       string
		context    registryChoiceContext
		wantStatus int // 0: the postcondition holds
	}{
		{"empty context", emptyChoiceContext(), 0},
		{"one disclosure", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "sync-1", 8)}}, 0},
		{"preapproval value and two disclosures", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{transferPreapprovalContextKey: {Tag: anyValueContractIDTag, Value: "cid-2"}}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "sync-1", 8), disclosed("cid-2", "sync-1", 8)}}, 0},
		{"exactly at the byte bound", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "sync-1", maxFlowResponseBytes-fixedBytes)}}, 0},
		{"one byte above the bound", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "sync-1", maxFlowResponseBytes-fixedBytes+1)}}, http.StatusBadGateway},
		{"two synchronizers", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "sync-1", 8), disclosed("cid-2", "sync-2", 8)}}, http.StatusInternalServerError},
		{"no synchronizer", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "", 8)}}, http.StatusInternalServerError},
		{"contract twice", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "sync-1", 8), disclosed("cid-1", "sync-1", 8)}}, http.StatusInternalServerError},
		{"no blob", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}, DisclosedContracts: []registryDisclosedContract{disclosed("cid-1", "sync-1", 0)}}, http.StatusInternalServerError},
		{"nil values", registryChoiceContext{DisclosedContracts: []registryDisclosedContract{}}, http.StatusInternalServerError},
		{"nil disclosures", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{}}}, http.StatusInternalServerError},
		{"value of another tag", registryChoiceContext{ChoiceContextData: registryChoiceContextData{Values: map[string]registryAnyValue{transferPreapprovalContextKey: {Tag: "AV_Text", Value: "x"}}}, DisclosedContracts: []registryDisclosedContract{}}, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkRegistryChoiceContext(tc.context)
			if tc.wantStatus == 0 {
				require.NoError(t, err)
				return
			}
			var flowErr *flowError
			require.ErrorAs(t, err, &flowErr)
			assert.Equal(t, tc.wantStatus, flowErr.status)
		})
	}
}

func TestRegistryChoiceContextBoundsEndToEnd(t *testing.T) {
	transferToBob := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), true)
	cases := []struct {
		name       string
		mutate     func(flowFixtures)
		wantStatus int
	}{
		{"factory and preapproval on one synchronizer", func(flowFixtures) {}, http.StatusOK},
		{"preapproval on another synchronizer gives offer", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = jsonArray(registryContractJSONWithPackage(tailTransferPreapproval, cidBobPreapprovalA, registryTestPackageNameOf(tailTransferPreapproval), "other-sync::1220", preapprovalArg(registryTestBob, registryTestGG, instrumentA)))
		}, http.StatusOK},
		{"factory blob above 4 MiB", func(fixtures flowFixtures) {
			fixtures[tailCoinFactory] = jsonArray(registryContractJSON(tailCoinFactory, cidCanonicalFactory, strings.Repeat("A", maxFlowResponseBytes+4), coinFactoryArg(registryTestGG)))
		}, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := registryTestFixtures()
			tc.mutate(fixtures)
			srv := newRegistryTestServer(t, fixtures)
			recorder := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, transferToBob)
			requireStatus(t, recorder, tc.wantStatus)
			if tc.wantStatus != http.StatusOK {
				errorText := requireRegistryErrorBody(t, recorder)
				assert.NotContains(t, errorText, "other-sync")
				assert.Less(t, recorder.Body.Len(), 1024)
			}
		})
	}
}

func TestDecodeActiveContractAmountTotals(t *testing.T) {
	template := registryQualifiedTemplate(tailCoin)
	coin := func(contractID, packageName, admin, instrumentID, amount string) string {
		return registryContractJSONWithPackage(tailCoin, contractID, packageName, registryTestSynchronizerID, registryCoinArg(registryTestAlice, admin, instrumentID, amount))
	}
	query := func(maxGroups int) amountTotalsQuery {
		return amountTotalsQuery{
			contracts: activeContractsQuery{
				readerParties:  []partyID{registryTestGG},
				template:       template,
				activeAtOffset: 42,
				fieldMatches:   []createArgumentFieldMatch{exactFieldMatch(registryTestGG, "instrumentId", "admin")},
			},
			groupFieldPath:  []string{"instrumentId", "id"},
			amountFieldPath: []string{"amount"},
			maxGroups:       maxGroups,
		}
	}
	cases := []struct {
		name       string
		body       string
		maxGroups  int
		wantTotals map[string]string
		wantErr    error
	}{
		{"empty", "[]", 2, map[string]string{}, nil},
		{"sums by instrument", jsonArray(coin("c1", "token-cip0056", registryTestGG, "A", "1.5"), coin("c2", "token-cip0056", registryTestGG, "A", "0.0000000001"), coin("c3", "token-cip0056", registryTestGG, "B", "2")), 2,
			map[string]string{"A": "1.5000000001", "B": "2.0000000000"}, nil},
		{"one contract seen by two readers counts once", jsonArray(coin("c1", "token-cip0056", registryTestGG, "A", "1"), coin("c1", "token-cip0056", registryTestGG, "A", "1")), 2, map[string]string{"A": "1.0000000000"}, nil},
		{"another admin is filtered out", jsonArray(coin("c1", "token-cip0056", registryTestIssuer, "A", "99"), coin("c2", "token-cip0056", registryTestGG, "A", "1")), 2, map[string]string{"A": "1.0000000000"}, nil},
		{"other contractEntry variants are skipped", `[{"contractEntry":{"JsEmpty":{}}}]`, 2, map[string]string{}, nil},
		{"groups at the cap", jsonArray(coin("c1", "token-cip0056", registryTestGG, "A", "1"), coin("c2", "token-cip0056", registryTestGG, "B", "1")), 2, map[string]string{"A": "1.0000000000", "B": "1.0000000000"}, nil},
		{"groups above the cap", jsonArray(coin("c1", "token-cip0056", registryTestGG, "A", "1"), coin("c2", "token-cip0056", registryTestGG, "B", "1"), coin("c3", "token-cip0056", registryTestGG, "C", "1")), 2, nil, errAmountTotalsGroupCapExceeded},
		{"zero amount", jsonArray(coin("c1", "token-cip0056", registryTestGG, "A", "0.0")), 2, nil, errUpstreamResponseMalformed},
		{"negative amount", jsonArray(coin("c1", "token-cip0056", registryTestGG, "A", "-1.0")), 2, nil, errUpstreamResponseMalformed},
		{"amount with 11 fraction digits", jsonArray(coin("c1", "token-cip0056", registryTestGG, "A", "0.00000000001")), 2, nil, errUpstreamResponseMalformed},
		{"another package", jsonArray(coin("c1", "evil-token", registryTestGG, "A", "1")), 2, nil, errUpstreamTemplateMismatch},
		{"not an array", `{}`, 2, nil, errUpstreamResponseMalformed},
		{"data after the array", `[] []`, 2, nil, errUpstreamResponseMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			totals, err := decodeActiveContractAmountTotals(strings.NewReader(tc.body), query(tc.maxGroups))
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			gotTotals := make(map[string]string, len(totals))
			for group, total := range totals {
				gotTotals[group] = formatDamlDecimalScaled(total)
			}
			assert.Equal(t, tc.wantTotals, gotTotals)
		})
	}
}

func TestCachingACSAmountTotalsReturnsCopies(t *testing.T) {
	inner := &countingACS{offset: 42}
	cache := newCachingACS(inner, time.Minute)
	query := amountTotalsQuery{
		contracts:       activeContractsQuery{readerParties: []partyID{registryTestGG}, template: registryQualifiedTemplate(tailCoin), activeAtOffset: 42},
		groupFieldPath:  []string{"instrumentId", "id"},
		amountFieldPath: []string{"amount"},
		maxGroups:       1,
	}
	first, err := cache.ActiveContractAmountTotals(context.Background(), query)
	require.NoError(t, err)
	for _, total := range first.totalsByGroup {
		total.Add(total, big.NewInt(1000))
	}
	second, err := cache.ActiveContractAmountTotals(context.Background(), query)
	require.NoError(t, err)
	require.Len(t, second.totalsByGroup, 1)
	for _, total := range second.totalsByGroup {
		assert.Equal(t, int64(1), total.Int64())
	}
	assert.Equal(t, int64(1), inner.totalsCalls.Load())
}
