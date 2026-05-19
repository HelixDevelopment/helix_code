// Unit tests for the internal/secrets Translator interface +
// NoopTranslator default. Mocks ALLOWED per CONST-050(A) (unit tests
// only). Round-225 §11.4 anti-bluff sweep (2026-05-19).
package i18n

import (
	"context"
	"errors"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_secrets_no_source_found", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_secrets_no_source_found" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "internal_secrets_no_source_found", 1, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "internal_secrets_no_source_found" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_T_IgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator returns the raw ID even when
	// templateData is provided. This guarantees a test using
	// NoopTranslator can detect a non-i18n call site by the literal
	// remaining unchanged (sentinel = raw ID, not interpolated).
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "internal_secrets_no_source_found", map[string]any{"Path": "/some/path"})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "internal_secrets_no_source_found" {
		t.Fatalf("NoopTranslator.T returned %q, want raw message ID (ignoring templateData)", got)
	}
}

// TestNoopTranslator_CONST042_NoSecretLeak proves that even when a
// caller mistakenly passes a secret-looking value through
// templateData, the NoopTranslator output does NOT include the value
// — it echoes the raw ID. This is the i18n-layer mirror of the
// loader.go CONST-042 §12.1 anti-leak guarantee: nothing about the
// translation seam can be tricked into emitting a credential.
func TestNoopTranslator_CONST042_NoSecretLeak(t *testing.T) {
	tr := NoopTranslator{}
	secretLookalike := "sk-this-is-not-a-real-key-but-tests-the-no-leak-invariant"
	got, err := tr.T(context.Background(), "internal_secrets_no_source_found", map[string]any{
		"FakeAPIKey": secretLookalike,
	})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got == "" {
		t.Fatal("NoopTranslator.T returned empty string; want raw message ID")
	}
	// Critical assertion: the "secret" value MUST NOT appear in the
	// returned string under any condition. Even if a future
	// refactor wires templateData interpolation, NoopTranslator's
	// contract is loud-echo of the raw ID — never substituted.
	if got == secretLookalike {
		t.Fatal("CONST-042 violation: NoopTranslator output exposed templateData value")
	}
	if got != "internal_secrets_no_source_found" {
		t.Fatalf("NoopTranslator.T returned %q, want raw message ID", got)
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
	got, err := tr.T(context.Background(), "internal_secrets_no_source_found", nil)
	if err != nil {
		t.Fatalf("fakeTranslator.T returned error: %v", err)
	}
	want := "<TRANSLATED:internal_secrets_no_source_found>"
	if got != want {
		t.Fatalf("fakeTranslator.T returned %q, want %q", got, want)
	}
}

func TestFakeTranslator_T_FailOnID(t *testing.T) {
	tr := fakeTranslator{failOnID: "internal_secrets_no_source_found"}
	if _, err := tr.T(context.Background(), "internal_secrets_no_source_found", nil); err == nil {
		t.Fatal("fakeTranslator.T expected to fail for matching ID, got nil error")
	}
	// Non-matching IDs should still succeed.
	if _, err := tr.T(context.Background(), "internal_secrets_other_msg", nil); err != nil {
		t.Fatalf("fakeTranslator.T returned error for non-failing ID: %v", err)
	}
}
