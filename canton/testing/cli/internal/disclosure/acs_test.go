package disclosure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// knownBlobBase64/knownBlobHex are the same bytes ("hello world") in the JSON API's base64
// wire format and the hex format Daml Script's Disclosure.blob requires -- the
// cmd/ntt-playground/transfer.go:68-84 gotcha this package must also handle.
const (
	knownBlobBase64 = "aGVsbG8gd29ybGQ="
	knownBlobHex    = "68656c6c6f20776f726c64"
)

// acsTestServer records the last /v2/state/active-contracts request body and serves canned
// ledger-end/active-contracts responses, plus asserting on the Authorization header.
type acsTestServer struct {
	*httptest.Server
	ledgerEnd      int64
	lastAuth       string
	lastReqBody    activeContractsRequest
	acsResponseRaw string
}

func newACSTestServer(t *testing.T, ledgerEnd int64, acsResponseRaw string) *acsTestServer {
	t.Helper()
	s := &acsTestServer{ledgerEnd: ledgerEnd, acsResponseRaw: acsResponseRaw}
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/state/ledger-end", func(w http.ResponseWriter, r *http.Request) {
		s.lastAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(struct {
			Offset int64 `json:"offset"`
		}{Offset: s.ledgerEnd})
	})
	mux.HandleFunc("/v2/state/active-contracts", func(w http.ResponseWriter, r *http.Request) {
		s.lastAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &s.lastReqBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.acsResponseRaw))
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// canned response: one genuine JsActiveContract, plus one other oneOf variant that Query must
// tolerate and skip (the plan's §5.2: "other oneOf variants exist").
const acsCannedResponse = `[
  {"contractEntry": {"JsActiveContract": {"createdEvent": {
      "contractId": "cid-1",
      "templateId": "#wormhole-core:Wormhole.Core.State:CoreState",
      "createArgument": {"owner": "Alice", "amount": "10.0"},
      "createdEventBlob": "` + knownBlobBase64 + `"
  }}}},
  {"contractEntry": {"JsEmpty": {}}}
]`

func TestActiveContracts_LedgerEnd(t *testing.T) {
	srv := newACSTestServer(t, 42, "[]")
	a := &ActiveContracts{BaseURL: srv.URL, Token: "tok-123", Party: "alice::party"}

	offset, err := a.LedgerEnd(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(42), offset)
	require.Equal(t, "Bearer tok-123", srv.lastAuth)
}

func TestActiveContracts_LedgerEnd_NonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/state/ledger-end", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	a := &ActiveContracts{BaseURL: srv.URL, Party: "alice::party"}
	_, err := a.LedgerEnd(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
	require.Contains(t, err.Error(), "boom")
}

func TestActiveContracts_Query_RequestShape(t *testing.T) {
	srv := newACSTestServer(t, 99, acsCannedResponse)
	a := &ActiveContracts{BaseURL: srv.URL, Token: "tok-456", Party: "alice::party"}

	_, offset, err := a.Query(context.Background(), []string{
		"Wormhole.Core.State:CoreState",
		"Wormhole.Ntt.Manager:NttManager",
	})
	require.NoError(t, err)
	require.Equal(t, int64(99), offset)

	req := srv.lastReqBody
	require.Equal(t, int64(99), req.ActiveAtOffset)
	require.False(t, req.EventFormat.Verbose)

	pf, ok := req.EventFormat.FiltersByParty["alice::party"]
	require.True(t, ok, "filtersByParty must be keyed on ActiveContracts.Party")
	require.Len(t, pf.Cumulative, 2)
	for _, cf := range pf.Cumulative {
		require.True(t, cf.IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob,
			"includeCreatedEventBlob must be true -- this is the whole blob-export mechanism (plan §2.3)")
	}
	require.Equal(t, "Wormhole.Core.State:CoreState", pf.Cumulative[0].IdentifierFilter.TemplateFilter.Value.TemplateID)
	require.Equal(t, "Wormhole.Ntt.Manager:NttManager", pf.Cumulative[1].IdentifierFilter.TemplateFilter.Value.TemplateID)

	require.Equal(t, "Bearer tok-456", srv.lastAuth)
}

func TestActiveContracts_Query_DecodesAndConvertsBlob(t *testing.T) {
	srv := newACSTestServer(t, 7, acsCannedResponse)
	a := &ActiveContracts{BaseURL: srv.URL, Party: "alice::party"}

	contracts, _, err := a.Query(context.Background(), []string{"Wormhole.Core.State:CoreState"})
	require.NoError(t, err)

	// The JsEmpty entry must be skipped, leaving exactly the one genuine contract.
	require.Len(t, contracts, 1)
	c := contracts[0]
	require.Equal(t, "cid-1", c.ContractID)
	require.Equal(t, "#wormhole-core:Wormhole.Core.State:CoreState", c.TemplateID)
	require.Equal(t, knownBlobHex, c.Blob, "createdEventBlob must be re-encoded base64 -> hex")
	require.JSONEq(t, `{"owner": "Alice", "amount": "10.0"}`, string(c.Payload))
}

func TestActiveContracts_Query_NonOKStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/state/ledger-end", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Offset int64 `json:"offset"`
		}{Offset: 1})
	})
	mux.HandleFunc("/v2/state/active-contracts", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	a := &ActiveContracts{BaseURL: srv.URL, Party: "alice::party"}
	_, _, err := a.Query(context.Background(), []string{"A:B"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
	require.Contains(t, err.Error(), "nope")
}
