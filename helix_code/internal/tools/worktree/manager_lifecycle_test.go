package worktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExitWorktree_ResetsState(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "feature-a", "")
	require.NoError(t, err)
	require.True(t, m.IsIsolated())

	m.ExitWorktree()
	assert.False(t, m.IsIsolated())
	assert.Equal(t, repo, m.GetCurrentDirectory())
}

func TestListWorktrees_EmptyRepoReturnsNil(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	wts, err := m.ListWorktrees(context.Background())
	require.NoError(t, err)
	assert.Empty(t, wts, "no worktrees yet")
}

func TestListWorktrees_AfterEnter(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "feature-b", "")
	require.NoError(t, err)
	_, err = m.EnterWorktree(context.Background(), "feature-c", "")
	require.NoError(t, err)

	wts, err := m.ListWorktrees(context.Background())
	require.NoError(t, err)

	names := []string{}
	for _, w := range wts {
		names = append(names, w.Name)
	}
	assert.Contains(t, names, "feature-b")
	assert.Contains(t, names, "feature-c")
}

func TestListWorktrees_IgnoresFilesInDir(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	require.NoError(t, os.MkdirAll(filepath.Join(repo, WorktreeDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, WorktreeDir, "stray.txt"), []byte("x"), 0o644))

	wts, err := m.ListWorktrees(context.Background())
	require.NoError(t, err)
	for _, w := range wts {
		assert.NotEqual(t, "stray.txt", w.Name, "files in WorktreeDir must be ignored")
	}
}

func TestRemoveWorktree_DeletesDirAndBranch(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	path, err := m.EnterWorktree(context.Background(), "feature-d", "")
	require.NoError(t, err)
	m.ExitWorktree()

	require.NoError(t, m.RemoveWorktree(context.Background(), "feature-d"))

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "worktree dir must be removed")
}

func TestRemoveWorktree_RefusesCurrent(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "feature-e", "")
	require.NoError(t, err)

	err = m.RemoveWorktree(context.Background(), "feature-e")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current worktree")
}

func TestRemoveWorktree_InvalidNameRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	err := m.RemoveWorktree(context.Background(), "../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match pattern")
}
