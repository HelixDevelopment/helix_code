package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/workflow/planmode"
)

func newRegistryWithGate(t *testing.T) (*ToolRegistry, planmode.ModeController, *planmode.DefaultPlanner) {
	t.Helper()
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	mc := planmode.NewModeController()
	p := planmode.NewDefaultPlanner()
	r.SetPlanModeGate(planmode.NewToolGate(mc, p))
	return r, mc, p
}

func TestRegistry_PlanModeGateBlocksDestructive(t *testing.T) {
	r, mc, _ := newRegistryWithGate(t)
	tool := &fakePlainTool{name: "Edit", finalResult: "ok"}
	r.Register(tool)
	require.NoError(t, mc.TransitionTo(planmode.ModePlan))

	_, err := r.Execute(context.Background(), "Edit", map[string]interface{}{"file_path": "x.go"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPlanModeGated))
}

func TestRegistry_PlanModeGateAllowsAllowList(t *testing.T) {
	r, mc, _ := newRegistryWithGate(t)
	tool := &fakePlainTool{name: "Read", finalResult: "content"}
	r.Register(tool)
	require.NoError(t, mc.TransitionTo(planmode.ModePlan))

	res, err := r.Execute(context.Background(), "Read", map[string]interface{}{"file_path": "x.go"})
	require.NoError(t, err)
	assert.Equal(t, "content", res)
}

func TestRegistry_PlanModeGateAllowsApprovedAction(t *testing.T) {
	r, mc, p := newRegistryWithGate(t)
	tool := &fakePlainTool{name: "Edit", finalResult: "edited"}
	r.Register(tool)
	require.NoError(t, mc.TransitionTo(planmode.ModePlan))

	plan := &planmode.Plan{
		ID:    "p1",
		Title: "T",
		Actions: []planmode.PlanAction{
			{ID: "a1", ToolName: "Edit", Args: map[string]any{"file_path": "foo.go"}},
		},
		Status: planmode.PlanPending,
	}
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApprovePlan("p1"))

	res, err := r.Execute(context.Background(), "Edit", map[string]interface{}{"file_path": "foo.go"})
	require.NoError(t, err)
	assert.Equal(t, "edited", res)

	// Re-execute the same approved action — now blocked because MarkExecuted ran.
	_, err = r.Execute(context.Background(), "Edit", map[string]interface{}{"file_path": "foo.go"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPlanModeGated))
}

func TestRegistry_PlanModeGateNormalModePassesThrough(t *testing.T) {
	r, _, _ := newRegistryWithGate(t)
	tool := &fakePlainTool{name: "Edit", finalResult: "ok"}
	r.Register(tool)
	// mode is Normal — gate is consulted but returns false.

	res, err := r.Execute(context.Background(), "Edit", map[string]interface{}{"file_path": "x.go"})
	require.NoError(t, err)
	assert.Equal(t, "ok", res)
}
