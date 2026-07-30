package network

import (
	"os/exec"
	"syscall"
)

// newDetachedCommand builds a command in its own process group (Setpgid), so it survives
// the launching CLI invocation exiting -- matching the SysProcAttr the existing
// canton/testing/go integration test uses to manage `dpm sandbox` (there, killed via
// t.Cleanup within one process's lifetime; here, killed later by a separate `network down`
// invocation via killProcessGroup).
func newDetachedCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// killProcessGroup sends SIGKILL to the process group led by pid.
func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
