package worktree

import (
	"context"
	"fmt"

	"dev.helix.code/internal/tools"
)

// ─── EnterWorktreeTool ────────────────────────────────────────────────

// EnterWorktreeTool implements the EnterWorktree agent tool.
type EnterWorktreeTool struct{ m *Manager }

// NewEnterWorktreeTool wires a Manager into an EnterWorktree tool.
func NewEnterWorktreeTool(m *Manager) *EnterWorktreeTool { return &EnterWorktreeTool{m: m} }

func (t *EnterWorktreeTool) Name() string { return "EnterWorktree" }

func (t *EnterWorktreeTool) Description() string {
	return "Enter a named git worktree for isolated development. Creates the worktree if it doesn't exist (using the worktree name as the branch name when no base-branch is supplied; otherwise uses the supplied base-branch). Submodules are NOT initialised — the meta-repo and the inner Go module at HelixCode/ are present, but submodule directories under HelixAgent/, Dependencies/, etc. are empty (uninitialised). If your work needs submodule code, run `git submodule update --init --recursive` from inside the worktree using Bash."
}

func (t *EnterWorktreeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "Enter a named git worktree for isolated development.",
		Properties: map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Worktree name. Must match ^[a-zA-Z0-9._-]+$ and be ≤ 64 chars.",
			},
			"baseBranch": map[string]interface{}{
				"type":        "string",
				"description": "Optional. Existing branch to base the worktree on. Defaults to the worktree name.",
			},
		},
		Required: []string{"name"},
	}
}

func (t *EnterWorktreeTool) Category() tools.ToolCategory { return tools.CategoryShell }

func (t *EnterWorktreeTool) Validate(params map[string]interface{}) error {
	name, ok := params["name"].(string)
	if !ok {
		return fmt.Errorf("EnterWorktree: missing or non-string parameter 'name'")
	}
	if err := t.m.ValidateName(name); err != nil {
		return fmt.Errorf("EnterWorktree: %w", err)
	}
	if bb, present := params["baseBranch"]; present {
		if _, ok := bb.(string); !ok {
			return fmt.Errorf("EnterWorktree: 'baseBranch' must be a string")
		}
	}
	return nil
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	name := params["name"].(string)
	baseBranch, _ := params["baseBranch"].(string)
	path, err := t.m.EnterWorktree(ctx, name, baseBranch)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"path": path}, nil
}

// ─── ExitWorktreeTool ─────────────────────────────────────────────────

// ExitWorktreeTool implements the ExitWorktree agent tool.
type ExitWorktreeTool struct{ m *Manager }

// NewExitWorktreeTool wires a Manager into an ExitWorktree tool.
func NewExitWorktreeTool(m *Manager) *ExitWorktreeTool { return &ExitWorktreeTool{m: m} }

func (t *ExitWorktreeTool) Name() string        { return "ExitWorktree" }
func (t *ExitWorktreeTool) Description() string { return "Return to the main worktree. No-op when not in a worktree." }

func (t *ExitWorktreeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "Return to the main worktree.",
		Properties:  map[string]interface{}{},
		Required:    []string{},
	}
}

func (t *ExitWorktreeTool) Category() tools.ToolCategory        { return tools.CategoryShell }
func (t *ExitWorktreeTool) Validate(map[string]interface{}) error { return nil }

func (t *ExitWorktreeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	t.m.ExitWorktree()
	return map[string]interface{}{"exited": true}, nil
}

// ─── ListWorktreesTool ────────────────────────────────────────────────

// ListWorktreesTool implements the ListWorktrees agent tool.
type ListWorktreesTool struct{ m *Manager }

// NewListWorktreesTool wires a Manager into a ListWorktrees tool.
func NewListWorktreesTool(m *Manager) *ListWorktreesTool { return &ListWorktreesTool{m: m} }

func (t *ListWorktreesTool) Name() string { return "ListWorktrees" }
func (t *ListWorktreesTool) Description() string {
	return "List all helix-managed git worktrees under .helix-worktrees/. Returns name, absolute path, and best-effort branch."
}

func (t *ListWorktreesTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "List all helix-managed worktrees.",
		Properties:  map[string]interface{}{},
		Required:    []string{},
	}
}

func (t *ListWorktreesTool) Category() tools.ToolCategory        { return tools.CategoryShell }
func (t *ListWorktreesTool) Validate(map[string]interface{}) error { return nil }

func (t *ListWorktreesTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	wts, err := t.m.ListWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"worktrees": wts}, nil
}

// ─── RemoveWorktreeTool ───────────────────────────────────────────────

// RemoveWorktreeTool implements the RemoveWorktree agent tool.
type RemoveWorktreeTool struct{ m *Manager }

// NewRemoveWorktreeTool wires a Manager into a RemoveWorktree tool.
func NewRemoveWorktreeTool(m *Manager) *RemoveWorktreeTool { return &RemoveWorktreeTool{m: m} }

func (t *RemoveWorktreeTool) Name() string { return "RemoveWorktree" }
func (t *RemoveWorktreeTool) Description() string {
	return "Delete a helix-managed git worktree and unregister its branch. Refuses to remove the currently-active worktree."
}

func (t *RemoveWorktreeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "Remove a helix-managed worktree.",
		Properties: map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Worktree name to remove.",
			},
		},
		Required: []string{"name"},
	}
}

func (t *RemoveWorktreeTool) Category() tools.ToolCategory { return tools.CategoryShell }

func (t *RemoveWorktreeTool) Validate(params map[string]interface{}) error {
	name, ok := params["name"].(string)
	if !ok {
		return fmt.Errorf("RemoveWorktree: missing or non-string parameter 'name'")
	}
	if err := t.m.ValidateName(name); err != nil {
		return fmt.Errorf("RemoveWorktree: %w", err)
	}
	return nil
}

func (t *RemoveWorktreeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	name := params["name"].(string)
	if err := t.m.RemoveWorktree(ctx, name); err != nil {
		return nil, err
	}
	return map[string]interface{}{"removed": true}, nil
}
