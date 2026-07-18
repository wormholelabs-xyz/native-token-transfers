package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// registerEmitterInput/registerEmitterOutput mirror Playground.Ops.daml's
// RegisterEmitterInput/RegisterEmitterOutput.
type registerEmitterInput struct {
	Operator string `json:"operator"`
	Owner    string `json:"owner"`
}

type registerEmitterOutput struct {
	EmitterID      int    `json:"emitterId"`
	EmitterAddress string `json:"emitterAddress"`
}

func newEmitterCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emitter",
		Short: "Manage standalone core-bridge emitters (not tied to an NTT deployment)",
	}
	cmd.AddCommand(newEmitterRegisterCmd(a))
	return cmd
}

func newEmitterRegisterCmd(a *app) *cobra.Command {
	var name, ownerHint string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new Emitter owned by a party hint",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			// Registering under a taken name would strand the old emitter's identity while
			// its contract stays live on the ledger; reject rather than silently overwrite.
			if existing, ok := s.Emitter(name); ok {
				return fmt.Errorf("emitter register: emitter %q already exists (emitterId %d)", name, existing.EmitterID)
			}
			owner, err := resolveParty(cmd, a, s, ownerHint)
			if err != nil {
				return err
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			var out registerEmitterOutput
			if err := runner.Run(ctx, "Playground.Ops:registerEmitter", registerEmitterInput{
				Operator: s.Operator,
				Owner:    owner,
			}, &out); err != nil {
				return err
			}

			s.Emitters[name] = state.Emitter{EmitterID: out.EmitterID, Address: out.EmitterAddress, Owner: owner}
			if err := a.saveState(s); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "emitter: name=%s emitterId=%d emitterAddress=%s owner=%s\n",
				name, out.EmitterID, out.EmitterAddress, owner)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "CLI-local name to key the emitter under in state")
	cmd.Flags().StringVar(&ownerHint, "owner", "", "party hint for the emitter's owner")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("owner")
	return cmd
}
