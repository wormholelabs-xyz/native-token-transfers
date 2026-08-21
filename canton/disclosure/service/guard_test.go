package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingACS records call counts and can block, so tests can observe fan-out and concurrency.
type countingACS struct {
	ledgerCalls atomic.Int64
	acCalls     atomic.Int64
	offset      int64

	release  chan struct{} // when non-nil, every call waits on it before returning
	inFlight atomic.Int64
	peak     atomic.Int64
	err      error
}

func (f *countingACS) track() func() {
	n := f.inFlight.Add(1)
	for {
		peak := f.peak.Load()
		if n <= peak || f.peak.CompareAndSwap(peak, n) {
			break
		}
	}
	return func() { f.inFlight.Add(-1) }
}

func (f *countingACS) LedgerEnd(context.Context) (int64, error) {
	defer f.track()()
	f.ledgerCalls.Add(1)
	if f.release != nil {
		<-f.release
	}
	if f.err != nil {
		return 0, f.err
	}
	return f.offset, nil
}

func (f *countingACS) ActiveContracts(_ context.Context, _ []string, template string, offset int64) ([]acsEntry, error) {
	defer f.track()()
	f.acCalls.Add(1)
	if f.release != nil {
		<-f.release
	}
	if f.err != nil {
		return nil, f.err
	}
	return []acsEntry{{ContractID: fmt.Sprintf("%s@%d", template, offset)}}, nil
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

func TestCachingACS_ServesRepeatReadsFromMemory(t *testing.T) {
	inner := &countingACS{offset: 42}
	c := newCachingACS(inner, 5*time.Second)

	for i := 0; i < 5; i++ {
		offset, err := c.LedgerEnd(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(42), offset)

		entries, err := c.ActiveContracts(context.Background(), []string{"Alice"}, "T", offset)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	}

	assert.Equal(t, int64(1), inner.ledgerCalls.Load(), "ledger-end should be fetched once")
	assert.Equal(t, int64(1), inner.acCalls.Load(), "active-contracts should be fetched once")
}

func TestCachingACS_RefetchesAfterTTL(t *testing.T) {
	inner := &countingACS{offset: 42}
	c := newCachingACS(inner, 5*time.Second)
	now := time.Now()
	c.now = func() time.Time { return now }

	_, err := c.LedgerEnd(context.Background())
	require.NoError(t, err)
	_, err = c.LedgerEnd(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), inner.ledgerCalls.Load())

	now = now.Add(6 * time.Second) // past the TTL
	_, err = c.LedgerEnd(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), inner.ledgerCalls.Load(), "an expired entry must be refetched")
}

// TestCachingACS_CollapsesConcurrentMisses pins the single-flight behavior: a burst arriving on
// a cold cache produces one upstream call, not one per caller.
func TestCachingACS_CollapsesConcurrentMisses(t *testing.T) {
	inner := &countingACS{offset: 42, release: make(chan struct{})}
	c := newCachingACS(inner, 5*time.Second)

	const callers = 20
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.LedgerEnd(context.Background())
		}(i)
	}

	require.Eventually(t, func() bool { return inner.ledgerCalls.Load() == 1 }, 2*time.Second, 5*time.Millisecond)
	close(inner.release)
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, int64(1), inner.ledgerCalls.Load(), "concurrent misses must share one upstream call")
}

func TestCachingACS_KeysOnOffsetAndTemplate(t *testing.T) {
	inner := &countingACS{}
	c := newCachingACS(inner, 5*time.Second)

	_, err := c.ActiveContracts(context.Background(), []string{"Alice"}, "T1", 1)
	require.NoError(t, err)
	_, err = c.ActiveContracts(context.Background(), []string{"Alice"}, "T2", 1)
	require.NoError(t, err)
	_, err = c.ActiveContracts(context.Background(), []string{"Alice"}, "T1", 2)
	require.NoError(t, err)
	_, err = c.ActiveContracts(context.Background(), []string{"Alice"}, "T1", 1)
	require.NoError(t, err)

	assert.Equal(t, int64(3), inner.acCalls.Load(), "distinct template/offset pairs are distinct keys")
}

// TestCachingACS_DoesNotCacheErrors keeps a transient upstream failure from being served for a
// whole TTL window.
func TestCachingACS_DoesNotCacheErrors(t *testing.T) {
	inner := &countingACS{err: fmt.Errorf("upstream down")}
	c := newCachingACS(inner, 5*time.Second)

	_, err := c.LedgerEnd(context.Background())
	require.Error(t, err)
	_, err = c.LedgerEnd(context.Background())
	require.Error(t, err)

	assert.Equal(t, int64(2), inner.ledgerCalls.Load(), "a failed fetch must not be cached")
}

func TestCachingACS_EvictsWhenFull(t *testing.T) {
	inner := &countingACS{}
	c := newCachingACS(inner, 5*time.Second)

	for i := 0; i < maxCacheEntries+50; i++ {
		_, err := c.ActiveContracts(context.Background(), []string{"Alice"}, "T", int64(i))
		require.NoError(t, err)
	}

	c.mu.Lock()
	size := len(c.entries)
	c.mu.Unlock()
	assert.LessOrEqual(t, size, maxCacheEntries, "the cache must stay bounded")
}

// ---------------------------------------------------------------------------
// Concurrency bound
// ---------------------------------------------------------------------------

// TestBoundedACS_CapsUpstreamConcurrency is the participant-protection invariant: no matter how
// many requests arrive, in-flight upstream calls never exceed the limit.
func TestBoundedACS_CapsUpstreamConcurrency(t *testing.T) {
	const limit = 3
	inner := &countingACS{offset: 42, release: make(chan struct{})}
	b := newBoundedACS(inner, limit)
	b.wait = time.Minute // callers queue rather than shed, so the peak is the thing measured

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = b.ActiveContracts(context.Background(), []string{"Alice"}, "T", 1)
		}()
	}

	require.Eventually(t, func() bool { return inner.inFlight.Load() == int64(limit) }, 2*time.Second, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond) // give any excess a chance to slip through
	assert.LessOrEqual(t, inner.peak.Load(), int64(limit), "upstream concurrency exceeded the cap")

	close(inner.release)
	wg.Wait()
	assert.LessOrEqual(t, inner.peak.Load(), int64(limit))
}

// TestBoundedACS_ShedsWhenSaturated pins the 503 path: a caller that cannot get a slot in time
// is refused rather than queued without limit.
func TestBoundedACS_ShedsWhenSaturated(t *testing.T) {
	inner := &countingACS{offset: 42, release: make(chan struct{})}
	b := newBoundedACS(inner, 1)
	b.wait = 20 * time.Millisecond

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = b.LedgerEnd(context.Background())
	}()
	require.Eventually(t, func() bool { return inner.inFlight.Load() == 1 }, time.Second, 5*time.Millisecond)

	_, err := b.LedgerEnd(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errUpstreamBusy)
	assert.Equal(t, http.StatusServiceUnavailable, upstreamErrorStatus(err))

	close(inner.release)
	wg.Wait()
}

func TestBoundedACS_RespectsContextCancellation(t *testing.T) {
	inner := &countingACS{offset: 42, release: make(chan struct{})}
	b := newBoundedACS(inner, 1)
	b.wait = time.Minute

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = b.LedgerEnd(context.Background())
	}()
	require.Eventually(t, func() bool { return inner.inFlight.Load() == 1 }, time.Second, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.LedgerEnd(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)

	close(inner.release)
	wg.Wait()
}

func TestUpstreamErrorStatus_UpstreamFaultIsBadGateway(t *testing.T) {
	assert.Equal(t, http.StatusBadGateway, upstreamErrorStatus(fmt.Errorf("connection refused")))
	assert.Equal(t, http.StatusServiceUnavailable, upstreamErrorStatus(errUpstreamBusy))
}

// TestGuardsCompose checks the wiring run() builds: a cache hit must not consume a semaphore
// slot, so repeat traffic stays cheap even when the bound is saturated.
func TestGuardsCompose(t *testing.T) {
	inner := &countingACS{offset: 42}
	guarded := newCachingACS(newBoundedACS(inner, 1), 5*time.Second)

	for i := 0; i < 10; i++ {
		offset, err := guarded.LedgerEnd(context.Background())
		require.NoError(t, err)
		require.Equal(t, int64(42), offset)
	}
	assert.Equal(t, int64(1), inner.ledgerCalls.Load())
}
