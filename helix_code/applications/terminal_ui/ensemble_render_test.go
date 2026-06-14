package main

import (
	"strings"
	"testing"
)

// joinLines is a small helper so substring assertions can scan the whole panel.
func joinLines(lines []string) string { return strings.Join(lines, "\n") }

// TestFormatEnsemblePanel_RendersEveryMember is the ANTI-BLUFF CORE: it proves
// the operator literally SEES every ensemble member's name + excerpt + score and
// the selected winner — the exact bug ("we have not seen multiple ensemble
// members responses in action"). A single member rendered would NOT satisfy
// this; we assert >=2 distinct member names appear.
func TestFormatEnsemblePanel_RendersEveryMember(t *testing.T) {
	meta := map[string]interface{}{
		"ensemble":                      true,
		"ensemble_strategy":             "verifier-weighted",
		"ensemble_total_providers":      4,
		"ensemble_successful_providers": 4,
		"ensemble_failed_providers":     0,
		"ensemble_participants":         []string{"DeepSeek", "Groq", "Mistral", "OpenRouter"},
		"ensemble_selected_provider":    "Groq",
		"ensemble_scores": map[string]float64{
			"DeepSeek":   0.71,
			"Groq":       0.94,
			"Mistral":    0.66,
			"OpenRouter": 0.80,
		},
		"ensemble_excerpts": map[string]string{
			"DeepSeek":   "DeepSeek says the answer is forty-two.",
			"Groq":       "Groq concludes the result is 42 precisely.",
			"Mistral":    "Mistral reasons it is about 42.",
			"OpenRouter": "OpenRouter routes to a 42 consensus.",
		},
	}

	lines := FormatEnsemblePanel(meta)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty panel for an ensemble response, got empty slice")
	}
	blob := joinLines(lines)

	participants := []string{"DeepSeek", "Groq", "Mistral", "OpenRouter"}
	seen := 0
	for _, p := range participants {
		if strings.Contains(blob, p) {
			seen++
		} else {
			t.Errorf("panel does not contain participant name %q\npanel:\n%s", p, blob)
		}
	}
	if seen < 2 {
		t.Fatalf("anti-bluff: expected >=2 distinct member names visible, saw %d\npanel:\n%s", seen, blob)
	}

	// Every excerpt substring must be visible (operator SEES each member's answer).
	excerpts := []string{
		"DeepSeek says the answer is forty-two.",
		"Groq concludes the result is 42 precisely.",
		"Mistral reasons it is about 42.",
		"OpenRouter routes to a 42 consensus.",
	}
	for _, ex := range excerpts {
		if !strings.Contains(blob, ex) {
			t.Errorf("panel does not contain excerpt %q\npanel:\n%s", ex, blob)
		}
	}

	// Each score must be visible.
	for _, s := range []string{"0.94", "0.71", "0.66", "0.80"} {
		if !strings.Contains(blob, s) {
			t.Errorf("panel does not contain score %q\npanel:\n%s", s, blob)
		}
	}

	// Winner must be clearly marked on Groq.
	if !strings.Contains(strings.ToLower(blob), "winner") {
		t.Errorf("panel does not mark a winner\npanel:\n%s", blob)
	}
	foundWinnerOnGroq := false
	for _, ln := range lines {
		if strings.Contains(strings.ToLower(ln), "winner") && strings.Contains(ln, "Groq") {
			foundWinnerOnGroq = true
			break
		}
	}
	if !foundWinnerOnGroq {
		t.Errorf("winner marker is not on the Groq line\npanel:\n%s", blob)
	}

	// Success/total count must be visible.
	if !strings.Contains(blob, "4/4") {
		t.Errorf("panel does not show success/total count 4/4\npanel:\n%s", blob)
	}
}

// TestFormatEnsemblePanel_RendersPerMemberModel is the per-member MODEL
// visibility proof: each member line MUST show the model it served (chosen via
// LLMsVerifier) AND the "(via LLMsVerifier)" provenance AND that member's own
// excerpt — for >=2 members. This is the TUI half of the "operator sees which
// model each ensemble member used" feature.
func TestFormatEnsemblePanel_RendersPerMemberModel(t *testing.T) {
	meta := map[string]interface{}{
		"ensemble":                      true,
		"ensemble_strategy":             "verifier-weighted",
		"ensemble_total_providers":      3,
		"ensemble_successful_providers": 3,
		"ensemble_participants":         []string{"DeepSeek", "Groq", "Mistral"},
		"ensemble_selected_provider":    "Groq",
		"ensemble_scores": map[string]float64{
			"DeepSeek": 0.71, "Groq": 0.94, "Mistral": 0.66,
		},
		"ensemble_excerpts": map[string]string{
			"DeepSeek": "DeepSeek says the answer is forty-two.",
			"Groq":     "Groq concludes the result is 42 precisely.",
			"Mistral":  "Mistral reasons it is about 42.",
		},
		"ensemble_models": map[string]string{
			"DeepSeek": "deepseek-chat",
			"Groq":     "llama-3.3-70b-versatile",
			"Mistral":  "mistral-small-latest",
		},
	}

	lines := FormatEnsemblePanel(meta)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty panel, got empty slice")
	}
	blob := joinLines(lines)

	// Each member's model id MUST be visible AND on the SAME line as its name,
	// annotated with the LLMsVerifier provenance.
	cases := []struct{ name, model, excerpt string }{
		{"DeepSeek", "deepseek-chat", "DeepSeek says the answer is forty-two."},
		{"Groq", "llama-3.3-70b-versatile", "Groq concludes the result is 42 precisely."},
		{"Mistral", "mistral-small-latest", "Mistral reasons it is about 42."},
	}
	shown := 0
	for _, c := range cases {
		var memberLine string
		for _, ln := range lines {
			if strings.Contains(ln, c.name) && strings.Contains(ln, c.model) {
				memberLine = ln
				break
			}
		}
		if memberLine == "" {
			t.Errorf("no line shows member %q WITH its model %q\npanel:\n%s", c.name, c.model, blob)
			continue
		}
		if !strings.Contains(memberLine, "(via LLMsVerifier)") {
			t.Errorf("member %q line missing LLMsVerifier provenance: %q", c.name, memberLine)
		}
		if !strings.Contains(blob, c.excerpt) {
			t.Errorf("panel missing excerpt for %q: %q\npanel:\n%s", c.name, c.excerpt, blob)
		}
		shown++
	}
	if shown < 2 {
		t.Fatalf("anti-bluff: expected >=2 members showing their model, saw %d\npanel:\n%s", shown, blob)
	}
}

// TestFormatEnsemblePanel_DefensiveFloat64 proves numeric counts arriving as
// float64 (the JSON / interface{}-channel path) are handled, not just int.
func TestFormatEnsemblePanel_DefensiveFloat64(t *testing.T) {
	meta := map[string]interface{}{
		"ensemble":                      true,
		"ensemble_total_providers":      float64(3),
		"ensemble_successful_providers": float64(2),
		"ensemble_participants":         []string{"Groq", "Mistral"},
		"ensemble_selected_provider":    "Groq",
		"ensemble_scores":               map[string]float64{"Groq": 0.9, "Mistral": 0.5},
		"ensemble_excerpts":             map[string]string{"Groq": "g-answer", "Mistral": "m-answer"},
	}
	lines := FormatEnsemblePanel(meta)
	blob := joinLines(lines)
	if !strings.Contains(blob, "2/3") {
		t.Errorf("float64 counts not rendered as 2/3\npanel:\n%s", blob)
	}
	if !strings.Contains(blob, "g-answer") || !strings.Contains(blob, "m-answer") {
		t.Errorf("excerpts missing under float64 path\npanel:\n%s", blob)
	}
}

// TestFormatEnsemblePanel_NonEnsemble proves a normal-provider response (no
// "ensemble"==true) renders nothing special — caller gets an empty slice.
func TestFormatEnsemblePanel_NonEnsemble(t *testing.T) {
	if got := FormatEnsemblePanel(nil); len(got) != 0 {
		t.Errorf("nil meta: expected empty slice, got %v", got)
	}
	if got := FormatEnsemblePanel(map[string]interface{}{"ensemble": false}); len(got) != 0 {
		t.Errorf("ensemble=false: expected empty slice, got %v", got)
	}
	if got := FormatEnsemblePanel(map[string]interface{}{"foo": "bar"}); len(got) != 0 {
		t.Errorf("missing ensemble key: expected empty slice, got %v", got)
	}
}

// TestFormatToolTrace_RendersToolAndOutput proves an agentic tool-call trace is
// rendered with the tool name and the real command output visible.
func TestFormatToolTrace_RendersToolAndOutput(t *testing.T) {
	entries := []ToolTraceLine{
		{
			ToolName:  "shell",
			Output:    "On branch main\nnothing to commit, working tree clean",
			Arguments: map[string]interface{}{"command": "git status"},
		},
	}
	lines := FormatToolTrace(entries)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty tool-trace lines, got empty slice")
	}
	blob := joinLines(lines)
	if !strings.Contains(blob, "shell") {
		t.Errorf("tool-trace does not contain tool name 'shell'\ntrace:\n%s", blob)
	}
	if !strings.Contains(blob, "On branch main") {
		t.Errorf("tool-trace does not contain the command output\ntrace:\n%s", blob)
	}
	if !strings.Contains(blob, "git status") {
		t.Errorf("tool-trace does not contain the argument 'git status'\ntrace:\n%s", blob)
	}
}

// TestFormatToolTrace_RendersError proves a failing tool surfaces its error.
func TestFormatToolTrace_RendersError(t *testing.T) {
	entries := []ToolTraceLine{
		{ToolName: "read_file", Err: "no such file or directory", Arguments: map[string]interface{}{"path": "/nope"}},
	}
	lines := FormatToolTrace(entries)
	blob := joinLines(lines)
	if !strings.Contains(blob, "read_file") {
		t.Errorf("missing tool name\ntrace:\n%s", blob)
	}
	if !strings.Contains(blob, "no such file or directory") {
		t.Errorf("missing error text\ntrace:\n%s", blob)
	}
}

// TestFormatToolTrace_Empty proves an empty trace yields an empty slice.
func TestFormatToolTrace_Empty(t *testing.T) {
	if got := FormatToolTrace(nil); len(got) != 0 {
		t.Errorf("nil entries: expected empty slice, got %v", got)
	}
}
