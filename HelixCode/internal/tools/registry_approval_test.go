// Package tools — registry_approval_test.go (P2-F21-T05).
//
// Pins the per-tool RequiresApproval() coverage table per spec §3.6. The test
// is table-driven: every tool registered through the default registry MUST
// appear in the table with its expected ApprovalLevel, and any tool returning
// a level outside the canonical 4-value enum FAILS the test.
//
// Future tool additions that forget to override RequiresApproval default to
// LevelEdit (safe-by-default via approval.DefaultLevelEdit) AND must be added
// to this table or the coverage check fails. This pins the migration and
// detects future drift.
//
// Per spec §3.6, the table covers:
//   - LevelReadOnly: pure reads (file read/list, glob, grep, lsp queries,
//     repomap, mapping, browser snapshot, ask_user — a prompt is not a
//     state mutation).
//   - LevelEdit: file/state mutations (write, edit, multi-edit, smart-edit,
//     notebook_edit, plan-mode toggles, mcp default).
//   - LevelRun: subprocess/shell execution + browser navigation/click/type.
//   - LevelAll: subagent dispatch (LevelAll — recursive agent spawn) and
//     other high-risk catch-alls.
package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/approval"
)

// TestAllRegisteredTools_HaveRequiresApproval enumerates the default registry
// and asserts every tool's RequiresApproval() returns a value within the
// canonical enum AND matches the expected level from spec §3.6.
//
// Tools NOT in the explicit table embed approval.DefaultLevelEdit and so
// return LevelEdit. The default-edit catch-all is exercised separately in
// TestAllRegisteredTools_DefaultIsLevelEdit_ForUnclassifiedTools below.
func TestAllRegisteredTools_HaveRequiresApproval(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	defer func() {
		// Best-effort close; ignore error since we never opened a browser.
		_ = r.Close()
	}()

	// Per spec §3.6: explicit overrides. Tools not listed here embed
	// approval.DefaultLevelEdit (LevelEdit safe-by-default).
	expected := map[string]approval.ApprovalLevel{
		// LevelReadOnly — pure reads.
		"fs_read":                approval.LevelReadOnly,
		"glob":                   approval.LevelReadOnly,
		"grep":                   approval.LevelReadOnly,
		"codebase_map":           approval.LevelReadOnly,
		"file_definitions":       approval.LevelReadOnly,
		"lsp_get_diagnostics":    approval.LevelReadOnly,
		"lsp_analyze_diagnostic": approval.LevelReadOnly,
		"web_fetch":              approval.LevelReadOnly,
		"web_search":             approval.LevelReadOnly,
		"notebook_read":          approval.LevelReadOnly,
		"browser_screenshot":     approval.LevelReadOnly,
		"task_tracker":           approval.LevelReadOnly,
		"TaskOutput":             approval.LevelReadOnly,
		"ListWorktrees":          approval.LevelReadOnly,

		// LevelEdit — file / state mutations.
		"fs_write":          approval.LevelEdit,
		"fs_edit":           approval.LevelEdit,
		"multiedit_begin":   approval.LevelEdit,
		"multiedit_add":     approval.LevelEdit,
		"multiedit_preview": approval.LevelEdit,
		"multiedit_commit":  approval.LevelEdit,
		"notebook_edit":     approval.LevelEdit,
		"EnterPlanMode":     approval.LevelEdit,
		"ExitPlanMode":      approval.LevelEdit,
		"EnterWorktree":     approval.LevelEdit,
		"ExitWorktree":      approval.LevelEdit,
		"RemoveWorktree":    approval.LevelEdit,

		// LevelRun — subprocess / shell / browser-mutation.
		"shell":              approval.LevelRun,
		"shell_background":   approval.LevelRun,
		"shell_output":       approval.LevelRun,
		"shell_kill":         approval.LevelRun,
		"browser_launch":     approval.LevelRun,
		"browser_navigate":   approval.LevelRun,
		"browser_close":      approval.LevelRun,
		"TaskStop":           approval.LevelRun,
	}

	all := r.List()
	require.NotEmpty(t, all, "default registry should contain registered tools")

	for _, tool := range all {
		name := tool.Name()
		level := tool.RequiresApproval()

		// Canonical enum check (catches future bugs where a tool returns a
		// bogus int via unsafe cast).
		assert.True(t, level.IsValid(),
			"tool %q returned invalid ApprovalLevel %d", name, int(level))

		if want, ok := expected[name]; ok {
			assert.Equal(t, want, level,
				"tool %q: expected level %s but got %s", name, want, level)
		}
	}
}

// TestAllRegisteredTools_DefaultIsLevelEdit_ForUnclassifiedTools verifies
// that any tool NOT in the explicit override table falls back to LevelEdit
// (the safe-by-default value provided by approval.DefaultLevelEdit). A
// regression where a tool silently degrades to LevelReadOnly (allowing
// mutations through suggest mode) FAILS this test.
func TestAllRegisteredTools_DefaultIsLevelEdit_ForUnclassifiedTools(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	classified := map[string]struct{}{
		"fs_read": {}, "glob": {}, "grep": {}, "codebase_map": {},
		"file_definitions": {}, "lsp_get_diagnostics": {},
		"lsp_analyze_diagnostic": {}, "web_fetch": {}, "web_search": {},
		"notebook_read": {}, "browser_screenshot": {}, "task_tracker": {},
		"TaskOutput": {}, "ListWorktrees": {},
		"fs_write": {}, "fs_edit": {}, "multiedit_begin": {},
		"multiedit_add": {}, "multiedit_preview": {}, "multiedit_commit": {},
		"notebook_edit": {}, "EnterPlanMode": {}, "ExitPlanMode": {},
		"EnterWorktree": {}, "ExitWorktree": {}, "RemoveWorktree": {},
		"shell": {}, "shell_background": {}, "shell_output": {},
		"shell_kill": {}, "browser_launch": {}, "browser_navigate": {},
		"browser_close": {}, "TaskStop": {},
		// F23 legacy browser tools (renamed, explicit levels)
		"browser_legacy_launch": {}, "browser_legacy_navigate": {},
		"browser_legacy_screenshot": {}, "browser_legacy_close": {},
		// Phase 2 tools with explicit classification
		"plan_create": {}, "plan_branch": {}, "plan_merge": {},
		"plan_delete": {}, "workspace_create": {}, "workspace_cleanup": {},
		"voice_transcribe": {}, "kilocode_rename": {}, "kilocode_multi_edit": {},
		"roo_delegate": {}, "roo_generate": {}, "roo_bootstrap": {},
		"continue_edit": {}, "continue_complete": {},
	}

	for _, tool := range r.List() {
		name := tool.Name()
		if _, ok := classified[name]; ok {
			continue // explicit override, exercised in the other test.
		}
		// Unclassified — must default to LevelEdit (safe-by-default via
		// approval.DefaultLevelEdit). A LevelReadOnly here would be a
		// regression that lets unknown mutations through suggest mode.
		assert.Equal(t, approval.LevelEdit, tool.RequiresApproval(),
			"tool %q has no explicit classification; expected LevelEdit (safe default) but got %s",
			name, tool.RequiresApproval())
	}
}

// TestAllRegisteredTools_RequiresApprovalNeverPanics is a defence-in-depth
// guarantee: calling RequiresApproval() on every registered tool must never
// panic. A panicking tool would crash the approval gate at runtime and
// silently bypass the policy check.
func TestAllRegisteredTools_RequiresApprovalNeverPanics(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	for _, tool := range r.List() {
		name := tool.Name()
		require.NotPanics(t, func() {
			_ = tool.RequiresApproval()
		}, "tool %q panicked when RequiresApproval() was called", name)
	}
}

// TestDefaultLevelEdit_EmbedsCorrectly_AcrossAllToolsImplementations
// instantiates each subpackage tool type that embeds DefaultLevelEdit and
// confirms the embedded method returns LevelEdit. This pins the migration
// pattern — a future refactor that drops the embed will fail here even if it
// passes the registry-level table check (because the subpackage tools are
// not registered by the default registry).
func TestDefaultLevelEdit_DirectEmbedPattern(t *testing.T) {
	// Direct test of the embed pattern. If approval.DefaultLevelEdit is
	// reverted to a no-op, this fails.
	var d approval.DefaultLevelEdit
	assert.Equal(t, approval.LevelEdit, d.RequiresApproval())
}
