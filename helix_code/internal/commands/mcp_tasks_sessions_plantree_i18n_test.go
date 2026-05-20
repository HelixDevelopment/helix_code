// Package commands — mcp_tasks_sessions_plantree_i18n_test.go.
//
// Round-393 CONST-046 Phase-4 (genuine-UI residual round-11)
// paired-mutation tests for the /mcp, /tasks, /sessions, and
// /plantree slash commands. Each test asserts a migrated
// user-facing literal now routes through the package tr() seam:
// with the sentinelTranslator wired the output MUST contain the
// sentinel-wrapped message ID. If a future change re-inlines any
// literal, the assertion fails — that is the paired mutation that
// proves the migration is real, not a bluff (§11.4 anti-bluff).
//
// Verbatim 2026-05-19 operator mandate: "all existing tests and
// Challenges do work in anti-bluff manner - they MUST confirm that
// all tested codebase really works as expected!"
//
// Mocks ALLOWED per CONST-050(A) (unit tests only).
package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/mcp"
	"dev.helix.code/internal/workflow"
)

// --- /mcp ---

func TestMCPCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewMCPCommand(nil)
	assert.Equal(t, "<TR:internal_commands_mcp_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_mcp_usage>", c.Usage())
}

func TestMCPCommand_TestUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewMCPCommand(mcp.NewManager())
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"test"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_mcp_test_usage>")
}

func TestMCPCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewMCPCommand(mcp.NewManager())
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_mcp_unknown_subcommand>")
}

// --- /tasks ---

func TestTasksCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewTasksCommand(nil)
	assert.Equal(t, "<TR:internal_commands_tasks_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_tasks_usage>", c.Usage())
}

func TestTasksCommand_OutputUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewTasksCommand(workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{}))
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"output"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_tasks_output_usage>")
}

func TestTasksCommand_StopUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewTasksCommand(workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{}))
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"stop"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_tasks_stop_usage>")
}

func TestTasksCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewTasksCommand(workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{}))
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_tasks_unknown_subcommand>")
}

// --- /sessions ---

func TestSessionsCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewSessionsCommand(nil, "")
	assert.Equal(t, "<TR:internal_commands_sessions_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_sessions_usage>", c.Usage())
}

func TestSessionsCommand_ShowUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewSessionsCommand(nil, "")
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_sessions_show_usage>")
}

func TestSessionsCommand_ResumeUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewSessionsCommand(nil, "")
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"resume"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_sessions_resume_usage>")
}

func TestSessionsCommand_DeleteUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewSessionsCommand(nil, "")
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"delete"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_sessions_delete_usage>")
}

func TestSessionsCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewSessionsCommand(nil, "")
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_sessions_unknown_subcommand>")
}

// --- /plantree ---

func TestPlanTreeCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPlanTreeCommand(nil, nil)
	assert.Equal(t, "<TR:internal_commands_plantree_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_plantree_usage>", c.Usage())
}
