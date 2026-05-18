// Round-50 §11.4 anti-bluff tests for LLMResponse.Err propagation —
// extends round-46 (commit d39251f) to four more providers:
//   - Gemini (Google AI)         finishReason: MAX_TOKENS / SAFETY / RECITATION
//   - DeepSeek (OpenAI-compat)   finish_reason: length / content_filter
//   - Groq    (OpenAI-compat)    finish_reason: length / content_filter
//   - Mistral (OpenAI-compat)    finish_reason: length / model_length
//
// Round-46 wired openai / anthropic / ollama (the top three providers);
// round-50 closes 4 more of the 17 deferred providers, leaving 13 for
// round-51+. Backward-compatibility invariant from round-46 preserved:
// the Err field is opt-in nil-by-default, so providers that don't
// participate yet emit nil Err — never worse than round-33's pre-fix
// state.
//
// CONST-035 / CONST-050(A)+(B) / Article XI §11.9: every PASS in this
// file is backed by an httptest fixture that exercises the real provider
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
// Gemini (Google AI)
// =========================================================================

// TestRound50_Gemini_Generate_FinishReasonMaxTokens_PopulatesErr asserts
// that Gemini's MAX_TOKENS finishReason maps to ErrResponseTruncated and
// that the partial Content survives.
func TestRound50_Gemini_Generate_FinishReasonMaxTokens_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]interface{}{
							{"text": "Once upon a time there was a kingdom that"},
						},
					},
					"finishReason": "MAX_TOKENS",
					"index":        0,
				},
			},
			"usageMetadata": map[string]interface{}{
				"promptTokenCount":     10,
				"candidatesTokenCount": 8,
				"totalTokenCount":      18,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(ProviderConfigEntry{
		Type:     ProviderTypeGemini,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID:        uuid.New(),
		Model:     "gemini-2.5-flash",
		Messages:  []Message{{Role: "user", Content: "Tell me a story"}},
		MaxTokens: 8,
	})
	require.NoError(t, err, "Generate MUST NOT return top-level error for MAX_TOKENS")
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "Content MUST hold partial output even when truncated")
	require.NotNil(t, resp.Err, "Err MUST be populated for finishReason=MAX_TOKENS")
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "MAX_TOKENS", resp.FinishReason,
		"FinishReason MUST preserve literal API value alongside Err")
}

// TestRound50_Gemini_Generate_FinishReasonSafety_PopulatesBlocked asserts
// that Gemini's SAFETY finishReason maps to ErrResponseContentBlocked.
func TestRound50_Gemini_Generate_FinishReasonSafety_PopulatesBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": ""}},
					},
					"finishReason": "SAFETY",
					"index":        0,
				},
			},
			"usageMetadata": map[string]interface{}{
				"promptTokenCount":     5,
				"candidatesTokenCount": 0,
				"totalTokenCount":      5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(ProviderConfigEntry{
		Type:     ProviderTypeGemini,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID:       uuid.New(),
		Model:    "gemini-2.5-flash",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Err, "Err MUST be populated for finishReason=SAFETY")
	assert.True(t, errors.Is(resp.Err, ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked; got %v", resp.Err)
	assert.Equal(t, "SAFETY", resp.FinishReason)
}

// TestRound50_Gemini_Generate_CleanStop_LeavesErrNil asserts that
// finishReason=STOP does not synthesise a spurious Err.
func TestRound50_Gemini_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "The answer is 4."}},
					},
					"finishReason": "STOP",
					"index":        0,
				},
			},
			"usageMetadata": map[string]interface{}{
				"promptTokenCount":     5,
				"candidatesTokenCount": 5,
				"totalTokenCount":      10,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(ProviderConfigEntry{
		Type:     ProviderTypeGemini,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID:       uuid.New(),
		Model:    "gemini-2.5-flash",
		Messages: []Message{{Role: "user", Content: "2+2?"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Err, "Err MUST be nil for clean finishReason=STOP")
	assert.Equal(t, "The answer is 4.", resp.Content)
}

// TestRound50_Gemini_Generate_PromptBlocked_PopulatesBlocked asserts
// that a promptFeedback.blockReason on an otherwise-clean response
// still populates Err (Gemini surfaces prompt-side blocks here).
func TestRound50_Gemini_Generate_PromptBlocked_PopulatesBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": ""}},
					},
					"finishReason": "OTHER",
					"index":        0,
				},
			},
			"promptFeedback": map[string]interface{}{
				"blockReason": "SAFETY",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(ProviderConfigEntry{
		Type:     ProviderTypeGemini,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID:       uuid.New(),
		Model:    "gemini-2.5-flash",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Err,
		"Err MUST be populated when promptFeedback.blockReason is set")
	assert.True(t, errors.Is(resp.Err, ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked when prompt is blocked; got %v", resp.Err)
}

// =========================================================================
// DeepSeek (OpenAI-compatible)
// =========================================================================

// TestRound50_DeepSeek_Generate_FinishReasonLength_PopulatesErr asserts
// that finish_reason="length" maps to ErrResponseTruncated.
func TestRound50_DeepSeek_Generate_FinishReasonLength_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chatcmpl-round50-deepseek-length",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "deepseek-chat",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Truncated DeepSeek partial output",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewDeepSeekProvider(ProviderConfigEntry{
		Type:     ProviderTypeDeepSeek,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID:        uuid.New(),
		Model:     "deepseek-chat",
		Messages:  []Message{{Role: "user", Content: "test"}},
		MaxTokens: 5,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "Content MUST hold partial output")
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "length", resp.FinishReason)
}

// TestRound50_DeepSeek_Generate_ContentFilter_PopulatesBlocked asserts
// that finish_reason="content_filter" maps to ErrResponseContentBlocked.
func TestRound50_DeepSeek_Generate_ContentFilter_PopulatesBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round50-deepseek-cf", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "deepseek-chat",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": ""},
					"finish_reason": "content_filter",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 5, "completion_tokens": 0, "total_tokens": 5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewDeepSeekProvider(ProviderConfigEntry{
		Type: ProviderTypeDeepSeek, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "deepseek-chat",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked; got %v", resp.Err)
}

// TestRound50_DeepSeek_Generate_CleanStop_LeavesErrNil asserts clean stop
// leaves Err nil (backward-compat invariant).
func TestRound50_DeepSeek_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "ok", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "deepseek-chat",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": "4"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 4, "completion_tokens": 1, "total_tokens": 5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewDeepSeekProvider(ProviderConfigEntry{
		Type: ProviderTypeDeepSeek, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "deepseek-chat",
		Messages: []Message{{Role: "user", Content: "2+2?"}},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Err, "Err MUST be nil for clean finish_reason=stop")
	assert.Equal(t, "4", resp.Content)
}

// =========================================================================
// Groq (OpenAI-compatible)
// =========================================================================

// TestRound50_Groq_Generate_FinishReasonLength_PopulatesErr asserts that
// Groq's "length" finish_reason maps to ErrResponseTruncated.
func TestRound50_Groq_Generate_FinishReasonLength_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round50-groq-length", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "llama-3.1-8b-instant",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Groq partial output here",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewGroqProvider(ProviderConfigEntry{
		Type: ProviderTypeGroq, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "llama-3.1-8b-instant",
		Messages:  []Message{{Role: "user", Content: "test"}},
		MaxTokens: 4,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "Content MUST hold partial output")
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "length", resp.FinishReason)
}

// TestRound50_Groq_Generate_ContentFilter_PopulatesBlocked asserts
// content_filter mapping.
func TestRound50_Groq_Generate_ContentFilter_PopulatesBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round50-groq-cf", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "llama-3.1-8b-instant",
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

	provider, err := NewGroqProvider(ProviderConfigEntry{
		Type: ProviderTypeGroq, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "llama-3.1-8b-instant",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked; got %v", resp.Err)
}

// TestRound50_Groq_Generate_CleanStop_LeavesErrNil asserts clean stop.
func TestRound50_Groq_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "ok", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "llama-3.1-8b-instant",
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

	provider, err := NewGroqProvider(ProviderConfigEntry{
		Type: ProviderTypeGroq, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "llama-3.1-8b-instant",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Err, "Err MUST be nil for clean finish_reason=stop")
}

// =========================================================================
// Mistral (OpenAI-compatible with model_length extension)
// =========================================================================

// TestRound50_Mistral_Generate_FinishReasonLength_PopulatesErr asserts
// that Mistral's "length" finish_reason maps to ErrResponseTruncated.
func TestRound50_Mistral_Generate_FinishReasonLength_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "cmpl-round50-mistral-length", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "mistral-large-latest",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Mistral partial",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 6, "completion_tokens": 2, "total_tokens": 8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewMistralProvider(ProviderConfigEntry{
		Type: ProviderTypeMistral, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "mistral-large-latest",
		Messages:  []Message{{Role: "user", Content: "test"}},
		MaxTokens: 2,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "length", resp.FinishReason)
}

// TestRound50_Mistral_Generate_ModelLength_PopulatesErr asserts that
// Mistral's "model_length" extension also maps to ErrResponseTruncated.
func TestRound50_Mistral_Generate_ModelLength_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "cmpl-round50-mistral-ml", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "mistral-small-latest",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": "Hit model cap"},
					"finish_reason": "model_length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 4, "completion_tokens": 3, "total_tokens": 7,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewMistralProvider(ProviderConfigEntry{
		Type: ProviderTypeMistral, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "mistral-small-latest",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Err,
		"Err MUST be populated for Mistral finish_reason=model_length")
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"model_length MUST map to ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "model_length", resp.FinishReason)
}

// TestRound50_Mistral_Generate_CleanStop_LeavesErrNil asserts clean stop.
func TestRound50_Mistral_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "ok", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "mistral-small-latest",
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

	provider, err := NewMistralProvider(ProviderConfigEntry{
		Type: ProviderTypeMistral, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		ID: uuid.New(), Model: "mistral-small-latest",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Err, "Err MUST be nil for clean finish_reason=stop")
}

// =========================================================================
// Paired-mutation mapper-pinning tests (closed-set regression)
// =========================================================================

// TestRound50_Gemini_FinishReasonMapper_AllCases pins the complete
// Gemini mapping for fast regression detection without HTTP fixtures.
// Per CONST-050(B) paired-mutation: if a future change adds a new
// finishReason value, this test (or its sibling) MUST be extended in
// the same commit.
func TestRound50_Gemini_FinishReasonMapper_AllCases(t *testing.T) {
	// Clean / unknown reasons → nil
	assert.Nil(t, mapGeminiFinishReasonToErr(""), "empty reason MUST be nil")
	assert.Nil(t, mapGeminiFinishReasonToErr("STOP"), "STOP MUST be nil")
	assert.Nil(t, mapGeminiFinishReasonToErr("OTHER"),
		"OTHER MUST be nil (unknown — cannot synthesise)")
	assert.Nil(t, mapGeminiFinishReasonToErr("LANGUAGE"),
		"LANGUAGE MUST be nil (unknown for round-50)")

	// Truncation
	assert.True(t, errors.Is(mapGeminiFinishReasonToErr("MAX_TOKENS"), ErrResponseTruncated),
		"MAX_TOKENS MUST map to ErrResponseTruncated")

	// Content-block reasons
	for _, reason := range []string{"SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII"} {
		assert.True(t, errors.Is(mapGeminiFinishReasonToErr(reason), ErrResponseContentBlocked),
			"%s MUST map to ErrResponseContentBlocked", reason)
	}
}

// TestRound50_Mistral_FinishReasonMapper_AllCases pins the Mistral
// mapping. Note: Mistral has NO content-filter signal on the wire, so
// ErrResponseContentBlocked is NOT reachable here (documented in
// helper's doc comment).
func TestRound50_Mistral_FinishReasonMapper_AllCases(t *testing.T) {
	assert.Nil(t, mapMistralFinishReasonToErr(""))
	assert.Nil(t, mapMistralFinishReasonToErr("stop"))
	assert.Nil(t, mapMistralFinishReasonToErr("tool_calls"))
	assert.Nil(t, mapMistralFinishReasonToErr("content_filter"),
		"Mistral has no content-filter on wire → MUST be nil even for content_filter string")

	assert.True(t, errors.Is(mapMistralFinishReasonToErr("length"), ErrResponseTruncated))
	assert.True(t, errors.Is(mapMistralFinishReasonToErr("model_length"), ErrResponseTruncated),
		"Mistral's model_length extension MUST also map to ErrResponseTruncated")
}

// TestRound50_DeepSeekAndGroq_ReuseOpenAIMapper documents and pins the
// architectural decision: DeepSeek + Groq are OpenAI-compatible so they
// reuse mapOpenAIFinishReasonToErr verbatim rather than introducing
// near-identical helpers. If a future change diverges either provider
// from OpenAI's mapping, this test MUST be replaced with provider-
// specific mapper tests in the same commit.
func TestRound50_DeepSeekAndGroq_ReuseOpenAIMapper(t *testing.T) {
	// Verify the shared mapper still covers the closed set both providers
	// emit; if OpenAI's mapper is extended in a way that breaks DeepSeek/
	// Groq semantics, the divergence MUST be split out here.
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("length"), ErrResponseTruncated))
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("content_filter"), ErrResponseContentBlocked))
	assert.Nil(t, mapOpenAIFinishReasonToErr("stop"))
	assert.Nil(t, mapOpenAIFinishReasonToErr("tool_calls"))
}

// TestRound50_HelpersDistinct asserts that the new round-50 helper
// (mapGeminiFinishReasonToErr, mapMistralFinishReasonToErr) and the
// reused round-46 helpers all map clean reasons to nil and known
// degradation reasons to one of the two canonical sentinels. This is
// the round-50 equivalent of round-46's TestRound46_Sentinels_Distinct.
func TestRound50_HelpersDistinct(t *testing.T) {
	cases := []struct {
		name    string
		fn      func(string) error
		clean   []string
		trunc   []string
		blocked []string
	}{
		{
			name:    "Gemini",
			fn:      mapGeminiFinishReasonToErr,
			clean:   []string{"", "STOP", "OTHER"},
			trunc:   []string{"MAX_TOKENS"},
			blocked: []string{"SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII"},
		},
		{
			name:    "Mistral",
			fn:      mapMistralFinishReasonToErr,
			clean:   []string{"", "stop", "tool_calls"},
			trunc:   []string{"length", "model_length"},
			blocked: nil, // not reachable on Mistral wire
		},
		{
			name:    "OpenAI(reused-by-DeepSeek+Groq)",
			fn:      mapOpenAIFinishReasonToErr,
			clean:   []string{"", "stop", "tool_calls", "function_call"},
			trunc:   []string{"length"},
			blocked: []string{"content_filter"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, r := range tc.clean {
				assert.Nil(t, tc.fn(r), "clean reason %q MUST be nil", r)
			}
			for _, r := range tc.trunc {
				err := tc.fn(r)
				require.NotNil(t, err, "truncation reason %q MUST be non-nil", r)
				assert.True(t, errors.Is(err, ErrResponseTruncated),
					"reason %q MUST map to ErrResponseTruncated; got %v", r, err)
			}
			for _, r := range tc.blocked {
				err := tc.fn(r)
				require.NotNil(t, err, "blocked reason %q MUST be non-nil", r)
				assert.True(t, errors.Is(err, ErrResponseContentBlocked),
					"reason %q MUST map to ErrResponseContentBlocked; got %v", r, err)
			}
		})
	}
}
