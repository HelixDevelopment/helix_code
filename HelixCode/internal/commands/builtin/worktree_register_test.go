package builtin_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/tools/worktree"
)

func TestRegisterBuiltinCommands_IncludesWorktree(t *testing.T) {
	tmp := t.TempDir()
	exec.Command("git", "init", tmp).Run()
	require.NoError(t, exec.Command("git", "-C", tmp, "config", "user.email", "x@y").Run())
	require.NoError(t, exec.Command("git", "-C", tmp, "config", "user.name", "x").Run())
	require.NoError(t, exec.Command("touch", filepath.Join(tmp, "f")).Run())
	require.NoError(t, exec.Command("git", "-C", tmp, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", tmp, "commit", "-m", "x", "--allow-empty").Run())

	m := worktree.NewManager(tmp)
	registry := commands.NewRegistry()
	require.NoError(t, builtin.RegisterBuiltinCommandsWithWorktree(registry, m))

	cmd, ok := registry.Get("worktree")
	require.True(t, ok)
	assert.Equal(t, "worktree", cmd.Name())

	cmd2, ok := registry.Get("wt")
	require.True(t, ok)
	assert.Equal(t, "worktree", cmd2.Name(), "alias resolves to worktree")
}
