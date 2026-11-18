package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Per-flow disclosure sets
// ---------------------------------------------------------------------------
//
// /v1/flows/{flow} assembles one NTT flow's disclosure set natively in Go, selecting over
// decoded createArgument payloads. Each set is derived from its choice body in
// canton/ntt/daml/Wormhole/Ntt/Manager.daml: the set holds exactly the contracts the choice
// fetches that a non-stakeholder submitter cannot see. Update the flow's assembler when a
// choice body gains or loses a fetch.

// Template tails this handler fetches. Each is a key of flowTemplateByTail.
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

// Role labels: one per set member, served to clients on each disclosure entry.
const (
	roleNttManager                      = "NttManager"
	roleCoreState                       = "CoreState"
	roleCoveringReplayNode              = "covering ReplayNode"
	roleLockedLedger                    = "LockedLedger"
	roleCustodyPotHolding               = "custody pot Holding"
	roleCommittedTransferFactory        = "committed TransferFactory"
	roleCommittedCanonicalCoinFactory   = "committed canonical CoinFactory"
	roleRecipientTransferPreapproval    = "recipient TransferPreapproval"
	roleCustodianTransferPreapproval    = "custodian TransferPreapproval"
	roleNewCustodianTransferPreapproval = "new custodian TransferPreapproval"
	roleDepositPreapproval              = "DepositPreapproval"
	roleAdminTransferProposal           = "AdminTransferProposal"
	roleTransceiverEmitter              = "transceiver Emitter"
	roleNttGovernance                   = "NttGovernance"
	roleEmitterRegistry                 = "EmitterRegistry"
	roleReplayRootRegistry              = "ReplayRootRegistry"
	roleCanonicalCoinFactory            = "canonical CoinFactory"
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

// reasonNotVisible is every "missing" entry's reason: no reader party is a stakeholder of that
// contract, so the client must supply it.
const reasonNotVisible = "no matching contract visible to the reader parties"

// --- Decode shapes: the templates' Daml-JSON createArgument payloads ---

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
	NttCustody           string         `json:"nttCustody"`
	ReserveCustodian     string         `json:"reserveCustodian"`
	Namespace            string         `json:"namespace"`
	ManagerAddress       string         `json:"managerAddress"`
	ChainId              json.Number    `json:"chainId"`
	ManagerId            json.Number    `json:"managerId"`
	TransceiverEmitterId json.Number    `json:"transceiverEmitterId"`
	InstrumentId         daInstrumentId `json:"instrumentId"`
	TokenConfig          string         `json:"tokenConfig"`
	Factory              daNttFactory   `json:"factory"`
	Paused               bool           `json:"paused"`
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

// exactFieldMatch requires the createArgument field at fieldPath to equal wantValue.
func exactFieldMatch(wantValue string, fieldPath ...string) createArgumentFieldMatch {
	return createArgumentFieldMatch{fieldPath: fieldPath, wantValues: []string{wantValue}}
}

// hexFieldMatch is exactFieldMatch with both sides lowercased.
func hexFieldMatch(wantHex string, fieldPath ...string) createArgumentFieldMatch {
	return createArgumentFieldMatch{fieldPath: fieldPath, wantValues: []string{wantHex}, compareHex: true}
}

// digestPrefixes returns the 65 prefixes of digest, from "" to the whole digest.
func digestPrefixes(digest string) []string {
	if len(digest) != 64 {
		panic(fmt.Sprintf("disclosure-service: digest has %d characters, expected 64", len(digest)))
	}
	prefixes := make([]string, 0, len(digest)+1)
	for prefixLength := 0; prefixLength <= len(digest); prefixLength++ {
		prefixes = append(prefixes, digest[:prefixLength])
	}
	return prefixes
}

// flowQuery builds a query for tail at offset, capped at maxMatchingContractsPerQuery.
func (s *server) flowQuery(tail string, offset int64, fieldMatches ...createArgumentFieldMatch) (activeContractsQuery, error) {
	template, ok := flowTemplateByTail[tail]
	if !ok {
		return activeContractsQuery{}, newFlowError(http.StatusInternalServerError, "template %q is not a flow template", tail)
	}
	return activeContractsQuery{
		readerParties:  s.readersFor(tail),
		template:       template,
		activeAtOffset: offset,
		fieldMatches:   fieldMatches,
		maxMatches:     maxMatchingContractsPerQuery,
	}, nil
}

// fetchDecoded runs flowQuery(tail, offset, fieldMatches...) and decodes each createArgument into T.
func fetchDecoded[T any](ctx context.Context, s *server, tail string, offset int64, fieldMatches ...createArgumentFieldMatch) ([]decodedEntry[T], error) {
	query, err := s.flowQuery(tail, offset, fieldMatches...)
	if err != nil {
		return nil, err
	}
	return fetchDecodedQuery[T](ctx, s, query)
}

func fetchDecodedQuery[T any](ctx context.Context, s *server, query activeContractsQuery) ([]decodedEntry[T], error) {
	entries, err := s.fetchEntries(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]decodedEntry[T], 0, len(entries))
	for _, e := range entries {
		var v T
		if err := json.Unmarshal(e.CreateArgument, &v); err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "decode %s createArgument for contract %s: %v", query.template, e.ContractID, err)
		}
		out = append(out, decodedEntry[T]{entry: e, value: v})
	}
	return out, nil
}

// fetchEntries runs query and checks each returned template id.
func (s *server) fetchEntries(ctx context.Context, query activeContractsQuery) ([]acsEntry, error) {
	entries, err := s.acs.ActiveContracts(ctx, query)
	if err != nil {
		return nil, newFlowError(upstreamErrorStatus(err), "query %s: %v", query.template, err)
	}
	if len(entries) > query.maxMatches {
		return nil, newFlowError(http.StatusBadGateway, "query %s: %d contracts, cap %d", query.template, len(entries), query.maxMatches)
	}
	for _, entry := range entries {
		if !templateIDMatches(entry.TemplateID, query.template) {
			return nil, newFlowError(http.StatusBadGateway, "upstream returned templateId %q for requested %q", entry.TemplateID, query.template)
		}
	}
	return entries, nil
}

// reserveCustodianFor mirrors Manager.daml's reserveCustodianFor: nttCustody while gg holds the
// admin role, else admin.
func reserveCustodianFor(guardianGovernance, nttCustody, admin string) string {
	if admin == guardianGovernance {
		return nttCustody
	}
	return admin
}

// checkReserveCustodianFollowsAdmin mirrors the NttManager template's ensure clause. A payload
// with no nttCustody predates ntt 0.4.0.
func checkReserveCustodianFollowsAdmin(mgr daNttManager) error {
	if mgr.NttCustody == "" {
		return newFlowError(http.StatusInternalServerError, "NttManager %s has no nttCustody: ntt 0.4.0 or later is required", mgr.ManagerAddress)
	}
	if mgr.ReserveCustodian == "" {
		return newFlowError(http.StatusInternalServerError, "NttManager %s has no reserveCustodian: ntt 0.4.0 or later is required", mgr.ManagerAddress)
	}
	if want := reserveCustodianFor(mgr.GuardianGovernance, mgr.NttCustody, mgr.Admin); mgr.ReserveCustodian != want {
		return newFlowError(http.StatusInternalServerError, "NttManager %s reserveCustodian is %s, expected %s", mgr.ManagerAddress, mgr.ReserveCustodian, want)
	}
	if mgr.ReserveCustodian == mgr.GuardianGovernance {
		return newFlowError(http.StatusInternalServerError, "NttManager %s reserveCustodian is guardianGovernance", mgr.ManagerAddress)
	}
	if mgr.Admin == mgr.NttCustody {
		return newFlowError(http.StatusInternalServerError, "NttManager %s admin is nttCustody", mgr.ManagerAddress)
	}
	if mgr.NttCustody == mgr.GuardianGovernance {
		return newFlowError(http.StatusInternalServerError, "NttManager %s nttCustody is guardianGovernance", mgr.ManagerAddress)
	}
	return nil
}

// --- Role selectors: one per set member ---

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

// coveringReplayNode finds the one node of consumer's namespace trie that covers digest.
// Exactly one node covers any digest (the trie partition invariant); more or fewer shows a
// forked or broken trie.
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

// checkCoinFactoryAdminIsGuardianGovernance mirrors requireCanonicalFactory's admin check.
func checkCoinFactoryAdminIsGuardianGovernance(factory decodedEntry[daCoinFactory], gg string) error {
	if factory.value.Admin != gg {
		return newFlowError(http.StatusInternalServerError, "committed CoinFactory %s has admin %s, expected guardianGovernance %s", factory.entry.ContractID, factory.value.Admin, gg)
	}
	return nil
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

// findAdminTransferProposal mirrors AcceptAdminTransferByVaa's three proposal asserts: admin,
// newAdmin == guardianGovernance, and managerAddress. Zero matches goes to the "missing" list;
// two or more is a 500.
func findAdminTransferProposal(entries []decodedEntry[daAdminTransferProposal], manager daNttManager) (decodedEntry[daAdminTransferProposal], bool, error) {
	match, matchCount := matchOne(entries, func(entry decodedEntry[daAdminTransferProposal]) bool {
		return entry.value.Admin == manager.Admin &&
			entry.value.NewAdmin == manager.GuardianGovernance &&
			strings.EqualFold(entry.value.ManagerAddress, manager.ManagerAddress)
	})
	switch {
	case matchCount == 0:
		return decodedEntry[daAdminTransferProposal]{}, false, nil
	case matchCount > 1:
		return decodedEntry[daAdminTransferProposal]{}, false, newFlowError(http.StatusInternalServerError, "%d AdminTransferProposal contracts found for managerAddress %s, expected at most 1", matchCount, manager.ManagerAddress)
	default:
		return match, true, nil
	}
}

// --- Fetch wrappers: one upstream query plus the matching selector ---

func (s *server) fetchManager(ctx context.Context, managerHex string, offset int64) (decodedEntry[daNttManager], error) {
	entries, err := fetchDecoded[daNttManager](ctx, s, tailNttManager, offset, hexFieldMatch(managerHex, "managerAddress"))
	if err != nil {
		return decodedEntry[daNttManager]{}, err
	}
	mgr, err := findManager(entries, managerHex)
	if err != nil {
		return decodedEntry[daNttManager]{}, err
	}
	if err := checkReserveCustodianFollowsAdmin(mgr.value); err != nil {
		return decodedEntry[daNttManager]{}, err
	}
	return mgr, nil
}

func (s *server) fetchCoreState(ctx context.Context, gg string, offset int64) (decodedEntry[daGuardianAnchor], error) {
	return s.fetchGuardianAnchor(ctx, tailCoreState, gg, roleCoreState, offset)
}

// fetchGuardianAnchor filters on guardianGovernance when gg is set.
func (s *server) fetchGuardianAnchor(ctx context.Context, tail, gg, label string, offset int64) (decodedEntry[daGuardianAnchor], error) {
	var fieldMatches []createArgumentFieldMatch
	if gg != "" {
		fieldMatches = append(fieldMatches, exactFieldMatch(gg, "guardianGovernance"))
	}
	entries, err := fetchDecoded[daGuardianAnchor](ctx, s, tail, offset, fieldMatches...)
	if err != nil {
		return decodedEntry[daGuardianAnchor]{}, err
	}
	return findGuardianAnchor(entries, gg, label)
}

func (s *server) fetchCoveringNode(ctx context.Context, consumer, namespace, digest string, offset int64) (decodedEntry[daReplayNode], error) {
	entries, err := fetchDecoded[daReplayNode](ctx, s, tailReplayNode, offset,
		exactFieldMatch(consumer, "consumer"),
		exactFieldMatch(namespace, "namespace"),
		createArgumentFieldMatch{fieldPath: []string{"prefix"}, wantValues: digestPrefixes(digest), compareHex: true})
	if err != nil {
		return decodedEntry[daReplayNode]{}, err
	}
	return coveringReplayNode(entries, consumer, namespace, digest)
}

func (s *server) fetchLedger(ctx context.Context, managerAddress string, offset int64) (decodedEntry[daLockedLedger], error) {
	managerAddress = strings.ToLower(managerAddress)
	entries, err := fetchDecoded[daLockedLedger](ctx, s, tailLockedLedger, offset, hexFieldMatch(managerAddress, "managerAddress"))
	if err != nil {
		return decodedEntry[daLockedLedger]{}, err
	}
	return findLockedLedger(entries, managerAddress)
}

// fetchCommittedFactory checks factory's tag against wantTag, then reads the CoinFactory that
// backs it by contract id.
func (s *server) fetchCommittedFactory(ctx context.Context, factory daNttFactory, wantTag string) (decodedEntry[daCoinFactory], error) {
	if err := requireFactoryTag(factory, wantTag); err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	factoryEntry, found, err := s.acs.ActiveContractByID(ctx, s.readersFor(tailCoinFactory), factory.Value)
	if err != nil {
		return decodedEntry[daCoinFactory]{}, newFlowError(upstreamErrorStatus(err), "read committed factory %s: %v", factory.Value, err)
	}
	if !found {
		return decodedEntry[daCoinFactory]{}, newFlowError(http.StatusInternalServerError, "no CoinFactory found for committed factory contractId %s", factory.Value)
	}
	if !templateIDMatches(factoryEntry.TemplateID, flowTemplateByTail[tailCoinFactory]) {
		return decodedEntry[daCoinFactory]{}, newFlowError(http.StatusInternalServerError, "committed factory %s has template %s, expected a CoinFactory", factory.Value, factoryEntry.TemplateID)
	}
	var coinFactory daCoinFactory
	if err := json.Unmarshal(factoryEntry.CreateArgument, &coinFactory); err != nil {
		return decodedEntry[daCoinFactory]{}, newFlowError(http.StatusInternalServerError, "decode committed factory %s createArgument: %v", factory.Value, err)
	}
	return decodedEntry[daCoinFactory]{entry: factoryEntry, value: coinFactory}, nil
}

// fetchCommittedCanonicalCoinFactory resolves a burn/mint deployment's committed factory and
// checks that gg administers it.
func (s *server) fetchCommittedCanonicalCoinFactory(ctx context.Context, mgr daNttManager, offset int64) (decodedEntry[daCoinFactory], error) {
	factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.Factory, factoryTagBurnMint)
	if err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	if err := checkCoinFactoryAdminIsGuardianGovernance(factoryEntry, mgr.GuardianGovernance); err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	return factoryEntry, nil
}

// appendTransferPreapprovalOrMissing adds owner's TransferPreapproval for the deployment's instrument
// under role, or names role as missing when the disclosing parties cannot see it.
func (s *server) appendTransferPreapprovalOrMissing(ctx context.Context, owner string, instrumentID daInstrumentId, role string, offset int64, disclosures []flowDisclosureEntry, missing []flowMissingEntry) ([]flowDisclosureEntry, []flowMissingEntry, error) {
	preapprovals, err := fetchDecoded[daPreapproval](ctx, s, tailTransferPreapproval, offset, preapprovalFieldMatches(owner, instrumentID)...)
	if err != nil {
		return nil, nil, err
	}
	pre, found, err := findPreapproval(preapprovals, owner, instrumentID, role)
	if err != nil {
		return nil, nil, err
	}
	if found {
		return append(disclosures, discloseAs(role, pre.entry)), missing, nil
	}
	return disclosures, append(missing, missingEntry(role)), nil
}

// isCustodyPotHoldingTemplate reports whether templateID is a template a custody pot can have.
func isCustodyPotHoldingTemplate(templateID string) bool {
	return templateIDMatches(templateID, flowTemplateByTail[tailCoin])
}

// fetchPotHolding reads the custody pot by the contract id its LockedLedger names. The read is
// not pinned to the request offset; an archived pot gives a failed submission and a retry.
func (s *server) fetchPotHolding(ctx context.Context, potContractID string) (acsEntry, bool, error) {
	pot, found, err := s.acs.ActiveContractByID(ctx, s.readersFor(tailCoin), potContractID)
	if err != nil {
		return acsEntry{}, false, newFlowError(upstreamErrorStatus(err), "read custody pot %s: %v", potContractID, err)
	}
	if !found {
		return acsEntry{}, false, nil
	}
	//SECURITY: the ledger names the pot; its template must be a custody holding template.
	if !isCustodyPotHoldingTemplate(pot.TemplateID) {
		return acsEntry{}, false, newFlowError(http.StatusBadGateway, "custody pot %s has template %s, expected a custody holding", potContractID, pot.TemplateID)
	}
	return pot, true, nil
}

// preapprovalFieldMatches selects the DepositPreapproval or TransferPreapproval of owner for instrumentID.
func preapprovalFieldMatches(owner string, instrumentID daInstrumentId) []createArgumentFieldMatch {
	return []createArgumentFieldMatch{
		exactFieldMatch(owner, "owner"),
		exactFieldMatch(instrumentID.Admin, "instrumentId", "admin"),
		exactFieldMatch(instrumentID.Id, "instrumentId", "id"),
	}
}

// fetchTransceiverEmitter selects the manager's transceiver Emitter, owned by gg.
func (s *server) fetchTransceiverEmitter(ctx context.Context, manager daNttManager, offset int64) (decodedEntry[daEmitter], error) {
	emitters, err := fetchDecoded[daEmitter](ctx, s, tailEmitter, offset,
		exactFieldMatch(manager.GuardianGovernance, "owner"),
		exactFieldMatch(manager.TransceiverEmitterId.String(), "emitterId"))
	if err != nil {
		return decodedEntry[daEmitter]{}, err
	}
	return findTransceiverEmitter(emitters, manager.TransceiverEmitterId, manager.GuardianGovernance)
}

// --- Per-flow assemblers ---

// assembleRelease mirrors releaseDisclosures.
func (s *server) assembleRelease(ctx context.Context, request releaseRequest, offset int64) (*flowResponse, error) {
	managerHex, digest, recipient := request.manager.hex, request.digest.hex, request.recipient

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
	factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer)
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
		pot, found, err := s.fetchPotHolding(ctx, *ledger.value.CustodyHoldingCid)
		if err != nil {
			return nil, err
		}
		if found {
			disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot))
		} else {
			missing = append(missing, missingEntry(roleCustodyPotHolding))
		}
	}

	disclosures = append(disclosures, discloseAs(roleCommittedTransferFactory, factoryEntry.entry))

	if recipient != "" {
		disclosures, missing, err = s.appendTransferPreapprovalOrMissing(ctx, recipient, mgr.value.InstrumentId, roleRecipientTransferPreapproval, offset, disclosures, missing)
		if err != nil {
			return nil, err
		}
	}

	return &flowResponse{Flow: "release", Disclosures: disclosures, Missing: missing}, nil
}

// assembleMint mirrors mintDisclosures.
func (s *server) assembleMint(ctx context.Context, request mintRequest, offset int64) (*flowResponse, error) {
	managerHex, digest, recipient := request.manager.hex, request.digest.hex, request.recipient

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

	deposits, err := fetchDecoded[daPreapproval](ctx, s, tailDepositPreapproval, offset, preapprovalFieldMatches(recipient, mgr.value.InstrumentId)...)
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

	factoryEntry, err := s.fetchCommittedCanonicalCoinFactory(ctx, mgr.value, offset)
	if err != nil {
		return nil, err
	}

	disclosures := []flowDisclosureEntry{
		discloseAs(roleNttManager, mgr.entry),
		discloseAs(roleCoreState, cs.entry),
		discloseAs(roleCoveringReplayNode, node.entry),
		discloseAs(roleDepositPreapproval, dep.entry),
		discloseAs(roleCommittedCanonicalCoinFactory, factoryEntry.entry),
	}
	return &flowResponse{Flow: "mint", Disclosures: disclosures, Missing: make([]flowMissingEntry, 0)}, nil
}

// assembleSetPeer mirrors setPeerByVaaDisclosures.
func (s *server) assembleSetPeer(ctx context.Context, request setPeerRequest, offset int64) (*flowResponse, error) {
	managerHex, digest := request.manager.hex, request.digest.hex

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	cs, err := s.fetchCoreState(ctx, mgr.value.GuardianGovernance, offset)
	if err != nil {
		return nil, err
	}
	emitter, err := s.fetchTransceiverEmitter(ctx, mgr.value, offset)
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
func (s *server) assembleAcceptAdmin(ctx context.Context, request acceptAdminRequest, offset int64) (*flowResponse, error) {
	managerHex, digest := request.manager.hex, request.digest.hex

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	if mgr.value.Admin == mgr.value.GuardianGovernance {
		return nil, newFlowError(http.StatusConflict, "flow accept-admin: the admin is already guardianGovernance")
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

	proposals, err := fetchDecoded[daAdminTransferProposal](ctx, s, tailAdminTransferProposal, offset,
		exactFieldMatch(mgr.value.Admin, "admin"),
		exactFieldMatch(mgr.value.GuardianGovernance, "newAdmin"),
		hexFieldMatch(mgr.value.ManagerAddress, "managerAddress"))
	if err != nil {
		return nil, err
	}
	prop, found, err := findAdminTransferProposal(proposals, mgr.value)
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
			pot, found, err := s.fetchPotHolding(ctx, *ledger.value.CustodyHoldingCid)
			if err != nil {
				return nil, err
			}
			if found {
				disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot))
			} else {
				missing = append(missing, missingEntry(roleCustodyPotHolding))
			}
			factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer)
			if err != nil {
				return nil, err
			}
			disclosures = append(disclosures, discloseAs(roleCommittedTransferFactory, factoryEntry.entry))

			// AcceptAdminTransferByVaa requires newAdmin == gg, so the pot moves to nttCustody.
			newCustodian := reserveCustodianFor(mgr.value.GuardianGovernance, mgr.value.NttCustody, mgr.value.GuardianGovernance)
			disclosures, missing, err = s.appendTransferPreapprovalOrMissing(ctx, newCustodian, mgr.value.InstrumentId, roleNewCustodianTransferPreapproval, offset, disclosures, missing)
			if err != nil {
				return nil, err
			}
		}

	case tokenConfigBurnMint:
		// The base disclosures (manager, core state, covering node, proposal) are the whole set.

	default:
		return nil, newFlowError(http.StatusInternalServerError, "NttManager has unknown tokenConfig %q", mgr.value.TokenConfig)
	}

	return &flowResponse{Flow: "accept-admin", Disclosures: disclosures, Missing: missing}, nil
}

// assembleTransfer mirrors transferDisclosures.
func (s *server) assembleTransfer(ctx context.Context, request transferRequest, offset int64) (*flowResponse, error) {
	managerHex := request.manager.hex

	mgr, err := s.fetchManager(ctx, managerHex, offset)
	if err != nil {
		return nil, err
	}
	emitter, err := s.fetchTransceiverEmitter(ctx, mgr.value, offset)
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
			pot, found, err := s.fetchPotHolding(ctx, *ledger.value.CustodyHoldingCid)
			if err != nil {
				return nil, err
			}
			if found {
				disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot))
			} else {
				missing = append(missing, missingEntry(roleCustodyPotHolding))
			}
		}

		factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, discloseAs(roleCommittedTransferFactory, factoryEntry.entry))

		disclosures, missing, err = s.appendTransferPreapprovalOrMissing(ctx, mgr.value.ReserveCustodian, mgr.value.InstrumentId, roleCustodianTransferPreapproval, offset, disclosures, missing)
		if err != nil {
			return nil, err
		}

	case tokenConfigBurnMint:
		factoryEntry, err := s.fetchCommittedCanonicalCoinFactory(ctx, mgr.value, offset)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, discloseAs(roleCommittedCanonicalCoinFactory, factoryEntry.entry))

	default:
		return nil, newFlowError(http.StatusInternalServerError, "NttManager has unknown tokenConfig %q", mgr.value.TokenConfig)
	}

	return &flowResponse{Flow: "transfer", Disclosures: disclosures, Missing: missing}, nil
}

// assembleRegister mirrors registerDisclosures.
func (s *server) assembleRegister(ctx context.Context, request registerRequest, offset int64) (*flowResponse, error) {
	gg, byVaa := request.guardianGovernance, request.byVAA

	gov, err := s.fetchGuardianAnchor(ctx, tailNttGovernance, gg, roleNttGovernance, offset)
	if err != nil {
		return nil, err
	}
	emRegEntry, err := s.fetchGuardianAnchor(ctx, tailEmitterRegistry, gg, roleEmitterRegistry, offset)
	if err != nil {
		return nil, err
	}
	rrRegEntry, err := s.fetchGuardianAnchor(ctx, tailReplayRootRegistry, gg, roleReplayRootRegistry, offset)
	if err != nil {
		return nil, err
	}
	cs, err := s.fetchCoreState(ctx, gg, offset)
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
		factories, err := fetchDecoded[daCoinFactory](ctx, s, tailCoinFactory, offset, exactFieldMatch(gg, "admin"))
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

// assembleConsolidate mirrors consolidateDisclosures. The caller supplies the reserve custodian's
// holdings as choice arguments; this set covers only the manager, ledger, and factory.
func (s *server) assembleConsolidate(ctx context.Context, request consolidateRequest, offset int64) (*flowResponse, error) {
	managerHex := request.manager.hex

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
	factoryEntry, err := s.fetchCommittedFactory(ctx, mgr.value.Factory, factoryTagTransfer)
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

// checkFlowEnabled refuses register without --operator-party, and a register gg other than the
// configured guardianGovernance party.
func (s *server) checkFlowEnabled(request flowRequest) error {
	register, isRegister := request.(registerRequest)
	if !isRegister {
		return nil
	}
	if s.opts.readerParties.operator == "" {
		return newFlowError(http.StatusNotFound, "flow register not enabled: the service has no operator party")
	}
	if register.guardianGovernance != "" && partyID(register.guardianGovernance) != s.opts.readerParties.guardianGovernance {
		return newFlowError(http.StatusBadRequest, "gg must be the service's guardianGovernance party")
	}
	return nil
}

// handleFlows serves GET /v1/flows/{flow}. It parses the query before any upstream call, resolves
// one LedgerEnd offset, then delegates to the flow's assembler.
func (s *server) handleFlows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request, err := parseFlowRequest(r.PathValue("flow"), r.URL.RawQuery)
	if err != nil {
		s.writeFlowError(w, r, err)
		return
	}
	if err := s.checkFlowEnabled(request); err != nil {
		s.writeFlowError(w, r, err)
		return
	}

	offset, err := s.acs.LedgerEnd(r.Context())
	if err != nil {
		s.writePublicError(w, r, upstreamErrorStatus(err), fmt.Errorf("ledger-end: %w", err))
		return
	}

	var resp *flowResponse
	switch typedRequest := request.(type) {
	case releaseRequest:
		resp, err = s.assembleRelease(r.Context(), typedRequest, offset)
	case mintRequest:
		resp, err = s.assembleMint(r.Context(), typedRequest, offset)
	case setPeerRequest:
		resp, err = s.assembleSetPeer(r.Context(), typedRequest, offset)
	case acceptAdminRequest:
		resp, err = s.assembleAcceptAdmin(r.Context(), typedRequest, offset)
	case transferRequest:
		resp, err = s.assembleTransfer(r.Context(), typedRequest, offset)
	case registerRequest:
		resp, err = s.assembleRegister(r.Context(), typedRequest, offset)
	case consolidateRequest:
		resp, err = s.assembleConsolidate(r.Context(), typedRequest, offset)
	default:
		err = newFlowError(http.StatusInternalServerError, "unhandled flow request %T", request)
	}
	if err != nil {
		s.writeFlowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------

// managerEntry is one visible NTT deployment, over the wire.
type managerEntry struct {
	ManagerAddress   string         `json:"managerAddress"`
	ContractID       string         `json:"contractId"`
	ChainId          json.Number    `json:"chainId"`
	ManagerId        json.Number    `json:"managerId"`
	TokenConfig      string         `json:"tokenConfig"`
	InstrumentId     daInstrumentId `json:"instrumentId"`
	ReserveCustodian string         `json:"reserveCustodian"`
	Paused           bool           `json:"paused"`
}

// handleManagers serves GET /v1/managers: every NttManager the disclosing parties see. Clients
// read the managerAddress here to fill the flow endpoints' manager param.
func (s *server) handleManagers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "disclosure-service: method not allowed", http.StatusMethodNotAllowed)
		return
	}
	offset, err := s.acs.LedgerEnd(r.Context())
	if err != nil {
		s.writePublicError(w, r, upstreamErrorStatus(err), fmt.Errorf("ledger-end: %w", err))
		return
	}
	managersQuery, err := s.flowQuery(tailNttManager, offset)
	if err != nil {
		s.writeFlowError(w, r, err)
		return
	}
	managersQuery.maxMatches = s.maxContracts
	mgrs, err := fetchDecodedQuery[daNttManager](r.Context(), s, managersQuery)
	if err != nil {
		s.writeFlowError(w, r, err)
		return
	}
	out := make([]managerEntry, 0, len(mgrs))
	for _, m := range mgrs {
		out = append(out, managerEntry{
			ManagerAddress:   m.value.ManagerAddress,
			ContractID:       m.entry.ContractID,
			ChainId:          m.value.ChainId,
			ManagerId:        m.value.ManagerId,
			TokenConfig:      m.value.TokenConfig,
			InstrumentId:     m.value.InstrumentId,
			ReserveCustodian: m.value.ReserveCustodian,
			Paused:           m.value.Paused,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ManagerAddress < out[j].ManagerAddress })
	writeJSON(w, http.StatusOK, map[string]any{"managers": out})
}
