package main

import (
	"encoding/json"
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
	// Remote carries a pre-fetched Playground.Prepare:preparePublish RemoteSeam, as raw JSON --
	// see transferOutInput.Remote's doc comment (transfer.go).
	Remote json.RawMessage `json:"remote"`
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

			// PublishMessage submits as the emitter's owner, so it routes to the owner's
			// home participant (plan §3), resolved via a reverse lookup of em.Owner (Emitter
			// only persists the party id, not the allocating hint). The DATA this needs
			// (Emitter/CoreState) lives on operator's own participant -- operator co-signs
			// both, regardless of who registered the standalone emitter (contrast
			// transferOut/receiveVaa, whose data owner is guardianGovernance -- see
			// remote.go's doc comment).
			actorRole := participantRoleForParty(s, em.Owner)

			// Record the actor's participant role as the --strict-participant-isolation
			// baseline (runner.go), immediately after it is known and before prepareRemoteSeam
			// below (which may need to cross a participant boundary).
			a.isolationBaselineRole = actorRole
			a.isolationBaselineSet = true

			dataOwnerRole := participantRoleForParty(s, s.Operator)
			remoteSeam, err := prepareRemoteSeam(ctx, cmd, a, s, actorRole, dataOwnerRole, "Playground.Prepare:preparePublish", "publish", func(templates []string) any {
				return preparePublishInput{
					Operator:          s.Operator,
					EmitterID:         em.EmitterID,
					DiscloseTemplates: templates,
				}
			})
			if err != nil {
				return fmt.Errorf("publish: prepare remote disclosure: %w", err)
			}

			runner, cleanup, err := a.newScriptRunnerFor(ctx, actorRole)
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "publish: emitter %q (emitterId=%d) → PublishMessage as owner=%s (feeAllocation=None under the fee-0 assumption)", emitterName, em.EmitterID, em.Owner)
			var out publishMessageOutput
			if err := runner.Run(ctx, "Playground.Ops:publishMessage", publishMessageInput{
				Operator:         s.Operator,
				Owner:            em.Owner,
				EmitterID:        em.EmitterID,
				Payload:          payloadHex,
				Nonce:            nonce,
				ConsistencyLevel: consistencyLevel,
				Remote:           remoteSeam,
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
				a.vlogf(cmd, "sign: digest=keccak256(keccak256(body)) over %d-byte body", len(vaaHex)/2-vaaHeaderLen)
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
