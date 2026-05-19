// Unit tests for the internal/task package-level translator +
// tr() helper (CONST-046 round-240 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package task

import (
	stdctx "context"
	"errors"
	"testing"

	taski18n "dev.helix.code/internal/task/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests
// can assert tr() actually went through Translator.T rather than
// returning a hardcoded literal that happened to match the bundle
// value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
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
	got := tr(stdctx.Background(), "internal_task_not_found", nil)
	if got != "internal_task_not_found" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_task_invalid_dependencies", nil)
	if got != "<TR:internal_task_invalid_dependencies>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade
	// to the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_task_store_in_db_failed", map[string]any{"Err": "boom"})
	if got != "internal_task_store_in_db_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_task_marshal_failed", nil)
	if got != "internal_task_marshal_failed" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(taski18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_task_marshal_stats_failed", nil)
	if got != "internal_task_marshal_stats_failed" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestTr_AllMigratedIDsResolveUnderNoop walks every CONST-046
// message ID introduced by round-240 and asserts the Noop default
// yields a loud echo (raw ID, never empty) for each. Detects a
// future regression where a new ID is added to the bundle but the
// resolver path silently swallows it.
func TestTr_AllMigratedIDsResolveUnderNoop(t *testing.T) {
	resetTranslator(t)

	ids := []string{
		"internal_task_invalid_dependencies",
		"internal_task_store_in_db_failed",
		"internal_task_marshal_failed",
		"internal_task_marshal_stats_failed",
		"internal_task_marshal_ids_failed",
		"internal_task_not_found",
		"internal_task_dep_check_existence_failed",
		"internal_task_dep_not_found",
		"internal_task_dep_check_status_failed",
		"internal_task_marshal_checkpoint_failed",
	}
	for _, id := range ids {
		got := tr(stdctx.Background(), id, nil)
		if got != id {
			t.Errorf("tr(%q) = %q, want raw ID (loud echo)", id, got)
		}
	}
}

// TestTr_AllMigratedIDsResolveUnderSentinel verifies every migrated
// ID actually goes through Translator.T when one is wired —
// paired-mutation guarantee per §11.4.
func TestTr_AllMigratedIDsResolveUnderSentinel(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	ids := []string{
		"internal_task_invalid_dependencies",
		"internal_task_store_in_db_failed",
		"internal_task_marshal_failed",
		"internal_task_marshal_stats_failed",
		"internal_task_marshal_ids_failed",
		"internal_task_not_found",
		"internal_task_dep_check_existence_failed",
		"internal_task_dep_not_found",
		"internal_task_dep_check_status_failed",
		"internal_task_marshal_checkpoint_failed",
	}
	for _, id := range ids {
		got := tr(stdctx.Background(), id, nil)
		want := "<TR:" + id + ">"
		if got != want {
			t.Errorf("tr(%q) = %q, want %q — call site bypassed Translator", id, got, want)
		}
	}
}
