package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/workflow/planmode"
)

func newPlanCommand(t *testing.T) (*PlanCommand, *planmode.DefaultPlanner, planmode.ModeController) {
	t.Helper()
	p := planmode.NewDefaultPlanner()
	mc := planmode.NewModeController()
	return NewPlanCommand(p, mc), p, mc
}

func samplePlanForTest() *planmode.Plan {
	return &planmode.Plan{
		ID:    "p1",
		Title: "Test Plan",
		Actions: []planmode.PlanAction{
			{ID: "a1", ToolName: "Edit", Args: map[string]any{"file_path": "foo.go"}, Description: "edit foo"},
			{ID: "a2", ToolName: "Bash", Args: map[string]any{"command": "go test ./..."}, Description: "run tests"},
		},
		Status: planmode.PlanPending,
	}
}

func TestSlashPlan_ShowEmptyPlan(t *testing.T) {
	c, _, _ := newPlanCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "No active plan")
}

func TestSlashPlan_ShowActivePlan(t *testing.T) {
	c, p, _ := newPlanCommand(t)
	require.NoError(t, p.SubmitPlan(samplePlanForTest()))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "Test Plan")
	assert.Contains(t, res.Output, "Edit")
	assert.Contains(t, res.Output, "Bash")
}

func TestSlashPlan_ApproveAll(t *testing.T) {
	c, p, _ := newPlanCommand(t)
	require.NoError(t, p.SubmitPlan(samplePlanForTest()))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"approve"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "approved")

	got, err := p.GetPlan("p1")
	require.NoError(t, err)
	assert.Equal(t, planmode.PlanApproved, got.Status)
}

func TestSlashPlan_ApproveSingleAction(t *testing.T) {
	c, p, _ := newPlanCommand(t)
	require.NoError(t, p.SubmitPlan(samplePlanForTest()))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"approve", "a1"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "a1")

	got, err := p.GetPlan("p1")
	require.NoError(t, err)
	require.NotNil(t, got.Actions[0].Approved)
	assert.True(t, *got.Actions[0].Approved)
	assert.Nil(t, got.Actions[1].Approved)
}

func TestSlashPlan_RejectExitsPlanMode(t *testing.T) {
	c, p, mc := newPlanCommand(t)
	require.NoError(t, mc.TransitionTo(planmode.ModePlan))
	require.NoError(t, p.SubmitPlan(samplePlanForTest()))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"reject"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "rejected")
	assert.Equal(t, planmode.ModeNormal, mc.GetMode())
}

func TestSlashPlan_StatusReportsMode(t *testing.T) {
	c, _, mc := newPlanCommand(t)
	require.NoError(t, mc.TransitionTo(planmode.ModePlan))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "Plan")
}

func TestSlashPlan_UnknownSubcommandErrors(t *testing.T) {
	c, _, _ := newPlanCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}
