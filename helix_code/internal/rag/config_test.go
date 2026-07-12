package rag

import (
	"strings"
	"testing"

	"digital.vasic.rag/pkg/retriever"
)

func fakeGetenv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestConfigFromEnv_DefaultsAreDisabled(t *testing.T) {
	cfg := ConfigFromEnv(fakeGetenv(nil))
	if cfg.Enabled {
		t.Fatal("expected RAG to default to disabled (CONST-040 §HXC-118 default-OFF)")
	}
	if cfg.OllamaBaseURL == "" || cfg.EmbedModel == "" || cfg.TopK <= 0 {
		t.Fatalf("expected sane non-empty defaults, got: %+v", cfg)
	}
}

func TestConfigFromEnv_NilGetenvIsDisabled(t *testing.T) {
	cfg := ConfigFromEnv(nil)
	if cfg.Enabled {
		t.Fatal("expected a nil getenv to still resolve to disabled, never panic or enable")
	}
}

func TestConfigFromEnv_ExplicitEnableAndOverrides(t *testing.T) {
	cfg := ConfigFromEnv(fakeGetenv(map[string]string{
		EnvEnabled:    "true",
		EnvOllamaBase: "http://example:12345",
		EnvEmbedModel: "custom-embed",
		EnvTopK:       "7",
	}))
	if !cfg.Enabled {
		t.Fatal("expected explicit true to enable RAG")
	}
	if cfg.OllamaBaseURL != "http://example:12345" {
		t.Fatalf("expected overridden base URL, got %q", cfg.OllamaBaseURL)
	}
	if cfg.EmbedModel != "custom-embed" {
		t.Fatalf("expected overridden embed model, got %q", cfg.EmbedModel)
	}
	if cfg.TopK != 7 {
		t.Fatalf("expected overridden TopK=7, got %d", cfg.TopK)
	}
}

func TestConfigFromEnv_InvalidValuesFallBackToDefaults(t *testing.T) {
	cfg := ConfigFromEnv(fakeGetenv(map[string]string{
		EnvEnabled: "not-a-bool",
		EnvTopK:    "not-a-number",
	}))
	if cfg.Enabled {
		t.Fatal("expected an unparsable Enabled value to fall back to the disabled default")
	}
	if cfg.TopK != 5 {
		t.Fatalf("expected an unparsable TopK to fall back to the default 5, got %d", cfg.TopK)
	}
}

func TestConfigFromEnv_FalseExplicitlyDisables(t *testing.T) {
	cfg := ConfigFromEnv(fakeGetenv(map[string]string{EnvEnabled: "false"}))
	if cfg.Enabled {
		t.Fatal("expected explicit false to stay disabled")
	}
}

func TestNewFromEnv_DisabledByDefault(t *testing.T) {
	adapter := NewFromEnv(fakeGetenv(nil))
	if adapter == nil {
		t.Fatal("expected a non-nil Adapter")
	}
	if adapter.Enabled() {
		t.Fatal("expected NewFromEnv's Adapter to be disabled by default")
	}
}

func TestNewFromEnv_EnabledViaEnv(t *testing.T) {
	adapter := NewFromEnv(fakeGetenv(map[string]string{EnvEnabled: "true"}))
	if !adapter.Enabled() {
		t.Fatal("expected NewFromEnv's Adapter to report enabled when HELIXCODE_RAG_ENABLED=true")
	}
}

func TestPrependContext_NoDocsReturnsPromptUnchanged(t *testing.T) {
	if got := PrependContext("original prompt", nil); got != "original prompt" {
		t.Fatalf("expected prompt unchanged with no docs, got %q", got)
	}
}

func TestPrependContext_WithDocsIncludesRealContentAndOriginalPrompt(t *testing.T) {
	docs := []retriever.Document{{ID: "1", Content: "real retrieved fact", Source: "kb://doc1"}}
	got := PrependContext("what is X?", docs)
	if !strings.Contains(got, "real retrieved fact") {
		t.Fatalf("expected the real retrieved content in the composed prompt, got: %q", got)
	}
	if !strings.Contains(got, "kb://doc1") {
		t.Fatalf("expected the real source citation in the composed prompt, got: %q", got)
	}
	if !strings.Contains(got, "what is X?") {
		t.Fatalf("expected the original prompt preserved verbatim, got: %q", got)
	}
}
