package hooks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileLoader_BothFilesMissing(t *testing.T) {
	tmp := t.TempDir()
	loader := &FileLoader{
		UserPath:    filepath.Join(tmp, "user.yaml"),
		ProjectPath: filepath.Join(tmp, "project.yaml"),
	}
	hooks, sources, err := loader.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, hooks)
	assert.Empty(t, sources)
}

func TestFileLoader_UserFileOnly(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	scriptPath := filepath.Join(tmp, "audit.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.hooks/v1
hooks:
  - id: audit
    event: before_tool_call
    script: `+scriptPath+`
    priority: 100
    enabled: true
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	hooks, sources, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	assert.Equal(t, "audit", hooks[0].ID)
	assert.Equal(t, HookTypeBeforeToolCall, hooks[0].Type)
	assert.Equal(t, HookPriority(100), hooks[0].Priority)
	assert.True(t, hooks[0].Enabled)
	assert.Equal(t, []string{userPath}, sources)
}

func TestFileLoader_ProjectOverridesUserSameID(t *testing.T) {
	tmp := t.TempDir()
	scriptA := filepath.Join(tmp, "a.sh")
	scriptB := filepath.Join(tmp, "b.sh")
	require.NoError(t, os.WriteFile(scriptA, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	require.NoError(t, os.WriteFile(scriptB, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.hooks/v1
hooks:
  - id: dup
    event: before_bash
    script: `+scriptA+`
    priority: 1
`), 0o600))

	projPath := filepath.Join(tmp, "project.yaml")
	require.NoError(t, os.WriteFile(projPath, []byte(`apiVersion: helixcode.hooks/v1
hooks:
  - id: dup
    event: before_bash
    script: `+scriptB+`
    priority: 999
`), 0o600))

	loader := &FileLoader{UserPath: userPath, ProjectPath: projPath}
	hooks, _, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, hooks, 1, "duplicate id collapses to one entry")
	assert.Equal(t, HookPriority(999), hooks[0].Priority, "project overrides user")
}

func TestFileLoader_DisabledHooksAreFiltered(t *testing.T) {
	tmp := t.TempDir()
	scriptPath := filepath.Join(tmp, "x.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.hooks/v1
hooks:
  - id: on
    event: on_error
    script: `+scriptPath+`
    enabled: true
  - id: off
    event: on_error
    script: `+scriptPath+`
    enabled: false
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	hooks, _, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, hooks, 1, "disabled hooks must not be returned")
	assert.Equal(t, "on", hooks[0].ID)
}

func TestFileLoader_MalformedYAMLIsError(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte("not: valid: yaml: ["), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	_, _, err := loader.Load(context.Background())
	assert.Error(t, err)
}

func TestFileLoader_MissingAPIVersionIsError(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`hooks:
  - id: x
    event: on_error
    script: /bin/true
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	_, _, err := loader.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiVersion")
}

func TestFileLoader_UnknownAPIVersionIsError(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.hooks/v999
hooks: []
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	_, _, err := loader.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported apiVersion")
}

func TestFileLoader_UnknownEventTypeRejectedAtLoad(t *testing.T) {
	tmp := t.TempDir()
	scriptPath := filepath.Join(tmp, "x.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.hooks/v1
hooks:
  - id: bad
    event: nonsense_event
    script: `+scriptPath+`
  - id: good
    event: before_tool_call
    script: `+scriptPath+`
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	hooks, _, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, hooks, 1, "unknown event types are skipped; valid hooks still load")
	assert.Equal(t, "good", hooks[0].ID)
}

func TestFileLoader_TimeoutParsesGoDuration(t *testing.T) {
	tmp := t.TempDir()
	scriptPath := filepath.Join(tmp, "x.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.hooks/v1
hooks:
  - id: slow
    event: before_tool_call
    script: `+scriptPath+`
    timeout: 5s
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	hooks, _, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	assert.Equal(t, 5*time.Second, hooks[0].Timeout)
}
