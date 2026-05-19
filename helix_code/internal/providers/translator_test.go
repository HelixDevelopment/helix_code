// Unit tests for the internal/providers package-level Translator
// seam (SetTranslator + tr()). Mocks ALLOWED per CONST-050(A) (unit
// tests only). Round-172 §11.4 anti-bluff sweep (2026-05-19).
package providers

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/internal/providers/i18n"
)

// recordingTranslator captures every T/TPlural call so call-site
// tests can assert the lookup was attempted with the expected
// message ID + templateData. Sentinel-wrapped output also lets
// callers verify the return value flowed through Translator.T
// instead of being a hardcoded literal that happens to match.
type recordingTranslator struct {
	lastID      string
	lastData    map[string]any
	failOnID    string
	emptyReturn bool
}

func (r *recordingTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	r.lastID = id
	r.lastData = data
	if r.failOnID != "" && id == r.failOnID {
		return "", errors.New("recordingTranslator: deliberate failure for " + id)
	}
	if r.emptyReturn {
		return "", nil
	}
	return "<TR:" + id + ">", nil
}

func (r *recordingTranslator) TPlural(_ context.Context, id string, _ int, data map[string]any) (string, error) {
	r.lastID = id
	r.lastData = data
	return "<TR:" + id + ">", nil
}

func TestSetTranslator_WiresInjectedTranslator(t *testing.T) {
	t.Cleanup(func() { SetTranslator(nil) })

	rec := &recordingTranslator{}
	SetTranslator(rec)

	got := tr(context.Background(), "internal_providers_ai_provider_not_found", map[string]any{"Name": "openai"})

	if rec.lastID != "internal_providers_ai_provider_not_found" {
		t.Fatalf("recordingTranslator did not see expected ID; got %q", rec.lastID)
	}
	if rec.lastData["Name"] != "openai" {
		t.Fatalf("recordingTranslator did not see expected templateData; got %v", rec.lastData)
	}
	if got != "<TR:internal_providers_ai_provider_not_found>" {
		t.Fatalf("tr() returned %q, want sentinel-wrapped ID", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	t.Cleanup(func() { SetTranslator(nil) })

	SetTranslator(&recordingTranslator{})
	SetTranslator(nil)

	got := tr(context.Background(), "internal_providers_vector_not_found", nil)
	if got != "internal_providers_vector_not_found" {
		t.Fatalf("after SetTranslator(nil), tr() returned %q, want loud message-ID echo", got)
	}
}

func TestTr_DefaultsToNoopWhenUnwired(t *testing.T) {
	// Belt-and-braces: even after no SetTranslator call ever, tr()
	// must fall back to NoopTranslator-style echo. This guards
	// against accidental nil-pointer dereferences at package init.
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "internal_providers_provider_not_initialized", nil)
	if got != "internal_providers_provider_not_initialized" {
		t.Fatalf("default tr() returned %q, want loud message-ID echo", got)
	}
}

func TestTr_FallsBackToIDOnTranslatorError(t *testing.T) {
	// Anti-bluff (paired mutation per §11.4): translator error MUST
	// degrade loudly to the raw message ID, NEVER swallow into an
	// empty string. An empty-string fallback would silently strip
	// user-visible error context — a §11.4 PASS-bluff at the i18n
	// failure path.
	t.Cleanup(func() { SetTranslator(nil) })

	SetTranslator(&recordingTranslator{failOnID: "internal_providers_personality_not_found"})

	got := tr(context.Background(), "internal_providers_personality_not_found", map[string]any{"ID": "p1"})
	if got != "internal_providers_personality_not_found" {
		t.Fatalf("on translator error, tr() returned %q, want raw message ID", got)
	}
}

func TestTr_FallsBackToIDOnEmptyReturn(t *testing.T) {
	t.Cleanup(func() { SetTranslator(nil) })

	SetTranslator(&recordingTranslator{emptyReturn: true})

	got := tr(context.Background(), "internal_providers_conversation_not_found", nil)
	if got != "internal_providers_conversation_not_found" {
		t.Fatalf("on empty translator return, tr() returned %q, want raw message ID", got)
	}
}

// Compile-time interface assertion — recordingTranslator MUST
// satisfy i18n.Translator. If the interface ever changes shape, this
// fails to compile, surfacing the break loudly at test time rather
// than at first runtime call.
var _ i18n.Translator = (*recordingTranslator)(nil)
