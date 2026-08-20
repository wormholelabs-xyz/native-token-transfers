package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestParseFlags_Defaults(t *testing.T) {
	opts, err := parseFlags([]string{"--json-api", "http://localhost:6975", "--disclosing-party", "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:7599", opts.listen)
	assert.Equal(t, "http://localhost:6975", opts.jsonAPIBaseURL)
	assert.Equal(t, "Alice", opts.disclosingParty)
	assert.Equal(t, "", opts.accessTokenFile)
	assert.Equal(t, "", opts.allowListPath)
	assert.Equal(t, 1000, opts.maxContractsPerTemplate)
	assert.False(t, opts.verbose)
}

func TestParseFlags_MaxContractsPerTemplate(t *testing.T) {
	opts, err := parseFlags([]string{
		"--json-api", "http://localhost:6975", "--disclosing-party", "Alice",
		"--max-contracts-per-template", "50000",
	})
	require.NoError(t, err)
	assert.Equal(t, 50000, opts.maxContractsPerTemplate)

	srv := newServer(opts, defaultAllowList(), nil)
	assert.Equal(t, 50000, srv.maxContracts)
}

func TestParseFlags_MaxContractsPerTemplate_RejectsNonPositive(t *testing.T) {
	for _, bad := range []string{"0", "-1"} {
		t.Run(bad, func(t *testing.T) {
			_, err := parseFlags([]string{
				"--json-api", "http://localhost:6975", "--disclosing-party", "Alice",
				"--max-contracts-per-template", bad,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--max-contracts-per-template")
		})
	}
}

func TestParseFlags_MissingRequired(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantErrSub string
	}{
		{
			name:       "missing json-api",
			args:       []string{"--disclosing-party", "Alice"},
			wantErrSub: "--json-api",
		},
		{
			name:       "missing party",
			args:       []string{"--json-api", "http://localhost:6975"},
			wantErrSub: "--disclosing-party",
		},
		{
			name:       "missing both",
			args:       []string{},
			wantErrSub: "--json-api",
		},
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
// Allow-list
// ---------------------------------------------------------------------------

func TestDefaultAllowList(t *testing.T) {
	// This list is package-qualified. The JSON Ledger API v2 returns HTTP 400 for unqualified
	// "Module:Entity" template filters (confirmed live).
	want := []string{
		"#ntt:Wormhole.Ntt.Manager:NttManager",
		"#ntt:Wormhole.Ntt.Manager:AdminTransferProposal",
		"#ntt:Wormhole.Ntt.Ledger:LockedLedger",
		"#ntt:Wormhole.Ntt.Deposit:DepositPreapproval",
		"#ntt:Wormhole.Ntt.Governance:NttGovernance",
		"#wormhole-core:Wormhole.Core.State:CoreState",
		"#wormhole-core:Wormhole.Core.State:Emitter",
		"#wormhole-core:Wormhole.Core.State:EmitterRegistry",
		"#wormhole-core:Wormhole.Core.State:ReplayRootRegistry",
		"#wormhole-core:Wormhole.Core.Replay:ReplayNode",
		"#token-cip0056:Token.CIP0056.CoinFactory:CoinFactory",
		"#token-cip0056:Token.CIP0056.Coin:Coin",
		"#token-cip0056:Token.CIP0056.CoinTransfer:TransferPreapproval",
	}
	assert.ElementsMatch(t, want, defaultAllowList())
}

// ---------------------------------------------------------------------------
// templateTail / templateIDMatches
// ---------------------------------------------------------------------------

func TestTemplateIDMatches(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"exact tail equality", "Module:Entity", "Module:Entity", true},
		{"package-id-qualified vs bare tail", "abcdef1234:Module:Entity", "Module:Entity", true},
		{"package-name-qualified vs bare tail", "#ntt:Module:Entity", "Module:Entity", true},
		{"both qualified, different prefixes", "abcdef1234:Module:Entity", "#ntt:Module:Entity", true},
		{"different entity", "abcdef1234:Module:Entity", "Module:Other", false},
		{"different module", "abcdef1234:Module:Entity", "Other:Entity", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, templateIDMatches(tc.a, tc.b))
		})
	}
}

func TestLoadAllowList(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid", func(t *testing.T) {
		p := filepath.Join(dir, "valid.json")
		require.NoError(t, os.WriteFile(p, []byte(`["#p:A.B:C", "#q:D.E:F"]`), 0o600))
		list, err := loadAllowList(p)
		require.NoError(t, err)
		assert.Equal(t, []string{"#p:A.B:C", "#q:D.E:F"}, list)
	})

	t.Run("unqualified entry rejected", func(t *testing.T) {
		p := filepath.Join(dir, "unqualified.json")
		require.NoError(t, os.WriteFile(p, []byte(`["A.B:C"]`), 0o600))
		_, err := loadAllowList(p)
		require.ErrorContains(t, err, "package-qualified")
	})

	t.Run("duplicate tail rejected", func(t *testing.T) {
		p := filepath.Join(dir, "dup.json")
		require.NoError(t, os.WriteFile(p, []byte(`["#p:A.B:C", "#q:A.B:C"]`), 0o600))
		_, err := loadAllowList(p)
		require.ErrorContains(t, err, "share the tail")
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadAllowList(filepath.Join(dir, "nope.json"))
		require.Error(t, err)
	})

	t.Run("empty array rejected", func(t *testing.T) {
		p := filepath.Join(dir, "empty.json")
		require.NoError(t, os.WriteFile(p, []byte(`[]`), 0o600))
		_, err := loadAllowList(p)
		require.Error(t, err)
	})

	t.Run("malformed json", func(t *testing.T) {
		p := filepath.Join(dir, "bad.json")
		require.NoError(t, os.WriteFile(p, []byte(`not json`), 0o600))
		_, err := loadAllowList(p)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Fake upstream JSON Ledger API v2
// ---------------------------------------------------------------------------

// newFakeJSONAPI starts an httptest server that stands in for the JSON Ledger API v2. It answers
// /v2/state/ledger-end with a fixed offset and routes /v2/state/active-contracts to acHandler.
// The returned capture holds the most recent Authorization header from both endpoints, for the
// bearer-token test.
func newFakeJSONAPI(t *testing.T, acHandler http.HandlerFunc) (*httptest.Server, *authCapture) {
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

// decodeRequestedTemplate pulls the single requested templateId (and disclosing party) out of an
// active-contracts request body. It matches activeContractsRequest's shape in main.go.
func decodeRequestedTemplate(t *testing.T, r *http.Request) (party, template string) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var req activeContractsRequest
	require.NoError(t, json.Unmarshal(body, &req))
	require.Len(t, req.EventFormat.FiltersByParty, 1, "expected exactly one party filter")
	for p, pf := range req.EventFormat.FiltersByParty {
		party = p
		require.Len(t, pf.Cumulative, 1, "expected exactly one cumulative filter")
		require.True(t, pf.Cumulative[0].IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob)
		template = pf.Cumulative[0].IdentifierFilter.TemplateFilter.Value.TemplateID
	}
	return party, template
}

// acsContractJSON renders one active-contracts response element in the JSON Ledger API v2 wire
// shape (contractEntry.JsActiveContract{createdEvent, synchronizerId, ...}). It is independent
// of main.go's internal decode types, so the test pins the actual wire contract as ground truth.
// synchronizerId sits beside createdEvent, at the same level (confirmed live).
func acsContractJSON(templateID, contractID, blob, synchronizerID string) string {
	return fmt.Sprintf(
		`{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":%q,"templateId":%q,"createdEventBlob":%q},"synchronizerId":%q}}}`,
		contractID, templateID, blob, synchronizerID,
	)
}

// ---------------------------------------------------------------------------
// /v1/disclosures
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T, upstream *httptest.Server, party string, allowList []string) *server {
	t.Helper()
	opts := &options{disclosingParty: party, jsonAPIBaseURL: upstream.URL, maxContractsPerTemplate: defaultMaxContractsPerTemplate}
	acs := newHTTPACSClient(upstream.URL, "")
	return newServer(opts, allowList, acs)
}

func TestHandleDisclosures_RejectsUnknownTemplate(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("active-contracts must not be queried for a rejected request")
	})
	srv := newTestServer(t, upstream, "Alice", []string{"#ntt:Wormhole.Ntt.Manager:NttManager"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template=Evil.Module:Backdoor", nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Evil.Module:Backdoor")
}

// TestHandleDisclosures_TailOnlyClientRequestForwardedQualified pins the 403 gate's matching
// rule. A client may ask by bare "Module:Entity" tail. The gate matches it against the
// allow-list by tail and forwards the CONFIGURED, package-qualified name upstream.
func TestHandleDisclosures_TailOnlyClientRequestForwardedQualified(t *testing.T) {
	const tail = "Wormhole.Ntt.Manager:NttManager"
	const qualified = "#ntt:" + tail
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, tmpl := decodeRequestedTemplate(t, r)
		assert.Equal(t, qualified, tmpl)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})
	srv := newTestServer(t, upstream, "Alice", []string{qualified})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template="+tail, nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDisclosures_HappyPath(t *testing.T) {
	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		party, tmpl := decodeRequestedTemplate(t, r)
		assert.Equal(t, "Alice", party)
		assert.Equal(t, template, tmpl)
		body := "[" +
			acsContractJSON("pkg123:"+template, "cid1", "YmxvYjE=", "sync1") + "," +
			acsContractJSON("pkg123:"+template, "cid2", "YmxvYjI=", "sync1") +
			"]"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	srv := newTestServer(t, upstream, "Alice", []string{template})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template="+template, nil)
	srv.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got []discloseEntry
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "cid1", got[0].ContractID)
	assert.Equal(t, "YmxvYjE=", got[0].CreatedEventBlob)
	assert.Equal(t, "pkg123:"+template, got[0].TemplateID)
	assert.Equal(t, "sync1", got[0].SynchronizerID)
	assert.Equal(t, "cid2", got[1].ContractID)
}

func TestHandleDisclosures_MultipleTemplates(t *testing.T) {
	const tA = "#wormhole-core:Wormhole.Core.State:CoreState"
	const tB = "#wormhole-core:Wormhole.Core.State:Emitter"
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, tmpl := decodeRequestedTemplate(t, r)
		body := "[" + acsContractJSON("pkg1:"+tmpl, "cid-"+tmpl, "Yg==", "") + "]"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	srv := newTestServer(t, upstream, "Alice", []string{tA, tB})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template="+tA+"&template="+tB, nil)
	srv.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got []discloseEntry
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Len(t, got, 2)
	templates := []string{got[0].TemplateID, got[1].TemplateID}
	assert.Contains(t, templates, "pkg1:"+tA)
	assert.Contains(t, templates, "pkg1:"+tB)
}

func TestHandleDisclosures_WholeAllowListDefault(t *testing.T) {
	allowList := defaultAllowList()
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, tmpl := decodeRequestedTemplate(t, r)
		body := "[" + acsContractJSON("pkg1:"+tmpl, "cid-"+tmpl, "Yg==", "") + "]"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	srv := newTestServer(t, upstream, "Alice", allowList)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures", nil)
	srv.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got []discloseEntry
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Len(t, got, len(allowList))
}

func TestHandleDisclosures_UpstreamError(t *testing.T) {
	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := newTestServer(t, upstream, "Alice", []string{template})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template="+template, nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
}

func TestHandleDisclosures_OverflowCapRejectsRatherThanTruncates(t *testing.T) {
	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		items := make([]string, 0, 3)
		for i := 0; i < 3; i++ {
			items = append(items, acsContractJSON("pkg1:"+template, fmt.Sprintf("cid%d", i), "Yg==", ""))
		}
		body := "[" + strings.Join(items, ",") + "]"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	srv := newTestServer(t, upstream, "Alice", []string{template})
	srv.maxContracts = 2 // force the cap below the fake's 3 contracts

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template="+template, nil)
	srv.ServeHTTP(rr, req)

	// The server rejects the request outright with 502 when the count exceeds the cap.
	assert.NotEqual(t, http.StatusOK, rr.Code)
	assert.Equal(t, http.StatusBadGateway, rr.Code)
}

func TestHandleDisclosures_EmptyResultIsValid(t *testing.T) {
	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})
	srv := newTestServer(t, upstream, "Alice", []string{template})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template="+template, nil)
	srv.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, "[]", rr.Body.String())
}

func TestHandleDisclosures_MismatchedUpstreamTemplateRejected(t *testing.T) {
	const template = "#ntt:Wormhole.Ntt.Manager:NttManager"
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		// Upstream returns a contract under a completely different template, even though it
		// came back on the requested-template query. The handler still verifies it.
		body := "[" + acsContractJSON("pkg1:Some.Other:Thing", "cid1", "Yg==", "") + "]"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	srv := newTestServer(t, upstream, "Alice", []string{template})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template="+template, nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
}

func TestHandleDisclosures_MethodNotAllowed(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("must not reach upstream on a rejected method")
	})
	srv := newTestServer(t, upstream, "Alice", defaultAllowList())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/disclosures", nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestUnknownPathReturns404(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {})
	srv := newTestServer(t, upstream, "Alice", defaultAllowList())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/nonsense", nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---------------------------------------------------------------------------
// /v1/healthz
// ---------------------------------------------------------------------------

func TestHandleHealthz(t *testing.T) {
	allowList := []string{"A.B:C", "D.E:F"}
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {})
	srv := newTestServer(t, upstream, "Alice", allowList)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	srv.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got healthzResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, "Alice", got.DisclosingParty)
	assert.Equal(t, 2, got.Templates)
	assert.Equal(t, int64(42), got.LedgerEnd) // from the fake's fixed ledger-end offset
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

	client := newHTTPACSClient(upstream.URL, tokenPath)
	_, err = client.ActiveContracts(context.Background(), "Alice", template, 42)
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

	client := newHTTPACSClient(upstream.URL, tokenPath)
	_, err := client.ActiveContracts(context.Background(), "Alice", template, 42)
	require.NoError(t, err)
	assert.Equal(t, "Bearer first-token", capture.get())

	require.NoError(t, os.WriteFile(tokenPath, []byte("second-token\n"), 0o600))

	_, err = client.ActiveContracts(context.Background(), "Alice", template, 42)
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
		jsonAPIBaseURL:          upstream.URL,
		disclosingParty:         "Alice",
		maxContractsPerTemplate: defaultMaxContractsPerTemplate,
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
	assert.Contains(t, banner, "Alice")
	assert.Contains(t, banner, "unauthenticated")
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
