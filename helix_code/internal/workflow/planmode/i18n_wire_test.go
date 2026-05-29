// i18n_wire_test.go — HXC-036 Phase 4 anti-bluff acceptance test for the
// internal/workflow/planmode CONST-046 wiring.
//
// Exercises the REAL CLI option-presenter render path
// (CLIOptionPresenter.Present, which resolves a dozen interpolated keys
// through the package-level tr() seam) AFTER injecting the real translator
// built from the embedded internal/workflow/i18n bundle — the exact
// construction i18nwiring.WireAll performs. Asserts the rendered output
// contains RESOLVED interpolated English text, NOT raw message-ID keys.
package planmode

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	workflowi18n "dev.helix.code/internal/workflow/i18n"
)

func TestPlanmode_PresentOptions_ResolvesRealBundleText(t *testing.T) {
	// Build + inject the real translator exactly as i18nwiring.WireAll does.
	tr, err := workflowi18n.NewTranslator()
	if err != nil {
		t.Fatalf("workflowi18n.NewTranslator() failed: %v", err)
	}
	SetTranslator(tr)
	t.Cleanup(func() { SetTranslator(nil) }) // restore NoopTranslator

	var out bytes.Buffer
	in := strings.NewReader("1\n") // self-driving: auto-select option 1, no human input
	p := NewCLIOptionPresenter(&out, in)

	opts := []*PlanOption{
		{
			ID:          "opt-1",
			Title:       "Incremental refactor",
			Description: "Refactor in small reviewable steps.",
			Pros:        []string{"low risk"},
			Cons:        []string{"slower"},
			Score:       87.5,
			Recommended: true,
			Plan: &Plan{
				Estimates: Estimates{
					Duration:   30 * time.Minute,
					Complexity: ComplexityLow,
					Confidence: 0.9,
				},
			},
		},
	}

	sel, err := p.Present(context.Background(), opts)
	if err != nil {
		t.Fatalf("Present() failed: %v", err)
	}
	if sel == nil || sel.OptionID != "opt-1" {
		t.Fatalf("Present() selection = %+v, want OptionID opt-1", sel)
	}

	rendered := out.String()

	// Assert RESOLVED interpolated text is present (not raw keys).
	wantFragments := []string{
		"Implementation Options",       // options_header
		"Option 1: Incremental refactor", // options_option_label interpolated {{.Index}}/{{.Title}}
		"RECOMMENDED",                  // options_recommended_tag
		"Score: 87.5/100",              // options_score_line interpolated {{.Score}}
		"Pros:",                        // options_pros_heading
		"Cons:",                        // options_cons_heading
		"Confidence: 90%",              // options_confidence_line interpolated {{.Confidence}}
		"Select an option (1-1): ",     // options_select_prompt interpolated {{.Count}}
	}
	for _, frag := range wantFragments {
		if !strings.Contains(rendered, frag) {
			t.Errorf("rendered option output missing resolved fragment %q\n---full output---\n%s", frag, rendered)
		}
	}

	// Anti-bluff: no raw message-ID key may leak into user-facing output.
	if strings.Contains(rendered, "internal_workflow_planmode_") {
		t.Errorf("rendered output leaked raw message-ID key(s):\n%s", rendered)
	}

	t.Logf("RESOLVED planmode option-presenter output:\n%s", rendered)
}
