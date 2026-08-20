// Command disclosure-service is a production-runnable HTTP endpoint. It serves createdEventBlobs
// for a hardcoded allow-list of Daml templates. It reads the templates from one stakeholder
// participant's ACS over the JSON Ledger API v2. This binary is the transport half. The
// canonical per-flow disclosure sets live in the Daml library Wormhole.Ntt.Disclosure. This
// binary serves the flat union of templates that library names.
//
// Posture: the service is unauthenticated and harness/ops-grade. It listens on loopback by
// default. The --access-token-file flag adds a single shared bearer token. The service forwards
// this token to the upstream JSON API.
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
	// maxUpstreamResponseBytes caps every upstream response body. The per-template contract cap
	// fires only after parsing, so this byte ceiling is the earlier bound.
	maxUpstreamResponseBytes = 64 << 20
	serverReadHeaderTimeout  = 10 * time.Second
	serverReadTimeout        = 30 * time.Second
	serverIdleTimeout        = 2 * time.Minute
)

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

// options is parseFlags' pure result. I/O happens later, when run() consumes it.
type options struct {
	listen                  string
	jsonAPIBaseURL          string
	disclosingParty         string
	accessTokenFile         string
	allowListPath           string
	maxContractsPerTemplate int
	verbose                 bool
}

// parseFlags parses args (normally os.Args[1:]) into options. It checks the required flags.
// run() reads the files that flags name. parseFlags returns an actionable error that names the
// missing flag.
func parseFlags(args []string) (*options, error) {
	fs := flag.NewFlagSet("disclosure-service", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller reports the error; this skips flag's own usage dump

	listen := fs.String("listen", defaultListen, "address to listen on")
	jsonAPI := fs.String("json-api", "", "base URL of the JSON Ledger API v2 (required)")
	disclosingParty := fs.String("disclosing-party", "", "the disclosing party: the party as which the service reads the ledger; every served disclosure is a contract this party sees (required)")
	tokenFile := fs.String("access-token-file", "", "path to a file holding a bearer token forwarded upstream; re-read on every upstream request")
	allowList := fs.String("allow-list", "", "path to a JSON array of package-qualified \"#name:Module:Entity\" template names, overriding the built-in allow-list")
	maxContracts := fs.Int("max-contracts-per-template", defaultMaxContractsPerTemplate, "cap on active contracts returned per template; a template that exceeds it is an error")
	verbose := fs.Bool("verbose", false, "verbose logging")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("disclosure-service: %w", err)
	}
	if *jsonAPI == "" {
		return nil, fmt.Errorf("disclosure-service: --json-api is required (base URL of the JSON Ledger API v2, e.g. http://localhost:6975)")
	}
	if *disclosingParty == "" {
		return nil, fmt.Errorf("disclosure-service: --disclosing-party is required (the party as which the service reads the ledger)")
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
		disclosingParty:         *disclosingParty,
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
// (canton/disclosure/daml/Wormhole/Ntt/Disclosure.daml). It is the flat union of every template
// that a disclosure-set function there names. The canonical, flow-aware sets live in that Daml
// module. This table only decides which templates this service may fetch.
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
// ACS client
// ---------------------------------------------------------------------------

// acsEntry is one active contract, as this service needs it. It holds enough data to build a
// Ledger API DisclosedContract client-side.
type acsEntry struct {
	TemplateID       string
	ContractID       string
	CreatedEventBlob string // base64, as returned by the JSON Ledger API
	SynchronizerID   string
}

// acsClient is the minimal seam between the HTTP handlers and the upstream participant. Tests
// can fake it in place of a real JSON Ledger API. httpACSClient is the only production
// implementation. The handler resolves one LedgerEnd offset per request and passes it to every
// ActiveContracts call, so a multi-template response is one consistent snapshot.
type acsClient interface {
	LedgerEnd(ctx context.Context) (int64, error)
	ActiveContracts(ctx context.Context, party, template string, activeAtOffset int64) ([]acsEntry, error)
}

// activeContractsRequest mirrors the JSON Ledger API v2's POST /v2/state/active-contracts
// request body. It has one filtersByParty entry for the disclosing party, one cumulative
// template filter per call, and includeCreatedEventBlob:true. This shape is confirmed live
// against a real participant.
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
	baseURL   string
	tokenFile string // path to the bearer token file; when set, every upstream request carries its bearer token
	client    *http.Client
}

func newHTTPACSClient(baseURL, tokenFile string) *httpACSClient {
	return &httpACSClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		tokenFile: tokenFile,
		client:    &http.Client{Timeout: httpClientTimeout},
	}
}

// authHeader sets Authorization. It re-reads tokenFile on every call. An operator can rotate the
// token file's contents, and the next request picks up the change immediately.
func (c *httpACSClient) authHeader(req *http.Request) error {
	if c.tokenFile == "" {
		return nil
	}
	token, err := readAccessToken(c.tokenFile)
	if err != nil {
		return fmt.Errorf("disclosure-service: reload access token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// readBounded reads a response body of at most maxUpstreamResponseBytes. A larger body is an
// error.
func readBounded(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxUpstreamResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxUpstreamResponseBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxUpstreamResponseBytes)
	}
	return body, nil
}

// LedgerEnd calls GET /v2/state/ledger-end. This pins ActiveContracts' activeAtOffset.
func (c *httpACSClient) LedgerEnd(ctx context.Context) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/state/ledger-end", nil)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: build ledger-end request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.authHeader(req); err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("disclosure-service: ledger-end request: %w", err)
	}
	defer resp.Body.Close()
	body, err := readBounded(resp.Body)
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

// ActiveContracts queries template as party, at activeAtOffset, with
// includeCreatedEventBlob:true. An empty result is a valid answer. A template can have zero
// live contracts for this reader.
func (c *httpACSClient) ActiveContracts(ctx context.Context, party, template string, activeAtOffset int64) ([]acsEntry, error) {
	var reqBody activeContractsRequest
	reqBody.ActiveAtOffset = activeAtOffset
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
	if err := c.authHeader(req); err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: active-contracts request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := readBounded(resp.Body)
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
			continue // other contractEntry variant; skip it
		}
		out = append(out, acsEntry{
			TemplateID:       ac.CreatedEvent.TemplateID,
			ContractID:       ac.CreatedEvent.ContractID,
			CreatedEventBlob: ac.CreatedEvent.CreatedEventBlob,
			SynchronizerID:   ac.SynchronizerID,
		})
	}
	return out, nil
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
	s.mux.HandleFunc("/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("/v1/disclosures", s.handleDisclosures)
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type healthzResponse struct {
	DisclosingParty string `json:"disclosingParty"`
	Templates       int    `json:"templates"`
	LedgerEnd       int64  `json:"ledgerEnd,omitempty"`
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := healthzResponse{
		DisclosingParty: s.opts.disclosingParty,
		Templates:       len(s.allowListOrdered),
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
		http.Error(w, fmt.Sprintf("disclosure-service: ledger-end: %v", err), http.StatusBadGateway)
		return
	}

	out := make([]discloseEntry, 0)
	for _, t := range resolved {
		entries, err := s.acs.ActiveContracts(r.Context(), s.opts.disclosingParty, t, offset)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// run / main
// ---------------------------------------------------------------------------

// readAccessToken reads and trims the bearer token file. It is separate from run() so the
// trim-whitespace behavior is independently testable. A file saved with a trailing newline is
// the common case. A file that trims to nothing is an error: it would send an empty bearer
// token upstream.
func readAccessToken(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("disclosure-service: read access token file %s: %w", path, err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("disclosure-service: access token file %s is empty", path)
	}
	return token, nil
}

// run builds and serves the service until ctx is canceled, then shuts down gracefully. It
// returns nil on a clean shutdown, including http.ErrServerClosed. Any other error is real.
func run(ctx context.Context, opts *options, stderr io.Writer) error {
	allowList := defaultAllowList()
	if opts.allowListPath != "" {
		var err error
		allowList, err = loadAllowList(opts.allowListPath)
		if err != nil {
			return err
		}
	}

	// Fail fast at startup when the token file is missing or unreadable. httpACSClient keeps
	// only the path; see readAccessToken's call in authHeader.
	if opts.accessTokenFile != "" {
		if _, err := readAccessToken(opts.accessTokenFile); err != nil {
			return err
		}
	}

	acs := newHTTPACSClient(opts.jsonAPIBaseURL, opts.accessTokenFile)
	srv := newServer(opts, allowList, acs)

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("disclosure-service: listen on %s: %w", opts.listen, err)
	}

	fmt.Fprintf(stderr, "disclosure-service: disclosing party %s, %d allow-listed template(s)\n", opts.disclosingParty, len(allowList))
	fmt.Fprintln(stderr, "disclosure-service: WARNING unauthenticated harness/ops-grade service -- do not expose beyond a trusted network")
	if host, _, splitErr := net.SplitHostPort(ln.Addr().String()); splitErr == nil {
		if host != "127.0.0.1" && host != "::1" && host != "localhost" {
			fmt.Fprintf(stderr, "disclosure-service: WARNING listening on non-loopback address %s\n", ln.Addr().String())
		}
	}
	fmt.Fprintf(stderr, "disclosure-service: listening on http://%s\n", ln.Addr().String())

	httpSrv := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		// The no-param default queries every allow-listed template plus one ledger-end call, each
		// bounded by httpClientTimeout, so the write timeout scales with the allow-list.
		WriteTimeout: time.Duration(len(allowList)+1) * httpClientTimeout,
		IdleTimeout:  serverIdleTimeout,
	}
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
