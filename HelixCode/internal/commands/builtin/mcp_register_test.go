package builtin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/mcp"
)

func TestRegisterBuiltinCommandsWithMCP(t *testing.T) {
	registry := commands.NewRegistry()
	mgr := mcp.NewManager()
	require.NoError(t, builtin.RegisterBuiltinCommandsWithMCP(registry, mgr))

	cmd, ok := registry.Get("mcp")
	require.True(t, ok, "/mcp command must be registered")
	assert.Equal(t, "mcp", cmd.Name())
}
