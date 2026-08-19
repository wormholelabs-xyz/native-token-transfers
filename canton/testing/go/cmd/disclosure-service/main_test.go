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
	opts, err := parseFlags([]string{"--json-api", "http://localhost:6975", "--party", "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:7599", opts.listen)
	assert.Equal(t, "http://localhost:6975", opts.jsonAPIBaseURL)
	assert.Equal(t, "Alice", opts.party)
	assert.Equal(t, "", opts.accessTokenFile)
	assert.Equal(t, "", opts.allowListPath)
	assert.False(t, opts.verbose)
}

func TestParseFlags_MissingRequired(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantErrSub string
	}{
		{
			name:       "missing json-api",
			args:       []string{"--party", "Alice"},
			wantErrSub: "--json-api",
		},
		{
			name:       "missing party",
			args:       []string{"--json-api", "http://localhost:6975"},
			wantErrSub: "--party",
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
	want := []string{
		"Wormhole.Ntt.Manager:NttManager",
		"Wormhole.Ntt.Manager:AdminTransferProposal",
		"Wormhole.Ntt.Ledger:LockedLedger",
		"Wormhole.Ntt.Deposit:DepositPreapproval",
		"Wormhole.Ntt.Governance:NttGovernance",
		"Wormhole.Core.State:CoreState",
		"Wormhole.Core.State:Emitter",
		"Wormhole.Core.State:EmitterRegistry",
		"Wormhole.Core.State:ReplayRootRegistry",
		"Wormhole.Core.Replay:ReplayNode",
		"Token.CIP0056.CoinFactory:CoinFactory",
		"Token.CIP0056.Coin:Coin",
	}
	assert.ElementsMatch(t, want, defaultAllowList())
}

func TestLoadAllowList(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid", func(t *testing.T) {
		p := filepath.Join(dir, "valid.json")
		require.NoError(t, os.WriteFile(p, []byte(`["A.B:C", "D.E:F"]`), 0o600))
		list, err := loadAllowList(p)
		require.NoError(t, err)
		assert.Equal(t, []string{"A.B:C", "D.E:F"}, list)
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

// newFakeJSONAPI starts an httptest server standing in for the JSON Ledger API v2. It always
// answers /v2/state/ledger-end with a fixed offset and routes /v2/state/active-contracts to
// acHandler. lastAuth captures the most recently seen Authorization header (both endpoints),
// for the bearer-token test.
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

// decodeRequestedTemplate pulls the single requested templateId (and reading party) out of an
// active-contracts request body, matching activeContractsRequest's shape in main.go.
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

// acsContractJSON renders one active-contracts response element in the JSON Ledger API v2
// wire shape (contractEntry.JsActiveContract.createdEvent), independent of main.go's internal
// decode types, so the test pins the actual wire contract rather than a struct's shape.
func acsContractJSON(templateID, contractID, blob, synchronizerID string) string {
	return fmt.Sprintf(
		`{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":%q,"templateId":%q,"createdEventBlob":%q,"synchronizerId":%q}}}}`,
		contractID, templateID, blob, synchronizerID,
	)
}

// ---------------------------------------------------------------------------
// /v1/disclosures
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T, upstream *httptest.Server, party string, allowList []string) *server {
	t.Helper()
	opts := &options{party: party, jsonAPIBaseURL: upstream.URL}
	acs := newHTTPACSClient(upstream.URL, "")
	return newServer(opts, allowList, acs)
}

func TestHandleDisclosures_RejectsUnknownTemplate(t *testing.T) {
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("active-contracts must not be queried for a rejected request")
	})
	srv := newTestServer(t, upstream, "Alice", []string{"Wormhole.Ntt.Manager:NttManager"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/disclosures?template=Evil.Module:Backdoor", nil)
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Evil.Module:Backdoor")
}

func TestHandleDisclosures_HappyPath(t *testing.T) {
	const template = "Wormhole.Ntt.Manager:NttManager"
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
	const tA = "Wormhole.Core.State:CoreState"
	const tB = "Wormhole.Core.State:Emitter"
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
	const template = "Wormhole.Ntt.Manager:NttManager"
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
	const template = "Wormhole.Ntt.Manager:NttManager"
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

	// Must reject outright, not silently truncate to the cap.
	assert.NotEqual(t, http.StatusOK, rr.Code)
	assert.Equal(t, http.StatusBadGateway, rr.Code)
}

func TestHandleDisclosures_EmptyResultIsValid(t *testing.T) {
	const template = "Wormhole.Ntt.Manager:NttManager"
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
	const template = "Wormhole.Ntt.Manager:NttManager"
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		// Upstream returns a contract under a completely different template -- must not be
		// trusted blindly even though it came back on the requested-template query.
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
	assert.Equal(t, "Alice", got.Party)
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

	const template = "Wormhole.Ntt.Manager:NttManager"
	upstream, capture := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})

	client := newHTTPACSClient(upstream.URL, token)
	_, err = client.ActiveContracts(context.Background(), "Alice", template)
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret-token-value", capture.get())
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

// listeningURL scans stderr output for the banner's final line and returns the URL it names,
// or "" if the line has not appeared yet.
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
		listen:         "127.0.0.1:0",
		jsonAPIBaseURL: upstream.URL,
		party:          "Alice",
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
