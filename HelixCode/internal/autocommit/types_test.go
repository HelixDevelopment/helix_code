// Package autocommit — types_test.go (P2-F22-T02).
//
// Pins constants and sentinel error identity byte-for-byte. The values
// MUST NOT drift across releases — env var name and co-author trailer in
// particular are part of the user-facing surface and the commit history.
package autocommit

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvVarName_Pin(t *testing.T) {
	require.Equal(t, "HELIXCODE_GIT_AUTO_COMMIT", EnvVarName)
}

func TestCoAuthorTrailer_Pin(t *testing.T) {
	require.Equal(t, "Co-Authored-By: HelixCode <noreply@helixcode.dev>", CoAuthorTrailer)
}

func TestSkipParamKey_Pin(t *testing.T) {
	require.Equal(t, "_helix_skip_git_commit", SkipParamKey)
}

func TestErrNotGitRepo_Wrapping(t *testing.T) {
	wrapped := fmt.Errorf("at /tmp/foo: %w", ErrNotGitRepo)
	require.ErrorIs(t, wrapped, ErrNotGitRepo)
	require.NotErrorIs(t, wrapped, ErrCommitFailed)
	require.NotErrorIs(t, wrapped, ErrLLMUnavailable)
}

func TestErrCommitFailed_Wrapping(t *testing.T) {
	wrapped := fmt.Errorf("git commit: %w", ErrCommitFailed)
	require.ErrorIs(t, wrapped, ErrCommitFailed)
	require.NotErrorIs(t, wrapped, ErrNotGitRepo)
}

func TestErrLLMUnavailable_Wrapping(t *testing.T) {
	wrapped := fmt.Errorf("provider down: %w", ErrLLMUnavailable)
	require.ErrorIs(t, wrapped, ErrLLMUnavailable)
	require.NotErrorIs(t, wrapped, ErrCommitFailed)
}

func TestSentinelErrors_DistinctFromEachOther(t *testing.T) {
	require.False(t, errors.Is(ErrNotGitRepo, ErrCommitFailed))
	require.False(t, errors.Is(ErrCommitFailed, ErrLLMUnavailable))
	require.False(t, errors.Is(ErrLLMUnavailable, ErrNotGitRepo))
}

func TestCommitContext_ZeroValueSafe(t *testing.T) {
	var c CommitContext
	require.Equal(t, "", c.ToolName)
	require.Nil(t, c.Args)
	require.Nil(t, c.MutatedPaths)
	require.False(t, c.SkipRequested)
}

func TestCommitContext_FieldsSet(t *testing.T) {
	c := CommitContext{
		ToolName:      "fs_write",
		Args:          map[string]interface{}{"path": "x.txt"},
		MutatedPaths:  []string{"x.txt"},
		SkipRequested: true,
	}
	require.Equal(t, "fs_write", c.ToolName)
	require.Equal(t, "x.txt", c.Args["path"])
	require.Equal(t, []string{"x.txt"}, c.MutatedPaths)
	require.True(t, c.SkipRequested)
}

func TestCommitResult_ZeroValueSafe(t *testing.T) {
	var r CommitResult
	require.Equal(t, "", r.SHA)
	require.Equal(t, "", r.Subject)
	require.Nil(t, r.Files)
	require.False(t, r.Skipped)
	require.Equal(t, "", r.Reason)
}

func TestCommitResult_FieldsSet(t *testing.T) {
	r := CommitResult{
		SHA:     "abc123",
		Subject: "Auto-edit: fs_write on x.txt",
		Files:   []string{"x.txt"},
		Skipped: false,
		Reason:  "",
	}
	require.Equal(t, "abc123", r.SHA)
	require.Equal(t, "Auto-edit: fs_write on x.txt", r.Subject)
	require.Equal(t, []string{"x.txt"}, r.Files)
}

func TestOptions_ZeroValueSafe(t *testing.T) {
	var o Options
	require.False(t, o.Enabled)
	require.Nil(t, o.Provider)
	require.Equal(t, "", o.WorkingDir)
	require.Nil(t, o.Logger)
	require.Nil(t, o.NowFunc)
}
