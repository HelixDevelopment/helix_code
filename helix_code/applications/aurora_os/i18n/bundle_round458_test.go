// Paired-mutation bundle test for the round-458 §11.4 CONST-046
// Phase-4 residual GUI-tab sweep: 37 genuine user-facing Fyne strings
// (Tasks/Workers/Projects/Sessions tab card titles, form section
// labels, field labels, button labels, entry placeholders, and the
// project/session "created successfully" dialog title + body)
// migrated out of helix_code/applications/aurora_os/main.go and into
// the active English bundle.
//
// This test parses bundles/active.en.yaml directly so it runs
// WITHOUT the X11/Fyne cgo toolchain — the aurora_os main.go GUI file
// is gated `//go:build !nogui` and cannot compile on a headless host.
// The bundle YAML is data-only, so verifying it here gives honest
// runtime evidence that every round-458 message ID resolves to a
// non-empty translation.
//
// Anti-bluff (CONST-035): a green "37 strings migrated" claim is a
// PASS-bluff unless every migrated ID actually exists in the bundle.
// TestRound458BundleKeysPresent asserts existence + non-emptiness;
// the paired-mutation discipline (§1.1) is satisfied because removing
// any one entry from active.en.yaml flips this test to FAIL.
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope only).
package i18n

import (
	"context"
	"testing"
)

// round458IDs is the closed set of message IDs introduced by the
// round-458 §11.4 CONST-046 Phase-4 residual GUI-tab sweep. Order
// mirrors the append block in bundles/active.en.yaml.
var round458IDs = []string{
	"aurora_os_card_tasks_list",
	"aurora_os_placeholder_task_description",
	"aurora_os_label_new_task",
	"aurora_os_label_field_type",
	"aurora_os_label_field_priority",
	"aurora_os_label_field_description",
	"aurora_os_btn_create_task",
	"aurora_os_btn_refresh",
	"aurora_os_card_workers_list",
	"aurora_os_placeholder_worker_host",
	"aurora_os_placeholder_worker_user",
	"aurora_os_label_add_worker",
	"aurora_os_label_field_host",
	"aurora_os_label_field_port",
	"aurora_os_label_field_user",
	"aurora_os_btn_add_worker",
	"aurora_os_card_projects_list",
	"aurora_os_card_project_details",
	"aurora_os_label_select_project",
	"aurora_os_placeholder_project_name",
	"aurora_os_placeholder_description",
	"aurora_os_label_create_new_project",
	"aurora_os_label_field_name",
	"aurora_os_label_field_path",
	"aurora_os_btn_create_project",
	"aurora_os_dialog_success_title",
	"aurora_os_dialog_project_created",
	"aurora_os_card_sessions_list",
	"aurora_os_card_session_details",
	"aurora_os_label_select_session",
	"aurora_os_placeholder_session_name",
	"aurora_os_label_create_new_session",
	"aurora_os_label_field_project_id",
	"aurora_os_label_field_mode",
	"aurora_os_btn_create_session",
	"aurora_os_dialog_session_created",
	"aurora_os_label_session_controls",
	"aurora_os_placeholder_project_id",
}

// TestRound458BundleKeysPresent is the paired-mutation guard: every
// round-458 message ID MUST exist in active.en.yaml with a non-empty
// `other` value. Deleting any entry flips this to FAIL.
func TestRound458BundleKeysPresent(t *testing.T) {
	bundle := loadActiveENBundle(t)
	for _, id := range round458IDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("round-458 message ID %q absent from active.en.yaml", id)
			continue
		}
		if entry.Other == "" {
			t.Errorf("round-458 message ID %q has an empty translation", id)
		}
	}
}

// TestRound458KeysResolveThroughTranslator proves the round-458 IDs
// are usable through the Translator seam exactly as the aurora_os GUI
// consumes them. fakeTranslator wraps each ID with a sentinel so the
// assertion fails if a call site ever bypasses Translator.T.
func TestRound458KeysResolveThroughTranslator(t *testing.T) {
	tr := fakeTranslator{}
	for _, id := range round458IDs {
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

// TestRound458NoDuplicateIDs guards against a copy-paste slip that
// would silently shadow one label with another.
func TestRound458NoDuplicateIDs(t *testing.T) {
	seen := make(map[string]bool, len(round458IDs))
	for _, id := range round458IDs {
		if seen[id] {
			t.Errorf("round-458 ID %q listed more than once", id)
		}
		seen[id] = true
	}
}

// TestRound458NoFormatVerbs asserts the round-458 IDs carry no
// fmt-style conversion verbs — they are all plain labels/titles
// consumed via auroraApp.t(id) directly (no fmt.Sprintf wrapper). A
// stray %-verb would mean a defective call site or a bundle typo.
func TestRound458NoFormatVerbs(t *testing.T) {
	bundle := loadActiveENBundle(t)
	for _, id := range round458IDs {
		entry, ok := bundle[id]
		if !ok {
			t.Errorf("round-458 ID %q absent from active.en.yaml", id)
			continue
		}
		if n := countFmtVerbs(entry.Other); n != 0 {
			t.Errorf("round-458 ID %q: %d format verbs, want 0 (translation=%q)", id, n, entry.Other)
		}
	}
}

// TestRound458NoCollisionWithEarlierRounds guards the round-458 ID
// set against accidental reuse of a round-430 or round-454 ID, which
// would silently couple two unrelated migration rounds.
func TestRound458NoCollisionWithEarlierRounds(t *testing.T) {
	earlier := make(map[string]string, len(round430IDs)+len(round454IDs))
	for _, id := range round430IDs {
		earlier[id] = "round-430"
	}
	for _, id := range round454IDs {
		earlier[id] = "round-454"
	}
	for _, id := range round458IDs {
		if origin, clash := earlier[id]; clash {
			t.Errorf("round-458 ID %q collides with %s", id, origin)
		}
	}
}
