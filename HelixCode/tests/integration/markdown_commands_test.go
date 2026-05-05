//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
)

// TestMarkdownCommands_LoadAndExecute exercises the full path: real .md file
// in a real tempdir, loaded via MarkdownLoader, executed via Registry.Get and
// Command.Execute. Asserts on real rendered output.
func TestMarkdownCommands_LoadAndExecute(t *testing.T) {
	dir := t.TempDir()
	cmds := filepath.Join(dir, ".helix", "commands")
	require.NoError(t, os.MkdirAll(cmds, 0755))
	body := "---\ndescription: echo arg\n---\n\nGot: {{ARG1}}\n"
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "echo.md"), []byte(body), 0644))

	reg := commands.NewRegistry()
	loader := commands.NewMarkdownLoader(reg, cmds, "")
	require.NoError(t, loader.Load())

	cmd, ok := reg.Get("echo")
	require.True(t, ok, "command 'echo' should be registered after Load")
	res, err := cmd.Execute(context.Background(), &commands.CommandContext{Args: []string{"hello-world"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "Got: hello-world")
}

// TestMarkdownCommands_ProjectOverridesUser confirms project files win when both
// dirs contain a file with the same stem name.
func TestMarkdownCommands_ProjectOverridesUser(t *testing.T) {
	projDir := t.TempDir()
	userDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "shared.md"),
		[]byte("---\ndescription: from user\n---\n\nuser body"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "shared.md"),
		[]byte("---\ndescription: from project\n---\n\nproject body"), 0644))

	reg := commands.NewRegistry()
	loader := commands.NewMarkdownLoader(reg, projDir, userDir)
	require.NoError(t, loader.Load())

	cmd, ok := reg.Get("shared")
	require.True(t, ok, "command 'shared' should be registered after Load")
	mc, ok := cmd.(*commands.MarkdownCommand)
	require.True(t, ok, "registered command must be a *MarkdownCommand")
	assert.Equal(t, "from project", mc.Description())
}

// TestMarkdownCommands_WatcherReloadsOnFileWrite uses the real fsnotify
// watcher against a real tempdir. Writes a file after the watcher starts
// and asserts the registry picks it up within 3 seconds.
func TestMarkdownCommands_WatcherReloadsOnFileWrite(t *testing.T) {
	dir := t.TempDir()
	cmds := filepath.Join(dir, ".helix", "commands")
	require.NoError(t, os.MkdirAll(cmds, 0755))

	reg := commands.NewRegistry()
	loader := commands.NewMarkdownLoader(reg, cmds, "")
	require.NoError(t, loader.Load())

	w, err := commands.NewMarkdownWatcher(loader, []string{cmds})
	require.NoError(t, err)
	w.SetDebounce(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	// Give the watcher time to initialise and start watching.
	time.Sleep(100 * time.Millisecond)

	// Write a new markdown command file into the watched directory.
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "added.md"),
		[]byte("---\ndescription: newly added\n---\n\nwatcher works"), 0644))

	// The watcher should debounce and reload within 3 seconds.
	require.Eventually(t, func() bool {
		_, ok := reg.Get("added")
		return ok
	}, 3*time.Second, 25*time.Millisecond, "registry should contain 'added' after file write")
}
