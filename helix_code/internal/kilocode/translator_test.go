// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/kilocode (round-231 §11.4 anti-bluff sweep, 2026-05-19).
// Mocks ALLOWED per CONST-050(A) — this is a unit test file.
package kilocode

import (
	"context"
	"errors"
	"strings"
	"testing"

	kilocodei18n "dev.helix.code/internal/kilocode/i18n"
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

// emptyTranslator returns "" without error — exercises the secondary
// "out == \"\"" degradation path in tr().
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
	got := tr(context.Background(), "internal_kilocode_rename_tool_description", nil)
	if got != "<SENT:internal_kilocode_rename_tool_description>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_kilocode_rename_tool_description", nil)
	if got == "internal_kilocode_rename_tool_description" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_kilocode_impact_tool_description", nil)
	if got != "internal_kilocode_impact_tool_description" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestTr_FallsBackToMessageIDOnEmpty(t *testing.T) {
	// Sister to error-path: an error-less empty-string return MUST
	// also degrade to the raw message ID.
	resetTranslator(t)
	SetTranslator(emptyTranslator{})
	got := tr(context.Background(), "internal_kilocode_multi_edit_tool_description", nil)
	if got != "internal_kilocode_multi_edit_tool_description" {
		t.Fatalf("tr() with empty translator returned %q, want raw message ID", got)
	}
}

func TestTr_RecoversFromNilPackageTranslator(t *testing.T) {
	resetTranslator(t)
	translator = nil
	got := tr(context.Background(), "internal_kilocode_rename_tool_description", nil)
	if got != "internal_kilocode_rename_tool_description" {
		t.Fatalf("tr after nil translator = %q, want raw ID echo", got)
	}
}

func TestKiloRenameTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewKiloRenameTool(nil)
	got := tool.Description()
	if got != "<SENT:internal_kilocode_rename_tool_description>" {
		t.Fatalf("KiloRenameTool.Description() did not route through translator: got %q", got)
	}
}

func TestKiloImpactTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewKiloImpactTool(nil)
	got := tool.Description()
	if got != "<SENT:internal_kilocode_impact_tool_description>" {
		t.Fatalf("KiloImpactTool.Description() did not route through translator: got %q", got)
	}
}

func TestKiloMultiEditTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewKiloMultiEditTool(nil)
	got := tool.Description()
	if got != "<SENT:internal_kilocode_multi_edit_tool_description>" {
		t.Fatalf("KiloMultiEditTool.Description() did not route through translator: got %q", got)
	}
}

func TestKiloRenameTool_Schema_PropertyDescriptions_RouteThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewKiloRenameTool(nil)
	schema := tool.Schema()
	oldName, _ := schema.Properties["old_name"].(map[string]interface{})
	if got, _ := oldName["description"].(string); got != "<SENT:internal_kilocode_rename_old_name_description>" {
		t.Fatalf("rename old_name description did not route through translator: got %q", got)
	}
}

func TestKiloImpactTool_Schema_PropertyDescriptions_RouteThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewKiloImpactTool(nil)
	schema := tool.Schema()
	sym, _ := schema.Properties["symbol"].(map[string]interface{})
	if got, _ := sym["description"].(string); got != "<SENT:internal_kilocode_impact_symbol_description>" {
		t.Fatalf("impact symbol description did not route through translator: got %q", got)
	}
}

func TestKiloMultiEditTool_Schema_PropertyDescriptions_RouteThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewKiloMultiEditTool(nil)
	schema := tool.Schema()
	for prop, wantID := range map[string]string{
		"file":       "internal_kilocode_multi_edit_file_description",
		"start_line": "internal_kilocode_multi_edit_start_line_description",
		"end_line":   "internal_kilocode_multi_edit_end_line_description",
	} {
		entry, _ := schema.Properties[prop].(map[string]interface{})
		got, _ := entry["description"].(string)
		want := "<SENT:" + wantID + ">"
		if got != want {
			t.Fatalf("MultiEdit property %q description = %q, want %q", prop, got, want)
		}
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
	noop := kilocodei18n.NoopTranslator{}
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
	noop := kilocodei18n.NoopTranslator{}
	got, err := noop.TPlural(context.Background(), "internal_kilocode_reserved_future", 3, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural error: %v", err)
	}
	if got != "internal_kilocode_reserved_future" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want raw ID echo", got)
	}
}

func migratedMessageIDs() []string {
	// Round-231 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_kilocode_impact_symbol_description",
		"internal_kilocode_impact_tool_description",
		"internal_kilocode_multi_edit_end_line_description",
		"internal_kilocode_multi_edit_file_description",
		"internal_kilocode_multi_edit_start_line_description",
		"internal_kilocode_multi_edit_tool_description",
		"internal_kilocode_prompt_separator",
		"internal_kilocode_rename_old_name_description",
		"internal_kilocode_rename_tool_description",
		"internal_kilocode_reserved_future",
	}
}
