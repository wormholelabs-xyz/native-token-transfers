package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
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

	a.vlogf(cmd, "party %q: allocating fresh party (Playground.Ops:allocatePlaygroundParty)", hint)
	runner, cleanup, err := a.newScriptRunner(ctx)
	if err != nil {
		return "", err
	}
	defer cleanup()

	var out allocatePartyOutput
	if err := runner.Run(ctx, "Playground.Ops:allocatePlaygroundParty", allocatePartyInput{Hint: hint}, &out); err != nil {
		return "", fmt.Errorf("resolveParty(%s): %w", hint, err)
	}
	a.vlogf(cmd, "allocated party %q → %s", hint, out.Party)
	if err := grantActAs(cmd, a, out.Party); err != nil {
		return "", fmt.Errorf("resolveParty(%s): %w", hint, err)
	}
	s.Users[hint] = out.Party
	if err := a.saveState(s); err != nil {
		return "", err
	}
	return out.Party, nil
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

func newPartyListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every party the participant knows, annotating playground-allocated hints",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			a.vlogf(cmd, "party list: querying the participant's known parties (Playground.Query:listParties), annotating hints from state")
			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// listParties takes no parameters; Daml's unit decodes from the `{}` an empty
			// struct marshals to, satisfying the uniform --input-file plumbing.
			var out listPartiesOutput
			if err := runner.Run(ctx, "Playground.Query:listParties", struct{}{}, &out); err != nil {
				return err
			}
			hints := knownHints(s)
			for _, p := range out.Parties {
				line := fmt.Sprintf("party=%s isLocal=%t", p.Party, p.IsLocal)
				if hint, ok := hints[p.Party]; ok {
					line += " hint=" + hint
				}
				fmt.Fprintln(cmd.OutOrStdout(), line)
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

// grantActAs grants the script user actAs rights on parties, on profiles with auth
// enabled (LocalNet). Allocating a party does NOT confer submission rights on the
// allocating user there -- without this grant the first submit as the new party fails
// with PERMISSION_DENIED. No-op on the sandbox, whose Ledger API runs without auth.
func grantActAs(cmd *cobra.Command, a *app, parties ...string) error {
	prof, err := a.resolvedProfile()
	if err != nil {
		return err
	}
	if !prof.RequiresAuth {
		a.vlogf(cmd, "sandbox: no auth — actAs grant skipped")
		return nil
	}
	token, err := network.MintUnsafeToken(network.LocalNetAdminUser, time.Hour)
	if err != nil {
		return err
	}
	if err := network.GrantLedgerAPIUserRights(cmd.Context(), prof.JSONAPIBaseURL, token, parties); err != nil {
		return err
	}
	for _, party := range parties {
		a.vlogf(cmd, "party %s: granted actAs to %s (JSON API /v2/users/.../rights)", party, network.LocalNetAdminUser)
	}
	return nil
}
