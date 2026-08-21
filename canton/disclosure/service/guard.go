// Upstream guards: a TTL cache and a concurrency bound, both wrapping acsClient.
//
// One HTTP request can fan out to every allow-listed template plus a ledger-end call, so
// inbound traffic reaches the participant amplified. These two decorators bound that.
//
// Compose them cache-outermost (see run()): a cache hit answers without holding a semaphore
// slot, so cheap repeat traffic never competes with work that must go upstream.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// errUpstreamBusy reports that the concurrency bound shed this call. Handlers map it to 503.
var errUpstreamBusy = errors.New("disclosure-service: upstream busy")

// upstreamErrorStatus separates load shedding, which a caller should retry, from an upstream
// fault, which it should not.
func upstreamErrorStatus(err error) int {
	if errors.Is(err, errUpstreamBusy) {
		return http.StatusServiceUnavailable
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

func (b *boundedACS) ActiveContracts(ctx context.Context, parties []string, template string, activeAtOffset int64) ([]acsEntry, error) {
	if err := b.acquire(ctx); err != nil {
		return nil, err
	}
	defer b.release()
	return b.inner.ActiveContracts(ctx, parties, template, activeAtOffset)
}

// ---------------------------------------------------------------------------
// TTL cache with single flight
// ---------------------------------------------------------------------------

type cacheEntry struct {
	value   any
	expires time.Time
}

// inflightCall lets concurrent callers for one key share a single upstream result.
type inflightCall struct {
	wg    sync.WaitGroup
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
	inner acsClient
	ttl   time.Duration
	now   func() time.Time // injectable so tests advance time without sleeping

	mu       sync.Mutex
	entries  map[string]cacheEntry
	inflight map[string]*inflightCall
}

func newCachingACS(inner acsClient, ttl time.Duration) *cachingACS {
	return &cachingACS{
		inner:    inner,
		ttl:      ttl,
		now:      time.Now,
		entries:  make(map[string]cacheEntry),
		inflight: make(map[string]*inflightCall),
	}
}

// do returns key's cached value, joins the call already fetching it, or fetches it.
func (c *cachingACS) do(key string, fetch func() (any, error)) (any, error) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.value, nil
	}
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		call.wg.Wait()
		return call.value, call.err
	}
	call := &inflightCall{}
	call.wg.Add(1)
	c.inflight[key] = call
	c.mu.Unlock()

	call.value, call.err = fetch()
	call.wg.Done()

	c.mu.Lock()
	delete(c.inflight, key)
	if call.err == nil {
		c.sweepLocked()
		c.entries[key] = cacheEntry{value: call.value, expires: c.now().Add(c.ttl)}
	}
	c.mu.Unlock()

	return call.value, call.err
}

// sweepLocked drops expired entries once the map reaches its cap, then drops arbitrary entries
// if every one is still live. The caller holds c.mu.
func (c *cachingACS) sweepLocked() {
	if len(c.entries) < maxCacheEntries {
		return
	}
	now := c.now()
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
	for k := range c.entries {
		if len(c.entries) < maxCacheEntries {
			break
		}
		delete(c.entries, k)
	}
}

func (c *cachingACS) LedgerEnd(ctx context.Context) (int64, error) {
	v, err := c.do("ledger-end", func() (any, error) { return c.inner.LedgerEnd(ctx) })
	if err != nil {
		return 0, err
	}
	offset, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("disclosure-service: cached ledger-end has type %T", v)
	}
	return offset, nil
}

func (c *cachingACS) ActiveContracts(ctx context.Context, parties []string, template string, activeAtOffset int64) ([]acsEntry, error) {
	key := fmt.Sprintf("ac\x00%d\x00%s\x00%s", activeAtOffset, template, strings.Join(parties, ","))
	v, err := c.do(key, func() (any, error) {
		return c.inner.ActiveContracts(ctx, parties, template, activeAtOffset)
	})
	if err != nil {
		return nil, err
	}
	entries, ok := v.([]acsEntry)
	if !ok {
		return nil, fmt.Errorf("disclosure-service: cached active-contracts has type %T", v)
	}
	return entries, nil
}
