// Package amulet is a small authenticated HTTP client for a Splice LocalNet validator's
// wallet and scan-proxy endpoints. Splice is the Canton Network stack; Amulet is its native
// Canton Coin token. A real Amulet CIP-56 custody deployment needs these calls off-ledger:
// onboarding a wallet user, tapping the devnet faucet for funds, creating a
// TransferPreapproval, and resolving the transfer-instruction registry's transfer factory and
// choice context. Every call mints a fresh per-user bearer token via network.MintUnsafeToken
// (LocalNet's unsafe shared-secret JWT scheme). The sender and the custody party each act as
// their own wallet user; the operator never acts for them.
//
// This package reaches the ledger only through the validator's HTTP APIs, never through a
// Daml Script or a gRPC Ledger API client. The CLI's single ledger entry point stays
// `dpm script` (internal/ledger.Runner). This package only resolves what a script's
// --input-file needs -- a wallet user's party, a transfer-factory contract id, and its choice
// context and disclosures -- before the script call that actually submits.
package amulet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
)

// defaultOnboardTimeout is deliberately generous. A brand-new user's first /v0/register call
// can take more than 10s, because the validator's onboarding automation (installing wallet
// contracts and allocating the primary party) is slow on a cold start. The call is
// idempotent, so a retry within 60s succeeds.
const defaultOnboardTimeout = 60 * time.Second

// defaultRetryInterval and defaultRetryTimeout back the tap and preapproval retry loops.
// Right after `network up`, LocalNet's mining-round ingestion can lag briefly. The wallet's
// TransferPreapproval creation waits on validator automation that can return HTTP 429 while
// still in flight. Two minutes is enough to cover both.
const (
	defaultRetryInterval = 5 * time.Second
	defaultRetryTimeout  = 2 * time.Minute
)

// Client is a per-LocalNet validator HTTP client. Zero value is usable once ValidatorBaseURL
// is set; HTTPClient/RetryInterval/RetryTimeout default when left zero (tests override the
// retry knobs to keep unit tests fast).
type Client struct {
	ValidatorBaseURL string
	HTTPClient       *http.Client
	RetryInterval    time.Duration
	RetryTimeout     time.Duration

	// Logf, when non-nil, narrates every HTTP call (method/path/user, retries). Nil-safe.
	Logf func(format string, args ...any)
}

func (c *Client) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

func (c *Client) httpClient(timeout time.Duration) *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: timeout}
}

func (c *Client) retryInterval() time.Duration {
	if c.RetryInterval > 0 {
		return c.RetryInterval
	}
	return defaultRetryInterval
}

func (c *Client) retryTimeout() time.Duration {
	if c.RetryTimeout > 0 {
		return c.RetryTimeout
	}
	return defaultRetryTimeout
}

// doJSON issues an authenticated request as sub (a fresh LocalNet unsafe JWT per call, never
// cached -- these calls are infrequent and correctness matters more than shaving a JWT sign),
// returning the response status and body. body may be nil (GET-shaped POST with no payload,
// e.g. CreateTransferPreapproval).
func (c *Client) doJSON(ctx context.Context, timeout time.Duration, method, path, sub string, body any) (int, []byte, error) {
	token, err := network.MintUnsafeToken(sub, timeout+time.Hour)
	if err != nil {
		return 0, nil, fmt.Errorf("amulet: mint token for %s: %w", sub, err)
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("amulet: marshal request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.ValidatorBaseURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	c.logf("amulet: %s %s (user=%s)", method, path, sub)
	resp, err := c.httpClient(timeout).Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("amulet: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("amulet: read response for %s %s: %w", method, path, err)
	}
	return resp.StatusCode, respBody, nil
}

// retryable reports whether status/body look like one of the two known transient failures:
// mining-round ingestion lag right after `network up` (an HTTP 503, or an error body naming
// "no open mining round"), or the preapproval endpoint's in-flight HTTP 429.
func retryable(status int, body []byte) bool {
	if status == http.StatusServiceUnavailable || status == http.StatusTooManyRequests {
		return true
	}
	return strings.Contains(strings.ToLower(string(body)), "no open mining round")
}

// isTransientTransportError reports whether err looks like a client-side timeout or network
// hiccup rather than a definitive failure. Tap and preapproval calls can occasionally take
// longer than the per-attempt HTTP timeout under validator automation load, which is not a
// real error, so they deserve the same retry treatment as an HTTP 503 or 429 rather than
// failing the whole call on one slow attempt.
func isTransientTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// ----------------------------------------------------------------------
// OnboardWalletUser
// ----------------------------------------------------------------------

type registerResponse struct {
	PartyID string `json:"party_id"`
}

// OnboardWalletUser onboards user as a validator wallet user (POST /v0/register with the new
// user's own JWT and an empty body) and returns its primary party. It is idempotent:
// re-registering an already-onboarded user returns the same party. A cold-start onboarding
// can exceed even the generous per-attempt timeout, so it retries a transient transport
// timeout for up to RetryTimeout.
func (c *Client) OnboardWalletUser(ctx context.Context, user string) (string, error) {
	deadline := time.Now().Add(c.retryTimeout())
	for {
		status, body, err := c.doJSON(ctx, defaultOnboardTimeout, http.MethodPost, "/api/validator/v0/register", user, nil)
		if err == nil && status >= 200 && status < 300 {
			var out registerResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return "", fmt.Errorf("amulet: parse register response for %s: %w\nraw: %s", user, err, body)
			}
			c.logf("amulet: onboarded %s → %s", user, out.PartyID)
			return out.PartyID, nil
		}
		if err != nil && isTransientTransportError(err) && time.Now().Before(deadline) {
			c.logf("amulet: onboard %s: transient transport error (%v), retrying", user, err)
			if !sleepOrDone(ctx, c.retryInterval()) {
				return "", ctx.Err()
			}
			continue
		}
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("amulet: onboard %s: HTTP %d: %s", user, status, body)
	}
}

// ----------------------------------------------------------------------
// Tap
// ----------------------------------------------------------------------

type tapRequest struct {
	Amount string `json:"amount"`
}

type tapResponse struct {
	ContractID string `json:"contract_id"`
}

// Tap mints Amulet to user's primary party via the devnet faucet (POST /v0/wallet/tap).
// usdAmount is in USD, not Canton Coin: the handler divides it by the current open round's
// Amulet price server-side. Retries the mining-round ingestion lag for up to RetryTimeout.
func (c *Client) Tap(ctx context.Context, user, usdAmount string) (string, error) {
	deadline := time.Now().Add(c.retryTimeout())
	for {
		status, body, err := c.doJSON(ctx, 45*time.Second, http.MethodPost, "/api/validator/v0/wallet/tap", user, tapRequest{Amount: usdAmount})
		if err == nil && status >= 200 && status < 300 {
			var out tapResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return "", fmt.Errorf("amulet: parse tap response for %s: %w\nraw: %s", user, err, body)
			}
			c.logf("amulet: tapped %s USD for %s → %s", usdAmount, user, out.ContractID)
			return out.ContractID, nil
		}
		if err != nil && isTransientTransportError(err) && time.Now().Before(deadline) {
			c.logf("amulet: tap %s: transient transport error (%v), retrying", user, err)
			if !sleepOrDone(ctx, c.retryInterval()) {
				return "", ctx.Err()
			}
			continue
		}
		if err == nil && retryable(status, body) && time.Now().Before(deadline) {
			c.logf("amulet: tap %s: transient failure (HTTP %d: %s), retrying", user, status, body)
			if !sleepOrDone(ctx, c.retryInterval()) {
				return "", ctx.Err()
			}
			continue
		}
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("amulet: tap %s: HTTP %d: %s", user, status, body)
	}
}

// ----------------------------------------------------------------------
// CreateTransferPreapproval
// ----------------------------------------------------------------------

type transferPreapprovalResponse struct {
	TransferPreapprovalContractID string `json:"transfer_preapproval_contract_id"`
}

// CreateTransferPreapproval creates (or, on 409, finds the already-existing)
// TransferPreapproval for user's primary party (POST /v0/wallet/transfer-preapproval, no
// request body). Retries 429 (validator automation still converting the proposal in flight)
// for up to RetryTimeout.
func (c *Client) CreateTransferPreapproval(ctx context.Context, user string) (string, error) {
	deadline := time.Now().Add(c.retryTimeout())
	for {
		status, body, err := c.doJSON(ctx, 45*time.Second, http.MethodPost, "/api/validator/v0/wallet/transfer-preapproval", user, nil)
		if err == nil && (status == http.StatusOK || status == http.StatusConflict) {
			var out transferPreapprovalResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return "", fmt.Errorf("amulet: parse transfer-preapproval response for %s: %w\nraw: %s", user, err, body)
			}
			if status == http.StatusConflict {
				c.logf("amulet: transfer-preapproval for %s already exists → %s", user, out.TransferPreapprovalContractID)
			} else {
				c.logf("amulet: created transfer-preapproval for %s → %s", user, out.TransferPreapprovalContractID)
			}
			return out.TransferPreapprovalContractID, nil
		}
		if err != nil && isTransientTransportError(err) && time.Now().Before(deadline) {
			c.logf("amulet: transfer-preapproval for %s: transient transport error (%v), retrying", user, err)
			if !sleepOrDone(ctx, c.retryInterval()) {
				return "", ctx.Err()
			}
			continue
		}
		if err == nil && status == http.StatusTooManyRequests && time.Now().Before(deadline) {
			c.logf("amulet: transfer-preapproval for %s: in flight (429), retrying", user)
			if !sleepOrDone(ctx, c.retryInterval()) {
				return "", ctx.Err()
			}
			continue
		}
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("amulet: transfer-preapproval for %s: HTTP %d: %s", user, status, body)
	}
}

// ----------------------------------------------------------------------
// GetTransferFactory
// ----------------------------------------------------------------------

// TransferArgs is what GetTransferFactory needs to build the registry's
// TransferFactory_Transfer choiceArguments.
type TransferArgs struct {
	DSO      string // instrumentId.admin / expectedAdmin
	Sender   string
	Receiver string
	Amount   string // decimal string at the instrument's own scale, e.g. "1.0000000000"
}

// DisclosedContract mirrors one element of the registry response's disclosedContracts.
type DisclosedContract struct {
	TemplateID       string `json:"templateId"`
	ContractID       string `json:"contractId"`
	CreatedEventBlob string `json:"createdEventBlob"`
	SynchronizerID   string `json:"synchronizerId"`
}

// TransferFactory is the registry's resolved factory cid, transfer kind, and choice context
// for one TransferFactory_Transfer call.
type TransferFactory struct {
	FactoryID          string
	TransferKind       string // must be "direct" for the custody lock (else the on-ledger hook aborts)
	ChoiceContextData  json.RawMessage
	DisclosedContracts []DisclosedContract
}

type choiceContextWire struct {
	ChoiceContextData  json.RawMessage     `json:"choiceContextData"`
	DisclosedContracts []DisclosedContract `json:"disclosedContracts"`
}

type transferFactoryResponse struct {
	FactoryID     string            `json:"factoryId"`
	TransferKind  string            `json:"transferKind"`
	ChoiceContext choiceContextWire `json:"choiceContext"`
}

// rfc3339Millis renders t the way the scan-proxy accepts it: millisecond-precision UTC, e.g.
// "2026-07-20T14:47:18.000Z".
func rfc3339Millis(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// GetTransferFactory posts the Daml-JSON TransferFactory_Transfer arguments to the
// scan-proxy's transfer-factory endpoint and returns the resolved factory cid, transfer
// kind, and choice context. Fails fast (without submitting anything) if transferKind is not
// "direct": anything else would settle Pending, and Cip56CustodyToken's synchronous-settle
// requirement would abort the on-ledger call anyway -- better to name the cause here.
func (c *Client) GetTransferFactory(ctx context.Context, user string, args TransferArgs) (TransferFactory, error) {
	now := time.Now()
	reqBody := map[string]any{
		"choiceArguments": map[string]any{
			"expectedAdmin": args.DSO,
			"transfer": map[string]any{
				"sender":           args.Sender,
				"receiver":         args.Receiver,
				"amount":           args.Amount,
				"instrumentId":     map[string]any{"admin": args.DSO, "id": "Amulet"},
				"requestedAt":      rfc3339Millis(now),
				"executeBefore":    rfc3339Millis(now.Add(time.Hour)),
				"inputHoldingCids": []string{},
				"meta":             map[string]any{"values": map[string]any{}},
			},
			"extraArgs": map[string]any{
				"context": map[string]any{"values": map[string]any{}},
				"meta":    map[string]any{"values": map[string]any{}},
			},
		},
		"excludeDebugFields": true,
	}

	deadline := time.Now().Add(c.retryTimeout())
	var status int
	var body []byte
	var err error
	for {
		status, body, err = c.doJSON(ctx, 45*time.Second, http.MethodPost,
			"/api/validator/v0/scan-proxy/registry/transfer-instruction/v1/transfer-factory", user, reqBody)
		if err == nil {
			break
		}
		if isTransientTransportError(err) && time.Now().Before(deadline) {
			c.logf("amulet: transfer-factory: transient transport error (%v), retrying", err)
			if !sleepOrDone(ctx, c.retryInterval()) {
				return TransferFactory{}, ctx.Err()
			}
			continue
		}
		return TransferFactory{}, err
	}
	if status < 200 || status >= 300 {
		return TransferFactory{}, fmt.Errorf("amulet: transfer-factory: HTTP %d: %s", status, body)
	}
	var out transferFactoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return TransferFactory{}, fmt.Errorf("amulet: parse transfer-factory response: %w\nraw: %s", err, body)
	}
	if out.TransferKind != "direct" {
		return TransferFactory{}, fmt.Errorf(
			"amulet: transfer-factory returned transferKind=%q, want \"direct\" (receiver has no TransferPreapproval -- deploy creates one; re-run deploy or check validator automation)",
			out.TransferKind)
	}
	c.logf("amulet: transfer-factory %s → transferKind=direct, %d disclosed contracts", out.FactoryID, len(out.ChoiceContext.DisclosedContracts))
	return TransferFactory{
		FactoryID:          out.FactoryID,
		TransferKind:       out.TransferKind,
		ChoiceContextData:  out.ChoiceContext.ChoiceContextData,
		DisclosedContracts: out.ChoiceContext.DisclosedContracts,
	}, nil
}

// sleepOrDone waits for d or ctx cancellation, reporting false if ctx ended first.
func sleepOrDone(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}
