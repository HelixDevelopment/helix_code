package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/mcp"
)

func TestSlashMCP_ListEmpty(t *testing.T) {
	c := NewMCPCommand(mcp.NewManager())
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "NAME")
}

func TestSlashMCP_UnknownSubcommand(t *testing.T) {
	c := NewMCPCommand(mcp.NewManager())
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"nope"}})
	assert.Error(t, err)
}
