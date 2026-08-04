package observer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
func streamFixtureFrame(sequence, offset int) string {
	return `{
	  "update": {
	    "Transaction": {
	      "value": {
	        "updateId": "upd-stream",
	        "effectiveAt": "2026-07-20T12:00:00Z",
	        "offset": ` + itoa(offset) + `,
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
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(1, 1)))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(2, 2)))
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
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(1, 1)))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(2, 2)))
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

// ----------------------------------------------------------------------
// Ping / read-deadline liveness
// ----------------------------------------------------------------------

func TestStream_SendsPings(t *testing.T) {
	pings := make(chan struct{}, 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil { // drain the subscription frame
			return
		}
		conn.SetPingHandler(func(string) error {
			_ = conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second))
			select {
			case pings <- struct{}{}:
			default:
			}
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil { // keep reading so control frames get processed
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = Stream(ctx, Config{
		JSONAPIBaseURL: srv.URL,
		Token:          "tok",
		ObserverParty:  "p",
		PingInterval:   20 * time.Millisecond, // ~15 intervals fit in the 300ms ctx window
	}, func(Observed) bool { return true })

	select {
	case <-pings:
	default:
		t.Fatalf("expected at least one ping to reach the server within the ctx window")
	}
}

func TestStream_StaleConnectionTimesOut(t *testing.T) {
	serverDone := make(chan struct{})
	defer close(serverDone)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if _, _, err := conn.ReadMessage(); err != nil { // drain the subscription frame
			conn.Close()
			return
		}
		// Stop reading entirely: no auto-pong, no data. Nothing on the wire will ever unblock
		// the client -- only its own read deadline can. This is the regression case for the
		// stale-socket gap (a dead connection the TCP stack hasn't noticed yet).
		<-serverDone
		conn.Close()
	}))
	defer srv.Close()

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		done <- Stream(context.Background(), Config{
			JSONAPIBaseURL: srv.URL,
			Token:          "tok",
			ObserverParty:  "p",
			PingInterval:   20 * time.Millisecond, // pongWait = 50ms
		}, func(Observed) bool { return true })
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "observer: read update frame") {
			t.Fatalf("expected a read-update-frame (deadline) error, got %v", err)
		}
		// Generous upper bound (several x pongWait, not exactly pongWait) to tolerate a loaded
		// machine; the point is that it returns at all, not the precise timing.
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("stale connection took too long to time out: %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Stream did not return within 2s of a background ctx (no deadline of its own)")
	}
}

func TestStream_PongsKeepConnectionAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		// gorilla's default ping handler auto-pongs; keep reading so control frames get
		// processed, but never send a data frame of our own.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond) // ~5x pongWait
	defer cancel()

	start := time.Now()
	err := Stream(ctx, Config{
		JSONAPIBaseURL: srv.URL,
		Token:          "tok",
		ObserverParty:  "p",
		PingInterval:   20 * time.Millisecond, // pongWait = 50ms
	}, func(Observed) bool { return true })
	elapsed := time.Since(start)

	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded once the ctx timeout elapses, got %v", err)
	}
	// Proves the read deadline was actually refreshed by pongs: the connection survived
	// multiple pongWait windows (50ms) and only ended at the ctx timeout (~250ms), not early.
	if elapsed < 150*time.Millisecond {
		t.Fatalf("connection ended too early (%s); read deadline may not be refreshing on pongs", elapsed)
	}
}

// ----------------------------------------------------------------------
// StreamWithReconnect
// ----------------------------------------------------------------------

func TestStreamWithReconnect_ResumesFromLastOffset(t *testing.T) {
	var connCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&connCount, 1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			return
		}
		var req getUpdatesRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("unmarshal GetUpdatesRequest: %v", err)
		}

		switch n {
		case 1:
			if req.BeginExclusive != 0 {
				t.Errorf("conn #1: expected beginExclusive=0, got %d", req.BeginExclusive)
			}
			_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(1, 7)))
			conn.Close() // abrupt close, no close frame -- forces a reconnect
		case 2:
			if req.BeginExclusive != 7 {
				t.Errorf("conn #2: expected beginExclusive=7 (resumed from conn #1's last offset), got %d", req.BeginExclusive)
			}
			defer conn.Close()
			_ = conn.WriteMessage(websocket.TextMessage, []byte(streamFixtureFrame(2, 9)))
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		default:
			t.Errorf("unexpected connection #%d", n)
			conn.Close()
		}
	}))
	defer srv.Close()

	var got []Observed
	err := StreamWithReconnect(context.Background(), Config{
		JSONAPIBaseURL:      srv.URL,
		Token:               "tok",
		ObserverParty:       "p",
		ReconnectMinBackoff: 5 * time.Millisecond,
	}, func(o Observed) bool {
		got = append(got, o)
		return len(got) < 2
	})
	if err != nil {
		t.Fatalf("StreamWithReconnect: %v", err)
	}
	if len(got) != 2 || got[0].Sequence != 1 || got[1].Sequence != 2 {
		t.Fatalf("expected [seq 1, seq 2], got %+v", got)
	}
	if n := atomic.LoadInt32(&connCount); n != 2 {
		t.Fatalf("expected exactly 2 connections, got %d", n)
	}
}

func TestStreamWithReconnect_DecodeErrorIsTerminal(t *testing.T) {
	var connCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&connCount, 1)
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

	err := StreamWithReconnect(context.Background(), Config{
		JSONAPIBaseURL:      srv.URL,
		Token:               "tok",
		ObserverParty:       "p",
		ReconnectMinBackoff: 5 * time.Millisecond,
	}, func(Observed) bool { return true })

	if err == nil || !errors.Is(err, ErrDecode) {
		t.Fatalf("expected a terminal ErrDecode error, got %v", err)
	}
	if n := atomic.LoadInt32(&connCount); n != 1 {
		t.Fatalf("expected exactly 1 connection (no retry on a decode error), got %d", n)
	}
}

func TestStreamWithReconnect_CtxCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := StreamWithReconnect(ctx, Config{
		JSONAPIBaseURL:      "http://127.0.0.1:1", // nobody listens here -- every dial fails
		Token:               "tok",
		ObserverParty:       "p",
		ReconnectMinBackoff: time.Second, // long enough that a prompt return proves the
		// select-against-ctx.Done() path fired, not a naturally expired backoff.
	}, func(Observed) bool { return true })
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("StreamWithReconnect took too long to notice ctx cancellation during backoff: %s", elapsed)
	}
	// Pins the M1 behavior: the last dial failure that triggered the pending backoff must be
	// visible in the returned error text, not just swallowed into a bare ctx error.
	if !strings.Contains(err.Error(), "observer: dial") {
		t.Fatalf("expected the last stream failure's text in the returned error, got %v", err)
	}
}
