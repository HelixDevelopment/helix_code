package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManager_CreateWorktreeForSubagent_DoesNotMutateState is the load-bearing
// anti-bluff anchor for P1-F15-T06: it proves CreateWorktreeForSubagent does
// NOT touch Manager.currentWorktree (unlike EnterWorktree, which does).
func TestManager_CreateWorktreeForSubagent_DoesNotMutateState(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	require.False(t, m.IsIsolated(), "fresh manager must not be isolated")
	require.Equal(t, repo, m.GetCurrentDirectory())

	path, cleanup, err := m.CreateWorktreeForSubagent(context.Background(), "subagent-x", "")
	require.NoError(t, err)
	require.NotEmpty(t, path)
	require.NotNil(t, cleanup)
	defer func() { _ = cleanup() }()

	// State must be unchanged.
	assert.False(t, m.IsIsolated(), "CreateWorktreeForSubagent must NOT set currentWorktree")
	assert.Equal(t, repo, m.GetCurrentDirectory(), "GetCurrentDirectory must still return repo root")
}

func TestManager_CreateWorktreeForSubagent_CreatesRealWorktree(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	path, cleanup, err := m.CreateWorktreeForSubagent(context.Background(), "subagent-real", "")
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	// Path must be absolute, under <repo>/.helix-worktrees/.
	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, filepath.Join(repo, WorktreeDir, "subagent-real"), path)

	// Worktree must exist on disk and contain the seed file.
	body, err := os.ReadFile(filepath.Join(path, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "seed\n", string(body))

	// `git worktree list` from the main repo must show it.
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), path)
}

func TestManager_CreateWorktreeForSubagent_CleanupRemovesWorktree(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	path, cleanup, err := m.CreateWorktreeForSubagent(context.Background(), "subagent-cleanup", "")
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	require.NoError(t, statErr, "worktree must exist before cleanup")

	require.NoError(t, cleanup())

	_, statErr = os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "cleanup must remove the worktree directory")
}

func TestManager_CreateWorktreeForSubagent_BaseBranchHonored(t *testing.T) {
	repo := initEphemeralRepo(t)

	// Create a real branch ahead of the call.
	cmd := exec.Command("git", "branch", "feature-base")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	m := NewManager(repo)
	path, cleanup, err := m.CreateWorktreeForSubagent(context.Background(), "uses-feature", "feature-base")
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	out, err := exec.Command("git", "-C", path, "branch", "--show-current").Output()
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(string(out)), "feature-base")
}

func TestManager_CreateWorktreeForSubagent_InvalidNameRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, _, err := m.CreateWorktreeForSubagent(context.Background(), "../etc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match pattern")
}
