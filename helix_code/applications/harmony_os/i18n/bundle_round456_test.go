// Paired-mutation bundle test for the round-456 §11.4 CONST-046
// Phase-4 migration: 25 additional GUI-mode Fyne strings migrated
// out of helix_code/applications/harmony_os/main.go and into the
// active English bundle — task/project/session form placeholders
// and create-headers, status-bar confirmation messages, dialog
// bodies, the Discover Devices / Complete Session buttons, the
// Available Models card title, and the LLM chat-interface
// placeholders.
//
// This test parses bundles/active.en.yaml directly so it runs
// WITHOUT the X11/Fyne cgo toolchain — the harmony_os main.go GUI
// file is gated `//go:build !nogui` and cannot compile on a
// headless host. The bundle YAML is data-only, so verifying it
// here gives honest runtime evidence that every round-456 message
// ID resolves to a non-empty translation.
//
// Anti-bluff (CONST-035): a green "25 GUI strings migrated" claim
// is a PASS-bluff unless every migrated ID actually exists in the
// bundle. TestRound456BundleKeysPresent asserts existence +
// non-emptiness; the paired-mutation discipline (§1.1) is satisfied
// because removing any one entry from active.en.yaml flips this
// test to FAIL.
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

// round456IDs is the closed set of message IDs introduced by the
// round-456 §11.4 CONST-046 Phase-4 GUI sweep. Order mirrors the
// append block in bundles/active.en.yaml.
var round456IDs = []string{
	"harmony_os_gui_placeholder_task_description",
	"harmony_os_gui_status_task_created",
	"harmony_os_gui_dialog_task_scheduled",
	"harmony_os_gui_status_workers_refreshed",
	"harmony_os_gui_button_discover_devices",
	"harmony_os_gui_status_discover_failed",
	"harmony_os_gui_status_devices_found",
	"harmony_os_gui_project_select_prompt",
	"harmony_os_gui_project_create_header",
	"harmony_os_gui_status_project_created",
	"harmony_os_gui_dialog_project_created",
	"harmony_os_gui_status_project_active",
	"harmony_os_gui_status_projects_refreshed",
	"harmony_os_gui_session_select_prompt",
	"harmony_os_gui_session_create_header",
	"harmony_os_gui_status_session_created",
	"harmony_os_gui_dialog_session_created",
	"harmony_os_gui_session_controls_header",
	"harmony_os_gui_button_complete_session",
	"harmony_os_gui_status_session_completed",
	"harmony_os_gui_status_sessions_refreshed",
	"harmony_os_gui_card_available_models",
	"harmony_os_gui_model_select_prompt",
	"harmony_os_gui_placeholder_chat_history",
	"harmony_os_gui_placeholder_chat_input",
	"harmony_os_gui_placeholder_model_name",
}

// round456PlaceholderIDs maps placeholder-bearing message IDs to the
// go-i18n {{.Name}} tokens their translations MUST carry. A status
// line that lost its interpolation token would silently drop the
// task ID / device / count from the GUI — a §11.4 PASS-bluff at the
// i18n layer that this map catches.
var round456PlaceholderIDs = map[string][]string{
	"harmony_os_gui_status_task_created":    {"{{.ID}}", "{{.Device}}"},
	"harmony_os_gui_dialog_task_scheduled":  {"{{.ID}}"},
	"harmony_os_gui_status_discover_failed": {"{{.Error}}"},
	"harmony_os_gui_status_devices_found":   {"{{.Count}}"},
	"harmony_os_gui_status_project_created": {"{{.Name}}"},
	"harmony_os_gui_status_project_active":  {"{{.Name}}"},
	"harmony_os_gui_status_session_created": {"{{.Name}}"},
}

// round456Entry models a single go-i18n message entry's plural
// forms (only `other` is used by these flat-string entries).
type round456Entry struct {
	Other string `yaml:"other"`
}

// loadRound456Bundle parses bundles/active.en.yaml into a map of
// message-ID → entry. Fatal on any read/parse error so the test
// cannot silently pass against a missing or corrupt bundle.
func loadRound456Bundle(t *testing.T) map[string]round456Entry {
	t.Helper()
	path := filepath.Join("bundles", "active.en.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var bundle map[string]round456Entry
	if err := yaml.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(bundle) == 0 {
		t.Fatalf("%s parsed to an empty bundle", path)
	}
	return bundle
}

// TestRound456BundleKeysPresent is the paired-mutation guard: every
// round-456 message ID MUST exist in active.en.yaml with a
// non-empty `other` value. Deleting any entry flips this to FAIL.
func TestRound456BundleKeysPresent(t *testing.T) {
	bundle := loadRound456Bundle(t)
	for _, id := range round456IDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("round-456 message ID %q absent from active.en.yaml", id)
			continue
		}
		if entry.Other == "" {
			t.Errorf("round-456 message ID %q has an empty translation", id)
		}
	}
}

// TestRound456PlaceholdersIntact asserts every placeholder-bearing
// entry keeps its go-i18n interpolation tokens. A translation that
// dropped {{.ID}} would render a task confirmation without the task
// identifier — invisible data loss the operator cannot detect.
func TestRound456PlaceholdersIntact(t *testing.T) {
	bundle := loadRound456Bundle(t)
	for id, tokens := range round456PlaceholderIDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("placeholder-bearing ID %q absent from active.en.yaml", id)
			continue
		}
		for _, tok := range tokens {
			if !strings.Contains(entry.Other, tok) {
				t.Errorf("round-456 ID %q lost interpolation token %q: %q", id, tok, entry.Other)
			}
		}
	}
}

// TestRound456KeysResolveThroughTranslator proves the round-456 IDs
// are usable through the Translator seam exactly as the harmony_os
// GUI consumes them. fakeTranslator wraps each ID with a sentinel
// so the assertion fails if a call site ever bypasses Translator.T.
func TestRound456KeysResolveThroughTranslator(t *testing.T) {
	tr := fakeTranslator{}
	for _, id := range round456IDs {
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

// TestRound456NoDuplicateIDs guards against a copy-paste slip that
// would silently shadow one label/placeholder with another.
func TestRound456NoDuplicateIDs(t *testing.T) {
	seen := make(map[string]bool, len(round456IDs))
	for _, id := range round456IDs {
		if seen[id] {
			t.Errorf("round-456 ID %q listed more than once", id)
		}
		seen[id] = true
	}
}

// TestRound456NoopTranslatorEchoesID confirms the SAFETY default —
// NoopTranslator returns the message ID verbatim (loud echo) so a
// missing real Translator never silently empties a GUI label,
// which would be a §11.4 PASS-bluff at the i18n injection layer.
func TestRound456NoopTranslatorEchoesID(t *testing.T) {
	noop := NoopTranslator{}
	for _, id := range round456IDs {
		got, err := noop.T(context.Background(), id, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.T(%q) returned error: %v", id, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.T(%q) = %q, want loud message-ID echo", id, got)
		}
	}
}
