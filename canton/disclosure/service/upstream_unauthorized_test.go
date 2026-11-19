package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTokenCheckingJSONAPI answers each JSON API route with 200 when acceptToken accepts the
// bearer token, else with rejectStatus. It counts the upstream calls.
func newTokenCheckingJSONAPI(t *testing.T, acceptToken func(token string) bool, rejectStatus int) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var callCount atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		token := r.Header.Get("Authorization")
		if !acceptToken(token) {
			http.Error(w, `{"code":"rejected"}`, rejectStatus)
			return
		}
		switch r.URL.Path {
		case "/v2/state/ledger-end":
			writeJSON(w, http.StatusOK, map[string]int64{"offset": 42})
		case "/v2/state/active-contracts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		default:
			http.Error(w, `{"code":"CONTRACT_EVENTS_NOT_FOUND"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)
	return upstream, &callCount
}

// upstreamOperations calls each httpACSClient route once.
var upstreamOperations = map[string]func(ctx context.Context, client *httpACSClient) error{
	"ledger-end": func(ctx context.Context, client *httpACSClient) error {
		_, err := client.LedgerEnd(ctx)
		return err
	},
	"active-contracts": func(ctx context.Context, client *httpACSClient) error {
		_, err := client.ActiveContracts(ctx, testQuery("#ntt:Wormhole.Ntt.Manager:NttManager", 42))
		return err
	},
	"events-by-contract-id": func(ctx context.Context, client *httpACSClient) error {
		_, _, err := client.ActiveContractByID(ctx, []partyID{"nttCustody::1"}, "pot-cid")
		return err
	},
}

func TestUpstream401DiscardsToken(t *testing.T) {
	cases := []struct {
		name             string
		acceptToken      func(token string) bool
		rejectStatus     int
		wantTokenFetches int64
		wantUpstreamCall int64
		wantErr          bool
	}{
		{"401 for the first token, 200 for the next", func(token string) bool { return token != "Bearer tok-1" }, http.StatusUnauthorized, 2, 2, false},
		{"401 for each token", func(string) bool { return false }, http.StatusUnauthorized, 2, 2, true},
		{"403 keeps the token", func(string) bool { return false }, http.StatusForbidden, 1, 1, true},
		{"200", func(string) bool { return true }, http.StatusUnauthorized, 1, 1, false},
	}
	for _, tc := range cases {
		for operationName, callOperation := range upstreamOperations {
			t.Run(tc.name+" "+operationName, func(t *testing.T) {
				endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 3600))
				upstream, upstreamCalls := newTokenCheckingJSONAPI(t, tc.acceptToken, tc.rejectStatus)
				tokens := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)
				err := callOperation(context.Background(), newTestHTTPACSClient(upstream.URL, tokens))
				if tc.wantErr {
					var httpError *upstreamHTTPError
					require.ErrorAs(t, err, &httpError)
					assert.Equal(t, tc.rejectStatus, httpError.statusCode)
				} else {
					require.NoError(t, err)
				}
				assert.Equal(t, tc.wantTokenFetches, endpoint.fetchCount.Load(), "token fetches")
				assert.Equal(t, tc.wantUpstreamCall, upstreamCalls.Load(), "upstream calls")
			})
		}
	}
}

func TestUpstream401ConcurrentRequestsFetchOneNewToken(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 3600))
	var rejectedArrivals sync.WaitGroup
	rejectedArrivals.Add(2)
	allRejectedArrived := make(chan struct{})
	go func() {
		rejectedArrivals.Wait()
		close(allRejectedArrived)
	}()
	upstream, upstreamCalls := newTokenCheckingJSONAPI(t, func(token string) bool {
		if token != "Bearer tok-1" {
			return true
		}
		rejectedArrivals.Done()
		select {
		case <-allRejectedArrived:
		case <-time.After(5 * time.Second):
			t.Error("the second request did not arrive with the first token")
		}
		return false
	}, http.StatusUnauthorized)
	tokens := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)
	client := newTestHTTPACSClient(upstream.URL, tokens)
	_, err := tokens.BearerToken(context.Background())
	require.NoError(t, err)

	var requests sync.WaitGroup
	requestErrors := make([]error, 2)
	for requestIndex := range requestErrors {
		requests.Add(1)
		go func() {
			defer requests.Done()
			_, requestErrors[requestIndex] = client.LedgerEnd(context.Background())
		}()
	}
	requests.Wait()
	for _, requestError := range requestErrors {
		require.NoError(t, requestError)
	}
	assert.Equal(t, int64(2), endpoint.fetchCount.Load(), "one fetch for tok-1, one more for both rejected requests")
	assert.Equal(t, int64(4), upstreamCalls.Load())
}

func TestFileTokenSourceRetriesOnceOn401(t *testing.T) {
	upstream, upstreamCalls := newTokenCheckingJSONAPI(t, func(string) bool { return false }, http.StatusUnauthorized)
	tokenPath := t.TempDir() + "/token"
	writeClientSecret(t, tokenPath, "file-token")
	_, err := newTestHTTPACSClient(upstream.URL, fileBearerTokenSource{path: tokenPath}).LedgerEnd(context.Background())
	var httpError *upstreamHTTPError
	require.ErrorAs(t, err, &httpError)
	assert.Equal(t, http.StatusUnauthorized, httpError.statusCode)
	assert.Equal(t, int64(2), upstreamCalls.Load())
}
