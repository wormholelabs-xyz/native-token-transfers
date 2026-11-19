package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEveryResponseHasNoStoreAndNoSniff(t *testing.T) {
	flowPath := "/v1/flows/transfer?manager=" + strings.Repeat("ab", 32)
	cases := []struct {
		name             string
		method           string
		path             string
		upstreamErr      error
		fillInboundSlots bool
		wantStatus       int
	}{
		{"index", http.MethodGet, "/v1", nil, false, http.StatusOK},
		{"healthz", http.MethodGet, "/v1/healthz", nil, false, http.StatusOK},
		{"readyz ready", http.MethodGet, "/v1/readyz", nil, false, http.StatusOK},
		{"readyz not ready", http.MethodGet, "/v1/readyz", errors.New("upstream down"), false, http.StatusServiceUnavailable},
		{"flow with a bad query", http.MethodGet, "/v1/flows/transfer?manager=zz", nil, false, http.StatusBadRequest},
		{"flow with an upstream fault", http.MethodGet, flowPath, errors.New("upstream down"), false, http.StatusBadGateway},
		{"unknown path", http.MethodGet, "/v1/nonsense", nil, false, http.StatusNotFound},
		{"method not allowed", http.MethodPost, "/v1/healthz", nil, false, http.StatusMethodNotAllowed},
		{"shed past the inbound cap", http.MethodGet, flowPath, nil, true, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServerWithClient(t, &countingACS{offset: 42, err: tc.upstreamErr})
			srv.logger, _ = newCapturedLogger()
			if tc.fillInboundSlots {
				for range cap(srv.inboundSlots) {
					srv.inboundSlots <- struct{}{}
				}
			}
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			assert.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
		})
	}
}
