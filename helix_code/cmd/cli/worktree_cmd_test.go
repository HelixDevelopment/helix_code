package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/worktree"
)

func initEphemeralRepoForCLI(t *testing.T) string {
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

func TestRunWorktreeList_EmptyShowsHeader(t *testing.T) {
	repo := initEphemeralRepoForCLI(t)
	m := worktree.NewManager(repo)
	var buf bytes.Buffer
	require.NoError(t, runWorktreeList(&buf, m))
	out := buf.String()
	assert.Contains(t, out, "NAME") // header row
}

func TestRunWorktreeList_AfterEnter(t *testing.T) {
	repo := initEphemeralRepoForCLI(t)
	m := worktree.NewManager(repo)

	// Use Manager directly to set up state
	_, err := m.EnterWorktree(context.Background(), "feature-cli", "")
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, runWorktreeList(&buf, m))
	assert.Contains(t, buf.String(), "feature-cli")
}

func TestRunWorktreeRemove_Works(t *testing.T) {
	repo := initEphemeralRepoForCLI(t)
	m := worktree.NewManager(repo)
	_, err := m.EnterWorktree(context.Background(), "feature-rm", "")
	require.NoError(t, err)
	m.ExitWorktree()

	require.NoError(t, runWorktreeRemove(m, "feature-rm"))
	_, statErr := os.Stat(filepath.Join(repo, worktree.WorktreeDir, "feature-rm"))
	assert.True(t, os.IsNotExist(statErr))
}

// TestRunWorktreeEnter_PrintsHelpAndErrors / ...Exit... — round-311: the
// stateful-subcommand help lines route through the i18n seam. The shared
// round311TestTranslator (i18n_test_translator_test.go) resolves the
// round-311 worktree IDs to their bundle text so these assertions check
// the real user-facing content rather than raw message IDs.
func TestRunWorktreeEnter_PrintsHelpAndErrors(t *testing.T) {
	prev := translator
	SetTranslator(round311TestTranslator{})
	defer func() { translator = prev }()

	var buf bytes.Buffer
	err := runWorktreeEnter(&buf, "feature-x", "")
	assert.Error(t, err, "stateful subcommand must error from CLI")
	assert.Contains(t, buf.String(), "helixcode chat",
		"output must direct user to interactive session")
}

func TestRunWorktreeExit_PrintsHelpAndErrors(t *testing.T) {
	prev := translator
	SetTranslator(round311TestTranslator{})
	defer func() { translator = prev }()

	var buf bytes.Buffer
	err := runWorktreeExit(&buf)
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "helixcode chat")
}
