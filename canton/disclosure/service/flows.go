package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

// Role labels: one per set member, served to clients on each disclosure entry.
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

// parseManagerDigest parses the two params every VAA-relay flow takes.
func parseManagerDigest(q url.Values) (manager, digest string, err error) {
	manager, err = requireHex64Param(q, "manager")
	if err != nil {
		return "", "", err
	}
	digest, err = requireHex64Param(q, "digest")
	if err != nil {
		return "", "", err
	}
	return manager, digest, nil
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
	entries, err := s.fetchEntries(ctx, tail, offset)
	if err != nil {
		return nil, err
	}
	out := make([]decodedEntry[T], 0, len(entries))
	for _, e := range entries {
		var v T
		if err := json.Unmarshal(e.CreateArgument, &v); err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "decode %s createArgument for contract %s: %v", tail, e.ContractID, err)
		}
		out = append(out, decodedEntry[T]{entry: e, value: v})
	}
	return out, nil
}

// fetchEntries resolves tail to its allow-listed template name, queries ActiveContracts at
// offset, and enforces the per-template contract cap.
func (s *server) fetchEntries(ctx context.Context, tail string, offset int64) ([]acsEntry, error) {
	canonical, ok := s.allowListByTail[tail]
	if !ok {
		return nil, newFlowError(http.StatusInternalServerError, "template %q is not in the allow-list", tail)
	}
	entries, err := s.acs.ActiveContracts(ctx, s.opts.disclosingParties, canonical, offset)
	if err != nil {
		return nil, newFlowError(upstreamErrorStatus(err), "query %s: %v", canonical, err)
	}
	if msg := s.contractCapError(canonical, len(entries)); msg != "" {
		return nil, newFlowError(http.StatusBadGateway, "%s", msg)
	}
	return entries, nil
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

func (s *server) fetchPotHolding(ctx context.Context, cid string, offset int64) (acsEntry, bool, error) {
	entries, err := s.fetchEntries(ctx, tailCoin, offset)
	if err != nil {
		return acsEntry{}, false, err
	}
	for _, e := range entries {
		if e.ContractID == cid {
			return e, true, nil
		}
	}
	return acsEntry{}, false, nil
}

// --- Per-flow assemblers ---

// assembleRelease mirrors releaseDisclosures.
func (s *server) assembleRelease(ctx context.Context, q url.Values, offset int64) (*flowResponse, error) {
	managerHex, digest, err := parseManagerDigest(q)
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
			disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot))
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
	managerHex, digest, err := parseManagerDigest(q)
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
	managerHex, digest, err := parseManagerDigest(q)
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
	managerHex, digest, err := parseManagerDigest(q)
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
				disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot))
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
				disclosures = append(disclosures, discloseAs(roleCustodyPotHolding, pot))
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
		http.Error(w, fmt.Sprintf("disclosure-service: ledger-end: %v", err), upstreamErrorStatus(err))
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
