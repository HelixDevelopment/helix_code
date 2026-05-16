// Package autocommit — git_test.go (P2-F22-T03).
//
// Tests use real `git init` + real subprocess git operations. CONST-035
// demands real ops in integration-flavour tests; mocking the wrapper
// defeats the wrapper's purpose. Each test creates its own t.TempDir.
package autocommit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// setupRealGitRepo initialises a brand-new git repo in t.TempDir() with a
// committer identity configured. Returns the directory path.
func setupRealGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@helixcode.dev"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	return dir
}

func TestGit_IsRepo_True_InsideRepo(t *testing.T) {
	dir := setupRealGitRepo(t)
	g := NewGit(dir, zap.NewNop())
	ok, err := g.IsRepo(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
}

func TestGit_IsRepo_False_OutsideRepo(t *testing.T) {
	g := NewGit(t.TempDir(), zap.NewNop())
	ok, _ := g.IsRepo(context.Background())
	require.False(t, ok)
}

func TestGit_StatusPorcelain_CleanReturnsEmpty(t *testing.T) {
	dir := setupRealGitRepo(t)
	g := NewGit(dir, zap.NewNop())
	out, err := g.StatusPorcelain(context.Background())
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(out))
}

func TestGit_StatusPorcelain_DirtyAfterWrite(t *testing.T) {
	dir := setupRealGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	g := NewGit(dir, zap.NewNop())
	out, err := g.StatusPorcelain(context.Background())
	require.NoError(t, err)
	require.Contains(t, out, "x.txt")
}

func TestGit_AddCommitHeadSHA_RoundTrip(t *testing.T) {
	dir := setupRealGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	g := NewGit(dir, zap.NewNop())
	require.NoError(t, g.Add(context.Background(), "x.txt"))
	sha, err := g.Commit(context.Background(), "test commit\n\nbody")
	require.NoError(t, err)
	require.NotEmpty(t, sha)
	require.Len(t, sha, 40, "SHA must be 40 hex chars")

	head, err := g.HeadSHA(context.Background())
	require.NoError(t, err)
	require.Equal(t, sha, head)

	// Working tree is clean after commit.
	out, _ := g.StatusPorcelain(context.Background())
	require.Empty(t, strings.TrimSpace(out))
}

func TestGit_DiffStaged_NonEmptyAfterAdd(t *testing.T) {
	dir := setupRealGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello\n"), 0644))
	g := NewGit(dir, zap.NewNop())
	require.NoError(t, g.Add(context.Background(), "x.txt"))
	diff, err := g.DiffStaged(context.Background())
	require.NoError(t, err)
	require.Contains(t, diff, "+hello")
}

func TestGit_DiffStaged_EmptyWhenClean(t *testing.T) {
	dir := setupRealGitRepo(t)
	g := NewGit(dir, zap.NewNop())
	diff, err := g.DiffStaged(context.Background())
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(diff))
}

func TestGit_DiffUnstaged_NonEmptyAfterWrite(t *testing.T) {
	dir := setupRealGitRepo(t)
	// Make an initial commit so DiffUnstaged has a baseline.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("v1\n"), 0644))
	g := NewGit(dir, zap.NewNop())
	require.NoError(t, g.Add(context.Background(), "x.txt"))
	_, err := g.Commit(context.Background(), "init")
	require.NoError(t, err)

	// Modify file (unstaged).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("v2\n"), 0644))
	diff, err := g.DiffUnstaged(context.Background())
	require.NoError(t, err)
	require.Contains(t, diff, "+v2")
}

func TestGit_HeadSHA_ErrorWhenNoCommits(t *testing.T) {
	dir := setupRealGitRepo(t)
	g := NewGit(dir, zap.NewNop())
	_, err := g.HeadSHA(context.Background())
	// git rev-parse HEAD on empty repo errors out — that's fine; we just
	// require an error so callers handle the unborn-HEAD case.
	require.Error(t, err)
}

func TestGit_Add_MultiplePaths(t *testing.T) {
	dir := setupRealGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644))
	g := NewGit(dir, zap.NewNop())
	require.NoError(t, g.Add(context.Background(), "a.txt", "b.txt"))
	out, _ := g.StatusPorcelain(context.Background())
	require.Contains(t, out, "a.txt")
	require.Contains(t, out, "b.txt")
}

func TestGit_Commit_MessageRoundTrips(t *testing.T) {
	dir := setupRealGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	g := NewGit(dir, zap.NewNop())
	require.NoError(t, g.Add(context.Background(), "x.txt"))
	_, err := g.Commit(context.Background(), "subject line\n\nbody paragraph")
	require.NoError(t, err)

	cmd := exec.Command("git", "-C", dir, "log", "-1", "--format=%B")
	out, err := cmd.Output()
	require.NoError(t, err)
	require.Contains(t, string(out), "subject line")
	require.Contains(t, string(out), "body paragraph")
}
