//go:build !linux

package shell

import (
	"fmt"
	"os/exec"
	"runtime"
)

// applyNetworkNamespace is the non-Linux counterpart of the Linux
// CLONE_NEWNET implementation (see sandbox_linux.go).
//
// §11.4.81(C) honest kernel-gap: Linux network-namespace isolation has no
// portable equivalent. macOS/XNU and the BSDs expose no unprivileged
// per-process network-namespace primitive comparable to CLONE_NEWNET, and
// Windows job objects do not provide network isolation either. Rather than
// silently run a process the caller asked to be network-isolated, the
// strict NetworkNone mode returns a clear error so callers can decide
// whether to refuse or degrade. The less-strict modes (NetworkHost,
// NetworkFull) share the host namespace exactly as on Linux, so they
// succeed without any platform-specific action.
func applyNetworkNamespace(cmd *exec.Cmd, mode NetworkMode) error {
	if mode == NetworkNone {
		return fmt.Errorf("network isolation (NetworkNone) is only supported on Linux; current OS: %s", runtime.GOOS)
	}
	return nil
}
