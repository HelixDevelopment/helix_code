// Package commands — registry_browser_edit_i18n_test.go.
//
// Round-399 CONST-046 Phase-4 (genuine-UI residual round-12)
// paired-mutation tests for the /browser, /edit, /git_auto_commit,
// /memory, /sandbox slash commands and the command Registry help
// surface. Each test asserts a migrated user-facing literal now
// routes through the package tr() seam: with the sentinelTranslator
// wired the output MUST contain the sentinel-wrapped message ID.
// If a future change re-inlines any literal, the assertion fails —
// that is the paired mutation that proves the migration is real,
// not a bluff (§11.4 anti-bluff).
//
// Verbatim 2026-05-19 operator mandate: "all existing tests and
// Challenges do work in anti-bluff manner - they MUST confirm that
// all tested codebase really works as expected! We had been in
// position that all tests do execute with success and all
// Challenges as well, but in reality the most of the features does
// not work and can't be used!"
//
// Mocks ALLOWED per CONST-050(A) (unit tests only).
package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- /browser ---

func TestBrowserCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewBrowserCommand(nil)
	assert.Equal(t, "<TR:internal_commands_browser_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_browser_usage>", c.Usage())
}

// --- /edit ---

func TestEditCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewEditCommand(nil)
	assert.Equal(t, "<TR:internal_commands_edit_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_edit_usage>", c.Usage())
}

// --- /git_auto_commit ---

func TestGitAutoCommitCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewGitAutoCommitCommand(nil)
	assert.Equal(t, "<TR:internal_commands_git_auto_commit_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_git_auto_commit_usage>", c.Usage())
}

// --- /memory ---

func TestMemoryCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewMemoryCommand(nil)
	assert.Equal(t, "<TR:internal_commands_memory_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_memory_usage>", c.Usage())
}

// --- /sandbox ---

func TestSandboxCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c := NewSandboxCommand(nil)
	assert.Equal(t, "<TR:internal_commands_sandbox_description>", c.Description())
	assert.Equal(t, "<TR:internal_commands_sandbox_usage>", c.Usage())
}

// --- Registry help surface ---

func TestRegistry_GetHelp_NotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	r := NewRegistry()
	out := r.GetHelp("nonexistent")
	assert.Contains(t, out, "<TR:internal_commands_registry_command_not_found>")
}

func TestRegistry_GetHelp_Found_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	r := NewRegistry()
	if err := r.Register(NewBrowserCommand(nil)); err != nil {
		t.Fatalf("register: %v", err)
	}
	out := r.GetHelp("browser")
	assert.Contains(t, out, "<TR:internal_commands_registry_help_command>")
	assert.Contains(t, out, "<TR:internal_commands_registry_help_description>")
	assert.Contains(t, out, "<TR:internal_commands_registry_help_usage>")
}

func TestRegistry_GetAllHelp_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	r := NewRegistry()
	if err := r.Register(NewBrowserCommand(nil)); err != nil {
		t.Fatalf("register: %v", err)
	}
	out := r.GetAllHelp()
	assert.True(t, strings.Contains(out, "<TR:internal_commands_registry_available_header>"),
		"GetAllHelp header must route through tr(); got %q", out)
}

// --- round-420: /edit + /git_auto_commit runtime-output migration ---
//
// Paired-mutation coverage for the genuine-(C) user-facing CLI output
// migrated in round-420. With the sentinelTranslator wired the runtime
// output MUST contain the sentinel-wrapped message ID. If a future
// change re-inlines any literal, these assertions fail — that is the
// paired mutation that proves the migration is real (§11.4 anti-bluff).

func TestEditCommand_StatusOutput_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	// nil inspector → handleStatus renders the "unavailable" branch.
	c := NewEditCommand(nil)
	res, err := c.Execute(context.Background(), &CommandContext{})
	assert.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_edit_status_heading>")
	assert.Contains(t, res.Output, "<TR:internal_commands_edit_status_unavailable>")
}

func TestGitAutoCommit_StatusOutput_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewGitAutoCommitCommand(newTestInspector(true, true))
	res, err := cmd.Execute(context.Background(), &CommandContext{})
	assert.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_git_auto_commit_status>")
}

func TestGitAutoCommit_OnOffShowOutput_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewGitAutoCommitCommand(newTestInspector(false, true))

	on, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"on"}})
	assert.NoError(t, err)
	assert.Contains(t, on.Output, "<TR:internal_commands_git_auto_commit_toggled_on>")

	off, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"off"}})
	assert.NoError(t, err)
	assert.Contains(t, off.Output, "<TR:internal_commands_git_auto_commit_toggled_off>")

	show, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	assert.NoError(t, err)
	assert.Contains(t, show.Output, "<TR:internal_commands_git_auto_commit_show>")
}
