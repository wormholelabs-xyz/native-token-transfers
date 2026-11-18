package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeClock struct {
	mu      sync.Mutex
	current time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{current: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
}

func (clock *fakeClock) now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.current
}

func (clock *fakeClock) advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.current = clock.current.Add(duration)
}

// fakeTokenEndpoint records every token request and answers with respond(fetchNumber).
type fakeTokenEndpoint struct {
	server      *httptest.Server
	fetchCount  atomic.Int64
	mu          sync.Mutex
	lastForm    url.Values
	lastContent string
}

func newFakeTokenEndpoint(t *testing.T, respond func(w http.ResponseWriter, fetchNumber int64)) *fakeTokenEndpoint {
	t.Helper()
	endpoint := &fakeTokenEndpoint{}
	endpoint.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("fake token endpoint: got method %s, want POST", r.Method)
		}
		require.NoError(t, r.ParseForm())
		endpoint.mu.Lock()
		endpoint.lastForm = r.PostForm
		endpoint.lastContent = r.Header.Get("Content-Type")
		endpoint.mu.Unlock()
		respond(w, endpoint.fetchCount.Add(1))
	}))
	t.Cleanup(endpoint.server.Close)
	return endpoint
}

func (endpoint *fakeTokenEndpoint) form() url.Values {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	return endpoint.lastForm
}

func bearerTokenResponse(accessToken string, expiresInSeconds int) func(http.ResponseWriter, int64) {
	return func(w http.ResponseWriter, fetchNumber int64) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":"%s-%d","token_type":"Bearer","expires_in":%d}`, accessToken, fetchNumber, expiresInSeconds)
	}
}

func writeClientSecret(t *testing.T, path, secret string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(secret+"\n"), 0o600))
}

func newTestOAuthConfig(t *testing.T, tokenURL string) oauthClientConfig {
	t.Helper()
	secretPath := filepath.Join(t.TempDir(), "client-secret")
	writeClientSecret(t, secretPath, "first-secret")
	return oauthClientConfig{
		tokenURL:         tokenURL,
		clientID:         "disclosure-gateway",
		clientSecretFile: secretPath,
		audience:         "https://canton.network.global",
		scope:            "daml_ledger_api",
	}
}

// ---------------------------------------------------------------------------
// Token source
// ---------------------------------------------------------------------------

func TestOAuthTokenSource_FirstFetchSendsClientCredentials(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 3600))
	config := newTestOAuthConfig(t, endpoint.server.URL)
	source := newOAuthBearerTokenSource(config, newTestUpstreamHTTPClient(t), newFakeClock().now)

	token, err := source.BearerToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", token)
	assert.Equal(t, int64(1), endpoint.fetchCount.Load())

	form := endpoint.form()
	assert.Equal(t, "client_credentials", form.Get("grant_type"))
	assert.Equal(t, "disclosure-gateway", form.Get("client_id"))
	assert.Equal(t, "first-secret", form.Get("client_secret"))
	assert.Equal(t, "https://canton.network.global", form.Get("audience"))
	assert.Equal(t, "daml_ledger_api", form.Get("scope"))
	assert.Equal(t, "application/x-www-form-urlencoded", endpoint.lastContent)
}

func TestOAuthTokenSource_OmitsEmptyScope(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 3600))
	config := newTestOAuthConfig(t, endpoint.server.URL)
	config.scope = ""
	source := newOAuthBearerTokenSource(config, newTestUpstreamHTTPClient(t), newFakeClock().now)

	_, err := source.BearerToken(context.Background())
	require.NoError(t, err)
	_, hasScope := endpoint.form()["scope"]
	assert.False(t, hasScope)
}

func TestOAuthTokenSource_ReusesThenRefreshesWithRotatedSecret(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 100))
	config := newTestOAuthConfig(t, endpoint.server.URL)
	clock := newFakeClock()
	source := newOAuthBearerTokenSource(config, newTestUpstreamHTTPClient(t), clock.now)

	// Lifetime 100s: margin min(60s, 10s) = 10s, so the refresh point is 90s.
	steps := []struct {
		name          string
		advanceBy     time.Duration
		rotateSecret  string
		wantToken     string
		wantFetches   int64
		wantSecretArg string
	}{
		{name: "first fetch", wantToken: "tok-1", wantFetches: 1, wantSecretArg: "first-secret"},
		{name: "reuse at 89s", advanceBy: 89 * time.Second, rotateSecret: "second-secret", wantToken: "tok-1", wantFetches: 1, wantSecretArg: "first-secret"},
		{name: "refresh at 90s with the rotated secret", advanceBy: time.Second, wantToken: "tok-2", wantFetches: 2, wantSecretArg: "second-secret"},
		{name: "reuse after refresh", advanceBy: 30 * time.Second, wantToken: "tok-2", wantFetches: 2, wantSecretArg: "second-secret"},
	}
	for _, step := range steps {
		clock.advance(step.advanceBy)
		if step.rotateSecret != "" {
			writeClientSecret(t, config.clientSecretFile, step.rotateSecret)
		}
		token, err := source.BearerToken(context.Background())
		require.NoError(t, err, step.name)
		assert.Equal(t, step.wantToken, token, step.name)
		assert.Equal(t, step.wantFetches, endpoint.fetchCount.Load(), step.name)
		assert.Equal(t, step.wantSecretArg, endpoint.form().Get("client_secret"), step.name)
	}
}

func TestOAuthRefreshMargin(t *testing.T) {
	cases := []struct {
		lifetime   time.Duration
		wantMargin time.Duration
	}{
		{lifetime: time.Second, wantMargin: 100 * time.Millisecond},
		{lifetime: 100 * time.Second, wantMargin: 10 * time.Second},
		{lifetime: 600 * time.Second, wantMargin: 60 * time.Second},
		{lifetime: time.Hour, wantMargin: 60 * time.Second},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.wantMargin, oauthRefreshMargin(tc.lifetime), tc.lifetime.String())
	}
}

func TestOAuthTokenSource_SingleFlight(t *testing.T) {
	release := make(chan struct{})
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, fetchNumber int64) {
		<-release
		bearerTokenResponse("tok", 3600)(w, fetchNumber)
	})
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)

	const callerCount = 16
	tokens := make([]string, callerCount)
	errs := make([]error, callerCount)
	var wg sync.WaitGroup
	for callerIndex := range callerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tokens[callerIndex], errs[callerIndex] = source.BearerToken(context.Background())
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	assert.Equal(t, int64(1), endpoint.fetchCount.Load())
	for callerIndex := range callerCount {
		require.NoError(t, errs[callerIndex])
		assert.Equal(t, "tok-1", tokens[callerIndex])
	}
}

func TestOAuthTokenSource_WaiterHonorsContext(t *testing.T) {
	release := make(chan struct{})
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, fetchNumber int64) {
		<-release
		bearerTokenResponse("tok", 3600)(w, fetchNumber)
	})
	t.Cleanup(func() { close(release) })
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)

	go func() { _, _ = source.BearerToken(context.Background()) }()
	require.Eventually(t, func() bool { return endpoint.fetchCount.Load() == 1 }, 3*time.Second, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := source.BearerToken(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestOAuthTokenSource_RejectsBadResponses(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantErrSub string
	}{
		{name: "endpoint 500", status: http.StatusInternalServerError, body: `{"error":"server_error"}`, wantErrSub: "HTTP 500 (server_error)"},
		{name: "endpoint 401", status: http.StatusUnauthorized, body: `{"error":"invalid_client","error_description":"leaked first-secret"}`, wantErrSub: "HTTP 401 (invalid_client)"},
		{name: "endpoint 400 with unsafe error code", status: http.StatusBadRequest, body: `{"error":"first-secret tok-leak"}`, wantErrSub: "HTTP 400"},
		{name: "malformed JSON", status: http.StatusOK, body: `{"access_token":`, wantErrSub: "parse token response"},
		{name: "missing access_token", status: http.StatusOK, body: `{"token_type":"Bearer","expires_in":60}`, wantErrSub: "access_token"},
		{name: "whitespace in access_token", status: http.StatusOK, body: `{"access_token":"a b","token_type":"Bearer","expires_in":60}`, wantErrSub: "access_token"},
		{name: "non-Bearer token type", status: http.StatusOK, body: `{"access_token":"tok","token_type":"mac","expires_in":60}`, wantErrSub: "token_type"},
		{name: "missing token type", status: http.StatusOK, body: `{"access_token":"tok","expires_in":60}`, wantErrSub: "token_type"},
		{name: "zero expires_in", status: http.StatusOK, body: `{"access_token":"tok","token_type":"Bearer","expires_in":0}`, wantErrSub: "expires_in"},
		{name: "missing expires_in", status: http.StatusOK, body: `{"access_token":"tok","token_type":"Bearer"}`, wantErrSub: "expires_in"},
		{name: "negative expires_in", status: http.StatusOK, body: `{"access_token":"tok","token_type":"Bearer","expires_in":-5}`, wantErrSub: "expires_in"},
		{name: "expires_in over the cap", status: http.StatusOK, body: `{"access_token":"tok","token_type":"Bearer","expires_in":99999999}`, wantErrSub: "expires_in"},
		{name: "string expires_in", status: http.StatusOK, body: `{"access_token":"tok","token_type":"Bearer","expires_in":"60"}`, wantErrSub: "parse token response"},
		{name: "oversize body", status: http.StatusOK, body: `{"access_token":"` + strings.Repeat("a", maxOAuthResponseBytes) + `"}`, wantErrSub: "exceeds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, _ int64) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)
			token, err := source.BearerToken(context.Background())
			require.Error(t, err)
			assert.Empty(t, token)
			assert.Contains(t, err.Error(), tc.wantErrSub)
			//SECURITY: errors reach logs and HTTP responses.
			assert.NotContains(t, err.Error(), "first-secret")
			assert.NotContains(t, err.Error(), "tok-leak")
		})
	}
}

func TestOAuthTokenSource_AcceptsCaseInsensitiveBearer(t *testing.T) {
	for _, tokenType := range []string{"Bearer", "bearer", "BEARER"} {
		endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, _ int64) {
			fmt.Fprintf(w, `{"access_token":"tok","token_type":%q,"expires_in":60}`, tokenType)
		})
		source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)
		token, err := source.BearerToken(context.Background())
		require.NoError(t, err, tokenType)
		assert.Equal(t, "tok", token, tokenType)
	}
}

func TestOAuthTokenSource_EmptySecretFileIsError(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 3600))
	config := newTestOAuthConfig(t, endpoint.server.URL)
	require.NoError(t, os.WriteFile(config.clientSecretFile, []byte("  \n"), 0o600))
	source := newOAuthBearerTokenSource(config, newTestUpstreamHTTPClient(t), newFakeClock().now)

	_, err := source.BearerToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
	assert.Equal(t, int64(0), endpoint.fetchCount.Load())
}

// ---------------------------------------------------------------------------
// End to end through the HTTP handler
// ---------------------------------------------------------------------------

// countingJSONAPI counts upstream calls and records the last Authorization header.
func countingJSONAPI(t *testing.T) (*httptest.Server, *atomic.Int64, *authCapture) {
	t.Helper()
	var callCount atomic.Int64
	capture := &authCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		capture.set(r.Header.Get("Authorization"))
		if r.URL.Path == "/v2/state/ledger-end" {
			writeJSON(w, http.StatusOK, map[string]int64{"offset": 42})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)
	return srv, &callCount, capture
}

func TestOAuth_ForwardsFetchedTokenUpstream(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 3600))
	upstream, _, capture := countingJSONAPI(t)
	httpClient := newTestUpstreamHTTPClient(t)
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), httpClient, newFakeClock().now)
	srv := newTestServerWithClient(t, newHTTPACSClient(upstream.URL, httpClient, source))

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/managers", nil))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "Bearer tok-1", capture.get())
}

func TestOAuth_EndpointFailureIs5xxAndSkipsUpstream(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, _ int64) {
		http.Error(w, `{"error":"temporarily_unavailable"}`, http.StatusServiceUnavailable)
	})
	upstream, upstreamCalls, _ := countingJSONAPI(t)
	httpClient := newTestUpstreamHTTPClient(t)
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), httpClient, newFakeClock().now)
	srv := newTestServerWithClient(t, newHTTPACSClient(upstream.URL, httpClient, source))
	logged := captureServerLog(srv)

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/managers", nil))
	requirePublicErrorWithLoggedDetail(t, rr, logged, http.StatusBadGateway, "HTTP 503")
	assert.Equal(t, int64(0), upstreamCalls.Load())
}

// ---------------------------------------------------------------------------
// Flags and startup
// ---------------------------------------------------------------------------

func TestParseFlags_UpstreamAuthModes(t *testing.T) {
	base := append([]string{"--json-api", "https://participant.example"}, testPartyFlags...)
	fullOAuth := []string{
		"--oauth-token-url", "https://idp.example/oauth/token",
		"--oauth-client-id", "gateway",
		"--oauth-client-secret-file", "secret",
		"--oauth-audience", "https://canton.network.global",
	}
	cases := []struct {
		name       string
		args       []string
		wantErrSub string
		wantOAuth  *oauthClientConfig
		wantFile   string
	}{
		{name: "no auth", args: nil},
		{name: "access token file", args: []string{"--access-token-file", "token"}, wantFile: "token"},
		{
			name:      "oauth",
			args:      fullOAuth,
			wantOAuth: &oauthClientConfig{tokenURL: "https://idp.example/oauth/token", clientID: "gateway", clientSecretFile: "secret", audience: "https://canton.network.global"},
		},
		{
			name:      "oauth with scope",
			args:      append(append([]string{}, fullOAuth...), "--oauth-scope", "daml_ledger_api"),
			wantOAuth: &oauthClientConfig{tokenURL: "https://idp.example/oauth/token", clientID: "gateway", clientSecretFile: "secret", audience: "https://canton.network.global", scope: "daml_ledger_api"},
		},
		{
			name:      "oauth over loopback http",
			args:      []string{"--oauth-token-url", "http://127.0.0.1:8080/token", "--oauth-client-id", "gateway", "--oauth-client-secret-file", "secret", "--oauth-audience", "aud"},
			wantOAuth: &oauthClientConfig{tokenURL: "http://127.0.0.1:8080/token", clientID: "gateway", clientSecretFile: "secret", audience: "aud"},
		},
		{name: "both auth modes", args: append(append([]string{}, fullOAuth...), "--access-token-file", "token"), wantErrSub: "mutually exclusive"},
		{name: "oauth missing audience", args: fullOAuth[:6], wantErrSub: "--oauth-audience"},
		{name: "oauth missing client id", args: []string{"--oauth-token-url", "https://idp.example/t", "--oauth-client-secret-file", "s", "--oauth-audience", "a"}, wantErrSub: "--oauth-client-id"},
		{name: "oauth missing secret file", args: []string{"--oauth-token-url", "https://idp.example/t", "--oauth-client-id", "c", "--oauth-audience", "a"}, wantErrSub: "--oauth-client-secret-file"},
		{name: "scope alone", args: []string{"--oauth-scope", "daml_ledger_api"}, wantErrSub: "--oauth-token-url"},
		{
			name:       "http token URL off loopback",
			args:       []string{"--oauth-token-url", "http://idp.example/token", "--oauth-client-id", "c", "--oauth-client-secret-file", "s", "--oauth-audience", "a"},
			wantErrSub: "https",
		},
		{
			name:       "token URL without host",
			args:       []string{"--oauth-token-url", "https:///token", "--oauth-client-id", "c", "--oauth-client-secret-file", "s", "--oauth-audience", "a"},
			wantErrSub: "host",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseFlags(append(append([]string{}, base...), tc.args...))
			if tc.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOAuth, opts.oauth)
			assert.Equal(t, tc.wantFile, opts.accessTokenFile)
		})
	}
}

func TestNewBearerTokenSource(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("tok\n"), 0o600))
	secretPath := filepath.Join(dir, "secret")
	writeClientSecret(t, secretPath, "first-secret")
	missingPath := filepath.Join(dir, "missing")
	oauthWith := func(secretFile string) *oauthClientConfig {
		return &oauthClientConfig{tokenURL: "https://idp.example/token", clientID: "c", clientSecretFile: secretFile, audience: "a"}
	}

	cases := []struct {
		name       string
		opts       options
		wantErrSub string
		wantMode   string
	}{
		{name: "none", opts: options{}, wantMode: "none"},
		{name: "file", opts: options{accessTokenFile: tokenPath}, wantMode: "file"},
		{name: "oauth", opts: options{oauth: oauthWith(secretPath)}, wantMode: "oauth"},
		{name: "missing token file", opts: options{accessTokenFile: missingPath}, wantErrSub: "read access token file"},
		{name: "missing secret file", opts: options{oauth: oauthWith(missingPath)}, wantErrSub: "--oauth-client-secret-file"},
		{name: "both modes", opts: options{accessTokenFile: tokenPath, oauth: oauthWith(secretPath)}, wantErrSub: "mutually exclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := newBearerTokenSource(&tc.opts, newTestUpstreamHTTPClient(t))
			if tc.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSub)
				assert.Nil(t, source)
				return
			}
			require.NoError(t, err)
			switch tc.wantMode {
			case "none":
				assert.Nil(t, source)
			case "file":
				assert.IsType(t, fileBearerTokenSource{}, source)
			case "oauth":
				assert.IsType(t, &oauthBearerTokenSource{}, source)
			}
		})
	}
}

func TestRun_RejectsBothAuthModes(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("tok\n"), 0o600))
	secretPath := filepath.Join(dir, "secret")
	writeClientSecret(t, secretPath, "first-secret")
	opts := &options{
		listen:                  "127.0.0.1:0",
		jsonAPIBaseURL:          "http://127.0.0.1:1",
		readerParties:           testReaderParties(),
		maxContractsPerTemplate: defaultMaxContractsPerTemplate,
		maxUpstreamConcurrency:  defaultMaxUpstreamConcurrency,
		accessTokenFile:         tokenPath,
		oauth:                   &oauthClientConfig{tokenURL: "https://idp.example/token", clientID: "c", clientSecretFile: secretPath, audience: "a"},
	}
	var out syncBuffer
	err := run(context.Background(), opts, &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
	assert.Empty(t, listeningURL(out.String()))
}
