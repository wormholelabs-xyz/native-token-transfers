package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

type allocatePartyInput struct {
	Hint string `json:"hint"`
}

type allocatePartyOutput struct {
	Party string `json:"party"`
}

// resolveParty maps a CLI-facing party hint (e.g. "Alice") to a full party id, allocating it
// via Playground.Ops:allocatePlaygroundParty on first use and remembering the result in the
// state file's Users map thereafter -- so a hint always resolves to the SAME party across
// separate CLI invocations. Deployment admins are resolved without a ledger round-trip since
// `deploy` already persisted them. Takes the command (rather than a bare context) so cache
// hits and fresh allocations can be narrated in verbose mode.
//
// Routing (plan §3): a fresh allocation resolves hint's participant role via
// resolveParticipantRole (state.UserParticipants > topology config's partyHosting > "*"
// wildcard > the profile's own default), allocates the party THERE (not always app-provider),
// grants actAs on that same participant's JSON API, and persists the resolved role into
// s.UserParticipants so later invocations stay pinned to it even if the topology config
// changes.
func resolveParty(cmd *cobra.Command, a *app, s *state.State, hint string) (string, error) {
	ctx := cmd.Context()
	if hint == "" {
		return "", fmt.Errorf("party hint must not be empty")
	}
	if s.Users == nil {
		s.Users = map[string]string{}
	}
	if party, ok := s.Users[hint]; ok {
		a.vlogf(cmd, "party %q: cached → %s", hint, party)
		return party, nil
	}

	prof, err := a.resolvedProfile()
	if err != nil {
		return "", err
	}
	cfg, err := a.topologyConfig()
	if err != nil {
		return "", err
	}
	role := resolveParticipantRole(hint, cfg, s)
	if role == "" {
		role = prof.DefaultParticipant
	}

	a.vlogf(cmd, "party %q: allocating fresh party on participant=%s (Playground.Ops:allocatePlaygroundParty)", hint, role)
	runner, cleanup, err := a.newScriptRunnerFor(ctx, role)
	if err != nil {
		return "", err
	}
	defer cleanup()

	var out allocatePartyOutput
	if err := runner.Run(ctx, "Playground.Ops:allocatePlaygroundParty", allocatePartyInput{Hint: hint}, &out); err != nil {
		return "", fmt.Errorf("resolveParty(%s): %w", hint, err)
	}
	a.vlogf(cmd, "allocated party %q → %s", hint, out.Party)
	if err := grantActAs(cmd, a, role, out.Party); err != nil {
		return "", fmt.Errorf("resolveParty(%s): %w", hint, err)
	}
	s.Users[hint] = out.Party
	if s.UserParticipants == nil {
		s.UserParticipants = map[string]string{}
	}
	s.UserParticipants[hint] = role
	if err := a.saveState(s); err != nil {
		return "", err
	}
	return out.Party, nil
}

// participantRoleForParty resolves the participant role a full party id was allocated on, by
// reverse-mapping it back to its CLI hint (knownHints) and looking that hint up in
// s.UserParticipants. Returns "" (meaning "the profile's default participant") when party has
// no known hint, or the hint predates routing (no recorded UserParticipants entry) -- e.g. a
// cip56-custody wallet party (onboarded via internal/amulet, never through resolveParty) --
// which is exactly the today's-behavior fallback those parties need (plan §3: cip56-custody
// wallet senders stay app-provider wallet users, unchanged).
func participantRoleForParty(s *state.State, party string) string {
	hint, ok := knownHints(s)[party]
	if !ok {
		return ""
	}
	return s.UserParticipants[hint]
}

// resolveParticipantRole resolves a CLI-facing party hint (e.g. "Alice") to the participant
// role (internal/profile.Profile.Participants key) it should be routed to, per the plan's §3
// precedence:
//
//  1. s.UserParticipants[hint] -- the hint's participant, if this state file already
//     allocated it there (keeps routing stable across invocations even if cfg changes).
//  2. cfg.PartyHosting[hint] -- the disclosure/topology config's explicit mapping.
//  3. cfg.PartyHosting["*"] -- the config's wildcard fallback.
//  4. "" -- nothing resolved; callers pass this straight to profile.Profile.Endpoint(""),
//     which already falls back to the profile's own DefaultParticipant.
func resolveParticipantRole(hint string, cfg disclosure.Config, s *state.State) string {
	if s != nil && s.UserParticipants != nil {
		if role, ok := s.UserParticipants[hint]; ok && role != "" {
			return role
		}
	}
	if role, ok := cfg.PartyHosting[hint]; ok && role != "" {
		return role
	}
	if role, ok := cfg.PartyHosting["*"]; ok && role != "" {
		return role
	}
	return ""
}

// partyInfo/listPartiesOutput mirror Playground.Query.daml's PartyInfo/ListPartiesOutput.
type partyInfo struct {
	Party   string `json:"party"`
	IsLocal bool   `json:"isLocal"`
}

type listPartiesOutput struct {
	Parties []partyInfo `json:"parties"`
}

func newPartyCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "party",
		Short: "List and allocate parties on the playground's participant",
	}
	cmd.AddCommand(newPartyListCmd(a), newPartyAllocateCmd(a))
	return cmd
}

// sortedParticipantRoles returns prof.Participants' keys in a deterministic (sorted) order,
// so `party list`'s per-participant output is stable across runs.
func sortedParticipantRoles(prof profile.Profile) []string {
	roles := make([]string, 0, len(prof.Participants))
	for role := range prof.Participants {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func newPartyListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every party each participant knows, prefixed by participant, annotating playground-allocated hints",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			prof, err := a.resolvedProfile()
			if err != nil {
				return err
			}
			hints := knownHints(s)
			for _, role := range sortedParticipantRoles(prof) {
				a.vlogf(cmd, "party list: querying participant=%s's known parties (Playground.Query:listParties)", role)
				runner, cleanup, err := a.newScriptRunnerFor(ctx, role)
				if err != nil {
					return err
				}

				// listParties takes no parameters; Daml's unit decodes from the `{}` an empty
				// struct marshals to, satisfying the uniform --input-file plumbing.
				var out listPartiesOutput
				runErr := runner.Run(ctx, "Playground.Query:listParties", struct{}{}, &out)
				cleanup()
				if runErr != nil {
					return fmt.Errorf("party list: participant=%s: %w", role, runErr)
				}
				for _, p := range out.Parties {
					line := fmt.Sprintf("participant=%s party=%s isLocal=%t", role, p.Party, p.IsLocal)
					if hint, ok := hints[p.Party]; ok {
						line += " hint=" + hint
					}
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}
			return nil
		},
	}
}

func newPartyAllocateCmd(a *app) *cobra.Command {
	var hint string
	cmd := &cobra.Command{
		Use:   "allocate",
		Short: "Allocate a party under a hint (idempotent: an existing hint returns its party)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.loadState()
			if err != nil {
				return err
			}
			a.vlogf(cmd, "party allocate: resolving hint %q (allocates + grants actAs on first use, cached thereafter)", hint)
			party, err := resolveParty(cmd, a, s, hint)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "party=%s\n", party)
			return nil
		},
	}
	cmd.Flags().StringVar(&hint, "hint", "", "party hint (e.g. Eve)")
	_ = cmd.MarkFlagRequired("hint")
	return cmd
}

// knownHints maps full party ids back to their CLI-facing hints for `party list`
// annotation -- on LocalNet, listKnownParties also returns DSO/validator parties, and the
// annotation is what distinguishes playground-owned ones. state.Users covers every party
// resolveParty ever allocated (including the operator/governance/observer trio and
// deployment admins), but the identity fields are also mapped directly, so a state file
// hand-edited to point at pre-existing parties still annotates; Users entries win on
// conflict since they carry the caller's chosen hint.
func knownHints(s *state.State) map[string]string {
	hints := map[string]string{}
	for _, id := range []struct{ hint, party string }{
		{"Operator", s.Operator},
		{"GuardianGovernance", s.GuardianGovernance},
		{"GuardianObserver", s.GuardianObserver},
	} {
		if id.party != "" {
			hints[id.party] = id.hint
		}
	}
	for name, d := range s.Deployments {
		if d.Admin != "" {
			hints[d.Admin] = name + "-admin"
		}
	}
	for hint, party := range s.Users {
		hints[party] = hint
	}
	return hints
}

// grantActAs grants the script user actAs rights on parties, against role's participant JSON
// API (profile.Profile.Endpoint(role) -- "" targets the profile's default participant, e.g.
// app-provider on LocalNet), on profiles with auth enabled (LocalNet). Allocating a party does
// NOT confer submission rights on the allocating user there -- without this grant the first
// submit as the new party fails with PERMISSION_DENIED. No-op on the sandbox, whose Ledger API
// runs without auth.
func grantActAs(cmd *cobra.Command, a *app, role string, parties ...string) error {
	prof, err := a.resolvedProfile()
	if err != nil {
		return err
	}
	if !prof.RequiresAuth {
		a.vlogf(cmd, "sandbox: no auth — actAs grant skipped")
		return nil
	}
	ep, err := prof.Endpoint(role)
	if err != nil {
		return err
	}
	token, err := network.MintUnsafeToken(network.LocalNetAdminUser, time.Hour)
	if err != nil {
		return err
	}
	if err := network.GrantLedgerAPIUserRights(cmd.Context(), ep.JSONAPIBaseURL, token, parties); err != nil {
		return err
	}
	for _, party := range parties {
		a.vlogf(cmd, "party %s: granted actAs to %s (participant=%s JSON API /v2/users/.../rights)", party, network.LocalNetAdminUser, roleOrDefault(role, prof))
	}
	return nil
}

// roleOrDefault renders role for narration, substituting prof.DefaultParticipant when role is
// empty -- purely cosmetic (grantActAs's --verbose narration), matching how
// profile.Profile.Endpoint("") itself resolves.
func roleOrDefault(role string, prof profile.Profile) string {
	if role == "" {
		return prof.DefaultParticipant
	}
	return role
}
