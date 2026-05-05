package sandbox

import (
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// findArgPair returns true when argv contains the two consecutive tokens
// (a, b) anywhere — useful for asserting `--ro-bind / /` etc.
func findArgPair(argv []string, a, b string) bool {
	for i := 0; i < len(argv)-1; i++ {
		if argv[i] == a && argv[i+1] == b {
			return true
		}
	}
	return false
}

// findArgTriple returns true when argv contains three consecutive tokens
// (a, b, c) anywhere — used for `--ro-bind <src> <tgt>` etc.
func findArgTriple(argv []string, a, b, c string) bool {
	for i := 0; i < len(argv)-2; i++ {
		if argv[i] == a && argv[i+1] == b && argv[i+2] == c {
			return true
		}
	}
	return false
}

func containsArg(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}

func TestBubblewrapBackend_Kind(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	if got := b.Kind(); got != BackendBubblewrap {
		t.Fatalf("Kind() = %v, want %v", got, BackendBubblewrap)
	}
}

func TestBuildArgv_DefaultPolicyDeniesNetwork(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(DefaultSandboxPolicy(), "echo hi")
	if !containsArg(argv, "--unshare-net") {
		t.Fatalf("default policy must deny network: --unshare-net missing in %v", argv)
	}
	if containsArg(argv, "--share-net") {
		t.Fatalf("default policy must NOT --share-net: %v", argv)
	}
}

func TestBuildArgv_NetworkAllowed(t *testing.T) {
	p := DefaultSandboxPolicy()
	p.NetworkAllowed = true
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(p, "echo hi")
	if !containsArg(argv, "--share-net") {
		t.Fatalf("NetworkAllowed=true must use --share-net: %v", argv)
	}
	if containsArg(argv, "--unshare-net") {
		t.Fatalf("NetworkAllowed=true must NOT --unshare-net: %v", argv)
	}
}

func TestBuildArgv_HasUnshareFlags(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(DefaultSandboxPolicy(), "true")
	for _, want := range []string{"--unshare-pid", "--unshare-ipc", "--unshare-uts", "--unshare-cgroup", "--unshare-user"} {
		if !containsArg(argv, want) {
			t.Errorf("missing %s in argv: %v", want, argv)
		}
	}
}

func TestBuildArgv_HasROBindRoot(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(DefaultSandboxPolicy(), "true")
	if !findArgTriple(argv, "--ro-bind", "/", "/") {
		t.Fatalf("missing `--ro-bind / /` in argv: %v", argv)
	}
}

func TestBuildArgv_HasWorkdirBindAndChdir(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/tmp/work")
	argv := b.BuildArgv(DefaultSandboxPolicy(), "true")
	if !findArgTriple(argv, "--bind", "/tmp/work", "/tmp/work") {
		t.Errorf("missing `--bind /tmp/work /tmp/work` in argv: %v", argv)
	}
	if !findArgPair(argv, "--chdir", "/tmp/work") {
		t.Errorf("missing `--chdir /tmp/work` in argv: %v", argv)
	}
}

func TestBuildArgv_HasProcDevTmpfs(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(DefaultSandboxPolicy(), "true")
	if !findArgPair(argv, "--proc", "/proc") {
		t.Errorf("missing `--proc /proc` in argv: %v", argv)
	}
	if !findArgPair(argv, "--dev", "/dev") {
		t.Errorf("missing `--dev /dev` in argv: %v", argv)
	}
	if !findArgPair(argv, "--tmpfs", "/tmp") {
		t.Errorf("missing `--tmpfs /tmp` in argv: %v", argv)
	}
}

func TestBuildArgv_DieWithParentAndNewSession(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(DefaultSandboxPolicy(), "true")
	if !containsArg(argv, "--die-with-parent") {
		t.Errorf("missing --die-with-parent in argv: %v", argv)
	}
	if !containsArg(argv, "--new-session") {
		t.Errorf("missing --new-session in argv: %v", argv)
	}
}

func TestBuildArgv_BindMounts_ReadWrite(t *testing.T) {
	p := DefaultSandboxPolicy()
	p.BindMounts = []BindMount{{Source: "/host/data", Target: "/sb/data", ReadOnly: false}}
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(p, "true")
	if !findArgTriple(argv, "--bind", "/host/data", "/sb/data") {
		t.Fatalf("missing rw bind mount in argv: %v", argv)
	}
}

func TestBuildArgv_BindMounts_ReadOnly(t *testing.T) {
	p := DefaultSandboxPolicy()
	p.BindMounts = []BindMount{{Source: "/host/ro", Target: "/sb/ro", ReadOnly: true}}
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	argv := b.BuildArgv(p, "true")
	if !findArgTriple(argv, "--ro-bind", "/host/ro", "/sb/ro") {
		t.Fatalf("missing ro bind mount in argv: %v", argv)
	}
}

func TestBuildArgv_PassesShellInvocation(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	cmd := "echo hi && pwd"
	argv := b.BuildArgv(DefaultSandboxPolicy(), cmd)
	if len(argv) < 4 {
		t.Fatalf("argv too short: %v", argv)
	}
	last4 := argv[len(argv)-4:]
	want := []string{"--", "/bin/sh", "-c", cmd}
	if !reflect.DeepEqual(last4, want) {
		t.Fatalf("argv tail = %v, want %v", last4, want)
	}
}

func TestBuildArgv_DeterministicOrder(t *testing.T) {
	p := DefaultSandboxPolicy()
	p.BindMounts = []BindMount{
		{Source: "/a", Target: "/sb/a", ReadOnly: false},
		{Source: "/b", Target: "/sb/b", ReadOnly: true},
	}
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	a1 := b.BuildArgv(p, "echo det")
	a2 := b.BuildArgv(p, "echo det")
	if !reflect.DeepEqual(a1, a2) {
		t.Fatalf("BuildArgv non-deterministic:\n  a1=%v\n  a2=%v", a1, a2)
	}
}

func TestBuildArgv_LeadingTokenIsBwrapPath(t *testing.T) {
	b := NewBubblewrapBackend("/opt/custom/bwrap", "/work")
	argv := b.BuildArgv(DefaultSandboxPolicy(), "true")
	if len(argv) == 0 || argv[0] != "/opt/custom/bwrap" {
		t.Fatalf("argv[0] = %q, want %q", argv[0], "/opt/custom/bwrap")
	}
}

// --- Real-bwrap gated tests ---

func TestRun_Gated_RealBwrap_HelloWorld(t *testing.T) {
	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("SKIP-OK: P1-F14-T04 bwrap not on PATH (apt install bubblewrap)")
	}
	work := t.TempDir()
	b := NewBubblewrapBackend(bwrapPath, work)
	ctx := context.Background()
	res, err := b.Run(ctx, "echo hello-from-sandbox", DefaultSandboxPolicy())
	if err != nil {
		t.Fatalf("Run failed: %v (stderr=%q)", err, func() string {
			if res != nil {
				return res.Stderr
			}
			return ""
		}())
	}
	if res.Backend != BackendBubblewrap {
		t.Errorf("Backend = %v, want %v", res.Backend, BackendBubblewrap)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; stderr=%q", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "hello-from-sandbox\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello-from-sandbox\n")
	}
	t.Logf("real-bwrap hello stdout = %q", res.Stdout)
}

func TestRun_Gated_RealBwrap_NetworkDeniedByDefault(t *testing.T) {
	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("SKIP-OK: P1-F14-T04 bwrap not on PATH (apt install bubblewrap)")
	}
	// Pick a network probe.
	var probe string
	if _, err := exec.LookPath("curl"); err == nil {
		probe = "curl -sS -m 3 https://example.com >/dev/null 2>&1 || echo NETDENIED"
	} else if _, err := exec.LookPath("getent"); err == nil {
		probe = "getent hosts example.com >/dev/null 2>&1 || echo NETDENIED"
	} else {
		t.Skip("SKIP-OK: P1-F14-T04 no network probe binary available")
	}
	work := t.TempDir()
	b := NewBubblewrapBackend(bwrapPath, work)
	ctx := context.Background()
	res, err := b.Run(ctx, probe, DefaultSandboxPolicy())
	if err != nil {
		t.Fatalf("Run failed: %v (stderr=%q)", err, func() string {
			if res != nil {
				return res.Stderr
			}
			return ""
		}())
	}
	if !strings.Contains(res.Stdout, "NETDENIED") {
		t.Fatalf("expected NETDENIED in stdout, got stdout=%q stderr=%q exit=%d",
			res.Stdout, res.Stderr, res.ExitCode)
	}
	t.Logf("real-bwrap network-deny verified: stdout=%q", res.Stdout)
}

// --- Stub Runner timeout test ---

func TestRun_TimeoutEnforced(t *testing.T) {
	b := NewBubblewrapBackend("/usr/bin/bwrap", "/work")
	// Stub runner: blocks on ctx.Done(), then returns -1 to mimic kill.
	b.Runner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		<-ctx.Done()
		return nil, []byte("killed by timeout"), -1, ctx.Err()
	}
	policy := DefaultSandboxPolicy()
	policy.Timeout = 50 * time.Millisecond

	start := time.Now()
	res, err := b.Run(context.Background(), "sleep 10", policy)
	elapsed := time.Since(start)

	if err != nil {
		// timeout path may surface ctx.Err() — accept either nil or DeadlineExceeded.
		if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "context") {
			t.Fatalf("unexpected err: %v", err)
		}
	}
	if res == nil {
		t.Fatalf("nil result on timeout")
	}
	if !res.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}
	if elapsed > 1*time.Second {
		t.Errorf("Run elapsed %v — timeout not enforced", elapsed)
	}
	if res.Backend != BackendBubblewrap {
		t.Errorf("Backend = %v, want %v", res.Backend, BackendBubblewrap)
	}
}

// Sanity: ensure path joins behave as expected on both styles of WorkDir
// (the BuildArgv must accept an arbitrary absolute path).
func TestBuildArgv_AbsoluteWorkdir(t *testing.T) {
	work := filepath.Clean("/srv/project")
	b := NewBubblewrapBackend("/usr/bin/bwrap", work)
	argv := b.BuildArgv(DefaultSandboxPolicy(), "true")
	if !findArgTriple(argv, "--bind", work, work) {
		t.Fatalf("missing absolute workdir bind in argv: %v", argv)
	}
}
