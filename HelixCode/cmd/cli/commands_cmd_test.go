package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
)

func setupTempCommands(t *testing.T) (string, *commands.MarkdownLoader, *commands.Registry) {
	t.Helper()
	dir := t.TempDir()
	cmds := filepath.Join(dir, ".helix", "commands")
	require.NoError(t, os.MkdirAll(cmds, 0755))
	reg := commands.NewRegistry()
	loader := commands.NewMarkdownLoader(reg, cmds, "")
	return cmds, loader, reg
}

func TestCommandsCmd_List(t *testing.T) {
	cmds, loader, reg := setupTempCommands(t)
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "hello.md"),
		[]byte("---\ndescription: Greet\n---\n\nHello {{ARG1}}"), 0644))
	require.NoError(t, loader.Load())

	cmd := newCommandsCmd(commandsCmdDeps{Loader: loader, Registry: reg})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "hello")
	assert.Contains(t, buf.String(), "Greet")
}

func TestCommandsCmd_ShowRendersBody(t *testing.T) {
	cmds, loader, reg := setupTempCommands(t)
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "hello.md"),
		[]byte("Hello {{ARG1}}"), 0644))
	require.NoError(t, loader.Load())

	cmd := newCommandsCmd(commandsCmdDeps{Loader: loader, Registry: reg})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"show", "hello"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Hello {{ARG1}}")
}

func TestCommandsCmd_RunRendersOutput(t *testing.T) {
	cmds, loader, reg := setupTempCommands(t)
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "echo.md"),
		[]byte("Got: {{ARG1}}"), 0644))
	require.NoError(t, loader.Load())

	cmd := newCommandsCmd(commandsCmdDeps{Loader: loader, Registry: reg})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"run", "echo", "world"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Got: world")
}
