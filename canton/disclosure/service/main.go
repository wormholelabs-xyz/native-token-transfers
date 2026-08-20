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
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
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
	disclosingParties       []string
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
	disclosingParty := fs.String("disclosing-party", "", "comma-separated disclosing parties: the parties as which the service reads the ledger; every served disclosure is a contract at least one of them sees; the access token must grant readAs for each (required)")
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
		return nil, fmt.Errorf("disclosure-service: --disclosing-party is required (the party or comma-separated parties as which the service reads the ledger)")
	}
	parties := make([]string, 0, 4)
	seenParty := make(map[string]bool, 4)
	for _, p := range strings.Split(*disclosingParty, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("disclosure-service: --disclosing-party has an empty entry")
		}
		if seenParty[p] {
			return nil, fmt.Errorf("disclosure-service: --disclosing-party lists %s twice", p)
		}
		seenParty[p] = true
		parties = append(parties, p)
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
		disclosingParties:       parties,
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
	CreateArgument   json.RawMessage // Daml-JSON create arguments; populated only when the request sets eventFormat.verbose
}

// acsClient is the minimal seam between the HTTP handlers and the upstream participant. Tests
// can fake it in place of a real JSON Ledger API. httpACSClient is the only production
// implementation. The handler resolves one LedgerEnd offset per request and passes it to every
// ActiveContracts call, so a multi-template response is one consistent snapshot.
type acsClient interface {
	LedgerEnd(ctx context.Context) (int64, error)
	ActiveContracts(ctx context.Context, parties []string, template string, activeAtOffset int64) ([]acsEntry, error)
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
	ContractID       string          `json:"contractId"`
	TemplateID       string          `json:"templateId"`
	CreatedEventBlob string          `json:"createdEventBlob"`
	CreateArgument   json.RawMessage `json:"createArgument"`
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

// ActiveContracts queries template as the union of parties, at activeAtOffset, with
// includeCreatedEventBlob:true. A contract two parties both see is returned once. An empty
// result is a valid answer. A template can have zero live contracts for these readers.
func (c *httpACSClient) ActiveContracts(ctx context.Context, parties []string, template string, activeAtOffset int64) ([]acsEntry, error) {
	if len(parties) == 0 {
		return nil, fmt.Errorf("disclosure-service: active-contracts: at least one disclosing party is required")
	}
	var reqBody activeContractsRequest
	reqBody.ActiveAtOffset = activeAtOffset
	var cf acsCumulativeFilter
	cf.IdentifierFilter.TemplateFilter.Value.TemplateID = template
	cf.IdentifierFilter.TemplateFilter.Value.IncludeCreatedEventBlob = true
	reqBody.EventFormat.FiltersByParty = make(map[string]acsPartyFilter, len(parties))
	for _, p := range parties {
		reqBody.EventFormat.FiltersByParty[p] = acsPartyFilter{Cumulative: []acsCumulativeFilter{cf}}
	}
	// verbose:true so createArgument carries field labels; the /v1/flows/{flow} selectors decode it.
	reqBody.EventFormat.Verbose = true

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
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		ac := e.ContractEntry.JsActiveContract
		if ac == nil || ac.CreatedEvent == nil {
			continue // other contractEntry variant; skip it
		}
		if seen[ac.CreatedEvent.ContractID] {
			continue // visible to more than one disclosing party; serve it once
		}
		seen[ac.CreatedEvent.ContractID] = true
		out = append(out, acsEntry{
			TemplateID:       ac.CreatedEvent.TemplateID,
			ContractID:       ac.CreatedEvent.ContractID,
			CreatedEventBlob: ac.CreatedEvent.CreatedEventBlob,
			SynchronizerID:   ac.SynchronizerID,
			CreateArgument:   ac.CreatedEvent.CreateArgument,
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
		http.Error(w, fmt.Sprintf("disclosure-service: ledger-end: %v", err), http.StatusBadGateway)
		return
	}

	out := make([]discloseEntry, 0)
	for _, t := range resolved {
		entries, err := s.acs.ActiveContracts(r.Context(), s.opts.disclosingParties, t, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf("disclosure-service: query %s: %v", t, err), http.StatusBadGateway)
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

// ---------------------------------------------------------------------------
// Per-flow disclosure sets
// ---------------------------------------------------------------------------
//
// /v1/flows/{flow} assembles one NTT flow's disclosure set natively in Go, selecting over
// decoded createArgument payloads instead of running a Daml interpreter.
// canton/disclosure/daml/Wormhole/Ntt/Disclosure.daml is the canonical definition of each flow's
// set; this section mirrors its table and reuses its role labels 1:1. Keep the two in step when
// a flow's set changes.

// Template tails this handler fetches. Each is present in defaultAllowList.
const (
	tailNttManager            = "Wormhole.Ntt.Manager:NttManager"
	tailAdminTransferProposal = "Wormhole.Ntt.Manager:AdminTransferProposal"
	tailLockedLedger          = "Wormhole.Ntt.Ledger:LockedLedger"
	tailDepositPreapproval    = "Wormhole.Ntt.Deposit:DepositPreapproval"
	tailNttGovernance         = "Wormhole.Ntt.Governance:NttGovernance"
	tailCoreState             = "Wormhole.Core.State:CoreState"
	tailEmitter               = "Wormhole.Core.State:Emitter"
	tailEmitterRegistry       = "Wormhole.Core.State:EmitterRegistry"
	tailReplayRootRegistry    = "Wormhole.Core.State:ReplayRootRegistry"
	tailReplayNode            = "Wormhole.Core.Replay:ReplayNode"
	tailCoinFactory           = "Token.CIP0056.CoinFactory:CoinFactory"
	tailCoin                  = "Token.CIP0056.Coin:Coin"
	tailTransferPreapproval   = "Token.CIP0056.CoinTransfer:TransferPreapproval"
)

// Role labels, reused 1:1 from Wormhole.Ntt.Disclosure so the two definitions diff cleanly.
const (
	roleNttManager                   = "NttManager"
	roleCoreState                    = "CoreState"
	roleCoveringReplayNode           = "covering ReplayNode"
	roleLockedLedger                 = "LockedLedger"
	roleCustodyPotHolding            = "custody pot Holding"
	roleCommittedTransferFactory     = "committed TransferFactory"
	roleCommittedBurnMintFactory     = "committed BurnMintFactory"
	roleRecipientTransferPreapproval = "recipient TransferPreapproval"
	roleCustodianTransferPreapproval = "custodian TransferPreapproval"
	roleDepositPreapproval           = "DepositPreapproval"
	roleAdminTransferProposal        = "AdminTransferProposal"
	roleTransceiverEmitter           = "transceiver Emitter"
	roleNttGovernance                = "NttGovernance"
	roleEmitterRegistry              = "EmitterRegistry"
	roleReplayRootRegistry           = "ReplayRootRegistry"
	roleCanonicalCoinFactory         = "canonical CoinFactory"
)

// NttTokenConfig's two constructors, decoded as plain strings.
const (
	tokenConfigLockUnlock = "LockUnlock"
	tokenConfigBurnMint   = "BurnMint"
)

// NttFactory's two variant tags.
const (
	factoryTagTransfer = "TransferFactoryCid"
	factoryTagBurnMint = "BurnMintFactoryCid"
)

// reasonNotVisible is every "missing" entry's reason: the disclosing party is not a stakeholder
// of that contract, so the client must supply it.
const reasonNotVisible = "no matching contract visible to the disclosing party"

// --- Decode shapes: Wormhole.Ntt.Disclosure's Daml-JSON createArgument payloads ---

// daInstrumentId decodes Splice.Api.Token.HoldingV1.InstrumentId.
type daInstrumentId struct {
	Admin string `json:"admin"`
	Id    string `json:"id"`
}

// daNttFactory decodes NttFactory's variant encoding: {"tag": "...Cid", "value": "<contractId>"}.
type daNttFactory struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

type daNttManager struct {
	Admin                string         `json:"admin"`
	GuardianGovernance   string         `json:"guardianGovernance"`
	Namespace            string         `json:"namespace"`
	ManagerAddress       string         `json:"managerAddress"`
	TransceiverEmitterId json.Number    `json:"transceiverEmitterId"`
	InstrumentId         daInstrumentId `json:"instrumentId"`
	TokenConfig          string         `json:"tokenConfig"`
	Factory              daNttFactory   `json:"factory"`
}

type daLockedLedger struct {
	ManagerAddress    string  `json:"managerAddress"`
	CustodyHoldingCid *string `json:"custodyHoldingCid"`
}

type daReplayNode struct {
	Consumer  string `json:"consumer"`
	Namespace string `json:"namespace"`
	Prefix    string `json:"prefix"`
}

// daGuardianAnchor decodes the guardianGovernance field shared by CoreState, NttGovernance,
// EmitterRegistry, and ReplayRootRegistry.
type daGuardianAnchor struct {
	GuardianGovernance string `json:"guardianGovernance"`
}

type daEmitter struct {
	Owner     string      `json:"owner"`
	EmitterId json.Number `json:"emitterId"`
}

// daPreapproval decodes both DepositPreapproval and TransferPreapproval: identical shape.
type daPreapproval struct {
	Owner        string         `json:"owner"`
	InstrumentId daInstrumentId `json:"instrumentId"`
}

type daAdminTransferProposal struct {
	Admin          string `json:"admin"`
	NewAdmin       string `json:"newAdmin"`
	ManagerAddress string `json:"managerAddress"`
}

type daCoinFactory struct {
	Admin string `json:"admin"`
}

// daCoin is a placeholder decode target: a pot Holding is matched by contractId alone.
type daCoin struct{}

// --- Wire response ---

// flowDisclosureEntry is one served, role-labeled contract, over the wire.
type flowDisclosureEntry struct {
	Role             string `json:"role"`
	TemplateID       string `json:"templateId"`
	ContractID       string `json:"contractId"`
	CreatedEventBlob string `json:"createdEventBlob"`
	SynchronizerID   string `json:"synchronizerId,omitempty"`
}

// flowMissingEntry names a set member the disclosing party cannot see; the client supplies it.
type flowMissingEntry struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

type flowResponse struct {
	Flow        string                `json:"flow"`
	Disclosures []flowDisclosureEntry `json:"disclosures"`
	Missing     []flowMissingEntry    `json:"missing"`
}

func discloseAs(role string, e acsEntry) flowDisclosureEntry {
	return flowDisclosureEntry{
		Role:             role,
		TemplateID:       e.TemplateID,
		ContractID:       e.ContractID,
		CreatedEventBlob: e.CreatedEventBlob,
		SynchronizerID:   e.SynchronizerID,
	}
}

func missingEntry(role string) flowMissingEntry {
	return flowMissingEntry{Role: role, Reason: reasonNotVisible}
}

// --- flowError: an error that names its own HTTP status ---

// flowError pairs an HTTP status with a message, so each selector names the exact status its
// violation deserves: 400 for a bad param, 404 for an unresolvable manager, 409 for a mode or
// ambiguity conflict, 500 for a uniqueness invariant the ledger's own data violated.
type flowError struct {
	status int
	msg    string
}

func (e *flowError) Error() string { return e.msg }

func newFlowError(status int, format string, args ...any) error {
	return &flowError{status: status, msg: "disclosure-service: " + fmt.Sprintf(format, args...)}
}

// writeFlowError reports err's status if it is a *flowError, else 500: a defensive default, since
// every error an assembler produces is expected to be a *flowError.
func writeFlowError(w http.ResponseWriter, err error) {
	var fe *flowError
	if errors.As(err, &fe) {
		http.Error(w, fe.msg, fe.status)
		return
	}
	http.Error(w, fmt.Sprintf("disclosure-service: %v", err), http.StatusInternalServerError)
}

// --- Param parsing (pure) ---

func requireParam(q url.Values, name string) (string, error) {
	v := q.Get(name)
	if v == "" {
		return "", newFlowError(http.StatusBadRequest, "%s is required", name)
	}
	return v, nil
}

// normalizeHex64 lowercases raw and requires exactly 64 hex characters (32 bytes), matching the
// Daml module's Bytes32/normalizeHex convention.
func normalizeHex64(raw, param string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if len(v) != 64 {
		return "", newFlowError(http.StatusBadRequest, "%s must be exactly 64 hex characters, got %d", param, len(v))
	}
	for _, c := range v {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return "", newFlowError(http.StatusBadRequest, "%s must be hex, got %q", param, raw)
		}
	}
	return v, nil
}

func requireHex64Param(q url.Values, name string) (string, error) {
	raw, err := requireParam(q, name)
	if err != nil {
		return "", err
	}
	return normalizeHex64(raw, name)
}

func parseByVaa(q url.Values) (bool, error) {
	raw := q.Get("by-vaa")
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, newFlowError(http.StatusBadRequest, "by-vaa must be a bool, got %q", raw)
	}
	return v, nil
}

// --- Generic decode + match helpers (pure once entries are in hand) ---

// decodedEntry pairs one ACS entry with its decoded createArgument.
type decodedEntry[T any] struct {
	entry acsEntry
	value T
}

// matchOne counts and returns the first entry satisfying pred.
func matchOne[T any](entries []decodedEntry[T], pred func(decodedEntry[T]) bool) (decodedEntry[T], int) {
	var match decodedEntry[T]
	count := 0
	for _, e := range entries {
		if pred(e) {
			count++
			if count == 1 {
				match = e
			}
		}
	}
	return match, count
}

// exactlyOne requires exactly one match; both zero and multiple matches are the same class of
// invariant violation for a selection-critical role.
func exactlyOne[T any](entries []decodedEntry[T], label string, pred func(decodedEntry[T]) bool) (decodedEntry[T], error) {
	m, n := matchOne(entries, pred)
	if n != 1 {
		return decodedEntry[T]{}, newFlowError(http.StatusInternalServerError, "found %d %s, expected exactly 1", n, label)
	}
	return m, nil
}

// findByContractID returns the entry whose contractId equals cid.
func findByContractID[T any](entries []decodedEntry[T], cid string) (decodedEntry[T], bool) {
	for _, e := range entries {
		if e.entry.ContractID == cid {
			return e, true
		}
	}
	return decodedEntry[T]{}, false
}

// fetchDecoded resolves tail to its allow-listed template name, queries ActiveContracts at
// offset, and decodes each entry's createArgument into T.
func fetchDecoded[T any](ctx context.Context, s *server, tail string, offset int64) ([]decodedEntry[T], error) {
	canonical, ok := s.allowListByTail[tail]
	if !ok {
		return nil, newFlowError(http.StatusInternalServerError, "template %q is not in the allow-list", tail)
	}
	entries, err := s.acs.ActiveContracts(ctx, s.opts.disclosingParties, canonical, offset)
	if err != nil {
		return nil, newFlowError(http.StatusBadGateway, "query %s: %v", canonical, err)
	}
	if msg := s.contractCapError(canonical, len(entries)); msg != "" {
		return nil, newFlowError(http.StatusBadGateway, "%s", msg)
	}
	out := make([]decodedEntry[T], 0, len(entries))
	for _, e := range entries {
		var v T
		if err := json.Unmarshal(e.CreateArgument, &v); err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "decode %s createArgument for contract %s: %v", canonical, e.ContractID, err)
		}
		out = append(out, decodedEntry[T]{entry: e, value: v})
	}
	return out, nil
}

// --- Role selectors: one per Wormhole.Ntt.Disclosure selection rule ---

// findManager returns the NttManager whose managerAddress equals manager (already normalized:
// lowercase, 64 hex chars). Zero matches is a 404: the client named an unknown deployment. Two
// or more is a 500: managerAddress is meant to be unique per deployment.
func findManager(entries []decodedEntry[daNttManager], manager string) (decodedEntry[daNttManager], error) {
	m, n := matchOne(entries, func(e decodedEntry[daNttManager]) bool {
		return strings.ToLower(e.value.ManagerAddress) == manager
	})
	switch {
	case n == 0:
		return decodedEntry[daNttManager]{}, newFlowError(http.StatusNotFound, "no NttManager found for managerAddress %s", manager)
	case n > 1:
		return decodedEntry[daNttManager]{}, newFlowError(http.StatusInternalServerError, "%d NttManager contracts found for managerAddress %s, expected exactly 1", n, manager)
	default:
		return m, nil
	}
}

// findGuardianAnchor selects the entry whose guardianGovernance equals gg. When gg is empty
// (the register flow's optional param), it instead requires the fetched set to hold exactly one
// entry; an ambiguous set without gg is a 409, because the client resolves it by supplying gg.
// Serves CoreState, NttGovernance, EmitterRegistry, and ReplayRootRegistry.
func findGuardianAnchor(entries []decodedEntry[daGuardianAnchor], gg, label string) (decodedEntry[daGuardianAnchor], error) {
	if gg != "" {
		m, n := matchOne(entries, func(e decodedEntry[daGuardianAnchor]) bool {
			return e.value.GuardianGovernance == gg
		})
		switch {
		case n == 1:
			return m, nil
		case n == 0:
			return decodedEntry[daGuardianAnchor]{}, newFlowError(http.StatusNotFound, "no %s for gg %s", label, gg)
		default:
			return decodedEntry[daGuardianAnchor]{}, newFlowError(http.StatusInternalServerError, "found %d %s for gg %s, expected exactly 1", n, label, gg)
		}
	}
	m, n := matchOne(entries, func(decodedEntry[daGuardianAnchor]) bool { return true })
	switch {
	case n == 1:
		return m, nil
	case n == 0:
		return decodedEntry[daGuardianAnchor]{}, newFlowError(http.StatusInternalServerError, "found 0 %s, expected exactly 1", label)
	default:
		return decodedEntry[daGuardianAnchor]{}, newFlowError(http.StatusConflict, "found %d %s and no gg param to disambiguate", n, label)
	}
}

// coveringReplayNode finds the one node of consumer's namespace trie that covers digest,
// mirroring Wormhole.Ntt.Disclosure.coveringNode's partition invariant.
func coveringReplayNode(entries []decodedEntry[daReplayNode], consumer, namespace, digest string) (decodedEntry[daReplayNode], error) {
	return exactlyOne(entries, roleCoveringReplayNode, func(e decodedEntry[daReplayNode]) bool {
		return e.value.Consumer == consumer &&
			e.value.Namespace == namespace &&
			strings.HasPrefix(digest, strings.ToLower(e.value.Prefix))
	})
}

func findLockedLedger(entries []decodedEntry[daLockedLedger], managerAddress string) (decodedEntry[daLockedLedger], error) {
	return exactlyOne(entries, roleLockedLedger, func(e decodedEntry[daLockedLedger]) bool {
		return strings.ToLower(e.value.ManagerAddress) == managerAddress
	})
}

// findTransceiverEmitter matches emitterId (Int, decoded via json.Number since the JSON API may
// emit either a number or a numeric string) and owner.
func findTransceiverEmitter(entries []decodedEntry[daEmitter], emitterID json.Number, owner string) (decodedEntry[daEmitter], error) {
	want, err := emitterID.Int64()
	if err != nil {
		return decodedEntry[daEmitter]{}, newFlowError(http.StatusInternalServerError, "manager transceiverEmitterId %q is not an integer: %v", emitterID, err)
	}
	return exactlyOne(entries, roleTransceiverEmitter, func(e decodedEntry[daEmitter]) bool {
		got, err := e.value.EmitterId.Int64()
		return err == nil && got == want && e.value.Owner == owner
	})
}

func findCanonicalCoinFactory(entries []decodedEntry[daCoinFactory], gg string) (decodedEntry[daCoinFactory], error) {
	return exactlyOne(entries, roleCanonicalCoinFactory, func(e decodedEntry[daCoinFactory]) bool {
		return e.value.Admin == gg
	})
}

// requireFactoryTag checks factory's variant tag against wantTag, mirroring
// requireTransferFactoryCid/requireBurnMintFactoryCid: the committed factory's type must match
// the flow's mode.
func requireFactoryTag(factory daNttFactory, wantTag string) error {
	if factory.Tag != wantTag {
		return newFlowError(http.StatusConflict, "committed factory is %q, this flow requires %q", factory.Tag, wantTag)
	}
	return nil
}

// requireTokenConfig checks mgr's tokenConfig against want, mirroring the Daml module's own
// per-flow mode assert (e.g. releaseDisclosures requires LockUnlock).
func requireTokenConfig(mgr daNttManager, want, flow string) error {
	if mgr.TokenConfig != want {
		return newFlowError(http.StatusConflict, "flow %s requires a %s deployment, this deployment is %s", flow, want, mgr.TokenConfig)
	}
	return nil
}

// findPreapproval matches owner and instrumentId, serving both DepositPreapproval and
// TransferPreapproval (identical decode shape). Zero matches reports !found, and the caller
// decides whether that is selection-critical (DepositPreapproval) or a "missing" entry
// (TransferPreapproval). Two or more is always a 500: ambiguous which contract applies.
func findPreapproval(entries []decodedEntry[daPreapproval], owner string, instrumentID daInstrumentId, label string) (decodedEntry[daPreapproval], bool, error) {
	m, n := matchOne(entries, func(e decodedEntry[daPreapproval]) bool {
		return e.value.Owner == owner && e.value.InstrumentId == instrumentID
	})
	switch {
	case n == 0:
		return decodedEntry[daPreapproval]{}, false, nil
	case n > 1:
		return decodedEntry[daPreapproval]{}, false, newFlowError(http.StatusInternalServerError, "%d %s contracts found for owner %s, expected at most 1", n, label, owner)
	default:
		return m, true, nil
	}
}

// findAdminTransferProposal matches managerAddress. Zero matches goes to the "missing" list; two
// or more is a 500.
func findAdminTransferProposal(entries []decodedEntry[daAdminTransferProposal], managerAddress string) (decodedEntry[daAdminTransferProposal], bool, error) {
	m, n := matchOne(entries, func(e decodedEntry[daAdminTransferProposal]) bool {
		return strings.ToLower(e.value.ManagerAddress) == managerAddress
	})
	switch {
	case n == 0:
		return decodedEntry[daAdminTransferProposal]{}, false, nil
	case n > 1:
		return decodedEntry[daAdminTransferProposal]{}, false, newFlowError(http.StatusInternalServerError, "%d AdminTransferProposal contracts found for managerAddress %s, expected at most 1", n, managerAddress)
	default:
		return m, true, nil
	}
}

// --- Fetch wrappers: one upstream query plus the matching selector ---

func (s *server) fetchManager(ctx context.Context, managerHex string, offset int64) (decodedEntry[daNttManager], error) {
	entries, err := fetchDecoded[daNttManager](ctx, s, tailNttManager, offset)
	if err != nil {
		return decodedEntry[daNttManager]{}, err
	}
	return findManager(entries, managerHex)
}

func (s *server) fetchCoreState(ctx context.Context, gg string, offset int64) (decodedEntry[daGuardianAnchor], error) {
	entries, err := fetchDecoded[daGuardianAnchor](ctx, s, tailCoreState, offset)
	if err != nil {
		return decodedEntry[daGuardianAnchor]{}, err
	}
	return findGuardianAnchor(entries, gg, roleCoreState)
}

func (s *server) fetchCoveringNode(ctx context.Context, consumer, namespace, digest string, offset int64) (decodedEntry[daReplayNode], error) {
	entries, err := fetchDecoded[daReplayNode](ctx, s, tailReplayNode, offset)
	if err != nil {
		return decodedEntry[daReplayNode]{}, err
	}
	return coveringReplayNode(entries, consumer, namespace, digest)
}

func (s *server) fetchLedger(ctx context.Context, managerAddress string, offset int64) (decodedEntry[daLockedLedger], error) {
	entries, err := fetchDecoded[daLockedLedger](ctx, s, tailLockedLedger, offset)
	if err != nil {
		return decodedEntry[daLockedLedger]{}, err
	}
	return findLockedLedger(entries, managerAddress)
}

// fetchCommittedFactory checks factory's tag against wantTag, then resolves it to the CoinFactory
// contract that backs it.
func (s *server) fetchCommittedFactory(ctx context.Context, factory daNttFactory, wantTag string, offset int64) (decodedEntry[daCoinFactory], error) {
	if err := requireFactoryTag(factory, wantTag); err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	entries, err := fetchDecoded[daCoinFactory](ctx, s, tailCoinFactory, offset)
	if err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	entry, ok := findByContractID(entries, factory.Value)
	if !ok {
		return decodedEntry[daCoinFactory]{}, newFlowError(http.StatusInternalServerError, "no CoinFactory found for committed factory contractId %s", factory.Value)
	}
	return entry, nil
}

func (s *server) fetchPotHolding(ctx context.Context, cid string, offset int64) (decodedEntry[daCoin], bool, error) {
	entries, err := fetchDecoded[daCoin](ctx, s, tailCoin, offset)
	if err != nil {
		return decodedEntry[daCoin]{}, false, err
	}
	entry, ok := findByContractID(entries, cid)
	return entry, ok, nil
}

// --- Per-flow assemblers ---

// assembleRelease mirrors releaseDisclosures.
func (s *server) assembleRelease(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	managerHex, err := requireHex64Param(q, "manager")
	if err != nil {
		return nil, err
	}
	digest, err := requireHex64Param(q, "digest")
	if err != nil {
		return nil, err
	}
	recipient := q.Get("recipient")

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	if err := requireTokenConfig(mgr.value, tokenConfigLockUnlock, "release"); err != nil {
		return nil, err
	}
	cs, err := s.fetchCoreState(ctx, mgr.value.GuardianGovernance, offset)
	if err != nil {
		return nil, err
	}
	node, err := s.fetchCoveringNode(ctx, mgr.value.GuardianGovernance, mgr.value.Namespace, digest, offset)
	if err != nil {
		return nil, err
	}
	ledger, err := s.fetchLedger(ctx, mgr.value.ManagerAddress, offset)
	if err != nil {
		return nil, err
	}
	factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer, offset)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttManager, mgr.entry),
		discloseAs(roleCoreState, cs.entry),
		discloseAs(roleCoveringReplayNode, node.entry),
		discloseAs(roleLockedLedger, ledger.entry),
	}
	missing := make([]flowMissingEntry, 0)

	if ledger.value.CustodyHoldingCid != nil {
		pot, found, err := s.fetchPotHolding(ctx, *ledger.value.CustodyHoldingCid, offset)
		if err != nil {
			return nil, err
		}
		if found {
			disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot.entry))
		} else {
			missing = append(missing, missingEntry(roleCustodyPotHolding))
		}
	}

	disclosures = append(disclosures, discloseAs(roleCommittedTransferFactory, factoryEntry.entry))

	if recipient != "" {
		preapprovals, err := fetchDecoded[daPreapproval](ctx, s, tailTransferPreapproval, offset)
		if err != nil {
			return nil, err
		}
		pre, found, err := findPreapproval(preapprovals, recipient, mgr.value.InstrumentId, roleRecipientTransferPreapproval)
		if err != nil {
			return nil, err
		}
		if found {
			disclosures = append(disclosures, discloseAs(roleRecipientTransferPreapproval, pre.entry))
		} else {
			missing = append(missing, missingEntry(roleRecipientTransferPreapproval))
		}
	}

	return &flowResponse{Flow: "release", Disclosures: disclosures, Missing: missing}, nil
}

// assembleMint mirrors mintDisclosures.
func (s *server) assembleMint(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	managerHex, err := requireHex64Param(q, "manager")
	if err != nil {
		return nil, err
	}
	digest, err := requireHex64Param(q, "digest")
	if err != nil {
		return nil, err
	}
	recipient, err := requireParam(q, "recipient")
	if err != nil {
		return nil, err
	}

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	if err := requireTokenConfig(mgr.value, tokenConfigBurnMint, "mint"); err != nil {
		return nil, err
	}
	cs, err := s.fetchCoreState(ctx, mgr.value.GuardianGovernance, offset)
	if err != nil {
		return nil, err
	}
	node, err := s.fetchCoveringNode(ctx, mgr.value.GuardianGovernance, mgr.value.Namespace, digest, offset)
	if err != nil {
		return nil, err
	}

	deposits, err := fetchDecoded[daPreapproval](ctx, s, tailDepositPreapproval, offset)
	if err != nil {
		return nil, err
	}
	dep, found, err := findPreapproval(deposits, recipient, mgr.value.InstrumentId, roleDepositPreapproval)
	if err != nil {
		return nil, err
	}
	if !found {
		// The recipient has not opted in yet; the VAA stays deliverable once they do.
		return nil, newFlowError(http.StatusNotFound, "no DepositPreapproval found for recipient %s and this deployment's instrument", recipient)
	}

	factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagBurnMint, offset)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttManager, mgr.entry),
		discloseAs(roleCoreState, cs.entry),
		discloseAs(roleCoveringReplayNode, node.entry),
		discloseAs(roleDepositPreapproval, dep.entry),
		discloseAs(roleCommittedBurnMintFactory, factoryEntry.entry),
	}
	return &flowResponse{Flow: "mint", Disclosures: disclosures, Missing: make([]flowMissingEntry, 0)}, nil
}

// assembleSetPeer mirrors setPeerByVaaDisclosures.
func (s *server) assembleSetPeer(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	managerHex, err := requireHex64Param(q, "manager")
	if err != nil {
		return nil, err
	}
	digest, err := requireHex64Param(q, "digest")
	if err != nil {
		return nil, err
	}

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	cs, err := s.fetchCoreState(ctx, mgr.value.GuardianGovernance, offset)
	if err != nil {
		return nil, err
	}
	emitters, err := fetchDecoded[daEmitter](ctx, s, tailEmitter, offset)
	if err != nil {
		return nil, err
	}
	emitter, err := findTransceiverEmitter(emitters, mgr.value.TransceiverEmitterId, mgr.value.GuardianGovernance)
	if err != nil {
		return nil, err
	}
	node, err := s.fetchCoveringNode(ctx, mgr.value.GuardianGovernance, mgr.value.Namespace, digest, offset)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttManager, mgr.entry),
		discloseAs(roleCoreState, cs.entry),
		discloseAs(roleTransceiverEmitter, emitter.entry),
		discloseAs(roleCoveringReplayNode, node.entry),
	}
	return &flowResponse{Flow: "set-peer", Disclosures: disclosures, Missing: make([]flowMissingEntry, 0)}, nil
}

// assembleAcceptAdmin mirrors acceptAdminDisclosures.
func (s *server) assembleAcceptAdmin(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	managerHex, err := requireHex64Param(q, "manager")
	if err != nil {
		return nil, err
	}
	digest, err := requireHex64Param(q, "digest")
	if err != nil {
		return nil, err
	}

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	cs, err := s.fetchCoreState(ctx, mgr.value.GuardianGovernance, offset)
	if err != nil {
		return nil, err
	}
	node, err := s.fetchCoveringNode(ctx, mgr.value.GuardianGovernance, mgr.value.Namespace, digest, offset)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttManager, mgr.entry),
		discloseAs(roleCoreState, cs.entry),
		discloseAs(roleCoveringReplayNode, node.entry),
	}
	missing := make([]flowMissingEntry, 0)

	proposals, err := fetchDecoded[daAdminTransferProposal](ctx, s, tailAdminTransferProposal, offset)
	if err != nil {
		return nil, err
	}
	prop, found, err := findAdminTransferProposal(proposals, mgr.value.ManagerAddress)
	if err != nil {
		return nil, err
	}
	if found {
		disclosures = append(disclosures, discloseAs(roleAdminTransferProposal, prop.entry))
	} else {
		missing = append(missing, missingEntry(roleAdminTransferProposal))
	}

	switch mgr.value.TokenConfig {
	case tokenConfigLockUnlock:
		ledger, err := s.fetchLedger(ctx, mgr.value.ManagerAddress, offset)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, discloseAs(roleLockedLedger, ledger.entry))

		if ledger.value.CustodyHoldingCid != nil {
			pot, found, err := s.fetchPotHolding(ctx, *ledger.value.CustodyHoldingCid, offset)
			if err != nil {
				return nil, err
			}
			if found {
				disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot.entry))
			} else {
				missing = append(missing, missingEntry(roleCustodyPotHolding))
			}
			factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer, offset)
			if err != nil {
				return nil, err
			}
			disclosures = append(disclosures, discloseAs(roleCommittedTransferFactory, factoryEntry.entry))
		}

	case tokenConfigBurnMint:
		// The base disclosures (manager, core state, covering node, proposal) are the whole set.

	default:
		return nil, newFlowError(http.StatusInternalServerError, "NttManager has unknown tokenConfig %q", mgr.value.TokenConfig)
	}

	return &flowResponse{Flow: "accept-admin", Disclosures: disclosures, Missing: missing}, nil
}

// assembleTransfer mirrors transferDisclosures.
func (s *server) assembleTransfer(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	managerHex, err := requireHex64Param(q, "manager")
	if err != nil {
		return nil, err
	}

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	emitters, err := fetchDecoded[daEmitter](ctx, s, tailEmitter, offset)
	if err != nil {
		return nil, err
	}
	emitter, err := findTransceiverEmitter(emitters, mgr.value.TransceiverEmitterId, mgr.value.GuardianGovernance)
	if err != nil {
		return nil, err
	}
	cs, err := s.fetchCoreState(ctx, mgr.value.GuardianGovernance, offset)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttManager, mgr.entry),
		discloseAs(roleTransceiverEmitter, emitter.entry),
		discloseAs(roleCoreState, cs.entry),
	}
	missing := make([]flowMissingEntry, 0)

	switch mgr.value.TokenConfig {
	case tokenConfigLockUnlock:
		ledger, err := s.fetchLedger(ctx, mgr.value.ManagerAddress, offset)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, discloseAs(roleLockedLedger, ledger.entry))

		if ledger.value.CustodyHoldingCid != nil {
			pot, found, err := s.fetchPotHolding(ctx, *ledger.value.CustodyHoldingCid, offset)
			if err != nil {
				return nil, err
			}
			if found {
				disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot.entry))
			} else {
				missing = append(missing, missingEntry(roleCustodyPotHolding))
			}
		}

		factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer, offset)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, discloseAs(roleCommittedTransferFactory, factoryEntry.entry))

		preapprovals, err := fetchDecoded[daPreapproval](ctx, s, tailTransferPreapproval, offset)
		if err != nil {
			return nil, err
		}
		pre, found, err := findPreapproval(preapprovals, mgr.value.Admin, mgr.value.InstrumentId, roleCustodianTransferPreapproval)
		if err != nil {
			return nil, err
		}
		if found {
			disclosures = append(disclosures, discloseAs(roleCustodianTransferPreapproval, pre.entry))
		} else {
			missing = append(missing, missingEntry(roleCustodianTransferPreapproval))
		}

	case tokenConfigBurnMint:
		factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagBurnMint, offset)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, discloseAs(roleCommittedBurnMintFactory, factoryEntry.entry))

	default:
		return nil, newFlowError(http.StatusInternalServerError, "NttManager has unknown tokenConfig %q", mgr.value.TokenConfig)
	}

	return &flowResponse{Flow: "transfer", Disclosures: disclosures, Missing: missing}, nil
}

// assembleRegister mirrors registerDisclosures.
func (s *server) assembleRegister(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	gg := q.Get("gg")
	byVaa, err := parseByVaa(q)
	if err != nil {
		return nil, err
	}
	if byVaa && gg == "" {
		return nil, newFlowError(http.StatusBadRequest, "gg is required when by-vaa=true")
	}

	govs, err := fetchDecoded[daGuardianAnchor](ctx, s, tailNttGovernance, offset)
	if err != nil {
		return nil, err
	}
	gov, err := findGuardianAnchor(govs, gg, roleNttGovernance)
	if err != nil {
		return nil, err
	}

	emReg, err := fetchDecoded[daGuardianAnchor](ctx, s, tailEmitterRegistry, offset)
	if err != nil {
		return nil, err
	}
	emRegEntry, err := findGuardianAnchor(emReg, gg, roleEmitterRegistry)
	if err != nil {
		return nil, err
	}

	rrReg, err := fetchDecoded[daGuardianAnchor](ctx, s, tailReplayRootRegistry, offset)
	if err != nil {
		return nil, err
	}
	rrRegEntry, err := findGuardianAnchor(rrReg, gg, roleReplayRootRegistry)
	if err != nil {
		return nil, err
	}

	coreStates, err := fetchDecoded[daGuardianAnchor](ctx, s, tailCoreState, offset)
	if err != nil {
		return nil, err
	}
	cs, err := findGuardianAnchor(coreStates, gg, roleCoreState)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttGovernance, gov.entry),
		discloseAs(roleEmitterRegistry, emRegEntry.entry),
		discloseAs(roleReplayRootRegistry, rrRegEntry.entry),
		discloseAs(roleCoreState, cs.entry),
	}

	if byVaa {
		factories, err := fetchDecoded[daCoinFactory](ctx, s, tailCoinFactory, offset)
		if err != nil {
			return nil, err
		}
		factory, err := findCanonicalCoinFactory(factories, gg)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, discloseAs(roleCanonicalCoinFactory, factory.entry))
	}

	return &flowResponse{Flow: "register", Disclosures: disclosures, Missing: make([]flowMissingEntry, 0)}, nil
}

// assembleConsolidate mirrors consolidateDisclosures. The caller supplies its own custody
// holdings as choice arguments; this set covers only the manager, ledger, and factory.
func (s *server) assembleConsolidate(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	managerHex, err := requireHex64Param(q, "manager")
	if err != nil {
		return nil, err
	}

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	if err := requireTokenConfig(mgr.value, tokenConfigLockUnlock, "consolidate"); err != nil {
		return nil, err
	}
	ledger, err := s.fetchLedger(ctx, mgr.value.ManagerAddress, offset)
	if err != nil {
		return nil, err
	}
	factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer, offset)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttManager, mgr.entry),
		discloseAs(roleLockedLedger, ledger.entry),
		discloseAs(roleCommittedTransferFactory, factoryEntry.entry),
	}
	return &flowResponse{Flow: "consolidate", Disclosures: disclosures, Missing: make([]flowMissingEntry, 0)}, nil
}

// handleFlows serves GET /v1/flows/{flow}. It resolves one LedgerEnd offset, then delegates to
// the named flow's assembler; every assembler fetches only its own flow's templates.
func (s *server) handleFlows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flow := r.PathValue("flow")
	q := r.URL.Query()

	offset, err := s.acs.LedgerEnd(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("disclosure-service: ledger-end: %v", err), http.StatusBadGateway)
		return
	}

	var resp *flowResponse
	switch flow {
	case "release":
		resp, err = s.assembleRelease(r.Context(), q, offset)
	case "mint":
		resp, err = s.assembleMint(r.Context(), q, offset)
	case "set-peer":
		resp, err = s.assembleSetPeer(r.Context(), q, offset)
	case "accept-admin":
		resp, err = s.assembleAcceptAdmin(r.Context(), q, offset)
	case "transfer":
		resp, err = s.assembleTransfer(r.Context(), q, offset)
	case "register":
		resp, err = s.assembleRegister(r.Context(), q, offset)
	case "consolidate":
		resp, err = s.assembleConsolidate(r.Context(), q, offset)
	default:
		http.Error(w, fmt.Sprintf("disclosure-service: unknown flow %q", flow), http.StatusNotFound)
		return
	}
	if err != nil {
		writeFlowError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) contractCapError(template string, count int) string {
	if count > s.maxContracts {
		return fmt.Sprintf("template %s: %d contracts exceeds cap %d -- refusing rather than truncating", template, count, s.maxContracts)
	}
	return ""
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

	fmt.Fprintf(stderr, "disclosure-service: disclosing parties %s, %d allow-listed template(s)\n", strings.Join(opts.disclosingParties, ", "), len(allowList))
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
