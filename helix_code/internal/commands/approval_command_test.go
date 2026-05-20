// Package commands — approval_command_test.go (P2-F21-T06).
//
// Unit tests for the /approval slash command (status/set/show). The tests use
// a fakeApprovalInspector to record SetMode calls and inject success/error
// responses, keeping the suite hermetic — no real ApprovalManager, no env
// reads, no I/O. The command interface mirrors F19/F20 slash precedent so
// the F21 surface composes with the existing registry.
package commands

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/approval"
)

// fakeApprovalInspector implements ApprovalInspector with recorded calls so
// the /approval tests can assert delegation + injected error paths.
type fakeApprovalInspector struct {
	mode     approval.ApprovalMode
	source   approval.ResolvedSource
	setErr   error
	setCalls []approval.ApprovalMode
}

func (f *fakeApprovalInspector) Mode() approval.ApprovalMode    { return f.mode }
func (f *fakeApprovalInspector) Source() approval.ResolvedSource { return f.source }
func (f *fakeApprovalInspector) SetMode(newMode approval.ApprovalMode) error {
	f.setCalls = append(f.setCalls, newMode)
	if f.setErr != nil {
		return f.setErr
	}
	f.mode = newMode
	f.source = approval.SourceRuntime
	return nil
}

func newFakeInspector(mode approval.ApprovalMode, source approval.ResolvedSource) *fakeApprovalInspector {
	return &fakeApprovalInspector{mode: mode, source: source}
}

func TestApprovalCommand_Name(t *testing.T) {
	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	assert.Equal(t, "approval", c.Name())
	// CONST-046: Description/Usage route through the i18n seam — default
	// NoopTranslator echoes the message ID verbatim.
	assert.Equal(t, "internal_commands_approval_description", c.Description())
	assert.Equal(t, "internal_commands_approval_usage", c.Usage())
	assert.Nil(t, c.Aliases())
}

func TestApprovalCommand_DefaultIsStatus(t *testing.T) {
	c := NewApprovalCommand(newFakeInspector(approval.ModeAutoEdit, approval.SourceEnv))
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: status header routes through the i18n seam — default
	// NoopTranslator echoes the message ID verbatim.
	assert.Contains(t, res.Output, "internal_commands_approval_status_header")
	assert.Contains(t, res.Output, "auto-edit")
}

func TestApprovalCommand_StatusShowsMode(t *testing.T) {
	for _, m := range approval.AllModes() {
		t.Run(m.String(), func(t *testing.T) {
			c := NewApprovalCommand(newFakeInspector(m, approval.SourceDefault))
			res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
			require.NoError(t, err)
			assert.Contains(t, res.Output, m.String())
		})
	}
}

func TestApprovalCommand_StatusShowsSource(t *testing.T) {
	// CONST-046: source labels route through the i18n seam — default
	// NoopTranslator echoes the message ID verbatim, so the assertion
	// targets the per-source message ID rather than the English literal.
	cases := []struct {
		name   string
		source approval.ResolvedSource
		want   string
	}{
		{"flag", approval.SourceFlag, "internal_commands_approval_source_flag"},
		{"env", approval.SourceEnv, "internal_commands_approval_source_env"},
		{"config", approval.SourceConfig, "internal_commands_approval_source_config"},
		{"default", approval.SourceDefault, "internal_commands_approval_source_default"},
		{"runtime", approval.SourceRuntime, "internal_commands_approval_source_runtime"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, tc.source))
			res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
			require.NoError(t, err)
			assert.Contains(t, res.Output, tc.want, "source label missing")
		})
	}
}

func TestApprovalCommand_StatusShowsSandboxRule(t *testing.T) {
	cases := []struct {
		mode    approval.ApprovalMode
		sandbox string
	}{
		{approval.ModeSuggest, "n/a"},
		{approval.ModeAutoEdit, "optional"},
		{approval.ModeFullAuto, "required"},
		{approval.ModeDangerous, "skipped"},
	}
	for _, tc := range cases {
		t.Run(tc.mode.String(), func(t *testing.T) {
			c := NewApprovalCommand(newFakeInspector(tc.mode, approval.SourceDefault))
			res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
			require.NoError(t, err)
			// CONST-046: label routes through the i18n seam (message-ID
			// echo); the rule value is descriptor metadata, unchanged.
			assert.Contains(t, res.Output, "internal_commands_approval_label_sandbox")
			assert.Contains(t, res.Output, tc.sandbox)
		})
	}
}

func TestApprovalCommand_SetCallsManager(t *testing.T) {
	insp := newFakeInspector(approval.ModeSuggest, approval.SourceDefault)
	c := NewApprovalCommand(insp)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "auto-edit"}})
	require.NoError(t, err)
	require.Len(t, insp.setCalls, 1)
	assert.Equal(t, approval.ModeAutoEdit, insp.setCalls[0])
}

func TestApprovalCommand_SetReportsSuccess(t *testing.T) {
	insp := newFakeInspector(approval.ModeSuggest, approval.SourceEnv)
	c := NewApprovalCommand(insp)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "auto-edit"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: the transition line routes through the i18n seam. The
	// default NoopTranslator echoes the message ID (old/new mode names
	// are template data, resolved by a real translator at boot).
	assert.Contains(t, res.Output, "internal_commands_approval_mode_set")
}

func TestApprovalCommand_SetFullAutoWarning(t *testing.T) {
	insp := newFakeInspector(approval.ModeSuggest, approval.SourceDefault)
	c := NewApprovalCommand(insp)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "full-auto"}})
	require.NoError(t, err)
	// CONST-046: the full-auto advisory routes through the i18n seam.
	assert.Contains(t, res.Output, "internal_commands_approval_warn_full_auto")
}

func TestApprovalCommand_SetReportsManagerError(t *testing.T) {
	insp := newFakeInspector(approval.ModeAutoEdit, approval.SourceEnv)
	insp.setErr = approval.ErrSandboxRequired
	c := NewApprovalCommand(insp)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "full-auto"}})
	// Manager-side error surfaces as command error.
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, approval.ErrSandboxRequired))
	// Mode was NOT changed in the inspector.
	assert.Equal(t, approval.ModeAutoEdit, insp.mode)
}

func TestApprovalCommand_SetMissingModeArgErrors(t *testing.T) {
	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing mode")
}

func TestApprovalCommand_SetUnknownModeErrors(t *testing.T) {
	insp := newFakeInspector(approval.ModeSuggest, approval.SourceDefault)
	c := NewApprovalCommand(insp)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"set", "banana"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, approval.ErrInvalidMode))
	// SetMode was NOT called for an unparseable input.
	assert.Empty(t, insp.setCalls)
}

func TestApprovalCommand_ShowSpecificMode(t *testing.T) {
	for _, m := range approval.AllModes() {
		t.Run(m.String(), func(t *testing.T) {
			c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
			res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", m.String()}})
			require.NoError(t, err)
			// CONST-046: descriptor labels route through the i18n seam —
			// default NoopTranslator echoes the message ID verbatim.
			assert.Contains(t, res.Output, "internal_commands_approval_label_mode")
			assert.Contains(t, res.Output, m.String())
			assert.Contains(t, res.Output, "internal_commands_approval_label_description")
			assert.Contains(t, res.Output, "internal_commands_approval_label_sandbox")
			assert.Contains(t, res.Output, "internal_commands_approval_label_network")
			assert.Contains(t, res.Output, "internal_commands_approval_label_safety")
		})
	}
}

func TestApprovalCommand_ShowAllListsAllFour(t *testing.T) {
	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "all"}})
	require.NoError(t, err)
	for _, m := range approval.AllModes() {
		assert.Contains(t, res.Output, m.String(), "mode %s missing from show all", m)
	}
	// Safety-order ladder is preserved: suggest appears before auto-edit
	// before full-auto before dangerously-bypass.
	out := res.Output
	idxSuggest := strings.Index(out, string(approval.ModeSuggest))
	idxAutoEdit := strings.Index(out, string(approval.ModeAutoEdit))
	idxFullAuto := strings.Index(out, string(approval.ModeFullAuto))
	idxDangerous := strings.Index(out, string(approval.ModeDangerous))
	assert.True(t, idxSuggest >= 0 && idxSuggest < idxAutoEdit)
	assert.True(t, idxAutoEdit < idxFullAuto)
	assert.True(t, idxFullAuto < idxDangerous)
}

func TestApprovalCommand_ShowMissingArgListsAll(t *testing.T) {
	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.NoError(t, err)
	for _, m := range approval.AllModes() {
		assert.Contains(t, res.Output, m.String())
	}
}

func TestApprovalCommand_ShowUnknownMode(t *testing.T) {
	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "banana"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, approval.ErrInvalidMode))
}

func TestApprovalCommand_UnknownSubcommandErrors(t *testing.T) {
	c := NewApprovalCommand(newFakeInspector(approval.ModeSuggest, approval.SourceDefault))
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown subcommand")
}
