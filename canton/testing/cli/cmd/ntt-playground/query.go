package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// decimalLiteral round-trips a Daml `Decimal` value through JSON as a bare number literal
// (e.g. `0.01`), matching how dpm script serializes Decimal fields -- NOT a quoted string.
// Its String() prints the literal as-is (Daml Decimals aren't precision-lossy in this range,
// and this avoids float64 round-tripping entirely).
type decimalLiteral string

func (d decimalLiteral) MarshalJSON() ([]byte, error) { return []byte(d), nil }

func (d *decimalLiteral) UnmarshalJSON(b []byte) error {
	*d = decimalLiteral(b)
	return nil
}

func (d decimalLiteral) String() string { return string(d) }

// statusInput/statusOutput mirror Playground.Query.daml's StatusInput/StatusOutput.
type statusInput struct {
	Operator string `json:"operator"`
}

// peerStatus mirrors Playground.Types.daml's PeerStatus record.
type peerStatus struct {
	Chain              int    `json:"chain"`
	ManagerAddress     string `json:"managerAddress"`
	TransceiverAddress string `json:"transceiverAddress"`
}

type deploymentStatus struct {
	ManagerID          int          `json:"managerId"`
	ManagerAddress     string       `json:"managerAddress"`
	TransceiverAddress string       `json:"transceiverAddress"`
	Admin              string       `json:"admin"`
	Mode               string       `json:"mode"`
	TokenDecimals      int          `json:"tokenDecimals"`
	OutboundSequence   int          `json:"outboundSequence"`
	Peers              []peerStatus `json:"peers"`
}

type statusOutput struct {
	GuardianSetIndex int                `json:"guardianSetIndex"`
	GuardianKeys     []string           `json:"guardianKeys"`
	MessageFee       int                `json:"messageFee"`
	Deployments      []deploymentStatus `json:"deployments"`
}

func (a *app) queryStatus(cmd *cobra.Command) (statusOutput, error) {
	ctx := cmd.Context()
	s, err := a.loadState()
	if err != nil {
		return statusOutput{}, err
	}
	runner, cleanup, err := a.newScriptRunner(ctx)
	if err != nil {
		return statusOutput{}, err
	}
	defer cleanup()

	var out statusOutput
	if err := runner.Run(ctx, "Playground.Query:queryStatus", statusInput{Operator: s.Operator}, &out); err != nil {
		return statusOutput{}, err
	}
	return out, nil
}

func newStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print the bridge's guardian set, fee, and every deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := a.queryStatus(cmd)
			if err != nil {
				return err
			}
			raw, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}
}

func newObserveCmd(a *app) *cobra.Command {
	var deployment string
	cmd := &cobra.Command{
		Use:   "observe",
		Short: "Print a deployment's current outbound sequence and peers, or stream real guardian observations (`observe stream`)",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := a.queryStatus(cmd)
			if err != nil {
				return err
			}
			// The ledger only knows managerId, not the CLI's local deployment name; resolve
			// the name to a managerId via local state first.
			s, err := a.loadState()
			if err != nil {
				return err
			}
			local, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("observe: unknown deployment %q", deployment)
			}
			for _, d := range out.Deployments {
				if d.ManagerID == local.ManagerID {
					raw, err := json.MarshalIndent(d, "", "  ")
					if err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(raw))
					return nil
				}
			}
			return fmt.Errorf("observe: deployment %q (managerId %d) not found on ledger", deployment, local.ManagerID)
		},
	}
	cmd.AddCommand(newObserveStreamCmd(a))
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name")
	_ = cmd.MarkFlagRequired("deployment")
	return cmd
}

// listContractsInput/listContractsOutput (and their element records) mirror
// Playground.Query.daml's ListContractsInput/ListContractsOutput.
type listContractsInput struct {
	Operator string `json:"operator"`
}

type emitterInfo struct {
	EmitterID      int    `json:"emitterId"`
	EmitterAddress string `json:"emitterAddress"`
	Owner          string `json:"owner"`
}

type managerInfo struct {
	ManagerID        int    `json:"managerId"`
	ManagerAddress   string `json:"managerAddress"`
	Mode             string `json:"mode"`
	OutboundSequence int    `json:"outboundSequence"`
}

type listContractsOutput struct {
	GuardianSetIndex int           `json:"guardianSetIndex"`
	MessageFee       int           `json:"messageFee"`
	Emitters         []emitterInfo `json:"emitters"`
	Managers         []managerInfo `json:"managers"`
	ReplayNodes      int           `json:"replayNodes"`
}

func newContractsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contracts",
		Short: "Inspect the live playground contracts on the ledger",
	}
	cmd.AddCommand(newContractsListCmd(a))
	return cmd
}

func newContractsListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print every live CoreState/Emitter/NttManager (plus the replay-node count) as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			runner, cleanup, err := a.newScriptRunner(ctx)
			if err != nil {
				return err
			}
			defer cleanup()

			a.vlogf(cmd, "contracts list: querying live playground contracts as operator (Playground.Query:listContracts)")
			var out listContractsOutput
			if err := runner.Run(ctx, "Playground.Query:listContracts", listContractsInput{Operator: s.Operator}, &out); err != nil {
				return err
			}
			raw, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}
}

// balancesInput/balancesOutput mirror Playground.Query.daml's BalancesInput/BalancesOutput.
type balancesInput struct {
	Reader          string `json:"reader"`
	Owner           string `json:"owner"`
	InstrumentAdmin string `json:"instrumentAdmin"`
	InstrumentID    string `json:"instrumentId"`
}

type balancesOutput struct {
	Cip56HoldingTotal decimalLiteral `json:"cip56HoldingTotal"`
}

// amuletBalanceInput/amuletBalanceOutput mirror Playground.Query.daml's
// AmuletBalanceInput/AmuletBalanceOutput.
type amuletBalanceInput struct {
	Owner           string `json:"owner"`
	InstrumentAdmin string `json:"instrumentAdmin"`
}

type amuletBalanceOutput struct {
	AmuletHoldingTotal decimalLiteral `json:"amuletHoldingTotal"`
}

func newBalanceCmd(a *app) *cobra.Command {
	var partyHint, deployment string

	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Print a party's mock/CIP-56/real-Amulet holdings for a deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			d, ok := s.Deployment(deployment)
			if !ok {
				return fmt.Errorf("balance: unknown deployment %q", deployment)
			}
			party, err := resolveParty(cmd, a, s, partyHint)
			if err != nil {
				return err
			}

			// Route to whichever party the underlying script actually queries AS: amuletBalance
			// reads as the owner itself (input.owner: real Amulet Holdings are queried via the
			// token-standard interface directly, no gg/mock registry involved); balances reads
			// as gg (input.reader -- Cip56MockHolding's signatory is gg, not admin, post the
			// CIP-56/namespace-retirement rework -- see Playground.Query.daml's BalancesInput
			// doc comment), never the party being checked or the deployment's admin. Either way
			// that querying party must be hosted on the participant the script runs against
			// (plan §3: "route the read-only script to the party's home participant").
			readerRole := participantRoleForParty(s, s.GuardianGovernance)
			if d.TokenKind == "amulet" {
				readerRole = participantRoleForParty(s, party)
			}
			runner, cleanup, err := a.newScriptRunnerFor(ctx, readerRole)
			if err != nil {
				return err
			}
			defer cleanup()

			if d.TokenKind == "amulet" {
				var out amuletBalanceOutput
				if err := runner.Run(ctx, "Playground.Query:amuletBalance", amuletBalanceInput{
					Owner:           party,
					InstrumentAdmin: d.InstrumentAdmin,
				}, &out); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "party=%s amuletHoldingTotal=%s\n", party, out.AmuletHoldingTotal)
				return nil
			}

			var out balancesOutput
			if err := runner.Run(ctx, "Playground.Query:balances", balancesInput{
				Reader:          s.GuardianGovernance,
				Owner:           party,
				InstrumentAdmin: d.InstrumentAdmin,
				InstrumentID:    d.InstrumentID,
			}, &out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "party=%s cip56HoldingTotal=%s\n", party, out.Cip56HoldingTotal)
			return nil
		},
	}
	cmd.Flags().StringVar(&partyHint, "party", "", "party hint to check")
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name (determines whose holdings are visible)")
	_ = cmd.MarkFlagRequired("party")
	_ = cmd.MarkFlagRequired("deployment")
	return cmd
}
