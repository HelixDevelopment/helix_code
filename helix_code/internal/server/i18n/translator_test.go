// Unit tests for the internal/server Translator interface +
// NoopTranslator default. Mocks ALLOWED per CONST-050(A) (unit tests
// only).
package i18n

import (
	"context"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_server_qa_engine_disabled", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_server_qa_engine_disabled" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "internal_server_authentication_required", 1, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "internal_server_authentication_required" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_T_IgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator returns the raw ID even when
	// templateData is provided. This guarantees a test using
	// NoopTranslator can detect a non-i18n call site by the literal
	// remaining unchanged (sentinel = raw ID, not interpolated).
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_server_failed_list_projects", map[string]any{"Err": "boom"})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_server_failed_list_projects" {
		t.Fatalf("NoopTranslator.T returned %q, want raw message ID (ignoring templateData)", got)
	}
}

func TestNoopTranslator_AssertsTranslatorContract(t *testing.T) {
	// Compile-time guarantee that NoopTranslator satisfies the
	// Translator interface. If the interface ever drifts, this test
	// will fail at build, not silently at runtime.
	var _ Translator = NoopTranslator{}
}
