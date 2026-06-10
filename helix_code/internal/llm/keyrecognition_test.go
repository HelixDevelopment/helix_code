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

// krRedMode mirrors the §11.4.115 polarity switch for the funnel-bridge guard.
//
//	RED_MODE=1 (default): assert the bridge correctly drops absent providers
//	            (the gap this bridge closes is that a string-keyed present set
//	            must NOT leak no-key providers into the funnel).
//	RED_MODE=0: standing GREEN regression guard for the same invariant.
func krRedMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "" || v == "1"
}

// TestPresentProviderNames_StringKeyedFunnelInput proves the bridge that feeds
// verifier.GetWorkingModels: present providers map to string keys equal to the
// verifier's Provider field, and absent providers never appear.
func TestPresentProviderNames_StringKeyedFunnelInput(t *testing.T) {
	clearAliasEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-realvalue-1234567890")
	// OPENAI_API_KEY deliberately NOT set — must be absent from the funnel input.

	names := PresentProviderNames()

	if !names["anthropic"] {
		t.Errorf("RED: anthropic key present but missing from string-keyed funnel input %v", names)
	}
	if names["openai"] {
		t.Errorf("RED: openai has NO key yet leaked into the funnel input %v (key-gate breach)", names)
	}
	// The key strings MUST string-equal the verifier's VerifiedModel.Provider
	// values; assert the type-bridge did not corrupt the key.
	if _, ok := names[string(ProviderTypeAnthropic)]; !ok {
		t.Errorf("string(ProviderTypeAnthropic)=%q not found as a funnel key %v",
			string(ProviderTypeAnthropic), names)
	}

	if krRedMode() {
		// RED polarity: prove an absent provider is genuinely excluded — this is
		// the gap (a naive bridge that copied every table entry would leak
		// no-key providers). With the bridge, only key-present survive.
		if len(names) != 1 {
			t.Fatalf("RED expected exactly 1 present provider (anthropic), got %d: %v", len(names), names)
		}
		return
	}
	// GREEN: same invariant, standing guard.
	if len(names) != 1 {
		t.Fatalf("GREEN: expected exactly 1 present provider, got %d: %v", len(names), names)
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
