package observer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ----------------------------------------------------------------------
// toWebsocketURL
// ----------------------------------------------------------------------

func TestToWebsocketURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://example.com:3975", "wss://example.com:3975"},
		{"http://localhost:3975", "ws://localhost:3975"},
		{"ws://already-ws:3975", "ws://already-ws:3975"}, // neither prefix matches -- returned as-is
	}
	for _, c := range cases {
		if got := toWebsocketURL(c.in); got != c.want {
			t.Fatalf("toWebsocketURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ----------------------------------------------------------------------
// buildGetUpdatesRequest
// ----------------------------------------------------------------------

func TestBuildGetUpdatesRequest(t *testing.T) {
	req := buildGetUpdatesRequest(42, "guardianObserver::abc")
	if req.BeginExclusive != 42 {
		t.Fatalf("beginExclusive: got %d", req.BeginExclusive)
	}
	if req.UpdateFormat.IncludeTransactions.TransactionShape != "TRANSACTION_SHAPE_LEDGER_EFFECTS" {
		t.Fatalf("transactionShape: got %q", req.UpdateFormat.IncludeTransactions.TransactionShape)
	}
	pf, ok := req.UpdateFormat.IncludeTransactions.EventFormat.FiltersByParty["guardianObserver::abc"]
	if !ok || len(pf.Cumulative) != 1 {
		t.Fatalf("filtersByParty missing the observer party: %+v", req.UpdateFormat.IncludeTransactions.EventFormat.FiltersByParty)
	}
	if pf.Cumulative[0].IdentifierFilter.TemplateFilter.Value.TemplateID != emitterTemplateID {
		t.Fatalf("templateId mismatch: %+v", pf.Cumulative[0])
	}
}

// ----------------------------------------------------------------------
// LedgerEnd
// ----------------------------------------------------------------------

func TestLedgerEnd_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/state/ledger-end" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("missing/wrong bearer auth: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"offset": 99}`))
	}))
	defer srv.Close()

	off, err := LedgerEnd(context.Background(), srv.URL, "tok")
	if err != nil {
		t.Fatalf("LedgerEnd: %v", err)
	}
	if off != 99 {
		t.Fatalf("offset mismatch: got %d", off)
	}
}

func TestLedgerEnd_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()

	_, err := LedgerEnd(context.Background(), srv.URL, "bad-token")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected an HTTP 401 error, got %v", err)
	}
}

func TestLedgerEnd_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	_, err := LedgerEnd(context.Background(), srv.URL, "tok")
	if err == nil || !strings.Contains(err.Error(), "parse ledger-end response") {
		t.Fatalf("expected a parse-response error, got %v", err)
	}
}

func TestLedgerEnd_RequestConstructionError(t *testing.T) {
	// A control character in the URL makes http.NewRequestWithContext fail net/url parsing.
	_, err := LedgerEnd(context.Background(), "http://\x7f", "tok")
	if err == nil {
		t.Fatalf("expected a request-construction error for an invalid URL")
	}
}

func TestLedgerEnd_DoError(t *testing.T) {
	_, err := LedgerEnd(context.Background(), "http://127.0.0.1:1", "tok")
	if err == nil || !strings.Contains(err.Error(), "ledger-end request") {
		t.Fatalf("expected a request-execution error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Stream
// ----------------------------------------------------------------------

var upgrader = websocket.Upgrader{}

func TestStream_DialFailure(t *testing.T) {
	err := Stream(context.Background(), Config{
		JSONAPIBaseURL: "http://127.0.0.1:1", // nobody listens here
		Token:          "tok",
		ObserverParty:  "guardianObserver::abc",
	}, func(Observed) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "observer: dial") {
		t.Fatalf("expected a dial error, got %v", err)
	}
}

func TestStream_DialRejectedWithHTTPStatus(t *testing.T) {
	// A plain (non-websocket) 401 handler makes the handshake fail with an HTTP status the
	// error message should surface.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	err := Stream(context.Background(), Config{
		JSONAPIBaseURL: srv.URL,
		Token:          "tok",
		ObserverParty:  "guardianObserver::abc",
	}, func(Observed) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "observer: dial") {
		t.Fatalf("expected a dial error carrying the HTTP status, got %v", err)
	}
}

// streamFixtureFrame is exercisedUpdateFixture's shape trimmed to what Stream's decode path
// needs -- one Transaction frame with a single consuming PublishMessage event.
func streamFixtureFrame(sequence int) string {
	return `{
	  "update": {
	    "Transaction": {
	      "value": {
	        "updateId": "upd-stream",
	        "effectiveAt": "2026-07-20T12:00:00Z",
	        "offset": 1,
	        "events": [
	          {
	            "ExercisedEvent": {
	              "choice": "PublishMessage",
	              "consuming": true,
	              "exerciseResult": {
	                "registrar": "operator::abc",
	                "owner": "admin::def",
	                "emitterId": 7,
	                "sequence": ` + itoa(sequence) + `,
	                "nonce": 0,
	                "consistencyLevel": 0,
	                "payload": "aabbcc"
	              }
	            }
	          }
	        ]
	      }
	    }
	  }
	}`
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestStream_ReceivesAndYields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("missing/wrong bearer auth on the dial: %q", got)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		// Drain the client's GetUpdatesRequest frame.
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(1)))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(2)))
		// Block until the client goes away instead of racing a close frame against the
		// client's second read.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	var got []Observed
	err := Stream(context.Background(), Config{
		JSONAPIBaseURL: srv.URL,
		Token:          "tok",
		ObserverParty:  "guardianObserver::abc",
	}, func(o Observed) bool {
		got = append(got, o)
		return len(got) < 1 // stop after the first message
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(got) != 1 || got[0].Sequence != 1 {
		t.Fatalf("expected exactly one observed message with sequence=1, got %+v", got)
	}
}

func TestStream_YieldFalseStopsWithoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(1)))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(2)))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	calls := 0
	err := Stream(context.Background(), Config{JSONAPIBaseURL: srv.URL, Token: "tok", ObserverParty: "p"}, func(Observed) bool {
		calls++
		return false // stop immediately on the first observation
	})
	if err != nil {
		t.Fatalf("Stream should return nil when yield stops the loop, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one yield call, got %d", calls)
	}
}

func TestStream_MalformedFrameReturnsDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`not json`))
	}))
	defer srv.Close()

	err := Stream(context.Background(), Config{JSONAPIBaseURL: srv.URL, Token: "tok", ObserverParty: "p"}, func(Observed) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "observer: parse update frame") {
		t.Fatalf("expected a decode error, got %v", err)
	}
}

func TestStream_ServerClosesAbruptly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			conn.Close()
			return
		}
		conn.Close() // no close frame -- the client's ReadMessage must surface a real error
	}))
	defer srv.Close()

	err := Stream(context.Background(), Config{JSONAPIBaseURL: srv.URL, Token: "tok", ObserverParty: "p"}, func(Observed) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "observer: read update frame") {
		t.Fatalf("expected a read-update-frame error, got %v", err)
	}
}

func TestStream_CtxCancelledUnblocksRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		// Never write anything -- the client must be unblocked by ctx cancellation, not a
		// server-side frame.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := Stream(ctx, Config{JSONAPIBaseURL: srv.URL, Token: "tok", ObserverParty: "p"}, func(Observed) bool { return true })
	if err == nil {
		t.Fatalf("expected ctx.Err() once ctx is cancelled while blocked on ReadMessage")
	}
}

// TestConfig_Logf_NilSafe proves logf never panics when Logf is unset (Config's zero value).
func TestConfig_Logf_NilSafe(t *testing.T) {
	var c Config
	c.logf("no logger set, args=%d", 1) // must not panic
}
