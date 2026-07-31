package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// discloseTemplateAllowList returns every template name configured in the topology config's
// disclose list. The config's "fetchAs" hint (see internal/disclosure.Entry) exists for
// documentation -- it names the party a template's disclosure is read as -- but this phase's
// orchestration does not branch on it for PARTICIPANT selection: each `prepare*` script always
// reads as a single fixed party (guardianGovernance for transferOut/receiveVaa, operator for
// publishMessage -- see their call sites and Playground.Prepare's own header for why), so every
// configured template name is simply passed through as the fetch-side allow-list.
func discloseTemplateAllowList(cfg disclosure.Config) []string {
	names := make([]string, 0, len(cfg.Disclose))
	for _, e := range cfg.Disclose {
		names = append(names, e.Template)
	}
	return names
}

// prepareRemoteSeam decides whether a submit needs a RemoteSeam (the actor's participant
// differs from ownerRole, the participant the data this op needs actually lives on) and, if so,
// obtains one -- returned as raw JSON, since the CLI never needs to parse or construct a
// RemoteSeam itself: it flows verbatim from wherever it was fetched into the submit script's
// stdin (see Playground.Disclose:RemoteSeam's doc comment). Returns (nil, nil) when actorRole
// already matches ownerRole -- the existing local fast path, byte-identical to pre-phase-5
// behavior (a nil json.RawMessage marshals to JSON `null`, i.e. Daml's `None`).
//
// Two ways to obtain a seam once the fast path above doesn't apply (design doc §5.3):
//   - a.disclosureServiceURL set: POST /v1/seam/{seamName} against a disclosure service
//     (internal/disclosure.Client/Service) fronting ownerRole's participant. The service
//     injects its OWN discloseTemplates allow-list server-side (design doc §1.4(3)), so
//     buildInput is called with a nil allow-list here -- passing a client-supplied one would be
//     silently overwritten by the service anyway (internal/disclosure/service.go's handleSeam).
//     This is the path that lets an actor submit a cross-participant op WITHOUT holding
//     ownerRole's own credentials, which is what --strict-participant-isolation (runner.go)
//     exists to prove is otherwise required.
//   - unset (default, byte-identical to pre-disclosure-service behavior): run prepareScript
//     directly against ownerRole's participant via a.newScriptRunnerFor(ctx, ownerRole), passing
//     the LOCAL discloseTemplateAllowList (the existing client-side allow-list, unchanged).
//
// ownerRole is caller-supplied rather than derived from a single global helper: which
// participant actually owns the needed data differs per entrypoint post-rework --
// transferOut/receiveVaa need guardianGovernance's participant (gg co-signs/owns everything a
// Mock-kind op touches, including the gg-sole-owned committed factory/preapproval templates
// operator's participant never receives), while publishMessage needs operator's (a standalone
// emitter's owner is not necessarily gg, but operator co-signs every Emitter/CoreState
// regardless of who registered it) -- see each call site.
//
// seamName is the POST /v1/seam/{name} path segment for this entrypoint ("transferOut",
// "receive", or "publish" -- see internal/disclosure/service.go's seamScripts map), used only on
// the disclosure-service path; the local path keeps using prepareScript's Daml script name as
// before.
//
// buildInput receives the config-derived discloseTemplates allow-list (nil on the
// disclosure-service path) and returns the fully populated input value for prepareScript / the
// seam POST body.
func prepareRemoteSeam(ctx context.Context, cmd *cobra.Command, a *app, s *state.State, actorRole, ownerRole, prepareScript, seamName string, buildInput func(discloseTemplates []string) any) (json.RawMessage, error) {
	prof, err := a.resolvedProfile()
	if err != nil {
		return nil, err
	}

	// A profile with a single participant (the sandbox: everything degenerates to "default",
	// plan §2) can never have a genuine actor/data-owner split, no matter what a
	// --topology-config's partyHosting map says (its built-in default table hardcodes
	// LocalNet's five-participant roles unconditionally -- disclosure.DefaultPartyHosting is
	// not profile-aware, e.g. it maps "GuardianGovernance" to the explicit
	// "guardian-governance" role rather than falling through the "*" wildcard the way
	// operator/admin do, which would otherwise wrongly force the remote path here on sandbox).
	// This guard keeps sandbox's fast local path exact regardless of that config table.
	if len(prof.Participants) <= 1 {
		return nil, nil
	}

	// Normalize BOTH sides to a concrete participant role before comparing: ownerRole is
	// usually concrete (participantRoleForParty/resolveParty never persist "" -- see party.go's
	// resolveParty, which substitutes prof.DefaultParticipant before saving), but actorRole
	// often ISN'T -- e.g. receive's default executor (no --executor) and an amulet wallet
	// sender (never routed through resolveParty) both pass "" meaning "the profile's default
	// participant", not a distinct, unrouted participant. Comparing "" against a resolved
	// "app-provider" directly would treat every such default-routed actor as if it were on a
	// DIFFERENT participant than the data owner, wrongly forcing the remote path.
	if actorRole == "" {
		actorRole = prof.DefaultParticipant
	}
	if ownerRole == "" {
		ownerRole = prof.DefaultParticipant
	}
	if actorRole == ownerRole {
		return nil, nil
	}

	// disclosure-service path (design doc §5.3): fetch the RemoteSeam over HTTP instead of
	// running prepareScript locally against ownerRole's own participant. buildInput(nil) is
	// deliberate -- the service is authoritative over which templates get disclosed (its own
	// Config.Disclose, injected server-side), so there is no client-side allow-list to pass;
	// see internal/disclosure/service.go's handleSeam, which overwrites discloseTemplates
	// unconditionally. This is what lets the actor obtain a seam without ever holding
	// ownerRole's participant's own credentials -- the property
	// --strict-participant-isolation's guard (runner.go) otherwise requires.
	if a.disclosureServiceURL != "" {
		cl := &disclosure.Client{BaseURL: a.disclosureServiceURL, Logf: a.verboseLogf()}
		a.vlogf(cmd, "disclosure: fetching a RemoteSeam from the disclosure service at %s (seam=%s)",
			a.disclosureServiceURL, seamName)
		return cl.Seam(ctx, seamName, buildInput(nil))
	}

	cfg, err := a.topologyConfig()
	if err != nil {
		return nil, err
	}
	templates := discloseTemplateAllowList(cfg)

	runner, cleanup, err := a.newScriptRunnerFor(ctx, ownerRole)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	a.vlogf(cmd, "disclosure: actor participant=%q differs from data-owner participant=%q -- fetching a RemoteSeam via %s (discloseTemplates=%v)",
		actorRole, ownerRole, prepareScript, templates)

	var seam json.RawMessage
	if err := runner.Run(ctx, prepareScript, buildInput(templates), &seam); err != nil {
		return nil, fmt.Errorf("prepare remote seam via %s: %w", prepareScript, err)
	}
	return seam, nil
}

// prepareTransferOutInput mirrors Playground.Prepare.daml's PrepareTransferOutInput.
type prepareTransferOutInput struct {
	GuardianGovernance string   `json:"guardianGovernance"`
	ManagerID          int      `json:"managerId"`
	DiscloseTemplates  []string `json:"discloseTemplates"`
}

// prepareReceiveInput mirrors Playground.Prepare.daml's PrepareReceiveInput.
type prepareReceiveInput struct {
	GuardianGovernance string   `json:"guardianGovernance"`
	ManagerID          int      `json:"managerId"`
	Recipient          string   `json:"recipient"`
	VaaBytes           string   `json:"vaaBytes"`
	DiscloseTemplates  []string `json:"discloseTemplates"`
}

// preparePublishInput mirrors Playground.Prepare.daml's PreparePublishInput.
type preparePublishInput struct {
	Operator          string   `json:"operator"`
	EmitterID         int      `json:"emitterId"`
	DiscloseTemplates []string `json:"discloseTemplates"`
}

// prepareDeployNttInput mirrors Playground.Prepare.daml's PrepareDeployNttInput.
type prepareDeployNttInput struct {
	GuardianGovernance string   `json:"guardianGovernance"`
	FactoryCid         string   `json:"factoryCid"`
	DiscloseTemplates  []string `json:"discloseTemplates"`
}

// prepareAcceptAdminInput mirrors Playground.Prepare.daml's PrepareAcceptAdminInput.
type prepareAcceptAdminInput struct {
	GuardianGovernance string   `json:"guardianGovernance"`
	ManagerID          int      `json:"managerId"`
	VaaBytes           string   `json:"vaaBytes"`
	DiscloseTemplates  []string `json:"discloseTemplates"`
}
