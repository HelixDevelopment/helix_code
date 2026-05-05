package planmode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newApprovalPlannerForTest returns a fresh DefaultPlanner for approval tests.
func newApprovalPlannerForTest(t *testing.T) ApprovalPlanner {
	t.Helper()
	return NewDefaultPlanner()
}

// sampleApprovalPlan returns a minimal Plan with two PlanActions for approval tests.
func sampleApprovalPlan(id string) *Plan {
	return &Plan{
		ID:    id,
		Title: "test plan",
		Actions: []PlanAction{
			{ID: "a1", ToolName: "Edit", Args: map[string]any{"file_path": "foo.go"}},
			{ID: "a2", ToolName: "Bash", Args: map[string]any{"command": "go test ./..."}},
		},
		Status: PlanPending,
	}
}

func TestPlanner_ApprovePlanMarksAllApproved(t *testing.T) {
	p := newApprovalPlannerForTest(t)
	plan := sampleApprovalPlan("p1")
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApprovePlan("p1"))

	got, err := p.GetPlan("p1")
	require.NoError(t, err)
	assert.Equal(t, PlanApproved, got.Status)
	for _, a := range got.Actions {
		require.NotNil(t, a.Approved)
		assert.True(t, *a.Approved, "action %s must be approved", a.ID)
	}
}

func TestPlanner_ApproveAction_OneOnly(t *testing.T) {
	p := newApprovalPlannerForTest(t)
	plan := sampleApprovalPlan("p2")
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.ApproveAction("p2", "a1"))

	got, err := p.GetPlan("p2")
	require.NoError(t, err)
	require.NotNil(t, got.Actions[0].Approved)
	assert.True(t, *got.Actions[0].Approved)
	assert.Nil(t, got.Actions[1].Approved)
}

func TestPlanner_RejectPlanSetsStatus(t *testing.T) {
	p := newApprovalPlannerForTest(t)
	plan := sampleApprovalPlan("p3")
	require.NoError(t, p.SubmitPlan(plan))
	require.NoError(t, p.RejectPlan("p3"))

	got, err := p.GetPlan("p3")
	require.NoError(t, err)
	assert.Equal(t, PlanRejected, got.Status)
}

func TestPlanner_ActivePlanReturnsLatestPending(t *testing.T) {
	p := newApprovalPlannerForTest(t)
	require.NoError(t, p.SubmitPlan(sampleApprovalPlan("p1")))
	require.NoError(t, p.SubmitPlan(sampleApprovalPlan("p2")))

	active := p.ActivePlan()
	require.NotNil(t, active)
	assert.Contains(t, []string{"p1", "p2"}, active.ID)
}

func TestPlanner_ApproveUnknownPlanErrors(t *testing.T) {
	p := newApprovalPlannerForTest(t)
	err := p.ApprovePlan("nonexistent")
	require.Error(t, err)
}
