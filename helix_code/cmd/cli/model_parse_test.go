package main

import "testing"

// TestParseModelSpec is the standing regression guard for HXC-096 (§11.4.135 /
// §11.4.115). The historical defect: handleGenerate stripped everything before
// the FIRST ":" unconditionally, mangling the Ollama native model tag
// "qwen2.5:3b" into "3b", which Ollama's /api/chat rejects with a 404. The fix
// (parseModelSpec / isKnownProviderPrefix) strips a leading "provider:" segment
// ONLY when it names a recognized provider type, keeping real "name:tag" model
// ids intact.
//
// The "qwen2.5:3b" → "qwen2.5:3b" case is the non-tautological RED-catcher:
// under the old split-on-first-colon code it would yield "3b" and FAIL here.
func TestParseModelSpec(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantProvider string
		wantModel    string
	}{
		{
			// HXC-096 bug case: bare Ollama name:tag, "qwen2.5" is NOT a
			// provider, so nothing is stripped — model stays intact.
			name:         "ollama_native_tag_qwen_kept_intact",
			input:        "qwen2.5:3b",
			wantProvider: "",
			wantModel:    "qwen2.5:3b",
		},
		{
			// Leading recognized provider stripped, the remaining "name:tag"
			// (itself containing a colon) is preserved.
			name:         "ollama_provider_prefix_stripped_tag_preserved",
			input:        "ollama:qwen2.5:3b",
			wantProvider: "ollama",
			wantModel:    "qwen2.5:3b",
		},
		{
			// Another bare Ollama name:tag — "llama3.2" is not a provider.
			name:         "ollama_native_tag_llama_kept_intact",
			input:        "llama3.2:1b",
			wantProvider: "",
			wantModel:    "llama3.2:1b",
		},
		{
			// Recognized provider prefix stripped, model name kept.
			name:         "anthropic_provider_prefix_stripped",
			input:        "anthropic:claude-3-5-sonnet",
			wantProvider: "anthropic",
			wantModel:    "claude-3-5-sonnet",
		},
		{
			// No colon at all — model returned unchanged.
			name:         "no_colon_plain_model",
			input:        "gpt-4o",
			wantProvider: "",
			wantModel:    "gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, gotModel := parseModelSpec(tt.input)
			if gotProvider != tt.wantProvider {
				t.Errorf("parseModelSpec(%q) provider = %q, want %q", tt.input, gotProvider, tt.wantProvider)
			}
			if gotModel != tt.wantModel {
				t.Errorf("parseModelSpec(%q) model = %q, want %q", tt.input, gotModel, tt.wantModel)
			}
		})
	}
}

// TestIsKnownProviderPrefix asserts the provider gate that distinguishes a real
// "provider:model" prefix from an Ollama "name:tag" model id. The false cases
// (qwen2.5, llama3.2) are the crux of HXC-096: if either were ever classified
// as a provider, the bug returns.
func TestIsKnownProviderPrefix(t *testing.T) {
	known := []string{
		"ollama", "openai", "anthropic", "gemini", "groq",
		"mistral", "deepseek", "openrouter", "xai", "local",
	}
	for _, p := range known {
		if !isKnownProviderPrefix(p) {
			t.Errorf("isKnownProviderPrefix(%q) = false, want true", p)
		}
	}
	// Case-insensitivity sanity check.
	if !isKnownProviderPrefix("Ollama") {
		t.Errorf("isKnownProviderPrefix(%q) = false, want true (case-insensitive)", "Ollama")
	}

	unknown := []string{"qwen2.5", "llama3.2", "gpt-4o", ""}
	for _, u := range unknown {
		if isKnownProviderPrefix(u) {
			t.Errorf("isKnownProviderPrefix(%q) = true, want false", u)
		}
	}
}
