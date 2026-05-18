// Unit tests for the iOS Translator interface + NoopTranslator
// default. Mocks ALLOWED per CONST-050(A) (unit tests only).
//
// Round-138 §11.4 anti-bluff sweep (2026-05-18): iOS scope is
// infrastructure-only (pure Swift application; zero Go-side user-
// facing strings currently). These tests exercise:
//
//   - NoopTranslator loud-echo behaviour (T + TPlural).
//   - fakeTranslator sentinel wrap (the contract every future call-
//     site test will use to prove the Translator was consulted).
//   - Interface conformance (compile-time + runtime nil-guard).
//
// When the first Go-side user-facing string lands in the iOS bridge
// surface, sentinel-call-site tests modelled on rounds 96/136/137
// will join this file via a sibling *_i18n_test.go.
package i18n

import (
	"context"
	"errors"
	"testing"
)

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "ios_bridge_seed_placeholder", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "ios_bridge_seed_placeholder" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(context.Background(), "ios_notification_seed_placeholder", 3, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "ios_notification_seed_placeholder" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_T_TemplateDataIgnored(t *testing.T) {
	// Loud-echo contract: NoopTranslator MUST ignore templateData
	// — interpolation is the real Translator's job. Verifies the
	// default never accidentally formats placeholders.
	tr := NoopTranslator{}
	got, err := tr.T(context.Background(), "ios_bridge_seed_placeholder", map[string]any{"Name": "Alice", "Count": 7})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "ios_bridge_seed_placeholder" {
		t.Fatalf("NoopTranslator.T should echo ID verbatim ignoring templateData; got %q", got)
	}
}

// fakeTranslator returns a sentinel-wrapped message ID so future
// call-site tests can assert the lookup actually went through
// Translator.T, not a hardcoded literal that happens to match the
// bundle value.
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
	got, err := tr.T(context.Background(), "ios_bridge_seed_placeholder", nil)
	if err != nil {
		t.Fatalf("fakeTranslator.T returned error: %v", err)
	}
	want := "<TRANSLATED:ios_bridge_seed_placeholder>"
	if got != want {
		t.Fatalf("fakeTranslator.T returned %q, want %q", got, want)
	}
}

func TestFakeTranslator_TPlural_WrapsID(t *testing.T) {
	tr := fakeTranslator{}
	got, err := tr.TPlural(context.Background(), "ios_notification_seed_placeholder", 5, nil)
	if err != nil {
		t.Fatalf("fakeTranslator.TPlural returned error: %v", err)
	}
	want := "<TRANSLATED:ios_notification_seed_placeholder>"
	if got != want {
		t.Fatalf("fakeTranslator.TPlural returned %q, want %q", got, want)
	}
}

func TestFakeTranslator_T_FailOnID(t *testing.T) {
	tr := fakeTranslator{failOnID: "ios_bridge_seed_placeholder"}
	_, err := tr.T(context.Background(), "ios_bridge_seed_placeholder", nil)
	if err == nil {
		t.Fatalf("fakeTranslator.T should have failed for configured failOnID")
	}
}

// Interface-conformance compile-time guard: both NoopTranslator and
// fakeTranslator MUST satisfy Translator. Compilation failure here
// catches accidental signature drift before any call site rebuilds.
var (
	_ Translator = NoopTranslator{}
	_ Translator = fakeTranslator{}
)
