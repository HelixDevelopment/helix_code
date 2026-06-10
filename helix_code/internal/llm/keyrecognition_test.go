package llm

import (
	"os"
	"testing"
)

// RED_MODE polarity switch (§11.4.115). RED_MODE=1 reproduces the gap on the
// pre-impl tree (no keyrecognition.go → table absent / single-alias). RED_MODE=0
// is the standing GREEN regression guard asserting the multi-alias recognition
// table is present and correct.
//
// These are UNIT tests (mocks/env-fixtures permitted here only — CONST-050(A)).

// TestKeyRecognition_MultiAliasTable_T112 asserts the decoupled multi-alias
// provider→env-var table exists and recognises every documented alias.
func TestKeyRecognition_MultiAliasTable_T112(t *testing.T) {
	tbl := ProviderEnvAliases()
	if len(tbl) == 0 {
		t.Fatalf("ProviderEnvAliases() returned empty table — key-recognition table absent (CONST-036/T1.1.2 RED)")
	}

	// Every provider MUST carry at least one alias; the multi-alias providers
	// (the whole point of T1.1.2 vs config.go's single-alias bindings) MUST
	// carry their secondary aliases too.
	wantSecondary := map[ProviderType][]string{
		ProviderTypeAnthropic: {"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"},
		ProviderTypeGemini:    {"GEMINI_API_KEY", "GOOGLE_API_KEY"},
		ProviderTypeDeepSeek:  {"DEEPSEEK_API_KEY"},
		ProviderTypeMistral:   {"MISTRAL_API_KEY"},
		ProviderTypeQwen:      {"QWEN_API_KEY", "DASHSCOPE_API_KEY"},
	}
	for pt, wants := range wantSecondary {
		aliases, ok := tbl[pt]
		if !ok {
			t.Errorf("provider %q missing from alias table", pt)
			continue
		}
		for _, w := range wants {
			if !krContains(aliases, w) {
				t.Errorf("provider %q: alias %q not present in %v", pt, w, aliases)
			}
		}
	}
}

func TestKeyRecognition_PresentProviders_AliasMatch_T112(t *testing.T) {
	// Clear every alias for the providers under test so the fixture is hermetic.
	clearAliasEnv(t)

	// Set ONLY the secondary alias for Anthropic — the single-alias config.go
	// binding would miss CLAUDE_API_KEY; the multi-alias table MUST catch it.
	t.Setenv("CLAUDE_API_KEY", "sk-ant-realvalue-1234567890")

	present := PresentProviders()
	if !present[ProviderTypeAnthropic] {
		t.Errorf("Anthropic not recognised via secondary alias CLAUDE_API_KEY (RED — multi-alias recognition missing)")
	}
	if present[ProviderTypeOpenAI] {
		t.Errorf("OpenAI must NOT be present (no key set) — key-presence gate leaked")
	}
}

func TestKeyRecognition_PresentProviders_AbsenceAndPlaceholder_T112(t *testing.T) {
	clearAliasEnv(t)

	// Placeholder value MUST NOT count as a present key (§11.4 anti-bluff —
	// a placeholder is not a working key).
	t.Setenv("OPENAI_API_KEY", "your-api-key")
	t.Setenv("MISTRAL_API_KEY", "   ") // whitespace-only also not present

	present := PresentProviders()
	if present[ProviderTypeOpenAI] {
		t.Errorf("OpenAI recognised from placeholder value — isPlaceholder gate failed")
	}
	if present[ProviderTypeMistral] {
		t.Errorf("Mistral recognised from whitespace-only value — blank gate failed")
	}
}

func TestKeyRecognition_IsPlaceholder_T112(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"sk-ant-realvalue-abc123", false},
		{"your-api-key", true},
		{"PLACEHOLDER", true},
		{"sk-xxx", true},
		{"", true},
		{"   ", true},
		{"api_key_here", true},
		{"INSERT_KEY", true},
		{"gsk_realgroqkey9876543210", false},
	}
	for _, c := range cases {
		if got := isPlaceholderKey(c.v); got != c.want {
			t.Errorf("isPlaceholderKey(%q) = %v, want %v", c.v, got, c.want)
		}
	}
}

// --- helpers ---

func krContains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// clearAliasEnv unsets every alias across the recognition table so each test
// starts from a known-empty environment. Uses t.Setenv-style restore by saving
// and deferring restore for variables that were set in the real environment.
func clearAliasEnv(t *testing.T) {
	t.Helper()
	for _, aliases := range ProviderEnvAliases() {
		for _, a := range aliases {
			if _, ok := os.LookupEnv(a); ok {
				t.Setenv(a, "")
			}
		}
	}
}
