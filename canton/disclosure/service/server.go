package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Flow templates
// ---------------------------------------------------------------------------

// flowTemplateByTail is the package-qualified name of each template that a flow selector reads.
// Each set comes from its choice body in canton/ntt/daml/Wormhole/Ntt/Manager.daml.
//
// The JSON Ledger API v2 returns HTTP 400 for an unqualified "Module:Entity" template filter
// (confirmed live). The package name is each DAR's own "name:" field.
var flowTemplateByTail = map[string]string{
	tailNttManager:            "#ntt:Wormhole.Ntt.Manager:NttManager",
	tailAdminTransferProposal: "#ntt:Wormhole.Ntt.Manager:AdminTransferProposal",
	tailLockedLedger:          "#ntt:Wormhole.Ntt.Ledger:LockedLedger",
	tailDepositPreapproval:    "#ntt:Wormhole.Ntt.Deposit:DepositPreapproval",
	tailNttGovernance:         "#ntt:Wormhole.Ntt.Governance:NttGovernance",
	tailCoreState:             "#wormhole-core:Wormhole.Core.State:CoreState",
	tailEmitter:               "#wormhole-core:Wormhole.Core.State:Emitter",
	tailEmitterRegistry:       "#wormhole-core:Wormhole.Core.State:EmitterRegistry",
	tailReplayRootRegistry:    "#wormhole-core:Wormhole.Core.State:ReplayRootRegistry",
	tailReplayNode:            "#wormhole-core:Wormhole.Core.Replay:ReplayNode",
	tailCoinFactory:           "#token-cip0056:Token.CIP0056.CoinFactory:CoinFactory",
	tailCoin:                  "#token-cip0056:Token.CIP0056.Coin:Coin",
	tailTransferPreapproval:   "#token-cip0056:Token.CIP0056.CoinTransfer:TransferPreapproval",
}

// custodyPotHoldingTemplateByTail is each template a custody pot can have. The service reads a
// pot by the contract id that its LockedLedger names. The ACS of these templates is never scanned.
var custodyPotHoldingTemplateByTail = map[string]string{
	tailCoin:   "#token-cip0056:Token.CIP0056.Coin:Coin",
	tailAmulet: "#splice-amulet:Splice.Amulet:Amulet",
}

// templateTail returns the trailing "Module:Entity" segment of a template id. It strips any
// package qualifier ("#name:" or a hex package id). An unqualified value (already exactly
// "Module:Entity") is returned unchanged. Module names always exclude ':', so the tail is always
// exactly the last two ':'-separated segments.
func templateTail(templateID string) string {
	parts := strings.Split(templateID, ":")
	if len(parts) <= 2 {
		return templateID
	}
	return strings.Join(parts[len(parts)-2:], ":")
}

// ---------------------------------------------------------------------------
// HTTP server
// ---------------------------------------------------------------------------

// server is the disclosure-service's http.Handler. Its upstream reads go through a short TTL cache.
type server struct {
	opts                   *options
	acs                    acsClient
	maxContracts           int // cap per template; exceeding it is an error
	mux                    *http.ServeMux
	logger                 *log.Logger
	inboundSlots           chan struct{}
	inboundRequestDeadline time.Duration
	readiness              *readinessProbe
	registry               *registryConfig // nil when the registry API is off
}

const (
	maxConcurrentInboundRequests = 64
	inboundRequestDeadline       = 60 * time.Second
)

var errInboundBusy = errors.New("disclosure-service: inbound request cap reached")

func newServer(opts *options, acs acsClient) *server {
	s := &server{
		opts:         opts,
		acs:          acs,
		maxContracts: opts.maxContractsPerTemplate,
		logger:       log.New(os.Stderr, "", log.LstdFlags),

		inboundSlots:           make(chan struct{}, maxConcurrentInboundRequests),
		inboundRequestDeadline: inboundRequestDeadline,
		readiness:              newReadinessProbe(acs),
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/v1", s.handleIndex)
	s.mux.HandleFunc("/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("/v1/readyz", s.handleReadyz)
	s.mux.HandleFunc("/v1/managers", s.handleManagers)
	s.mux.HandleFunc("/v1/flows/{flow}", s.handleFlows)
	s.mountRegistryRoutes()
	return s
}

// pathsOutsideInboundCap are the routes with a constant cost or their own upstream bound
// (readinessProbe). An orchestrator probe gets an answer while flow requests fill the cap.
var pathsOutsideInboundCap = map[string]bool{
	"/v1":         true,
	"/v1/healthz": true,
	"/v1/readyz":  true,
}

// corsGETPatterns are the mux patterns of the public GET routes. Each response on them, success
// or error, carries Access-Control-Allow-Origin: *. An OPTIONS request to them is a CORS
// preflight. The registry POST routes are not in the set.
var corsGETPatterns = map[string]bool{
	"/v1":                                   true,
	"/v1/healthz":                           true,
	"/v1/readyz":                            true,
	"/v1/managers":                          true,
	"/v1/flows/{flow}":                      true,
	"GET /registry/metadata/v1/info":        true,
	"GET /registry/metadata/v1/instruments": true,
	"GET /registry/metadata/v1/instruments/{instrumentId}": true,
}

// CORS values. The routes are public and take no credentials, so the origin is "*" and the
// service never sends Access-Control-Allow-Credentials.
const (
	corsAllowedMethods       = "GET"
	corsAllowedHeaders       = "Accept, Content-Type"
	corsExposedHeaders       = "X-Request-Id"
	corsPreflightMaxAgeValue = "600"
)

// isCORSGETRoute reports whether a GET to r's path reaches one of corsGETPatterns.
func (s *server) isCORSGETRoute(r *http.Request) bool {
	probe := r.WithContext(r.Context())
	probe.Method = http.MethodGet
	_, pattern := s.mux.Handler(probe)
	return corsGETPatterns[pattern]
}

// writeCORSPreflight answers OPTIONS on a public GET route with 204. It reads no body, calls no
// upstream, and has a constant cost, so it runs outside the inbound cap like the probes.
func writeCORSPreflight(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Access-Control-Allow-Methods", corsAllowedMethods)
	header.Set("Access-Control-Allow-Headers", corsAllowedHeaders)
	header.Set("Access-Control-Max-Age", corsPreflightMaxAgeValue)
	header.Set("Allow", "GET, OPTIONS")
	w.WriteHeader(http.StatusNoContent)
}

// ServeHTTP sheds a request past maxConcurrentInboundRequests with a 503 and bounds each request
// by inboundRequestDeadline. The per-client rate limit belongs to the edge proxy.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if s.isCORSGETRoute(r) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", corsExposedHeaders)
		if r.Method == http.MethodOptions {
			writeCORSPreflight(w)
			return
		}
	}
	if pathsOutsideInboundCap[r.URL.Path] {
		s.mux.ServeHTTP(w, r)
		return
	}
	select {
	case s.inboundSlots <- struct{}{}:
	default:
		s.writePublicError(w, r, http.StatusServiceUnavailable, errInboundBusy)
		return
	}
	defer func() { <-s.inboundSlots }()
	ctx, cancel := context.WithTimeout(r.Context(), s.inboundRequestDeadline)
	defer cancel()
	s.mux.ServeHTTP(w, r.WithContext(ctx))
}

// probeResponse is the public body of /v1/healthz and /v1/readyz.
type probeResponse struct {
	Status string `json:"status"`
}

// handleHealthz is liveness: 200 for each GET, at a constant cost, also while the inbound cap is
// full. The reader parties go to the startup log only.
func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, probeResponse{Status: "ok"})
}

// handleReadyz is 200 only when the readiness probe's last ledger-end call succeeded; else 503
// with the fixed public body.
func (s *server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	checkErr, ranUpstreamCheck := s.readiness.checkLedgerEnd(r.Context())
	if checkErr != nil {
		readyzErr := fmt.Errorf("readyz: ledger-end: %w", checkErr)
		// One log line for each upstream check bounds the log rate during an upstream failure.
		if ranUpstreamCheck {
			s.writePublicError(w, r, http.StatusServiceUnavailable, readyzErr)
		} else {
			writeUnloggedPublicError(w, http.StatusServiceUnavailable, readyzErr)
		}
		return
	}
	writeJSON(w, http.StatusOK, probeResponse{Status: "ready"})
}

// flowIndexEndpoints are the /v1 routes, mounted on every server.
var flowIndexEndpoints = []map[string]any{
	{"method": "GET", "path": "/v1", "about": "this index"},
	{"method": "GET", "path": "/v1/healthz", "about": "liveness: 200 while the process serves HTTP"},
	{"method": "GET", "path": "/v1/readyz", "about": "readiness: 200 when the upstream answers, else 503"},
	{"method": "GET", "path": "/v1/managers", "about": "visible NTT deployments with tokenDecimals, admin and peers; read managerAddress here for the flow endpoints' manager param"},
	{"method": "GET", "path": "/v1/flows/{flow}", "about": "one flow's assembled disclosure set, role-labeled, plus a missing list the client supplies; each entry carries the contract's createArgument (JSON Ledger API encoding) when the upstream returned one", "flows": map[string][]string{
		"release":      {"manager", "digest", "recipient (optional)"},
		"mint":         {"manager", "digest", "recipient"},
		"set-peer":     {"manager", "digest"},
		"accept-admin": {"manager", "digest"},
		"transfer":     {"manager"},
		"register":     {"admin", "instrument-admin", "instrument-id", "gg (optional)", "by-vaa (optional bool)"},
		"consolidate":  {"manager"},
	}},
}

// registryIndexEndpoints are the CIP-0056 registry routes, served only when the registry API is on.
var registryIndexEndpoints = []map[string]any{
	{"method": "GET", "path": "/registry/metadata/v1/info", "about": "CIP-0056 token-metadata-v1: registry admin and supported APIs"},
	{"method": "GET", "path": "/registry/metadata/v1/instruments", "params": []string{"pageSize (optional)", "pageToken (optional)"}, "about": "CIP-0056 token-metadata-v1: instruments administered by the registry admin"},
	{"method": "GET", "path": "/registry/metadata/v1/instruments/{instrumentId}", "about": "CIP-0056 token-metadata-v1: one instrument"},
	{"method": "POST", "path": "/registry/transfer-instruction/v1/transfer-factory", "about": "CIP-0056 transfer-instruction-v1: transfer factory and choice context"},
	{"method": "POST", "path": "/registry/transfer-instruction/v1/{transferInstructionId}/choice-contexts/{accept|reject|withdraw}", "about": "CIP-0056 transfer-instruction-v1: choice context for a pending transfer"},
	{"method": "POST", "path": "/registry/allocation-instruction/v1/allocation-factory", "about": "CIP-0056 allocation-instruction-v1: allocation factory and choice context"},
	{"method": "POST", "path": "/registry/allocations/v1/{allocationId}/choice-contexts/{execute-transfer|withdraw|cancel}", "about": "CIP-0056 allocation-v1: choice context for an allocation"},
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.apiIndex())
}

// apiIndex is /v1's payload: every endpoint this server answers and its parameters.
func (s *server) apiIndex() map[string]any {
	endpoints := append([]map[string]any{}, flowIndexEndpoints...)
	if s.registry != nil {
		endpoints = append(endpoints, registryIndexEndpoints...)
	}
	return map[string]any{
		"service":   "disclosure-service",
		"endpoints": endpoints,
	}
}
