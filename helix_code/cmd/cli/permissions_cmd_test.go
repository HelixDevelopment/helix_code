package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionsList_ShowsLoadedRules(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v1
mode: default
rules:
  - pattern: "Bash(git status*)"
    action: allow
    priority: 100
    description: "user-level git status"
`), 0o600))

	var buf bytes.Buffer
	err := runPermissionsList(&buf, userPath, filepath.Join(tmp, "missing"), "")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Bash(git status*)")
	assert.Contains(t, out, "allow")
	assert.Contains(t, out, "user")
}

func TestPermissionsAdd_WritesToUserFile(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	err := runPermissionsAdd(userPath, "Bash(git status*)", "allow", 100, "added by test")
	require.NoError(t, err)
	body, err := os.ReadFile(userPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Bash(git status*)")
	assert.Contains(t, string(body), "allow")
}

func TestPermissionsRemove_DropsRule(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, runPermissionsAdd(userPath, "Bash(rm*)", "deny", 1000, ""))
	require.NoError(t, runPermissionsRemove(userPath, "Bash(rm*)"))
	body, err := os.ReadFile(userPath)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "Bash(rm*)")
}

func TestPermissionsCheck_DryRun(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, runPermissionsAdd(userPath, "Bash(git status*)", "allow", 100, ""))

	var buf bytes.Buffer
	err := runPermissionsCheck(&buf, userPath, filepath.Join(tmp, "missing"), "", "Bash", "git status -sb")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "allow")
	assert.Contains(t, buf.String(), "Bash(git status*)")
}
