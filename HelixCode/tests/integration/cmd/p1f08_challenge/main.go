// p1f08_challenge runs the full plan-mode gating flow against a real registry
// and a real shell tool. Runtime-evidence harness for the F08 Challenge.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"dev.helix.code/internal/tools"
	shellpkg "dev.helix.code/internal/tools/shell"
	"dev.helix.code/internal/workflow/planmode"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	mc := planmode.NewModeController()
	planner := planmode.NewDefaultPlanner()
	gate := planmode.NewToolGate(mc, planner)
	reg.SetPlanModeGate(gate)

	ctx := context.Background()

	fmt.Println("==> step 1: transition to plan mode")
	if err := mc.TransitionTo(planmode.ModePlan); err != nil {
		return fmt.Errorf("transition: %w", err)
	}
	if mc.GetMode() != planmode.ModePlan {
		return fmt.Errorf("expected ModePlan, got %v", mc.GetMode())
	}
	fmt.Println("    mode =", mc.GetMode())

	fmt.Println("==> step 2: try shell echo hi without approved plan -> expect blocked")
	_, err = reg.Execute(ctx, "shell", map[string]interface{}{"command": "echo hi"})
	if err == nil {
		return fmt.Errorf("expected ErrPlanModeGated, got nil")
	}
	if !errors.Is(err, tools.ErrPlanModeGated) {
		return fmt.Errorf("expected ErrPlanModeGated, got: %v", err)
	}
	fmt.Println("    blocked correctly:", err)

	fmt.Println("==> step 3: submit + approve plan with shell echo hi action")
	plan := &planmode.Plan{
		ID:    "p1f08",
		Title: "F08 challenge",
		Actions: []planmode.PlanAction{
			{ID: "a1", ToolName: "shell", Args: map[string]any{"command": "echo hi"}, Description: "echo"},
		},
		Status: planmode.PlanPending,
	}
	if err := planner.SubmitPlan(plan); err != nil {
		return fmt.Errorf("submit: %w", err)
	}
	if err := planner.ApprovePlan("p1f08"); err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	fmt.Println("    plan approved.")

	fmt.Println("==> step 4: run shell echo hi -> expect success with 'hi' in output")
	res, err := reg.Execute(ctx, "shell", map[string]interface{}{"command": "echo hi"})
	if err != nil {
		return fmt.Errorf("expected success, got: %w", err)
	}
	out := outputString(res)
	fmt.Println("    output =", out)
	if !strings.Contains(out, "hi") {
		return fmt.Errorf("expected output to contain 'hi'; got %q", out)
	}

	fmt.Println("==> step 5: try shell echo bye -> expect blocked (key-arg mismatch)")
	_, err = reg.Execute(ctx, "shell", map[string]interface{}{"command": "echo bye"})
	if err == nil {
		return fmt.Errorf("expected ErrPlanModeGated for unapproved command, got nil")
	}
	if !errors.Is(err, tools.ErrPlanModeGated) {
		return fmt.Errorf("expected ErrPlanModeGated, got: %v", err)
	}
	fmt.Println("    blocked correctly (key-arg mismatch):", err)

	fmt.Println("==> step 6: ExitPlanMode -> shell echo bye now succeeds")
	exitTool := tools.NewExitPlanModeTool(mc)
	if _, err := exitTool.Execute(ctx, nil); err != nil {
		return fmt.Errorf("exit: %w", err)
	}
	res, err = reg.Execute(ctx, "shell", map[string]interface{}{"command": "echo bye"})
	if err != nil {
		return fmt.Errorf("expected success in normal mode, got: %w", err)
	}
	out = outputString(res)
	fmt.Println("    output =", out)
	if !strings.Contains(out, "bye") {
		return fmt.Errorf("expected output to contain 'bye'; got %q", out)
	}

	fmt.Println("==> P1-F08 challenge harness PASS")
	return nil
}

// outputString extracts text from the shell tool result. The ShellTool
// returns *shell.ExecutionResult from registry.Execute. We also handle the
// map[string]interface{}{"output": "..."} shape and a fmt.Sprintf fallback.
func outputString(v interface{}) string {
	if v == nil {
		return ""
	}
	// *shell.ExecutionResult — the actual type returned by ShellTool.Execute
	if r, ok := v.(*shellpkg.ExecutionResult); ok {
		return strings.TrimSpace(r.Stdout)
	}
	// map variant (future-proofing)
	if m, ok := v.(map[string]interface{}); ok {
		if s, ok := m["output"].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	// Fallback: stringify and look for substring matches
	return fmt.Sprintf("%+v", v)
}
