// wire_phase4_workflow_test.go — HXC-036 Phase 4 acceptance test (final 2/74).
//
// Anti-bluff (§11.4 / §11.9): proves the shared internal/workflow/i18n
// translator — which serves BOTH internal/workflow/autonomy and
// internal/workflow/planmode via their per-package SetTranslator seams —
// renders REAL resolved + interpolated text through its boot-time
// NewTranslator constructor, NOT the NoopTranslator{} raw-message-ID echo.
//
// TestPhase4_WireAll_Succeeds re-asserts WireAll() returns nil now that the
// final two packages are covered (74/74).
package i18nwiring

import (
	"context"
	"strings"
	"testing"

	workflowi18n "dev.helix.code/internal/workflow/i18n"
)

func TestPhase4_WireAll_Succeeds(t *testing.T) {
	// WireAll now constructs translators for all 74 CONST-046 packages,
	// including internal/workflow/{autonomy,planmode}. A nil return proves
	// every embedded bundle loaded.
	if err := WireAll(); err != nil {
		t.Fatalf("WireAll() returned error (a package failed to build a real translator): %v", err)
	}
}

func TestPhase4_Workflow_ResolvesAutonomyLabel(t *testing.T) {
	tr, err := workflowi18n.NewTranslator()
	if err != nil {
		t.Fatalf("workflow NewTranslator: %v", err)
	}
	got, err := tr.T(context.Background(), "internal_workflow_mode_full_auto_label", nil)
	if err != nil {
		t.Fatalf("workflow T: %v", err)
	}
	want := "Full Auto (Fully Autonomous)"
	if got != want {
		t.Fatalf("workflow autonomy label mismatch:\n got=%q\nwant=%q", got, want)
	}
	if strings.Contains(got, "internal_workflow_") {
		t.Fatalf("workflow rendered a raw message-ID key (NoopTranslator regression): %q", got)
	}
	t.Logf("workflow autonomy label resolved: %q", got)
}

func TestPhase4_Workflow_ResolvesPlanmodeInterpolated(t *testing.T) {
	tr, err := workflowi18n.NewTranslator()
	if err != nil {
		t.Fatalf("workflow NewTranslator: %v", err)
	}
	got, err := tr.T(context.Background(), "internal_workflow_planmode_options_select_prompt", map[string]any{"Count": 3})
	if err != nil {
		t.Fatalf("workflow T: %v", err)
	}
	want := "Select an option (1-3): "
	if got != want {
		t.Fatalf("workflow planmode interpolated text mismatch:\n got=%q\nwant=%q", got, want)
	}
	if strings.Contains(got, "internal_workflow_") {
		t.Fatalf("workflow rendered a raw message-ID key (NoopTranslator regression): %q", got)
	}
	t.Logf("workflow planmode interpolated resolved: %q", got)
}
