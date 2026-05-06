// Package sandbox — sandboxed_shell_tool.go (P1-F14-T07).
//
// SandboxedShellTool is the agent-callable Tool that runs shell commands
// inside the F14 sandbox. It is registered with the tool registry under the
// stable name "shell_sandboxed" and category tools.CategorySandbox.
//
// The tool depends on the SandboxedShellExecutor seam — a subset of
// *SandboxManager — so it can be exercised in unit tests with a fake without
// needing a live bwrap/userns setup. SandboxManager itself satisfies the seam
// in production via its Execute + Capabilities methods.
//
// Argument shape (JSON, map[string]interface{} via Tool.Execute):
//
//	command          string  required;  shell command to run
//	network          bool    optional;  default false (DENY)
//	timeout_seconds  int     optional;  default 30; range [1, 600]
//	memory_limit_mb  int     optional;  default 0 (no limit); >= 0
//
// Anti-bluff contract: this tool NEVER simulates execution. It always routes
// to a real SandboxManager.Execute (or, in tests, a fake that records the
// dispatch). DenyError and FailClosedError surface verbatim to the caller so
// the agent sees the matched-rule / install-hint text the user needs.
package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

// SandboxedShellExecutor is the subset of *SandboxManager that the tool
// depends on. Defining it here keeps the tool testable with a fake. Both
// methods MUST behave identically to the *SandboxManager implementations:
// Execute applies CONST-033 deny + user deny + fail-closed checks before
// dispatch; Capabilities is a cheap snapshot.
type SandboxedShellExecutor interface {
	Execute(ctx context.Context, command string, policy SandboxPolicy) (*SandboxResult, error)
	Capabilities() SandboxCapabilities
}

// SandboxedShellTool is the Tool implementation registered as
// "shell_sandboxed". A nil executor is rejected at Execute time with a clear
// error rather than at construction so the registry can wire the tool before
// the manager is fully constructed (mirrors the LSP-tool pattern).
type SandboxedShellTool struct {
	executor SandboxedShellExecutor
}

// NewSandboxedShellTool wires the tool to a SandboxManager (or any other
// SandboxedShellExecutor implementation, e.g. a test fake).
func NewSandboxedShellTool(m SandboxedShellExecutor) *SandboxedShellTool {
	return &SandboxedShellTool{executor: m}
}

// Name returns the stable tool name for registry registration.
func (t *SandboxedShellTool) Name() string { return "shell_sandboxed" }

// RequiresApproval — even sandboxed, this is still a process exec. Per spec
// §3.6, the "sandboxing is optional in auto-edit, required in full-auto"
// composition is enforced by the policy layer, not by downgrading the level.
func (t *SandboxedShellTool) RequiresApproval() approval.ApprovalLevel {
	return approval.LevelRun
}

// Description is shown to the agent so it knows when to call this tool.
func (t *SandboxedShellTool) Description() string {
	return "Run a shell command inside an isolated sandbox. Network is denied by default; pass network=true to opt in. Honours CONST-033 (host power-management ban) and user-configured deny rules. Returns stdout, stderr, exit code, timeout flag, and the backend used."
}

// Category returns tools.CategorySandbox so registry filtering by category
// surfaces sandbox-only tools together.
func (t *SandboxedShellTool) Category() tools.ToolCategory { return tools.CategorySandbox }

// Schema returns the JSON schema for the tool's args. The shape matches the
// argument contract in the package doc comment above.
func (t *SandboxedShellTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to run inside the sandbox. Must be non-empty.",
			},
			"network": map[string]interface{}{
				"type":        "boolean",
				"description": "Optional. Allow network access from inside the sandbox. Default false (DENY).",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Optional. Timeout in seconds. Default 30. Range [1, 600].",
				"minimum":     1,
				"maximum":     600,
			},
			"memory_limit_mb": map[string]interface{}{
				"type":        "integer",
				"description": "Optional. Memory limit in MiB. Default 0 (no limit). Must be >= 0.",
				"minimum":     0,
			},
		},
		Required:    []string{"command"},
		Description: "Run a shell command inside the F14 sandbox.",
	}
}

// Validate enforces the args contract before Execute. The registry calls
// this before dispatch (registry.Execute path); we also call it defensively
// from inside Execute so direct callers (bypassing the registry) get the
// same protection.
func (t *SandboxedShellTool) Validate(params map[string]interface{}) error {
	rawCmd, ok := params["command"]
	if !ok {
		return fmt.Errorf("command is required")
	}
	cmdStr, isString := rawCmd.(string)
	if !isString {
		return fmt.Errorf("command must be a string, got %T", rawCmd)
	}
	if strings.TrimSpace(cmdStr) == "" {
		return fmt.Errorf("command must not be empty")
	}

	if v, present := params["network"]; present {
		if _, isBool := v.(bool); !isBool {
			return fmt.Errorf("network must be a boolean, got %T", v)
		}
	}

	if v, present := params["timeout_seconds"]; present {
		secs, ok := toInt(v)
		if !ok {
			return fmt.Errorf("timeout_seconds must be an integer, got %T", v)
		}
		if secs < 1 || secs > 600 {
			return fmt.Errorf("timeout_seconds must be in range [1, 600], got %d", secs)
		}
	}

	if v, present := params["memory_limit_mb"]; present {
		mb, ok := toInt(v)
		if !ok {
			return fmt.Errorf("memory_limit_mb must be an integer, got %T", v)
		}
		if mb < 0 {
			return fmt.Errorf("memory_limit_mb must be >= 0, got %d", mb)
		}
	}

	return nil
}

// Execute builds a SandboxPolicy from args (default policy + caller
// overrides), dispatches to the executor, and returns the SandboxResult on
// success. DenyError and FailClosedError are wrapped — but not stripped —
// so callers can both read a friendly message via err.Error() and assert
// against the typed errors with errors.As / errors.Is.
func (t *SandboxedShellTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	if t.executor == nil {
		return nil, fmt.Errorf("sandboxed shell tool: no executor wired")
	}

	command, _ := params["command"].(string)

	policy := DefaultSandboxPolicy()

	if v, present := params["network"]; present {
		if b, ok := v.(bool); ok {
			policy.NetworkAllowed = b
		}
	}
	if v, present := params["timeout_seconds"]; present {
		if secs, ok := toInt(v); ok && secs > 0 {
			policy.Timeout = time.Duration(secs) * time.Second
		}
	}
	if v, present := params["memory_limit_mb"]; present {
		if mb, ok := toInt(v); ok && mb >= 0 {
			policy.MemoryLimitMB = mb
		}
	}

	result, err := t.executor.Execute(ctx, command, policy)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// toInt converts a JSON-decoded numeric value into an int. JSON numbers
// usually arrive as float64 (encoding/json default) or json.Number, but
// in-process callers may pass int / int32 / int64 directly. Anything else
// returns (0, false) so Validate can surface a clear type error.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		// Reject non-integer floats so 1.5 is not silently truncated.
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
