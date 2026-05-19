// Unit tests for the internal/workflow Translator interface +
// NoopTranslator default. Mocks ALLOWED per CONST-050(A) (unit tests
// only).
package i18n

import (
	"context"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_workflow_mode_none_label", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_workflow_mode_none_label" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "internal_workflow_planmode_status_generating_options", 1, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "internal_workflow_planmode_status_generating_options" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_T_IgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator returns the raw ID even when
	// templateData is provided. This guarantees a test using
	// NoopTranslator can detect a non-i18n call site by the literal
	// remaining unchanged (sentinel = raw ID, not interpolated).
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_workflow_mode_full_auto_label", map[string]any{"Detail": "ignored"})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_workflow_mode_full_auto_label" {
		t.Fatalf("NoopTranslator.T returned %q, want raw message ID (ignoring templateData)", got)
	}
}

func TestNoopTranslator_AssertsTranslatorContract(t *testing.T) {
	// Compile-time guarantee that NoopTranslator satisfies the
	// Translator interface. If the interface ever drifts, this test
	// will fail at build, not silently at runtime.
	var _ Translator = NoopTranslator{}
}
