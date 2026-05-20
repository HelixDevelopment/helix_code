// Round-63 §11.4 anti-bluff tests for the Cerebras Cloud provider's
// LLMResponse.Err propagation.
//
// Speed programme P5-T02 (R1 B21): these tests moved verbatim (behaviour
// unchanged) out of internal/llm/response_err_round63_test.go together
// with the Cerebras provider implementation. They now reference
// cerebras.NewProvider / *cerebras.Provider and the shared llm.* types.
// The HTTP-fixture bodies, the asserted sentinels, and every PASS
// condition are byte-identical to the pre-move tests — this is a pure
// structural relocation, not a test rewrite.
//
// CONST-035 / CONST-050(A)+(B) / Article XI §11.9: every PASS in this
// file is backed by an httptest fixture exercising the real provider
// HTTP code path (real JSON encode/decode, real http.Client, real
// LLMResponse construction). No mocks of internal helpers — only the
// remote API is faked at the HTTP transport boundary, which is the
// canonical pattern for provider-layer unit tests.
package cerebras

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRound63_Cerebras_NewProvider_NoAPIKey_ReturnsConfigSentinel asserts
// the Cerebras provider refuses to construct without an API key — neither
// CEREBRAS_API_KEY nor ProviderConfigEntry.APIKey. Real configuration
// error surfaces so callers can fall back to a different provider per
// CONST-039 multi-provider mandate.
func TestRound63_Cerebras_NewProvider_NoAPIKey_ReturnsConfigSentinel(t *testing.T) {
	// Clear env to avoid environmental contamination.
	oldKey := os.Getenv("CEREBRAS_API_KEY")
	_ = os.Unsetenv("CEREBRAS_API_KEY")
	defer func() {
		if oldKey != "" {
			_ = os.Setenv("CEREBRAS_API_KEY", oldKey)
		}
	}()

	_, err := NewProvider(llm.ProviderConfigEntry{
		Type:    llm.ProviderTypeCerebras,
		Enabled: true,
	})
	require.Error(t, err, "cerebras.NewProvider MUST refuse construction without an API key")
	assert.True(t,
		strings.Contains(err.Error(), "CEREBRAS_API_KEY") ||
			strings.Contains(err.Error(), "no API key"),
		"error MUST mention the env-var name or 'no API key'; got %q", err.Error())
}

// TestRound63_Cerebras_Generate_FinishReasonLength_PopulatesTruncated
// asserts that Cerebras's "length" finish_reason maps to
// ErrResponseTruncated, end-to-end through the real HTTP code path.
func TestRound63_Cerebras_Generate_FinishReasonLength_PopulatesTruncated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round63-cerebras-length", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "llama3.1-70b",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Cerebras partial output here",
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

	provider, err := NewProvider(llm.ProviderConfigEntry{
		Type:     llm.ProviderTypeCerebras,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &llm.LLMRequest{
		ID:        uuid.New(),
		Model:     "llama3.1-70b",
		Messages:  []llm.Message{{Role: "user", Content: "test"}},
		MaxTokens: 4,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "Content MUST hold partial output")
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, llm.ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "length", resp.FinishReason)
}

// TestRound63_Cerebras_Generate_ContentFilter_PopulatesBlocked asserts
// content_filter mapping.
func TestRound63_Cerebras_Generate_ContentFilter_PopulatesBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "chatcmpl-round63-cerebras-cf", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "llama3.1-70b",
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

	provider, err := NewProvider(llm.ProviderConfigEntry{
		Type: llm.ProviderTypeCerebras, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(), Model: "llama3.1-70b",
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Err)
	assert.True(t, errors.Is(resp.Err, llm.ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked; got %v", resp.Err)
}

// TestRound63_Cerebras_Generate_CleanStop_LeavesErrNil asserts that the
// backward-compat invariant (clean stop → Err nil) holds for the new
// Cerebras provider too.
func TestRound63_Cerebras_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id": "ok", "object": "chat.completion",
			"created": time.Now().Unix(), "model": "llama3.1-70b",
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

	provider, err := NewProvider(llm.ProviderConfigEntry{
		Type: llm.ProviderTypeCerebras, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(), Model: "llama3.1-70b",
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Err, "Err MUST be nil for clean finish_reason=stop")
	assert.Equal(t, "OK", resp.Content)
}

// TestRound63_Cerebras_Stream_FinishReasonLength_PropagatesToTerminalFrame
// asserts that streaming-path truncation emits a terminal Err-bearing
// frame on the channel.
func TestRound63_Cerebras_Stream_FinishReasonLength_PropagatesToTerminalFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		chunks := []map[string]interface{}{
			{
				"id": "s1", "object": "chat.completion.chunk", "model": "llama3.1-70b",
				"choices": []map[string]interface{}{
					{"index": 0, "delta": map[string]interface{}{"role": "assistant", "content": "Hi"}, "finish_reason": ""},
				},
			},
			{
				"id": "s2", "object": "chat.completion.chunk", "model": "llama3.1-70b",
				"choices": []map[string]interface{}{
					{"index": 0, "delta": map[string]interface{}{"content": " there"}, "finish_reason": ""},
				},
			},
			{
				"id": "s3", "object": "chat.completion.chunk", "model": "llama3.1-70b",
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

	provider, err := NewProvider(llm.ProviderConfigEntry{
		Type: llm.ProviderTypeCerebras, APIKey: "test-key", Endpoint: server.URL, Enabled: true,
	})
	require.NoError(t, err)

	ch := make(chan llm.LLMResponse, 16)
	go func() {
		_ = provider.GenerateStream(context.Background(), &llm.LLMRequest{
			ID: uuid.New(), Model: "llama3.1-70b", Stream: true,
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
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
				assert.True(t, errors.Is(resp.Err, llm.ErrResponseTruncated),
					"terminal stream Err MUST be ErrResponseTruncated; got %v", resp.Err)
				assert.Equal(t, "length", resp.FinishReason)
			}
		case <-deadline:
			break loop
		}
	}
	assert.True(t, sawErrFrame, "stream MUST emit a terminal Err-bearing frame on finish_reason=length")
}

// TestRound63_CerebrasReusesOpenAIMapper pins the architectural decision
// that Cerebras Cloud advertises an OpenAI-compatible chat completions
// API and reuses the canonical OpenAI finish_reason mapper (exported as
// llm.MapOpenAIFinishReasonToErr — speed programme P5-T02 façade). If
// Cerebras adds a vendor-specific finish_reason value, this test MUST be
// replaced with a Cerebras-specific mapper in the same commit.
func TestRound63_CerebrasReusesOpenAIMapper(t *testing.T) {
	assert.True(t, errors.Is(llm.MapOpenAIFinishReasonToErr("length"), llm.ErrResponseTruncated))
	assert.True(t, errors.Is(llm.MapOpenAIFinishReasonToErr("content_filter"), llm.ErrResponseContentBlocked))
	assert.Nil(t, llm.MapOpenAIFinishReasonToErr("stop"))
	assert.Nil(t, llm.MapOpenAIFinishReasonToErr("tool_calls"))
}

// TestP5T02_CerebrasProvider_SatisfiesLLMProviderInterface is the
// structural-move no-regression guard for P5-T02: it confirms at runtime
// that the relocated *cerebras.Provider is still a usable llm.Provider —
// the same guarantee the openai/anthropic/etc. providers gave while they
// shared package llm. (The var _ llm.Provider assertion in cerebras.go
// gives the compile-time half; this gives a runtime, evidence-producing
// half per the anti-bluff mandate.)
func TestP5T02_CerebrasProvider_SatisfiesLLMProviderInterface(t *testing.T) {
	provider, err := NewProvider(llm.ProviderConfigEntry{
		Type: llm.ProviderTypeCerebras, APIKey: "test-key", Enabled: true,
	})
	require.NoError(t, err)

	var p llm.Provider = provider
	assert.Equal(t, llm.ProviderTypeCerebras, p.GetType())
	assert.Equal(t, "Cerebras", p.GetName())
	assert.NotEmpty(t, p.GetModels(), "moved provider MUST still seed its model list")
	assert.NotEmpty(t, p.GetCapabilities(), "moved provider MUST still report capabilities")
	assert.Equal(t, 128_000, p.GetContextWindow())

	n, err := p.CountTokens("hello world")
	require.NoError(t, err)
	assert.Positive(t, n, "CountTokens MUST return a positive estimate")

	require.NoError(t, p.Close())
}
