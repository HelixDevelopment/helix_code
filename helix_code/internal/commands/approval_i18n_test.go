// Package commands — approval_i18n_test.go.
//
// Round-412 §11.4 CONST-046 Phase-4 (genuine-UI round-15) paired-mutation
// tests for the /approval slash command. Each test asserts a migrated
// user-facing literal now routes through the package tr() seam: with the
// sentinelTranslator wired the output MUST contain the sentinel-wrapped
// message ID. If a future change re-inlines any literal, the assertion
// fails — that is the paired mutation that proves the migration is real,
// not a bluff.
package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/approval"
)

func TestApprovalCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	assert.Equal(t, "<TR:internal_commands_approval_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_approval_usage>", c.Usage())
}

func TestApprovalCommand_Status_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewApprovalCommand(newFakeInspector(approval.ModeAutoEdit, approval.SourceEnv))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_status_header>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_mode>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_source>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_sandbox>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_network>")
	// SourceEnv resolves through the env-source message ID.
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_source_env>")
}

func TestApprovalCommand_StatusSourceLabels_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cases := []struct {
		source approval.ResolvedSource
		msgID  string
	}{
		{approval.SourceFlag, "<TR:internal_commands_approval_source_flag>"},
		{approval.SourceEnv, "<TR:internal_commands_approval_source_env>"},
		{approval.SourceConfig, "<TR:internal_commands_approval_source_config>"},
		{approval.SourceDefault, "<TR:internal_commands_approval_source_default>"},
		{approval.SourceRuntime, "<TR:internal_commands_approval_source_runtime>"},
	}
	for _, tc := range cases {
		c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, tc.source))
		res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
		require.NoError(t, err)
		assert.Contains(t, res.Output, tc.msgID, "source label %d must route through translator", tc.source)
	}
}

func TestApprovalCommand_Set_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "auto-edit"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_mode_set>")
}

func TestApprovalCommand_SetFullAutoWarning_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "full-auto"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_warn_full_auto>")
}

func TestApprovalCommand_SetDangerousWarning_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "dangerously-bypass"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_warn_dangerous>")
}

func TestApprovalCommand_Show_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "all"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_mode>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_description>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_sandbox>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_network>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_label_safety>")
	// Safety-ladder rungs all route through the translator.
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_safety_most_restrictive>")
	assert.Contains(t, res.Output, "<TR:internal_commands_approval_safety_least_restrictive>")
}
