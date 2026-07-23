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
	// user-controlled dial the plan's §4 describes. Empty (the zero value, and what a
	// missing config file resolves to -- see Load below) means "disclose nothing": the
	// deliberately-failing default posture.
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

// Load reads and parses the topology config file at path.
//
// Decision (documented per the phase-1 plan, since the plan text left this ambiguous): a
// MISSING file is not an error -- Load returns the built-in defaults (DefaultPartyHosting,
// empty Disclose) so a caller can pass the CLI's default --topology-config path
// unconditionally, whether or not the user ever created one; this matches the plan's "absent
// file ⇒ built-in localnet defaults for partyHosting, empty disclose list" phrasing in §4
// more directly than making every caller special-case a not-found error. A file that DOES
// exist but fails to parse (malformed JSON, or an unknown top-level key -- rejected via
// json.Decoder.DisallowUnknownFields so a typo like "partyHostng" fails loudly instead of
// silently falling back to defaults) still returns an error.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{PartyHosting: DefaultPartyHosting()}, nil
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
