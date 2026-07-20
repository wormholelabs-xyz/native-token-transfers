package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/amulet"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// deployConfig is the user-written deployment config file's shape (the playground plan's
// "deploy config" -- {name, mode, tokenKind, decimals, peers}).
type deployConfig struct {
	Name      string       `json:"name"`
	Mode      string       `json:"mode"`      // "burn-mint" | "lock-unlock"
	TokenKind string       `json:"tokenKind"` // "mock-admin-signed" | "cip56-burn-mint-mock" | "cip56-custody-mock" | "cip56-custody"
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
	Operator           string  `json:"operator"`
	GuardianGovernance string  `json:"guardianGovernance"`
	Admin              string  `json:"admin"`
	Mode               string  `json:"mode"`
	TokenKind          string  `json:"tokenKind"`
	TokenDecimals      int     `json:"tokenDecimals"`
	Custody            *string `json:"custody"`         // "cip56-custody" only; null otherwise
	InstrumentAdmin    *string `json:"instrumentAdmin"` // "cip56-custody" only; null otherwise
}

// amuletTapUSD is the belt-and-braces amount tapped to the validator's own wallet
// (app-provider) before requesting the custody's TransferPreapproval, since the validator
// pays the preapproval's creation fee (findings §8). Price-independent cushion, matching the
// plan's "generous USD amount" guidance for tap.
const amuletTapUSD = "1000"

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
			a.vlogf(cmd, "deploy %q: mode=%s tokenKind=%s decimals=%d peers=%d adminHint=%s",
				deploymentName, cfg.Mode, cfg.TokenKind, cfg.Decimals, len(cfg.Peers), adminHint)

			s, err := a.loadState()
			if err != nil {
				return err
			}

			prof, err := a.resolvedProfile()
			if err != nil {
				return err
			}

			// cip56-custody drives real Canton Coin (Amulet) rather than a local mock
			// registry: it needs a profile with a real DSO (AmuletAvailable), only supports
			// lock-unlock (Amulet has no BurnMintFactory), and requires the custody party to
			// be an onboarded validator wallet user with a standing TransferPreapproval
			// BEFORE the deploy script runs (Playground.Deploy:deployNtt's Cip56Custody
			// branch just creates the token hook against an already-resolved custody/DSO
			// party pair -- see internal/amulet for the off-ledger onboarding/tap/preapproval
			// calls).
			var custodyPartyPtr, instrumentAdminPtr *string
			var custodyUser string
			if cfg.TokenKind == "cip56-custody" {
				if !prof.AmuletAvailable {
					return fmt.Errorf("deploy: cip56-custody requires a profile with real Amulet (localnet)")
				}
				if cfg.Mode != "lock-unlock" {
					return fmt.Errorf("deploy: cip56-custody only supports mode=lock-unlock (Amulet has no BurnMintFactory)")
				}
				custodyParty, custodyUserID, dsoParty, err := setupCip56Custody(ctx, cmd, a, deploymentName, prof)
				if err != nil {
					return fmt.Errorf("deploy: cip56-custody setup: %w", err)
				}
				custodyPartyPtr, instrumentAdminPtr = &custodyParty, &dsoParty
				custodyUser = custodyUserID
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// The admin party is allocated (and granted actAs on auth-enabled profiles)
			// before deployNtt submits as it -- see grantActAs.
			admin, err := resolveParty(cmd, a, s, adminHint)
			if err != nil {
				return err
			}

			a.vlogf(cmd, "deploy %q: deployNtt = register transceiver Emitter → create %s token → RegisterManager → claim replay-trie root (consumer=admin)",
				deploymentName, cfg.TokenKind)
			var out deployOutput
			if err := runner.Run(ctx, "Playground.Deploy:deployNtt", deployInput{
				Operator:           s.Operator,
				GuardianGovernance: s.GuardianGovernance,
				Admin:              admin,
				Mode:               cfg.Mode,
				TokenKind:          cfg.TokenKind,
				TokenDecimals:      cfg.Decimals,
				Custody:            custodyPartyPtr,
				InstrumentAdmin:    instrumentAdminPtr,
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
			if custodyPartyPtr != nil {
				d.CustodyParty = *custodyPartyPtr
				d.CustodyUser = custodyUser
				d.InstrumentAdmin = *instrumentAdminPtr
			}

			for _, p := range cfg.Peers {
				a.vlogf(cmd, "set peer chain=%d manager=%s transceiver=%s", p.Chain, p.Manager, p.Transceiver)
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

// setupCip56Custody resolves everything Playground.Deploy:deployNtt's Cip56Custody branch
// needs before it runs: the real DSO party, the custody wallet user's onboarded party (with
// actAs granted to the script user), and a standing TransferPreapproval for it. The validator
// pays the preapproval's creation fee, so its own wallet (app-provider) is tapped first --
// belt-and-braces, matching the plan's design decision. None of this is a `dpm script` call:
// it is all off-ledger HTTP against the validator (internal/amulet) plus one localnet.go
// lookup, exactly the CLI's "ledger chokepoint stays dpm script for everything that submits"
// rule -- these calls resolve inputs, they don't submit commands themselves.
func setupCip56Custody(ctx context.Context, cmd *cobra.Command, a *app, deploymentName string, prof profile.Profile) (custodyParty, custodyUser, dsoParty string, err error) {
	m, err := a.localNetManager()
	if err != nil {
		return "", "", "", err
	}
	dsoParty, err = m.DSOPartyID(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve DSO party: %w", err)
	}
	a.vlogf(cmd, "cip56-custody: DSO party → %s", dsoParty)

	client := &amulet.Client{ValidatorBaseURL: prof.ValidatorBaseURL, Logf: a.verboseLogf()}

	custodyUser = deploymentName + "-custody"
	a.vlogf(cmd, "cip56-custody: onboarding wallet user %q", custodyUser)
	custodyParty, err = client.OnboardWalletUser(ctx, custodyUser)
	if err != nil {
		return "", "", "", fmt.Errorf("onboard custody wallet user %q: %w", custodyUser, err)
	}
	if err := grantActAs(cmd, a, custodyParty); err != nil {
		return "", "", "", fmt.Errorf("grant actAs on custody party %s: %w", custodyParty, err)
	}

	a.vlogf(cmd, "cip56-custody: tapping the validator's own wallet (%s) before requesting the preapproval (it pays the creation fee)", network.LocalNetWalletUser)
	if _, err := client.Tap(ctx, network.LocalNetWalletUser, amuletTapUSD); err != nil {
		return "", "", "", fmt.Errorf("tap validator wallet %s: %w", network.LocalNetWalletUser, err)
	}

	a.vlogf(cmd, "cip56-custody: creating TransferPreapproval for %s (%s)", custodyUser, custodyParty)
	if _, err := client.CreateTransferPreapproval(ctx, custodyUser); err != nil {
		return "", "", "", fmt.Errorf("create TransferPreapproval for %s: %w", custodyUser, err)
	}

	return custodyParty, custodyUser, dsoParty, nil
}
