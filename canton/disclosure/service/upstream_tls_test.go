package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test PKI
// ---------------------------------------------------------------------------

type testCertificateAuthority struct {
	certificate *x509.Certificate
	privateKey  *ecdsa.PrivateKey
	pemPath     string
}

type testIssuedCertificate struct {
	certPEMPath string
	keyPEMPath  string
	tlsPair     tls.Certificate
}

var testSerialNumber int64

func nextTestSerialNumber() *big.Int {
	testSerialNumber++
	return big.NewInt(testSerialNumber)
}

func writePEMFile(t *testing.T, dir, name, blockType string, der []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600))
	return path
}

func newTestCertificateAuthority(t *testing.T, dir, name string) *testCertificateAuthority {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          nextTestSerialNumber(),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &testCertificateAuthority{
		certificate: certificate,
		privateKey:  privateKey,
		pemPath:     writePEMFile(t, dir, name+"-ca.pem", "CERTIFICATE", der),
	}
}

// issue signs a leaf for 127.0.0.1, valid over [notBefore, notAfter].
func (ca *testCertificateAuthority) issue(t *testing.T, dir, name string, usage x509.ExtKeyUsage, notBefore, notAfter time.Time) *testIssuedCertificate {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: nextTestSerialNumber(),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &privateKey.PublicKey, ca.privateKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	certPath := writePEMFile(t, dir, name+"-cert.pem", "CERTIFICATE", der)
	keyPath := writePEMFile(t, dir, name+"-key.pem", "PRIVATE KEY", keyDER)
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	return &testIssuedCertificate{certPEMPath: certPath, keyPEMPath: keyPath, tlsPair: pair}
}

type testUpstreamPKI struct {
	dir               string
	trustedCA         *testCertificateAuthority
	untrustedCA       *testCertificateAuthority
	serverCertificate *testIssuedCertificate
	validClient       *testIssuedCertificate
	expiredClient     *testIssuedCertificate
}

func newTestUpstreamPKI(t *testing.T) *testUpstreamPKI {
	t.Helper()
	dir := t.TempDir()
	trustedCA := newTestCertificateAuthority(t, dir, "trusted")
	now := time.Now()
	return &testUpstreamPKI{
		dir:               dir,
		trustedCA:         trustedCA,
		untrustedCA:       newTestCertificateAuthority(t, dir, "untrusted"),
		serverCertificate: trustedCA.issue(t, dir, "server", x509.ExtKeyUsageServerAuth, now.Add(-time.Hour), now.Add(time.Hour)),
		validClient:       trustedCA.issue(t, dir, "client", x509.ExtKeyUsageClientAuth, now.Add(-time.Hour), now.Add(time.Hour)),
		expiredClient:     trustedCA.issue(t, dir, "expired-client", x509.ExtKeyUsageClientAuth, now.Add(-2*time.Hour), now.Add(-time.Hour)),
	}
}

// newMutualTLSJSONAPI starts a TLS JSON Ledger API stand-in that requires a client certificate
// signed by pki.trustedCA.
func newMutualTLSJSONAPI(t *testing.T, pki *testUpstreamPKI) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/state/ledger-end", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]int64{"offset": 42})
	})
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(pki.trustedCA.certificate)
	srv := httptest.NewUnstartedServer(mux)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{pki.serverCertificate.tlsPair},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

// ---------------------------------------------------------------------------
// Handshake against a mutual-TLS upstream
// ---------------------------------------------------------------------------

func TestUpstreamTLS_MutualTLSHandshake(t *testing.T) {
	pki := newTestUpstreamPKI(t)
	upstream := newMutualTLSJSONAPI(t, pki)

	cases := []struct {
		name string
		// configLoadTime is the clock loadUpstreamTLSConfig checks the client certificate against.
		configLoadTime time.Time
		files          upstreamTLSFiles
		wantErrSub     string
	}{
		{
			name:           "trusted CA and valid client certificate",
			configLoadTime: time.Now(),
			files: upstreamTLSFiles{
				caFile:         pki.trustedCA.pemPath,
				clientCertFile: pki.validClient.certPEMPath,
				clientKeyFile:  pki.validClient.keyPEMPath,
			},
		},
		{
			name:           "no client certificate",
			configLoadTime: time.Now(),
			files:          upstreamTLSFiles{caFile: pki.trustedCA.pemPath},
			wantErrSub:     "certificate required",
		},
		{
			name:           "untrusted server CA",
			configLoadTime: time.Now(),
			files: upstreamTLSFiles{
				caFile:         pki.untrustedCA.pemPath,
				clientCertFile: pki.validClient.certPEMPath,
				clientKeyFile:  pki.validClient.keyPEMPath,
			},
			wantErrSub: "certificate signed by unknown authority",
		},
		{
			name: "system roots only",
			// System roots fail to verify the test CA's server certificate.
			configLoadTime: time.Now(),
			files: upstreamTLSFiles{
				clientCertFile: pki.validClient.certPEMPath,
				clientKeyFile:  pki.validClient.keyPEMPath,
			},
			wantErrSub: "failed to verify certificate",
		},
		{
			// Loaded while still valid, so the server's check is what rejects it.
			name:           "client certificate expired at handshake",
			configLoadTime: time.Now().Add(-90 * time.Minute),
			files: upstreamTLSFiles{
				caFile:         pki.trustedCA.pemPath,
				clientCertFile: pki.expiredClient.certPEMPath,
				clientKeyFile:  pki.expiredClient.keyPEMPath,
			},
			wantErrSub: "expired certificate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tlsConfig, err := loadUpstreamTLSConfig(tc.files, tc.configLoadTime)
			require.NoError(t, err)
			client := newHTTPACSClient(upstream.URL, newUpstreamHTTPClient(tlsConfig), nil)

			offset, err := client.LedgerEnd(context.Background())
			if tc.wantErrSub == "" {
				require.NoError(t, err)
				assert.Equal(t, int64(42), offset)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrSub)
		})
	}
}

// ---------------------------------------------------------------------------
// Startup loading
// ---------------------------------------------------------------------------

func TestLoadUpstreamTLSConfig(t *testing.T) {
	pki := newTestUpstreamPKI(t)
	garbagePath := filepath.Join(pki.dir, "garbage.pem")
	require.NoError(t, os.WriteFile(garbagePath, []byte("not a pem file\n"), 0o600))
	keyAsCAPath := pki.validClient.keyPEMPath
	trailingGarbagePath := filepath.Join(pki.dir, "trailing.pem")
	caPEM, err := os.ReadFile(pki.trustedCA.pemPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(trailingGarbagePath, append(caPEM, []byte("junk")...), 0o600))
	oversizePath := filepath.Join(pki.dir, "oversize.pem")
	require.NoError(t, os.WriteFile(oversizePath, make([]byte, maxPEMFileBytes+1), 0o600))
	missingPath := filepath.Join(pki.dir, "missing.pem")

	cases := []struct {
		name       string
		files      upstreamTLSFiles
		wantErrSub string
		wantRoots  bool
		wantPairs  int
	}{
		{name: "no files: system roots", files: upstreamTLSFiles{}},
		{name: "CA only", files: upstreamTLSFiles{caFile: pki.trustedCA.pemPath}, wantRoots: true},
		{
			name:      "CA and client pair",
			files:     upstreamTLSFiles{caFile: pki.trustedCA.pemPath, clientCertFile: pki.validClient.certPEMPath, clientKeyFile: pki.validClient.keyPEMPath},
			wantRoots: true,
			wantPairs: 1,
		},
		{name: "missing CA file", files: upstreamTLSFiles{caFile: missingPath}, wantErrSub: "--json-api-ca-file"},
		{name: "CA file without PEM", files: upstreamTLSFiles{caFile: garbagePath}, wantErrSub: "no PEM certificate"},
		{name: "CA file holds a private key", files: upstreamTLSFiles{caFile: keyAsCAPath}, wantErrSub: "PEM block of type"},
		{name: "CA file with trailing bytes", files: upstreamTLSFiles{caFile: trailingGarbagePath}, wantErrSub: "trailing data"},
		{name: "CA file over the size cap", files: upstreamTLSFiles{caFile: oversizePath}, wantErrSub: "exceeds"},
		{name: "cert without key", files: upstreamTLSFiles{clientCertFile: pki.validClient.certPEMPath}, wantErrSub: "both or neither"},
		{name: "key without cert", files: upstreamTLSFiles{clientKeyFile: pki.validClient.keyPEMPath}, wantErrSub: "both or neither"},
		{name: "missing client cert file", files: upstreamTLSFiles{clientCertFile: missingPath, clientKeyFile: pki.validClient.keyPEMPath}, wantErrSub: "--json-api-client-cert-file"},
		{
			name:       "mismatched cert and key",
			files:      upstreamTLSFiles{clientCertFile: pki.validClient.certPEMPath, clientKeyFile: pki.expiredClient.keyPEMPath},
			wantErrSub: "client key pair",
		},
		{
			name:       "expired client certificate",
			files:      upstreamTLSFiles{clientCertFile: pki.expiredClient.certPEMPath, clientKeyFile: pki.expiredClient.keyPEMPath},
			wantErrSub: "expired",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tlsConfig, err := loadUpstreamTLSConfig(tc.files, time.Now())
			if tc.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSub)
				assert.Nil(t, tlsConfig)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
			assert.Equal(t, tc.wantRoots, tlsConfig.RootCAs != nil)
			assert.Len(t, tlsConfig.Certificates, tc.wantPairs)
		})
	}
}

func TestLoadUpstreamTLSConfig_RejectsNotYetValidClientCertificate(t *testing.T) {
	pki := newTestUpstreamPKI(t)
	files := upstreamTLSFiles{clientCertFile: pki.validClient.certPEMPath, clientKeyFile: pki.validClient.keyPEMPath}
	_, err := loadUpstreamTLSConfig(files, time.Now().Add(-2*time.Hour))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid before")
}

func TestUpstreamHTTPClient_RefusesRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("redirect target was called")
	}))
	t.Cleanup(target.Close)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirector.Close)

	tlsConfig, err := loadUpstreamTLSConfig(upstreamTLSFiles{}, time.Now())
	require.NoError(t, err)
	client := newHTTPACSClient(redirector.URL, newUpstreamHTTPClient(tlsConfig), nil)
	_, err = client.LedgerEnd(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 307")
}

// ---------------------------------------------------------------------------
// Flag validation
// ---------------------------------------------------------------------------

func TestParseFlags_UpstreamTLS(t *testing.T) {
	base := testPartyFlags
	cases := []struct {
		name       string
		args       []string
		wantErrSub string
		wantFiles  upstreamTLSFiles
	}{
		{
			name:      "https with CA and client pair",
			args:      []string{"--json-api", "https://participant.example:7575", "--json-api-ca-file", "ca.pem", "--json-api-client-cert-file", "c.pem", "--json-api-client-key-file", "k.pem"},
			wantFiles: upstreamTLSFiles{caFile: "ca.pem", clientCertFile: "c.pem", clientKeyFile: "k.pem"},
		},
		{
			name:      "https without TLS flags",
			args:      []string{"--json-api", "https://participant.example:7575"},
			wantFiles: upstreamTLSFiles{},
		},
		{
			name:       "http with a CA file",
			args:       []string{"--json-api", "http://127.0.0.1:6975", "--json-api-ca-file", "ca.pem"},
			wantErrSub: "require an https --json-api",
		},
		{
			name:       "http with a client pair",
			args:       []string{"--json-api", "http://127.0.0.1:6975", "--json-api-client-cert-file", "c.pem", "--json-api-client-key-file", "k.pem"},
			wantErrSub: "require an https --json-api",
		},
		{
			name:       "cert without key",
			args:       []string{"--json-api", "https://participant.example", "--json-api-client-cert-file", "c.pem"},
			wantErrSub: "both or neither",
		},
		{
			name:       "key without cert",
			args:       []string{"--json-api", "https://participant.example", "--json-api-client-key-file", "k.pem"},
			wantErrSub: "both or neither",
		},
		{
			name:       "unsupported scheme",
			args:       []string{"--json-api", "ftp://participant.example"},
			wantErrSub: "http or https",
		},
		{
			name:       "missing host",
			args:       []string{"--json-api", "https://"},
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
			assert.Equal(t, tc.wantFiles, opts.upstreamTLS)
		})
	}
}

func TestRun_FailsFastOnInvalidUpstreamTLSFile(t *testing.T) {
	missingCA := filepath.Join(t.TempDir(), "missing-ca.pem")
	opts := &options{
		listen:                  "127.0.0.1:0",
		upstreamAuth:            upstreamAuthNone{},
		jsonAPIBaseURL:          "https://127.0.0.1:1",
		readerParties:           testReaderParties(),
		maxContractsPerTemplate: defaultMaxContractsPerTemplate,
		maxUpstreamConcurrency:  defaultMaxUpstreamConcurrency,
		upstreamTLS:             upstreamTLSFiles{caFile: missingCA},
	}
	var out syncBuffer
	err := run(context.Background(), opts, &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--json-api-ca-file")
	assert.Empty(t, listeningURL(out.String()))
}

// ---------------------------------------------------------------------------
// Token endpoint client
// ---------------------------------------------------------------------------

// newTLSTokenEndpoint serves bearer tokens over TLS with serverCertificate. It requests a client
// certificate and records how many the client sent.
func newTLSTokenEndpoint(t *testing.T, serverCertificate *testIssuedCertificate, peerCertificateCount *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerCertificateCount.Store(int64(len(r.TLS.PeerCertificates)))
		bearerTokenResponse("tok", 3600)(w, 1)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCertificate.tlsPair},
		ClientAuth:   tls.RequestClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func TestTokenEndpointClientTrust(t *testing.T) {
	tokenEndpointPKI := newTestUpstreamPKI(t)
	participantPKI := newTestUpstreamPKI(t)
	participant := newMutualTLSJSONAPI(t, participantPKI)
	var peerCertificateCount atomic.Int64
	tokenEndpoint := newTLSTokenEndpoint(t, tokenEndpointPKI.serverCertificate, &peerCertificateCount)
	secretPath := filepath.Join(t.TempDir(), "client-secret")
	writeClientSecret(t, secretPath, "secret")
	participantTLS := upstreamTLSFiles{
		caFile:         participantPKI.trustedCA.pemPath,
		clientCertFile: participantPKI.validClient.certPEMPath,
		clientKeyFile:  participantPKI.validClient.keyPEMPath,
	}

	cases := []struct {
		name        string
		oauthCAFile string
		wantErrSub  string
	}{
		{"token endpoint CA from --oauth-ca-file, participant CA from --json-api-ca-file", tokenEndpointPKI.trustedCA.pemPath, ""},
		{"token endpoint CA absent from the system roots", "", "startup OAuth2 token"},
		{"participant CA given as --oauth-ca-file", participantPKI.trustedCA.pemPath, "startup OAuth2 token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peerCertificateCount.Store(-1)
			clientConfig := mustParseOAuthClientConfig(t, oauthClientFlags{
				tokenURL:         tokenEndpoint.URL + "/token",
				clientID:         "gateway",
				clientSecretFile: secretPath,
				audience:         "aud",
				caFile:           tc.oauthCAFile,
			})
			tokens, err := newBearerTokenSource(upstreamAuthOAuth2{clientConfig: clientConfig})
			if tc.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSub)
				assert.Contains(t, err.Error(), "certificate")
				return
			}
			require.NoError(t, err)
			tlsConfig, err := loadUpstreamTLSConfig(participantTLS, time.Now())
			require.NoError(t, err)
			offset, err := newHTTPACSClient(participant.URL, newUpstreamHTTPClient(tlsConfig), tokens).LedgerEnd(context.Background())
			require.NoError(t, err)
			assert.Equal(t, int64(42), offset)
			assert.Zero(t, peerCertificateCount.Load(), "the token endpoint sees no client certificate")
		})
	}
}

func TestNewTokenEndpointHTTPClient(t *testing.T) {
	pki := newTestUpstreamPKI(t)
	cases := []struct {
		name        string
		oauthCAFile string
		wantRootCAs bool
		wantErrSub  string
	}{
		{"system roots", "", false, ""},
		{"CA bundle", pki.trustedCA.pemPath, true, ""},
		{"missing CA bundle", filepath.Join(pki.dir, "missing.pem"), false, "--oauth-ca-file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := newTokenEndpointHTTPClient(tc.oauthCAFile)
			if tc.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSub)
				return
			}
			require.NoError(t, err)
			transport, isTransport := client.Transport.(*http.Transport)
			require.True(t, isTransport)
			assert.Nil(t, transport.Proxy, "the token client ignores HTTP_PROXY and HTTPS_PROXY")
			assert.Empty(t, transport.TLSClientConfig.Certificates)
			assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
			assert.Equal(t, tc.wantRootCAs, transport.TLSClientConfig.RootCAs != nil)
			assert.NotNil(t, client.CheckRedirect)
		})
	}
}

// The token file, the client secret file, and the client key file permit owner access and group
// read only: the Kubernetes Secret mount with fsGroup and defaultMode 0440 passes.
func TestSecretFilesRefuseGroupWriteAndOtherPermissions(t *testing.T) {
	pki := newTestUpstreamPKI(t)
	secretDirectory := t.TempDir()
	secretReaders := []struct {
		name       string
		path       string
		readSecret func(path string) error
	}{
		{"access token file", filepath.Join(secretDirectory, "token"), func(path string) error {
			_, err := readAccessToken(path)
			return err
		}},
		{"client secret file", filepath.Join(secretDirectory, "client-secret"), func(path string) error {
			_, err := readClientSecret(path)
			return err
		}},
		{"client key file", pki.validClient.keyPEMPath, func(path string) error {
			_, err := loadClientKeyPair(upstreamTLSFiles{clientCertFile: pki.validClient.certPEMPath, clientKeyFile: path}, time.Now())
			return err
		}},
	}
	require.NoError(t, os.WriteFile(secretReaders[0].path, []byte("tok\n"), 0o600))
	require.NoError(t, os.WriteFile(secretReaders[1].path, []byte("secret\n"), 0o600))
	modes := []struct {
		mode    os.FileMode
		wantErr bool
	}{
		{0o400, false},
		{0o440, false},
		{0o600, false},
		{0o640, false},
		{0o444, true},
		{0o404, true},
		{0o644, true},
		{0o460, true},
		{0o620, true},
		{0o660, true},
		{0o410, true},
		{0o450, true},
		{0o402, true},
		{0o401, true},
		{0o602, true},
		{0o601, true},
	}
	for _, reader := range secretReaders {
		for _, modeCase := range modes {
			t.Run(fmt.Sprintf("%s %04o", reader.name, modeCase.mode), func(t *testing.T) {
				require.NoError(t, os.Chmod(reader.path, modeCase.mode))
				err := reader.readSecret(reader.path)
				if !modeCase.wantErr {
					require.NoError(t, err)
					return
				}
				require.Error(t, err)
				assert.Contains(t, err.Error(), "group write or execute, or other users")
			})
		}
	}
}

// A Kubernetes Secret volume links token -> ..data/token -> ..<timestamp>/token. The check
// applies to the mode of the final regular file.
func TestSecretFileModeThroughKubernetesSymlinkChain(t *testing.T) {
	cases := []struct {
		name    string
		mode    os.FileMode
		wantErr bool
	}{
		{"defaultMode 0440 with fsGroup", 0o440, false},
		{"defaultMode 0400", 0o400, false},
		{"defaultMode 0444", 0o444, true},
		{"defaultMode 0644", 0o644, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mountDirectory := t.TempDir()
			timestampDirectory := filepath.Join(mountDirectory, "..2026_11_19_00_00_00.000000001")
			require.NoError(t, os.Mkdir(timestampDirectory, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(timestampDirectory, "token"), []byte("tok\n"), 0o600))
			require.NoError(t, os.Chmod(filepath.Join(timestampDirectory, "token"), tc.mode))
			require.NoError(t, os.Symlink(filepath.Base(timestampDirectory), filepath.Join(mountDirectory, "..data")))
			require.NoError(t, os.Symlink(filepath.Join("..data", "token"), filepath.Join(mountDirectory, "token")))

			token, err := readAccessToken(filepath.Join(mountDirectory, "token"))
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "group write or execute, or other users")
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, token)
		})
	}
}
