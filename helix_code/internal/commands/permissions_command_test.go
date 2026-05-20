package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionsCommand_Name(t *testing.T) {
	cmd := NewPermissionsCommand()
	assert.Equal(t, "permissions", cmd.Name())
}

func TestPermissionsCommand_ListSubaction(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	userDir := filepath.Join(tmp, ".helixcode")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "permissions.yaml"), []byte(`apiVersion: helixcode.permissions/v1
mode: default
rules:
  - pattern: "Bash(git status*)"
    action: allow
`), 0o600))

	cmd := NewPermissionsCommand()
	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{},
		RawInput: "/permissions",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, res.Output, "Bash(git status*)")
}

func TestPermissionsCommand_ModeSubaction(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// round-432: /permissions mode confirmation is CONST-046-migrated;
	// wire the interpolatingTranslator so the rendered output carries
	// the real mode value for the assertion below.
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	cmd := NewPermissionsCommand()
	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"mode", "dontAsk"},
		RawInput: "/permissions mode dontAsk",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "dontAsk")
}

func TestPermissionsCommand_RejectsUnknownMode(t *testing.T) {
	cmd := NewPermissionsCommand()
	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"mode", "nonsense"},
		RawInput: "/permissions mode nonsense",
	})
	assert.Error(t, err)
	if res != nil {
		assert.NotEmpty(t, res.Output)
	}
}
