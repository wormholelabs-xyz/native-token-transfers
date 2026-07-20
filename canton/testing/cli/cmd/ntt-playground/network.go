package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
)

func newNetworkCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage the local Canton devnet backing the playground",
	}
	cmd.AddCommand(newNetworkUpCmd(a), newNetworkDownCmd(a), newNetworkStatusCmd(a))
	return cmd
}

func (a *app) sandboxManager() (*network.SandboxManager, error) {
	dpmPath := a.dpmPath
	if dpmPath == "" {
		var err error
		dpmPath, err = ledger.FindDpm()
		if err != nil {
			return nil, err
		}
	}
	return &network.SandboxManager{DpmPath: dpmPath, RunDir: a.resolvedRunDir(), Port: profile.SandboxPort(), Logf: a.verboseLogf()}, nil
}

// vlogNetwork narrates the resolved profile and the environment that shapes it, before a
// network subcommand delegates to the managers.
func (a *app) vlogNetwork(cmd *cobra.Command, verb string) {
	switch a.profile {
	case profile.LocalNet:
		a.vlogf(cmd, "network %s: profile=localnet LOCALNET_DIR=%s IMAGE_TAG=%s", verb, os.Getenv("LOCALNET_DIR"), os.Getenv("IMAGE_TAG"))
	default:
		a.vlogf(cmd, "network %s: profile=sandbox port=%d run-dir=%s", verb, profile.SandboxPort(), a.resolvedRunDir())
	}
}

func (a *app) localNetManager() (*network.LocalNetManager, error) {
	composeDir := os.Getenv("LOCALNET_DIR")
	if composeDir == "" {
		return nil, fmt.Errorf("network: LOCALNET_DIR must point at the extracted splice-node/docker-compose/localnet directory")
	}
	return &network.LocalNetManager{ComposeDir: composeDir, ImageTag: os.Getenv("IMAGE_TAG"), Logf: a.verboseLogf()}, nil
}

func newNetworkUpCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start the network for the selected profile (blocks until ready)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a.vlogNetwork(cmd, "up")
			switch a.profile {
			case profile.LocalNet:
				m, err := a.localNetManager()
				if err != nil {
					return err
				}
				if err := m.Up(ctx, 10*time.Minute); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "localnet: ready")
				return nil
			default:
				m, err := a.sandboxManager()
				if err != nil {
					return err
				}
				info, err := m.Up(ctx, 180*time.Second)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "sandbox: ready on port %d (pid %d)\n", info.Port, info.PID)
				return nil
			}
		},
	}
}

func newNetworkDownCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop the network for the selected profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a.vlogNetwork(cmd, "down")
			switch a.profile {
			case profile.LocalNet:
				m, err := a.localNetManager()
				if err != nil {
					return err
				}
				return m.Down(ctx)
			default:
				m, err := a.sandboxManager()
				if err != nil {
					return err
				}
				return m.Down()
			}
		},
	}
}

func newNetworkStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report whether the network for the selected profile is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			a.vlogNetwork(cmd, "status")
			switch a.profile {
			case profile.LocalNet:
				m, err := a.localNetManager()
				if err != nil {
					return err
				}
				services, err := m.Status(cmd.Context())
				if err != nil {
					return err
				}
				// Exit non-zero when nothing is up, so scripts can gate on `network status`
				// the same way they would on the sandbox pid check.
				if len(services) == 0 {
					return fmt.Errorf("network status: localnet is not running (no compose services up)")
				}
				for _, svc := range services {
					fmt.Fprintf(cmd.OutOrStdout(), "localnet: service=%s state=%s health=%s\n",
						svc.Service, svc.State, svc.Health)
				}
				return nil
			default:
				m, err := a.sandboxManager()
				if err != nil {
					return err
				}
				running, info, err := m.Status()
				if err != nil {
					return err
				}
				if !running {
					fmt.Fprintln(cmd.OutOrStdout(), "sandbox: not running")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "sandbox: running on port %d (pid %d)\n", info.Port, info.PID)
				return nil
			}
		},
	}
}
