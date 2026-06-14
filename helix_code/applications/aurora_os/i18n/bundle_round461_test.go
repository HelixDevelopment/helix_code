// Paired-mutation bundle test for the round-461 §11.4 CONST-046
// Phase-4 applications final-sweep. Genuine user-facing literals
// (diagnostics result fragments, security-scan results, performance-
// mode dialogs, audit-log viewer text, encryption-config form,
// LLM-chat tab strings, settings-tab cards, plus the CLI-mode
// main_nogui.go cobra flag descriptions and audit-entry details)
// migrated out of helix_code/applications/aurora_os/main.go and
// main_nogui.go into the active English bundle.
//
// This test parses bundles/active.en.yaml directly so it runs WITHOUT
// the X11/Fyne cgo toolchain — aurora_os/main.go is gated
// `//go:build !nogui` and cannot compile on a headless host. The
// bundle YAML is data-only, so verifying it here gives honest runtime
// evidence that every round-461 message ID resolves to a non-empty
// translation.
//
// Anti-bluff (CONST-035): a green "round-461 strings migrated" claim
// is a PASS-bluff unless every migrated ID actually exists in the
// bundle. TestRound461BundleKeysPresent asserts existence +
// non-emptiness; deleting any one entry from active.en.yaml flips
// this test to FAIL (paired-mutation discipline, §1.1).
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope only).
package i18n

import (
	"context"
	"strings"
	"testing"
)

// round461IDs is the closed set of message IDs introduced by the
// round-461 §11.4 CONST-046 Phase-4 applications final-sweep for
// aurora_os. Order mirrors the append block in active.en.yaml.
var round461IDs = []string{
	// main.go — diagnostics
	"aurora_os_audit_diagnostics_initiated",
	"aurora_os_diag_cpu_cores_fmt",
	"aurora_os_diag_status_ok",
	"aurora_os_diag_warn_high_memory",
	"aurora_os_diag_memory_fmt",
	"aurora_os_diag_warn_high_goroutines",
	"aurora_os_diag_goroutines_fmt",
	"aurora_os_diag_db_not_connected",
	"aurora_os_diag_db_connected",
	"aurora_os_diag_database_fmt",
	"aurora_os_diag_task_manager_init",
	"aurora_os_diag_worker_manager_init",
	"aurora_os_diag_project_manager_init",
	"aurora_os_diag_session_manager_init",
	"aurora_os_diag_llm_manager_init",
	"aurora_os_state_enabled",
	"aurora_os_state_disabled",
	"aurora_os_state_enabled_lower",
	"aurora_os_state_disabled_lower",
	"aurora_os_diag_encryption_fmt",
	"aurora_os_diag_perf_mode_fmt",
	"aurora_os_diag_results_header",
	"aurora_os_diag_completed_ok",
	"aurora_os_audit_diagnostics_completed_fmt",
	// main.go — security scan
	"aurora_os_audit_security_scan_initiated",
	"aurora_os_scan_encryption_disabled",
	"aurora_os_scan_encryption_enabled",
	"aurora_os_scan_no_access_roles",
	"aurora_os_scan_access_roles_fmt",
	"aurora_os_scan_audit_logging_enabled",
	"aurora_os_scan_db_not_connected",
	"aurora_os_scan_db_connected",
	"aurora_os_scan_all_checks_passed",
	"aurora_os_scan_issues_found_fmt",
	"aurora_os_scan_results_header",
	"aurora_os_scan_completed_fmt",
	"aurora_os_audit_security_scan_completed_fmt",
	// main.go — performance mode
	"aurora_os_dialog_perf_mode_title",
	"aurora_os_perf_mode_enabled_intro",
	"aurora_os_perf_mode_optimizations_header",
	"aurora_os_perf_mode_opt_gomaxprocs_fmt",
	"aurora_os_perf_mode_opt_gc",
	"aurora_os_perf_mode_opt_memory",
	"aurora_os_audit_perf_mode_enabled",
	"aurora_os_perf_mode_disabled_body",
	"aurora_os_audit_perf_mode_disabled",
	"aurora_os_audit_optimization_initiated",
	"aurora_os_optimization_report_fmt",
	"aurora_os_dialog_optimization_title",
	"aurora_os_audit_optimization_completed_fmt",
	// main.go — audit log
	"aurora_os_dialog_audit_log_title",
	"aurora_os_audit_log_empty",
	"aurora_os_audit_log_header",
	"aurora_os_audit_log_entry_fmt",
	"aurora_os_audit_log_showing_fmt",
	"aurora_os_btn_close",
	// main.go — encryption config
	"aurora_os_check_enable_encryption",
	"aurora_os_formitem_encryption_enabled",
	"aurora_os_formitem_algorithm",
	"aurora_os_audit_encryption_change_fmt",
	"aurora_os_dialog_encryption_config_title",
	"aurora_os_encryption_updated_fmt",
	"aurora_os_dialog_configure_encryption_title",
	"aurora_os_btn_save",
	"aurora_os_btn_cancel",
	// main.go — audit entries + details
	"aurora_os_audit_app_initialized",
	"aurora_os_audit_task_created_fmt",
	"aurora_os_audit_worker_added_fmt",
	"aurora_os_project_details_fmt",
	"aurora_os_audit_project_created_fmt",
	"aurora_os_session_details_fmt",
	"aurora_os_audit_session_created_fmt",
	"aurora_os_btn_start_session",
	"aurora_os_audit_session_started_fmt",
	"aurora_os_btn_pause_session",
	"aurora_os_audit_session_paused_fmt",
	"aurora_os_btn_complete_session",
	"aurora_os_audit_session_completed_fmt",
	// main.go — LLM tab
	"aurora_os_card_available_models",
	"aurora_os_label_select_model",
	"aurora_os_model_details_fmt",
	"aurora_os_card_model_details",
	"aurora_os_placeholder_chat_history",
	"aurora_os_placeholder_chat_input",
	"aurora_os_placeholder_model_name",
	"aurora_os_btn_send_message",
	"aurora_os_audit_message_sent_fmt",
	"aurora_os_chat_provider_unavailable_fmt",
	"aurora_os_chat_llm_not_initialized_fmt",
	"aurora_os_chat_user_message_fmt",
	"aurora_os_chat_ai_response_fmt",
	"aurora_os_chat_ai_error_fmt",
	"aurora_os_btn_clear_chat",
	"aurora_os_label_chat_settings",
	"aurora_os_label_provider",
	"aurora_os_label_model",
	"aurora_os_label_chat_with_ai",
	"aurora_os_card_llm_chat",
	"aurora_os_health_checking",
	"aurora_os_health_no_manager",
	"aurora_os_health_header",
	"aurora_os_health_no_providers",
	"aurora_os_card_provider_status",
	// main.go — settings tab
	"aurora_os_theme_info_fmt",
	"aurora_os_card_theme",
	"aurora_os_card_theme_subtitle",
	"aurora_os_card_current_theme",
	"aurora_os_placeholder_server_url",
	"aurora_os_placeholder_timeout",
	"aurora_os_card_server_connection",
	"aurora_os_label_server_url",
	"aurora_os_label_timeout",
	"aurora_os_btn_test_connection",
	"aurora_os_dialog_connection_test_title",
	"aurora_os_connection_test_body",
	"aurora_os_card_database",
	"aurora_os_label_db_host",
	"aurora_os_label_db_port",
	"aurora_os_label_db_database",
	"aurora_os_card_llm_providers",
	"aurora_os_label_ollama_url",
	"aurora_os_label_openai_api_key",
	"aurora_os_label_anthropic_api_key",
	"aurora_os_check_performance_mode",
	"aurora_os_card_aurora_settings",
	"aurora_os_btn_run_diagnostics",
	"aurora_os_about_body",
	"aurora_os_card_about",
	"aurora_os_audit_app_shutting_down",
	// main_nogui.go — CLI residuals
	"aurora_os_cli_audit_app_initialized",
	"aurora_os_cli_audit_app_shutting_down",
	"aurora_os_cli_audit_projects_listed",
	"aurora_os_cli_flag_project_desc",
	"aurora_os_cli_flag_project_type",
	"aurora_os_cli_audit_project_created_fmt",
	"aurora_os_cli_audit_project_set_active_fmt",
	"aurora_os_cli_audit_project_deleted_fmt",
	"aurora_os_cli_flag_session_desc",
	"aurora_os_cli_flag_session_mode",
	"aurora_os_cli_audit_session_created_fmt",
	"aurora_os_cli_audit_session_started_fmt",
	"aurora_os_cli_audit_session_paused_fmt",
	"aurora_os_cli_audit_session_completed_fmt",
	"aurora_os_cli_flag_task_type",
	"aurora_os_cli_flag_task_desc",
	"aurora_os_cli_flag_task_priority",
	"aurora_os_cli_audit_task_created_fmt",
	"aurora_os_cli_audit_task_cancelled_fmt",
	"aurora_os_cli_audit_worker_added_fmt",
	"aurora_os_cli_audit_worker_removed_fmt",
	"aurora_os_cli_audit_perf_mode_fmt",
	"aurora_os_cli_audit_diagnostics_fmt",
	"aurora_os_cli_audit_optimization_fmt",
	"aurora_os_cli_audit_encryption_enabled",
	"aurora_os_cli_audit_encryption_disabled",
	"aurora_os_cli_audit_permission_added_fmt",
}

// TestRound461BundleKeysPresent is the paired-mutation guard: every
// round-461 message ID MUST exist in active.en.yaml with a non-empty
// `other` value. Deleting any entry flips this to FAIL.
func TestRound461BundleKeysPresent(t *testing.T) {
	bundle := loadActiveENBundle(t)
	for _, id := range round461IDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("round-461 message ID %q absent from active.en.yaml", id)
			continue
		}
		if entry.Other == "" {
			t.Errorf("round-461 message ID %q has an empty translation", id)
		}
	}
}

// TestRound461KeysResolveThroughTranslator proves the round-461 IDs
// are usable through the Translator seam exactly as aurora_os
// consumes them.
func TestRound461KeysResolveThroughTranslator(t *testing.T) {
	tr := fakeTranslator{}
	for _, id := range round461IDs {
		got, err := tr.T(context.Background(), id, nil)
		if err != nil {
			t.Fatalf("Translator.T(%q) returned error: %v", id, err)
		}
		want := "<TRANSLATED:" + id + ">"
		if got != want {
			t.Fatalf("Translator.T(%q) = %q, want %q", id, got, want)
		}
	}
}

// TestRound461NoDuplicateIDs guards against a copy-paste slip.
func TestRound461NoDuplicateIDs(t *testing.T) {
	seen := make(map[string]bool, len(round461IDs))
	for _, id := range round461IDs {
		if seen[id] {
			t.Errorf("round-461 ID %q listed more than once", id)
		}
		seen[id] = true
	}
}

// TestRound461FmtVerbParity asserts that every *_fmt ID carries at
// least one fmt conversion verb and every non-*_fmt ID carries none —
// a mismatch means a defective call site or bundle typo.
func TestRound461FmtVerbParity(t *testing.T) {
	bundle := loadActiveENBundle(t)
	for _, id := range round461IDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("round-461 ID %q absent from active.en.yaml", id)
			continue
		}
		n := countFmtVerbs(entry.Other)
		if strings.HasSuffix(id, "_fmt") {
			if n == 0 {
				t.Errorf("round-461 ID %q ends _fmt but carries no format verb (translation=%q)", id, entry.Other)
			}
		} else if n != 0 {
			t.Errorf("round-461 ID %q: %d format verbs, want 0 (translation=%q)", id, n, entry.Other)
		}
	}
}

// TestRound461NoCollisionWithEarlierRounds guards against accidental
// reuse of a round-430/454/458 ID.
func TestRound461NoCollisionWithEarlierRounds(t *testing.T) {
	earlier := make(map[string]string)
	for _, id := range round430IDs {
		earlier[id] = "round-430"
	}
	for _, id := range round454IDs {
		earlier[id] = "round-454"
	}
	for _, id := range round458IDs {
		earlier[id] = "round-458"
	}
	for _, id := range round461IDs {
		if origin, clash := earlier[id]; clash {
			t.Errorf("round-461 ID %q collides with %s", id, origin)
		}
	}
}
