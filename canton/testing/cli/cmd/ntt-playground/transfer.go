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
	GuardianGovernance string         `json:"guardianGovernance"`
	Admin              string         `json:"admin"`
	Owner              string         `json:"owner"`
	Amount             decimalLiteral `json:"amount"` // Daml Decimal serializes as a bare JSON number, not a quoted string
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
	Amulet           *amuletSeamJSON `json:"amulet"` // "amulet" only; null otherwise
	// Remote carries a pre-fetched Playground.Prepare:prepareTransferOut RemoteSeam, as raw
	// JSON -- nil marshals to `null` (Daml's None), the unchanged same-participant fast path.
	// See cmd/ntt-playground/remote.go: the CLI never parses or constructs this value, only
	// relays it between two `dpm script` calls.
	Remote json.RawMessage `json:"remote"`
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

// defaultTapUSD is the sender's default tap amount for an "amulet" transfer, in USD. The
// devnet Amulet price is not fixed on LocalNet, so the resulting Canton Coin amount is not
// asserted; the value is just a cushion. "0" skips the tap, e.g. a second transfer by an
// already-funded sender.
const defaultTapUSD = "100"

func newTransferCmd(a *app) *cobra.Command {
	var deployment, userHint, recipientAddressHex, sourceTokenHex, tapUSD string
	var chain int
	var amount int64
	var nonce, consistencyLevel int
	var sign, noFund bool

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

			// "amulet" uses real Canton Coin (Amulet): the sender must be a validator wallet
			// user, since only wallet users can be tapped, never a bare script-allocated
			// party. The "mock" kind is unchanged.
			var userParty string
			var amuletClient *amulet.Client
			if d.TokenKind == "amulet" {
				if !prof.AmuletAvailable {
					return fmt.Errorf("transfer: amulet requires a profile with real Amulet (localnet)")
				}
				amuletClient = &amulet.Client{ValidatorBaseURL: prof.ValidatorBaseURL, Logf: a.verboseLogf()}
				if cached, ok := s.Users[userHint]; ok {
					userParty = cached
					a.vlogf(cmd, "party %q: cached → %s", userHint, userParty)
				} else {
					a.vlogf(cmd, "transfer: onboarding wallet user %q (amulet sender)", userHint)
					party, err := amuletClient.OnboardWalletUser(ctx, userHint)
					if err != nil {
						return fmt.Errorf("transfer: onboard sender wallet user %q: %w", userHint, err)
					}
					if err := grantActAs(cmd, a, "", party); err != nil {
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

			// Record the actor's participant role as the --strict-participant-isolation
			// baseline (runner.go), immediately after it is known and before any call that
			// might cross a participant boundary (the preapprove/fundUser/prepareRemoteSeam
			// calls below). s.UserParticipants[userHint] is "" for an "amulet" sender (never
			// routed through resolveParty), matching how prepareRemoteSeam's own actorRole
			// normalization treats it -- "" means the profile's default participant, not a
			// distinct, unrouted one.
			a.isolationBaselineRole = s.UserParticipants[userHint]
			a.isolationBaselineSet = true

			// Daml's `[ContractId Holding]` needs a JSON array, never `null` -- must start
			// non-nil (a nil Go slice marshals to `null`).
			holdingCids := []string{}
			// --strict-participant-isolation implies --no-fund: fundUser is a gg-side submit
			// (gg is the mint's sole signatory, see Playground.Ops:fundUser) that no disclosure
			// service can stand in for -- a DisclosedContract grants visibility, never
			// authority. So a participant-isolated sender skips this block entirely and leaves
			// holdingCids empty; Playground.Ops:transferOut's Mock branch then enumerates the
			// sender's own holdings in-script (Playground.Ops:mockHoldings, design doc §5.5),
			// which needs no cross-participant disclosure at all. Use `fund` (fund.go) first,
			// on the operator's own CLI, to mint them.
			skipFund := noFund || a.strictParticipantIsolation
			if d.TokenKind == "mock" && !skipFund {
				// fundUser mints via the sender's own standing DepositPreapproval (the
				// preapproval-based faucet Deposit -- see Playground.Ops:fundUser), so the CLI
				// ensures it exists first; idempotent if the sender already opted in. This is
				// ALWAYS the burn-mint-style DepositPreapproval, even on a lock-unlock
				// deployment -- fundUser is the shared funding faucet for both modes (a
				// lock-unlock mock sender needs no MockTransferPreapproval of its own to lock;
				// only the admin-as-custodian needs one, self-created at deploy time), so
				// passing d.Mode here would ensure the wrong template for lock-unlock deployments.
				// preapprove is a self-signed create by the sender, so it routes to the
				// sender's own participant (plan §3).
				preRunner, preCleanup, err := a.newScriptRunnerFor(ctx, s.UserParticipants[userHint])
				if err != nil {
					return err
				}
				a.vlogf(cmd, "transfer: ensuring sender %q has a standing pre-approval (Playground.Ops:preapprove)", userHint)
				var preOut preapproveOutput
				preErr := preRunner.Run(ctx, "Playground.Ops:preapprove", preapproveInput{
					Admin:              d.Admin,
					GuardianGovernance: s.GuardianGovernance,
					TokenKind:          d.TokenKind,
					Mode:               "burn-mint",
					User:               userParty,
				}, &preOut)
				preCleanup()
				if preErr != nil {
					return fmt.Errorf("transfer: ensure sender pre-approval: %w", preErr)
				}

				// fundUser submits (and queries the faucet factory/preapproval) as gg alone --
				// see Playground.Ops:fundUser -- so it routes to gg's OWN participant, not the
				// sender's or the default one (plan §8's "fundUser stays two routed calls").
				ggRole := participantRoleForParty(s, s.GuardianGovernance)
				fundRunner, fundCleanup, err := a.newScriptRunnerFor(ctx, ggRole)
				if err != nil {
					return err
				}
				decimals := wire.TrimDecimals(d.TokenDecimals)
				a.vlogf(cmd, "transfer: funding sender with %s via Playground.Ops:fundUser (holding cid feeds transferOut)", formatDecimal(amount, decimals))
				var fundOut fundUserOutput
				fundErr := fundRunner.Run(ctx, "Playground.Ops:fundUser", fundUserInput{
					GuardianGovernance: s.GuardianGovernance,
					Admin:              d.Admin,
					Owner:              userParty,
					Amount:             decimalLiteral(formatDecimal(amount, decimals)),
				}, &fundOut)
				fundCleanup()
				if fundErr != nil {
					return fmt.Errorf("transfer: funding user for a mock deployment: %w", fundErr)
				}
				holdingCids = []string{fundOut.HoldingCid}
			}

			// "amulet" resolves the real transfer factory and choice context fresh for this
			// call and never caches them, so a submit failure can be retried against a
			// current factory. It leaves inputHoldingCids empty so Playground.Amulet:amuletHoldings
			// enumerates the sender's unlocked Amulet in-script. The receiver is the
			// deployment's admin (lock/unlock custody is admin-owned by construction; there
			// is no separate custody party any more).
			var amuletSeam *amuletSeamJSON
			if d.TokenKind == "amulet" {
				amountDecimal := formatDecimal(amount, d.TokenDecimals) // full instrument scale (e.g. 10 decimals for Amulet), not the 8-decimal wire trim
				// The lock's receiver is whoever CURRENTLY custodies the reserve (the manager's
				// live `admin` field on-ledger), not the registering admin -- they can differ
				// once `admin accept-gg-vaa` has run.
				receiver := d.CurrentAdminOrAdmin()
				a.vlogf(cmd, "transfer: resolving real transfer-factory (sender=%s receiver=%s amount=%s)", userParty, receiver, amountDecimal)
				factory, err := amuletClient.GetTransferFactory(ctx, userHint, amulet.TransferArgs{
					DSO:      d.InstrumentAdmin,
					Sender:   userParty,
					Receiver: receiver,
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

			// transferOut submits AS the user, so it routes to the user's home participant
			// (plan §3) -- s.UserParticipants[userHint] was just populated (if not already
			// set) by resolveParty above; an amulet sender never goes through resolveParty
			// (it's a validator wallet user, see the branch above), so its UserParticipants
			// entry is unset and this correctly falls back to the default (app-provider),
			// matching "amulet wallet senders remain app-provider wallet users -- unchanged".
			//
			// The data owner for a Mock-kind prepare fetch is gg's OWN participant, not
			// operator's: guardianGovernance co-signs/owns everything transferOut needs
			// (NttManager/CoreState/Emitter/the committed factory/preapproval -- see
			// Playground.Prepare's header), which operator's participant cannot serve for the
			// gg-sole-owned factory/preapproval templates. "amulet" is exempt: it never touches
			// the playground's own gg-owned mock registry (its committed factory is the real
			// Splice registry's, handled entirely by the amuletSeam above), so
			// prepareTransferOut -- which only knows how to resolve MOCK factory/preapproval
			// shapes -- must never run for it (an amulet sender is also never routed through
			// resolveParty, so it would otherwise always look cross-participant here and
			// wrongly trigger the mock-only prepare path).
			var remoteSeam json.RawMessage
			actorRole := s.UserParticipants[userHint]
			if d.TokenKind != "amulet" {
				ownerRole := participantRoleForParty(s, s.GuardianGovernance)
				remoteSeam, err = prepareRemoteSeam(ctx, cmd, a, s, actorRole, ownerRole, "Playground.Prepare:prepareTransferOut", "transferOut", func(templates []string) any {
					return prepareTransferOutInput{
						GuardianGovernance: s.GuardianGovernance,
						ManagerID:          d.ManagerID,
						DiscloseTemplates:  templates,
					}
				})
				if err != nil {
					return fmt.Errorf("transfer: prepare remote disclosure: %w", err)
				}
			}

			userRunner, userCleanup, err := a.newScriptRunnerFor(ctx, actorRole)
			if err != nil {
				return err
			}
			defer userCleanup()

			var out transferOutOutput
			if err := userRunner.Run(ctx, "Playground.Ops:transferOut", transferOutInput{
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
				Remote:           remoteSeam,
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
	cmd.Flags().BoolVar(&noFund, "no-fund", false, "skip the mock-kind ensure-preapproval+fundUser step (implied by --strict-participant-isolation); transferOut enumerates the sender's own holdings in-script instead -- fund the sender separately with `fund`")
	cmd.Flags().StringVar(&tapUSD, "tap-usd", defaultTapUSD, "amulet only: USD amount to tap for the sender before transferring (\"0\" skips)")
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
