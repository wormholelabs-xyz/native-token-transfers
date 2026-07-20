// Package amulet is a thin authenticated HTTP client for the Splice LocalNet validator's
// wallet/scan-proxy endpoints that a real Canton Coin (Amulet) CIP-56 custody deployment
// needs off-ledger: wallet-user onboarding, devnet tap (funding), TransferPreapproval
// creation, and the transfer-instruction registry's transfer-factory + choice-context
// resolution. Every call mints a fresh per-user bearer token via network.MintUnsafeToken --
// LocalNet's unsafe shared-secret JWT scheme -- matching the plan's "Party & user model"
// (each of sender/custody act as their own wallet user, never the operator).
//
// This package deliberately touches the ledger only through the validator's HTTP APIs, never
// through a Daml Script or a gRPC Ledger API client -- the CLI's one ledger chokepoint stays
// `dpm script` (internal/ledger.Runner); this package only resolves what a script's `--input-
// file` needs (a wallet user's party, a transfer-factory cid, and its choice context/
// disclosures) before the script call that actually submits.
package amulet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
)

// defaultOnboardTimeout is generous: a brand-new user's FIRST /v0/register call was observed
// live to take longer than 10s (the validator's onboarding automation -- wallet install
// contracts, primary party allocation -- is slow on a cold start); a retry within 60s always
// succeeded (idempotent). See the playground plan's "Verify-live results", V1.
const defaultOnboardTimeout = 60 * time.Second

// defaultRetryInterval/defaultRetryTimeout back the tap/preapproval retry loops: LocalNet's
// mining-round ingestion can lag briefly right after `network up` (findings §5), and the
// wallet's TransferPreapproval creation blocks on validator automation that can 429 while in
// flight (findings §3). Two minutes matches the plan's stated bound.
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

// retryable reports whether status/body look like the two known transient-failure shapes:
// round-ingestion lag right after `network up` (findings §5, surfaced as a 503 or an error
// body naming "no open mining round"), or the preapproval endpoint's in-flight 429
// (findings §3).
func retryable(status int, body []byte) bool {
	if status == http.StatusServiceUnavailable || status == http.StatusTooManyRequests {
		return true
	}
	return strings.Contains(strings.ToLower(string(body)), "no open mining round")
}

// ----------------------------------------------------------------------
// OnboardWalletUser
// ----------------------------------------------------------------------

type registerResponse struct {
	PartyID string `json:"party_id"`
}

// OnboardWalletUser onboards user as a validator wallet user (POST /v0/register, the new
// user's own JWT, empty body) and returns its primary party. Idempotent: re-registering an
// already-onboarded user returns the same party (findings §8).
func (c *Client) OnboardWalletUser(ctx context.Context, user string) (string, error) {
	status, body, err := c.doJSON(ctx, defaultOnboardTimeout, http.MethodPost, "/api/validator/v0/register", user, nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("amulet: onboard %s: HTTP %d: %s", user, status, body)
	}
	var out registerResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("amulet: parse register response for %s: %w\nraw: %s", user, err, body)
	}
	c.logf("amulet: onboarded %s → %s", user, out.PartyID)
	return out.PartyID, nil
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

// Tap mints Amulet to user's primary party via the devnet tap (POST /v0/wallet/tap).
// usdAmount is USD, not CC -- the handler divides by the current open round's amuletPrice
// server-side (findings §2). Retries the round-ingestion-lag failure mode for up to
// RetryTimeout.
func (c *Client) Tap(ctx context.Context, user, usdAmount string) (string, error) {
	deadline := time.Now().Add(c.retryTimeout())
	for {
		status, body, err := c.doJSON(ctx, 30*time.Second, http.MethodPost, "/api/validator/v0/wallet/tap", user, tapRequest{Amount: usdAmount})
		if err == nil && status >= 200 && status < 300 {
			var out tapResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return "", fmt.Errorf("amulet: parse tap response for %s: %w\nraw: %s", user, err, body)
			}
			c.logf("amulet: tapped %s USD for %s → %s", usdAmount, user, out.ContractID)
			return out.ContractID, nil
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
		status, body, err := c.doJSON(ctx, 30*time.Second, http.MethodPost, "/api/validator/v0/wallet/transfer-preapproval", user, nil)
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
// TransferFactory_Transfer choiceArguments (findings §4/V2).
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

// rfc3339Millis renders t the way the scan-proxy accepted live (millisecond-precision UTC,
// e.g. "2026-07-20T14:47:18.000Z" -- see the plan's V2 verify-live result).
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

	status, body, err := c.doJSON(ctx, 30*time.Second, http.MethodPost,
		"/api/validator/v0/scan-proxy/registry/transfer-instruction/v1/transfer-factory", user, reqBody)
	if err != nil {
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
