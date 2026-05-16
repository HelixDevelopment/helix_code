//go:build linux

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// helperEnvVar is the marker env-var the parent sets on the re-exec child to
// signal "you are running as the in-namespace helper, do helper setup then
// exec the user command". main.go (T10) checks IsHelperInvocation() at
// startup and dispatches to RunAsHelper before normal CLI parsing.
const helperEnvVar = "HELIX_SANDBOX_NATIVE_HELPER"

// helperArgsEnvVar carries a JSON-encoded helperPayload from the parent to
// the helper child. Env-var (rather than a file or stdin) keeps the helper
// path dependency-free: no temp files to clean up, no pipes to wire.
const helperArgsEnvVar = "HELIX_SANDBOX_NATIVE_HELPER_ARGS"

// NativeBackend implements SandboxBackend using Go's clone-with-namespaces
// (CLONE_NEWPID | CLONE_NEWNET | CLONE_NEWNS | CLONE_NEWUSER | CLONE_NEWUTS
// | CLONE_NEWIPC). The child is a re-exec of the host binary in helper
// mode: helper-mode bootstraps the namespace-internal setup (mount /proc,
// apply rlimits, chdir) and then exec's `/bin/sh -c <command>`.
//
// Why re-exec rather than a forkExec hook?
//
//   - Go's runtime is multi-threaded; doing complex work between fork and
//     exec inside a Go process is unsafe (the child inherits one thread of
//     a multi-threaded runtime). Re-exec'ing the host binary lets the
//     helper start fresh as a new Go process inside the new namespaces.
//
//   - It mirrors the runc / unshare(1) pattern, which is the prevailing
//     idiom for unprivileged-userns-based sandboxing on Linux.
//
// Resource limits: cgroup-v2 enforcement lives in the manager (T06). The
// helper applies an RLIMIT_AS fallback when policy.MemoryLimitMB > 0 so
// hosts without cgroup-v2 still get a memory ceiling.
type NativeBackend struct {
	// HostBinary is the absolute path used for the re-exec. Defaults to
	// os.Executable() in NewNativeBackend; tests may overwrite to point at
	// a fake helper binary.
	HostBinary string

	// WorkDir is the host path the helper will chdir into after namespace
	// setup. Bound RW into the sandbox (see helperSetupNamespace).
	WorkDir string

	// Runner is the injectable execution seam. The signature is wide
	// enough to surface the SysProcAttr fields tests want to assert on
	// (Cloneflags, UidMappings, GidMappings) without needing reflection.
	// Defaults to defaultNativeRunner installed by NewNativeBackend.
	//
	// Contract: Runner MUST honour ctx cancellation. The default runner
	// does so via exec.CommandContext.
	Runner func(ctx context.Context, name string, args []string, env []string, cloneflags uintptr, uidMaps []syscall.SysProcIDMap, gidMaps []syscall.SysProcIDMap) (stdout, stderr []byte, exitCode int, err error)
}

// helperPayload is the JSON shape passed from parent → helper child via
// helperArgsEnvVar. Round-trip-tested by TestPayloadJSONRoundTrip.
type helperPayload struct {
	Command        string      `json:"command"`
	NetworkAllowed bool        `json:"network_allowed"`
	WorkDir        string      `json:"work_dir"`
	BindMounts     []BindMount `json:"bind_mounts"`
	MemoryLimitMB  int         `json:"memory_limit_mb"`
}

// NewNativeBackend constructs a backend wired to the real os/exec runner
// and with HostBinary populated from os.Executable(). Production code
// should always use this constructor; tests may overwrite HostBinary or
// Runner after construction.
func NewNativeBackend(workDir string) (*NativeBackend, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("native sandbox: resolving host binary: %w", err)
	}
	return &NativeBackend{
		HostBinary: exe,
		WorkDir:    workDir,
		Runner:     defaultNativeRunner,
	}, nil
}

// Kind returns BackendNative.
func (n *NativeBackend) Kind() BackendKind { return BackendNative }

// buildCloneflags is a pure helper: returns the Cloneflags bitmask required
// for `policy`. CLONE_NEWUSER/PID/NS/UTS/IPC are always set; CLONE_NEWNET is
// set only when NetworkAllowed is false. PURE function — no I/O, asserted
// by the TestBuildCloneflags_* unit tests.
func buildCloneflags(policy SandboxPolicy) uintptr {
	var flags uintptr = syscall.CLONE_NEWUSER |
		syscall.CLONE_NEWPID |
		syscall.CLONE_NEWNS |
		syscall.CLONE_NEWUTS |
		syscall.CLONE_NEWIPC
	if !policy.NetworkAllowed {
		flags |= syscall.CLONE_NEWNET
	}
	return flags
}

// buildUIDGIDMaps returns the UID/GID maps that map the child's root (UID
// 0 inside the new userns) to the parent's real UID/GID. Go's stdlib
// SysProcAttr.{Uid,Gid}Mappings handles writing these to
// /proc/<pid>/uid_map and gid_map.
func buildUIDGIDMaps() ([]syscall.SysProcIDMap, []syscall.SysProcIDMap) {
	uid := os.Getuid()
	gid := os.Getgid()
	return []syscall.SysProcIDMap{{ContainerID: 0, HostID: uid, Size: 1}},
		[]syscall.SysProcIDMap{{ContainerID: 0, HostID: gid, Size: 1}}
}

// IsHelperInvocation reports whether the current process should run as the
// native sandbox helper. main.go (T10) calls this at startup; if true, it
// dispatches to RunAsHelper and exits before normal CLI dispatch.
func IsHelperInvocation() bool {
	return os.Getenv(helperEnvVar) != ""
}

// RunAsHelper is invoked from main.go when IsHelperInvocation() returns
// true. It reads the helperPayload from the env, performs in-namespace
// setup (remount /proc, apply rlimits, chdir into WorkDir), then exec's
// `/bin/sh -c <command>`. Exec replaces the process so this function
// never returns under normal flow; on any pre-exec error it returns a
// non-zero exit code suitable for os.Exit.
func RunAsHelper() (exitCode int) {
	raw := os.Getenv(helperArgsEnvVar)
	if raw == "" {
		fmt.Fprintln(os.Stderr, "native sandbox helper: missing payload env")
		return 2
	}
	var payload helperPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		fmt.Fprintf(os.Stderr, "native sandbox helper: malformed payload: %v\n", err)
		return 2
	}
	if err := helperSetupNamespace(payload); err != nil {
		fmt.Fprintf(os.Stderr, "native sandbox helper: setup failed: %v\n", err)
		return 2
	}

	// Strip the helper env so the child shell does not see them.
	env := stripHelperEnv(os.Environ())

	if err := syscall.Exec("/bin/sh", []string{"/bin/sh", "-c", payload.Command}, env); err != nil {
		fmt.Fprintf(os.Stderr, "native sandbox helper: exec /bin/sh failed: %v\n", err)
		return 127
	}
	// Unreachable on success — Exec replaces the process.
	return 0
}

// helperSetupNamespace performs the minimal namespace-internal bootstrap:
//   - Mount a fresh /proc so PID 1 inside the namespace sees only its own
//     process tree.
//   - Apply RLIMIT_AS when MemoryLimitMB > 0 (cgroup-v2 enforcement is the
//     manager's job; this is a fallback for cgroup-v1-only hosts).
//   - chdir into WorkDir so the user command runs in the project tree.
//
// Bind mounts requested via payload.BindMounts are honoured by mounting
// each Source onto Target inside the new mount namespace. Read-only mounts
// are achieved with MS_BIND followed by MS_REMOUNT|MS_RDONLY|MS_BIND.
func helperSetupNamespace(payload helperPayload) error {
	// 1. Mount /proc fresh so getpid() reflects the namespace's PID 1.
	//    MS_PRIVATE on / first to prevent mount events leaking back to
	//    the host (defensive — userns + new mountns should already
	//    isolate, but explicit is better).
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("mount-make-private /: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}

	// 2. Bind mounts from payload.
	for _, bm := range payload.BindMounts {
		if err := syscall.Mount(bm.Source, bm.Target, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("bind %s -> %s: %w", bm.Source, bm.Target, err)
		}
		if bm.ReadOnly {
			if err := syscall.Mount("", bm.Target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, ""); err != nil {
				return fmt.Errorf("remount-ro %s: %w", bm.Target, err)
			}
		}
	}

	// 3. Memory rlimit fallback. cgroup-v2 (T06) is preferred; this is the
	//    cgroup-v1-only safety net so a malformed user command can't OOM
	//    the host even on legacy systems.
	if payload.MemoryLimitMB > 0 {
		bytes := uint64(payload.MemoryLimitMB) * 1024 * 1024
		rl := &syscall.Rlimit{Cur: bytes, Max: bytes}
		if err := syscall.Setrlimit(syscall.RLIMIT_AS, rl); err != nil {
			return fmt.Errorf("setrlimit RLIMIT_AS=%dMB: %w", payload.MemoryLimitMB, err)
		}
	}

	// 4. chdir into the project tree.
	if payload.WorkDir != "" {
		if err := os.Chdir(payload.WorkDir); err != nil {
			return fmt.Errorf("chdir %s: %w", payload.WorkDir, err)
		}
	}
	return nil
}

// stripHelperEnv returns env without the helper marker / payload entries —
// the user's shell should not inherit them.
func stripHelperEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if startsWithKey(e, helperEnvVar) || startsWithKey(e, helperArgsEnvVar) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func startsWithKey(entry, key string) bool {
	if len(entry) <= len(key) {
		return false
	}
	if entry[len(key)] != '=' {
		return false
	}
	return entry[:len(key)] == key
}

// defaultNativeRunner is the production execution path: exec.CommandContext
// with SysProcAttr populated for the requested Cloneflags + UID/GID maps.
// Returns (stdout, stderr, exitCode, err); a non-nil exec.ExitError is
// folded into exitCode rather than err so callers can distinguish "child
// exited non-zero" from "failed to start".
func defaultNativeRunner(ctx context.Context, name string, args []string, env []string, cloneflags uintptr, uidMaps []syscall.SysProcIDMap, gidMaps []syscall.SysProcIDMap) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   cloneflags,
		UidMappings:  uidMaps,
		GidMappings:  gidMaps,
		GidMappingsEnableSetgroups: false,
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	var exitErr *exec.ExitError
	if runErr != nil && errors.As(runErr, &exitErr) {
		runErr = nil
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode, runErr
}

// Run launches the helper subprocess with the appropriate Cloneflags +
// UID/GID maps. Captures stdout/stderr separately. Honors policy.Timeout
// via context.WithTimeout.
//
// The result is always non-nil so callers can rely on result.Backend even
// on error paths — except when we cannot construct the runner at all
// (empty HostBinary, nil Runner), which is a programmer error and returns
// (nil, error).
func (n *NativeBackend) Run(ctx context.Context, command string, policy SandboxPolicy) (*SandboxResult, error) {
	if n.HostBinary == "" {
		return nil, fmt.Errorf("sandbox: native backend constructed with empty HostBinary")
	}
	if n.Runner == nil {
		return nil, fmt.Errorf("sandbox: native backend constructed without Runner")
	}

	payload := helperPayload{
		Command:        command,
		NetworkAllowed: policy.NetworkAllowed,
		WorkDir:        n.WorkDir,
		BindMounts:     policy.BindMounts,
		MemoryLimitMB:  policy.MemoryLimitMB,
	}
	payloadJSON, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("sandbox: marshal helper payload: %w", err)
	}

	// Pass payload via env. Inherit the parent's env so PATH etc. flow
	// through, then add the two helper variables.
	env := append([]string(nil), os.Environ()...)
	env = stripHelperEnv(env) // belt-and-braces: don't pass nested helper markers
	env = append(env,
		helperEnvVar+"=1",
		helperArgsEnvVar+"="+string(payloadJSON),
	)

	flags := buildCloneflags(policy)
	uidMaps, gidMaps := buildUIDGIDMaps()

	runCtx := ctx
	var cancel context.CancelFunc
	if policy.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, policy.Timeout)
		defer cancel()
	}

	start := time.Now()
	stdout, stderr, exitCode, runErr := n.Runner(runCtx, n.HostBinary, nil, env, flags, uidMaps, gidMaps)
	duration := time.Since(start)

	result := &SandboxResult{
		Stdout:   string(stdout),
		Stderr:   string(stderr),
		ExitCode: exitCode,
		Backend:  BackendNative,
		Duration: duration,
	}

	if runCtxErr := runCtx.Err(); runCtxErr != nil {
		if errors.Is(runCtxErr, context.DeadlineExceeded) {
			result.TimedOut = true
			if result.ExitCode == 0 {
				result.ExitCode = -1
			}
		}
	}

	return result, runErr
}
