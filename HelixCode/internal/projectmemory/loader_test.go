// Package projectmemory — loader_test.go (P2-F24-T03).
//
// All tests use real t.TempDir() filesystems and real os.WriteFile/ReadFile.
// No mock filesystems — real OS semantics (case-insensitive APFS, symlinks,
// permissions) are part of the contract.
package projectmemory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoader_Discover_FindsHelixcodeMd_AtCwd(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("FIXTURE_24"), 0644))
	// Pin XDG to a tempdir to keep tests isolated from the real user overlay.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	l := NewMemoryLoader(zap.NewNop())
	m, err := l.Discover(dir)
	require.NoError(t, err)
	require.Contains(t, m.Project, "FIXTURE_24")
	require.NotEmpty(t, m.ProjectPath)
	require.True(t, filepath.IsAbs(m.ProjectPath))
	require.Equal(t, "helixcode.md", filepath.Base(m.ProjectPath))
}

func TestLoader_Discover_FindsAtParent_ParentWalk(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("PARENT_24"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(sub)
	require.NoError(t, err)
	require.Contains(t, m.Project, "PARENT_24")
}

func TestLoader_Discover_OrderHelixcodeBeforeCodex(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("HELIX_WIN"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "codex.md"), []byte("CODEX_LOSE"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(dir)
	require.NoError(t, err)
	require.Contains(t, m.Project, "HELIX_WIN")
	require.NotContains(t, m.Project, "CODEX_LOSE")
}

func TestLoader_Discover_OrderCodexBeforeAgents(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "codex.md"), []byte("CODEX_WIN"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("AGENTS_LOSE"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(dir)
	require.NoError(t, err)
	require.Contains(t, m.Project, "CODEX_WIN")
	require.NotContains(t, m.Project, "AGENTS_LOSE")
}

func TestLoader_Discover_StopsAtGitRoot(t *testing.T) {
	// Layout: outerdir/projectroot/sub
	// projectroot has .git → walk should stop at projectroot, NOT visit outerdir.
	outer := t.TempDir()
	root := filepath.Join(outer, "projectroot")
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0755))
	// Memory file is OUTSIDE the git root — must NOT be loaded.
	require.NoError(t, os.WriteFile(filepath.Join(outer, "AGENTS.md"), []byte("OUTSIDE_LOSE"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(sub)
	require.NoError(t, err)
	require.NotContains(t, m.Project, "OUTSIDE_LOSE")
	require.Empty(t, m.ProjectPath)
}

func TestLoader_Discover_MissingFile_NoError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(dir)
	require.NoError(t, err)
	require.Empty(t, m.ProjectPath)
	require.Empty(t, m.Project)
	require.Empty(t, m.UserPath)
	require.Empty(t, m.User)
	require.False(t, m.TruncatedProject)
	require.False(t, m.TruncatedUser)
}

func TestLoader_Discover_UserOverlay_LoadedAfterProject(t *testing.T) {
	proj := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(proj, "helixcode.md"), []byte("PROJECT_BODY_24"), 0644))
	xdg := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(xdg, "helixcode"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(xdg, "helixcode", "memory.md"), []byte("USER_BODY_24"), 0644))
	t.Setenv("XDG_CONFIG_HOME", xdg)

	m, err := NewMemoryLoader(zap.NewNop()).Discover(proj)
	require.NoError(t, err)
	require.Equal(t, "PROJECT_BODY_24", m.Project)
	require.Equal(t, "USER_BODY_24", m.User)
	require.NotEmpty(t, m.ProjectPath)
	require.NotEmpty(t, m.UserPath)
	require.Equal(t, filepath.Join(xdg, "helixcode", "memory.md"), m.UserPath)

	rendered := m.Render()
	require.Less(t, strings.Index(rendered, "PROJECT_BODY_24"), strings.Index(rendered, "USER_BODY_24"))
}

func TestLoader_Discover_UserOverlay_Missing_NoError(t *testing.T) {
	proj := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(proj, "helixcode.md"), []byte("P"), 0644))
	xdg := t.TempDir() // exists but no helixcode/memory.md
	t.Setenv("XDG_CONFIG_HOME", xdg)
	m, err := NewMemoryLoader(zap.NewNop()).Discover(proj)
	require.NoError(t, err)
	require.Equal(t, "P", m.Project)
	require.Empty(t, m.User)
	require.Empty(t, m.UserPath)
}

func TestLoader_Discover_TruncatesLargeProject_SetsFlag(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("X", MaxMemoryBytes+100)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte(big), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(dir)
	require.NoError(t, err)
	require.Equal(t, MaxMemoryBytes, len(m.Project))
	require.True(t, m.TruncatedProject)
	require.False(t, m.TruncatedUser)
}

func TestLoader_Discover_TruncatesLargeUser_SetsFlag(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("P"), 0644))
	xdg := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(xdg, "helixcode"), 0755))
	big := strings.Repeat("Y", MaxMemoryBytes+50)
	require.NoError(t, os.WriteFile(filepath.Join(xdg, "helixcode", "memory.md"), []byte(big), 0644))
	t.Setenv("XDG_CONFIG_HOME", xdg)
	m, err := NewMemoryLoader(zap.NewNop()).Discover(dir)
	require.NoError(t, err)
	require.Equal(t, MaxMemoryBytes, len(m.User))
	require.True(t, m.TruncatedUser)
	require.False(t, m.TruncatedProject)
}

func TestLoader_Discover_CaseInsensitiveAgents_Lower(t *testing.T) {
	// On case-sensitive filesystems (ext4, btrfs), 'agents.md' should still
	// match the 'AGENTS.md' canonical name via the phase-2 dir scan.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agents.md"), []byte("LOWER_24"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(dir)
	require.NoError(t, err)
	require.Contains(t, m.Project, "LOWER_24")
}

func TestLoader_Discover_LoadedAt_Set(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("X"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover(dir)
	require.NoError(t, err)
	require.False(t, m.LoadedAt.IsZero())
}

func TestLoader_Discover_EmptyCwd_NoError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := NewMemoryLoader(zap.NewNop()).Discover("")
	require.NoError(t, err)
	require.Empty(t, m.ProjectPath)
}

func TestNewMemoryLoader_NilLog_Safe(t *testing.T) {
	// Regression: passing nil log must not panic.
	l := NewMemoryLoader(nil)
	require.NotNil(t, l)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	_, err := l.Discover(dir)
	require.NoError(t, err)
}
