package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/observer"
)

// guardianWatcherUser is the observer stream's dedicated reader ledger user. It is granted
// only CanReadAs(guardianObserver) and never actAs, which shows that guardian observation
// works with read-only rights alone.
const guardianWatcherUser = "guardian-watcher"

// observedOutput is the JSON shape `observe stream` prints -- camelCase, machine-parseable,
// one array element per observed WormholeMessage.
type observedOutput struct {
	EmitterChain     int    `json:"emitterChain"` // 72, cantonChainId -- constant
	EmitterAddress   string `json:"emitterAddress"`
	Sequence         int    `json:"sequence"`
	Nonce            int    `json:"nonce"`
	ConsistencyLevel int    `json:"consistencyLevel"`
	Payload          string `json:"payload"`
	EffectiveAt      string `json:"effectiveAt"`
	UpdateID         string `json:"updateId"`
}

func toObservedOutput(o observer.Observed) observedOutput {
	return observedOutput{
		EmitterChain:     72,
		EmitterAddress:   o.EmitterAddress,
		Sequence:         int(o.Sequence),
		Nonce:            int(o.Nonce),
		ConsistencyLevel: int(o.ConsistencyLevel),
		Payload:          o.Payload,
		EffectiveAt:      o.EffectiveAt,
		UpdateID:         o.UpdateID,
	}
}

// newObserveStreamCmd builds `observe stream`, attached as a subcommand of the existing
// `observe` command (query.go's newObserveCmd) -- cobra runs the parent's own RunE when
// invoked with no subcommand token, so `observe --deployment X` keeps its pre-existing
// behavior unchanged; `observe stream ...` is matched first as a subcommand.
func newObserveStreamCmd(a *app) *cobra.Command {
	var deployment string
	var fromOffset int64
	var count int
	var timeout time.Duration
	var printOffset bool
	var anyEmitter bool

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Read the real Ledger API v2 update stream as the guardianObserver reader (read-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			prof, err := a.resolvedProfile()
			if err != nil {
				return err
			}
			// The guardian observation genuinely happens on the guardians' own node: GO is an
			// observer of Emitter/CoreState, so its OWN participant is what receives those
			// projections and must serve the stream (plan §3's "observe stream" routing).
			ep, err := prof.Endpoint("guardian-observer")
			if err != nil {
				return err
			}
			if ep.JSONAPIBaseURL == "" {
				return fmt.Errorf("observe stream: requires the localnet profile")
			}
			s, err := a.loadState()
			if err != nil {
				return err
			}
			if s.GuardianObserver == "" {
				return fmt.Errorf("observe stream: no guardianObserver party in state -- run `init` first")
			}

			var wantAddress string
			if !anyEmitter {
				d, ok := s.Deployment(deployment)
				if !ok {
					return fmt.Errorf("observe stream: unknown deployment %q", deployment)
				}
				wantAddress = d.TransceiverAddress
			}

			adminToken, err := network.MintUnsafeToken(network.LocalNetAdminUser, time.Hour)
			if err != nil {
				return err
			}
			// The reader user carries ONLY CanReadAs(guardianObserver) -- no actAs, ever.
			// This is the CLI's one non-`dpm script` ledger surface, and it is strictly
			// read-only: no command on this path submits anything.
			a.vlogf(cmd, "observe stream: ensuring reader user %q exists with ONLY CanReadAs(%s)", guardianWatcherUser, s.GuardianObserver)
			if err := network.CreateLedgerUser(ctx, ep.JSONAPIBaseURL, adminToken, guardianWatcherUser, []string{s.GuardianObserver}, nil); err != nil {
				return fmt.Errorf("observe stream: create reader user: %w", err)
			}
			watcherToken, err := network.MintUnsafeToken(guardianWatcherUser, timeout+time.Hour)
			if err != nil {
				return err
			}

			if printOffset {
				ledgerEnd, err := observer.LedgerEnd(ctx, ep.JSONAPIBaseURL, watcherToken)
				if err != nil {
					return fmt.Errorf("observe stream --print-offset: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "ledgerEnd=%d\n", ledgerEnd)
				return nil
			}

			begin := fromOffset
			if !cmd.Flags().Changed("from-offset") {
				begin, err = observer.LedgerEnd(ctx, ep.JSONAPIBaseURL, watcherToken)
				if err != nil {
					return fmt.Errorf("observe stream: resolve default --from-offset: %w", err)
				}
			}
			a.vlogf(cmd, "observe stream: beginExclusive=%d count=%d timeout=%s any-emitter=%t", begin, count, timeout, anyEmitter)

			streamCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			var results []observedOutput
			streamErr := observer.Stream(streamCtx, observer.Config{
				JSONAPIBaseURL: ep.JSONAPIBaseURL,
				Token:          watcherToken,
				ObserverParty:  s.GuardianObserver,
				BeginExclusive: begin,
				Logf:           a.verboseLogf(),
			}, func(o observer.Observed) bool {
				if wantAddress != "" && o.EmitterAddress != wantAddress {
					a.vlogf(cmd, "observe stream: skipping message from a different emitter (%s)", o.EmitterAddress)
					return true // not a match -- keep listening
				}
				results = append(results, toObservedOutput(o))
				return len(results) < count
			})
			if streamErr != nil && !errors.Is(streamErr, context.DeadlineExceeded) {
				return fmt.Errorf("observe stream: %w", streamErr)
			}
			if len(results) < count {
				return fmt.Errorf("observe stream: timed out after %s waiting for %d message(s) (observed %d)", timeout, count, len(results))
			}

			raw, err := json.Marshal(results)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment name (matches observations by its derived transceiver emitter address, unless --any-emitter)")
	cmd.Flags().Int64Var(&fromOffset, "from-offset", 0, "ledger offset to start from, exclusive (default: current ledger end)")
	cmd.Flags().IntVar(&count, "count", 1, "number of matching messages to collect before printing")
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Minute, "how long to wait for --count matching messages")
	cmd.Flags().BoolVar(&printOffset, "print-offset", false, "print the current ledger end as ledgerEnd=<n> and exit")
	cmd.Flags().BoolVar(&anyEmitter, "any-emitter", false, "don't filter by this deployment's transceiver address")
	_ = cmd.MarkFlagRequired("deployment")
	return cmd
}
