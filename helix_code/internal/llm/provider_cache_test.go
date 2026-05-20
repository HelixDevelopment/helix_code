package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// provider_cache_test.go — speed programme P1-T05.
//
// Tests for `cache_control` / prompt-caching opt-in across the cache-capable
// providers (OpenAI, DeepSeek, Gemini — Anthropic was covered by P1-T04).
//
// Test layers (CONST-050):
//   - unit         : the cache-metric extraction normalises the OpenAI and
//                    DeepSeek field shapes; the provider-capability map is
//                    correct; prefix tracking does not panic / mutate.
//   - integration  : an httptest shim per provider returns a cache-hit usage
//                    payload and the provider surfaces it in ProviderMetadata
//                    AND the request body it sent is byte-identical across two
//                    requests (the implicit-caching precondition).
//   - benchmark    : per-tool prefix-canonicalization cost.
//
// Real-API anti-bluff proof: the plan asks for a pasted real-API
// cache_read / cached_tokens metric per provider. No OPENAI_API_KEY,
// DEEPSEEK_API_KEY, or GEMINI_API_KEY is present in the environment, so the
// real-API metric is recorded as SKIP-OK per CONST-§11.4.3/§11.4.6 in
// TestProviderCache_RealAPIMetric_SkipOK below; the httptest shim integration
// tests are the in-reach proof that the wiring is correct.

// ---------------------------------------------------------------------------
// Unit — cache-metric extraction
// ---------------------------------------------------------------------------

func TestOpenAICacheUsageFields_CachedPromptTokens(t *testing.T) {
	t.Run("OpenAI shape — prompt_tokens_details.cached_tokens", func(t *testing.T) {
		var u openAICacheUsageFields
		require.NoError(t, json.Unmarshal([]byte(
			`{"prompt_tokens_details":{"cached_tokens":1536}}`), &u))
		assert.Equal(t, 1536, u.cachedPromptTokens())
	})

	t.Run("DeepSeek shape — prompt_cache_hit_tokens", func(t *testing.T) {
		var u openAICacheUsageFields
		require.NoError(t, json.Unmarshal([]byte(
			`{"prompt_cache_hit_tokens":2048,"prompt_cache_miss_tokens":128}`), &u))
		assert.Equal(t, 2048, u.cachedPromptTokens())
	})

	t.Run("cache miss — no fields", func(t *testing.T) {
		var u openAICacheUsageFields
		require.NoError(t, json.Unmarshal([]byte(`{}`), &u))
		assert.Equal(t, 0, u.cachedPromptTokens())
		assert.Nil(t, u.cacheMetadata(),
			"a cache miss must yield nil metadata so ProviderMetadata stays unset")
	})

	t.Run("OpenAI metadata fragment", func(t *testing.T) {
		var u openAICacheUsageFields
		require.NoError(t, json.Unmarshal([]byte(
			`{"prompt_tokens_details":{"cached_tokens":900}}`), &u))
		meta := u.cacheMetadata()
		require.NotNil(t, meta)
		assert.Equal(t, 900, meta["cached_prompt_tokens"])
		_, hasMiss := meta["cache_miss_prompt_tokens"]
		assert.False(t, hasMiss, "OpenAI shape does not carry a miss count")
	})

	t.Run("DeepSeek metadata fragment carries hit and miss", func(t *testing.T) {
		var u openAICacheUsageFields
		require.NoError(t, json.Unmarshal([]byte(
			`{"prompt_cache_hit_tokens":3000,"prompt_cache_miss_tokens":256}`), &u))
		meta := u.cacheMetadata()
		require.NotNil(t, meta)
		assert.Equal(t, 3000, meta["cached_prompt_tokens"])
		assert.Equal(t, 256, meta["cache_miss_prompt_tokens"])
	})
}

func TestProviderSupportsPromptCache(t *testing.T) {
	// The four cache-capable providers.
	for _, pt := range []ProviderType{
		ProviderTypeAnthropic, ProviderTypeOpenAI,
		ProviderTypeDeepSeek, ProviderTypeGemini,
	} {
		assert.True(t, providerSupportsPromptCache(pt),
			"%s must be marked cache-capable", pt)
	}
	// A representative no-caching provider — must be left untouched.
	assert.False(t, providerSupportsPromptCache(ProviderTypeOllama),
		"Ollama (local inference) has no remote prompt cache and must stay untouched")
}

// ---------------------------------------------------------------------------
// Unit — prefix tracking does not panic and is observation-only
// ---------------------------------------------------------------------------

func TestTrackPromptCachePrefixGeneric_NilDetectorIsNoOp(t *testing.T) {
	// A nil detector must be tolerated silently — prefix tracking is an
	// optimisation concern and must never break a request.
	assert.NotPanics(t, func() {
		trackPromptCachePrefixGeneric(nil, "test", "system prompt", nil)
	})
}

func TestCanonicalizeToolForPrefix_StableAcrossRuns(t *testing.T) {
	// A tool whose JSON-Schema `required` array is assembled from a Go map
	// would otherwise hash differently per run. canonicalizeToolForPrefix
	// must normalise it so the prefix is byte-stable.
	tool := Tool{
		Function: ToolFunction{
			Name:        "edit_file",
			Description: "Edit a file",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"path", "content"},
			},
		},
	}
	var first []byte
	for i := 0; i < 50; i++ {
		canon := canonicalizeToolForPrefix(tool)
		b, err := json.Marshal(canon)
		require.NoError(t, err)
		if i == 0 {
			first = b
			continue
		}
		assert.Equal(t, string(first), string(b),
			"canonicalizeToolForPrefix must be byte-stable across runs (run %d)", i)
	}
}

// ---------------------------------------------------------------------------
// Integration — httptest shim: cache hit surfaced + request byte-stability
// ---------------------------------------------------------------------------

// TestOpenAIProvider_PromptCacheHit_Shim drives the OpenAI provider against a
// shim that returns a cache-hit usage payload and asserts (1) the cached-token
// metric is surfaced in ProviderMetadata, and (2) the request body the
// provider sent is byte-identical across two requests with the same prefix —
// the precondition for OpenAI implicit caching to actually hit.
func TestOpenAIProvider_PromptCacheHit_Shim(t *testing.T) {
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)
		resp := map[string]interface{}{
			"id":     "chatcmpl-cache",
			"object": "chat.completion",
			"model":  "gpt-4o",
			"choices": []map[string]interface{}{{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": "ok"},
				"finish_reason": "stop",
			}},
			"usage": map[string]interface{}{
				"prompt_tokens":         2000,
				"completion_tokens":     5,
				"total_tokens":          2005,
				"prompt_tokens_details": map[string]interface{}{"cached_tokens": 1792},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(ProviderConfigEntry{
		Type: ProviderTypeOpenAI, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	req := &LLMRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "system", Content: "You are a precise coding assistant."},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Generate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.ProviderMetadata, "cache-hit usage must populate ProviderMetadata")
	assert.Equal(t, 1792, resp.ProviderMetadata["cached_prompt_tokens"],
		"OpenAI cached_tokens must surface as cached_prompt_tokens")

	// Second request with the SAME prefix — the body must be byte-identical.
	_, err = provider.Generate(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, bodies, 2)
	assert.Equal(t, string(bodies[0]), string(bodies[1]),
		"implicit caching only hits when the request prefix is byte-stable")
}

// TestOpenAIProvider_PromptCacheMiss_NoMetadata proves a cache MISS leaves
// ProviderMetadata unset — the response stays byte-identical to pre-P1-T05.
func TestOpenAIProvider_PromptCacheMiss_NoMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":     "chatcmpl-miss",
			"object": "chat.completion",
			"model":  "gpt-4o",
			"choices": []map[string]interface{}{{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": "ok"},
				"finish_reason": "stop",
			}},
			"usage": map[string]interface{}{
				"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(ProviderConfigEntry{
		Type: ProviderTypeOpenAI, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		Model:     "gpt-4o",
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 50,
	})
	require.NoError(t, err)
	assert.Nil(t, resp.ProviderMetadata,
		"a cache miss must leave ProviderMetadata unset (no-regression)")
}

// TestDeepSeekProvider_PromptCacheHit_Shim drives the DeepSeek provider
// against a shim returning DeepSeek's flat hit/miss fields.
func TestDeepSeekProvider_PromptCacheHit_Shim(t *testing.T) {
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)
		resp := map[string]interface{}{
			"id":     "deepseek-cache",
			"object": "chat.completion",
			"model":  "deepseek-chat",
			"choices": []map[string]interface{}{{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": "ok"},
				"finish_reason": "stop",
			}},
			"usage": map[string]interface{}{
				"prompt_tokens":           1500,
				"completion_tokens":       4,
				"total_tokens":            1504,
				"prompt_cache_hit_tokens":  1408,
				"prompt_cache_miss_tokens": 92,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewDeepSeekProvider(ProviderConfigEntry{
		Type: ProviderTypeDeepSeek, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	req := &LLMRequest{
		Model: "deepseek-chat",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Generate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.ProviderMetadata, "DeepSeek cache hit must populate ProviderMetadata")
	assert.Equal(t, 1408, resp.ProviderMetadata["cached_prompt_tokens"])
	assert.Equal(t, 92, resp.ProviderMetadata["cache_miss_prompt_tokens"])

	// Byte-stable prefix across two requests (implicit-caching precondition).
	_, err = provider.Generate(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, bodies, 2)
	assert.Equal(t, string(bodies[0]), string(bodies[1]),
		"DeepSeek implicit context caching needs a byte-stable request prefix")
}

// TestGeminiProvider_PromptCacheHit_Shim drives the Gemini provider against a
// shim returning usageMetadata.cachedContentTokenCount and asserts the metric
// is surfaced and the request body is byte-stable across two requests.
func TestGeminiProvider_PromptCacheHit_Shim(t *testing.T) {
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, b)
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{{
				"content": map[string]interface{}{
					"role":  "model",
					"parts": []map[string]interface{}{{"text": "ok"}},
				},
				"finishReason": "STOP",
				"index":        0,
			}},
			"usageMetadata": map[string]interface{}{
				"promptTokenCount":        1800,
				"candidatesTokenCount":    3,
				"totalTokenCount":         1803,
				"cachedContentTokenCount": 1664,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(ProviderConfigEntry{
		Type: ProviderTypeGemini, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	req := &LLMRequest{
		Model: "gemini-2.5-flash",
		Messages: []Message{
			{Role: "system", Content: "You are a precise assistant."},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Generate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.ProviderMetadata,
		"Gemini cachedContentTokenCount must populate ProviderMetadata")
	assert.Equal(t, 1664, resp.ProviderMetadata["cached_content_tokens"])

	// Byte-stable prefix across two requests (implicit-caching precondition).
	_, err = provider.Generate(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, bodies, 2)
	assert.Equal(t, string(bodies[0]), string(bodies[1]),
		"Gemini implicit context caching needs a byte-stable request prefix")
}

// TestProviderCache_RealAPIMetric_SkipOK records the real-API cache-hit metric
// requirement. No real provider API key is present in the environment, so the
// real-API proof is honestly skipped rather than fabricated (CONST-§11.4.3 /
// §11.4.6 — no guessing, no bluffing). The httptest shim tests above are the
// in-reach proof that the wiring is correct.
func TestProviderCache_RealAPIMetric_SkipOK(t *testing.T) {
	// SKIP-OK: openai real-API cache-hit metric — no API key in environment
	// SKIP-OK: deepseek real-API cache-hit metric — no API key in environment
	// SKIP-OK: gemini real-API cache-hit metric — no API key in environment
	t.Skip("SKIP-OK: real-API prompt-cache-hit metric for openai/deepseek/gemini " +
		"requires OPENAI_API_KEY / DEEPSEEK_API_KEY / GEMINI_API_KEY — absent " +
		"in this environment; httptest shim tests provide the in-reach proof")
}

// ---------------------------------------------------------------------------
// Benchmark — per-tool prefix-canonicalization cost
// ---------------------------------------------------------------------------

func BenchmarkCanonicalizeToolForPrefix(b *testing.B) {
	tool := Tool{
		Function: ToolFunction{
			Name:        "run_command",
			Description: "Execute a shell command and return its output",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
					"cwd":     map[string]interface{}{"type": "string"},
					"timeout": map[string]interface{}{"type": "integer"},
					"env": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []interface{}{"command"},
			},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = canonicalizeToolForPrefix(tool)
	}
}
