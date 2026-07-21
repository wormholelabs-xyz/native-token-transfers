package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/amulet"
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
	Operator         string          `json:"operator"`
	ManagerID        int             `json:"managerId"`
	Admin            string          `json:"admin"`
	TokenKind        string          `json:"tokenKind"`
	User             string          `json:"user"`
	RecipientChain   int             `json:"recipientChain"`
	RecipientAddress string          `json:"recipientAddress"`
	SourceToken      string          `json:"sourceToken"`
	RawAmount        int64           `json:"rawAmount"`
	Nonce            int             `json:"nonce"`
	ConsistencyLevel int             `json:"consistencyLevel"`
	InputHoldingCids []string        `json:"inputHoldingCids"`
	Amulet           *amuletSeamJSON `json:"amulet"` // "cip56-custody" only; null otherwise
}

// amuletSeamJSON/disclosedContractInJSON mirror Playground.Amulet.daml's
// AmuletSeam/DisclosedContractIn. They carry the Amulet registry's resolved TransferFactory
// (Splice's contract for building an Amulet transfer): its contract id, choice context, and
// disclosed contracts. These are fetched fresh for each transfer via
// internal/amulet.GetTransferFactory and passed through verbatim; the choice context stays as
// raw Daml-JSON, untouched by Go.
type amuletSeamJSON struct {
	FactoryCid    string                    `json:"factoryCid"`
	ChoiceContext json.RawMessage           `json:"choiceContext"`
	Disclosed     []disclosedContractInJSON `json:"disclosed"`
}

type disclosedContractInJSON struct {
	TemplateID string `json:"templateId"`
	ContractID string `json:"contractId"`
	Blob       string `json:"blob"`
}

// toAmuletSeamJSON adapts the registry's disclosed-contract shape for `dpm script`. It drops
// SynchronizerID, because Daml Script's Disclosure type has no field for it, and re-encodes
// createdEventBlob from base64 (the registry's wire format) to hex. The script's
// Disclosure.blob field must be hex: passing the registry's base64 straight through fails with
// "cannot parse HexString". Both encodings carry the same bytes.
func toAmuletSeamJSON(f amulet.TransferFactory) (*amuletSeamJSON, error) {
	disclosed := make([]disclosedContractInJSON, len(f.DisclosedContracts))
	for i, dc := range f.DisclosedContracts {
		raw, err := base64.StdEncoding.DecodeString(dc.CreatedEventBlob)
		if err != nil {
			return nil, fmt.Errorf("amulet: decode createdEventBlob (base64) for %s: %w", dc.TemplateID, err)
		}
		disclosed[i] = disclosedContractInJSON{TemplateID: dc.TemplateID, ContractID: dc.ContractID, Blob: hex.EncodeToString(raw)}
	}
	return &amuletSeamJSON{FactoryCid: f.FactoryID, ChoiceContext: f.ChoiceContextData, Disclosed: disclosed}, nil
}

type transferOutOutput struct {
	OutboundSequence int    `json:"outboundSequence"`
	EmitterChain     int    `json:"emitterChain"`
	EmitterAddress   string `json:"emitterAddress"`
	Nonce            int    `json:"nonce"`
	ConsistencyLevel int    `json:"consistencyLevel"`
	Payload          string `json:"payload"`
}

// defaultTapUSD is the sender's default tap amount for a cip56-custody transfer, in USD. The
// devnet Amulet price is not fixed on LocalNet, so the resulting Canton Coin amount is not
// asserted; the value is just a cushion. "0" skips the tap, e.g. a second transfer by an
// already-funded sender.
const defaultTapUSD = "100"

func newTransferCmd(a *app) *cobra.Command {
	var deployment, userHint, recipientAddressHex, sourceTokenHex, tapUSD string
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
			prof, err := a.resolvedProfile()
			if err != nil {
				return err
			}

			if sourceTokenHex == "" {
				sourceTokenHex = strings.Repeat("00", 32)
			}

			// cip56-custody uses real Canton Coin (Amulet): the sender must be a validator
			// wallet user, since only wallet users can be tapped, never a bare
			// script-allocated party. Every other token kind is unchanged.
			var userParty string
			var amuletClient *amulet.Client
			if d.TokenKind == "cip56-custody" {
				if !prof.AmuletAvailable {
					return fmt.Errorf("transfer: cip56-custody requires a profile with real Amulet (localnet)")
				}
				amuletClient = &amulet.Client{ValidatorBaseURL: prof.ValidatorBaseURL, Logf: a.verboseLogf()}
				if cached, ok := s.Users[userHint]; ok {
					userParty = cached
					a.vlogf(cmd, "party %q: cached → %s", userHint, userParty)
				} else {
					a.vlogf(cmd, "transfer: onboarding wallet user %q (cip56-custody sender)", userHint)
					party, err := amuletClient.OnboardWalletUser(ctx, userHint)
					if err != nil {
						return fmt.Errorf("transfer: onboard sender wallet user %q: %w", userHint, err)
					}
					if err := grantActAs(cmd, a, party); err != nil {
						return fmt.Errorf("transfer: grant actAs on sender party %s: %w", party, err)
					}
					s.Users[userHint] = party
					if err := a.saveState(s); err != nil {
						return err
					}
					userParty = party
				}
				if tapUSD != "0" {
					a.vlogf(cmd, "transfer: tapping %s USD for sender %q", tapUSD, userHint)
					if _, err := amuletClient.Tap(ctx, userHint, tapUSD); err != nil {
						return fmt.Errorf("transfer: tap sender %q: %w", userHint, err)
					}
				}
			} else {
				userParty, err = resolveParty(cmd, a, s, userHint)
				if err != nil {
					return err
				}
			}
			a.vlogf(cmd, "transfer: user %q → %s, deployment %q (managerId=%d)", userHint, userParty, deployment, d.ManagerID)

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			// Daml's `[ContractId Holding]` needs a JSON array, never `null` -- must start
			// non-nil (a nil Go slice marshals to `null`).
			holdingCids := []string{}
			if d.TokenKind != "mock-admin-signed" && d.TokenKind != "cip56-custody" {
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

			// cip56-custody resolves the real transfer factory and choice context fresh for
			// this call and never caches them, so a submit failure can be retried against a
			// current factory. It leaves inputHoldingCids empty so Playground.Amulet:amuletHoldings
			// enumerates the sender's unlocked Amulet in-script.
			var amuletSeam *amuletSeamJSON
			if d.TokenKind == "cip56-custody" {
				amountDecimal := formatDecimal(amount, d.TokenDecimals) // full instrument scale (e.g. 10 decimals for Amulet), not the 8-decimal wire trim
				a.vlogf(cmd, "transfer: resolving real transfer-factory (sender=%s receiver=%s amount=%s)", userParty, d.CustodyParty, amountDecimal)
				factory, err := amuletClient.GetTransferFactory(ctx, userHint, amulet.TransferArgs{
					DSO:      d.InstrumentAdmin,
					Sender:   userParty,
					Receiver: d.CustodyParty,
					Amount:   amountDecimal,
				})
				if err != nil {
					return fmt.Errorf("transfer: resolve transfer-factory: %w", err)
				}
				a.vlogf(cmd, "transfer: transfer-factory %s resolved (transferKind=direct, %d disclosed contracts)", factory.FactoryID, len(factory.DisclosedContracts))
				amuletSeam, err = toAmuletSeamJSON(factory)
				if err != nil {
					return fmt.Errorf("transfer: %w", err)
				}
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
				Amulet:           amuletSeam,
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
	cmd.Flags().StringVar(&tapUSD, "tap-usd", defaultTapUSD, "cip56-custody only: USD amount to tap for the sender before transferring (\"0\" skips)")
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
