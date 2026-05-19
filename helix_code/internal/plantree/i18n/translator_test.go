// Unit tests for the internal/plantree/i18n Translator + NoopTranslator
// (CONST-046 round-233 §11.4 anti-bluff sweep, 2026-05-19).
//
// Mocks ALLOWED per CONST-050(A) (unit tests only). These tests guard
// the interface contract + the Noop fallback so the consumer-side
// tr() helper has a stable foundation.
package i18n

import (
	"context"
	"testing"
)

func TestNoopTranslator_TReturnsIDUnchanged(t *testing.T) {
	n := NoopTranslator{}
	got, err := n.T(context.Background(), "internal_plantree_some_id", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error %v, want nil", err)
	}
	if got != "internal_plantree_some_id" {
		t.Fatalf("NoopTranslator.T = %q, want raw id (loud echo)", got)
	}
}

func TestNoopTranslator_TIgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator MUST NOT silently template-render
	// the data into the string (would produce surprise output that
	// hides a missing real translator). It MUST echo the id raw so
	// the operator sees an obvious "id leaked" signal.
	n := NoopTranslator{}
	got, _ := n.T(context.Background(), "internal_plantree_failed_x", map[string]any{"Err": "boom"})
	if got != "internal_plantree_failed_x" {
		t.Fatalf("NoopTranslator.T with data = %q, want raw id", got)
	}
}

func TestNoopTranslator_TPluralReturnsIDUnchanged(t *testing.T) {
	n := NoopTranslator{}
	got, err := n.TPlural(context.Background(), "internal_plantree_plural_id", 5, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error %v, want nil", err)
	}
	if got != "internal_plantree_plural_id" {
		t.Fatalf("NoopTranslator.TPlural = %q, want raw id (loud echo)", got)
	}
}

// TestNoopTranslator_SatisfiesTranslatorInterface is a compile-time
// guarantee that NoopTranslator implements Translator. If a future
// edit breaks the interface contract, this test fails to compile —
// no false PASS from a missing method.
func TestNoopTranslator_SatisfiesTranslatorInterface(t *testing.T) {
	var _ Translator = NoopTranslator{}
}
