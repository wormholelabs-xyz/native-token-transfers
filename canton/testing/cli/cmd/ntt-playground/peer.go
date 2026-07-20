package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// setPeerInput/setPeerOutput mirror Playground.Ops.daml's SetPeerInput/SetPeerOutput.
type setPeerInput struct {
	Operator        string `json:"operator"`
	ManagerID       int    `json:"managerId"`
	Admin           string `json:"admin"`
	Chain           int    `json:"chain"`
	PeerManager     string `json:"peerManager"`
	PeerTransceiver string `json:"peerTransceiver"`
}

type setPeerOutput struct {
	ManagerID int `json:"managerId"`
}

// setPeerOnLedger runs Playground.Ops:setPeer against an already-open runner -- shared by
// `deploy` (config-file peers) and the standalone `peer set` command.
func setPeerOnLedger(ctx context.Context, a *app, runner *ledger.Runner, operator string, managerID int, admin string, chain int, peerManager, peerTransceiver string) error {
	var out setPeerOutput
	return runner.Run(ctx, "Playground.Ops:setPeer", setPeerInput{
		Operator:        operator,
		ManagerID:       managerID,
		Admin:           admin,
		Chain:           chain,
		PeerManager:     peerManager,
		PeerTransceiver: peerTransceiver,
	}, &out)
}

func newPeerCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peer",
		Short: "Manage a deployment's peer configuration",
	}
	cmd.AddCommand(newPeerSetCmd(a))
	return cmd
}

func newPeerSetCmd(a *app) *cobra.Command {
	var deployment string
	var chain int
	var manager string
	var transceiver string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Configure (or replace) a deployment's peer for a remote chain",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("peer set: unknown deployment %q", deployment)
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			if err := setPeerOnLedger(ctx, a, runner, s.Operator, d.ManagerID, d.Admin, chain, manager, transceiver); err != nil {
				return err
			}

			if d.Peers == nil {
				d.Peers = map[int]state.Peer{}
			}
			d.Peers[chain] = state.Peer{ManagerAddress: manager, TransceiverAddress: transceiver}
			s.Deployments[deployment] = d
			if err := a.saveState(s); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "peer set: %s chain=%d manager=%s transceiver=%s\n", deployment, chain, manager, transceiver)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().IntVar(&chain, "chain", 0, "remote Wormhole chain id")
	cmd.Flags().StringVar(&manager, "manager", "", "peer manager address (32-byte hex)")
	cmd.Flags().StringVar(&transceiver, "transceiver", "", "peer transceiver address (32-byte hex)")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("chain")
	_ = cmd.MarkFlagRequired("manager")
	_ = cmd.MarkFlagRequired("transceiver")
	return cmd
}
