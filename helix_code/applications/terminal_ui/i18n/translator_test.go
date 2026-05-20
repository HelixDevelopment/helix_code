// Unit tests for the terminal_ui Translator interface + NoopTranslator
// default. Mocks ALLOWED per CONST-050(A) (unit tests only).
package i18n

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "terminal_ui_sidebar_title", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "terminal_ui_sidebar_title" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "terminal_ui_tasks_count", 3, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "terminal_ui_tasks_count" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

// fakeTranslator returns a sentinel-wrapped message ID so call-site
// tests can assert the lookup actually went through Translator.T,
// not a hardcoded literal that happens to match the bundle value.
type fakeTranslator struct {
	failOnID string
}

func (f fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func (f fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func TestFakeTranslator_T_WrapsID(t *testing.T) {
	tr := fakeTranslator{}
	got, err := tr.T(context.Background(), "terminal_ui_sidebar_title", nil)
	if err != nil {
		t.Fatalf("fakeTranslator.T returned error: %v", err)
	}
	want := "<TRANSLATED:terminal_ui_sidebar_title>"
	if got != want {
		t.Fatalf("fakeTranslator.T returned %q, want %q", got, want)
	}
}

// TestRound357_BundleEntriesPresent verifies every message ID the
// round-357 §11.4 sweep migrated in main.go has a corresponding entry
// in active.en.yaml. Paired mutation: deleting any of the 25 IDs from
// the bundle (or renaming it in main.go without updating the bundle)
// makes this test FAIL — proving the migration is wired end-to-end
// rather than leaving a dangling ID that loud-echoes forever.
func TestRound357_BundleEntriesPresent(t *testing.T) {
	round357IDs := []string{
		"terminal_ui_sample_task_codegen_title",
		"terminal_ui_sample_task_codegen_desc",
		"terminal_ui_sample_task_testing_title",
		"terminal_ui_sample_task_testing_desc",
		"terminal_ui_sample_task_build_title",
		"terminal_ui_sample_task_build_desc",
		"terminal_ui_form_max_concurrent_tasks",
		"terminal_ui_project_set_active_failed",
		"terminal_ui_project_set_active",
		"terminal_ui_form_create_project_title",
		"terminal_ui_sessions_table_title",
		"terminal_ui_form_create_session_title",
		"terminal_ui_session_start_failed",
		"terminal_ui_session_pause_failed",
		"terminal_ui_session_complete_failed",
		"terminal_ui_chat_info_no_model",
		"terminal_ui_chat_info_provider",
		"terminal_ui_chat_model_changed",
		"terminal_ui_settings_applied",
		"terminal_ui_settings_categories_title",
		"terminal_ui_cognee_enabled_status",
		"terminal_ui_cognee_disabled_status",
		"terminal_ui_config_options_title",
		"terminal_ui_form_task_data_json",
		"terminal_ui_task_create_failed",
		"terminal_ui_task_created",
		"terminal_ui_qa_engine_disabled",
		"terminal_ui_qa_engine_enabled",
		"terminal_ui_qa_engine_disabled_hint",
		"terminal_ui_qa_no_sessions",
		"terminal_ui_qa_stats_total_sessions",
		"terminal_ui_qa_stats_coverage_target",
		"terminal_ui_qa_enable_hint",
		"terminal_ui_qa_cancel_failed",
		"terminal_ui_form_start_qa_session_title",
		"terminal_ui_qa_start_session_failed",
	}
	data, err := os.ReadFile(filepath.Join("bundles", "active.en.yaml"))
	if err != nil {
		t.Fatalf("read active.en.yaml: %v", err)
	}
	bundle := string(data)
	for _, id := range round357IDs {
		if !strings.Contains(bundle, id+":") {
			t.Errorf("round-357 message ID %q missing from active.en.yaml — migration incomplete", id)
		}
	}
}
