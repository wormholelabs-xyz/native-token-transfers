package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// dataOwnerRole is the participant role hosting the playground's own data: the operator, and
// (in this topology) every deployment admin, since neither is ever independently routed by a
// --topology-config's partyHosting map -- only Alice/Bob/GuardianGovernance/GuardianObserver
// ever move off the default participant (see internal/disclosure.DefaultPartyHosting and the
// plan's §3). One participant is therefore always the right fetch target for a `prepare*`
// script, regardless of which specific data (operator-owned or admin-owned) it resolves.
func dataOwnerRole(s *state.State) string {
	return participantRoleForParty(s, s.Operator)
}

// discloseTemplateAllowList returns every template name configured in the topology config's
// disclose list. The config's "fetchAs" hint (see internal/disclosure.Entry) exists for
// documentation/future use -- it names the party a template's disclosure is read as -- but
// this phase's orchestration does not branch on it: the reader identity and participant for
// each template are fixed inside the corresponding Playground.Prepare script (always
// operator/admin, always co-located per dataOwnerRole above), so every configured template
// name is simply passed through as the fetch-side allow-list.
func discloseTemplateAllowList(cfg disclosure.Config) []string {
	names := make([]string, 0, len(cfg.Disclose))
	for _, e := range cfg.Disclose {
		names = append(names, e.Template)
	}
	return names
}

// prepareRemoteSeam decides whether a submit needs a RemoteSeam (the actor's participant
// differs from the data owner's) and, if so, runs prepareScript against the data owner's home
// participant to obtain one -- returned as raw JSON, since the CLI never needs to parse or
// construct a RemoteSeam itself: it flows verbatim from the prepare script's stdout into the
// submit script's stdin (see Playground.Disclose:RemoteSeam's doc comment). Returns (nil, nil)
// when actorRole already matches the data owner's participant -- the existing local fast path,
// byte-identical to pre-phase-5 behavior (a nil json.RawMessage marshals to JSON `null`, i.e.
// Daml's `None`).
//
// buildInput receives the config-derived discloseTemplates allow-list and returns the fully
// populated input value for prepareScript.
func prepareRemoteSeam(ctx context.Context, cmd *cobra.Command, a *app, s *state.State, actorRole, prepareScript string, buildInput func(discloseTemplates []string) any) (json.RawMessage, error) {
	prof, err := a.resolvedProfile()
	if err != nil {
		return nil, err
	}
	owner := dataOwnerRole(s)

	// Normalize BOTH sides to a concrete participant role before comparing: dataOwnerRole is
	// always concrete (participantRoleForParty/resolveParty never persist "" -- see party.go's
	// resolveParty, which substitutes prof.DefaultParticipant before saving), but actorRole
	// often ISN'T -- e.g. receive's default executor (no --executor) and a cip56-custody
	// wallet sender (never routed through resolveParty) both pass "" meaning "the profile's
	// default participant", not a distinct, unrouted participant. Comparing "" against a
	// resolved "app-provider" directly would treat every such default-routed actor as if it
	// were on a DIFFERENT participant than the data owner, wrongly forcing the remote path
	// (and, worse, gating away disclosures the actor never needed fetched at all, since same-
	// participant submits resolve locally via the shared ledger-api-user's actAs/readAs grants
	// regardless of the submitting party's own stakeholder status).
	if actorRole == "" {
		actorRole = prof.DefaultParticipant
	}
	if owner == "" {
		owner = prof.DefaultParticipant
	}
	if actorRole == owner {
		return nil, nil
	}

	cfg, err := a.topologyConfig()
	if err != nil {
		return nil, err
	}
	templates := discloseTemplateAllowList(cfg)

	runner, cleanup, err := a.newScriptRunnerFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	a.vlogf(cmd, "disclosure: actor participant=%q differs from data-owner participant=%q -- fetching a RemoteSeam via %s (discloseTemplates=%v)",
		actorRole, owner, prepareScript, templates)

	var seam json.RawMessage
	if err := runner.Run(ctx, prepareScript, buildInput(templates), &seam); err != nil {
		return nil, fmt.Errorf("prepare remote seam via %s: %w", prepareScript, err)
	}
	return seam, nil
}

// prepareTransferOutInput mirrors Playground.Prepare.daml's PrepareTransferOutInput.
type prepareTransferOutInput struct {
	Operator          string   `json:"operator"`
	ManagerID         int      `json:"managerId"`
	Admin             string   `json:"admin"`
	TokenKind         string   `json:"tokenKind"`
	DiscloseTemplates []string `json:"discloseTemplates"`
}

// preparePreapproveInput mirrors Playground.Prepare.daml's PreparePreapproveInput.
type preparePreapproveInput struct {
	Admin             string   `json:"admin"`
	TokenKind         string   `json:"tokenKind"`
	DiscloseTemplates []string `json:"discloseTemplates"`
}

// prepareReceiveInput mirrors Playground.Prepare.daml's PrepareReceiveInput.
type prepareReceiveInput struct {
	Operator          string   `json:"operator"`
	ManagerID         int      `json:"managerId"`
	Admin             string   `json:"admin"`
	TokenKind         string   `json:"tokenKind"`
	Recipient         string   `json:"recipient"`
	VaaBytes          string   `json:"vaaBytes"`
	DiscloseTemplates []string `json:"discloseTemplates"`
}

// preparePublishInput mirrors Playground.Prepare.daml's PreparePublishInput.
type preparePublishInput struct {
	Operator          string   `json:"operator"`
	EmitterID         int      `json:"emitterId"`
	DiscloseTemplates []string `json:"discloseTemplates"`
}
