//go:build unix

package hooks

import (
	"os/exec"
	"syscall"
)

// setProcessGroup places the child process in its own process group so that
// killProcessGroup can send a signal to the entire group (script + all
// grandchildren such as nested `sleep` calls).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the process group whose PGID equals the
// child's PID. The negative PID passed to syscall.Kill targets every process
// in the group, not just the direct child.
func killProcessGroup(cmd *exec.Cmd) {
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
