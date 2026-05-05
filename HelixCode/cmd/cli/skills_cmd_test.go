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

func setupTempSkills(t *testing.T) (string, *commands.SkillLoader, *commands.SkillRegistry) {
	t.Helper()
	dir := t.TempDir()
	skills := filepath.Join(dir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))
	reg := commands.NewSkillRegistry()
	loader := commands.NewSkillLoader(reg, skills, "")
	return skills, loader, reg
}

func TestSkillsCmd_List(t *testing.T) {
	skills, loader, reg := setupTempSkills(t)
	require.NoError(t, os.WriteFile(filepath.Join(skills, "ref.md"),
		[]byte("---\ndescription: Refactor\ntriggers: [\"^refactor\"]\n---\nbody"), 0644))
	require.NoError(t, loader.Load())

	cmd := newSkillsCmd(skillsCmdDeps{Loader: loader, Registry: reg})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "ref")
	assert.Contains(t, buf.String(), "Refactor")
}

func TestSkillsCmd_ShowRendersMetaAndBody(t *testing.T) {
	skills, loader, reg := setupTempSkills(t)
	require.NoError(t, os.WriteFile(filepath.Join(skills, "x.md"),
		[]byte("---\ndescription: d\ntriggers: [\"^x\"]\n---\nbody-x"), 0644))
	require.NoError(t, loader.Load())

	cmd := newSkillsCmd(skillsCmdDeps{Loader: loader, Registry: reg})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"show", "x"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "body-x")
	assert.Contains(t, buf.String(), "^x")
}

func TestSkillsCmd_InvokeRenders(t *testing.T) {
	skills, loader, reg := setupTempSkills(t)
	require.NoError(t, os.WriteFile(filepath.Join(skills, "echo.md"),
		[]byte("---\ndescription: e\ntriggers: [\"^x\"]\n---\nGot: {{ARG1}}"), 0644))
	require.NoError(t, loader.Load())

	cmd := newSkillsCmd(skillsCmdDeps{Loader: loader, Registry: reg})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"invoke", "echo", "world"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Got: world")
}
