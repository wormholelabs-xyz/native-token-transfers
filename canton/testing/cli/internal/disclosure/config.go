// Package disclosure loads the playground's topology/disclosure config: the
// hint→participant-role hosting map, and the set of Daml templates the CLI is allowed to
// fetch explicit disclosures for before a cross-participant submit. Consulted by
// cmd/ntt-playground/remote.go's prepareRemoteSeam, which intersects a command's needed
// templates with Config.Disclose before a cross-participant submit, fetches payloads via
// Playground/Prepare.daml, and attaches them via Playground/Disclose.daml's RemoteSeam.
package disclosure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Entry pairs one Daml template's qualified name ("Module:Entity", e.g.
// "Wormhole.Ntt.Manager:NttManager") with the party hint whose home participant the CLI
// should fetch that template's disclosure payload from ("fetchAs" -- e.g. "Operator", or the
// literal "admin" placeholder resolved per-deployment to "<name>-admin").
type Entry struct {
	Template string `json:"template"`
	FetchAs  string `json:"fetchAs"`
}

// Config is the parsed contents of the CLI's --topology-config file (see the plan's §4;
// default path is "playground.topology.json" next to the state file).
type Config struct {
	// PartyHosting maps a CLI-facing party hint to the participant role
	// (internal/profile.Profile.Participants key) it should be allocated/routed to. The
	// literal key "*" is the fallback for any hint not listed explicitly.
	PartyHosting map[string]string `json:"partyHosting"`

	// Disclose is the allow-list of templates the CLI is permitted to fetch and attach an
	// explicit disclosure for. A template NOT in this list is simply never fetched, so a
	// cross-participant submit needing it fails on-ledger with CONTRACT_NOT_FOUND -- the
	// user-controlled dial the plan's §4 describes. An EXPLICIT config file that sets this to
	// `[]` (or omits it) genuinely means "disclose nothing" -- see testdata/topology-empty.json,
	// used specifically to pin that failing posture. A MISSING config file is different (see
	// Load's DefaultDisclose below): it is not the same as an explicit empty list.
	Disclose []Entry `json:"disclose"`
}

// DefaultPartyHosting is the built-in localnet hint→participant-role mapping used when no
// --topology-config file is present (see Load) or when a present file omits "partyHosting"
// entirely. Mirrors the plan's §3 built-in default table exactly.
func DefaultPartyHosting() map[string]string {
	return map[string]string{
		"Alice":              "app-user",
		"Bob":                "bob",
		"GuardianGovernance": "guardian-governance",
		"GuardianObserver":   "guardian-observer",
		"*":                  "app-provider",
	}
}

// DefaultDisclose is the built-in localnet disclosure allow-list used ONLY when no
// --topology-config file is present at all (see Load) -- reconciled against the CIP-56/
// namespace-retirement rework (integration plan §8): guardianGovernance (gg) is now a genuine,
// separate participant from operator/admin on LocalNet, and gg co-signs/owns every contract a
// Mock-kind transferOut/receiveVaa needs (NttManager, CoreState, Emitter, ReplayNode, the
// committed factory, and the recipient's preapproval -- see Playground.Prepare's header). Since
// this crossing happens for the DEFAULT executor/sender on every deployment, not just an
// explicitly-routed hint like Bob, an empty default here would make the CLI's basic `deploy`/
// `transfer`/`receive` flow fail out of the box on LocalNet absent a hand-written config file --
// clearly not the intent (the "user-controlled dial" is for DELIBERATELY narrowing disclosure,
// e.g. testdata/topology-empty.json/topology-missing-corestate.json, not for gating the common
// case). An explicit file (even one that just omits "disclose") still means exactly what it
// says -- this default applies only to a config path that does not exist at all.
func DefaultDisclose() []Entry {
	return []Entry{
		{Template: "Wormhole.Ntt.Manager:NttManager", FetchAs: "GuardianGovernance"},
		{Template: "Wormhole.Ntt.Governance:NttGovernance", FetchAs: "Operator"},
		{Template: "Wormhole.Ntt.Ledger:LockedLedger", FetchAs: "GuardianGovernance"},
		{Template: "Wormhole.Core.State:CoreState", FetchAs: "GuardianGovernance"},
		{Template: "Wormhole.Core.State:Emitter", FetchAs: "GuardianGovernance"},
		{Template: "Wormhole.Core.Replay:ReplayNode", FetchAs: "GuardianGovernance"},
		{Template: "Wormhole.Ntt.Deposit:DepositPreapproval", FetchAs: "GuardianGovernance"},
		{Template: "Playground.MockRegistry:MockTransferPreapproval", FetchAs: "GuardianGovernance"},
		{Template: "Playground.MockRegistry:MockPreapprovedTransferFactory", FetchAs: "GuardianGovernance"},
		{Template: "Token.CIP0056.CoinFactory:CoinFactory", FetchAs: "GuardianGovernance"},
		{Template: "Test.TestNtt:Cip56MockHolding", FetchAs: "GuardianGovernance"},
	}
}

// Load reads and parses the topology config file at path.
//
// Decision (documented per the phase-1 plan, since the plan text left this ambiguous): a
// MISSING file is not an error -- Load returns the built-in defaults (DefaultPartyHosting,
// DefaultDisclose) so a caller can pass the CLI's default --topology-config path
// unconditionally, whether or not the user ever created one. This matches the plan's original
// "absent file ⇒ built-in localnet defaults" phrasing in §4 for partyHosting; for Disclose,
// the integration plan's §8 reconciliation revised the DEFAULT itself (see DefaultDisclose's
// doc comment) once gg's own participant became load-bearing for the common case, not just an
// explicitly-routed hint. A file that DOES exist but fails to parse (malformed JSON, or an
// unknown top-level key -- rejected via json.Decoder.DisallowUnknownFields so a typo like
// "partyHostng" fails loudly instead of silently falling back to defaults) still returns an
// error.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{PartyHosting: DefaultPartyHosting(), Disclose: DefaultDisclose()}, nil
		}
		return Config{}, fmt.Errorf("disclosure: read %s: %w", path, err)
	}

	var c Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("disclosure: parse %s: %w", path, err)
	}
	if c.PartyHosting == nil {
		c.PartyHosting = DefaultPartyHosting()
	}
	return c, nil
}

// Entry looks up the Disclose allow-list entry for template (the qualified "Module:Entity"
// name), returning ok=false if it is not configured for disclosure.
func (c Config) Entry(template string) (Entry, bool) {
	for _, e := range c.Disclose {
		if e.Template == template {
			return e, true
		}
	}
	return Entry{}, false
}
