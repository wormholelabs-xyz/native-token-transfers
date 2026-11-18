package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// maxOAuthResponseBytes caps the token endpoint's response body.
	maxOAuthResponseBytes = 64 << 10
	// maxOAuthSecretFileBytes caps the client secret file.
	maxOAuthSecretFileBytes = 16 << 10
	// maxOAuthTokenLifetimeSeconds bounds expires_in at 30 days.
	maxOAuthTokenLifetimeSeconds = 30 * 24 * 60 * 60
	// maxOAuthRefreshMargin is the most a token is refreshed ahead of its expiry.
	maxOAuthRefreshMargin = 60 * time.Second
	// maxOAuthErrorCodeLength caps the endpoint's "error" code echoed into our error message.
	maxOAuthErrorCodeLength = 64
)

// oauthClientConfig is an OAuth2 client-credentials grant against one token endpoint.
type oauthClientConfig struct {
	tokenURL         string
	clientID         string
	clientSecretFile string
	audience         string
	scope            string // optional
}

// checkOAuthClientConfig enforces every required field and an https (or loopback http) token URL.
func checkOAuthClientConfig(config oauthClientConfig) error {
	required := []struct{ flagName, flagValue string }{
		{"--oauth-token-url", config.tokenURL},
		{"--oauth-client-id", config.clientID},
		{"--oauth-client-secret-file", config.clientSecretFile},
		{"--oauth-audience", config.audience},
	}
	for _, field := range required {
		if field.flagValue == "" {
			return fmt.Errorf("disclosure-service: OAuth2 mode needs %s (set all of --oauth-token-url, --oauth-client-id, --oauth-client-secret-file, --oauth-audience)", field.flagName)
		}
	}
	parsed, err := url.Parse(config.tokenURL)
	if err != nil {
		return fmt.Errorf("disclosure-service: --oauth-token-url %q: %w", config.tokenURL, err)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("disclosure-service: --oauth-token-url %q: missing host", config.tokenURL)
	}
	//SECURITY: the request body carries the client secret; plain http is only for a loopback test endpoint.
	switch parsed.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(parsed.Hostname()) {
			return fmt.Errorf("disclosure-service: --oauth-token-url %q: must be https (http only for a loopback host)", config.tokenURL)
		}
	default:
		return fmt.Errorf("disclosure-service: --oauth-token-url %q: must be https", config.tokenURL)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// readClientSecret reads and trims the client secret file. An empty secret is an error.
func readClientSecret(path string) (string, error) {
	raw, err := readFileAtMost(path, maxOAuthSecretFileBytes)
	if err != nil {
		return "", fmt.Errorf("disclosure-service: --oauth-client-secret-file: %w", err)
	}
	secret := strings.TrimSpace(string(raw))
	if secret == "" {
		return "", fmt.Errorf("disclosure-service: --oauth-client-secret-file %s is empty", path)
	}
	return secret, nil
}

// oauthBearerTokenSource caches one client-credentials access token and refreshes it before
// expiry. One fetch runs at a time; concurrent callers wait for it and share its result.
type oauthBearerTokenSource struct {
	config     oauthClientConfig
	httpClient *http.Client
	now        func() time.Time
	// fetchSlot is a one-slot semaphore. Holding it guards cachedToken and refreshAt.
	fetchSlot   chan struct{}
	cachedToken string
	refreshAt   time.Time
}

func newOAuthBearerTokenSource(config oauthClientConfig, httpClient *http.Client, now func() time.Time) *oauthBearerTokenSource {
	if httpClient == nil {
		panic("disclosure-service: newOAuthBearerTokenSource: nil http client")
	}
	if now == nil {
		panic("disclosure-service: newOAuthBearerTokenSource: nil clock")
	}
	return &oauthBearerTokenSource{
		config:     config,
		httpClient: httpClient,
		now:        now,
		fetchSlot:  make(chan struct{}, 1),
	}
}

// BearerToken returns the cached token, or fetches a new one at or after the refresh point.
func (source *oauthBearerTokenSource) BearerToken(ctx context.Context) (string, error) {
	select {
	case source.fetchSlot <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-source.fetchSlot }()

	requestedAt := source.now()
	if source.cachedToken != "" && requestedAt.Before(source.refreshAt) {
		return source.cachedToken, nil
	}

	token, lifetime, err := source.fetchToken(ctx)
	if err != nil {
		return "", err
	}
	if token == "" || lifetime <= 0 {
		panic("disclosure-service: fetchToken returned an empty token or non-positive lifetime")
	}
	// Measured from before the request: the cached lifetime is at most the real one.
	source.cachedToken = token
	source.refreshAt = requestedAt.Add(lifetime - oauthRefreshMargin(lifetime))
	return token, nil
}

// oauthRefreshMargin is min(60s, 10% of lifetime).
func oauthRefreshMargin(lifetime time.Duration) time.Duration {
	return min(maxOAuthRefreshMargin, lifetime/10)
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// fetchToken runs one client-credentials grant. The secret file is re-read on every fetch so an
// operator can rotate it.
//
// SECURITY: do not put the secret or a token in a returned error; errors reach logs and HTTP responses.
func (source *oauthBearerTokenSource) fetchToken(ctx context.Context) (string, time.Duration, error) {
	secret, err := readClientSecret(source.config.clientSecretFile)
	if err != nil {
		return "", 0, err
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {source.config.clientID},
		"client_secret": {secret},
		"audience":      {source.config.audience},
	}
	if source.config.scope != "" {
		form.Set("scope", source.config.scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, source.config.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("disclosure-service: build OAuth2 token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := source.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("disclosure-service: OAuth2 token request: %w", err)
	}
	defer resp.Body.Close()
	body, err := readAtMost(resp.Body, maxOAuthResponseBytes)
	if err != nil {
		return "", 0, fmt.Errorf("disclosure-service: read OAuth2 token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("disclosure-service: OAuth2 token endpoint: HTTP %d%s", resp.StatusCode, oauthErrorCodeSuffix(body))
	}

	var parsed oauthTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", 0, fmt.Errorf("disclosure-service: parse token response: %w", err)
	}
	if err := checkOAuthTokenResponse(parsed); err != nil {
		return "", 0, err
	}
	return parsed.AccessToken, time.Duration(parsed.ExpiresIn) * time.Second, nil
}

// checkOAuthTokenResponse enforces a non-empty header-safe Bearer token with a bounded lifetime.
func checkOAuthTokenResponse(parsed oauthTokenResponse) error {
	if parsed.AccessToken == "" {
		return fmt.Errorf("disclosure-service: OAuth2 token response: missing access_token")
	}
	for _, character := range parsed.AccessToken {
		if character <= ' ' || character > '~' {
			return fmt.Errorf("disclosure-service: OAuth2 token response: access_token has a character outside printable ASCII")
		}
	}
	//SECURITY: only a Bearer token is valid in the Authorization header the upstream checks.
	if !strings.EqualFold(parsed.TokenType, "Bearer") {
		return fmt.Errorf("disclosure-service: OAuth2 token response: token_type %q, want Bearer", truncateForError(parsed.TokenType))
	}
	if parsed.ExpiresIn <= 0 {
		return fmt.Errorf("disclosure-service: OAuth2 token response: expires_in %d, want a positive value", parsed.ExpiresIn)
	}
	if parsed.ExpiresIn > maxOAuthTokenLifetimeSeconds {
		return fmt.Errorf("disclosure-service: OAuth2 token response: expires_in %d exceeds %d", parsed.ExpiresIn, maxOAuthTokenLifetimeSeconds)
	}
	return nil
}

// oauthErrorCodeSuffix returns " (<code>)" for an RFC 6749 error code made of [a-z_] only, else "".
func oauthErrorCodeSuffix(body []byte) string {
	var errorBody struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errorBody) != nil {
		return ""
	}
	if errorBody.Error == "" || len(errorBody.Error) > maxOAuthErrorCodeLength {
		return ""
	}
	for _, character := range errorBody.Error {
		if (character < 'a' || character > 'z') && character != '_' {
			return ""
		}
	}
	return " (" + errorBody.Error + ")"
}

// newBearerTokenSource picks the upstream auth mode. A nil source means an unauthenticated
// upstream. It reads the token or secret file once, so a bad path fails at startup.
func newBearerTokenSource(opts *options, httpClient *http.Client) (bearerTokenSource, error) {
	//SECURITY: exactly one auth mode, or none. parseFlags enforces this too.
	if opts.accessTokenFile != "" && opts.oauth != nil {
		return nil, errBothAuthModes
	}
	switch {
	case opts.oauth != nil:
		if err := checkOAuthClientConfig(*opts.oauth); err != nil {
			return nil, err
		}
		if _, err := readClientSecret(opts.oauth.clientSecretFile); err != nil {
			return nil, err
		}
		return newOAuthBearerTokenSource(*opts.oauth, httpClient, time.Now), nil
	case opts.accessTokenFile != "":
		if _, err := readAccessToken(opts.accessTokenFile); err != nil {
			return nil, err
		}
		return fileBearerTokenSource{path: opts.accessTokenFile}, nil
	default:
		return nil, nil
	}
}

var errBothAuthModes = fmt.Errorf("disclosure-service: --access-token-file and the --oauth-* flags are mutually exclusive")
