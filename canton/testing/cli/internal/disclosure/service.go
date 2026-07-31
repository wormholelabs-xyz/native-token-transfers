package disclosure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ScriptRunner is the subset of internal/ledger.Runner the service needs to run the
// Playground.Prepare seam scripts -- lifted here so Service is unit-testable with a fake and
// carries no `dpm` dependency in tests. Mirrors cmd/ntt-playground/runner.go's scriptRunner
// interface shape exactly; *ledger.Runner satisfies both structurally.
type ScriptRunner interface {
	Run(ctx context.Context, scriptName string, input, output any) error
}

// seamScripts maps the POST /v1/seam/{name} path segment to the Playground.Prepare script it
// runs. Only these four exist today (the plan's §5.6: no new Daml resolution logic beyond
// prepareAcceptAdmin, itself mechanical);
// any other name 404s.
var seamScripts = map[string]string{
	"transferOut":         "Playground.Prepare:prepareTransferOut",
	"receive":             "Playground.Prepare:prepareReceive",
	"publish":             "Playground.Prepare:preparePublish",
	"acceptAdminTransfer": "Playground.Prepare:prepareAcceptAdmin",
}

// Service is the allow-list-gated, stateless disclosure server fronting one participant (the
// plan's §5.2/§5.3).
//
// Security posture (the plan's §4) -- this is a harness-grade implementation, not a production
// one:
//   - No authentication, no TLS. It is still a strict privilege reduction versus today's CLI,
//     which holds the fronted participant's admin JWT and can read its entire ACS and submit as
//     it; this service can only read allow-listed templates and can never submit.
//   - The caller (cmd/ntt-playground) is responsible for binding to 127.0.0.1 by default and
//     requiring an explicit opt-in to listen more broadly.
//   - A production deployment needs, at minimum: mTLS or OAuth client-credentials auth,
//     per-consumer (not global) allow-lists, rate limiting, and an audit log of
//     (consumer, template, contract id, offset). Whether to echo the create argument payload at
//     all (as /v1/disclosures does) is a separate decision a production deployment must make
//     explicitly -- it is no more information than the blob, but it is far more casually
//     consumed.
type Service struct {
	Cfg         Config
	Participant string           // role name served, for /v1/healthz (e.g. "guardian-governance")
	ACS         *ActiveContracts // generic /v1/disclosures backend; may be nil (endpoint then 503s)

	// NewRunner builds the ScriptRunner used by POST /v1/seam/*, run against the participant
	// this service fronts. The returned cleanup func is always called once the script
	// completes (or fails). May be left nil in a service that only ever serves /v1/disclosures.
	NewRunner func(ctx context.Context) (ScriptRunner, func(), error)

	Logf func(string, ...any) // nil-safe
}

func (s *Service) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// Handler builds the service's http.Handler. Uses net/http's ServeMux only -- no router
// dependency, matching the plan's "no new dependency" mandate.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/healthz", s.handleHealthz)
	mux.HandleFunc("/v1/disclosures", s.handleDisclosures)
	mux.HandleFunc("/v1/seam/", s.handleSeam)
	return mux
}

type healthzResponse struct {
	Participant string   `json:"participant"`
	Templates   []string `json:"templates"`
	LedgerEnd   int64    `json:"ledgerEnd,omitempty"`
}

func (s *Service) handleHealthz(w http.ResponseWriter, r *http.Request) {
	resp := healthzResponse{
		Participant: s.Participant,
		Templates:   s.Cfg.Templates(),
	}
	if s.ACS != nil {
		offset, err := s.ACS.LedgerEnd(r.Context())
		if err != nil {
			s.logf("disclosure: healthz: ledger-end: %v", err)
		} else {
			resp.LedgerEnd = offset
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type disclosuresResponse struct {
	ActiveAtOffset int64      `json:"activeAtOffset"`
	Contracts      []Contract `json:"contracts"`
}

func (s *Service) handleDisclosures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	templates := r.URL.Query()["template"]
	if len(templates) == 0 {
		http.Error(w, `disclosure: missing required "template" query parameter`, http.StatusBadRequest)
		return
	}
	// Server-side allow-list enforcement (fixes the plan's §1.4(3)): a template the client
	// asks for but that is not in this service's own Config.Disclose is refused outright,
	// naming the offender, rather than silently fetched and handed over.
	for _, t := range templates {
		if _, ok := s.Cfg.Entry(t); !ok {
			http.Error(w, fmt.Sprintf("disclosure: template %q is not in the allow-list", t), http.StatusForbidden)
			return
		}
	}
	if s.ACS == nil {
		http.Error(w, "disclosure: no ACS backend configured for this service", http.StatusServiceUnavailable)
		return
	}
	contracts, offset, err := s.ACS.Query(r.Context(), templates)
	if err != nil {
		http.Error(w, fmt.Sprintf("disclosure: query active contracts: %v", err), http.StatusInternalServerError)
		return
	}
	for _, t := range templates {
		s.logf("disclosure: served template %s", t)
	}
	writeJSON(w, http.StatusOK, disclosuresResponse{ActiveAtOffset: offset, Contracts: contracts})
}

func (s *Service) handleSeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "disclosure: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/v1/seam/")
	scriptName, ok := seamScripts[name]
	if !ok {
		http.Error(w, fmt.Sprintf("disclosure: unknown seam %q", name), http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("disclosure: read request body: %v", err), http.StatusBadRequest)
		return
	}
	var input map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &input); err != nil {
			http.Error(w, fmt.Sprintf("disclosure: decode request body: %v", err), http.StatusBadRequest)
			return
		}
	}
	if input == nil {
		input = map[string]any{}
	}
	// The server's own allow-list is authoritative (the plan's §1.4(3)): overwrite whatever
	// the client sent for discloseTemplates, if anything, rather than trusting it.
	input["discloseTemplates"] = s.Cfg.Templates()

	if s.NewRunner == nil {
		http.Error(w, "disclosure: seam execution not configured for this service", http.StatusInternalServerError)
		return
	}
	runner, cleanup, err := s.NewRunner(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("disclosure: create script runner: %v", err), http.StatusBadGateway)
		return
	}
	defer cleanup()

	var output json.RawMessage
	if err := runner.Run(r.Context(), scriptName, input, &output); err != nil {
		http.Error(w, fmt.Sprintf("disclosure: run %s: %v", scriptName, err), http.StatusBadGateway)
		return
	}
	s.logf("disclosure: seam %s ok (script=%s, templates=%d)", name, scriptName, len(s.Cfg.Templates()))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
