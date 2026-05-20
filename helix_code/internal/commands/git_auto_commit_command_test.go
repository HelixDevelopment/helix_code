// Package commands — git_auto_commit_command_test.go (P2-F22-T07).
//
// Tests use an in-package fakeInspector to simulate a committer that
// keeps its own enabled state. Real AutoCommitter integration is
// covered by T08 main.go integration tests.
package commands

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/autocommit"
)

type fakeInspector struct {
	enabled atomic.Bool
	isRepo  bool
}

func (f *fakeInspector) Enabled() bool      { return f.enabled.Load() }
func (f *fakeInspector) SetEnabled(v bool)  { f.enabled.Store(v) }
func (f *fakeInspector) IsGitRepo() bool    { return f.isRepo }

func newTestInspector(enabled, isRepo bool) *fakeInspector {
	f := &fakeInspector{isRepo: isRepo}
	f.enabled.Store(enabled)
	return f
}

func TestGitAutoCommit_Name_IsGitAutoCommit(t *testing.T) {
	require.Equal(t, "git_auto_commit", NewGitAutoCommitCommand(nil).Name())
}

func TestGitAutoCommit_Aliases_None(t *testing.T) {
	require.Nil(t, NewGitAutoCommitCommand(nil).Aliases())
}

func TestGitAutoCommit_Description_NonEmpty(t *testing.T) {
	require.NotEmpty(t, NewGitAutoCommitCommand(nil).Description())
}

func TestGitAutoCommit_Usage_HasSubcommands(t *testing.T) {
	// Usage routes through the CONST-046 tr() seam; under the default
	// NoopTranslator it echoes the message ID (round-399). The
	// subcommand-substring contract is enforced by the bundle entry
	// internal_commands_git_auto_commit_usage in active.en.yaml.
	u := NewGitAutoCommitCommand(nil).Usage()
	require.Contains(t, u, "internal_commands_git_auto_commit_usage")
}

func TestGitAutoCommit_Status_Default_PrintsState(t *testing.T) {
	c := newTestInspector(true, true)
	cmd := NewGitAutoCommitCommand(c)
	res, err := cmd.Execute(context.Background(), &CommandContext{})
	require.NoError(t, err)
	require.Contains(t, res.Output, "git_auto_commit: on")
	require.Contains(t, res.Output, "git_repo: yes")
}

func TestGitAutoCommit_Status_Explicit_PrintsState(t *testing.T) {
	c := newTestInspector(false, false)
	cmd := NewGitAutoCommitCommand(c)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "git_auto_commit: off")
	require.Contains(t, res.Output, "git_repo: no")
	require.Contains(t, res.Output, autocommit.CoAuthorTrailer)
}

func TestGitAutoCommit_On_FlipsState(t *testing.T) {
	c := newTestInspector(false, true)
	cmd := NewGitAutoCommitCommand(c)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"on"}})
	require.NoError(t, err)
	require.True(t, c.Enabled())
	require.Contains(t, res.Output, "on")
}

func TestGitAutoCommit_Off_FlipsState(t *testing.T) {
	c := newTestInspector(true, true)
	cmd := NewGitAutoCommitCommand(c)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"off"}})
	require.NoError(t, err)
	require.False(t, c.Enabled())
	require.Contains(t, res.Output, "off")
}

func TestGitAutoCommit_Show_PrintsTrailer(t *testing.T) {
	c := newTestInspector(true, true)
	cmd := NewGitAutoCommitCommand(c)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "Co-Authored-By: HelixCode <noreply@helixcode.dev>")
	require.Contains(t, res.Output, autocommit.EnvVarName)
	require.Contains(t, res.Output, autocommit.SkipParamKey)
}

func TestGitAutoCommit_UnknownSubcommand_Err(t *testing.T) {
	c := newTestInspector(true, true)
	cmd := NewGitAutoCommitCommand(c)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"nope"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
}

func TestGitAutoCommit_NilCommitter_StatusGracefulOff(t *testing.T) {
	cmd := NewGitAutoCommitCommand(nil)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "git_auto_commit: off")
	require.Contains(t, res.Output, "git_repo: no")
}

func TestGitAutoCommit_NilCommitter_OnReturnsError(t *testing.T) {
	cmd := NewGitAutoCommitCommand(nil)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"on"}})
	require.Error(t, err)
}

func TestGitAutoCommit_NilCommitter_OffReturnsError(t *testing.T) {
	cmd := NewGitAutoCommitCommand(nil)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"off"}})
	require.Error(t, err)
}

func TestGitAutoCommit_Status_CaseInsensitive(t *testing.T) {
	c := newTestInspector(true, true)
	cmd := NewGitAutoCommitCommand(c)
	for _, sub := range []string{"STATUS", "Status", "status"} {
		res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{sub}})
		require.NoError(t, err, "subcommand %q", sub)
		require.True(t, strings.Contains(res.Output, "git_auto_commit"))
	}
}
