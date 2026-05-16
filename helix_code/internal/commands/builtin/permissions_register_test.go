package builtin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
)

func TestRegisterBuiltinCommands_IncludesPermissions(t *testing.T) {
	registry := commands.NewRegistry()
	builtin.RegisterBuiltinCommands(registry)

	cmd, ok := registry.Get("permissions")
	require.True(t, ok, "permissions command must be registered")
	assert.Equal(t, "permissions", cmd.Name())
}

func TestRegisterBuiltinCommands_IncludesPermsAlias(t *testing.T) {
	registry := commands.NewRegistry()
	builtin.RegisterBuiltinCommands(registry)

	cmd, ok := registry.Get("perms")
	require.True(t, ok, "perms alias must resolve to permissions command")
	assert.Equal(t, "permissions", cmd.Name())
}
