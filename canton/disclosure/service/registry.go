package main

// CIP-0056 off-ledger registry API, v1, for the burn/mint instruments the registry admin (gg)
// administers. Response shapes follow openapi/*.yaml. Each choice context comes from its choice
// body in the token-cip0056 package (CantonExamples tokens/cip0056/daml/Token/CIP0056):
//
//   - TransferFactory_Transfer (CoinFactory.daml:56-77, CoinTransfer.daml:152-169): the sender
//     cannot see the CoinFactory (signatory admin only). transferCore exercises the receiver's
//     TransferPreapproval (signatory owner, observer admin) when extraArgs.context carries its id
//     under transferPreapprovalContextKey (CoinTransfer.daml:17-24, 164-168).
//   - AllocationFactory_Allocate (CoinFactory.daml:87-109): the sender cannot see the CoinFactory.
//   - TransferInstruction accept/reject/withdraw (CoinTransfer.daml:78-89) and Allocation
//     execute-transfer/cancel/withdraw (CoinAllocation.daml:24-38) only create Coins; their
//     controllers are stakeholders of the instruction or allocation. The context is empty.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	tailCoinTransferInstruction = "Token.CIP0056.CoinTransfer:CoinTransferInstruction"
	tailCoinAllocation          = "Token.CIP0056.CoinAllocation:CoinAllocation"
)

// transferPreapprovalContextKey is CoinTransfer.daml transferPreapprovalContextKey. The value is
// an AnyValue AV_ContractId (lookupTransferPreapprovalCid).
const (
	transferPreapprovalContextKey = "token-cip0056.transferPreapprovalCid"
	anyValueContractIDTag         = "AV_ContractId"
)

const (
	transferKindSelf   = "self"
	transferKindDirect = "direct"
	transferKindOffer  = "offer"
)

const (
	defaultInstrumentPageSize   = 25
	maxInstrumentPageSize       = 100
	maxRegistryRequestBodyBytes = 64 << 10
	maxInstrumentsFileBytes     = 1 << 20
	maxRegistryInstruments      = 1000
	maxRegistryIdentifierLength = 512 // party, instrument id, contract id
	maxDisplayTextLength        = 256
	maxTokenDecimals            = 10 // Daml Decimal scale
	damlDecimalScale            = 10
	damlDecimalMaxDigits        = 38
)

// registrySupportedApis are the four v1 off-ledger APIs this service implements, at minor version 1.
func registrySupportedApis() map[string]int {
	return map[string]int{
		"splice-api-token-metadata-v1":               1,
		"splice-api-token-transfer-instruction-v1":   1,
		"splice-api-token-allocation-instruction-v1": 1,
		"splice-api-token-allocation-v1":             1,
	}
}

// registryTemplateByTail is the package-qualified name of each template the registry API reads.
var registryTemplateByTail = map[string]string{
	tailNttManager:              "#ntt:Wormhole.Ntt.Manager:NttManager",
	tailCoinFactory:             "#token-cip0056:Token.CIP0056.CoinFactory:CoinFactory",
	tailCoin:                    "#token-cip0056:Token.CIP0056.Coin:Coin",
	tailTransferPreapproval:     "#token-cip0056:Token.CIP0056.CoinTransfer:TransferPreapproval",
	tailCoinTransferInstruction: "#token-cip0056:Token.CIP0056.CoinTransfer:CoinTransferInstruction",
	tailCoinAllocation:          "#token-cip0056:Token.CIP0056.CoinAllocation:CoinAllocation",
}

// registryQuery reads tail as the registry admin (guardianGovernance) only, capped at the
// per-template contract cap.
func (s *server) registryQuery(tail string, offset int64) (activeContractsQuery, error) {
	template, known := registryTemplateByTail[tail]
	if !known {
		return activeContractsQuery{}, newFlowError(http.StatusInternalServerError, "template %q is not a registry template", tail)
	}
	return activeContractsQuery{
		readerParties:  []partyID{s.opts.readerParties.guardianGovernance},
		template:       template,
		activeAtOffset: offset,
		maxMatches:     s.maxContracts,
	}, nil
}

func fetchRegistryDecoded[T any](ctx context.Context, s *server, tail string, offset int64) ([]decodedEntry[T], error) {
	query, err := s.registryQuery(tail, offset)
	if err != nil {
		return nil, err
	}
	return fetchDecodedQuery[T](ctx, s, query)
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

type registryInstrumentDisplay struct {
	name   string
	symbol string
}

type registryConfig struct {
	adminParty            string
	displayByInstrumentID map[string]registryInstrumentDisplay

	unconfiguredLogMutex sync.Mutex
	unconfiguredLogged   map[string]bool
}

func newRegistryConfig(adminParty string, displayByInstrumentID map[string]registryInstrumentDisplay) *registryConfig {
	if adminParty == "" {
		panic("disclosure-service: newRegistryConfig: empty admin party")
	}
	if displayByInstrumentID == nil {
		panic("disclosure-service: newRegistryConfig: nil display config")
	}
	return &registryConfig{
		adminParty:            adminParty,
		displayByInstrumentID: displayByInstrumentID,
		unconfiguredLogged:    make(map[string]bool),
	}
}

// logUnconfiguredOnce logs an instrument with no display config the first time a request meets it.
func (config *registryConfig) logUnconfiguredOnce(instrumentID string) {
	config.unconfiguredLogMutex.Lock()
	defer config.unconfiguredLogMutex.Unlock()
	if config.unconfiguredLogged[instrumentID] || len(config.unconfiguredLogged) >= maxRegistryInstruments {
		return
	}
	config.unconfiguredLogged[instrumentID] = true
	log.Printf("disclosure-service: registry: instrument %q has no entry in --registry-instruments-file; it is left out of the metadata API", instrumentID)
}

// loadRegistryConfig reads the instruments file. The registry admin is the guardianGovernance party.
func loadRegistryConfig(opts *options) (*registryConfig, error) {
	raw, err := readFileAtMost(opts.registryInstrumentsPath, maxInstrumentsFileBytes)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: read instruments file %s: %w", opts.registryInstrumentsPath, err)
	}
	display, err := parseRegistryInstrumentsFile(raw)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: %s: %w", opts.registryInstrumentsPath, err)
	}
	return newRegistryConfig(string(opts.readerParties.guardianGovernance), display), nil
}

// parseRegistryInstrumentsFile parses {"<instrumentId>": {"name": "...", "symbol": "..."}, ...}.
// Duplicate ids, duplicate or unknown fields, and missing or empty values are errors.
func parseRegistryInstrumentsFile(raw []byte) (map[string]registryInstrumentDisplay, error) {
	if len(raw) > maxInstrumentsFileBytes {
		return nil, fmt.Errorf("instruments file exceeds %d bytes", maxInstrumentsFileBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := expectJSONDelim(decoder, '{', "instruments file"); err != nil {
		return nil, err
	}
	out := make(map[string]registryInstrumentDisplay)
	for decoder.More() {
		if len(out) == maxRegistryInstruments {
			return nil, fmt.Errorf("instruments file lists more than %d instruments", maxRegistryInstruments)
		}
		instrumentID, err := readJSONObjectKey(decoder, "instruments file")
		if err != nil {
			return nil, err
		}
		if instrumentID == "" {
			return nil, fmt.Errorf("instruments file: instrument id must not be empty")
		}
		if len(instrumentID) > maxRegistryIdentifierLength {
			return nil, fmt.Errorf("instruments file: instrument id longer than %d bytes", maxRegistryIdentifierLength)
		}
		if _, duplicate := out[instrumentID]; duplicate {
			return nil, fmt.Errorf("instruments file: duplicate instrument id %q", instrumentID)
		}
		display, err := parseInstrumentDisplay(decoder, instrumentID)
		if err != nil {
			return nil, err
		}
		out[instrumentID] = display
	}
	if err := expectJSONDelim(decoder, '}', "instruments file"); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("instruments file: trailing data after the top-level object")
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("instruments file lists no instruments")
	}
	return out, nil
}

func parseInstrumentDisplay(decoder *json.Decoder, instrumentID string) (registryInstrumentDisplay, error) {
	where := fmt.Sprintf("instruments file: instrument %q", instrumentID)
	if err := expectJSONDelim(decoder, '{', where); err != nil {
		return registryInstrumentDisplay{}, err
	}
	values := make(map[string]string, 2)
	for decoder.More() {
		field, err := readJSONObjectKey(decoder, where)
		if err != nil {
			return registryInstrumentDisplay{}, err
		}
		if field != "name" && field != "symbol" {
			return registryInstrumentDisplay{}, fmt.Errorf("%s: unknown field %q", where, field)
		}
		if _, duplicate := values[field]; duplicate {
			return registryInstrumentDisplay{}, fmt.Errorf("%s: duplicate field %q", where, field)
		}
		token, err := decoder.Token()
		if err != nil {
			return registryInstrumentDisplay{}, fmt.Errorf("%s: field %q: %w", where, field, err)
		}
		text, isString := token.(string)
		if !isString {
			return registryInstrumentDisplay{}, fmt.Errorf("%s: field %q must be a string", where, field)
		}
		if text == "" {
			return registryInstrumentDisplay{}, fmt.Errorf("%s: field %q must not be empty", where, field)
		}
		if len(text) > maxDisplayTextLength {
			return registryInstrumentDisplay{}, fmt.Errorf("%s: field %q longer than %d bytes", where, field, maxDisplayTextLength)
		}
		values[field] = text
	}
	if err := expectJSONDelim(decoder, '}', where); err != nil {
		return registryInstrumentDisplay{}, err
	}
	for _, required := range []string{"name", "symbol"} {
		if _, present := values[required]; !present {
			return registryInstrumentDisplay{}, fmt.Errorf("%s: missing field %q", where, required)
		}
	}
	return registryInstrumentDisplay{name: values["name"], symbol: values["symbol"]}, nil
}

func expectJSONDelim(decoder *json.Decoder, want json.Delim, where string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != want {
		if want == '{' {
			return fmt.Errorf("%s: must be an object", where)
		}
		return fmt.Errorf("%s: expected %q, got %v", where, want, token)
	}
	return nil
}

func readJSONObjectKey(decoder *json.Decoder, where string) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", fmt.Errorf("%s: %w", where, err)
	}
	key, ok := token.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected an object key, got %v", where, token)
	}
	return key, nil
}

// ---------------------------------------------------------------------------
// Daml Decimal: exact arithmetic on values scaled by 10^10
// ---------------------------------------------------------------------------

var damlDecimalUnit = new(big.Int).Exp(big.NewInt(10), big.NewInt(damlDecimalScale), nil)

// parseDamlDecimalScaled parses Daml Decimal text ("-12.5", "3.0000000000") into value*10^10.
func parseDamlDecimalScaled(text string) (*big.Int, error) {
	digits := strings.TrimPrefix(text, "-")
	integerPart, fractionPart, hasPoint := strings.Cut(digits, ".")
	if !isASCIIDigits(integerPart) || (hasPoint && !isASCIIDigits(fractionPart)) {
		return nil, fmt.Errorf("%q is not a Daml Decimal", text)
	}
	if len(fractionPart) > damlDecimalScale {
		return nil, fmt.Errorf("%q has more than %d fractional digits", text, damlDecimalScale)
	}
	if len(strings.TrimLeft(integerPart, "0"))+damlDecimalScale > damlDecimalMaxDigits {
		return nil, fmt.Errorf("%q has more than %d digits", text, damlDecimalMaxDigits)
	}
	padded := integerPart + fractionPart + strings.Repeat("0", damlDecimalScale-len(fractionPart))
	scaled, ok := new(big.Int).SetString(padded, 10)
	if !ok {
		return nil, fmt.Errorf("%q is not a Daml Decimal", text)
	}
	if strings.HasPrefix(text, "-") {
		scaled.Neg(scaled)
	}
	return scaled, nil
}

func isASCIIDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// formatDamlDecimalScaled renders value*10^10 with exactly 10 fractional digits.
func formatDamlDecimalScaled(scaled *big.Int) string {
	magnitude := new(big.Int).Abs(scaled)
	quotient, remainder := new(big.Int).QuoRem(magnitude, damlDecimalUnit, new(big.Int))
	sign := ""
	if scaled.Sign() < 0 {
		sign = "-"
	}
	fraction := remainder.String()
	return sign + quotient.String() + "." + strings.Repeat("0", damlDecimalScale-len(fraction)) + fraction
}

// ---------------------------------------------------------------------------
// Wire types (openapi/*.yaml)
// ---------------------------------------------------------------------------

type registryErrorResponse struct {
	Error string `json:"error"`
}

type registryInfoResponse struct {
	AdminID       string         `json:"adminId"`
	SupportedApis map[string]int `json:"supportedApis"`
}

type registryInstrumentResponse struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Symbol          string         `json:"symbol"`
	TotalSupply     string         `json:"totalSupply"`
	TotalSupplyAsOf string         `json:"totalSupplyAsOf"`
	Decimals        int            `json:"decimals"`
	SupportedApis   map[string]int `json:"supportedApis"`
}

type listInstrumentsResponse struct {
	Instruments   []registryInstrumentResponse `json:"instruments"`
	NextPageToken string                       `json:"nextPageToken,omitempty"`
}

type registryDisclosedContract struct {
	TemplateID       string          `json:"templateId"`
	ContractID       string          `json:"contractId"`
	CreatedEventBlob string          `json:"createdEventBlob"`
	SynchronizerID   string          `json:"synchronizerId"`
	DebugPackageName string          `json:"debugPackageName,omitempty"`
	DebugPayload     json.RawMessage `json:"debugPayload,omitempty"`
	DebugCreatedAt   string          `json:"debugCreatedAt,omitempty"`
}

// registryAnyValue is the Daml-JSON encoding of a Splice.Api.Token.MetadataV1.AnyValue variant.
type registryAnyValue struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

// registryChoiceContextData is the Daml-JSON encoding of Splice.Api.Token.MetadataV1.ChoiceContext.
// A wallet passes it unchanged as extraArgs.context.
type registryChoiceContextData struct {
	Values map[string]registryAnyValue `json:"values"`
}

type registryChoiceContext struct {
	ChoiceContextData  registryChoiceContextData   `json:"choiceContextData"`
	DisclosedContracts []registryDisclosedContract `json:"disclosedContracts"`
}

type transferFactoryWithChoiceContext struct {
	FactoryID     string                `json:"factoryId"`
	TransferKind  string                `json:"transferKind"`
	ChoiceContext registryChoiceContext `json:"choiceContext"`
}

type allocationFactoryWithChoiceContext struct {
	FactoryID     string                `json:"factoryId"`
	ChoiceContext registryChoiceContext `json:"choiceContext"`
}

func emptyChoiceContext() registryChoiceContext {
	return registryChoiceContext{
		ChoiceContextData:  registryChoiceContextData{Values: map[string]registryAnyValue{}},
		DisclosedContracts: []registryDisclosedContract{},
	}
}

// --- Request bodies ---

type getFactoryRequest struct {
	ChoiceArguments    json.RawMessage `json:"choiceArguments"`
	ExcludeDebugFields bool            `json:"excludeDebugFields"`
}

type getChoiceContextRequest struct {
	Meta               map[string]string `json:"meta"`
	ExcludeDebugFields bool              `json:"excludeDebugFields"`
}

// transferFactoryChoiceArguments is the JSON Ledger API encoding of TransferFactory_Transfer,
// reduced to the fields the registry checks.
type transferFactoryChoiceArguments struct {
	ExpectedAdmin string `json:"expectedAdmin"`
	Transfer      *struct {
		Sender       string          `json:"sender"`
		Receiver     string          `json:"receiver"`
		Amount       json.Number     `json:"amount"`
		InstrumentID *daInstrumentId `json:"instrumentId"`
	} `json:"transfer"`
}

// allocationFactoryChoiceArguments is the JSON Ledger API encoding of AllocationFactory_Allocate,
// reduced to the fields the registry checks.
type allocationFactoryChoiceArguments struct {
	ExpectedAdmin string `json:"expectedAdmin"`
	Allocation    *struct {
		TransferLeg *struct {
			Sender       string          `json:"sender"`
			Receiver     string          `json:"receiver"`
			Amount       json.Number     `json:"amount"`
			InstrumentID *daInstrumentId `json:"instrumentId"`
		} `json:"transferLeg"`
	} `json:"allocation"`
}

// --- Daml-JSON decode shapes ---

type daCoin struct {
	InstrumentId daInstrumentId `json:"instrumentId"`
	Owner        string         `json:"owner"`
	Amount       json.Number    `json:"amount"`
}

type daCoinTransferInstruction struct {
	InstrumentId daInstrumentId `json:"instrumentId"`
}

type daCoinAllocation struct {
	Allocation struct {
		TransferLeg struct {
			InstrumentId daInstrumentId `json:"instrumentId"`
		} `json:"transferLeg"`
	} `json:"allocation"`
}

// ---------------------------------------------------------------------------
// Routing and error replies
// ---------------------------------------------------------------------------

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

// writeRegistryError replies with the spec's ErrorResponse and the status a *flowError names.
func writeRegistryError(w http.ResponseWriter, err error) {
	var fe *flowError
	if errors.As(err, &fe) {
		writeJSON(w, fe.status, registryErrorResponse{Error: fe.msg})
		return
	}
	writeJSON(w, http.StatusInternalServerError, registryErrorResponse{Error: fmt.Sprintf("disclosure-service: %v", err)})
}

func (s *server) handleRegistryUnknownPath(w http.ResponseWriter, r *http.Request) {
	writeRegistryError(w, newFlowError(http.StatusNotFound, "no registry endpoint %s %s", r.Method, r.URL.Path))
}

// registryOrNotFound returns the registry config, or replies 404 when the registry API is off.
func (s *server) registryOrNotFound(w http.ResponseWriter) (*registryConfig, bool) {
	if s.registry == nil {
		writeRegistryError(w, newFlowError(http.StatusNotFound, "the registry API is off: start the service with --registry-instruments-file"))
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

// decodeRegistryRequestObject reads a bounded JSON object body into target. Any read, syntax,
// type or trailing-data fault is a 400.
func decodeRegistryRequestObject(w http.ResponseWriter, r *http.Request, target any) error {
	body := http.MaxBytesReader(w, r.Body, maxRegistryRequestBodyBytes)
	decoder := json.NewDecoder(body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return newFlowError(http.StatusBadRequest, "request body: %v", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return newFlowError(http.StatusBadRequest, "request body: trailing data after the JSON object")
	}
	if !isJSONObject(raw) {
		return newFlowError(http.StatusBadRequest, "request body must be a JSON object")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return newFlowError(http.StatusBadRequest, "request body: %v", err)
	}
	return nil
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// ---------------------------------------------------------------------------
// Request validation (pure)
// ---------------------------------------------------------------------------

func checkRegistryIdentifier(field, value string) error {
	if value == "" {
		return newFlowError(http.StatusBadRequest, "%s is required", field)
	}
	if len(value) > maxRegistryIdentifierLength {
		return newFlowError(http.StatusBadRequest, "%s is longer than %d bytes", field, maxRegistryIdentifierLength)
	}
	return nil
}

// checkContractIDText enforces the Canton contract id text form: lowercase hex.
func checkContractIDText(field, contractID string) error {
	if err := checkRegistryIdentifier(field, contractID); err != nil {
		return err
	}
	for _, character := range contractID {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return newFlowError(http.StatusBadRequest, "%s %q is not a hex contract id", field, contractID)
		}
	}
	return nil
}

func checkPositiveAmount(field string, amount json.Number) error {
	scaled, err := parseDamlDecimalScaled(amount.String())
	if err != nil {
		return newFlowError(http.StatusBadRequest, "%s: %v", field, err)
	}
	if scaled.Sign() <= 0 {
		return newFlowError(http.StatusBadRequest, "%s must be positive, got %s", field, amount)
	}
	return nil
}

// checkInstrumentOfRegistry mirrors the factory choice bodies' admin asserts: expectedAdmin must
// equal the instrument admin (400), and the instrument admin must be this registry's admin (404).
func checkInstrumentOfRegistry(expectedAdmin string, instrument daInstrumentId, registryAdmin string) error {
	if expectedAdmin != instrument.Admin {
		return newFlowError(http.StatusBadRequest, "expectedAdmin %s does not match instrumentId.admin %s", expectedAdmin, instrument.Admin)
	}
	if instrument.Admin != registryAdmin {
		return newFlowError(http.StatusNotFound, "instrument admin %s is not this registry's admin %s", instrument.Admin, registryAdmin)
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
		return getFactoryRequest{}, newFlowError(http.StatusBadRequest, "choiceArguments: %v", err)
	}
	return request, nil
}

// requestedTransfer is the validated part of a TransferFactory_Transfer request.
type requestedTransfer struct {
	expectedAdmin string
	sender        string
	receiver      string
	instrument    daInstrumentId
}

func checkTransferFactoryChoiceArguments(arguments transferFactoryChoiceArguments) (requestedTransfer, error) {
	if err := checkRegistryIdentifier("choiceArguments.expectedAdmin", arguments.ExpectedAdmin); err != nil {
		return requestedTransfer{}, err
	}
	transfer := arguments.Transfer
	if transfer == nil {
		return requestedTransfer{}, newFlowError(http.StatusBadRequest, "choiceArguments.transfer is required")
	}
	if transfer.InstrumentID == nil {
		return requestedTransfer{}, newFlowError(http.StatusBadRequest, "choiceArguments.transfer.instrumentId is required")
	}
	fields := []struct{ name, value string }{
		{"choiceArguments.transfer.sender", transfer.Sender},
		{"choiceArguments.transfer.receiver", transfer.Receiver},
		{"choiceArguments.transfer.instrumentId.admin", transfer.InstrumentID.Admin},
		{"choiceArguments.transfer.instrumentId.id", transfer.InstrumentID.Id},
	}
	for _, field := range fields {
		if err := checkRegistryIdentifier(field.name, field.value); err != nil {
			return requestedTransfer{}, err
		}
	}
	if err := checkPositiveAmount("choiceArguments.transfer.amount", transfer.Amount); err != nil {
		return requestedTransfer{}, err
	}
	return requestedTransfer{
		expectedAdmin: arguments.ExpectedAdmin,
		sender:        transfer.Sender,
		receiver:      transfer.Receiver,
		instrument:    *transfer.InstrumentID,
	}, nil
}

// checkAllocationFactoryChoiceArguments returns expectedAdmin and the leg's instrument.
func checkAllocationFactoryChoiceArguments(arguments allocationFactoryChoiceArguments) (string, daInstrumentId, error) {
	if err := checkRegistryIdentifier("choiceArguments.expectedAdmin", arguments.ExpectedAdmin); err != nil {
		return "", daInstrumentId{}, err
	}
	if arguments.Allocation == nil || arguments.Allocation.TransferLeg == nil {
		return "", daInstrumentId{}, newFlowError(http.StatusBadRequest, "choiceArguments.allocation.transferLeg is required")
	}
	leg := arguments.Allocation.TransferLeg
	if leg.InstrumentID == nil {
		return "", daInstrumentId{}, newFlowError(http.StatusBadRequest, "choiceArguments.allocation.transferLeg.instrumentId is required")
	}
	fields := []struct{ name, value string }{
		{"choiceArguments.allocation.transferLeg.sender", leg.Sender},
		{"choiceArguments.allocation.transferLeg.receiver", leg.Receiver},
		{"choiceArguments.allocation.transferLeg.instrumentId.admin", leg.InstrumentID.Admin},
		{"choiceArguments.allocation.transferLeg.instrumentId.id", leg.InstrumentID.Id},
	}
	for _, field := range fields {
		if err := checkRegistryIdentifier(field.name, field.value); err != nil {
			return "", daInstrumentId{}, err
		}
	}
	if err := checkPositiveAmount("choiceArguments.allocation.transferLeg.amount", leg.Amount); err != nil {
		return "", daInstrumentId{}, err
	}
	return arguments.ExpectedAdmin, *leg.InstrumentID, nil
}

// parseInstrumentPageSize clamps pageSize to [1, maxInstrumentPageSize]. An absent or unparsable
// value gives the spec default, as the endpoint declares only 200, 404 and 500.
func parseInstrumentPageSize(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return defaultInstrumentPageSize
	}
	size, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return defaultInstrumentPageSize
	}
	return min(max(size, 1), maxInstrumentPageSize)
}

// ---------------------------------------------------------------------------
// Ledger reads
// ---------------------------------------------------------------------------

// registryInstrument is one burn/mint instrument of the registry admin.
type registryInstrument struct {
	id         string
	decimals   int
	factoryCid string // the canonical CoinFactory the NttManager commits
}

// fetchRegistryInstruments lists the instruments of the burn/mint NttManagers whose instrument
// admin and guardianGovernance are the registry admin, sorted by id.
func (s *server) fetchRegistryInstruments(ctx context.Context, registry *registryConfig, offset int64) ([]registryInstrument, error) {
	managers, err := fetchRegistryDecoded[daNttManager](ctx, s, tailNttManager, offset)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]registryInstrument)
	for _, manager := range managers {
		value := manager.value
		if value.TokenConfig != tokenConfigBurnMint || value.InstrumentId.Admin != registry.adminParty || value.GuardianGovernance != registry.adminParty {
			continue
		}
		if err := requireFactoryTag(value.Factory, factoryTagBurnMint); err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s: %v", manager.entry.ContractID, err)
		}
		decimals, err := value.TokenDecimals.Int64()
		if err != nil || decimals < 0 || decimals > maxTokenDecimals {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s: tokenDecimals %q is outside 0..%d", manager.entry.ContractID, value.TokenDecimals, maxTokenDecimals)
		}
		instrument := registryInstrument{id: value.InstrumentId.Id, decimals: int(decimals), factoryCid: value.Factory.Value}
		if previous, seen := byID[instrument.id]; seen && previous != instrument {
			return nil, newFlowError(http.StatusInternalServerError, "NttManagers disagree on instrument %q: %+v and %+v", instrument.id, previous, instrument)
		}
		byID[instrument.id] = instrument
	}
	out := make([]registryInstrument, 0, len(byID))
	for _, instrument := range byID {
		out = append(out, instrument)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

func (s *server) findRegistryInstrument(ctx context.Context, registry *registryConfig, instrumentID string, offset int64) (registryInstrument, error) {
	instruments, err := s.fetchRegistryInstruments(ctx, registry, offset)
	if err != nil {
		return registryInstrument{}, err
	}
	for _, instrument := range instruments {
		if instrument.id == instrumentID {
			return instrument, nil
		}
	}
	return registryInstrument{}, newFlowError(http.StatusNotFound, "instrument %q is not a registry instrument: no burn/mint NttManager of admin %s", instrumentID, registry.adminParty)
}

// fetchTotalSupplies sums the active Coin amounts per instrument id of the registry admin.
func (s *server) fetchTotalSupplies(ctx context.Context, registry *registryConfig, offset int64) (map[string]*big.Int, error) {
	coins, err := fetchRegistryDecoded[daCoin](ctx, s, tailCoin, offset)
	if err != nil {
		return nil, err
	}
	supplies := make(map[string]*big.Int)
	for _, coin := range coins {
		if coin.value.InstrumentId.Admin != registry.adminParty {
			continue
		}
		amount, err := parseDamlDecimalScaled(coin.value.Amount.String())
		if err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "Coin %s amount: %v", coin.entry.ContractID, err)
		}
		// Coin.daml:18 ensure amount > 0.0.
		if amount.Sign() <= 0 {
			return nil, newFlowError(http.StatusInternalServerError, "Coin %s amount %s is not positive", coin.entry.ContractID, coin.value.Amount)
		}
		total, ok := supplies[coin.value.InstrumentId.Id]
		if !ok {
			total = new(big.Int)
			supplies[coin.value.InstrumentId.Id] = total
		}
		total.Add(total, amount)
	}
	return supplies, nil
}

// requireActiveContract selects contractID from entries. Absent is a 404 naming label.
func requireActiveContract[T any](entries []decodedEntry[T], contractID, label string) (decodedEntry[T], error) {
	var active *decodedEntry[T]
	for index := range entries {
		if entries[index].entry.ContractID == contractID {
			active = &entries[index]
		}
	}
	if active == nil {
		return decodedEntry[T]{}, newFlowError(http.StatusNotFound, "no active %s %s visible to the registry", label, contractID)
	}
	return *active, nil
}

// fetchCanonicalFactory returns the CoinFactory an NttManager commits for instrument.
func (s *server) fetchCanonicalFactory(ctx context.Context, registry *registryConfig, instrument registryInstrument, offset int64) (decodedEntry[daCoinFactory], error) {
	factories, err := fetchRegistryDecoded[daCoinFactory](ctx, s, tailCoinFactory, offset)
	if err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	factory, err := requireActiveContract(factories, instrument.factoryCid, "CoinFactory")
	if err != nil {
		var fe *flowError
		if errors.As(err, &fe) && fe.status == http.StatusNotFound {
			return decodedEntry[daCoinFactory]{}, newFlowError(http.StatusInternalServerError, "committed CoinFactory %s of instrument %q is not active", instrument.factoryCid, instrument.id)
		}
		return decodedEntry[daCoinFactory]{}, err
	}
	//SECURITY: CoinFactory.daml:32-33 serves only instruments whose admin is the factory admin.
	if err := checkCoinFactoryAdminIsGuardianGovernance(factory, partyID(registry.adminParty)); err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	return factory, nil
}

// findReceiverPreapproval returns the receiver's TransferPreapproval for instrument, the lowest
// contract id first when several exist. found is false when there is none.
func findReceiverPreapproval(entries []decodedEntry[daPreapproval], receiver string, instrument daInstrumentId) (decodedEntry[daPreapproval], bool, error) {
	var chosen *decodedEntry[daPreapproval]
	for index := range entries {
		candidate := &entries[index]
		if candidate.value.Owner != receiver || candidate.value.InstrumentId != instrument {
			continue
		}
		if chosen == nil || candidate.entry.ContractID < chosen.entry.ContractID {
			chosen = candidate
		}
	}
	if chosen != nil {
		return *chosen, true, nil
	}
	return decodedEntry[daPreapproval]{}, false, nil
}

// registryDisclosure renders entry as a DisclosedContract.
func registryDisclosure(entry acsEntry, excludeDebugFields bool) (registryDisclosedContract, error) {
	//SECURITY: an active contract always has a synchronizer; the spec requires the field.
	if entry.SynchronizerID == "" {
		return registryDisclosedContract{}, newFlowError(http.StatusBadGateway, "contract %s has no synchronizer assignment", entry.ContractID)
	}
	if entry.CreatedEventBlob == "" {
		return registryDisclosedContract{}, newFlowError(http.StatusBadGateway, "contract %s has no createdEventBlob", entry.ContractID)
	}
	disclosed := registryDisclosedContract{
		TemplateID:       entry.TemplateID,
		ContractID:       entry.ContractID,
		CreatedEventBlob: entry.CreatedEventBlob,
		SynchronizerID:   entry.SynchronizerID,
	}
	if excludeDebugFields {
		return disclosed, nil
	}
	disclosed.DebugPackageName = entry.PackageName
	if isJSONObject(entry.CreateArgument) {
		disclosed.DebugPayload = entry.CreateArgument
	}
	if _, err := time.Parse(time.RFC3339Nano, entry.CreatedAt); err == nil {
		disclosed.DebugCreatedAt = entry.CreatedAt
	}
	return disclosed, nil
}

// ---------------------------------------------------------------------------
// token-metadata-v1
// ---------------------------------------------------------------------------

func (s *server) handleRegistryInfo(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.registryOrNotFound(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, registryInfoResponse{AdminID: registry.adminParty, SupportedApis: registrySupportedApis()})
}

// listedInstruments returns the registry instruments that have display config, in id order.
func (s *server) listedInstruments(ctx context.Context, registry *registryConfig) ([]registryInstrumentResponse, error) {
	offset, err := s.registryLedgerEnd(ctx)
	if err != nil {
		return nil, err
	}
	asOf := s.now().UTC().Format(time.RFC3339Nano)
	instruments, err := s.fetchRegistryInstruments(ctx, registry, offset)
	if err != nil {
		return nil, err
	}
	supplies, err := s.fetchTotalSupplies(ctx, registry, offset)
	if err != nil {
		return nil, err
	}
	out := make([]registryInstrumentResponse, 0, len(instruments))
	for _, instrument := range instruments {
		display, configured := registry.displayByInstrumentID[instrument.id]
		if !configured {
			registry.logUnconfiguredOnce(instrument.id)
			continue
		}
		supply, minted := supplies[instrument.id]
		if !minted {
			supply = new(big.Int)
		}
		out = append(out, registryInstrumentResponse{
			ID:              instrument.id,
			Name:            display.name,
			Symbol:          display.symbol,
			TotalSupply:     formatDamlDecimalScaled(supply),
			TotalSupplyAsOf: asOf,
			Decimals:        instrument.decimals,
			SupportedApis:   registrySupportedApis(),
		})
	}
	return out, nil
}

// handleListInstruments pages by id. pageToken is the last id of the previous page; the next
// page starts at the first id greater than it. Ids stay in order when the set changes.
func (s *server) handleListInstruments(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.registryOrNotFound(w)
	if !ok {
		return
	}
	listed, err := s.listedInstruments(r.Context(), registry)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	pageSize := parseInstrumentPageSize(r.URL.Query().Get("pageSize"))
	pageToken := r.URL.Query().Get("pageToken")
	start := 0
	if pageToken != "" {
		start = sort.Search(len(listed), func(index int) bool { return listed[index].ID > pageToken })
	}
	end := min(start+pageSize, len(listed))
	page := listInstrumentsResponse{Instruments: listed[start:end]}
	if end < len(listed) {
		page.NextPageToken = listed[end-1].ID
	}
	if len(page.Instruments) > maxInstrumentPageSize {
		writeRegistryError(w, newFlowError(http.StatusInternalServerError, "page of %d instruments exceeds %d", len(page.Instruments), maxInstrumentPageSize))
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *server) handleGetInstrument(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.registryOrNotFound(w)
	if !ok {
		return
	}
	instrumentID := r.PathValue("instrumentId")
	if err := checkRegistryIdentifier("instrumentId", instrumentID); err != nil {
		writeRegistryError(w, newFlowError(http.StatusNotFound, "%v", err))
		return
	}
	listed, err := s.listedInstruments(r.Context(), registry)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	for _, instrument := range listed {
		if instrument.ID == instrumentID {
			writeJSON(w, http.StatusOK, instrument)
			return
		}
	}
	writeRegistryError(w, newFlowError(http.StatusNotFound, "no instrument %q in this registry", instrumentID))
}

// ---------------------------------------------------------------------------
// transfer-instruction-v1
// ---------------------------------------------------------------------------

func (s *server) handleTransferFactory(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.registryOrNotFound(w)
	if !ok {
		return
	}
	var arguments transferFactoryChoiceArguments
	request, err := parseFactoryRequest(w, r, &arguments)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	transfer, err := checkTransferFactoryChoiceArguments(arguments)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	response, err := s.assembleTransferFactory(r.Context(), registry, transfer, request.ExcludeDebugFields)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// assembleTransferFactory mirrors TransferFactory_Transfer (CoinFactory.daml:56-77) and
// transferCore (CoinTransfer.daml:152-169).
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
	factory, err := s.fetchCanonicalFactory(ctx, registry, instrument, offset)
	if err != nil {
		return nil, err
	}
	factoryDisclosure, err := registryDisclosure(factory.entry, excludeDebugFields)
	if err != nil {
		return nil, err
	}
	response := &transferFactoryWithChoiceContext{FactoryID: factory.entry.ContractID, ChoiceContext: emptyChoiceContext()}
	response.ChoiceContext.DisclosedContracts = append(response.ChoiceContext.DisclosedContracts, factoryDisclosure)

	// transferCore completes a self-transfer on the receiver authority of the sender.
	if transfer.sender == transfer.receiver {
		response.TransferKind = transferKindSelf
		return response, nil
	}
	preapprovals, err := fetchRegistryDecoded[daPreapproval](ctx, s, tailTransferPreapproval, offset)
	if err != nil {
		return nil, err
	}
	preapproval, found, err := findReceiverPreapproval(preapprovals, transfer.receiver, transfer.instrument)
	if err != nil {
		return nil, err
	}
	if !found {
		response.TransferKind = transferKindOffer
		return response, nil
	}
	preapprovalDisclosure, err := registryDisclosure(preapproval.entry, excludeDebugFields)
	if err != nil {
		return nil, err
	}
	response.TransferKind = transferKindDirect
	response.ChoiceContext.ChoiceContextData.Values[transferPreapprovalContextKey] = registryAnyValue{Tag: anyValueContractIDTag, Value: preapproval.entry.ContractID}
	response.ChoiceContext.DisclosedContracts = append(response.ChoiceContext.DisclosedContracts, preapprovalDisclosure)
	return response, nil
}

// handleTransferInstructionChoiceContext serves accept, reject and withdraw for a
// CoinTransferInstruction. The choice bodies (CoinTransfer.daml:78-89) only create Coins, and
// each controller is a stakeholder: the context is empty.
func (s *server) handleTransferInstructionChoiceContext(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.registryOrNotFound(w)
	if !ok {
		return
	}
	choice := r.PathValue("choice")
	if choice != "accept" && choice != "reject" && choice != "withdraw" {
		writeRegistryError(w, newFlowError(http.StatusNotFound, "unknown transfer instruction choice %q", choice))
		return
	}
	instructionID := r.PathValue("transferInstructionId")
	if err := checkContractIDText("transferInstructionId", instructionID); err != nil {
		writeRegistryError(w, err)
		return
	}
	var request getChoiceContextRequest
	if err := decodeRegistryRequestObject(w, r, &request); err != nil {
		writeRegistryError(w, err)
		return
	}
	offset, err := s.registryLedgerEnd(r.Context())
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	instructions, err := fetchRegistryDecoded[daCoinTransferInstruction](r.Context(), s, tailCoinTransferInstruction, offset)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	instruction, err := requireActiveContract(instructions, instructionID, "CoinTransferInstruction")
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	if instruction.value.InstrumentId.Admin != registry.adminParty {
		writeRegistryError(w, newFlowError(http.StatusNotFound, "CoinTransferInstruction %s is for an instrument of admin %s", instructionID, instruction.value.InstrumentId.Admin))
		return
	}
	writeJSON(w, http.StatusOK, emptyChoiceContext())
}

// ---------------------------------------------------------------------------
// allocation-instruction-v1 and allocation-v1
// ---------------------------------------------------------------------------

// handleAllocationFactory mirrors AllocationFactory_Allocate (CoinFactory.daml:87-109): the
// disclosure set is the CoinFactory, and the context data is empty.
func (s *server) handleAllocationFactory(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.registryOrNotFound(w)
	if !ok {
		return
	}
	var arguments allocationFactoryChoiceArguments
	request, err := parseFactoryRequest(w, r, &arguments)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	expectedAdmin, requestedInstrument, err := checkAllocationFactoryChoiceArguments(arguments)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	if err := checkInstrumentOfRegistry(expectedAdmin, requestedInstrument, registry.adminParty); err != nil {
		writeRegistryError(w, err)
		return
	}
	offset, err := s.registryLedgerEnd(r.Context())
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	instrument, err := s.findRegistryInstrument(r.Context(), registry, requestedInstrument.Id, offset)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	factory, err := s.fetchCanonicalFactory(r.Context(), registry, instrument, offset)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	factoryDisclosure, err := registryDisclosure(factory.entry, request.ExcludeDebugFields)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	response := allocationFactoryWithChoiceContext{FactoryID: factory.entry.ContractID, ChoiceContext: emptyChoiceContext()}
	response.ChoiceContext.DisclosedContracts = append(response.ChoiceContext.DisclosedContracts, factoryDisclosure)
	writeJSON(w, http.StatusOK, response)
}

// handleAllocationChoiceContext serves execute-transfer, withdraw and cancel for a CoinAllocation.
// The choice bodies (CoinAllocation.daml:24-38) only create Coins, and each controller is a
// stakeholder: the context is empty.
func (s *server) handleAllocationChoiceContext(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.registryOrNotFound(w)
	if !ok {
		return
	}
	choice := r.PathValue("choice")
	if choice != "execute-transfer" && choice != "withdraw" && choice != "cancel" {
		writeRegistryError(w, newFlowError(http.StatusNotFound, "unknown allocation choice %q", choice))
		return
	}
	allocationID := r.PathValue("allocationId")
	if err := checkContractIDText("allocationId", allocationID); err != nil {
		writeRegistryError(w, err)
		return
	}
	var request getChoiceContextRequest
	if err := decodeRegistryRequestObject(w, r, &request); err != nil {
		writeRegistryError(w, err)
		return
	}
	offset, err := s.registryLedgerEnd(r.Context())
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	allocations, err := fetchRegistryDecoded[daCoinAllocation](r.Context(), s, tailCoinAllocation, offset)
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	allocation, err := requireActiveContract(allocations, allocationID, "CoinAllocation")
	if err != nil {
		writeRegistryError(w, err)
		return
	}
	if admin := allocation.value.Allocation.TransferLeg.InstrumentId.Admin; admin != registry.adminParty {
		writeRegistryError(w, newFlowError(http.StatusNotFound, "CoinAllocation %s is for an instrument of admin %s", allocationID, admin))
		return
	}
	writeJSON(w, http.StatusOK, emptyChoiceContext())
}
