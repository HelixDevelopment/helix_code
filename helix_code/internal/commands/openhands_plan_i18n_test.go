// Package commands — openhands_plan_i18n_test.go.
//
// Round-370 CONST-046 Phase-4 (round-90) paired-mutation tests for
// the /openhands and /plan slash commands. Each test asserts a
// migrated user-facing literal now routes through the package tr()
// seam: with the sentinelTranslator wired the output MUST contain
// the sentinel-wrapped message ID. If a future change re-inlines any
// literal, the assertion fails — that is the paired mutation that
// proves the migration is real, not a bluff.
//
// Description()/Usage() and the no-args / unknown-subcommand /
// usage-hint result branches do not dereference command dependencies
// (workspace manager / planner / mode controller), so nil deps are
// safe for these assertions (mirrors refactor_commands_i18n_test.go).
package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- /openhands ---

func TestOpenhandsCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewOpenhandsCommand(nil)
	assert.Equal(t, "<TR:internal_commands_openhands_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_openhands_usage>", c.Usage())
}

func TestOpenhandsCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewOpenhandsCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_openhands_unknown_subcommand>")
}

func TestOpenhandsCommand_CreateUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewOpenhandsCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"create"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_openhands_create_usage>")
}

func TestOpenhandsCommand_CleanupUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewOpenhandsCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"cleanup"}})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Message, "<TR:internal_commands_openhands_cleanup_usage>")
}

// --- /plan ---

func TestPlanCommand_Description_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewPlanCommand(nil, nil)
	assert.Equal(t, "<TR:internal_commands_plan_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_plan_usage>", c.Usage())
}
