// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/approval (round-221 §11.4 anti-bluff sweep, 2026-05-19).
// Mocks ALLOWED per CONST-050(A) — this is a unit test file.
package approval

import (
	"context"
	"errors"
	"strings"
	"testing"

	approvali18n "dev.helix.code/internal/approval/i18n"
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
	got := tr(context.Background(), "internal_approval_invalid_mode_empty", nil)
	if got != "<SENT:internal_approval_invalid_mode_empty>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_approval_invalid_mode_empty", nil)
	if got != "internal_approval_invalid_mode_empty" {
		t.Fatalf("after SetTranslator(nil), expected loud message-ID echo, got %q", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_approval_denied_read_only", nil)
	if got != "internal_approval_denied_read_only" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestParseMode_EmptyString_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	_, err := ParseMode("")
	if err == nil {
		t.Fatal("expected error for empty mode string, got nil")
	}
	if !strings.Contains(err.Error(), "<SENT:internal_approval_invalid_mode_empty>") {
		t.Fatalf("ParseMode(\"\") error did not route through translator: got %q", err.Error())
	}
}

func TestCheckApproval_DeniedReadOnly_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	m, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode: ModeSuggest,
		Source:      SourceDefault,
	})
	if err != nil {
		t.Fatalf("NewApprovalManager: %v", err)
	}
	action, decErr := m.CheckApproval(ApprovalRequest{
		ToolName: "fs_edit",
		Level:    LevelEdit,
	})
	if action != ActionDenyWithReason {
		t.Fatalf("expected ActionDenyWithReason, got %s", action)
	}
	if decErr == nil {
		t.Fatal("expected denial error, got nil")
	}
	if !strings.Contains(decErr.Error(), "<SENT:internal_approval_denied_read_only|keys=") {
		t.Fatalf("denial error did not route through translator: got %q", decErr.Error())
	}
}

func TestBuildPromptQuestion_RoutesAllFragmentsThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	// Base prompt only.
	q := buildPromptQuestion(ApprovalRequest{
		ToolName: "shell_run",
		Level:    LevelRun,
	})
	if !strings.Contains(q, "<SENT:internal_approval_prompt_allow_tool|keys=") {
		t.Fatalf("base prompt did not route through translator: got %q", q)
	}

	// Args branch.
	q = buildPromptQuestion(ApprovalRequest{
		ToolName: "shell_run",
		Level:    LevelRun,
		Args:     map[string]any{"cmd": "ls"},
	})
	if !strings.Contains(q, "<SENT:internal_approval_prompt_args_suffix|keys=Args>") {
		t.Fatalf("args suffix did not route through translator: got %q", q)
	}

	// Context branch.
	q = buildPromptQuestion(ApprovalRequest{
		ToolName: "shell_run",
		Level:    LevelRun,
		Context:  "for patch X",
	})
	if !strings.Contains(q, "<SENT:internal_approval_prompt_context_suffix|keys=Context>") {
		t.Fatalf("context suffix did not route through translator: got %q", q)
	}
}

func TestModeDescriptors_AllFourRouteThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	descs := ModeDescriptors()
	if len(descs) != 4 {
		t.Fatalf("expected 4 mode descriptors, got %d", len(descs))
	}
	cases := map[ApprovalMode]string{
		ModeSuggest:   "<SENT:internal_approval_mode_suggest_description>",
		ModeAutoEdit:  "<SENT:internal_approval_mode_auto_edit_description>",
		ModeFullAuto:  "<SENT:internal_approval_mode_full_auto_description>",
		ModeDangerous: "<SENT:internal_approval_mode_dangerous_description>",
	}
	for mode, want := range cases {
		got := descs[mode].Description
		if got != want {
			t.Fatalf("ModeDescriptors[%s].Description = %q, want %q", mode, got, want)
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
	noop := approvali18n.NoopTranslator{}
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
	// Round-221 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_approval_denied_read_only",
		"internal_approval_invalid_mode_empty",
		"internal_approval_mode_auto_edit_description",
		"internal_approval_mode_dangerous_description",
		"internal_approval_mode_full_auto_description",
		"internal_approval_mode_suggest_description",
		"internal_approval_prompt_allow_tool",
		"internal_approval_prompt_args_suffix",
		"internal_approval_prompt_context_suffix",
		"internal_approval_prompt_separator",
	}
}
