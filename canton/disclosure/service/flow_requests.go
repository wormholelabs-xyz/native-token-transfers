package main

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// maxFlowQueryBytes bounds the raw query string of a flow request.
const maxFlowQueryBytes = 1024

// flowParamNames lists the only query parameters each flow accepts.
var flowParamNames = map[string][]string{
	"release":      {"manager", "digest", "recipient"},
	"mint":         {"manager", "digest", "recipient"},
	"set-peer":     {"manager", "digest"},
	"accept-admin": {"manager", "digest"},
	"transfer":     {"manager"},
	"register":     {"gg", "by-vaa"},
	"consolidate":  {"manager"},
}

// checkFlowQuery returns each parameter's single value. A repeated or unknown parameter is a 400.
func checkFlowQuery(flowName, rawQuery string) (map[string]string, error) {
	if len(rawQuery) > maxFlowQueryBytes {
		return nil, newFlowError(http.StatusRequestURITooLong, "query is %d bytes, the limit is %d", len(rawQuery), maxFlowQueryBytes)
	}
	allowedNames, known := flowParamNames[flowName]
	if !known {
		return nil, newFlowError(http.StatusNotFound, "unknown flow %q", truncateForError(flowName))
	}
	parsedQuery, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, newFlowError(http.StatusBadRequest, "malformed query")
	}
	singleValues := make(map[string]string, len(parsedQuery))
	for name, values := range parsedQuery {
		if !slices.Contains(allowedNames, name) {
			return nil, newFlowError(http.StatusBadRequest, "unknown parameter %q for flow %s", truncateForError(name), flowName)
		}
		if len(values) != 1 {
			return nil, newFlowError(http.StatusBadRequest, "parameter %s given %d times", name, len(values))
		}
		singleValues[name] = values[0]
	}
	if len(singleValues) > len(allowedNames) {
		panic("disclosure-service: checkFlowQuery kept more parameters than the flow allows")
	}
	return singleValues, nil
}

// managerAddressHex is 64 lowercase hex characters. Only parseManagerAddressHex builds one.
type managerAddressHex struct{ hex string }

// vaaDigestHex is 64 lowercase hex characters. Only parseVAADigestHex builds one.
type vaaDigestHex struct{ hex string }

func (address managerAddressHex) String() string { return address.hex }
func (digest vaaDigestHex) String() string       { return digest.hex }

func parseManagerAddressHex(raw string) (managerAddressHex, error) {
	lowercaseHex, err := parseHex64(raw, "manager")
	return managerAddressHex{hex: lowercaseHex}, err
}

func parseVAADigestHex(raw string) (vaaDigestHex, error) {
	lowercaseHex, err := parseHex64(raw, "digest")
	return vaaDigestHex{hex: lowercaseHex}, err
}

// parseHex64 lowercases raw and requires exactly 64 hex characters (32 bytes): the lowercase
// form of the Daml 32-byte types (NttManagerAddress, VaaDigest).
func parseHex64(raw, paramName string) (string, error) {
	trimmedHex := strings.ToLower(strings.TrimSpace(raw))
	if len(trimmedHex) != 64 {
		return "", newFlowError(http.StatusBadRequest, "%s must be exactly 64 hex characters, got %d", paramName, len(trimmedHex))
	}
	for _, character := range trimmedHex {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return "", newFlowError(http.StatusBadRequest, "%s must be hex, got %q", paramName, truncateForError(raw))
		}
	}
	return trimmedHex, nil
}

// parseByVAA reads the register flow's by-vaa value. An absent value is false.
func parseByVAA(raw string, present bool) (bool, error) {
	if !present {
		return false, nil
	}
	byVAA, err := strconv.ParseBool(raw)
	if err != nil {
		return false, newFlowError(http.StatusBadRequest, "by-vaa must be a bool, got %q", truncateForError(raw))
	}
	return byVAA, nil
}

// flowRequest is one flow's parsed query: releaseRequest, mintRequest, setPeerRequest,
// acceptAdminRequest, transferRequest, registerRequest, or consolidateRequest.
type flowRequest interface {
	flowName() string
}

type releaseRequest struct {
	manager   managerAddressHex
	digest    vaaDigestHex
	recipient partyID // optional
}

type mintRequest struct {
	manager   managerAddressHex
	digest    vaaDigestHex
	recipient partyID
}

type setPeerRequest struct {
	manager managerAddressHex
	digest  vaaDigestHex
}

type acceptAdminRequest struct {
	manager managerAddressHex
	digest  vaaDigestHex
}

type transferRequest struct {
	manager managerAddressHex
}

type registerRequest struct {
	guardianGovernance partyID // optional
	byVAA              bool
}

type consolidateRequest struct {
	manager managerAddressHex
}

func (releaseRequest) flowName() string     { return "release" }
func (mintRequest) flowName() string        { return "mint" }
func (setPeerRequest) flowName() string     { return "set-peer" }
func (acceptAdminRequest) flowName() string { return "accept-admin" }
func (transferRequest) flowName() string    { return "transfer" }
func (registerRequest) flowName() string    { return "register" }
func (consolidateRequest) flowName() string { return "consolidate" }

// parseFlowRequest checks the query and builds the flow's typed request. It is pure.
func parseFlowRequest(flowName, rawQuery string) (flowRequest, error) {
	params, err := checkFlowQuery(flowName, rawQuery)
	if err != nil {
		return nil, err
	}
	switch flowName {
	case "release":
		manager, digest, err := parseManagerAndDigest(params)
		if err != nil {
			return nil, err
		}
		recipient, err := optionalPartyParam(params, "recipient")
		if err != nil {
			return nil, err
		}
		return releaseRequest{manager: manager, digest: digest, recipient: recipient}, nil
	case "mint":
		manager, digest, err := parseManagerAndDigest(params)
		if err != nil {
			return nil, err
		}
		rawRecipient, err := requiredParam(params, "recipient")
		if err != nil {
			return nil, err
		}
		recipient, err := parseRequestPartyID(rawRecipient, "recipient")
		if err != nil {
			return nil, err
		}
		return mintRequest{manager: manager, digest: digest, recipient: recipient}, nil
	case "set-peer":
		manager, digest, err := parseManagerAndDigest(params)
		if err != nil {
			return nil, err
		}
		return setPeerRequest{manager: manager, digest: digest}, nil
	case "accept-admin":
		manager, digest, err := parseManagerAndDigest(params)
		if err != nil {
			return nil, err
		}
		return acceptAdminRequest{manager: manager, digest: digest}, nil
	case "transfer":
		manager, err := parseManagerParam(params)
		if err != nil {
			return nil, err
		}
		return transferRequest{manager: manager}, nil
	case "consolidate":
		manager, err := parseManagerParam(params)
		if err != nil {
			return nil, err
		}
		return consolidateRequest{manager: manager}, nil
	case "register":
		guardianGovernance, err := optionalPartyParam(params, "gg")
		if err != nil {
			return nil, err
		}
		rawByVAA, byVAAPresent := params["by-vaa"]
		byVAA, err := parseByVAA(rawByVAA, byVAAPresent)
		if err != nil {
			return nil, err
		}
		if byVAA && guardianGovernance == "" {
			return nil, newFlowError(http.StatusBadRequest, "gg is required when by-vaa=true")
		}
		return registerRequest{guardianGovernance: guardianGovernance, byVAA: byVAA}, nil
	default:
		panic("disclosure-service: flowParamNames and parseFlowRequest disagree on flow " + flowName)
	}
}

func requiredParam(params map[string]string, name string) (string, error) {
	paramValue := params[name]
	if paramValue == "" {
		return "", newFlowError(http.StatusBadRequest, "%s is required", name)
	}
	return paramValue, nil
}

// optionalParam returns an absent parameter as "", false. A present, empty parameter is a 400.
func optionalParam(params map[string]string, name string) (string, bool, error) {
	paramValue, present := params[name]
	if !present {
		return "", false, nil
	}
	if paramValue == "" {
		return "", true, newFlowError(http.StatusBadRequest, "%s is empty", name)
	}
	return paramValue, true, nil
}

// optionalPartyParam returns an absent parameter as "". A present value must parse as a party id.
func optionalPartyParam(params map[string]string, name string) (partyID, error) {
	rawParty, present, err := optionalParam(params, name)
	if err != nil || !present {
		return "", err
	}
	return parseRequestPartyID(rawParty, name)
}

func parseManagerParam(params map[string]string) (managerAddressHex, error) {
	rawManager, err := requiredParam(params, "manager")
	if err != nil {
		return managerAddressHex{}, err
	}
	return parseManagerAddressHex(rawManager)
}

func parseManagerAndDigest(params map[string]string) (managerAddressHex, vaaDigestHex, error) {
	manager, err := parseManagerParam(params)
	if err != nil {
		return managerAddressHex{}, vaaDigestHex{}, err
	}
	rawDigest, err := requiredParam(params, "digest")
	if err != nil {
		return managerAddressHex{}, vaaDigestHex{}, err
	}
	digest, err := parseVAADigestHex(rawDigest)
	if err != nil {
		return managerAddressHex{}, vaaDigestHex{}, err
	}
	return manager, digest, nil
}
