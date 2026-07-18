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
	TokenKind          string       `json:"tokenKind"`
	TokenDecimals      int          `json:"tokenDecimals"`
	Peers              map[int]Peer `json:"peers"` // keyed by chain id
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

	// Users maps CLI-facing party hints (e.g. "Alice") to the full party id
	// `Playground.Ops:allocatePlaygroundParty` allocated for that hint, so a hint always
	// resolves to the same party across separate CLI invocations (see resolveParty in
	// cmd/ntt-playground/party.go).
	Users map[string]string `json:"users"`

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
		GuardianSequences: map[string]uint64{},
		Users:             map[string]string{},
	}
}

// Load reads and parses the state file at path.
func Load(path string) (*State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	if s.Deployments == nil {
		s.Deployments = map[string]Deployment{}
	}
	if s.GuardianSequences == nil {
		s.GuardianSequences = map[string]uint64{}
	}
	if s.Users == nil {
		s.Users = map[string]string{}
	}
	return &s, nil
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

// Peer looks up a deployment's peer for chain, returning ok=false if none is configured.
func (d Deployment) Peer(chain int) (Peer, bool) {
	p, ok := d.Peers[chain]
	return p, ok
}
