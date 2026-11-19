package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckJSONAPIURL(t *testing.T) {
	cases := []struct {
		rawURL           string
		wantErrWithToken bool
		wantErrNoToken   bool
	}{
		{"https://participant:7575", false, false},
		{"https://participant:7575/", false, false},
		{"http://participant:7575", true, false},
		{"http://127.0.0.1:7575", false, false},
		{"http://127.0.0.2:7575", false, false},
		{"http://[::1]:7575", false, false},
		{"http://localhost:7575", true, false},
		{"http://10.0.0.5:7575", true, false},
		{"https://user:pw@participant:7575", true, true},
		{"https://participant:7575/?a=1", true, true},
		{"https://participant:7575/#f", true, true},
		{"ftp://participant:7575", true, true},
		{"https://", true, true},
		{"participant:7575", true, true},
		{"", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.rawURL, func(t *testing.T) {
			errWithToken := checkJSONAPIURL(tc.rawURL, true, upstreamTLSFiles{})
			errNoToken := checkJSONAPIURL(tc.rawURL, false, upstreamTLSFiles{})
			assert.Equal(t, tc.wantErrWithToken, errWithToken != nil, "with token: %v", errWithToken)
			assert.Equal(t, tc.wantErrNoToken, errNoToken != nil, "without token: %v", errNoToken)
		})
	}
}

func TestParseFlagsRejectsTokenOverCleartextToRemoteHost(t *testing.T) {
	args := append([]string{"--json-api", "http://participant.internal:7575", "--access-token-file", "/run/secrets/token"}, testPartyFlags...)
	_, err := parseFlags(args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestNewHTTPACSClientAssertsURLRule(t *testing.T) {
	assert.Panics(t, func() {
		newTestHTTPACSClient("http://10.0.0.5:7575", fileBearerTokenSource{path: "/run/secrets/token"})
	})
	assert.NotPanics(t, func() {
		newTestHTTPACSClient("http://127.0.0.1:7575", fileBearerTokenSource{path: "/run/secrets/token"})
	})
	assert.NotPanics(t, func() { newTestHTTPACSClient("http://10.0.0.5:7575", nil) })
}

func TestIsLoopbackIPAddress(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.255.255.254", true},
		{"::1", true},
		{"localhost", false},
		{"0.0.0.0", false},
		{"::", false},
		{"10.0.0.5", false},
		{"", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, isLoopbackIPAddress(tc.host), tc.host)
	}
}

func TestDirectUpstreamHTTPClient(t *testing.T) {
	client := newUpstreamHTTPClient(&tls.Config{MinVersion: tls.VersionTLS12})
	transport, isTransport := client.Transport.(*http.Transport)
	require.True(t, isTransport)
	assert.Nil(t, transport.Proxy, "the upstream client ignores HTTP_PROXY and HTTPS_PROXY")
	assert.Equal(t, httpClientTimeout, client.Timeout)

	var redirectTargetHits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			redirectTargetHits.Add(1)
			return
		}
		http.Redirect(w, r, "/target", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(upstream.Close)
	resp, err := client.Get(upstream.URL + "/v2/state/ledger-end")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	assert.Zero(t, redirectTargetHits.Load())

	assert.NotNil(t, newTestHTTPACSClient("http://127.0.0.1:1", nil).client.CheckRedirect, "the ACS client uses the direct client")
}

func TestReadAccessTokenSize(t *testing.T) {
	directory := t.TempDir()
	cases := []struct {
		name      string
		contents  string
		wantToken string
		wantErr   bool
	}{
		{"16 KiB exactly", strings.Repeat("t", maxAccessTokenFileBytes), strings.Repeat("t", maxAccessTokenFileBytes), false},
		{"16 KiB plus 1", strings.Repeat("t", maxAccessTokenFileBytes+1), "", true},
		{"empty after trim", " \n\t", "", true},
		{"trailing newline trimmed", "secret-token\n", "secret-token", false},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenPath := filepath.Join(directory, "token-"+string(rune('a'+index)))
			require.NoError(t, os.WriteFile(tokenPath, []byte(tc.contents), 0o600))
			token, err := readAccessToken(tokenPath)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantToken, token)
		})
	}
}

func TestReadAtMost(t *testing.T) {
	cases := []struct {
		name     string
		contents string
		maxBytes int64
		wantErr  bool
	}{
		{"under the limit", "abc", 4, false},
		{"at the limit", "abcd", 4, false},
		{"over the limit", "abcde", 4, true},
		{"empty", "", 4, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readAtMost(strings.NewReader(tc.contents), tc.maxBytes)
			if tc.wantErr {
				assert.ErrorIs(t, err, errReadLimitExceeded)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.contents, string(got))
		})
	}
}

func TestParseFlagsJSONAPIURLRuleForEachAuthMode(t *testing.T) {
	authModeFlags := map[string][]string{
		"none":       nil,
		"token file": {"--access-token-file", "/run/secrets/token"},
		"OAuth2": {
			"--oauth-token-url", "https://idp.example/token",
			"--oauth-client-id", "gateway",
			"--oauth-client-secret-file", "/run/secrets/client-secret",
			"--oauth-audience", "https://ledger_api.example",
		},
	}
	cases := []struct {
		jsonAPIURL      string
		wantErrNone     bool
		wantErrWithAuth bool
	}{
		{"https://participant:7575", false, false},
		{"http://participant.internal:7575", false, true},
		{"http://127.0.0.1:7575", false, false},
		{"http://[::1]:7575", false, false},
		{"http://localhost:7575", false, true},
	}
	for _, tc := range cases {
		for authModeName, flags := range authModeFlags {
			t.Run(authModeName+" "+tc.jsonAPIURL, func(t *testing.T) {
				args := append(append([]string{"--json-api", tc.jsonAPIURL}, testPartyFlags...), flags...)
				_, err := parseFlags(args)
				wantErr := tc.wantErrWithAuth
				if authModeName == "none" {
					wantErr = tc.wantErrNone
				}
				if wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "an upstream token needs https")
					return
				}
				require.NoError(t, err)
			})
		}
	}
}
