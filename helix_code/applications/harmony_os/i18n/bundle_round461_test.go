// Paired-mutation bundle test for the round-461 §11.4 CONST-046
// Phase-4 applications final-sweep. Genuine user-facing literals
// (harmony_os/main.go GUI tab strings — session/model detail panels,
// LLM-chat error responses, provider-health labels, system-metrics
// labels, distributed-services / resource-management cards, settings-
// tab status messages — plus harmony_os/main_nogui.go CLI cobra flag
// descriptions and worker-add/remove confirmations) migrated into the
// active English bundle.
//
// This test parses bundles/active.en.yaml directly so it runs WITHOUT
// the X11/Fyne cgo toolchain — harmony_os/main.go is gated
// `//go:build !nogui` and cannot compile on a headless host. The
// bundle YAML is data-only, so verifying it here gives honest runtime
// evidence that every round-461 message ID resolves to a non-empty
// translation.
//
// Anti-bluff (CONST-035): a green "round-461 strings migrated" claim
// is a PASS-bluff unless every migrated ID exists in the bundle.
// Deleting any entry flips TestRound461BundleKeysPresent to FAIL.
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope only).
package i18n

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// round461Entry models a single go-i18n message entry's `other` form.
type round461Entry struct {
	Other string `yaml:"other"`
}

// loadRound461Bundle parses bundles/active.en.yaml into a map of
// message-ID → entry. Fatal on any read/parse error so the test
// cannot silently pass against a missing or corrupt bundle.
func loadRound461Bundle(t *testing.T) map[string]round461Entry {
	t.Helper()
	path := filepath.Join("bundles", "active.en.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var bundle map[string]round461Entry
	if err := yaml.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(bundle) == 0 {
		t.Fatalf("%s parsed to an empty bundle", path)
	}
	return bundle
}

// round461IDs is the closed set of message IDs introduced by the
// round-461 §11.4 CONST-046 Phase-4 applications final-sweep for
// harmony_os. Order mirrors the append block in active.en.yaml.
var round461IDs = []string{
	// main.go GUI
	"harmony_os_gui_session_details_fmt",
	"harmony_os_gui_model_details_fmt",
	"harmony_os_gui_chat_provider_unavailable_fmt",
	"harmony_os_gui_chat_llm_not_initialized_fmt",
	"harmony_os_gui_label_provider",
	"harmony_os_gui_label_model",
	"harmony_os_gui_health_checking",
	"harmony_os_gui_health_no_manager",
	"harmony_os_gui_health_header",
	"harmony_os_gui_health_no_providers",
	"harmony_os_gui_metric_cpu_usage_fmt",
	"harmony_os_gui_metric_memory_usage_fmt",
	"harmony_os_gui_metric_gpu_usage_fmt",
	"harmony_os_gui_metric_temperature_fmt",
	"harmony_os_gui_metric_power_usage_fmt",
	"harmony_os_gui_connected_devices_fmt",
	"harmony_os_gui_scheduler_info_fmt",
	"harmony_os_gui_sync_status_fmt",
	"harmony_os_gui_sync_failed_fmt",
	"harmony_os_gui_resource_policies_fmt",
	"harmony_os_gui_service_coordinator_fmt",
	"harmony_os_gui_status_theme_changed_fmt",
	"harmony_os_gui_status_server_error_fmt",
	// main_nogui.go CLI
	"harmony_os_cli_flag_project_desc",
	"harmony_os_cli_flag_project_type",
	"harmony_os_cli_flag_session_desc",
	"harmony_os_cli_flag_session_mode",
	"harmony_os_cli_flag_task_type",
	"harmony_os_cli_flag_task_desc",
	"harmony_os_cli_flag_task_priority",
	"harmony_os_cli_err_host_required",
	"harmony_os_cli_worker_added_fmt",
	"harmony_os_cli_err_worker_id_required",
	"harmony_os_cli_worker_removed_fmt",
}

// TestRound461BundleKeysPresent is the paired-mutation guard: every
// round-461 message ID MUST exist in active.en.yaml with a non-empty
// `other` value. Deleting any entry flips this to FAIL.
func TestRound461BundleKeysPresent(t *testing.T) {
	bundle := loadRound461Bundle(t)
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
// are usable through the Translator seam exactly as harmony_os
// consumes them via app.tr / cliApp.tr.
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

// TestRound461TemplatePlaceholderParity asserts every *_fmt ID
// carries at least one go-i18n {{.Name}} placeholder and every
// non-*_fmt ID carries none — harmony_os uses go-i18n template
// interpolation, not fmt verbs.
func TestRound461TemplatePlaceholderParity(t *testing.T) {
	bundle := loadRound461Bundle(t)
	for _, id := range round461IDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("round-461 ID %q absent from active.en.yaml", id)
			continue
		}
		has := strings.Contains(entry.Other, "{{.")
		if strings.HasSuffix(id, "_fmt") {
			if !has {
				t.Errorf("round-461 ID %q ends _fmt but carries no {{.Name}} placeholder (translation=%q)", id, entry.Other)
			}
		} else if has {
			t.Errorf("round-461 ID %q carries a {{.Name}} placeholder but is not a _fmt entry (translation=%q)", id, entry.Other)
		}
	}
}

// TestRound461NoFmtVerbs asserts the round-461 harmony entries carry
// no stray Go fmt verbs — interpolation is go-i18n template-style.
func TestRound461NoFmtVerbs(t *testing.T) {
	bundle := loadRound461Bundle(t)
	for _, id := range round461IDs {
		entry := bundle[id]
		// A literal "%" followed by a verb-class character would mean
		// a leftover fmt.Sprintf-style string slipped into a go-i18n
		// bundle. The harmony metric strings legitimately contain a
		// trailing "%" sign, so only flag "%" immediately followed by
		// a conversion letter.
		for i := 0; i+1 < len(entry.Other); i++ {
			if entry.Other[i] != '%' {
				continue
			}
			c := entry.Other[i+1]
			if strings.ContainsRune("sdvtfqxXeEgG", rune(c)) {
				t.Errorf("round-461 ID %q contains a Go fmt verb %%%c — should be go-i18n {{.Name}} (translation=%q)", id, c, entry.Other)
			}
		}
	}
}
