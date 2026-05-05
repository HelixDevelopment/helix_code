package builtin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/hooks"
)

func TestRegisterBuiltinCommands_IncludesHooks(t *testing.T) {
	registry := commands.NewRegistry()
	mgr := hooks.NewManager()
	require.NoError(t, builtin.RegisterBuiltinCommandsWithHooks(registry, mgr))

	cmd, ok := registry.Get("hooks")
	require.True(t, ok)
	assert.Equal(t, "hooks", cmd.Name())

	cmd2, ok := registry.Get("hk")
	require.True(t, ok)
	assert.Equal(t, "hooks", cmd2.Name(), "alias resolves to hooks")
}
