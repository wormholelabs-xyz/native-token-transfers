// Command ntt-playground is a devnet playground CLI for the Canton NTT contracts under
// canton/: stand up a network, deploy an NTT, control a 1/1 Wormhole guardian to sign VAAs
// on the fly, and drive inbound/outbound transfers end to end. See the README in this
// directory for full usage and the LocalNet/MainNet paths.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

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
}

func newRootCmd() *cobra.Command {
	a := &app{}

	root := &cobra.Command{
		Use:           "ntt-playground",
		Short:         "Playground CLI for the Canton NTT contracts (devnet only)",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.PersistentFlags().StringVar(&a.stateFile, "state-file", state.DefaultFileName, "path to the playground state file")
	root.PersistentFlags().StringVar(&a.dpmPath, "dpm-path", "", "path to the dpm binary (default: PATH or ~/.dpm/bin)")
	root.PersistentFlags().StringVar(&a.cantonDir, "canton-dir", "", "path to the canton/ directory (default: discovered by walking up from the working directory)")
	root.PersistentFlags().StringVar(&a.runDir, "run-dir", "", "directory for network run-state and script I/O files (default: alongside the state file)")
	root.PersistentFlags().StringVar((*string)(&a.profile), "profile", string(profile.Sandbox), "network profile: sandbox|localnet")

	root.AddCommand(
		newNetworkCmd(a),
		newInitCmd(a),
		newDeployCmd(a),
		newPeerCmd(a),
		newTransferCmd(a),
		newReceiveCmd(a),
		newGuardianCmd(a),
		newStatusCmd(a),
		newBalanceCmd(a),
		newObserveCmd(a),
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
