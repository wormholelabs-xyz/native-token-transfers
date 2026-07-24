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

// deployConfig is the shape of the user-written deployment config file:
// {name, mode, tokenKind, decimals, peers}.
type deployConfig struct {
	Name      string       `json:"name"`
	Mode      string       `json:"mode"`      // "burn-mint" | "lock-unlock"
	TokenKind string       `json:"tokenKind"` // "mock" | "amulet"
	Decimals  int          `json:"decimals"`
	AdminHint string       `json:"adminHint,omitempty"` // display-name hint for the fresh admin party; defaults to name+"-admin"
	Peers     []peerConfig `json:"peers,omitempty"`
}

type peerConfig struct {
	Chain       int    `json:"chain"`
	Manager     string `json:"manager"`
	Transceiver string `json:"transceiver"`
	Decimals    int    `json:"decimals"`
}

// validTokenKinds are the only Playground.Types.daml TokenKind strings the CLI accepts
// (Mock | Amulet). Collapsed from the pre-CIP-56-rework 4-value scheme now that
// Playground.MockToken (the admin-signed double needing no recipient consent) is gone --
// see Playground.Types.daml's TokenKind doc comment.
var validTokenKinds = map[string]bool{"mock": true, "amulet": true}

// legacyTokenKindHints maps each pre-CIP-56-rework tokenKind string to the current kind that
// replaces it, so a stale config file's error names the fix instead of just "unknown".
var legacyTokenKindHints = map[string]string{
	"mock-admin-signed":    "mock",
	"cip56-burn-mint-mock": "mock",
	"cip56-custody-mock":   "mock",
	"cip56-custody":        "amulet",
}

// parseTokenKind validates a deploy config's tokenKind against the current 2-value model
// (Playground.Types.daml's TokenKind = Mock | Amulet). Pure Go logic -- no Daml dependency --
// so a stale config fails fast with a clear migration hint at the CLI layer instead of
// surfacing a less legible abort from deep inside the Daml script.
func parseTokenKind(raw string) (string, error) {
	if validTokenKinds[raw] {
		return raw, nil
	}
	if hint, ok := legacyTokenKindHints[raw]; ok {
		return "", fmt.Errorf("deploy: tokenKind %q was removed in the CIP-56 rework -- use %q instead", raw, hint)
	}
	return "", fmt.Errorf("deploy: unknown tokenKind %q (want \"mock\" or \"amulet\")", raw)
}

// validateDeployConfig checks a deploy config's cross-field invariants that need no ledger
// round-trip. Currently: "amulet" deployments are lock-unlock only (Splice's Amulet registry
// has no BurnMintFactory to bridge burn/mint against). Assumes cfg.TokenKind has already been
// normalized by parseTokenKind.
func validateDeployConfig(cfg deployConfig) error {
	if cfg.TokenKind == "amulet" && cfg.Mode != "lock-unlock" {
		return fmt.Errorf("deploy: tokenKind \"amulet\" only supports mode=lock-unlock (Amulet has no BurnMintFactory)")
	}
	for _, p := range cfg.Peers {
		if p.Decimals < 1 || p.Decimals > 255 {
			return fmt.Errorf("deploy: peer chain %d has invalid decimals %d (want 1-255)", p.Chain, p.Decimals)
		}
	}
	return nil
}

// deployRegistryInput/deployRegistryOutput mirror Playground.Deploy.daml's
// DeployRegistryInput/DeployRegistryOutput -- the gg-submitted step that stands up this
// deployment's mock registry factory contract(s) before deployNtt (admin-submitted) resolves
// them.
type deployRegistryInput struct {
	GuardianGovernance string `json:"guardianGovernance"`
	Admin              string `json:"admin"`
	Mode               string `json:"mode"`
	TokenKind          string `json:"tokenKind"`
	InstrumentNonce    int    `json:"instrumentNonce"`
}

type deployRegistryOutput struct {
	Registered bool `json:"registered"`
	// FactoryCid is the deployment's committed factory cid, created by THIS call (mock only;
	// null for "amulet") -- threaded straight into deployInput.FactoryCid below rather than
	// re-resolved by deployNtt via a fresh query: gg (the factory's sole signatory) lives on
	// its own participant in the multi-participant topology, invisible to a query running on
	// admin's (see Playground.Deploy.daml's DeployRegistryOutput.factoryCid doc comment).
	FactoryCid *string `json:"factoryCid"`
}

// deployInput/deployOutput mirror Playground.Deploy.daml's DeployInput/DeployOutput.
type deployInput struct {
	Operator           string  `json:"operator"`
	GuardianGovernance string  `json:"guardianGovernance"`
	Admin              string  `json:"admin"`
	Mode               string  `json:"mode"`
	TokenKind          string  `json:"tokenKind"`
	TokenDecimals      int     `json:"tokenDecimals"`
	InstrumentNonce    int     `json:"instrumentNonce"`
	InstrumentAdmin    *string `json:"instrumentAdmin"`  // "amulet" only (the real DSO party); null otherwise
	AmuletFactoryCid   *string `json:"amuletFactoryCid"` // "amulet" only (setupAmuletAdmin's resolved transfer-factory cid); null otherwise
	FactoryCid         *string `json:"factoryCid"`       // "mock" only (deployRegistryOutput.FactoryCid, passed straight through); null otherwise
}

// amuletTapUSD is the belt-and-braces amount tapped to the validator's own wallet
// (app-provider) before requesting the admin's TransferPreapproval, since the validator pays
// the preapproval's creation fee (mirrors the pre-rework setupCip56Custody this replaces).
// Price-independent cushion.
const amuletTapUSD = "1000"

// amuletFactoryProbeAmount is the nominal amount setupAmuletAdmin uses to resolve the
// registry's current transfer-factory cid (internal/amulet.GetTransferFactory) via a
// self-transfer probe (sender == receiver == the newly onboarded admin). The value is
// irrelevant -- this call never submits anything on-ledger, it only resolves the registry's
// currently-committed factory contract id and requires a "direct" transferKind, which needs
// the receiver (here, the admin itself) to already have a standing TransferPreapproval --
// true by this point, since it was just created above.
const amuletFactoryProbeAmount = "1"

type deployOutput struct {
	ManagerID          int    `json:"managerId"`
	ManagerAddress     string `json:"managerAddress"`
	TransceiverAddress string `json:"transceiverAddress"`
	Admin              string `json:"admin"`
	InstrumentAdmin    string `json:"instrumentAdmin"`
	InstrumentID       string `json:"instrumentId"`
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

			tokenKind, err := parseTokenKind(cfg.TokenKind)
			if err != nil {
				return err
			}
			cfg.TokenKind = tokenKind
			if err := validateDeployConfig(cfg); err != nil {
				return err
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

			// "amulet" uses real Canton Coin instead of a local mock registry: it needs a
			// profile with a real DSO (the Amulet operator party), and the deployment's admin
			// becomes a validator-onboarded wallet user (replacing the old custody-party
			// split -- admin owns lock/unlock custody automatically now, see
			// Wormhole.Ntt.Manager's header). setupAmuletAdmin onboards the admin itself under
			// adminHint (proven machinery from the pre-rework setupCip56Custody, retargeted)
			// and caches the resulting party into state BEFORE resolveParty runs below, so
			// resolveParty's cache hit returns the SAME onboarded wallet party instead of
			// allocating an unrelated bare script party under the same hint.
			var instrumentAdminPtr *string
			var amuletFactoryCidPtr *string
			if cfg.TokenKind == "amulet" {
				if !prof.AmuletAvailable {
					return fmt.Errorf("deploy: amulet requires a profile with real Amulet (localnet)")
				}
				adminParty, dsoParty, factoryCid, err := setupAmuletAdmin(ctx, cmd, a, adminHint, prof)
				if err != nil {
					return fmt.Errorf("deploy: amulet setup: %w", err)
				}
				instrumentAdminPtr = &dsoParty
				amuletFactoryCidPtr = &factoryCid
				if s.Users == nil {
					s.Users = map[string]string{}
				}
				s.Users[adminHint] = adminParty
				if err := a.saveState(s); err != nil {
					return err
				}
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// The admin party is allocated (and granted actAs on auth-enabled profiles)
			// before deployRegistry/deployNtt submit as it -- see grantActAs. For "amulet" it
			// was already onboarded and cached above by setupAmuletAdmin; this just reads it
			// back (cache hit -- no fresh allocation, no re-grant).
			admin, err := resolveParty(cmd, a, s, adminHint)
			if err != nil {
				return err
			}

			// deployRegistry submits AS gg (it creates gg-signed mock factory contracts), so
			// it routes to gg's OWN participant -- not the default one deployNtt/setPeerOnLedger
			// use below (admin stays co-located with operator under this topology's "*"
			// wildcard, plan §8's bullet 2).
			ggRunner, ggCleanup, err := a.newScriptRunnerFor(ctx, participantRoleForParty(s, s.GuardianGovernance))
			if err != nil {
				return err
			}
			defer ggCleanup()

			a.vlogf(cmd, "deploy %q: deployRegistry (gg) = stand up the mock registry factory for mode=%s tokenKind=%s",
				deploymentName, cfg.Mode, cfg.TokenKind)
			var regOut deployRegistryOutput
			if err := ggRunner.Run(ctx, "Playground.Deploy:deployRegistry", deployRegistryInput{
				GuardianGovernance: s.GuardianGovernance,
				Admin:              admin,
				Mode:               cfg.Mode,
				TokenKind:          cfg.TokenKind,
				InstrumentNonce:    0,
			}, &regOut); err != nil {
				return fmt.Errorf("deploy: deployRegistry: %w", err)
			}

			a.vlogf(cmd, "deploy %q: deployNtt (admin) = resolve registries/factory → RegisterManager", deploymentName)
			var out deployOutput
			if err := runner.Run(ctx, "Playground.Deploy:deployNtt", deployInput{
				Operator:           s.Operator,
				GuardianGovernance: s.GuardianGovernance,
				Admin:              admin,
				Mode:               cfg.Mode,
				TokenKind:          cfg.TokenKind,
				TokenDecimals:      cfg.Decimals,
				InstrumentNonce:    0,
				InstrumentAdmin:    instrumentAdminPtr,
				AmuletFactoryCid:   amuletFactoryCidPtr,
				FactoryCid:         regOut.FactoryCid,
			}, &out); err != nil {
				return err
			}

			d := state.Deployment{
				Name:               deploymentName,
				ManagerID:          out.ManagerID,
				ManagerAddress:     out.ManagerAddress,
				TransceiverAddress: out.TransceiverAddress,
				Admin:              out.Admin, // the REGISTERING admin -- fixed forever, see state.Deployment's doc comment
				CurrentAdmin:       out.Admin, // == Admin at deploy time; diverges only after `admin accept-gg-vaa`
				Mode:               cfg.Mode,
				TokenKind:          cfg.TokenKind,
				TokenDecimals:      cfg.Decimals,
				Peers:              map[int]state.Peer{},
				InstrumentAdmin:    out.InstrumentAdmin,
				InstrumentID:       out.InstrumentID,
			}

			for _, p := range cfg.Peers {
				a.vlogf(cmd, "set peer chain=%d manager=%s transceiver=%s decimals=%d", p.Chain, p.Manager, p.Transceiver, p.Decimals)
				if err := setPeerOnLedger(ctx, a, runner, s.Operator, out.ManagerID, out.Admin, p.Chain, p.Manager, p.Transceiver, p.Decimals); err != nil {
					return fmt.Errorf("deploy: set peer chain %d: %w", p.Chain, err)
				}
				d.Peers[p.Chain] = state.Peer{ManagerAddress: p.Manager, TransceiverAddress: p.Transceiver, Decimals: p.Decimals}
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

// setupAmuletAdmin onboards an "amulet" deployment's admin as a validator wallet user (plan
// §4.3's spike, now live-verified against a real LocalNet validator -- an onboarded wallet
// party composes cleanly with Daml-script actAs AND with serving as an NttManager
// co-signatory at RegisterManager-creation time; see the plan's Phase 4 checkpoint for the
// narrow probe that proved this before deployNtt was built on top of it). Mirrors the
// pre-rework setupCip56Custody one-for-one, retargeted from a separate custody party to the
// admin itself (Wormhole.Ntt.Manager's custody is admin-owned by construction now, so the
// admin takes the old custody party's exact seat): OnboardWalletUser(adminHint) + grantActAs
// + the validator's own wallet Tap (pays the preapproval's creation fee) +
// CreateTransferPreapproval for the admin. Additionally resolves the registry's current
// transfer-factory cid (plan §4.4) via a self-transfer probe now that the admin's own
// preapproval exists, so deployNtt's RegisterManager call can commit a real factory instead
// of aborting.
func setupAmuletAdmin(ctx context.Context, cmd *cobra.Command, a *app, adminHint string, prof profile.Profile) (adminParty, dsoParty, factoryCid string, err error) {
	m, err := a.localNetManager()
	if err != nil {
		return "", "", "", err
	}
	dsoParty, err = m.DSOPartyID(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve DSO party: %w", err)
	}
	a.vlogf(cmd, "amulet: DSO party → %s", dsoParty)

	client := &amulet.Client{ValidatorBaseURL: prof.ValidatorBaseURL, Logf: a.verboseLogf()}

	a.vlogf(cmd, "amulet: onboarding admin wallet user %q", adminHint)
	adminParty, err = client.OnboardWalletUser(ctx, adminHint)
	if err != nil {
		return "", "", "", fmt.Errorf("onboard admin wallet user %q: %w", adminHint, err)
	}
	// setupAmuletAdmin's admin stays on the default participant (amulet wallet users are never
	// routed by --topology-config's partyHosting map, plan §3) -- role="" targets it.
	if err := grantActAs(cmd, a, "", adminParty); err != nil {
		return "", "", "", fmt.Errorf("grant actAs on admin party %s: %w", adminParty, err)
	}

	a.vlogf(cmd, "amulet: tapping the validator's own wallet (%s) before requesting the admin's preapproval (it pays the creation fee)", network.LocalNetWalletUser)
	if _, err := client.Tap(ctx, network.LocalNetWalletUser, amuletTapUSD); err != nil {
		return "", "", "", fmt.Errorf("tap validator wallet %s: %w", network.LocalNetWalletUser, err)
	}

	a.vlogf(cmd, "amulet: creating TransferPreapproval for admin %q (%s)", adminHint, adminParty)
	if _, err := client.CreateTransferPreapproval(ctx, adminHint); err != nil {
		return "", "", "", fmt.Errorf("create TransferPreapproval for admin %s: %w", adminParty, err)
	}

	// The admin now has its own standing preapproval, so a probe transfer TO the admin
	// resolves transferKind="direct" and yields the registry's current transfer-factory cid
	// -- this deployment's committed factory (RegisterManager just stores it; it never
	// fetches/exercises it, so no disclosure is needed at deploy time). sender == receiver
	// (a "self" transfer) resolves a different transferKind and does not exercise the
	// receiver-preapproval path real transfers use, so the probe's sender is the DSO party
	// instead -- any valid party works here (the call never submits anything, it only
	// resolves the registry's currently-committed factory + choice context for a
	// hypothetical transfer of this shape).
	a.vlogf(cmd, "amulet: resolving current transfer-factory cid (probe transfer to admin)")
	factory, err := client.GetTransferFactory(ctx, adminHint, amulet.TransferArgs{
		DSO:      dsoParty,
		Sender:   dsoParty,
		Receiver: adminParty,
		Amount:   amuletFactoryProbeAmount,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("resolve transfer-factory: %w", err)
	}

	return adminParty, dsoParty, factory.FactoryID, nil
}
