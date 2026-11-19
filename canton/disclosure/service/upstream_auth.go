package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errBothAuthModes = errors.New("disclosure-service: --access-token-file and the --oauth-* flags are mutually exclusive")

// upstreamAuthMode is one of upstreamAuthNone, upstreamAuthTokenFile, upstreamAuthOAuth2.
type upstreamAuthMode interface {
	sendsBearerToken() bool
	sealUpstreamAuthMode()
}

// upstreamAuthNone is an unauthenticated upstream, for a local development participant.
type upstreamAuthNone struct{}

// upstreamAuthTokenFile sends the bearer token in tokenFilePath, read again for each request.
type upstreamAuthTokenFile struct {
	tokenFilePath string
}

// upstreamAuthOAuth2 sends a bearer token from an OAuth2 client-credentials grant.
type upstreamAuthOAuth2 struct {
	clientConfig validatedOAuthClientConfig
}

func (upstreamAuthNone) sendsBearerToken() bool      { return false }
func (upstreamAuthTokenFile) sendsBearerToken() bool { return true }
func (upstreamAuthOAuth2) sendsBearerToken() bool    { return true }

func (upstreamAuthNone) sealUpstreamAuthMode()      {}
func (upstreamAuthTokenFile) sealUpstreamAuthMode() {}
func (upstreamAuthOAuth2) sealUpstreamAuthMode()    {}

// parseUpstreamAuthMode builds the one auth mode that the flags select.
func parseUpstreamAuthMode(tokenFilePath string, oauthFlags oauthClientFlags) (upstreamAuthMode, error) {
	if oauthFlags.anySet() {
		if tokenFilePath != "" {
			return nil, errBothAuthModes
		}
		clientConfig, err := parseOAuthClientConfig(oauthFlags)
		if err != nil {
			return nil, err
		}
		return upstreamAuthOAuth2{clientConfig: clientConfig}, nil
	}
	if tokenFilePath != "" {
		return upstreamAuthTokenFile{tokenFilePath: tokenFilePath}, nil
	}
	return upstreamAuthNone{}, nil
}

// describeUpstreamAuth names the auth mode for the startup log.
func describeUpstreamAuth(mode upstreamAuthMode) string {
	switch typedMode := mode.(type) {
	case upstreamAuthOAuth2:
		return fmt.Sprintf("OAuth2 client credentials (client id %s)", typedMode.clientConfig.clientID)
	case upstreamAuthTokenFile:
		return "bearer token file"
	case upstreamAuthNone:
		return "none"
	default:
		panic(fmt.Sprintf("disclosure-service: unknown upstream auth mode %T", mode))
	}
}

// newBearerTokenSource builds the token source of mode. A nil source means an unauthenticated
// upstream. It reads the token file, or fetches one OAuth2 token, so a bad setting fails at startup.
func newBearerTokenSource(mode upstreamAuthMode) (bearerTokenSource, error) {
	switch typedMode := mode.(type) {
	case upstreamAuthOAuth2:
		if _, err := readClientSecret(typedMode.clientConfig.clientSecretFile); err != nil {
			return nil, err
		}
		tokenEndpointClient, err := newTokenEndpointHTTPClient(typedMode.clientConfig.caFile)
		if err != nil {
			return nil, err
		}
		tokenSource := newOAuthBearerTokenSource(typedMode.clientConfig, tokenEndpointClient, time.Now)
		if _, err := tokenSource.BearerToken(context.Background()); err != nil {
			return nil, fmt.Errorf("disclosure-service: startup OAuth2 token: %w", err)
		}
		return tokenSource, nil
	case upstreamAuthTokenFile:
		if _, err := readAccessToken(typedMode.tokenFilePath); err != nil {
			return nil, err
		}
		return fileBearerTokenSource{path: typedMode.tokenFilePath}, nil
	case upstreamAuthNone:
		return nil, nil
	default:
		panic(fmt.Sprintf("disclosure-service: unknown upstream auth mode %T", mode))
	}
}
