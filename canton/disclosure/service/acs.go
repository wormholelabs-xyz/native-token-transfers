package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// ACS client
// ---------------------------------------------------------------------------

// acsEntry is one active contract, as this service needs it. It holds enough data to build a
// Ledger API DisclosedContract client-side.
type acsEntry struct {
	TemplateID       string
	ContractID       string
	CreatedEventBlob string // base64, as returned by the JSON Ledger API
	SynchronizerID   string
	CreateArgument   json.RawMessage // Daml-JSON create arguments; populated only when the request sets eventFormat.verbose
}

// acsClient is the minimal seam between the HTTP handlers and the upstream participant. Tests
// can fake it in place of a real JSON Ledger API. httpACSClient is the only production
// implementation. The handler resolves one LedgerEnd offset per request and passes it to every
// ActiveContracts call, so a multi-template response is one consistent snapshot.
type acsClient interface {
	LedgerEnd(ctx context.Context) (int64, error)
	ActiveContracts(ctx context.Context, parties []string, template string, activeAtOffset int64) ([]acsEntry, error)
}

// activeContractsRequest mirrors the JSON Ledger API v2's POST /v2/state/active-contracts
// request body. It has one filtersByParty entry for the disclosing party, one cumulative
// template filter per call, and includeCreatedEventBlob:true. This shape is confirmed live
// against a real participant.
type activeContractsRequest struct {
	ActiveAtOffset int64 `json:"activeAtOffset"`
	EventFormat    struct {
		FiltersByParty map[string]acsPartyFilter `json:"filtersByParty"`
		Verbose        bool                      `json:"verbose"`
	} `json:"eventFormat"`
}

type acsPartyFilter struct {
	Cumulative []acsCumulativeFilter `json:"cumulative"`
}

type acsCumulativeFilter struct {
	IdentifierFilter struct {
		TemplateFilter struct {
			Value struct {
				TemplateID              string `json:"templateId"`
				IncludeCreatedEventBlob bool   `json:"includeCreatedEventBlob"`
			} `json:"value"`
		} `json:"TemplateFilter"`
	} `json:"identifierFilter"`
}

// acsCreatedEvent is the subset of a JSON Ledger API CreatedEvent this service needs.
type acsCreatedEvent struct {
	ContractID       string          `json:"contractId"`
	TemplateID       string          `json:"templateId"`
	CreatedEventBlob string          `json:"createdEventBlob"`
	CreateArgument   json.RawMessage `json:"createArgument"`
}

// acsActiveContract is the JsActiveContract variant of a contractEntry oneOf. synchronizerId is
// a sibling field on JsActiveContract, alongside createdEvent (confirmed live).
type acsActiveContract struct {
	CreatedEvent   *acsCreatedEvent `json:"createdEvent"`
	SynchronizerID string           `json:"synchronizerId"`
}

// acsContractEntry is a Daml-JSON sum type. Only JsActiveContract carries a createdEvent. Other
// variants (e.g. an incomplete-reassignment marker) decode with a nil field and are skipped.
type acsContractEntry struct {
	JsActiveContract *acsActiveContract `json:"JsActiveContract"`
}

type acsResponseEntry struct {
	ContractEntry acsContractEntry `json:"contractEntry"`
}

// httpACSClient is the production acsClient. It talks to one participant's JSON Ledger API v2.
type httpACSClient struct {
	baseURL   string
	tokenFile string // path to the bearer token file; when set, every upstream request carries its bearer token
	client    *http.Client
}

func newHTTPACSClient(baseURL, tokenFile string) *httpACSClient {
	return &httpACSClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		tokenFile: tokenFile,
		client:    &http.Client{Timeout: httpClientTimeout},
	}
}

// authHeader sets Authorization. It re-reads tokenFile on every call. An operator can rotate the
// token file's contents, and the next request picks up the change immediately.
func (c *httpACSClient) authHeader(req *http.Request) error {
	if c.tokenFile == "" {
		return nil
	}
	token, err := readAccessToken(c.tokenFile)
	if err != nil {
		return fmt.Errorf("disclosure-service: reload access token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// readBounded reads a response body of at most maxUpstreamResponseBytes. A larger body is an
// error.
func readBounded(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxUpstreamResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxUpstreamResponseBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxUpstreamResponseBytes)
	}
	return body, nil
}

// LedgerEnd calls GET /v2/state/ledger-end. This pins ActiveContracts' activeAtOffset.
func (c *httpACSClient) LedgerEnd(ctx context.Context) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/state/ledger-end", nil)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: build ledger-end request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.authHeader(req); err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: ledger-end request: %w", err)
	}
	defer resp.Body.Close()
	body, err := readBounded(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: read ledger-end response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("disclosure-service: ledger-end: HTTP %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Offset int64 `json:"offset"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("disclosure-service: parse ledger-end response: %w", err)
	}
	return out.Offset, nil
}

// ActiveContracts queries template as the union of parties, at activeAtOffset, with
// includeCreatedEventBlob:true. A contract two parties both see is returned once. An empty
// result is a valid answer. A template can have zero live contracts for these readers.
func (c *httpACSClient) ActiveContracts(ctx context.Context, parties []string, template string, activeAtOffset int64) ([]acsEntry, error) {
	if len(parties) == 0 {
		return nil, fmt.Errorf("disclosure-service: active-contracts: at least one disclosing party is required")
	}
	var reqBody activeContractsRequest
	reqBody.ActiveAtOffset = activeAtOffset
	var cf acsCumulativeFilter
	cf.IdentifierFilter.TemplateFilter.Value.TemplateID = template
	cf.IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob = true
	reqBody.EventFormat.FiltersByParty = make(map[string]acsPartyFilter, len(parties))
	for _, p := range parties {
		reqBody.EventFormat.FiltersByParty[p] = acsPartyFilter{Cumulative: []acsCumulativeFilter{cf}}
	}
	// verbose:true so createArgument carries field labels; the /v1/flows/{flow} selectors decode it.
	reqBody.EventFormat.Verbose = true

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: marshal active-contracts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/state/active-contracts", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: build active-contracts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.authHeader(req); err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: active-contracts request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := readBounded(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: read active-contracts response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("disclosure-service: active-contracts: HTTP %d: %s", resp.StatusCode, respBody)
	}

	var entries []acsResponseEntry
	if err := json.Unmarshal(respBody, &entries); err != nil {
		return nil, fmt.Errorf("disclosure-service: parse active-contracts response: %w", err)
	}

	out := make([]acsEntry, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		ac := e.ContractEntry.JsActiveContract
		if ac == nil || ac.CreatedEvent == nil {
			continue // other contractEntry variant; skip it
		}
		if seen[ac.CreatedEvent.ContractID] {
			continue // visible to more than one disclosing party; serve it once
		}
		seen[ac.CreatedEvent.ContractID] = true
		out = append(out, acsEntry{
			TemplateID:       ac.CreatedEvent.TemplateID,
			ContractID:       ac.CreatedEvent.ContractID,
			CreatedEventBlob: ac.CreatedEvent.CreatedEventBlob,
			SynchronizerID:   ac.SynchronizerID,
			CreateArgument:   ac.CreatedEvent.CreateArgument,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------

// readAccessToken reads and trims the bearer token file. It is separate from run() so the
// trim-whitespace behavior is independently testable. A file saved with a trailing newline is
// the common case. A file that trims to nothing is an error: it would send an empty bearer
// token upstream.
func readAccessToken(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("disclosure-service: read access token file %s: %w", path, err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("disclosure-service: access token file %s is empty", path)
	}
	return token, nil
}
