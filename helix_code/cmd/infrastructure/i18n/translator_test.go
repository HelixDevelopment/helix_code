// Unit tests for the infrastructure CLI Translator interface +
// NoopTranslator default. Mocks ALLOWED per CONST-050(A) (unit tests
// only).
package i18n

import (
	"context"
	"errors"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "infra_start_starting", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "infra_start_starting" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "infra_start_count_started", 3, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "infra_start_count_started" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_T_IgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator returns the raw ID even when
	// templateData is provided. This guarantees a test using
	// NoopTranslator can detect a non-i18n call site by the literal
	// remaining unchanged (sentinel = raw ID, not interpolated).
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "infra_error_unknown_mode", map[string]any{"Mode": "bogus"})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "infra_error_unknown_mode" {
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
	got, err := tr.T(context.Background(), "infra_status_heading", nil)
	if err != nil {
		t.Fatalf("fakeTranslator.T returned error: %v", err)
	}
	want := "<TRANSLATED:infra_status_heading>"
	if got != want {
		t.Fatalf("fakeTranslator.T returned %q, want %q", got, want)
	}
}

func TestFakeTranslator_T_FailOnID(t *testing.T) {
	tr := fakeTranslator{failOnID: "infra_stop_stopping"}
	if _, err := tr.T(context.Background(), "infra_stop_stopping", nil); err == nil {
		t.Fatal("fakeTranslator.T expected to fail for matching ID, got nil error")
	}
	// Non-matching IDs should still succeed.
	if _, err := tr.T(context.Background(), "infra_start_starting", nil); err != nil {
		t.Fatalf("fakeTranslator.T returned error for non-failing ID: %v", err)
	}
}
