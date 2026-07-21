// LocalNet backend: Splice LocalNet docker-compose lifecycle, readiness polling, unsafe
// shared-secret JWT minting, and DAR upload via the JSON Ledger API v2. The values below come
// from the Splice LocalNet release notes; see the CLI README's LocalNet section for the exact
// release they were checked against:
//
//   - app-provider participant: gRPC Ledger API 3901, JSON Ledger API v2 3975, validator API
//     3903.
//   - Auth: HS256, shared secret "unsafe", aud "https://canton.network.global". Ledger
//     admin/operator user id "ledger-api-user"; wallet user id "app-provider".
//   - Readiness: GET :3903/api/validator/readyz, then GET
//     :3903/api/validator/v0/scan-proxy/dso-party-id (also yields the DSO party id).
package network

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// postgresOverride renames the bundle's hardcoded `container_name: postgres` and moves
// the published host port off 5432 (defaults: splice-localnet-postgres / 15432, both
// overridable via LOCALNET_POSTGRES_CONTAINER_NAME / LOCALNET_POSTGRES_HOST_PORT), so
// LocalNet coexists with a locally running PostgreSQL and with other stopped containers
// that claimed the "postgres" name. Materialized into ComposeDir so every compose
// invocation (up/down/status) sees the same config.
//
//go:embed localnet_postgres_override.yaml
var postgresOverride []byte

// postgresOverrideFile is the override's filename inside ComposeDir.
const postgresOverrideFile = "wormhole-postgres-override.yaml"

// LocalNetAudience is the unsafe shared-secret JWT audience Splice LocalNet expects.
const LocalNetAudience = "https://canton.network.global"

// localNetUnsafeSecret is the well-known, publicly documented devnet-only HMAC secret
// Splice LocalNet's unsafe auth mode validates against. Never used against a real network.
const localNetUnsafeSecret = "unsafe"

// LocalNetAdminUser is the participant admin / Ledger API user id (also the topology admin).
const LocalNetAdminUser = "ledger-api-user"

// LocalNetWalletUser is the app-provider wallet user id (tap/transfer-factory calls).
const LocalNetWalletUser = "app-provider"

// LocalNetManager drives one Splice LocalNet docker-compose stack, rooted at ComposeDir
// (the extracted `<ver>_splice-node.tar.gz`'s splice-node/docker-compose/localnet).
type LocalNetManager struct {
	ComposeDir       string
	ImageTag         string
	ValidatorBaseURL string // defaults to http://localhost:3903 if empty
	LedgerHost       string // defaults to "localhost" if empty -- the gRPC Ledger API `dpm script` connects to
	LedgerPort       int    // defaults to 3901 if zero

	// Logf, when non-nil, narrates the compose lifecycle (up/down/readiness, plus a
	// per-service listing after up). The caller-supplied closure owns any prefix; nil (the
	// default) disables narration.
	Logf func(format string, args ...any)
}

// logf forwards to Logf when set -- nil-safe, so instrumentation never needs a guard.
func (m *LocalNetManager) logf(format string, args ...any) {
	if m.Logf != nil {
		m.Logf(format, args...)
	}
}

func (m *LocalNetManager) validatorBaseURL() string {
	if m.ValidatorBaseURL != "" {
		return m.ValidatorBaseURL
	}
	return "http://localhost:3903"
}

// ledgerAddr is the gRPC Ledger API address `dpm script` actually connects to (port 3901,
// distinct from the validator's HTTP API on 3903) -- readiness must gate on THIS being
// reachable, not just the validator's HTTP endpoints, since a script call can still fail with
// a transient gRPC connection error even after readyz/scan-proxy report healthy (observed
// under host resource pressure).
func (m *LocalNetManager) ledgerAddr() string {
	host := m.LedgerHost
	if host == "" {
		if v := os.Getenv("LOCALNET_LEDGER_HOST"); v != "" {
			host = v
		} else {
			host = "localhost"
		}
	}
	port := m.LedgerPort
	if port == 0 {
		port = 3901
		if v := os.Getenv("LOCALNET_LEDGER_PORT"); v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				port = p
			}
		}
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func (m *LocalNetManager) composeArgs(rest ...string) []string {
	args := []string{
		"compose",
		"--env-file", filepath.Join(m.ComposeDir, "compose.env"),
		"--env-file", filepath.Join(m.ComposeDir, "env", "common.env"),
		"-f", filepath.Join(m.ComposeDir, "compose.yaml"),
		"-f", filepath.Join(m.ComposeDir, "resource-constraints.yaml"),
		"-f", filepath.Join(m.ComposeDir, postgresOverrideFile),
		"--profile", "sv",
		"--profile", "app-provider",
	}
	return append(args, rest...)
}

// ensurePostgresOverride writes the embedded postgres override into ComposeDir (or
// refreshes it if stale) so composeArgs' -f reference always resolves.
func (m *LocalNetManager) ensurePostgresOverride() error {
	path := filepath.Join(m.ComposeDir, postgresOverrideFile)
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, postgresOverride) {
		m.logf("localnet: postgres override up-to-date (%s)", path)
		return nil
	}
	if err := os.WriteFile(path, postgresOverride, 0o644); err != nil {
		return fmt.Errorf("network: writing postgres override: %w", err)
	}
	m.logf("localnet: postgres override written (%s)", path)
	return nil
}

func (m *LocalNetManager) env() []string {
	env := os.Environ()
	if m.ImageTag != "" {
		env = append(env, "IMAGE_TAG="+m.ImageTag)
	}
	return env
}

// Up brings the smallest useful LocalNet topology up (sv + app-provider profiles,
// APP_USER_PROFILE=off per compose.env's default) and waits for docker compose's own
// healthchecks (--wait), then polls the validator readyz endpoint as a belt-and-braces
// check. First boot bootstraps the DSO (the network's Decentralized Synchronizer Operator
// party) and is documented at 2-6 minutes, so the timeout should be generous.
func (m *LocalNetManager) Up(ctx context.Context, timeout time.Duration) error {
	tag := m.ImageTag
	if tag == "" {
		tag = "default"
	}
	m.logf("localnet: compose dir=%s image-tag=%s profiles=[sv app-provider]", m.ComposeDir, tag)
	if err := m.ensurePostgresOverride(); err != nil {
		return err
	}
	m.logf("localnet: docker compose up -d --wait (first boot's DSO bootstrap takes 2-6 minutes)")
	start := time.Now()
	args := m.composeArgs("up", "-d", "--wait")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = m.env()
	cmd.Dir = m.ComposeDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("network: docker compose up failed: %w\n%s", err, out)
	}
	m.logf("localnet: docker compose up done (%s)", time.Since(start).Round(time.Second))
	if m.Logf != nil {
		m.logComposeServices(ctx)
	}
	return m.waitReady(ctx, timeout)
}

// logComposeServices narrates one line per compose service (name, state, health) after an
// up -- the "what actually got spun up" answer for verbose runs. Only invoked when Logf is
// set: it costs an extra docker invocation and is purely informational, so failures are
// logged and swallowed rather than propagated.
func (m *LocalNetManager) logComposeServices(ctx context.Context) {
	args := m.composeArgs("ps", "--format", "{{.Service}} {{.State}} {{.Health}}")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = m.env()
	cmd.Dir = m.ComposeDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		m.logf("localnet: docker compose ps failed (informational only): %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			m.logf("localnet: service %s", line)
		}
	}
}

// Down tears the stack down and wipes state (parties/DARs/ledger data) with `down -v`, so
// the next Up starts from a clean genesis.
func (m *LocalNetManager) Down(ctx context.Context) error {
	if err := m.ensurePostgresOverride(); err != nil {
		return err
	}
	m.logf("localnet: docker compose down -v (wipes parties/DARs/ledger state)")
	args := m.composeArgs("down", "-v")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = m.env()
	cmd.Dir = m.ComposeDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("network: docker compose down failed: %w\n%s", err, out)
	}
	return nil
}

// ComposeService is the subset of one `docker compose ps --format json` row the CLI
// reports: service name, run state, and health ("" for services without a healthcheck).
type ComposeService struct {
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
}

// Status lists the stack's compose services via `docker compose ps`. Compose >= 2.21 emits
// one JSON object per line (NDJSON); older versions emit a single array -- both are handled,
// since the stack only requires >= 2.24 for the override's `!override` tag, not an exact
// version. An empty result means the stack is down (`ps` lists nothing after `down`).
func (m *LocalNetManager) Status(ctx context.Context) ([]ComposeService, error) {
	if err := m.ensurePostgresOverride(); err != nil {
		return nil, err
	}
	m.logf("localnet: docker compose ps --format json (listing running services)")
	args := m.composeArgs("ps", "--format", "json")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = m.env()
	cmd.Dir = m.ComposeDir
	// Keep stderr out of stdout -- compose logs progress noise there, which would corrupt
	// the JSON parse below.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("network: docker compose ps failed: %w\n%s", err, stderr.Bytes())
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var services []ComposeService
		if err := json.Unmarshal(trimmed, &services); err != nil {
			return nil, fmt.Errorf("network: parse compose ps output: %w", err)
		}
		return services, nil
	}
	var services []ComposeService
	for _, line := range bytes.Split(trimmed, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var svc ComposeService
		if err := json.Unmarshal(line, &svc); err != nil {
			return nil, fmt.Errorf("network: parse compose ps line %q: %w", line, err)
		}
		services = append(services, svc)
	}
	return services, nil
}

// readyStabilityStreak is how many CONSECUTIVE successful poll iterations (all three probes:
// readyz, scan-proxy, and the ledger gRPC port) are required before waitReady declares the
// stack ready. One-shot success is not enough: a participant can accept a TCP connection or
// answer one HTTP request immediately after starting up and still drop the next gRPC call a
// few seconds later while it finishes warming up -- exactly the failure mode observed live
// (a `dpm script` call failing with a transient connection reset shortly after `network up`
// returned). Requiring a short stable window catches that before the suite starts submitting.
const readyStabilityStreak = 3

func (m *LocalNetManager) waitReady(ctx context.Context, timeout time.Duration) error {
	// readyz is unauthenticated, but the v0 scan-proxy endpoint rejects bare requests with
	// a 401 (verified against a live 0.6.12 stack), so the second probe must carry an
	// admin bearer token -- without it this loop spins until the timeout even though the
	// validator is fully ready.
	token, err := MintUnsafeToken(LocalNetAdminUser, timeout+time.Hour)
	if err != nil {
		return err
	}
	ledgerAddr := m.ledgerAddr()
	m.logf("localnet: polling readyz + scan-proxy (validator %s) + ledger gRPC port (%s), requiring %d consecutive clean checks",
		m.validatorBaseURL(), ledgerAddr, readyStabilityStreak)
	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(timeout)
	streak := 0
	for {
		ok := probeGET(ctx, client, m.validatorBaseURL()+"/api/validator/readyz", "") &&
			probeGET(ctx, client, m.validatorBaseURL()+"/api/validator/v0/scan-proxy/dso-party-id", token) &&
			probeTCP(ctx, ledgerAddr, 5*time.Second)
		if ok {
			streak++
			m.logf("localnet: readiness check %d/%d clean", streak, readyStabilityStreak)
			if streak >= readyStabilityStreak {
				m.logf("localnet: validator ready (%s); participant endpoints: ledger-gRPC :3901, JSON-API :3975, validator :3903", time.Since(start).Round(time.Second))
				return nil
			}
		} else {
			streak = 0
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("network: LocalNet validator API never became stably ready within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// probeGET reports whether url answers 2xx; a non-empty bearerToken is sent as the
// Authorization header (the validator's v0 API requires one, readyz does not).
func probeGET(ctx context.Context, client *http.Client, url, bearerToken string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// probeTCP reports whether addr accepts a TCP connection within timeout -- the gRPC Ledger
// API `dpm script` actually dials (port 3901), distinct from the validator's HTTP API (3903)
// the other two probes check. A successful HTTP readyz/scan-proxy response does not imply
// this port is equally ready; both must hold before the suite starts submitting scripts.
func probeTCP(ctx context.Context, addr string, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// DSOPartyID fetches the DSO party id from the validator's scan-proxy. Amulet instrument ids
// need it, and the lookup is part of the LocalNet backend contract even where the current CLI
// does not use it.
func (m *LocalNetManager) DSOPartyID(ctx context.Context) (string, error) {
	token, err := MintUnsafeToken(LocalNetAdminUser, time.Hour)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.validatorBaseURL()+"/api/validator/v0/scan-proxy/dso-party-id", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("network: dso-party-id request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("network: dso-party-id: HTTP %d: %s", resp.StatusCode, body)
	}
	var out struct {
		DsoPartyID string `json:"dso_party_id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("network: parse dso-party-id response: %w", err)
	}
	return out.DsoPartyID, nil
}

// MintUnsafeToken mints an HS256 JWT for sub (LocalNetAdminUser or LocalNetWalletUser),
// signed with LocalNet's publicly documented "unsafe" shared secret -- devnet-only auth,
// never valid against a real participant.
func MintUnsafeToken(sub string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": sub,
		"aud": LocalNetAudience,
		"exp": time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(localNetUnsafeSecret))
	if err != nil {
		return "", fmt.Errorf("network: sign LocalNet token: %w", err)
	}
	return signed, nil
}

// WriteTokenFile mints a token for sub and writes it to path, for `dpm script
// --access-token-file`.
func WriteTokenFile(path, sub string, ttl time.Duration) error {
	token, err := MintUnsafeToken(sub, ttl)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0o600)
}

// UploadDAR uploads darPath's raw bytes to the JSON Ledger API v2 (:3975/v2/packages) using
// an admin-scoped bearer token. This is the LocalNet-specific alternative to `dpm script
// --upload-dar true` for cases where a DAR needs to be vetted before any script runs (e.g.
// priming the ledger before the first `dpm script` call); ledger.Runner still passes
// --upload-dar true on every script call as the primary path.
func (m *LocalNetManager) UploadDAR(ctx context.Context, jsonLedgerAPIBaseURL, darPath, adminToken string) error {
	raw, err := os.ReadFile(darPath)
	if err != nil {
		return fmt.Errorf("network: read DAR %s: %w", darPath, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jsonLedgerAPIBaseURL+"/v2/packages", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("network: upload DAR request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("network: upload DAR: HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}

// CreateLedgerUser creates userID on the JSON Ledger API v2 (POST /v2/users) with the given
// readAs/actAs party rights, treating an already-exists response as success (idempotent, the
// same convention as GrantLedgerAPIUserRights). Used for the observer's "guardian-watcher"
// reader user, which must carry ONLY CanReadAs(guardianObserver) -- proving the stream
// observation works with observer rights alone, never actAs.
func CreateLedgerUser(ctx context.Context, jsonLedgerAPIBaseURL, adminToken, userID string, readAsParties, actAsParties []string) error {
	// Same `{"kind": {"CanReadAs": {"value": {"party": ...}}}}` wrapper convention already
	// proven for CanActAs (verified against 0.6.12; see GrantLedgerAPIUserRights).
	type right struct {
		Kind map[string]map[string]map[string]string `json:"kind"`
	}
	var rights []right
	for _, party := range readAsParties {
		rights = append(rights, right{Kind: map[string]map[string]map[string]string{
			"CanReadAs": {"value": {"party": party}},
		}})
	}
	for _, party := range actAsParties {
		rights = append(rights, right{Kind: map[string]map[string]map[string]string{
			"CanActAs": {"value": {"party": party}},
		}})
	}
	payload, err := json.Marshal(map[string]any{
		"user": map[string]any{
			"id":            userID,
			"isDeactivated": false,
		},
		"rights": rights,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jsonLedgerAPIBaseURL+"/v2/users", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("network: create ledger user %s request: %w", userID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return nil // already exists -- idempotent, matching GrantLedgerAPIUserRights' convention
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		// The user creation itself may already-exist under a different error shape (some
		// participants report ALREADY_EXISTS as 400/409 with a message rather than a bare
		// 409); treat that case as success too, and everything else as a real failure.
		if resp.StatusCode == http.StatusBadRequest && bytes.Contains(bytes.ToLower(body), []byte("already exists")) {
			return nil
		}
		return fmt.Errorf("network: create ledger user %s: HTTP %d: %s", userID, resp.StatusCode, body)
	}
	return nil
}

// GrantLedgerAPIUserRights grants actAs/readAs rights on parties to LocalNetAdminUser. It
// covers the case where daml-script running as ledger-api-user does not automatically get
// actAs for parties it allocates itself. jsonLedgerAPIBaseURL is the same host:3975 UploadDAR
// uses.
func GrantLedgerAPIUserRights(ctx context.Context, jsonLedgerAPIBaseURL, adminToken string, actAsParties []string) error {
	// The v2 OpenAPI encodes the rights oneOf with a `value` wrapper:
	// {"kind": {"CanActAs": {"value": {"party": ...}}}} -- omitting it yields
	// HTTP 400 "Missing required field at 'value'" (verified against 0.6.12).
	type right struct {
		Kind struct {
			CanActAs struct {
				Value struct {
					Party string `json:"party"`
				} `json:"value"`
			} `json:"CanActAs"`
		} `json:"kind"`
	}
	rights := make([]right, len(actAsParties))
	for i, party := range actAsParties {
		rights[i].Kind.CanActAs.Value.Party = party
	}
	// The user must be named in the body as well as the path ("does not match user in
	// body" otherwise -- verified against 0.6.12).
	payload, err := json.Marshal(map[string]any{"userId": LocalNetAdminUser, "rights": rights})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v2/users/%s/rights", jsonLedgerAPIBaseURL, LocalNetAdminUser)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("network: grant rights request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("network: grant rights: HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}
