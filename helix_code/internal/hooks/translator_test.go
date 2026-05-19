// Unit tests for the internal/hooks package-level translator +
// tr() helper (CONST-046 round-160 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package hooks

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	hooksi18n "dev.helix.code/internal/hooks/i18n"
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
	got := tr(stdctx.Background(), "internal_hooks_id_empty", nil)
	if got != "internal_hooks_id_empty" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_hooks_name_empty", nil)
	if got != "<TR:internal_hooks_name_empty>" {
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

	got := tr(stdctx.Background(), "internal_hooks_not_found", map[string]any{"ID": "x"})
	if got != "internal_hooks_not_found" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_hooks_handler_nil", nil)
	if got != "internal_hooks_handler_nil" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(hooksi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_hooks_type_empty", nil)
	if got != "internal_hooks_type_empty" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestHookValidate_EmptyID_GoesThroughTranslator asserts the
// "hook ID cannot be empty" guard surfaces through tr() — proving
// the literal is NOT hardcoded on the path.
func TestHookValidate_EmptyID_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	h := &Hook{ID: "", Name: "n", Type: HookTypeBeforeTask, Handler: func(stdctx.Context, *Event) error { return nil }}
	err := h.Validate()
	if err == nil {
		t.Fatal("Validate on empty ID returned no error")
	}
	want := "<TR:internal_hooks_id_empty>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestHookValidate_EmptyID_RawTextByDefault asserts the Noop-default
// surface for the empty-ID guard.
func TestHookValidate_EmptyID_RawTextByDefault(t *testing.T) {
	resetTranslator(t)

	h := &Hook{}
	err := h.Validate()
	if err == nil {
		t.Fatal("Validate on empty ID returned no error")
	}
	if !strings.Contains(err.Error(), "internal_hooks_id_empty") {
		t.Fatalf("Validate err = %q, want raw message ID (Noop echo)", err.Error())
	}
}

// TestHookValidate_EmptyName_GoesThroughTranslator covers the
// "hook name cannot be empty" path.
func TestHookValidate_EmptyName_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	h := &Hook{ID: "id1", Name: "", Type: HookTypeBeforeTask, Handler: func(stdctx.Context, *Event) error { return nil }}
	err := h.Validate()
	if err == nil {
		t.Fatal("Validate on empty Name returned no error")
	}
	want := "<TR:internal_hooks_name_empty>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestHookValidate_EmptyType_GoesThroughTranslator covers the
// "hook type cannot be empty" path.
func TestHookValidate_EmptyType_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	h := &Hook{ID: "id1", Name: "n", Type: "", Handler: func(stdctx.Context, *Event) error { return nil }}
	err := h.Validate()
	if err == nil {
		t.Fatal("Validate on empty Type returned no error")
	}
	want := "<TR:internal_hooks_type_empty>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestHookValidate_NilHandler_GoesThroughTranslator covers the
// "hook handler cannot be nil" path.
func TestHookValidate_NilHandler_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	h := &Hook{ID: "id1", Name: "n", Type: HookTypeBeforeTask, Handler: nil}
	err := h.Validate()
	if err == nil {
		t.Fatal("Validate on nil Handler returned no error")
	}
	want := "<TR:internal_hooks_handler_nil>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestHookValidate_PriorityOutOfRange_GoesThroughTranslator covers
// the "invalid priority" path with placeholder interpolation.
func TestHookValidate_PriorityOutOfRange_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	h := &Hook{
		ID:       "id1",
		Name:     "n",
		Type:     HookTypeBeforeTask,
		Handler:  func(stdctx.Context, *Event) error { return nil },
		Priority: PriorityHighest + 100,
	}
	err := h.Validate()
	if err == nil {
		t.Fatal("Validate on out-of-range Priority returned no error")
	}
	want := "<TR:internal_hooks_priority_out_of_range>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerRegister_DuplicateID_GoesThroughTranslator covers the
// "hook with ID 'X' already registered" path on Manager.Register.
func TestManagerRegister_DuplicateID_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	h := &Hook{
		ID:       "dup",
		Name:     "n",
		Type:     HookTypeBeforeTask,
		Handler:  func(stdctx.Context, *Event) error { return nil },
		Priority: PriorityNormal,
	}
	if err := m.Register(h); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	// second registration must trip the duplicate guard
	err := m.Register(h)
	if err == nil {
		t.Fatal("Register duplicate returned no error")
	}
	want := "<TR:internal_hooks_id_already_registered>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Register err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerUnregister_NotFound_GoesThroughTranslator covers the
// "hook not found" path on Manager.Unregister.
func TestManagerUnregister_NotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	err := m.Unregister("nonexistent")
	if err == nil {
		t.Fatal("Unregister on missing ID returned no error")
	}
	want := "<TR:internal_hooks_not_found>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Unregister err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerGet_NotFound_GoesThroughTranslator covers the
// "hook not found" path on Manager.Get.
func TestManagerGet_NotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Fatal("Get on missing ID returned no error")
	}
	want := "<TR:internal_hooks_not_found>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Get err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerRegister_InvalidHook_GoesThroughTranslator covers the
// "invalid hook: %w" wrapper on Manager.Register. The wrapped inner
// error is itself a tr()-produced message from Validate, so both
// layers should carry sentinel markers.
func TestManagerRegister_InvalidHook_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	// Hook with empty ID — Validate will return tr(internal_hooks_id_empty),
	// Register will wrap with tr(internal_hooks_invalid_hook).
	h := &Hook{}
	err := m.Register(h)
	if err == nil {
		t.Fatal("Register invalid hook returned no error")
	}
	wantOuter := "<TR:internal_hooks_invalid_hook>"
	wantInner := "<TR:internal_hooks_id_empty>"
	if !strings.Contains(err.Error(), wantOuter) {
		t.Fatalf("Register err = %q, want contain %q (outer wrapper)", err.Error(), wantOuter)
	}
	if !strings.Contains(err.Error(), wantInner) {
		t.Fatalf("Register err = %q, want contain %q (inner Validate)", err.Error(), wantInner)
	}
}

// TestValidateAPIVersion_Missing_GoesThroughTranslator covers the
// "missing apiVersion" path on the YAML loader.
func TestValidateAPIVersion_Missing_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	err := validateAPIVersion("")
	if err == nil {
		t.Fatal("validateAPIVersion(\"\") returned no error")
	}
	want := "<TR:internal_hooks_yaml_missing_api_version>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("validateAPIVersion err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestValidateAPIVersion_Unsupported_GoesThroughTranslator covers
// the "unsupported apiVersion" path on the YAML loader.
func TestValidateAPIVersion_Unsupported_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	err := validateAPIVersion("helixcode.hooks/v999")
	if err == nil {
		t.Fatal("validateAPIVersion on unsupported version returned no error")
	}
	want := "<TR:internal_hooks_yaml_unsupported_api_version>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("validateAPIVersion err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}
