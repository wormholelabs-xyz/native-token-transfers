package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
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
	config := &tls.Config{MinVersion: tls.VersionTLS12}

	if files.caFile != "" {
		roots, err := loadCABundle(files.caFile)
		if err != nil {
			return nil, err
		}
		//SECURITY: replaces the system roots; only this bundle can vouch for the upstream.
		config.RootCAs = roots
	}

	if files.clientCertFile != "" {
		pair, err := loadClientKeyPair(files, now)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{pair}
	}

	if (files.caFile != "") != (config.RootCAs != nil) {
		panic("disclosure-service: RootCAs disagrees with --json-api-ca-file")
	}
	if (files.clientCertFile != "") != (len(config.Certificates) == 1) {
		panic("disclosure-service: client certificate count disagrees with --json-api-client-cert-file")
	}
	return config, nil
}

// loadCABundle parses every PEM block in path as a CA certificate. Any other block type or
// trailing bytes are an error.
func loadCABundle(path string) (*x509.CertPool, error) {
	pemBytes, err := readFileAtMost(path, maxPEMFileBytes)
	if err != nil {
		return nil, fmt.Errorf("disclosure-service: --json-api-ca-file: %w", err)
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
			return nil, fmt.Errorf("disclosure-service: --json-api-ca-file %s: PEM block of type %q, want CERTIFICATE", path, block.Type)
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("disclosure-service: --json-api-ca-file %s: parse certificate %d: %w", path, certificateCount+1, err)
		}
		roots.AddCert(certificate)
		certificateCount++
	}
	if certificateCount == 0 {
		return nil, fmt.Errorf("disclosure-service: --json-api-ca-file %s: no PEM certificate found", path)
	}
	if len(bytes.TrimSpace(remaining)) != 0 {
		return nil, fmt.Errorf("disclosure-service: --json-api-ca-file %s: trailing data after certificate %d (or more than %d certificates)", path, certificateCount, maxCABundleCertificates)
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
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: client key pair %s, %s: %w", files.clientCertFile, files.clientKeyFile, err)
	}
	if len(pair.Certificate) == 0 {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: no certificate", files.clientCertFile)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: %w", files.clientCertFile, err)
	}
	if now.Before(leaf.NotBefore) {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: not valid before %s", files.clientCertFile, leaf.NotBefore.UTC().Format(time.RFC3339))
	}
	if now.After(leaf.NotAfter) {
		return tls.Certificate{}, fmt.Errorf("disclosure-service: --json-api-client-cert-file %s: expired at %s", files.clientCertFile, leaf.NotAfter.UTC().Format(time.RFC3339))
	}
	pair.Leaf = leaf
	return pair, nil
}
