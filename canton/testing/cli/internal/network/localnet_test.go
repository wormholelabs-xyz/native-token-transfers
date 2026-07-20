package network

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ----------------------------------------------------------------------
// LocalNetManager: pure/local helpers (no docker/network required)
// ----------------------------------------------------------------------

func TestLocalNetManager_ValidatorBaseURL(t *testing.T) {
	var m LocalNetManager
	if got := m.validatorBaseURL(); got != "http://localhost:3903" {
		t.Fatalf("default validatorBaseURL: got %q", got)
	}
	m.ValidatorBaseURL = "http://custom:9999"
	if got := m.validatorBaseURL(); got != "http://custom:9999" {
		t.Fatalf("override validatorBaseURL: got %q", got)
	}
}

func TestLocalNetManager_LedgerAddr(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("LOCALNET_LEDGER_HOST", "")
		t.Setenv("LOCALNET_LEDGER_PORT", "")
		var m LocalNetManager
		if got := m.ledgerAddr(); got != "localhost:3901" {
			t.Fatalf("default ledgerAddr: got %q", got)
		}
	})
	t.Run("struct fields override", func(t *testing.T) {
		m := LocalNetManager{LedgerHost: "canton.example", LedgerPort: 4444}
		if got := m.ledgerAddr(); got != "canton.example:4444" {
			t.Fatalf("struct-field ledgerAddr: got %q", got)
		}
	})
	t.Run("env overrides when fields are zero", func(t *testing.T) {
		t.Setenv("LOCALNET_LEDGER_HOST", "env-host")
		t.Setenv("LOCALNET_LEDGER_PORT", "5555")
		var m LocalNetManager
		if got := m.ledgerAddr(); got != "env-host:5555" {
			t.Fatalf("env-override ledgerAddr: got %q", got)
		}
	})
	t.Run("malformed env port is ignored", func(t *testing.T) {
		t.Setenv("LOCALNET_LEDGER_HOST", "")
		t.Setenv("LOCALNET_LEDGER_PORT", "not-a-port")
		var m LocalNetManager
		if got := m.ledgerAddr(); got != "localhost:3901" {
			t.Fatalf("malformed-env-port ledgerAddr: got %q", got)
		}
	})
}

func TestLocalNetManager_ComposeArgs(t *testing.T) {
	m := LocalNetManager{ComposeDir: "/tmp/compose-dir"}
	args := m.composeArgs("up", "-d")
	joined := strings.Join(args, " ")
	for _, want := range []string{"compose", "--profile", "sv", "--profile", "app-provider", "up", "-d", "/tmp/compose-dir"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("composeArgs missing %q: %v", want, args)
		}
	}
}

func TestLocalNetManager_Env(t *testing.T) {
	m := LocalNetManager{}
	base := len(m.env())
	m.ImageTag = "v1.2.3"
	env := m.env()
	if len(env) != base+1 {
		t.Fatalf("expected exactly one extra entry when ImageTag is set, got %d vs base %d", len(env), base)
	}
	found := false
	for _, e := range env {
		if e == "IMAGE_TAG=v1.2.3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("IMAGE_TAG not present in env: %v", env)
	}
}

func TestEnsurePostgresOverride_WritesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	m := LocalNetManager{ComposeDir: dir}
	if err := m.ensurePostgresOverride(); err != nil {
		t.Fatalf("ensurePostgresOverride: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, postgresOverrideFile))
	if err != nil {
		t.Fatalf("read override: %v", err)
	}
	if string(got) != string(postgresOverride) {
		t.Fatalf("override contents mismatch")
	}
}

func TestEnsurePostgresOverride_UpToDateSkipsRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, postgresOverrideFile)
	if err := os.WriteFile(path, postgresOverride, 0o644); err != nil {
		t.Fatalf("seed override: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	before := info.ModTime()
	time.Sleep(10 * time.Millisecond)

	m := LocalNetManager{ComposeDir: dir}
	if err := m.ensurePostgresOverride(); err != nil {
		t.Fatalf("ensurePostgresOverride: %v", err)
	}
	info2, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !info2.ModTime().Equal(before) {
		t.Fatalf("file was rewritten even though contents already matched")
	}
}

func TestEnsurePostgresOverride_StaleContentIsRewritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, postgresOverrideFile)
	if err := os.WriteFile(path, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("seed stale override: %v", err)
	}
	m := LocalNetManager{ComposeDir: dir}
	if err := m.ensurePostgresOverride(); err != nil {
		t.Fatalf("ensurePostgresOverride: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read override: %v", err)
	}
	if string(got) != string(postgresOverride) {
		t.Fatalf("stale override was not refreshed")
	}
}

// ----------------------------------------------------------------------
// probeGET / probeTCP
// ----------------------------------------------------------------------

func TestProbeGET(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	if !probeGET(context.Background(), client, srv.URL+"/ok", "tok") {
		t.Fatalf("expected probeGET to report true for a 2xx response")
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("expected bearer token to be set when provided, got %q", gotAuth)
	}
	if probeGET(context.Background(), client, srv.URL+"/fail", "") {
		t.Fatalf("expected probeGET to report false for a 500 response")
	}
	if probeGET(context.Background(), client, "http://127.0.0.1:1/unreachable", "") {
		t.Fatalf("expected probeGET to report false when the connection is refused")
	}
	if probeGET(context.Background(), client, "http://\x7f invalid", "") {
		t.Fatalf("expected probeGET to report false for an unconstructable request")
	}
}

func TestProbeTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	if !probeTCP(context.Background(), ln.Addr().String(), time.Second) {
		t.Fatalf("expected probeTCP to succeed against a listening port")
	}
	if probeTCP(context.Background(), "127.0.0.1:1", 200*time.Millisecond) {
		t.Fatalf("expected probeTCP to fail against a closed port")
	}
}

// ----------------------------------------------------------------------
// MintUnsafeToken / WriteTokenFile
// ----------------------------------------------------------------------

func TestMintUnsafeToken(t *testing.T) {
	tok, err := MintUnsafeToken("app-provider", time.Hour)
	if err != nil {
		t.Fatalf("MintUnsafeToken: %v", err)
	}
	parsed, err := jwt.Parse(tok, func(*jwt.Token) (any, error) { return []byte(localNetUnsafeSecret), nil })
	if err != nil || !parsed.Valid {
		t.Fatalf("token did not verify against the unsafe secret: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("unexpected claims type %T", parsed.Claims)
	}
	if claims["sub"] != "app-provider" {
		t.Fatalf("sub claim mismatch: %+v", claims)
	}
	if claims["aud"] != LocalNetAudience {
		t.Fatalf("aud claim mismatch: %+v", claims)
	}
}

func TestWriteTokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.jwt")
	if err := WriteTokenFile(path, "ledger-api-user", time.Hour); err != nil {
		t.Fatalf("WriteTokenFile: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("token file is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token file perm: got %v, want 0600", info.Mode().Perm())
	}
}
