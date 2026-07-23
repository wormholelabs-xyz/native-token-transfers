// Package state persists the playground's stable identities -- parties, the guardian key,
// and per-deployment addresses/peers -- across CLI invocations. It never stores a contract
// id: CoreState, NttManager, and Emitter all churn on every consuming exercise, so every
// `dpm script` entrypoint re-resolves them from the ACS by identity instead (see
// canton/test/daml/Playground/Types.daml's resolvers). This file is the only thing that
// makes the CLI feel like a long-running session rather than a pile of one-shot scripts.
package state

import (
	"encoding/json"
	"fmt"
	"os"
)

// DefaultFileName is the state file the CLI reads/writes in the current directory unless
// overridden by --state-file.
const DefaultFileName = "playground.state.json"

// Peer is a configured remote-chain peer for one deployment (Wormhole.Ntt.Manager.Peer,
// mirrored 1:1: both fields are 32-byte hex).
type Peer struct {
	ManagerAddress     string `json:"managerAddress"`
	TransceiverAddress string `json:"transceiverAddress"`
}

// Deployment is one NTT deployment's stable identity: everything `transfer`/`receive`/
// `guardian sign-transfer` need without touching the ledger.
type Deployment struct {
	Name               string       `json:"name"`
	ManagerID          int          `json:"managerId"`
	ManagerAddress     string       `json:"managerAddress"`
	TransceiverAddress string       `json:"transceiverAddress"`
	Admin              string       `json:"admin"` // full party id
	Mode               string       `json:"mode"`  // "burn-mint" | "lock-unlock"
	TokenKind          string       `json:"tokenKind"` // "mock" | "amulet"
	TokenDecimals      int          `json:"tokenDecimals"`
	Peers              map[int]Peer `json:"peers"` // keyed by chain id

	// InstrumentAdmin/InstrumentID are the deployment's bridged CIP-56 instrument identity
	// (Splice.Api.Token.HoldingV1.InstrumentId): InstrumentAdmin is gg for "mock", the real
	// DSO party for "amulet"; InstrumentID is the instrument's text id (the
	// Wormhole.Ntt.Manager.nttInstrumentIdFor-bound hash for burn/mint "mock", the shared
	// "NTT" text for lock/unlock "mock", "Amulet" for "amulet"). There is no separate
	// custody party any more: lock/unlock custody is admin-owned (see
	// Wormhole.Ntt.Manager's header) -- Admin above doubles as the custody party.
	InstrumentAdmin string `json:"instrumentAdmin,omitempty"`
	InstrumentID    string `json:"instrumentId,omitempty"`
}

// legacyStateMarkers are JSON keys that only ever appeared in a pre-CIP-56-rework state
// file (the old 4-value tokenKind scheme's custody-party fields). Their presence means the
// file predates this rework and must be rejected with a migration hint rather than silently
// (and incorrectly) loaded -- CustodyParty/CustodyUser no longer exist on Deployment, so a
// naive json.Unmarshal would just silently drop them, masking a stale-state bug as an
// empty-field one.
var legacyDeploymentMarkers = []string{"custodyParty", "custodyUser"}

// legacyTokenKinds are the pre-CIP-56-rework 4-value tokenKind strings, no longer valid.
var legacyTokenKinds = map[string]bool{
	"mock-admin-signed":    true,
	"cip56-burn-mint-mock": true,
	"cip56-custody-mock":   true,
	"cip56-custody":        true,
}

// Emitter is one standalone core-bridge emitter's stable identity (`emitter register`):
// the registry-allocated emitterId, the derived 32-byte hex address, and the owning party.
// The Emitter contract itself churns on every publish (its sequence counter bumps), so --
// per the package rule -- its cid is never stored; `publish` re-resolves it by emitterId.
type Emitter struct {
	EmitterID int    `json:"emitterId"`
	Address   string `json:"address"`
	Owner     string `json:"owner"` // full party id
}

// Guardian is the playground's 1/1 guardian key. PrivateKeyHex is a devnet-only convenience
// (the whole point of a local playground is not needing an external signer); a real
// deployment would never persist a guardian key like this -- see the CLI README's MainNet
// path.
type Guardian struct {
	PrivateKeyHex string `json:"privateKeyHex"`
	Address       string `json:"address"` // 20-byte hex
}

// State is the full contents of playground.state.json.
type State struct {
	Profile            string                `json:"profile"`
	Operator           string                `json:"operator"`
	GuardianGovernance string                `json:"guardianGovernance"`
	GuardianObserver   string                `json:"guardianObserver"`
	Guardian           Guardian              `json:"guardian"`
	Deployments        map[string]Deployment `json:"deployments"`

	// Emitters maps CLI-local names (`emitter register --name`) to standalone emitters,
	// so `publish --emitter NAME` can re-resolve the live contract by stable identity.
	Emitters map[string]Emitter `json:"emitters"`

	// Users maps CLI-facing party hints (e.g. "Alice") to the full party id
	// `Playground.Ops:allocatePlaygroundParty` allocated for that hint, so a hint always
	// resolves to the same party across separate CLI invocations (see resolveParty in
	// cmd/ntt-playground/party.go).
	Users map[string]string `json:"users"`

	// UserParticipants maps a CLI-facing party hint (e.g. "Alice") to the participant role
	// (internal/profile.Profile.Participants key, e.g. "app-user") it was allocated on, so
	// routing stays stable across invocations even if the disclosure/topology config's
	// partyHosting map later changes. Populated at allocation time; consulted BEFORE the
	// config's partyHosting map and its "*" fallback (see the plan's §3 precedence:
	// state.UserParticipants > config partyHosting > "*").
	//
	// TODO(phase 3): populate this in resolveParty (cmd/ntt-playground/party.go) when
	// allocating a party against a routed participant; nothing writes it yet.
	UserParticipants map[string]string `json:"userParticipants,omitempty"`

	// GuardianSequences tracks the next VAA sequence number this CLI hands out per
	// (kind, emitterChain) pair -- e.g. "transfer:2" -- so repeated `guardian sign-transfer`
	// calls never collide digests (Wormhole.Core.Replay's trie rejects exact repeats, and a
	// stable digest would look like a replay attempt rather than a fresh transfer).
	GuardianSequences map[string]uint64 `json:"guardianSequences"`
}

// New returns an empty State ready to be populated by `init`.
func New() *State {
	return &State{
		Deployments:       map[string]Deployment{},
		Emitters:          map[string]Emitter{},
		GuardianSequences: map[string]uint64{},
		Users:             map[string]string{},
		UserParticipants:  map[string]string{},
	}
}

// Load reads and parses the state file at path.
func Load(path string) (*State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	if err := rejectLegacyFormat(raw); err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	if s.Deployments == nil {
		s.Deployments = map[string]Deployment{}
	}
	if s.Emitters == nil {
		s.Emitters = map[string]Emitter{}
	}
	if s.GuardianSequences == nil {
		s.GuardianSequences = map[string]uint64{}
	}
	if s.Users == nil {
		s.Users = map[string]string{}
	}
	if s.UserParticipants == nil {
		s.UserParticipants = map[string]string{}
	}
	return &s, nil
}

// rejectLegacyFormat scans raw for the pre-CIP-56-rework state shape -- the old 4-value
// tokenKind scheme's custody-party fields, or one of its tokenKind strings -- and returns a
// clear migration-hinting error if found. Without this check, the current Deployment struct
// would silently drop custodyParty/custodyUser (unknown fields to json.Unmarshal) and accept
// a now-meaningless tokenKind, turning a stale state file into confusing downstream failures
// instead of a loud, actionable one at load time. Malformed JSON is left to the caller's real
// json.Unmarshal, which reports a clearer parse error.
func rejectLegacyFormat(raw []byte) error {
	var probe struct {
		Deployments map[string]map[string]json.RawMessage `json:"deployments"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	for name, d := range probe.Deployments {
		for _, marker := range legacyDeploymentMarkers {
			if _, ok := d[marker]; ok {
				return fmt.Errorf(
					"state: deployment %q looks like a pre-CIP-56-rework state file (found %q) -- "+
						"this format is no longer supported; re-run `init`/`deploy` against the current CLI",
					name, marker)
			}
		}
		if rawKind, ok := d["tokenKind"]; ok {
			var kind string
			if err := json.Unmarshal(rawKind, &kind); err == nil && legacyTokenKinds[kind] {
				return fmt.Errorf(
					"state: deployment %q has tokenKind %q, removed in the CIP-56 rework -- "+
						"this state file predates the current CLI; re-run `init`/`deploy` (valid kinds: \"mock\", \"amulet\")",
					name, kind)
			}
		}
	}
	return nil
}

// Save writes the state file at path, pretty-printed for easy inspection/diffing.
func (s *State) Save(path string) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("state: write %s: %w", path, err)
	}
	return nil
}

// NextGuardianSequence returns the next sequence number for kind (e.g. "transfer") on
// emitterChain, and persists the increment. Starts at 1 (0 is reserved so a caller that
// forgets to call this and leaves the field zero-valued fails loudly rather than silently
// colliding with a real sequence).
func (s *State) NextGuardianSequence(kind string, emitterChain uint16) uint64 {
	key := fmt.Sprintf("%s:%d", kind, emitterChain)
	s.GuardianSequences[key]++
	return s.GuardianSequences[key]
}

// Deployment looks up a deployment by name, returning ok=false if it doesn't exist.
func (s *State) Deployment(name string) (Deployment, bool) {
	d, ok := s.Deployments[name]
	return d, ok
}

// Emitter looks up a standalone emitter by its CLI-local name, returning ok=false if it
// doesn't exist.
func (s *State) Emitter(name string) (Emitter, bool) {
	e, ok := s.Emitters[name]
	return e, ok
}

// Peer looks up a deployment's peer for chain, returning ok=false if none is configured.
func (d Deployment) Peer(chain int) (Peer, bool) {
	p, ok := d.Peers[chain]
	return p, ok
}
