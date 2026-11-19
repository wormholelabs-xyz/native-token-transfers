package main

import (
	"context"
	"encoding/json"
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

func TestCheckListenAddress(t *testing.T) {
	cases := []struct {
		name            string
		listenAddress   string
		behindEdgeProxy bool
		wantErr         bool
	}{
		{"IPv4 loopback", "127.0.0.1:7599", false, false},
		{"IPv6 loopback", "[::1]:7599", false, false},
		{"localhost name without flag", "localhost:7599", false, true},
		{"all IPv4 interfaces without flag", "0.0.0.0:7599", false, true},
		{"all interfaces, empty host, without flag", ":7599", false, true},
		{"all IPv6 interfaces without flag", "[::]:7599", false, true},
		{"private address without flag", "10.0.0.5:7599", false, true},
		{"all IPv4 interfaces with flag", "0.0.0.0:7599", true, false},
		{"private address with flag", "10.0.0.5:7599", true, false},
		{"malformed", "127.0.0.1", false, true},
		{"malformed with flag", "not an address", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkListenAddress(tc.listenAddress, tc.behindEdgeProxy)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestParseFlags_NonLoopbackListenNeedsProxyFlag(t *testing.T) {
	cases := []struct {
		name      string
		extraArgs []string
		wantErr   bool
	}{
		{"default loopback", nil, false},
		{"non-loopback without flag", []string{"--listen", "0.0.0.0:7599"}, true},
		{"non-loopback with flag", []string{"--listen", "0.0.0.0:7599", "--behind-edge-proxy"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{"--json-api", "http://127.0.0.1:6975"}, testPartyFlags...), tc.extraArgs...)
			opts, err := parseFlags(args)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "--behind-edge-proxy")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, len(tc.extraArgs) == 3, opts.behindEdgeProxy)
		})
	}
}

func TestRun_NonLoopbackBindWithoutProxyFlagStops(t *testing.T) {
	opts := &options{
		listen:                  "0.0.0.0:0",
		upstreamAuth:            upstreamAuthNone{},
		jsonAPIBaseURL:          "http://127.0.0.1:1",
		readerParties:           testReaderParties(),
		maxContractsPerTemplate: defaultMaxContractsPerTemplate,
		maxUpstreamConcurrency:  defaultMaxUpstreamConcurrency,
	}
	var stderr syncBuffer
	done := make(chan error, 1)
	go func() { done <- run(context.Background(), opts, &stderr) }()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--behind-edge-proxy")
		assert.Empty(t, listeningURL(stderr.String()))
	case <-time.After(3 * time.Second):
		t.Fatal("run() served a non-loopback bind without --behind-edge-proxy")
	}
}

// blockingLedgerEndACS holds each LedgerEnd call until release closes or ctx ends.
type blockingLedgerEndACS struct {
	release  chan struct{}
	inFlight atomic.Int64
}

func (blocking *blockingLedgerEndACS) LedgerEnd(ctx context.Context) (int64, error) {
	blocking.inFlight.Add(1)
	defer blocking.inFlight.Add(-1)
	select {
	case <-blocking.release:
		return 42, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (blocking *blockingLedgerEndACS) ActiveContracts(context.Context, activeContractsQuery) ([]acsEntry, error) {
	return nil, nil
}

func (blocking *blockingLedgerEndACS) ActiveContractByID(context.Context, []partyID, contractID) (acsEntry, bool, error) {
	return acsEntry{}, false, nil
}

func (blocking *blockingLedgerEndACS) ActiveContractAmountTotals(context.Context, amountTotalsQuery) (amountTotalsByGroup, error) {
	return amountTotalsByGroup{}, nil
}

func TestServeHTTPInboundBounds(t *testing.T) {
	flowPath := "/v1/flows/transfer?manager=" + strings.Repeat("ab", 32)

	t.Run("the request after the inbound cap gets 503 at once", func(t *testing.T) {
		upstream := &blockingLedgerEndACS{release: make(chan struct{})}
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
		require.Eventually(t, func() bool { return upstream.inFlight.Load() == maxConcurrentInboundRequests }, 3*time.Second, time.Millisecond)

		startedAt := time.Now()
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, flowPath, nil))
		assert.Less(t, time.Since(startedAt), time.Second)
		require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
		var body publicErrorBody
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		assert.Equal(t, publicMessageForStatus(http.StatusServiceUnavailable), body.Error)

		close(upstream.release)
		heldRequests.Wait()
		recorder = httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/healthz", nil))
		assert.Equal(t, http.StatusOK, recorder.Code, "slots are free again after the held requests end")
	})

	t.Run("a blocked upstream ends at the request deadline", func(t *testing.T) {
		upstream := &blockingLedgerEndACS{release: make(chan struct{})}
		srv := newTestServerWithClient(t, upstream)
		srv.logger, _ = newCapturedLogger()
		srv.inboundRequestDeadline = 50 * time.Millisecond
		startedAt := time.Now()
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, flowPath, nil))
		assert.Less(t, time.Since(startedAt), time.Second)
		assert.Equal(t, http.StatusGatewayTimeout, recorder.Code, recorder.Body.String())
		close(upstream.release)
	})

	t.Run("bound values", func(t *testing.T) {
		assert.Equal(t, 64, maxConcurrentInboundRequests)
		assert.Equal(t, 60*time.Second, inboundRequestDeadline)
		srv := newTestServerWithClient(t, &blockingLedgerEndACS{})
		assert.Equal(t, inboundRequestDeadline, srv.inboundRequestDeadline)
		assert.Equal(t, maxConcurrentInboundRequests, cap(srv.inboundSlots))
	})
}

func TestNewHTTPServerBounds(t *testing.T) {
	httpServer := newHTTPServer(http.NotFoundHandler())
	assert.Equal(t, inboundRequestDeadline+5*time.Second, httpServer.WriteTimeout)
	assert.Equal(t, 8<<10, httpServer.MaxHeaderBytes)
	assert.Equal(t, serverReadHeaderTimeout, httpServer.ReadHeaderTimeout)
}
