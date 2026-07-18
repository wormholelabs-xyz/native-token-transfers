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

// grantActAs grants the script user actAs rights on parties, on profiles with auth
// enabled (LocalNet). Allocating a party does NOT confer submission rights on the
// allocating user there -- without this grant the first submit as the new party fails
// with PERMISSION_DENIED (verified against a live 0.6.12 stack). No-op on the sandbox,
// whose Ledger API runs without auth.
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
