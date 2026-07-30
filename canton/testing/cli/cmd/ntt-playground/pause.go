package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// setPausedInput/setPausedOutput mirror Playground.Ops.daml's SetPausedInput/SetPausedOutput.
type setPausedInput struct {
	Operator  string `json:"operator"`
	ManagerID int    `json:"managerId"`
	Admin     string `json:"admin"`
	Paused    bool   `json:"paused"`
}

type setPausedOutput struct {
	ManagerID int  `json:"managerId"`
	Paused    bool `json:"paused"`
}

// setPausedOnLedger runs Playground.Ops:setPaused against an already-open runner -- shared by
// `pause` and `unpause` (newPauseUnpauseCmd below), mirroring peer.go's setPeerOnLedger.
func setPausedOnLedger(ctx context.Context, runner scriptRunner, operator string, managerID int, admin string, paused bool) error {
	var out setPausedOutput
	return runner.Run(ctx, "Playground.Ops:setPaused", setPausedInput{
		Operator:  operator,
		ManagerID: managerID,
		Admin:     admin,
		Paused:    paused,
	}, &out)
}

// newPauseUnpauseCmd builds `pause`/`unpause`: identical shape (resolve the deployment, open a
// runner routed the same way `peer set` is -- the deployment's admin is co-located with the
// operator -- exercise SetPaused via the admin), differing only in the paused value sent and
// the verb printed.
func newPauseUnpauseCmd(a *app, use, short string, paused bool) *cobra.Command {
	var deployment string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("%s: unknown deployment %q", use, deployment)
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// SetPaused's controller is the manager's live `admin` signatory -- submit as
			// whoever CURRENTLY holds the role, not the registering admin (they differ once
			// `admin accept-gg-vaa` has run); mirrors peer.go's setPeerOnLedger call.
			if err := setPausedOnLedger(ctx, runner, s.Operator, d.ManagerID, d.CurrentAdminOrAdmin(), paused); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s paused=%t\n", use, deployment, paused)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	_ = cmd.MarkFlagRequired("deployment")
	return cmd
}

func newPauseCmd(a *app) *cobra.Command {
	return newPauseUnpauseCmd(a, "pause",
		"Halt a deployment's value movement (Transfer/Release/Mint) via the admin-controlled SetPaused",
		true)
}

func newUnpauseCmd(a *app) *cobra.Command {
	return newPauseUnpauseCmd(a, "unpause",
		"Resume a paused deployment's value movement via the admin-controlled SetPaused",
		false)
}
