package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"time"
)

const (
	registryParamPageSize  = "pageSize"
	registryParamPageToken = "pageToken"
)

func (s *server) mountRegistryRoutes() {
	s.mux.HandleFunc("GET /registry/metadata/v1/info", s.handleRegistryInfo)
	s.mux.HandleFunc("GET /registry/metadata/v1/instruments", s.handleListInstruments)
	s.mux.HandleFunc("GET /registry/metadata/v1/instruments/{instrumentId}", s.handleGetInstrument)
	s.mux.HandleFunc("POST /registry/transfer-instruction/v1/transfer-factory", s.handleTransferFactory)
	s.mux.HandleFunc("POST /registry/transfer-instruction/v1/{transferInstructionId}/choice-contexts/{choice}", s.handleTransferInstructionChoiceContext)
	s.mux.HandleFunc("POST /registry/allocation-instruction/v1/allocation-factory", s.handleAllocationFactory)
	s.mux.HandleFunc("POST /registry/allocations/v1/{allocationId}/choice-contexts/{choice}", s.handleAllocationChoiceContext)
	s.mux.HandleFunc("/registry/", s.handleRegistryUnknownPath)
}

// ---------------------------------------------------------------------------
// Error replies
// ---------------------------------------------------------------------------

// writeRegistryError writes the spec's ErrorResponse with a new request id in X-Request-Id. A 4xx
// flowError keeps its own message. Every other error is a fixed 5xx text that names the request
// id; the detail goes to the log only.
//
// SECURITY: err can hold upstream bodies, URLs, IP addresses, and file paths. Only the log sees a 5xx err.
func (s *server) writeRegistryError(w http.ResponseWriter, r *http.Request, err error) {
	requestID := newRequestID()
	w.Header().Set("X-Request-Id", requestID)
	var flowErr *flowError
	if errors.As(err, &flowErr) && flowErr.status >= 400 && flowErr.status <= 499 {
		writeJSON(w, flowErr.status, registryErrorResponse{Error: flowErr.msg})
		return
	}
	status := http.StatusInternalServerError
	if flowErr != nil && flowErr.status >= 500 && flowErr.status <= 599 {
		status = flowErr.status
	}
	s.logger.Printf("disclosure-service: request %s %q: status %d: %q", requestID, r.URL.Path, status, err.Error())
	writeJSON(w, status, registryErrorResponse{Error: registryPublicErrorText(status, err, requestID)})
}

// registryPublicErrorText is the fixed client text of a 5xx registry error.
func registryPublicErrorText(status int, err error, requestID string) string {
	if status < 500 || status > 599 {
		panic(fmt.Sprintf("disclosure-service: registryPublicErrorText needs a 5xx status, got %d", status))
	}
	return fmt.Sprintf("disclosure-service: %s; request id %s", publicMessageFor(status, err), requestID)
}

func (s *server) handleRegistryUnknownPath(w http.ResponseWriter, r *http.Request) {
	s.writeRegistryError(w, r, newFlowError(http.StatusNotFound, "no registry endpoint %q %q", truncateForError(r.Method), truncateForError(r.URL.Path)))
}

// ---------------------------------------------------------------------------
// Request checks (pure; each runs before any upstream call)
// ---------------------------------------------------------------------------

// checkRegistryQuery returns each parameter's single value. An oversize query is a 414; a
// malformed, repeated, or unknown parameter is a 400.
func checkRegistryQuery(rawQuery string, allowedNames ...string) (map[string]string, error) {
	if len(rawQuery) > maxFlowQueryBytes {
		return nil, newFlowError(http.StatusRequestURITooLong, "query is %d bytes, the limit is %d", len(rawQuery), maxFlowQueryBytes)
	}
	parsedQuery, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, newFlowError(http.StatusBadRequest, "malformed query")
	}
	singleValues := make(map[string]string, len(parsedQuery))
	for name, values := range parsedQuery {
		if !slices.Contains(allowedNames, name) {
			return nil, newFlowError(http.StatusBadRequest, "unknown parameter %q", truncateForError(name))
		}
		if len(values) != 1 {
			return nil, newFlowError(http.StatusBadRequest, "parameter %s given %d times", name, len(values))
		}
		singleValues[name] = values[0]
	}
	if len(singleValues) > len(allowedNames) {
		panic("disclosure-service: checkRegistryQuery kept more parameters than the route allows")
	}
	return singleValues, nil
}

// listInstrumentsRequest is the parsed query of GET /registry/metadata/v1/instruments.
type listInstrumentsRequest struct {
	pageSize          int    // 1 to maxInstrumentPageSize
	afterInstrumentID string // the decoded pageToken; "" for the first page
}

// encodeInstrumentPageToken is the pageToken of the page after instrumentID: unpadded base64url,
// so the token needs no percent-encoding in a query.
func encodeInstrumentPageToken(instrumentID string) string {
	if instrumentID == "" {
		panic("disclosure-service: page token of an empty instrument id")
	}
	pageToken := base64.RawURLEncoding.EncodeToString([]byte(instrumentID))
	if len(pageToken) > maxEncodedPageTokenLength {
		panic("disclosure-service: page token exceeds maxEncodedPageTokenLength")
	}
	return pageToken
}

// parseInstrumentPageToken decodes a pageToken into an instrument id of 1 to
// maxRegistryIdentifierLength bytes.
func parseInstrumentPageToken(pageToken string) (string, error) {
	if pageToken == "" {
		return "", newFlowError(http.StatusBadRequest, "pageToken must not be empty")
	}
	if len(pageToken) > maxEncodedPageTokenLength {
		return "", newFlowError(http.StatusBadRequest, "pageToken is longer than %d bytes", maxEncodedPageTokenLength)
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(pageToken)
	if err != nil {
		return "", newFlowError(http.StatusBadRequest, "pageToken is not a nextPageToken of this registry")
	}
	if err := checkRegistryIdentifier(registryParamPageToken, string(decoded)); err != nil {
		return "", err
	}
	return string(decoded), nil
}

// parseListInstrumentsRequest reads pageSize and pageToken. pageSize is an integer; a value
// outside 1 to maxInstrumentPageSize moves to the nearest limit.
func parseListInstrumentsRequest(rawQuery string) (listInstrumentsRequest, error) {
	params, err := checkRegistryQuery(rawQuery, registryParamPageSize, registryParamPageToken)
	if err != nil {
		return listInstrumentsRequest{}, err
	}
	request := listInstrumentsRequest{pageSize: defaultInstrumentPageSize}
	if rawPageSize, present := params[registryParamPageSize]; present {
		requestedPageSize, err := strconv.Atoi(rawPageSize)
		if err != nil {
			return listInstrumentsRequest{}, newFlowError(http.StatusBadRequest, "pageSize must be an integer, got %q", truncateForError(rawPageSize))
		}
		request.pageSize = min(max(requestedPageSize, 1), maxInstrumentPageSize)
	}
	if pageToken, present := params[registryParamPageToken]; present {
		afterInstrumentID, err := parseInstrumentPageToken(pageToken)
		if err != nil {
			return listInstrumentsRequest{}, err
		}
		request.afterInstrumentID = afterInstrumentID
	}
	if request.pageSize < 1 || request.pageSize > maxInstrumentPageSize {
		panic("disclosure-service: pageSize left its bounds")
	}
	return request, nil
}

// describeRequestBodyFault is a 400 with a fixed text for a body decode fault. A json error can
// quote the full client value, so only a bounded field name is kept.
func describeRequestBodyFault(where string, err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return newFlowError(http.StatusBadRequest, "%s is larger than %d bytes", where, maxRegistryRequestBodyBytes)
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return newFlowError(http.StatusBadRequest, "%s: field %q has the wrong type", where, truncateForError(typeErr.Field))
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return newFlowError(http.StatusBadRequest, "%s is not valid JSON at byte %d", where, syntaxErr.Offset)
	}
	return newFlowError(http.StatusBadRequest, "%s does not match the request schema", where)
}

// decodeRegistryRequestObject reads a bounded JSON object body into target. Any read, syntax,
// type or trailing-data fault is a 400.
func decodeRegistryRequestObject(w http.ResponseWriter, r *http.Request, target any) error {
	body := http.MaxBytesReader(w, r.Body, maxRegistryRequestBodyBytes)
	decoder := json.NewDecoder(body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return describeRequestBodyFault("request body", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return newFlowError(http.StatusBadRequest, "request body: trailing data after the JSON object")
	}
	if !isJSONObject(raw) {
		return newFlowError(http.StatusBadRequest, "request body must be a JSON object")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return describeRequestBodyFault("request body", err)
	}
	return nil
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func checkRegistryIdentifier(field, value string) error {
	if value == "" {
		return newFlowError(http.StatusBadRequest, "%s is required", field)
	}
	if len(value) > maxRegistryIdentifierLength {
		return newFlowError(http.StatusBadRequest, "%s is longer than %d bytes", field, maxRegistryIdentifierLength)
	}
	return nil
}

// parseContractIDText enforces the Canton contract id text form: lowercase hex.
func parseContractIDText(field, rawContractID string) (contractID, error) {
	if err := checkRegistryIdentifier(field, rawContractID); err != nil {
		return "", err
	}
	for _, character := range rawContractID {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return "", newFlowError(http.StatusBadRequest, "%s %q is not a hex contract id", field, truncateForError(rawContractID))
		}
	}
	return contractID(rawContractID), nil
}

func checkPositiveAmount(field string, amount json.Number) error {
	scaled, err := parseDamlDecimalScaled(amount.String())
	if err != nil {
		return newFlowError(http.StatusBadRequest, "%s is not a Daml Decimal", field)
	}
	if scaled.Sign() <= 0 {
		return newFlowError(http.StatusBadRequest, "%s must be positive", field)
	}
	return nil
}

// checkInstrumentOfRegistry mirrors the factory choice bodies' admin asserts: expectedAdmin must
// equal the instrument admin (400), and the instrument admin must be this registry's admin (404).
func checkInstrumentOfRegistry(expectedAdmin partyID, instrument daInstrumentId, registryAdmin partyID) error {
	if expectedAdmin != instrument.Admin {
		return newFlowError(http.StatusBadRequest, "expectedAdmin %q does not match instrumentId.admin %q", truncateForError(string(expectedAdmin)), truncateForError(string(instrument.Admin)))
	}
	if instrument.Admin != registryAdmin {
		return newFlowError(http.StatusNotFound, "instrument admin %q is not this registry's admin", truncateForError(string(instrument.Admin)))
	}
	return nil
}

// parseFactoryRequest reads a GetFactoryRequest and decodes its choiceArguments into arguments.
func parseFactoryRequest(w http.ResponseWriter, r *http.Request, arguments any) (getFactoryRequest, error) {
	var request getFactoryRequest
	if err := decodeRegistryRequestObject(w, r, &request); err != nil {
		return getFactoryRequest{}, err
	}
	if !isJSONObject(request.ChoiceArguments) {
		return getFactoryRequest{}, newFlowError(http.StatusBadRequest, "choiceArguments must be a JSON object")
	}
	if err := json.Unmarshal(request.ChoiceArguments, arguments); err != nil {
		return getFactoryRequest{}, describeRequestBodyFault("choiceArguments", err)
	}
	return request, nil
}

// requestedTransfer is the validated part of a TransferFactory_Transfer request.
type requestedTransfer struct {
	expectedAdmin partyID
	sender        partyID
	receiver      partyID
	instrument    daInstrumentId
}

func checkTransferFactoryChoiceArguments(arguments transferFactoryChoiceArguments) (requestedTransfer, error) {
	expectedAdmin, err := parseRequestPartyID(arguments.ExpectedAdmin, "choiceArguments.expectedAdmin")
	if err != nil {
		return requestedTransfer{}, err
	}
	transfer := arguments.Transfer
	if transfer == nil {
		return requestedTransfer{}, newFlowError(http.StatusBadRequest, "choiceArguments.transfer is required")
	}
	instrument, err := parseRequestInstrumentID("choiceArguments.transfer.instrumentId", transfer.InstrumentID)
	if err != nil {
		return requestedTransfer{}, err
	}
	sender, err := parseRequestPartyID(transfer.Sender, "choiceArguments.transfer.sender")
	if err != nil {
		return requestedTransfer{}, err
	}
	receiver, err := parseRequestPartyID(transfer.Receiver, "choiceArguments.transfer.receiver")
	if err != nil {
		return requestedTransfer{}, err
	}
	if err := checkPositiveAmount("choiceArguments.transfer.amount", transfer.Amount); err != nil {
		return requestedTransfer{}, err
	}
	return requestedTransfer{
		expectedAdmin: expectedAdmin,
		sender:        sender,
		receiver:      receiver,
		instrument:    instrument,
	}, nil
}

// checkAllocationFactoryChoiceArguments returns expectedAdmin and the leg's instrument.
func checkAllocationFactoryChoiceArguments(arguments allocationFactoryChoiceArguments) (partyID, daInstrumentId, error) {
	expectedAdmin, err := parseRequestPartyID(arguments.ExpectedAdmin, "choiceArguments.expectedAdmin")
	if err != nil {
		return "", daInstrumentId{}, err
	}
	if arguments.Allocation == nil {
		return "", daInstrumentId{}, newFlowError(http.StatusBadRequest, "choiceArguments.allocation is required")
	}
	if arguments.Allocation.TransferLeg == nil {
		return "", daInstrumentId{}, newFlowError(http.StatusBadRequest, "choiceArguments.allocation.transferLeg is required")
	}
	leg := arguments.Allocation.TransferLeg
	instrument, err := parseRequestInstrumentID("choiceArguments.allocation.transferLeg.instrumentId", leg.InstrumentID)
	if err != nil {
		return "", daInstrumentId{}, err
	}
	if _, err := parseRequestPartyID(leg.Sender, "choiceArguments.allocation.transferLeg.sender"); err != nil {
		return "", daInstrumentId{}, err
	}
	if _, err := parseRequestPartyID(leg.Receiver, "choiceArguments.allocation.transferLeg.receiver"); err != nil {
		return "", daInstrumentId{}, err
	}
	if err := checkPositiveAmount("choiceArguments.allocation.transferLeg.amount", leg.Amount); err != nil {
		return "", daInstrumentId{}, err
	}
	return expectedAdmin, instrument, nil
}

// parseRequestInstrumentID parses the admin as a party id and bounds the id. The admin must then
// equal expectedAdmin (checkInstrumentOfRegistry).
func parseRequestInstrumentID(fieldPrefix string, rawInstrument *requestInstrumentID) (daInstrumentId, error) {
	if rawInstrument == nil {
		return daInstrumentId{}, newFlowError(http.StatusBadRequest, "%s is required", fieldPrefix)
	}
	admin, err := parseRequestPartyID(rawInstrument.Admin, fieldPrefix+".admin")
	if err != nil {
		return daInstrumentId{}, err
	}
	if err := checkRegistryIdentifier(fieldPrefix+".id", rawInstrument.Id); err != nil {
		return daInstrumentId{}, err
	}
	return daInstrumentId{Admin: admin, Id: rawInstrument.Id}, nil
}

// beginRegistryRequest runs the checks of every registry route: the registry API is on, and the
// query holds only allowedQueryNames. It writes the error reply when a check fails.
func (s *server) beginRegistryRequest(w http.ResponseWriter, r *http.Request, allowedQueryNames ...string) (*registryConfig, bool) {
	if s.registry == nil {
		s.writeRegistryError(w, r, newFlowError(http.StatusNotFound, "the registry API is off: start the service with --registry-instruments-file"))
		return nil, false
	}
	if _, err := checkRegistryQuery(r.URL.RawQuery, allowedQueryNames...); err != nil {
		s.writeRegistryError(w, r, err)
		return nil, false
	}
	return s.registry, true
}

func (s *server) registryLedgerEnd(ctx context.Context) (int64, error) {
	offset, err := s.acs.LedgerEnd(ctx)
	if err != nil {
		return 0, newFlowError(upstreamErrorStatus(err), "ledger-end: %v", err)
	}
	return offset, nil
}

// ---------------------------------------------------------------------------
// Response postconditions and writers
// ---------------------------------------------------------------------------

// countChoiceContextBytes sums the variable-size bytes of each disclosed contract.
func countChoiceContextBytes(choiceContext registryChoiceContext) int {
	totalBytes := 0
	for _, disclosed := range choiceContext.DisclosedContracts {
		totalBytes += len(disclosed.CreatedEventBlob) + len(disclosed.ContractID) + len(disclosed.TemplateID) + len(disclosed.SynchronizerID)
		totalBytes += len(disclosed.DebugPackageName) + len(disclosed.DebugPayload) + len(disclosed.DebugCreatedAt)
	}
	for key, contextValue := range choiceContext.ChoiceContextData.Values {
		totalBytes += len(key) + len(contextValue.Tag) + len(contextValue.Value)
	}
	return totalBytes
}

// checkRegistryChoiceContext is the postcondition of each served choice context: complete
// disclosures, each contract once, one synchronizer (500), and maxFlowResponseBytes (502).
func checkRegistryChoiceContext(choiceContext registryChoiceContext) error {
	if choiceContext.ChoiceContextData.Values == nil {
		return newFlowError(http.StatusInternalServerError, "choice context values map is nil")
	}
	if choiceContext.DisclosedContracts == nil {
		return newFlowError(http.StatusInternalServerError, "choice context disclosedContracts is nil")
	}
	for key, contextValue := range choiceContext.ChoiceContextData.Values {
		if contextValue.Tag != anyValueContractIDTag {
			return newFlowError(http.StatusInternalServerError, "choice context value %s has tag %q", key, contextValue.Tag)
		}
		if contextValue.Value == "" {
			return newFlowError(http.StatusInternalServerError, "choice context value %s is empty", key)
		}
	}
	seenContractIDs := make(map[string]bool, len(choiceContext.DisclosedContracts))
	for _, disclosed := range choiceContext.DisclosedContracts {
		if disclosed.ContractID == "" {
			return newFlowError(http.StatusInternalServerError, "disclosed contract has no contract id")
		}
		if disclosed.TemplateID == "" {
			return newFlowError(http.StatusInternalServerError, "disclosed contract %s has no template id", disclosed.ContractID)
		}
		if disclosed.CreatedEventBlob == "" {
			return newFlowError(http.StatusInternalServerError, "disclosed contract %s has no createdEventBlob", disclosed.ContractID)
		}
		if disclosed.SynchronizerID == "" {
			return newFlowError(http.StatusInternalServerError, "disclosed contract %s has no synchronizer", disclosed.ContractID)
		}
		if disclosed.SynchronizerID != choiceContext.DisclosedContracts[0].SynchronizerID {
			return newFlowError(http.StatusInternalServerError, "choice context spans synchronizers %s and %s", choiceContext.DisclosedContracts[0].SynchronizerID, disclosed.SynchronizerID)
		}
		if seenContractIDs[disclosed.ContractID] {
			return newFlowError(http.StatusInternalServerError, "choice context holds contract %s twice", disclosed.ContractID)
		}
		seenContractIDs[disclosed.ContractID] = true
	}
	if totalBytes := countChoiceContextBytes(choiceContext); totalBytes > maxFlowResponseBytes {
		return newFlowError(http.StatusBadGateway, "choice context of %d bytes exceeds %d", totalBytes, maxFlowResponseBytes)
	}
	return nil
}

// writeRegistryChoiceContext checks choiceContext before the status line, then encodes response.
func (s *server) writeRegistryChoiceContext(w http.ResponseWriter, r *http.Request, choiceContext registryChoiceContext, response any) {
	if err := checkRegistryChoiceContext(choiceContext); err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// writeInstrumentsPage encodes one instrument at a time into a ListInstrumentsResponse.
func writeInstrumentsPage(w http.ResponseWriter, instruments []registryInstrumentResponse, nextPageToken string) {
	if len(instruments) > maxInstrumentPageSize {
		panic("disclosure-service: instruments page exceeds maxInstrumentPageSize")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, `{"instruments":[`); err != nil {
		return
	}
	encoder := json.NewEncoder(w)
	for index, instrument := range instruments {
		if index > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return
			}
		}
		if err := encoder.Encode(instrument); err != nil {
			return
		}
	}
	if _, err := io.WriteString(w, "]"); err != nil {
		return
	}
	if nextPageToken != "" {
		if _, err := io.WriteString(w, `,"nextPageToken":`); err != nil {
			return
		}
		if err := encoder.Encode(nextPageToken); err != nil {
			return
		}
	}
	_, _ = io.WriteString(w, "}\n")
}

// ---------------------------------------------------------------------------
// token-metadata-v1
// ---------------------------------------------------------------------------

func (s *server) handleRegistryInfo(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.beginRegistryRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, registryInfoResponse{AdminID: string(registry.adminParty), SupportedApis: registrySupportedApis()})
}

// instrumentMetadata renders one instrument. A missing supply total is zero. Unavailable totals
// leave out totalSupply and totalSupplyAsOf.
func instrumentMetadata(instrument registryInstrument, display registryInstrumentDisplay, supplies registrySupplyTotals) registryInstrumentResponse {
	metadata := registryInstrumentResponse{
		ID:            instrument.id,
		Name:          display.name,
		Symbol:        display.symbol,
		Decimals:      instrument.decimals,
		SupportedApis: registrySupportedApis(),
	}
	if !supplies.available {
		return metadata
	}
	supply, minted := supplies.read.totalsByGroup[instrument.id]
	if !minted {
		supply = new(big.Int)
	}
	metadata.TotalSupply = formatDamlDecimalScaled(supply)
	metadata.TotalSupplyAsOf = supplies.read.readAt.UTC().Format(time.RFC3339Nano)
	if metadata.TotalSupply == "" {
		panic("disclosure-service: available totals rendered an empty totalSupply")
	}
	if metadata.TotalSupplyAsOf == "" {
		panic("disclosure-service: available totals rendered an empty totalSupplyAsOf")
	}
	return metadata
}

// listedInstruments returns the registry instruments that have display config, in id order.
func (s *server) listedInstruments(ctx context.Context, registry *registryConfig) ([]registryInstrumentResponse, error) {
	offset, err := s.registryLedgerEnd(ctx)
	if err != nil {
		return nil, err
	}
	instruments, err := s.fetchRegistryInstruments(ctx, registry, offset)
	if err != nil {
		return nil, err
	}
	supplies, err := s.fetchTotalSuppliesWithinBounds(ctx, registry, offset)
	if err != nil {
		return nil, err
	}
	listed := make([]registryInstrumentResponse, 0, len(instruments))
	for _, instrument := range instruments {
		display, configured := registry.displayByInstrumentID[instrument.id]
		if !configured {
			registry.logUnconfiguredOnce(s.logger, instrument.id)
			continue
		}
		listed = append(listed, instrumentMetadata(instrument, display, supplies))
	}
	return listed, nil
}

// handleListInstruments pages by id. pageToken encodes the last id of the previous page; the next
// page starts at the first id greater than it. Ids stay in order when the set changes.
func (s *server) handleListInstruments(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.beginRegistryRequest(w, r, registryParamPageSize, registryParamPageToken)
	if !ok {
		return
	}
	request, err := parseListInstrumentsRequest(r.URL.RawQuery)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	listed, err := s.listedInstruments(r.Context(), registry)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	start := 0
	if request.afterInstrumentID != "" {
		start = sort.Search(len(listed), func(index int) bool { return listed[index].ID > request.afterInstrumentID })
	}
	end := min(start+request.pageSize, len(listed))
	nextPageToken := ""
	if end < len(listed) {
		nextPageToken = encodeInstrumentPageToken(listed[end-1].ID)
	}
	writeInstrumentsPage(w, listed[start:end], nextPageToken)
}

func (s *server) handleGetInstrument(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.beginRegistryRequest(w, r)
	if !ok {
		return
	}
	instrumentID := r.PathValue("instrumentId")
	if err := checkRegistryIdentifier("instrumentId", instrumentID); err != nil {
		s.writeRegistryError(w, r, newFlowError(http.StatusNotFound, "no instrument %q in this registry", truncateForError(instrumentID)))
		return
	}
	display, configured := registry.displayByInstrumentID[instrumentID]
	if !configured {
		s.writeRegistryError(w, r, newFlowError(http.StatusNotFound, "no instrument %q in this registry", truncateForError(instrumentID)))
		return
	}
	offset, err := s.registryLedgerEnd(r.Context())
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	instrument, err := s.findRegistryInstrument(r.Context(), registry, instrumentID, offset)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	supplies, err := s.fetchTotalSuppliesWithinBounds(r.Context(), registry, offset)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, instrumentMetadata(instrument, display, supplies))
}

// ---------------------------------------------------------------------------
// transfer-instruction-v1
// ---------------------------------------------------------------------------

func (s *server) handleTransferFactory(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.beginRegistryRequest(w, r)
	if !ok {
		return
	}
	var arguments transferFactoryChoiceArguments
	request, err := parseFactoryRequest(w, r, &arguments)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	transfer, err := checkTransferFactoryChoiceArguments(arguments)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	response, err := s.assembleTransferFactory(r.Context(), registry, transfer, request.ExcludeDebugFields)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	s.writeRegistryChoiceContext(w, r, response.ChoiceContext, response)
}

// assembleTransferFactory mirrors TransferFactory_Transfer (CoinFactory.daml:56-77) and
// transferCore (CoinTransfer.daml:151-168).
//
// SECURITY: offer is the fallback when no usable preapproval is known. With no preapproval value,
// lookupTransferPreapprovalCid gives None (CoinTransfer.daml:20-24), and the factory creates a
// CoinTransferInstruction for the receiver (CoinTransfer.daml:168, CoinFactory.daml:69-77).
func (s *server) assembleTransferFactory(ctx context.Context, registry *registryConfig, transfer requestedTransfer, excludeDebugFields bool) (*transferFactoryWithChoiceContext, error) {
	if err := checkInstrumentOfRegistry(transfer.expectedAdmin, transfer.instrument, registry.adminParty); err != nil {
		return nil, err
	}
	offset, err := s.registryLedgerEnd(ctx)
	if err != nil {
		return nil, err
	}
	instrument, err := s.findRegistryInstrument(ctx, registry, transfer.instrument.Id, offset)
	if err != nil {
		return nil, err
	}
	factory, err := s.fetchCanonicalFactory(ctx, registry, instrument)
	if err != nil {
		return nil, err
	}
	response := &transferFactoryWithChoiceContext{FactoryID: factory.entry.ContractID, ChoiceContext: emptyChoiceContext()}
	response.ChoiceContext.DisclosedContracts = append(response.ChoiceContext.DisclosedContracts, registryDisclosure(factory.entry, excludeDebugFields))

	// transferCore completes a self-transfer on the receiver authority of the sender.
	if transfer.sender == transfer.receiver {
		response.TransferKind = transferKindSelf
		return response, nil
	}
	preapproval, found, err := s.fetchReceiverPreapproval(ctx, registry, transfer.receiver, transfer.instrument, offset)
	if err != nil {
		if !isPreapprovalReadBoundError(err) {
			return nil, err
		}
		s.logger.Printf("disclosure-service: registry: transferKind offer: the TransferPreapproval read stopped at a bound: %q", err.Error())
		response.TransferKind = transferKindOffer
		return response, nil
	}
	if !found {
		response.TransferKind = transferKindOffer
		return response, nil
	}
	if preapproval.entry.SynchronizerID != factory.entry.SynchronizerID {
		s.logger.Printf("disclosure-service: registry: transferKind offer: TransferPreapproval %s is on synchronizer %q, the CoinFactory on %q", preapproval.entry.ContractID, preapproval.entry.SynchronizerID, factory.entry.SynchronizerID)
		response.TransferKind = transferKindOffer
		return response, nil
	}
	response.TransferKind = transferKindDirect
	response.ChoiceContext.ChoiceContextData.Values[transferPreapprovalContextKey] = registryAnyValue{Tag: anyValueContractIDTag, Value: preapproval.entry.ContractID}
	response.ChoiceContext.DisclosedContracts = append(response.ChoiceContext.DisclosedContracts, registryDisclosure(preapproval.entry, excludeDebugFields))
	return response, nil
}

// handleTransferInstructionChoiceContext serves accept, reject and withdraw for a
// CoinTransferInstruction. The choice bodies (CoinTransfer.daml:77-88) only create Coins, and
// each controller is a stakeholder: the context is empty.
func (s *server) handleTransferInstructionChoiceContext(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.beginRegistryRequest(w, r)
	if !ok {
		return
	}
	choice := r.PathValue("choice")
	if choice != "accept" && choice != "reject" && choice != "withdraw" {
		s.writeRegistryError(w, r, newFlowError(http.StatusNotFound, "unknown transfer instruction choice %q", truncateForError(choice)))
		return
	}
	instructionID, err := parseContractIDText("transferInstructionId", r.PathValue("transferInstructionId"))
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	var request getChoiceContextRequest
	if err := decodeRegistryRequestObject(w, r, &request); err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	instruction, err := fetchRegistryContractByID[daCoinTransferInstruction](r.Context(), s, registry, tailCoinTransferInstruction, instructionID)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	if instruction.value.InstrumentId.Admin != registry.adminParty {
		s.logger.Printf("disclosure-service: registry: 404 for CoinTransferInstruction %q: instrument admin %q", string(instructionID), string(instruction.value.InstrumentId.Admin))
		s.writeRegistryError(w, r, registryContractNotFound(tailCoinTransferInstruction))
		return
	}
	choiceContext := emptyChoiceContext()
	s.writeRegistryChoiceContext(w, r, choiceContext, choiceContext)
}

// ---------------------------------------------------------------------------
// allocation-instruction-v1 and allocation-v1
// ---------------------------------------------------------------------------

// handleAllocationFactory mirrors AllocationFactory_Allocate (CoinFactory.daml:87-104): the
// disclosure set is the CoinFactory, and the context data is empty.
func (s *server) handleAllocationFactory(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.beginRegistryRequest(w, r)
	if !ok {
		return
	}
	var arguments allocationFactoryChoiceArguments
	request, err := parseFactoryRequest(w, r, &arguments)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	expectedAdmin, requestedInstrument, err := checkAllocationFactoryChoiceArguments(arguments)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	if err := checkInstrumentOfRegistry(expectedAdmin, requestedInstrument, registry.adminParty); err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	offset, err := s.registryLedgerEnd(r.Context())
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	instrument, err := s.findRegistryInstrument(r.Context(), registry, requestedInstrument.Id, offset)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	factory, err := s.fetchCanonicalFactory(r.Context(), registry, instrument)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	response := allocationFactoryWithChoiceContext{FactoryID: factory.entry.ContractID, ChoiceContext: emptyChoiceContext()}
	response.ChoiceContext.DisclosedContracts = append(response.ChoiceContext.DisclosedContracts, registryDisclosure(factory.entry, request.ExcludeDebugFields))
	s.writeRegistryChoiceContext(w, r, response.ChoiceContext, response)
}

// handleAllocationChoiceContext serves execute-transfer, withdraw and cancel for a CoinAllocation.
// The choice bodies (CoinAllocation.daml:22-36) only create Coins, and each controller is a
// stakeholder: the context is empty.
func (s *server) handleAllocationChoiceContext(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.beginRegistryRequest(w, r)
	if !ok {
		return
	}
	choice := r.PathValue("choice")
	if choice != "execute-transfer" && choice != "withdraw" && choice != "cancel" {
		s.writeRegistryError(w, r, newFlowError(http.StatusNotFound, "unknown allocation choice %q", truncateForError(choice)))
		return
	}
	allocationID, err := parseContractIDText("allocationId", r.PathValue("allocationId"))
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	var request getChoiceContextRequest
	if err := decodeRegistryRequestObject(w, r, &request); err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	allocation, err := fetchRegistryContractByID[daCoinAllocation](r.Context(), s, registry, tailCoinAllocation, allocationID)
	if err != nil {
		s.writeRegistryError(w, r, err)
		return
	}
	if allocation.value.Allocation.TransferLeg.InstrumentId.Admin != registry.adminParty {
		s.logger.Printf("disclosure-service: registry: 404 for CoinAllocation %q: instrument admin %q", string(allocationID), string(allocation.value.Allocation.TransferLeg.InstrumentId.Admin))
		s.writeRegistryError(w, r, registryContractNotFound(tailCoinAllocation))
		return
	}
	choiceContext := emptyChoiceContext()
	s.writeRegistryChoiceContext(w, r, choiceContext, choiceContext)
}
