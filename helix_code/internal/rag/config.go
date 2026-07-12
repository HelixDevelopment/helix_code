// Env-driven configuration + wiring for CONST-040 §HXC-118 Phase 2/3.
//
// The whole feature is default-OFF: unless HELIXCODE_RAG_ENABLED is
// explicitly set to a truthy value, ConfigFromEnv resolves Enabled=false
// and NewFromEnv's Adapter reports Enabled()==false, so a caller that
// always constructs via NewFromEnv but never sets the env var gets an
// Adapter whose Retrieve is a documented no-op (Adapter.Retrieve,
// internal/rag/adapter.go, Phase 1) — no HTTP call, no behavior change
// versus a codebase that never imported this package at all.
package rag

import (
	"fmt"
	"strconv"
	"strings"

	"digital.vasic.rag/pkg/retriever"
)

// Environment variable names controlling RAG retrieval (CONST-046 note:
// these are configuration keys, not user-facing content).
const (
	// EnvEnabled turns RAG retrieval on. Must be an explicit truthy value
	// ("1", "true", "TRUE", ...) per strconv.ParseBool; unset, empty, or
	// unparsable => disabled (the safe default).
	EnvEnabled = "HELIXCODE_RAG_ENABLED"
	// EnvOllamaBase overrides the Ollama base URL used for real embedding
	// calls. Defaults to "http://localhost:11434".
	EnvOllamaBase = "HELIXCODE_RAG_OLLAMA_BASE_URL"
	// EnvEmbedModel overrides the Ollama embeddings model. Defaults to
	// "nomic-embed-text".
	EnvEmbedModel = "HELIXCODE_RAG_EMBED_MODEL"
	// EnvTopK overrides the number of documents retrieved per query.
	// Defaults to 5.
	EnvTopK = "HELIXCODE_RAG_TOP_K"
)

// Config is the resolved, environment-sourced RAG configuration.
type Config struct {
	Enabled       bool
	OllamaBaseURL string
	EmbedModel    string
	TopK          int
}

// ConfigFromEnv resolves Config from environment variables via the
// supplied getenv function (dependency-injected so tests never touch
// process-global env — the same pattern already used elsewhere in this
// codebase, e.g. theme.DefaultThemePath(os.Getenv) in cmd/cli/main.go).
// A nil getenv resolves to the all-defaults (disabled) Config rather than
// panicking.
func ConfigFromEnv(getenv func(string) string) Config {
	cfg := Config{
		Enabled:       false,
		OllamaBaseURL: "http://localhost:11434",
		EmbedModel:    "nomic-embed-text",
		TopK:          5,
	}
	if getenv == nil {
		return cfg
	}
	if v := strings.TrimSpace(getenv(EnvEnabled)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Enabled = b
		}
		// An unparsable value is left at the safe default (false) rather
		// than erroring — CONST-040 Phase 2/3 default-OFF posture.
	}
	if v := strings.TrimSpace(getenv(EnvOllamaBase)); v != "" {
		cfg.OllamaBaseURL = v
	}
	if v := strings.TrimSpace(getenv(EnvEmbedModel)); v != "" {
		cfg.EmbedModel = v
	}
	if v := strings.TrimSpace(getenv(EnvTopK)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.TopK = n
		}
	}
	return cfg
}

// NewFromEnv builds a fully-wired Adapter (a real OllamaEmbedder over a
// fresh, empty InMemoryVectorStore) from environment configuration. The
// returned Adapter's Enabled() mirrors Config.Enabled — default false.
// Construction performs no network I/O; the real Ollama call only happens
// inside Retrieve, and only when the adapter is enabled (Adapter.Retrieve,
// Phase 1, never calls the underlying retriever while disabled).
func NewFromEnv(getenv func(string) string) *Adapter {
	cfg := ConfigFromEnv(getenv)
	embedder := NewOllamaEmbedder(cfg.OllamaBaseURL, cfg.EmbedModel)
	store := NewInMemoryVectorStore(embedder)
	adapter := NewAdapter(store)
	adapter.SetEnabled(cfg.Enabled)
	return adapter
}

// RetrieveOptionsFromEnv resolves retriever.Options (currently just TopK)
// from environment configuration, so cmd/cli callers do not need to
// re-derive the whole Config just to build a retrieval request.
func RetrieveOptionsFromEnv(getenv func(string) string) retriever.Options {
	opts := retriever.DefaultOptions()
	opts.TopK = ConfigFromEnv(getenv).TopK
	return opts
}

// PrependContext formats retrieved documents as a labelled context block
// and prepends it to the original prompt — the same "labelled section +
// verbatim original prompt" shape MCP tool-result injection and
// LSP-diagnostic injection already use elsewhere in cmd/cli (design doc
// §3.4), so the LLM sees the real retrieved content verbatim, never a
// fabricated summary of it. Returns prompt unchanged when there is no
// retrieved context (nothing to prepend).
//
// Honest scope note: the section header below is a literal, not yet
// routed through the CONST-046 i18n mechanism (cmd/cli's tr() helper is
// not reachable from this package without a wider import than Phase 2/3's
// declared scope permits). This is a known, tracked gap for a follow-up
// pass, not a silent omission.
func PrependContext(prompt string, docs []retriever.Document) string {
	if len(docs) == 0 {
		return prompt
	}
	var b strings.Builder
	b.WriteString("Context retrieved from the local knowledge base:\n\n")
	for i, d := range docs {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, d.Content)
		if d.Source != "" {
			fmt.Fprintf(&b, "(source: %s)\n", d.Source)
		}
		b.WriteString("\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(prompt)
	return b.String()
}
