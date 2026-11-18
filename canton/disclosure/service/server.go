package main

import (
	"context"
	"errors"
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

// server is the disclosure-service's http.Handler. It is stateless: it queries per request.
type server struct {
	opts                   *options
	acs                    acsClient
	maxContracts           int // cap per template; exceeding it is an error
	mux                    *http.ServeMux
	logger                 *log.Logger
	inboundSlots           chan struct{}
	inboundRequestDeadline time.Duration
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
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/v1", s.handleIndex)
	s.mux.HandleFunc("/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("/v1/managers", s.handleManagers)
	s.mux.HandleFunc("/v1/flows/{flow}", s.handleFlows)
	return s
}

// ServeHTTP sheds a request past maxConcurrentInboundRequests with a 503 and bounds each request
// by inboundRequestDeadline. The per-client rate limit belongs to the authenticating proxy.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

type healthzResponse struct {
	ReaderParties map[string]partyID `json:"readerParties"`
	Templates     int                `json:"templates"`
	LedgerEnd     int64              `json:"ledgerEnd,omitempty"`
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := healthzResponse{
		ReaderParties: s.opts.readerParties.byRoleName(),
		Templates:     len(flowTemplateByTail),
	}
	if offset, err := s.acs.LedgerEnd(r.Context()); err == nil {
		resp.LedgerEnd = offset
	} else if s.opts.verbose {
		log.Printf("disclosure-service: healthz: ledger-end: %v", err)
	}
	writeJSON(w, http.StatusOK, resp)
}

// apiIndex is /v1's static payload: every endpoint and its parameters.
var apiIndex = map[string]any{
	"service": "disclosure-service",
	"endpoints": []map[string]any{
		{"method": "GET", "path": "/v1", "about": "this index"},
		{"method": "GET", "path": "/v1/healthz", "about": "liveness: reader parties by role, template count, ledger end"},
		{"method": "GET", "path": "/v1/managers", "about": "visible NTT deployments; read managerAddress here for the flow endpoints' manager param"},
		{"method": "GET", "path": "/v1/flows/{flow}", "about": "one flow's assembled disclosure set, role-labeled, plus a missing list the client supplies", "flows": map[string][]string{
			"release":      {"manager", "digest", "recipient (optional)"},
			"mint":         {"manager", "digest", "recipient"},
			"set-peer":     {"manager", "digest"},
			"accept-admin": {"manager", "digest"},
			"transfer":     {"manager"},
			"register":     {"gg (optional)", "by-vaa (optional bool)"},
			"consolidate":  {"manager"},
		}},
	},
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, apiIndex)
}
