// Unit tests for the internal/database package-level translator + tr()
// helper (CONST-046 round-152 §11.4 anti-bluff sweep, 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package database

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	databasei18n "dev.helix.code/internal/database/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
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
	got := tr(stdctx.Background(), "internal_database_pool_not_initialized", nil)
	if got != "internal_database_pool_not_initialized" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_database_ping_failed", nil)
	if got != "<TR:internal_database_ping_failed>" {
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

	got := tr(stdctx.Background(), "internal_database_schema_create_failed", nil)
	if got != "internal_database_schema_create_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_database_pool_not_initialized", nil)
	if got != "internal_database_pool_not_initialized" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(databasei18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_database_config_parse_failed", nil)
	if got != "internal_database_config_parse_failed" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestGetDB_NotInitialized_GoesThroughTranslator covers the pre-init
// guard on GetDB. With a sentinel translator wired, the error MUST
// surface the sentinel-wrapped message ID — proving the literal was
// NOT hardcoded on the path.
func TestGetDB_NotInitialized_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	db := &Database{Pool: nil}
	_, err := db.GetDB()
	if err == nil {
		t.Fatal("GetDB(nil pool) returned no error")
	}
	want := "<TR:internal_database_pool_not_initialized>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("GetDB error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestHealthCheck_NotInitialized_GoesThroughTranslator covers the
// pre-init guard on HealthCheck.
func TestHealthCheck_NotInitialized_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	db := &Database{Pool: nil}
	err := db.HealthCheck()
	if err == nil {
		t.Fatal("HealthCheck(nil pool) returned no error")
	}
	want := "<TR:internal_database_pool_not_initialized>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("HealthCheck error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), the pre-init guard emits the bundle message ID —
// confirming the migration didn't accidentally pass an empty string
// or a different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	db := &Database{Pool: nil}
	_, err := db.GetDB()
	if err == nil {
		t.Fatal("GetDB(nil pool) returned no error")
	}
	if !strings.Contains(err.Error(), "internal_database_pool_not_initialized") {
		t.Fatalf("GetDB error = %q, want raw message ID (Noop echo)", err.Error())
	}
}
