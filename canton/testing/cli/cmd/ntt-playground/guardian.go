package main

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/guardian"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// applyGovernanceInput/applyGovernanceOutput mirror Playground.Ops.daml's
// ApplyGovernanceInput/ApplyGovernanceOutput.
type applyGovernanceInput struct {
	Operator string              `json:"operator"`
	VaaBytes string              `json:"vaaBytes"`
	PubKeys  []ledger.PubKeyHint `json:"pubKeys"`
}

type applyGovernanceOutput struct {
	MessageFee int `json:"messageFee"`
}

// verifyVaaInput/verifyVaaOutput mirror Playground.Query.daml's VerifyVaaInput/Output.
type verifyVaaInput struct {
	Operator string              `json:"operator"`
	Verifier string              `json:"verifier"`
	VaaBytes string              `json:"vaaBytes"`
	PubKeys  []ledger.PubKeyHint `json:"pubKeys"`
}

type verifyVaaOutput struct {
	GuardianSetIndex int    `json:"guardianSetIndex"`
	EmitterChain     int    `json:"emitterChain"`
	EmitterAddress   string `json:"emitterAddress"`
	Sequence         int    `json:"sequence"`
}

func newGuardianCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardian",
		Short: "Act as the playground's 1/1 guardian: sign VAAs, verify them on-ledger",
	}
	cmd.AddCommand(
		newGuardianSignTransferCmd(a),
		newGuardianSignVaaCmd(a),
		newGuardianSignGovernanceCmd(a),
		newGuardianVerifyVaaCmd(a),
	)
	return cmd
}

func newGuardianSignTransferCmd(a *app) *cobra.Command {
	var deployment, toRecipientHint, sourceTokenHex string
	var sourceChain int
	var amount int64
	var sequence uint64
	var nonce, consistencyLevel int

	cmd := &cobra.Command{
		Use:   "sign-transfer",
		Short: "Sign an inbound NTT transfer VAA as if it came from a peer chain",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("guardian sign-transfer: unknown deployment %q", deployment)
			}
			peer, ok := d.Peer(sourceChain)
			if !ok {
				return fmt.Errorf("guardian sign-transfer: deployment %q has no peer configured for chain %d", deployment, sourceChain)
			}
			if s.Guardian.PrivateKeyHex == "" {
				return fmt.Errorf("guardian sign-transfer: no guardian key in state -- run `init` first")
			}
			key, err := guardian.KeyFromHex(s.Guardian.PrivateKeyHex)
			if err != nil {
				return err
			}
			recipientParty, err := resolveParty(cmd, a, s, toRecipientHint)
			if err != nil {
				return err
			}

			seqNote := ""
			if sequence == 0 {
				sequence = s.NextGuardianSequence("transfer", uint16(sourceChain)) //nolint:gosec // playground chain ids are small
				seqNote = " (auto)"
				if err := a.saveState(s); err != nil {
					return err
				}
			}
			a.vlogf(cmd, "sign-transfer: sequence=%d%s source-chain=%d", sequence, seqNote, sourceChain)
			if sourceTokenHex == "" {
				sourceTokenHex = strings.Repeat("00", 32)
			}

			sourceManager, err := decodeHex32(peer.ManagerAddress)
			if err != nil {
				return fmt.Errorf("guardian sign-transfer: peer manager address: %w", err)
			}
			sourceTransceiver, err := decodeHex32(peer.TransceiverAddress)
			if err != nil {
				return fmt.Errorf("guardian sign-transfer: peer transceiver address: %w", err)
			}
			recipientManager, err := decodeHex32(d.ManagerAddress)
			if err != nil {
				return fmt.Errorf("guardian sign-transfer: deployment manager address: %w", err)
			}
			sourceToken, err := decodeHex32(sourceTokenHex)
			if err != nil {
				return fmt.Errorf("guardian sign-transfer: source token: %w", err)
			}

			recipientAddress := wire.RecipientAddressFor(recipientParty)

			vaa, err := guardian.SignTransfer(key, guardian.TransferParams{
				SourceChain:       uint16(sourceChain), //nolint:gosec // playground chain ids are small
				SourceManager:     sourceManager,
				SourceTransceiver: sourceTransceiver,
				RecipientManager:  recipientManager,
				SourceToken:       sourceToken,
				RecipientAddress:  recipientAddress,
				Decimals:          uint8(wire.TrimDecimals(d.TokenDecimals)), //nolint:gosec // <= 8
				Amount:            uint64(amount),
				Sequence:          sequence,
				Nonce:             uint32(nonce),           //nolint:gosec // playground nonces are small
				ConsistencyLevel:  uint8(consistencyLevel), //nolint:gosec // playground consistency levels are small
			})
			if err != nil {
				return err
			}
			a.vlogf(cmd, "sign-transfer: signed %d-byte body", len(vaa)-vaaHeaderLen)
			fmt.Fprintf(cmd.OutOrStdout(), "vaa=%s\npubkey=%s\nrecipient=%s\n",
				hex.EncodeToString(vaa), hex.EncodeToString(key.PubKeyUncompressed()), recipientParty)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	cmd.Flags().StringVar(&toRecipientHint, "to-recipient", "", "party hint for the mint recipient")
	cmd.Flags().IntVar(&sourceChain, "source-chain", 2, "source (peer) chain id")
	cmd.Flags().Int64Var(&amount, "amount", 0, "trimmed amount to mint")
	cmd.Flags().Uint64Var(&sequence, "sequence", 0, "VAA sequence (default: auto-incremented per source chain)")
	cmd.Flags().StringVar(&sourceTokenHex, "source-token", "", "32-byte hex source token address (default: zero)")
	cmd.Flags().IntVar(&nonce, "nonce", 0, "VAA nonce")
	cmd.Flags().IntVar(&consistencyLevel, "consistency-level", 0, "VAA consistency level")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("to-recipient")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}

func newGuardianSignVaaCmd(a *app) *cobra.Command {
	var emitterChain int
	var emitterHex string
	var sequence uint64
	var payloadHex string
	var nonce, consistencyLevel int

	cmd := &cobra.Command{
		Use:   "sign-vaa",
		Short: "Sign an arbitrary payload as a VAA",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.loadState()
			if err != nil {
				return err
			}
			if s.Guardian.PrivateKeyHex == "" {
				return fmt.Errorf("guardian sign-vaa: no guardian key in state -- run `init` first")
			}
			key, err := guardian.KeyFromHex(s.Guardian.PrivateKeyHex)
			if err != nil {
				return err
			}
			emitterAddr, err := decodeHex32(emitterHex)
			if err != nil {
				return fmt.Errorf("guardian sign-vaa: emitter address: %w", err)
			}
			payload, err := hex.DecodeString(payloadHex)
			if err != nil {
				return fmt.Errorf("guardian sign-vaa: payload: %w", err)
			}
			vaa, err := guardian.Sign(key, guardian.VAAParams{
				Nonce:            uint32(nonce),        //nolint:gosec // playground nonces are small
				EmitterChain:     uint16(emitterChain), //nolint:gosec // playground chain ids are small
				EmitterAddress:   emitterAddr,
				Sequence:         sequence,
				ConsistencyLevel: uint8(consistencyLevel), //nolint:gosec // playground consistency levels are small
				Payload:          payload,
			})
			if err != nil {
				return err
			}
			a.vlogf(cmd, "sign-vaa: sequence=%d, signed %d-byte body", sequence, len(vaa)-vaaHeaderLen)
			fmt.Fprintf(cmd.OutOrStdout(), "vaa=%s\npubkey=%s\n", hex.EncodeToString(vaa), hex.EncodeToString(key.PubKeyUncompressed()))
			return nil
		},
	}
	cmd.Flags().IntVar(&emitterChain, "emitter-chain", 0, "emitter chain id")
	cmd.Flags().StringVar(&emitterHex, "emitter", "", "32-byte hex emitter address")
	cmd.Flags().Uint64Var(&sequence, "sequence", 0, "VAA sequence")
	cmd.Flags().StringVar(&payloadHex, "payload", "", "hex-encoded payload")
	cmd.Flags().IntVar(&nonce, "nonce", 0, "VAA nonce")
	cmd.Flags().IntVar(&consistencyLevel, "consistency-level", 0, "VAA consistency level")
	_ = cmd.MarkFlagRequired("emitter-chain")
	_ = cmd.MarkFlagRequired("emitter")
	_ = cmd.MarkFlagRequired("sequence")
	_ = cmd.MarkFlagRequired("payload")
	return cmd
}

func newGuardianSignGovernanceCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign-governance",
		Short: "Sign (and optionally apply) a Core governance VAA",
	}
	cmd.AddCommand(newGuardianSignGovernanceSetFeeCmd(a))
	return cmd
}

func newGuardianSignGovernanceSetFeeCmd(a *app) *cobra.Command {
	var fee uint64
	var apply bool

	cmd := &cobra.Command{
		Use:   "set-fee",
		Short: "Sign a Core SetMessageFee governance VAA targeting the Canton chain",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			if s.Guardian.PrivateKeyHex == "" {
				return fmt.Errorf("guardian sign-governance set-fee: no guardian key in state -- run `init` first")
			}
			key, err := guardian.KeyFromHex(s.Guardian.PrivateKeyHex)
			if err != nil {
				return err
			}
			seq := s.NextGuardianSequence("governance", guardian.DefaultGovernanceChain)
			if err := a.saveState(s); err != nil {
				return err
			}
			a.vlogf(cmd, "sign-governance set-fee: sequence=%d (auto) fee=%d", seq, fee)
			vaa, err := guardian.SignSetMessageFee(key, guardian.GovernanceParams{Sequence: seq}, wire.CantonChainID, fee)
			if err != nil {
				return err
			}
			a.vlogf(cmd, "sign-governance set-fee: signed %d-byte body", len(vaa)-vaaHeaderLen)
			fmt.Fprintf(cmd.OutOrStdout(), "vaa=%s\n", hex.EncodeToString(vaa))

			if apply {
				runner, cleanup, err := a.newScriptRunner(ctx)
				if err != nil {
					return err
				}
				defer cleanup()
				var out applyGovernanceOutput
				if err := runner.Run(ctx, "Playground.Ops:applyGovernance", applyGovernanceInput{
					Operator: s.Operator,
					VaaBytes: hex.EncodeToString(vaa),
					PubKeys:  []ledger.PubKeyHint{{Index: 0, Key: hex.EncodeToString(key.PubKeyUncompressed())}},
				}, &out); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "applied: messageFee=%d\n", out.MessageFee)
			}
			return nil
		},
	}
	cmd.Flags().Uint64Var(&fee, "fee", 0, "new message fee")
	cmd.Flags().BoolVar(&apply, "apply", false, "also submit the VAA to CoreState via SubmitGovernanceVAA")
	_ = cmd.MarkFlagRequired("fee")
	return cmd
}

func newGuardianVerifyVaaCmd(a *app) *cobra.Command {
	var deployment, vaaHex, pubKeyHex, verifierHint string

	cmd := &cobra.Command{
		Use:   "verify-vaa",
		Short: "Verify a signed VAA on-ledger via CoreState.ParseAndVerifyVAA (no replay-consume)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			verifier := s.Operator
			if verifierHint != "" {
				verifier, err = resolveParty(cmd, a, s, verifierHint)
				if err != nil {
					return err
				}
			} else if deployment != "" {
				if d, ok := s.Deployment(deployment); ok {
					verifier = d.Admin
				}
			}
			a.vlogf(cmd, "verify-vaa: verifier=%s", verifier)
			if pubKeyHex == "" {
				if s.Guardian.PrivateKeyHex == "" {
					return fmt.Errorf("guardian verify-vaa: --pubkey is required (no guardian key in state to derive it from)")
				}
				key, err := guardian.KeyFromHex(s.Guardian.PrivateKeyHex)
				if err != nil {
					return err
				}
				pubKeyHex = hex.EncodeToString(key.PubKeyUncompressed())
			}

			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			var out verifyVaaOutput
			if err := runner.Run(ctx, "Playground.Query:verifyVaa", verifyVaaInput{
				Operator: s.Operator,
				Verifier: verifier,
				VaaBytes: vaaHex,
				PubKeys:  []ledger.PubKeyHint{{Index: 0, Key: pubKeyHex}},
			}, &out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "guardianSetIndex=%d emitterChain=%d emitterAddress=%s sequence=%d\n",
				out.GuardianSetIndex, out.EmitterChain, out.EmitterAddress, out.Sequence)
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name (verifier defaults to its admin)")
	cmd.Flags().StringVar(&vaaHex, "vaa", "", "hex-encoded VAA to verify")
	cmd.Flags().StringVar(&pubKeyHex, "pubkey", "", "signing guardian's 65-byte uncompressed public key, hex (default: the playground guardian's)")
	cmd.Flags().StringVar(&verifierHint, "verifier", "", "party hint to submit as (default: the deployment's admin, or operator)")
	_ = cmd.MarkFlagRequired("vaa")
	return cmd
}

// vaaHeaderLen is the byte length of everything preceding a 1-signature VAA's body:
// version(1) + guardianSetIndex(4) + sigCount(1) + guardianIndex(1) + signature(65). The
// playground always signs 1/1, so len(vaa)-vaaHeaderLen is the signed body's length.
const vaaHeaderLen = 1 + 4 + 1 + 1 + 65

// signRecomputedVAA signs an observed published message with the playground's 1/1 guardian
// key, standing in for the off-ledger guardian watcher -- the shared tail of
// `transfer --sign` and `publish --sign`. The caller supplies exactly the message fields
// the ledger emitted (or, in `transfer`'s case, bit-exact recomputations of them). Returns
// the hex-encoded VAA; callers wrap errors with their own command prefix.
func signRecomputedVAA(s *state.State, emitterChain int, emitterAddressHex string, sequence uint64, nonce, consistencyLevel int, payloadHex string) (string, error) {
	if s.Guardian.PrivateKeyHex == "" {
		return "", fmt.Errorf("no guardian private key in state (init with --guardian-key, or without one to generate a fresh key)")
	}
	key, err := guardian.KeyFromHex(s.Guardian.PrivateKeyHex)
	if err != nil {
		return "", err
	}
	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return "", fmt.Errorf("decode published payload: %w", err)
	}
	emitterAddr, err := decodeHex32(emitterAddressHex)
	if err != nil {
		return "", fmt.Errorf("decode emitter address: %w", err)
	}
	vaa, err := guardian.Sign(key, guardian.VAAParams{
		Nonce:            uint32(nonce),        //nolint:gosec // playground nonces are small
		EmitterChain:     uint16(emitterChain), //nolint:gosec // playground chain ids are small
		EmitterAddress:   emitterAddr,
		Sequence:         sequence,
		ConsistencyLevel: uint8(consistencyLevel), //nolint:gosec // playground consistency levels are small
		Payload:          payload,
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(vaa), nil
}

func decodeHex32(s string) ([32]byte, error) {
	var out [32]byte
	raw, err := hex.DecodeString(trimHexPrefix(s))
	if err != nil {
		return out, err
	}
	if len(raw) > 32 {
		return out, fmt.Errorf("hex value longer than 32 bytes: %d bytes", len(raw))
	}
	copy(out[32-len(raw):], raw)
	return out, nil
}

func trimHexPrefix(s string) string {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		return s[2:]
	}
	return s
}
