// Unit tests for the internal/template package-level translator +
// tr() helper (CONST-046 round-180 §11.4 anti-bluff sweep,
// 2026-05-19, Phase 4 round 73).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package template

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	templatei18n "dev.helix.code/internal/template/i18n"
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
	got := tr(stdctx.Background(), "internal_template_validate_name_empty", nil)
	if got == "internal_template_validate_name_empty" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_template_validate_content_empty", nil)
	if got != "<TR:internal_template_validate_content_empty>" {
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

	got := tr(stdctx.Background(), "internal_template_manager_not_found", map[string]any{"ID": "x"})
	if got != "internal_template_manager_not_found" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_template_validate_invalid_type", nil)
	if got == "internal_template_validate_invalid_type" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(templatei18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_template_manager_duplicate_name", nil)
	if got != "internal_template_manager_duplicate_name" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestTemplateValidate_EmptyName_GoesThroughTranslator asserts the
// "template name cannot be empty" guard surfaces through tr() —
// proving the literal is NOT hardcoded on the path.
func TestTemplateValidate_EmptyName_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tpl := &Template{Name: "", Content: "x", Type: TypeCode}
	err := tpl.Validate()
	if err == nil {
		t.Fatal("Validate on empty Name returned no error")
	}
	want := "<TR:internal_template_validate_name_empty>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestTemplateValidate_EmptyName_RawTextByDefault asserts the
// Noop-default surface for the empty-name guard.
func TestTemplateValidate_EmptyName_RawTextByDefault(t *testing.T) {
	resetTranslator(t)

	tpl := &Template{Name: "", Content: "x", Type: TypeCode}
	err := tpl.Validate()
	if err == nil {
		t.Fatal("Validate on empty Name returned no error")
	}
	if !strings.Contains(err.Error(), "internal_template_validate_name_empty") {
		t.Fatalf("Validate err = %q, want raw message ID (Noop echo)", err.Error())
	}
}

// TestTemplateValidate_EmptyContent_GoesThroughTranslator covers
// the "template content cannot be empty" path.
func TestTemplateValidate_EmptyContent_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tpl := &Template{Name: "n", Content: "", Type: TypeCode}
	err := tpl.Validate()
	if err == nil {
		t.Fatal("Validate on empty Content returned no error")
	}
	want := "<TR:internal_template_validate_content_empty>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestTemplateValidate_InvalidType_GoesThroughTranslator covers
// the "invalid template type" path with placeholder interpolation.
func TestTemplateValidate_InvalidType_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tpl := &Template{Name: "n", Content: "x", Type: Type("bogus")}
	err := tpl.Validate()
	if err == nil {
		t.Fatal("Validate on invalid Type returned no error")
	}
	want := "<TR:internal_template_validate_invalid_type>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestTemplateValidate_DeclaredVarMissing_GoesThroughTranslator
// covers the "declared variable not found in content" path.
func TestTemplateValidate_DeclaredVarMissing_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tpl := &Template{
		Name:      "n",
		Content:   "no placeholders here",
		Type:      TypeCode,
		Variables: []Variable{{Name: "ghost", Required: true, Type: "string"}},
	}
	err := tpl.Validate()
	if err == nil {
		t.Fatal("Validate on missing declared var returned no error")
	}
	want := "<TR:internal_template_validate_declared_variable_missing>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestTemplateRender_RequiredVarMissing_GoesThroughTranslator
// covers the "required variable is missing" path.
func TestTemplateRender_RequiredVarMissing_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tpl := &Template{
		Name:      "n",
		Content:   "{{x}}",
		Type:      TypeCode,
		Variables: []Variable{{Name: "x", Required: true, Type: "string"}},
	}
	_, err := tpl.Render(map[string]interface{}{})
	if err == nil {
		t.Fatal("Render with missing required var returned no error")
	}
	want := "<TR:internal_template_render_required_variable_missing>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Render err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestTemplateRender_UnreplacedPlaceholders_GoesThroughTranslator
// covers the "template has unreplaced placeholders" path. We
// declare no variables (so ValidateVariables passes) but leave a
// placeholder in content unresolved.
func TestTemplateRender_UnreplacedPlaceholders_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tpl := &Template{
		Name:    "n",
		Content: "{{leftover}}",
		Type:    TypeCode,
		// no declared Variables, so ValidateVariables short-circuits
	}
	_, err := tpl.Render(map[string]interface{}{})
	if err == nil {
		t.Fatal("Render with unreplaced placeholders returned no error")
	}
	want := "<TR:internal_template_render_unreplaced_placeholders>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Render err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerRegister_DuplicateName_GoesThroughTranslator covers the
// "template with name 'X' already exists" path on Manager.Register.
func TestManagerRegister_DuplicateName_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	tpl1 := NewTemplate("dup-name", "d", TypeCode)
	tpl1.SetContent("hello")
	if err := m.Register(tpl1); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	tpl2 := NewTemplate("dup-name", "d", TypeCode)
	tpl2.SetContent("hello")
	err := m.Register(tpl2)
	if err == nil {
		t.Fatal("Register duplicate name returned no error")
	}
	want := "<TR:internal_template_manager_duplicate_name>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Register err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerGet_NotFound_GoesThroughTranslator covers the
// "template not found" path on Manager.Get.
func TestManagerGet_NotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Fatal("Get on missing ID returned no error")
	}
	want := "<TR:internal_template_manager_not_found>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Get err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerGetByName_NotFound_GoesThroughTranslator covers the
// "template not found" path on Manager.GetByName.
func TestManagerGetByName_NotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	_, err := m.GetByName("nonexistent")
	if err == nil {
		t.Fatal("GetByName on missing name returned no error")
	}
	want := "<TR:internal_template_manager_not_found>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("GetByName err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestManagerDelete_NotFound_GoesThroughTranslator covers the
// "template not found" path on Manager.Delete.
func TestManagerDelete_NotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	m := NewManager()
	err := m.Delete("nonexistent")
	if err == nil {
		t.Fatal("Delete on missing ID returned no error")
	}
	want := "<TR:internal_template_manager_not_found>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Delete err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}
