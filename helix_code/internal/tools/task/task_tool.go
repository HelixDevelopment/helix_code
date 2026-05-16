// Package task — task_tool.go (P1-F15-T07).
//
// TaskTool dispatches subagents via a SubagentManager and synchronously
// returns the SubagentResult. It is registered with the tool registry under
// the stable name "task" (claude-code-compatible) and category
// tools.CategorySubagent.
//
// The tool depends on the TaskExecutor seam — a subset of
// *subagent.SubagentManager — so it can be exercised in unit tests with a fake
// without spinning up real spawners. SubagentManager itself satisfies the
// seam in production via its Dispatch + WaitAll methods.
//
// Argument shape (JSON, map[string]interface{} via Tool.Execute):
//
//	description      string  required;  3-5 word task summary
//	prompt           string  required;  inner agent's prompt
//	isolation        string  optional;  "none" (default) | "worktree"
//	subagent_type    string  optional;  profile name
//	timeout_seconds  int     optional;  range [1, 1800]
//	base_branch      string  optional;  worktree base branch
//	merge_on_success bool    optional;  worktree merge flag
//
// Anti-bluff contract: this tool NEVER fabricates a SubagentResult. It always
// routes to the real SubagentManager (or, in tests, a fake that records the
// dispatch). Dispatch errors and WaitAll errors surface verbatim so the
// caller can both read a friendly message via err.Error() and assert against
// typed errors with errors.Is / errors.As.
//
// Placement note: this tool lives in `internal/tools/task/` (a subpackage)
// rather than directly in `internal/tools/` to avoid a test-only import
// cycle through internal/tools/worktree, which itself imports internal/tools.
// The pattern mirrors F14's `internal/tools/sandbox/sandboxed_shell_tool.go`.
package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

// taskTimeoutMinSeconds and taskTimeoutMaxSeconds bound the optional
// timeout_seconds argument. The 1800s ceiling matches subagent.HardTaskTimeoutCeiling
// (30 minutes) per F15 spec — values above are clamped by the manager anyway,
// but we reject early so the agent gets a clear validation error rather than
// a silent clamp.
const (
	taskTimeoutMinSeconds = 1
	taskTimeoutMaxSeconds = 1800
)

// TaskExecutor is the subset of *subagent.SubagentManager that TaskTool
// depends on. Defining the interface in this package keeps the tool testable
// with a fake (hexagonal seam).
type TaskExecutor interface {
	Dispatch(ctx context.Context, task subagent.SubagentTask) (string, error)
	WaitAll(ctx context.Context, taskIDs []string) ([]subagent.SubagentResult, error)
}

// TaskTool is the Tool implementation registered as "task". A nil executor
// is rejected at Execute time with a clear error rather than at construction
// so the registry can wire the tool before the manager is fully constructed
// (mirrors the LSP/sandbox tool patterns).
type TaskTool struct {
	executor TaskExecutor
}

// NewTaskTool wires the tool to a SubagentManager (or any TaskExecutor — e.g.
// a unit-test fake).
func NewTaskTool(executor TaskExecutor) *TaskTool {
	return &TaskTool{executor: executor}
}

// Name returns "task" — the claude-code-compatible name. Keeping the name
// stable across CLI agents lets CLAUDE.md / AGENTS.md prompts that mention
// "use the task tool to dispatch a subagent" work without per-CLI rewrites.
func (t *TaskTool) Name() string { return "task" }

// RequiresApproval — recursive agent spawn. Per spec §3.6, subagent dispatch
// is LevelAll because the inner agent can re-trigger any tool — gating it
// behind ModeDangerous ensures only "dangerously-bypass" permits unattended
// recursive use.
func (t *TaskTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelAll }

// Description is shown to the agent so it knows when to call this tool. The
// word "subagent" appears verbatim — TestTaskTool_DescriptionMentionsSubagent
// asserts against it as an anti-bluff check that the description still
// reflects the tool's purpose.
func (t *TaskTool) Description() string {
	return "Dispatch a subagent to handle an independent subtask. Blocks until the subagent completes and returns its full SubagentResult (state, output, duration, worktree info if isolation=worktree). Use this when work can be done in parallel or in isolation from the main conversation."
}

// Category returns tools.CategorySubagent so registry filtering by category
// surfaces subagent-related tools together.
func (t *TaskTool) Category() tools.ToolCategory { return tools.CategorySubagent }

// Schema returns the JSON schema for the tool's args. The shape matches the
// argument contract in the package doc comment above.
func (t *TaskTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Short (3-5 word) summary of the subtask. Required. Shown in /subagents listings.",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "The full prompt the subagent will execute. Required. Should be self-contained: the subagent does not see the parent's conversation history.",
			},
			"isolation": map[string]interface{}{
				"type":        "string",
				"description": "Optional. Sandboxing mode: \"none\" (in-process goroutine, shared cwd) or \"worktree\" (subprocess in a git worktree). Default \"none\".",
				"enum":        []string{"none", "worktree"},
			},
			"subagent_type": map[string]interface{}{
				"type":        "string",
				"description": "Optional. Subagent profile name (e.g. \"code-reviewer\", \"researcher\").",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Optional. Per-task timeout in seconds. Range [1, 1800]. Default applied by the manager (300s).",
				"minimum":     taskTimeoutMinSeconds,
				"maximum":     taskTimeoutMaxSeconds,
			},
			"base_branch": map[string]interface{}{
				"type":        "string",
				"description": "Optional. For isolation=worktree: branch the worktree is forked from.",
			},
			"merge_on_success": map[string]interface{}{
				"type":        "boolean",
				"description": "Optional. For isolation=worktree: when true, the worktree's diff is merged back on a successful run.",
			},
		},
		Required:    []string{"description", "prompt"},
		Description: "Dispatch a subagent and block until it returns a SubagentResult.",
	}
}

// Validate enforces the args contract before Execute. The registry calls
// this before dispatch (registry.Execute path); we also call it defensively
// from inside Execute so direct callers (bypassing the registry) get the
// same protection.
func (t *TaskTool) Validate(params map[string]interface{}) error {
	rawDesc, ok := params["description"]
	if !ok {
		return fmt.Errorf("description is required")
	}
	descStr, isString := rawDesc.(string)
	if !isString {
		return fmt.Errorf("description must be a string, got %T", rawDesc)
	}
	if strings.TrimSpace(descStr) == "" {
		return fmt.Errorf("description must not be empty")
	}

	rawPrompt, ok := params["prompt"]
	if !ok {
		return fmt.Errorf("prompt is required")
	}
	promptStr, isString := rawPrompt.(string)
	if !isString {
		return fmt.Errorf("prompt must be a string, got %T", rawPrompt)
	}
	if strings.TrimSpace(promptStr) == "" {
		return fmt.Errorf("prompt must not be empty")
	}

	if v, present := params["isolation"]; present {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("isolation must be a string, got %T", v)
		}
		switch s {
		case "none", "worktree":
			// ok
		default:
			return fmt.Errorf("isolation must be one of [\"none\", \"worktree\"], got %q", s)
		}
	}

	if v, present := params["subagent_type"]; present {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("subagent_type must be a string, got %T", v)
		}
	}

	if v, present := params["timeout_seconds"]; present {
		secs, ok := toTaskInt(v)
		if !ok {
			return fmt.Errorf("timeout_seconds must be an integer, got %T", v)
		}
		if secs < taskTimeoutMinSeconds || secs > taskTimeoutMaxSeconds {
			return fmt.Errorf("timeout_seconds must be in range [%d, %d], got %d",
				taskTimeoutMinSeconds, taskTimeoutMaxSeconds, secs)
		}
	}

	if v, present := params["base_branch"]; present {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("base_branch must be a string, got %T", v)
		}
	}

	if v, present := params["merge_on_success"]; present {
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("merge_on_success must be a boolean, got %T", v)
		}
	}

	return nil
}

// Execute builds a SubagentTask from args, dispatches it via the executor,
// and blocks on WaitAll for the single resulting ID. Errors from Dispatch
// and WaitAll are wrapped with descriptive context but the underlying
// sentinel error (e.g. subagent.ErrMaxConcurrency) is preserved so callers
// can match with errors.Is.
//
// Returns the single SubagentResult on success.
func (t *TaskTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	if t.executor == nil {
		return nil, errors.New("task tool: no executor wired")
	}

	saTask := subagent.SubagentTask{
		Description: params["description"].(string),
		Prompt:      params["prompt"].(string),
	}

	if v, ok := params["isolation"].(string); ok {
		saTask.Isolation = subagent.Isolation(v)
	} else {
		saTask.Isolation = subagent.IsolationNone
	}

	if v, ok := params["subagent_type"].(string); ok {
		saTask.SubagentType = v
	}

	if v, present := params["timeout_seconds"]; present {
		if secs, ok := toTaskInt(v); ok {
			saTask.Timeout = time.Duration(secs) * time.Second
		}
	}

	if v, ok := params["base_branch"].(string); ok {
		saTask.BaseBranch = v
	}

	if v, ok := params["merge_on_success"].(bool); ok {
		saTask.MergeOnSuccess = v
	}

	id, err := t.executor.Dispatch(ctx, saTask)
	if err != nil {
		// Friendly wrapping for the common max-concurrency case while
		// preserving errors.Is matchability via %w.
		if errors.Is(err, subagent.ErrMaxConcurrency) {
			return nil, fmt.Errorf("max subagent concurrency reached; retry later: %w", err)
		}
		return nil, fmt.Errorf("task dispatch failed: %w", err)
	}

	results, err := t.executor.WaitAll(ctx, []string{id})
	if err != nil {
		return nil, fmt.Errorf("task wait failed: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("task wait returned no results for id %q", id)
	}
	return results[0], nil
}

// toTaskInt converts a JSON-decoded numeric value into an int. JSON numbers
// usually arrive as float64 (encoding/json default), but in-process callers
// may pass int / int32 / int64 directly. Anything else (including non-integer
// floats) returns (0, false) so Validate can surface a clear type error.
func toTaskInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int64(n)) {
			return 0, false
		}
		return int(n), true
	case float32:
		f := float64(n)
		if f != float64(int64(f)) {
			return 0, false
		}
		return int(f), true
	default:
		return 0, false
	}
}
