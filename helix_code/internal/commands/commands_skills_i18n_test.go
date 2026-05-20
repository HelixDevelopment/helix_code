// Package commands — commands_skills_i18n_test.go.
//
// Round-389 CONST-046 Phase-4 (round-10) paired-mutation tests for the
// /commands and /skills slash commands. Each test asserts a migrated
// user-facing literal now routes through the package tr() seam: with
// the sentinelTranslator wired the output MUST contain the
// sentinel-wrapped message ID. If a future change re-inlines any
// literal, the assertion fails — that is the paired mutation that
// proves the migration is real, not a bluff (§11.4 anti-bluff).
//
// Mocks ALLOWED per CONST-050(A) (unit tests only).
package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- /commands ---

func TestCommandsCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newCommandsCommandWithLoader(t)
	assert.Equal(t, "<TR:internal_commands_commands_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_commands_usage>", c.Usage())
}

func TestCommandsCommand_ListHeader_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newCommandsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_commands_table_header>")
}

func TestCommandsCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newCommandsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_commands_unknown_subcommand>")
}

func TestCommandsCommand_ShowNotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newCommandsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "ghost"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_commands_show_not_found>")
}

func TestCommandsCommand_ShowDetail_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, loader, cmds := newCommandsCommandWithLoader(t)
	require.NoError(t, os.WriteFile(filepath.Join(cmds, "hello.md"),
		[]byte("Hello {{ARG1}}"), 0644))
	require.NoError(t, loader.Reload())
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "hello"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_commands_show_detail>")
}

func TestCommandsCommand_ReloadResult_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newCommandsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"reload"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_commands_reload_result>")
}

func TestCommandsCommand_RunNotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newCommandsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"run", "ghost"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_commands_run_not_found>")
}

// --- /skills ---

func TestSkillsCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newSkillsCommandWithLoader(t)
	assert.Equal(t, "<TR:internal_commands_skills_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_skills_usage>", c.Usage())
}

func TestSkillsCommand_ListHeader_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newSkillsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_skills_table_header>")
}

func TestSkillsCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newSkillsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_skills_unknown_subcommand>")
}

func TestSkillsCommand_ShowNotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newSkillsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "ghost"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_skills_show_not_found>")
}

func TestSkillsCommand_Show_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, loader, dir := newSkillsCommandWithLoader(t)
	writeSkill(t, dir, "iso",
		"---\ndescription: x\ntriggers: [\"^pat$\"]\nrequires_isolation: true\n---\nthe-body")
	require.NoError(t, loader.Reload())
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "iso"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_skills_show_detail>")
}

func TestSkillsCommand_ReloadResult_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newSkillsCommandWithLoader(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"reload"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_skills_reload_result>")
}

func TestSkillsCommand_InvokeNotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _, _, _ := newSkillsCommandWithLoader(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"invoke", "ghost"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_skills_invoke_not_found>")
}
