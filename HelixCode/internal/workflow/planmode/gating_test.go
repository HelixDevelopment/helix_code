package planmode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGateForTest(t *testing.T) (*ToolGate, ModeController, *DefaultPlanner) {
	t.Helper()
	mc := NewModeController()
	p := NewDefaultPlanner()
	return NewToolGate(mc, p), mc, p
}

func TestToolGate_NormalModeNeverBlocks(t *testing.T) {
	g, _, _ := newGateForTest(t)
	blocked, _ := g.IsBlocked("Edit", map[string]any{"file_path": "x"})
	assert.False(t, blocked)
}

func TestToolGate_AllowListPasses(t *testing.T) {
	g, mc, _ := newGateForTest(t)
	require.NoError(t, mc.TransitionTo(ModePlan))
	for _, name := range []string{"Read", "Glob", "Grep", "View", "TaskOutput"} {
		blocked, _ := g.IsBlocked(name, nil)
		assert.False(t, blocked, "%s should be in allow-list", name)
	}
}

func TestToolGate_NoActivePlanBlocks(t *testing.T) {
	g, mc, _ := newGateForTest(t)
	require.NoError(t, mc.TransitionTo(ModePlan))
	blocked, reason := g.IsBlocked("Edit", map[string]any{"file_path": "x"})
	assert.True(t, blocked)
	assert.Contains(t, reason, "no active plan")
}

func TestToolGate_ApprovedActionAllows(t *testing.T) {
	g, mc, p := newGateForTest(t)
	require.NoError(t, mc.TransitionTo(ModePlan))
	plan := &Plan{
		ID:    "p1",
		Title: "T",
		Actions: []PlanAction{
			{ID: "a1", ToolName: "Edit", Args: map[string]any{"file_path": "foo.go"}},
		},
		Status: PlanPending,
	}
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApprovePlan("p1"))

	blocked, _ := g.IsBlocked("Edit", map[string]any{"file_path": "foo.go"})
	assert.False(t, blocked)
}

func TestToolGate_KeyArgMismatchBlocks(t *testing.T) {
	g, mc, p := newGateForTest(t)
	require.NoError(t, mc.TransitionTo(ModePlan))
	plan := &Plan{
		ID: "p1",
		Actions: []PlanAction{
			{ID: "a1", ToolName: "Edit", Args: map[string]any{"file_path": "foo.go"}},
		},
		Status: PlanPending,
	}
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApprovePlan("p1"))

	blocked, reason := g.IsBlocked("Edit", map[string]any{"file_path": "OTHER.go"})
	assert.True(t, blocked)
	assert.Contains(t, reason, "no approved")
}

func TestToolGate_ExecutedActionDoesNotReauthorise(t *testing.T) {
	g, mc, p := newGateForTest(t)
	require.NoError(t, mc.TransitionTo(ModePlan))
	plan := &Plan{
		ID: "p1",
		Actions: []PlanAction{
			{ID: "a1", ToolName: "Edit", Args: map[string]any{"file_path": "foo.go"}},
		},
		Status: PlanPending,
	}
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApprovePlan("p1"))

	planID, matched, ok := g.MatchApprovedAction("Edit", map[string]any{"file_path": "foo.go"})
	require.True(t, ok)
	g.MarkExecuted(planID, matched.ID)

	blocked, _ := g.IsBlocked("Edit", map[string]any{"file_path": "foo.go"})
	assert.True(t, blocked)
}

func TestToolGate_NoKeyArgFallsBackToNameOnly(t *testing.T) {
	g, mc, p := newGateForTest(t)
	require.NoError(t, mc.TransitionTo(ModePlan))
	plan := &Plan{
		ID: "p1",
		Actions: []PlanAction{
			{ID: "a1", ToolName: "CustomTool", Args: nil},
		},
		Status: PlanPending,
	}
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApprovePlan("p1"))

	blocked, _ := g.IsBlocked("CustomTool", map[string]any{"any": "thing"})
	assert.False(t, blocked)
}

func TestToolGate_WithAllowListExtends(t *testing.T) {
	g, mc, _ := newGateForTest(t)
	g2 := g.WithAllowList([]string{"MyCustomReader"})
	require.NoError(t, mc.TransitionTo(ModePlan))
	blocked, _ := g2.IsBlocked("MyCustomReader", nil)
	assert.False(t, blocked)
	// Original gate still has only defaults
	blocked2, _ := g.IsBlocked("MyCustomReader", nil)
	assert.True(t, blocked2)
}
