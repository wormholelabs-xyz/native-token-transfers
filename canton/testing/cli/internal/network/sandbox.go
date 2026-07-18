// Package network manages the two backends `ntt-playground network up|down|status` can
// drive: a bare `dpm sandbox` (this file) and a Splice LocalNet docker-compose stack
// (localnet.go). Both run detached from the CLI process -- `network up` starts it in the
// background and returns; `network down`/`status` are separate invocations that find the
// running instance again via a small run-state file. This mirrors the lifecycle
// canton/testing/go's integration test manages in-process (via t.Cleanup), just spread
// across independent CLI invocations instead of one Go test's lifetime.
package network

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SandboxReadyLogLine is the log line canton/testing/go/ntt_recipient_match_integration_test.go
// polls for; dpm sandbox writes it once the ledger API is serving.
const SandboxReadyLogLine = "Canton sandbox is ready"

// SandboxInfo is what's persisted in the run-state file between `network up` and later
// `network down`/`status`/any command needing the port.
type SandboxInfo struct {
	PID     int    `json:"pid"`
	Port    int    `json:"port"`
	LogPath string `json:"logPath"`
}

// SandboxManager starts/stops/checks a `dpm sandbox` process, tracked via a PID file in
// RunDir (created if it doesn't exist).
type SandboxManager struct {
	DpmPath string
	RunDir  string
	Port    int // 0 means let dpm sandbox pick its default (6865)

	// Logf, when non-nil, narrates the sandbox lifecycle (reuse/start/wait/kill). The
	// caller-supplied closure owns any prefix; nil (the default) disables narration.
	Logf func(format string, args ...any)
}

// logf forwards to Logf when set -- nil-safe, so instrumentation never needs a guard.
func (m *SandboxManager) logf(format string, args ...any) {
	if m.Logf != nil {
		m.Logf(format, args...)
	}
}

func (m *SandboxManager) pidFile() string  { return filepath.Join(m.RunDir, "sandbox.pid") }
func (m *SandboxManager) logFile() string  { return filepath.Join(m.RunDir, "sandbox.log") }
func (m *SandboxManager) portFile() string { return filepath.Join(m.RunDir, "sandbox.port") }

// Status reports whether a sandbox this manager started is still alive.
func (m *SandboxManager) Status() (running bool, info SandboxInfo, err error) {
	pidRaw, err := os.ReadFile(m.pidFile())
	if errors.Is(err, os.ErrNotExist) {
		return false, SandboxInfo{}, nil
	}
	if err != nil {
		return false, SandboxInfo{}, fmt.Errorf("network: read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidRaw)))
	if err != nil {
		return false, SandboxInfo{}, fmt.Errorf("network: parse pid file: %w", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, SandboxInfo{}, nil
	}
	// On Unix, FindProcess always succeeds; signal 0 checks liveness without side effects.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, SandboxInfo{}, nil
	}
	port := 6865
	if portRaw, err := os.ReadFile(m.portFile()); err == nil {
		if p, err := strconv.Atoi(strings.TrimSpace(string(portRaw))); err == nil {
			port = p
		}
	}
	return true, SandboxInfo{PID: pid, Port: port, LogPath: m.logFile()}, nil
}

// Up starts `dpm sandbox --no-tty` in the background (its own process group, detached from
// this CLI invocation) and blocks until its log reports readiness or timeout elapses. If a
// sandbox this manager started is already running, it is reused.
func (m *SandboxManager) Up(ctx context.Context, timeout time.Duration) (SandboxInfo, error) {
	if running, info, err := m.Status(); err != nil {
		return SandboxInfo{}, err
	} else if running {
		m.logf("sandbox: reusing running instance pid=%d port=%d", info.PID, info.Port)
		return info, nil
	}

	if err := os.MkdirAll(m.RunDir, 0o755); err != nil {
		return SandboxInfo{}, fmt.Errorf("network: create run dir: %w", err)
	}

	port := m.Port
	if port == 0 {
		port = 6865
	}

	logPath := m.logFile()
	logFile, err := os.Create(logPath)
	if err != nil {
		return SandboxInfo{}, fmt.Errorf("network: create sandbox log: %w", err)
	}
	defer logFile.Close()

	cmd := newDetachedCommand(m.DpmPath, "sandbox", "--no-tty")
	cmd.Dir = m.RunDir // dpm itself writes a log/canton.log relative to its cwd; keep it contained here
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), fmt.Sprintf("CANTON_SANDBOX_PORT=%d", port))

	m.logf("sandbox: starting dpm sandbox --no-tty port=%d log=%s", port, logPath)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return SandboxInfo{}, fmt.Errorf("network: start dpm sandbox: %w", err)
	}
	if err := os.WriteFile(m.pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		return SandboxInfo{}, fmt.Errorf("network: write pid file: %w", err)
	}
	if err := os.WriteFile(m.portFile(), []byte(strconv.Itoa(port)), 0o600); err != nil {
		return SandboxInfo{}, fmt.Errorf("network: write port file: %w", err)
	}

	m.logf("sandbox: waiting for %q", SandboxReadyLogLine)
	if err := waitForLogLine(ctx, logPath, SandboxReadyLogLine, timeout); err != nil {
		_ = killProcessGroup(cmd.Process.Pid)
		_ = os.Remove(m.pidFile())
		return SandboxInfo{}, fmt.Errorf("network: sandbox never became ready: %w", err)
	}
	m.logf("sandbox: ready (%s)", time.Since(start).Round(time.Second))

	return SandboxInfo{PID: cmd.Process.Pid, Port: port, LogPath: logPath}, nil
}

// Down kills the sandbox process group (if running) and clears the run-state file.
func (m *SandboxManager) Down() error {
	running, info, err := m.Status()
	if err != nil {
		return err
	}
	if !running {
		return nil
	}
	m.logf("sandbox: killing process group pid=%d", info.PID)
	if err := killProcessGroup(info.PID); err != nil {
		return fmt.Errorf("network: stop sandbox: %w", err)
	}
	_ = os.Remove(m.pidFile())
	return nil
}

// waitForLogLine polls path for a line containing needle until it appears or timeout elapses.
func waitForLogLine(ctx context.Context, path, needle string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), needle) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %q in %s", timeout, needle, path)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
