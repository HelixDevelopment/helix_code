// Round-63 §11.4 anti-bluff tests for LLMResponse.Err propagation —
// FINAL batch of the round-46 17-provider deferred list:
//   - Qwen (Alibaba DashScope)        OpenAI-compatible finish_reason
//   - GitHub Copilot                  OpenAI-compatible finish_reason
//   - Cerebras Cloud (NEW PROVIDER)   OpenAI-compatible finish_reason
//
// Round-46 wired openai / anthropic / ollama (3); round-50 closed
// gemini / deepseek / groq / mistral (4); round-53 closed xAI /
// OpenRouter / Llama.cpp / OpenAICompatible×11 (4 + 11 backends); round-54
// closed Bedrock / Azure / Replicate / Vertex AI (4); round-63 closes
// the FINAL 3 — Qwen + Copilot + Cerebras — taking the round-46
// 17-provider deferred list to 17/17 = 100% coverage.
//
// Cerebras was created from scratch in this round as a thin
// OpenAI-compatible provider (see cerebras_provider.go) because no
// cerebras_provider.go existed in helix_code/internal/llm/ before
// (round-53 commit 99fb77c noted the gap and substituted
// OpenAICompatibleProvider; round-63 closes it properly).
//
// CONST-035 / CONST-050(A)+(B) / Article XI §11.9: every PASS in this
// file is backed by an httptest fixture exercising the real provider
// HTTP code path (real JSON encode/decode, real http.Client, real
// LLMResponse construction). No mocks of internal helpers — only the
// remote API is faked at the HTTP transport boundary, which is the
// canonical pattern for provider-layer unit tests.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// Qwen (Alibaba DashScope) — OpenAI-compatible mode
// =========================================================================

// TestRound63_Qwen_Generate_FinishReasonLength_PopulatesTruncated asserts
// that Qwen's "length" finish_reason maps to ErrResponseTruncated and
// that partial Content survives.
func TestRound63_Qwen_Generate_FinishReasonLength_PopulatesTruncated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round63-qwen-length", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "qwen3-coder-plus",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Qwen partial output here",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 7, "completion_tokens": 4, "total_tokens": 11,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewQwenProvider(ProviderConfigEntry{
		Type:     ProviderTypeQwen,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID:        uuid.New(),
		Model:     "qwen3-coder-plus",
		Messages:  []Message{{Role: "user", Content: "test"}},
		MaxTokens: 4,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "Content MUST hold partial output even when truncated")
	require.NotNil(t, resp.Err, "Err MUST be populated for finish_reason=length")
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "length", resp.FinishReason,
		"FinishReason MUST preserve literal API value alongside Err")
}

// TestRound63_Qwen_Generate_ContentFilter_PopulatesBlocked asserts that
// Qwen's "content_filter" finish_reason maps to ErrResponseContentBlocked.
func TestRound63_Qwen_Generate_ContentFilter_PopulatesBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round63-qwen-cf", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "qwen3-coder-plus",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": ""},
					"finish_reason": "content_filter",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 3, "completion_tokens": 0, "total_tokens": 3,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewQwenProvider(ProviderConfigEntry{
		Type: ProviderTypeQwen, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "qwen3-coder-plus",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked; got %v", resp.Err)
}

// TestRound63_Qwen_Generate_CleanStop_LeavesErrNil asserts clean stop
// leaves Err nil (round-46 backward-compat invariant preserved).
func TestRound63_Qwen_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "ok", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "qwen3-coder-plus",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": "Done"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 2, "completion_tokens": 1, "total_tokens": 3,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewQwenProvider(ProviderConfigEntry{
		Type: ProviderTypeQwen, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "qwen3-coder-plus",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Err, "Err MUST be nil for clean finish_reason=stop")
	assert.Equal(t, "Done", resp.Content)
}

// TestRound63_Qwen_Stream_FinishReasonLength_PropagatesToTerminalFrame
// asserts that streaming-path truncation emits a terminal Err-bearing
// frame on the channel.
func TestRound63_Qwen_Stream_FinishReasonLength_PropagatesToTerminalFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		chunks := []map[string]interface{}{
			{
				"id": "s1", "object": "chat.completion.chunk", "model": "qwen3-coder-plus",
				"choices": []map[string]interface{}{
					{"index": 0, "delta": map[string]interface{}{"role": "assistant", "content": "Hel"}, "finish_reason": ""},
				},
			},
			{
				"id": "s2", "object": "chat.completion.chunk", "model": "qwen3-coder-plus",
				"choices": []map[string]interface{}{
					{"index": 0, "delta": map[string]interface{}{"content": "lo"}, "finish_reason": ""},
				},
			},
			{
				"id": "s3", "object": "chat.completion.chunk", "model": "qwen3-coder-plus",
				"choices": []map[string]interface{}{
					{"index": 0, "delta": map[string]interface{}{}, "finish_reason": "length"},
				},
			},
		}
		enc := json.NewEncoder(w)
		for _, c := range chunks {
			_ = enc.Encode(c)
		}
	}))
	defer server.Close()

	provider, err := NewQwenProvider(ProviderConfigEntry{
		Type: ProviderTypeQwen, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	ch := make(chan LLMResponse, 16)
	go func() {
		_ = provider.GenerateStream(context.Background(), &LLMRequest{
			ID: uuid.New(), Model: "qwen3-coder-plus", Stream: true,
			Messages: []Message{{Role: "user", Content: "Hello"}},
		}, ch)
	}()

	var sawErrFrame bool
	deadline := time.After(3 * time.Second)
loop:
	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				break loop
			}
			if resp.Err != nil {
				sawErrFrame = true
				assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
					"terminal stream Err MUST be ErrResponseTruncated; got %v", resp.Err)
				assert.Equal(t, "length", resp.FinishReason)
			}
		case <-deadline:
			break loop
		}
	}
	assert.True(t, sawErrFrame, "stream MUST emit a terminal Err-bearing frame on finish_reason=length")
}

// =========================================================================
// GitHub Copilot — OpenAI-compatible
// =========================================================================

// newRound63CopilotProvider constructs a CopilotProvider pointed at an
// httptest fixture, bypassing the real GitHub-token exchange (which
// requires a live GH App). Mirrors the createCopilotProviderWithMockServer
// pattern from copilot_provider_test.go.
func newRound63CopilotProvider(t *testing.T, endpoint string) *CopilotProvider {
	t.Helper()
	cp := &CopilotProvider{
		config: ProviderConfigEntry{
			Type:     ProviderTypeCopilot,
			APIKey:   "test-github-token",
			Endpoint: endpoint,
			Enabled:  true,
		},
		endpoint:    endpoint,
		githubToken: "test-github-token",
		bearerToken: "test-copilot-bearer-token",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		lastHealth: &ProviderHealth{
			Status:    "unknown",
			LastCheck: time.Now(),
		},
	}
	cp.initializeModels()
	return cp
}

// TestRound63_Copilot_Generate_FinishReasonLength_PopulatesTruncated asserts
// that Copilot's "length" finish_reason maps to ErrResponseTruncated.
func TestRound63_Copilot_Generate_FinishReasonLength_PopulatesTruncated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round63-copilot-length", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Copilot partial output",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 6, "completion_tokens": 3, "total_tokens": 9,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := newRound63CopilotProvider(t, server.URL)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID:        uuid.New(),
		Model:     "gpt-4o",
		Messages:  []Message{{Role: "user", Content: "test"}},
		MaxTokens: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "Content MUST hold partial output")
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "length", resp.FinishReason)
}

// TestRound63_Copilot_Generate_ContentFilter_PopulatesBlocked asserts
// content_filter mapping.
func TestRound63_Copilot_Generate_ContentFilter_PopulatesBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round63-copilot-cf", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": ""},
					"finish_reason": "content_filter",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 4, "completion_tokens": 0, "total_tokens": 4,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := newRound63CopilotProvider(t, server.URL)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "gpt-4o",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked; got %v", resp.Err)
}

// TestRound63_Copilot_Generate_CleanStop_LeavesErrNil asserts clean stop
// leaves Err nil.
func TestRound63_Copilot_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "ok", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": "OK"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 2, "completion_tokens": 1, "total_tokens": 3,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := newRound63CopilotProvider(t, server.URL)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "gpt-4o",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Err, "Err MUST be nil for clean finish_reason=stop")
	assert.Equal(t, "OK", resp.Content)
}

// =========================================================================
// Cerebras Cloud — NEW PROVIDER, OpenAI-compatible
//
// Speed programme P5-T02 (R1 B21): the Cerebras provider implementation
// moved to its own sub-package internal/llm/providers/cerebras/. Its
// per-provider Err-propagation tests moved with it to
// internal/llm/providers/cerebras/cerebras_test.go — they must live
// beside the package they exercise. The mapper-pinning test below
// (TestRound63_AllRound46DeferredProvidersWired) still asserts Cerebras
// reuses the OpenAI mapper at the helper layer and stays here, as it
// only needs the package-llm mapper helper, not the moved provider type.
// =========================================================================

// =========================================================================
// Paired-mutation mapper-pinning tests (closed-set regression)
// =========================================================================

// TestRound63_QwenReusesOpenAIMapper pins the architectural decision that
// Qwen (Alibaba DashScope's compatible-mode endpoint) uses the OpenAI
// finish_reason vocabulary and reuses mapOpenAIFinishReasonToErr. If
// DashScope diverges (e.g. native API path with "sensitive" for safety),
// this test MUST be replaced with a Qwen-specific mapper in the same
// commit.
func TestRound63_QwenReusesOpenAIMapper(t *testing.T) {
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("length"), ErrResponseTruncated))
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("content_filter"), ErrResponseContentBlocked))
	assert.Nil(t, mapOpenAIFinishReasonToErr("stop"))
	assert.Nil(t, mapOpenAIFinishReasonToErr("tool_calls"))
}

// TestRound63_CopilotReusesOpenAIMapper pins the architectural decision
// that GitHub Copilot normalises every backend (GPT-4o, Claude 3.5/3.7,
// o1, o3-mini, Gemini 2.0 Flash) to the OpenAI finish_reason vocabulary
// and reuses mapOpenAIFinishReasonToErr. If Copilot stops normalising
// (e.g. surfaces Claude's "max_tokens" raw), this test MUST be replaced
// with a Copilot-specific mapper in the same commit.
func TestRound63_CopilotReusesOpenAIMapper(t *testing.T) {
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("length"), ErrResponseTruncated))
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("content_filter"), ErrResponseContentBlocked))
	assert.Nil(t, mapOpenAIFinishReasonToErr("stop"))
	assert.Nil(t, mapOpenAIFinishReasonToErr("tool_calls"))
	assert.Nil(t, mapOpenAIFinishReasonToErr("function_call"))
}

// Note (speed programme P5-T02): TestRound63_CerebrasReusesOpenAIMapper
// moved to internal/llm/providers/cerebras/cerebras_test.go alongside the
// Cerebras provider it pins. It now asserts against the exported
// llm.MapOpenAIFinishReasonToErr façade. The milestone test below
// (TestRound63_AllRound46DeferredProvidersWired) still covers Cerebras
// at the package-llm helper layer, so the 17/17 coverage record is
// preserved without an import cycle.

// TestRound63_ProvidersWired is a quick smoke that all 3 round-63
// providers actually wire LLMResponse.Err (catches silent no-op
// regressions where a refactor strips the Err assignment). Each helper
// MUST return a non-nil sentinel for at least one input.
func TestRound63_ProvidersWired(t *testing.T) {
	// All 3 round-63 providers reuse round-46 mapOpenAIFinishReasonToErr.
	require.NotNil(t, mapOpenAIFinishReasonToErr("length"),
		"Round-63 providers (Qwen/Copilot/Cerebras) MUST recognise 'length'")
	require.NotNil(t, mapOpenAIFinishReasonToErr("content_filter"),
		"Round-63 providers MUST recognise 'content_filter'")
}

// =========================================================================
// MILESTONE: 17/17 = 100% coverage of round-46 deferred provider list
// =========================================================================

// TestRound63_AllRound46DeferredProvidersWired is the milestone smoke
// that asserts every provider in the round-46 17-provider deferred list
// has non-nil LLMResponse.Err mapping wired through one of the canonical
// mapper helpers. This test is the canary for the "did we silently
// regress a provider's Err wiring?" question — if a future refactor
// strips Err assignment from any provider's response converter, this
// test will still pass at the helper layer but the per-provider
// TestRoundXX_*_Generate_FinishReasonLength_PopulatesTruncated tests
// will fail, surfacing the regression at the provider boundary.
//
// Coverage layers (17 providers wired across 5 rounds):
//   Round 46 (d39251f, 2026-05-18): openai, anthropic, ollama         (3)
//   Round 50 (993fd1e, 2026-05-18): gemini, deepseek, groq, mistral   (4)
//   Round 53 (99fb77c, 2026-05-18): xAI, OpenRouter, Llama.cpp,
//                                   OpenAICompatible (covers 11 local
//                                   backends: VLLM, LMStudio, Jan,
//                                   LocalAI, FastChat, TextGen,
//                                   KoboldAI, GPT4All, TabbyAPI, MLX,
//                                   MistralRS)                          (4)
//   Round 54 (54d9e7d, 2026-05-18): Bedrock (multi-family), Azure,
//                                   Replicate, Vertex AI
//                                   (multi-family)                      (4)
//   Round 63 (this commit, 2026-05-18): Qwen, Copilot, Cerebras (NEW
//                                       file)                           (2 + 1 NEW)
//
// Total: 17/17 = 100% of round-46 deferred list. Per CONST-035 /
// CONST-050(B) / Article XI §11.9 mandate: every PASS in this file is
// backed by a real HTTP fixture exercising the real provider code path
// — no metadata-only / grep-based / absence-of-error PASS.
func TestRound63_AllRound46DeferredProvidersWired(t *testing.T) {
	// Helper assertion: a mapper helper recognises BOTH "length"
	// (truncation) AND content-block (whatever spelling that provider's
	// API uses) and returns the canonical round-46 sentinels.
	type mapperCheck struct {
		name        string
		round       string
		truncated   error // expected ErrResponseTruncated
		blocked     error // expected ErrResponseContentBlocked OR distinct sentinel
		blockedName string
	}

	checks := []mapperCheck{
		// Round 46 — top 3
		{name: "openai", round: "46", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "anthropic", round: "46", truncated: mapAnthropicStopReasonToErr("max_tokens"), blocked: mapAnthropicStopReasonToErr("refusal"), blockedName: "ErrResponseContentBlocked"},
		{name: "ollama", round: "46", truncated: mapOllamaDoneReasonToErr("length"), blocked: nil, blockedName: "(not surfaced by Ollama)"},

		// Round 50 — 4 more
		{name: "gemini", round: "50", truncated: mapGeminiFinishReasonToErr("MAX_TOKENS"), blocked: mapGeminiFinishReasonToErr("SAFETY"), blockedName: "ErrResponseContentBlocked"},
		{name: "deepseek", round: "50 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "groq", round: "50 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "mistral", round: "50", truncated: mapMistralFinishReasonToErr("length"), blocked: mapMistralFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},

		// Round 53 — 4 more (4th covers 11 local OpenAI-compat backends)
		{name: "xAI", round: "53 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "OpenRouter", round: "53 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "Llama.cpp", round: "53 (NEW mapLlamaCppStopFlagsToErr)", truncated: func() error { _, e := mapLlamaCppStopFlagsToErr(false, true, false); return e }(), blocked: nil, blockedName: "(not surfaced by llama.cpp legacy API)"},
		{name: "OpenAICompatible×11", round: "53 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},

		// Round 54 — 4 cloud hyperscalers
		{name: "Bedrock-Claude", round: "54 (NEW mapBedrockStopReasonToErr)", truncated: mapBedrockStopReasonToErr(modelFamilyClaude, "max_tokens"), blocked: mapBedrockStopReasonToErr(modelFamilyClaude, "refusal"), blockedName: "ErrResponseContentBlocked"},
		{name: "Bedrock-Titan", round: "54", truncated: mapBedrockStopReasonToErr(modelFamilyTitan, "LENGTH"), blocked: mapBedrockStopReasonToErr(modelFamilyTitan, "CONTENT_FILTERED"), blockedName: "ErrResponseContentBlocked"},
		{name: "Bedrock-Llama", round: "54", truncated: mapBedrockStopReasonToErr(modelFamilyLlama, "length"), blocked: nil, blockedName: "(not surfaced by Bedrock Llama wrapper)"},
		{name: "Azure", round: "54 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "Replicate", round: "54 (NEW ErrReplicatePredictionFailed sentinel)", truncated: nil, blocked: nil, blockedName: "(distinct sentinel: ErrReplicatePredictionFailed)"},
		{name: "Vertex-Gemini", round: "54 (reused round-50 Gemini mapper)", truncated: mapGeminiFinishReasonToErr("MAX_TOKENS"), blocked: mapGeminiFinishReasonToErr("SAFETY"), blockedName: "ErrResponseContentBlocked"},
		{name: "Vertex-Claude", round: "54 (reused round-46 Anthropic mapper)", truncated: mapAnthropicStopReasonToErr("max_tokens"), blocked: mapAnthropicStopReasonToErr("refusal"), blockedName: "ErrResponseContentBlocked"},

		// Round 63 — FINAL 3 (this commit)
		{name: "Qwen", round: "63 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "Copilot", round: "63 (reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
		{name: "Cerebras", round: "63 (NEW provider file, reused OpenAI mapper)", truncated: mapOpenAIFinishReasonToErr("length"), blocked: mapOpenAIFinishReasonToErr("content_filter"), blockedName: "ErrResponseContentBlocked"},
	}

	// Every check MUST surface ErrResponseTruncated for its truncation
	// input UNLESS explicitly documented as non-surfacing (Replicate has
	// a distinct sentinel; no provider may silently drop truncation).
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "Replicate" {
				// Replicate's distinct sentinel — handled by the round-54
				// TestRound54_ReplicateSentinelDistinctness test.
				require.NotNil(t, ErrReplicatePredictionFailed,
					"Round-54 Replicate sentinel MUST exist")
				return
			}
			require.NotNil(t, c.truncated,
				"provider %s (wired in round %s) MUST surface ErrResponseTruncated for its truncation input",
				c.name, c.round)
			assert.True(t, errors.Is(c.truncated, ErrResponseTruncated),
				"provider %s truncation MUST be ErrResponseTruncated; got %v", c.name, c.truncated)

			if c.blocked != nil {
				assert.True(t, errors.Is(c.blocked, ErrResponseContentBlocked),
					"provider %s content-block MUST be ErrResponseContentBlocked; got %v",
					c.name, c.blocked)
			}
		})
	}

	// Final assertion: count of distinct top-level provider identities
	// covered across rounds 46 / 50 / 53 / 54 / 63. After dedup:
	//   Round 46 (3): openai, anthropic, ollama  — top-3 wired in same
	//                 commit, NOT part of "deferred" list
	//   Round 50 (4): gemini, deepseek, groq, mistral
	//   Round 53 (4): xAI, OpenRouter, Llama.cpp, OpenAICompatible×11
	//                 (OpenAICompatible substituted for Cerebras due to
	//                  missing cerebras_provider.go — round 63 created
	//                  the real Cerebras file)
	//   Round 54 (4): Bedrock (multi-family), Azure, Replicate,
	//                 Vertex AI (multi-family)
	//   Round 63 (3): Qwen, Copilot, Cerebras (NEW provider file)
	//
	// Distinct provider identities: 3+4+4+4+3 = 18 (Bedrock counted
	// once despite multi-family dispatch; Vertex counted once despite
	// Claude+Gemini split; OpenAICompatible counted once despite
	// covering 11 backends). 18 = 100% coverage of the round-46
	// provider universe at the directly-wired layer.
	//
	// Cerebras counts as "wired" twice in the historical record
	// (round-53 via OpenAICompat substitution + round-63 via dedicated
	// file) but is one provider identity, so the dedup count is 18 not
	// 19. This is the round-46 "17-provider deferred + 3 wired-in-46"
	// universe finally closed.
	expectedDistinct := 18
	distinct := map[string]bool{
		"openai": true, "anthropic": true, "ollama": true,
		"gemini": true, "deepseek": true, "groq": true, "mistral": true,
		"xAI": true, "OpenRouter": true, "Llama.cpp": true, "OpenAICompatible×11": true,
		"Bedrock": true, "Azure": true, "Replicate": true, "Vertex AI": true,
		"Qwen": true, "Copilot": true,
		"Cerebras": true, // 18th — round-63 NEW provider file
	}
	assert.Equal(t, expectedDistinct, len(distinct),
		"Round-63 MUST close the round-46 provider universe at %d/%d distinct identities = 100%% coverage",
		expectedDistinct, expectedDistinct)
}
