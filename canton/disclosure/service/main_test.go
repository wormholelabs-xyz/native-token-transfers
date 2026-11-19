package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseFlags
// ---------------------------------------------------------------------------

// testPartyFlags are the required reader-party flags with the test parties.
var testPartyFlags = []string{"--guardian-governance-party", testGuardianGovernanceParty, "--ntt-custody-party", testNttCustodyParty}

func TestParseFlags_Defaults(t *testing.T) {
	opts, err := parseFlags(append([]string{"--json-api", "http://127.0.0.1:6975"}, testPartyFlags...))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:7599", opts.listen)
	assert.Equal(t, "http://127.0.0.1:6975", opts.jsonAPIBaseURL)
	assert.Equal(t, readerPartiesByRole{guardianGovernance: testGuardianGovernanceParty, nttCustody: testNttCustodyParty}, opts.readerParties)
	assert.Equal(t, upstreamAuthNone{}, opts.upstreamAuth)
	assert.Equal(t, 1000, opts.maxContractsPerTemplate)
	assert.False(t, opts.verbose)
}

func TestParseFlags_MaxContractsPerTemplate(t *testing.T) {
	opts, err := parseFlags(append([]string{"--json-api", "http://127.0.0.1:6975", "--max-contracts-per-template", "5000"}, testPartyFlags...))
	require.NoError(t, err)
	assert.Equal(t, 5000, opts.maxContractsPerTemplate)

	srv := newServer(opts, nil)
	assert.Equal(t, 5000, srv.maxContracts)
}

func TestParseFlags_MaxContractsPerTemplate_RejectsNonPositive(t *testing.T) {
	for _, bad := range []string{"0", "-1"} {
		t.Run(bad, func(t *testing.T) {
			_, err := parseFlags(append([]string{"--json-api", "http://127.0.0.1:6975", "--max-contracts-per-template", bad}, testPartyFlags...))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--max-contracts-per-template")
		})
	}
}

func TestActiveContracts_MultiPartyFilter(t *testing.T) {
	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	var gotBody []byte
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})

	client := newTestHTTPACSClient(upstream.URL, nil)
	_, err := client.ActiveContracts(context.Background(), activeContractsQuery{readerParties: []partyID{"Alice", "Bob"}, template: template, activeAtOffset: 42, maxMatches: maxMatchingContractsPerQuery})
	require.NoError(t, err)

	var req activeContractsRequest
	require.NoError(t, json.Unmarshal(gotBody, &req))
	require.Len(t, req.EventFormat.FiltersByParty, 2)
	for _, party := range []string{"Alice", "Bob"} {
		filter, ok := req.EventFormat.FiltersByParty[party]
		require.True(t, ok, "missing filter for %s", party)
		require.Len(t, filter.Cumulative, 1)
		assert.Equal(t, template, filter.Cumulative[0].IdentifierFilter.TemplateFilter.Value.TemplateID)
		assert.True(t, filter.Cumulative[0].IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob)
	}
}

// TestActiveContracts_DedupsSharedContracts pins the union-view invariant: a contract visible
// to more than one disclosing party is served once.
func TestActiveContracts_DedupsSharedContracts(t *testing.T) {
	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	shared := acsContractJSON("pkg:Wormhole.Ntt.Manager:NttManager", "00shared", "AAA=", "sync")
	only := acsContractJSON("pkg:Wormhole.Ntt.Manager:NttManager", "00only", "BBB=", "sync")
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[" + shared + "," + shared + "," + only + "]"))
	})

	client := newTestHTTPACSClient(upstream.URL, nil)
	entries, err := client.ActiveContracts(context.Background(), activeContractsQuery{readerParties: []partyID{"Alice", "Bob"}, template: template, activeAtOffset: 42, maxMatches: maxMatchingContractsPerQuery})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "00shared", entries[0].ContractID)
	assert.Equal(t, "00only", entries[1].ContractID)
}

func TestParseFlags_MissingRequired(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantErrSub string
	}{
		{"missing json-api", testPartyFlags, "--json-api"},
		{"missing parties", []string{"--json-api", "http://127.0.0.1:6975"}, "--guardian-governance-party"},
		{"missing everything", []string{}, "--json-api"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFlags(tc.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrSub)
		})
	}
}

// ---------------------------------------------------------------------------
// checkUpstreamTemplate
// ---------------------------------------------------------------------------

func TestCheckUpstreamTemplate(t *testing.T) {
	const requested = "#ntt:Wormhole.Ntt.Manager:NttManager"
	cases := []struct {
		name               string
		returnedPackage    string
		returnedTemplateID string
		wantErr            bool
	}{
		{"right name and tail, package-id qualified", "ntt", "0123abcd:Wormhole.Ntt.Manager:NttManager", false},
		{"right name and tail, name qualified", "ntt", "#ntt:Wormhole.Ntt.Manager:NttManager", false},
		{"right tail, packageName ntt-evil", "ntt-evil", "0123abcd:Wormhole.Ntt.Manager:NttManager", true},
		{"packageName missing", "", "0123abcd:Wormhole.Ntt.Manager:NttManager", true},
		{"right name, wrong entity", "ntt", "0123abcd:Wormhole.Ntt.Manager:AdminTransferProposal", true},
		{"right name, wrong module", "ntt", "0123abcd:Wormhole.Ntt.Ledger:NttManager", true},
		{"template id with no package qualifier", "ntt", "Wormhole.Ntt.Manager:NttManager", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUpstreamTemplate(requested, tc.returnedPackage, tc.returnedTemplateID)
			if tc.wantErr {
				assert.ErrorIs(t, err, errUpstreamTemplateMismatch)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestFlowRejectsWrongPackageName(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	wrongPackageManager := strings.Replace(b.managerJSON(), `"packageName":"ntt"`, `"packageName":"ntt-evil"`, 1)
	require.NotEqual(t, b.managerJSON(), wrongPackageManager)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(wrongPackageManager)})
	logged := captureServerLog(srv)
	recorder := requestFlow(t, srv, "transfer", "manager="+b.managerAddr)
	requirePublicErrorWithLoggedDetail(t, recorder, logged, http.StatusBadGateway, "ntt-evil")
}

// ---------------------------------------------------------------------------
// Fake upstream JSON Ledger API v2
// ---------------------------------------------------------------------------

// newTestUpstreamHTTPClient is the production upstream client with default TLS files.
func newTestUpstreamHTTPClient(t *testing.T) *http.Client {
	t.Helper()
	tlsConfig, err := loadUpstreamTLSConfig(upstreamTLSFiles{}, time.Now())
	require.NoError(t, err)
	return newUpstreamHTTPClient(tlsConfig)
}

// newTestHTTPACSClient is an ACS client with the default TLS config and the given token source.
func newTestHTTPACSClient(baseURL string, tokens bearerTokenSource) *httpACSClient {
	return newHTTPACSClient(baseURL, newUpstreamHTTPClient(&tls.Config{MinVersion: tls.VersionTLS12}), tokens)
}

// newFakeJSONAPI starts an httptest server that stands in for the JSON Ledger API v2. It answers
// /v2/state/ledger-end with a fixed offset and routes /v2/state/active-contracts to acHandler.
// The returned capture holds the most recent Authorization header from both endpoints, for the
// bearer-token test.
func newFakeJSONAPI(t *testing.T, acHandler http.HandlerFunc) (*httptest.Server, *authCapture) {
	t.Helper()
	return newFakeJSONAPIWithEvents(t, acHandler, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":"CONTRACT_EVENTS_NOT_FOUND"}`, http.StatusNotFound)
	})
}

// newFakeJSONAPIWithEvents is newFakeJSONAPI plus /v2/events/events-by-contract-id, routed to eventsHandler.
func newFakeJSONAPIWithEvents(t *testing.T, acHandler, eventsHandler http.HandlerFunc) (*httptest.Server, *authCapture) {
	t.Helper()
	capture := &authCapture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/state/ledger-end", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("fake json-api: ledger-end: got method %s, want GET", r.Method)
		}
		capture.set(r.Header.Get("Authorization"))
		writeJSON(w, http.StatusOK, map[string]int64{"offset": 42})
	})
	mux.HandleFunc("/v2/state/active-contracts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("fake json-api: active-contracts: got method %s, want POST", r.Method)
		}
		capture.set(r.Header.Get("Authorization"))

		// Real participant behavior: unqualified "Module:Entity" template filters get HTTP
		// 400. Only "#name:Module:Entity" or "packageId:Module:Entity" are accepted.
		// Re-buffer the body so acHandler can decode it too.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		r.Body = io.NopCloser(bytes.NewReader(body))
		var req activeContractsRequest
		require.NoError(t, json.Unmarshal(body, &req))
		for _, pf := range req.EventFormat.FiltersByParty {
			for _, cf := range pf.Cumulative {
				tmpl := cf.IdentifierFilter.TemplateFilter.Value.TemplateID
				if strings.Count(tmpl, ":") != 2 {
					http.Error(w, fmt.Sprintf("fake json-api: unqualified templateId %q", tmpl), http.StatusBadRequest)
					return
				}
			}
		}

		acHandler(w, r)
	})
	mux.HandleFunc("/v2/events/events-by-contract-id", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("fake json-api: events-by-contract-id: got method %s, want POST", r.Method)
		}
		capture.set(r.Header.Get("Authorization"))
		eventsHandler(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, capture
}

type authCapture struct {
	mu   sync.Mutex
	last string
}

func (a *authCapture) set(v string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.last = v
}

func (a *authCapture) get() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

// decodeRequestedTemplate returns the reader parties and the one templateId of an
// active-contracts request body. Every party filter must name the same template.
func decodeRequestedTemplate(t *testing.T, r *http.Request) (parties []partyID, template string) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var req activeContractsRequest
	require.NoError(t, json.Unmarshal(body, &req))
	require.NotEmpty(t, req.EventFormat.FiltersByParty, "expected at least one party filter")
	for party, partyFilter := range req.EventFormat.FiltersByParty {
		parties = append(parties, partyID(party))
		require.Len(t, partyFilter.Cumulative, 1, "expected exactly one cumulative filter")
		partyTemplate := partyFilter.Cumulative[0].IdentifierFilter.TemplateFilter.Value.TemplateID
		// The registry Coin totals read omits blobs; every other read includes them.
		wantCreatedEventBlob := templateTail(partyTemplate) != tailCoin
		require.Equal(t, wantCreatedEventBlob, partyFilter.Cumulative[0].IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob)
		if template != "" {
			require.Equal(t, template, partyTemplate, "party filters name different templates")
		}
		template = partyTemplate
	}
	return parties, template
}

// acsContractJSON renders one active-contracts response element in the JSON Ledger API v2 wire
// shape (contractEntry.JsActiveContract{createdEvent, synchronizerId, ...}). It is independent
// of main.go's internal decode types, so the test pins the actual wire contract as ground truth.
// synchronizerId sits beside createdEvent, at the same level (confirmed live).
func acsContractJSON(templateID, contractID, blob, synchronizerID string) string {
	return fmt.Sprintf(
		`{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":%q,"templateId":%q,"packageName":%q,"createdEventBlob":%q},"synchronizerId":%q}}}`,
		contractID, templateID, testPackageName(templateID), blob, synchronizerID,
	)
}

// testPackageName is the package name of the flow template with templateID's tail.
func testPackageName(templateID string) string {
	qualifiedTemplate, known := flowTemplateByTail[templateTail(templateID)]
	if !known {
		qualifiedTemplate, known = custodyPotHoldingTemplateByTail[templateTail(templateID)]
	}
	if !known {
		qualifiedTemplate, known = registryTemplateByTail[templateTail(templateID)]
	}
	if !known {
		return "unknown-package"
	}
	return strings.TrimPrefix(strings.SplitN(qualifiedTemplate, ":", 2)[0], "#")
}

// ---------------------------------------------------------------------------
// Test servers
// ---------------------------------------------------------------------------

// testPartyFingerprintSuffix is a well-formed "::1220" namespace fingerprint for test party ids.
const testPartyFingerprintSuffix = "::1220" + "7d1a8b3c5e9f0a2b4c6d8e0f1a3b5c7d9e1f3a5b7c9d1e3f5a7b9c1d3e5f7a9b"

const (
	testGuardianGovernanceParty = "gg" + testPartyFingerprintSuffix
	testNttCustodyParty         = "nttCustody" + testPartyFingerprintSuffix
	testOperatorParty           = "operator" + testPartyFingerprintSuffix
)

// testReaderParties holds the three test reader parties.
func testReaderParties() readerPartiesByRole {
	return readerPartiesByRole{guardianGovernance: testGuardianGovernanceParty, nttCustody: testNttCustodyParty, operator: testOperatorParty}
}

func newTestServer(t *testing.T, upstream *httptest.Server) *server {
	t.Helper()
	return newTestServerWithClient(t, newTestHTTPACSClient(upstream.URL, nil))
}

// newTestServerWithClient builds a server over acs with testReaderParties.
func newTestServerWithClient(t *testing.T, acs acsClient) *server {
	t.Helper()
	opts := &options{readerParties: testReaderParties(), maxContractsPerTemplate: defaultMaxContractsPerTemplate}
	return newServer(opts, acs)
}

func TestUnknownPathReturns404(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {})
	srv := newTestServer(t, upstream)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/nonsense", nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---------------------------------------------------------------------------
// /v1/healthz
// ---------------------------------------------------------------------------

func TestHandleHealthz(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {})
	srv := newTestServer(t, upstream)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	srv.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rr.Body.String())
	for _, party := range []string{testGuardianGovernanceParty, testNttCustodyParty, testOperatorParty} {
		assert.NotContains(t, rr.Body.String(), party, "a public response carries no reader party id")
	}
}

// ---------------------------------------------------------------------------
// Bearer token forwarding
// ---------------------------------------------------------------------------

func TestBearerTokenFromFileForwardedUpstream(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("secret-token-value\n"), 0o600))

	token, err := readAccessToken(tokenPath)
	require.NoError(t, err)
	assert.Equal(t, "secret-token-value", token) // trimmed

	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	upstream, capture := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})

	client := newTestHTTPACSClient(upstream.URL, fileBearerTokenSource{path: tokenPath})
	_, err = client.ActiveContracts(context.Background(), testQuery(template, 42))
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret-token-value", capture.get())
}

// TestBearerTokenRereadOnEveryRequest pins the rotation behavior: each upstream request reads
// the token file fresh, so a rotated token takes effect on the next request.
func TestBearerTokenRereadOnEveryRequest(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("first-token\n"), 0o600))

	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	upstream, capture := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})

	client := newTestHTTPACSClient(upstream.URL, fileBearerTokenSource{path: tokenPath})
	_, err := client.ActiveContracts(context.Background(), testQuery(template, 42))
	require.NoError(t, err)
	assert.Equal(t, "Bearer first-token", capture.get())

	require.NoError(t, os.WriteFile(tokenPath, []byte("second-token\n"), 0o600))

	_, err = client.ActiveContracts(context.Background(), testQuery(template, 42))
	require.NoError(t, err)
	assert.Equal(t, "Bearer second-token", capture.get())
}

// ---------------------------------------------------------------------------
// run(): banner + graceful shutdown
// ---------------------------------------------------------------------------

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// listeningURL scans stderr output for the banner's final line and returns the URL it names. It
// returns "" until that line appears.
func listeningURL(s string) string {
	const prefix = "disclosure-service: listening on "
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

func TestRun_BannerHealthzAndGracefulShutdown(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})

	opts := &options{
		listen:                  "127.0.0.1:0",
		upstreamAuth:            upstreamAuthNone{},
		jsonAPIBaseURL:          upstream.URL,
		readerParties:           testReaderParties(),
		maxContractsPerTemplate: defaultMaxContractsPerTemplate,
		maxUpstreamConcurrency:  defaultMaxUpstreamConcurrency,
	}

	var out syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, opts, &out) }()

	var addr string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if u := listeningURL(out.String()); u != "" {
			addr = u
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.NotEmpty(t, addr, "run() never printed a listening banner; stderr so far:\n%s", out.String())

	banner := out.String()
	assert.Contains(t, banner, testGuardianGovernanceParty)
	assert.Contains(t, banner, "put an edge proxy (TLS, rate limit, WAF) in front")
	assert.NotContains(t, banner, "non-loopback") // 127.0.0.1 is loopback

	resp, err := http.Get(addr + "/v1/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	select {
	case runErr := <-done:
		assert.NoError(t, runErr)
	case <-time.After(3 * time.Second):
		t.Fatal("run() did not return within 3s of context cancellation")
	}
}

// ---------------------------------------------------------------------------
// /v1/flows/{flow}: fixtures
// ---------------------------------------------------------------------------

const testRecipient = "recipient" + testPartyFingerprintSuffix

// acsContractJSONWithArg is acsContractJSON plus a createArgument payload (raw JSON).
func acsContractJSONWithArg(templateID, contractID, blob, synchronizerID, createArgument string) string {
	return fmt.Sprintf(
		`{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":%q,"templateId":%q,"packageName":%q,"createdEventBlob":%q,"createArgument":%s},"synchronizerId":%q}}}`,
		contractID, templateID, testPackageName(templateID), blob, createArgument, synchronizerID,
	)
}

func jsonArray(items ...string) string {
	return "[" + strings.Join(items, ",") + "]"
}

func nttManagerArg(admin, gg, nttCustody, reserveCustodian, namespace, managerAddress string, transceiverEmitterID int, instrumentAdmin, instrumentID, tokenConfig, factoryTag, factoryCid string) string {
	return fmt.Sprintf(
		`{"admin":%q,"guardianGovernance":%q,"nttCustody":%q,"reserveCustodian":%q,"namespace":%q,"managerAddress":%q,"transceiverEmitterId":%d,"instrumentId":{"admin":%q,"id":%q},"tokenConfig":%q,"factory":{"tag":%q,"value":%q}}`,
		admin, gg, nttCustody, reserveCustodian, namespace, managerAddress, transceiverEmitterID, instrumentAdmin, instrumentID, tokenConfig, factoryTag, factoryCid,
	)
}

// nttManagerArgBeforeNtt040 is an ntt 0.3.x payload: no nttCustody and no reserveCustodian.
func nttManagerArgBeforeNtt040(admin, gg, managerAddress, tokenConfig, factoryTag, factoryCid string) string {
	return fmt.Sprintf(
		`{"admin":%q,"guardianGovernance":%q,"namespace":"ns","managerAddress":%q,"transceiverEmitterId":7,"instrumentId":{"admin":%q,"id":"instrument-1"},"tokenConfig":%q,"factory":{"tag":%q,"value":%q}}`,
		admin, gg, managerAddress, gg, tokenConfig, factoryTag, factoryCid,
	)
}

func ledgerArg(managerAddress string, potCid *string) string {
	pot := "null"
	if potCid != nil {
		pot = fmt.Sprintf("%q", *potCid)
	}
	return fmt.Sprintf(`{"managerAddress":%q,"custodyHoldingCid":%s}`, managerAddress, pot)
}

func replayNodeArg(consumer, namespace, prefix string) string {
	return fmt.Sprintf(`{"consumer":%q,"namespace":%q,"prefix":%q}`, consumer, namespace, prefix)
}

func guardianAnchorArg(gg string) string {
	return fmt.Sprintf(`{"guardianGovernance":%q}`, gg)
}

func emitterArg(owner string, emitterID int) string {
	return fmt.Sprintf(`{"owner":%q,"emitterId":%d}`, owner, emitterID)
}

func preapprovalArg(owner, instrumentAdmin, instrumentID string) string {
	return fmt.Sprintf(`{"owner":%q,"instrumentId":{"admin":%q,"id":%q}}`, owner, instrumentAdmin, instrumentID)
}

func adminTransferProposalArg(admin, newAdmin, managerAddress string) string {
	return fmt.Sprintf(`{"admin":%q,"newAdmin":%q,"managerAddress":%q}`, admin, newAdmin, managerAddress)
}

func coinFactoryArg(admin string) string {
	return fmt.Sprintf(`{"admin":%q}`, admin)
}

// baseFixtures holds one deployment's identity fields, reused across a flow test's per-template
// fixtures so every fixture agrees on the same manager, namespace, and factory.
type baseFixtures struct {
	admin, gg, nttCustody, namespace, managerAddr, digest, instrumentAdmin, instrumentID string
	transceiverEmitterID                                                                 int
	tokenConfig, factoryTag, factoryCid                                                  string
}

// newBaseFixturesAdminedBy is newBaseFixtures with the admin role held by admin.
func newBaseFixturesAdminedBy(tokenConfig, admin string) baseFixtures {
	b := newBaseFixtures(tokenConfig)
	b.admin = admin
	return b
}

func newBaseFixtures(tokenConfig string) baseFixtures {
	factoryTag := factoryTagTransfer
	if tokenConfig == tokenConfigBurnMint {
		factoryTag = factoryTagBurnMint
	}
	return baseFixtures{
		admin:                testRecipient, // distinct from gg, so custodian-vs-recipient preapproval owners differ
		gg:                   testGuardianGovernanceParty,
		nttCustody:           testNttCustodyParty,
		namespace:            "ntt:test-namespace",
		managerAddr:          strings.Repeat("ab", 32),
		digest:               strings.Repeat("cd", 32),
		instrumentAdmin:      testGuardianGovernanceParty,
		instrumentID:         "wormhole-ntt:instrument-1",
		transceiverEmitterID: 7,
		tokenConfig:          tokenConfig,
		factoryTag:           factoryTag,
		factoryCid:           "factory-cid-1",
	}
}

func (b baseFixtures) reserveCustodian() string {
	return string(reserveCustodianFor(partyID(b.gg), partyID(b.nttCustody), partyID(b.admin)))
}

func (b baseFixtures) managerJSON() string {
	return acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-1", "bWdy", "sync1",
		nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, b.managerAddr, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid))
}

// transferPreapprovalJSON is owner's TransferPreapproval for the deployment's instrument, with
// contract id "pre-" + owner.
func (b baseFixtures) transferPreapprovalJSON(owner string) string {
	return acsContractJSONWithArg("pkg:"+tailTransferPreapproval, "pre-"+owner, "cHJl", "sync1", preapprovalArg(owner, b.instrumentAdmin, b.instrumentID))
}

func (b baseFixtures) coreStateJSON() string {
	return acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-1", "Y3M=", "sync1", guardianAnchorArg(b.gg))
}

func (b baseFixtures) replayNodeJSON(prefix string) string {
	return acsContractJSONWithArg("pkg:"+tailReplayNode, "node-cid-1", "bm9kZQ==", "sync1", replayNodeArg(b.gg, b.namespace, prefix))
}

func (b baseFixtures) factoryJSON() string {
	return acsContractJSONWithArg("pkg:"+tailCoinFactory, b.factoryCid, "ZmFjdA==", "sync1", coinFactoryArg(b.gg))
}

func (b baseFixtures) ledgerJSON(potCid *string) string {
	return acsContractJSONWithArg("pkg:"+tailLockedLedger, "ledger-cid-1", "bGVkZ2Vy", "sync1", ledgerArg(b.managerAddr, potCid))
}

func (b baseFixtures) emitterJSON() string {
	return acsContractJSONWithArg("pkg:"+tailEmitter, "emitter-cid-1", "ZW1pdA==", "sync1", emitterArg(b.gg, b.transceiverEmitterID))
}

// flowFixtures maps a template's "Module:Entity" tail to the JSON array body the fake upstream
// returns when queried for that template. A tail with no entry answers "[]".
type flowFixtures map[string]string

func newFlowTestServer(t *testing.T, fixtures flowFixtures) *server {
	t.Helper()
	srv, _ := newRecordingFlowTestServer(t, fixtures)
	return srv
}

// archivedFixtureKey marks contractID as archived for the events-by-contract-id fake.
func archivedFixtureKey(contractID string) string { return "archived:" + contractID }

// upstreamStatusFixtureKey makes the active-contracts fake answer tail with the HTTP status in
// the fixture value, for example "413".
func upstreamStatusFixtureKey(tail string) string { return "upstream-status:" + tail }

// isFixtureMarkerKey reports a fixture key that holds no contract list.
func isFixtureMarkerKey(key string) bool {
	return strings.HasPrefix(key, "archived:") || strings.HasPrefix(key, "upstream-status:")
}

// queriedTemplateRecorder records the template tail of each active-contracts query, and the
// reader parties of each upstream read by tail.
type queriedTemplateRecorder struct {
	mu            sync.Mutex
	recordedTails []string
	readersByTail map[string][]partyID
}

func (recorder *queriedTemplateRecorder) record(tail string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.recordedTails = append(recorder.recordedTails, tail)
}

func (recorder *queriedTemplateRecorder) recordReaders(tail string, parties []partyID) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.readersByTail == nil {
		recorder.readersByTail = map[string][]partyID{}
	}
	recorder.readersByTail[tail] = append(recorder.readersByTail[tail], parties...)
}

func (recorder *queriedTemplateRecorder) readerPartiesByTail() map[string][]partyID {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	copied := make(map[string][]partyID, len(recorder.readersByTail))
	for tail, parties := range recorder.readersByTail {
		copied[tail] = append([]partyID(nil), parties...)
	}
	return copied
}

func (recorder *queriedTemplateRecorder) tails() []string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]string(nil), recorder.recordedTails...)
}

// newRecordingFlowTestServer serves fixtures for active-contracts queries. The
// events-by-contract-id fake answers with the fixture contract of that id, or a 404.
func newRecordingFlowTestServer(t *testing.T, fixtures flowFixtures) (*server, *queriedTemplateRecorder) {
	t.Helper()
	recorder := &queriedTemplateRecorder{}
	upstream, _ := newFakeJSONAPIWithEvents(t, func(w http.ResponseWriter, r *http.Request) {
		parties, tmpl := decodeRequestedTemplate(t, r)
		recorder.record(templateTail(tmpl))
		recorder.recordReaders(templateTail(tmpl), parties)
		if statusText, hasStatus := fixtures[upstreamStatusFixtureKey(templateTail(tmpl))]; hasStatus {
			statusCode, err := strconv.Atoi(statusText)
			require.NoError(t, err)
			http.Error(w, `{"code":"FAKE_UPSTREAM_STATUS"}`, statusCode)
			return
		}
		body, ok := fixtures[templateTail(tmpl)]
		if !ok {
			body = "[]"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}, func(w http.ResponseWriter, r *http.Request) {
		var request eventsByContractIDRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		created, found := findFixtureContract(t, fixtures, request.ContractID)
		readerParties := make([]partyID, 0, len(request.EventFormat.FiltersByParty))
		for party := range request.EventFormat.FiltersByParty {
			readerParties = append(readerParties, partyID(party))
		}
		if found {
			var createdContract struct {
				CreatedEvent struct {
					TemplateID string `json:"templateId"`
				} `json:"createdEvent"`
			}
			require.NoError(t, json.Unmarshal([]byte(created), &createdContract))
			recorder.recordReaders(templateTail(createdContract.CreatedEvent.TemplateID), readerParties)
		}
		if !found {
			http.Error(w, `{"code":"CONTRACT_EVENTS_NOT_FOUND"}`, http.StatusNotFound)
			return
		}
		response := `{"created":` + created
		if _, archived := fixtures[archivedFixtureKey(request.ContractID)]; archived {
			response += fmt.Sprintf(`,"archived":{"archivedEvent":{"contractId":%q},"synchronizerId":"sync1"}`, request.ContractID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response + "}"))
	})
	return newTestServer(t, upstream), recorder
}

// findFixtureContract returns the JsActiveContract JSON of contractID from any fixture array.
func findFixtureContract(t *testing.T, fixtures flowFixtures, contractID string) (string, bool) {
	t.Helper()
	for key, body := range fixtures {
		if isFixtureMarkerKey(key) {
			continue
		}
		var entries []struct {
			ContractEntry struct {
				JsActiveContract json.RawMessage `json:"JsActiveContract"`
			} `json:"contractEntry"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &entries))
		for _, entry := range entries {
			if len(entry.ContractEntry.JsActiveContract) == 0 {
				continue
			}
			var activeContract struct {
				CreatedEvent struct {
					ContractID string `json:"contractId"`
				} `json:"createdEvent"`
			}
			require.NoError(t, json.Unmarshal(entry.ContractEntry.JsActiveContract, &activeContract))
			if activeContract.CreatedEvent.ContractID == contractID {
				return string(entry.ContractEntry.JsActiveContract), true
			}
		}
	}
	return "", false
}

func requestFlow(t *testing.T, srv *server, flow, query string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/flows/"+flow+"?"+query, nil)
	srv.ServeHTTP(rr, req)
	return rr
}

func decodeFlowResponse(t *testing.T, rr *httptest.ResponseRecorder) flowResponse {
	t.Helper()
	var resp flowResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp
}

func rolesOf(entries []flowDisclosureEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Role
	}
	return out
}

func missingRolesOf(entries []flowMissingEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Role
	}
	return out
}

func requireFlowErrorStatus(t *testing.T, err error, want int) {
	t.Helper()
	var fe *flowError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, want, fe.status)
}

// ---------------------------------------------------------------------------
// /v1/flows/{flow}: pure selector unit tests
// ---------------------------------------------------------------------------

func TestParseHex64(t *testing.T) {
	valid := strings.Repeat("ab", 32)
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"valid lowercase", valid, valid, false},
		{"valid uppercase normalizes", strings.ToUpper(valid), valid, false},
		{"too short", "abcd", "", true},
		{"too long", valid + "ab", "", true},
		{"non-hex characters", strings.Repeat("zz", 32), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := parseManagerAddressHex(tc.raw)
			if tc.wantErr {
				requireFlowErrorStatus(t, err, http.StatusBadRequest)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, manager.hex)
		})
	}
}

func TestParseByVAA(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		present bool
		want    bool
		wantErr bool
	}{
		{"absent defaults to false", "", false, false, false},
		{"true", "true", true, true, false},
		{"false", "false", true, false, false},
		{"not a bool", "yes", true, false, true},
		{"present and empty", "", true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseByVAA(tc.raw, tc.present)
			if tc.wantErr {
				requireFlowErrorStatus(t, err, http.StatusBadRequest)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFindManager(t *testing.T) {
	managerAt := func(addr string) decodedEntry[daNttManager] {
		return decodedEntry[daNttManager]{entry: acsEntry{ContractID: "cid-" + addr}, value: daNttManager{ManagerAddress: addr}}
	}
	addrA := strings.Repeat("aa", 32)
	addrB := strings.Repeat("bb", 32)

	t.Run("found", func(t *testing.T) {
		match, err := findManager([]decodedEntry[daNttManager]{managerAt(addrA), managerAt(addrB)}, managerAddressHex{hex: addrA})
		require.NoError(t, err)
		assert.Equal(t, "cid-"+addrA, match.entry.ContractID)
	})
	t.Run("not found is 404", func(t *testing.T) {
		_, err := findManager([]decodedEntry[daNttManager]{managerAt(addrA)}, managerAddressHex{hex: addrB})
		requireFlowErrorStatus(t, err, http.StatusNotFound)
	})
	t.Run("duplicate is 500", func(t *testing.T) {
		_, err := findManager([]decodedEntry[daNttManager]{managerAt(addrA), managerAt(addrA)}, managerAddressHex{hex: addrA})
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindGuardianAnchor(t *testing.T) {
	anchorOf := func(gg string) decodedEntry[daGuardianAnchor] {
		return decodedEntry[daGuardianAnchor]{entry: acsEntry{ContractID: "cid-" + gg}, value: daGuardianAnchor{GuardianGovernance: partyID(gg)}}
	}
	t.Run("gg given, exactly one match", func(t *testing.T) {
		match, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{anchorOf("gg1"), anchorOf("gg2")}, "gg1", "CoreState")
		require.NoError(t, err)
		assert.Equal(t, "cid-gg1", match.entry.ContractID)
	})
	t.Run("gg given, zero matches is 404", func(t *testing.T) {
		_, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{anchorOf("gg2")}, "gg1", "CoreState")
		requireFlowErrorStatus(t, err, http.StatusNotFound)
	})
	t.Run("gg empty, sole entry", func(t *testing.T) {
		match, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{anchorOf("gg1")}, "", "CoreState")
		require.NoError(t, err)
		assert.Equal(t, "cid-gg1", match.entry.ContractID)
	})
	t.Run("gg empty, zero entries is 500", func(t *testing.T) {
		_, err := findGuardianAnchor(nil, "", "CoreState")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("gg empty, two entries is 409", func(t *testing.T) {
		_, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{anchorOf("gg1"), anchorOf("gg2")}, "", "CoreState")
		requireFlowErrorStatus(t, err, http.StatusConflict)
	})
}

func TestCoveringReplayNode(t *testing.T) {
	replayNode := func(consumer, namespace, prefix string) decodedEntry[daReplayNode] {
		return decodedEntry[daReplayNode]{entry: acsEntry{ContractID: "cid-" + prefix}, value: daReplayNode{Consumer: partyID(consumer), Namespace: namespace, Prefix: prefix}}
	}
	digest := vaaDigestHex{hex: strings.Repeat("cd", 32)}

	t.Run("found by prefix", func(t *testing.T) {
		match, err := coveringReplayNode([]decodedEntry[daReplayNode]{replayNode("gg1", "ns1", "cd")}, "gg1", "ns1", digest)
		require.NoError(t, err)
		assert.Equal(t, "cid-cd", match.entry.ContractID)
	})
	t.Run("zero covering nodes is 500", func(t *testing.T) {
		_, err := coveringReplayNode([]decodedEntry[daReplayNode]{replayNode("gg1", "ns1", "ff")}, "gg1", "ns1", digest)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("two covering nodes is 500", func(t *testing.T) {
		entries := []decodedEntry[daReplayNode]{replayNode("gg1", "ns1", ""), replayNode("gg1", "ns1", "cd")}
		_, err := coveringReplayNode(entries, "gg1", "ns1", digest)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindLockedLedger(t *testing.T) {
	addr := strings.Repeat("aa", 32)
	manager := managerAddressHex{hex: addr}
	ledgerOfManager := func() decodedEntry[daLockedLedger] {
		return decodedEntry[daLockedLedger]{entry: acsEntry{ContractID: "cid-" + addr}, value: daLockedLedger{ManagerAddress: addr}}
	}
	t.Run("found", func(t *testing.T) {
		match, err := findLockedLedger([]decodedEntry[daLockedLedger]{ledgerOfManager()}, manager)
		require.NoError(t, err)
		assert.Equal(t, "cid-"+addr, match.entry.ContractID)
	})
	t.Run("not found is 500", func(t *testing.T) {
		_, err := findLockedLedger(nil, manager)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("duplicate is 500", func(t *testing.T) {
		_, err := findLockedLedger([]decodedEntry[daLockedLedger]{ledgerOfManager(), ledgerOfManager()}, manager)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindTransceiverEmitter(t *testing.T) {
	emitterOf := func(owner string, id json.Number) decodedEntry[daEmitter] {
		return decodedEntry[daEmitter]{entry: acsEntry{ContractID: "emitter-cid"}, value: daEmitter{Owner: partyID(owner), EmitterId: id}}
	}
	t.Run("found, numeric emitterId", func(t *testing.T) {
		match, err := findTransceiverEmitter([]decodedEntry[daEmitter]{emitterOf("gg1", json.Number("7"))}, json.Number("7"), "gg1")
		require.NoError(t, err)
		assert.Equal(t, "emitter-cid", match.entry.ContractID)
	})
	t.Run("found, string emitterId", func(t *testing.T) {
		_, err := findTransceiverEmitter([]decodedEntry[daEmitter]{emitterOf("gg1", json.Number("7"))}, json.Number("7"), "gg1")
		require.NoError(t, err)
	})
	t.Run("owner mismatch is 500", func(t *testing.T) {
		_, err := findTransceiverEmitter([]decodedEntry[daEmitter]{emitterOf("gg2", json.Number("7"))}, json.Number("7"), "gg1")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("id mismatch is 500", func(t *testing.T) {
		_, err := findTransceiverEmitter([]decodedEntry[daEmitter]{emitterOf("gg1", json.Number("8"))}, json.Number("7"), "gg1")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindCanonicalCoinFactory(t *testing.T) {
	factoryAdminedBy := func(admin string) decodedEntry[daCoinFactory] {
		return decodedEntry[daCoinFactory]{entry: acsEntry{ContractID: "cid-" + admin}, value: daCoinFactory{Admin: partyID(admin)}}
	}
	t.Run("found", func(t *testing.T) {
		match, err := findCanonicalCoinFactory([]decodedEntry[daCoinFactory]{factoryAdminedBy("gg1")}, "gg1")
		require.NoError(t, err)
		assert.Equal(t, "cid-gg1", match.entry.ContractID)
	})
	t.Run("not found is 500", func(t *testing.T) {
		_, err := findCanonicalCoinFactory([]decodedEntry[daCoinFactory]{factoryAdminedBy("gg2")}, "gg1")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestRequireFactoryTag(t *testing.T) {
	require.NoError(t, requireFactoryTag(daNttFactory{Tag: factoryTagTransfer}, factoryTagTransfer))
	err := requireFactoryTag(daNttFactory{Tag: factoryTagBurnMint}, factoryTagTransfer)
	requireFlowErrorStatus(t, err, http.StatusConflict)
}

func TestRequireTokenConfig(t *testing.T) {
	require.NoError(t, requireTokenConfig(daNttManager{TokenConfig: tokenConfigLockUnlock}, tokenConfigLockUnlock, "release"))
	err := requireTokenConfig(daNttManager{TokenConfig: tokenConfigBurnMint}, tokenConfigLockUnlock, "release")
	requireFlowErrorStatus(t, err, http.StatusConflict)
	assert.Contains(t, err.Error(), "BurnMint")
}

func TestFindPreapproval(t *testing.T) {
	inst := daInstrumentId{Admin: "gg1", Id: "instr-1"}
	preapprovalOf := func(owner string) decodedEntry[daPreapproval] {
		return decodedEntry[daPreapproval]{entry: acsEntry{ContractID: "cid-" + owner}, value: daPreapproval{Owner: partyID(owner), InstrumentId: inst}}
	}
	t.Run("found", func(t *testing.T) {
		match, found, err := findPreapproval([]decodedEntry[daPreapproval]{preapprovalOf("alice")}, "alice", inst, "TransferPreapproval")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "cid-alice", match.entry.ContractID)
	})
	t.Run("absent is not an error", func(t *testing.T) {
		_, found, err := findPreapproval(nil, "alice", inst, "TransferPreapproval")
		require.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("duplicate is 500", func(t *testing.T) {
		_, _, err := findPreapproval([]decodedEntry[daPreapproval]{preapprovalOf("alice"), preapprovalOf("alice")}, "alice", inst, "TransferPreapproval")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindAdminTransferProposal(t *testing.T) {
	manager := daNttManager{Admin: "admin::1", GuardianGovernance: "gg::1", ManagerAddress: strings.Repeat("aa", 32)}
	proposal := func(contractID string, admin, newAdmin partyID, managerAddress string) decodedEntry[daAdminTransferProposal] {
		return decodedEntry[daAdminTransferProposal]{
			entry: acsEntry{ContractID: contractID},
			value: daAdminTransferProposal{Admin: admin, NewAdmin: newAdmin, ManagerAddress: managerAddress},
		}
	}
	exact := proposal("prop-exact", manager.Admin, manager.GuardianGovernance, manager.ManagerAddress)
	cases := []struct {
		name           string
		proposals      []decodedEntry[daAdminTransferProposal]
		wantFound      bool
		wantContractID string
		wantStatus     int
	}{
		{"exact", []decodedEntry[daAdminTransferProposal]{exact}, true, "prop-exact", 0},
		{"exact, upper-case managerAddress", []decodedEntry[daAdminTransferProposal]{proposal("prop-upper", manager.Admin, manager.GuardianGovernance, strings.ToUpper(manager.ManagerAddress))}, true, "prop-upper", 0},
		{"admin wrong", []decodedEntry[daAdminTransferProposal]{proposal("p", "stranger::1", manager.GuardianGovernance, manager.ManagerAddress)}, false, "", 0},
		{"newAdmin wrong", []decodedEntry[daAdminTransferProposal]{proposal("p", manager.Admin, "stranger::1", manager.ManagerAddress)}, false, "", 0},
		{"managerAddress wrong", []decodedEntry[daAdminTransferProposal]{proposal("p", manager.Admin, manager.GuardianGovernance, strings.Repeat("bb", 32))}, false, "", 0},
		{"stranger plus real", []decodedEntry[daAdminTransferProposal]{proposal("p", "stranger::1", manager.GuardianGovernance, manager.ManagerAddress), exact}, true, "prop-exact", 0},
		{"none", nil, false, "", 0},
		{"two exact is 500", []decodedEntry[daAdminTransferProposal]{exact, exact}, false, "", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			match, found, err := findAdminTransferProposal(tc.proposals, manager)
			if tc.wantStatus != 0 {
				requireFlowErrorStatus(t, err, tc.wantStatus)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantFound, found)
			assert.Equal(t, tc.wantContractID, match.entry.ContractID)
		})
	}
}

// ---------------------------------------------------------------------------
// /v1/flows/release
// ---------------------------------------------------------------------------

func TestHandleFlows_Release_HappyPath_NoPotNoRecipient(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailCoreState:    jsonArray(b.coreStateJSON()),
		tailReplayNode:   jsonArray(b.replayNodeJSON("")),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)

	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t, "release", resp.Flow)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleLockedLedger, roleCommittedTransferFactory},
		rolesOf(resp.Disclosures))
	assert.Empty(t, resp.Missing)
}

func TestHandleFlows_Release_HappyPath_WithPotAndRecipient(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	potCid := "pot-cid-1"
	fixtures := flowFixtures{
		tailNttManager:          jsonArray(b.managerJSON()),
		tailCoreState:           jsonArray(b.coreStateJSON()),
		tailReplayNode:          jsonArray(b.replayNodeJSON("")),
		tailLockedLedger:        jsonArray(b.ledgerJSON(&potCid)),
		tailCoinFactory:         jsonArray(b.factoryJSON()),
		tailCoin:                jsonArray(acsContractJSONWithArg("pkg:"+tailCoin, potCid, "Y29pbg==", "sync1", "{}")),
		tailTransferPreapproval: jsonArray(acsContractJSONWithArg("pkg:"+tailTransferPreapproval, "pre-cid-1", "cHJl", "sync1", preapprovalArg(testRecipient, b.instrumentAdmin, b.instrumentID))),
	}
	srv := newFlowTestServer(t, fixtures)

	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleLockedLedger, roleCustodyPotHolding, roleCommittedTransferFactory, roleRecipientTransferPreapproval},
		rolesOf(resp.Disclosures))
	assert.Empty(t, resp.Missing)
}

func TestHandleFlows_Release_PotHintedNotVisible_Missing(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	potCid := "pot-cid-missing"
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailCoreState:    jsonArray(b.coreStateJSON()),
		tailReplayNode:   jsonArray(b.replayNodeJSON("")),
		tailLockedLedger: jsonArray(b.ledgerJSON(&potCid)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)

	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Contains(t, missingRolesOf(resp.Missing), roleCustodyPotHolding)
}

func TestHandleFlows_Release_RecipientPreapprovalAbsent_Missing(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailCoreState:    jsonArray(b.coreStateJSON()),
		tailReplayNode:   jsonArray(b.replayNodeJSON("")),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)

	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Contains(t, missingRolesOf(resp.Missing), roleRecipientTransferPreapproval)
}

func TestHandleFlows_Release_UnknownManager_404(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: "[]"})
	rr := requestFlow(t, srv, "release", "manager="+strings.Repeat("11", 32)+"&digest="+strings.Repeat("22", 32))
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleFlows_Release_WrongMode_409(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())})
	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest)
	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "BurnMint")
}

func TestHandleFlows_CoveringNode_ZeroMatches_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailCoreState:    jsonArray(b.coreStateJSON()),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	logged := captureServerLog(srv)
	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest)
	requirePublicErrorWithLoggedDetail(t, rr, logged, http.StatusInternalServerError, roleCoveringReplayNode)
}

func TestHandleFlows_CoveringNode_TwoMatches_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager: jsonArray(b.managerJSON()),
		tailCoreState:  jsonArray(b.coreStateJSON()),
		tailReplayNode: jsonArray(
			b.replayNodeJSON(""),
			acsContractJSONWithArg("pkg:"+tailReplayNode, "node-cid-2", "bm9kZQ==", "sync1", replayNodeArg(b.gg, b.namespace, "")),
		),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// /v1/flows/mint
// ---------------------------------------------------------------------------

func TestHandleFlows_Mint_HappyPath(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	fixtures := flowFixtures{
		tailNttManager:         jsonArray(b.managerJSON()),
		tailCoreState:          jsonArray(b.coreStateJSON()),
		tailReplayNode:         jsonArray(b.replayNodeJSON("")),
		tailDepositPreapproval: jsonArray(acsContractJSONWithArg("pkg:"+tailDepositPreapproval, "dep-cid-1", "ZGVw", "sync1", preapprovalArg(testRecipient, b.instrumentAdmin, b.instrumentID))),
		tailCoinFactory:        jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "mint", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleDepositPreapproval, roleCommittedCanonicalCoinFactory},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Mint_DepositPreapprovalAbsent_Error(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	fixtures := flowFixtures{
		tailNttManager: jsonArray(b.managerJSON()),
		tailCoreState:  jsonArray(b.coreStateJSON()),
		tailReplayNode: jsonArray(b.replayNodeJSON("")),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "mint", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), roleDepositPreapproval)
}

func TestHandleFlows_Mint_MissingRecipientParam_400(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())})
	rr := requestFlow(t, srv, "mint", "manager="+b.managerAddr+"&digest="+b.digest)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleFlows_Mint_WrongMode_409(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())})
	rr := requestFlow(t, srv, "mint", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// ---------------------------------------------------------------------------
// /v1/flows/set-peer
// ---------------------------------------------------------------------------

func TestHandleFlows_SetPeer_HappyPath(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager: jsonArray(b.managerJSON()),
		tailCoreState:  jsonArray(b.coreStateJSON()),
		tailEmitter:    jsonArray(b.emitterJSON()),
		tailReplayNode: jsonArray(b.replayNodeJSON("")),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "set-peer", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleTransceiverEmitter, roleCoveringReplayNode},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_SetPeer_EmitterAbsent_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager: jsonArray(b.managerJSON()),
		tailCoreState:  jsonArray(b.coreStateJSON()),
		tailReplayNode: jsonArray(b.replayNodeJSON("")),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "set-peer", "manager="+b.managerAddr+"&digest="+b.digest)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// /v1/flows/accept-admin
// ---------------------------------------------------------------------------

func TestHandleFlows_AcceptAdmin_LockUnlock_WithPot(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	potCid := "pot-cid-2"
	fixtures := flowFixtures{
		tailNttManager:            jsonArray(b.managerJSON()),
		tailCoreState:             jsonArray(b.coreStateJSON()),
		tailReplayNode:            jsonArray(b.replayNodeJSON("")),
		tailAdminTransferProposal: jsonArray(acsContractJSONWithArg("pkg:"+tailAdminTransferProposal, "prop-cid-1", "cHJvcA==", "sync1", adminTransferProposalArg(b.admin, b.gg, b.managerAddr))),
		tailLockedLedger:          jsonArray(b.ledgerJSON(&potCid)),
		tailCoin:                  jsonArray(acsContractJSONWithArg("pkg:"+tailCoin, potCid, "Y29pbg==", "sync1", "{}")),
		tailCoinFactory:           jsonArray(b.factoryJSON()),
		tailTransferPreapproval:   jsonArray(b.transferPreapprovalJSON(b.admin), b.transferPreapprovalJSON(b.gg), b.transferPreapprovalJSON(b.nttCustody)),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleAdminTransferProposal, roleLockedLedger, roleCustodyPotHolding, roleCommittedTransferFactory, roleNewCustodianTransferPreapproval},
		rolesOf(resp.Disclosures))
	assert.Equal(t, "pre-"+b.nttCustody, resp.Disclosures[len(resp.Disclosures)-1].ContractID)
	assert.Empty(t, resp.Missing)
}

func TestHandleFlows_AcceptAdmin_LockUnlock_NewCustodianPreapprovalAbsent_Missing(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	potCid := "pot-cid-4"
	fixtures := flowFixtures{
		tailNttManager:            jsonArray(b.managerJSON()),
		tailCoreState:             jsonArray(b.coreStateJSON()),
		tailReplayNode:            jsonArray(b.replayNodeJSON("")),
		tailAdminTransferProposal: jsonArray(acsContractJSONWithArg("pkg:"+tailAdminTransferProposal, "prop-cid-3", "cHJvcA==", "sync1", adminTransferProposalArg(b.admin, b.gg, b.managerAddr))),
		tailLockedLedger:          jsonArray(b.ledgerJSON(&potCid)),
		tailCoin:                  jsonArray(acsContractJSONWithArg("pkg:"+tailCoin, potCid, "Y29pbg==", "sync1", "{}")),
		tailCoinFactory:           jsonArray(b.factoryJSON()),
		tailTransferPreapproval:   jsonArray(b.transferPreapprovalJSON(b.gg)),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.NotContains(t, rolesOf(resp.Disclosures), roleNewCustodianTransferPreapproval)
	assert.Equal(t, []string{roleNewCustodianTransferPreapproval}, missingRolesOf(resp.Missing))
}

func TestHandleFlows_AcceptAdmin_LockUnlock_EmptyPot_NoNewCustodianPreapproval(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:            jsonArray(b.managerJSON()),
		tailCoreState:             jsonArray(b.coreStateJSON()),
		tailReplayNode:            jsonArray(b.replayNodeJSON("")),
		tailAdminTransferProposal: jsonArray(acsContractJSONWithArg("pkg:"+tailAdminTransferProposal, "prop-cid-4", "cHJvcA==", "sync1", adminTransferProposalArg(b.admin, b.gg, b.managerAddr))),
		tailLockedLedger:          jsonArray(b.ledgerJSON(nil)),
		tailTransferPreapproval:   jsonArray(b.transferPreapprovalJSON(b.nttCustody)),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleAdminTransferProposal, roleLockedLedger},
		rolesOf(resp.Disclosures))
	assert.Empty(t, resp.Missing)
}

func TestHandleFlows_AcceptAdmin_BurnMint_NoLedger(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	fixtures := flowFixtures{
		tailNttManager:            jsonArray(b.managerJSON()),
		tailCoreState:             jsonArray(b.coreStateJSON()),
		tailReplayNode:            jsonArray(b.replayNodeJSON("")),
		tailAdminTransferProposal: jsonArray(acsContractJSONWithArg("pkg:"+tailAdminTransferProposal, "prop-cid-2", "cHJvcA==", "sync1", adminTransferProposalArg(b.admin, b.gg, b.managerAddr))),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleAdminTransferProposal},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_AcceptAdmin_ProposalAbsent_Missing(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	fixtures := flowFixtures{
		tailNttManager: jsonArray(b.managerJSON()),
		tailCoreState:  jsonArray(b.coreStateJSON()),
		tailReplayNode: jsonArray(b.replayNodeJSON("")),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Contains(t, missingRolesOf(resp.Missing), roleAdminTransferProposal)
}

// ---------------------------------------------------------------------------
// /v1/flows/transfer
// ---------------------------------------------------------------------------

func TestHandleFlows_Transfer_LockUnlock(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	potCid := "pot-cid-3"
	fixtures := flowFixtures{
		tailNttManager:          jsonArray(b.managerJSON()),
		tailEmitter:             jsonArray(b.emitterJSON()),
		tailCoreState:           jsonArray(b.coreStateJSON()),
		tailLockedLedger:        jsonArray(b.ledgerJSON(&potCid)),
		tailCoin:                jsonArray(acsContractJSONWithArg("pkg:"+tailCoin, potCid, "Y29pbg==", "sync1", "{}")),
		tailCoinFactory:         jsonArray(b.factoryJSON()),
		tailTransferPreapproval: jsonArray(b.transferPreapprovalJSON(b.reserveCustodian())),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "transfer", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleTransceiverEmitter, roleCoreState, roleLockedLedger, roleCustodyPotHolding, roleCommittedTransferFactory, roleCustodianTransferPreapproval},
		rolesOf(resp.Disclosures))
	assert.Empty(t, resp.Missing)
}

func TestHandleFlows_Transfer_LockUnlock_CustodianPreapprovalAbsent_Missing(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailEmitter:      jsonArray(b.emitterJSON()),
		tailCoreState:    jsonArray(b.coreStateJSON()),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "transfer", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Contains(t, missingRolesOf(resp.Missing), roleCustodianTransferPreapproval)
}

func TestHandleFlows_Transfer_BurnMint(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	fixtures := flowFixtures{
		tailNttManager:  jsonArray(b.managerJSON()),
		tailEmitter:     jsonArray(b.emitterJSON()),
		tailCoreState:   jsonArray(b.coreStateJSON()),
		tailCoinFactory: jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "transfer", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleTransceiverEmitter, roleCoreState, roleCommittedCanonicalCoinFactory},
		rolesOf(resp.Disclosures))
}

// ---------------------------------------------------------------------------
// /v1/flows/register
// ---------------------------------------------------------------------------

func TestHandleFlows_Register_Basic(t *testing.T) {
	const gg = testGuardianGovernanceParty
	fixtures := flowFixtures{
		tailNttGovernance:      jsonArray(acsContractJSONWithArg("pkg:"+tailNttGovernance, "gov-cid-1", "Z292", "sync1", guardianAnchorArg(gg))),
		tailEmitterRegistry:    jsonArray(acsContractJSONWithArg("pkg:"+tailEmitterRegistry, "emreg-cid-1", "ZW1yZWc=", "sync1", guardianAnchorArg(gg))),
		tailReplayRootRegistry: jsonArray(acsContractJSONWithArg("pkg:"+tailReplayRootRegistry, "rrreg-cid-1", "cnJyZWc=", "sync1", guardianAnchorArg(gg))),
		tailCoreState:          jsonArray(acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-2", "Y3M=", "sync1", guardianAnchorArg(gg))),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "register", "")
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttGovernance, roleEmitterRegistry, roleReplayRootRegistry, roleCoreState},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Register_ByVaa(t *testing.T) {
	const gg = testGuardianGovernanceParty
	fixtures := flowFixtures{
		tailNttGovernance:      jsonArray(acsContractJSONWithArg("pkg:"+tailNttGovernance, "gov-cid-2", "Z292", "sync1", guardianAnchorArg(gg))),
		tailEmitterRegistry:    jsonArray(acsContractJSONWithArg("pkg:"+tailEmitterRegistry, "emreg-cid-2", "ZW1yZWc=", "sync1", guardianAnchorArg(gg))),
		tailReplayRootRegistry: jsonArray(acsContractJSONWithArg("pkg:"+tailReplayRootRegistry, "rrreg-cid-2", "cnJyZWc=", "sync1", guardianAnchorArg(gg))),
		tailCoreState:          jsonArray(acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-3", "Y3M=", "sync1", guardianAnchorArg(gg))),
		tailCoinFactory:        jsonArray(acsContractJSONWithArg("pkg:"+tailCoinFactory, "canon-factory-cid", "ZmFjdA==", "sync1", coinFactoryArg(gg))),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "register", "gg="+gg+"&by-vaa=true")
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttGovernance, roleEmitterRegistry, roleReplayRootRegistry, roleCoreState, roleCanonicalCoinFactory},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Register_ByVaaWithoutGG_400(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{})
	rr := requestFlow(t, srv, "register", "by-vaa=true")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleFlows_Register_TwoCoreStatesNoGG_409(t *testing.T) {
	fixtures := flowFixtures{
		tailNttGovernance:      jsonArray(acsContractJSONWithArg("pkg:"+tailNttGovernance, "gov-cid-3", "Z292", "sync1", guardianAnchorArg("gg::1"))),
		tailEmitterRegistry:    jsonArray(acsContractJSONWithArg("pkg:"+tailEmitterRegistry, "emreg-cid-3", "ZW1yZWc=", "sync1", guardianAnchorArg("gg::1"))),
		tailReplayRootRegistry: jsonArray(acsContractJSONWithArg("pkg:"+tailReplayRootRegistry, "rrreg-cid-3", "cnJyZWc=", "sync1", guardianAnchorArg("gg::1"))),
		tailCoreState: jsonArray(
			acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-4", "Y3M=", "sync1", guardianAnchorArg("gg::1")),
			acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-5", "Y3M=", "sync1", guardianAnchorArg("gg::2")),
		),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "register", "")
	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), roleCoreState)
}

// ---------------------------------------------------------------------------
// /v1/flows/consolidate
// ---------------------------------------------------------------------------

func TestHandleFlows_Consolidate_HappyPath(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleLockedLedger, roleCommittedTransferFactory},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Consolidate_WrongMode_409(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())})
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestHandleFlows_Consolidate_LedgerAbsent_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())})
	logged := captureServerLog(srv)
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	requirePublicErrorWithLoggedDetail(t, rr, logged, http.StatusInternalServerError, roleLockedLedger)
}

func TestHandleFlows_Consolidate_FactoryNotVisible_Missing(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	b.instrumentAdmin = "dso::1"
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{roleCommittedTransferFactory}, missingRolesOf(decodeFlowResponse(t, rr).Missing))
}

func TestHandleFlows_Consolidate_WrongFactoryTag_409(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	b.factoryTag = factoryTagBurnMint
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
		tailCoinFactory:  jsonArray(b.factoryJSON()),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
}

// ---------------------------------------------------------------------------
// /v1/flows/{flow}: cross-cutting
// ---------------------------------------------------------------------------

func TestHandleFlows_UnknownFlow_404(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{})
	rr := requestFlow(t, srv, "not-a-flow", "manager=x")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleFlows_MethodNotAllowed(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/release", nil)
	srv.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleFlows_MissingManagerParam_400(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{})
	rr := requestFlow(t, srv, "release", "digest="+strings.Repeat("11", 32))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleFlows_BadHexParam_400(t *testing.T) {
	validHex := strings.Repeat("11", 32)
	cases := []struct {
		name  string
		query string
	}{
		{"manager wrong length", "manager=abcd&digest=" + validHex},
		{"manager non-hex characters", "manager=" + strings.Repeat("zz", 32) + "&digest=" + validHex},
		{"digest wrong length", "manager=" + validHex + "&digest=abcd"},
		{"digest non-hex characters", "manager=" + validHex + "&digest=" + strings.Repeat("zz", 32)},
	}
	srv := newFlowTestServer(t, flowFixtures{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := requestFlow(t, srv, "release", tc.query)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestHandleIndex(t *testing.T) {
	srv := newServer(&options{readerParties: testReaderParties(), maxContractsPerTemplate: 10}, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "/v1/managers")
	assert.Contains(t, rr.Body.String(), "/v1/flows/{flow}")
	assert.Contains(t, rr.Body.String(), "accept-admin")
	assert.NotContains(t, rr.Body.String(), "/v1/disclosures")
}

func TestHandleManagers(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	second := acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-2", "bWdy", "sync1",
		nttManagerArg(b.gg, b.gg, b.nttCustody, b.nttCustody, "ns2", strings.Repeat("b", 64), 8, b.gg, "otherCoin", tokenConfigBurnMint, factoryTagBurnMint, "cid-f2"))
	fixtures := map[string]string{
		tailNttManager: jsonArray(b.managerJSON(), second),
	}
	srv := newFlowTestServer(t, fixtures)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/managers", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var got struct {
		Managers []managerEntry `json:"managers"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Len(t, got.Managers, 2)
	assert.Equal(t, b.managerAddr, got.Managers[0].ManagerAddress)
	assert.Equal(t, strings.Repeat("b", 64), got.Managers[1].ManagerAddress)
	assert.Equal(t, tokenConfigBurnMint, got.Managers[1].TokenConfig)
	assert.Equal(t, "otherCoin", got.Managers[1].InstrumentId.Id)
	assert.Equal(t, partyID(b.admin), got.Managers[0].ReserveCustodian)
	assert.Equal(t, partyID(b.nttCustody), got.Managers[1].ReserveCustodian)
}

// ---------------------------------------------------------------------------
// ntt 0.4.0: reserveCustodian, nttCustody, canonical CoinFactory
// ---------------------------------------------------------------------------

func TestReserveCustodianFor(t *testing.T) {
	cases := []struct {
		name        string
		admin, want partyID
	}{
		{"third-party admin keeps its reserve", "admin::1", "admin::1"},
		{"gg admin hands the reserve to nttCustody", "gg::1", "nttCustody::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, reserveCustodianFor("gg::1", "nttCustody::1", tc.admin))
		})
	}
}

func TestCheckReserveCustodianFollowsAdmin(t *testing.T) {
	valid := daNttManager{Admin: "admin::1", GuardianGovernance: "gg::1", NttCustody: "nttCustody::1", ReserveCustodian: "admin::1"}
	cases := []struct {
		name      string
		mutate    func(*daNttManager)
		wantError string
	}{
		{"third-party admin", func(*daNttManager) {}, ""},
		{"gg admin with nttCustody reserve", func(m *daNttManager) { m.Admin, m.ReserveCustodian = "gg::1", "nttCustody::1" }, ""},
		{"payload before ntt 0.4.0", func(m *daNttManager) { m.NttCustody, m.ReserveCustodian = "", "" }, "no nttCustody"},
		{"reserveCustodian absent", func(m *daNttManager) { m.ReserveCustodian = "" }, "no reserveCustodian"},
		{"reserveCustodian is not the admin", func(m *daNttManager) { m.ReserveCustodian = "nttCustody::1" }, "expected admin::1"},
		{"gg admin keeps a gg reserve", func(m *daNttManager) { m.Admin, m.ReserveCustodian = "gg::1", "gg::1" }, "expected nttCustody::1"},
		{"admin is nttCustody", func(m *daNttManager) { m.Admin, m.ReserveCustodian = "nttCustody::1", "nttCustody::1" }, "admin is nttCustody"},
		{"nttCustody is gg", func(m *daNttManager) { m.NttCustody = "gg::1" }, "nttCustody is guardianGovernance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := valid
			tc.mutate(&mgr)
			err := checkReserveCustodianFollowsAdmin(mgr)
			if tc.wantError == "" {
				require.NoError(t, err)
				return
			}
			requireFlowErrorStatus(t, err, http.StatusInternalServerError)
			assert.Contains(t, err.Error(), tc.wantError)
		})
	}
}

func TestHandleFlows_Transfer_LockUnlock_CustodianPreapprovalIsReserveCustodians(t *testing.T) {
	cases := []struct {
		name               string
		admin              string
		wantPreapprovalCid string
	}{
		{"third-party admin", testRecipient, "pre-" + testRecipient},
		{"gg admin", testGuardianGovernanceParty, "pre-" + testNttCustodyParty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBaseFixturesAdminedBy(tokenConfigLockUnlock, tc.admin)
			fixtures := flowFixtures{
				tailNttManager:          jsonArray(b.managerJSON()),
				tailEmitter:             jsonArray(b.emitterJSON()),
				tailCoreState:           jsonArray(b.coreStateJSON()),
				tailLockedLedger:        jsonArray(b.ledgerJSON(nil)),
				tailCoinFactory:         jsonArray(b.factoryJSON()),
				tailTransferPreapproval: jsonArray(b.transferPreapprovalJSON(testRecipient), b.transferPreapprovalJSON(b.gg), b.transferPreapprovalJSON(b.nttCustody)),
			}
			srv := newFlowTestServer(t, fixtures)
			rr := requestFlow(t, srv, "transfer", "manager="+b.managerAddr)
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			resp := decodeFlowResponse(t, rr)
			last := resp.Disclosures[len(resp.Disclosures)-1]
			assert.Equal(t, roleCustodianTransferPreapproval, last.Role)
			assert.Equal(t, tc.wantPreapprovalCid, last.ContractID)
			assert.NotEqual(t, "pre-"+b.gg, last.ContractID)
		})
	}
}

func TestHandleFlows_BurnMint_CommittedFactoryNotGuardianGovernance_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	fixtures := flowFixtures{
		tailNttManager:         jsonArray(b.managerJSON()),
		tailEmitter:            jsonArray(b.emitterJSON()),
		tailCoreState:          jsonArray(b.coreStateJSON()),
		tailReplayNode:         jsonArray(b.replayNodeJSON("")),
		tailDepositPreapproval: jsonArray(acsContractJSONWithArg("pkg:"+tailDepositPreapproval, "dep-cid-2", "ZGVw", "sync1", preapprovalArg(testRecipient, b.instrumentAdmin, b.instrumentID))),
		tailCoinFactory:        jsonArray(acsContractJSONWithArg("pkg:"+tailCoinFactory, b.factoryCid, "ZmFjdA==", "sync1", coinFactoryArg("external-minter::1"))),
	}
	cases := []struct {
		flow, query string
	}{
		{"mint", "manager=" + b.managerAddr + "&digest=" + b.digest + "&recipient=" + testRecipient},
		{"transfer", "manager=" + b.managerAddr},
	}
	srv := newFlowTestServer(t, fixtures)
	for _, tc := range cases {
		t.Run(tc.flow, func(t *testing.T) {
			logged := captureServerLog(srv)
			rr := requestFlow(t, srv, tc.flow, tc.query)
			requirePublicErrorWithLoggedDetail(t, rr, logged, http.StatusInternalServerError, "expected guardianGovernance")
		})
	}
}

func TestHandleFlows_ManagerPayloadBeforeNtt040_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager: jsonArray(acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-old", "bWdy", "sync1",
			nttManagerArgBeforeNtt040(b.admin, b.gg, b.managerAddr, tokenConfigLockUnlock, factoryTagTransfer, b.factoryCid))),
	}
	cases := []struct {
		flow, query string
	}{
		{"release", "manager=" + b.managerAddr + "&digest=" + b.digest},
		{"set-peer", "manager=" + b.managerAddr + "&digest=" + b.digest},
		{"accept-admin", "manager=" + b.managerAddr + "&digest=" + b.digest},
		{"transfer", "manager=" + b.managerAddr},
		{"consolidate", "manager=" + b.managerAddr},
	}
	srv := newFlowTestServer(t, fixtures)
	for _, tc := range cases {
		t.Run(tc.flow, func(t *testing.T) {
			logged := captureServerLog(srv)
			rr := requestFlow(t, srv, tc.flow, tc.query)
			requirePublicErrorWithLoggedDetail(t, rr, logged, http.StatusInternalServerError, "ntt 0.4.0 or later is required")
		})
	}
}

// ---------------------------------------------------------------------------
// Canton Coin (Amulet) lock/unlock: the registry TransferFactory is a DSO contract the disclosing
// parties cannot see; the reserve pot is an Amulet owned by the reserve custodian.
// ---------------------------------------------------------------------------

func TestHandleFlows_AmuletLockUnlock(t *testing.T) {
	b := newBaseFixturesAdminedBy(tokenConfigLockUnlock, "gg::1")
	b.instrumentAdmin = "dso::1"
	b.instrumentID = "Amulet"
	b.factoryCid = "amulet-external-party-rules-cid"
	potCid := "amulet-pot-cid"
	amuletPot := acsContractJSONWithArg("pkg:"+tailAmulet, potCid, "YW11bGV0", "sync1", "{}")
	cases := []struct {
		name            string
		flow, query     string
		wantDisclosures []string
		wantMissing     []string
	}{
		{
			name:            "release",
			flow:            "release",
			query:           "manager=" + b.managerAddr + "&digest=" + b.digest + "&recipient=" + testRecipient,
			wantDisclosures: []string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleLockedLedger, roleCustodyPotHolding},
			wantMissing:     []string{roleCommittedTransferFactory, roleRecipientTransferPreapproval},
		},
		{
			name:            "transfer",
			flow:            "transfer",
			query:           "manager=" + b.managerAddr,
			wantDisclosures: []string{roleNttManager, roleTransceiverEmitter, roleCoreState, roleLockedLedger, roleCustodyPotHolding},
			wantMissing:     []string{roleCommittedTransferFactory, roleCustodianTransferPreapproval},
		},
		{
			name:            "consolidate",
			flow:            "consolidate",
			query:           "manager=" + b.managerAddr,
			wantDisclosures: []string{roleNttManager, roleLockedLedger},
			wantMissing:     []string{roleCommittedTransferFactory},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := flowFixtures{
				tailNttManager:   jsonArray(b.managerJSON()),
				tailEmitter:      jsonArray(b.emitterJSON()),
				tailCoreState:    jsonArray(b.coreStateJSON()),
				tailReplayNode:   jsonArray(b.replayNodeJSON("")),
				tailLockedLedger: jsonArray(b.ledgerJSON(&potCid)),
				tailAmulet:       jsonArray(amuletPot),
			}
			srv := newFlowTestServer(t, fixtures)
			rr := requestFlow(t, srv, tc.flow, tc.query)
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			resp := decodeFlowResponse(t, rr)
			assert.Equal(t, tc.wantDisclosures, rolesOf(resp.Disclosures))
			assert.Equal(t, tc.wantMissing, missingRolesOf(resp.Missing))
			for _, d := range resp.Disclosures {
				if d.Role == roleCustodyPotHolding {
					assert.Equal(t, potCid, d.ContractID)
				}
			}
		})
	}
}

func TestHandleFlows_BurnMint_CanonicalCoinFactoryNotVisible_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	cases := []struct {
		flow, query string
	}{
		{"mint", "manager=" + b.managerAddr + "&digest=" + b.digest + "&recipient=" + testRecipient},
		{"transfer", "manager=" + b.managerAddr},
	}
	for _, tc := range cases {
		t.Run(tc.flow, func(t *testing.T) {
			fixtures := flowFixtures{
				tailNttManager:         jsonArray(b.managerJSON()),
				tailEmitter:            jsonArray(b.emitterJSON()),
				tailCoreState:          jsonArray(b.coreStateJSON()),
				tailReplayNode:         jsonArray(b.replayNodeJSON("")),
				tailDepositPreapproval: jsonArray(acsContractJSONWithArg("pkg:"+tailDepositPreapproval, "dep-cid-1", "ZGVw", "sync1", preapprovalArg(testRecipient, b.instrumentAdmin, b.instrumentID))),
			}
			srv := newFlowTestServer(t, fixtures)
			rr := requestFlow(t, srv, tc.flow, tc.query)
			assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		})
	}
}
