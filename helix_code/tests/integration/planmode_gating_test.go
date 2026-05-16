//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/workflow/planmode"
)

func skipIfWindowsPM(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: shell-based plan-mode integration tests are POSIX-only on this branch")
	}
}

// TestPlanMode_BlocksUnapprovedShell exercises the full gating path: real
// ToolRegistry, real shell tool, real plan mode → real ErrPlanModeGated.
func TestPlanMode_BlocksUnapprovedShell(t *testing.T) {
	skipIfWindowsPM(t)
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	mc := planmode.NewModeController()
	p := planmode.NewDefaultPlanner()
	reg.SetPlanModeGate(planmode.NewToolGate(mc, p))

	require.NoError(t, mc.TransitionTo(planmode.ModePlan))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = reg.Execute(ctx, "shell", map[string]interface{}{"command": "echo blocked"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrPlanModeGated))
}

// TestPlanMode_AllowsApprovedShell submits + approves a plan with a matching
// shell action, then runs the shell tool — expects success and real output.
func TestPlanMode_AllowsApprovedShell(t *testing.T) {
	skipIfWindowsPM(t)
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	mc := planmode.NewModeController()
	p := planmode.NewDefaultPlanner()
	reg.SetPlanModeGate(planmode.NewToolGate(mc, p))

	require.NoError(t, mc.TransitionTo(planmode.ModePlan))
	plan := &planmode.Plan{
		ID:    "p1",
		Title: "T",
		Actions: []planmode.PlanAction{
			{ID: "a1", ToolName: "shell", Args: map[string]any{"command": "echo allowed"}, Description: "echo"},
		},
		Status: planmode.PlanPending,
	}
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApprovePlan("p1"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := reg.Execute(ctx, "shell", map[string]interface{}{"command": "echo allowed"})
	require.NoError(t, err)
	rs := planModeResultString(res)
	assert.Contains(t, rs, "allowed")
}

// TestPlanMode_ExitReturnsToNormal: ExitPlanMode tool runs, then a previously-
// blocked shell call now succeeds.
func TestPlanMode_ExitReturnsToNormal(t *testing.T) {
	skipIfWindowsPM(t)
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	mc := planmode.NewModeController()
	p := planmode.NewDefaultPlanner()
	reg.SetPlanModeGate(planmode.NewToolGate(mc, p))

	exitTool := tools.NewExitPlanModeTool(mc)
	reg.Register(exitTool)

	require.NoError(t, mc.TransitionTo(planmode.ModePlan))
	_, err = exitTool.Execute(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, planmode.ModeNormal, mc.GetMode())

	// Shell call now succeeds without an approved plan.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := reg.Execute(ctx, "shell", map[string]interface{}{"command": "echo unblocked"})
	require.NoError(t, err)
	assert.Contains(t, planModeResultString(res), "unblocked")
}

// planModeResultString extracts the stdout/output text from a shell tool result.
// The foreground ShellTool returns a *shell.ExecutionResult struct, so we use
// fmt.Sprintf to coerce to string and then grep for the relevant field.
func planModeResultString(v interface{}) string {
	if v == nil {
		return ""
	}
	// Map-based result (background shell task or similar)
	if m, ok := v.(map[string]interface{}); ok {
		for _, key := range []string{"output", "stdout", "Stdout"} {
			if s, ok := m[key].(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	// Struct-based result — reflect via fmt to get a string we can scan
	s := fmt.Sprintf("%+v", v)
	return strings.TrimSpace(s)
}
