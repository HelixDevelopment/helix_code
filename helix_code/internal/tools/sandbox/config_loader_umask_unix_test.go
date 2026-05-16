//go:build !windows

package sandbox

import "syscall"

// syscallUmask is a thin wrapper around syscall.Umask used by the
// permissive-umask test in config_loader_test.go. It is split into a
// build-tagged helper so the test file compiles on Windows (where
// syscall.Umask is absent) without needing per-test runtime guards in
// every assertion.
func syscallUmask(mask int) int {
	return syscall.Umask(mask)
}
