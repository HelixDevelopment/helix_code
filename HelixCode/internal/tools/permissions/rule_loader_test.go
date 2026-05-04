package permissions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/confirmation"
)

func TestLoad_BothFilesMissingUsesPresetOnly(t *testing.T) {
	tmp := t.TempDir()
	loader := &FileLoader{
		UserPath:    filepath.Join(tmp, "user.yaml"),
		ProjectPath: filepath.Join(tmp, "project.yaml"),
		Mode:        "default",
	}
	rs, err := loader.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "default", rs.Mode)
	assert.Empty(t, rs.Rules, "default preset has no rules")
	assert.Empty(t, rs.Sources)
}

func TestLoad_UserFileOnly(t *testing.T) {
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
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing"), Mode: ""}
	rs, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, rs.Rules, 1)
	assert.Equal(t, "Bash(git status*)", rs.Rules[0].Pattern)
	assert.Equal(t, confirmation.ActionAllow, rs.Rules[0].Action)
	assert.Equal(t, ScopeUser, rs.Rules[0].Source)
}

func TestLoad_ProjectOverridesUserSamePattern(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	projPath := filepath.Join(tmp, "project.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v1
rules:
  - pattern: "Bash(rm*)"
    action: allow
`), 0o600))
	require.NoError(t, os.WriteFile(projPath, []byte(`apiVersion: helixcode.permissions/v1
rules:
  - pattern: "Bash(rm*)"
    action: deny
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: projPath}
	rs, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, rs.Rules, 1, "project replaces user for identical pattern")
	assert.Equal(t, confirmation.ActionDeny, rs.Rules[0].Action)
	assert.Equal(t, ScopeProject, rs.Rules[0].Source)
}

func TestLoad_MalformedYAMLIsError(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte("not: valid: yaml: ["), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing")}
	_, err := loader.Load(context.Background())
	assert.Error(t, err)
}

func TestLoad_UnknownPresetIsError(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v1
mode: nonsense
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing")}
	_, err := loader.Load(context.Background())
	assert.Error(t, err)
}

func TestLoad_UnknownAPIVersionIsError(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte(`apiVersion: helixcode.permissions/v999
mode: default
`), 0o600))
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing")}
	_, err := loader.Load(context.Background())
	assert.Error(t, err)
}

func TestLoad_PresetRulesIncluded(t *testing.T) {
	tmp := t.TempDir()
	loader := &FileLoader{
		UserPath:    filepath.Join(tmp, "user.yaml"),
		ProjectPath: filepath.Join(tmp, "project.yaml"),
		Mode:        "dontAsk",
	}
	rs, err := loader.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "dontAsk", rs.Mode)
	patterns := patternsOf(rs.Rules)
	assert.Contains(t, patterns, "Read(*)")
	assert.Contains(t, patterns, "Glob(*)")
}

func TestSave_WritesFileWith0600Mode(t *testing.T) {
	tmp := t.TempDir()
	userPath := filepath.Join(tmp, "perms", "user.yaml")
	loader := &FileLoader{UserPath: userPath, ProjectPath: filepath.Join(tmp, "missing")}
	rule := Rule{
		Pattern:     "Bash(git status*)",
		Action:      confirmation.ActionAllow,
		Priority:    100,
		Description: "saved by test",
	}
	require.NoError(t, loader.Save(context.Background(), ScopeUser, rule))
	info, err := os.Stat(userPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	dirInfo, err := os.Stat(filepath.Dir(userPath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}
