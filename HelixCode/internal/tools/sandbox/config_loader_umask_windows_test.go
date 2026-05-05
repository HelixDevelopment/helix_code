//go:build windows

package sandbox

// syscallUmask is a no-op on Windows: the host POSIX permission model
// does not apply, and the only test that uses it is gated behind a
// runtime.GOOS == "windows" t.Skip.
func syscallUmask(mask int) int { return 0 }
