package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// deployConfig is the user-written deployment config file's shape (the playground plan's
// "deploy config" -- {name, mode, tokenKind, decimals, peers}).
type deployConfig struct {
	Name      string       `json:"name"`
	Mode      string       `json:"mode"`      // "burn-mint" | "lock-unlock"
	TokenKind string       `json:"tokenKind"` // "mock-admin-signed" | "cip56-burn-mint-mock" | "cip56-custody-mock"
	Decimals  int          `json:"decimals"`
	AdminHint string       `json:"adminHint,omitempty"` // display-name hint for the fresh admin party; defaults to name+"-admin"
	Peers     []peerConfig `json:"peers,omitempty"`
}

type peerConfig struct {
	Chain       int    `json:"chain"`
	Manager     string `json:"manager"`
	Transceiver string `json:"transceiver"`
}

// deployInput/deployOutput mirror Playground.Deploy.daml's DeployInput/DeployOutput.
type deployInput struct {
	Operator           string `json:"operator"`
	GuardianGovernance string `json:"guardianGovernance"`
	Admin              string `json:"admin"`
	Mode               string `json:"mode"`
	TokenKind          string `json:"tokenKind"`
	TokenDecimals      int    `json:"tokenDecimals"`
}

type deployOutput struct {
	ManagerID          int    `json:"managerId"`
	ManagerAddress     string `json:"managerAddress"`
	TransceiverAddress string `json:"transceiverAddress"`
	Admin              string `json:"admin"`
}

func newDeployCmd(a *app) *cobra.Command {
	var configPath string
	var name string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a new NTT manager + transceiver + token seam",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			raw, err := os.ReadFile(configPath)
			if err != nil {
				return fmt.Errorf("deploy: read config %s: %w", configPath, err)
			}
			var cfg deployConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return fmt.Errorf("deploy: parse config %s: %w", configPath, err)
			}

			deploymentName := name
			if deploymentName == "" {
				deploymentName = cfg.Name
			}
			if deploymentName == "" {
				return fmt.Errorf("deploy: config must set \"name\" (or pass --name)")
			}
			adminHint := cfg.AdminHint
			if adminHint == "" {
				adminHint = deploymentName + "-admin"
			}

			s, err := a.loadState()
			if err != nil {
				return err
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// The admin party is allocated (and granted actAs on auth-enabled profiles)
			// before deployNtt submits as it -- see grantActAs.
			admin, err := resolveParty(ctx, a, s, adminHint)
			if err != nil {
				return err
			}

			var out deployOutput
			if err := runner.Run(ctx, "Playground.Deploy:deployNtt", deployInput{
				Operator:           s.Operator,
				GuardianGovernance: s.GuardianGovernance,
				Admin:              admin,
				Mode:               cfg.Mode,
				TokenKind:          cfg.TokenKind,
				TokenDecimals:      cfg.Decimals,
			}, &out); err != nil {
				return err
			}

			d := state.Deployment{
				Name:               deploymentName,
				ManagerID:          out.ManagerID,
				ManagerAddress:     out.ManagerAddress,
				TransceiverAddress: out.TransceiverAddress,
				Admin:              out.Admin,
				Mode:               cfg.Mode,
				TokenKind:          cfg.TokenKind,
				TokenDecimals:      cfg.Decimals,
				Peers:              map[int]state.Peer{},
			}

			for _, p := range cfg.Peers {
				if err := setPeerOnLedger(ctx, a, runner, s.Operator, out.ManagerID, out.Admin, p.Chain, p.Manager, p.Transceiver); err != nil {
					return fmt.Errorf("deploy: set peer chain %d: %w", p.Chain, err)
				}
				d.Peers[p.Chain] = state.Peer{ManagerAddress: p.Manager, TransceiverAddress: p.Transceiver}
			}

			s.Deployments[deploymentName] = d
			if err := a.saveState(s); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "deploy: %s managerId=%d managerAddress=%s transceiverAddress=%s admin=%s\n",
				deploymentName, out.ManagerID, out.ManagerAddress, out.TransceiverAddress, out.Admin)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "path to the deployment config JSON file")
	cmd.Flags().StringVar(&name, "name", "", "deployment name to key state under (default: the config file's \"name\")")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}
