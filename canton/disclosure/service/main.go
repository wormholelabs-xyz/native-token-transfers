// Command disclosure-service is a thin, production-runnable HTTP endpoint that serves
// createdEventBlobs for a hardcoded allow-list of Daml templates, read from one stakeholder
// participant's ACS over the JSON Ledger API v2. It is the transport half: the canonical
// per-flow disclosure SETS live in the Daml library (Wormhole.Ntt.Disclosure); this binary
// serves the flat union of templates that library names.
//
// Posture: unauthenticated, harness/ops-grade. Loopback by default; --access-token-file adds a
// single shared bearer token forwarded to the upstream JSON API.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	defaultListen                  = "127.0.0.1:7599"
	defaultMaxContractsPerTemplate = 1000
	httpClientTimeout              = 30 * time.Second
	shutdownTimeout                = 5 * time.Second
)

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

// options is parseFlags' pure result: no I/O happens until run() consumes it.
type options struct {
	listen                  string
	jsonAPIBaseURL          string
	party                   string
	accessTokenFile         string
	allowListPath           string
	maxContractsPerTemplate int
	verbose                 bool
}

// parseFlags parses args (normally os.Args[1:]) into options. It performs no I/O -- required
// fields are checked here, but files named by flags are read later, in run(). Returns an
// actionable error naming the missing flag; never calls os.Exit.
func parseFlags(args []string) (*options, error) {
	fs := flag.NewFlagSet("disclosure-service", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // caller reports the error; avoid flag's own usage dump

	listen := fs.String("listen", defaultListen, "address to listen on")
	jsonAPI := fs.String("json-api", "", "base URL of the JSON Ledger API v2 (required)")
	party := fs.String("party", "", "reading party whose ACS is served (required)")
	tokenFile := fs.String("access-token-file", "", "path to a file holding a bearer token forwarded upstream")
	allowList := fs.String("allow-list", "", "path to a JSON array of \"Module:Entity\" template names, overriding the built-in allow-list")
	maxContracts := fs.Int("max-contracts-per-template", defaultMaxContractsPerTemplate, "cap on active contracts returned per template; a template that exceeds it is an error")
	verbose := fs.Bool("verbose", false, "verbose logging")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("disclosure-service: %w", err)
	}
	if *jsonAPI == "" {
		return nil, fmt.Errorf("disclosure-service: --json-api is required (base URL of the JSON Ledger API v2, e.g. http://localhost:6975)")
	}
	if *party == "" {
		return nil, fmt.Errorf("disclosure-service: --party is required (the reading party whose ACS this service serves)")
	}
	if *listen == "" {
		return nil, fmt.Errorf("disclosure-service: --listen must not be empty")
	}
	if *maxContracts <= 0 {
		return nil, fmt.Errorf("disclosure-service: --max-contracts-per-template must be positive, got %d", *maxContracts)
	}
	return &options{
		listen:                  *listen,
		jsonAPIBaseURL:          *jsonAPI,
		party:                   *party,
		accessTokenFile:         *tokenFile,
		allowListPath:           *allowList,
		maxContractsPerTemplate: *maxContracts,
		verbose:                 *verbose,
	}, nil
}

// ---------------------------------------------------------------------------
// Allow-list
// ---------------------------------------------------------------------------

// defaultAllowList is the transport-side projection of Wormhole.Ntt.Disclosure's module header
// (canton/disclosure/daml/Wormhole/Ntt/Disclosure.daml): the flat union of every template any
// disclosure-set function there ever names. The canonical, flow-aware sets live in that Daml
// module; this table only decides which templates this service may ever be asked to fetch.
func defaultAllowList() []string {
	return []string{
		"Wormhole.Ntt.Manager:NttManager",
		"Wormhole.Ntt.Manager:AdminTransferProposal",
		"Wormhole.Ntt.Ledger:LockedLedger",
		"Wormhole.Ntt.Deposit:DepositPreapproval",
		"Wormhole.Ntt.Governance:NttGovernance",
		"Wormhole.Core.State:CoreState",
		"Wormhole.Core.State:Emitter",
		"Wormhole.Core.State:EmitterRegistry",
		"Wormhole.Core.State:ReplayRootRegistry",
		"Wormhole.Core.Replay:ReplayNode",
		"Token.CIP0056.CoinFactory:CoinFactory",
		"Token.CIP0056.Coin:Coin",
	}
}

// loadAllowList reads a JSON array of "Module:Entity" strings from path, overriding
// defaultAllowList. An empty array is rejected -- an operator who wants "serve nothing" should
// not run this service at all; a genuinely empty override is far more likely a mistake.
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
	return list, nil
}

// templateIDMatches reports whether a fully-qualified templateId ("packageId:Module:Entity",
// as returned by the ACS) belongs to the given "Module:Entity" allow-list name. The package-id
// prefix varies by deployment/upgrade, so matching is by suffix, not equality.
func templateIDMatches(fullyQualified, moduleEntity string) bool {
	return fullyQualified == moduleEntity || strings.HasSuffix(fullyQualified, ":"+moduleEntity)
}

// ---------------------------------------------------------------------------
// ACS client
// ---------------------------------------------------------------------------

// acsEntry is one active contract as this service needs it: enough to build a Ledger API
// DisclosedContract client-side.
type acsEntry struct {
	TemplateID       string
	ContractID       string
	CreatedEventBlob string // base64, as returned by the JSON Ledger API
	SynchronizerID   string
}

// acsClient is the minimal seam between the HTTP handlers and the upstream participant, so
// tests can fake it without a real JSON Ledger API. httpACSClient is the only production
// implementation.
type acsClient interface {
	ActiveContracts(ctx context.Context, party, template string) ([]acsEntry, error)
}

// ledgerEnder is an optional capability an acsClient may additionally provide -- used only to
// enrich /v1/healthz. Absence is not an error; healthz just omits the field.
type ledgerEnder interface {
	LedgerEnd(ctx context.Context) (int64, error)
}

// activeContractsRequest mirrors the JSON Ledger API v2's POST /v2/state/active-contracts
// request body: one filtersByParty entry for the reading party, one cumulative template filter
// per call, includeCreatedEventBlob:true. Shape confirmed live against a real participant (see
// canton/disclosure-service-port's internal/disclosure/acs.go).
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

// acsCreatedEvent is the subset of a JSON Ledger API CreatedEvent this service needs.
type acsCreatedEvent struct {
	ContractID       string `json:"contractId"`
	TemplateID       string `json:"templateId"`
	CreatedEventBlob string `json:"createdEventBlob"`
	SynchronizerID   string `json:"synchronizerId"`
}

// acsActiveContract is the JsActiveContract variant of a contractEntry oneOf.
type acsActiveContract struct {
	CreatedEvent *acsCreatedEvent `json:"createdEvent"`
}

// acsContractEntry is a Daml-JSON sum type; only JsActiveContract carries a createdEvent. Other
// variants (e.g. an incomplete-reassignment marker) decode with a nil field and are skipped.
type acsContractEntry struct {
	JsActiveContract *acsActiveContract `json:"JsActiveContract"`
}

type acsResponseEntry struct {
	ContractEntry acsContractEntry `json:"contractEntry"`
}

// httpACSClient is the production acsClient, talking to one participant's JSON Ledger API v2.
type httpACSClient struct {
	baseURL string
	token   string // bearer token; empty sends no Authorization header
	client  *http.Client
}

func newHTTPACSClient(baseURL, token string) *httpACSClient {
	return &httpACSClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: httpClientTimeout},
	}
}

func (c *httpACSClient) authHeader(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// LedgerEnd calls GET /v2/state/ledger-end, pinning ActiveContracts' activeAtOffset.
func (c *httpACSClient) LedgerEnd(ctx context.Context) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/state/ledger-end", nil)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: build ledger-end request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.authHeader(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: ledger-end request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: read ledger-end response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("disclosure-service: ledger-end: HTTP %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Offset int64 `json:"offset"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("disclosure-service: parse ledger-end response: %w", err)
	}
	return out.Offset, nil
}

// ActiveContracts queries template as party, at the current ledger end, with
// includeCreatedEventBlob:true. Empty results are not an error -- a template with no live
// contracts for this reader is a perfectly valid answer.
func (c *httpACSClient) ActiveContracts(ctx context.Context, party, template string) ([]acsEntry, error) {
	offset, err := c.LedgerEnd(ctx)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: active-contracts: %w", err)
	}

	var reqBody activeContractsRequest
	reqBody.ActiveAtOffset = offset
	var cf acsCumulativeFilter
	cf.IdentifierFilter.TemplateFilter.Value.TemplateID = template
	cf.IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob = true
	reqBody.EventFormat.FiltersByParty = map[string]acsPartyFilter{
		party: {Cumulative: []acsCumulativeFilter{cf}},
	}
	reqBody.EventFormat.Verbose = false

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: marshal active-contracts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/state/active-contracts", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: build active-contracts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.authHeader(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: active-contracts request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: read active-contracts response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("disclosure-service: active-contracts: HTTP %d: %s", resp.StatusCode, respBody)
	}

	var entries []acsResponseEntry
	if err := json.Unmarshal(respBody, &entries); err != nil {
		return nil, fmt.Errorf("disclosure-service: parse active-contracts response: %w", err)
	}

	out := make([]acsEntry, 0, len(entries))
	for _, e := range entries {
		ac := e.ContractEntry.JsActiveContract
		if ac == nil || ac.CreatedEvent == nil {
			continue // non-active-contract oneOf variant; not an error
		}
		out = append(out, acsEntry{
			TemplateID:       ac.CreatedEvent.TemplateID,
			ContractID:       ac.CreatedEvent.ContractID,
			CreatedEventBlob: ac.CreatedEvent.CreatedEventBlob,
			SynchronizerID:   ac.CreatedEvent.SynchronizerID,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// HTTP server
// ---------------------------------------------------------------------------

// server is the disclosure-service's http.Handler. Stateless and query-on-demand: no contract
// id is ever cached across requests.
type server struct {
	opts             *options
	allowList        map[string]bool // exact "Module:Entity" membership, for the 403 gate
	allowListOrdered []string        // preserves the configured order for the no-param default
	acs              acsClient
	maxContracts     int // cap per template; exceeding it is an error
	mux              *http.ServeMux
}

func newServer(opts *options, allowList []string, acs acsClient) *server {
	s := &server{
		opts:             opts,
		allowList:        make(map[string]bool, len(allowList)),
		allowListOrdered: allowList,
		acs:              acs,
		maxContracts:     opts.maxContractsPerTemplate,
	}
	for _, t := range allowList {
		s.allowList[t] = true
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("/v1/disclosures", s.handleDisclosures)
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type healthzResponse struct {
	Party     string `json:"party"`
	Templates int    `json:"templates"`
	LedgerEnd int64  `json:"ledgerEnd,omitempty"`
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := healthzResponse{
		Party:     s.opts.party,
		Templates: len(s.allowListOrdered),
	}
	if le, ok := s.acs.(ledgerEnder); ok {
		if offset, err := le.LedgerEnd(r.Context()); err == nil {
			resp.LedgerEnd = offset
		} else if s.opts.verbose {
			log.Printf("disclosure-service: healthz: ledger-end: %v", err)
		}
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

	// Validate every requested template before querying any of them: a bad request must not
	// have partial upstream side effects. The server-side list is authoritative; a client
	// asking for something outside it is refused outright, naming the offender.
	for _, t := range requested {
		if !s.allowList[t] {
			http.Error(w, fmt.Sprintf("disclosure-service: template %q is not in the allow-list", t), http.StatusForbidden)
			return
		}
	}

	out := make([]discloseEntry, 0)
	for _, t := range requested {
		entries, err := s.acs.ActiveContracts(r.Context(), s.opts.party, t)
		if err != nil {
			http.Error(w, fmt.Sprintf("disclosure-service: query %s: %v", t, err), http.StatusBadGateway)
			return
		}
		if len(entries) > s.maxContracts {
			http.Error(w, fmt.Sprintf(
				"disclosure-service: template %s: %d contracts exceeds cap %d -- refusing rather than truncating",
				t, len(entries), s.maxContracts,
			), http.StatusBadGateway)
			return
		}
		for _, e := range entries {
			// Defense in depth: even though the query itself was filtered to t, do not
			// trust the upstream's echoed templateId blindly before serving it onward.
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// run / main
// ---------------------------------------------------------------------------

// readAccessToken reads and trims the bearer token file. Separated from run() so the
// trim-whitespace behavior (a file saved with a trailing newline is the common case) is
// independently testable.
func readAccessToken(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("disclosure-service: read access token file %s: %w", path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// run builds and serves the service until ctx is canceled, then shuts down gracefully. Returns
// nil on a clean shutdown (including http.ErrServerClosed); any other error is real.
func run(ctx context.Context, opts *options, stderr io.Writer) error {
	allowList := defaultAllowList()
	if opts.allowListPath != "" {
		var err error
		allowList, err = loadAllowList(opts.allowListPath)
		if err != nil {
			return err
		}
	}

	token := ""
	if opts.accessTokenFile != "" {
		var err error
		token, err = readAccessToken(opts.accessTokenFile)
		if err != nil {
			return err
		}
	}

	acs := newHTTPACSClient(opts.jsonAPIBaseURL, token)
	srv := newServer(opts, allowList, acs)

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("disclosure-service: listen on %s: %w", opts.listen, err)
	}

	fmt.Fprintf(stderr, "disclosure-service: fronting party %s, %d allow-listed template(s)\n", opts.party, len(allowList))
	fmt.Fprintln(stderr, "disclosure-service: WARNING unauthenticated harness/ops-grade service -- do not expose beyond a trusted network")
	if host, _, splitErr := net.SplitHostPort(ln.Addr().String()); splitErr == nil {
		if host != "127.0.0.1" && host != "::1" && host != "localhost" {
			fmt.Fprintf(stderr, "disclosure-service: WARNING listening on non-loopback address %s\n", ln.Addr().String())
		}
	}
	fmt.Fprintf(stderr, "disclosure-service: listening on http://%s\n", ln.Addr().String())

	httpSrv := &http.Server{Handler: srv}
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("disclosure-service: shutdown: %w", err)
		}
		<-serveErr // Serve always returns once Shutdown completes
		return nil
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("disclosure-service: serve: %w", err)
		}
		return nil
	}
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, opts, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
