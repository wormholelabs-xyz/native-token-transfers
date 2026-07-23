package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/ledger"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
)

// newScriptRunner builds a ledger.Runner against the profile's DEFAULT participant (e.g.
// app-provider on LocalNet) -- the zero-arg wrapper the plan's §3 keeps around so read-only,
// operator-side commands (status, contracts, observe, applyGovernance, deploy admins, ...)
// stay unchanged. Equivalent to newScriptRunnerFor(ctx, "").
func (a *app) newScriptRunner(ctx context.Context) (*ledger.Runner, func(), error) {
	return a.newScriptRunnerFor(ctx, "")
}

// newScriptRunnerFor builds a ledger.Runner against the participant role resolves to via the
// current profile's Endpoint(role) (see internal/profile.Profile.Endpoint) -- "" or an
// unrecognized role falls back to the profile's default participant. Mints a LocalNet access
// token file when required. The returned cleanup func removes the per-invocation script I/O
// directory.
func (a *app) newScriptRunnerFor(ctx context.Context, role string) (*ledger.Runner, func(), error) {
	prof, err := a.resolvedProfile()
	if err != nil {
		return nil, nil, err
	}
	ep, err := prof.Endpoint(role)
	if err != nil {
		return nil, nil, err
	}
	dar, err := a.darPath()
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(dar); err != nil {
		return nil, nil, fmt.Errorf("runner: DAR not found at %s -- run `dpm build --all` in canton/ first: %w", dar, err)
	}

	workDir, err := os.MkdirTemp("", "ntt-playground-script-*")
	if err != nil {
		return nil, nil, fmt.Errorf("runner: create script work dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(workDir) }

	var tokenFile string
	if prof.RequiresAuth {
		tokenFile = filepath.Join(workDir, "token.jwt")
		if err := network.WriteTokenFile(tokenFile, ep.UserID, time.Hour); err != nil {
			cleanup()
			return nil, nil, err
		}
	}

	r, err := ledger.NewRunner(a.dpmPath, dar, prof, ep, tokenFile, workDir)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	r.Logf = a.verboseLogf() // nil unless --verbose; narrates every dpm script invocation
	return r, cleanup, nil
}
