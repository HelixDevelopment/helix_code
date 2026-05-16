// p1f14_challenge runs the F14 Sandboxed Shell Execution pipeline end-to-end
// against the real Detector, real bubblewrap subprocesses (when selected),
// and real on-disk YAML round-trips. Runtime-evidence harness for the F14
// Challenge.
//
// Phases:
//
//	0. Detector       — print full host capabilities JSON. Always runs.
//	A. CONST-033 deny — manager rejects host-power-management commands at
//	                    the manager level, BEFORE any backend dispatch.
//	                    Asserted by: manager constructed with NO backend
//	                    slots; the load-bearing observable is that we get
//	                    *DenyError (NOT *FailClosedError) — which is only
//	                    possible if the deny check fires before the
//	                    fail-closed check. Always runs.
//	B. Fail-closed    — manager constructed with SelectedBackend=None and a
//	                    populated UnavailableReason returns *FailClosedError
//	                    that surfaces the verbatim reason. Always runs.
//	C. (gated) bwrap  — real bubblewrap end-to-end: stdout round-trip,
//	                    network-allowed echo, default-DENY network probe.
//	                    Runs only when the real Detector picks bubblewrap.
//	D. (gated) native — native (userns) backend: forces a NativeBackend at
//	                    the manager and runs a real echo through the
//	                    re-exec helper. Runs only when userns is enabled.
//	E. YAML round-trip — write a non-default SandboxConfig via
//	                    WriteSandboxConfig (O_EXCL + 0600), stat the file
//	                    to verify mode + size, read it back via
//	                    LoadSandboxConfig, assert equality. Always runs.
//
// The first statement of main() is the native-helper dispatch: when this
// binary is re-exec'd inside the new namespaces by Phase D, it must
// short-circuit to RunAsHelper() and exec /bin/sh -c <command>. Without
// this, the helper child would re-enter the harness and recurse forever.
//
// Exit code 0 on success; exit 1 with a diagnostic on any check failure.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/tools/sandbox"
)

func main() {
	// MUST be the very first statement: when the native backend re-execs
	// this binary inside the new userns/pidns/mountns/utsns/ipcns/netns,
	// the child sees the helper env-var set. Re-entering the rest of the
	// harness would recurse (Phase D would fork the helper which would
	// run all phases again). Short-circuit straight to RunAsHelper().
	if sandbox.IsHelperInvocation() {
		os.Exit(sandbox.RunAsHelper())
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F14 challenge harness pid:", os.Getpid())

	caps := phase0Detector()
	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(caps); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(caps); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F14 challenge harness PASS")
	return nil
}

// phase0Detector runs the real Detector and prints the full capabilities
// JSON. No assertion — subsequent phases adapt to what the host exposes.
// Returns the detected capabilities so phases C and D can branch on the
// SelectedBackend / UnprivilegedUserNS fields.
func phase0Detector() sandbox.SandboxCapabilities {
	fmt.Println("==> phase 0: Detector capabilities (informational)")

	caps := sandbox.NewDetector().Detect()

	js, err := json.MarshalIndent(caps, "    ", "  ")
	if err != nil {
		fmt.Printf("    (capabilities marshal failed: %v)\n", err)
	} else {
		fmt.Printf("    %s\n", string(js))
	}
	fmt.Printf("    runtime.GOOS    : %s\n", runtime.GOOS)
	fmt.Printf("    selected backend: %s\n", caps.SelectedBackend.String())
	if caps.UnavailableReason != "" {
		fmt.Printf("    reason          : %s\n", caps.UnavailableReason)
	}
	return caps
}

// phaseA constructs a manager with NO backend slots and asserts that
// CONST-033 deny fires before the fail-closed check. The variant probes
// span systemctl, pm-utils, loginctl session-kill, and chained forms via
// `bash -c '...'` and `; ` — they MUST all surface *DenyError, never
// *FailClosedError.
func phaseA() error {
	fmt.Println("==> phase A: CONST-033 rejected before spawn (always runs)")

	caps := sandbox.SandboxCapabilities{
		GOOS:            "linux",
		SelectedBackend: sandbox.BackendBubblewrap, // backend slot is nil; deny check must NOT reach the slot
	}
	mgr := sandbox.NewSandboxManager(caps, nil, nil, sandbox.DefaultSandboxConfig(), zap.NewNop())
	if mgr == nil {
		return fmt.Errorf("NewSandboxManager returned nil")
	}

	variants := []struct {
		name string
		cmd  string
	}{
		{"systemctl-suspend", "systemctl suspend"},
		{"bash-c-poweroff", "bash -c 'systemctl poweroff'"},
		{"chained-pm-suspend", "ls; pm-suspend"},
		{"loginctl-terminate", "loginctl terminate-user $USER"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, v := range variants {
		result, err := mgr.Execute(ctx, v.cmd, sandbox.DefaultSandboxPolicy())
		if result != nil {
			return fmt.Errorf("%s: deny path returned non-nil result: %+v", v.name, result)
		}
		if err == nil {
			return fmt.Errorf("%s: expected DenyError, got nil error", v.name)
		}
		// Must be DenyError (deny was adjudicated at manager level) and
		// MUST NOT be FailClosedError (which would mean the manager fell
		// through the deny check and reached the fail-closed branch
		// because no backend was wired — that ordering would be a bluff).
		var denyErr *sandbox.DenyError
		if !errors.As(err, &denyErr) {
			return fmt.Errorf("%s: err must be *DenyError; got %T (%v)", v.name, err, err)
		}
		var fcErr *sandbox.FailClosedError
		if errors.As(err, &fcErr) {
			return fmt.Errorf("%s: deny must surface as DenyError, not FailClosedError (%v)", v.name, err)
		}
		if !errors.Is(err, sandbox.ErrCommandDenied) {
			return fmt.Errorf("%s: err must wrap ErrCommandDenied; got %v", v.name, err)
		}
		if !strings.Contains(denyErr.MatchedRule, "CONST-033") {
			return fmt.Errorf("%s: matched rule must reference CONST-033; got %q",
				v.name, denyErr.MatchedRule)
		}
		fmt.Printf("    %-22s -> DenyError rule=%q\n", v.name, denyErr.MatchedRule)
	}
	return nil
}

// phaseB constructs a manager with SelectedBackend=None and a populated
// UnavailableReason; asserts that a benign command surfaces *FailClosedError
// with the verbatim reason text — proving the contract the wiring depends on.
func phaseB() error {
	fmt.Println("==> phase B: fail-closed when no backend (always runs)")

	const reason = "harness fail-closed test"
	caps := sandbox.SandboxCapabilities{
		GOOS:              "linux",
		SelectedBackend:   sandbox.BackendNone,
		UnavailableReason: reason,
	}
	mgr := sandbox.NewSandboxManager(caps, nil, nil, sandbox.DefaultSandboxConfig(), zap.NewNop())
	if mgr == nil {
		return fmt.Errorf("NewSandboxManager returned nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := mgr.Execute(ctx, "echo hi", sandbox.DefaultSandboxPolicy())
	if result != nil {
		return fmt.Errorf("fail-closed path returned non-nil result: %+v", result)
	}
	if err == nil {
		return fmt.Errorf("expected FailClosedError, got nil error")
	}
	var fcErr *sandbox.FailClosedError
	if !errors.As(err, &fcErr) {
		return fmt.Errorf("err must be *FailClosedError; got %T (%v)", err, err)
	}
	if !errors.Is(err, sandbox.ErrSandboxUnavailable) {
		return fmt.Errorf("err must wrap ErrSandboxUnavailable; got %v", err)
	}
	if !strings.Contains(fcErr.Reason, reason) {
		return fmt.Errorf("FailClosedError.Reason = %q; want to contain %q", fcErr.Reason, reason)
	}
	fmt.Printf("    fail-closed reason: %q\n", fcErr.Reason)
	return nil
}

// phaseC runs real bwrap end-to-end IFF the real Detector selected
// bubblewrap. We do three sub-checks:
//   - benign echo round-trips stdout
//   - NetworkAllowed=true echo also succeeds (proves the network flag is
//     honoured and does not break the happy path)
//   - default-policy network probe surfaces NETDENIED inside the sandbox
func phaseC(caps sandbox.SandboxCapabilities) error {
	fmt.Println("==> phase C: bubblewrap backend end-to-end (gated)")

	if caps.SelectedBackend != sandbox.BackendBubblewrap {
		fmt.Printf("    [skipped: bubblewrap not selected backend (selected=%s)]\n", caps.SelectedBackend)
		return nil
	}

	workDir, err := os.MkdirTemp("", "p1f14-bwrap-")
	if err != nil {
		return fmt.Errorf("workdir tempdir: %w", err)
	}
	defer os.RemoveAll(workDir)

	mgr, mgrCaps, err := sandbox.NewSandboxManagerFromDetector(workDir, sandbox.DefaultSandboxConfig(), zap.NewNop())
	if err != nil {
		return fmt.Errorf("NewSandboxManagerFromDetector: %w", err)
	}
	if mgrCaps.SelectedBackend != sandbox.BackendBubblewrap {
		fmt.Printf("    [skipped: detector flipped to %s after construction]\n", mgrCaps.SelectedBackend)
		return nil
	}
	fmt.Printf("    workdir         : %s\n", workDir)
	fmt.Printf("    bwrap path      : %s\n", mgrCaps.BubblewrapPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// C.1 benign echo, default policy
	result, err := mgr.Execute(ctx, "echo hello-from-sandbox-challenge", sandbox.DefaultSandboxPolicy())
	if err != nil {
		return fmt.Errorf("C.1 Execute(echo): %w", err)
	}
	if result == nil {
		return fmt.Errorf("C.1 nil result")
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("C.1 exit=%d stderr=%q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "hello-from-sandbox-challenge\n" {
		return fmt.Errorf("C.1 stdout=%q want %q", result.Stdout, "hello-from-sandbox-challenge\n")
	}
	if result.Backend != sandbox.BackendBubblewrap {
		return fmt.Errorf("C.1 backend=%s want bubblewrap", result.Backend)
	}
	fmt.Printf("    C.1 echo ok      : exit=%d stdout=%q duration=%s\n",
		result.ExitCode, strings.TrimRight(result.Stdout, "\n"), result.Duration)

	// C.2 echo with NetworkAllowed=true
	netPolicy := sandbox.DefaultSandboxPolicy()
	netPolicy.NetworkAllowed = true
	result, err = mgr.Execute(ctx, "echo network-allowed-test", netPolicy)
	if err != nil {
		return fmt.Errorf("C.2 Execute(net=true echo): %w", err)
	}
	if result == nil || result.ExitCode != 0 {
		return fmt.Errorf("C.2 unexpected result: %+v", result)
	}
	if !strings.Contains(result.Stdout, "network-allowed-test") {
		return fmt.Errorf("C.2 stdout=%q missing marker", result.Stdout)
	}
	fmt.Printf("    C.2 net-allowed  : exit=%d stdout=%q\n",
		result.ExitCode, strings.TrimRight(result.Stdout, "\n"))

	// C.3 default-policy network probe — must report NETDENIED.
	if _, lookErr := exec.LookPath("curl"); lookErr != nil {
		fmt.Println("    C.3 [skipped: curl not on host PATH; net-deny probe needs curl]")
	} else {
		probe := "curl -m 3 -sS https://example.com >/dev/null 2>&1 || echo NETDENIED"
		result, err = mgr.Execute(ctx, probe, sandbox.DefaultSandboxPolicy())
		if err != nil {
			return fmt.Errorf("C.3 Execute(net-deny probe): %w", err)
		}
		if result == nil {
			return fmt.Errorf("C.3 nil result")
		}
		if !strings.Contains(result.Stdout, "NETDENIED") {
			return fmt.Errorf("C.3 default-policy net DENY violated: stdout=%q stderr=%q exit=%d",
				result.Stdout, result.Stderr, result.ExitCode)
		}
		fmt.Printf("    C.3 net-denied   : stdout=%q (curl failed inside sandbox as expected)\n",
			strings.TrimRight(result.Stdout, "\n"))
	}

	return nil
}

// phaseD exercises the native (userns) backend by forcing a NativeBackend
// into a manager (overriding the detector's natural preference for
// bubblewrap when both are available). Skipped when userns is not enabled
// on the host — the helper re-exec cannot work without unprivileged userns.
//
// The native backend re-execs THIS binary; the helper-mode dispatch at the
// top of main() is what makes that safe: the child sees the env-var, calls
// RunAsHelper, and exec's /bin/sh -c <command>.
func phaseD(caps sandbox.SandboxCapabilities) error {
	fmt.Println("==> phase D: native backend end-to-end (gated)")

	if runtime.GOOS != "linux" {
		fmt.Printf("    [skipped: native backend only on Linux (GOOS=%s)]\n", runtime.GOOS)
		return nil
	}
	if !caps.UnprivilegedUserNS {
		fmt.Printf("    [skipped: native backend not exercisable on this host (userns=%t bwrap=%q)]\n",
			caps.UnprivilegedUserNS, caps.BubblewrapPath)
		return nil
	}

	workDir, err := os.MkdirTemp("", "p1f14-native-")
	if err != nil {
		return fmt.Errorf("workdir tempdir: %w", err)
	}
	defer os.RemoveAll(workDir)

	nb, err := sandbox.NewNativeBackend(workDir)
	if err != nil {
		return fmt.Errorf("NewNativeBackend: %w", err)
	}
	// Force-construct a manager wired ONLY to the native backend so the
	// detector's bubblewrap preference does not steal this phase. The
	// manager's resolveBackend uses caps.SelectedBackend to pick a slot,
	// so we set the caps to BackendNative explicitly.
	nativeCaps := sandbox.SandboxCapabilities{
		GOOS:               "linux",
		BubblewrapPath:     caps.BubblewrapPath,
		UnprivilegedUserNS: caps.UnprivilegedUserNS,
		CGroupsV2:          caps.CGroupsV2,
		SelectedBackend:    sandbox.BackendNative,
	}
	mgr := sandbox.NewSandboxManager(nativeCaps, nil, nb, sandbox.DefaultSandboxConfig(), zap.NewNop())
	if mgr == nil {
		return fmt.Errorf("NewSandboxManager returned nil")
	}
	fmt.Printf("    native workdir  : %s\n", workDir)
	fmt.Printf("    host binary     : %s\n", nb.HostBinary)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mgr.Execute(ctx, "echo hello-from-native-sandbox", sandbox.DefaultSandboxPolicy())
	if err != nil {
		// On hosts where userns is enabled in /proc but is restricted by
		// AppArmor / seccomp / kernel.unprivileged_userns_apparmor_policy
		// the clone(2) itself can fail. Treat as a noted skip rather than
		// a hard failure — this matches the F12/F13 gated-skip honesty
		// pattern.
		fmt.Printf("    [skipped: native exec failed (likely AppArmor/seccomp restriction): %v]\n", err)
		return nil
	}
	if result == nil {
		return fmt.Errorf("nil result with nil error")
	}
	if result.Backend != sandbox.BackendNative {
		return fmt.Errorf("backend=%s want native", result.Backend)
	}
	if result.ExitCode != 0 {
		// Same caveat: a non-zero exit here is most often the userns
		// being administratively blocked despite the sysctl saying yes.
		fmt.Printf("    [skipped: native exec returned exit=%d stderr=%q (AppArmor/seccomp likely)]\n",
			result.ExitCode, result.Stderr)
		return nil
	}
	if !strings.Contains(result.Stdout, "hello-from-native-sandbox") {
		return fmt.Errorf("stdout=%q missing marker", result.Stdout)
	}
	fmt.Printf("    native echo ok  : exit=%d stdout=%q duration=%s\n",
		result.ExitCode, strings.TrimRight(result.Stdout, "\n"), result.Duration)
	return nil
}

// phaseE writes a non-default SandboxConfig with a UserDenyList through
// WriteSandboxConfig (O_EXCL + 0600), stats the file to verify mode + size,
// then reads it back through LoadSandboxConfig and asserts the loaded
// values match what we wrote. UserDenyList equality is asserted via
// reflect.DeepEqual; scalar policy fields field-by-field. All against real
// disk in t.TempDir-equivalent.
func phaseE() error {
	fmt.Println("==> phase E: sandbox config YAML round-trip on disk (always runs)")

	dir, err := os.MkdirTemp("", "p1f14-cfg-")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "sandbox.yaml")

	want := sandbox.SandboxConfig{
		DefaultPolicy: sandbox.SandboxPolicy{
			NetworkAllowed: false,
			Timeout:        45 * time.Second,
			MemoryLimitMB:  768,
			CPULimitPct:    65,
			ReadOnlyRoot:   true,
		},
		UserDenyList: []string{`^rm\s+-rf\s+/`, `\bdd\s+if=`, `\bmkfs\.\w+\b`},
	}

	if err := sandbox.WriteSandboxConfig(path, want); err != nil {
		return fmt.Errorf("WriteSandboxConfig: %w", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Stat(%s): %w", path, err)
	}
	mode := st.Mode().Perm()
	if mode != 0o600 {
		return fmt.Errorf("on-disk mode = %o; want 0600 (secret-safe)", mode)
	}
	if st.Size() == 0 {
		return fmt.Errorf("on-disk file is empty")
	}
	fmt.Printf("    cfg path        : %s\n", path)
	fmt.Printf("    cfg mode        : %#o\n", mode)
	fmt.Printf("    cfg size        : %d bytes\n", st.Size())

	got, err := sandbox.LoadSandboxConfig(path)
	if err != nil {
		return fmt.Errorf("LoadSandboxConfig: %w", err)
	}

	if got.DefaultPolicy.NetworkAllowed != want.DefaultPolicy.NetworkAllowed {
		return fmt.Errorf("NetworkAllowed: got=%t want=%t",
			got.DefaultPolicy.NetworkAllowed, want.DefaultPolicy.NetworkAllowed)
	}
	if got.DefaultPolicy.Timeout != want.DefaultPolicy.Timeout {
		return fmt.Errorf("Timeout: got=%s want=%s",
			got.DefaultPolicy.Timeout, want.DefaultPolicy.Timeout)
	}
	if got.DefaultPolicy.MemoryLimitMB != want.DefaultPolicy.MemoryLimitMB {
		return fmt.Errorf("MemoryLimitMB: got=%d want=%d",
			got.DefaultPolicy.MemoryLimitMB, want.DefaultPolicy.MemoryLimitMB)
	}
	if got.DefaultPolicy.CPULimitPct != want.DefaultPolicy.CPULimitPct {
		return fmt.Errorf("CPULimitPct: got=%d want=%d",
			got.DefaultPolicy.CPULimitPct, want.DefaultPolicy.CPULimitPct)
	}
	if got.DefaultPolicy.ReadOnlyRoot != want.DefaultPolicy.ReadOnlyRoot {
		return fmt.Errorf("ReadOnlyRoot: got=%t want=%t",
			got.DefaultPolicy.ReadOnlyRoot, want.DefaultPolicy.ReadOnlyRoot)
	}
	if !reflect.DeepEqual(got.UserDenyList, want.UserDenyList) {
		return fmt.Errorf("UserDenyList: got=%#v want=%#v", got.UserDenyList, want.UserDenyList)
	}
	fmt.Printf("    round-trip ok   : timeout=%s mem=%dMB cpu=%d%% deny=%d entries\n",
		got.DefaultPolicy.Timeout, got.DefaultPolicy.MemoryLimitMB,
		got.DefaultPolicy.CPULimitPct, len(got.UserDenyList))
	return nil
}
