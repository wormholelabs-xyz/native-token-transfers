package main

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
)

// ----------------------------------------------------------------------
// parseFlags
// ----------------------------------------------------------------------

func TestParseFlags_Defaults(t *testing.T) {
	opts, err := parseFlags(nil)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:7599", opts.listen)
	require.Equal(t, "guardian-governance", opts.participant)
	require.Equal(t, "sandbox", opts.profileName)
	require.Equal(t, "", opts.topologyConfig)
	require.Equal(t, "", opts.dar)
	require.Equal(t, "", opts.dpmPath)
	require.Equal(t, "", opts.accessTokenFile)
	require.Equal(t, "", opts.acsParty)
	require.False(t, opts.verbose)
}

func TestParseFlags_UnknownFlag_Errors(t *testing.T) {
	_, err := parseFlags([]string{"--nope"})
	require.Error(t, err)
}

// ----------------------------------------------------------------------
// buildService: sandbox posture (no ACS)
// ----------------------------------------------------------------------

func TestBuildService_Sandbox_NoACS(t *testing.T) {
	opts, err := parseFlags(nil)
	require.NoError(t, err)

	svc, _, err := buildService(opts)
	require.NoError(t, err)
	require.Nil(t, svc.ACS)

	srv := httpTestServer(t, svc)
	resp, err := http.Get(srv + "/v1/disclosures?template=" + disclosure.DefaultDisclose()[0].Template)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// ----------------------------------------------------------------------
// buildService: localnet wires ACS from the access-token-file contents
// ----------------------------------------------------------------------

func TestBuildService_LocalNet_WiresACS(t *testing.T) {
	tokenFile := writeTempFile(t, "tok")
	opts, err := parseFlags([]string{
		"--profile", "localnet",
		"--participant", "guardian-governance",
		"--acs-party", "gg::x",
		"--access-token-file", tokenFile,
	})
	require.NoError(t, err)

	svc, prof, err := buildService(opts)
	require.NoError(t, err)
	require.NotNil(t, svc.ACS)

	ep, err := prof.Endpoint("guardian-governance")
	require.NoError(t, err)
	require.NotEmpty(t, ep.JSONAPIBaseURL)

	require.Equal(t, ep.JSONAPIBaseURL, svc.ACS.BaseURL)
	require.Equal(t, "gg::x", svc.ACS.Party)
	require.Equal(t, "tok", svc.ACS.Token)
}

func TestBuildService_LocalNet_EmptyAccessTokenFile_Errors(t *testing.T) {
	opts, err := parseFlags([]string{
		"--profile", "localnet",
		"--participant", "guardian-governance",
		"--acs-party", "gg::x",
	})
	require.NoError(t, err)

	_, _, err = buildService(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--access-token-file")
}

// ----------------------------------------------------------------------
// buildService: topology-config wiring
// ----------------------------------------------------------------------

func TestBuildService_TopologyConfig_Explicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topology.json")
	const contents = `{"disclose": [{"template": "Foo:Bar", "fetchAs": "Operator"}]}`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	opts, err := parseFlags([]string{"--topology-config", path})
	require.NoError(t, err)

	svc, _, err := buildService(opts)
	require.NoError(t, err)
	require.Equal(t, []string{"Foo:Bar"}, svc.Cfg.Templates())
}

func TestBuildService_TopologyConfig_Missing_UsesDefaults(t *testing.T) {
	opts, err := parseFlags(nil)
	require.NoError(t, err)

	svc, _, err := buildService(opts)
	require.NoError(t, err)

	want := make([]string, len(disclosure.DefaultDisclose()))
	for i, e := range disclosure.DefaultDisclose() {
		want[i] = e.Template
	}
	require.Equal(t, want, svc.Cfg.Templates())
}

// ----------------------------------------------------------------------
// resolvedCantonDir / resolvedDarPath: canton-dir walk-up
// ----------------------------------------------------------------------

func TestResolvedDarPath_WalksUpToMultiPackageYaml(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "multi-package.yaml"), []byte(""), 0o600))

	darDir := filepath.Join(root, "test", ".daml", "dist")
	require.NoError(t, os.MkdirAll(darDir, 0o755))
	darPath := filepath.Join(darDir, "ntt-test-0.1.0.dar")
	require.NoError(t, os.WriteFile(darPath, []byte("dar"), 0o600))

	nested := filepath.Join(root, "testing", "cli", "cmd", "disclosure-service")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	got, err := resolvedDarPath("")
	require.NoError(t, err)
	require.Equal(t, darPath, got)
}

func TestResolvedDarPath_MissingDar_ActionableError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "multi-package.yaml"), []byte(""), 0o600))
	t.Chdir(root)

	_, err := resolvedDarPath("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dpm build --all")
}

func TestResolvedDarPath_NoMultiPackageYaml_Errors(t *testing.T) {
	t.Chdir(t.TempDir())

	_, err := resolvedDarPath("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "multi-package.yaml")
}

// ----------------------------------------------------------------------
// isLoopbackListenAddr
// ----------------------------------------------------------------------

func TestIsLoopbackListenAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:7599", true},
		{"localhost:7599", true},
		{"[::1]:7599", true},
		{"0.0.0.0:7599", false},
		{"192.168.1.5:7599", false},
		{":7599", false}, // empty host binds every interface -- not loopback
	}
	for _, c := range cases {
		require.Equal(t, c.want, isLoopbackListenAddr(c.addr), "addr=%s", c.addr)
	}
}

// ----------------------------------------------------------------------
// run: listen, banner, healthz, graceful shutdown
// ----------------------------------------------------------------------

func TestRun_ListensServesHealthzAndShutsDown(t *testing.T) {
	opts, err := parseFlags([]string{"--listen", "127.0.0.1:0"})
	require.NoError(t, err)

	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- run(ctx, opts, stderrW) }()

	addr := readListenAddr(t, stderrR)

	resp, err := http.Get("http://" + addr + "/v1/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	require.Contains(t, string(body[:n]), "guardian-governance")

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after context cancellation")
	}
	_ = stderrW.Close()
}

// readListenAddr scans stderr for the "disclosure serve: listening on http://" marker line and
// returns the host:port that follows. Bounded so a missing/garbled banner fails fast rather than
// hanging the test.
func readListenAddr(t *testing.T, r *os.File) string {
	t.Helper()
	const marker = "disclosure serve: listening on http://"
	lineCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, marker) {
				lineCh <- line
				return
			}
		}
	}()
	select {
	case line := <-lineCh:
		return strings.TrimPrefix(strings.TrimSpace(line), marker)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the listening banner line")
		return ""
	}
}

// ----------------------------------------------------------------------
// test helpers
// ----------------------------------------------------------------------

func writeTempFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

// httpTestServer starts an httptest server over svc.Handler() and returns its base URL.
func httpTestServer(t *testing.T, svc interface{ Handler() http.Handler }) string {
	t.Helper()
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)
	return srv.URL
}
