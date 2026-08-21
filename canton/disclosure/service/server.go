package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// Allow-list
// ---------------------------------------------------------------------------

// defaultAllowList is the flat union of every template the per-flow sets (flows.go) name; each
// set comes from its choice body in canton/ntt/daml/Wormhole/Ntt/Manager.daml. This table only
// decides which templates this service may fetch.
//
// Entries are package-name-qualified ("#<package-name>:Module:Entity"). The JSON Ledger API v2
// returns HTTP 400 for an unqualified "Module:Entity" template filter (confirmed live). The
// package name is each DAR's own "name:" field. See canton/dars/*.conf inside the DAR, and
// canton/ntt/daml.yaml for "ntt".
func defaultAllowList() []string {
	return []string{
		"#ntt:Wormhole.Ntt.Manager:NttManager",
		"#ntt:Wormhole.Ntt.Manager:AdminTransferProposal",
		"#ntt:Wormhole.Ntt.Ledger:LockedLedger",
		"#ntt:Wormhole.Ntt.Deposit:DepositPreapproval",
		"#ntt:Wormhole.Ntt.Governance:NttGovernance",
		"#wormhole-core:Wormhole.Core.State:CoreState",
		"#wormhole-core:Wormhole.Core.State:Emitter",
		"#wormhole-core:Wormhole.Core.State:EmitterRegistry",
		"#wormhole-core:Wormhole.Core.State:ReplayRootRegistry",
		"#wormhole-core:Wormhole.Core.Replay:ReplayNode",
		"#token-cip0056:Token.CIP0056.CoinFactory:CoinFactory",
		"#token-cip0056:Token.CIP0056.Coin:Coin",
		"#token-cip0056:Token.CIP0056.CoinTransfer:TransferPreapproval",
	}
}

// loadAllowList reads a JSON array of template names from path. This overrides defaultAllowList.
// Entries must be package-qualified ("#name:Module:Entity"), matching defaultAllowList's
// convention: the upstream rejects unqualified filters, and allowListByTail keeps one qualified
// name per "Module:Entity" tail, so an unqualified or duplicate entry is a startup error rather
// than a template that silently stops being served. loadAllowList rejects an empty array: a
// genuinely empty override is most likely a mistake. An operator who wants to serve nothing
// should stop the service instead.
func loadAllowList(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: read allow-list %s: %w", path, err)
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("disclosure-service: parse allow-list %s: %w", path, err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("disclosure-service: allow-list %s must not be empty", path)
	}
	seen := make(map[string]string, len(list))
	for _, t := range list {
		if !strings.HasPrefix(t, "#") || len(strings.Split(t, ":")) != 3 {
			return nil, fmt.Errorf("disclosure-service: allow-list %s: entry %q must be package-qualified \"#name:Module:Entity\"", path, t)
		}
		tail := templateTail(t)
		if prev, dup := seen[tail]; dup {
			return nil, fmt.Errorf("disclosure-service: allow-list %s: entries %q and %q share the tail %q", path, prev, t, tail)
		}
		seen[tail] = t
	}
	return list, nil
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

// templateIDMatches reports whether two template ids name the same "Module:Entity". Each id may
// be bare, "#name:"-qualified, or package-ID-qualified, as the ACS returns them. Comparing tails
// lets a package-name-qualified allow-list entry match a package-ID-qualified upstream response.
func templateIDMatches(a, b string) bool {
	return templateTail(a) == templateTail(b)
}

// ---------------------------------------------------------------------------
// HTTP server
// ---------------------------------------------------------------------------

// server is the disclosure-service's http.Handler. It is stateless: it queries per request.
type server struct {
	opts             *options
	allowListByTail  map[string]string // "Module:Entity" tail -> configured qualified name, for the 403 gate
	allowListOrdered []string          // preserves the configured order for the no-param default
	acs              acsClient
	maxContracts     int // cap per template; exceeding it is an error
	mux              *http.ServeMux
}

func newServer(opts *options, allowList []string, acs acsClient) *server {
	s := &server{
		opts:             opts,
		allowListByTail:  make(map[string]string, len(allowList)),
		allowListOrdered: allowList,
		acs:              acs,
		maxContracts:     opts.maxContractsPerTemplate,
	}
	for _, t := range allowList {
		s.allowListByTail[templateTail(t)] = t
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/v1", s.handleIndex)
	s.mux.HandleFunc("/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("/v1/managers", s.handleManagers)
	s.mux.HandleFunc("/v1/disclosures", s.handleDisclosures)
	s.mux.HandleFunc("/v1/flows/{flow}", s.handleFlows)
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type healthzResponse struct {
	DisclosingParties []string `json:"disclosingParties"`
	Templates         int      `json:"templates"`
	LedgerEnd         int64    `json:"ledgerEnd,omitempty"`
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := healthzResponse{
		DisclosingParties: s.opts.disclosingParties,
		Templates:         len(s.allowListOrdered),
	}
	if offset, err := s.acs.LedgerEnd(r.Context()); err == nil {
		resp.LedgerEnd = offset
	} else if s.opts.verbose {
		log.Printf("disclosure-service: healthz: ledger-end: %v", err)
	}
	writeJSON(w, http.StatusOK, resp)
}

// discloseEntry is one served contract, over the wire.
type discloseEntry struct {
	TemplateID       string `json:"templateId"`
	ContractID       string `json:"contractId"`
	CreatedEventBlob string `json:"createdEventBlob"`
	SynchronizerID   string `json:"synchronizerId,omitempty"`
}

func (s *server) handleDisclosures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requested := r.URL.Query()["template"]
	if len(requested) == 0 {
		requested = s.allowListOrdered
	}

	// Resolve every requested template to its configured, qualified allow-list entry before
	// querying any of them. This keeps a bad request free of partial upstream side effects. The
	// server-side list is authoritative. A client that asks for something outside it (by
	// Module:Entity tail) is refused outright, and the error names the offender. A matched
	// request goes upstream under its configured, qualified spelling.
	resolved := make([]string, len(requested))
	for i, t := range requested {
		canonical, ok := s.allowListByTail[templateTail(t)]
		if !ok {
			http.Error(w, fmt.Sprintf("disclosure-service: template %q is not in the allow-list", t), http.StatusForbidden)
			return
		}
		resolved[i] = canonical
	}

	// One offset for the whole request: a multi-template response is one consistent snapshot.
	offset, err := s.acs.LedgerEnd(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("disclosure-service: ledger-end: %v", err), upstreamErrorStatus(err))
		return
	}

	out := make([]discloseEntry, 0)
	for _, t := range resolved {
		entries, err := s.acs.ActiveContracts(r.Context(), s.opts.disclosingParties, t, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf("disclosure-service: query %s: %v", t, err), upstreamErrorStatus(err))
			return
		}
		if msg := s.contractCapError(t, len(entries)); msg != "" {
			http.Error(w, "disclosure-service: "+msg, http.StatusBadGateway)
			return
		}
		for _, e := range entries {
			// Defense in depth: the query was filtered to t, but the handler still verifies
			// the upstream's echoed templateId before serving it onward.
			if !templateIDMatches(e.TemplateID, t) {
				http.Error(w, fmt.Sprintf(
					"disclosure-service: upstream returned mismatched templateId %q for requested %q",
					e.TemplateID, t,
				), http.StatusBadGateway)
				return
			}
			out = append(out, discloseEntry{
				TemplateID:       e.TemplateID,
				ContractID:       e.ContractID,
				CreatedEventBlob: e.CreatedEventBlob,
				SynchronizerID:   e.SynchronizerID,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// apiIndex is /v1's static payload: every endpoint and its parameters.
var apiIndex = map[string]any{
	"service": "disclosure-service",
	"endpoints": []map[string]any{
		{"method": "GET", "path": "/v1", "about": "this index"},
		{"method": "GET", "path": "/v1/healthz", "about": "liveness: disclosing parties, template count, ledger end"},
		{"method": "GET", "path": "/v1/managers", "about": "visible NTT deployments; read managerAddress here for the flow endpoints' manager param"},
		{"method": "GET", "path": "/v1/disclosures", "params": []string{"template (Module:Entity, repeatable; empty = whole allow-list)"}, "about": "raw createdEventBlobs for allow-listed templates"},
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
