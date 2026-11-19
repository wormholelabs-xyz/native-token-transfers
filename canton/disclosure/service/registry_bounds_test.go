package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests of the registry behavior at the participant list limit and the service read bounds,
// and of the review fixes N3 to N10 of PR #77.

// upstreamListAboveScanBound is an active-contracts body one element above the scan bound.
func upstreamListAboveScanBound() string {
	return "[" + strings.Repeat("{},", maxScannedContractsPerQuery) + "{}]"
}

// registryTestClock is a settable clock for the cache and the upstream read time.
type registryTestClock struct{ current time.Time }

func (clock *registryTestClock) now() time.Time { return clock.current }

// newCachedRegistryTestServer serves fixtures through a cache with ttl. One clock drives the
// cache and the upstream read time. The recorder counts the upstream reads of each template.
func newCachedRegistryTestServer(t *testing.T, fixtures flowFixtures, ttl time.Duration) (*server, *queriedTemplateRecorder, *registryTestClock) {
	t.Helper()
	srv, recorder := newRecordingRegistryTestServer(t, fixtures)
	clock := &registryTestClock{current: registryTestNow}
	httpClient, isHTTPClient := srv.acs.(*httpACSClient)
	require.True(t, isHTTPClient)
	httpClient.now = clock.now
	cache := newCachingACS(httpClient, ttl)
	cache.now = clock.now
	srv.acs = cache
	return srv, recorder, clock
}

func countTail(recorder *queriedTemplateRecorder, tail string) int {
	count := 0
	for _, recordedTail := range recorder.tails() {
		if recordedTail == tail {
			count++
		}
	}
	return count
}

func decodeJSONObject(t *testing.T, body string) map[string]any {
	t.Helper()
	var object map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &object), body)
	return object
}

func TestRegistryCoinTotalsAboveAReadBoundOmitTotalSupply(t *testing.T) {
	routes := []struct {
		name, target, pathTemplate string
		instrumentOf               func(t *testing.T, body string) map[string]any
	}{
		{"list", pathInstruments, pathInstruments, func(t *testing.T, body string) map[string]any {
			instruments, isArray := decodeJSONObject(t, body)["instruments"].([]any)
			require.True(t, isArray, body)
			require.Len(t, instruments, 3, body)
			instrument, isObject := instruments[0].(map[string]any)
			require.True(t, isObject, body)
			return instrument
		}},
		{"one instrument", pathInstruments + "/" + url.PathEscape(instrumentA), pathInstrumentTemplate, func(t *testing.T, body string) map[string]any {
			return decodeJSONObject(t, body)
		}},
	}
	cases := []struct {
		name          string
		mutate        func(flowFixtures)
		wantStatus    int
		wantSupply    bool
		wantLogged    string
		wantNotLogged string
	}{
		{"Coin totals within the bounds", func(flowFixtures) {}, http.StatusOK, true, "", "totalSupply omitted"},
		{"Coin list above the participant list limit (413)", func(fixtures flowFixtures) {
			fixtures[upstreamStatusFixtureKey(tailCoin)] = "413"
		}, http.StatusOK, false, "http-list-max-elements-limit", ""},
		{"Coin list above the scan bound", func(fixtures flowFixtures) {
			fixtures[tailCoin] = upstreamListAboveScanBound()
		}, http.StatusOK, false, "scanned-contract bound", ""},
		{"Coin read fails for another reason", func(fixtures flowFixtures) {
			fixtures[upstreamStatusFixtureKey(tailCoin)] = "500"
		}, http.StatusBadGateway, false, "", "totalSupply omitted"},
	}
	for _, route := range routes {
		for _, tc := range cases {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				fixtures := registryTestFixtures()
				tc.mutate(fixtures)
				srv, recorder, _ := newCachedRegistryTestServer(t, fixtures, 5*time.Second)
				logged := captureServerLog(srv)
				for range 3 {
					response := serveRegistry(t, srv, http.MethodGet, route.target, "")
					requireStatus(t, response, tc.wantStatus)
					requireConformsToSpec(t, response, specTokenMetadataV1, "GET", route.pathTemplate)
					if tc.wantStatus != http.StatusOK {
						requireRegistryErrorBody(t, response)
						continue
					}
					instrument := route.instrumentOf(t, response.Body.String())
					assert.Equal(t, instrumentA, instrument["id"])
					assert.Equal(t, "Token A", instrument["name"])
					assert.Equal(t, "TKA", instrument["symbol"])
					assert.EqualValues(t, 6, instrument["decimals"])
					assert.NotEmpty(t, instrument["supportedApis"])
					_, hasSupply := instrument["totalSupply"]
					_, hasSupplyAsOf := instrument["totalSupplyAsOf"]
					assert.Equal(t, tc.wantSupply, hasSupply, response.Body.String())
					assert.Equal(t, tc.wantSupply, hasSupplyAsOf, response.Body.String())
				}
				assert.Equal(t, 1, countTail(recorder, tailCoin), "Coin reads in one cache window")
				if tc.wantLogged != "" {
					assert.Contains(t, logged.String(), "totalSupply omitted")
					assert.Contains(t, logged.String(), tc.wantLogged)
					assert.Contains(t, logged.String(), registryQualifiedTemplate(tailCoin))
				}
				if tc.wantNotLogged != "" {
					assert.NotContains(t, logged.String(), tc.wantNotLogged)
				}
			})
		}
	}
}

func TestTransferFactoryPreapprovalReadAboveABoundGivesOffer(t *testing.T) {
	transferToBob := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false)
	receiverPreapprovals := func(count int, synchronizerID string) string {
		contracts := make([]string, 0, count)
		for index := range count {
			contracts = append(contracts, registryContractJSONWithPackage(tailTransferPreapproval, registryTestCid(fmt.Sprintf("pre-bob-%d", index)), registryTestPackageName, synchronizerID, preapprovalArg(registryTestBob, registryTestGG, instrumentA)))
		}
		return jsonArray(contracts...)
	}
	cases := []struct {
		name       string
		mutate     func(flowFixtures)
		wantStatus int
		wantKind   string
		wantLogged string
	}{
		{"one receiver preapproval", func(flowFixtures) {}, http.StatusOK, transferKindDirect, ""},
		{"8 receiver preapprovals", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = receiverPreapprovals(8, registryTestSynchronizerID)
		}, http.StatusOK, transferKindDirect, ""},
		{"no receiver preapproval", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = "[]"
		}, http.StatusOK, transferKindOffer, ""},
		{"TransferPreapproval list above the participant list limit (413)", func(fixtures flowFixtures) {
			fixtures[upstreamStatusFixtureKey(tailTransferPreapproval)] = "413"
		}, http.StatusOK, transferKindOffer, "http-list-max-elements-limit"},
		{"TransferPreapproval list above the scan bound", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = upstreamListAboveScanBound()
		}, http.StatusOK, transferKindOffer, "scanned-contract bound"},
		{"9 receiver preapprovals exceed the match cap", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = receiverPreapprovals(9, registryTestSynchronizerID)
		}, http.StatusOK, transferKindOffer, "matching contracts exceed the query cap"},
		{"receiver preapproval on another synchronizer", func(fixtures flowFixtures) {
			fixtures[tailTransferPreapproval] = receiverPreapprovals(1, "other-domain::1220sync")
		}, http.StatusOK, transferKindOffer, "other-domain::1220sync"},
		{"TransferPreapproval read fails for another reason", func(fixtures flowFixtures) {
			fixtures[upstreamStatusFixtureKey(tailTransferPreapproval)] = "500"
		}, http.StatusBadGateway, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := registryTestFixtures()
			tc.mutate(fixtures)
			srv, recorder, _ := newCachedRegistryTestServer(t, fixtures, 5*time.Second)
			logged := captureServerLog(srv)
			for range 3 {
				response := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, transferToBob)
				requireStatus(t, response, tc.wantStatus)
				requireConformsToSpec(t, response, specTransferInstructionV1, "POST", pathTransferFactory)
				if tc.wantStatus != http.StatusOK {
					requireRegistryErrorBody(t, response)
					continue
				}
				got := decodeRecorded[transferFactoryWithChoiceContext](t, response)
				assert.Equal(t, tc.wantKind, got.TransferKind)
				assert.Equal(t, cidCanonicalFactory, got.FactoryID)
				if tc.wantKind != transferKindOffer {
					continue
				}
				assert.Empty(t, got.ChoiceContext.ChoiceContextData.Values)
				require.Len(t, got.ChoiceContext.DisclosedContracts, 1)
				assert.Equal(t, cidCanonicalFactory, got.ChoiceContext.DisclosedContracts[0].ContractID)
			}
			assert.Equal(t, 1, countTail(recorder, tailTransferPreapproval), "TransferPreapproval reads in one cache window")
			if tc.wantLogged != "" {
				assert.Contains(t, logged.String(), "transferKind offer")
				assert.Contains(t, logged.String(), tc.wantLogged)
			} else {
				assert.NotContains(t, logged.String(), "transferKind offer")
			}
		})
	}

	t.Run("a self transfer does not read preapprovals", func(t *testing.T) {
		fixtures := registryTestFixtures()
		fixtures[upstreamStatusFixtureKey(tailTransferPreapproval)] = "413"
		srv, recorder, _ := newCachedRegistryTestServer(t, fixtures, 5*time.Second)
		selfTransfer := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestBob, registryTestBob, "1", registryTestGG, instrumentA), false)
		response := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, selfTransfer)
		requireStatus(t, response, http.StatusOK)
		assert.Equal(t, transferKindSelf, decodeRecorded[transferFactoryWithChoiceContext](t, response).TransferKind)
		assert.Zero(t, countTail(recorder, tailTransferPreapproval))
	})
}

func TestTotalSupplyAsOfIsTheReadTime(t *testing.T) {
	srv, recorder, clock := newCachedRegistryTestServer(t, registryTestFixtures(), 5*time.Second)
	readTime := registryTestNow
	cases := []struct {
		name       string
		advance    time.Duration
		wantAsOf   time.Time
		wantCoinRd int
	}{
		{"first read", 0, readTime, 1},
		{"cached totals keep their read time", 3 * time.Second, readTime, 1},
		{"a new read after the cache window", 3 * time.Second, readTime.Add(6 * time.Second), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock.current = clock.current.Add(tc.advance)
			response := serveRegistry(t, srv, http.MethodGet, pathInstruments+"/"+url.PathEscape(instrumentA), "")
			requireStatus(t, response, http.StatusOK)
			got := decodeRecorded[registryInstrumentResponse](t, response)
			assert.Equal(t, tc.wantAsOf.UTC().Format(time.RFC3339Nano), got.TotalSupplyAsOf)
			assert.Equal(t, tc.wantCoinRd, countTail(recorder, tailCoin))
		})
	}
}

func TestRegistryBadRequestEchoesABoundedClientValue(t *testing.T) {
	longValue := strings.Repeat("7", 60_000)
	longWord := strings.Repeat("w", 60_000)
	cases := []struct {
		name, method, target, body string
		wantStatus                 int
	}{
		{"amount that is not a number", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, longWord, registryTestGG, instrumentA), false), http.StatusBadRequest},
		{"amount with a long digit string", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, longValue+"x", registryTestGG, instrumentA), false), http.StatusBadRequest},
		{"body that is not JSON", http.MethodPost, pathTransferFactory, `{"choiceArguments":` + longWord + `}`, http.StatusBadRequest},
		{"field of the wrong type", http.MethodPost, pathTransferFactory, `{"choiceArguments":{"expectedAdmin":["` + longWord + `"]}}`, http.StatusBadRequest},
		{"choice body field of the wrong type", http.MethodPost, "/registry/transfer-instruction/v1/" + cidInstructionA + "/choice-contexts/accept", `{"meta":{"` + longWord + `":7}}`, http.StatusBadRequest},
		{"long expectedAdmin", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(longWord, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false), http.StatusBadRequest},
		{"long instrument id", http.MethodPost, pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, longWord), false), http.StatusBadRequest},
		{"long unknown path", http.MethodGet, "/registry/" + longWord[:4000], "", http.StatusNotFound},
		{"long pageSize", http.MethodGet, pathInstruments + "?pageSize=" + longWord[:900], "", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newRegistryTestServer(t, registryTestFixtures())
			response := serveRegistry(t, srv, tc.method, tc.target, tc.body)
			requireStatus(t, response, tc.wantStatus)
			errorText := requireRegistryErrorBody(t, response)
			assert.LessOrEqual(t, len(errorText), 256, errorText)
			assert.NotContains(t, errorText, longValue[:maxEchoedClientValueBytes+1])
			assert.NotContains(t, errorText, longWord[:maxEchoedClientValueBytes+1])
		})
	}
}

func TestRegistryContractNotFoundHasOneText(t *testing.T) {
	cidArchived := registryTestCid("instruction-archived")
	fixtures := registryTestFixtures()
	fixtures[tailCoinTransferInstruction] = jsonArray(
		registryContractJSON(tailCoinTransferInstruction, cidInstructionA, "aW5zdEE=", registryInstructionArg(registryTestGG, instrumentA)),
		registryContractJSON(tailCoinTransferInstruction, cidInstructionIssuer, "aW5zdFQ=", registryInstructionArg(registryTestIssuer, instrumentThirdParty)),
		registryContractJSON(tailCoinTransferInstruction, cidArchived, "YXJj", registryInstructionArg(registryTestGG, instrumentA)),
	)
	fixtures[archivedFixtureKey(cidArchived)] = ""
	cases := []struct {
		name       string
		contractID string
	}{
		{"unknown id", registryTestCid("unknown")},
		{"archived instruction", cidArchived},
		{"id of an NttManager", registryTestCid("mgr-A")},
		{"instruction of another admin", cidInstructionIssuer},
	}
	errorTexts := make(map[string]string, len(cases))
	for _, tc := range cases {
		srv := newRegistryTestServer(t, fixtures)
		response := serveRegistry(t, srv, http.MethodPost, "/registry/transfer-instruction/v1/"+tc.contractID+"/choice-contexts/accept", `{}`)
		requireStatus(t, response, http.StatusNotFound)
		errorText := requireRegistryErrorBody(t, response)
		assert.NotContains(t, errorText, tc.contractID, tc.name)
		errorTexts[tc.name] = errorText
	}
	for _, tc := range cases {
		assert.Equal(t, errorTexts[cases[0].name], errorTexts[tc.name], tc.name)
	}
}

func TestRegistryParsesTheRequestInstrumentAdmin(t *testing.T) {
	cases := []struct {
		name, target, body string
	}{
		{"transfer instrument admin without a fingerprint", pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", "gg", instrumentA), false)},
		{"transfer instrument admin in uppercase hex", pathTransferFactory, transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", strings.ToUpper(registryTestGG), instrumentA), false)},
		{"allocation instrument admin without a fingerprint", pathAllocationFactory, allocationFactoryTestBody(allocationChoiceArguments(registryTestGG, registryTestAlice, "gg::1220aa", instrumentA))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, counter := newCountedRegistryTestServer(t, registryTestFixtures())
			response := serveRegistry(t, srv, http.MethodPost, tc.target, tc.body)
			requireStatus(t, response, http.StatusBadRequest)
			assert.Contains(t, requireRegistryErrorBody(t, response), "instrumentId.admin must be a party id")
			assert.Zero(t, counter.totalCalls())
		})
	}
}

// contractByIDMissACS answers each read by contract id with "not found" and counts the calls.
type contractByIDMissACS struct {
	countingACS
}

func (miss *contractByIDMissACS) ActiveContractByID(context.Context, []partyID, contractID) (acsEntry, bool, error) {
	miss.byIDCalls.Add(1)
	return acsEntry{}, false, nil
}

func TestCachingACSContractByIDMissesAreBriefAndCapped(t *testing.T) {
	inner := &contractByIDMissACS{}
	cache := newCachingACS(inner, 5*time.Second)
	clock := &registryTestClock{current: registryTestNow}
	cache.now = clock.now
	readers := []partyID{registryTestGG}
	missedID := contractID(registryTestCid("missing"))

	t.Run("a miss is cached for at most maxNegativeCacheTTL", func(t *testing.T) {
		_, found, err := cache.ActiveContractByID(context.Background(), readers, missedID)
		require.NoError(t, err)
		require.False(t, found)
		clock.current = clock.current.Add(maxNegativeCacheTTL / 2)
		_, _, err = cache.ActiveContractByID(context.Background(), readers, missedID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), inner.byIDCalls.Load())
		clock.current = clock.current.Add(maxNegativeCacheTTL)
		_, _, err = cache.ActiveContractByID(context.Background(), readers, missedID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), inner.byIDCalls.Load())
	})

	t.Run("misses fill at most maxNegativeCacheEntries", func(t *testing.T) {
		for index := range maxCacheEntries {
			_, _, err := cache.ActiveContractByID(context.Background(), readers, contractID(registryTestCid(fmt.Sprintf("flood-%d", index))))
			require.NoError(t, err)
		}
		cache.mu.Lock()
		missEntries := 0
		for _, cached := range cache.entries {
			if cached.isMiss {
				missEntries++
			}
		}
		cache.mu.Unlock()
		assert.LessOrEqual(t, missEntries, maxNegativeCacheEntries)
		assert.Positive(t, missEntries)
	})

	t.Run("a hit keeps the full ttl", func(t *testing.T) {
		hitInner := &countingACS{offset: 42}
		hitCache := newCachingACS(hitInner, 5*time.Second)
		hitCache.now = clock.now
		_, found, err := hitCache.ActiveContractByID(context.Background(), readers, missedID)
		require.NoError(t, err)
		require.True(t, found)
		clock.current = clock.current.Add(4 * time.Second)
		_, _, err = hitCache.ActiveContractByID(context.Background(), readers, missedID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), hitInner.byIDCalls.Load())
	})
}

func TestNewHTTPServerBoundsTheRequestBodyTime(t *testing.T) {
	httpServer := newHTTPServer(http.NotFoundHandler())
	assert.Equal(t, 10*time.Second, httpServer.ReadTimeout)
	assert.Equal(t, 10*time.Second, httpServer.ReadHeaderTimeout)
}

func TestParseFlags_RegistryNeedsTheCache(t *testing.T) {
	base := []string{"--json-api", "http://127.0.0.1:6975", "--guardian-governance-party", registryTestGG, "--ntt-custody-party", registryTestNttCustody}
	cases := []struct {
		name     string
		extra    []string
		wantText string
	}{
		{"registry with the default cache", []string{"--registry-instruments-file", "i.json"}, ""},
		{"registry with the minimum cache", []string{"--registry-instruments-file", "i.json", "--cache-ttl", minRegistryCacheTTL.String()}, ""},
		{"registry without the cache", []string{"--registry-instruments-file", "i.json", "--cache-ttl", "0"}, "--cache-ttl"},
		{"registry below the minimum cache", []string{"--registry-instruments-file", "i.json", "--cache-ttl", "500ms"}, "--cache-ttl"},
		{"no registry without the cache", []string{"--cache-ttl", "0"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFlags(append(append([]string(nil), base...), tc.extra...))
			if tc.wantText == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantText)
		})
	}
}

func TestListInstrumentsPagesThroughLongInstrumentIDs(t *testing.T) {
	longIDs := []string{
		"wormhole-ntt:" + strings.Repeat(":", maxRegistryIdentifierLength-len("wormhole-ntt:")),
		"wormhole-ntt;" + strings.Repeat("%", maxRegistryIdentifierLength-len("wormhole-ntt;")),
		"wormhole-nttz" + strings.Repeat("&", maxRegistryIdentifierLength-len("wormhole-nttz")),
	}
	fixtures := registryTestFixtures()
	managers := make([]string, 0, len(longIDs))
	display := registryTestDisplay()
	for index, instrumentID := range longIDs {
		require.Len(t, instrumentID, maxRegistryIdentifierLength)
		managers = append(managers, registryContractJSON(tailNttManager, registryTestCid(fmt.Sprintf("mgr-long-%d", index)), "bWdy", registryManagerArg(registryTestGG, instrumentID, tokenConfigBurnMint, 6, factoryTagBurnMint, cidCanonicalFactory)))
		display[instrumentID] = registryInstrumentDisplay{name: "Long", symbol: "LNG"}
	}
	fixtures[tailNttManager] = jsonArray(managers...)
	srv := newRegistryTestServer(t, fixtures)
	srv.registry = newRegistryConfig(registryTestGG, display)

	var pagedIDs []string
	pageToken := ""
	for range len(longIDs) + 1 {
		query := url.Values{registryParamPageSize: {"1"}}
		if pageToken != "" {
			query.Set(registryParamPageToken, pageToken)
		}
		response := serveRegistry(t, srv, http.MethodGet, pathInstruments+"?"+query.Encode(), "")
		requireStatus(t, response, http.StatusOK)
		page := decodeRecorded[listInstrumentsResponse](t, response)
		for _, instrument := range page.Instruments {
			pagedIDs = append(pagedIDs, instrument.ID)
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	assert.Equal(t, longIDs, pagedIDs)
}

func TestParseListInstrumentsRequestPageToken(t *testing.T) {
	encodedPageToken := func(instrumentID string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(instrumentID))
	}
	cases := []struct {
		name          string
		rawQuery      string
		wantPageToken string
		wantStatus    int
	}{
		{"encoded instrument id", "pageToken=" + encodedPageToken(instrumentA), instrumentA, 0},
		{"encoded id at the length limit", "pageToken=" + encodedPageToken(strings.Repeat("%", maxRegistryIdentifierLength)), strings.Repeat("%", maxRegistryIdentifierLength), 0},
		{"encoded id above the length limit", "pageToken=" + encodedPageToken(strings.Repeat("a", maxRegistryIdentifierLength+1)), "", http.StatusBadRequest},
		{"token that is not base64url", "pageToken=" + url.QueryEscape("wormhole-ntt:A"), "", http.StatusBadRequest},
		{"padded base64", "pageToken=" + base64.URLEncoding.EncodeToString([]byte("ab")), "", http.StatusBadRequest},
		{"empty token", "pageToken=", "", http.StatusBadRequest},
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
			assert.Equal(t, tc.wantPageToken, request.afterInstrumentID)
		})
	}
}

func TestUnconfiguredInstrumentIsLoggedToTheServerLogger(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	logged := captureServerLog(srv)
	requireStatus(t, serveRegistry(t, srv, http.MethodGet, pathInstruments, ""), http.StatusOK)
	requireStatus(t, serveRegistry(t, srv, http.MethodGet, pathInstruments, ""), http.StatusOK)
	assert.Equal(t, 1, strings.Count(logged.String(), instrumentUnconfigured), logged.String())
}
