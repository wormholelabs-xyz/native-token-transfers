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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"strings"
	"sync"
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

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

type registryInstrumentDisplay struct {
	name   string
	symbol string
}

type registryConfig struct {
	adminParty            partyID
	displayByInstrumentID map[string]registryInstrumentDisplay

	unconfiguredLogMutex sync.Mutex
	unconfiguredLogged   map[string]bool
}

func newRegistryConfig(adminParty partyID, displayByInstrumentID map[string]registryInstrumentDisplay) *registryConfig {
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
	return newRegistryConfig(opts.readerParties.guardianGovernance, display), nil
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
