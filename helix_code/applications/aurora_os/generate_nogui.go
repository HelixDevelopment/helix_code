//go:build nogui

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"dev.helix.code/internal/llm"
)

// Generate sends a prompt to the configured LLM provider and returns the
// generated text. The signature uses only simple, binding-friendly types
// (string in, string + error out), mirroring the proven mobile gomobile core
// (shared/mobile_core/mobile.go) so Aurora OS reaches LLM generation through
// the SAME canonical path as every other HelixCode client.
//
// Anti-bluff (BLUFF-001 / CONST-035 / CONST-036): this returns the provider's
// genuine output, never a canned/fabricated string. It resolves a REAL
// llm.Provider via the same path cmd/cli and mobile_core use
// (llm.Select -> llm.NewCloudProvider, falling back to a local Ollama
// provider) and makes a REAL provider.Generate call. When no provider is
// reachable (e.g. Ollama not running and no cloud credentials configured),
// the underlying provider returns a real transport error which is surfaced
// here verbatim — never swallowed into a fake success.
func (cliApp *CLIApp) Generate(prompt string) (string, error) {
	return cliApp.generateInternal(context.Background(), prompt)
}

// generateInternal performs the real LLM generation. It resolves a real
// llm.Provider, builds a real *llm.LLMRequest carrying the prompt as a user
// message, and calls provider.Generate. The returned text is the provider's
// actual response content.
func (cliApp *CLIApp) generateInternal(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("generate: prompt must not be empty")
	}

	provider := buildAuroraLLMProvider(ctx)
	if provider == nil {
		return "", fmt.Errorf("generate: no LLM provider available (cloud unconfigured and local Ollama unreachable)")
	}

	req := &llm.LLMRequest{
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := provider.Generate(ctx, req)
	if err != nil {
		return "", fmt.Errorf("generate: provider call failed: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("generate: provider returned nil response")
	}
	return resp.Content, nil
}

// buildAuroraLLMProvider resolves a real cloud provider from the environment,
// falling back to a local Ollama provider. Returns nil only when no provider
// at all could be constructed (the caller turns that into an explicit error).
//
// This mirrors mobile_core's buildMobileLLMProvider: it resolves the cloud
// provider type from the HELIX_LLM_PROVIDER environment variable via
// llm.Select, constructs it via llm.NewCloudProvider, and falls back to a
// local Ollama provider on the standard port when no cloud provider is
// configured or its construction fails.
//
// Anti-bluff: this never returns a stub/fake provider.
func buildAuroraLLMProvider(_ context.Context) llm.Provider {
	selectorInput := llm.SelectorInput{
		Env: os.Getenv("HELIX_LLM_PROVIDER"),
	}
	ptype, selErr := llm.Select(selectorInput)
	switch {
	case errors.Is(selErr, llm.ErrNoProviderConfigured):
		// No cloud provider configured — fall through to the local default.
	case selErr != nil:
		// Unknown provider name — log and fall back rather than aborting.
		log.Printf("aurora_os: provider selector error: %v (falling back to local default)", selErr)
	default:
		entry := llm.ProviderConfigEntry{Type: ptype}
		cloud, cErr := llm.NewCloudProvider(ptype, entry)
		if cErr == nil && cloud != nil {
			return cloud
		}
		log.Printf("aurora_os: failed to construct cloud provider %q (%v); falling back to local default", ptype, cErr)
	}

	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{
		DefaultModel: "llama3.2",
		BaseURL:      "http://localhost:11434",
	})
	if err != nil {
		log.Printf("aurora_os: default Ollama provider construction failed: %v", err)
		return nil
	}
	return provider
}
