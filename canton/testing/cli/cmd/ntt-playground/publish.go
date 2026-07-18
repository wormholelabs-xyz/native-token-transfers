package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// publishMessageInput/publishMessageOutput mirror Playground.Ops.daml's
// PublishMessageInput/PublishMessageOutput.
type publishMessageInput struct {
	Operator         string `json:"operator"`
	Owner            string `json:"owner"`
	EmitterID        int    `json:"emitterId"`
	Payload          string `json:"payload"`
	Nonce            int    `json:"nonce"`
	ConsistencyLevel int    `json:"consistencyLevel"`
}

type publishMessageOutput struct {
	Sequence       int    `json:"sequence"`
	EmitterChain   int    `json:"emitterChain"`
	EmitterAddress string `json:"emitterAddress"`
	Payload        string `json:"payload"`
}

func newPublishCmd(a *app) *cobra.Command {
	var emitterName, payloadHex string
	var nonce, consistencyLevel int
	var sign bool

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish an arbitrary core message from a registered emitter, optionally signing the resulting VAA",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			em, ok := s.Emitter(emitterName)
			if !ok {
				return fmt.Errorf("publish: unknown emitter %q -- run `emitter register` first", emitterName)
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			var out publishMessageOutput
			if err := runner.Run(ctx, "Playground.Ops:publishMessage", publishMessageInput{
				Operator:         s.Operator,
				Owner:            em.Owner,
				EmitterID:        em.EmitterID,
				Payload:          payloadHex,
				Nonce:            nonce,
				ConsistencyLevel: consistencyLevel,
			}, &out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "publish: sequence=%d emitterChain=%d emitterAddress=%s payload=%s\n",
				out.Sequence, out.EmitterChain, out.EmitterAddress, out.Payload)

			if sign {
				// Sign the ledger-normalized payload the script returned, not the raw flag
				// value -- the watcher signs what was published.
				vaaHex, err := signRecomputedVAA(s, out.EmitterChain, out.EmitterAddress,
					uint64(out.Sequence), nonce, consistencyLevel, out.Payload)
				if err != nil {
					return fmt.Errorf("publish --sign: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "publish --sign: vaa=%s\n", vaaHex)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&emitterName, "emitter", "", "registered emitter name (see `emitter register`)")
	cmd.Flags().StringVar(&payloadHex, "payload", "", "hex-encoded payload (max 750 bytes)")
	cmd.Flags().IntVar(&nonce, "nonce", 0, "message nonce")
	cmd.Flags().IntVar(&consistencyLevel, "consistency-level", 0, "message consistency level")
	cmd.Flags().BoolVar(&sign, "sign", false, "also sign the published message as a VAA with the playground's guardian key")
	_ = cmd.MarkFlagRequired("emitter")
	_ = cmd.MarkFlagRequired("payload")
	return cmd
}
