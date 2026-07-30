package network

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeFakeDpm writes an executable shell script standing in for the real `dpm` binary,
// returning its path. body is the script's shell source (after the shebang line).
func writeFakeDpm(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "dpm")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write fake dpm: %v", err)
	}
	return path
}

// ----------------------------------------------------------------------
// SandboxManager.Status
// ----------------------------------------------------------------------

func TestSandboxManager_Status_NoPidFile(t *testing.T) {
	m := SandboxManager{RunDir: t.TempDir()}
	running, info, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if running {
		t.Fatalf("expected not-running with no pid file, got %+v", info)
	}
}

func TestSandboxManager_Status_MalformedPidFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sandbox.pid"), []byte("not-a-pid"), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}
	m := SandboxManager{RunDir: dir}
	_, _, err := m.Status()
	if err == nil || !strings.Contains(err.Error(), "parse pid file") {
		t.Fatalf("expected a parse-pid-file error, got %v", err)
	}
}

func TestSandboxManager_Status_StalePidNoLongerRunning(t *testing.T) {
	dir := t.TempDir()
	// A pid that is syntactically valid but (overwhelmingly likely) refers to no live
	// process on this host.
	if err := os.WriteFile(filepath.Join(dir, "sandbox.pid"), []byte("999999"), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}
	m := SandboxManager{RunDir: dir}
	running, _, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if running {
		t.Fatalf("expected a stale pid to report not-running")
	}
}

func TestSandboxManager_Status_RunningWithPortFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sandbox.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sandbox.port"), []byte("7777"), 0o600); err != nil {
		t.Fatalf("seed port file: %v", err)
	}
	m := SandboxManager{RunDir: dir}
	running, info, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !running {
		t.Fatalf("expected running=true for the current process's own pid")
	}
	if info.Port != 7777 {
		t.Fatalf("port mismatch: got %d", info.Port)
	}
	if info.PID != os.Getpid() {
		t.Fatalf("pid mismatch: got %d", info.PID)
	}
}

func TestSandboxManager_Status_RunningWithoutPortFileDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sandbox.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}
	m := SandboxManager{RunDir: dir}
	running, info, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !running || info.Port != 6865 {
		t.Fatalf("expected default port 6865 when no port file exists, got running=%v info=%+v", running, info)
	}
}

// ----------------------------------------------------------------------
// SandboxManager.Down
// ----------------------------------------------------------------------

func TestSandboxManager_Down_NotRunning(t *testing.T) {
	m := SandboxManager{RunDir: t.TempDir()}
	if err := m.Down(); err != nil {
		t.Fatalf("Down on a never-started manager should be a no-op, got %v", err)
	}
}

func TestSandboxManager_Down_KillsRunningProcess(t *testing.T) {
	dir := t.TempDir()
	cmd := newDetachedCommand("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start fixture process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	if err := os.WriteFile(filepath.Join(dir, "sandbox.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}

	m := SandboxManager{RunDir: dir}
	if err := m.Down(); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sandbox.pid")); !os.IsNotExist(err) {
		t.Fatalf("expected the pid file to be removed after Down")
	}

	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("fixture process was not killed by Down")
	}
}

// ----------------------------------------------------------------------
// SandboxManager.Up
// ----------------------------------------------------------------------

func TestSandboxManager_Up_ReusesRunningInstance(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sandbox.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sandbox.port"), []byte("6900"), 0o600); err != nil {
		t.Fatalf("seed port file: %v", err)
	}
	m := SandboxManager{DpmPath: "dpm-must-not-be-invoked", RunDir: dir}
	info, err := m.Up(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if info.PID != os.Getpid() || info.Port != 6900 {
		t.Fatalf("expected Up to reuse the already-running instance, got %+v", info)
	}
}

func TestSandboxManager_Up_StartFailure(t *testing.T) {
	m := SandboxManager{DpmPath: filepath.Join(t.TempDir(), "does-not-exist"), RunDir: t.TempDir()}
	_, err := m.Up(context.Background(), time.Second)
	if err == nil || !strings.Contains(err.Error(), "start dpm sandbox") {
		t.Fatalf("expected a start-failure error, got %v", err)
	}
}

func TestSandboxManager_Up_NeverBecomesReady(t *testing.T) {
	dpm := writeFakeDpm(t, "sleep 30\n") // never prints SandboxReadyLogLine
	dir := t.TempDir()
	m := SandboxManager{DpmPath: dpm, RunDir: dir}

	_, err := m.Up(context.Background(), 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "sandbox never became ready") {
		t.Fatalf("expected a never-became-ready error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "sandbox.pid")); !os.IsNotExist(statErr) {
		t.Fatalf("expected the pid file to be cleaned up after a failed Up")
	}
}

func TestSandboxManager_Up_BecomesReady(t *testing.T) {
	dpm := writeFakeDpm(t, "echo \""+SandboxReadyLogLine+"\"\nsleep 30\n")
	dir := t.TempDir()
	m := SandboxManager{DpmPath: dpm, RunDir: dir, Port: 6901}

	info, err := m.Up(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if info.Port != 6901 {
		t.Fatalf("port mismatch: got %d", info.Port)
	}
	t.Cleanup(func() { _ = killProcessGroup(info.PID) })

	running, _, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !running {
		t.Fatalf("expected the sandbox to be reported running after a successful Up")
	}
}

// ----------------------------------------------------------------------
// waitForLogLine
// ----------------------------------------------------------------------

func TestWaitForLogLine_FindsExistingLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(path, []byte("some preamble\n"+SandboxReadyLogLine+"\n"), 0o600); err != nil {
		t.Fatalf("seed log: %v", err)
	}
	if err := waitForLogLine(context.Background(), path, SandboxReadyLogLine, time.Second); err != nil {
		t.Fatalf("waitForLogLine: %v", err)
	}
}

func TestWaitForLogLine_TimesOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(path, []byte("never matches\n"), 0o600); err != nil {
		t.Fatalf("seed log: %v", err)
	}
	err := waitForLogLine(context.Background(), path, "not-present", 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout error, got %v", err)
	}
}

func TestWaitForLogLine_CtxCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(path, []byte("never matches\n"), 0o600); err != nil {
		t.Fatalf("seed log: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForLogLine(ctx, path, "not-present", time.Minute)
	if err == nil {
		t.Fatalf("expected ctx.Err() when ctx is already cancelled")
	}
}

func TestWaitForLogLine_MissingFileIsToleratedUntilItAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log-not-yet-created")
	done := make(chan error, 1)
	go func() {
		done <- waitForLogLine(context.Background(), path, "ready", 5*time.Second)
	}()
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, []byte("ready\n"), 0o600); err != nil {
		t.Fatalf("write log after the fact: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waitForLogLine: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("waitForLogLine did not notice the log file appearing")
	}
}
