package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const (
	registryTestGG             = "gg" + testPartyFingerprintSuffix
	registryTestNttCustody     = "nttCustody" + testPartyFingerprintSuffix
	registryTestIssuer         = "issuer" + testPartyFingerprintSuffix
	registryTestAlice          = "alice" + testPartyFingerprintSuffix
	registryTestBob            = "bob" + testPartyFingerprintSuffix
	registryTestExecutor       = "venue" + testPartyFingerprintSuffix
	registryTestSynchronizerID = "global-domain::1220sync"
	registryTestCreatedAt      = "2026-11-18T09:00:00.123456Z"
	registryTestPackageName    = "token-cip0056"
)

// Instruments. A, B, C and U are gg burn/mint instruments; the display config lists A, B and C.
const (
	instrumentA            = "wormhole-ntt:A"
	instrumentB            = "wormhole-ntt:B"
	instrumentC            = "wormhole-ntt:C"
	instrumentUnconfigured = "wormhole-ntt:U"
	instrumentLockUnlock   = "wormhole-ntt:L"
	instrumentThirdParty   = "issuer:T"
)

var registryTestNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

// registryTestCid is a hex contract id unique per label.
func registryTestCid(label string) string {
	return "00" + fmt.Sprintf("%x", label) + strings.Repeat("0", 64)
}

var (
	cidCanonicalFactory  = registryTestCid("factory-gg")
	cidIssuerFactory     = registryTestCid("factory-issuer")
	cidBobPreapprovalA   = registryTestCid("preapproval-bob-A")
	cidBobPreapprovalB   = registryTestCid("preapproval-bob-B")
	cidInstructionA      = registryTestCid("instruction-A")
	cidInstructionIssuer = registryTestCid("instruction-issuer")
	cidAllocationA       = registryTestCid("allocation-A")
	cidAllocationIssuer  = registryTestCid("allocation-issuer")
)

func registryContractJSON(tail, contractID, blob, createArgument string) string {
	return fmt.Sprintf(
		`{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":%q,"templateId":%q,"createdEventBlob":%q,"createArgument":%s,"createdAt":%q,"packageName":%q},"synchronizerId":%q}}}`,
		contractID, "pkgid:"+tail, blob, createArgument, registryTestCreatedAt, registryTestPackageNameOf(tail), registryTestSynchronizerID,
	)
}

// registryTestPackageNameOf is the package name of a registry template tail.
func registryTestPackageNameOf(tail string) string {
	return strings.TrimPrefix(strings.SplitN(registryTemplateByTail[tail], ":", 2)[0], "#")
}

func registryManagerArg(instrumentAdmin, instrumentID, tokenConfig string, tokenDecimals int, factoryTag, factoryCid string) string {
	return fmt.Sprintf(
		`{"admin":%q,"guardianGovernance":%q,"nttCustody":%q,"reserveCustodian":%q,"managerAddress":{"tag":"NttManagerAddress","value":%q},"managerId":"`+testNttManagerID+`","instrumentId":{"admin":%q,"id":%q},"tokenConfig":%q,"tokenDecimals":"%d","factory":{"tag":%q,"value":%q},"paused":false}`,
		registryTestGG, registryTestGG, registryTestNttCustody, registryTestNttCustody,
		fmt.Sprintf("%x", sha256.Sum256([]byte(instrumentID))), instrumentAdmin, instrumentID, tokenConfig, tokenDecimals, factoryTag, factoryCid,
	)
}

func registryCoinArg(owner, instrumentAdmin, instrumentID, amount string) string {
	return fmt.Sprintf(`{"instrumentId":{"admin":%q,"id":%q},"owner":%q,"amount":%q}`, instrumentAdmin, instrumentID, owner, amount)
}

func registryInstructionArg(instrumentAdmin, instrumentID string) string {
	return fmt.Sprintf(`{"instrumentId":{"admin":%q,"id":%q},"sender":%q,"receiver":%q,"amount":"1.0000000000","requestedAt":"2026-11-18T09:00:00Z","executeBefore":"2026-11-19T09:00:00Z"}`,
		instrumentAdmin, instrumentID, registryTestAlice, registryTestBob)
}

func registryAllocationArg(instrumentAdmin, instrumentID string) string {
	return fmt.Sprintf(`{"allocation":{"settlement":{"executor":%q,"settlementRef":{"id":"trade-1","cid":null},"requestedAt":"2026-11-18T09:00:00Z","allocateBefore":"2026-11-19T09:00:00Z","settleBefore":"2026-11-20T09:00:00Z","meta":{"values":{}}},"transferLegId":"leg-1","transferLeg":{"sender":%q,"receiver":%q,"amount":"1.0000000000","instrumentId":{"admin":%q,"id":%q},"meta":{"values":{}}}}}`,
		registryTestExecutor, registryTestAlice, registryTestBob, instrumentAdmin, instrumentID)
}

func registryTestFixtures() flowFixtures {
	return flowFixtures{
		tailNttManager: jsonArray(
			registryContractJSON(tailNttManager, registryTestCid("mgr-A"), "bWdyQQ==", registryManagerArg(registryTestGG, instrumentA, tokenConfigBurnMint, 6, factoryTagBurnMint, cidCanonicalFactory)),
			registryContractJSON(tailNttManager, registryTestCid("mgr-C"), "bWdyQw==", registryManagerArg(registryTestGG, instrumentC, tokenConfigBurnMint, 10, factoryTagBurnMint, cidCanonicalFactory)),
			registryContractJSON(tailNttManager, registryTestCid("mgr-B"), "bWdyQg==", registryManagerArg(registryTestGG, instrumentB, tokenConfigBurnMint, 8, factoryTagBurnMint, cidCanonicalFactory)),
			registryContractJSON(tailNttManager, registryTestCid("mgr-U"), "bWdyVQ==", registryManagerArg(registryTestGG, instrumentUnconfigured, tokenConfigBurnMint, 0, factoryTagBurnMint, cidCanonicalFactory)),
			registryContractJSON(tailNttManager, registryTestCid("mgr-L"), "bWdyTA==", registryManagerArg(registryTestGG, instrumentLockUnlock, tokenConfigLockUnlock, 6, factoryTagTransfer, cidCanonicalFactory)),
			registryContractJSON(tailNttManager, registryTestCid("mgr-T"), "bWdyVA==", registryManagerArg(registryTestIssuer, instrumentThirdParty, tokenConfigBurnMint, 6, factoryTagBurnMint, cidIssuerFactory)),
		),
		tailCoinFactory: jsonArray(
			registryContractJSON(tailCoinFactory, cidCanonicalFactory, "ZmFjdG9yeQ==", coinFactoryArg(registryTestGG)),
			registryContractJSON(tailCoinFactory, cidIssuerFactory, "aXNzdWVy", coinFactoryArg(registryTestIssuer)),
		),
		tailCoin: jsonArray(
			registryContractJSON(tailCoin, registryTestCid("coin-1"), "Y29pbjE=", registryCoinArg(registryTestAlice, registryTestGG, instrumentA, "10.5000000000")),
			registryContractJSON(tailCoin, registryTestCid("coin-2"), "Y29pbjI=", registryCoinArg(registryTestBob, registryTestGG, instrumentA, "2.2500000001")),
			registryContractJSON(tailCoin, registryTestCid("coin-3"), "Y29pbjM=", registryCoinArg(registryTestBob, registryTestGG, instrumentB, "0.0000000001")),
			registryContractJSON(tailCoin, registryTestCid("coin-4"), "Y29pbjQ=", registryCoinArg(registryTestBob, registryTestIssuer, instrumentA, "99.0")),
		),
		tailTransferPreapproval: jsonArray(
			registryContractJSON(tailTransferPreapproval, cidBobPreapprovalA, "cHJlQQ==", preapprovalArg(registryTestBob, registryTestGG, instrumentA)),
			registryContractJSON(tailTransferPreapproval, cidBobPreapprovalB, "cHJlQg==", preapprovalArg(registryTestBob, registryTestGG, instrumentB)),
		),
		tailCoinTransferInstruction: jsonArray(
			registryContractJSON(tailCoinTransferInstruction, cidInstructionA, "aW5zdEE=", registryInstructionArg(registryTestGG, instrumentA)),
			registryContractJSON(tailCoinTransferInstruction, cidInstructionIssuer, "aW5zdFQ=", registryInstructionArg(registryTestIssuer, instrumentThirdParty)),
		),
		tailCoinAllocation: jsonArray(
			registryContractJSON(tailCoinAllocation, cidAllocationA, "YWxsb2NB", registryAllocationArg(registryTestGG, instrumentA)),
			registryContractJSON(tailCoinAllocation, cidAllocationIssuer, "YWxsb2NU", registryAllocationArg(registryTestIssuer, instrumentThirdParty)),
		),
	}
}

func registryTestDisplay() map[string]registryInstrumentDisplay {
	return map[string]registryInstrumentDisplay{
		instrumentA:          {name: "Token A", symbol: "TKA"},
		instrumentB:          {name: "Token B", symbol: "TKB"},
		instrumentC:          {name: "Token C", symbol: "TKC"},
		instrumentLockUnlock: {name: "Locked", symbol: "LCK"},
		instrumentThirdParty: {name: "Third", symbol: "THD"},
	}
}

func newRegistryTestServer(t *testing.T, fixtures flowFixtures) *server {
	t.Helper()
	srv := newFlowTestServer(t, fixtures)
	useRegistryTestParties(srv)
	srv.registry = newRegistryConfig(registryTestGG, registryTestDisplay())
	useRegistryTestReadClock(srv)
	return srv
}

// useRegistryTestReadClock stamps each upstream totals read with registryTestNow.
func useRegistryTestReadClock(srv *server) {
	httpClient, isHTTPClient := srv.acs.(*httpACSClient)
	if !isHTTPClient {
		panic("useRegistryTestReadClock: the test server upstream is not an httpACSClient")
	}
	httpClient.now = func() time.Time { return registryTestNow }
}

// useRegistryTestParties makes registryTestGG the guardianGovernance reader party.
func useRegistryTestParties(srv *server) {
	opts := *srv.opts
	opts.readerParties = readerPartiesByRole{guardianGovernance: registryTestGG, nttCustody: registryTestNttCustody}
	srv.opts = &opts
}

func serveRegistry(t *testing.T, srv *server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	srv.ServeHTTP(recorder, request)
	return recorder
}

func decodeRecorded[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	decoder := json.NewDecoder(strings.NewReader(recorder.Body.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		t.Fatalf("decode %T: %v: %s", out, err, recorder.Body.String())
	}
	return out
}

func requireStatus(t *testing.T, recorder *httptest.ResponseRecorder, want int) {
	t.Helper()
	if recorder.Code != want {
		t.Fatalf("status %d, want %d: %s", recorder.Code, want, recorder.Body.String())
	}
}

func transferChoiceArguments(expectedAdmin, sender, receiver, amount, instrumentAdmin, instrumentID string) string {
	return fmt.Sprintf(`{"expectedAdmin":%q,"transfer":{"sender":%q,"receiver":%q,"amount":%q,"instrumentId":{"admin":%q,"id":%q},"requestedAt":"2026-11-18T09:00:00Z","executeBefore":"2026-11-19T09:00:00Z","inputHoldingCids":[%q],"meta":{"values":{}}},"extraArgs":{"context":{"values":{}},"meta":{"values":{}}}}`,
		expectedAdmin, sender, receiver, amount, instrumentAdmin, instrumentID, registryTestCid("coin-1"))
}

func transferFactoryBody(choiceArguments string, excludeDebugFields bool) string {
	return fmt.Sprintf(`{"choiceArguments":%s,"excludeDebugFields":%t}`, choiceArguments, excludeDebugFields)
}

func allocationChoiceArguments(expectedAdmin, sender, instrumentAdmin, instrumentID string) string {
	return fmt.Sprintf(`{"expectedAdmin":%q,"allocation":{"settlement":{"executor":%q,"settlementRef":{"id":"trade-1","cid":null},"requestedAt":"2026-11-18T09:00:00Z","allocateBefore":"2026-11-19T09:00:00Z","settleBefore":"2026-11-20T09:00:00Z","meta":{"values":{}}},"transferLegId":"leg-1","transferLeg":{"sender":%q,"receiver":%q,"amount":"1.5","instrumentId":{"admin":%q,"id":%q},"meta":{"values":{}}}},"requestedAt":"2026-11-18T09:00:00Z","inputHoldingCids":[],"extraArgs":{"context":{"values":{}},"meta":{"values":{}}}}`,
		expectedAdmin, registryTestExecutor, sender, registryTestBob, instrumentAdmin, instrumentID)
}

const (
	pathTransferFactory             = "/registry/transfer-instruction/v1/transfer-factory"
	pathTransferInstructionTemplate = "/registry/transfer-instruction/v1/{transferInstructionId}/choice-contexts/"
	pathAllocationFactory           = "/registry/allocation-instruction/v1/allocation-factory"
	pathAllocationTemplate          = "/registry/allocations/v1/{allocationId}/choice-contexts/"
	pathRegistryInfo                = "/registry/metadata/v1/info"
	pathInstruments                 = "/registry/metadata/v1/instruments"
	pathInstrumentTemplate          = "/registry/metadata/v1/instruments/{instrumentId}"
)

// ---------------------------------------------------------------------------
// Instruments file
// ---------------------------------------------------------------------------

func TestParseRegistryInstrumentsFile(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		want     map[string]registryInstrumentDisplay
		wantText string
	}{
		{
			name: "valid",
			raw:  `{"wormhole-ntt:A":{"name":"Token A","symbol":"TKA"},"wormhole-ntt:B":{"symbol":"TKB","name":"Token B"}}`,
			want: map[string]registryInstrumentDisplay{
				"wormhole-ntt:A": {name: "Token A", symbol: "TKA"},
				"wormhole-ntt:B": {name: "Token B", symbol: "TKB"},
			},
		},
		{name: "duplicate instrument id", raw: `{"x":{"name":"a","symbol":"b"},"x":{"name":"c","symbol":"d"}}`, wantText: "duplicate instrument id"},
		{name: "duplicate field", raw: `{"x":{"name":"a","name":"c","symbol":"b"}}`, wantText: "duplicate field"},
		{name: "unknown field", raw: `{"x":{"name":"a","symbol":"b","decimals":6}}`, wantText: "unknown field"},
		{name: "missing name", raw: `{"x":{"symbol":"b"}}`, wantText: "missing field \"name\""},
		{name: "missing symbol", raw: `{"x":{"name":"a"}}`, wantText: "missing field \"symbol\""},
		{name: "empty name", raw: `{"x":{"name":"","symbol":"b"}}`, wantText: "must not be empty"},
		{name: "non-string symbol", raw: `{"x":{"name":"a","symbol":7}}`, wantText: "must be a string"},
		{name: "empty instrument id", raw: `{"":{"name":"a","symbol":"b"}}`, wantText: "instrument id must not be empty"},
		{name: "entry not an object", raw: `{"x":["a"]}`, wantText: "must be an object"},
		{name: "top level array", raw: `[{"name":"a","symbol":"b"}]`, wantText: "must be an object"},
		{name: "no instruments", raw: `{}`, wantText: "no instruments"},
		{name: "trailing data", raw: `{"x":{"name":"a","symbol":"b"}} {}`, wantText: "trailing data"},
		{name: "truncated", raw: `{"x":{"name":"a","symbol":"b"}`, wantText: "instruments file"},
		{name: "too large", raw: `{"x":{"name":"` + strings.Repeat("n", maxInstrumentsFileBytes) + `","symbol":"b"}}`, wantText: "exceeds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRegistryInstrumentsFile([]byte(tc.raw))
			if tc.wantText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantText) {
					t.Fatalf("error %v, want one containing %q", err, tc.wantText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseFlags_Registry(t *testing.T) {
	base := []string{"--json-api", "http://127.0.0.1:6975", "--guardian-governance-party", registryTestGG, "--ntt-custody-party", registryTestNttCustody}
	cases := []struct {
		name     string
		extra    []string
		wantPath string
		wantText string
	}{
		{name: "disabled", extra: nil},
		{name: "enabled", extra: []string{"--registry-instruments-file", "i.json"}, wantPath: "i.json"},
		{name: "removed admin flag", extra: []string{"--registry-admin-party", registryTestGG, "--registry-instruments-file", "i.json"}, wantText: "registry-admin-party"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseFlags(append(append([]string(nil), base...), tc.extra...))
			if tc.wantText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantText) {
					t.Fatalf("error %v, want one containing %q", err, tc.wantText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opts.registryInstrumentsPath != tc.wantPath {
				t.Fatalf("registryInstrumentsPath %q, want %q", opts.registryInstrumentsPath, tc.wantPath)
			}
		})
	}
}

func TestLoadRegistryConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instruments.json")
	if err := os.WriteFile(path, []byte(`{"wormhole-ntt:A":{"name":"Token A","symbol":"TKA"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		path     string
		wantText string
	}{
		{name: "valid file", path: path},
		{name: "missing file", path: filepath.Join(dir, "absent.json"), wantText: "read instruments file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &options{readerParties: readerPartiesByRole{guardianGovernance: registryTestGG, nttCustody: registryTestNttCustody}, registryInstrumentsPath: tc.path}
			config, err := loadRegistryConfig(opts)
			if tc.wantText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantText) {
					t.Fatalf("error %v, want one containing %q", err, tc.wantText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.adminParty != registryTestGG || len(config.displayByInstrumentID) != 1 {
				t.Fatalf("config %+v", config)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Daml Decimal arithmetic
// ---------------------------------------------------------------------------

func TestDamlDecimal(t *testing.T) {
	cases := []struct {
		text     string
		want     string
		wantText string
	}{
		{text: "0", want: "0.0000000000"},
		{text: "10.5", want: "10.5000000000"},
		{text: "10.5000000000", want: "10.5000000000"},
		{text: "0.0000000001", want: "0.0000000001"},
		{text: "-2.25", want: "-2.2500000000"},
		{text: "9999999999999999999999999999.9999999999", want: "9999999999999999999999999999.9999999999"},
		{text: "10000000000000000000000000000", wantText: "more than 38 digits"},
		{text: "1.00000000001", wantText: "more than 10"},
		{text: "1e3", wantText: "not a Daml Decimal"},
		{text: ".5", wantText: "not a Daml Decimal"},
		{text: "5.", wantText: "not a Daml Decimal"},
		{text: "", wantText: "not a Daml Decimal"},
		{text: "+1", wantText: "not a Daml Decimal"},
		{text: "100000000000000000000000000000", wantText: "more than 38 digits"},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			scaled, err := parseDamlDecimalScaled(tc.text)
			if tc.wantText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantText) {
					t.Fatalf("error %v, want one containing %q", err, tc.wantText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := formatDamlDecimalScaled(scaled); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// token-metadata-v1
// ---------------------------------------------------------------------------

func TestRegistryInfo(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	recorder := serveRegistry(t, srv, http.MethodGet, pathRegistryInfo, "")
	requireStatus(t, recorder, http.StatusOK)
	requireConformsToSpec(t, recorder, specTokenMetadataV1, "GET", pathRegistryInfo)
	got := decodeRecorded[registryInfoResponse](t, recorder)
	if got.AdminID != registryTestGG {
		t.Fatalf("adminId %q", got.AdminID)
	}
	want := map[string]int{
		"splice-api-token-metadata-v1":               1,
		"splice-api-token-transfer-instruction-v1":   1,
		"splice-api-token-allocation-instruction-v1": 1,
		"splice-api-token-allocation-v1":             1,
	}
	if !reflect.DeepEqual(got.SupportedApis, want) {
		t.Fatalf("supportedApis %v, want %v", got.SupportedApis, want)
	}
}

func TestListInstruments_Content(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	recorder := serveRegistry(t, srv, http.MethodGet, pathInstruments, "")
	requireStatus(t, recorder, http.StatusOK)
	requireConformsToSpec(t, recorder, specTokenMetadataV1, "GET", pathInstruments)
	got := decodeRecorded[listInstrumentsResponse](t, recorder)
	if got.NextPageToken != "" {
		t.Fatalf("nextPageToken %q on the only page", got.NextPageToken)
	}
	want := []registryInstrumentResponse{
		{ID: instrumentA, Name: "Token A", Symbol: "TKA", TotalSupply: "12.7500000001", TotalSupplyAsOf: "2026-11-18T12:00:00Z", Decimals: 6, SupportedApis: registrySupportedApis()},
		{ID: instrumentB, Name: "Token B", Symbol: "TKB", TotalSupply: "0.0000000001", TotalSupplyAsOf: "2026-11-18T12:00:00Z", Decimals: 8, SupportedApis: registrySupportedApis()},
		{ID: instrumentC, Name: "Token C", Symbol: "TKC", TotalSupply: "0.0000000000", TotalSupplyAsOf: "2026-11-18T12:00:00Z", Decimals: 10, SupportedApis: registrySupportedApis()},
	}
	if !reflect.DeepEqual(got.Instruments, want) {
		t.Fatalf("instruments\n got %+v\nwant %+v", got.Instruments, want)
	}
}

// instrumentPageToken is the pageToken of the page after instrumentID.
func instrumentPageToken(instrumentID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(instrumentID))
}

func TestListInstruments_Pagination(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	cases := []struct {
		name          string
		query         string
		wantIDs       []string
		wantNextToken string
	}{
		{name: "default page size", query: "", wantIDs: []string{instrumentA, instrumentB, instrumentC}},
		{name: "first page", query: "pageSize=1", wantIDs: []string{instrumentA}, wantNextToken: instrumentPageToken(instrumentA)},
		{name: "middle page", query: "pageSize=1&pageToken=" + instrumentPageToken(instrumentA), wantIDs: []string{instrumentB}, wantNextToken: instrumentPageToken(instrumentB)},
		{name: "last page exact", query: "pageSize=1&pageToken=" + instrumentPageToken(instrumentB), wantIDs: []string{instrumentC}},
		{name: "page size equals the count", query: "pageSize=3", wantIDs: []string{instrumentA, instrumentB, instrumentC}},
		{name: "short last page", query: "pageSize=2&pageToken=" + instrumentPageToken(instrumentA), wantIDs: []string{instrumentB, instrumentC}},
		{name: "token after the last id", query: "pageToken=" + instrumentPageToken(instrumentC), wantIDs: []string{}},
		{name: "token between ids", query: "pageToken=" + instrumentPageToken(instrumentA+"0"), wantIDs: []string{instrumentB, instrumentC}},
		{name: "zero page size clamps to 1", query: "pageSize=0", wantIDs: []string{instrumentA}, wantNextToken: instrumentPageToken(instrumentA)},
		{name: "negative page size clamps to 1", query: "pageSize=-5", wantIDs: []string{instrumentA}, wantNextToken: instrumentPageToken(instrumentA)},
		{name: "huge page size clamps to the max", query: "pageSize=2147483648", wantIDs: []string{instrumentA, instrumentB, instrumentC}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serveRegistry(t, srv, http.MethodGet, pathInstruments+"?"+tc.query, "")
			requireStatus(t, recorder, http.StatusOK)
			requireConformsToSpec(t, recorder, specTokenMetadataV1, "GET", pathInstruments)
			got := decodeRecorded[listInstrumentsResponse](t, recorder)
			ids := make([]string, 0, len(got.Instruments))
			for _, instrument := range got.Instruments {
				ids = append(ids, instrument.ID)
			}
			if !reflect.DeepEqual(ids, tc.wantIDs) {
				t.Fatalf("ids %v, want %v", ids, tc.wantIDs)
			}
			if got.NextPageToken != tc.wantNextToken {
				t.Fatalf("nextPageToken %q, want %q", got.NextPageToken, tc.wantNextToken)
			}
		})
	}
}

func TestGetInstrument(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	cases := []struct {
		name         string
		instrumentID string
		wantStatus   int
		wantSupply   string
	}{
		{name: "configured", instrumentID: instrumentA, wantStatus: http.StatusOK, wantSupply: "12.7500000001"},
		{name: "no display config", instrumentID: instrumentUnconfigured, wantStatus: http.StatusNotFound},
		{name: "unknown", instrumentID: "wormhole-ntt:nope", wantStatus: http.StatusNotFound},
		{name: "lock/unlock deployment", instrumentID: instrumentLockUnlock, wantStatus: http.StatusNotFound},
		{name: "admin is not gg", instrumentID: instrumentThirdParty, wantStatus: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serveRegistry(t, srv, http.MethodGet, pathInstruments+"/"+url.PathEscape(tc.instrumentID), "")
			requireStatus(t, recorder, tc.wantStatus)
			requireConformsToSpec(t, recorder, specTokenMetadataV1, "GET", pathInstrumentTemplate)
			if tc.wantStatus != http.StatusOK {
				return
			}
			got := decodeRecorded[registryInstrumentResponse](t, recorder)
			if got.ID != tc.instrumentID || got.TotalSupply != tc.wantSupply {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestListInstruments_ConflictingDecimalsIs500(t *testing.T) {
	fixtures := registryTestFixtures()
	fixtures[tailNttManager] = jsonArray(
		registryContractJSON(tailNttManager, registryTestCid("mgr-A"), "AA==", registryManagerArg(registryTestGG, instrumentA, tokenConfigBurnMint, 6, factoryTagBurnMint, cidCanonicalFactory)),
		registryContractJSON(tailNttManager, registryTestCid("mgr-A2"), "AA==", registryManagerArg(registryTestGG, instrumentA, tokenConfigBurnMint, 8, factoryTagBurnMint, cidCanonicalFactory)),
	)
	srv := newRegistryTestServer(t, fixtures)
	recorder := serveRegistry(t, srv, http.MethodGet, pathInstruments, "")
	requireStatus(t, recorder, http.StatusInternalServerError)
	requireConformsToSpec(t, recorder, specTokenMetadataV1, "GET", pathInstruments)
}

func TestListInstruments_DecimalsOutOfRangeIs500(t *testing.T) {
	fixtures := registryTestFixtures()
	fixtures[tailNttManager] = jsonArray(
		registryContractJSON(tailNttManager, registryTestCid("mgr-A"), "AA==", registryManagerArg(registryTestGG, instrumentA, tokenConfigBurnMint, 11, factoryTagBurnMint, cidCanonicalFactory)),
	)
	srv := newRegistryTestServer(t, fixtures)
	recorder := serveRegistry(t, srv, http.MethodGet, pathInstruments, "")
	requireStatus(t, recorder, http.StatusInternalServerError)
}

// ---------------------------------------------------------------------------
// transfer-instruction-v1: transfer-factory
// ---------------------------------------------------------------------------

func TestTransferFactory_TransferKind(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	factoryDisclosure := registryDisclosedContract{
		TemplateID: "pkgid:" + tailCoinFactory, ContractID: cidCanonicalFactory, CreatedEventBlob: "ZmFjdG9yeQ==", SynchronizerID: registryTestSynchronizerID,
	}
	cases := []struct {
		name            string
		sender          string
		receiver        string
		instrumentID    string
		wantKind        string
		wantContext     map[string]registryAnyValue
		wantDisclosures []registryDisclosedContract
	}{
		{
			name: "self", sender: registryTestAlice, receiver: registryTestAlice, instrumentID: instrumentA,
			wantKind: transferKindSelf, wantContext: map[string]registryAnyValue{}, wantDisclosures: []registryDisclosedContract{factoryDisclosure},
		},
		{
			name: "self with a receiver preapproval", sender: registryTestBob, receiver: registryTestBob, instrumentID: instrumentA,
			wantKind: transferKindSelf, wantContext: map[string]registryAnyValue{}, wantDisclosures: []registryDisclosedContract{factoryDisclosure},
		},
		{
			name: "direct", sender: registryTestAlice, receiver: registryTestBob, instrumentID: instrumentA,
			wantKind: transferKindDirect,
			wantContext: map[string]registryAnyValue{
				"token-cip0056.transferPreapprovalCid": {Tag: "AV_ContractId", Value: cidBobPreapprovalA},
			},
			wantDisclosures: []registryDisclosedContract{
				factoryDisclosure,
				{TemplateID: "pkgid:" + tailTransferPreapproval, ContractID: cidBobPreapprovalA, CreatedEventBlob: "cHJlQQ==", SynchronizerID: registryTestSynchronizerID},
			},
		},
		{
			name: "direct picks the preapproval of the requested instrument", sender: registryTestAlice, receiver: registryTestBob, instrumentID: instrumentB,
			wantKind: transferKindDirect,
			wantContext: map[string]registryAnyValue{
				"token-cip0056.transferPreapprovalCid": {Tag: "AV_ContractId", Value: cidBobPreapprovalB},
			},
			wantDisclosures: []registryDisclosedContract{
				factoryDisclosure,
				{TemplateID: "pkgid:" + tailTransferPreapproval, ContractID: cidBobPreapprovalB, CreatedEventBlob: "cHJlQg==", SynchronizerID: registryTestSynchronizerID},
			},
		},
		{
			name: "offer", sender: registryTestBob, receiver: registryTestAlice, instrumentID: instrumentA,
			wantKind: transferKindOffer, wantContext: map[string]registryAnyValue{}, wantDisclosures: []registryDisclosedContract{factoryDisclosure},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := transferFactoryBody(transferChoiceArguments(registryTestGG, tc.sender, tc.receiver, "1.5", registryTestGG, tc.instrumentID), true)
			recorder := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, body)
			requireStatus(t, recorder, http.StatusOK)
			requireConformsToSpec(t, recorder, specTransferInstructionV1, "POST", pathTransferFactory)
			got := decodeRecorded[transferFactoryWithChoiceContext](t, recorder)
			if got.FactoryID != cidCanonicalFactory {
				t.Fatalf("factoryId %q", got.FactoryID)
			}
			if got.TransferKind != tc.wantKind {
				t.Fatalf("transferKind %q, want %q", got.TransferKind, tc.wantKind)
			}
			if !reflect.DeepEqual(got.ChoiceContext.ChoiceContextData.Values, tc.wantContext) {
				t.Fatalf("choiceContextData %+v, want %+v", got.ChoiceContext.ChoiceContextData.Values, tc.wantContext)
			}
			if !reflect.DeepEqual(got.ChoiceContext.DisclosedContracts, tc.wantDisclosures) {
				t.Fatalf("disclosedContracts\n got %+v\nwant %+v", got.ChoiceContext.DisclosedContracts, tc.wantDisclosures)
			}
		})
	}
}

// TestTransferFactory_ContextDataWireEncoding pins the exact JSON a wallet copies into
// extraArgs.context: the Daml-JSON encoding of ChoiceContext with an AV_ContractId value.
func TestTransferFactory_ContextDataWireEncoding(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	body := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), true)
	recorder := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, body)
	requireStatus(t, recorder, http.StatusOK)
	var raw struct {
		ChoiceContext struct {
			ChoiceContextData json.RawMessage `json:"choiceContextData"`
		} `json:"choiceContext"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	want := `{"values":{"token-cip0056.transferPreapprovalCid":{"tag":"AV_ContractId","value":"` + cidBobPreapprovalA + `"}}}`
	if string(raw.ChoiceContext.ChoiceContextData) != want {
		t.Fatalf("choiceContextData %s, want %s", raw.ChoiceContext.ChoiceContextData, want)
	}
}

func TestTransferFactory_Errors(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	valid := transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1.5", registryTestGG, instrumentA)
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantText   string
	}{
		{name: "unknown instrument", body: transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, "wormhole-ntt:nope"), false), wantStatus: 404, wantText: "not a registry instrument"},
		{name: "lock/unlock instrument", body: transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentLockUnlock), false), wantStatus: 404, wantText: "not a registry instrument"},
		{name: "instrument admin is not gg", body: transferFactoryBody(transferChoiceArguments(registryTestIssuer, registryTestAlice, registryTestBob, "1", registryTestIssuer, instrumentThirdParty), false), wantStatus: 404, wantText: "is not this registry's admin"},
		{name: "expectedAdmin mismatch", body: transferFactoryBody(transferChoiceArguments(registryTestIssuer, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false), wantStatus: 400, wantText: "expectedAdmin"},
		{name: "not JSON", body: `{"choiceArguments":`, wantStatus: 400},
		{name: "empty body", body: ``, wantStatus: 400},
		{name: "JSON null", body: `null`, wantStatus: 400},
		{name: "top level array", body: `[]`, wantStatus: 400},
		{name: "missing choiceArguments", body: `{"excludeDebugFields":true}`, wantStatus: 400, wantText: "choiceArguments"},
		{name: "choiceArguments not an object", body: `{"choiceArguments":"x"}`, wantStatus: 400, wantText: "choiceArguments"},
		{name: "excludeDebugFields not a bool", body: `{"choiceArguments":` + valid + `,"excludeDebugFields":"yes"}`, wantStatus: 400},
		{name: "missing transfer", body: `{"choiceArguments":{"expectedAdmin":"` + registryTestGG + `"}}`, wantStatus: 400, wantText: "transfer"},
		{name: "missing sender", body: transferFactoryBody(transferChoiceArguments(registryTestGG, "", registryTestBob, "1", registryTestGG, instrumentA), false), wantStatus: 400, wantText: "sender"},
		{name: "missing instrument id", body: transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, ""), false), wantStatus: 400, wantText: "instrumentId.id"},
		{name: "amount not a Decimal", body: transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1e3", registryTestGG, instrumentA), false), wantStatus: 400, wantText: "amount"},
		{name: "amount zero", body: transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "0.0", registryTestGG, instrumentA), false), wantStatus: 400, wantText: "positive"},
		{name: "trailing data", body: transferFactoryBody(valid, false) + "{}", wantStatus: 400, wantText: "trailing"},
		{name: "body too large", body: `{"choiceArguments":` + valid + `,"pad":"` + strings.Repeat("x", maxRegistryRequestBodyBytes) + `"}`, wantStatus: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, tc.body)
			requireStatus(t, recorder, tc.wantStatus)
			requireConformsToSpec(t, recorder, specTransferInstructionV1, "POST", pathTransferFactory)
			if !strings.Contains(recorder.Body.String(), tc.wantText) {
				t.Fatalf("body %s does not mention %q", recorder.Body.String(), tc.wantText)
			}
		})
	}
}

func TestTransferFactory_ExcludeDebugFields(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	cases := []struct {
		name      string
		body      string
		wantDebug bool
	}{
		{name: "excluded", body: transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), true), wantDebug: false},
		{name: "included", body: transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false), wantDebug: true},
		{name: "default is included", body: `{"choiceArguments":` + transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA) + `}`, wantDebug: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, tc.body)
			requireStatus(t, recorder, http.StatusOK)
			requireConformsToSpec(t, recorder, specTransferInstructionV1, "POST", pathTransferFactory)
			got := decodeRecorded[transferFactoryWithChoiceContext](t, recorder)
			if len(got.ChoiceContext.DisclosedContracts) != 2 {
				t.Fatalf("disclosedContracts %+v", got.ChoiceContext.DisclosedContracts)
			}
			for _, disclosed := range got.ChoiceContext.DisclosedContracts {
				hasDebug := disclosed.DebugPackageName != "" || len(disclosed.DebugPayload) > 0 || disclosed.DebugCreatedAt != ""
				if hasDebug != tc.wantDebug {
					t.Fatalf("debug fields present %t, want %t: %+v", hasDebug, tc.wantDebug, disclosed)
				}
				if !tc.wantDebug {
					continue
				}
				if disclosed.DebugPackageName != registryTestPackageName || disclosed.DebugCreatedAt != registryTestCreatedAt {
					t.Fatalf("debug fields %+v", disclosed)
				}
				if !strings.HasPrefix(string(disclosed.DebugPayload), "{") {
					t.Fatalf("debugPayload %s is not an object", disclosed.DebugPayload)
				}
			}
			if tc.wantDebug && strings.Contains(recorder.Body.String(), `"debugPayload":{"admin":"`+registryTestGG+`"}`) == false {
				t.Fatalf("factory debugPayload missing: %s", recorder.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// transfer-instruction-v1: choice contexts
// ---------------------------------------------------------------------------

func TestTransferInstructionChoiceContext(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	cases := []struct {
		name          string
		instructionID string
		choice        string
		body          string
		wantStatus    int
	}{
		{name: "accept", instructionID: cidInstructionA, choice: "accept", body: `{}`, wantStatus: 200},
		{name: "reject", instructionID: cidInstructionA, choice: "reject", body: `{"meta":{"k":"v"}}`, wantStatus: 200},
		{name: "withdraw", instructionID: cidInstructionA, choice: "withdraw", body: `{"excludeDebugFields":true}`, wantStatus: 200},
		{name: "a CoinFactory id is not a CoinTransferInstruction", instructionID: cidCanonicalFactory, choice: "accept", body: `{}`, wantStatus: 404},
		{name: "an allocation id is not a CoinTransferInstruction", instructionID: cidAllocationA, choice: "accept", body: `{}`, wantStatus: 404},
		{name: "instruction of another admin", instructionID: cidInstructionIssuer, choice: "accept", body: `{}`, wantStatus: 404},
		{name: "malformed id", instructionID: "not-hex", choice: "accept", body: `{}`, wantStatus: 400},
		{name: "malformed body", instructionID: cidInstructionA, choice: "accept", body: `{"meta":{"k":1}}`, wantStatus: 400},
		{name: "empty body", instructionID: cidInstructionA, choice: "accept", body: ``, wantStatus: 400},
		{name: "array body", instructionID: cidInstructionA, choice: "accept", body: `[]`, wantStatus: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := "/registry/transfer-instruction/v1/" + tc.instructionID + "/choice-contexts/" + tc.choice
			recorder := serveRegistry(t, srv, http.MethodPost, target, tc.body)
			requireStatus(t, recorder, tc.wantStatus)
			requireConformsToSpec(t, recorder, specTransferInstructionV1, "POST", pathTransferInstructionTemplate+tc.choice)
			if tc.wantStatus != http.StatusOK {
				return
			}
			got := decodeRecorded[registryChoiceContext](t, recorder)
			if len(got.ChoiceContextData.Values) != 0 || len(got.DisclosedContracts) != 0 {
				t.Fatalf("choice context %+v, want empty context and no disclosures", got)
			}
		})
	}
}

func TestTransferInstructionChoiceContext_UnknownChoiceIs404(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	recorder := serveRegistry(t, srv, http.MethodPost, "/registry/transfer-instruction/v1/"+cidInstructionA+"/choice-contexts/update", `{}`)
	requireStatus(t, recorder, http.StatusNotFound)
	decodeRecorded[registryErrorResponse](t, recorder)
}

// ---------------------------------------------------------------------------
// allocation-instruction-v1 and allocation-v1
// ---------------------------------------------------------------------------

func TestAllocationFactory(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "gg instrument", body: transferFactoryBody(allocationChoiceArguments(registryTestGG, registryTestAlice, registryTestGG, instrumentA), false), wantStatus: 200},
		{name: "unknown instrument", body: transferFactoryBody(allocationChoiceArguments(registryTestGG, registryTestAlice, registryTestGG, "wormhole-ntt:nope"), false), wantStatus: 404},
		{name: "instrument admin is not gg", body: transferFactoryBody(allocationChoiceArguments(registryTestIssuer, registryTestAlice, registryTestIssuer, instrumentThirdParty), false), wantStatus: 404},
		{name: "expectedAdmin mismatch", body: transferFactoryBody(allocationChoiceArguments(registryTestIssuer, registryTestAlice, registryTestGG, instrumentA), false), wantStatus: 400},
		{name: "missing sender", body: transferFactoryBody(allocationChoiceArguments(registryTestGG, "", registryTestGG, instrumentA), false), wantStatus: 400},
		{name: "missing allocation", body: `{"choiceArguments":{"expectedAdmin":"` + registryTestGG + `"}}`, wantStatus: 400},
		{name: "not JSON", body: `{`, wantStatus: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serveRegistry(t, srv, http.MethodPost, pathAllocationFactory, tc.body)
			requireStatus(t, recorder, tc.wantStatus)
			requireConformsToSpec(t, recorder, specAllocationInstructionV1, "POST", pathAllocationFactory)
			if tc.wantStatus != http.StatusOK {
				return
			}
			got := decodeRecorded[allocationFactoryWithChoiceContext](t, recorder)
			if got.FactoryID != cidCanonicalFactory {
				t.Fatalf("factoryId %q", got.FactoryID)
			}
			if len(got.ChoiceContext.ChoiceContextData.Values) != 0 {
				t.Fatalf("choiceContextData %+v", got.ChoiceContext.ChoiceContextData)
			}
			if len(got.ChoiceContext.DisclosedContracts) != 1 || got.ChoiceContext.DisclosedContracts[0].ContractID != cidCanonicalFactory {
				t.Fatalf("disclosedContracts %+v", got.ChoiceContext.DisclosedContracts)
			}
		})
	}
}

func TestAllocationChoiceContext(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	cases := []struct {
		name         string
		allocationID string
		choice       string
		wantStatus   int
	}{
		{name: "execute-transfer", allocationID: cidAllocationA, choice: "execute-transfer", wantStatus: 200},
		{name: "withdraw", allocationID: cidAllocationA, choice: "withdraw", wantStatus: 200},
		{name: "cancel", allocationID: cidAllocationA, choice: "cancel", wantStatus: 200},
		{name: "a transfer instruction id is not a CoinAllocation", allocationID: cidInstructionA, choice: "cancel", wantStatus: 404},
		{name: "allocation of another admin", allocationID: cidAllocationIssuer, choice: "cancel", wantStatus: 404},
		{name: "malformed id", allocationID: "zz", choice: "cancel", wantStatus: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := "/registry/allocations/v1/" + tc.allocationID + "/choice-contexts/" + tc.choice
			recorder := serveRegistry(t, srv, http.MethodPost, target, `{}`)
			requireStatus(t, recorder, tc.wantStatus)
			requireConformsToSpec(t, recorder, specAllocationV1, "POST", pathAllocationTemplate+tc.choice)
			if tc.wantStatus != http.StatusOK {
				return
			}
			got := decodeRecorded[registryChoiceContext](t, recorder)
			if len(got.ChoiceContextData.Values) != 0 || len(got.DisclosedContracts) != 0 {
				t.Fatalf("choice context %+v, want empty context and no disclosures", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Service-level behavior
// ---------------------------------------------------------------------------

func TestRegistry_DisabledIs404(t *testing.T) {
	srv := newFlowTestServer(t, registryTestFixtures())
	for _, target := range []string{pathRegistryInfo, pathInstruments} {
		recorder := serveRegistry(t, srv, http.MethodGet, target, "")
		requireStatus(t, recorder, http.StatusNotFound)
		requireConformsToSpec(t, recorder, specTokenMetadataV1, "GET", target)
	}
	recorder := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, `{}`)
	requireStatus(t, recorder, http.StatusNotFound)
	requireConformsToSpec(t, recorder, specTransferInstructionV1, "POST", pathTransferFactory)
}

func TestRegistry_UnknownPathIsJSON404(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	recorder := serveRegistry(t, srv, http.MethodGet, "/registry/metadata/v2/info", "")
	requireStatus(t, recorder, http.StatusNotFound)
	decodeRecorded[registryErrorResponse](t, recorder)
}

// TestRegistryAPI_EveryOperationConformsToSpec exercises every operation of the four vendored
// specs, success and error, and requires that each one ran through the validator.
func TestRegistryAPI_EveryOperationConformsToSpec(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	transferBody := transferFactoryBody(transferChoiceArguments(registryTestGG, registryTestAlice, registryTestBob, "1", registryTestGG, instrumentA), false)
	allocationBody := transferFactoryBody(allocationChoiceArguments(registryTestGG, registryTestAlice, registryTestGG, instrumentA), false)
	type registryRequest struct {
		specFile, method, pathTemplate, target, body string
		wantStatus                                   int
	}
	requests := []registryRequest{
		{specTokenMetadataV1, "GET", pathRegistryInfo, pathRegistryInfo, "", 200},
		{specTokenMetadataV1, "GET", pathInstruments, pathInstruments + "?pageSize=1", "", 200},
		{specTokenMetadataV1, "GET", pathInstrumentTemplate, pathInstruments + "/" + url.PathEscape(instrumentB), "", 200},
		{specTokenMetadataV1, "GET", pathInstrumentTemplate, pathInstruments + "/nope", "", 404},
		{specTransferInstructionV1, "POST", pathTransferFactory, pathTransferFactory, transferBody, 200},
		{specTransferInstructionV1, "POST", pathTransferFactory, pathTransferFactory, `{}`, 400},
		{specAllocationInstructionV1, "POST", pathAllocationFactory, pathAllocationFactory, allocationBody, 200},
		{specAllocationInstructionV1, "POST", pathAllocationFactory, pathAllocationFactory, `{}`, 400},
	}
	for _, choice := range []string{"accept", "reject", "withdraw"} {
		requests = append(requests,
			registryRequest{specTransferInstructionV1, "POST", pathTransferInstructionTemplate + choice, "/registry/transfer-instruction/v1/" + cidInstructionA + "/choice-contexts/" + choice, `{}`, 200},
			registryRequest{specTransferInstructionV1, "POST", pathTransferInstructionTemplate + choice, "/registry/transfer-instruction/v1/" + cidCanonicalFactory + "/choice-contexts/" + choice, `{}`, 404},
		)
	}
	for _, choice := range []string{"execute-transfer", "withdraw", "cancel"} {
		requests = append(requests,
			registryRequest{specAllocationV1, "POST", pathAllocationTemplate + choice, "/registry/allocations/v1/" + cidAllocationA + "/choice-contexts/" + choice, `{}`, 200},
			registryRequest{specAllocationV1, "POST", pathAllocationTemplate + choice, "/registry/allocations/v1/" + cidCanonicalFactory + "/choice-contexts/" + choice, `{}`, 404},
		)
	}
	covered := make(map[string]bool)
	for _, request := range requests {
		recorder := serveRegistry(t, srv, request.method, request.target, request.body)
		requireStatus(t, recorder, request.wantStatus)
		requireConformsToSpec(t, recorder, request.specFile, request.method, request.pathTemplate)
		covered[request.specFile+" "+request.method+" "+request.pathTemplate] = true
	}
	for _, specFile := range vendoredSpecFiles() {
		for _, operation := range loadOpenAPISpec(t, specFile).operations() {
			if !covered[specFile+" "+operation] {
				t.Errorf("%s %s has no conformance request", specFile, operation)
			}
		}
	}
}
