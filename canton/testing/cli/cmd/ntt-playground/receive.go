package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
)

// receiveVaaInput/receiveVaaOutput mirror Playground.Ops.daml's ReceiveVaaInput/Output.
type receiveVaaInput struct {
	Operator  string              `json:"operator"`
	ManagerID int                 `json:"managerId"`
	Admin     string              `json:"admin"`
	TokenKind string              `json:"tokenKind"`
	Executor  string              `json:"executor"`
	Recipient string              `json:"recipient"`
	VaaBytes  string              `json:"vaaBytes"`
	PubKeys   []ledger.PubKeyHint `json:"pubKeys"`
	// Remote carries a pre-fetched Playground.Prepare:prepareReceive RemoteSeam, as raw JSON --
	// see transferOutInput.Remote's doc comment (transfer.go).
	Remote json.RawMessage `json:"remote"`
}

type receiveVaaOutput struct {
	RecipientChain int `json:"recipientChain"`
	Amount         int `json:"amount"`
	Decimals       int `json:"decimals"`
}

func newReceiveCmd(a *app) *cobra.Command {
	var deployment, vaaHex, recipientHint, executorHint, pubKeyHex string

	cmd := &cobra.Command{
		Use:   "receive",
		Short: "Relay a guardian-signed inbound VAA through NttManager.Mint/Release (recipient must have run `preapprove` first, both modes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("receive: unknown deployment %q", deployment)
			}
			if d.TokenKind == "amulet" {
				return fmt.Errorf("receive: receiving is out of scope for real Amulet (amulet)")
			}
			recipientParty, err := resolveParty(cmd, a, s, recipientHint)
			if err != nil {
				return err
			}
			// executor defaults to the Operator (stays on the default/app-provider
			// participant, executorRole=""); an explicit --executor routes the submit to
			// THAT hint's own participant instead (plan §3).
			executorParty := s.Operator
			executorRole := ""
			if executorHint != "" {
				executorParty, err = resolveParty(cmd, a, s, executorHint)
				if err != nil {
					return err
				}
				executorRole = s.UserParticipants[executorHint]
			}
			if pubKeyHex == "" {
				return fmt.Errorf("receive: --pubkey is required (the signing guardian's 65-byte uncompressed pubkey)")
			}

			// The data owner for a Mock-kind prepare fetch is gg's OWN participant, not
			// operator's -- see remote.go's doc comment and transfer.go's identical reasoning.
			ownerRole := participantRoleForParty(s, s.GuardianGovernance)
			remoteSeam, err := prepareRemoteSeam(ctx, cmd, a, s, executorRole, ownerRole, "Playground.Prepare:prepareReceive", func(templates []string) any {
				return prepareReceiveInput{
					GuardianGovernance: s.GuardianGovernance,
					ManagerID:          d.ManagerID,
					Recipient:          recipientParty,
					VaaBytes:           vaaHex,
					DiscloseTemplates:  templates,
				}
			})
			if err != nil {
				return fmt.Errorf("receive: prepare remote disclosure: %w", err)
			}

			runner, cleanup, err := a.newScriptRunnerFor(ctx, executorRole)
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "receive: relaying %d-byte VAA through NttManager.Receive (replay-trie covering node resolved in-script)", len(vaaHex)/2)
			var out receiveVaaOutput
			if err := runner.Run(ctx, "Playground.Ops:receiveVaa", receiveVaaInput{
				Operator:  s.Operator,
				ManagerID: d.ManagerID,
				Admin:     d.Admin,
				TokenKind: d.TokenKind,
				Executor:  executorParty,
				Recipient: recipientParty,
				VaaBytes:  vaaHex,
				PubKeys:   []ledger.PubKeyHint{{Index: 0, Key: pubKeyHex}},
				Remote:    remoteSeam,
			}, &out); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "receive: recipientChain=%d amount=%d decimals=%d\n", out.RecipientChain, out.Amount, out.Decimals)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().StringVar(&vaaHex, "vaa", "", "hex-encoded signed VAA to relay")
	cmd.Flags().StringVar(&recipientHint, "recipient", "", "party hint for the transfer's recipient")
	cmd.Flags().StringVar(&executorHint, "executor", "", "party hint for the relaying executor (default: operator)")
	cmd.Flags().StringVar(&pubKeyHex, "pubkey", "", "the signing guardian's 65-byte uncompressed public key, hex")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("vaa")
	_ = cmd.MarkFlagRequired("recipient")
	_ = cmd.MarkFlagRequired("pubkey")
	return cmd
}
