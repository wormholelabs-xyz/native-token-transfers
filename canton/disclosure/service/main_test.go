package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	assert.Equal(t, []string{"Alice"}, opts.disclosingParties)
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

func TestParseFlags_MultiPartyList(t *testing.T) {
	opts, err := parseFlags([]string{
		"--json-api", "http://localhost:6975",
		"--disclosing-party", "Operator, GuardianGovernance,Alice",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Operator", "GuardianGovernance", "Alice"}, opts.disclosingParties)
}

func TestParseFlags_RejectsBadPartyLists(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"duplicate", "Alice,Alice", "twice"},
		{"duplicate after trim", "Alice, Alice", "twice"},
		{"empty entry", "Alice,,Bob", "empty entry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFlags([]string{"--json-api", "http://localhost:6975", "--disclosing-party", tc.value})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
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

	client := newHTTPACSClient(upstream.URL, "")
	_, err := client.ActiveContracts(context.Background(), []string{"Alice", "Bob"}, template, 42)
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
	shared := `{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":"00shared","templateId":"pkg:Wormhole.Ntt.Manager:NttManager","createdEventBlob":"AAA="},"synchronizerId":"sync"}}}`
	only := `{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":"00only","templateId":"pkg:Wormhole.Ntt.Manager:NttManager","createdEventBlob":"BBB="},"synchronizerId":"sync"}}}`
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[" + shared + "," + shared + "," + only + "]"))
	})

	client := newHTTPACSClient(upstream.URL, "")
	entries, err := client.ActiveContracts(context.Background(), []string{"Alice", "Bob"}, template, 42)
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
	opts := &options{disclosingParties: []string{party}, jsonAPIBaseURL: upstream.URL, maxContractsPerTemplate: defaultMaxContractsPerTemplate}
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
	assert.Equal(t, []string{"Alice"}, got.DisclosingParties)
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
	_, err = client.ActiveContracts(context.Background(), []string{"Alice"}, template, 42)
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
	_, err := client.ActiveContracts(context.Background(), []string{"Alice"}, template, 42)
	require.NoError(t, err)
	assert.Equal(t, "Bearer first-token", capture.get())

	require.NoError(t, os.WriteFile(tokenPath, []byte("second-token\n"), 0o600))

	_, err = client.ActiveContracts(context.Background(), []string{"Alice"}, template, 42)
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
		disclosingParties:       []string{"Alice"},
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

// ---------------------------------------------------------------------------
// /v1/flows/{flow}: fixtures
// ---------------------------------------------------------------------------

const testRecipient = "recipient::1"

// acsContractJSONWithArg is acsContractJSON plus a createArgument payload (raw JSON).
func acsContractJSONWithArg(templateID, contractID, blob, synchronizerID, createArgument string) string {
	return fmt.Sprintf(
		`{"contractEntry":{"JsActiveContract":{"createdEvent":{"contractId":%q,"templateId":%q,"createdEventBlob":%q,"createArgument":%s},"synchronizerId":%q}}}`,
		contractID, templateID, blob, createArgument, synchronizerID,
	)
}

func jsonArray(items ...string) string {
	return "[" + strings.Join(items, ",") + "]"
}

func nttManagerArg(admin, gg, namespace, managerAddress string, transceiverEmitterID int, instrumentAdmin, instrumentID, tokenConfig, factoryTag, factoryCid string) string {
	return fmt.Sprintf(
		`{"admin":%q,"guardianGovernance":%q,"namespace":%q,"managerAddress":%q,"transceiverEmitterId":%d,"instrumentId":{"admin":%q,"id":%q},"tokenConfig":%q,"factory":{"tag":%q,"value":%q}}`,
		admin, gg, namespace, managerAddress, transceiverEmitterID, instrumentAdmin, instrumentID, tokenConfig, factoryTag, factoryCid,
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
	admin, gg, namespace, managerAddr, digest, instrumentAdmin, instrumentID string
	transceiverEmitterID                                                     int
	tokenConfig, factoryTag, factoryCid                                      string
}

func newBaseFixtures(tokenConfig string) baseFixtures {
	factoryTag := factoryTagTransfer
	if tokenConfig == tokenConfigBurnMint {
		factoryTag = factoryTagBurnMint
	}
	return baseFixtures{
		admin:                testRecipient, // distinct from gg, so custodian-vs-recipient preapproval owners differ
		gg:                   "gg::1",
		namespace:            "ntt:test-namespace",
		managerAddr:          strings.Repeat("ab", 32),
		digest:               strings.Repeat("cd", 32),
		instrumentAdmin:      "gg::1",
		instrumentID:         "wormhole-ntt:instrument-1",
		transceiverEmitterID: 7,
		tokenConfig:          tokenConfig,
		factoryTag:           factoryTag,
		factoryCid:           "factory-cid-1",
	}
}

func (b baseFixtures) managerJSON() string {
	return acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-1", "bWdy", "sync1",
		nttManagerArg(b.admin, b.gg, b.namespace, b.managerAddr, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid))
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

func newFlowTestServer(t *testing.T, fixtures flowFixtures, party string) *server {
	t.Helper()
	upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, tmpl := decodeRequestedTemplate(t, r)
		body, ok := fixtures[templateTail(tmpl)]
		if !ok {
			body = "[]"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	return newTestServer(t, upstream, party, defaultAllowList())
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

func TestNormalizeHex64(t *testing.T) {
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
			got, err := normalizeHex64(tc.raw, "manager")
			if tc.wantErr {
				requireFlowErrorStatus(t, err, http.StatusBadRequest)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseByVaa(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    bool
		wantErr bool
	}{
		{"absent defaults to false", "", false, false},
		{"true", "true", true, false},
		{"false", "false", false, false},
		{"not a bool", "yes", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := url.Values{}
			if tc.raw != "" {
				q.Set("by-vaa", tc.raw)
			}
			got, err := parseByVaa(q)
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
	mk := func(addr string) decodedEntry[daNttManager] {
		return decodedEntry[daNttManager]{entry: acsEntry{ContractID: "cid-" + addr}, value: daNttManager{ManagerAddress: addr}}
	}
	addrA := strings.Repeat("aa", 32)
	addrB := strings.Repeat("bb", 32)

	t.Run("found", func(t *testing.T) {
		m, err := findManager([]decodedEntry[daNttManager]{mk(addrA), mk(addrB)}, addrA)
		require.NoError(t, err)
		assert.Equal(t, "cid-"+addrA, m.entry.ContractID)
	})
	t.Run("not found is 404", func(t *testing.T) {
		_, err := findManager([]decodedEntry[daNttManager]{mk(addrA)}, addrB)
		requireFlowErrorStatus(t, err, http.StatusNotFound)
	})
	t.Run("duplicate is 500", func(t *testing.T) {
		_, err := findManager([]decodedEntry[daNttManager]{mk(addrA), mk(addrA)}, addrA)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindGuardianAnchor(t *testing.T) {
	mk := func(gg string) decodedEntry[daGuardianAnchor] {
		return decodedEntry[daGuardianAnchor]{entry: acsEntry{ContractID: "cid-" + gg}, value: daGuardianAnchor{GuardianGovernance: gg}}
	}
	t.Run("gg given, exactly one match", func(t *testing.T) {
		m, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{mk("gg1"), mk("gg2")}, "gg1", "CoreState")
		require.NoError(t, err)
		assert.Equal(t, "cid-gg1", m.entry.ContractID)
	})
	t.Run("gg given, zero matches is 404", func(t *testing.T) {
		_, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{mk("gg2")}, "gg1", "CoreState")
		requireFlowErrorStatus(t, err, http.StatusNotFound)
	})
	t.Run("gg empty, sole entry", func(t *testing.T) {
		m, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{mk("gg1")}, "", "CoreState")
		require.NoError(t, err)
		assert.Equal(t, "cid-gg1", m.entry.ContractID)
	})
	t.Run("gg empty, zero entries is 500", func(t *testing.T) {
		_, err := findGuardianAnchor(nil, "", "CoreState")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("gg empty, two entries is 409", func(t *testing.T) {
		_, err := findGuardianAnchor([]decodedEntry[daGuardianAnchor]{mk("gg1"), mk("gg2")}, "", "CoreState")
		requireFlowErrorStatus(t, err, http.StatusConflict)
	})
}

func TestCoveringReplayNode(t *testing.T) {
	mk := func(consumer, namespace, prefix string) decodedEntry[daReplayNode] {
		return decodedEntry[daReplayNode]{entry: acsEntry{ContractID: "cid-" + prefix}, value: daReplayNode{Consumer: consumer, Namespace: namespace, Prefix: prefix}}
	}
	digest := strings.Repeat("cd", 32)

	t.Run("found by prefix", func(t *testing.T) {
		m, err := coveringReplayNode([]decodedEntry[daReplayNode]{mk("gg1", "ns1", "cd")}, "gg1", "ns1", digest)
		require.NoError(t, err)
		assert.Equal(t, "cid-cd", m.entry.ContractID)
	})
	t.Run("zero covering nodes is 500", func(t *testing.T) {
		_, err := coveringReplayNode([]decodedEntry[daReplayNode]{mk("gg1", "ns1", "ff")}, "gg1", "ns1", digest)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("two covering nodes is 500", func(t *testing.T) {
		entries := []decodedEntry[daReplayNode]{mk("gg1", "ns1", ""), mk("gg1", "ns1", "cd")}
		_, err := coveringReplayNode(entries, "gg1", "ns1", digest)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindLockedLedger(t *testing.T) {
	addr := strings.Repeat("aa", 32)
	mk := func() decodedEntry[daLockedLedger] {
		return decodedEntry[daLockedLedger]{entry: acsEntry{ContractID: "cid-" + addr}, value: daLockedLedger{ManagerAddress: addr}}
	}
	t.Run("found", func(t *testing.T) {
		m, err := findLockedLedger([]decodedEntry[daLockedLedger]{mk()}, addr)
		require.NoError(t, err)
		assert.Equal(t, "cid-"+addr, m.entry.ContractID)
	})
	t.Run("not found is 500", func(t *testing.T) {
		_, err := findLockedLedger(nil, addr)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("duplicate is 500", func(t *testing.T) {
		_, err := findLockedLedger([]decodedEntry[daLockedLedger]{mk(), mk()}, addr)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindTransceiverEmitter(t *testing.T) {
	mk := func(owner string, id json.Number) decodedEntry[daEmitter] {
		return decodedEntry[daEmitter]{entry: acsEntry{ContractID: "emitter-cid"}, value: daEmitter{Owner: owner, EmitterId: id}}
	}
	t.Run("found, numeric emitterId", func(t *testing.T) {
		m, err := findTransceiverEmitter([]decodedEntry[daEmitter]{mk("gg1", json.Number("7"))}, json.Number("7"), "gg1")
		require.NoError(t, err)
		assert.Equal(t, "emitter-cid", m.entry.ContractID)
	})
	t.Run("found, string emitterId", func(t *testing.T) {
		_, err := findTransceiverEmitter([]decodedEntry[daEmitter]{mk("gg1", json.Number("7"))}, json.Number("7"), "gg1")
		require.NoError(t, err)
	})
	t.Run("owner mismatch is 500", func(t *testing.T) {
		_, err := findTransceiverEmitter([]decodedEntry[daEmitter]{mk("gg2", json.Number("7"))}, json.Number("7"), "gg1")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
	t.Run("id mismatch is 500", func(t *testing.T) {
		_, err := findTransceiverEmitter([]decodedEntry[daEmitter]{mk("gg1", json.Number("8"))}, json.Number("7"), "gg1")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindCanonicalCoinFactory(t *testing.T) {
	mk := func(admin string) decodedEntry[daCoinFactory] {
		return decodedEntry[daCoinFactory]{entry: acsEntry{ContractID: "cid-" + admin}, value: daCoinFactory{Admin: admin}}
	}
	t.Run("found", func(t *testing.T) {
		m, err := findCanonicalCoinFactory([]decodedEntry[daCoinFactory]{mk("gg1")}, "gg1")
		require.NoError(t, err)
		assert.Equal(t, "cid-gg1", m.entry.ContractID)
	})
	t.Run("not found is 500", func(t *testing.T) {
		_, err := findCanonicalCoinFactory([]decodedEntry[daCoinFactory]{mk("gg2")}, "gg1")
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
	mk := func(owner string) decodedEntry[daPreapproval] {
		return decodedEntry[daPreapproval]{entry: acsEntry{ContractID: "cid-" + owner}, value: daPreapproval{Owner: owner, InstrumentId: inst}}
	}
	t.Run("found", func(t *testing.T) {
		m, found, err := findPreapproval([]decodedEntry[daPreapproval]{mk("alice")}, "alice", inst, "TransferPreapproval")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "cid-alice", m.entry.ContractID)
	})
	t.Run("absent is not an error", func(t *testing.T) {
		_, found, err := findPreapproval(nil, "alice", inst, "TransferPreapproval")
		require.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("duplicate is 500", func(t *testing.T) {
		_, _, err := findPreapproval([]decodedEntry[daPreapproval]{mk("alice"), mk("alice")}, "alice", inst, "TransferPreapproval")
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
}

func TestFindAdminTransferProposal(t *testing.T) {
	addr := strings.Repeat("aa", 32)
	mk := func() decodedEntry[daAdminTransferProposal] {
		return decodedEntry[daAdminTransferProposal]{entry: acsEntry{ContractID: "prop-cid"}, value: daAdminTransferProposal{ManagerAddress: addr}}
	}
	t.Run("found", func(t *testing.T) {
		m, found, err := findAdminTransferProposal([]decodedEntry[daAdminTransferProposal]{mk()}, addr)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "prop-cid", m.entry.ContractID)
	})
	t.Run("absent is not an error", func(t *testing.T) {
		_, found, err := findAdminTransferProposal(nil, addr)
		require.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("duplicate is 500", func(t *testing.T) {
		_, _, err := findAdminTransferProposal([]decodedEntry[daAdminTransferProposal]{mk(), mk()}, addr)
		requireFlowErrorStatus(t, err, http.StatusInternalServerError)
	})
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
	srv := newFlowTestServer(t, fixtures, "Alice")

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
	srv := newFlowTestServer(t, fixtures, "Alice")

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
	srv := newFlowTestServer(t, fixtures, "Alice")

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
	srv := newFlowTestServer(t, fixtures, "Alice")

	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Contains(t, missingRolesOf(resp.Missing), roleRecipientTransferPreapproval)
}

func TestHandleFlows_Release_UnknownManager_404(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: "[]"}, "Alice")
	rr := requestFlow(t, srv, "release", "manager="+strings.Repeat("11", 32)+"&digest="+strings.Repeat("22", 32))
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleFlows_Release_WrongMode_409(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())}, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "release", "manager="+b.managerAddr+"&digest="+b.digest)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), roleCoveringReplayNode)
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
	srv := newFlowTestServer(t, fixtures, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "mint", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleDepositPreapproval, roleCommittedBurnMintFactory},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Mint_DepositPreapprovalAbsent_Error(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	fixtures := flowFixtures{
		tailNttManager: jsonArray(b.managerJSON()),
		tailCoreState:  jsonArray(b.coreStateJSON()),
		tailReplayNode: jsonArray(b.replayNodeJSON("")),
	}
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "mint", "manager="+b.managerAddr+"&digest="+b.digest+"&recipient="+testRecipient)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), roleDepositPreapproval)
}

func TestHandleFlows_Mint_MissingRecipientParam_400(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())}, "Alice")
	rr := requestFlow(t, srv, "mint", "manager="+b.managerAddr+"&digest="+b.digest)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleFlows_Mint_WrongMode_409(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())}, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
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
	}
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "accept-admin", "manager="+b.managerAddr+"&digest="+b.digest)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleCoreState, roleCoveringReplayNode, roleAdminTransferProposal, roleLockedLedger, roleCustodyPotHolding, roleCommittedTransferFactory},
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
	srv := newFlowTestServer(t, fixtures, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
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
		tailTransferPreapproval: jsonArray(acsContractJSONWithArg("pkg:"+tailTransferPreapproval, "pre-cid-2", "cHJl", "sync1", preapprovalArg(b.admin, b.instrumentAdmin, b.instrumentID))),
	}
	srv := newFlowTestServer(t, fixtures, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "transfer", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleTransceiverEmitter, roleCoreState, roleCommittedBurnMintFactory},
		rolesOf(resp.Disclosures))
}

// ---------------------------------------------------------------------------
// /v1/flows/register
// ---------------------------------------------------------------------------

func TestHandleFlows_Register_Basic(t *testing.T) {
	const gg = "gg::1"
	fixtures := flowFixtures{
		tailNttGovernance:      jsonArray(acsContractJSONWithArg("pkg:"+tailNttGovernance, "gov-cid-1", "Z292", "sync1", guardianAnchorArg(gg))),
		tailEmitterRegistry:    jsonArray(acsContractJSONWithArg("pkg:"+tailEmitterRegistry, "emreg-cid-1", "ZW1yZWc=", "sync1", guardianAnchorArg(gg))),
		tailReplayRootRegistry: jsonArray(acsContractJSONWithArg("pkg:"+tailReplayRootRegistry, "rrreg-cid-1", "cnJyZWc=", "sync1", guardianAnchorArg(gg))),
		tailCoreState:          jsonArray(acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-2", "Y3M=", "sync1", guardianAnchorArg(gg))),
	}
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "register", "")
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttGovernance, roleEmitterRegistry, roleReplayRootRegistry, roleCoreState},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Register_ByVaa(t *testing.T) {
	const gg = "gg::1"
	fixtures := flowFixtures{
		tailNttGovernance:      jsonArray(acsContractJSONWithArg("pkg:"+tailNttGovernance, "gov-cid-2", "Z292", "sync1", guardianAnchorArg(gg))),
		tailEmitterRegistry:    jsonArray(acsContractJSONWithArg("pkg:"+tailEmitterRegistry, "emreg-cid-2", "ZW1yZWc=", "sync1", guardianAnchorArg(gg))),
		tailReplayRootRegistry: jsonArray(acsContractJSONWithArg("pkg:"+tailReplayRootRegistry, "rrreg-cid-2", "cnJyZWc=", "sync1", guardianAnchorArg(gg))),
		tailCoreState:          jsonArray(acsContractJSONWithArg("pkg:"+tailCoreState, "cs-cid-3", "Y3M=", "sync1", guardianAnchorArg(gg))),
		tailCoinFactory:        jsonArray(acsContractJSONWithArg("pkg:"+tailCoinFactory, "canon-factory-cid", "ZmFjdA==", "sync1", coinFactoryArg(gg))),
	}
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "register", "gg="+gg+"&by-vaa=true")
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttGovernance, roleEmitterRegistry, roleReplayRootRegistry, roleCoreState, roleCanonicalCoinFactory},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Register_ByVaaWithoutGG_400(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{}, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
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
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	resp := decodeFlowResponse(t, rr)
	assert.Equal(t,
		[]string{roleNttManager, roleLockedLedger, roleCommittedTransferFactory},
		rolesOf(resp.Disclosures))
}

func TestHandleFlows_Consolidate_WrongMode_409(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())}, "Alice")
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestHandleFlows_Consolidate_LedgerAbsent_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	srv := newFlowTestServer(t, flowFixtures{tailNttManager: jsonArray(b.managerJSON())}, "Alice")
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), roleLockedLedger)
}

func TestHandleFlows_Consolidate_FactoryAbsent_500(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	fixtures := flowFixtures{
		tailNttManager:   jsonArray(b.managerJSON()),
		tailLockedLedger: jsonArray(b.ledgerJSON(nil)),
	}
	srv := newFlowTestServer(t, fixtures, "Alice")
	rr := requestFlow(t, srv, "consolidate", "manager="+b.managerAddr)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// /v1/flows/{flow}: cross-cutting
// ---------------------------------------------------------------------------

func TestHandleFlows_UnknownFlow_404(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{}, "Alice")
	rr := requestFlow(t, srv, "not-a-flow", "manager=x")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleFlows_MethodNotAllowed(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{}, "Alice")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/flows/release", nil)
	srv.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleFlows_MissingManagerParam_400(t *testing.T) {
	srv := newFlowTestServer(t, flowFixtures{}, "Alice")
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
	srv := newFlowTestServer(t, flowFixtures{}, "Alice")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := requestFlow(t, srv, "release", tc.query)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}
