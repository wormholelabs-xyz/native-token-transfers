package main

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/guardian"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// initInput/initOutput mirror Playground.Init.daml's InitInput/InitOutput JSON shapes.
type initInput struct {
	Operator           string   `json:"operator"`
	GuardianGovernance string   `json:"guardianGovernance"`
	GuardianObserver   string   `json:"guardianObserver"`
	GuardianAddresses  []string `json:"guardianAddresses"`
}

type initOutput struct {
	Operator           string `json:"operator"`
	GuardianGovernance string `json:"guardianGovernance"`
	GuardianObserver   string `json:"guardianObserver"`
}

func newInitCmd(a *app) *cobra.Command {
	var guardianKeyHex string
	var fee uint64

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a fresh CoreState + registries with a 1/1 guardian set",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var key *guardian.Key
			var err error
			if guardianKeyHex != "" {
				key, err = guardian.KeyFromHex(guardianKeyHex)
			} else {
				key, err = guardian.NewRandomKey()
			}
			if err != nil {
				return err
			}
			addr := key.Address()
			addrHex := hex.EncodeToString(addr[:])
			// Address only -- the private key never appears in any log, verbose or not.
			a.vlogf(cmd, "guardian: 1/1 set, address=%s", addrHex)

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// Parties are allocated (and actAs rights granted, on auth-enabled profiles)
			// BEFORE initPlayground submits as them -- see grantActAs.
			s := state.New()
			s.Profile = string(a.profile)
			operator, err := resolveParty(cmd, a, s, "Operator")
			if err != nil {
				return err
			}
			guardianGovernance, err := resolveParty(cmd, a, s, "GuardianGovernance")
			if err != nil {
				return err
			}
			guardianObserver, err := resolveParty(cmd, a, s, "GuardianObserver")
			if err != nil {
				return err
			}

			a.vlogf(cmd, "init: creating genesis contracts as operator+guardianGovernance: CoreState (guardian set idx 0), EmitterRegistry, ReplayRootRegistry, NttManagerRegistry")
			var out initOutput
			if err := runner.Run(ctx, "Playground.Init:initPlayground", initInput{
				Operator:           operator,
				GuardianGovernance: guardianGovernance,
				GuardianObserver:   guardianObserver,
				GuardianAddresses:  []string{addrHex},
			}, &out); err != nil {
				return err
			}
			s.Operator = out.Operator
			s.GuardianGovernance = out.GuardianGovernance
			s.GuardianObserver = out.GuardianObserver
			s.Guardian = state.Guardian{PrivateKeyHex: key.HexPrivate(), Address: addrHex}

			if fee > 0 {
				seq := s.NextGuardianSequence("governance", guardian.DefaultGovernanceChain)
				a.vlogf(cmd, "init: signing SetMessageFee governance VAA (fee=%d, seq=%d) and applying via SubmitGovernanceVAA", fee, seq)
				vaa, err := guardian.SignSetMessageFee(key, guardian.GovernanceParams{Sequence: seq}, 72, fee)
				if err != nil {
					return err
				}
				var applyOut applyGovernanceOutput
				if err := runner.Run(ctx, "Playground.Ops:applyGovernance", applyGovernanceInput{
					Operator: out.Operator,
					VaaBytes: hex.EncodeToString(vaa),
					PubKeys:  []ledger.PubKeyHint{{Index: 0, Key: hex.EncodeToString(key.PubKeyUncompressed())}},
				}, &applyOut); err != nil {
					return fmt.Errorf("init: applying initial fee %d failed: %w", fee, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "init: message fee set to %d\n", applyOut.MessageFee)
			}

			if err := a.saveState(s); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "init: operator=%s guardianGovernance=%s guardian=%s\n",
				out.Operator, out.GuardianGovernance, addrHex)
			return nil
		},
	}
	cmd.Flags().StringVar(&guardianKeyHex, "guardian-key", "", "hex secp256k1 private key for the 1/1 guardian (default: generate a fresh key)")
	cmd.Flags().Uint64Var(&fee, "fee", 0, "initial message fee to set via a self-signed governance VAA (0 = leave at genesis default)")
	return cmd
}
