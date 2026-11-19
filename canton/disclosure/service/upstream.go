package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// maxAccessTokenFileBytes bounds the upstream bearer token file.
const maxAccessTokenFileBytes = 16 << 10

// secretFileGroupAndOtherPermissions are the mode bits that a secret file must not set.
const secretFileGroupAndOtherPermissions os.FileMode = 0o077

var errReadLimitExceeded = errors.New("disclosure-service: input exceeds the read limit")

// isLoopbackIPAddress is true for a literal loopback IP. A host name is false: DNS can point it elsewhere.
func isLoopbackIPAddress(host string) bool {
	ipAddress := net.ParseIP(host)
	return ipAddress != nil && ipAddress.IsLoopback()
}

// checkJSONAPIURL enforces an http(s) URL with a host and no userinfo, query, or fragment.
// upstreamAuthConfigured needs https, or http to a loopback IP address. TLS files need https.
func checkJSONAPIURL(rawURL string, upstreamAuthConfigured bool, upstreamTLS upstreamTLSFiles) error {
	jsonAPIURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("disclosure-service: --json-api %q: %w", rawURL, err)
	}
	if jsonAPIURL.Scheme != "http" && jsonAPIURL.Scheme != "https" {
		return fmt.Errorf("disclosure-service: --json-api %q: the scheme must be http or https", rawURL)
	}
	if jsonAPIURL.Hostname() == "" {
		return fmt.Errorf("disclosure-service: --json-api %q has no host", rawURL)
	}
	if jsonAPIURL.User != nil {
		return fmt.Errorf("disclosure-service: --json-api must not hold user information")
	}
	if jsonAPIURL.RawQuery != "" || jsonAPIURL.ForceQuery {
		return fmt.Errorf("disclosure-service: --json-api %q must not hold a query", rawURL)
	}
	if jsonAPIURL.Fragment != "" || jsonAPIURL.RawFragment != "" {
		return fmt.Errorf("disclosure-service: --json-api %q must not hold a fragment", rawURL)
	}
	//SECURITY: a bearer token on http to a remote host is readable on the network path.
	if upstreamAuthConfigured && jsonAPIURL.Scheme == "http" && !isLoopbackIPAddress(jsonAPIURL.Hostname()) {
		return fmt.Errorf("disclosure-service: --json-api %q: an upstream token needs https, or http to a loopback IP address", rawURL)
	}
	if err := upstreamTLS.checkClientPairComplete(); err != nil {
		return err
	}
	//SECURITY: TLS flags on an http URL would leave the operator believing the link is encrypted.
	if upstreamTLS.anySet() && jsonAPIURL.Scheme != "https" {
		return fmt.Errorf("disclosure-service: --json-api-ca-file, --json-api-client-cert-file and --json-api-client-key-file require an https --json-api, got %q", rawURL)
	}
	return nil
}

// newUpstreamHTTPClient is the client for the JSON Ledger API.
func newUpstreamHTTPClient(tlsConfig *tls.Config) *http.Client {
	return newDirectHTTPClient(tlsConfig, httpClientTimeout)
}

// newDirectHTTPClient connects directly with tlsConfig and ignores proxy variables. It returns a
// redirect to the caller as the response.
func newDirectHTTPClient(tlsConfig *tls.Config, timeout time.Duration) *http.Client {
	if tlsConfig == nil {
		panic("disclosure-service: newDirectHTTPClient: nil TLS config")
	}
	if tlsConfig.MinVersion < tls.VersionTLS12 {
		panic("disclosure-service: newDirectHTTPClient: TLS minimum below 1.2")
	}
	if timeout <= 0 {
		panic("disclosure-service: newDirectHTTPClient: non-positive timeout")
	}
	defaultTransport, isTransport := http.DefaultTransport.(*http.Transport)
	if !isTransport {
		panic("disclosure-service: http.DefaultTransport is not an *http.Transport")
	}
	transport := defaultTransport.Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = tlsConfig
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		//SECURITY: a 307/308 re-sends the request body, which can carry the OAuth2 client secret, to any scheme or host.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// readAtMost reads source to the end and fails when it yields more than maxBytes.
func readAtMost(source io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes < 0 {
		panic("disclosure-service: readAtMost needs a non-negative limit")
	}
	contents, err := io.ReadAll(io.LimitReader(source, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > maxBytes {
		return nil, fmt.Errorf("%w: more than %d bytes", errReadLimitExceeded, maxBytes)
	}
	return contents, nil
}

// readFileAtMost reads the file at path, failing when it holds more than maxBytes.
func readFileAtMost(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readAtMost(file, maxBytes)
}

// readOwnerOnlyFileAtMost is readFileAtMost for a secret file. The file must give no permission
// to the group or to other users.
func readOwnerOnlyFileAtMost(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	//SECURITY: checked on the open descriptor, so a change of the path after the check has no effect.
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if permissions := fileInfo.Mode().Perm(); permissions&secretFileGroupAndOtherPermissions != 0 {
		return nil, fmt.Errorf("%s has mode %04o, which gives a permission to the group or other users; set mode 0400 or 0600", path, permissions)
	}
	return readAtMost(file, maxBytes)
}
