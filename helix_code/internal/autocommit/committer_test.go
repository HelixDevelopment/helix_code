// Package autocommit — committer_test.go (P2-F22-T05).
//
// Tests run the full MaybeCommit pipeline against real tempdir + real git
// + real subprocess commits. Positive evidence assertions (SHA equality,
// porcelain empty, co-author trailer present in `git log -1 --format=%B`)
// are MANDATORY per CONST-035.
package autocommit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/llm"
)

func newRealCommitter(t *testing.T, dir string, p llm.Provider, enabled bool) *AutoCommitter {
	t.Helper()
	return NewAutoCommitter(Options{
		Enabled:    enabled,
		Provider:   p,
		WorkingDir: dir,
		Logger:     zap.NewNop(),
	})
}

// initialCommit makes a baseline commit so HEAD exists and the working tree
// has a deterministic starting point.
func initialCommit(t *testing.T, dir string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644))
	for _, args := range [][]string{
		{"-C", dir, "add", ".gitkeep"},
		{"-C", dir, "commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func TestCommitter_DefaultOn_EditCommitsRealCommit(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{response: "SUMMARY"}, true)
	res, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.NoError(t, err)
	require.False(t, res.Skipped)
	require.NotEmpty(t, res.SHA)
	require.Equal(t, "SUMMARY", res.Subject)

	// Real evidence: git log shows the commit.
	out, _ := exec.Command("git", "-C", dir, "log", "-1", "--format=%H").Output()
	require.Equal(t, res.SHA, strings.TrimSpace(string(out)))
	out, _ = exec.Command("git", "-C", dir, "log", "-1", "--format=%B").Output()
	require.Contains(t, string(out), CoAuthorTrailer)
	out, _ = exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	require.Empty(t, strings.TrimSpace(string(out)))
}

func TestCommitter_Disabled_SkipsCommit(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{response: "S"}, false)
	res, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.NoError(t, err)
	require.True(t, res.Skipped)
	// CONST-046 round-229: Reason resolved via NoopTranslator → loud
	// message-ID echo. Substring "disabled" survives the migration.
	require.Contains(t, res.Reason, "disabled")
}

func TestCommitter_SkipRequested_Honoured(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{response: "S"}, true)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"}, SkipRequested: true,
	})
	require.True(t, res.Skipped)
	// CONST-046 round-229: Reason resolved via NoopTranslator → loud
	// echo of "internal_autocommit_skipped_per_edit_skip_requested".
	require.Contains(t, res.Reason, "per_edit_skip")
}

func TestCommitter_NotAGitRepo_Skips(t *testing.T) {
	dir := t.TempDir() // NOT a git repo
	c := newRealCommitter(t, dir, nil, true)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.True(t, res.Skipped)
	// CONST-046 round-229: Reason resolved via NoopTranslator → loud
	// echo of "internal_autocommit_skipped_not_a_git_repo".
	require.Contains(t, res.Reason, "not_a_git_repo")
}

func TestCommitter_CleanTree_NoChanges(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	c := newRealCommitter(t, dir, nil, true)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.True(t, res.Skipped)
	// CONST-046 round-229: Reason resolved via NoopTranslator → loud
	// echo of "internal_autocommit_skipped_no_changes_to_commit".
	require.Contains(t, res.Reason, "no_changes")
}

func TestCommitter_LLMUnavailable_FallsBack_StillCommits(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{err: errors.New("boom")}, true)
	res, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.NoError(t, err)
	require.False(t, res.Skipped)
	// CONST-046 round-229: deterministic-fallback subject resolved via
	// NoopTranslator → loud echo of
	// "internal_autocommit_subject_auto_edit_prefix".
	require.Contains(t, res.Subject, "auto_edit_prefix")
}

func TestCommitter_SetEnabled_AtomicSwap_NextCallSeesNewState(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	c := newRealCommitter(t, dir, &fakeProvider{response: "S"}, false)
	require.False(t, c.Enabled())
	c.SetEnabled(true)
	require.True(t, c.Enabled())

	require.NoError(t, os.WriteFile(filepath.Join(dir, "y.txt"), []byte("hi"), 0644))
	res, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"y.txt"},
	})
	require.NoError(t, err)
	require.False(t, res.Skipped)
}

func TestCommitter_CoAuthorTrailer_AppendedAlways(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{response: "subject"}, true)
	_, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.NoError(t, err)
	out, _ := exec.Command("git", "-C", dir, "log", "-1", "--format=%B").Output()
	require.Contains(t, string(out), "Co-Authored-By: HelixCode <noreply@helixcode.dev>")
}

func TestCommitter_CoAuthorTrailer_AppendedOnFallbackToo(t *testing.T) {
	// Even when the LLM fails and we use the deterministic fallback,
	// the co-author trailer MUST be present. Q3=A is unconditional.
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{err: errors.New("boom")}, true)
	_, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.NoError(t, err)
	out, _ := exec.Command("git", "-C", dir, "log", "-1", "--format=%B").Output()
	require.Contains(t, string(out), CoAuthorTrailer)
}

func TestCommitter_IsGitRepo_TrueInsideRepo(t *testing.T) {
	dir := setupRealGitRepo(t)
	c := newRealCommitter(t, dir, nil, true)
	require.True(t, c.IsGitRepo())
}

func TestCommitter_IsGitRepo_FalseOutsideRepo(t *testing.T) {
	c := newRealCommitter(t, t.TempDir(), nil, true)
	require.False(t, c.IsGitRepo())
}

func TestCommitter_NilOptions_SafeDefaults(t *testing.T) {
	// Constructing with zero-value Options shouldn't panic.
	dir := t.TempDir()
	c := NewAutoCommitter(Options{WorkingDir: dir})
	require.NotNil(t, c)
	require.False(t, c.Enabled())
}

func TestCommitter_StagesFromMutatedPaths_PorcelainEmptyAfter(t *testing.T) {
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{response: "two files"}, true)
	res, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "multiedit_commit", MutatedPaths: []string{"a.txt", "b.txt"},
	})
	require.NoError(t, err)
	require.False(t, res.Skipped)
	out, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	require.Empty(t, strings.TrimSpace(string(out)))
	// Both files in the new commit.
	statOut, _ := exec.Command("git", "-C", dir, "show", "--stat", "HEAD").Output()
	require.Contains(t, string(statOut), "a.txt")
	require.Contains(t, string(statOut), "b.txt")
}

func TestCommitter_SecretInSubject_Redacted(t *testing.T) {
	// LLM returns a string containing a fake AKIA — the subject as
	// committed must contain [REDACTED] and NOT the original key.
	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir,
		&fakeProvider{response: "leak AKIAABCDEFGHIJKLMNOP add"}, true)
	_, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.NoError(t, err)
	out, _ := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output()
	require.NotContains(t, string(out), "AKIAABCDEFGHIJKLMNOP")
	require.Contains(t, string(out), "[REDACTED]")
}
