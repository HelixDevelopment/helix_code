//go:build linux

package shell

import (
	"os/exec"
	"syscall"
)

// applyNetworkNamespace applies Linux network-namespace isolation to cmd
// based on mode.
//
//   - NetworkNone creates a fresh network namespace via CLONE_NEWNET; the
//     new namespace has only a loopback interface (lo) which is DOWN by
//     default, so the process is fully isolated from all networking.
//   - NetworkHost and NetworkFull share the parent's network namespace, so
//     no clone flag is added.
//
// This is the Linux half of the §11.4.81 cross-platform split; the
// non-Linux counterpart lives in sandbox_nonlinux.go.
func applyNetworkNamespace(cmd *exec.Cmd, mode NetworkMode) error {
	switch mode {
	case NetworkNone:
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	case NetworkHost, NetworkFull:
		// Share the parent network namespace — no additional flags.
	}
	return nil
}
