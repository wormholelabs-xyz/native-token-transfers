package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// fundUserInput/fundUserOutput mirror Playground.Ops.daml's FundUserInput/FundUserOutput.
type fundUserInput struct {
	Admin  string         `json:"admin"`
	Owner  string         `json:"owner"`
	Amount decimalLiteral `json:"amount"` // Daml Decimal serializes as a bare JSON number, not a quoted string
}

type fundUserOutput struct {
	HoldingCid string `json:"holdingCid"`
}

// transferOutInput/transferOutOutput mirror Playground.Ops.daml's TransferOutInput/Output.
type transferOutInput struct {
	Operator         string   `json:"operator"`
	ManagerID        int      `json:"managerId"`
	Admin            string   `json:"admin"`
	TokenKind        string   `json:"tokenKind"`
	User             string   `json:"user"`
	RecipientChain   int      `json:"recipientChain"`
	RecipientAddress string   `json:"recipientAddress"`
	SourceToken      string   `json:"sourceToken"`
	RawAmount        int64    `json:"rawAmount"`
	Nonce            int      `json:"nonce"`
	ConsistencyLevel int      `json:"consistencyLevel"`
	InputHoldingCids []string `json:"inputHoldingCids"`
}

type transferOutOutput struct {
	OutboundSequence int    `json:"outboundSequence"`
	EmitterChain     int    `json:"emitterChain"`
	EmitterAddress   string `json:"emitterAddress"`
	Nonce            int    `json:"nonce"`
	ConsistencyLevel int    `json:"consistencyLevel"`
	Payload          string `json:"payload"`
}

func newTransferCmd(a *app) *cobra.Command {
	var deployment, userHint, recipientAddressHex, sourceTokenHex string
	var chain int
	var amount int64
	var nonce, consistencyLevel int
	var sign bool

	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Send an outbound NTT transfer (Transfer), optionally signing the resulting VAA as the guardian",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("transfer: unknown deployment %q", deployment)
			}
			userParty, err := resolveParty(cmd, a, s, userHint)
			if err != nil {
				return err
			}
			a.vlogf(cmd, "transfer: user %q → %s, deployment %q (managerId=%d)", userHint, userParty, deployment, d.ManagerID)

			if sourceTokenHex == "" {
				sourceTokenHex = strings.Repeat("00", 32)
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// Daml's `[ContractId Holding]` needs a JSON array, never `null` -- must start
			// non-nil (a nil Go slice marshals to `null`).
			holdingCids := []string{}
			if d.TokenKind != "mock-admin-signed" {
				decimals := wire.TrimDecimals(d.TokenDecimals)
				a.vlogf(cmd, "transfer: funding sender with %s via Playground.Ops:fundUser (holding cid feeds transferOut)", formatDecimal(amount, decimals))
				var fundOut fundUserOutput
				if err := runner.Run(ctx, "Playground.Ops:fundUser", fundUserInput{
					Admin:  d.Admin,
					Owner:  userParty,
					Amount: decimalLiteral(formatDecimal(amount, decimals)),
				}, &fundOut); err != nil {
					return fmt.Errorf("transfer: funding user for a Cip56 deployment: %w", err)
				}
				holdingCids = []string{fundOut.HoldingCid}
			}

			var out transferOutOutput
			if err := runner.Run(ctx, "Playground.Ops:transferOut", transferOutInput{
				Operator:         s.Operator,
				ManagerID:        d.ManagerID,
				Admin:            d.Admin,
				TokenKind:        d.TokenKind,
				User:             userParty,
				RecipientChain:   chain,
				RecipientAddress: recipientAddressHex,
				SourceToken:      sourceTokenHex,
				RawAmount:        amount,
				Nonce:            nonce,
				ConsistencyLevel: consistencyLevel,
				InputHoldingCids: holdingCids,
			}, &out); err != nil {
				return err
			}

			a.vlogf(cmd, "transfer: recomputed published message: sequence=%d emitter=%d/%s", out.OutboundSequence, out.EmitterChain, out.EmitterAddress)
			fmt.Fprintf(cmd.OutOrStdout(), "transfer: sequence=%d emitterChain=%d emitterAddress=%s payload=%s\n",
				out.OutboundSequence, out.EmitterChain, out.EmitterAddress, out.Payload)

			if sign {
				vaaHex, err := signRecomputedVAA(s, out.EmitterChain, out.EmitterAddress,
					uint64(out.OutboundSequence), out.Nonce, out.ConsistencyLevel, out.Payload)
				if err != nil {
					return fmt.Errorf("transfer --sign: %w", err)
				}
				a.vlogf(cmd, "sign: digest=keccak256(keccak256(body)) over %d-byte body", len(vaaHex)/2-vaaHeaderLen)
				fmt.Fprintf(cmd.OutOrStdout(), "transfer --sign: vaa=%s\n", vaaHex)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().StringVar(&userHint, "user", "", "party hint for the sending user")
	cmd.Flags().IntVar(&chain, "chain", 0, "recipient (peer) chain id")
	cmd.Flags().StringVar(&recipientAddressHex, "recipient-address", "", "32-byte hex recipient address on the peer chain")
	cmd.Flags().StringVar(&sourceTokenHex, "source-token", "", "32-byte hex source token address (default: zero)")
	cmd.Flags().Int64Var(&amount, "amount", 0, "raw amount to transfer")
	cmd.Flags().IntVar(&nonce, "nonce", 0, "transceiver nonce")
	cmd.Flags().IntVar(&consistencyLevel, "consistency-level", 0, "transceiver consistency level")
	cmd.Flags().BoolVar(&sign, "sign", false, "also sign the resulting VAA with the playground's guardian key")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("chain")
	_ = cmd.MarkFlagRequired("recipient-address")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}

// formatDecimal renders rawAmount (an integer at `decimals` scale) as a fixed-point decimal
// string, e.g. formatDecimal(1_000_000, 8) == "0.01" -- avoiding float64 for exactness.
func formatDecimal(rawAmount int64, decimals int) string {
	if decimals <= 0 {
		return fmt.Sprintf("%d", rawAmount)
	}
	s := fmt.Sprintf("%0*d", decimals+1, rawAmount)
	intPart := s[:len(s)-decimals]
	fracPart := s[len(s)-decimals:]
	return intPart + "." + fracPart
}
