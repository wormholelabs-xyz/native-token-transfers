package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/network"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/profile"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
)

// resolveGuardianObserverEndpoint resolves the guardian-observer participant's endpoint and
// loads state, applying the two guards every `observe` subcommand needs: the profile must be
// localnet (guardian-observer has no JSON API on the sandbox), and state must already have a
// guardianObserver party (from `init`). cmdName prefixes returned errors, matching each
// caller's own command name.
func resolveGuardianObserverEndpoint(a *app, cmdName string) (profile.Endpoint, *state.State, error) {
	prof, err := a.resolvedProfile()
	if err != nil {
		return profile.Endpoint{}, nil, err
	}
	// The guardian observation genuinely happens on the guardians' own node: GO is an
	// observer of Emitter/CoreState, so its OWN participant is what receives those
	// projections and must serve the stream (plan §3's "observe stream" routing).
	ep, err := prof.Endpoint("guardian-observer")
	if err != nil {
		return profile.Endpoint{}, nil, err
	}
	if ep.JSONAPIBaseURL == "" {
		return profile.Endpoint{}, nil, fmt.Errorf("%s: requires the localnet profile", cmdName)
	}
	s, err := a.loadState()
	if err != nil {
		return profile.Endpoint{}, nil, err
	}
	if s.GuardianObserver == "" {
		return profile.Endpoint{}, nil, fmt.Errorf("%s: no guardianObserver party in state -- run `init` first", cmdName)
	}
	return ep, s, nil
}

// mintGuardianReaderToken ensures guardianWatcherUser exists on ep's participant with ONLY
// CanReadAs(guardianObserver) -- never actAs -- then mints a fresh token for it with the
// given ttl. This is the CLI's one non-`dpm script` ledger surface, and it is strictly
// read-only: no command on this path submits anything.
func mintGuardianReaderToken(ctx context.Context, a *app, cmd *cobra.Command, cmdName string, ep profile.Endpoint, guardianObserver string, ttl time.Duration) (string, error) {
	adminToken, err := network.MintUnsafeToken(network.LocalNetAdminUser, time.Hour)
	if err != nil {
		return "", err
	}
	a.vlogf(cmd, "%s: ensuring reader user %q exists with ONLY CanReadAs(%s)", cmdName, guardianWatcherUser, guardianObserver)
	if err := network.CreateLedgerUser(ctx, ep.JSONAPIBaseURL, adminToken, guardianWatcherUser, []string{guardianObserver}, nil); err != nil {
		return "", fmt.Errorf("%s: create reader user: %w", cmdName, err)
	}
	return network.MintUnsafeToken(guardianWatcherUser, ttl)
}
