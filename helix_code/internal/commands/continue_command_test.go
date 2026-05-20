// Package commands — continue_command_test.go.
//
// Round-362 CONST-046 paired-mutation tests for the /continue slash
// command. Each test asserts the migrated user-facing literal now
// routes through the package tr() seam: with a sentinel translator
// wired the output MUST contain the sentinel-wrapped message ID; an
// inlined literal fails the assertion.
package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/continua"
)

func newContinueCommand() *ContinueCommand {
	return NewContinueCommand(
		continua.NewWorkspaceEditor(),
		continua.NewCompletionEngine(),
		continua.NewChatManager(),
	)
}

func TestContinueCommand_NameAndAliases(t *testing.T) {
	c := newContinueCommand()
	assert.Equal(t, "continue", c.Name())
	assert.Equal(t, []string{"cont"}, c.Aliases())
}

func TestContinueCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	assert.Equal(t, "<TR:internal_commands_continue_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_continue_usage>", c.Usage())
}

func TestContinueCommand_NoArgs_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_usage_full>")
}

func TestContinueCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_unknown_subcommand>")
}

func TestContinueCommand_EditUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"edit"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_edit_usage>")
}

func TestContinueCommand_CompleteUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"complete"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_complete_usage>")
}

func TestContinueCommand_ChatUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"chat"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_chat_usage>")
}

func TestContinueCommand_ChatCreate_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"chat", "create", "demo"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_chat_created>")
}

func TestContinueCommand_ChatList_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"chat", "list"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_chat_count>")
}

func TestContinueCommand_ChatAdd_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"chat", "add"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_chat_add_hint>")
}

func TestContinueCommand_DiffUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"diff"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_diff_usage>")
}

func TestContinueCommand_DiffHint_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := newContinueCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"diff", "a.go", "b.go"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_continue_diff_hint>")
}
