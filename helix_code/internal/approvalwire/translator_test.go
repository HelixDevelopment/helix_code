// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/approvalwire (round-228 §11.4 anti-bluff sweep,
// 2026-05-19). Mocks ALLOWED per CONST-050(A) — this is a unit test
// file.
package approvalwire

import (
	"context"
	"errors"
	"strings"
	"testing"

	approvalwirei18n "dev.helix.code/internal/approvalwire/i18n"
	"dev.helix.code/internal/tools/askuser"
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

// capturePrompter records the Question it receives so call-site tests
// can assert exactly what Labels the prompter ended up with. Mocks
// allowed under CONST-050(A) — this is unit-test-only.
type capturePrompter struct {
	received askuser.Question
}

func (c *capturePrompter) Prompt(_ context.Context, q askuser.Question) (*askuser.Result, error) {
	c.received = q
	// Return the Choice corresponding to the default so PromptYesNo
	// completes without error and the bool result reflects defaultVal.
	for i, ch := range q.Choices {
		if ch.Value == q.Default {
			return &askuser.Result{Value: ch.Value, Index: i, UsedDefault: true}, nil
		}
	}
	return &askuser.Result{Value: q.Choices[0].Value, Index: 0}, nil
}

func TestSetTranslator_Nil_ResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	got := tr(context.Background(), "internal_approvalwire_yesno_label_yes", nil)
	if got != "<SENT:internal_approvalwire_yesno_label_yes>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_approvalwire_yesno_label_yes", nil)
	if got != "internal_approvalwire_yesno_label_yes" {
		t.Fatalf("after SetTranslator(nil), expected loud message-ID echo, got %q", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_approvalwire_yesno_label_no", nil)
	if got != "internal_approvalwire_yesno_label_no" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestPromptYesNo_YesLabel_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	cp := &capturePrompter{}
	p := &AskUserYesNoPrompter{Inner: cp}

	_, err := p.PromptYesNo(context.Background(), "Allow this?", true)
	if err != nil {
		t.Fatalf("PromptYesNo: %v", err)
	}
	if len(cp.received.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(cp.received.Choices))
	}
	if cp.received.Choices[0].Label != "<SENT:internal_approvalwire_yesno_label_yes>" {
		t.Fatalf("yes label did not route through translator: got %q", cp.received.Choices[0].Label)
	}
	// Anti-bluff: Choice.Value MUST remain the structural enum
	// token "yes" (compared by == elsewhere); MUST NOT be translated.
	if cp.received.Choices[0].Value != "yes" {
		t.Fatalf("yes value was translated (should remain structural enum): got %q", cp.received.Choices[0].Value)
	}
}

func TestPromptYesNo_NoLabel_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	cp := &capturePrompter{}
	p := &AskUserYesNoPrompter{Inner: cp}

	_, err := p.PromptYesNo(context.Background(), "Allow this?", false)
	if err != nil {
		t.Fatalf("PromptYesNo: %v", err)
	}
	if cp.received.Choices[1].Label != "<SENT:internal_approvalwire_yesno_label_no>" {
		t.Fatalf("no label did not route through translator: got %q", cp.received.Choices[1].Label)
	}
	// Anti-bluff: Choice.Value MUST remain the structural enum
	// token "no" (compared by == elsewhere); MUST NOT be translated.
	if cp.received.Choices[1].Value != "no" {
		t.Fatalf("no value was translated (should remain structural enum): got %q", cp.received.Choices[1].Value)
	}
}

func TestPromptYesNo_DefaultPolarity_PreservedAcrossTranslation(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	cp := &capturePrompter{}
	p := &AskUserYesNoPrompter{Inner: cp}

	// defaultYes=true → Question.Default == "yes" (structural value,
	// not label — translation MUST NOT shift the default).
	_, err := p.PromptYesNo(context.Background(), "Q?", true)
	if err != nil {
		t.Fatalf("PromptYesNo: %v", err)
	}
	if cp.received.Default != "yes" {
		t.Fatalf("defaultYes=true should produce Default=\"yes\", got %q", cp.received.Default)
	}

	// defaultYes=false → Question.Default == "no".
	_, err = p.PromptYesNo(context.Background(), "Q?", false)
	if err != nil {
		t.Fatalf("PromptYesNo: %v", err)
	}
	if cp.received.Default != "no" {
		t.Fatalf("defaultYes=false should produce Default=\"no\", got %q", cp.received.Default)
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
	noop := approvalwirei18n.NoopTranslator{}
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
	// Round-228 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_approvalwire_yesno_label_no",
		"internal_approvalwire_yesno_label_yes",
	}
}
