package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBrowserOrigin = "https://dapp.example"

// requireCORSResponseHeaders checks the CORS headers of a GET-route response.
func requireCORSResponseHeaders(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "X-Request-Id", recorder.Header().Get("Access-Control-Expose-Headers"))
	assert.Empty(t, recorder.Header().Values("Access-Control-Allow-Credentials"), "no credentials")
}

func serveWithOrigin(srv *server, method, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Origin", testBrowserOrigin)
	srv.ServeHTTP(recorder, request)
	return recorder
}

// corsTransferFixtures is one BurnMint deployment: /v1/managers lists it, and
// /v1/flows/transfer?manager=<b.managerAddr> answers 200.
func corsTransferFixtures(b baseFixtures) flowFixtures {
	return flowFixtures{
		tailNttManager:  jsonArray(b.managerJSON()),
		tailEmitter:     jsonArray(b.emitterJSON()),
		tailCoreState:   jsonArray(b.coreStateJSON()),
		tailCoinFactory: jsonArray(b.factoryJSON()),
	}
}

func TestCORS_SuccessfulGETResponses(t *testing.T) {
	b := newBaseFixtures(tokenConfigBurnMint)
	transferPath := "/v1/flows/transfer?manager=" + b.managerAddr
	srv := newFlowTestServer(t, corsTransferFixtures(b))
	for _, target := range []string{"/v1", "/v1/healthz", "/v1/readyz", "/v1/managers", transferPath} {
		t.Run(target, func(t *testing.T) {
			recorder := serveWithOrigin(srv, http.MethodGet, target)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			requireCORSResponseHeaders(t, recorder)
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
		})
	}
	t.Run("without an Origin header", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, transferPath, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		requireCORSResponseHeaders(t, recorder)
	})
}

func TestCORS_ErrorResponses(t *testing.T) {
	flowPath := "/v1/flows/transfer?manager=" + strings.Repeat("ab", 32)
	cases := []struct {
		name             string
		method           string
		path             string
		upstreamErr      error
		fillInboundSlots bool
		wantStatus       int
	}{
		{"flow with a bad query", http.MethodGet, "/v1/flows/transfer?manager=zz", nil, false, http.StatusBadRequest},
		{"unknown flow", http.MethodGet, "/v1/flows/nope", nil, false, http.StatusNotFound},
		{"flow with an unknown parameter", http.MethodGet, flowPath + "&gg=x", nil, false, http.StatusBadRequest},
		{"query over the limit", http.MethodGet, "/v1/flows/transfer?x=" + strings.Repeat("a", maxFlowQueryBytes), nil, false, http.StatusRequestURITooLong},
		{"flow with an upstream fault", http.MethodGet, flowPath, errors.New("upstream down"), false, http.StatusBadGateway},
		{"managers with an upstream fault", http.MethodGet, "/v1/managers", errors.New("upstream down"), false, http.StatusBadGateway},
		{"readyz not ready", http.MethodGet, "/v1/readyz", errors.New("upstream down"), false, http.StatusServiceUnavailable},
		{"shed past the inbound cap", http.MethodGet, flowPath, nil, true, http.StatusServiceUnavailable},
		{"method not allowed", http.MethodPost, "/v1/healthz", nil, false, http.StatusMethodNotAllowed},
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
			recorder := serveWithOrigin(srv, tc.method, tc.path)
			assert.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			requireCORSResponseHeaders(t, recorder)
		})
	}
	t.Run("flow with a missing contract", func(t *testing.T) {
		b := newBaseFixtures(tokenConfigBurnMint)
		fixtures := corsTransferFixtures(b)
		delete(fixtures, tailCoreState)
		srv := newFlowTestServer(t, fixtures)
		captureServerLog(srv)
		recorder := serveWithOrigin(srv, http.MethodGet, "/v1/flows/transfer?manager="+b.managerAddr)
		assert.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
		requireCORSResponseHeaders(t, recorder)
	})
}

func TestCORS_Preflight(t *testing.T) {
	for _, target := range []string{"/v1", "/v1/healthz", "/v1/readyz", "/v1/managers", "/v1/flows/transfer", "/v1/flows/release?manager=x"} {
		t.Run(target, func(t *testing.T) {
			acs := &countingACS{offset: 42}
			srv := newTestServerWithClient(t, acs)
			// A preflight has a constant cost and runs outside the inbound cap.
			for range cap(srv.inboundSlots) {
				srv.inboundSlots <- struct{}{}
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodOptions, target, nil)
			request.Header.Set("Origin", testBrowserOrigin)
			request.Header.Set("Access-Control-Request-Method", http.MethodGet)
			request.Header.Set("Access-Control-Request-Headers", "content-type")
			srv.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
			assert.Empty(t, recorder.Body.String())
			requireCORSResponseHeaders(t, recorder)
			assert.Equal(t, "GET", recorder.Header().Get("Access-Control-Allow-Methods"))
			assert.Equal(t, "Accept, Content-Type", recorder.Header().Get("Access-Control-Allow-Headers"))
			assert.Equal(t, "600", recorder.Header().Get("Access-Control-Max-Age"))
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			assert.Zero(t, acs.ledgerCalls.Load()+acs.acCalls.Load()+acs.byIDCalls.Load()+acs.totalsCalls.Load(), "a preflight reaches no upstream")
		})
	}
}

// TestCORS_KeepsTheMethodChecks pins that CORS opens OPTIONS only: each other method other than
// GET still gets 405 on the /v1 routes.
func TestCORS_KeepsTheMethodChecks(t *testing.T) {
	srv := newTestServerWithClient(t, &countingACS{offset: 42})
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead} {
		for _, target := range []string{"/v1", "/v1/healthz", "/v1/readyz", "/v1/managers", "/v1/flows/transfer"} {
			recorder := serveWithOrigin(srv, method, target)
			assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code, "%s %s", method, target)
		}
	}
}

func TestCORS_OnlyOnThePublicGETRoutes(t *testing.T) {
	srv := newRegistryTestServer(t, registryTestFixtures())
	t.Run("registry metadata GET", func(t *testing.T) {
		recorder := serveRegistry(t, srv, http.MethodGet, pathRegistryInfo, "")
		requireStatus(t, recorder, http.StatusOK)
		requireCORSResponseHeaders(t, recorder)
	})
	t.Run("registry metadata preflight", func(t *testing.T) {
		recorder := serveRegistry(t, srv, http.MethodOptions, pathInstruments, "")
		requireStatus(t, recorder, http.StatusNoContent)
		requireCORSResponseHeaders(t, recorder)
	})
	t.Run("registry POST route", func(t *testing.T) {
		recorder := serveRegistry(t, srv, http.MethodPost, pathTransferFactory, `{}`)
		requireStatus(t, recorder, http.StatusBadRequest)
		assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
	})
	t.Run("registry POST route preflight", func(t *testing.T) {
		recorder := serveRegistry(t, srv, http.MethodOptions, pathTransferFactory, "")
		requireStatus(t, recorder, http.StatusNotFound)
		assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
	})
	for _, target := range []string{"/v1/nonsense", "/v1/flows/", "/other"} {
		t.Run("unknown path "+target, func(t *testing.T) {
			preflight := serveWithOrigin(srv, http.MethodOptions, target)
			assert.Equal(t, http.StatusNotFound, preflight.Code)
			assert.Empty(t, preflight.Header().Get("Access-Control-Allow-Origin"))
			assert.Empty(t, preflight.Header().Get("Access-Control-Allow-Methods"))
		})
	}
}
