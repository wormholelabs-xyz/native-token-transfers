package network

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----------------------------------------------------------------------
// DSOPartyID
// ----------------------------------------------------------------------

func TestDSOPartyID_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/validator/v0/scan-proxy/dso-party-id" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing bearer auth")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"dso_party_id":"DSO::abc"}`))
	}))
	defer srv.Close()

	m := LocalNetManager{ValidatorBaseURL: srv.URL}
	party, err := m.DSOPartyID(context.Background())
	if err != nil {
		t.Fatalf("DSOPartyID: %v", err)
	}
	if party != "DSO::abc" {
		t.Fatalf("party mismatch: %q", party)
	}
}

func TestDSOPartyID_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	m := LocalNetManager{ValidatorBaseURL: srv.URL}
	_, err := m.DSOPartyID(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("expected an HTTP 403 error, got %v", err)
	}
}

func TestDSOPartyID_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	m := LocalNetManager{ValidatorBaseURL: srv.URL}
	_, err := m.DSOPartyID(context.Background())
	if err == nil || !strings.Contains(err.Error(), "parse dso-party-id response") {
		t.Fatalf("expected a parse-response error, got %v", err)
	}
}

func TestDSOPartyID_DoError(t *testing.T) {
	m := LocalNetManager{ValidatorBaseURL: "http://127.0.0.1:1"}
	_, err := m.DSOPartyID(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dso-party-id request") {
		t.Fatalf("expected a request-execution error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// UploadDAR
// ----------------------------------------------------------------------

func TestUploadDAR_Success(t *testing.T) {
	dar := filepath.Join(t.TempDir(), "test.dar")
	if err := os.WriteFile(dar, []byte("fake-dar-bytes"), 0o600); err != nil {
		t.Fatalf("write fixture DAR: %v", err)
	}

	var gotContentType, gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/packages" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		gotContentType = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var m LocalNetManager
	if err := m.UploadDAR(context.Background(), srv.URL, dar, "tok"); err != nil {
		t.Fatalf("UploadDAR: %v", err)
	}
	if gotContentType != "application/octet-stream" {
		t.Fatalf("content-type mismatch: %q", gotContentType)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth mismatch: %q", gotAuth)
	}
	if string(gotBody) != "fake-dar-bytes" {
		t.Fatalf("body mismatch: %q", gotBody)
	}
}

func TestUploadDAR_MissingFile(t *testing.T) {
	var m LocalNetManager
	err := m.UploadDAR(context.Background(), "http://unused.invalid", filepath.Join(t.TempDir(), "missing.dar"), "tok")
	if err == nil || !strings.Contains(err.Error(), "read DAR") {
		t.Fatalf("expected a read-DAR error, got %v", err)
	}
}

func TestUploadDAR_NonOKStatus(t *testing.T) {
	dar := filepath.Join(t.TempDir(), "test.dar")
	if err := os.WriteFile(dar, []byte("bytes"), 0o600); err != nil {
		t.Fatalf("write fixture DAR: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad dar"))
	}))
	defer srv.Close()

	var m LocalNetManager
	err := m.UploadDAR(context.Background(), srv.URL, dar, "tok")
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("expected an HTTP 400 error, got %v", err)
	}
}

func TestUploadDAR_DoError(t *testing.T) {
	dar := filepath.Join(t.TempDir(), "test.dar")
	if err := os.WriteFile(dar, []byte("bytes"), 0o600); err != nil {
		t.Fatalf("write fixture DAR: %v", err)
	}
	var m LocalNetManager
	err := m.UploadDAR(context.Background(), "http://127.0.0.1:1", dar, "tok")
	if err == nil || !strings.Contains(err.Error(), "upload DAR request") {
		t.Fatalf("expected a request-execution error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// CreateLedgerUser
// ----------------------------------------------------------------------

func TestCreateLedgerUser_Success(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/users" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		decodeJSONBody(t, r, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := CreateLedgerUser(context.Background(), srv.URL, "admin-tok", "guardian-watcher", []string{"guardianObserver::abc"}, []string{"operator::abc"})
	if err != nil {
		t.Fatalf("CreateLedgerUser: %v", err)
	}
	user, _ := gotBody["user"].(map[string]any)
	if user["id"] != "guardian-watcher" {
		t.Fatalf("user id mismatch: %+v", gotBody)
	}
	rights, _ := gotBody["rights"].([]any)
	if len(rights) != 2 {
		t.Fatalf("expected 2 rights (1 readAs + 1 actAs), got %+v", rights)
	}
}

func TestCreateLedgerUser_ConflictIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	err := CreateLedgerUser(context.Background(), srv.URL, "admin-tok", "guardian-watcher", nil, nil)
	if err != nil {
		t.Fatalf("a 409 must be treated as success, got %v", err)
	}
}

func TestCreateLedgerUser_AlreadyExistsAsBadRequestIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"USER_ALREADY_EXISTS: the user Already Exists"}`))
	}))
	defer srv.Close()

	err := CreateLedgerUser(context.Background(), srv.URL, "admin-tok", "guardian-watcher", nil, nil)
	if err != nil {
		t.Fatalf("a 400 naming 'already exists' must be treated as success, got %v", err)
	}
}

func TestCreateLedgerUser_GenuineError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"malformed request"}`))
	}))
	defer srv.Close()

	err := CreateLedgerUser(context.Background(), srv.URL, "admin-tok", "guardian-watcher", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("expected an HTTP 400 error, got %v", err)
	}
}

func TestCreateLedgerUser_DoError(t *testing.T) {
	err := CreateLedgerUser(context.Background(), "http://127.0.0.1:1", "admin-tok", "guardian-watcher", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "create ledger user") {
		t.Fatalf("expected a request-execution error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// GrantLedgerAPIUserRights
// ----------------------------------------------------------------------

func TestGrantLedgerAPIUserRights_Success(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		decodeJSONBody(t, r, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := GrantLedgerAPIUserRights(context.Background(), srv.URL, "admin-tok", []string{"party-a::x", "party-b::y"})
	if err != nil {
		t.Fatalf("GrantLedgerAPIUserRights: %v", err)
	}
	wantPath := "/v2/users/" + LocalNetAdminUser + "/rights"
	if gotPath != wantPath {
		t.Fatalf("path mismatch: got %q want %q", gotPath, wantPath)
	}
	if gotBody["userId"] != LocalNetAdminUser {
		t.Fatalf("userId mismatch: %+v", gotBody)
	}
	rights, _ := gotBody["rights"].([]any)
	if len(rights) != 2 {
		t.Fatalf("expected 2 rights, got %+v", rights)
	}
}

func TestGrantLedgerAPIUserRights_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	err := GrantLedgerAPIUserRights(context.Background(), srv.URL, "admin-tok", []string{"party::x"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected an HTTP 500 error, got %v", err)
	}
}

func TestGrantLedgerAPIUserRights_DoError(t *testing.T) {
	err := GrantLedgerAPIUserRights(context.Background(), "http://127.0.0.1:1", "admin-tok", []string{"party::x"})
	if err == nil || !strings.Contains(err.Error(), "grant rights request") {
		t.Fatalf("expected a request-execution error, got %v", err)
	}
}

// decodeJSONBody reads and decodes r's JSON body into out, failing the test on error.
func decodeJSONBody(t *testing.T, r *http.Request, out any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
}
