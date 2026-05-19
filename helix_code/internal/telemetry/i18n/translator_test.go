// Unit tests for the internal/telemetry Translator interface +
// NoopTranslator default. Mocks ALLOWED per CONST-050(A) (unit tests
// only). Round-237 §11.4 anti-bluff sweep (2026-05-19) — NO-OP INFRA
// scaffold round.
package i18n

import (
	"context"
	"errors"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_telemetry_reserved_placeholder", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_telemetry_reserved_placeholder" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "internal_telemetry_reserved_placeholder", 1, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "internal_telemetry_reserved_placeholder" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_T_IgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator returns the raw ID even when
	// templateData is provided. This guarantees a test using
	// NoopTranslator can detect a non-i18n call site by the literal
	// remaining unchanged (sentinel = raw ID, not interpolated).
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_telemetry_reserved_placeholder", map[string]any{"Exporter": "otlp"})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_telemetry_reserved_placeholder" {
		t.Fatalf("NoopTranslator.T returned %q, want raw message ID (ignoring templateData)", got)
	}
}

// fakeTranslator returns a sentinel-wrapped message ID so call-site
// tests can assert the lookup actually went through Translator.T,
// not a hardcoded literal that happens to match the bundle value.
type fakeTranslator struct {
	failOnID string
}

func (f fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func (f fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func TestFakeTranslator_T_WrapsID(t *testing.T) {
	tr := fakeTranslator{}
	got, err := tr.T(context.Background(), "internal_telemetry_reserved_placeholder", nil)
	if err != nil {
		t.Fatalf("fakeTranslator.T returned error: %v", err)
	}
	want := "<TRANSLATED:internal_telemetry_reserved_placeholder>"
	if got != want {
		t.Fatalf("fakeTranslator.T returned %q, want %q", got, want)
	}
}

func TestFakeTranslator_TPlural_WrapsID(t *testing.T) {
	tr := fakeTranslator{}
	got, err := tr.TPlural(context.Background(), "internal_telemetry_reserved_placeholder", 2, nil)
	if err != nil {
		t.Fatalf("fakeTranslator.TPlural returned error: %v", err)
	}
	want := "<TRANSLATED:internal_telemetry_reserved_placeholder>"
	if got != want {
		t.Fatalf("fakeTranslator.TPlural returned %q, want %q", got, want)
	}
}

func TestFakeTranslator_T_FailOnID(t *testing.T) {
	tr := fakeTranslator{failOnID: "internal_telemetry_reserved_placeholder"}
	_, err := tr.T(context.Background(), "internal_telemetry_reserved_placeholder", nil)
	if err == nil {
		t.Fatalf("fakeTranslator.T should have returned deliberate-failure error")
	}
}
