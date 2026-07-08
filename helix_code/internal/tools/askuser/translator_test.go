// Unit tests for the internal/tools/askuser package-level translator +
// tr() helper (CONST-046 round-440 §11.4 anti-bluff sweep, 2026-05-20).
//
// Paired-mutation per §11.4: a planted Translator yields output
// distinguishable from the un-planted (NoopTranslator) baseline at the
// seam, so a regression that bypasses the translator surfaces immediately.
// Mocks/fakes ALLOWED per CONST-050(A) (unit-test scope only).
package askuser

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	askuseri18n "dev.helix.code/internal/tools/askuser/i18n"
)

// sentinelTranslator wraps the message ID so seam tests can prove tr()
// actually went through Translator.T rather than returning a hardcoded
// literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

// errTranslator forces tr() onto the degradation path so we can prove it
// surfaces the raw message ID instead of an empty string (a blank prompt
// would be a §11.4 PASS-bluff at the i18n layer).
type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// emptyTranslator returns "" without error — exercises the secondary
// "out == \"\"" degradation path in tr().
type emptyTranslator struct{}

func (emptyTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}
func (emptyTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", nil
}

func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "askuser_prompt_enter_choice_no_default", nil)
	if got != "askuser_prompt_enter_choice_no_default" {
		t.Fatalf("tr default = %q, want resolved prose", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "askuser_prompt_invalid_choice_hint", nil)
	want := "<TR:askuser_prompt_invalid_choice_hint>"
	if got != want {
		t.Fatalf("tr = %q, want %q — seam bypassed Translator", got, want)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "askuser_prompt_choice_preview_label", nil)
	if got != "askuser_prompt_choice_preview_label" {
		t.Fatalf("tr on translator error = %q, want raw message ID", got)
	}
}

func TestTr_EmptyTranslationReturnsMessageID(t *testing.T) {
	resetTranslator(t)
	SetTranslator(emptyTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "askuser_prompt_enter_choice_with_default", nil)
	if got != "askuser_prompt_enter_choice_with_default" {
		t.Fatalf("tr on empty translation = %q, want raw message ID", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil)
	got := tr(stdctx.Background(), "askuser_prompt_invalid_choice_hint", nil)
	if got != "askuser_prompt_invalid_choice_hint" {
		t.Fatalf("after SetTranslator(nil) tr = %q, want resolved prose", got)
	}
}

func TestNoopTranslator_EchoesID(t *testing.T) {
	var n askuseri18n.NoopTranslator
	out, err := n.T(stdctx.Background(), "askuser_prompt_enter_choice_no_default", nil)
	if err != nil || out != "askuser_prompt_enter_choice_no_default" {
		t.Fatalf("NoopTranslator.T = (%q,%v), want loud echo", out, err)
	}
	pl, err := n.TPlural(stdctx.Background(), "askuser_prompt_invalid_choice_hint", 3, nil)
	if err != nil || pl != "askuser_prompt_invalid_choice_hint" {
		t.Fatalf("NoopTranslator.TPlural = (%q,%v), want loud echo", pl, err)
	}
}

// TestFormatQuestion_RoutesThroughTranslator is the paired-mutation proof
// that FormatQuestion's user-facing footer + preview label go through the
// CONST-046 seam: a sentinel translator must change the rendered output.
func TestFormatQuestion_RoutesThroughTranslator(t *testing.T) {
	q := Question{
		Question: "Pick one",
		Choices: []Choice{
			{Label: "Yes", Value: "yes", Preview: "diff"},
			{Label: "No", Value: "no"},
		},
		Default: "no",
	}

	resetTranslator(t)
	baseline := FormatQuestion(q) // NoopTranslator — raw message IDs echoed
	if !strings.Contains(baseline, "askuser_prompt_enter_choice_with_default") {
		t.Fatalf("baseline produced resolved prose: %q", baseline)
	}
	if !strings.Contains(baseline, "askuser_prompt_choice_preview_label") {
		t.Fatalf("baseline did not echo preview message ID: %q", baseline)
	}

	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)
	planted := FormatQuestion(q)
	if planted == baseline {
		t.Fatalf("FormatQuestion output identical with planted translator — seam bypassed")
	}
	if !strings.Contains(planted, "<TR:askuser_prompt_enter_choice_with_default>") {
		t.Fatalf("planted output did not route footer through translator: %q", planted)
	}
}

// TestInvalidChoiceHint_RoutesThroughTranslator proves the retry hint goes
// through the seam.
func TestInvalidChoiceHint_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	baseline := invalidChoiceHint(stdctx.Background(), 3)
	if baseline != "askuser_prompt_invalid_choice_hint" {
		t.Fatalf("baseline hint = %q, want resolved prose", baseline)
	}

	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)
	planted := invalidChoiceHint(stdctx.Background(), 3)
	if planted != "<TR:askuser_prompt_invalid_choice_hint>" {
		t.Fatalf("planted hint = %q — seam bypassed", planted)
	}
}
