// Package commands — memory_command_test.go (P2-F24-T06).
//
// Tests use real tempdirs + real registry + real Loader. The editor seam
// is replaced with "true" (unix exit-0 binary) for /memory edit so CI
// doesn't try to spawn an interactive editor.
package commands

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/projectmemory"
)

func newTestRegistry(t *testing.T, projectFiles map[string]string) *projectmemory.MemoryRegistry {
	t.Helper()
	dir := t.TempDir()
	for name, content := range projectFiles {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	r := projectmemory.NewMemoryRegistry(projectmemory.NewMemoryLoader(zap.NewNop()), dir)
	_, err := r.Reload(context.Background())
	require.NoError(t, err)
	return r
}

func TestMemoryCommand_Name(t *testing.T) {
	require.Equal(t, "memory", NewMemoryCommand(nil).Name())
}

func TestMemoryCommand_Aliases_Empty(t *testing.T) {
	require.Empty(t, NewMemoryCommand(nil).Aliases())
}

func TestMemoryCommand_DescriptionAndUsage(t *testing.T) {
	cmd := NewMemoryCommand(nil)
	require.NotEmpty(t, cmd.Description())
	require.Contains(t, cmd.Usage(), "/memory")
}

func TestMemoryCommand_Status_NoMemory(t *testing.T) {
	r := newTestRegistry(t, nil)
	cmd := NewMemoryCommand(r)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "Project size:")
	require.Contains(t, res.Output, "0 bytes")
	require.Contains(t, res.Output, "(none)")
}

func TestMemoryCommand_Status_DefaultSubcommand(t *testing.T) {
	// /memory with no args == /memory status
	r := newTestRegistry(t, nil)
	cmd := NewMemoryCommand(r)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	require.Contains(t, res.Output, "Project memory status")
}

func TestMemoryCommand_Status_WithMemory(t *testing.T) {
	r := newTestRegistry(t, map[string]string{"helixcode.md": "STATUS_FIXTURE_24"})
	cmd := NewMemoryCommand(r)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "Project size:")
	// 17 = len("STATUS_FIXTURE_24")
	require.Contains(t, res.Output, "17 bytes")
	require.Contains(t, res.Output, "Project truncated:")
	require.Contains(t, res.Output, "false")
}

func TestMemoryCommand_Show_NoMemory(t *testing.T) {
	r := newTestRegistry(t, nil)
	cmd := NewMemoryCommand(r)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "no project memory loaded")
}

func TestMemoryCommand_Show_RendersContent(t *testing.T) {
	r := newTestRegistry(t, map[string]string{"helixcode.md": "SHOW_24_PROJECT"})
	cmd := NewMemoryCommand(r)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "SHOW_24_PROJECT")
}

func TestMemoryCommand_Edit_RunsEditor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P2-F24 unix-only 'true' binary used as editor seam")
	}
	r := newTestRegistry(t, map[string]string{"helixcode.md": "EDIT_24"})
	cmd := NewMemoryCommand(r)
	cmd.editor = func() string { return "true" } // POSIX: exits 0
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"edit"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	require.Contains(t, res.Output, "edited:")
}

func TestMemoryCommand_Edit_NoFile_OpensCwdHelixcodeMd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P2-F24 unix-only 'true' binary used as editor seam")
	}
	r := newTestRegistry(t, nil) // no project memory file
	cmd := NewMemoryCommand(r)
	cmd.editor = func() string { return "true" }
	wd := t.TempDir()
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"edit"}, WorkingDir: wd})
	require.NoError(t, err)
	require.Contains(t, res.Output, filepath.Join(wd, "helixcode.md"))
}

func TestMemoryCommand_Edit_EditorFails_PropagatesError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #P2-F24 unix-only 'false' binary used as failing editor seam")
	}
	r := newTestRegistry(t, map[string]string{"helixcode.md": "X"})
	cmd := NewMemoryCommand(r)
	cmd.editor = func() string { return "false" } // POSIX: exits 1
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"edit"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "/memory edit")
}

func TestMemoryCommand_Reload_RealTempdir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "helixcode.md")
	require.NoError(t, os.WriteFile(file, []byte("R1"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := projectmemory.NewMemoryRegistry(projectmemory.NewMemoryLoader(zap.NewNop()), dir)
	cmd := NewMemoryCommand(r)

	// First reload sees R1.
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"reload"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "project=2 bytes")
	require.Contains(t, r.Snapshot().Project, "R1")

	// Rewrite file; second reload sees R2.
	require.NoError(t, os.WriteFile(file, []byte("R2_LONGER_24"), 0644))
	res, err = cmd.Execute(context.Background(), &CommandContext{Args: []string{"reload"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "project=12 bytes")
	require.Contains(t, r.Snapshot().Project, "R2_LONGER_24")
}

func TestMemoryCommand_UnknownSubcommand_Err(t *testing.T) {
	r := newTestRegistry(t, nil)
	cmd := NewMemoryCommand(r)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"nope"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}

func TestDefaultEditor_FallsBackToVi(t *testing.T) {
	t.Setenv("EDITOR", "")
	require.Equal(t, "vi", defaultEditor())
}

func TestDefaultEditor_RespectsEditorEnv(t *testing.T) {
	t.Setenv("EDITOR", "emacs")
	require.Equal(t, "emacs", defaultEditor())
}

func TestDefaultEditor_TrimsWhitespace(t *testing.T) {
	t.Setenv("EDITOR", "   ")
	require.Equal(t, "vi", defaultEditor())
}

func TestMemoryCommand_Show_OnlyUserOverlay(t *testing.T) {
	// Only the user overlay is present — Render() should still produce output.
	xdg := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(xdg, "helixcode"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(xdg, "helixcode", "memory.md"), []byte("USER_ONLY_24"), 0644))
	t.Setenv("XDG_CONFIG_HOME", xdg)

	r := projectmemory.NewMemoryRegistry(projectmemory.NewMemoryLoader(zap.NewNop()), t.TempDir())
	_, err := r.Reload(context.Background())
	require.NoError(t, err)

	cmd := NewMemoryCommand(r)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "USER_ONLY_24")
}

// Sanity check: the status block formatting we depend on contains the
// substrings the test fixtures expect ("Project size:", "Loaded at:").
func TestMemoryCommand_Status_FormatStable(t *testing.T) {
	r := newTestRegistry(t, map[string]string{"helixcode.md": "S"})
	cmd := NewMemoryCommand(r)
	res, _ := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	for _, want := range []string{
		"Project memory status",
		"Project path:",
		"Project size:",
		"Project truncated:",
		"User path:",
		"User size:",
		"User truncated:",
		"Loaded at:",
	} {
		require.True(t, strings.Contains(res.Output, want), "status output missing %q\nfull:\n%s", want, res.Output)
	}
}
