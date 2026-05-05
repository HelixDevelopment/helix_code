package tools

import (
	"context"
	"fmt"

	"dev.helix.code/internal/workflow/planmode"
)

// EnterPlanModeTool is the agent-callable tool that switches the agent into
// plan mode. Side-effect: ModeController.TransitionTo(ModePlan).
type EnterPlanModeTool struct {
	mc planmode.ModeController
}

// NewEnterPlanModeTool returns the agent-callable EnterPlanMode tool.
func NewEnterPlanModeTool(mc planmode.ModeController) *EnterPlanModeTool {
	return &EnterPlanModeTool{mc: mc}
}

func (t *EnterPlanModeTool) Name() string { return "EnterPlanMode" }
func (t *EnterPlanModeTool) Description() string {
	return "Enter plan mode: a read-only operational mode where destructive tools are blocked unless authorised by an approved plan action."
}
func (t *EnterPlanModeTool) Category() ToolCategory { return CategoryFileSystem }
func (t *EnterPlanModeTool) Schema() ToolSchema {
	return ToolSchema{
		Type:        "object",
		Description: "EnterPlanMode takes no parameters.",
		Properties:  map[string]interface{}{},
		Required:    []string{},
	}
}
func (t *EnterPlanModeTool) Validate(params map[string]interface{}) error { return nil }
func (t *EnterPlanModeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.mc.TransitionTo(planmode.ModePlan); err != nil {
		return nil, fmt.Errorf("enter plan mode: %w", err)
	}
	return map[string]interface{}{
		"mode":    "plan",
		"message": "Plan mode active. Propose a plan via the planner; user approves via /plan approve.",
	}, nil
}

// ExitPlanModeTool returns the agent to normal execution.
type ExitPlanModeTool struct {
	mc planmode.ModeController
}

// NewExitPlanModeTool returns the agent-callable ExitPlanMode tool.
func NewExitPlanModeTool(mc planmode.ModeController) *ExitPlanModeTool {
	return &ExitPlanModeTool{mc: mc}
}

func (t *ExitPlanModeTool) Name() string { return "ExitPlanMode" }
func (t *ExitPlanModeTool) Description() string {
	return "Exit plan mode and return to normal execution. Optional allowed_prompts param (claude-code parity, currently informational)."
}
func (t *ExitPlanModeTool) Category() ToolCategory { return CategoryFileSystem }
func (t *ExitPlanModeTool) Schema() ToolSchema {
	return ToolSchema{
		Type:        "object",
		Description: "ExitPlanMode optionally takes allowed_prompts; the gate ignores it today.",
		Properties: map[string]interface{}{
			"allowed_prompts": map[string]interface{}{
				"type":        "array",
				"description": "User prompts that would re-enter plan mode (informational; consult F05 hooks for enforcement).",
			},
		},
	}
}
func (t *ExitPlanModeTool) Validate(params map[string]interface{}) error { return nil }
func (t *ExitPlanModeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.mc.TransitionTo(planmode.ModeNormal); err != nil {
		return nil, fmt.Errorf("exit plan mode: %w", err)
	}
	return map[string]interface{}{
		"mode":    "normal",
		"message": "Returned to normal execution.",
	}, nil
}

// Compile-time interface assertions.
var (
	_ Tool = (*EnterPlanModeTool)(nil)
	_ Tool = (*ExitPlanModeTool)(nil)
)
