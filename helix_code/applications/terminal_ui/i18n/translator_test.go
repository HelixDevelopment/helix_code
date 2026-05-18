// Unit tests for the terminal_ui Translator interface + NoopTranslator
// default. Mocks ALLOWED per CONST-050(A) (unit tests only).
package i18n

import (
	"context"
	"errors"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "terminal_ui_sidebar_title", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "terminal_ui_sidebar_title" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "terminal_ui_tasks_count", 3, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "terminal_ui_tasks_count" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
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
	got, err := tr.T(context.Background(), "terminal_ui_sidebar_title", nil)
	if err != nil {
		t.Fatalf("fakeTranslator.T returned error: %v", err)
	}
	want := "<TRANSLATED:terminal_ui_sidebar_title>"
	if got != want {
		t.Fatalf("fakeTranslator.T returned %q, want %q", got, want)
	}
}
