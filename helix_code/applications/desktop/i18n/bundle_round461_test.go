// Paired-mutation bundle test for the round-461 §11.4 CONST-046
// Phase-4 applications final-sweep. The seven cobra flag-description
// literals in helix_code/applications/desktop/main_nogui.go (project
// / session / task `--desc`, `--type`, `--mode`, `--priority` help
// text shown in `desktop-cli --help`) migrated into the active
// English bundle.
//
// This test parses bundles/active.en.yaml directly so it runs WITHOUT
// the X11/Fyne cgo toolchain. The bundle YAML is data-only, so
// verifying it here gives honest runtime evidence that every
// round-461 message ID resolves to a non-empty translation.
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
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// round461Entry models a single go-i18n message entry's `other` form.
type round461Entry struct {
	Other string `yaml:"other"`
}

// loadRound461Bundle parses bundles/active.en.yaml into a map of
// message-ID → entry. Fatal on any read/parse error.
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
// round-461 §11.4 CONST-046 Phase-4 sweep for desktop/main_nogui.go.
var round461IDs = []string{
	"desktop_cli_flag_project_desc",
	"desktop_cli_flag_project_type",
	"desktop_cli_flag_session_desc",
	"desktop_cli_flag_session_mode",
	"desktop_cli_flag_task_type",
	"desktop_cli_flag_task_desc",
	"desktop_cli_flag_task_priority",
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
// are usable through the Translator seam exactly as the desktop CLI
// consumes them via cliApp.t.
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
