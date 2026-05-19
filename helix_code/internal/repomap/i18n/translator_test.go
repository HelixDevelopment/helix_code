// Unit tests for internal/repomap/i18n.Translator contract +
// NoopTranslator (CONST-046 round-198 §11.4 anti-bluff sweep,
// 2026-05-19; recovery from round-174 stall).
//
// Paired-mutation tests per §11.4: NoopTranslator round-trips IDs
// verbatim — a regression that silently translated them (or returned
// empty) would mask broken bundle wiring. Mocks ALLOWED per
// CONST-050(A) (unit tests only).
package i18n

import (
	"context"
	"testing"
)

func TestNoopTranslator_TEchoesID(t *testing.T) {
	got, err := NoopTranslator{}.T(context.Background(), "internal_repomap_tool_description", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T err = %v, want nil", err)
	}
	if got != "internal_repomap_tool_description" {
		t.Fatalf("NoopTranslator.T = %q, want raw ID echo", got)
	}
}

func TestNoopTranslator_TPluralEchoesID(t *testing.T) {
	got, err := NoopTranslator{}.TPlural(context.Background(), "internal_repomap_tool_description", 7, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural err = %v, want nil", err)
	}
	if got != "internal_repomap_tool_description" {
		t.Fatalf("NoopTranslator.TPlural = %q, want raw ID echo", got)
	}
}

func TestNoopTranslator_IgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator MUST NOT silently inject template
	// data into the raw ID (that would be a §11.4 PASS-bluff at the
	// i18n layer — caller would think interpolation worked when no
	// real Translator is wired).
	got, _ := NoopTranslator{}.T(context.Background(), "internal_repomap_msg_with_{{.placeholder}}", map[string]any{"placeholder": "VALUE"})
	if got != "internal_repomap_msg_with_{{.placeholder}}" {
		t.Fatalf("NoopTranslator.T interpolated template data = %q, want raw ID echo", got)
	}
}

func TestNoopTranslator_SatisfiesTranslatorInterface(t *testing.T) {
	// Compile-time contract assertion: NoopTranslator implements
	// Translator. If the interface signature drifts, this test fails
	// at compile time.
	var _ Translator = NoopTranslator{}
}

// TestNoopTranslator_NilTemplateDataIsSafe guards the contract that
// callers may pass nil templateData without panicking — a common
// call-site pattern when no interpolation is required.
func TestNoopTranslator_NilTemplateDataIsSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NoopTranslator.T panicked on nil templateData: %v", r)
		}
	}()
	_, _ = NoopTranslator{}.T(context.Background(), "internal_repomap_tool_description", nil)
}

// TestNoopTranslator_EmptyMessageIDEchoes guards the contract that
// even an empty ID is echoed verbatim — silent substitution would
// be a §11.4 PASS-bluff.
func TestNoopTranslator_EmptyMessageIDEchoes(t *testing.T) {
	got, _ := NoopTranslator{}.T(context.Background(), "", nil)
	if got != "" {
		t.Fatalf("NoopTranslator.T empty ID = %q, want empty echo", got)
	}
}
