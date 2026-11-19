package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// oauthClientFlags are the raw --oauth-* flag values.
type oauthClientFlags struct {
	tokenURL         string
	clientID         string
	clientSecretFile string
	audience         string
	scope            string
	caFile           string
}

func (flags oauthClientFlags) anySet() bool {
	return flags.tokenURL != "" || flags.clientID != "" || flags.clientSecretFile != "" || flags.audience != "" || flags.scope != "" || flags.caFile != ""
}

// validatedOAuthClientConfig is an OAuth2 client-credentials grant against one token endpoint.
// Build it only with parseOAuthClientConfig.
type validatedOAuthClientConfig struct {
	tokenURL         *url.URL
	clientID         string
	clientSecretFile string
	audience         string
	scope            string // optional
	caFile           string // optional; empty uses the system roots
}

// parseOAuthClientConfig enforces every required field and an https (or loopback http) token URL.
func parseOAuthClientConfig(flags oauthClientFlags) (validatedOAuthClientConfig, error) {
	required := []struct{ flagName, flagValue string }{
		{"--oauth-token-url", flags.tokenURL},
		{"--oauth-client-id", flags.clientID},
		{"--oauth-client-secret-file", flags.clientSecretFile},
		{"--oauth-audience", flags.audience},
	}
	for _, field := range required {
		if field.flagValue == "" {
			return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: OAuth2 mode needs %s (set all of --oauth-token-url, --oauth-client-id, --oauth-client-secret-file, --oauth-audience)", field.flagName)
		}
	}
	tokenURL, err := url.Parse(flags.tokenURL)
	if err != nil {
		return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-token-url %q: %w", flags.tokenURL, err)
	}
	if tokenURL.Hostname() == "" {
		return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-token-url %q: missing host", flags.tokenURL)
	}
	if tokenURL.User != nil {
		return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-token-url must not hold user information")
	}
	if tokenURL.RawQuery != "" || tokenURL.ForceQuery {
		return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-token-url %q must not hold a query", flags.tokenURL)
	}
	if tokenURL.Fragment != "" || tokenURL.RawFragment != "" {
		return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-token-url %q must not hold a fragment", flags.tokenURL)
	}
	//SECURITY: the request body carries the client secret; plain http is only for a loopback IP address.
	switch tokenURL.Scheme {
	case "https":
	case "http":
		if !isLoopbackIPAddress(tokenURL.Hostname()) {
			return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-token-url %q: must be https (http only to a loopback IP address)", flags.tokenURL)
		}
	default:
		return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-token-url %q: must be https", flags.tokenURL)
	}
	if flags.caFile != "" && tokenURL.Scheme != "https" {
		return validatedOAuthClientConfig{}, fmt.Errorf("disclosure-service: --oauth-ca-file requires an https --oauth-token-url, got %q", flags.tokenURL)
	}
	return validatedOAuthClientConfig{
		tokenURL:         tokenURL,
		clientID:         flags.clientID,
		clientSecretFile: flags.clientSecretFile,
		audience:         flags.audience,
		scope:            flags.scope,
		caFile:           flags.caFile,
	}, nil
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
	clientConfig validatedOAuthClientConfig
	httpClient   *http.Client
	now          func() time.Time
	// fetchSlot is a one-slot semaphore. Holding it guards cachedToken and refreshAt.
	fetchSlot   chan struct{}
	cachedToken string
	refreshAt   time.Time
}

func newOAuthBearerTokenSource(clientConfig validatedOAuthClientConfig, httpClient *http.Client, now func() time.Time) *oauthBearerTokenSource {
	if httpClient == nil {
		panic("disclosure-service: newOAuthBearerTokenSource: nil http client")
	}
	if now == nil {
		panic("disclosure-service: newOAuthBearerTokenSource: nil clock")
	}
	if clientConfig.tokenURL == nil {
		panic("disclosure-service: newOAuthBearerTokenSource: unvalidated client config")
	}
	return &oauthBearerTokenSource{
		clientConfig: clientConfig,
		httpClient:   httpClient,
		now:          now,
		fetchSlot:    make(chan struct{}, 1),
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
	secret, err := readClientSecret(source.clientConfig.clientSecretFile)
	if err != nil {
		return "", 0, err
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {source.clientConfig.clientID},
		"client_secret": {secret},
		"audience":      {source.clientConfig.audience},
	}
	if source.clientConfig.scope != "" {
		form.Set("scope", source.clientConfig.scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, source.clientConfig.tokenURL.String(), strings.NewReader(form.Encode()))
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
