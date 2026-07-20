package amulet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return &Client{
		ValidatorBaseURL: srv.URL,
		RetryInterval:    time.Millisecond,
		RetryTimeout:     time.Second,
	}
}

// TestTap_RequestAndResponse pins the tap body/response shape (findings §2): a bare
// {"amount": "<usd decimal string>"} POST, response {"contract_id": "..."}.
func TestTap_RequestAndResponse(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"contract_id":"00deadbeef"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	cid, err := c.Tap(context.Background(), "cc-sender", "100")
	if err != nil {
		t.Fatalf("Tap: %v", err)
	}
	if cid != "00deadbeef" {
		t.Fatalf("Tap contract id: got %q", cid)
	}
	if gotPath != "/api/validator/v0/wallet/tap" {
		t.Fatalf("Tap path: got %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("Tap missing bearer auth: %q", gotAuth)
	}
	if gotBody["amount"] != "100" {
		t.Fatalf("Tap body: got %+v", gotBody)
	}
}

// TestTap_RetriesNoOpenMiningRound pins the plan's "retry ~2 min on no open mining round"
// mitigation (findings §5/§2): the first response looks like the round-ingestion-lag failure
// mode, and a subsequent call succeeds -- Tap must retry rather than surface the first error.
func TestTap_RetriesNoOpenMiningRound(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"no open mining round"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"contract_id":"00cafe"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	cid, err := c.Tap(context.Background(), "cc-sender", "100")
	if err != nil {
		t.Fatalf("Tap: %v", err)
	}
	if cid != "00cafe" {
		t.Fatalf("Tap contract id after retry: got %q", cid)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected Tap to retry at least once, got %d calls", calls)
	}
}

// TestCreateTransferPreapproval_409IsSuccess pins findings §3: a 409 means the
// TransferPreapproval already exists and must be treated as success, not an error.
func TestCreateTransferPreapproval_409IsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/validator/v0/wallet/transfer-preapproval" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"transfer_preapproval_contract_id":"00existing"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	cid, err := c.CreateTransferPreapproval(context.Background(), "cc-custody")
	if err != nil {
		t.Fatalf("CreateTransferPreapproval: %v", err)
	}
	if cid != "00existing" {
		t.Fatalf("CreateTransferPreapproval cid: got %q", cid)
	}
}

// TestCreateTransferPreapproval_RetriesOn429 pins findings §3: 429 means the validator's
// automation is still converting the proposal in flight -- retry with backoff.
func TestCreateTransferPreapproval_RetriesOn429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"transfer_preapproval_contract_id":"00new"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	cid, err := c.CreateTransferPreapproval(context.Background(), "cc-custody")
	if err != nil {
		t.Fatalf("CreateTransferPreapproval: %v", err)
	}
	if cid != "00new" {
		t.Fatalf("CreateTransferPreapproval cid: got %q", cid)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected a retry after 429, got %d calls", calls)
	}
}

// TestGetTransferFactory_RequestAndResponse pins findings §4: the choiceArguments encoding
// (instrumentId.id == "Amulet"), and the response decode including disclosedContracts.
func TestGetTransferFactory_RequestAndResponse(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/validator/v0/scan-proxy/registry/transfer-instruction/v1/transfer-factory" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotReq); err != nil {
			t.Fatalf("decode request body: %v\n%s", err, body)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"factoryId": "00factory",
			"transferKind": "direct",
			"choiceContext": {
				"choiceContextData": {"values": {"transfer-preapproval": {"tag": "AV_ContractId", "value": "00pre"}}},
				"disclosedContracts": [
					{"templateId": "pkg:Splice.AmuletRules:AmuletRules", "contractId": "00ar",
					 "createdEventBlob": "YmFzZTY0", "synchronizerId": "global-domain::1220ab"}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	factory, err := c.GetTransferFactory(context.Background(), "cc-sender", TransferArgs{
		DSO:      "DSO::abc",
		Sender:   "cc-sender::abc",
		Receiver: "cc-custody-custody::abc",
		Amount:   "1.0000000000",
	})
	if err != nil {
		t.Fatalf("GetTransferFactory: %v", err)
	}
	if factory.FactoryID != "00factory" || factory.TransferKind != "direct" {
		t.Fatalf("factory decode mismatch: %+v", factory)
	}
	if len(factory.DisclosedContracts) != 1 || factory.DisclosedContracts[0].TemplateID != "pkg:Splice.AmuletRules:AmuletRules" {
		t.Fatalf("disclosedContracts decode mismatch: %+v", factory.DisclosedContracts)
	}
	if len(factory.ChoiceContextData) == 0 {
		t.Fatalf("choiceContextData must be captured verbatim (non-empty raw JSON)")
	}

	choiceArgs, _ := gotReq["choiceArguments"].(map[string]any)
	transfer, _ := choiceArgs["transfer"].(map[string]any)
	instrumentID, _ := transfer["instrumentId"].(map[string]any)
	if instrumentID["id"] != "Amulet" {
		t.Fatalf("request must target the Amulet instrument, got %+v", instrumentID)
	}
	if transfer["sender"] != "cc-sender::abc" || transfer["receiver"] != "cc-custody-custody::abc" {
		t.Fatalf("request sender/receiver mismatch: %+v", transfer)
	}
}

// TestGetTransferFactory_RejectsNonDirect pins the plan's hard gate: anything other than
// "direct" would settle Pending and the on-ledger custody hook would abort -- Go must fail
// fast instead of submitting.
func TestGetTransferFactory_RejectsNonDirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"factoryId": "00factory",
			"transferKind": "offer",
			"choiceContext": {"choiceContextData": {}, "disclosedContracts": []}
		}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.GetTransferFactory(context.Background(), "cc-sender", TransferArgs{
		DSO: "DSO::abc", Sender: "cc-sender::abc", Receiver: "cc-custody-custody::abc", Amount: "1.0000000000",
	})
	if err == nil {
		t.Fatalf("expected an error for a non-direct transferKind")
	}
	if !strings.Contains(err.Error(), "offer") {
		t.Fatalf("error should name the unexpected transferKind: %v", err)
	}
}

// TestOnboardWalletUser_RequestAndResponse pins findings §8: POST /v0/register with the
// new user's own JWT and an empty body, response {"party_id": "..."}.
func TestOnboardWalletUser_RequestAndResponse(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"party_id":"cc-sender::abc"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	party, err := c.OnboardWalletUser(context.Background(), "cc-sender")
	if err != nil {
		t.Fatalf("OnboardWalletUser: %v", err)
	}
	if party != "cc-sender::abc" {
		t.Fatalf("OnboardWalletUser party: got %q", party)
	}
	if gotPath != "/api/validator/v0/register" {
		t.Fatalf("OnboardWalletUser path: got %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("OnboardWalletUser missing bearer auth: %q", gotAuth)
	}
	if strings.TrimSpace(gotBody) != "{}" {
		t.Fatalf("OnboardWalletUser body should be an empty object, got %q", gotBody)
	}
}
