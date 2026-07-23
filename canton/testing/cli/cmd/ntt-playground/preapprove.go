package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// preapproveInput/preapproveOutput mirror Playground.Ops.daml's PreapproveInput/PreapproveOutput.
// No RemoteSeam field: preapprove is a self-signed create by the user (owner), with only plain
// Party values as arguments -- no cross-participant contract fetch is ever needed, so it always
// routes to the user's own participant and runs there unassisted (see Ops.daml's doc comment on
// 'preapprove').
type preapproveInput struct {
	Admin              string `json:"admin"`
	GuardianGovernance string `json:"guardianGovernance"`
	TokenKind          string `json:"tokenKind"` // "mock" | "amulet"
	Mode               string `json:"mode"`      // "burn-mint" | "lock-unlock"
	User               string `json:"user"`
}

type preapproveOutput struct {
	Preapproved bool `json:"preapproved"`
}

// revokeInput/revokeOutput mirror Playground.Ops.daml's RevokeInput/RevokeOutput.
type revokeInput struct {
	GuardianGovernance string `json:"guardianGovernance"`
	TokenKind          string `json:"tokenKind"`
	Mode               string `json:"mode"`
	User               string `json:"user"`
}

type revokeOutput struct {
	Revoked bool `json:"revoked"`
}

// newPreapproveCmd builds `preapprove` (a recipient's standing deposit opt-in) plus its
// `revoke` subcommand (owner-only teardown). Pre-approval is a per-recipient, recipient-
// authorized opt-in, so it is deliberately NOT folded into `deploy` (admin-driven, runs before
// recipients exist) -- the negative receive path requires a deployment whose recipient has not
// opted in yet.
func newPreapproveCmd(a *app) *cobra.Command {
	var deployment, userHint string

	cmd := &cobra.Command{
		Use:   "preapprove",
		Short: "Pre-approve a recipient's inbound deliveries (DepositPreapproval or MockTransferPreapproval, per mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("preapprove: unknown deployment %q", deployment)
			}
			userParty, err := resolveParty(cmd, a, s, userHint)
			if err != nil {
				return err
			}

			// preapprove is a recipient-authorized opt-in, so it routes to the user's own
			// participant (plan §3) -- and needs no RemoteSeam (see preapproveInput's doc
			// comment): a self-signed create by the user, no cross-participant contract fetch.
			runner, cleanup, err := a.newScriptRunnerFor(ctx, s.UserParticipants[userHint])
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "preapprove: user %q → %s opting in on deployment %q (tokenKind=%s mode=%s)", userHint, userParty, deployment, d.TokenKind, d.Mode)
			var out preapproveOutput
			if err := runner.Run(ctx, "Playground.Ops:preapprove", preapproveInput{
				Admin:              d.Admin,
				GuardianGovernance: s.GuardianGovernance,
				TokenKind:          d.TokenKind,
				Mode:               d.Mode,
				User:               userParty,
			}, &out); err != nil {
				return err
			}

			if out.Preapproved {
				fmt.Fprintf(cmd.OutOrStdout(), "preapprove: deployment=%s user=%s preapproved=true\n", deployment, userParty)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "preapprove: deployment=%s user=%s preapproved=false (no pre-approval required for this token kind)\n", deployment, userParty)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().StringVar(&userHint, "user", "", "party hint for the recipient opting in")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("user")

	cmd.AddCommand(newPreapproveRevokeCmd(a))
	return cmd
}

func newPreapproveRevokeCmd(a *app) *cobra.Command {
	var deployment, userHint string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a recipient's standing deposit pre-approval (owner-only DepositPreapproval.Revoke)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("preapprove revoke: unknown deployment %q", deployment)
			}
			userParty, err := resolveParty(cmd, a, s, userHint)
			if err != nil {
				return err
			}

			// revoke, like preapprove, routes to the user's own participant (plan §3).
			runner, cleanup, err := a.newScriptRunnerFor(ctx, s.UserParticipants[userHint])
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "preapprove revoke: user %q → %s tearing down its standing consent on deployment %q", userHint, userParty, deployment)
			var out revokeOutput
			if err := runner.Run(ctx, "Playground.Ops:revoke", revokeInput{
				GuardianGovernance: s.GuardianGovernance,
				TokenKind:          d.TokenKind,
				Mode:               d.Mode,
				User:               userParty,
			}, &out); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "preapprove revoke: deployment=%s user=%s revoked=%v\n", deployment, userParty, out.Revoked)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().StringVar(&userHint, "user", "", "party hint for the recipient revoking")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("user")
	return cmd
}
