//go:build windows

package hooks

import (
	"os/exec"
)

// setProcessGroup is a no-op on Windows. Windows does not support Unix-style
// process groups via SysProcAttr.Setpgid. Process tree cleanup on timeout is
// handled by killProcessGroup calling cmd.Process.Kill(), which terminates the
// direct child; exec.CommandContext's built-in cancellation handles the rest.
func setProcessGroup(_ *exec.Cmd) {}

// killProcessGroup kills the direct child process on Windows. Unlike Unix,
// there is no portable way to kill an entire process tree via a single syscall
// without importing golang.org/x/sys/windows. For the hook-runner use-case
// (short-lived scripts) terminating the direct process is sufficient; any
// orphaned grandchildren will be reparented and eventually cleaned up by the OS.
func killProcessGroup(cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
}
