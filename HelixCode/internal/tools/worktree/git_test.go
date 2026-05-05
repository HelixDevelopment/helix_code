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

// initEphemeralRepo creates a real temporary git repo with one seed commit
// on `main`. Returns the absolute path. Test fails if `git` is not on PATH
// or any setup step fails.
func initEphemeralRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %s: %s", strings.Join(args, " "), string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@helixcode.dev")
	run("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "seed")
	return tmp
}

func TestGitRevParseToplevel_Resolves(t *testing.T) {
	repo := initEphemeralRepo(t)
	got, err := gitRevParseToplevel(context.Background(), repo)
	require.NoError(t, err)
	// On macOS the temp path may resolve through /private/var; tolerate that.
	assert.True(t, strings.HasSuffix(got, filepath.Base(repo)),
		"expected toplevel ending with %q, got %q", filepath.Base(repo), got)
}

func TestGitRevParseToplevel_NotARepo(t *testing.T) {
	tmp := t.TempDir()
	_, err := gitRevParseToplevel(context.Background(), tmp)
	assert.Error(t, err, "non-repo dir must error")
}

func TestGitWorktreeAdd_NewBranch(t *testing.T) {
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "feature-x")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))

	out, err := gitWorktreeAddNewBranch(context.Background(), repo, "feature-x", wtPath)
	require.NoError(t, err, "output: %s", out)

	// Worktree is on disk
	info, err := os.Stat(wtPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Worktree's seed file is the same as main's
	body, err := os.ReadFile(filepath.Join(wtPath, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "seed\n", string(body))
}

func TestGitWorktreeAdd_ExistingBranchFails(t *testing.T) {
	// gitWorktreeAdd attaches an existing branch; if the branch doesn't exist,
	// git refuses with a clear error.
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "non-existent")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))

	out, err := gitWorktreeAdd(context.Background(), repo, "non-existent-branch", wtPath)
	assert.Error(t, err)
	assert.Contains(t, string(out), "invalid reference",
		"git's error must mention the missing branch; output: %s", string(out))
}

func TestGitWorktreeList_AfterAdd(t *testing.T) {
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "feature-y")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))
	_, err := gitWorktreeAddNewBranch(context.Background(), repo, "feature-y", wtPath)
	require.NoError(t, err)

	out, err := gitWorktreeList(context.Background(), repo)
	require.NoError(t, err)
	assert.Contains(t, string(out), wtPath)
	assert.Contains(t, string(out), "feature-y")
}

func TestGitStatusPorcelain_CleanThenDirty(t *testing.T) {
	repo := initEphemeralRepo(t)

	// Clean repo: empty output
	out, err := gitStatusPorcelain(context.Background(), repo)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(out)))

	// Dirty repo: untracked file shows up
	require.NoError(t, os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x"), 0o644))
	out, err = gitStatusPorcelain(context.Background(), repo)
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(string(out)))
}

func TestGitWorktreeRemove_Roundtrip(t *testing.T) {
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "feature-z")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))
	_, err := gitWorktreeAddNewBranch(context.Background(), repo, "feature-z", wtPath)
	require.NoError(t, err)

	out, err := gitWorktreeRemove(context.Background(), repo, wtPath, false)
	require.NoError(t, err, "output: %s", out)

	_, statErr := os.Stat(wtPath)
	assert.True(t, os.IsNotExist(statErr), "worktree dir must be removed")
}
