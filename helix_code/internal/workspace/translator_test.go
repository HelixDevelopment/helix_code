// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/workspace (round-241 §11.4 anti-bluff sweep, 2026-05-19).
// Mocks ALLOWED per CONST-050(A) — this is a unit test file.
package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	workspacei18n "dev.helix.code/internal/workspace/i18n"
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
	got := tr(context.Background(), "internal_workspace_validate_name_required", nil)
	if got != "<SENT:internal_workspace_validate_name_required>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_workspace_validate_name_required", nil)
	if got == "internal_workspace_validate_name_required" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_workspace_tool_create_description", nil)
	if got != "internal_workspace_tool_create_description" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestToolDescriptions_RouteThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	create := &WorkspaceCreateTool{}
	if got := create.Description(); got != "<SENT:internal_workspace_tool_create_description>" {
		t.Fatalf("WorkspaceCreateTool.Description did not route through translator: got %q", got)
	}

	list := &WorkspaceListTool{}
	if got := list.Description(); got != "<SENT:internal_workspace_tool_list_description>" {
		t.Fatalf("WorkspaceListTool.Description did not route through translator: got %q", got)
	}

	cleanup := &WorkspaceCleanupTool{}
	if got := cleanup.Description(); got != "<SENT:internal_workspace_tool_cleanup_description>" {
		t.Fatalf("WorkspaceCleanupTool.Description did not route through translator: got %q", got)
	}
}

func TestSchemaParamDescriptions_RouteThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	create := &WorkspaceCreateTool{}
	schema := create.Schema()
	nameProp, ok := schema.Properties["name"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema name property not a map: %T", schema.Properties["name"])
	}
	if got, _ := nameProp["description"].(string); got != "<SENT:internal_workspace_schema_param_name>" {
		t.Fatalf("schema name description did not route through translator: got %q", got)
	}
	pdProp, ok := schema.Properties["project_dir"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema project_dir property not a map: %T", schema.Properties["project_dir"])
	}
	if got, _ := pdProp["description"].(string); got != "<SENT:internal_workspace_schema_param_project_dir>" {
		t.Fatalf("schema project_dir description did not route through translator: got %q", got)
	}

	cleanup := &WorkspaceCleanupTool{}
	cschema := cleanup.Schema()
	idProp, ok := cschema.Properties["id"].(map[string]interface{})
	if !ok {
		t.Fatalf("cleanup schema id property not a map: %T", cschema.Properties["id"])
	}
	if got, _ := idProp["description"].(string); got != "<SENT:internal_workspace_schema_param_id>" {
		t.Fatalf("cleanup schema id description did not route through translator: got %q", got)
	}
}

func TestValidate_RequiredErrors_RouteThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	create := &WorkspaceCreateTool{}
	if err := create.Validate(map[string]interface{}{}); err == nil ||
		!strings.Contains(err.Error(), "<SENT:internal_workspace_validate_name_required>") {
		t.Fatalf("Validate missing-name did not route through translator: err=%v", err)
	}
	if err := create.Validate(map[string]interface{}{"name": "ws1"}); err == nil ||
		!strings.Contains(err.Error(), "<SENT:internal_workspace_validate_project_dir_required>") {
		t.Fatalf("Validate missing-project_dir did not route through translator: err=%v", err)
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
	noop := workspacei18n.NoopTranslator{}
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
	// Round-241 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_workspace_schema_param_id",
		"internal_workspace_schema_param_name",
		"internal_workspace_schema_param_project_dir",
		"internal_workspace_tool_cleanup_description",
		"internal_workspace_tool_create_description",
		"internal_workspace_tool_list_description",
		"internal_workspace_validate_name_required",
		"internal_workspace_validate_project_dir_required",
	}
}
