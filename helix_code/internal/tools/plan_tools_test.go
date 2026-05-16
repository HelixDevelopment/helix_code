package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/workflow/planmode"
)

func TestEnterPlanModeTool_TransitionsToPlanMode(t *testing.T) {
	mc := planmode.NewModeController()
	tool := NewEnterPlanModeTool(mc)

	res, err := tool.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, planmode.ModePlan, mc.GetMode())
	m := res.(map[string]interface{})
	assert.Equal(t, "plan", m["mode"])
}

func TestEnterPlanModeTool_FromInvalidStateErrors(t *testing.T) {
	mc := planmode.NewModeController()
	require.NoError(t, mc.TransitionTo(planmode.ModePlan))
	require.NoError(t, mc.TransitionTo(planmode.ModeAct))
	tool := NewEnterPlanModeTool(mc)

	// Mode Act -> Plan is not a valid transition (per existing transition table)
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	require.Error(t, err)
}

func TestExitPlanModeTool_TransitionsToNormal(t *testing.T) {
	mc := planmode.NewModeController()
	require.NoError(t, mc.TransitionTo(planmode.ModePlan))

	tool := NewExitPlanModeTool(mc)
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, planmode.ModeNormal, mc.GetMode())
}

func TestErrPlanModeGated_IsExported(t *testing.T) {
	assert.NotNil(t, ErrPlanModeGated)
	assert.Contains(t, ErrPlanModeGated.Error(), "plan mode")
}

func TestEnterExitPlanModeTools_ImplementToolInterface(t *testing.T) {
	var _ Tool = (*EnterPlanModeTool)(nil)
	var _ Tool = (*ExitPlanModeTool)(nil)
}
