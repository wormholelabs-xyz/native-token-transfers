package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// preapproveDepositInput/preapproveDepositOutput mirror Playground.Ops.daml's
// PreapproveDepositInput/PreapproveDepositOutput.
type preapproveDepositInput struct {
	Admin     string `json:"admin"`
	TokenKind string `json:"tokenKind"`
	User      string `json:"user"`
	// Remote carries a pre-fetched Playground.Prepare:preparePreapprove RemoteSeam, as raw
	// JSON -- see transferOutInput.Remote's doc comment (transfer.go).
	Remote json.RawMessage `json:"remote"`
}

type preapproveDepositOutput struct {
	Preapproved bool `json:"preapproved"`
}

// revokeDepositInput/revokeDepositOutput mirror Playground.Ops.daml's
// RevokeDepositInput/RevokeDepositOutput.
type revokeDepositInput struct {
	Admin string `json:"admin"`
	User  string `json:"user"`
}

type revokeDepositOutput struct {
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
		Short: "Pre-approve a recipient's inbound deposits (NttToken.PreApproveDeposit)",
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
			// participant (plan §3).
			actorRole := s.UserParticipants[userHint]
			remoteSeam, err := prepareRemoteSeam(ctx, cmd, a, s, actorRole, "Playground.Prepare:preparePreapprove", func(templates []string) any {
				return preparePreapproveInput{
					Admin:             d.Admin,
					TokenKind:         d.TokenKind,
					DiscloseTemplates: templates,
				}
			})
			if err != nil {
				return fmt.Errorf("preapprove: prepare remote disclosure: %w", err)
			}

			runner, cleanup, err := a.newScriptRunnerFor(ctx, actorRole)
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "preapprove: user %q → %s opting in on deployment %q (tokenKind=%s)", userHint, userParty, deployment, d.TokenKind)
			var out preapproveDepositOutput
			if err := runner.Run(ctx, "Playground.Ops:preapproveDeposit", preapproveDepositInput{
				Admin:     d.Admin,
				TokenKind: d.TokenKind,
				User:      userParty,
				Remote:    remoteSeam,
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
			var out revokeDepositOutput
			if err := runner.Run(ctx, "Playground.Ops:revokeDeposit", revokeDepositInput{
				Admin: d.Admin,
				User:  userParty,
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
