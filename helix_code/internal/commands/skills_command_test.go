package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSkillsCommandWithLoader(t *testing.T) (*SkillsCommand, *SkillRegistry, *SkillLoader, string) {
	t.Helper()
	dir := t.TempDir()
	skills := filepath.Join(dir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))
	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())
	return NewSkillsCommand(loader, reg), reg, loader, skills
}

func writeSkill(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0644))
}

func TestSlashSkills_ListEmpty(t *testing.T) {
	c, _, _, _ := newSkillsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "NAME")
}

func TestSlashSkills_ListShowsLoaded(t *testing.T) {
	c, _, loader, dir := newSkillsCommandWithLoader(t)
	writeSkill(t, dir, "refactor",
		"---\ndescription: Refactor a component\ntriggers: [\"^refactor\"]\n---\nbody")
	require.NoError(t, loader.Reload())
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "refactor")
	assert.Contains(t, res.Output, "Refactor a component")
}

func TestSlashSkills_ShowReturnsBodyAndTriggers(t *testing.T) {
	c, _, loader, dir := newSkillsCommandWithLoader(t)
	writeSkill(t, dir, "iso",
		"---\ndescription: x\ntriggers: [\"^pat$\"]\nrequires_isolation: true\n---\nthe-body")
	require.NoError(t, loader.Reload())
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "iso"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "the-body")
	assert.Contains(t, res.Output, "^pat$")
	assert.Contains(t, res.Output, "true") // requires_isolation
}

func TestSlashSkills_ShowUnknownErrors(t *testing.T) {
	c, _, _, _ := newSkillsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "ghost"}})
	require.Error(t, err)
}

func TestSlashSkills_ReloadRefreshes(t *testing.T) {
	c, reg, _, dir := newSkillsCommandWithLoader(t)
	writeSkill(t, dir, "fresh", "---\ndescription: f\ntriggers: [\"^x\"]\n---\nbody")
	_, ok := reg.Get("fresh")
	assert.False(t, ok)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"reload"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "reload")
	_, ok = reg.Get("fresh")
	assert.True(t, ok)
}

func TestSlashSkills_InvokeRenders(t *testing.T) {
	c, _, loader, dir := newSkillsCommandWithLoader(t)
	writeSkill(t, dir, "echo",
		"---\ndescription: e\ntriggers: [\"^x\"]\nvariables:\n  default_arg: \"hello\"\n---\nGot: {{ARG.default_arg}}")
	require.NoError(t, loader.Reload())
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"invoke", "echo"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "Got: hello")
}

func TestSlashSkills_InvokeUnknownErrors(t *testing.T) {
	c, _, _, _ := newSkillsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"invoke", "ghost"}})
	require.Error(t, err)
}

func TestSlashSkills_DefaultIsList(t *testing.T) {
	c, _, _, _ := newSkillsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "NAME")
}

func TestSlashSkills_UnknownSubcommandErrors(t *testing.T) {
	c, _, _, _ := newSkillsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}
