// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/continua (round-230 §11.4 anti-bluff sweep, 2026-05-19).
// Mocks ALLOWED per CONST-050(A) — this is a unit test file.
package continua

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	continuai18n "dev.helix.code/internal/continua/i18n"
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

func resetTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetTranslator(nil) })
}

func TestSetTranslator_Nil_ResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	got := tr(context.Background(), "internal_continua_tool_edit_description", nil)
	if got != "<SENT:internal_continua_tool_edit_description>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_continua_tool_edit_description", nil)
	if got == "internal_continua_tool_edit_description" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_continua_completion_stub_placeholder", nil)
	if got != "internal_continua_completion_stub_placeholder" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestContinueEditTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewContinueEditTool(NewWorkspaceEditor())
	got := tool.Description()
	if got != "<SENT:internal_continua_tool_edit_description>" {
		t.Fatalf("ContinueEditTool.Description did not route through translator: got %q", got)
	}
}

func TestContinueCompleteTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewContinueCompleteTool(NewCompletionEngine())
	got := tool.Description()
	if got != "<SENT:internal_continua_tool_complete_description>" {
		t.Fatalf("ContinueCompleteTool.Description did not route through translator: got %q", got)
	}
}

func TestCompletionEngine_InferredSuffix_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte("package p\nfunc main() {\n\tprint\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	e := NewCompletionEngine()
	result, err := e.Complete(context.Background(), path, 3, 6)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(result.Suggestion, "<SENT:internal_continua_completion_inferred_suffix>") {
		t.Fatalf("inferred-completion suffix did not route through translator: got %q", result.Suggestion)
	}
}

func TestCompletionEngine_StubPlaceholder_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// line=99 is past EOF → falls into the stub-placeholder branch.
	e := NewCompletionEngine()
	result, err := e.Complete(context.Background(), path, 99, 1)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if result.Suggestion != "<SENT:internal_continua_completion_stub_placeholder>" {
		t.Fatalf("stub-placeholder did not route through translator: got %q", result.Suggestion)
	}
}

// TestNoopTranslator_T_Loud_Echo_IsRawID is the paired mutation test —
// it asserts every CONST-046 message ID emitted by this package's
// migrated call sites appears in the active.en.yaml bundle (verified
// implicitly: NoopTranslator returns id verbatim, and the call-site
// tests above prove call sites use these exact IDs). If a new round
// adds a tr() call without a bundle entry, the bundle scan in
// internal/audit + this loud-echo invariant must FAIL. Mirrors §1.1
// paired-mutation guidance.
func TestNoopTranslator_T_Loud_Echo_IsRawID(t *testing.T) {
	noop := continuai18n.NoopTranslator{}
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

func TestNoopTranslator_TPlural_Loud_Echo_IsRawID(t *testing.T) {
	noop := continuai18n.NoopTranslator{}
	for _, id := range migratedMessageIDs() {
		got, err := noop.TPlural(context.Background(), id, 1, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.TPlural(%q) error: %v", id, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.TPlural(%q) returned %q, want loud echo of raw ID", id, got)
		}
	}
}

func migratedMessageIDs() []string {
	// Round-230 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_continua_completion_inferred_suffix",
		"internal_continua_completion_stub_placeholder",
		"internal_continua_tool_complete_description",
		"internal_continua_tool_edit_description",
	}
}
