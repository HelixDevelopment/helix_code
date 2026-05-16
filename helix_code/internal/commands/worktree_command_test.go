package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/worktree"
)

func initEphemeralRepoForCommands(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		require.NoError(t, cmd.Run())
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@helixcode.dev")
	run("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "seed")
	return tmp
}

func TestWorktreeCommand_NameAliases(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	assert.Equal(t, "worktree", cmd.Name())
	assert.Contains(t, cmd.Aliases(), "wt")
}

func TestWorktreeCommand_ListSubaction_Empty(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{},
		RawInput: "/worktree",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, res.Output, "(no worktrees)",
		"empty list must explicitly say so")
}

func TestWorktreeCommand_EnterAndExit(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"enter", "feature-cmd"},
		RawInput: "/worktree enter feature-cmd",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "feature-cmd")
	assert.True(t, m.IsIsolated())

	res, err = cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"exit"},
		RawInput: "/worktree exit",
	})
	require.NoError(t, err)
	assert.False(t, m.IsIsolated())
}

func TestWorktreeCommand_RemoveSubaction(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	_, err := m.EnterWorktree(context.Background(), "feature-rm-cmd", "")
	require.NoError(t, err)
	m.ExitWorktree()

	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"remove", "feature-rm-cmd"},
		RawInput: "/worktree remove feature-rm-cmd",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "feature-rm-cmd")
}

func TestWorktreeCommand_RejectsUnknownSubaction(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	_, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"frobnicate"},
		RawInput: "/worktree frobnicate",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestWorktreeCommand_EnterRequiresName(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	_, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"enter"},
		RawInput: "/worktree enter",
	})
	require.Error(t, err)
}
