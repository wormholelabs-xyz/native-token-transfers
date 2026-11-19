package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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
	// oauthFailureBackoffInitial is the wait after the first failed fetch; it doubles to oauthFailureBackoffMax.
	oauthFailureBackoffInitial = 1 * time.Second
	oauthFailureBackoffMax     = 30 * time.Second
	// oauthTokenFetchTimeout bounds one token fetch, inside the inbound request deadline.
	oauthTokenFetchTimeout = 10 * time.Second
)

var errOAuthTokenResponseMalformed = errors.New("disclosure-service: parse token response: the body is not a valid token response")

var errOAuthTokenEndpointBackoff = errors.New("disclosure-service: OAuth2 token endpoint failed recently; waiting before the next fetch")

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
	rawSecret, err := readOwnerOnlyFileAtMost(path, maxOAuthSecretFileBytes)
	if err != nil {
		return "", fmt.Errorf("disclosure-service: --oauth-client-secret-file: %w", err)
	}
	secret := strings.TrimSpace(string(rawSecret))
	if secret == "" {
		return "", fmt.Errorf("disclosure-service: --oauth-client-secret-file %s is empty", path)
	}
	return secret, nil
}

// oauthBearerTokenSource caches one client-credentials access token and refreshes it before
// expiry. One fetch runs at a time; concurrent callers wait for it and share its result.
type oauthBearerTokenSource struct {
	clientConfig      validatedOAuthClientConfig
	httpClient        *http.Client
	now               func() time.Time
	tokenFetchTimeout time.Duration
	logger            *log.Logger
	// fetchSlot is a one-slot semaphore. Its holder runs the only token fetch.
	fetchSlot chan struct{}
	// mu guards the fields below it. Do not hold it across I/O.
	mu          sync.Mutex
	cachedToken string
	refreshAt   time.Time
	// cachedTokenUsableUntil ends the use of the cached token after a failed refresh.
	cachedTokenUsableUntil time.Time
	// latestFetchedToken is the newest fetched token not yet counted as rejected.
	latestFetchedToken string
	failureBackoff     time.Duration
	retryNotBefore     time.Time
	// upstreamRejectionUnresolved keeps failureBackoff across a fetch success until the upstream accepts a token.
	upstreamRejectionUnresolved bool
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
		clientConfig:      clientConfig,
		httpClient:        httpClient,
		now:               now,
		tokenFetchTimeout: oauthTokenFetchTimeout,
		logger:            log.New(os.Stderr, "", log.LstdFlags),
		fetchSlot:         make(chan struct{}, 1),
	}
}

// oauthTokenFetchResult is the answer of one fetch to the caller that started it.
type oauthTokenFetchResult struct {
	token string
	err   error
}

// BearerToken returns the cached token, or fetches a new one at or after the refresh point. After
// a failed fetch it returns the cached token while it is usable, else errOAuthTokenEndpointBackoff
// until the backoff ends.
func (source *oauthBearerTokenSource) BearerToken(ctx context.Context) (string, error) {
	if token, answered, err := source.answerWithoutFetch(); answered {
		return token, err
	}
	select {
	case source.fetchSlot <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	// Another caller can have fetched a token during the wait for the slot.
	if token, answered, err := source.answerWithoutFetch(); answered {
		<-source.fetchSlot
		return token, err
	}

	//SECURITY: the fetch outlives a canceled caller, so only a token endpoint failure arms the backoff.
	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), source.tokenFetchTimeout)
	fetchResult := make(chan oauthTokenFetchResult, 1)
	go func() {
		defer func() { <-source.fetchSlot }()
		defer cancel()
		requestedAt := source.now()
		token, lifetime, err := source.fetchToken(fetchCtx)
		fetchResult <- source.recordFetchResult(requestedAt, token, lifetime, err)
	}()
	select {
	case result := <-fetchResult:
		return result.token, result.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// answerWithoutFetch answers from the cache or from the backoff. answered is false when a fetch is due.
func (source *oauthBearerTokenSource) answerWithoutFetch() (token string, answered bool, err error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	now := source.now()
	if source.cachedToken != "" && now.Before(source.refreshAt) {
		return source.cachedToken, true, nil
	}
	if !now.Before(source.retryNotBefore) {
		return "", false, nil
	}
	if source.cachedTokenUsableLocked(now) {
		return source.cachedToken, true, nil
	}
	return "", true, errOAuthTokenEndpointBackoff
}

func (source *oauthBearerTokenSource) cachedTokenUsableLocked(now time.Time) bool {
	return source.cachedToken != "" && now.Before(source.cachedTokenUsableUntil)
}

// recordFetchResult caches a fetched token, or arms the backoff from the failure time.
func (source *oauthBearerTokenSource) recordFetchResult(requestedAt time.Time, token string, lifetime time.Duration, fetchErr error) oauthTokenFetchResult {
	if fetchErr != nil {
		source.mu.Lock()
		failedAt := source.now()
		source.armFailureBackoffLocked(failedAt)
		cachedTokenUsable := source.cachedTokenUsableLocked(failedAt)
		cachedToken, cachedTokenUsableUntil := source.cachedToken, source.cachedTokenUsableUntil
		source.mu.Unlock()
		if !cachedTokenUsable {
			return oauthTokenFetchResult{err: fetchErr}
		}
		source.logger.Printf("disclosure-service: OAuth2 token refresh failed; the cached token stays in use until %s: %v", cachedTokenUsableUntil.UTC().Format(time.RFC3339), fetchErr)
		return oauthTokenFetchResult{token: cachedToken}
	}
	if token == "" || lifetime <= 0 {
		panic("disclosure-service: fetchToken returned an empty token or non-positive lifetime")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	// Measured from before the request: the cached lifetime is at most the real one.
	source.cachedToken = token
	source.latestFetchedToken = token
	source.refreshAt = requestedAt.Add(lifetime - oauthRefreshMargin(lifetime))
	source.cachedTokenUsableUntil = requestedAt.Add(lifetime - oauthExpirySafetyMargin(lifetime))
	if !source.refreshAt.Before(source.cachedTokenUsableUntil) {
		panic("disclosure-service: the refresh point is not before the end of the token use")
	}
	source.retryNotBefore = time.Time{}
	if !source.upstreamRejectionUnresolved {
		source.failureBackoff = 0
	}
	return oauthTokenFetchResult{token: token}
}

// armFailureBackoffLocked doubles the backoff, from 1 s to 30 s, and starts it at failedAt.
func (source *oauthBearerTokenSource) armFailureBackoffLocked(failedAt time.Time) {
	source.failureBackoff = min(max(2*source.failureBackoff, oauthFailureBackoffInitial), oauthFailureBackoffMax)
	source.retryNotBefore = failedAt.Add(source.failureBackoff)
}

// discardCachedTokenLocked clears the cache when it still holds rejectedToken.
func (source *oauthBearerTokenSource) discardCachedTokenLocked(rejectedToken string) {
	if rejectedToken == "" || source.cachedToken != rejectedToken {
		return
	}
	source.cachedToken = ""
	source.refreshAt = time.Time{}
	source.cachedTokenUsableUntil = time.Time{}
}

// DiscardRejectedBearerToken clears the cache when it still holds rejectedToken, so the next
// BearerToken call fetches a new token. A token that another caller already replaced stays.
func (source *oauthBearerTokenSource) DiscardRejectedBearerToken(rejectedToken string) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.discardCachedTokenLocked(rejectedToken)
}

// RecordRejectedRetryBearerToken discards rejectedToken and counts one token source failure. A
// token that is not the latest fetch, or that is already counted, changes nothing.
func (source *oauthBearerTokenSource) RecordRejectedRetryBearerToken(rejectedToken string) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if rejectedToken == "" || rejectedToken != source.latestFetchedToken {
		return
	}
	source.latestFetchedToken = ""
	source.discardCachedTokenLocked(rejectedToken)
	source.armFailureBackoffLocked(source.now())
	source.upstreamRejectionUnresolved = true
}

// RecordAcceptedBearerToken resets the backoff that upstream rejections armed, when acceptedToken
// is the cached token.
func (source *oauthBearerTokenSource) RecordAcceptedBearerToken(acceptedToken string) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if !source.upstreamRejectionUnresolved || acceptedToken == "" || acceptedToken != source.cachedToken {
		return
	}
	source.upstreamRejectionUnresolved = false
	source.failureBackoff = 0
}

// oauthRefreshMargin is min(60s, 10% of lifetime).
func oauthRefreshMargin(lifetime time.Duration) time.Duration {
	return min(maxOAuthRefreshMargin, lifetime/10)
}

// oauthExpirySafetyMargin is half the refresh margin: a cached token stays in use until this long before its expiry.
func oauthExpirySafetyMargin(lifetime time.Duration) time.Duration {
	return oauthRefreshMargin(lifetime) / 2
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

	var tokenResponse oauthTokenResponse
	//SECURITY: a json error can quote a number from the body.
	if json.Unmarshal(body, &tokenResponse) != nil {
		return "", 0, errOAuthTokenResponseMalformed
	}
	if err := checkOAuthTokenResponse(tokenResponse); err != nil {
		return "", 0, err
	}
	return tokenResponse.AccessToken, time.Duration(tokenResponse.ExpiresIn) * time.Second, nil
}

// checkOAuthTokenResponse enforces a non-empty header-safe Bearer token with a bounded lifetime.
func checkOAuthTokenResponse(tokenResponse oauthTokenResponse) error {
	if tokenResponse.AccessToken == "" {
		return fmt.Errorf("disclosure-service: OAuth2 token response: missing access_token")
	}
	for _, character := range tokenResponse.AccessToken {
		if character <= ' ' || character > '~' {
			return fmt.Errorf("disclosure-service: OAuth2 token response: access_token has a character outside printable ASCII")
		}
	}
	//SECURITY: only a Bearer token is valid in the Authorization header the upstream checks.
	if !strings.EqualFold(tokenResponse.TokenType, "Bearer") {
		return fmt.Errorf("disclosure-service: OAuth2 token response: token_type %q, want Bearer", truncateForError(tokenResponse.TokenType))
	}
	if tokenResponse.ExpiresIn <= 0 {
		return fmt.Errorf("disclosure-service: OAuth2 token response: expires_in %d, want a positive value", tokenResponse.ExpiresIn)
	}
	if tokenResponse.ExpiresIn > maxOAuthTokenLifetimeSeconds {
		return fmt.Errorf("disclosure-service: OAuth2 token response: expires_in %d exceeds %d", tokenResponse.ExpiresIn, maxOAuthTokenLifetimeSeconds)
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
