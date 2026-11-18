package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// newCapturedLogger returns a logger that writes into the returned buffer.
func newCapturedLogger() (*log.Logger, *syncBuffer) {
	captured := &syncBuffer{}
	return log.New(captured, "", 0), captured
}

// captureServerLog points srv's logger at a new buffer and returns it.
func captureServerLog(srv *server) *syncBuffer {
	logger, captured := newCapturedLogger()
	srv.logger = logger
	return captured
}

// requirePublicErrorWithLoggedDetail checks the fixed public body and that only the log has wantDetail.
func requirePublicErrorWithLoggedDetail(t *testing.T, recorder *httptest.ResponseRecorder, logged *syncBuffer, wantStatus int, wantDetail string) {
	t.Helper()
	require.Equal(t, wantStatus, recorder.Code, recorder.Body.String())
	var body publicErrorBody
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
	assert.Equal(t, publicMessageForStatus(wantStatus), body.Error)
	assert.Regexp(t, requestIDPattern, body.RequestID)
	assert.NotContains(t, recorder.Body.String(), wantDetail)
	assert.Contains(t, logged.String(), body.RequestID)
	assert.Contains(t, logged.String(), wantDetail)
}

func TestPublicErrorsHideUpstreamDetail(t *testing.T) {
	const internalDetailBody = `{"cause":"internal detail","idp":"https://idp.internal/realm"}`
	tokenDirectory := t.TempDir()
	tokenPath := filepath.Join(tokenDirectory, "upstream-token")

	cases := []struct {
		name             string
		upstreamURL      func(t *testing.T) string
		tokenPath        string
		removeTokenFile  bool
		wantStatus       int
		wantLoggedDetail string
		forbiddenInBody  []string
	}{
		{
			name: "upstream 401 with an internal body",
			upstreamURL: func(t *testing.T) string {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, internalDetailBody, http.StatusUnauthorized)
				}))
				t.Cleanup(upstream.Close)
				return upstream.URL
			},
			wantStatus:       http.StatusBadGateway,
			wantLoggedDetail: "internal detail",
			forbiddenInBody:  []string{"internal detail", "idp.internal", "http", "401"},
		},
		{
			name:             "transport error to a closed port",
			upstreamURL:      func(*testing.T) string { return "http://127.0.0.1:1" },
			wantStatus:       http.StatusBadGateway,
			wantLoggedDetail: "127.0.0.1:1",
			forbiddenInBody:  []string{"127.0.0.1", "http", "connect"},
		},
		{
			name: "token file removed after start",
			upstreamURL: func(t *testing.T) string {
				upstream, _ := newFakeJSONAPI(t, func(w http.ResponseWriter, r *http.Request) {})
				return upstream.URL
			},
			tokenPath:        tokenPath,
			removeTokenFile:  true,
			wantStatus:       http.StatusBadGateway,
			wantLoggedDetail: tokenPath,
			forbiddenInBody:  []string{tokenDirectory, "upstream-token", "no such file"},
		},
		{
			name: "upstream 500 with a 70 MiB body",
			upstreamURL: func(t *testing.T) string {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					chunk := bytes.Repeat([]byte("internal detail "), 4096)
					for written := 0; written < 70<<20; written += len(chunk) {
						if _, err := w.Write(chunk); err != nil {
							return
						}
					}
				}))
				t.Cleanup(upstream.Close)
				return upstream.URL
			},
			wantStatus:       http.StatusBadGateway,
			wantLoggedDetail: "internal detail",
			forbiddenInBody:  []string{"internal detail"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.tokenPath != "" {
				require.NoError(t, os.WriteFile(tc.tokenPath, []byte("token\n"), 0o600))
			}
			upstreamURL := tc.upstreamURL(t)
			srv := newTestServerWithClient(t, newHTTPACSClient(upstreamURL, tc.tokenPath))
			logger, captured := newCapturedLogger()
			srv.logger = logger
			if tc.removeTokenFile {
				require.NoError(t, os.Remove(tc.tokenPath))
			}

			recorder := requestFlow(t, srv, "transfer", "manager="+strings.Repeat("ab", 32))

			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			var body publicErrorBody
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
			assert.Equal(t, publicMessageForStatus(tc.wantStatus), body.Error)
			assert.Regexp(t, requestIDPattern, body.RequestID)
			assert.Equal(t, body.RequestID, recorder.Header().Get("X-Request-Id"))
			for _, forbidden := range tc.forbiddenInBody {
				assert.NotContains(t, recorder.Body.String(), forbidden)
			}

			logged := captured.String()
			assert.Contains(t, logged, body.RequestID)
			assert.Contains(t, logged, tc.wantLoggedDetail)
			assert.Less(t, len(logged), 2048, "the log line holds an excerpt, not the whole upstream body")
		})
	}
}

func TestPublicMessageForStatus(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusBadGateway, "upstream ledger error"},
		{http.StatusServiceUnavailable, "service busy, retry"},
		{http.StatusGatewayTimeout, "upstream timeout"},
		{http.StatusInternalServerError, "internal error"},
		{http.StatusNotImplemented, "internal error"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, publicMessageForStatus(tc.status), "status %d", tc.status)
	}
}

func TestTruncateForError(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"short value unchanged", "abc", "abc"},
		{"exactly 32 bytes unchanged", strings.Repeat("x", 32), strings.Repeat("x", 32)},
		{"33 bytes cut to 32 plus a marker", strings.Repeat("x", 33), strings.Repeat("x", 32) + "..."},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, truncateForError(tc.input))
		})
	}
}
