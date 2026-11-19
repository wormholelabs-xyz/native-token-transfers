// Upstream guards: a TTL cache and a concurrency bound, both wrapping acsClient.
//
// One flow request fans out to several templates plus a ledger-end call, so inbound traffic
// reaches the participant amplified. These two decorators bound that.
//
// Compose them cache-outermost (see run()): a cache hit answers without holding a semaphore
// slot, so cheap repeat traffic never competes with work that must go upstream.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// errUpstreamBusy reports that the concurrency bound shed this call. Handlers map it to 503.
var errUpstreamBusy = errors.New("disclosure-service: upstream busy")

// errUpstreamFetchPanicked reports a recovered panic inside a shared upstream fetch.
var errUpstreamFetchPanicked = errors.New("disclosure-service: upstream fetch panicked")

// upstreamFetchTimeout bounds one shared upstream fetch. It is above httpClientTimeout, so the
// HTTP client times out first.
const upstreamFetchTimeout = 35 * time.Second

// upstreamErrorStatus separates load shedding (503) and a timeout (504) from an upstream fault (502).
func upstreamErrorStatus(err error) int {
	if errors.Is(err, errUpstreamBusy) {
		return http.StatusServiceUnavailable
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

// upstreamAcquireTimeout is how long a call waits for a semaphore slot before it is shed. It
// absorbs short bursts; sustained overload sheds rather than queues without limit.
const upstreamAcquireTimeout = 2 * time.Second

// maxCacheEntries caps the cache. Keys carry a ledger offset, so they turn over as the ledger
// advances; a sweep on write keeps the map from growing without bound.
const maxCacheEntries = 512

// ---------------------------------------------------------------------------
// Concurrency bound
// ---------------------------------------------------------------------------

// boundedACS caps in-flight upstream calls. The participant is shared with other workloads, so
// the cap is what makes the service's load on it independent of inbound request volume.
type boundedACS struct {
	inner acsClient
	sem   chan struct{}
	wait  time.Duration
}

func newBoundedACS(inner acsClient, limit int) *boundedACS {
	return &boundedACS{inner: inner, sem: make(chan struct{}, limit), wait: upstreamAcquireTimeout}
}

func (bounded *boundedACS) acquire(ctx context.Context) error {
	timer := time.NewTimer(bounded.wait)
	defer timer.Stop()
	select {
	case bounded.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errUpstreamBusy
	}
}

func (bounded *boundedACS) release() { <-bounded.sem }

func (bounded *boundedACS) LedgerEnd(ctx context.Context) (int64, error) {
	if err := bounded.acquire(ctx); err != nil {
		return 0, err
	}
	defer bounded.release()
	return bounded.inner.LedgerEnd(ctx)
}

func (bounded *boundedACS) ActiveContracts(ctx context.Context, query activeContractsQuery) ([]acsEntry, error) {
	if err := bounded.acquire(ctx); err != nil {
		return nil, err
	}
	defer bounded.release()
	return bounded.inner.ActiveContracts(ctx, query)
}

func (bounded *boundedACS) ActiveContractByID(ctx context.Context, readerParties []partyID, wantContractID contractID) (acsEntry, bool, error) {
	if err := bounded.acquire(ctx); err != nil {
		return acsEntry{}, false, err
	}
	defer bounded.release()
	return bounded.inner.ActiveContractByID(ctx, readerParties, wantContractID)
}

func (b *boundedACS) ActiveContractAmountTotals(ctx context.Context, query amountTotalsQuery) (amountTotalsByGroup, error) {
	if err := b.acquire(ctx); err != nil {
		return nil, err
	}
	defer b.release()
	return b.inner.ActiveContractAmountTotals(ctx, query)
}

// ---------------------------------------------------------------------------
// TTL cache with single flight
// ---------------------------------------------------------------------------

// cacheEntry is one upstream answer: a result, or the error of a failed fetch.
type cacheEntry struct {
	result  any
	err     error
	expires time.Time
}

// inflightCall lets concurrent callers for one key share a single upstream result. result and err
// are written before done is closed.
type inflightCall struct {
	done   chan struct{}
	result any
	err    error
}

// cachingACS serves repeat reads from memory for ttl.
//
// Caching LedgerEnd is what makes the ActiveContracts cache effective: within one ttl window
// every request resolves the same offset, so their ActiveContracts keys collide and hit. A
// caller can therefore receive a set up to ttl old; a contract archived inside that window
// yields a blob whose submission fails, and the caller refetches.
//
// A failed fetch is cached for ttl also, so a template above an upstream bound costs one upstream
// read per window. Two errors are not cached: errUpstreamBusy (local load shedding) and
// errUpstreamFetchPanicked (a service bug).
type cachingACS struct {
	inner        acsClient
	ttl          time.Duration
	now          func() time.Time // injectable so tests advance time without sleeping
	fetchTimeout time.Duration
	logger       *log.Logger

	mu       sync.Mutex
	entries  map[string]cacheEntry
	inflight map[string]*inflightCall
}

func newCachingACS(inner acsClient, ttl time.Duration) *cachingACS {
	return &cachingACS{
		inner:        inner,
		ttl:          ttl,
		now:          time.Now,
		fetchTimeout: upstreamFetchTimeout,
		logger:       log.New(os.Stderr, "", log.LstdFlags),
		entries:      make(map[string]cacheEntry),
		inflight:     make(map[string]*inflightCall),
	}
}

// loadOrFetch returns key's cached answer, joins the call already fetching it, or starts the
// fetch. The fetch runs under a service-owned deadline: each caller's cancel ends only its own
// wait. Each caller stops waiting when its own ctx ends.
func (cache *cachingACS) loadOrFetch(ctx context.Context, key string, fetch func(context.Context) (any, error)) (any, error) {
	cache.mu.Lock()
	if cached, ok := cache.entries[key]; ok && cache.now().Before(cached.expires) {
		cache.mu.Unlock()
		return cached.result, cached.err
	}
	if call, ok := cache.inflight[key]; ok {
		cache.mu.Unlock()
		return awaitInflightCall(ctx, call)
	}
	call := &inflightCall{done: make(chan struct{})}
	cache.inflight[key] = call
	cache.mu.Unlock()

	go cache.runInflightFetch(key, call, fetch)
	return awaitInflightCall(ctx, call)
}

// runInflightFetch runs fetch under fetchTimeout. The deferred cleanup also runs on a panic.
func (cache *cachingACS) runInflightFetch(key string, call *inflightCall, fetch func(context.Context) (any, error)) {
	fetchCtx, cancel := context.WithTimeout(context.Background(), cache.fetchTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			call.result = nil
			call.err = fmt.Errorf("%w: %v", errUpstreamFetchPanicked, recovered)
			cache.logger.Printf("disclosure-service: upstream fetch %q panic: %q\n%s", key, fmt.Sprint(recovered), debug.Stack())
		}
		cache.mu.Lock()
		if cache.inflight[key] != call {
			panic("disclosure-service: inflight entry replaced while its fetch ran")
		}
		delete(cache.inflight, key)
		if isCacheableFetchResult(call.err) {
			cache.sweepLocked()
			cache.entries[key] = cacheEntry{result: call.result, err: call.err, expires: cache.now().Add(cache.ttl)}
		}
		cache.mu.Unlock()
		close(call.done)
	}()
	call.result, call.err = fetch(fetchCtx)
}

func isCacheableFetchResult(fetchErr error) bool {
	return !errors.Is(fetchErr, errUpstreamBusy) && !errors.Is(fetchErr, errUpstreamFetchPanicked)
}

func awaitInflightCall(ctx context.Context, call *inflightCall) (any, error) {
	select {
	case <-call.done:
		return call.result, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sweepLocked drops expired entries once the map reaches its cap, then drops arbitrary entries
// if every one is still live. The caller holds cache.mu.
func (cache *cachingACS) sweepLocked() {
	if len(cache.entries) < maxCacheEntries {
		return
	}
	now := cache.now()
	for key, cached := range cache.entries {
		if !now.Before(cached.expires) {
			delete(cache.entries, key)
		}
	}
	for key := range cache.entries {
		if len(cache.entries) < maxCacheEntries {
			break
		}
		delete(cache.entries, key)
	}
}

func (cache *cachingACS) LedgerEnd(ctx context.Context) (int64, error) {
	cachedValue, err := cache.loadOrFetch(ctx, "ledger-end", func(fetchCtx context.Context) (any, error) { return cache.inner.LedgerEnd(fetchCtx) })
	if err != nil {
		return 0, err
	}
	offset, ok := cachedValue.(int64)
	if !ok {
		return 0, fmt.Errorf("disclosure-service: cached ledger-end has type %T", cachedValue)
	}
	if offset < 1 {
		return 0, fmt.Errorf("%w: cached %d", errLedgerEndOffsetNotPositive, offset)
	}
	return offset, nil
}

// ActiveContracts caches the filtered result, so an entry holds at most query.maxMatches contracts.
func (cache *cachingACS) ActiveContracts(ctx context.Context, query activeContractsQuery) ([]acsEntry, error) {
	cachedValue, err := cache.loadOrFetch(ctx, query.cacheKey(), func(fetchCtx context.Context) (any, error) {
		return cache.inner.ActiveContracts(fetchCtx, query)
	})
	if err != nil {
		return nil, err
	}
	entries, ok := cachedValue.([]acsEntry)
	if !ok {
		return nil, fmt.Errorf("disclosure-service: cached active-contracts has type %T", cachedValue)
	}
	return entries, nil
}

// contractByIDResult is one cached ActiveContractByID answer.
type contractByIDResult struct {
	entry acsEntry
	found bool
}

func (cache *cachingACS) ActiveContractByID(ctx context.Context, readerParties []partyID, wantContractID contractID) (acsEntry, bool, error) {
	encodedKey, err := json.Marshal(struct {
		Kind       string
		Readers    []partyID
		ContractID contractID
	}{"contract-by-id", readerParties, wantContractID})
	if err != nil {
		return acsEntry{}, false, fmt.Errorf("disclosure-service: encode cache key: %w", err)
	}
	cachedValue, err := cache.loadOrFetch(ctx, string(encodedKey), func(fetchCtx context.Context) (any, error) {
		entry, found, err := cache.inner.ActiveContractByID(fetchCtx, readerParties, wantContractID)
		return contractByIDResult{entry: entry, found: found}, err
	})
	if err != nil {
		return acsEntry{}, false, err
	}
	result, ok := cachedValue.(contractByIDResult)
	if !ok {
		return acsEntry{}, false, fmt.Errorf("disclosure-service: cached contract-by-id has type %T", cachedValue)
	}
	return result.entry, result.found, nil
}

// ActiveContractAmountTotals gives each caller its own copy of the cached totals.
func (c *cachingACS) ActiveContractAmountTotals(ctx context.Context, query amountTotalsQuery) (amountTotalsByGroup, error) {
	cachedValue, err := c.do(ctx, query.cacheKey(), func(fetchCtx context.Context) (any, error) {
		return c.inner.ActiveContractAmountTotals(fetchCtx, query)
	})
	if err != nil {
		return nil, err
	}
	totals, ok := cachedValue.(amountTotalsByGroup)
	if !ok {
		return nil, fmt.Errorf("disclosure-service: cached amount totals has type %T", cachedValue)
	}
	return totals.clone(), nil
}
