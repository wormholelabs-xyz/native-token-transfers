package disclosure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Contract is one disclosed contract served by the disclosure service: the fields a Daml
// Script needs to reconstitute a Disclosure (see Playground/Disclose.daml's
// DisclosedContractIn, mkSeedDisclosure/toDisclosureByName), plus the decoded create argument
// for a generic HTTP consumer that has no Daml Script of its own to run.
type Contract struct {
	TemplateID string          `json:"templateId"`
	ContractID string          `json:"contractId"`
	Blob       string          `json:"blob"`    // hex, converted from the JSON API's base64 (see toHexBlob)
	Payload    json.RawMessage `json:"payload"` // Daml-JSON createArgument
}

// activeContractsRequest mirrors the /v2/state/active-contracts request body confirmed live
// against Splice LocalNet 0.6.12 (the plan's §2.3): filtersByParty keyed on the reading party,
// one TemplateFilter per requested template with includeCreatedEventBlob:true. Reuses the exact
// nested-struct idiom internal/observer/observer.go uses for GetUpdatesRequest -- copied rather
// than imported so this package has no internal/observer dependency (the plan's §3: observer is
// a shape reference, not a shared implementation).
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

// acsCreatedEvent is the subset of CreatedEvent fields this package needs off a JsActiveContract.
type acsCreatedEvent struct {
	ContractID       string          `json:"contractId"`
	TemplateID       string          `json:"templateId"`
	CreateArgument   json.RawMessage `json:"createArgument"`
	CreatedEventBlob string          `json:"createdEventBlob"` // base64, per the JSON API
}

// acsContractEntry is one active-contracts response element's "contractEntry" -- a Daml-JSON
// sum type. Only the JsActiveContract variant carries a createdEvent; other variants (e.g. an
// incomplete-reassignment marker) decode with JsActiveContract left nil and are skipped by
// Query, not treated as an error.
type acsContractEntry struct {
	JsActiveContract *struct {
		CreatedEvent *acsCreatedEvent `json:"createdEvent"`
	} `json:"JsActiveContract"`
}

type acsResponseEntry struct {
	ContractEntry acsContractEntry `json:"contractEntry"`
}

// ActiveContracts queries one participant's ACS over the JSON Ledger API v2, with
// includeCreatedEventBlob:true so the response carries everything a Daml Script's
// mkSeedDisclosure/toDisclosureByName needs to reconstitute a Disclosure (the plan's §2.3).
type ActiveContracts struct {
	BaseURL    string               // e.g. http://localhost:6975
	Token      string               // bearer token; empty sends no Authorization header
	Party      string               // the reader party -- the filtersByParty key
	HTTPClient *http.Client         // nil → http.DefaultClient
	Logf       func(string, ...any) // nil-safe
}

func (a *ActiveContracts) httpClient() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return http.DefaultClient
}

func (a *ActiveContracts) logf(format string, args ...any) {
	if a.Logf != nil {
		a.Logf(format, args...)
	}
}

func (a *ActiveContracts) authHeader(req *http.Request) {
	if a.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
}

// LedgerEnd fetches GET /v2/state/ledger-end -- the offset Query pins its activeAtOffset to.
// Mirrors internal/observer.LedgerEnd exactly (same endpoint, same response shape), copied
// rather than imported for the same reason as activeContractsRequest above.
func (a *ActiveContracts) LedgerEnd(ctx context.Context) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.BaseURL+"/v2/state/ledger-end", nil)
	if err != nil {
		return 0, fmt.Errorf("disclosure: build ledger-end request: %w", err)
	}
	a.authHeader(req)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return 0, fmt.Errorf("disclosure: ledger-end request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("disclosure: read ledger-end response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("disclosure: ledger-end: HTTP %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Offset int64 `json:"offset"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("disclosure: parse ledger-end response: %w\nraw: %s", err, body)
	}
	return out.Offset, nil
}

// toHexBlob re-encodes a JSON API createdEventBlob from base64 (the API's wire format) to hex
// (what Daml Script's Disclosure.blob requires). Same reasoning as
// cmd/ntt-playground/transfer.go's toAmuletSeamJSON: passing the API's base64 straight through
// to a `dpm script` input fails with "cannot parse HexString". Both encodings carry the same
// bytes.
func toHexBlob(base64Blob string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(base64Blob)
	if err != nil {
		return "", fmt.Errorf("disclosure: decode createdEventBlob (base64): %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// Query fetches the current ACS for templates (each a "Module:Entity" or
// "#<package-name>:Module:Entity" name, per observer.go's package-name convention) as a.Party,
// with includeCreatedEventBlob:true, and returns the decoded contracts plus the activeAtOffset
// the read was pinned to. Stateless and query-on-demand by design (the plan's §3): no cid is
// ever cached across calls.
func (a *ActiveContracts) Query(ctx context.Context, templates []string) ([]Contract, int64, error) {
	offset, err := a.LedgerEnd(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: query: %w", err)
	}

	cumulative := make([]acsCumulativeFilter, len(templates))
	for i, t := range templates {
		var cf acsCumulativeFilter
		cf.IdentifierFilter.TemplateFilter.Value.TemplateID = t
		cf.IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob = true
		cumulative[i] = cf
	}
	var reqBody activeContractsRequest
	reqBody.ActiveAtOffset = offset
	reqBody.EventFormat.FiltersByParty = map[string]acsPartyFilter{
		a.Party: {Cumulative: cumulative},
	}
	reqBody.EventFormat.Verbose = false

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: marshal active-contracts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+"/v2/state/active-contracts", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: build active-contracts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	a.authHeader(req)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: active-contracts request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("disclosure: read active-contracts response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("disclosure: active-contracts: HTTP %d: %s", resp.StatusCode, respBody)
	}

	var entries []acsResponseEntry
	if err := json.Unmarshal(respBody, &entries); err != nil {
		return nil, 0, fmt.Errorf("disclosure: parse active-contracts response: %w\nraw: %s", err, respBody)
	}

	contracts := make([]Contract, 0, len(entries))
	for _, e := range entries {
		ac := e.ContractEntry.JsActiveContract
		if ac == nil || ac.CreatedEvent == nil {
			// A non-JsActiveContract oneOf variant (e.g. an incomplete-reassignment
			// marker) -- not an active contract, not an error.
			continue
		}
		hexBlob, err := toHexBlob(ac.CreatedEvent.CreatedEventBlob)
		if err != nil {
			return nil, 0, fmt.Errorf("disclosure: contract %s: %w", ac.CreatedEvent.ContractID, err)
		}
		contracts = append(contracts, Contract{
			TemplateID: ac.CreatedEvent.TemplateID,
			ContractID: ac.CreatedEvent.ContractID,
			Blob:       hexBlob,
			Payload:    ac.CreatedEvent.CreateArgument,
		})
	}

	a.logf("disclosure: acs query as %s: %d template(s), %d contract(s) at offset %d", a.Party, len(templates), len(contracts), offset)
	return contracts, offset, nil
}
