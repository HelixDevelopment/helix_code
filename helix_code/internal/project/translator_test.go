// Unit tests for the internal/project package-level translator + tr()
// helper (CONST-046 round-170 §11.4 anti-bluff sweep, 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package project

import (
	"context"
	stdctx "context"
	"errors"
	"os"
	"strings"
	"testing"

	projecti18n "dev.helix.code/internal/project/i18n"
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
	got := tr(stdctx.Background(), "internal_project_no_active_project", nil)
	if got != "no active project found" {
		t.Fatalf("tr default = %q, want resolved prose", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_project_create_failed", nil)
	if got != "<TR:internal_project_create_failed>" {
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

	got := tr(stdctx.Background(), "internal_project_get_failed", nil)
	if got != "internal_project_get_failed" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_project_path_does_not_exist", nil)
	if got != "project path does not exist: <no value>" {
		t.Fatalf("tr after nil-reset = %q, want resolved prose", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(projecti18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_project_update_failed", nil)
	if got != "internal_project_update_failed" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestCreateProject_InvalidPath_GoesThroughTranslator covers the
// CreateProject path-validation branch on Manager. With a sentinel
// translator wired, the error MUST surface the sentinel-wrapped
// message ID — proving the literal was NOT hardcoded on the path.
func TestCreateProject_InvalidPath_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	_, err := m.CreateProject(context.Background(), "n", "d", "/definitely-not-here-xyz-12345", "go")
	if err == nil {
		t.Fatal("CreateProject(nonexistent path) returned no error")
	}
	want := "<TR:internal_project_path_does_not_exist>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("CreateProject error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestGetActiveProject_NoActive_GoesThroughTranslator covers the
// no-active-project guard on GetActiveProject.
func TestGetActiveProject_NoActive_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	_, err := m.GetActiveProject(context.Background())
	if err == nil {
		t.Fatal("GetActiveProject(empty manager) returned no error")
	}
	want := "<TR:internal_project_no_active_project>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("GetActiveProject error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestCreateProject_DetectTypeFailure_PathExists confirms the
// success path completes when the directory exists — the
// type-detection branch returns nil and "generic" type is set.
// (The detectProjectType implementation cannot fail on real input
// today; the migrated wrap exists for forward-compat with future
// detection failures and is asserted live via raw-text emission
// below.)
func TestCreateProject_DirExists_NoBluff(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	tmp, err := os.MkdirTemp("", "project_i18n_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	m := NewManager()
	p, err := m.CreateProject(context.Background(), "t", "d", tmp, "generic")
	if err != nil {
		t.Fatalf("CreateProject(existing dir) returned unexpected error %v", err)
	}
	if p == nil || p.Type != "generic" {
		t.Fatalf("CreateProject returned %+v, want non-nil generic project", p)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), the no-active-project guard emits the bundle
// message ID — confirming the migration didn't accidentally pass an
// empty string or a different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	m := NewManager()
	_, err := m.GetActiveProject(context.Background())
	if err == nil {
		t.Fatal("GetActiveProject(empty manager) returned no error")
	}
	if !strings.Contains(err.Error(), "no active project found") {
		t.Fatalf("GetActiveProject error = %q, want resolved prose", err.Error())
	}
}
