// Unit tests for the internal/tools/confirmation package-level
// translator + tr() helper (CONST-046 round-382 §11.4 anti-bluff
// sweep, 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site
// (defaultDangerPatterns, PromptFormatter.Format, defaultOptions).
// A regression that re-hardcodes any literal makes the sentinel
// assertion fail. Mocks ALLOWED per CONST-050(A) (unit tests only).
package confirmation

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	confirmi18n "dev.helix.code/internal/tools/confirmation/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests
// can assert tr() actually went through Translator.T rather than
// returning a hardcoded literal that happened to match the bundle
// value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_tools_confirmation_danger_rm_rf", nil)
	if got == "internal_tools_confirmation_danger_rm_rf" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_tools_confirmation_danger_sudo", nil)
	if got != "<TR:internal_tools_confirmation_danger_sudo>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — operator sees blank output). Implementation MUST
	// degrade to the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_tools_confirmation_prompt_title", nil)
	if got != "internal_tools_confirmation_prompt_title" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_tools_confirmation_option_allow_label", nil)
	if got == "internal_tools_confirmation_option_allow_label" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(confirmi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_tools_confirmation_option_deny_label", nil)
	if got != "internal_tools_confirmation_option_deny_label" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestDefaultDangerPatterns_DescriptionsGoThroughTranslator covers
// every danger.go Description() call site. With a sentinel
// translator wired, EVERY pattern description MUST surface the
// sentinel-wrapped message ID — proving no literal was hardcoded.
func TestDefaultDangerPatterns_DescriptionsGoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	want := map[string]string{
		"delete_operation":      "<TR:internal_tools_confirmation_danger_delete_operation>",
		"system_files":          "<TR:internal_tools_confirmation_danger_system_files>",
		"rm_rf_command":         "<TR:internal_tools_confirmation_danger_rm_rf>",
		"git_force_push":        "<TR:internal_tools_confirmation_danger_git_force_push>",
		"main_branch_operation": "<TR:internal_tools_confirmation_danger_main_branch>",
		"network_request":       "<TR:internal_tools_confirmation_danger_network_request>",
		"sudo_command":          "<TR:internal_tools_confirmation_danger_sudo>",
		"chmod_777":             "<TR:internal_tools_confirmation_danger_chmod_777>",
		"drop_table":            "<TR:internal_tools_confirmation_danger_drop_table>",
		"truncate_table":        "<TR:internal_tools_confirmation_danger_truncate_table>",
		"npm_publish":           "<TR:internal_tools_confirmation_danger_npm_publish>",
		"pip_upload":            "<TR:internal_tools_confirmation_danger_pip_upload>",
		"docker_system_prune":   "<TR:internal_tools_confirmation_danger_docker_prune>",
		"format_disk":           "<TR:internal_tools_confirmation_danger_format_disk>",
		"kill_process":          "<TR:internal_tools_confirmation_danger_kill_process>",
	}
	patterns := defaultDangerPatterns()
	if len(patterns) != len(want) {
		t.Fatalf("defaultDangerPatterns count = %d, want %d", len(patterns), len(want))
	}
	for _, p := range patterns {
		exp, ok := want[p.Name]
		if !ok {
			t.Errorf("unexpected pattern %q", p.Name)
			continue
		}
		if p.Description != exp {
			t.Errorf("pattern %q Description = %q, want %q — call site bypassed tr()",
				p.Name, p.Description, exp)
		}
	}
}

// TestDefaultDangerPatterns_RawIDsByDefault asserts that with no
// translator wired, every description echoes its bundle message ID
// (NoopTranslator) — confirming the migration didn't accidentally
// emit an empty string.
func TestDefaultDangerPatterns_RawIDsByDefault(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	for _, p := range defaultDangerPatterns() {
		if p.Description == "" {
			t.Errorf("pattern %q Description is empty — migration emitted blank", p.Name)
		}
		if !strings.HasPrefix(p.Description, "internal_tools_confirmation_danger_") {
			t.Errorf("pattern %q Description = %q, want resolved danger description", p.Name, p.Description)
		}
	}
}

// TestDefaultOptions_LabelsGoThroughTranslator covers every
// defaultOptions() Label/Description call site.
func TestDefaultOptions_LabelsGoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	opts := defaultOptions()
	if len(opts) != 4 {
		t.Fatalf("defaultOptions count = %d, want 4", len(opts))
	}
	type wantPair struct{ label, desc string }
	wants := map[Choice]wantPair{
		ChoiceAllow:  {"<TR:internal_tools_confirmation_option_allow_label>", "<TR:internal_tools_confirmation_option_allow_description>"},
		ChoiceDeny:   {"<TR:internal_tools_confirmation_option_deny_label>", "<TR:internal_tools_confirmation_option_deny_description>"},
		ChoiceAlways: {"<TR:internal_tools_confirmation_option_always_label>", "<TR:internal_tools_confirmation_option_always_description>"},
		ChoiceNever:  {"<TR:internal_tools_confirmation_option_never_label>", "<TR:internal_tools_confirmation_option_never_description>"},
	}
	for _, o := range opts {
		w, ok := wants[o.Choice]
		if !ok {
			t.Errorf("unexpected option choice %v", o.Choice)
			continue
		}
		if o.Label != w.label {
			t.Errorf("option %v Label = %q, want %q — call site bypassed tr()", o.Choice, o.Label, w.label)
		}
		if o.Description != w.desc {
			t.Errorf("option %v Description = %q, want %q — call site bypassed tr()", o.Choice, o.Description, w.desc)
		}
	}
}

// TestPromptFormatter_TitleGoesThroughTranslator covers the
// PromptFormatter.Format() Title call site, including the {{.Tool}}
// placeholder interpolation contract.
func TestPromptFormatter_TitleGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	pf := &PromptFormatter{}
	got := pf.Format(PromptRequest{Tool: "bash"})
	if got.Title != "<TR:internal_tools_confirmation_prompt_title>" {
		t.Fatalf("prompt Title = %q, want sentinel-wrapped ID — call site bypassed tr()", got.Title)
	}
}

// TestPromptFormatter_IrreversibleWarningGoesThroughTranslator
// covers the "NOT reversible" warning call site in Format().
func TestPromptFormatter_IrreversibleWarningGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	pf := &PromptFormatter{}
	got := pf.Format(PromptRequest{
		Tool: "bash",
		Danger: &DangerAssessment{
			Dangers:    []string{"some danger"},
			Reversible: false,
		},
	})
	found := false
	for _, d := range got.Details {
		if d == "<TR:internal_tools_confirmation_danger_irreversible_warning>" {
			found = true
		}
	}
	if !found {
		t.Fatalf("irreversible warning not surfaced via tr(); Details = %v", got.Details)
	}
}
