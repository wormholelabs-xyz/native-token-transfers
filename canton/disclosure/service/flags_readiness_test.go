package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlagsNumericBounds(t *testing.T) {
	cases := []struct {
		name    string
		flag    string
		value   string
		wantErr bool
	}{
		{"upstream concurrency below minimum", "--max-upstream-concurrency", "0", true},
		{"upstream concurrency negative", "--max-upstream-concurrency", "-1", true},
		{"upstream concurrency minimum", "--max-upstream-concurrency", "1", false},
		{"upstream concurrency maximum", "--max-upstream-concurrency", "64", false},
		{"upstream concurrency above maximum", "--max-upstream-concurrency", "65", true},
		{"cache ttl negative", "--cache-ttl", "-1s", true},
		{"cache ttl zero turns the cache off", "--cache-ttl", "0s", false},
		{"cache ttl maximum", "--cache-ttl", "60s", false},
		{"cache ttl above maximum", "--cache-ttl", "61s", true},
		{"contracts per template below minimum", "--max-contracts-per-template", "0", true},
		{"contracts per template minimum", "--max-contracts-per-template", "1", false},
		{"contracts per template maximum", "--max-contracts-per-template", "10000", false},
		{"contracts per template above maximum", "--max-contracts-per-template", "10001", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--json-api", "http://127.0.0.1:6975", tc.flag, tc.value}, testPartyFlags...)
			_, err := parseFlags(args)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.flag)
				return
			}
			assert.NoError(t, err)
		})
	}
	assert.Equal(t, 64, maxUpstreamConcurrencyLimit)
	assert.Equal(t, 60*time.Second, maxCacheTTL)
	assert.Equal(t, 10_000, maxContractsPerTemplateLimit)
}

func TestHandleReadyz(t *testing.T) {
	cases := []struct {
		name        string
		upstreamErr error
		wantStatus  int
	}{
		{"upstream ok", nil, http.StatusOK},
		{"upstream fault", errors.New("upstream 500: internal detail"), http.StatusServiceUnavailable},
		{"upstream busy", errUpstreamBusy, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &countingACS{offset: 42, err: tc.upstreamErr}
			srv := newTestServerWithClient(t, upstream)
			logged := captureServerLog(srv)
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
			if tc.wantStatus != http.StatusOK {
				requirePublicErrorWithLoggedDetail(t, recorder, logged, tc.wantStatus, "ledger-end")
				return
			}
			require.Equal(t, http.StatusOK, recorder.Code)
			assert.JSONEq(t, `{"status":"ready"}`, recorder.Body.String())
			assert.NotContains(t, recorder.Body.String(), "42", "a public response carries no ledger offset")
		})
	}
}

func TestHandleHealthzIsLivenessOnly(t *testing.T) {
	upstream := &countingACS{offset: 42, err: errors.New("upstream down")}
	srv := newTestServerWithClient(t, upstream)
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/healthz", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Zero(t, upstream.ledgerCalls.Load(), "healthz reads no upstream")
	assert.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}
