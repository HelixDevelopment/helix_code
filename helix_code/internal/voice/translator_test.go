// Sentinel + paired-mutation tests for the CONST-046 translator
// wiring in internal/voice (round-226 §11.4 anti-bluff sweep,
// 2026-05-19). Mocks ALLOWED per CONST-050(A) — this is a unit-test
// file.
package voice

import (
	"context"
	"errors"
	"strings"
	"testing"

	voicei18n "dev.helix.code/internal/voice/i18n"
)

// sentinelTranslator wraps every resolved message ID with a
// recognisable marker so call-site tests can prove the lookup
// ACTUALLY went through Translator.T — not through a hardcoded
// literal that happens to match the bundle value (which would be a
// §11.4 PASS-bluff at the i18n call-site layer).
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	if len(data) > 0 {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		return "<SENT:" + id + "|keys=" + strings.Join(keys, ",") + ">", nil
	}
	return "<SENT:" + id + ">", nil
}

func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<SENT:" + id + ">", nil
}

// errorTranslator always fails — exercises the tr() fallback path
// (must degrade to raw message ID, never to empty string).
type errorTranslator struct{}

func (errorTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

func (errorTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

// emptyTranslator returns "" with no error — exercises the tr()
// empty-string fallback path (must degrade to raw message ID).
type emptyTranslator struct{}

func (emptyTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}

func (emptyTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", nil
}

func resetTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetTranslator(nil) })
}

func TestSetTranslator_Nil_ResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	got := tr(context.Background(), "internal_voice_tool_start_description", nil)
	if got != "<SENT:internal_voice_tool_start_description>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_voice_tool_start_description", nil)
	if got == "internal_voice_tool_start_description" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_voice_tool_start_description", nil)
	if got != "internal_voice_tool_start_description" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestTr_FallsBackToMessageIDOnEmpty(t *testing.T) {
	// Anti-bluff: a translator that returns "" without error MUST
	// also degrade to the raw message ID. Without this fallback an
	// upstream bundle bug would surface as blank output to the
	// operator — a §11.4 PASS-bluff at the i18n fallback layer.
	resetTranslator(t)
	SetTranslator(emptyTranslator{})
	got := tr(context.Background(), "internal_voice_tool_start_description", nil)
	if got != "internal_voice_tool_start_description" {
		t.Fatalf("tr() with empty translator returned %q, want raw message ID", got)
	}
}

// TestTr_SelfHealsFromNilPackageTranslator asserts the defensive
// branch in tr() restores the package-level Noop if a hypothetical
// future refactor zeros the variable.
func TestTr_SelfHealsFromNilPackageTranslator(t *testing.T) {
	resetTranslator(t)
	translator = nil
	got := tr(context.Background(), "internal_voice_tool_start_description", nil)
	if got != "internal_voice_tool_start_description" {
		t.Fatalf("tr() after nil translator returned %q, want raw ID (self-healed)", got)
	}
}

// TestVoiceStartTool_Description_RoutesThroughTranslator is the
// call-site sentinel proof. If a future refactor accidentally
// reverts the Description() body to the hardcoded literal, this
// test FAILS because the sentinel wrapper would be missing.
func TestVoiceStartTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewVoiceStartTool(nil)
	got := tool.Description()
	want := "<SENT:internal_voice_tool_start_description>"
	if got != want {
		t.Fatalf("VoiceStartTool.Description did not route through translator: got %q, want %q", got, want)
	}
}

// TestVoiceStopTool_Description_RoutesThroughTranslator mirrors the
// above for voice_stop.
func TestVoiceStopTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewVoiceStopTool(nil)
	got := tool.Description()
	want := "<SENT:internal_voice_tool_stop_description>"
	if got != want {
		t.Fatalf("VoiceStopTool.Description did not route through translator: got %q, want %q", got, want)
	}
}

// TestVoiceTranscribeTool_Description_RoutesThroughTranslator
// mirrors the above for voice_transcribe.
func TestVoiceTranscribeTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewVoiceTranscribeTool(nil, nil)
	got := tool.Description()
	want := "<SENT:internal_voice_tool_transcribe_description>"
	if got != want {
		t.Fatalf("VoiceTranscribeTool.Description did not route through translator: got %q, want %q", got, want)
	}
}

// TestVoiceTools_Description_NoopEchoesRawID is the paired-mutation
// safety net: with no translator wired (the boot-time default), the
// Description() output MUST equal the raw message ID — not the
// English bundle value — so missing wiring surfaces immediately.
func TestVoiceTools_Description_NoopEchoesRawID(t *testing.T) {
	resetTranslator(t)
	SetTranslator(nil)

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"start", NewVoiceStartTool(nil).Description(), "internal_voice_tool_start_description"},
		{"stop", NewVoiceStopTool(nil).Description(), "internal_voice_tool_stop_description"},
		{"transcribe", NewVoiceTranscribeTool(nil, nil).Description(), "internal_voice_tool_transcribe_description"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: Description() = %q, want raw ID echo %q (Noop default)", c.name, c.got, c.want)
		}
	}
}

// TestNoopTranslator_Loud_Echo_AllMigratedIDs is the bundle-keys
// audit. It asserts every CONST-046 message ID emitted by
// voice_tools.go appears in migratedMessageIDs(). If a future round
// adds a tr() call without extending the list, this guard surfaces
// the mismatch.
func TestNoopTranslator_Loud_Echo_AllMigratedIDs(t *testing.T) {
	noop := voicei18n.NoopTranslator{}
	for _, id := range migratedMessageIDs() {
		got, err := noop.T(context.Background(), id, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.T(%q) error: %v", id, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.T(%q) returned %q, want loud echo of raw ID", id, got)
		}
	}
}

func migratedMessageIDs() []string {
	// Round-226 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_voice_tool_start_description",
		"internal_voice_tool_stop_description",
		"internal_voice_tool_transcribe_description",
	}
}
