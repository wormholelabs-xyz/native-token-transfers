package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
)

// defaultDisclosureListenAddr is `disclosure serve`'s default bind address. Loopback-only by
// default is a deliberate posture, not an accident: the service is unauthenticated (design doc
// §4), so an operator must opt in explicitly (via --listen) to expose it more broadly.
const defaultDisclosureListenAddr = "127.0.0.1:7599"

// defaultDisclosureParticipant is the participant role `disclosure serve` fronts absent an
// explicit --participant -- guardianGovernance, since that is the participant every existing
// prepare* seam (transferOut/receive) needs a cross-participant fetch against (see remote.go).
const defaultDisclosureParticipant = "guardian-governance"

func newDisclosureCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disclosure",
		Short: "Run the disclosure service that fronts one participant's allow-listed contracts",
	}
	cmd.AddCommand(newDisclosureServeCmd(a))
	return cmd
}

// newDisclosureServeCmd builds `ntt-playground disclosure serve` -- the design doc's §5.2
// harness-grade disclosure service: an HTTP sidecar fronting ONE participant (--participant),
// serving the generic ACS-backed GET /v1/disclosures and the typed POST /v1/seam/* endpoints
// (see internal/disclosure.Service). This is what lets a consumer obtain a cross-participant
// RemoteSeam -- or raw disclosed contracts -- WITHOUT holding the fronted participant's own
// credentials, the property --disclosure-service-url/--strict-participant-isolation exist to
// make load-bearing (remote.go's prepareRemoteSeam, runner.go's isolation guard).
func newDisclosureServeCmd(a *app) *cobra.Command {
	var listen, participant string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the disclosure service (GET /v1/disclosures, POST /v1/seam/*) fronting one participant",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.loadState()
			if err != nil {
				return err
			}
			if s.GuardianGovernance == "" {
				return fmt.Errorf("disclosure serve: no guardianGovernance party in state -- run `init` first")
			}

			prof, err := a.resolvedProfile()
			if err != nil {
				return err
			}
			ep, err := prof.Endpoint(participant)
			if err != nil {
				return err
			}

			cfg, err := a.topologyConfig()
			if err != nil {
				return err
			}

			// The generic /v1/disclosures endpoint needs a live ACS reader; the sandbox
			// profile has no JSON Ledger API at all (Endpoint.JSONAPIBaseURL is empty there),
			// so ACS stays nil and that endpoint 503s (internal/disclosure/service.go's
			// handleDisclosures). POST /v1/seam/* -- the only thing the acceptance criterion
			// needs -- is unaffected: it runs entirely through NewRunner's `dpm script` path
			// below, never through ACS.
			var acs *disclosure.ActiveContracts
			if ep.JSONAPIBaseURL != "" {
				var token string
				if prof.RequiresAuth {
					token, err = network.MintUnsafeToken(ep.UserID, 24*time.Hour)
					if err != nil {
						return fmt.Errorf("disclosure serve: mint ACS reader token: %w", err)
					}
				}
				acs = &disclosure.ActiveContracts{
					BaseURL: ep.JSONAPIBaseURL,
					Token:   token,
					Party:   s.GuardianGovernance,
					Logf:    a.verboseLogf(),
				}
			}

			svc := &disclosure.Service{
				Cfg:         cfg,
				Participant: participant,
				ACS:         acs,
				// a.newScriptRunnerFor returns this package's scriptRunner interface;
				// disclosure.ScriptRunner has the identical single-method shape
				// (Run(ctx, scriptName string, input, output any) error), so the interface
				// value is directly assignable here -- no adapter needed.
				NewRunner: func(ctx context.Context) (disclosure.ScriptRunner, func(), error) {
					return a.newScriptRunnerFor(ctx, participant)
				},
				Logf: a.verboseLogf(),
			}

			ln, err := net.Listen("tcp", listen)
			if err != nil {
				return fmt.Errorf("disclosure serve: listen on %s: %w", listen, err)
			}

			// The startup banner and the resolved-address line below are ALWAYS printed (not
			// gated on --verbose): the security posture is load-bearing information, not
			// narration, and the address line is what an e2e harness parses to learn an
			// ephemeral ":0" port after Listen resolves it.
			stderr := a.stderr
			if stderr == nil {
				stderr = cmd.ErrOrStderr()
			}
			fmt.Fprintf(stderr, "disclosure serve: fronting participant=%s templates=%d\n", participant, len(cfg.Templates()))
			fmt.Fprintln(stderr, "disclosure serve: unauthenticated harness-grade service; keep it loopback-bound unless you know why")
			if !isLoopbackListenAddr(listen) {
				fmt.Fprintln(stderr, "disclosure serve: WARNING: --listen is not loopback-bound -- this exposes allow-listed contract payloads, unauthenticated, to anything that can reach this address")
			}
			fmt.Fprintf(stderr, "disclosure serve: listening on http://%s\n", ln.Addr().String())

			srv := &http.Server{Handler: svc.Handler()}
			// Shut down gracefully once the command's context is done (e.g. the process
			// receives SIGINT via cobra's signal handling, or a test cancels the context) --
			// srv.Serve returns http.ErrServerClosed in that case, which is not itself an
			// error worth propagating.
			context.AfterFunc(ctx, func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			})

			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("disclosure serve: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&listen, "listen", defaultDisclosureListenAddr, "address to listen on (host:port)")
	cmd.Flags().StringVar(&participant, "participant", defaultDisclosureParticipant, "participant role this service fronts (internal/profile.Profile.Participants key)")
	return cmd
}

// isLoopbackListenAddr reports whether addr's host part names a loopback interface --
// "127.0.0.1", "localhost", or "::1". Any other host (including an empty one, which
// net.Listen binds to every interface) is treated as non-loopback -- an operator opting into a
// broader bind should see the warning, not have it silently suppressed by an edge case.
func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}
