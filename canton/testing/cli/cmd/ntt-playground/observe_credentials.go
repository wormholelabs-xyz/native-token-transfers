package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// observeCredentialsOutput is `observe credentials`'s output shape: everything an external
// process needs to subscribe to the guardian-observer participant's gRPC Ledger API as
// guardianObserver, read-only. This is the cross-repo seam plan §5.3 describes -- the
// Wormhole guardian node's LocalNet integration test shells this instead of knowing anything
// about Splice.
type observeCredentialsOutput struct {
	GRPCAddress      string `json:"grpcAddress"`
	GuardianObserver string `json:"guardianObserver"`
	ReaderToken      string `json:"readerToken"`
	JSONAPIBaseURL   string `json:"jsonApiBaseUrl"`
}

// newObserveCredentialsCmd builds `observe credentials`, attached alongside `observe stream`
// under the `observe` parent command.
func newObserveCredentialsCmd(a *app) *cobra.Command {
	var tokenTTL time.Duration
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "Print the guardian-observer participant's gRPC address and a read-only guardianObserver reader token",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ep, s, err := resolveGuardianObserverEndpoint(a, "observe credentials")
			if err != nil {
				return err
			}
			token, err := mintGuardianReaderToken(ctx, a, cmd, "observe credentials", ep, s.GuardianObserver, tokenTTL)
			if err != nil {
				return err
			}
			out := observeCredentialsOutput{
				GRPCAddress:      fmt.Sprintf("%s:%d", ep.LedgerHost, ep.LedgerPort),
				GuardianObserver: s.GuardianObserver,
				ReaderToken:      token,
				JSONAPIBaseURL:   ep.JSONAPIBaseURL,
			}
			if asJSON {
				raw, err := json.Marshal(out)
				if err != nil {
					return err
				}
				// Exactly the JSON object, nothing else -- callers pipe this straight into a
				// JSON parser (see plan §5.3); all narration goes to stderr via a.vlogf.
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "grpcAddress=%s guardianObserver=%s readerToken=%s jsonApiBaseUrl=%s\n",
				out.GRPCAddress, out.GuardianObserver, out.ReaderToken, out.JSONAPIBaseURL)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a single JSON object instead of key=value")
	cmd.Flags().DurationVar(&tokenTTL, "token-ttl", 2*time.Hour, "lifetime of the minted reader token (long enough for an integration test run)")
	return cmd
}
