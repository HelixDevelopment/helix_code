//go:build integration

package worktree_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/worktree"
)

func initEphemeralRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@helixcode.dev")
	run("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "seed")
	return tmp
}

// TestIntegration_EnterCommitDoesNotPolluteMain proves that a commit made
// in a worktree does NOT change main's HEAD. NO mocks.
func TestIntegration_EnterCommitDoesNotPolluteMain(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := worktree.NewManager(repo)

	mainHEADBefore, err := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
	require.NoError(t, err)

	wtPath, err := m.EnterWorktree(context.Background(), "feature-x", "")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "new.txt"), []byte("isolated"), 0o644))
	add := exec.Command("git", "-C", wtPath, "add", ".")
	require.NoError(t, add.Run())
	commit := exec.Command("git", "-C", wtPath, "commit", "-m", "isolated work")
	require.NoError(t, commit.Run())

	mainHEADAfter, err := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(mainHEADBefore)), strings.TrimSpace(string(mainHEADAfter)),
		"main's HEAD must not change after committing inside the worktree")

	_, statErr := os.Stat(filepath.Join(repo, "new.txt"))
	assert.True(t, os.IsNotExist(statErr), "new.txt must only exist in the worktree, not main")
}

// TestIntegration_RoundTripCreateRemove proves end-to-end worktree creation
// and removal against a real git repo, no mocks.
func TestIntegration_RoundTripCreateRemove(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := worktree.NewManager(repo)

	wtPath, err := m.EnterWorktree(context.Background(), "feature-y", "")
	require.NoError(t, err)
	assert.True(t, m.IsIsolated())

	info, err := os.Stat(wtPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	m.ExitWorktree()
	require.NoError(t, m.RemoveWorktree(context.Background(), "feature-y"))

	_, statErr := os.Stat(wtPath)
	assert.True(t, os.IsNotExist(statErr))

	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "feature-y")
}

// TestIntegration_PathTraversalRejected proves that ValidateName rejects
// names that would escape the persistence directory.
func TestIntegration_PathTraversalRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := worktree.NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "../etc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match pattern")
}
