package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/hooks"
)

func writeYAML(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "hooks.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func writeShellScript(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "hook.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	return path
}

func TestRunHooksList_EmptyShowsHeader(t *testing.T) {
	tmp := t.TempDir()
	user := writeYAML(t, tmp, "apiVersion: helixcode.hooks/v1\nhooks: []\n")
	var buf bytes.Buffer
	require.NoError(t, runHooksList(&buf, user, filepath.Join(tmp, "missing.yaml")))
	out := buf.String()
	assert.Contains(t, out, "ID") // header
}

func TestRunHooksList_AfterLoad(t *testing.T) {
	tmp := t.TempDir()
	script := writeShellScript(t, tmp, "exit 0")
	user := writeYAML(t, tmp, `apiVersion: helixcode.hooks/v1
hooks:
  - id: audit
    event: before_tool_call
    script: `+script+`
    enabled: true
`)
	var buf bytes.Buffer
	require.NoError(t, runHooksList(&buf, user, filepath.Join(tmp, "missing.yaml")))
	out := buf.String()
	assert.Contains(t, out, "audit")
	assert.Contains(t, out, "before_tool_call")
}

func TestRunHooksValidate_GoodYAML(t *testing.T) {
	tmp := t.TempDir()
	script := writeShellScript(t, tmp, "exit 0")
	user := writeYAML(t, tmp, `apiVersion: helixcode.hooks/v1
hooks:
  - id: x
    event: on_error
    script: `+script+`
`)
	var buf bytes.Buffer
	require.NoError(t, runHooksValidate(&buf, user, filepath.Join(tmp, "missing.yaml")))
	assert.Contains(t, buf.String(), "OK")
}

func TestRunHooksValidate_BadYAML(t *testing.T) {
	tmp := t.TempDir()
	user := writeYAML(t, tmp, "not: valid: yaml: [")
	var buf bytes.Buffer
	err := runHooksValidate(&buf, user, filepath.Join(tmp, "missing.yaml"))
	assert.Error(t, err)
}

func TestRunHooksTest_FiresHandlersForEvent(t *testing.T) {
	tmp := t.TempDir()
	script := writeShellScript(t, tmp, "echo 'hello'; exit 0")
	user := writeYAML(t, tmp, `apiVersion: helixcode.hooks/v1
hooks:
  - id: t
    event: before_tool_call
    script: `+script+`
`)
	var buf bytes.Buffer
	require.NoError(t, runHooksTest(&buf, user, filepath.Join(tmp, "missing.yaml"), "before_tool_call"))
	out := buf.String()
	assert.Contains(t, out, "t")
}

func TestRunHooksEnable_FlipsEnabled(t *testing.T) {
	tmp := t.TempDir()
	script := writeShellScript(t, tmp, "exit 0")
	user := writeYAML(t, tmp, `apiVersion: helixcode.hooks/v1
hooks:
  - id: x
    event: on_error
    script: `+script+`
    enabled: false
`)
	require.NoError(t, runHooksEnable(user, "x"))

	loader := &hooks.FileLoader{UserPath: user, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	hs, _, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, hs, 1, "after enable, hook should be loadable")
	assert.Equal(t, "x", hs[0].ID)
}

func TestRunHooksDisable_FlipsEnabled(t *testing.T) {
	tmp := t.TempDir()
	script := writeShellScript(t, tmp, "exit 0")
	user := writeYAML(t, tmp, `apiVersion: helixcode.hooks/v1
hooks:
  - id: x
    event: on_error
    script: `+script+`
    enabled: true
`)
	require.NoError(t, runHooksDisable(user, "x"))

	loader := &hooks.FileLoader{UserPath: user, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	hs, _, err := loader.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, hs, "after disable, hook should be filtered out by Load")
}
