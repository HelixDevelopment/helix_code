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

// TestTr_DefaultsToRealBundleTranslator — §11.4.120 reconcile (HXC-097,
// 2026-06-15). The package default is now the real embedded-bundle
// translator installed at init(), so the default path resolves to bundle
// PROSE. HXC-097 regression guard: a revert to NoopTranslator{} (or an
// embed-load failure) re-surfaces the raw key and FAILs this test.
func TestTr_DefaultsToRealBundleTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_database_pool_not_initialized", nil)
	if got != "database pool is not initialized" {
		t.Fatalf("tr default = %q, want resolved bundle prose %q "+
			"(HXC-097: default must be the real embedded-bundle translator, not Noop)",
			got, "database pool is not initialized")
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

// TestSetTranslator_NilRestoresDefault — §11.4.120 reconcile (HXC-097,
// 2026-06-15). SetTranslator(nil) now restores the package DEFAULT (the
// real embedded-bundle translator), not NoopTranslator{} — so after wiring
// a sentinel and nil-resetting, tr() resolves to bundle prose.
func TestSetTranslator_NilRestoresDefault(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // restore default
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_database_pool_not_initialized", nil)
	if got != "database pool is not initialized" {
		t.Fatalf("tr after nil-reset = %q, want resolved bundle prose %q (default restored)",
			got, "database pool is not initialized")
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

// TestResolvedProse_EmittedByDefault — §11.4.120 reconcile (HXC-097,
// 2026-06-15). With the real embedded-bundle translator as the package
// default, the pre-init guard surfaces the resolved bundle PROSE — the
// actual text an end user sees. HXC-097 regression guard at the GetDB
// call site: a revert to Noop re-surfaces the raw key and FAILs here.
func TestResolvedProse_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	db := &Database{Pool: nil}
	_, err := db.GetDB()
	if err == nil {
		t.Fatal("GetDB(nil pool) returned no error")
	}
	if !strings.Contains(err.Error(), "database pool is not initialized") {
		t.Fatalf("GetDB error = %q, want resolved bundle prose %q (real-bundle default)",
			err.Error(), "database pool is not initialized")
	}
	if strings.Contains(err.Error(), "internal_database_pool_not_initialized") {
		t.Fatalf("GetDB error = %q still contains the raw message-ID key "+
			"(HXC-097 regression: package fell back to NoopTranslator)", err.Error())
	}
}
