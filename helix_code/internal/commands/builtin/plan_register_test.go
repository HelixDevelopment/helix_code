package builtin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/workflow/planmode"
)

func TestRegisterBuiltinCommandsWithPlanMode(t *testing.T) {
	registry := commands.NewRegistry()
	planner := planmode.NewDefaultPlanner()
	mc := planmode.NewModeController()
	require.NoError(t, builtin.RegisterBuiltinCommandsWithPlanMode(registry, planner, mc))

	cmd, ok := registry.Get("plan")
	require.True(t, ok, "/plan command must be registered")
	assert.Equal(t, "plan", cmd.Name())
}
