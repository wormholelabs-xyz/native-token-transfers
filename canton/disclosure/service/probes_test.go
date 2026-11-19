package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingActiveContractsACS answers LedgerEnd at once and holds each ActiveContracts call until
// release closes or ctx ends.
type blockingActiveContractsACS struct {
	release                 chan struct{}
	activeContractsInFlight atomic.Int64
}

func (blocking *blockingActiveContractsACS) LedgerEnd(context.Context) (int64, error) {
	return 42, nil
}

func (blocking *blockingActiveContractsACS) ActiveContracts(ctx context.Context, _ activeContractsQuery) ([]acsEntry, error) {
	blocking.activeContractsInFlight.Add(1)
	defer blocking.activeContractsInFlight.Add(-1)
	select {
	case <-blocking.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (blocking *blockingActiveContractsACS) ActiveContractByID(context.Context, []partyID, contractID) (acsEntry, bool, error) {
	return acsEntry{}, false, nil
}

func TestProbesAnswerWhileInboundCapIsFull(t *testing.T) {
	flowPath := "/v1/flows/transfer?manager=" + strings.Repeat("ab", 32)
	upstream := &blockingActiveContractsACS{release: make(chan struct{})}
	srv := newTestServerWithClient(t, upstream)
	srv.logger, _ = newCapturedLogger()
	var heldRequests sync.WaitGroup
	for range maxConcurrentInboundRequests {
		heldRequests.Add(1)
		go func() {
			defer heldRequests.Done()
			srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, flowPath, nil))
		}()
	}
	require.Eventually(t, func() bool { return upstream.activeContractsInFlight.Load() == maxConcurrentInboundRequests }, 3*time.Second, time.Millisecond)
	t.Cleanup(func() {
		close(upstream.release)
		heldRequests.Wait()
	})

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"healthz", "/v1/healthz", http.StatusOK, `{"status":"ok"}`},
		{"readyz", "/v1/readyz", http.StatusOK, `{"status":"ready"}`},
		{"index", "/v1", http.StatusOK, ""},
		{"flow request past the cap", flowPath, http.StatusServiceUnavailable, ""},
		{"managers past the cap", "/v1/managers", http.StatusServiceUnavailable, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			startedAt := time.Now()
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Less(t, time.Since(startedAt), time.Second)
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			if tc.wantBody != "" {
				assert.JSONEq(t, tc.wantBody, recorder.Body.String())
			}
		})
	}
}

func TestReadinessProbeCachesItsResult(t *testing.T) {
	cases := []struct {
		name        string
		upstreamErr error
		wantStatus  int
	}{
		{"ready result", nil, http.StatusOK},
		{"failed result", errors.New("upstream down"), http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &countingACS{offset: 42, err: tc.upstreamErr}
			srv := newTestServerWithClient(t, upstream)
			srv.logger, _ = newCapturedLogger()
			now := time.Now()
			srv.readiness.now = func() time.Time { return now }

			for range 5 {
				recorder := httptest.NewRecorder()
				srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
				require.Equal(t, tc.wantStatus, recorder.Code)
			}
			assert.Equal(t, int64(1), upstream.ledgerCalls.Load(), "one upstream check in one result window")

			now = now.Add(readinessResultTTL)
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
			require.Equal(t, tc.wantStatus, recorder.Code)
			assert.Equal(t, int64(2), upstream.ledgerCalls.Load(), "an expired result is checked again")
		})
	}
}

func TestReadinessProbeSharesOneConcurrentCheck(t *testing.T) {
	upstream := &countingACS{offset: 42, release: make(chan struct{})}
	srv := newTestServerWithClient(t, upstream)
	const concurrentProbes = 20
	statuses := make([]int, concurrentProbes)
	var probes sync.WaitGroup
	for index := range concurrentProbes {
		probes.Add(1)
		go func() {
			defer probes.Done()
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
			statuses[index] = recorder.Code
		}()
	}
	require.Eventually(t, func() bool { return upstream.ledgerCalls.Load() == 1 }, 3*time.Second, time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	close(upstream.release)
	probes.Wait()
	assert.Equal(t, int64(1), upstream.ledgerCalls.Load())
	for _, status := range statuses {
		assert.Equal(t, http.StatusOK, status)
	}
}

func TestReadinessProbeEndsAtItsTimeout(t *testing.T) {
	upstream := &blockingLedgerEndACS{release: make(chan struct{})}
	defer close(upstream.release)
	srv := newTestServerWithClient(t, upstream)
	srv.logger, _ = newCapturedLogger()
	srv.readiness.checkTimeout = 50 * time.Millisecond

	startedAt := time.Now()
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
	assert.Less(t, time.Since(startedAt), time.Second)
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestReadinessProbeBounds(t *testing.T) {
	assert.Equal(t, 3*time.Second, readinessCheckTimeout)
	assert.Equal(t, 2*time.Second, readinessResultTTL)
	srv := newTestServerWithClient(t, &countingACS{offset: 42})
	assert.Equal(t, readinessCheckTimeout, srv.readiness.checkTimeout)
	assert.Equal(t, readinessResultTTL, srv.readiness.resultTTL)
}

func TestReadinessFailureLogsOncePerUpstreamCheck(t *testing.T) {
	cases := []struct {
		name           string
		upstreamErr    error
		wantStatus     int
		wantLogsPerTTL int
	}{
		{"ready result logs nothing", nil, http.StatusOK, 0},
		{"failed result logs one line for each check", errors.New("upstream down"), http.StatusServiceUnavailable, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &countingACS{offset: 42, err: tc.upstreamErr}
			srv := newTestServerWithClient(t, upstream)
			logged := captureServerLog(srv)
			now := time.Now()
			srv.readiness.now = func() time.Time { return now }

			for window := 1; window <= 2; window++ {
				for range 5 {
					recorder := httptest.NewRecorder()
					srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
					require.Equal(t, tc.wantStatus, recorder.Code)
					if tc.wantStatus != http.StatusOK {
						assert.NotEmpty(t, recorder.Header().Get("X-Request-Id"))
						assert.NotContains(t, recorder.Body.String(), "upstream down")
					}
				}
				assert.Equal(t, int64(window), upstream.ledgerCalls.Load())
				assert.Equal(t, window*tc.wantLogsPerTTL, strings.Count(logged.String(), "\n"), logged.String())
				now = now.Add(readinessResultTTL)
			}
			if tc.wantLogsPerTTL > 0 {
				assert.Contains(t, logged.String(), "ledger-end")
				assert.Contains(t, logged.String(), "upstream down")
			}
		})
	}
}
