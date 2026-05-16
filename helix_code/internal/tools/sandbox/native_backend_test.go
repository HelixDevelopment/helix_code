//go:build linux

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Pure tests — Cloneflags computation
// ---------------------------------------------------------------------------

func TestNativeBackend_Kind(t *testing.T) {
	n, err := NewNativeBackend("/work")
	if err != nil {
		t.Fatalf("NewNativeBackend: %v", err)
	}
	if got := n.Kind(); got != BackendNative {
		t.Fatalf("Kind() = %v, want %v", got, BackendNative)
	}
}

func TestBuildCloneflags_NetworkAllowed_OmitsNewNet(t *testing.T) {
	p := DefaultSandboxPolicy()
	p.NetworkAllowed = true
	flags := buildCloneflags(p)
	if flags&syscall.CLONE_NEWNET != 0 {
		t.Fatalf("NetworkAllowed=true must omit CLONE_NEWNET; got flags=0x%x", flags)
	}
}

func TestBuildCloneflags_DefaultDeniesNetwork_IncludesNewNet(t *testing.T) {
	p := DefaultSandboxPolicy()
	flags := buildCloneflags(p)
	if flags&syscall.CLONE_NEWNET == 0 {
		t.Fatalf("default policy must include CLONE_NEWNET; got flags=0x%x", flags)
	}
}

func TestBuildCloneflags_AlwaysIncludesUserPidNsMnt(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    SandboxPolicy
	}{
		{"default", DefaultSandboxPolicy()},
		{"network-allowed", func() SandboxPolicy {
			p := DefaultSandboxPolicy()
			p.NetworkAllowed = true
			return p
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := buildCloneflags(tc.p)
			required := []struct {
				name string
				flag uintptr
			}{
				{"CLONE_NEWUSER", syscall.CLONE_NEWUSER},
				{"CLONE_NEWPID", syscall.CLONE_NEWPID},
				{"CLONE_NEWNS", syscall.CLONE_NEWNS},
				{"CLONE_NEWUTS", syscall.CLONE_NEWUTS},
				{"CLONE_NEWIPC", syscall.CLONE_NEWIPC},
			}
			for _, r := range required {
				if flags&r.flag == 0 {
					t.Errorf("flag %s missing; got 0x%x", r.name, flags)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Constructor wiring
// ---------------------------------------------------------------------------

func TestNewNativeBackend_PopulatesHostBinary(t *testing.T) {
	n, err := NewNativeBackend("/some/work")
	if err != nil {
		t.Fatalf("NewNativeBackend: %v", err)
	}
	if n.HostBinary == "" {
		t.Fatalf("HostBinary should be populated from os.Executable()")
	}
	exe, exeErr := os.Executable()
	if exeErr == nil && n.HostBinary != exe {
		t.Errorf("HostBinary = %q, want %q (os.Executable())", n.HostBinary, exe)
	}
	if n.WorkDir != "/some/work" {
		t.Errorf("WorkDir = %q, want %q", n.WorkDir, "/some/work")
	}
	if n.Runner == nil {
		t.Errorf("Runner should be installed by NewNativeBackend")
	}
}

// ---------------------------------------------------------------------------
// helperPayload JSON round-trip
// ---------------------------------------------------------------------------

func TestPayloadJSONRoundTrip(t *testing.T) {
	in := helperPayload{
		Command:        "echo hi",
		NetworkAllowed: true,
		WorkDir:        "/tmp/work",
		BindMounts: []BindMount{
			{Source: "/etc", Target: "/etc", ReadOnly: true},
			{Source: "/data", Target: "/data", ReadOnly: false},
		},
		MemoryLimitMB: 256,
	}
	data, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out helperPayload
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n in=%#v\nout=%#v", in, out)
	}
}

// ---------------------------------------------------------------------------
// IsHelperInvocation
// ---------------------------------------------------------------------------

func TestIsHelperInvocation_FalseWhenEnvUnset(t *testing.T) {
	t.Setenv(helperEnvVar, "")
	if IsHelperInvocation() {
		t.Fatalf("IsHelperInvocation() must be false when env unset")
	}
}

func TestIsHelperInvocation_TrueWhenEnvSet(t *testing.T) {
	t.Setenv(helperEnvVar, "1")
	if !IsHelperInvocation() {
		t.Fatalf("IsHelperInvocation() must be true when env=%q is set", helperEnvVar)
	}
}

// ---------------------------------------------------------------------------
// Run via stub Runner — verifies result.Backend, timeout enforcement,
// and that the helper payload is propagated as env on the child invocation.
// (Real namespace creation is exercised by T11's challenge harness.)
// ---------------------------------------------------------------------------

func TestRun_StubRunner_ResultBackendIsNative(t *testing.T) {
	n, err := NewNativeBackend("/work")
	if err != nil {
		t.Fatalf("NewNativeBackend: %v", err)
	}
	n.HostBinary = "/dev/null/host-binary-stub"
	n.Runner = func(ctx context.Context, name string, args []string, env []string, _ uintptr, _ []syscall.SysProcIDMap, _ []syscall.SysProcIDMap) ([]byte, []byte, int, error) {
		return []byte("ok\n"), nil, 0, nil
	}

	res, err := n.Run(context.Background(), "true", DefaultSandboxPolicy())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res == nil {
		t.Fatalf("Run returned nil result")
	}
	if res.Backend != BackendNative {
		t.Errorf("Backend = %v, want %v", res.Backend, BackendNative)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if res.Stdout != "ok\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "ok\n")
	}
}

func TestRun_StubRunner_PassesPayloadAndCloneflags(t *testing.T) {
	n, err := NewNativeBackend("/work")
	if err != nil {
		t.Fatalf("NewNativeBackend: %v", err)
	}
	n.HostBinary = "/usr/local/bin/helixcode"

	p := DefaultSandboxPolicy()
	p.NetworkAllowed = true
	p.MemoryLimitMB = 128
	p.BindMounts = []BindMount{{Source: "/data", Target: "/data", ReadOnly: false}}

	var capturedName string
	var capturedEnv []string
	var capturedFlags uintptr

	n.Runner = func(ctx context.Context, name string, args []string, env []string, flags uintptr, _ []syscall.SysProcIDMap, _ []syscall.SysProcIDMap) ([]byte, []byte, int, error) {
		capturedName = name
		capturedEnv = env
		capturedFlags = flags
		return nil, nil, 0, nil
	}

	if _, err := n.Run(context.Background(), "echo hi", p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if capturedName != n.HostBinary {
		t.Errorf("name = %q, want %q (HostBinary)", capturedName, n.HostBinary)
	}

	// Helper marker env present, helper args env contains JSON payload.
	hasMarker := false
	var argsValue string
	for _, e := range capturedEnv {
		if strings.HasPrefix(e, helperEnvVar+"=") && strings.TrimPrefix(e, helperEnvVar+"=") != "" {
			hasMarker = true
		}
		if strings.HasPrefix(e, helperArgsEnvVar+"=") {
			argsValue = strings.TrimPrefix(e, helperArgsEnvVar+"=")
		}
	}
	if !hasMarker {
		t.Errorf("env missing %s marker: %v", helperEnvVar, capturedEnv)
	}
	if argsValue == "" {
		t.Fatalf("env missing %s payload: %v", helperArgsEnvVar, capturedEnv)
	}
	var payload helperPayload
	if err := json.Unmarshal([]byte(argsValue), &payload); err != nil {
		t.Fatalf("payload not JSON: %v (raw=%q)", err, argsValue)
	}
	if payload.Command != "echo hi" {
		t.Errorf("payload.Command = %q, want %q", payload.Command, "echo hi")
	}
	if !payload.NetworkAllowed {
		t.Errorf("payload.NetworkAllowed = false, want true")
	}
	if payload.WorkDir != "/work" {
		t.Errorf("payload.WorkDir = %q, want %q", payload.WorkDir, "/work")
	}
	if payload.MemoryLimitMB != 128 {
		t.Errorf("payload.MemoryLimitMB = %d, want 128", payload.MemoryLimitMB)
	}
	if !reflect.DeepEqual(payload.BindMounts, p.BindMounts) {
		t.Errorf("payload.BindMounts = %#v, want %#v", payload.BindMounts, p.BindMounts)
	}

	// Cloneflags reflect NetworkAllowed=true (no NEWNET).
	if capturedFlags&syscall.CLONE_NEWNET != 0 {
		t.Errorf("Cloneflags should omit CLONE_NEWNET when NetworkAllowed; got 0x%x", capturedFlags)
	}
	if capturedFlags&syscall.CLONE_NEWUSER == 0 {
		t.Errorf("Cloneflags missing CLONE_NEWUSER; got 0x%x", capturedFlags)
	}
}

func TestRun_StubRunner_TimeoutEnforced(t *testing.T) {
	n, err := NewNativeBackend("/work")
	if err != nil {
		t.Fatalf("NewNativeBackend: %v", err)
	}
	n.HostBinary = "/dev/null/host-binary-stub"

	// Stub runner that respects ctx — blocks until ctx is done, then
	// returns a deadline-exceeded-ish synthetic result.
	n.Runner = func(ctx context.Context, name string, args []string, env []string, _ uintptr, _ []syscall.SysProcIDMap, _ []syscall.SysProcIDMap) ([]byte, []byte, int, error) {
		<-ctx.Done()
		return nil, []byte("killed"), -1, ctx.Err()
	}

	p := DefaultSandboxPolicy()
	p.Timeout = 50 * time.Millisecond

	start := time.Now()
	res, err := n.Run(context.Background(), "sleep 60", p)
	dur := time.Since(start)

	if dur > 2*time.Second {
		t.Fatalf("Run took too long under timeout: %v", dur)
	}
	if res == nil {
		t.Fatalf("result must be non-nil even on timeout")
	}
	if !res.TimedOut {
		t.Errorf("TimedOut = false, want true (err=%v)", err)
	}
	if res.Backend != BackendNative {
		t.Errorf("Backend = %v, want %v", res.Backend, BackendNative)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		// Stub returns ctx.Err(); we expect DeadlineExceeded.
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestRun_EmptyHostBinary_Errors(t *testing.T) {
	n := &NativeBackend{WorkDir: "/work"}
	_, err := n.Run(context.Background(), "true", DefaultSandboxPolicy())
	if err == nil {
		t.Fatalf("Run with empty HostBinary must error")
	}
}

func TestRun_NilRunner_Errors(t *testing.T) {
	n := &NativeBackend{HostBinary: "/some/bin", WorkDir: "/work"}
	_, err := n.Run(context.Background(), "true", DefaultSandboxPolicy())
	if err == nil {
		t.Fatalf("Run with nil Runner must error")
	}
}
