// Package observer reads the real Ledger API v2 update stream (JSON Ledger API, over
// WebSocket) as the guardianObserver party's dedicated reader user, decoding every consuming
// `PublishMessage` exercise into a WormholeMessage -- the guardian *observation*, as opposed
// to Playground.Ops:transferOut's recomputed payload. This is the CLI's only path that
// reaches the ledger without `dpm script`, and it is strictly read-only: no command it issues
// ever submits anything.
//
// The wire shapes below are pinned against a live Splice LocalNet 0.6.12 stack. The top-level
// `update` sum type is "value"-wrapped ({"update": {"Transaction": {"value": <JsTransaction>}}}),
// but individual `events[]` items are flat ({"ExercisedEvent": {...fields directly...}}).
package observer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// Config is everything Stream needs to read the update stream as one reader user.
type Config struct {
	JSONAPIBaseURL string // e.g. http://localhost:3975 (profile.JSONAPIBaseURL)
	Token          string // JWT for the reading user (guardian-watcher)
	ObserverParty  string // guardianObserver party id -- the event-format filter party
	BeginExclusive int64

	// PingInterval controls Stream's keepalive cadence: zero (or negative) uses
	// defaultPingInterval. The read deadline (pongWait) is derived from it rather than being a
	// second knob -- see pongWaitMultiplier.
	PingInterval time.Duration

	// ReconnectMinBackoff is StreamWithReconnect's initial (and post-success) retry delay; zero
	// (or negative) uses defaultReconnectMinBackoff. The cap is fixed at
	// reconnectBackoffCapFactor times this value, not a separate knob.
	ReconnectMinBackoff time.Duration

	// Logf, when non-nil, narrates connection/request/frame events. Nil-safe.
	Logf func(format string, args ...any)
}

func (c Config) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// Observed is one decoded WormholeMessage observation: everything a real off-ledger watcher
// would read off the consuming PublishMessage exercise, plus the update envelope's own
// timestamp/identity fields and the derived emitter address (proving this message came from a
// specific deployment's transceiver, per internal/wire.DerivedAddress).
type Observed struct {
	Registrar        string
	Owner            string
	EmitterID        int64
	Sequence         int64
	Nonce            int64
	ConsistencyLevel int64
	Payload          string // hex (Bytes = Text in wormhole-core)
	EffectiveAt      string // JsTransaction.effectiveAt
	UpdateID         string
	Offset           int64
	EmitterAddress   string // hex(wire.DerivedAddress(EmitterAddressTag, Registrar, Owner, EmitterID))
}

// flexInt64 decodes a JSON int64 that may render as a bare number or a quoted string.
// Canton's JSON API renders large int64 fields either way depending on context.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("observer: parse int64 from %q: %w", b, err)
	}
	*f = flexInt64(v)
	return nil
}

// wormholeMessage mirrors Wormhole.Core.State.WormholeMessage, the PublishMessage choice
// result decoded from exerciseResult.
type wormholeMessage struct {
	Registrar        string    `json:"registrar"`
	Owner            string    `json:"owner"`
	EmitterID        flexInt64 `json:"emitterId"`
	Sequence         flexInt64 `json:"sequence"`
	Nonce            flexInt64 `json:"nonce"`
	ConsistencyLevel flexInt64 `json:"consistencyLevel"`
	Payload          string    `json:"payload"`
}

// jsExercisedEvent is the subset of the JSON Ledger API's ExercisedEvent fields this package
// needs; everything else is ignored, not modeled.
type jsExercisedEvent struct {
	Choice         string          `json:"choice"`
	Consuming      bool            `json:"consuming"`
	ExerciseResult json.RawMessage `json:"exerciseResult"`
}

// jsEvent is one events[] element: exactly one of ExercisedEvent/CreatedEvent is present
// (Daml-JSON's sum-type-per-key convention); only ExercisedEvent is ever non-nil after
// decoding a CreatedEvent frame, since the field tags don't overlap.
type jsEvent struct {
	ExercisedEvent *jsExercisedEvent `json:"ExercisedEvent"`
}

// jsTransaction is the subset of JsTransaction this package reads.
type jsTransaction struct {
	UpdateID    string    `json:"updateId"`
	EffectiveAt string    `json:"effectiveAt"`
	Offset      int64     `json:"offset"`
	Events      []jsEvent `json:"events"`
}

// updateFrame is one `{"update": {...}}` frame. Only the "Transaction" kind is modeled.
// OffsetCheckpoint/Reassignment/TopologyTransaction frames decode with Update.Transaction
// left nil and are skipped by DecodeUpdateFrame.
type updateFrame struct {
	Update struct {
		Transaction *struct {
			Value jsTransaction `json:"value"`
		} `json:"Transaction"`
	} `json:"update"`
}

// ErrDecode is the sentinel every DecodeUpdateFrame failure wraps. These are deterministic
// wire-shape/programming errors (malformed JSON, a shape drift against the pinned Ledger API
// schema): retrying the same frame after a reconnect would just reproduce the same failure
// forever, so StreamWithReconnect uses errors.Is(err, ErrDecode) to treat them as terminal
// rather than retryable.
var ErrDecode = errors.New("observer: decode")

// decodeErr carries ErrDecode plus the underlying parse error through Unwrap, while Error()
// reproduces the historical formatted message (including the raw offending bytes) unchanged --
// callers that only ever did strings.Contains(err.Error(), ...) keep working verbatim.
type decodeErr struct {
	msg string
	err error
}

func (e *decodeErr) Error() string   { return e.msg }
func (e *decodeErr) Unwrap() []error { return []error{ErrDecode, e.err} }

func newDecodeErr(err error, format string, args ...any) error {
	return &decodeErr{msg: fmt.Sprintf(format, args...), err: err}
}

// DecodeUpdateFrame parses one raw update-stream frame (a WS text frame, or one element of
// the `POST /v2/updates` blocking-list array) into zero or more Observed messages -- one per
// consuming `PublishMessage` ExercisedEvent the frame's transaction carries. Returns (nil,
// nil) for non-Transaction frames (nothing to observe, not an error).
func DecodeUpdateFrame(raw []byte) ([]Observed, error) {
	var frame updateFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, newDecodeErr(err, "observer: parse update frame: %v\nraw: %s", err, raw)
	}
	if frame.Update.Transaction == nil {
		return nil, nil
	}
	tx := frame.Update.Transaction.Value

	var out []Observed
	for _, ev := range tx.Events {
		ex := ev.ExercisedEvent
		if ex == nil || !ex.Consuming || ex.Choice != "PublishMessage" {
			continue
		}
		var msg wormholeMessage
		if err := json.Unmarshal(ex.ExerciseResult, &msg); err != nil {
			return nil, newDecodeErr(err, "observer: parse PublishMessage exerciseResult: %v\nraw: %s", err, ex.ExerciseResult)
		}
		addr := wire.DerivedAddress(wire.EmitterAddressTag, msg.Registrar, msg.Owner, uint64(msg.EmitterID))
		out = append(out, Observed{
			Registrar:        msg.Registrar,
			Owner:            msg.Owner,
			EmitterID:        int64(msg.EmitterID),
			Sequence:         int64(msg.Sequence),
			Nonce:            int64(msg.Nonce),
			ConsistencyLevel: int64(msg.ConsistencyLevel),
			Payload:          msg.Payload,
			EffectiveAt:      tx.EffectiveAt,
			UpdateID:         tx.UpdateID,
			Offset:           tx.Offset,
			EmitterAddress:   hex.EncodeToString(addr[:]),
		})
	}
	return out, nil
}

// LedgerEnd fetches the current ledger end (GET /v2/state/ledger-end), the CLI's default
// --from-offset when the caller wants to observe only what happens after this call.
func LedgerEnd(ctx context.Context, baseURL, token string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/state/ledger-end", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("observer: ledger-end request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("observer: read ledger-end response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("observer: ledger-end: HTTP %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Offset int64 `json:"offset"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("observer: parse ledger-end response: %w\nraw: %s", err, body)
	}
	return out.Offset, nil
}

// emitterTemplateID is the wormhole-core Emitter template, addressed by package name rather
// than package-id. The JSON API accepts "#<package-name>:<Module>:<Entity>" and resolves it
// to whatever package-id is currently vetted, so no package-id computation is needed here.
const emitterTemplateID = "#wormhole-core:Wormhole.Core.State:Emitter"

// getUpdatesRequest mirrors the GetUpdatesRequest shape confirmed live: a single-party
// TemplateFilter on Emitter with the LEDGER_EFFECTS transaction shape, so witnessed exercises
// (not just creates) come through, since guardianObserver is only ever an observer, never a
// signatory, of the consuming PublishMessage exercise.
type getUpdatesRequest struct {
	BeginExclusive int64 `json:"beginExclusive"`
	UpdateFormat   struct {
		IncludeTransactions struct {
			EventFormat struct {
				FiltersByParty map[string]partyFilter `json:"filtersByParty"`
				Verbose        bool                   `json:"verbose"`
			} `json:"eventFormat"`
			TransactionShape string `json:"transactionShape"`
		} `json:"includeTransactions"`
	} `json:"updateFormat"`
}

type partyFilter struct {
	Cumulative []cumulativeFilter `json:"cumulative"`
}

type cumulativeFilter struct {
	IdentifierFilter struct {
		TemplateFilter struct {
			Value struct {
				TemplateID              string `json:"templateId"`
				IncludeCreatedEventBlob bool   `json:"includeCreatedEventBlob"`
			} `json:"value"`
		} `json:"TemplateFilter"`
	} `json:"identifierFilter"`
}

func buildGetUpdatesRequest(beginExclusive int64, observerParty string) getUpdatesRequest {
	var req getUpdatesRequest
	req.BeginExclusive = beginExclusive
	req.UpdateFormat.IncludeTransactions.TransactionShape = "TRANSACTION_SHAPE_LEDGER_EFFECTS"
	req.UpdateFormat.IncludeTransactions.EventFormat.Verbose = false
	var cf cumulativeFilter
	cf.IdentifierFilter.TemplateFilter.Value.TemplateID = emitterTemplateID
	cf.IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob = false
	req.UpdateFormat.IncludeTransactions.EventFormat.FiltersByParty = map[string]partyFilter{
		observerParty: {Cumulative: []cumulativeFilter{cf}},
	}
	return req
}

// Liveness/reconnect defaults and derivations. pongWaitMultiplier and reconnectBackoffCapFactor
// are deliberately not separate Config knobs -- one dial per concern keeps the surface small.
const (
	defaultPingInterval = 30 * time.Second
	pongWaitMultiplier  = 2.5 // pongWait = pingInterval * pongWaitMultiplier

	// pingWriteDeadline bounds the control-frame write gorilla/websocket issues for each
	// keepalive ping; short because a ping that can't be written promptly means the socket
	// is already in trouble and the read deadline will catch it regardless.
	pingWriteDeadline = 5 * time.Second

	defaultReconnectMinBackoff = 500 * time.Millisecond
	reconnectBackoffCapFactor  = 20 // backoff cap = ReconnectMinBackoff * reconnectBackoffCapFactor
)

// toWebsocketURL converts an http(s):// JSON API base URL to its ws(s):// equivalent.
func toWebsocketURL(httpBaseURL string) string {
	switch {
	case strings.HasPrefix(httpBaseURL, "https://"):
		return "wss://" + strings.TrimPrefix(httpBaseURL, "https://")
	case strings.HasPrefix(httpBaseURL, "http://"):
		return "ws://" + strings.TrimPrefix(httpBaseURL, "http://")
	default:
		return httpBaseURL
	}
}

// Stream dials ws(s)://<JSONAPIBaseURL>/v2/updates, sends one GetUpdatesRequest frame, and
// invokes yield for every consuming PublishMessage ExercisedEvent until ctx is done or yield
// returns false. Never submits anything -- purely a read-only subscription, proving the
// observation with the reader's CanReadAs(observerParty) rights alone.
//
// Auth: the Canton 3.5 JSON API docs specify two `Sec-WebSocket-Protocol` subprotocols
// ("daml.ws.auth", "jwt.token.<jwt>") for browser clients, which cannot set arbitrary headers
// on a WebSocket handshake. That convention was tried first against a live 0.6.12 LocalNet and
// failed: the server accepted the handshake but rejected the very first request with
// `{"grpcCodeValue":16,...}` (UNAUTHENTICATED, "a security-sensitive error"). A plain
// `Authorization: Bearer <token>` header, which a non-browser Go client can set freely, was
// then confirmed to work: two real PublishMessage exercises streamed back intact.
func Stream(ctx context.Context, cfg Config, yield func(Observed) bool) error {
	wsURL := toWebsocketURL(cfg.JSONAPIBaseURL) + "/v2/updates"
	cfg.logf("observer: dialing %s (beginExclusive=%d, party=%s)", wsURL, cfg.BeginExclusive, cfg.ObserverParty)

	header := http.Header{}
	header.Set("Authorization", "Bearer "+cfg.Token)
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		detail := ""
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			detail = fmt.Sprintf(" (HTTP %d: %s)", resp.StatusCode, body)
		}
		return fmt.Errorf("observer: dial %s: %w%s", wsURL, err, detail)
	}
	defer conn.Close()

	req := buildGetUpdatesRequest(cfg.BeginExclusive, cfg.ObserverParty)
	if err := conn.WriteJSON(req); err != nil {
		return fmt.Errorf("observer: send GetUpdatesRequest: %w", err)
	}
	cfg.logf("observer: subscribed, streaming updates as %s", cfg.ObserverParty)

	pingInterval := cfg.PingInterval
	if pingInterval <= 0 {
		pingInterval = defaultPingInterval
	}
	pongWait := time.Duration(float64(pingInterval) * pongWaitMultiplier)

	// A dead-but-unclosed TCP connection (NAT timeout, participant restart without FIN) would
	// otherwise block ReadMessage forever, indistinguishable from "no traffic". The read
	// deadline is refreshed by every pong and every successful data read, so it only fires
	// once nothing -- not even a keepalive pong -- has come back for pongWait.
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return fmt.Errorf("observer: set initial read deadline: %w", err)
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	// gorilla/websocket's Conn.ReadMessage has no context parameter; close the connection
	// when ctx ends so a blocked Read unblocks with an error instead of leaking the goroutine
	// past the caller's deadline/cancellation. The same goroutine drives the keepalive ping:
	// WriteControl is documented safe to call concurrently with reads and with other writes to
	// the same connection, and the only other write (the subscription frame above) already
	// happened before this goroutine starts, so there is no write/write race either.
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				_ = conn.Close()
				return
			case <-done:
				return
			case <-ticker.C:
				_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(pingWriteDeadline))
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("observer: read update frame: %w", err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return fmt.Errorf("observer: refresh read deadline: %w", err)
		}
		observed, err := DecodeUpdateFrame(raw)
		if err != nil {
			return err
		}
		for _, o := range observed {
			cfg.logf("observer: observed PublishMessage sequence=%d emitterId=%d emitterAddress=%s", o.Sequence, o.EmitterID, o.EmitterAddress)
			if !yield(o) {
				return nil
			}
		}
	}
}

// StreamWithReconnect wraps Stream with automatic re-subscription from the last yielded
// offset, so a dropped connection self-heals instead of surfacing as a fatal error. Same
// contract as Stream: nil once yield stops the loop, ctx.Err() on cancellation.
//
// Exactly-once argument: decoding one frame is all-or-nothing (DecodeUpdateFrame either
// produces the frame's observations or returns an error, never a partial set), and every yield
// for that frame happens with no intervening network I/O. A connection failure therefore always
// falls strictly between frames, never mid-frame, so resuming a fresh GetUpdatesRequest with
// beginExclusive set to the last offset this call actually yielded can never re-deliver an
// already-yielded observation. Any transactions between that offset and the point of failure
// that didn't match the filter are simply re-scanned and re-skipped on reconnect -- harmless.
//
// Decode errors are the one exception: errors.Is(err, ErrDecode) marks a deterministic
// wire-shape/programming error, and retrying would just replay the same bad frame forever, so
// those are returned immediately rather than retried. Retries are otherwise unbounded by
// design -- the caller's ctx (the CLI's --timeout) is what bounds the total wait.
//
// If ctx ends while a stream failure is already pending retry, the returned error wraps ctx's
// error together with that last failure's text (still satisfying errors.Is(err, ctx.Err()) via
// %w) so the caller sees the underlying cause -- e.g. an HTTP 401 on dial -- rather than a bare
// "context deadline exceeded". If ctx ends with no prior failure (e.g. cancelled mid-stream),
// ctx.Err() is returned unchanged.
func StreamWithReconnect(ctx context.Context, cfg Config, yield func(Observed) bool) error {
	minBackoff := cfg.ReconnectMinBackoff
	if minBackoff <= 0 {
		minBackoff = defaultReconnectMinBackoff
	}
	maxBackoff := minBackoff * reconnectBackoffCapFactor
	backoff := minBackoff

	var lastOffset int64
	haveOffset := false
	var lastErr error // the retryable failure that triggered the most recent backoff, if any

	wrappedYield := func(o Observed) bool {
		lastOffset = o.Offset
		haveOffset = true
		backoff = minBackoff // reset after any successful observation
		return yield(o)
	}

	for {
		err := Stream(ctx, cfg, wrappedYield)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctxErrWithCause(ctx.Err(), lastErr)
		}
		if errors.Is(err, ErrDecode) {
			return err
		}

		lastErr = err
		cfg.logf("observer: stream error, reconnecting in %s: %v", backoff, err)
		select {
		case <-ctx.Done():
			return ctxErrWithCause(ctx.Err(), lastErr)
		case <-time.After(backoff):
		}

		if haveOffset {
			cfg.BeginExclusive = lastOffset
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// ctxErrWithCause annotates ctxErr with the last retryable stream failure that preceded it, when
// there was one. errors.Is(result, ctxErr) keeps working since %w preserves the wrapped chain.
func ctxErrWithCause(ctxErr, lastErr error) error {
	if lastErr == nil {
		return ctxErr
	}
	return fmt.Errorf("%w (last attempt: %v)", ctxErr, lastErr)
}
