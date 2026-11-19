package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveContractsFieldMatches(t *testing.T) {
	const preapproval = `{"owner":"alice::1","instrumentId":{"admin":"gg::1","id":"coin-1"},"emitterId":7,"managerAddress":"ABCD","nothing":null}`
	cases := []struct {
		name           string
		createArgument string
		fieldMatch     createArgumentFieldMatch
		want           bool
	}{
		{"exact top-level match kept", preapproval, createArgumentFieldMatch{fieldPath: []string{"owner"}, wantValues: []string{"alice::1"}}, true},
		{"top-level wrong by one character dropped", preapproval, createArgumentFieldMatch{fieldPath: []string{"owner"}, wantValues: []string{"alice::2"}}, false},
		{"nested exact match kept", preapproval, createArgumentFieldMatch{fieldPath: []string{"instrumentId", "id"}, wantValues: []string{"coin-1"}}, true},
		{"nested wrong by one character dropped", preapproval, createArgumentFieldMatch{fieldPath: []string{"instrumentId", "admin"}, wantValues: []string{"gg::2"}}, false},
		{"nested path missing dropped", preapproval, createArgumentFieldMatch{fieldPath: []string{"instrumentId", "issuer"}, wantValues: []string{"gg::1"}}, false},
		{"path through a string dropped", preapproval, createArgumentFieldMatch{fieldPath: []string{"owner", "id"}, wantValues: []string{"alice::1"}}, false},
		{"upper-case hex kept with compareHex", preapproval, createArgumentFieldMatch{fieldPath: []string{"managerAddress"}, wantValues: []string{"abcd"}, compareHex: true}, true},
		{"upper-case hex dropped without compareHex", preapproval, createArgumentFieldMatch{fieldPath: []string{"managerAddress"}, wantValues: []string{"abcd"}}, false},
		{"number compared by its JSON text", preapproval, createArgumentFieldMatch{fieldPath: []string{"emitterId"}, wantValues: []string{"7"}}, true},
		{"number text differs", preapproval, createArgumentFieldMatch{fieldPath: []string{"emitterId"}, wantValues: []string{"8"}}, false},
		{"numeric string compared by its value", `{"emitterId":"7"}`, createArgumentFieldMatch{fieldPath: []string{"emitterId"}, wantValues: []string{"7"}}, true},
		{"null dropped", preapproval, createArgumentFieldMatch{fieldPath: []string{"nothing"}, wantValues: []string{"null", ""}}, false},
		{"one of several wanted values kept", preapproval, createArgumentFieldMatch{fieldPath: []string{"owner"}, wantValues: []string{"bob::1", "alice::1"}}, true},
		{"empty want list dropped", preapproval, createArgumentFieldMatch{fieldPath: []string{"owner"}}, false},
		{"malformed createArgument dropped", `{"owner":`, createArgumentFieldMatch{fieldPath: []string{"owner"}, wantValues: []string{"alice::1"}}, false},
		{"createArgument not an object dropped", `["alice::1"]`, createArgumentFieldMatch{fieldPath: []string{"owner"}, wantValues: []string{"alice::1"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.fieldMatch.matchesCreateArgument(json.RawMessage(tc.createArgument)))
		})
	}
}

func TestActiveContractsQueryCacheKey(t *testing.T) {
	base := activeContractsQuery{
		readerParties:  []partyID{"gg::1"},
		template:       "#ntt:Wormhole.Ntt.Manager:NttManager",
		activeAtOffset: 42,
		fieldMatches:   []createArgumentFieldMatch{{fieldPath: []string{"owner"}, wantValues: []string{"a"}}},
		maxMatches:     maxMatchingContractsPerQuery,
	}
	variants := []struct {
		name   string
		mutate func(*activeContractsQuery)
	}{
		{"other party", func(query *activeContractsQuery) { query.readerParties = []partyID{"nttCustody::1"} }},
		{"other template", func(query *activeContractsQuery) { query.template = "#ntt:Wormhole.Ntt.Ledger:LockedLedger" }},
		{"other offset", func(query *activeContractsQuery) { query.activeAtOffset = 43 }},
		{"other wanted value", func(query *activeContractsQuery) {
			query.fieldMatches = []createArgumentFieldMatch{{fieldPath: []string{"owner"}, wantValues: []string{"b"}}}
		}},
		{"separator inside a value", func(query *activeContractsQuery) {
			query.fieldMatches = []createArgumentFieldMatch{{fieldPath: []string{"owner"}, wantValues: []string{"a\x00"}}}
		}},
		{"hex comparison", func(query *activeContractsQuery) {
			query.fieldMatches = []createArgumentFieldMatch{{fieldPath: []string{"owner"}, wantValues: []string{"a"}, compareHex: true}}
		}},
		{"no field matches", func(query *activeContractsQuery) { query.fieldMatches = nil }},
		{"other match cap", func(query *activeContractsQuery) { query.maxMatches = 1000 }},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			changed := base
			variant.mutate(&changed)
			assert.NotEqual(t, base.cacheKey(), changed.cacheKey())
		})
	}
	assert.Equal(t, base.cacheKey(), base.cacheKey())
}

// newStaticACSUpstream answers every active-contracts call with body.
func newStaticACSUpstream(t *testing.T, body string) *httpACSClient {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(upstream.Close)
	return newTestHTTPACSClient(upstream.URL, nil)
}

func managerQuery() activeContractsQuery {
	return activeContractsQuery{
		readerParties:  []partyID{"gg::1"},
		template:       "#ntt:Wormhole.Ntt.Manager:NttManager",
		activeAtOffset: 42,
		maxMatches:     maxMatchingContractsPerQuery,
	}
}

func TestActiveContractsRejectsMalformedStream(t *testing.T) {
	oneEntry := acsContractJSON("pkg:"+tailNttManager, "cid-1", "YmxvYg==", "sync1")
	cases := []struct {
		name string
		body string
	}{
		{"object, not an array", `{"contractEntry":{}}`},
		{"string, not an array", `"[]"`},
		{"truncated array", "[" + oneEntry},
		{"truncated entry", "[" + oneEntry[:len(oneEntry)/2]},
		{"trailing data after the array", "[" + oneEntry + "]x"},
		{"second array after the array", "[" + oneEntry + "][]"},
		{"empty body", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newStaticACSUpstream(t, tc.body).ActiveContracts(context.Background(), managerQuery())
			assert.ErrorIs(t, err, errUpstreamResponseMalformed)
		})
	}
	t.Run("well-formed array accepted", func(t *testing.T) {
		entries, err := newStaticACSUpstream(t, "["+oneEntry+"]\n").ActiveContracts(context.Background(), managerQuery())
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})
}

func TestActiveContractsStreamBounds(t *testing.T) {
	nonActiveEntry := `{"contractEntry":{}}`
	repeatEntries := func(count int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(nonActiveEntry+",", count), ",") + "]"
	}
	matchingEntry := func(index int) string {
		return acsContractJSONWithArg("pkg:"+tailNttManager, fmt.Sprintf("cid-%d", index), "YmxvYg==", "sync1", `{"owner":"alice::1"}`)
	}
	ownerQuery := managerQuery()
	ownerQuery.fieldMatches = []createArgumentFieldMatch{{fieldPath: []string{"owner"}, wantValues: []string{"alice::1"}}}

	cases := []struct {
		name     string
		body     string
		query    activeContractsQuery
		bodySize int64
		wantErr  error
		wantLen  int
	}{
		{"scan bound exactly", repeatEntries(maxScannedContractsPerQuery), managerQuery(), 0, nil, 0},
		{"scan bound plus one", repeatEntries(maxScannedContractsPerQuery + 1), managerQuery(), 0, errUpstreamScanBoundExceeded, 0},
		{"match cap exactly", "[" + matchingEntry(1) + "," + matchingEntry(2) + "]", withMaxMatches(ownerQuery, 2), 0, nil, 2},
		{"match cap plus one", "[" + matchingEntry(1) + "," + matchingEntry(2) + "," + matchingEntry(3) + "]", withMaxMatches(ownerQuery, 2), 0, errMatchCapExceeded, 0},
		{"body at the byte limit", "[" + matchingEntry(1) + "]", ownerQuery, int64(len("[" + matchingEntry(1) + "]")), nil, 1},
		{"body one byte over the limit", "[" + matchingEntry(1) + "]", ownerQuery, int64(len("["+matchingEntry(1)+"]")) - 1, errUpstreamResponseTooLarge, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newStaticACSUpstream(t, tc.body)
			if tc.bodySize != 0 {
				client.maxResponseBytes = tc.bodySize
			}
			entries, err := client.ActiveContracts(context.Background(), tc.query)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, entries, tc.wantLen)
		})
	}
	assert.Equal(t, int64(maxUpstreamResponseBytes), newTestHTTPACSClient("http://127.0.0.1:1", nil).maxResponseBytes)
}

func withMaxMatches(query activeContractsQuery, maxMatches int) activeContractsQuery {
	query.maxMatches = maxMatches
	return query
}

// strangerEntries renders count contracts of tail with createArgument, contract ids "stranger-<n>".
func strangerEntries(tail string, count int, createArgument func(index int) string) []string {
	entries := make([]string, 0, count)
	for index := range count {
		entries = append(entries, acsContractJSONWithArg("pkg:"+tail, fmt.Sprintf("stranger-%d", index), "c3RyYW5nZXI=", "sync1", createArgument(index)))
	}
	return entries
}

func TestFlowsIgnoreStrangerContractsBeyondCap(t *testing.T) {
	const strangerCount = 1001
	lockUnlock := newBaseFixtures(tokenConfigLockUnlock)
	burnMint := newBaseFixtures(tokenConfigBurnMint)
	realDeposit := acsContractJSONWithArg("pkg:"+tailDepositPreapproval, "dep-real", "ZGVw", "sync1", preapprovalArg(testRecipient, burnMint.instrumentAdmin, burnMint.instrumentID))
	strangerDeposits := strangerEntries(tailDepositPreapproval, strangerCount, func(index int) string {
		return preapprovalArg(testRecipient, "other-admin::1", fmt.Sprintf("other-instrument-%d", index))
	})
	strangerTransferPreapprovals := strangerEntries(tailTransferPreapproval, strangerCount, func(index int) string {
		return preapprovalArg(fmt.Sprintf("stranger::%d", index), lockUnlock.instrumentAdmin, lockUnlock.instrumentID)
	})
	strangerReplayNodes := strangerEntries(tailReplayNode, strangerCount, func(index int) string {
		return replayNodeArg(lockUnlock.gg, fmt.Sprintf("other-namespace-%d", index), "")
	})
	ownReplayNodes := make([]string, 0, 16)
	for _, prefix := range strings.Split("0123456789abcdef", "") {
		ownReplayNodes = append(ownReplayNodes, acsContractJSONWithArg("pkg:"+tailReplayNode, "node-"+prefix, "bm9kZQ==", "sync1", replayNodeArg(lockUnlock.gg, lockUnlock.namespace, prefix)))
	}
	nineCustodianPreapprovals := make([]string, 0, 9)
	for index := range 9 {
		nineCustodianPreapprovals = append(nineCustodianPreapprovals, acsContractJSONWithArg("pkg:"+tailTransferPreapproval, fmt.Sprintf("dup-%d", index), "cHJl", "sync1", preapprovalArg(lockUnlock.reserveCustodian(), lockUnlock.instrumentAdmin, lockUnlock.instrumentID)))
	}

	mintFixtures := func(deposits ...string) flowFixtures {
		return flowFixtures{
			tailNttManager:         jsonArray(burnMint.managerJSON()),
			tailCoreState:          jsonArray(burnMint.coreStateJSON()),
			tailReplayNode:         jsonArray(burnMint.replayNodeJSON("")),
			tailDepositPreapproval: jsonArray(deposits...),
			tailCoinFactory:        jsonArray(burnMint.factoryJSON()),
		}
	}
	transferFixtures := func(preapprovals ...string) flowFixtures {
		return flowFixtures{
			tailNttManager:          jsonArray(lockUnlock.managerJSON()),
			tailEmitter:             jsonArray(lockUnlock.emitterJSON()),
			tailCoreState:           jsonArray(lockUnlock.coreStateJSON()),
			tailLockedLedger:        jsonArray(lockUnlock.ledgerJSON(nil)),
			tailCoinFactory:         jsonArray(lockUnlock.factoryJSON()),
			tailTransferPreapproval: jsonArray(preapprovals...),
		}
	}
	releaseFixtures := flowFixtures{
		tailNttManager:   jsonArray(lockUnlock.managerJSON()),
		tailCoreState:    jsonArray(lockUnlock.coreStateJSON()),
		tailReplayNode:   jsonArray(append(append([]string{}, strangerReplayNodes...), ownReplayNodes...)...),
		tailLockedLedger: jsonArray(lockUnlock.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(lockUnlock.factoryJSON()),
	}
	mintQuery := "manager=" + burnMint.managerAddr + "&digest=" + burnMint.digest + "&recipient=" + testRecipient

	cases := []struct {
		name           string
		fixtures       flowFixtures
		flow, query    string
		wantStatus     int
		wantRole       string
		wantContractID string
	}{
		{"mint: 1001 stranger DepositPreapproval plus the real one", mintFixtures(append(append([]string{}, strangerDeposits...), realDeposit)...), "mint", mintQuery, http.StatusOK, roleDepositPreapproval, "dep-real"},
		{"transfer: 1001 stranger TransferPreapproval plus the real one", transferFixtures(append(append([]string{}, strangerTransferPreapprovals...), lockUnlock.transferPreapprovalJSON(lockUnlock.reserveCustodian()))...), "transfer", "manager=" + lockUnlock.managerAddr, http.StatusOK, roleCustodianTransferPreapproval, "pre-" + lockUnlock.reserveCustodian()},
		{"release: 1001 ReplayNode of other namespaces plus 16 of ours", releaseFixtures, "release", "manager=" + lockUnlock.managerAddr + "&digest=" + lockUnlock.digest, http.StatusOK, roleCoveringReplayNode, "node-c"},
		{"mint: two real DepositPreapproval is ambiguous", mintFixtures(realDeposit, strings.Replace(realDeposit, "dep-real", "dep-real-2", 1)), "mint", mintQuery, http.StatusInternalServerError, "", ""},
		{"transfer: nine real custodian preapprovals exceed the match cap", transferFixtures(nineCustodianPreapprovals...), "transfer", "manager=" + lockUnlock.managerAddr, http.StatusBadGateway, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newFlowTestServer(t, tc.fixtures)
			recorder := requestFlow(t, srv, tc.flow, tc.query)
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			if tc.wantRole == "" {
				return
			}
			response := decodeFlowResponse(t, recorder)
			found := false
			for _, disclosure := range response.Disclosures {
				if disclosure.Role == tc.wantRole {
					found = true
					assert.Equal(t, tc.wantContractID, disclosure.ContractID)
				}
				assert.NotContains(t, disclosure.ContractID, "stranger")
			}
			assert.True(t, found, "role %s not served", tc.wantRole)
		})
	}
}

func TestBoundedBodyReaderStaysOverLimit(t *testing.T) {
	cases := []struct {
		name           string
		source         string
		remainingBytes int64
		wantOverLimit  bool
	}{
		{"under the limit", "ab", 3, false},
		{"at the limit", "abc", 3, false},
		{"one byte over the limit", "abcd", 3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := &boundedBodyReader{source: strings.NewReader(tc.source), remainingBytes: tc.remainingBytes}
			_, firstErr := io.ReadAll(reader)
			var probe [1]byte
			_, secondErr := reader.Read(probe[:])
			assert.Equal(t, tc.wantOverLimit, reader.limitExceeded)
			if tc.wantOverLimit {
				assert.ErrorIs(t, firstErr, errUpstreamResponseTooLarge)
				assert.ErrorIs(t, secondErr, errUpstreamResponseTooLarge, "a read after the limit reports the limit again")
				return
			}
			assert.NoError(t, firstErr)
			assert.ErrorIs(t, secondErr, io.EOF)
		})
	}
}

// TestActiveContractsOverLimitAtArrayEnd: the cut falls on the closing "]", where Decoder.More drops the read error.
func TestActiveContractsOverLimitAtArrayEnd(t *testing.T) {
	oneEntry := acsContractJSON("pkg:"+tailNttManager, "cid-1", "YmxvYg==", "sync1")
	body := "[" + oneEntry + "]"
	for _, cutBytes := range []int64{1, 2, int64(len(oneEntry)) / 2} {
		t.Run(fmt.Sprintf("limit is body minus %d", cutBytes), func(t *testing.T) {
			_, err := decodeBoundedActiveContracts(&boundedBodyReader{source: strings.NewReader(body), remainingBytes: int64(len(body)) - cutBytes}, managerQuery())
			assert.ErrorIs(t, err, errUpstreamResponseTooLarge)
		})
	}
}
