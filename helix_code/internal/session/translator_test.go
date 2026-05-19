// Unit tests for the internal/session package-level translator + tr()
// helper (CONST-046 round-178 §11.4 anti-bluff sweep, 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package session

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	sessioni18n "dev.helix.code/internal/session/i18n"
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
	got := tr(stdctx.Background(), "internal_session_id_empty", nil)
	if got != "internal_session_id_empty" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_session_name_empty", nil)
	if got != "<TR:internal_session_name_empty>" {
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

	got := tr(stdctx.Background(), "internal_session_invalid_mode", nil)
	if got != "internal_session_invalid_mode" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_session_start_completed", nil)
	if got != "internal_session_start_completed" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(sessioni18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_session_project_id_empty", nil)
	if got != "internal_session_project_id_empty" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestSessionValidate_IDEmpty_GoesThroughTranslator covers the
// session.Validate ID-empty branch. With a sentinel translator wired,
// the error MUST surface the sentinel-wrapped message ID — proving
// the literal was NOT hardcoded on the path.
func TestSessionValidate_IDEmpty_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	s := &Session{ID: "", ProjectID: "p", Name: "n", Mode: ModePlanning, Status: StatusActive}
	err := s.Validate()
	if err == nil {
		t.Fatal("Validate(empty ID) returned no error")
	}
	want := "<TR:internal_session_id_empty>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestSessionValidate_InvalidMode_GoesThroughTranslator covers the
// session.Validate invalid-mode branch with templated placeholder.
func TestSessionValidate_InvalidMode_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	s := &Session{ID: "x", ProjectID: "p", Name: "n", Mode: Mode("nope"), Status: StatusActive}
	err := s.Validate()
	if err == nil {
		t.Fatal("Validate(invalid mode) returned no error")
	}
	want := "<TR:internal_session_invalid_mode>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerCreate_InvalidMode_GoesThroughTranslator covers the
// Manager.Create invalid-mode validation branch.
func TestManagerCreate_InvalidMode_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	_, err := m.Create("p1", "n1", "desc", Mode("nope"))
	if err == nil {
		t.Fatal("Create(invalid mode) returned no error")
	}
	want := "<TR:internal_session_create_invalid_mode>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Create error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerCreate_ProjectIDEmpty_GoesThroughTranslator covers the
// Manager.Create projectID-empty validation branch.
func TestManagerCreate_ProjectIDEmpty_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	_, err := m.Create("", "n1", "desc", ModePlanning)
	if err == nil {
		t.Fatal("Create(empty projectID) returned no error")
	}
	want := "<TR:internal_session_project_id_empty>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Create error = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), the validation error emits the bundle message ID
// — confirming the migration didn't accidentally pass an empty string
// or a different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	s := &Session{ID: "", ProjectID: "p", Name: "n", Mode: ModePlanning, Status: StatusActive}
	err := s.Validate()
	if err == nil {
		t.Fatal("Validate(empty ID) returned no error")
	}
	if !strings.Contains(err.Error(), "internal_session_id_empty") {
		t.Fatalf("Validate error = %q, want raw message ID (Noop echo)", err.Error())
	}
}
