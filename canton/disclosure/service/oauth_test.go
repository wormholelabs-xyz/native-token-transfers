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

func newTestOAuthConfig(t *testing.T, tokenURL string) validatedOAuthClientConfig {
	t.Helper()
	secretPath := filepath.Join(t.TempDir(), "client-secret")
	writeClientSecret(t, secretPath, "first-secret")
	return mustParseOAuthClientConfig(t, oauthClientFlags{
		tokenURL:         tokenURL,
		clientID:         "disclosure-gateway",
		clientSecretFile: secretPath,
		audience:         "https://canton.network.global",
		scope:            "daml_ledger_api",
	})
}

func mustParseOAuthClientConfig(t *testing.T, flags oauthClientFlags) validatedOAuthClientConfig {
	t.Helper()
	clientConfig, err := parseOAuthClientConfig(flags)
	require.NoError(t, err)
	return clientConfig
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
		{name: "fractional expires_in", status: http.StatusOK, body: `{"access_token":"tok","token_type":"Bearer","expires_in":98765432123.5}`, wantErrSub: "parse token response"},
		{name: "expires_in over int64", status: http.StatusOK, body: `{"access_token":"tok","token_type":"Bearer","expires_in":98765432123000000000000}`, wantErrSub: "parse token response"},
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
			assert.NotContains(t, err.Error(), "98765432123")
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
		wantMode   upstreamAuthMode
	}{
		{name: "no auth", args: nil, wantMode: upstreamAuthNone{}},
		{name: "access token file", args: []string{"--access-token-file", "token"}, wantMode: upstreamAuthTokenFile{tokenFilePath: "token"}},
		{
			name:     "oauth",
			args:     fullOAuth,
			wantMode: upstreamAuthOAuth2{clientConfig: mustParseOAuthClientConfig(t, oauthClientFlags{tokenURL: "https://idp.example/oauth/token", clientID: "gateway", clientSecretFile: "secret", audience: "https://canton.network.global"})},
		},
		{
			name:     "oauth with scope",
			args:     append(append([]string{}, fullOAuth...), "--oauth-scope", "daml_ledger_api"),
			wantMode: upstreamAuthOAuth2{clientConfig: mustParseOAuthClientConfig(t, oauthClientFlags{tokenURL: "https://idp.example/oauth/token", clientID: "gateway", clientSecretFile: "secret", audience: "https://canton.network.global", scope: "daml_ledger_api"})},
		},
		{
			name:     "oauth over loopback http",
			args:     []string{"--oauth-token-url", "http://127.0.0.1:8080/token", "--oauth-client-id", "gateway", "--oauth-client-secret-file", "secret", "--oauth-audience", "aud"},
			wantMode: upstreamAuthOAuth2{clientConfig: mustParseOAuthClientConfig(t, oauthClientFlags{tokenURL: "http://127.0.0.1:8080/token", clientID: "gateway", clientSecretFile: "secret", audience: "aud"})},
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
			assert.Equal(t, tc.wantMode, opts.upstreamAuth)
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
	endpoint := newFakeTokenEndpoint(t, bearerTokenResponse("tok", 3600))
	oauthWith := func(secretFile string) upstreamAuthOAuth2 {
		return upstreamAuthOAuth2{clientConfig: mustParseOAuthClientConfig(t, oauthClientFlags{tokenURL: endpoint.server.URL + "/token", clientID: "c", clientSecretFile: secretFile, audience: "a"})}
	}

	cases := []struct {
		name       string
		mode       upstreamAuthMode
		wantErrSub string
		wantMode   string
	}{
		{name: "none", mode: upstreamAuthNone{}, wantMode: "none"},
		{name: "file", mode: upstreamAuthTokenFile{tokenFilePath: tokenPath}, wantMode: "file"},
		{name: "oauth", mode: oauthWith(secretPath), wantMode: "oauth"},
		{name: "missing token file", mode: upstreamAuthTokenFile{tokenFilePath: missingPath}, wantErrSub: "read access token file"},
		{name: "missing secret file", mode: oauthWith(missingPath), wantErrSub: "--oauth-client-secret-file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := newBearerTokenSource(tc.mode)
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

func TestParseOAuthClientConfigTokenURL(t *testing.T) {
	cases := []struct {
		tokenURL   string
		wantErrSub string
	}{
		{"https://idp.example/token", ""},
		{"http://127.0.0.1:8080/token", ""},
		{"http://[::1]:8080/token", ""},
		{"http://localhost:8080/token", "https"},
		{"http://idp.example/token", "https"},
		{"https://user:password@idp.example/token", "user information"},
		{"https://idp.example/token?realm=internal", "query"},
		{"https://idp.example/token?", "query"},
		{"https://idp.example/token#fragment", "fragment"},
		{"ftp://idp.example/token", "https"},
		{"https:///token", "host"},
	}
	t.Run("--oauth-ca-file needs https", func(t *testing.T) {
		_, err := parseOAuthClientConfig(oauthClientFlags{tokenURL: "http://127.0.0.1:8080/token", clientID: "c", clientSecretFile: "s", audience: "a", caFile: "ca.pem"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--oauth-ca-file")
	})
	for _, tc := range cases {
		t.Run(tc.tokenURL, func(t *testing.T) {
			_, err := parseOAuthClientConfig(oauthClientFlags{tokenURL: tc.tokenURL, clientID: "c", clientSecretFile: "s", audience: "a"})
			if tc.wantErrSub == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrSub)
		})
	}
}

// scriptedTokenEndpoint answers fetch n with statusByFetch[n-1], or 200 after the script ends.
func scriptedTokenEndpoint(statusByFetch ...int) func(http.ResponseWriter, int64) {
	return func(w http.ResponseWriter, fetchNumber int64) {
		if fetchNumber <= int64(len(statusByFetch)) && statusByFetch[fetchNumber-1] != http.StatusOK {
			http.Error(w, `{"error":"slow_down"}`, statusByFetch[fetchNumber-1])
			return
		}
		bearerTokenResponse("tok", 3600)(w, fetchNumber)
	}
}

func TestOAuthTokenBackoff(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, scriptedTokenEndpoint(http.StatusTooManyRequests, http.StatusTooManyRequests))
	clock := newFakeClock()
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), clock.now)
	steps := []struct {
		name           string
		advance        time.Duration
		wantFetchCount int64
		wantErr        error
		wantAnyErr     bool
	}{
		{"t0: 429", 0, 1, nil, true},
		{"t0+0.5s: inside the 1 s backoff", 500 * time.Millisecond, 1, errOAuthTokenEndpointBackoff, true},
		{"t0+1s: 429 again", 500 * time.Millisecond, 2, nil, true},
		{"t0+2.9s: inside the 2 s backoff", 1900 * time.Millisecond, 2, errOAuthTokenEndpointBackoff, true},
		{"t0+3s: 200", 100 * time.Millisecond, 3, nil, false},
	}
	for _, step := range steps {
		clock.advance(step.advance)
		_, err := source.BearerToken(context.Background())
		assert.Equal(t, step.wantFetchCount, endpoint.fetchCount.Load(), step.name)
		if !step.wantAnyErr {
			require.NoError(t, err, step.name)
			continue
		}
		require.Error(t, err, step.name)
		if step.wantErr != nil {
			assert.ErrorIs(t, err, step.wantErr, step.name)
		} else {
			assert.NotErrorIs(t, err, errOAuthTokenEndpointBackoff, step.name)
		}
	}
	for range 100 {
		_, err := source.BearerToken(context.Background())
		require.NoError(t, err)
	}
	assert.Equal(t, int64(3), endpoint.fetchCount.Load(), "a success resets the backoff and caches the token")
}

func TestOAuthTokenBackoffCap(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, _ int64) {
		http.Error(w, `{"error":"temporarily_unavailable"}`, http.StatusServiceUnavailable)
	})
	clock := newFakeClock()
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), clock.now)
	wantBackoffs := []time.Duration{1, 2, 4, 8, 16, 30, 30, 30, 30, 30}
	for failureIndex, wantBackoffSeconds := range wantBackoffs {
		_, err := source.BearerToken(context.Background())
		require.Error(t, err)
		require.Equal(t, int64(failureIndex+1), endpoint.fetchCount.Load())
		wantBackoff := wantBackoffSeconds * time.Second
		clock.advance(wantBackoff - time.Millisecond)
		_, err = source.BearerToken(context.Background())
		require.ErrorIs(t, err, errOAuthTokenEndpointBackoff, "failure %d", failureIndex+1)
		require.Equal(t, int64(failureIndex+1), endpoint.fetchCount.Load())
		clock.advance(time.Millisecond)
	}
	assert.Equal(t, 30*time.Second, oauthFailureBackoffMax)
}

func TestOAuthTokenFetchTimeout(t *testing.T) {
	releaseEndpoint := make(chan struct{})
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, fetchNumber int64) {
		select {
		case <-releaseEndpoint:
		case <-time.After(5 * time.Second):
		}
		bearerTokenResponse("tok", 3600)(w, fetchNumber)
	})
	// Registered after the endpoint, so it runs before the endpoint's Close waits for the handler.
	t.Cleanup(func() { close(releaseEndpoint) })
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)
	source.tokenFetchTimeout = 50 * time.Millisecond
	startedAt := time.Now()
	_, err := source.BearerToken(context.Background())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(startedAt), time.Second)
	assert.Equal(t, 10*time.Second, oauthTokenFetchTimeout)
}

func TestTokenEndpointClientTimeout(t *testing.T) {
	client, err := newTokenEndpointHTTPClient("")
	require.NoError(t, err)
	assert.Equal(t, oauthTokenFetchTimeout, client.Timeout)
}

func TestFlowRequestEndsAtDeadlineWhileTokenEndpointHangs(t *testing.T) {
	releaseEndpoint := make(chan struct{})
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, fetchNumber int64) {
		select {
		case <-releaseEndpoint:
		case <-time.After(5 * time.Second):
		}
		bearerTokenResponse("tok", 3600)(w, fetchNumber)
	})
	// Registered after the endpoint, so it runs before the endpoint's Close waits for the handler.
	t.Cleanup(func() { close(releaseEndpoint) })
	upstream, upstreamCalls, _ := countingJSONAPI(t)
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)
	srv := newTestServerWithClient(t, newHTTPACSClient(upstream.URL, newTestUpstreamHTTPClient(t), source))
	srv.logger, _ = newCapturedLogger()
	srv.inboundRequestDeadline = 50 * time.Millisecond
	startedAt := time.Now()
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/managers", nil))
	assert.Less(t, time.Since(startedAt), time.Second)
	assert.Equal(t, http.StatusGatewayTimeout, recorder.Code, recorder.Body.String())
	assert.Zero(t, upstreamCalls.Load())
}

func TestRunFailsOnStartupTokenError(t *testing.T) {
	cases := []struct {
		name        string
		respond     func(http.ResponseWriter, int64)
		wantServing bool
	}{
		{"token endpoint 401", func(w http.ResponseWriter, _ int64) {
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
		}, false},
		{"token endpoint 200", bearerTokenResponse("tok", 3600), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := newFakeTokenEndpoint(t, tc.respond)
			upstream, _, _ := countingJSONAPI(t)
			opts := &options{
				listen:                  "127.0.0.1:0",
				upstreamAuth:            upstreamAuthOAuth2{clientConfig: newTestOAuthConfig(t, endpoint.server.URL)},
				jsonAPIBaseURL:          upstream.URL,
				readerParties:           testReaderParties(),
				maxContractsPerTemplate: defaultMaxContractsPerTemplate,
				maxUpstreamConcurrency:  defaultMaxUpstreamConcurrency,
			}
			var out syncBuffer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- run(ctx, opts, &out) }()
			if !tc.wantServing {
				select {
				case err := <-done:
					require.Error(t, err)
					assert.Contains(t, err.Error(), "startup OAuth2 token")
					assert.Contains(t, err.Error(), "invalid_client")
				case <-time.After(3 * time.Second):
					t.Fatal("run did not stop on the startup token error")
				}
				assert.Empty(t, listeningURL(out.String()))
				assert.Equal(t, int64(1), endpoint.fetchCount.Load())
				return
			}
			require.Eventually(t, func() bool { return listeningURL(out.String()) != "" }, 3*time.Second, 5*time.Millisecond)
			assert.Equal(t, int64(1), endpoint.fetchCount.Load(), "one token at startup")
			cancel()
			require.NoError(t, <-done)
		})
	}
}

// A caller that ends its wait does not end the fetch; only the fetch result sets the backoff.
func TestOAuthCanceledCallerLeavesBackoffToFetchResult(t *testing.T) {
	cases := []struct {
		name             string
		endpointStatus   int
		wantSecondErr    error
		wantSecondToken  string
		wantTokenFetches int64
	}{
		{name: "slow success", endpointStatus: http.StatusOK, wantSecondToken: "tok-1", wantTokenFetches: 1},
		{name: "slow failure", endpointStatus: http.StatusServiceUnavailable, wantSecondErr: errOAuthTokenEndpointBackoff, wantTokenFetches: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			releaseEndpoint := make(chan struct{})
			endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, fetchNumber int64) {
				select {
				case <-releaseEndpoint:
				case <-time.After(5 * time.Second):
				}
				scriptedTokenEndpoint(tc.endpointStatus)(w, fetchNumber)
			})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(releaseEndpoint) }) }
			t.Cleanup(release)
			source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), newFakeClock().now)

			canceledCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			startedAt := time.Now()
			_, err := source.BearerToken(canceledCtx)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			assert.Less(t, time.Since(startedAt), time.Second)

			release()
			token, err := source.BearerToken(context.Background())
			if tc.wantSecondErr != nil {
				require.ErrorIs(t, err, tc.wantSecondErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantSecondToken, token)
			assert.Equal(t, tc.wantTokenFetches, endpoint.fetchCount.Load())
		})
	}
}

// The backoff runs from the failure time: a slow failed fetch still gets the full wait.
func TestOAuthBackoffStartsAtFailureTime(t *testing.T) {
	clock := newFakeClock()
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, _ int64) {
		clock.advance(10 * time.Second)
		http.Error(w, `{"error":"temporarily_unavailable"}`, http.StatusServiceUnavailable)
	})
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), clock.now)
	steps := []struct {
		name             string
		advanceBy        time.Duration
		wantBackoffErr   bool
		wantTokenFetches int64
	}{
		{"t0: fetch fails at t0+10s", 0, false, 1},
		{"t0+10s: inside the 1 s backoff", 0, true, 1},
		{"t0+10.999s: inside the 1 s backoff", 999 * time.Millisecond, true, 1},
		{"t0+11s: fetch again", time.Millisecond, false, 2},
	}
	for _, step := range steps {
		clock.advance(step.advanceBy)
		_, err := source.BearerToken(context.Background())
		require.Error(t, err, step.name)
		if step.wantBackoffErr {
			assert.ErrorIs(t, err, errOAuthTokenEndpointBackoff, step.name)
		} else {
			assert.NotErrorIs(t, err, errOAuthTokenEndpointBackoff, step.name)
		}
		assert.Equal(t, step.wantTokenFetches, endpoint.fetchCount.Load(), step.name)
	}
}

// Lifetime 100 s: refresh at 90 s; after a failed refresh the token stays in use until 95 s.
func TestOAuthFailedRefreshKeepsValidCachedToken(t *testing.T) {
	endpoint := newFakeTokenEndpoint(t, func(w http.ResponseWriter, fetchNumber int64) {
		if fetchNumber == 1 {
			bearerTokenResponse("tok", 100)(w, fetchNumber)
			return
		}
		http.Error(w, `{"error":"temporarily_unavailable"}`, http.StatusServiceUnavailable)
	})
	clock := newFakeClock()
	source := newOAuthBearerTokenSource(newTestOAuthConfig(t, endpoint.server.URL), newTestUpstreamHTTPClient(t), clock.now)
	steps := []struct {
		name             string
		advanceBy        time.Duration
		wantToken        string
		wantErr          error
		wantAnyErr       bool
		wantTokenFetches int64
	}{
		{name: "t0: first fetch", wantToken: "tok-1", wantTokenFetches: 1},
		{name: "t90: refresh fails, cached token", advanceBy: 90 * time.Second, wantToken: "tok-1", wantTokenFetches: 2},
		{name: "t90.5: inside the 1 s backoff, cached token", advanceBy: 500 * time.Millisecond, wantToken: "tok-1", wantTokenFetches: 2},
		{name: "t91: refresh fails, cached token", advanceBy: 500 * time.Millisecond, wantToken: "tok-1", wantTokenFetches: 3},
		{name: "t93: refresh fails, cached token", advanceBy: 2 * time.Second, wantToken: "tok-1", wantTokenFetches: 4},
		{name: "t95: inside the 4 s backoff, cached token too old", advanceBy: 2 * time.Second, wantErr: errOAuthTokenEndpointBackoff, wantAnyErr: true, wantTokenFetches: 4},
		{name: "t97: refresh fails, no usable token", advanceBy: 2 * time.Second, wantAnyErr: true, wantTokenFetches: 5},
	}
	for _, step := range steps {
		clock.advance(step.advanceBy)
		token, err := source.BearerToken(context.Background())
		assert.Equal(t, step.wantTokenFetches, endpoint.fetchCount.Load(), step.name)
		assert.Equal(t, step.wantToken, token, step.name)
		if !step.wantAnyErr {
			require.NoError(t, err, step.name)
			continue
		}
		require.Error(t, err, step.name)
		if step.wantErr != nil {
			assert.ErrorIs(t, err, step.wantErr, step.name)
		} else {
			assert.NotErrorIs(t, err, errOAuthTokenEndpointBackoff, step.name)
		}
	}
}
