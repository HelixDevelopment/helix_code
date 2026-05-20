// subagents_command_test.go — P1-F15-T09 tests for the /subagents slash command.
//
// These tests are TDD-first: they define the contract for SubagentsCommand
// before the implementation exists.
//
// CONST-042 anchor: TestSubagentsCommand_StatusStructHasNoPromptField uses
// reflection to assert that subagent.SubagentStatus has no Prompt field.
// This is the structural anti-leak guarantee — if a future change adds a
// Prompt field, the test fails immediately, blocking the leak at the type
// level rather than at the rendering level.
package commands

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/agent/subagent"
)

// fakeSubagentManager is an in-test implementation of the
// commands.SubagentManager interface. It records calls to Kill so tests
// can assert dispatch behaviour without spawning real subagents.
type fakeSubagentManager struct {
	statuses  []subagent.SubagentStatus
	killCalls []string
	killErr   error
}

func (f *fakeSubagentManager) Status() []subagent.SubagentStatus {
	return f.statuses
}

func (f *fakeSubagentManager) Kill(id string) error {
	f.killCalls = append(f.killCalls, id)
	return f.killErr
}

func newSubagentsCommand(t *testing.T) (*SubagentsCommand, *fakeSubagentManager) {
	t.Helper()
	mgr := &fakeSubagentManager{}
	return NewSubagentsCommand(mgr), mgr
}

func TestSubagentsCommand_NameDescription(t *testing.T) {
	c, _ := newSubagentsCommand(t)
	assert.Equal(t, "subagents", c.Name())
	// Description()/Usage() route through the CONST-046 tr() seam; the
	// default NoopTranslator echoes the message ID verbatim.
	assert.Equal(t, "internal_commands_subagents_description", c.Description())
	assert.Equal(t, "internal_commands_subagents_usage", c.Usage())
	assert.Nil(t, c.Aliases())
}

func TestSubagentsCommand_DefaultIsList(t *testing.T) {
	c, mgr := newSubagentsCommand(t)
	mgr.statuses = []subagent.SubagentStatus{
		{
			ID:          "sub-abc123",
			Description: "refactor login",
			Isolation:   subagent.IsolationWorktree,
			StartedAt:   time.Now().Add(-12 * time.Second),
			Elapsed:     12 * time.Second,
		},
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// list output must include ID, DESCRIPTION, ISOLATION, ELAPSED columns
	// but NOT the STARTED-AT column (that's status-only).
	assert.Contains(t, res.Output, "ID")
	assert.Contains(t, res.Output, "DESCRIPTION")
	assert.Contains(t, res.Output, "ISOLATION")
	assert.Contains(t, res.Output, "ELAPSED")
	assert.NotContains(t, res.Output, "STARTED-AT")
	assert.Contains(t, res.Output, "sub-abc123")
	assert.Contains(t, res.Output, "refactor login")
	assert.Contains(t, res.Output, "worktree")
}

func TestSubagentsCommand_ListShowsRunning(t *testing.T) {
	c, mgr := newSubagentsCommand(t)
	mgr.statuses = []subagent.SubagentStatus{
		{
			ID:          "sub-abc123",
			Description: "refactor login",
			Isolation:   subagent.IsolationWorktree,
			StartedAt:   time.Now().Add(-12 * time.Second),
			Elapsed:     12 * time.Second,
		},
		{
			ID:          "sub-def456",
			Description: "explore codebase",
			Isolation:   subagent.IsolationNone,
			StartedAt:   time.Now().Add(-3 * time.Second),
			Elapsed:     3 * time.Second,
		},
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "sub-abc123")
	assert.Contains(t, res.Output, "sub-def456")
	assert.Contains(t, res.Output, "refactor login")
	assert.Contains(t, res.Output, "explore codebase")
	assert.Contains(t, res.Output, "worktree")
	assert.Contains(t, res.Output, "none")
}

func TestSubagentsCommand_ListEmpty(t *testing.T) {
	c, _ := newSubagentsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "internal_commands_subagents_none_running", strings.TrimSpace(res.Output))
}

func TestSubagentsCommand_StatusShowsExtraColumns(t *testing.T) {
	c, mgr := newSubagentsCommand(t)
	started := time.Date(2026, 5, 6, 1, 55, 0, 0, time.UTC)
	mgr.statuses = []subagent.SubagentStatus{
		{
			ID:          "sub-abc123",
			Description: "refactor login",
			Isolation:   subagent.IsolationWorktree,
			StartedAt:   started,
			Elapsed:     12 * time.Second,
		},
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// Status must include the STARTED-AT header (the extra column).
	assert.Contains(t, res.Output, "STARTED-AT")
	assert.Contains(t, res.Output, "ID")
	assert.Contains(t, res.Output, "DESCRIPTION")
	assert.Contains(t, res.Output, "ISOLATION")
	assert.Contains(t, res.Output, "ELAPSED")
	assert.Contains(t, res.Output, "sub-abc123")
	// RFC3339 timestamp of the started time must appear (UTC).
	assert.Contains(t, res.Output, "2026-05-06T01:55:00Z")
}

func TestSubagentsCommand_StatusEmpty(t *testing.T) {
	c, _ := newSubagentsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "internal_commands_subagents_none_running", strings.TrimSpace(res.Output))
}

func TestSubagentsCommand_KillCallsManager(t *testing.T) {
	c, mgr := newSubagentsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"kill", "sub-abc123"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, []string{"sub-abc123"}, mgr.killCalls)
	// Output routes through the CONST-046 tr() seam; the NoopTranslator
	// echoes the message ID (the subagent ID is template data).
	assert.Contains(t, res.Output, "internal_commands_subagents_killed")
}

func TestSubagentsCommand_KillMissingID(t *testing.T) {
	c, mgr := newSubagentsCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"kill"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/subagents kill <id>")
	// Manager should NOT have been invoked.
	assert.Empty(t, mgr.killCalls)
}

func TestSubagentsCommand_KillUnknownIDPropagates(t *testing.T) {
	c, mgr := newSubagentsCommand(t)
	mgr.killErr = errors.New(`subagent: Kill: no running subagent with id "sub-nope"`)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"kill", "sub-nope"}})
	require.Error(t, err)
	// Error message must include the manager's error text so the caller
	// can see exactly why the kill failed.
	assert.Contains(t, err.Error(), "no running subagent")
	assert.Equal(t, []string{"sub-nope"}, mgr.killCalls)
}

func TestSubagentsCommand_UnknownSubcommandErrors(t *testing.T) {
	c, _ := newSubagentsCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown subcommand")
	assert.Contains(t, err.Error(), "bogus")
}

// TestSubagentsCommand_StatusStructHasNoPromptField is the CONST-042
// structural guarantee: subagent.SubagentStatus is the ONLY public surface
// for status output, and it MUST NOT carry a Prompt (or any prompt-shaped)
// field. If a future change adds one, this test fails before any leak can
// reach the user-visible /subagents output.
func TestSubagentsCommand_StatusStructHasNoPromptField(t *testing.T) {
	var s subagent.SubagentStatus
	typ := reflect.TypeOf(s)
	require.Equal(t, reflect.Struct, typ.Kind())
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		// CONST-042: any field named "Prompt" or with a "prompt" json tag
		// is forbidden — the prompt is the secret-bearing surface.
		if name == "Prompt" {
			t.Fatalf("CONST-042 violation: SubagentStatus must NOT have a Prompt field (found field %q)", name)
		}
		tag := typ.Field(i).Tag.Get("json")
		if strings.HasPrefix(tag, "prompt") {
			t.Fatalf("CONST-042 violation: SubagentStatus field %q has json tag %q (must not expose prompt)", name, tag)
		}
	}
}
