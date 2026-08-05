package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// newFundCmd builds `fund` -- an operator/faucet action lifted verbatim out of `transfer`
// (design doc §5.5): ensure the recipient's standing pre-approval (Playground.Ops:preapprove,
// on the user's own participant), then mint via the deployment's committed faucet
// (Playground.Ops:fundUser, on gg's own participant -- gg is the mint's sole signatory).
//
// Splitting this out of `transfer` is what makes `--strict-participant-isolation` meaningful
// for a sender whose participant differs from gg's: fundUser is a gg-side submit that no
// disclosure service can stand in for (a DisclosedContract grants visibility, never
// authority -- see internal/disclosure's package doc), so it must run on gg's own
// participant, by an operator who legitimately holds gg's credentials. `fund` is that
// operator-side command; `transfer --no-fund` (transfer.go) is what a participant-isolated
// sender uses instead, leaving `Playground.Ops:transferOut` to enumerate the sender's own
// holdings in-script (Playground.Ops:mockHoldings).
//
// Deliberately does NOT set a.isolationBaselineRole: `fund` legitimately spans the user's and
// gg's participants by design (like `init`/`deploy`/`party`), so --strict-participant-isolation
// must never fire for it -- see runner.go's newScriptRunnerFor doc comment.
func newFundCmd(a *app) *cobra.Command {
	var deployment, userHint string
	var amount int64

	cmd := &cobra.Command{
		Use:   "fund",
		Short: "Mint mock-kind funds to a user via the deployment's faucet (Playground.Ops:preapprove + fundUser)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("fund: unknown deployment %q", deployment)
			}
			if d.TokenKind == "amulet" {
				return fmt.Errorf("fund: mock deployments only; amulet senders tap via transfer --tap-usd")
			}

			userParty, err := resolveParty(cmd, a, s, userHint)
			if err != nil {
				return err
			}
			a.vlogf(cmd, "fund: user %q → %s, deployment %q (managerId=%d)", userHint, userParty, deployment, d.ManagerID)

			// preapprove is a self-signed create by the recipient, so it routes to the
			// recipient's own participant (see Playground.Ops's doc comment on 'preapprove');
			// idempotent, so calling it here even for an already-preapproved user is a no-op.
			preRunner, preCleanup, err := a.newScriptRunnerFor(ctx, s.UserParticipants[userHint])
			if err != nil {
				return err
			}
			a.vlogf(cmd, "fund: ensuring user %q has a standing pre-approval (Playground.Ops:preapprove)", userHint)
			var preOut preapproveOutput
			preErr := preRunner.Run(ctx, "Playground.Ops:preapprove", preapproveInput{
				Admin:              d.Admin,
				GuardianGovernance: s.GuardianGovernance,
				TokenKind:          d.TokenKind,
				Mode:               "burn-mint",
				User:               userParty,
			}, &preOut)
			preCleanup()
			if preErr != nil {
				return fmt.Errorf("fund: ensure user pre-approval: %w", preErr)
			}

			// fundUser submits (and queries the faucet factory/preapproval) as gg alone -- see
			// Playground.Ops:fundUser -- so it routes to gg's OWN participant, never the
			// user's or the default one.
			ggRole := participantRoleForParty(s, s.GuardianGovernance)
			fundRunner, fundCleanup, err := a.newScriptRunnerFor(ctx, ggRole)
			if err != nil {
				return err
			}
			defer fundCleanup()

			decimals := wire.TrimDecimals(d.TokenDecimals)
			formatted := formatDecimal(amount, decimals)
			a.vlogf(cmd, "fund: minting %s to %q via Playground.Ops:fundUser", formatted, userHint)
			var fundOut fundUserOutput
			if err := fundRunner.Run(ctx, "Playground.Ops:fundUser", fundUserInput{
				GuardianGovernance: s.GuardianGovernance,
				Admin:              d.Admin,
				Owner:              userParty,
				Amount:             decimalLiteral(formatted),
			}, &fundOut); err != nil {
				return fmt.Errorf("fund: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "fund: holdingCid=%s amount=%s\n", fundOut.HoldingCid, formatted)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().StringVar(&userHint, "user", "", "party hint for the user to fund")
	cmd.Flags().Int64Var(&amount, "amount", 0, "raw amount to mint")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}
