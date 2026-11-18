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

func (b *boundedACS) acquire(ctx context.Context) error {
	timer := time.NewTimer(b.wait)
	defer timer.Stop()
	select {
	case b.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errUpstreamBusy
	}
}

func (b *boundedACS) release() { <-b.sem }

func (b *boundedACS) LedgerEnd(ctx context.Context) (int64, error) {
	if err := b.acquire(ctx); err != nil {
		return 0, err
	}
	defer b.release()
	return b.inner.LedgerEnd(ctx)
}

func (b *boundedACS) ActiveContracts(ctx context.Context, query activeContractsQuery) ([]acsEntry, error) {
	if err := b.acquire(ctx); err != nil {
		return nil, err
	}
	defer b.release()
	return b.inner.ActiveContracts(ctx, query)
}

func (b *boundedACS) ActiveContractByID(ctx context.Context, readerParties []partyID, wantContractID contractID) (acsEntry, bool, error) {
	if err := b.acquire(ctx); err != nil {
		return acsEntry{}, false, err
	}
	defer b.release()
	return b.inner.ActiveContractByID(ctx, readerParties, wantContractID)
}

// ---------------------------------------------------------------------------
// TTL cache with single flight
// ---------------------------------------------------------------------------

type cacheEntry struct {
	value   any
	expires time.Time
}

// inflightCall lets concurrent callers for one key share a single upstream result. value and err
// are written before done is closed.
type inflightCall struct {
	done  chan struct{}
	value any
	err   error
}

// cachingACS serves repeat reads from memory for ttl.
//
// Caching LedgerEnd is what makes the ActiveContracts cache effective: within one ttl window
// every request resolves the same offset, so their ActiveContracts keys collide and hit. A
// caller can therefore receive a set up to ttl old; a contract archived inside that window
// yields a blob whose submission fails, and the caller refetches.
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

// do returns key's cached value, joins the call already fetching it, or starts the fetch. The
// fetch runs under a service-owned deadline, so one caller's cancel does not fail the others. Each
// caller stops waiting when its own ctx ends.
func (c *cachingACS) do(ctx context.Context, key string, fetch func(context.Context) (any, error)) (any, error) {
	c.mu.Lock()
	if cached, ok := c.entries[key]; ok && c.now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.value, nil
	}
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		return awaitInflightCall(ctx, call)
	}
	call := &inflightCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	go c.runInflightFetch(key, call, fetch)
	return awaitInflightCall(ctx, call)
}

// runInflightFetch runs fetch under fetchTimeout. The deferred cleanup also runs on a panic.
func (c *cachingACS) runInflightFetch(key string, call *inflightCall, fetch func(context.Context) (any, error)) {
	fetchCtx, cancel := context.WithTimeout(context.Background(), c.fetchTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			call.value = nil
			call.err = fmt.Errorf("%w: %v", errUpstreamFetchPanicked, recovered)
			c.logger.Printf("disclosure-service: upstream fetch %q panic: %v\n%s", key, recovered, debug.Stack())
		}
		c.mu.Lock()
		if c.inflight[key] != call {
			panic("disclosure-service: inflight entry replaced while its fetch ran")
		}
		delete(c.inflight, key)
		if call.err == nil {
			c.sweepLocked()
			c.entries[key] = cacheEntry{value: call.value, expires: c.now().Add(c.ttl)}
		}
		c.mu.Unlock()
		close(call.done)
	}()
	call.value, call.err = fetch(fetchCtx)
}

func awaitInflightCall(ctx context.Context, call *inflightCall) (any, error) {
	select {
	case <-call.done:
		return call.value, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sweepLocked drops expired entries once the map reaches its cap, then drops arbitrary entries
// if every one is still live. The caller holds c.mu.
func (c *cachingACS) sweepLocked() {
	if len(c.entries) < maxCacheEntries {
		return
	}
	now := c.now()
	for key, cached := range c.entries {
		if !now.Before(cached.expires) {
			delete(c.entries, key)
		}
	}
	for key := range c.entries {
		if len(c.entries) < maxCacheEntries {
			break
		}
		delete(c.entries, key)
	}
}

func (c *cachingACS) LedgerEnd(ctx context.Context) (int64, error) {
	cachedValue, err := c.do(ctx, "ledger-end", func(fetchCtx context.Context) (any, error) { return c.inner.LedgerEnd(fetchCtx) })
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
func (c *cachingACS) ActiveContracts(ctx context.Context, query activeContractsQuery) ([]acsEntry, error) {
	cachedValue, err := c.do(ctx, query.cacheKey(), func(fetchCtx context.Context) (any, error) {
		return c.inner.ActiveContracts(fetchCtx, query)
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

func (c *cachingACS) ActiveContractByID(ctx context.Context, readerParties []partyID, wantContractID contractID) (acsEntry, bool, error) {
	encodedKey, err := json.Marshal(struct {
		Kind       string
		Readers    []partyID
		ContractID contractID
	}{"contract-by-id", readerParties, wantContractID})
	if err != nil {
		return acsEntry{}, false, fmt.Errorf("disclosure-service: encode cache key: %w", err)
	}
	cachedValue, err := c.do(ctx, string(encodedKey), func(fetchCtx context.Context) (any, error) {
		entry, found, err := c.inner.ActiveContractByID(fetchCtx, readerParties, wantContractID)
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
