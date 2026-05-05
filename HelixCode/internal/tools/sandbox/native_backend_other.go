//go:build !linux

package sandbox

import (
	"context"
	"fmt"
)

// NativeBackend on non-Linux is a fail-closed stub. The Linux primitives
// (CLONE_NEWUSER + friends, /proc remount, RLIMIT_AS) have no portable
// equivalents on macOS / Windows; the spec defers macOS Seatbelt and
// Windows Job Object integration to F14.5.
//
// All methods compile cleanly on every GOOS so that cross-builds of the
// rest of the codebase succeed; Run always returns a clear error so any
// caller that bypasses the detector gets an honest failure rather than a
// silent no-op.
type NativeBackend struct {
	WorkDir string
}

// NewNativeBackend returns a stub backend. The error slot is reserved for
// future use (e.g. macOS-specific permission probes); v1 always succeeds.
func NewNativeBackend(workDir string) (*NativeBackend, error) {
	return &NativeBackend{WorkDir: workDir}, nil
}

// Kind returns BackendNative — the stub still identifies as the native
// backend so /sandbox status surfaces the right label.
func (n *NativeBackend) Kind() BackendKind { return BackendNative }

// Run always errors on non-Linux: there is no native sandbox available.
// Callers should check the detector's SandboxCapabilities first; this
// error is the last-line-of-defence if they don't.
func (n *NativeBackend) Run(ctx context.Context, command string, policy SandboxPolicy) (*SandboxResult, error) {
	return nil, fmt.Errorf("native sandbox backend unavailable on non-Linux (deferred to F14.5)")
}

// IsHelperInvocation always returns false on non-Linux: there is no
// re-exec path, so main.go will never dispatch into RunAsHelper here.
func IsHelperInvocation() bool { return false }

// RunAsHelper is a no-op stub on non-Linux. Returns 0 so that, if
// somehow invoked, it does not poison the parent's exit status.
func RunAsHelper() (exitCode int) { return 0 }
