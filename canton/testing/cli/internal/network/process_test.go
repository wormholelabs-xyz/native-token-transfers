package network

import (
	"syscall"
	"testing"
	"time"
)

// TestNewDetachedCommand_KillProcessGroup proves a command started via newDetachedCommand
// runs in its own process group and that killProcessGroup actually terminates it -- the
// exact mechanism SandboxManager.Down relies on.
func TestNewDetachedCommand_KillProcessGroup(t *testing.T) {
	cmd := newDetachedCommand("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid

	// The process must be alive immediately after Start.
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("expected process %d to be alive: %v", pid, err)
	}

	if err := killProcessGroup(pid); err != nil {
		t.Fatalf("killProcessGroup: %v", err)
	}

	// Reap it so it doesn't linger as a zombie, and give the kill a moment to land.
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("process %d did not exit after killProcessGroup", pid)
	}
}
