package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"
)

const (
	// maxPEMFileBytes caps every CA, certificate and key file read at startup.
	maxPEMFileBytes = 1 << 20
	// maxCABundleCertificates caps the PEM blocks parsed from one CA bundle.
	maxCABundleCertificates = 512
)

// upstreamTLSFiles names the PEM files for the JSON Ledger API connection. An empty caFile uses
// the system roots.
type upstreamTLSFiles struct {
	caFile         string
	clientCertFile string
	clientKeyFile  string
}

func (files upstreamTLSFiles) anySet() bool {
	return files.caFile != "" || files.clientCertFile != "" || files.clientKeyFile != ""
}

// checkClientPairComplete enforces that the client certificate and key come together.
func (files upstreamTLSFiles) checkClientPairComplete() error {
	//SECURITY: one half of the pair alone would silently drop mutual TLS.
	if (files.clientCertFile == "") != (files.clientKeyFile == "") {
		return fmt.Errorf("disclosure-service: --json-api-client-cert-file and --json-api-client-key-file need both or neither")
	}
	return nil
}

// loadUpstreamTLSConfig reads the PEM files and builds the client TLS config, minimum TLS 1.2.
// now is the clock the client certificate's validity window is checked against.
func loadUpstreamTLSConfig(files upstreamTLSFiles, now time.Time) (*tls.Config, error) {
	if err := files.checkClientPairComplete(); err != nil {
		return nil, err
	}
	if now.IsZero() {
		return nil, fmt.Errorf("disclosure-service: loadUpstreamTLSConfig: zero clock")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if files.caFile != "" {
		roots, err := loadCABundle("--json-api-ca-file", files.caFile)
		if err != nil {
			return nil, err
		}
		//SECURITY: replaces the system roots; only this bundle can vouch for the upstream.
		tlsConfig.RootCAs = roots
	}

	if files.clientCertFile != "" {
		clientKeyPair, err := loadClientKeyPair(files, now)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{clientKeyPair}
	}

	if (files.caFile != "") != (tlsConfig.RootCAs != nil) {
		panic("disclosure-service: RootCAs disagrees with --json-api-ca-file")
	}
	if (files.clientCertFile != "") != (len(tlsConfig.Certificates) == 1) {
		panic("disclosure-service: client certificate count disagrees with --json-api-client-cert-file")
	}
	return tlsConfig, nil
}

// loadCABundle parses every PEM block in path as a CA certificate. Any other block type or
// trailing bytes are an error. flagName names the flag in each error.
func loadCABundle(flagName, path string) (*x509.CertPool, error) {
	pemBytes, err := readFileAtMost(path, maxPEMFileBytes)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: %s: %w", flagName, err)
	}
	roots := x509.NewCertPool()
	remaining := pemBytes
	certificateCount := 0
	for blockIndex := 0; blockIndex < maxCABundleCertificates; blockIndex++ {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		remaining = rest
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("disclosure-service: %s %s: PEM block of type %q, want CERTIFICATE", flagName, path, block.Type)
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("disclosure-service: %s %s: parse certificate %d: %w", flagName, path, certificateCount+1, err)
		}
		roots.AddCert(certificate)
		certificateCount++
	}
	if certificateCount == 0 {
		return nil, fmt.Errorf("disclosure-service: %s %s: no PEM certificate found", flagName, path)
	}
	if len(bytes.TrimSpace(remaining)) != 0 {
		return nil, fmt.Errorf("disclosure-service: %s %s: trailing data after certificate %d (or more than %d certificates)", flagName, path, certificateCount, maxCABundleCertificates)
	}
	return roots, nil
}

// loadClientKeyPair loads the mutual-TLS pair and checks the leaf is valid at now.
func loadClientKeyPair(files upstreamTLSFiles, now time.Time) (tls.Certificate, error) {
	certPEM, err := readFileAtMost(files.clientCertFile, maxPEMFileBytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file: %w", err)
	}
	keyPEM, err := readFileAtMost(files.clientKeyFile, maxPEMFileBytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-key-file: %w", err)
	}
	clientKeyPair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: client key pair %s, %s: %w", files.clientCertFile, files.clientKeyFile, err)
	}
	if len(clientKeyPair.Certificate) == 0 {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: no certificate", files.clientCertFile)
	}
	leaf, err := x509.ParseCertificate(clientKeyPair.Certificate[0])
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: %w", files.clientCertFile, err)
	}
	if now.Before(leaf.NotBefore) {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: not valid before %s", files.clientCertFile, leaf.NotBefore.UTC().Format(time.RFC3339))
	}
	if now.After(leaf.NotAfter) {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: expired at %s", files.clientCertFile, leaf.NotAfter.UTC().Format(time.RFC3339))
	}
	clientKeyPair.Leaf = leaf
	return clientKeyPair, nil
}

// newTokenEndpointHTTPClient is the client for the OAuth2 token endpoint: system roots or the
// --oauth-ca-file bundle and TLS 1.2 minimum. The participant client certificate stays on the
// JSON API client.
func newTokenEndpointHTTPClient(oauthCAFile string) (*http.Client, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if oauthCAFile != "" {
		roots, err := loadCABundle("--oauth-ca-file", oauthCAFile)
		if err != nil {
			return nil, err
		}
		//SECURITY: replaces the system roots; only this bundle can vouch for the token endpoint.
		tlsConfig.RootCAs = roots
	}
	if len(tlsConfig.Certificates) != 0 {
		panic("disclosure-service: the token endpoint client has a client certificate")
	}
	return newDirectHTTPClient(tlsConfig, oauthTokenFetchTimeout), nil
}
