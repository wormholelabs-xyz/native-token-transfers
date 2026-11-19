package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// ACS client
// ---------------------------------------------------------------------------

const (
	// maxMatchingContractsPerQuery caps the contracts a filtered query keeps. Each selector needs
	// one; the selector reports more than one as ambiguous.
	maxMatchingContractsPerQuery = 8
	// maxScannedContractsPerQuery bounds the array elements read from one upstream response.
	maxScannedContractsPerQuery = 100_000
)

var (
	errLedgerEndOffsetMissing     = errors.New("disclosure-service: ledger-end response has no offset")
	errLedgerEndOffsetNotPositive = errors.New("disclosure-service: ledger-end offset is not positive")
	errUpstreamResponseTooLarge   = errors.New("disclosure-service: upstream response exceeds the byte limit")
	errUpstreamResponseMalformed  = errors.New("disclosure-service: upstream response is malformed")
	errUpstreamScanBoundExceeded  = errors.New("disclosure-service: upstream response exceeds the scanned-contract bound")
	errMatchCapExceeded           = errors.New("disclosure-service: matching contracts exceed the query cap")
	errUpstreamTemplateMismatch   = errors.New("disclosure-service: upstream contract has another package or template")
	// errUpstreamListLimitExceeded is the participant's 413: the template has more visible
	// contracts than its http-list-max-elements-limit.
	errUpstreamListLimitExceeded = errors.New("disclosure-service: upstream list exceeds the participant http-list-max-elements-limit (HTTP 413)")
)

// partyID is a Canton party identifier.
type partyID string

// contractID is a Canton contract identifier.
type contractID string

// acsEntry is one active contract, as this service needs it. It holds enough data to build a
// Ledger API DisclosedContract client-side.
type acsEntry struct {
	TemplateID       string
	PackageName      string
	ContractID       string
	CreatedEventBlob string // base64, as returned by the JSON Ledger API
	SynchronizerID   string
	CreateArgument   json.RawMessage // Daml-JSON create arguments; populated only when the request sets eventFormat.verbose
	CreatedAt        string          // ledger effective time of the create
}

// createArgumentFieldMatch requires one createArgument field to equal one of wantValues.
type createArgumentFieldMatch struct {
	fieldPath  []string // for example {"instrumentId", "admin"}
	wantValues []string // one value, or the candidate prefixes of a digest
	compareHex bool     // lowercase both sides
}

// activeContractsQuery is one template read at one offset, filtered before the match cap.
type activeContractsQuery struct {
	readerParties  []partyID
	template       string
	activeAtOffset int64
	fieldMatches   []createArgumentFieldMatch // empty: every contract
	maxMatches     int                        // more kept contracts is errMatchCapExceeded
}

// cacheKey is a JSON encoding of every query field: each distinct query has its own key.
func (query activeContractsQuery) cacheKey() string {
	type fieldMatchKey struct {
		Path       []string
		Want       []string
		CompareHex bool
	}
	fieldMatchKeys := make([]fieldMatchKey, 0, len(query.fieldMatches))
	for _, fieldMatch := range query.fieldMatches {
		fieldMatchKeys = append(fieldMatchKeys, fieldMatchKey{Path: fieldMatch.fieldPath, Want: fieldMatch.wantValues, CompareHex: fieldMatch.compareHex})
	}
	encodedKey, err := json.Marshal(struct {
		Kind        string
		Readers     []partyID
		Template    string
		Offset      int64
		FieldMatch  []fieldMatchKey
		MaxMatching int
	}{"active-contracts", query.readerParties, query.template, query.activeAtOffset, fieldMatchKeys, query.maxMatches})
	if err != nil {
		panic(fmt.Sprintf("disclosure-service: encode cache key: %v", err))
	}
	return string(encodedKey)
}

func (query activeContractsQuery) check() error {
	if len(query.readerParties) == 0 {
		return errors.New("disclosure-service: active-contracts: at least one reader party is required")
	}
	if query.template == "" {
		return errors.New("disclosure-service: active-contracts: template is empty")
	}
	if query.activeAtOffset < 1 {
		return fmt.Errorf("%w: activeAtOffset %d", errLedgerEndOffsetNotPositive, query.activeAtOffset)
	}
	if query.maxMatches < 1 {
		return fmt.Errorf("disclosure-service: active-contracts: maxMatches %d is not positive", query.maxMatches)
	}
	return nil
}

// matchesCreateArgument reports whether every field match holds for createArgument.
func (query activeContractsQuery) matchesCreateArgument(createArgument json.RawMessage) bool {
	for _, fieldMatch := range query.fieldMatches {
		if !fieldMatch.matchesCreateArgument(createArgument) {
			return false
		}
	}
	return true
}

// matchesCreateArgument compares the field at fieldPath. A string compares by its value, any
// other scalar by its JSON text. A missing field, null, or a path through a non-object is false.
func (fieldMatch createArgumentFieldMatch) matchesCreateArgument(createArgument json.RawMessage) bool {
	gotValue, found := lookupCreateArgumentScalar(createArgument, fieldMatch.fieldPath)
	if !found {
		return false
	}
	if fieldMatch.compareHex {
		gotValue = strings.ToLower(gotValue)
	}
	return slices.ContainsFunc(fieldMatch.wantValues, func(wantValue string) bool {
		if fieldMatch.compareHex {
			wantValue = strings.ToLower(wantValue)
		}
		return gotValue == wantValue
	})
}

// lookupCreateArgumentScalar returns the scalar at fieldPath: a string by its value, any other
// scalar by its JSON text. A missing field, null, an object, an array, or a path through a
// non-object is not found.
func lookupCreateArgumentScalar(createArgument json.RawMessage, fieldPath []string) (string, bool) {
	if len(fieldPath) == 0 {
		panic("disclosure-service: createArgument field path is empty")
	}
	fieldValue := createArgument
	for _, fieldName := range fieldPath {
		var fieldsByName map[string]json.RawMessage
		if err := json.Unmarshal(fieldValue, &fieldsByName); err != nil {
			return "", false
		}
		next, present := fieldsByName[fieldName]
		if !present {
			return "", false
		}
		fieldValue = next
	}
	trimmedValue := bytes.TrimSpace(fieldValue)
	if len(trimmedValue) == 0 || bytes.Equal(trimmedValue, []byte("null")) {
		return "", false
	}
	if trimmedValue[0] == '{' || trimmedValue[0] == '[' {
		return "", false
	}
	if trimmedValue[0] != '"' {
		return string(trimmedValue), true
	}
	var stringValue string
	if err := json.Unmarshal(trimmedValue, &stringValue); err != nil {
		return "", false
	}
	return stringValue, true
}

// acsClient is the minimal seam between the HTTP handlers and the upstream participant. Tests
// can fake it in place of a real JSON Ledger API. httpACSClient is the only production
// implementation. The handler resolves one LedgerEnd offset per request and passes it to every
// ActiveContracts call, so a multi-template response is one consistent snapshot.
type acsClient interface {
	LedgerEnd(ctx context.Context) (int64, error)
	ActiveContracts(ctx context.Context, query activeContractsQuery) ([]acsEntry, error)
	ActiveContractByID(ctx context.Context, readerParties []partyID, wantContractID contractID) (acsEntry, bool, error)
	ActiveContractAmountTotals(ctx context.Context, query amountTotalsQuery) (amountTotalsRead, error)
}

// activeContractsRequest mirrors the JSON Ledger API v2's POST /v2/state/active-contracts
// request body. It has one filtersByParty entry per reader party, one cumulative template filter
// per call, and includeCreatedEventBlob:true. This shape is confirmed live against a real
// participant.
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

// eventsByContractIDRequest mirrors POST /v2/events/events-by-contract-id: one wildcard filter per
// reader party, with includeCreatedEventBlob:true.
type eventsByContractIDRequest struct {
	ContractID  string `json:"contractId"`
	EventFormat struct {
		FiltersByParty map[string]wildcardPartyFilter `json:"filtersByParty"`
		Verbose        bool                           `json:"verbose"`
	} `json:"eventFormat"`
}

type wildcardPartyFilter struct {
	Cumulative []wildcardCumulativeFilter `json:"cumulative"`
}

type wildcardCumulativeFilter struct {
	IdentifierFilter struct {
		WildcardFilter struct {
			Value struct {
				IncludeCreatedEventBlob bool `json:"includeCreatedEventBlob"`
			} `json:"value"`
		} `json:"WildcardFilter"`
	} `json:"identifierFilter"`
}

// eventsByContractIDResponse is the subset of JsGetEventsByContractIdResponse this service needs.
type eventsByContractIDResponse struct {
	Created *struct {
		CreatedEvent   *acsCreatedEvent `json:"createdEvent"`
		SynchronizerID string           `json:"synchronizerId"`
	} `json:"created"`
	Archived json.RawMessage `json:"archived"`
}

// acsCreatedEvent is the subset of a JSON Ledger API CreatedEvent this service needs.
type acsCreatedEvent struct {
	ContractID       string          `json:"contractId"`
	TemplateID       string          `json:"templateId"`
	PackageName      string          `json:"packageName"`
	CreatedEventBlob string          `json:"createdEventBlob"`
	CreateArgument   json.RawMessage `json:"createArgument"`
	CreatedAt        string          `json:"createdAt"`
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
	baseURL          string
	tokens           bearerTokenSource // nil for an unauthenticated upstream
	client           *http.Client
	maxResponseBytes int64
	now              func() time.Time // read time of the Coin totals
}

// newHTTPACSClient panics when baseURL breaks checkJSONAPIURL; parseFlags checks it first.
func newHTTPACSClient(baseURL string, client *http.Client, tokens bearerTokenSource) *httpACSClient {
	if client == nil {
		panic("disclosure-service: newHTTPACSClient: nil http client")
	}
	if err := checkJSONAPIURL(baseURL, tokens != nil, upstreamTLSFiles{}); err != nil {
		panic(err.Error())
	}
	return &httpACSClient{
		baseURL:          strings.TrimRight(baseURL, "/"),
		tokens:           tokens,
		client:           client,
		maxResponseBytes: maxUpstreamResponseBytes,
		now:              time.Now,
	}
}

// maxUpstreamAttemptsPerRequest is one send plus one retry after an upstream 401.
const maxUpstreamAttemptsPerRequest = 2

// bearerToken returns the token for one request, or "" for an unauthenticated upstream.
func (c *httpACSClient) bearerToken(ctx context.Context) (string, error) {
	if c.tokens == nil {
		return "", nil
	}
	token, err := c.tokens.BearerToken(ctx)
	if err != nil {
		return "", fmt.Errorf("disclosure-service: upstream bearer token: %w", err)
	}
	//SECURITY: an auth mode is configured; an empty token is an error.
	if token == "" {
		return "", fmt.Errorf("disclosure-service: upstream bearer token is empty")
	}
	return token, nil
}

// sendUpstreamRequest sends one request to the JSON API. On an upstream 401 it discards the
// token it used and sends once more. A 401 on the retry is a token source failure. The caller
// closes the response body.
func (c *httpACSClient) sendUpstreamRequest(ctx context.Context, operation, method, path string, requestBody []byte) (*http.Response, error) {
	for attempt := 1; attempt <= maxUpstreamAttemptsPerRequest; attempt++ {
		var bodyReader io.Reader
		if requestBody != nil {
			bodyReader = bytes.NewReader(requestBody)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("disclosure-service: build %s request: %w", operation, err)
		}
		req.Header.Set("Content-Type", "application/json")
		token, err := c.bearerToken(ctx)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("disclosure-service: %s request: %w", operation, err)
		}
		if token == "" {
			return resp, nil
		}
		if resp.StatusCode != http.StatusUnauthorized {
			c.tokens.RecordAcceptedBearerToken(token)
			return resp, nil
		}
		if attempt == maxUpstreamAttemptsPerRequest {
			//SECURITY: bounds the token requests while the upstream rejects each new token.
			c.tokens.RecordRejectedRetryBearerToken(token)
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxLoggedUpstreamBodyBytes))
		_ = resp.Body.Close()
		c.tokens.DiscardRejectedBearerToken(token)
	}
	panic("disclosure-service: sendUpstreamRequest: the attempt loop returns on its last attempt")
}

// boundedBodyReader fails with errUpstreamResponseTooLarge once source yields more than
// remainingBytes. limitExceeded stays set, and each later Read fails the same way.
type boundedBodyReader struct {
	source         io.Reader
	remainingBytes int64
	limitExceeded  bool
}

func (reader *boundedBodyReader) Read(buffer []byte) (int, error) {
	if reader.limitExceeded {
		return 0, errUpstreamResponseTooLarge
	}
	if reader.remainingBytes <= 0 {
		var probe [1]byte
		probeCount, err := reader.source.Read(probe[:])
		if probeCount > 0 {
			reader.limitExceeded = true
			return 0, errUpstreamResponseTooLarge
		}
		return 0, err
	}
	if int64(len(buffer)) > reader.remainingBytes {
		buffer = buffer[:reader.remainingBytes]
	}
	readCount, err := reader.source.Read(buffer)
	reader.remainingBytes -= int64(readCount)
	return readCount, err
}

// newUpstreamHTTPError keeps at most maxLoggedUpstreamBodyBytes of a non-200 body.
func newUpstreamHTTPError(operation string, resp *http.Response) error {
	bodyExcerpt, _ := io.ReadAll(io.LimitReader(resp.Body, maxLoggedUpstreamBodyBytes))
	return &upstreamHTTPError{operation: operation, statusCode: resp.StatusCode, bodyExcerpt: bodyExcerpt}
}

// LedgerEnd calls GET /v2/state/ledger-end. This pins ActiveContracts' activeAtOffset.
func (c *httpACSClient) LedgerEnd(ctx context.Context) (int64, error) {
	resp, err := c.sendUpstreamRequest(ctx, "ledger-end", http.MethodGet, "/v2/state/ledger-end", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, newUpstreamHTTPError("ledger-end", resp)
	}
	body, err := readAtMost(resp.Body, maxUpstreamResponseBytes)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: read ledger-end response: %w", err)
	}
	var out struct {
		Offset *int64 `json:"offset"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("disclosure-service: parse ledger-end response: %w", err)
	}
	if out.Offset == nil {
		return 0, errLedgerEndOffsetMissing
	}
	if *out.Offset < 1 {
		return 0, fmt.Errorf("%w: %d", errLedgerEndOffsetNotPositive, *out.Offset)
	}
	return *out.Offset, nil
}

// ActiveContracts reads query.template as the union of query.readerParties, at
// query.activeAtOffset, with includeCreatedEventBlob:true. It stream-decodes the response and
// keeps only the contracts whose createArgument passes every field match. A contract two readers
// both see is kept once. An empty result is a valid answer.
func (c *httpACSClient) ActiveContracts(ctx context.Context, query activeContractsQuery) ([]acsEntry, error) {
	if err := query.check(); err != nil {
		return nil, err
	}
	bodyBytes, err := marshalActiveContractsRequest(query, true)
	if err != nil {
		return nil, err
	}

	resp, err := c.sendUpstreamRequest(ctx, "active-contracts", http.MethodPost, "/v2/state/active-contracts", bodyBytes)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		return nil, fmt.Errorf("%w: template %s: %w", errUpstreamListLimitExceeded, query.template, newUpstreamHTTPError("active-contracts", resp))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newUpstreamHTTPError("active-contracts", resp)
	}
	entries, err := decodeBoundedActiveContracts(&boundedBodyReader{source: resp.Body, remainingBytes: c.maxResponseBytes}, query)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: active-contracts %s: %w", query.template, err)
	}
	return entries, nil
}

// marshalActiveContractsRequest builds the request body: one template filter per reader party.
func marshalActiveContractsRequest(query activeContractsQuery, includeCreatedEventBlob bool) ([]byte, error) {
	var reqBody activeContractsRequest
	reqBody.ActiveAtOffset = query.activeAtOffset
	var cumulativeFilter acsCumulativeFilter
	cumulativeFilter.IdentifierFilter.TemplateFilter.Value.TemplateID = query.template
	cumulativeFilter.IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob = includeCreatedEventBlob
	reqBody.EventFormat.FiltersByParty = make(map[string]acsPartyFilter, len(query.readerParties))
	for _, party := range query.readerParties {
		reqBody.EventFormat.FiltersByParty[string(party)] = acsPartyFilter{Cumulative: []acsCumulativeFilter{cumulativeFilter}}
	}
	// verbose:true so createArgument carries field labels; the field matches and selectors decode it.
	reqBody.EventFormat.Verbose = true
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: marshal active-contracts request: %w", err)
	}
	return bodyBytes, nil
}

// ActiveContractByID calls POST /v2/events/events-by-contract-id. found means a created event and
// no archived event. An upstream 404 means the readers cannot see the contract: found is false.
func (c *httpACSClient) ActiveContractByID(ctx context.Context, readerParties []partyID, wantContractID contractID) (acsEntry, bool, error) {
	if len(readerParties) == 0 {
		return acsEntry{}, false, errors.New("disclosure-service: events-by-contract-id: at least one reader party is required")
	}
	if wantContractID == "" {
		return acsEntry{}, false, errors.New("disclosure-service: events-by-contract-id: contract id is empty")
	}
	var reqBody eventsByContractIDRequest
	reqBody.ContractID = string(wantContractID)
	var wildcardFilter wildcardCumulativeFilter
	wildcardFilter.IdentifierFilter.WildcardFilter.Value.IncludeCreatedEventBlob = true
	reqBody.EventFormat.FiltersByParty = make(map[string]wildcardPartyFilter, len(readerParties))
	for _, party := range readerParties {
		reqBody.EventFormat.FiltersByParty[string(party)] = wildcardPartyFilter{Cumulative: []wildcardCumulativeFilter{wildcardFilter}}
	}
	reqBody.EventFormat.Verbose = true
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return acsEntry{}, false, fmt.Errorf("disclosure-service: marshal events-by-contract-id request: %w", err)
	}

	resp, err := c.sendUpstreamRequest(ctx, "events-by-contract-id", http.MethodPost, "/v2/events/events-by-contract-id", bodyBytes)
	if err != nil {
		return acsEntry{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return acsEntry{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return acsEntry{}, false, newUpstreamHTTPError("events-by-contract-id", resp)
	}
	body, err := io.ReadAll(&boundedBodyReader{source: resp.Body, remainingBytes: c.maxResponseBytes})
	if err != nil {
		return acsEntry{}, false, fmt.Errorf("disclosure-service: read events-by-contract-id response: %w", err)
	}
	var events eventsByContractIDResponse
	if err := json.Unmarshal(body, &events); err != nil {
		return acsEntry{}, false, fmt.Errorf("%w: events-by-contract-id: %v", errUpstreamResponseMalformed, err)
	}
	if events.Created == nil || events.Created.CreatedEvent == nil {
		return acsEntry{}, false, nil
	}
	createdEvent := events.Created.CreatedEvent
	if contractID(createdEvent.ContractID) != wantContractID {
		return acsEntry{}, false, fmt.Errorf("%w: events-by-contract-id returned contract %q for %q", errUpstreamResponseMalformed, createdEvent.ContractID, wantContractID)
	}
	if len(events.Archived) > 0 && !bytes.Equal(bytes.TrimSpace(events.Archived), []byte("null")) {
		return acsEntry{}, false, nil
	}
	return acsEntry{
		TemplateID:       createdEvent.TemplateID,
		PackageName:      createdEvent.PackageName,
		ContractID:       createdEvent.ContractID,
		CreatedEventBlob: createdEvent.CreatedEventBlob,
		SynchronizerID:   events.Created.SynchronizerID,
		CreateArgument:   createdEvent.CreateArgument,
		CreatedAt:        createdEvent.CreatedAt,
	}, true, nil
}

// decodeBoundedActiveContracts reports errUpstreamResponseTooLarge whenever body passed its limit.
// json.Decoder.More drops a read error, so the decode error alone can name another cause.
func decodeBoundedActiveContracts(body *boundedBodyReader, query activeContractsQuery) ([]acsEntry, error) {
	entries, err := decodeActiveContractsStream(body, query)
	if body.limitExceeded {
		return nil, errUpstreamResponseTooLarge
	}
	return entries, err
}

// decodeActiveContractsStream reads one JSON array element at a time. It fails past
// maxScannedContractsPerQuery elements or query.maxMatches kept contracts.
func decodeActiveContractsStream(body io.Reader, query activeContractsQuery) ([]acsEntry, error) {
	decoder := json.NewDecoder(body)
	if err := expectJSONDelimiter(decoder, '['); err != nil {
		return nil, err
	}
	matches := make([]acsEntry, 0, 1)
	seenContractIDs := make(map[string]bool)
	scannedCount := 0
	for decoder.More() {
		scannedCount++
		if scannedCount > maxScannedContractsPerQuery {
			return nil, fmt.Errorf("%w: more than %d", errUpstreamScanBoundExceeded, maxScannedContractsPerQuery)
		}
		var responseEntry acsResponseEntry
		if err := decoder.Decode(&responseEntry); err != nil {
			return nil, wrapStreamError(err)
		}
		activeContract := responseEntry.ContractEntry.JsActiveContract
		if activeContract == nil || activeContract.CreatedEvent == nil {
			continue // other contractEntry variant
		}
		createdEvent := activeContract.CreatedEvent
		if err := checkUpstreamTemplate(query.template, createdEvent.PackageName, createdEvent.TemplateID); err != nil {
			return nil, err
		}
		if seenContractIDs[createdEvent.ContractID] {
			continue // visible to more than one reader party
		}
		seenContractIDs[createdEvent.ContractID] = true
		if !query.matchesCreateArgument(createdEvent.CreateArgument) {
			continue
		}
		matches = append(matches, acsEntry{
			TemplateID:       createdEvent.TemplateID,
			PackageName:      createdEvent.PackageName,
			ContractID:       createdEvent.ContractID,
			CreatedEventBlob: createdEvent.CreatedEventBlob,
			SynchronizerID:   activeContract.SynchronizerID,
			CreateArgument:   createdEvent.CreateArgument,
			CreatedAt:        createdEvent.CreatedAt,
		})
		if len(matches) > query.maxMatches {
			return nil, fmt.Errorf("%w: more than %d for %s", errMatchCapExceeded, query.maxMatches, query.template)
		}
	}
	if err := expectJSONDelimiter(decoder, ']'); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if errors.Is(err, errUpstreamResponseTooLarge) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: data after the array", errUpstreamResponseMalformed)
	}
	if scannedCount > maxScannedContractsPerQuery || len(matches) > query.maxMatches {
		panic("disclosure-service: decodeActiveContractsStream exceeded its own bounds")
	}
	return matches, nil
}

// checkUpstreamTemplate requires the requested "#name:Module:Entity" to equal the returned
// package name and "Module:Entity" tail. The package name survives a package-id change; the
// Daml requireCanonicalFactory check uses the same rule.
func checkUpstreamTemplate(requestedTemplate, returnedPackageName, returnedTemplateID string) error {
	requestedParts := strings.Split(requestedTemplate, ":")
	if len(requestedParts) != 3 || !strings.HasPrefix(requestedParts[0], "#") {
		panic(fmt.Sprintf("disclosure-service: requested template %q is not \"#name:Module:Entity\"", requestedTemplate))
	}
	requestedPackageName := strings.TrimPrefix(requestedParts[0], "#")
	if returnedPackageName != requestedPackageName {
		return fmt.Errorf("%w: package %q, expected %q", errUpstreamTemplateMismatch, returnedPackageName, requestedPackageName)
	}
	if strings.Count(returnedTemplateID, ":") != 2 {
		return fmt.Errorf("%w: template id %q is not package-qualified", errUpstreamTemplateMismatch, returnedTemplateID)
	}
	if templateTail(returnedTemplateID) != templateTail(requestedTemplate) {
		return fmt.Errorf("%w: template %q, expected %q", errUpstreamTemplateMismatch, returnedTemplateID, requestedTemplate)
	}
	return nil
}

func expectJSONDelimiter(decoder *json.Decoder, wantDelimiter json.Delim) error {
	token, err := decoder.Token()
	if err != nil {
		return wrapStreamError(err)
	}
	if gotDelimiter, isDelimiter := token.(json.Delim); !isDelimiter || gotDelimiter != wantDelimiter {
		return fmt.Errorf("%w: expected %q, got %v", errUpstreamResponseMalformed, wantDelimiter, token)
	}
	return nil
}

// wrapStreamError keeps errUpstreamResponseTooLarge and reports every other decode error as malformed.
func wrapStreamError(err error) error {
	if errors.Is(err, errUpstreamResponseTooLarge) {
		return err
	}
	return fmt.Errorf("%w: %v", errUpstreamResponseMalformed, err)
}

// ---------------------------------------------------------------------------
// Bearer token sources
// ---------------------------------------------------------------------------

// bearerTokenSource yields the bearer token for one upstream request.
type bearerTokenSource interface {
	BearerToken(ctx context.Context) (string, error)
	// DiscardRejectedBearerToken drops rejectedToken after an upstream 401, if it is still cached.
	DiscardRejectedBearerToken(rejectedToken string)
	// RecordRejectedRetryBearerToken counts an upstream 401 on the retry as a token source failure.
	RecordRejectedRetryBearerToken(rejectedToken string)
	// RecordAcceptedBearerToken notes an upstream answer other than 401 for acceptedToken.
	RecordAcceptedBearerToken(acceptedToken string)
}

// fileBearerTokenSource re-reads the token file on every call, so a rotated file takes effect on
// the next request.
type fileBearerTokenSource struct {
	path string
}

func (source fileBearerTokenSource) BearerToken(context.Context) (string, error) {
	return readAccessToken(source.path)
}

// DiscardRejectedBearerToken is a no-op: BearerToken reads the file again for each request.
func (fileBearerTokenSource) DiscardRejectedBearerToken(string) {}

// RecordRejectedRetryBearerToken is a no-op: the file source sends no token endpoint request.
func (fileBearerTokenSource) RecordRejectedRetryBearerToken(string) {}

func (fileBearerTokenSource) RecordAcceptedBearerToken(string) {}

// readAccessToken reads and trims the bearer token file. It is separate from run() so the
// trim-whitespace behavior is independently testable. A file saved with a trailing newline is
// the common case. A file that trims to nothing is an error: it would send an empty bearer
// token upstream.
func readAccessToken(path string) (string, error) {
	raw, err := readSecretFileAtMost(path, maxAccessTokenFileBytes)
	if err != nil {
		return "", fmt.Errorf("disclosure-service: read access token file %s: %w", path, err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("disclosure-service: access token file %s is empty", path)
	}
	return token, nil
}
