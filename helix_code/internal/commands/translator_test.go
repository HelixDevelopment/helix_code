// Unit tests for the internal/commands package-level translator + tr()
// helper (CONST-046 round-149 §11.4 anti-bluff sweep, 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	commandsi18n "dev.helix.code/internal/commands/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

// interpolatingTranslator renders a tiny set of known message IDs
// with their templateData. Used by call-site tests that need the
// *rendered* output (with real data interpolated) rather than the
// sentinel-wrapped message ID — e.g. asserting a hook ID surfaces in
// a /hooks test result line.
type interpolatingTranslator struct{}

func (interpolatingTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	switch id {
	case "internal_commands_hooks_test_result":
		return fmt.Sprintf("%v: status=%v err=%v duration=%v",
			data["HookID"], data["Status"], data["Error"], data["Duration"]), nil
	// --- round-432: /permissions + /sessions migrated literals ---
	case "internal_commands_permissions_mode_set":
		return fmt.Sprintf("session permission mode set to %v\n", data["Mode"]), nil
	case "internal_commands_sessions_col_id":
		return "ID", nil
	case "internal_commands_sessions_col_project":
		return "PROJECT", nil
	case "internal_commands_sessions_col_started":
		return "STARTED", nil
	case "internal_commands_sessions_col_last_activity":
		return "LAST-ACTIVITY", nil
	case "internal_commands_sessions_col_msg_count":
		return "MSG-COUNT", nil
	case "internal_commands_sessions_show_session":
		return fmt.Sprintf("Session: %v", data["ID"]), nil
	case "internal_commands_sessions_show_project":
		return fmt.Sprintf("Project: %v (%v)", data["Name"], data["Path"]), nil
	case "internal_commands_sessions_show_started":
		return fmt.Sprintf("Started: %v", data["Started"]), nil
	case "internal_commands_sessions_show_last_activity":
		return fmt.Sprintf("Last activity: %v", data["LastActivity"]), nil
	case "internal_commands_sessions_show_messages":
		return fmt.Sprintf("Messages: %v", data["Count"]), nil
	case "internal_commands_sessions_show_transcript_header":
		return "--- Transcript (last 20) ---", nil
	case "internal_commands_sessions_deleted":
		return fmt.Sprintf("deleted session %v", data["ID"]), nil
	default:
		return id, nil
	}
}
func (interpolatingTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}

type errTranslator struct{}

func (errTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(context.Background(), "internal_commands_user_id_is_required", nil)
	if got != "internal_commands_user_id_is_required" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_commands_mcp_manager_not_initialised", nil)
	if got != "<TR:internal_commands_mcp_manager_not_initialised>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade to
	// the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_commands_tasks_manager_not_initialised", nil)
	if got != "internal_commands_tasks_manager_not_initialised" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_commands_usage_hooks_test", nil)
	if got != "internal_commands_usage_hooks_test" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

// TestValidateContext_UserIDMissing_GoesThroughTranslator is the
// call-site paired-mutation: with a sentinel translator wired, the
// migrated fmt.Errorf path MUST surface the sentinel-wrapped message
// ID — proving the literal was NOT hardcoded anywhere on the path.
// If a future refactor inlines the string, this test fails.
func TestValidateContext_UserIDMissing_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	exec := NewExecutor(NewRegistry())
	err := exec.ValidateContext(&CommandContext{}, []string{"user_id"})
	if err == nil {
		t.Fatal("ValidateContext(user_id=\"\") returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_commands_user_id_is_required>") {
		t.Fatalf("ValidateContext error = %q, want sentinel-wrapped message ID — string bypassed tr()", err.Error())
	}
}

// TestValidateContext_OtherFields_GoTroughTranslator table-tests the
// remaining three ValidateContext branches (session_id, project_id,
// working_dir) — same sentinel-wrapped paired-mutation assertion as
// the user_id case above, factored as a table to keep the file under
// the round LOC budget.
func TestValidateContext_OtherFields_GoTroughTranslator(t *testing.T) {
	cases := []struct {
		field, wantID string
	}{
		{"session_id", "<TR:internal_commands_session_id_is_required>"},
		{"project_id", "<TR:internal_commands_project_id_is_required>"},
		{"working_dir", "<TR:internal_commands_working_dir_is_required>"},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			resetTranslator(t)
			SetTranslator(sentinelTranslator{})
			defer resetTranslator(t)

			exec := NewExecutor(NewRegistry())
			err := exec.ValidateContext(&CommandContext{}, []string{tc.field})
			if err == nil {
				t.Fatalf("ValidateContext(%s=\"\") returned no error", tc.field)
			}
			if !strings.Contains(err.Error(), tc.wantID) {
				t.Fatalf("ValidateContext(%s) error = %q, want %q", tc.field, err.Error(), tc.wantID)
			}
		})
	}
}

// TestMCPCommand_NilManager_GoesThroughTranslator verifies the mcp
// manager-not-initialised error path uses the translator.
func TestMCPCommand_NilManager_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewMCPCommand(nil)
	_, err := cmd.Execute(context.Background(), &CommandContext{})
	if err == nil {
		t.Fatal("MCPCommand.Execute(nil manager) returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_commands_mcp_manager_not_initialised>") {
		t.Fatalf("MCPCommand error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestTasksCommand_NilManager_GoesThroughTranslator verifies the tasks
// manager-not-initialised error path uses the translator.
func TestTasksCommand_NilManager_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewTasksCommand(nil)
	_, err := cmd.Execute(context.Background(), &CommandContext{})
	if err == nil {
		t.Fatal("TasksCommand.Execute(nil manager) returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_commands_tasks_manager_not_initialised>") {
		t.Fatalf("TasksCommand error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestPermissionsCommand_ModeUsage_GoesThroughTranslator verifies the
// /permissions mode usage hint uses the translator.
func TestPermissionsCommand_ModeUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewPermissionsCommand()
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"mode"}})
	if err == nil {
		t.Fatal("PermissionsCommand mode (no preset) returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_commands_usage_permissions_mode>") {
		t.Fatalf("PermissionsCommand error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestPermissionsCommand_AddUsage_GoesThroughTranslator verifies the
// /permissions add usage hint uses the translator.
func TestPermissionsCommand_AddUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewPermissionsCommand()
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"add"}})
	if err == nil {
		t.Fatal("PermissionsCommand add (no args) returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_commands_usage_permissions_add>") {
		t.Fatalf("PermissionsCommand error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestSetTranslator_AcceptsNoopExplicit confirms the public API
// allows an explicit NoopTranslator (used by tests + ad-hoc tools)
// without unexpected behaviour.
func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(commandsi18n.NoopTranslator{})
	got := tr(context.Background(), "internal_commands_usage_permissions_remove", nil)
	if got != "internal_commands_usage_permissions_remove" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}
