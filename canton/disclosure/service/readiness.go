package main

import (
	"context"
	"sync"
	"time"
)

const (
	// readinessCheckTimeout bounds one upstream ledger-end call of /v1/readyz.
	readinessCheckTimeout = 3 * time.Second
	// readinessResultTTL is how long each /v1/readyz caller gets the last result.
	readinessResultTTL = 2 * time.Second
)

// readinessProbe bounds the upstream load of /v1/readyz: at most one ledger-end call in each
// resultTTL window, each under checkTimeout. /v1/readyz is outside the inbound cap, so this
// bound is the only one that applies to it.
type readinessProbe struct {
	acs          acsClient
	checkTimeout time.Duration
	resultTTL    time.Duration
	now          func() time.Time
	checkSlot    chan struct{} // capacity 1: one upstream check at a time

	mu            sync.Mutex
	hasResult     bool
	lastCheckedAt time.Time
	lastCheckErr  error
}

func newReadinessProbe(acs acsClient) *readinessProbe {
	return &readinessProbe{
		acs:          acs,
		checkTimeout: readinessCheckTimeout,
		resultTTL:    readinessResultTTL,
		now:          time.Now,
		checkSlot:    make(chan struct{}, 1),
	}
}

// cachedResult returns the last result while it is younger than resultTTL.
func (probe *readinessProbe) cachedResult() (fresh bool, checkErr error) {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if !probe.hasResult || !probe.now().Before(probe.lastCheckedAt.Add(probe.resultTTL)) {
		return false, nil
	}
	return true, probe.lastCheckErr
}

// checkLedgerEnd returns nil when the upstream answered a ledger-end call in the last resultTTL.
// A caller waits for the check in progress, or until its own ctx ends. ranUpstreamCheck is true
// for the one caller in each resultTTL window that made the upstream call.
func (probe *readinessProbe) checkLedgerEnd(ctx context.Context) (checkErr error, ranUpstreamCheck bool) {
	if fresh, cachedErr := probe.cachedResult(); fresh {
		return cachedErr, false
	}
	select {
	case probe.checkSlot <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err(), false
	}
	defer func() { <-probe.checkSlot }()
	if fresh, cachedErr := probe.cachedResult(); fresh {
		return cachedErr, false
	}

	// The check outlives a canceled caller, so its result is not that caller's cancellation.
	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), probe.checkTimeout)
	defer cancel()
	_, checkErr = probe.acs.LedgerEnd(checkCtx)

	probe.mu.Lock()
	probe.hasResult = true
	probe.lastCheckedAt = probe.now()
	probe.lastCheckErr = checkErr
	probe.mu.Unlock()
	return checkErr, true
}
