package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
)

// proposeAdminTransferToGgInput/proposeAdminTransferToGgOutput mirror Playground.Ops.daml's
// ProposeAdminTransferToGgInput/ProposeAdminTransferToGgOutput. Admin is the deployment's
// CURRENT admin (Deployment.CurrentAdminOrAdmin()), not operator: the script is single-party --
// reads and submits as Admin alone -- which is what lets the CLI route the whole call to
// Admin's own participant (see newAdminProposeGgCmd's doc comment).
type proposeAdminTransferToGgInput struct {
	Admin     string `json:"admin"`
	ManagerID int    `json:"managerId"`
}

type proposeAdminTransferToGgOutput struct {
	ManagerAddress string `json:"managerAddress"`
	FactoryEpoch   int    `json:"factoryEpoch"`
}

// acceptAdminTransferByVaaInput/acceptAdminTransferByVaaOutput mirror Playground.Ops.daml's
// AcceptAdminTransferByVaaInput/AcceptAdminTransferByVaaOutput.
type acceptAdminTransferByVaaInput struct {
	Operator  string              `json:"operator"`
	ManagerID int                 `json:"managerId"`
	Executor  string              `json:"executor"`
	VaaBytes  string              `json:"vaaBytes"`
	PubKeys   []ledger.PubKeyHint `json:"pubKeys"`
	// Remote carries a pre-fetched Playground.Prepare:prepareAcceptAdmin RemoteSeam, as raw
	// JSON -- see transferOutInput.Remote's doc comment (transfer.go).
	Remote json.RawMessage `json:"remote"`
}

type acceptAdminTransferByVaaOutput struct {
	ManagerAddress string `json:"managerAddress"`
	Admin          string `json:"admin"`
}

// newAdminCmd builds the `admin` command group: the two halves of the VAA-gated gg custody
// opt-in (Wormhole.Ntt.Manager's header) -- `propose-gg` (the current admin's standing offer)
// and `accept-gg-vaa` (permissionlessly relaying the guardians' signed acceptance). `propose-gg`
// is single-party (Playground.Ops:proposeAdminTransferToGg reads AND submits as the CURRENT
// admin alone), so it routes the whole script run to THAT party's own participant --
// participantRoleForParty(s, d.CurrentAdminOrAdmin()) -- rather than the default runner: before
// any handoff the current admin is the registering admin, co-located with operator under the
// default topology, so this is byte-identical to before; after a successful `accept-gg-vaa` the
// current admin is guardianGovernance, hosted on its own participant, and the old
// always-default-participant routing would fail PERMISSION_DENIED there (the bug a second
// propose/accept cycle post-handoff exposed). `accept-gg-vaa` gets the standard prepareRemoteSeam
// treatment mirroring `receive`'s (see remote.go): the actor (the relaying `--executor`, or
// operator by default) and the data owner (guardianGovernance) are compared, and a RemoteSeam is
// fetched via Playground.Prepare:prepareAcceptAdmin whenever they differ -- unconditionally the
// case on a multi-participant LocalNet topology once guardianGovernance became its own
// participant, where no single participant hosts both operator and gg.
func newAdminCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage the guardian-quorum custody opt-in: propose the admin handoff, then accept it by relaying a guardian-signed VAA",
	}
	cmd.AddCommand(newAdminProposeGgCmd(a), newAdminAcceptGgVaaCmd(a))
	return cmd
}

func newAdminProposeGgCmd(a *app) *cobra.Command {
	var deployment string

	cmd := &cobra.Command{
		Use:   "propose-gg",
		Short: "Have the deployment's current admin create a standing offer to hand the operational role to guardianGovernance",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("admin propose-gg: unknown deployment %q", deployment)
			}

			// The proposal is a self-signed create by the CURRENT admin -- route the script
			// run to THAT party's own participant, not the default one: pre-handoff this is
			// the registering admin (co-located with operator under the default topology,
			// participantRoleForParty resolves "" -- unchanged from before this fix); after a
			// successful `accept-gg-vaa` it is guardianGovernance, hosted elsewhere (see
			// newAdminCmd's doc comment).
			adminParty := d.CurrentAdminOrAdmin()
			role := participantRoleForParty(s, adminParty)
			runner, cleanup, err := a.newScriptRunnerFor(ctx, role)
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "admin propose-gg: deployment=%s admin=%s -> guardianGovernance=%s", deployment, adminParty, s.GuardianGovernance)
			var out proposeAdminTransferToGgOutput
			if err := runner.Run(ctx, "Playground.Ops:proposeAdminTransferToGg", proposeAdminTransferToGgInput{
				Admin:     adminParty,
				ManagerID: d.ManagerID,
			}, &out); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "admin propose-gg: managerAddress=%s factoryEpoch=%d\n", out.ManagerAddress, out.FactoryEpoch)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	_ = cmd.MarkFlagRequired("deployment")
	return cmd
}

func newAdminAcceptGgVaaCmd(a *app) *cobra.Command {
	var deployment, vaaHex, pubKeyHex, executorHint string

	cmd := &cobra.Command{
		Use:   "accept-gg-vaa",
		Short: "Permissionlessly relay a guardian-signed accept-admin VAA, completing the gg custody opt-in (NttManager.AcceptAdminTransferByVaa)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("admin accept-gg-vaa: unknown deployment %q", deployment)
			}
			if pubKeyHex == "" {
				return fmt.Errorf("admin accept-gg-vaa: --pubkey is required (the signing guardian's 65-byte uncompressed pubkey)")
			}

			// executor defaults to the Operator (stays on the default/app-provider
			// participant, executorRole=""); an explicit --executor routes the submit to
			// THAT hint's own participant instead -- mirrors `receive`'s executor routing.
			executorParty := s.Operator
			executorRole := ""
			if executorHint != "" {
				executorParty, err = resolveParty(cmd, a, s, executorHint)
				if err != nil {
					return err
				}
				executorRole = s.UserParticipants[executorHint]
			}

			// Record the actor's participant role as the --strict-participant-isolation
			// baseline (runner.go), immediately after it is known and before
			// prepareRemoteSeam below (which may need to cross a participant boundary).
			a.isolationBaselineRole = executorRole
			a.isolationBaselineSet = true

			// The data owner for a Mock-kind prepare fetch is gg's OWN participant, not
			// operator's -- see remote.go's doc comment and receive.go's identical reasoning.
			ownerRole := participantRoleForParty(s, s.GuardianGovernance)
			remoteSeam, err := prepareRemoteSeam(ctx, cmd, a, s, executorRole, ownerRole, "Playground.Prepare:prepareAcceptAdmin", "acceptAdminTransfer", func(templates []string) any {
				return prepareAcceptAdminInput{
					GuardianGovernance: s.GuardianGovernance,
					ManagerID:          d.ManagerID,
					VaaBytes:           vaaHex,
					DiscloseTemplates:  templates,
				}
			})
			if err != nil {
				return fmt.Errorf("admin accept-gg-vaa: prepare remote disclosure: %w", err)
			}

			runner, cleanup, err := a.newScriptRunnerFor(ctx, executorRole)
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "admin accept-gg-vaa: relaying %d-byte VAA through NttManager.AcceptAdminTransferByVaa (executor=%s)", len(vaaHex)/2, executorParty)
			var out acceptAdminTransferByVaaOutput
			if err := runner.Run(ctx, "Playground.Ops:acceptAdminTransferByVaa", acceptAdminTransferByVaaInput{
				Operator:  s.Operator,
				ManagerID: d.ManagerID,
				Executor:  executorParty,
				VaaBytes:  vaaHex,
				PubKeys:   []ledger.PubKeyHint{{Index: 0, Key: pubKeyHex}},
				Remote:    remoteSeam,
			}, &out); err != nil {
				return err
			}

			// On success the deployment's CURRENT admin is now guardianGovernance -- update
			// CurrentAdmin (never Admin: that field is the REGISTERING admin, fixed forever,
			// and stays the key every instrument/address derivation hashes against -- see
			// state.Deployment's doc comment) so later commands (status, a subsequent
			// `receive`, `peer set`, ...) see the new admin without a fresh ledger round-trip.
			d.CurrentAdmin = out.Admin
			s.Deployments[deployment] = d
			if err := a.saveState(s); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "admin accept-gg-vaa: managerAddress=%s admin=%s\n", out.ManagerAddress, out.Admin)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().StringVar(&vaaHex, "vaa", "", "hex-encoded signed accept-admin governance VAA")
	cmd.Flags().StringVar(&pubKeyHex, "pubkey", "", "the signing guardian's 65-byte uncompressed public key, hex")
	cmd.Flags().StringVar(&executorHint, "executor", "", "party hint for the relaying executor (default: operator)")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("vaa")
	_ = cmd.MarkFlagRequired("pubkey")
	return cmd
}
