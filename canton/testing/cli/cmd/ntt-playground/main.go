// Command ntt-playground is a devnet playground CLI for the Canton NTT contracts under
// canton/: stand up a network, deploy an NTT, control a 1/1 Wormhole guardian to sign VAAs
// on the fly, and drive inbound/outbound transfers end to end. See the README in this
// directory for full usage and the LocalNet/MainNet paths.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// app holds every flag/dependency the subcommands share, resolved once in
// PersistentPreRunE and threaded through via the root command's context.
type app struct {
	stateFile string
	dpmPath   string
	cantonDir string
	runDir    string
	profile   profile.Name
	verbose   bool

	// topologyConfigPath is the --topology-config flag's value; empty means "use the default
	// path next to the state file" (see resolvedTopologyConfigPath).
	topologyConfigPath string

	// disclosureServiceURL is the --disclosure-service-url flag's value. Empty (the default)
	// means "unchanged behavior": prepareRemoteSeam (remote.go) runs the local
	// `Playground.Prepare:prepare*` script directly against the data owner's own participant,
	// exactly as before this flag existed. When set, prepareRemoteSeam instead issues a
	// POST /v1/seam/{name} against this base URL (internal/disclosure.Client) -- the property
	// that lets a consumer fetch a cross-participant RemoteSeam without holding the data
	// owner's own credentials (design doc §5.3).
	disclosureServiceURL string

	// strictParticipantIsolation is the --strict-participant-isolation flag's value. When set,
	// newScriptRunnerFor (runner.go) refuses to build a runner targeting any participant other
	// than isolationBaselineRole below -- the negative control that proves a flow genuinely
	// needs --disclosure-service-url rather than silently working because the harness happens
	// to hold every participant's credentials (design doc §7 R8).
	strictParticipantIsolation bool

	// isolationBaselineRole is the current invocation's actor participant role -- the baseline
	// strictParticipantIsolation compares every newScriptRunnerFor call's target against. This
	// is NOT a cobra flag: it is set once per invocation by the three actor-routing commands
	// (transfer.go, receive.go, publish.go), immediately after the actor's own role is resolved
	// and before any call that might cross a participant boundary. Commands that legitimately
	// span participants by design -- init, deploy, fund, party -- never set it, so it stays ""
	// for them, which the guard treats as "no actor-routing command is in play" and therefore
	// never fires (design doc §7 R8: the guard must not break those commands even when
	// --strict-participant-isolation is passed globally).
	isolationBaselineRole string

	// stderr is the invoked command's error stream, captured once in PersistentPreRun so
	// helpers constructed without a *cobra.Command in scope (the script runner and network
	// managers' Logf closures) narrate to the same stream vlogf writes to.
	stderr io.Writer

	// runnerOverride, when set, is used by newScriptRunner instead of constructing a real
	// ledger.Runner -- test-only (see cmd_wiring_test.go's fakeRunner), never set by any
	// cobra flag or production code path.
	runnerOverride scriptRunner
}

func newRootCmd() *cobra.Command {
	return newRootCmdForApp(&app{})
}

// newRootCmdForApp builds the root command around a caller-supplied *app -- split out from
// newRootCmd so wiring tests (cmd_wiring_test.go) can pre-set a's test-only fields (namely
// runnerOverride) before Execute() runs, without touching any cobra flag.
func newRootCmdForApp(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "ntt-playground",
		Short:         "Playground CLI for the Canton NTT contracts (devnet only)",
		SilenceUsage:  true,
		SilenceErrors: false,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			a.stderr = cmd.ErrOrStderr()
			a.resolveProfileFromState(cmd)
		},
	}

	root.PersistentFlags().StringVar(&a.stateFile, "state-file", state.DefaultFileName, "path to the playground state file")
	root.PersistentFlags().StringVar(&a.dpmPath, "dpm-path", "", "path to the dpm binary (default: PATH or ~/.dpm/bin)")
	root.PersistentFlags().StringVar(&a.cantonDir, "canton-dir", "", "path to the canton/ directory (default: discovered by walking up from the working directory)")
	root.PersistentFlags().StringVar(&a.runDir, "run-dir", "", "directory for network run-state and script I/O files (default: alongside the state file)")
	root.PersistentFlags().StringVar((*string)(&a.profile), "profile", string(profile.Sandbox), "network profile: sandbox|localnet")
	root.PersistentFlags().BoolVar(&a.verbose, "verbose", false, "narrate every sub-step (network bring-up, party allocation, each dpm script run) on stderr")
	root.PersistentFlags().StringVar(&a.topologyConfigPath, "topology-config", "", "path to the topology/disclosure config file (default: playground.topology.json next to the state file)")
	root.PersistentFlags().StringVar(&a.disclosureServiceURL, "disclosure-service-url", "", "base URL of a disclosure service to fetch cross-participant RemoteSeams from (default: run the prepare script directly against the data owner's participant)")
	root.PersistentFlags().BoolVar(&a.strictParticipantIsolation, "strict-participant-isolation", false, "fail any step that would target a participant other than the actor's own (see --disclosure-service-url)")

	root.AddCommand(
		newNetworkCmd(a),
		newInitCmd(a),
		newDeployCmd(a),
		newPeerCmd(a),
		newPauseCmd(a),
		newUnpauseCmd(a),
		newPartyCmd(a),
		newEmitterCmd(a),
		newPublishCmd(a),
		newFundCmd(a),
		newTransferCmd(a),
		newReceiveCmd(a),
		newPreapproveCmd(a),
		newAdminCmd(a),
		newGuardianCmd(a),
		newStatusCmd(a),
		newContractsCmd(a),
		newBalanceCmd(a),
		newObserveCmd(a),
		newDisclosureCmd(a),
	)
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// ----------------------------------------------------------------------
// Shared app helpers
// ----------------------------------------------------------------------

// vlogf narrates one verbose sub-step to the command's stderr with the "[v] " prefix. No-op
// unless --verbose is set, and never writes to stdout -- machine-parseable output (the
// field=value tokens the e2e suite extracts) stays byte-identical either way.
func (a *app) vlogf(cmd *cobra.Command, format string, args ...any) {
	if !a.verbose {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "[v] "+format+"\n", args...)
}

// verboseLogf returns a "[v] "-prefixed logger for the internal packages' nil-safe Logf
// fields (ledger.Runner, network managers), or nil when --verbose is off -- the packages
// stay prefix-agnostic and silent by default.
func (a *app) verboseLogf() func(format string, args ...any) {
	if !a.verbose {
		return nil
	}
	w := a.stderr
	if w == nil {
		w = os.Stderr
	}
	return func(format string, args ...any) {
		fmt.Fprintf(w, "[v] "+format+"\n", args...)
	}
}

// resolvedCantonDir returns a.cantonDir if set, else discovers it by walking up from the
// working directory looking for multi-package.yaml (the canton/ root's marker file).
func (a *app) resolvedCantonDir() (string, error) {
	if a.cantonDir != "" {
		return a.cantonDir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("canton-dir: getwd: %w", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "multi-package.yaml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("canton-dir: could not find multi-package.yaml by walking up from the working directory; pass --canton-dir")
}

func (a *app) darPath() (string, error) {
	dir, err := a.resolvedCantonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "test", ".daml", "dist", "ntt-test-0.1.0.dar"), nil
}

func (a *app) resolvedRunDir() string {
	if a.runDir != "" {
		return a.runDir
	}
	dir := filepath.Dir(a.stateFile)
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, ".ntt-playground-run")
}

func (a *app) loadState() (*state.State, error) {
	if _, err := os.Stat(a.stateFile); err != nil {
		return nil, fmt.Errorf("no playground state at %s -- run `init` first", a.stateFile)
	}
	return state.Load(a.stateFile)
}

func (a *app) saveState(s *state.State) error {
	return s.Save(a.stateFile)
}

func (a *app) resolvedProfile() (profile.Profile, error) {
	return profile.Get(a.profile)
}

// resolvedTopologyConfigPath returns a.topologyConfigPath if set, else the default path
// (playground.topology.json alongside the state file) -- mirrors resolvedRunDir's pattern.
func (a *app) resolvedTopologyConfigPath() string {
	if a.topologyConfigPath != "" {
		return a.topologyConfigPath
	}
	dir := filepath.Dir(a.stateFile)
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "playground.topology.json")
}

// topologyConfig loads the party-hosting/disclosure config used to route a hint to its
// participant (see resolveParticipantRole in party.go) and to gate which templates a
// cross-participant submit is allowed to fetch a disclosure for (see cmd/ntt-playground/
// remote.go). A missing file at the resolved path is not an error -- disclosure.Load already
// degrades to the built-in localnet defaults (empty Disclose list, default partyHosting) --
// but a PRESENT, malformed file is: propagated to the caller rather than silently ignored, so
// a typo in a hand-written --topology-config fails loudly instead of quietly disclosing
// nothing.
func (a *app) topologyConfig() (disclosure.Config, error) {
	return disclosure.Load(a.resolvedTopologyConfigPath())
}

// resolveProfileFromState defaults a.profile to the profile recorded in the state file at
// a.stateFile, but only when the user did not pass --profile explicitly on this invocation.
// This is what lets a bare `ntt-playground party list` after `just localnet-cli` (which
// records "profile":"localnet" in the state file) target the same ledger the user just bootstrapped,
// instead of silently falling back to the sandbox default and failing against a JVM that was
// never started. An explicit --profile always wins; a missing state file, an unreadable one, or
// one with no recorded profile leaves the "sandbox" default alone. `init` is unaffected: it
// populates the profile field, and with no prior state file this is a no-op.
func (a *app) resolveProfileFromState(cmd *cobra.Command) {
	if cmd.Flags().Changed("profile") {
		return
	}
	s, err := state.Load(a.stateFile)
	if err != nil || s.Profile == "" {
		return
	}
	a.profile = profile.Name(s.Profile)
}
