package main

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/guardian"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// proposeGenesisInput/proposeGenesisOutput mirror Playground.Init.daml's
// ProposeGenesisInput/ProposeGenesisOutput JSON shapes -- the operator-only half of genesis
// (see the module header for why genesis is a propose/accept pair, not a single co-signed
// create, post participant-split).
type proposeGenesisInput struct {
	Operator           string   `json:"operator"`
	GuardianGovernance string   `json:"guardianGovernance"`
	GuardianObserver   string   `json:"guardianObserver"`
	GuardianAddresses  []string `json:"guardianAddresses"`
}

type proposeGenesisOutput struct {
	Operator           string `json:"operator"`
	GuardianGovernance string `json:"guardianGovernance"`
	GuardianObserver   string `json:"guardianObserver"`
	ProposalCid        string `json:"proposalCid"`
}

// acceptGenesisInput/acceptGenesisOutput mirror Playground.Init.daml's
// AcceptGenesisInput/AcceptGenesisOutput JSON shapes -- guardianGovernance's half of genesis,
// the only step that actually creates Wormhole.Core.State:CoreState.
type acceptGenesisInput struct {
	GuardianGovernance string `json:"guardianGovernance"`
	ProposalCid        string `json:"proposalCid"`
}

type acceptGenesisOutput struct {
	CoreStateCid string `json:"coreStateCid"`
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

			// Parties are allocated (and actAs rights granted, on auth-enabled profiles)
			// BEFORE the genesis scripts submit as them -- see grantActAs. resolveParty
			// routes each hint to its own participant (plan §3) and records the resolved
			// role in s.UserParticipants, which is what the two genesis submits below use
			// to pick the right participant for each half.
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

			// Genesis is a propose/accept pair (plan §5.1): no single participant hosts both
			// operator and guardianGovernance post-split, so no single `dpm script`
			// invocation can submit as both. proposeGenesis runs on operator's own
			// participant (operator-only authority); acceptGenesis runs on
			// guardianGovernance's own participant and is the step that actually creates
			// CoreState AND the NTT root NttGovernance -- one ceremony, gg signs once for
			// both roots (see Playground.Genesis's header).
			operatorRunner, operatorCleanup, err := a.newScriptRunnerFor(ctx, s.UserParticipants["Operator"])
			if err != nil {
				return err
			}
			defer operatorCleanup()

			a.vlogf(cmd, "init: proposing genesis as operator: CoreStateBootstrapProposal (guardian set idx 0), EmitterRegistry, ReplayRootRegistry")
			var proposeOut proposeGenesisOutput
			if err := operatorRunner.Run(ctx, "Playground.Init:proposeGenesis", proposeGenesisInput{
				Operator:           operator,
				GuardianGovernance: guardianGovernance,
				GuardianObserver:   guardianObserver,
				GuardianAddresses:  []string{addrHex},
			}, &proposeOut); err != nil {
				return err
			}

			ggRunner, ggCleanup, err := a.newScriptRunnerFor(ctx, s.UserParticipants["GuardianGovernance"])
			if err != nil {
				return err
			}
			defer ggCleanup()

			a.vlogf(cmd, "init: accepting genesis as guardianGovernance: creating CoreState + NttGovernance")
			var acceptOut acceptGenesisOutput
			if err := ggRunner.Run(ctx, "Playground.Init:acceptGenesis", acceptGenesisInput{
				GuardianGovernance: guardianGovernance,
				ProposalCid:        proposeOut.ProposalCid,
			}, &acceptOut); err != nil {
				return err
			}

			s.Operator = proposeOut.Operator
			s.GuardianGovernance = proposeOut.GuardianGovernance
			s.GuardianObserver = proposeOut.GuardianObserver
			s.Guardian = state.Guardian{PrivateKeyHex: key.HexPrivate(), Address: addrHex}

			if fee > 0 {
				seq := s.NextGuardianSequence("governance", guardian.DefaultGovernanceChain)
				a.vlogf(cmd, "init: signing SetMessageFee governance VAA (fee=%d, seq=%d) and applying via SubmitGovernanceVAA", fee, seq)
				vaa, err := guardian.SignSetMessageFee(key, guardian.GovernanceParams{Sequence: seq}, wire.CantonChainID, fee)
				if err != nil {
					return err
				}
				var applyOut applyGovernanceOutput
				if err := operatorRunner.Run(ctx, "Playground.Ops:applyGovernance", applyGovernanceInput{
					Operator: proposeOut.Operator,
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
				proposeOut.Operator, proposeOut.GuardianGovernance, addrHex)
			return nil
		},
	}
	cmd.Flags().StringVar(&guardianKeyHex, "guardian-key", "", "hex secp256k1 private key for the 1/1 guardian (default: generate a fresh key)")
	cmd.Flags().Uint64Var(&fee, "fee", 0, "initial message fee to set via a self-signed governance VAA (0 = leave at genesis default)")
	return cmd
}
