package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCommandsCommandWithLoader(t *testing.T) (*CommandsCommand, *Registry, *MarkdownLoader, string) {
	t.Helper()
	dir := t.TempDir()
	cmds := filepath.Join(dir, ".helix", "commands")
	require.NoError(t, os.MkdirAll(cmds, 0755))
	reg := NewRegistry()
	loader := NewMarkdownLoader(reg, cmds, "")
	require.NoError(t, loader.Load())
	return NewCommandsCommand(loader, reg), reg, loader, cmds
}

func TestSlashCommands_ListEmpty(t *testing.T) {
	c, _, _, _ := newCommandsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	// The table header routes through the CONST-046 tr() seam; the
	// default NoopTranslator echoes the message ID verbatim.
	assert.Contains(t, res.Output, "internal_commands_commands_table_header")
}

func TestSlashCommands_ListShowsLoadedCommands(t *testing.T) {
	c, _, loader, cmds := newCommandsCommandWithLoader(t)
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "echo.md"),
		[]byte("---\ndescription: Echo arg\n---\n\nGot: {{ARG1}}"), 0644))
	require.NoError(t, loader.Reload())

	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "echo")
	assert.Contains(t, res.Output, "Echo arg")
}

func TestSlashCommands_ShowReturnsBody(t *testing.T) {
	c, _, loader, cmds := newCommandsCommandWithLoader(t)
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "hello.md"),
		[]byte("Hello {{ARG1}}"), 0644))
	require.NoError(t, loader.Reload())

	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "hello"}})
	require.NoError(t, err)
	// The show-detail template routes through the CONST-046 tr() seam;
	// the default NoopTranslator echoes the message ID verbatim. Body
	// interpolation is covered by the i18n_wiring integration test.
	assert.Contains(t, res.Output, "internal_commands_commands_show_detail")
}

func TestSlashCommands_ShowUnknownErrors(t *testing.T) {
	c, _, _, _ := newCommandsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "nonexistent"}})
	require.Error(t, err)
}

func TestSlashCommands_ReloadRefreshesRegistry(t *testing.T) {
	c, reg, _, cmds := newCommandsCommandWithLoader(t)
	// Add a file BEFORE reload — it shouldn't be in registry yet.
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "fresh.md"), []byte("body"), 0644))
	_, ok := reg.Get("fresh")
	assert.False(t, ok)

	// /commands reload should pick it up.
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"reload"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "reload")

	_, ok = reg.Get("fresh")
	assert.True(t, ok)
}

func TestSlashCommands_RunRendersOutput(t *testing.T) {
	c, _, loader, cmds := newCommandsCommandWithLoader(t)
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "echo.md"),
		[]byte("Got: {{ARG1}}"), 0644))
	require.NoError(t, loader.Reload())

	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"run", "echo", "hello"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "Got: hello")
}

func TestSlashCommands_RunUnknownErrors(t *testing.T) {
	c, _, _, _ := newCommandsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"run", "nonexistent"}})
	require.Error(t, err)
}

func TestSlashCommands_DefaultIsList(t *testing.T) {
	c, _, _, _ := newCommandsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	// Default subcommand is list; header routes through the tr() seam.
	assert.Contains(t, res.Output, "internal_commands_commands_table_header")
}

func TestSlashCommands_UnknownSubcommandErrors(t *testing.T) {
	c, _, _, _ := newCommandsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}
