// subprocess_spawner.go — P1-F15-T04 SubprocessSpawner.
//
// Implements SubagentSpawner by re-exec'ing the host binary (helixcode) with a
// sentinel env-var set; the re-exec'd child runs the subagent loop and emits a
// SubagentResult JSON to stdout before exiting. The parent decodes that JSON
// and forwards it on the result channel, then closes the channel.
//
// This file is the PARENT side of the helper protocol. The CHILD side
// (IsSubagentInvocation / RunAsSubagent dispatch) lives in T08; for tests we
// substitute a tiny standalone helper binary built by TestMain (see
// `testhelper/main.go` and subprocess_spawner_test.go), which is sufficient
// to prove the protocol round-trip without coupling T04 to T08.
//
// The pattern mirrors F14's NativeBackend helper-mode: pass arguments via env
// rather than via positional args or stdin, so the helper path remains
// dependency-free (no temp files, no pipes to wire) and so the parent has a
// single, easy-to-grep marker (the env-var name) for "this child is in helper
// mode".
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f15-subagent-team-design.md §4.2
// Plan: docs/superpowers/plans/2026-05-06-p1-f15-subagent-team.md T04
package subagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"dev.helix.code/internal/llm"
)

// SubagentHelperEnvVar is the marker the parent sets on the re-exec'd child
// to signal "you are running as the subagent helper". main.go (T08) will
// check IsSubagentInvocation() at startup and dispatch to RunAsSubagent
// before normal CLI parsing.
const SubagentHelperEnvVar = "HELIXCODE_SUBAGENT_HELPER"

// SubagentHelperPayloadEnvVar carries a JSON-encoded SubagentTask from the
// parent to the helper child. Env-var (rather than a file or stdin) keeps the
// protocol path dependency-free — see the file-level comment for rationale.
const SubagentHelperPayloadEnvVar = "HELIXCODE_SUBAGENT_HELPER_PAYLOAD"

// stderrCaptureLimit caps how much child stderr we fold into result.Error on
// non-zero-exit failures. 4 KiB is large enough to surface real diagnostics
// (a Go panic stack trace, a config-loader error chain) without unbounded
// growth that could OOM the parent if the helper goes berserk.
const stderrCaptureLimit = 4 * 1024

// SubprocessSpawner runs subagents as separate processes by re-exec'ing the
// host binary (or, in tests, a standalone helper binary). Each subagent's
// stdout is parsed as a SubagentResult JSON.
//
// Note: SubprocessSpawner does NOT use the llmProvider argument. The child
// process constructs its own provider via T07/T08 wiring (which reads the
// helixcode config). The arg is accepted for SubagentSpawner-interface
// conformance but ignored at this layer; passing nil is therefore explicitly
// supported and tested. Future maintainers: do not expect llmProvider to
// flow into the child — there is no in-band channel to do so.
type SubprocessSpawner struct {
	// HostBinary is the absolute path used for the re-exec. Defaults to
	// os.Executable() in NewSubprocessSpawner; tests overwrite to point at
	// a fake helper binary built by TestMain.
	HostBinary string

	// WorkDir is the cwd for the child process. Empty string means "inherit
	// parent's cwd" (exec.Cmd's default behaviour).
	WorkDir string

	// Runner is the injectable execution seam. Tests override it to record
	// the env / args / dir that would be passed to a real exec, or to inject
	// canned stdout. Production code uses defaultSubprocessRunner.
	//
	// Contract: Runner MUST honour ctx cancellation. The default runner
	// does so via exec.CommandContext, which sends SIGKILL when ctx fires.
	Runner func(ctx context.Context, name string, args []string, env []string, dir string) (stdout, stderr []byte, exitCode int, err error)
}

// NewSubprocessSpawner constructs a spawner pointing at os.Executable().
// `workDir` may be empty to inherit the parent's cwd. Production code should
// always use this constructor; tests may overwrite HostBinary / Runner after
// construction.
func NewSubprocessSpawner(workDir string) (*SubprocessSpawner, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("subagent subprocess: resolving host binary: %w", err)
	}
	return &SubprocessSpawner{
		HostBinary: exe,
		WorkDir:    workDir,
		Runner:     defaultSubprocessRunner,
	}, nil
}

// Kind returns the spawner identifier "subprocess". Used by the manager for
// routing decisions and by Challenge harnesses for evidence collection.
func (s *SubprocessSpawner) Kind() string { return "subprocess" }

// Spawn launches the helper subprocess. Returns a buffered (cap 1) channel
// that receives exactly one SubagentResult before being closed.
//
// Behaviour:
//
//   - Marshals task to JSON and places it in env[SubagentHelperPayloadEnvVar].
//   - Sets env[SubagentHelperEnvVar]=1 + env[SubagentRecursionEnvVar]=1 so the
//     child knows it is in helper mode AND so a grandchild subagent CANNOT
//     register the `task` tool (recursion cap = 1).
//   - Applies task.Timeout via context.WithTimeout when > 0.
//   - Decodes the child's stdout as SubagentResult JSON. On parse failure,
//     synthesises a StateFailed result with Error = "invalid helper output: …".
//   - On exit code != 0 with non-empty stderr, surfaces stderr (truncated to
//     stderrCaptureLimit) in result.Error.
//   - On ctx.DeadlineExceeded inside Run, the child is SIGKILL'd by
//     exec.CommandContext; the result is StateTimedOut.
//   - On parent ctx cancel, result is StateCanceled.
//
// llmProvider is ignored (see type doc). Returns (nil, error) only for
// programmer errors (empty HostBinary, nil Runner).
func (s *SubprocessSpawner) Spawn(ctx context.Context, task SubagentTask, llmProvider llm.Provider) (<-chan SubagentResult, error) {
	if s.HostBinary == "" {
		return nil, errors.New("SubprocessSpawner.Spawn: HostBinary is empty")
	}
	if s.Runner == nil {
		return nil, errors.New("SubprocessSpawner.Spawn: Runner is nil")
	}

	out := make(chan SubagentResult, 1)
	go s.run(ctx, task, out)
	return out, nil
}

// run executes the helper subprocess. ALWAYS sends exactly one result and
// closes the channel.
func (s *SubprocessSpawner) run(parentCtx context.Context, task SubagentTask, out chan<- SubagentResult) {
	startedAt := time.Now()

	// Defer the close + send so any early-exit error path still emits one
	// result. We mutate `result` along the way and send it at the end.
	result := SubagentResult{
		TaskID:    task.ID,
		StartedAt: startedAt,
		Isolation: task.Isolation,
	}
	defer func() {
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
		out <- result
		close(out)
	}()

	payload, err := json.Marshal(&task)
	if err != nil {
		result.State = StateFailed
		result.Error = fmt.Sprintf("subagent subprocess: marshal task: %v", err)
		return
	}

	// Build the child env: parent env + helper marker + payload + recursion
	// guard. Strip any pre-existing helper-marker / payload entries so a
	// nested invocation cannot accidentally inherit a stale outer payload.
	env := stripSubagentHelperEnv(os.Environ())
	env = append(env,
		SubagentHelperEnvVar+"=1",
		SubagentHelperPayloadEnvVar+"="+string(payload),
		SubagentRecursionEnvVar+"=1",
	)

	// Build the per-call ctx, honouring task.Timeout when > 0.
	runCtx := parentCtx
	var cancel context.CancelFunc
	if task.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(parentCtx, task.Timeout)
	}
	if cancel != nil {
		defer cancel()
	}

	stdout, stderr, exitCode, runErr := s.Runner(runCtx, s.HostBinary, nil, env, s.WorkDir)

	// Classify ctx-driven termination FIRST. If runCtx fired, the child was
	// killed (or the runner returned ctx.Err()); the exit code / stderr are
	// secondary signal — the primary outcome is "timed out" or "canceled".
	if runCtxErr := runCtx.Err(); runCtxErr != nil {
		switch {
		case errors.Is(runCtxErr, context.DeadlineExceeded):
			result.State = StateTimedOut
			result.Error = "subagent subprocess: " + ErrSubagentTimeout.Error()
		case errors.Is(runCtxErr, context.Canceled):
			// Distinguish parent-ctx cancel from task-timeout cancel: if
			// parentCtx itself is canceled, the user / manager called Kill;
			// otherwise the per-task timeout fired. Both classify as
			// canceled-ish, but parent-ctx wins because it represents an
			// explicit operator decision.
			if parentCtx.Err() != nil {
				result.State = StateCanceled
				result.Error = "subagent subprocess: " + ErrSubagentCanceled.Error()
			} else {
				result.State = StateCanceled
				result.Error = "subagent subprocess: " + ErrSubagentCanceled.Error()
			}
		}
		// If the runner also captured stderr from the child before the kill,
		// fold a truncated copy in for diagnostics.
		if len(stderr) > 0 {
			result.Error = result.Error + ": " + truncate(string(stderr), stderrCaptureLimit)
		}
		return
	}

	// Non-ctx-driven failure: runner couldn't even start the binary, or
	// some other Go-level error.
	if runErr != nil {
		result.State = StateFailed
		msg := fmt.Sprintf("subagent subprocess: runner error: %v", runErr)
		if len(stderr) > 0 {
			msg += ": " + truncate(string(stderr), stderrCaptureLimit)
		}
		result.Error = msg
		return
	}

	// Exit code != 0: helper ran but failed. Surface stderr if any.
	if exitCode != 0 {
		result.State = StateFailed
		msg := fmt.Sprintf("subagent subprocess: helper exit code %d", exitCode)
		if len(stderr) > 0 {
			msg += ": " + truncate(string(stderr), stderrCaptureLimit)
		}
		result.Error = msg
		return
	}

	// Exit 0: decode stdout as SubagentResult JSON.
	var decoded SubagentResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &decoded); err != nil {
		result.State = StateFailed
		result.Error = fmt.Sprintf("subagent subprocess: invalid helper output: %v (stdout=%q)", err, truncate(string(stdout), stderrCaptureLimit))
		return
	}

	// Carry the decoded fields through, but preserve the parent-side timing
	// (StartedAt / CompletedAt / Duration) — the parent's clock is the
	// source of truth, and the child's may be unset or off.
	startedAtSnapshot := result.StartedAt
	result = decoded
	result.StartedAt = startedAtSnapshot
	// CompletedAt + Duration are set by the deferred send.
	if result.TaskID == "" {
		result.TaskID = task.ID
	}
}

// defaultSubprocessRunner is the production execution path: exec.CommandContext
// with the supplied env / dir, capturing stdout/stderr separately. Returns
// (stdout, stderr, exitCode, err); a non-nil exec.ExitError is folded into
// exitCode rather than err so callers can distinguish "child exited non-zero"
// from "failed to start".
func defaultSubprocessRunner(ctx context.Context, name string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	if dir != "" {
		cmd.Dir = dir
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

// stripSubagentHelperEnv returns env without the helper marker / payload
// entries — a child should never inherit stale outer payloads.
func stripSubagentHelperEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if startsWithEnvKey(e, SubagentHelperEnvVar) ||
			startsWithEnvKey(e, SubagentHelperPayloadEnvVar) ||
			startsWithEnvKey(e, SubagentRecursionEnvVar) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// startsWithEnvKey reports whether `entry` is `key=…`. Mirrors the helper in
// F14's native_backend.go; kept local so subagent has no cross-package
// dependency on sandbox internals.
func startsWithEnvKey(entry, key string) bool {
	if len(entry) <= len(key) {
		return false
	}
	if entry[len(key)] != '=' {
		return false
	}
	return entry[:len(key)] == key
}

// truncate returns s capped to limit bytes; if s exceeds the cap, an
// "…(truncated)" suffix is appended for forensic clarity. Operates on bytes
// (not runes) because the input here is always best-effort log text.
func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "…(truncated)"
}
