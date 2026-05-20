// Package commands — permissions_sessions_i18n_test.go.
//
// Round-432 CONST-046 Phase-4 (genuine-UI residual round-20)
// paired-mutation tests for the /permissions and /sessions slash
// commands. Each test asserts a migrated user-facing literal now
// routes through the package tr()/trc() seam: with the
// sentinelTranslator wired the output MUST contain the
// sentinel-wrapped message ID. If a future change re-inlines any
// literal, the assertion fails — that is the paired mutation that
// proves the migration is real, not a bluff.
package commands

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- /permissions ---

func TestPermissionsCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPermissionsCommand()
	assert.Equal(t, "<TR:internal_commands_permissions_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_permissions_usage>", c.Usage())
}

func TestPermissionsCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPermissionsCommand()
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_permissions_unknown_subcommand>")
}

func TestPermissionsCommand_List_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPermissionsCommand()
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_permissions_col_pattern>")
	assert.Contains(t, res.Output, "<TR:internal_commands_permissions_col_action>")
	assert.Contains(t, res.Output, "<TR:internal_commands_permissions_col_priority>")
	assert.Contains(t, res.Output, "<TR:internal_commands_permissions_col_source>")
	assert.Contains(t, res.Output, "<TR:internal_commands_permissions_col_description>")
	assert.Contains(t, res.Output, "<TR:internal_commands_permissions_list_footer>")
}

func TestPermissionsCommand_SetModeInvalid_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPermissionsCommand()
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"mode", "not-a-mode"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_permissions_unknown_mode>")
}

func TestPermissionsCommand_AddRemove_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPermissionsCommand()
	addRes, err := c.Execute(context.Background(),
		&CommandContext{Args: []string{"add", "Bash(*)", "allow"}})
	require.NoError(t, err)
	assert.Contains(t, addRes.Output, "<TR:internal_commands_permissions_rule_added>")

	rmRes, err := c.Execute(context.Background(),
		&CommandContext{Args: []string{"remove", "Bash(*)"}})
	require.NoError(t, err)
	assert.Contains(t, rmRes.Output, "<TR:internal_commands_permissions_rule_removed>")
}

func TestPermissionsCommand_InvalidAction_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPermissionsCommand()
	_, err := c.Execute(context.Background(),
		&CommandContext{Args: []string{"add", "Bash(*)", "nonsense"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_permissions_invalid_action>")
}

// --- /sessions ---

func TestSessionsCommand_ListHeader_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSessionsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_col_id>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_col_project>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_col_started>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_col_last_activity>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_col_msg_count>")
}

func TestSessionsCommand_Show_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, store := newSessionsCommand(t)
	seedSession(t, store, "s1", "/p/test", time.Now().UTC().Truncate(time.Second))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "s1"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_show_session>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_show_project>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_show_started>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_show_last_activity>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_show_messages>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_show_transcript_header>")
}

func TestSessionsCommand_Delete_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, store := newSessionsCommand(t)
	seedSession(t, store, "s1", "/p/test", time.Now().UTC().Truncate(time.Second))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"delete", "s1"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_sessions_deleted>")
}
