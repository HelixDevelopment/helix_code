package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterWorktree_NewBranchPath(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	path, err := m.EnterWorktree(context.Background(), "feature-x", "")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, filepath.Join(repo, WorktreeDir, "feature-x"), path)

	// Worktree dir exists and contains the seed file
	body, err := os.ReadFile(filepath.Join(path, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "seed\n", string(body))

	// Manager state updated
	assert.True(t, m.IsIsolated())
	assert.Equal(t, path, m.GetCurrentDirectory())
}

func TestEnterWorktree_ExistingBranchPath(t *testing.T) {
	repo := initEphemeralRepo(t)

	// Create a branch ahead of EnterWorktree
	cmd := exec.Command("git", "branch", "release-1.0")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	m := NewManager(repo)
	path, err := m.EnterWorktree(context.Background(), "release-1.0", "")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(repo, WorktreeDir, "release-1.0"), path)
	assert.True(t, m.IsIsolated())
}

func TestEnterWorktree_DirtyExistingDirRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	// First entry creates the worktree
	path, err := m.EnterWorktree(context.Background(), "feature-y", "")
	require.NoError(t, err)

	// Dirty the worktree
	require.NoError(t, os.WriteFile(filepath.Join(path, "uncommitted.txt"), []byte("dirty"), 0o644))

	// Second entry must reject
	_, err = m.EnterWorktree(context.Background(), "feature-y", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
}

func TestEnterWorktree_CleanExistingDirReuses(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	first, err := m.EnterWorktree(context.Background(), "feature-z", "")
	require.NoError(t, err)

	// Re-entry into the same worktree (still clean)
	second, err := m.EnterWorktree(context.Background(), "feature-z", "")
	require.NoError(t, err)
	assert.Equal(t, first, second, "re-entry returns same path")
}

func TestEnterWorktree_InvalidNameRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "../etc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match pattern")
}

func TestEnterWorktree_NotARepoFails(t *testing.T) {
	tmp := t.TempDir() // not a git repo
	m := NewManager(tmp)

	_, err := m.EnterWorktree(context.Background(), "feature-w", "")
	require.Error(t, err)
}

func TestEnterWorktree_BaseBranchOverridesName(t *testing.T) {
	repo := initEphemeralRepo(t)

	// Create a base branch with extra commits
	cmd := exec.Command("git", "branch", "stable")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	m := NewManager(repo)
	// name=feature-from-stable, baseBranch=stable → branch should be 'stable'
	path, err := m.EnterWorktree(context.Background(), "feature-from-stable", "stable")
	require.NoError(t, err)

	// Verify the worktree is on the 'stable' branch
	out, err := exec.Command("git", "-C", path, "branch", "--show-current").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "stable")
}
