package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
)

// proposeAdminTransferToGgInput/proposeAdminTransferToGgOutput mirror Playground.Ops.daml's
// ProposeAdminTransferToGgInput/ProposeAdminTransferToGgOutput.
type proposeAdminTransferToGgInput struct {
	Operator  string `json:"operator"`
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
}

type acceptAdminTransferByVaaOutput struct {
	ManagerAddress string `json:"managerAddress"`
	Admin          string `json:"admin"`
}

// newAdminCmd builds the `admin` command group: the two halves of the VAA-gated gg custody
// opt-in (Wormhole.Ntt.Manager's header) -- `propose-gg` (the current admin's standing offer)
// and `accept-gg-vaa` (permissionlessly relaying the guardians' signed acceptance). Neither
// subcommand builds a Playground.Disclose.RemoteSeam: both run the sandbox-first `remote =
// None` path Playground.Ops:acceptAdminTransferByVaa supports today (see that script's doc
// comment). This works unmodified on the single-participant sandbox profile; on a
// multi-participant LocalNet topology, `accept-gg-vaa` needs the executing participant to see
// operator/admin/guardianGovernance's contracts directly, so pass an `--executor` hint that
// resolves to a party already co-located with them (the default, no `--executor`, stays on the
// operator's own participant). A RemoteSeam extension mirroring `receive`'s is follow-up work.
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

			// The proposal is a self-signed create by the CURRENT admin; the deployment's
			// admin is co-located with operator under the default topology (mirrors
			// `deploy`'s deployNtt routing), so the default runner suffices.
			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "admin propose-gg: deployment=%s admin=%s -> guardianGovernance=%s", deployment, d.CurrentAdminOrAdmin(), s.GuardianGovernance)
			var out proposeAdminTransferToGgOutput
			if err := runner.Run(ctx, "Playground.Ops:proposeAdminTransferToGg", proposeAdminTransferToGgInput{
				Operator:  s.Operator,
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
			// participant); an explicit --executor routes the submit to THAT hint's own
			// participant instead -- mirrors `receive`'s executor routing, minus the
			// RemoteSeam fallback (see newAdminCmd's doc comment).
			executorParty := s.Operator
			executorRole := ""
			if executorHint != "" {
				executorParty, err = resolveParty(cmd, a, s, executorHint)
				if err != nil {
					return err
				}
				executorRole = s.UserParticipants[executorHint]
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
