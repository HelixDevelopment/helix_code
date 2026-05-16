package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// BubblewrapBackend implements SandboxBackend using the `bwrap(1)` binary.
//
// Construction: use `NewBubblewrapBackend(bwrapPath, workDir)`. `bwrapPath`
// MUST be the absolute path discovered by the Detector (T03); we do not
// look up `bwrap` ourselves to keep capability discovery centralised.
//
// `BuildArgv` is a PURE function — no I/O, no logging, no time-of-day
// dependence — so the manager (T06) and unit tests can compare argv byte
// for byte. `Run` performs the actual execution by handing argv to the
// injectable `Runner` field; production code uses the default real
// `os/exec` runner installed by `NewBubblewrapBackend`.
//
// Resource limits (`policy.MemoryLimitMB`, `policy.CPULimitPct`) are
// intentionally NOT applied here: bwrap does not expose cgroup knobs
// directly, so cgroup-v2 enforcement lives in the manager (T06). The
// backend silently ignores those policy fields by design.
//
// Read-only-root note: this backend ALWAYS adds `--ro-bind / /` regardless
// of `policy.ReadOnlyRoot`. The flag exists in the policy struct as a
// reservation for future RW-root experiments (e.g. running an installer
// inside a throwaway sandbox); v1 keeps RO-root unconditional for safety.
type BubblewrapBackend struct {
	// BwrapPath is the absolute path to the `bwrap` binary, as resolved by
	// the Detector. Must be non-empty; an empty path will cause `Run` to
	// return an error from `exec`.
	BwrapPath string

	// WorkDir is the host path bound RW into the sandbox at the same path
	// (see argv comment for details). It is also the `--chdir` target.
	WorkDir string

	// Runner is the injectable execution seam. Defaults to a real
	// `os/exec.CommandContext` runner installed by `NewBubblewrapBackend`.
	// Tests inject stubs to exercise timeout / error paths without
	// spawning bwrap.
	//
	// Contract: `Runner` MUST honour `ctx` cancellation. The default
	// implementation does so via `exec.CommandContext`. When `ctx`
	// expires the command is killed, stdout/stderr captured up to that
	// point are returned, and `exitCode == -1`.
	Runner func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)
}

// defaultRunner is the production execution path: `exec.CommandContext`
// with separate stdout/stderr buffers. It returns `(stdout, stderr,
// exitCode, err)`. Note: a non-nil `err` does NOT preclude useful stdout
// — bwrap may still produce diagnostic output before failing, and the
// child program may have written before being killed.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	// `exec.ExitError` is normal — the child returned non-zero. Surface
	// only "real" errors (failed to start, killed by ctx, etc.) to the
	// caller; non-zero exit is reported via exitCode.
	var exitErr *exec.ExitError
	if runErr != nil && errors.As(runErr, &exitErr) {
		runErr = nil
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode, runErr
}

// NewBubblewrapBackend constructs a backend with the production
// `os/exec`-based runner. Tests that need to stub Runner may overwrite
// the field after construction.
func NewBubblewrapBackend(bwrapPath, workDir string) *BubblewrapBackend {
	return &BubblewrapBackend{
		BwrapPath: bwrapPath,
		WorkDir:   workDir,
		Runner:    defaultRunner,
	}
}

// Kind returns `BackendBubblewrap`.
func (b *BubblewrapBackend) Kind() BackendKind { return BackendBubblewrap }

// BuildArgv returns the bwrap argv for `(policy, command)`. PURE function:
// same inputs ⇒ byte-identical output (asserted by
// `TestBuildArgv_DeterministicOrder`).
//
// Argv form (spec §4.3, Bubblewrap argv):
//
//	bwrap --die-with-parent --new-session
//	      --unshare-pid --unshare-ipc --unshare-uts --unshare-cgroup
//	      [--unshare-net | --share-net]
//	      --unshare-user
//	      --proc /proc --dev /dev --tmpfs /tmp
//	      --ro-bind / /
//	      --bind <WorkDir> <WorkDir>
//	      [--ro-bind|--bind <bm.Source> <bm.Target>]*    # in policy.BindMounts order
//	      --chdir <WorkDir>
//	      --
//	      /bin/sh -c <command>
//
// Order rationale:
//   - Lifecycle / session flags first (so a parent-death during option
//     parsing still tears down anything bwrap has set up).
//   - Namespace unshares before any mounts (mounts apply inside the new
//     namespaces).
//   - Network share/unshare next to keep network-related flags grouped
//     and easy to grep in audit logs.
//   - User-namespace last among unshares (matches bwrap docs ordering).
//   - Filesystem virtualisation (--proc/--dev/--tmpfs) before bind mounts
//     so /proc, /dev, /tmp exist before paths under them are bound.
//   - --ro-bind / / before specific binds so per-path binds shadow root.
//   - WorkDir bind before user bind mounts so user mounts may target
//     paths under WorkDir if desired.
//   - --chdir AFTER all binds; "--" separator before /bin/sh -c command.
func (b *BubblewrapBackend) BuildArgv(policy SandboxPolicy, command string) []string {
	argv := make([]string, 0, 32+3*len(policy.BindMounts))

	argv = append(argv, b.BwrapPath)

	// Lifecycle / session
	argv = append(argv,
		"--die-with-parent",
		"--new-session",
	)

	// Namespace unshares (always applied)
	argv = append(argv,
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--unshare-cgroup",
	)

	// Network share/unshare — explicit, never default-implicit.
	if policy.NetworkAllowed {
		argv = append(argv, "--share-net")
	} else {
		argv = append(argv, "--unshare-net")
	}

	// User namespace last among unshares (bwrap convention).
	argv = append(argv, "--unshare-user")

	// Filesystem virtualisation.
	argv = append(argv,
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
	)

	// Read-only root (always; see type doc for rationale).
	argv = append(argv, "--ro-bind", "/", "/")

	// Working directory: bound RW so the child can read/write project
	// files. We bind WorkDir at the same path inside the sandbox to keep
	// any tooling that emits absolute paths (compilers, linters, LSP)
	// working without translation.
	argv = append(argv, "--bind", b.WorkDir, b.WorkDir)

	// Additional bind mounts — preserved in user-supplied order so the
	// argv is deterministic and `TestBuildArgv_DeterministicOrder`
	// passes. Later bind mounts shadow earlier ones inside bwrap, which
	// matches Linux mount semantics — users opt in to this when they
	// list multiple binds at the same target.
	for _, bm := range policy.BindMounts {
		flag := "--bind"
		if bm.ReadOnly {
			flag = "--ro-bind"
		}
		argv = append(argv, flag, bm.Source, bm.Target)
	}

	// Working directory and command separator.
	argv = append(argv,
		"--chdir", b.WorkDir,
		"--",
		"/bin/sh", "-c", command,
	)

	return argv
}

// Run executes `command` in a bwrap sandbox using `policy`. Honors
// `policy.Timeout` via `context.WithTimeout` (only when > 0). The
// returned `*SandboxResult` is always non-nil so callers can rely on
// `result.Backend` even on error paths — except when argv synthesis
// itself fails (e.g. empty BwrapPath), which is a programmer error and
// produces `(nil, error)`.
//
// Timeout / cancellation semantics:
//   - `policy.Timeout > 0` ⇒ child is killed when the deadline elapses;
//     `result.TimedOut = true`, `result.ExitCode = -1`, the wrapped
//     `context.DeadlineExceeded` is returned as `err`.
//   - The caller-provided `ctx` is honoured first: if the caller cancels
//     before our deadline, we propagate the caller's error.
func (b *BubblewrapBackend) Run(ctx context.Context, command string, policy SandboxPolicy) (*SandboxResult, error) {
	if b.BwrapPath == "" {
		return nil, fmt.Errorf("sandbox: bubblewrap backend constructed with empty BwrapPath")
	}
	if b.Runner == nil {
		return nil, fmt.Errorf("sandbox: bubblewrap backend constructed without Runner")
	}

	argv := b.BuildArgv(policy, command)
	// argv[0] is the bwrap path; the runner takes (name, args...).
	name := argv[0]
	args := argv[1:]

	runCtx := ctx
	var cancel context.CancelFunc
	if policy.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, policy.Timeout)
		defer cancel()
	}

	start := time.Now()
	stdout, stderr, exitCode, err := b.Runner(runCtx, name, args...)
	duration := time.Since(start)

	result := &SandboxResult{
		Stdout:   string(stdout),
		Stderr:   string(stderr),
		ExitCode: exitCode,
		Backend:  BackendBubblewrap,
		Duration: duration,
	}

	// Timeout detection: prefer runCtx's reason — a `context.DeadlineExceeded`
	// or `context.Canceled` here means the runner returned because the
	// deadline (or caller cancellation) fired. We mark TimedOut only for
	// the deadline case to keep the field honest about its name.
	if runCtxErr := runCtx.Err(); runCtxErr != nil {
		if errors.Is(runCtxErr, context.DeadlineExceeded) {
			result.TimedOut = true
			if result.ExitCode == 0 {
				// Some runners may return 0 even after a kill — normalise.
				result.ExitCode = -1
			}
		}
	}

	return result, err
}
