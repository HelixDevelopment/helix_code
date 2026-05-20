// Package commands — theme_lsp_subagents_i18n_test.go.
//
// Round-374 CONST-046 Phase-4 (round-6) paired-mutation tests for the
// /theme, /lsp, and /subagents slash commands. Each test asserts a
// migrated user-facing literal now routes through the package tr()
// seam: with the sentinelTranslator wired the output MUST contain the
// sentinel-wrapped message ID. If a future change re-inlines any
// literal, the assertion fails — that is the paired mutation that
// proves the migration is real, not a bluff.
package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/theme"
)

// --- /theme ---

func TestThemeCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceEnv, nil)
	assert.Equal(t, "<TR:internal_commands_theme_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_theme_usage>", c.Usage())
}

func TestThemeCommand_Status_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceEnv, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_status_header>")
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_label_name>")
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_label_depth>")
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_label_source>")
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_label_custom>")
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_custom_none>")
}

func TestThemeCommand_List_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceEnv, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_list_header>")
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_tag_builtin>")
}

func TestThemeCommand_Show_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceEnv, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "dark"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_show_heading>")
	assert.Contains(t, res.Output, "<TR:internal_commands_theme_sample_text>")
}

// --- /lsp ---

func TestLSPCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newLSPCommand(t)
	assert.Equal(t, "<TR:internal_commands_lsp_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_lsp_usage>", c.Usage())
}

func TestLSPCommand_StatusEmpty_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, "<TR:internal_commands_lsp_no_servers_running>", res.Output)
}

func TestLSPCommand_RestartStop_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"restart", "gopls"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_lsp_restarted>")

	res, err = c.Execute(context.Background(), &CommandContext{Args: []string{"stop", "gopls"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_lsp_stopped>")
}

// --- /subagents ---

func TestSubagentsCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSubagentsCommand(t)
	assert.Equal(t, "<TR:internal_commands_subagents_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_subagents_usage>", c.Usage())
}

func TestSubagentsCommand_NoneRunning_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSubagentsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, "<TR:internal_commands_subagents_none_running>", res.Output)

	res, err = c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Equal(t, "<TR:internal_commands_subagents_none_running>", res.Output)
}

func TestSubagentsCommand_Kill_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSubagentsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"kill", "sa-1"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_subagents_killed>")
}
