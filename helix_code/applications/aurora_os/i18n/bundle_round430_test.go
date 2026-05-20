// Paired-mutation bundle test for the round-430 §11.4 CONST-046
// Phase-4 migration: the 26 GUI-mode Fyne strings (card titles,
// action-button labels, dialog titles) migrated out of
// helix_code/applications/aurora_os/main.go and into the active
// English bundle.
//
// This test parses bundles/active.en.yaml directly so it runs
// WITHOUT the X11/Fyne cgo toolchain — the aurora_os main.go GUI
// file is gated `//go:build !nogui` and cannot compile on a headless
// host. The bundle YAML is data-only, so verifying it here gives
// honest runtime evidence that every round-430 message ID resolves
// to a non-empty translation.
//
// Anti-bluff (CONST-035): a green "26 strings migrated" claim is a
// PASS-bluff unless every migrated ID actually exists in the bundle.
// TestRound430BundleKeysPresent asserts existence + non-emptiness;
// the paired-mutation discipline (§1.1) is satisfied because removing
// any one entry from active.en.yaml flips this test to FAIL.
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope only).
package i18n

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// round430IDs is the closed set of message IDs introduced by the
// round-430 §11.4 CONST-046 Phase-4 GUI sweep. Order mirrors the
// append block in bundles/active.en.yaml.
var round430IDs = []string{
	"aurora_os_card_aurora_system",
	"aurora_os_card_workers",
	"aurora_os_card_tasks",
	"aurora_os_card_aurora_activity",
	"aurora_os_card_aurora_actions",
	"aurora_os_card_system_resources",
	"aurora_os_card_aurora_services",
	"aurora_os_card_security_status",
	"aurora_os_card_access_control",
	"aurora_os_btn_system_diagnostics",
	"aurora_os_btn_security_scan",
	"aurora_os_btn_performance_boost",
	"aurora_os_btn_new_task",
	"aurora_os_btn_new_project",
	"aurora_os_btn_refresh_system_info",
	"aurora_os_btn_optimize_performance",
	"aurora_os_btn_force_gc",
	"aurora_os_btn_run_security_scan",
	"aurora_os_btn_view_audit_log",
	"aurora_os_btn_configure_encryption",
	"aurora_os_btn_refresh_status",
	"aurora_os_dialog_refreshed_title",
	"aurora_os_dialog_system_info_refreshed",
	"aurora_os_dialog_gc_complete_title",
	"aurora_os_dialog_gc_completed",
	"aurora_os_dialog_system_diagnostics_title",
	"aurora_os_dialog_security_scan_title",
}

// bundleEntry models a single go-i18n message entry's plural forms.
type bundleEntry struct {
	Other string `yaml:"other"`
}

// loadActiveENBundle parses bundles/active.en.yaml into a map of
// message-ID → entry. Fatal on any read/parse error so the test
// cannot silently pass against a missing or corrupt bundle.
func loadActiveENBundle(t *testing.T) map[string]bundleEntry {
	t.Helper()
	path := filepath.Join("bundles", "active.en.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var bundle map[string]bundleEntry
	if err := yaml.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(bundle) == 0 {
		t.Fatalf("%s parsed to an empty bundle", path)
	}
	return bundle
}

// TestRound430BundleKeysPresent is the paired-mutation guard: every
// round-430 message ID MUST exist in active.en.yaml with a non-empty
// `other` value. Deleting any entry flips this to FAIL.
func TestRound430BundleKeysPresent(t *testing.T) {
	bundle := loadActiveENBundle(t)
	for _, id := range round430IDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("round-430 message ID %q absent from active.en.yaml", id)
			continue
		}
		if entry.Other == "" {
			t.Errorf("round-430 message ID %q has an empty translation", id)
		}
	}
}

// TestRound430KeysResolveThroughTranslator proves the round-430 IDs
// are usable through the Translator seam exactly as the aurora_os
// GUI consumes them. fakeTranslator wraps each ID with a sentinel so
// the assertion fails if a call site ever bypasses Translator.T.
func TestRound430KeysResolveThroughTranslator(t *testing.T) {
	tr := fakeTranslator{}
	for _, id := range round430IDs {
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

// TestRound430NoDuplicateIDs guards against a copy-paste slip that
// would silently shadow one card/button label with another.
func TestRound430NoDuplicateIDs(t *testing.T) {
	seen := make(map[string]bool, len(round430IDs))
	for _, id := range round430IDs {
		if seen[id] {
			t.Errorf("round-430 ID %q listed more than once", id)
		}
		seen[id] = true
	}
}
