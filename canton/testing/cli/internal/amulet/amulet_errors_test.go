package amulet

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ----------------------------------------------------------------------
// Pure-function unit tests: retryable, isTransientTransportError, sleepOrDone
// ----------------------------------------------------------------------

func TestRetryable(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"503 is retryable regardless of body", http.StatusServiceUnavailable, "", true},
		{"429 is retryable regardless of body", http.StatusTooManyRequests, "", true},
		{"200 with 'no open mining round' body is retryable", http.StatusOK, `{"error":"No Open Mining Round right now"}`, true},
		{"200 with unrelated body is not retryable", http.StatusOK, `{"ok":true}`, false},
		{"500 with unrelated body is not retryable", http.StatusInternalServerError, "boom", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := retryable(c.status, []byte(c.body)); got != c.want {
				t.Fatalf("retryable(%d, %q) = %v, want %v", c.status, c.body, got, c.want)
			}
		})
	}
}

// timeoutErr is a synthetic net.Error whose Timeout() is true, distinct from
// context.DeadlineExceeded, to exercise isTransientTransportError's errors.As fallback.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "synthetic i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsTransientTransportError(t *testing.T) {
	if isTransientTransportError(nil) {
		t.Fatalf("nil error must not be transient")
	}
	if !isTransientTransportError(context.DeadlineExceeded) {
		t.Fatalf("context.DeadlineExceeded must be transient")
	}
	if !isTransientTransportError(errors.Join(errors.New("wrap"), timeoutErr{})) {
		t.Fatalf("a wrapped net.Error with Timeout()==true must be transient")
	}
	if isTransientTransportError(errors.New("some other error")) {
		t.Fatalf("a plain non-timeout error must not be transient")
	}
}

func TestSleepOrDone_CompletesNormally(t *testing.T) {
	if !sleepOrDone(context.Background(), time.Millisecond) {
		return
	}
}

func TestSleepOrDone_CtxCancelledFirst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepOrDone(ctx, time.Minute) {
		t.Fatalf("sleepOrDone must report false when ctx is already done")
	}
}

// ----------------------------------------------------------------------
// HTTP client helpers for simulating transport-level failures.
// ----------------------------------------------------------------------

// flakyTransport fails the first `failN` round trips with a synthetic timeout error, then
// delegates to the real transport (reaching the httptest server) for every call after.
type flakyTransport struct {
	failN int32
	calls int32
}

func (f *flakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if atomic.AddInt32(&f.calls, 1) <= f.failN {
		return nil, timeoutErr{}
	}
	return http.DefaultTransport.RoundTrip(req)
}

// alwaysTimeoutTransport never reaches the server -- every call fails with a transient
// transport error, for exercising a retry loop that runs out its RetryTimeout deadline.
type alwaysTimeoutTransport struct{}

func (alwaysTimeoutTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, timeoutErr{}
}

// brokenBodyTransport returns a 200 response whose body errors on Read, to exercise doJSON's
// io.ReadAll failure branch.
type brokenBodyTransport struct{}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("synthetic read failure") }
func (errReadCloser) Close() error             { return nil }

func (brokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       errReadCloser{},
		Header:     make(http.Header),
	}, nil
}

// ----------------------------------------------------------------------
// OnboardWalletUser negative paths
// ----------------------------------------------------------------------

func TestOnboardWalletUser_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.OnboardWalletUser(context.Background(), "cc-sender")
	if err == nil || !strings.Contains(err.Error(), "parse register response") {
		t.Fatalf("expected a parse-response error, got %v", err)
	}
}

func TestOnboardWalletUser_NonTransientHTTPErrorIsNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.OnboardWalletUser(context.Background(), "cc-sender")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected an HTTP 500 error, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("a non-transient HTTP error must not be retried, got %d calls", n)
	}
}

func TestOnboardWalletUser_RetriesTransientTransportErrorThenSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"party_id":"cc-sender::abc"}`))
	}))
	defer srv.Close()

	transport := &flakyTransport{failN: 1}
	c := &Client{
		ValidatorBaseURL: srv.URL,
		HTTPClient:       &http.Client{Transport: transport},
		RetryInterval:    time.Millisecond,
		RetryTimeout:     time.Second,
	}
	party, err := c.OnboardWalletUser(context.Background(), "cc-sender")
	if err != nil {
		t.Fatalf("OnboardWalletUser: %v", err)
	}
	if party != "cc-sender::abc" {
		t.Fatalf("party mismatch: %q", party)
	}
	if atomic.LoadInt32(&transport.calls) < 2 {
		t.Fatalf("expected a retry after the transient transport error")
	}
}

func TestOnboardWalletUser_TransientErrorExhaustsRetryTimeout(t *testing.T) {
	c := &Client{
		ValidatorBaseURL: "http://unused.invalid",
		HTTPClient:       &http.Client{Transport: alwaysTimeoutTransport{}},
		RetryInterval:    time.Millisecond,
		RetryTimeout:     10 * time.Millisecond,
	}
	_, err := c.OnboardWalletUser(context.Background(), "cc-sender")
	if err == nil {
		t.Fatalf("expected an error once the retry deadline is exhausted")
	}
}

func TestOnboardWalletUser_CtxCancelledDuringRetryWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	c := &Client{
		ValidatorBaseURL: "http://unused.invalid",
		HTTPClient:       &http.Client{Transport: alwaysTimeoutTransport{}},
		RetryInterval:    time.Minute, // longer than ctx's timeout: ctx.Done() must win the select
		RetryTimeout:     time.Hour,
	}
	_, err := c.OnboardWalletUser(ctx, "cc-sender")
	if err == nil {
		t.Fatalf("expected ctx.Err() once ctx is cancelled mid-retry-wait")
	}
}

func TestOnboardWalletUser_ReadResponseBodyError(t *testing.T) {
	c := &Client{
		ValidatorBaseURL: "http://unused.invalid",
		HTTPClient:       &http.Client{Transport: brokenBodyTransport{}},
		RetryInterval:    time.Millisecond,
		RetryTimeout:     50 * time.Millisecond,
	}
	_, err := c.OnboardWalletUser(context.Background(), "cc-sender")
	if err == nil || !strings.Contains(err.Error(), "read response for") {
		t.Fatalf("expected a read-response error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Tap negative paths
// ----------------------------------------------------------------------

func TestTap_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"contract_id":`)) // truncated
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.Tap(context.Background(), "cc-sender", "100")
	if err == nil || !strings.Contains(err.Error(), "parse tap response") {
		t.Fatalf("expected a parse-response error, got %v", err)
	}
}

func TestTap_NonTransientHTTPErrorIsNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad amount"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.Tap(context.Background(), "cc-sender", "-1")
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("expected an HTTP 400 error, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("a non-transient HTTP error must not be retried, got %d calls", n)
	}
}

func TestTap_RetriesTransientTransportErrorThenSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"contract_id":"00feed"}`))
	}))
	defer srv.Close()

	transport := &flakyTransport{failN: 1}
	c := &Client{
		ValidatorBaseURL: srv.URL,
		HTTPClient:       &http.Client{Transport: transport},
		RetryInterval:    time.Millisecond,
		RetryTimeout:     time.Second,
	}
	cid, err := c.Tap(context.Background(), "cc-sender", "100")
	if err != nil {
		t.Fatalf("Tap: %v", err)
	}
	if cid != "00feed" {
		t.Fatalf("contract id mismatch: %q", cid)
	}
}

func TestTap_ImmediateNonTransientTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should never be reached")
	}))
	srv.Close() // closed before any call -- every request is refused, not a timeout

	c := testClient(t, srv)
	_, err := c.Tap(context.Background(), "cc-sender", "100")
	if err == nil {
		t.Fatalf("expected a connection error")
	}
}

// TestTap_RetryableBodyCtxCancelledDuringWait proves the retryable-status-code retry branch
// (as opposed to the transient-transport-error branch) also respects ctx cancellation.
func TestTap_RetryableBodyCtxCancelledDuringWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	c := &Client{
		ValidatorBaseURL: srv.URL,
		RetryInterval:    time.Minute,
		RetryTimeout:     time.Hour,
	}
	_, err := c.Tap(ctx, "cc-sender", "100")
	if err == nil {
		t.Fatalf("expected ctx.Err() once ctx is cancelled mid-retry-wait")
	}
}

// ----------------------------------------------------------------------
// CreateTransferPreapproval negative paths
// ----------------------------------------------------------------------

func TestCreateTransferPreapproval_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{{{`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.CreateTransferPreapproval(context.Background(), "cc-custody")
	if err == nil || !strings.Contains(err.Error(), "parse transfer-preapproval response") {
		t.Fatalf("expected a parse-response error, got %v", err)
	}
}

func TestCreateTransferPreapproval_NonTransientHTTPErrorIsNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.CreateTransferPreapproval(context.Background(), "cc-custody")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("expected an HTTP 403 error, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("a non-transient HTTP error must not be retried, got %d calls", n)
	}
}

func TestCreateTransferPreapproval_RetriesTransientTransportErrorThenSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"transfer_preapproval_contract_id":"00abc"}`))
	}))
	defer srv.Close()

	transport := &flakyTransport{failN: 1}
	c := &Client{
		ValidatorBaseURL: srv.URL,
		HTTPClient:       &http.Client{Transport: transport},
		RetryInterval:    time.Millisecond,
		RetryTimeout:     time.Second,
	}
	cid, err := c.CreateTransferPreapproval(context.Background(), "cc-custody")
	if err != nil {
		t.Fatalf("CreateTransferPreapproval: %v", err)
	}
	if cid != "00abc" {
		t.Fatalf("contract id mismatch: %q", cid)
	}
}

// ----------------------------------------------------------------------
// GetTransferFactory negative paths
// ----------------------------------------------------------------------

func transferArgs() TransferArgs {
	return TransferArgs{DSO: "DSO::abc", Sender: "cc-sender::abc", Receiver: "cc-custody-custody::abc", Amount: "1.0000000000"}
}

func TestGetTransferFactory_NonTransientTransportErrorIsNotRetried(t *testing.T) {
	c := &Client{
		ValidatorBaseURL: "http://127.0.0.1:1", // nobody listens here -- connection refused, not a timeout
		RetryInterval:    time.Millisecond,
		RetryTimeout:     time.Second,
	}
	_, err := c.GetTransferFactory(context.Background(), "cc-sender", transferArgs())
	if err == nil {
		t.Fatalf("expected a connection error")
	}
}

func TestGetTransferFactory_RetriesTransientTransportErrorThenSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"factoryId":"00f","transferKind":"direct","choiceContext":{"choiceContextData":{},"disclosedContracts":[]}}`))
	}))
	defer srv.Close()

	transport := &flakyTransport{failN: 1}
	c := &Client{
		ValidatorBaseURL: srv.URL,
		HTTPClient:       &http.Client{Transport: transport},
		RetryInterval:    time.Millisecond,
		RetryTimeout:     time.Second,
	}
	factory, err := c.GetTransferFactory(context.Background(), "cc-sender", transferArgs())
	if err != nil {
		t.Fatalf("GetTransferFactory: %v", err)
	}
	if factory.FactoryID != "00f" {
		t.Fatalf("factory mismatch: %+v", factory)
	}
}

func TestGetTransferFactory_TransientErrorExhaustsRetryTimeout(t *testing.T) {
	c := &Client{
		ValidatorBaseURL: "http://unused.invalid",
		HTTPClient:       &http.Client{Transport: alwaysTimeoutTransport{}},
		RetryInterval:    time.Millisecond,
		RetryTimeout:     10 * time.Millisecond,
	}
	_, err := c.GetTransferFactory(context.Background(), "cc-sender", transferArgs())
	if err == nil {
		t.Fatalf("expected an error once the retry deadline is exhausted")
	}
}

func TestGetTransferFactory_CtxCancelledDuringRetryWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	c := &Client{
		ValidatorBaseURL: "http://unused.invalid",
		HTTPClient:       &http.Client{Transport: alwaysTimeoutTransport{}},
		RetryInterval:    time.Minute,
		RetryTimeout:     time.Hour,
	}
	_, err := c.GetTransferFactory(ctx, "cc-sender", transferArgs())
	if err == nil {
		t.Fatalf("expected ctx.Err() once ctx is cancelled mid-retry-wait")
	}
}

func TestGetTransferFactory_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.GetTransferFactory(context.Background(), "cc-sender", transferArgs())
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected an HTTP 500 error, got %v", err)
	}
}

func TestGetTransferFactory_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, err := c.GetTransferFactory(context.Background(), "cc-sender", transferArgs())
	if err == nil || !strings.Contains(err.Error(), "parse transfer-factory response") {
		t.Fatalf("expected a parse-response error, got %v", err)
	}
}

// io.Reader guard to keep goimports from dropping the io import if every direct use above is
// ever refactored away; also documents the errReadCloser's Read signature intent.
var _ io.Reader = errReadCloser{}
