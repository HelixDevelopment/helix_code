package builtin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/workflow"
)

func TestRegisterBuiltinCommandsWithTasks(t *testing.T) {
	registry := commands.NewRegistry()
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	defer bm.Close()
	require.NoError(t, builtin.RegisterBuiltinCommandsWithTasks(registry, bm))

	cmd, ok := registry.Get("tasks")
	require.True(t, ok, "/tasks command must be registered")
	assert.Equal(t, "tasks", cmd.Name())
}
