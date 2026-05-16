// helper_mode.go — P1-F15-T08 child-side helper-mode dispatch.
//
// This file is the CHILD side of the subagent subprocess protocol introduced
// by T04 (subprocess_spawner.go). The parent re-exec's the host binary with:
//
//   HELIXCODE_SUBAGENT_HELPER=1
//   HELIXCODE_SUBAGENT_HELPER_PAYLOAD=<json-encoded SubagentTask>
//   HELIXCODE_SUBAGENT_NO_RECURSE=1
//
// At process startup, main.go MUST call IsSubagentInvocation() BEFORE any
// other dispatch (including the F14 sandbox helper-mode dispatch — see the
// anchor comment in cmd/cli/main.go). When true, main.go calls RunAsSubagent
// with a provider-factory closure and os.Exit's with the returned code.
//
// The helper:
//
//   1. Decodes SubagentTask from HELIXCODE_SUBAGENT_HELPER_PAYLOAD.
//   2. Calls llmFactory to construct the LLM provider (the child has its own
//      provider — there is no in-band channel for the parent to pass one).
//   3. Runs an InProcessSpawner against that provider with the decoded task.
//      Isolation is forced to IsolationNone in the child: by the time we get
//      here we are already in whatever worktree / cwd the parent set up; a
//      grandchild subprocess would also be forbidden by the recursion guard.
//   4. Drains the (cap-1) result channel.
//   5. JSON-marshals the SubagentResult and writes it to stdout.
//   6. Returns exit code:
//        - 0 on successful protocol round-trip (even if State=StateFailed —
//          the child exits cleanly so the parent can decode the failure).
//        - 1 on programmer/protocol errors (nil factory, missing payload,
//          malformed payload, marshal failure). A SubagentResult is STILL
//          emitted to stdout so the parent surfaces a useful diagnostic.
//
// Recursion contract: when HELIXCODE_SUBAGENT_NO_RECURSE=1 is present (set by
// the parent's SubprocessSpawner), the inner agent's tool registry MUST refuse
// to register the `task` tool — capping recursion depth at 1. T08 documents
// the contract; T07's TaskTool registration site honours it. We do NOT register
// the `task` tool in this child path because we are not building an agent
// runtime here — we run InProcessSpawner directly.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f15-subagent-team-design.md §4.2
// Plan: docs/superpowers/plans/2026-05-06-p1-f15-subagent-team.md T08
package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"dev.helix.code/internal/llm"
)

// SubagentLLMProviderFactory is the closure main.go supplies to RunAsSubagent
// for constructing the child's LLM provider. The factory MUST honour the
// supplied context for cancellation (e.g. quick cred-load timeouts). Returning
// (nil, err) is acceptable; RunAsSubagent surfaces the error as a StateFailed
// SubagentResult on stdout.
type SubagentLLMProviderFactory func(ctx context.Context) (llm.Provider, error)

// IsSubagentInvocation reports whether the current process should run as the
// subagent helper child. main.go (T08) calls this at startup BEFORE any other
// helper dispatch (notably before sandbox.IsHelperInvocation()) and, when
// true, calls RunAsSubagent and os.Exit's with the returned code.
//
// Returns true ONLY when SubagentHelperEnvVar=="1". Other truthy-looking
// values ("yes", "true", "0") all return false — the parent always sets "1"
// explicitly, so a strict equality check rejects accidental inheritance from
// outer shells.
func IsSubagentInvocation() bool {
	return os.Getenv(SubagentHelperEnvVar) == "1"
}

// RunAsSubagent is invoked from main.go when IsSubagentInvocation() returns
// true. See the file-level comment for the full contract.
//
// Returns 0 on successful protocol round-trip, 1 on protocol errors.
//
// Production callers pass os.Stdout as the implicit writer (handled below);
// tests use runAsSubagentWithWriter directly.
func RunAsSubagent(llmFactory SubagentLLMProviderFactory) int {
	return runAsSubagentWithWriter(os.Stdout, llmFactory)
}

// runAsSubagentWithWriter is the writer-injectable variant of RunAsSubagent.
// Splitting out the writer lets unit tests capture stdout cleanly into a
// bytes.Buffer without redirecting os.Stdout (which fights with the `go test`
// reporter).
func runAsSubagentWithWriter(w io.Writer, llmFactory SubagentLLMProviderFactory) int {
	startedAt := time.Now()

	// 1. Programmer error: nil factory. Emit a defensive failure result so a
	// hypothetical parent reading our stdout sees a clean SubagentResult
	// rather than empty bytes.
	if llmFactory == nil {
		_ = emitResult(w, SubagentResult{
			State:       StateFailed,
			Error:       "RunAsSubagent: llmFactory is nil (programmer error in main.go wiring)",
			StartedAt:   startedAt,
			CompletedAt: time.Now(),
		})
		return 1
	}

	// 2. Decode SubagentTask from the env-var payload. Missing or malformed
	// payload is a hard protocol error: emit a failure result + exit 1.
	task, decodeErr := decodeTaskFromEnv()
	if decodeErr != nil {
		// Best-effort: preserve task ID if we partially decoded one; otherwise
		// leave empty. decodeTaskFromEnv returns a zero task on error.
		_ = emitResult(w, SubagentResult{
			TaskID:      task.ID,
			State:       StateFailed,
			Error:       fmt.Sprintf("RunAsSubagent: %v", decodeErr),
			StartedAt:   startedAt,
			CompletedAt: time.Now(),
			Isolation:   task.Isolation,
		})
		return 1
	}

	// 3. Construct the LLM provider via the factory. Failures are SOFT: emit a
	// StateFailed result with exit 0 so the parent can surface a structured
	// failure to the user via the result channel.
	ctx := context.Background()
	provider, providerErr := llmFactory(ctx)
	if providerErr != nil {
		_ = emitResult(w, SubagentResult{
			TaskID:      task.ID,
			State:       StateFailed,
			Error:       fmt.Sprintf("failed to construct LLM provider: %v", providerErr),
			StartedAt:   startedAt,
			CompletedAt: time.Now(),
			Isolation:   task.Isolation,
		})
		return 0
	}
	if provider == nil {
		_ = emitResult(w, SubagentResult{
			TaskID:      task.ID,
			State:       StateFailed,
			Error:       "failed to construct LLM provider: factory returned (nil, nil)",
			StartedAt:   startedAt,
			CompletedAt: time.Now(),
			Isolation:   task.Isolation,
		})
		return 0
	}

	// 4. Run the subagent body via InProcessSpawner. Isolation is forced to
	// IsolationNone here: by the time we're in the helper child the parent
	// has already arranged whatever filesystem isolation it wanted (worktree
	// or none); a grandchild subprocess would be illegal under the recursion
	// guard anyway.
	result := runWithLLM(ctx, task, provider)

	// 5. Emit the result. JSON-marshal failure here is exceedingly unlikely
	// (SubagentResult is plain data) but we treat it as a hard protocol
	// error — write a minimal JSON to stdout and exit 1.
	if err := emitResult(w, result); err != nil {
		_, _ = fmt.Fprintf(w, `{"state":"failed","error":"emit result failed: %s"}`, err.Error())
		return 1
	}

	// 6. Even when State=StateFailed (provider returned an error mid-run), we
	// exit 0: the protocol round-trip succeeded, the failure is in-band.
	return 0
}

// decodeTaskFromEnv reads SubagentHelperPayloadEnvVar and JSON-decodes it into
// a SubagentTask. Returns a non-nil error on missing or malformed payload.
func decodeTaskFromEnv() (SubagentTask, error) {
	raw := os.Getenv(SubagentHelperPayloadEnvVar)
	if raw == "" {
		return SubagentTask{}, fmt.Errorf("missing payload env-var %s", SubagentHelperPayloadEnvVar)
	}
	var task SubagentTask
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		return SubagentTask{}, fmt.Errorf("decode payload from %s: %w", SubagentHelperPayloadEnvVar, err)
	}
	return task, nil
}

// emitResult marshals r and writes the JSON to w followed by a trailing
// newline. The newline is a courtesy for `helixcode | jq` and similar tools;
// the parent's SubprocessSpawner does bytes.TrimSpace before unmarshal.
func emitResult(w io.Writer, r SubagentResult) error {
	out, err := json.Marshal(&r)
	if err != nil {
		return fmt.Errorf("marshal SubagentResult: %w", err)
	}
	if _, err := w.Write(out); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

// runWithLLM executes the subagent body by constructing an InProcessSpawner
// and draining its (cap-1) result channel. Returns whatever SubagentResult
// the spawner emitted; if the spawner itself errors out (only possible on a
// nil-provider programmer error), synthesises a StateFailed result.
//
// The spawner closes its channel after sending; we receive once and rely on
// that contract. A defensive timeout is NOT applied here — the per-task
// timeout in `task.Timeout` is honoured by InProcessSpawner itself.
func runWithLLM(ctx context.Context, task SubagentTask, provider llm.Provider) SubagentResult {
	spawner := NewInProcessSpawner()
	ch, err := spawner.Spawn(ctx, task, provider)
	if err != nil {
		return SubagentResult{
			TaskID:      task.ID,
			State:       StateFailed,
			Error:       fmt.Sprintf("InProcessSpawner.Spawn: %v", err),
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
			Isolation:   task.Isolation,
		}
	}
	res, ok := <-ch
	if !ok {
		// Channel closed without a value — should not happen per the spawner
		// contract, but we guard against it.
		return SubagentResult{
			TaskID:      task.ID,
			State:       StateFailed,
			Error:       "InProcessSpawner channel closed without result",
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
			Isolation:   task.Isolation,
		}
	}
	return res
}

// Compile-time guard: SubagentLLMProviderFactory must be assignable to a
// func taking context.Context and returning (llm.Provider, error). If this
// breaks, the signature has drifted and main.go's wiring is broken.
var _ SubagentLLMProviderFactory = func(ctx context.Context) (llm.Provider, error) {
	return nil, errors.New("compile-time guard")
}
