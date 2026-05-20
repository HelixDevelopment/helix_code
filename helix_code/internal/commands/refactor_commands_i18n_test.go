// Package commands — refactor_commands_i18n_test.go.
//
// Round-366 CONST-046 Phase-4 (round-86) paired-mutation tests for
// the /aider, /kilocode, and /roocode slash commands. Each test
// asserts a migrated user-facing literal now routes through the
// package tr() seam: with the sentinelTranslator wired the output
// MUST contain the sentinel-wrapped message ID. If a future change
// re-inlines any literal, the assertion fails — that is the paired
// mutation that proves the migration is real, not a bluff.
//
// Description()/Usage() and the no-args / unknown-subcommand result
// branches do not dereference command dependencies, so nil deps are
// safe for these assertions (mirrors continue_command_test.go).
package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- /aider ---

func TestAiderCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewAiderCommand(nil, nil)
	assert.Equal(t, "<TR:internal_commands_aider_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_aider_usage>", c.Usage())
}

func TestAiderCommand_NoArgs_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewAiderCommand(nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_aider_usage_full>")
}

func TestAiderCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewAiderCommand(nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_aider_unknown_subcommand>")
}

func TestAiderCommand_VoiceUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewAiderCommand(nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"voice"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_aider_voice_usage>")
}

func TestAiderCommand_RepoMapHint_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewAiderCommand(nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"repomap"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_aider_repomap_hint>")
}

// --- /kilocode ---

func TestKilocodeCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewKilocodeCommand(nil, nil, nil)
	assert.Equal(t, "<TR:internal_commands_kilocode_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_kilocode_usage>", c.Usage())
}

func TestKilocodeCommand_NoArgs_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewKilocodeCommand(nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_kilocode_usage_full>")
}

func TestKilocodeCommand_RenameUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewKilocodeCommand(nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"rename", "x"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_kilocode_rename_usage>")
}

func TestKilocodeCommand_EditHint_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewKilocodeCommand(nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"edit"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_kilocode_edit_hint>")
}

func TestKilocodeCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewKilocodeCommand(nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_kilocode_unknown_subcommand>")
}

// --- /roocode ---

func TestRooCodeCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewRooCodeCommand(nil, nil, nil, nil)
	assert.Equal(t, "<TR:internal_commands_roocode_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_roocode_usage>", c.Usage())
}

func TestRooCodeCommand_NoArgs_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewRooCodeCommand(nil, nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_roocode_usage_full>")
}

func TestRooCodeCommand_DelegateUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewRooCodeCommand(nil, nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"delegate"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_roocode_delegate_usage>")
}

func TestRooCodeCommand_GenerateUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewRooCodeCommand(nil, nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"generate", "go"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_roocode_generate_usage>")
}

func TestRooCodeCommand_ReviewUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewRooCodeCommand(nil, nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"review"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_roocode_review_usage>")
}

func TestRooCodeCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewRooCodeCommand(nil, nil, nil, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_roocode_unknown_subcommand>")
}
