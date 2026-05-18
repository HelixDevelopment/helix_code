// Round-46 §11.4 anti-bluff tests for LLMResponse.Err propagation.
//
// Closes the round-33 anchored limitation in tool_provider.go:201/:251
// (CONST-035 / Article XI §11.9): LLMResponse previously had no Err
// field, so callers could not distinguish "ok empty chunk" from
// "mid-stream failure". Round 46 added LLMResponse.Err + sentinels
// (ErrResponseTruncated, ErrResponseContentBlocked) + provider wiring
// for OpenAI / Anthropic / Ollama + tool_provider.go honoring strategy.
//
// These tests assert end-to-end:
//   1. Sentinel distinctness (ErrResponseTruncated != ErrResponseContentBlocked).
//   2. JSON marshal/unmarshal round-trip preserves sentinel identity
//      via errors.Is(...) comparison.
//   3. OpenAI Generate populates Err on finish_reason="length"/"content_filter".
//   4. OpenAI GenerateStream emits a terminal frame with Err populated
//      when the stream's last chunk carries finish_reason="length".
//   5. Anthropic Generate populates Err on stop_reason="max_tokens".
//   6. Ollama Generate populates Err on done_reason="length".
//   7. tool_provider.go honors LLMResponse.Err by surfacing it on the
//      ToolStreamChunk.Error field.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRound46_Sentinels_Distinct asserts that the two new sentinels
// are distinct values — preventing accidental collision in errors.Is
// comparisons.
func TestRound46_Sentinels_Distinct(t *testing.T) {
	assert.NotEqual(t, ErrResponseTruncated, ErrResponseContentBlocked,
		"round-46 sentinels MUST be distinct values for errors.Is dispatch")
	assert.False(t, errors.Is(ErrResponseTruncated, ErrResponseContentBlocked),
		"errors.Is(Truncated, ContentBlocked) MUST be false")
	assert.False(t, errors.Is(ErrResponseContentBlocked, ErrResponseTruncated),
		"errors.Is(ContentBlocked, Truncated) MUST be false")
	assert.True(t, errors.Is(ErrResponseTruncated, ErrResponseTruncated),
		"sentinels MUST be self-identical")
	assert.True(t, errors.Is(ErrResponseContentBlocked, ErrResponseContentBlocked),
		"sentinels MUST be self-identical")
}

// TestRound46_LLMResponse_JSONRoundTrip_PreservesSentinel asserts that
// the custom MarshalJSON / UnmarshalJSON on LLMResponse preserves the
// sentinel identity across a JSON round-trip (essential when the
// LLMResponse is cached or sent over a wire). Generic errors degrade
// to errors.New(msg) on the unmarshal side.
func TestRound46_LLMResponse_JSONRoundTrip_PreservesSentinel(t *testing.T) {
	cases := []struct {
		name           string
		in             *LLMResponse
		assertSentinel func(t *testing.T, decoded *LLMResponse)
	}{
		{
			name: "ErrResponseTruncated_survives_round_trip",
			in: &LLMResponse{
				ID:           uuid.New(),
				Content:      "partial output",
				FinishReason: "length",
				Err:          ErrResponseTruncated,
			},
			assertSentinel: func(t *testing.T, decoded *LLMResponse) {
				require.NotNil(t, decoded.Err)
				assert.True(t, errors.Is(decoded.Err, ErrResponseTruncated),
					"decoded Err MUST be ErrResponseTruncated; got %v", decoded.Err)
			},
		},
		{
			name: "ErrResponseContentBlocked_survives_round_trip",
			in: &LLMResponse{
				ID:           uuid.New(),
				Content:      "",
				FinishReason: "content_filter",
				Err:          ErrResponseContentBlocked,
			},
			assertSentinel: func(t *testing.T, decoded *LLMResponse) {
				require.NotNil(t, decoded.Err)
				assert.True(t, errors.Is(decoded.Err, ErrResponseContentBlocked),
					"decoded Err MUST be ErrResponseContentBlocked; got %v", decoded.Err)
			},
		},
		{
			name: "GenericErr_degrades_to_errors_New",
			in: &LLMResponse{
				ID:      uuid.New(),
				Content: "ok-ish",
				Err:     errors.New("custom mid-stream parse failure"),
			},
			assertSentinel: func(t *testing.T, decoded *LLMResponse) {
				require.NotNil(t, decoded.Err)
				assert.False(t, errors.Is(decoded.Err, ErrResponseTruncated),
					"generic Err MUST NOT match ErrResponseTruncated")
				assert.False(t, errors.Is(decoded.Err, ErrResponseContentBlocked),
					"generic Err MUST NOT match ErrResponseContentBlocked")
				assert.Equal(t, "custom mid-stream parse failure", decoded.Err.Error(),
					"generic Err message MUST round-trip verbatim")
			},
		},
		{
			name: "NilErr_round_trips_as_nil",
			in: &LLMResponse{
				ID:      uuid.New(),
				Content: "clean response",
				Err:     nil,
			},
			assertSentinel: func(t *testing.T, decoded *LLMResponse) {
				assert.Nil(t, decoded.Err, "nil Err MUST round-trip as nil")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.in)
			require.NoError(t, err, "marshal MUST succeed")

			var decoded LLMResponse
			require.NoError(t, json.Unmarshal(data, &decoded), "unmarshal MUST succeed")

			assert.Equal(t, tc.in.Content, decoded.Content, "Content MUST round-trip")
			assert.Equal(t, tc.in.FinishReason, decoded.FinishReason, "FinishReason MUST round-trip")
			tc.assertSentinel(t, &decoded)
		})
	}
}

// TestRound46_OpenAI_Generate_FinishReasonLength_PopulatesErr asserts
// that the OpenAI Generate path populates Err with ErrResponseTruncated
// when the API returns finish_reason="length".
func TestRound46_OpenAI_Generate_FinishReasonLength_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chatcmpl-round46-length",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Once upon a time there was a kingdom that",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 8,
				"total_tokens":      18,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(ProviderConfigEntry{
		Type:     ProviderTypeOpenAI,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		Model:     "gpt-4o",
		Messages:  []Message{{Role: "user", Content: "Tell me a story"}},
		MaxTokens: 8,
	})
	require.NoError(t, err, "Generate MUST NOT return a top-level error for finish_reason=length")
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "Content MUST hold partial output even when truncated")
	require.NotNil(t, resp.Err, "Err MUST be populated for finish_reason=length")
	assert.True(t, errors.Is(resp.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", resp.Err)
	assert.Equal(t, "length", resp.FinishReason,
		"FinishReason MUST preserve the literal API value alongside Err")
}

// TestRound46_OpenAI_Generate_ContentFilter_PopulatesErr asserts that
// finish_reason="content_filter" maps to ErrResponseContentBlocked.
func TestRound46_OpenAI_Generate_ContentFilter_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chatcmpl-round46-filter",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
					},
					"finish_reason": "content_filter",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     5,
				"completion_tokens": 0,
				"total_tokens":      5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(ProviderConfigEntry{
		Type:     ProviderTypeOpenAI,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "blocked prompt"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Err, "Err MUST be populated for finish_reason=content_filter")
	assert.True(t, errors.Is(resp.Err, ErrResponseContentBlocked),
		"Err MUST be ErrResponseContentBlocked; got %v", resp.Err)
}

// TestRound46_OpenAI_Generate_CleanStop_LeavesErrNil asserts no false
// positives — finish_reason="stop" leaves Err nil. This is the
// backward-compat invariant for existing callers.
func TestRound46_OpenAI_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chatcmpl-round46-stop",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello, world!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     3,
				"completion_tokens": 4,
				"total_tokens":      7,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(ProviderConfigEntry{
		Type:     ProviderTypeOpenAI,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	resp, err := provider.Generate(context.Background(), &LLMRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Err, "clean finish_reason=stop MUST leave Err nil")
}

// TestRound46_OpenAI_GenerateStream_TruncatedChunk_EmitsErrFrame
// asserts that the streaming path emits a terminal frame with Err
// populated when the API closes the stream with finish_reason="length".
func TestRound46_OpenAI_GenerateStream_TruncatedChunk_EmitsErrFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// First frame: partial content, no finish_reason.
		chunk1 := OpenAIStreamResponse{
			ID:      "chatcmpl-stream-round46",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []struct {
				Index int `json:"index"`
				Delta struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Delta: struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					}{Content: "partial "},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(chunk1)
		// Second frame: terminal with finish_reason=length.
		chunk2 := OpenAIStreamResponse{
			ID:      "chatcmpl-stream-round46",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []struct {
				Index int `json:"index"`
				Delta struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Delta: struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					}{Content: "output"},
					FinishReason: "length",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(chunk2)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(ProviderConfigEntry{
		Type:     ProviderTypeOpenAI,
		APIKey:   "test-key",
		Endpoint: server.URL,
		Enabled:  true,
	})
	require.NoError(t, err)

	ch := make(chan LLMResponse, 8)
	streamErr := provider.GenerateStream(context.Background(), &LLMRequest{
		Model:     "gpt-4o",
		Messages:  []Message{{Role: "user", Content: "Tell me a long story"}},
		MaxTokens: 16,
		Stream:    true,
	}, ch)
	require.NoError(t, streamErr, "GenerateStream MUST NOT fail at the transport layer")
	// Provider closes ch via defer close(ch) on its own — do NOT close here.

	var sawTruncatedErr bool
	var collected []LLMResponse
	for resp := range ch {
		collected = append(collected, resp)
		if resp.Err != nil && errors.Is(resp.Err, ErrResponseTruncated) {
			sawTruncatedErr = true
			assert.Equal(t, "length", resp.FinishReason,
				"Err-bearing chunk MUST preserve FinishReason='length'")
		}
	}
	require.NotEmpty(t, collected, "stream MUST emit at least one chunk")
	assert.True(t, sawTruncatedErr,
		"stream MUST emit a chunk with Err=ErrResponseTruncated when API returned finish_reason=length; collected=%d", len(collected))
}

// TestRound46_Anthropic_Generate_MaxTokens_PopulatesErr asserts
// stop_reason="max_tokens" maps to ErrResponseTruncated.
func TestRound46_Anthropic_Generate_MaxTokens_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:   "msg_round46_maxtokens",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Long answer cut off here"},
			},
			Model:      "claude-3-5-sonnet-latest",
			StopReason: "max_tokens",
			Usage: anthropicUsage{
				InputTokens:  10,
				OutputTokens: 8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:     "anthropic",
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	out, err := provider.Generate(context.Background(), &LLMRequest{
		ID:        uuid.New(),
		Model:     "claude-3-5-sonnet-latest",
		Messages:  []Message{{Role: "user", Content: "Tell me a story"}},
		MaxTokens: 8,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.Content, "Content MUST hold partial output on truncation")
	require.NotNil(t, out.Err)
	assert.True(t, errors.Is(out.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", out.Err)
	assert.Equal(t, "max_tokens", out.FinishReason)
}

// TestRound46_Anthropic_Generate_EndTurn_LeavesErrNil — backward-compat
// invariant: clean stop_reason="end_turn" leaves Err nil.
func TestRound46_Anthropic_Generate_EndTurn_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:   "msg_round46_endturn",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Hello!"},
			},
			Model:      "claude-3-5-sonnet-latest",
			StopReason: "end_turn",
			Usage: anthropicUsage{
				InputTokens:  3,
				OutputTokens: 2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:     "anthropic",
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	out, err := provider.Generate(context.Background(), &LLMRequest{
		ID:       uuid.New(),
		Model:    "claude-3-5-sonnet-latest",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Nil(t, out.Err, "clean stop_reason=end_turn MUST leave Err nil")
}

// TestRound46_Ollama_Generate_DoneReasonLength_PopulatesErr asserts
// done_reason="length" maps to ErrResponseTruncated.
func TestRound46_Ollama_Generate_DoneReasonLength_PopulatesErr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3:latest"}},
			})
			return
		}
		if r.URL.Path == "/api/chat" {
			resp := OllamaAPIResponse{
				Model:      "llama3:latest",
				CreatedAt:  time.Now().Format(time.RFC3339),
				Response:   "Cut off mid-",
				Done:       true,
				DoneReason: "length",
				EvalCount:  4,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider, err := NewOllamaProvider(OllamaConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3:latest",
		Timeout:      10 * time.Second,
	})
	require.NoError(t, err)

	out, err := provider.Generate(context.Background(), &LLMRequest{
		Model:     "llama3:latest",
		Messages:  []Message{{Role: "user", Content: "Tell me a story"}},
		MaxTokens: 4,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.Content, "Content MUST hold partial output")
	require.NotNil(t, out.Err)
	assert.True(t, errors.Is(out.Err, ErrResponseTruncated),
		"Err MUST be ErrResponseTruncated; got %v", out.Err)
	assert.Equal(t, "length", out.FinishReason)
}

// TestRound46_Ollama_Generate_CleanStop_LeavesErrNil — backward-compat.
func TestRound46_Ollama_Generate_CleanStop_LeavesErrNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3:latest"}},
			})
			return
		}
		if r.URL.Path == "/api/chat" {
			resp := OllamaAPIResponse{
				Model:      "llama3:latest",
				CreatedAt:  time.Now().Format(time.RFC3339),
				Response:   "Hello!",
				Done:       true,
				DoneReason: "stop",
				EvalCount:  2,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider, err := NewOllamaProvider(OllamaConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama3:latest",
		Timeout:      10 * time.Second,
	})
	require.NoError(t, err)

	out, err := provider.Generate(context.Background(), &LLMRequest{
		Model:    "llama3:latest",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Nil(t, out.Err, "clean done_reason=stop MUST leave Err nil")
}

// TestRound46_Ollama_OlderVersion_EmptyDoneReason_LeavesErrNil —
// backward-compat for Ollama versions older than 0.1.30 which do not
// emit done_reason. The mapper treats "" as a clean stop.
func TestRound46_Ollama_OlderVersion_EmptyDoneReason_LeavesErrNil(t *testing.T) {
	assert.Nil(t, mapOllamaDoneReasonToErr(""),
		"empty done_reason (older Ollama) MUST map to nil Err")
	assert.Nil(t, mapOllamaDoneReasonToErr("stop"))
	assert.True(t, errors.Is(mapOllamaDoneReasonToErr("length"), ErrResponseTruncated))
}

// TestRound46_OpenAI_FinishReasonMapper_AllCases pins the complete
// mapping for fast regression detection without HTTP fixtures.
func TestRound46_OpenAI_FinishReasonMapper_AllCases(t *testing.T) {
	assert.Nil(t, mapOpenAIFinishReasonToErr(""))
	assert.Nil(t, mapOpenAIFinishReasonToErr("stop"))
	assert.Nil(t, mapOpenAIFinishReasonToErr("tool_calls"))
	assert.Nil(t, mapOpenAIFinishReasonToErr("function_call"))
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("length"), ErrResponseTruncated))
	assert.True(t, errors.Is(mapOpenAIFinishReasonToErr("content_filter"), ErrResponseContentBlocked))
}

// TestRound46_Anthropic_StopReasonMapper_AllCases pins the complete
// mapping for fast regression detection.
func TestRound46_Anthropic_StopReasonMapper_AllCases(t *testing.T) {
	assert.Nil(t, mapAnthropicStopReasonToErr(""))
	assert.Nil(t, mapAnthropicStopReasonToErr("end_turn"))
	assert.Nil(t, mapAnthropicStopReasonToErr("stop_sequence"))
	assert.Nil(t, mapAnthropicStopReasonToErr("tool_use"))
	assert.True(t, errors.Is(mapAnthropicStopReasonToErr("max_tokens"), ErrResponseTruncated))
	assert.True(t, errors.Is(mapAnthropicStopReasonToErr("refusal"), ErrResponseContentBlocked))
	assert.True(t, errors.Is(mapAnthropicStopReasonToErr("safety"), ErrResponseContentBlocked))
}

// TestRound46_ToolProvider_HonorsErr_OnFirstLoop asserts the
// tool_provider.go:201 anchor closure: when a streamed LLMResponse
// carries a non-nil Err, the tool provider surfaces it on the
// ToolStreamChunk's Error field instead of silently dropping it.
func TestRound46_ToolProvider_HonorsErr_OnFirstLoop(t *testing.T) {
	mockBase := &MockProvider{
		generateStreamFunc: func(ctx context.Context, req *LLMRequest, ch chan<- LLMResponse) error {
			ch <- LLMResponse{ID: uuid.New(), Content: "partial "}
			ch <- LLMResponse{
				ID:           uuid.New(),
				Content:      "answer",
				FinishReason: "length",
				Err:          ErrResponseTruncated,
			}
			close(ch)
			return nil
		},
	}
	tp := NewToolCallingProvider(mockBase)

	chunkCh, err := tp.StreamWithTools(context.Background(), ToolGenerationRequest{
		ID:     uuid.New(),
		Prompt: "Tell me a story",
		Tools:  nil, // no tools → first loop's terminal handling
	})
	require.NoError(t, err)

	var sawErrChunk bool
	var collected []ToolStreamChunk
	for chunk := range chunkCh {
		collected = append(collected, chunk)
		if chunk.Error != "" && strings.Contains(chunk.Error, ErrResponseTruncated.Error()) {
			sawErrChunk = true
		}
	}
	require.NotEmpty(t, collected, "tool stream MUST emit at least one chunk")
	assert.True(t, sawErrChunk,
		"tool_provider MUST surface LLMResponse.Err on a chunk's Error field; collected=%+v", collected)
}

// TestRound46_RoundTripAnchor verifies a forensic-anchor comment is
// present in the file by re-asserting the literal sentinel error
// strings remain stable (so external callers / cache consumers that
// matched on the message survive across revisions). If a future
// revision changes the message, that revision MUST update this
// pinning test in the same commit (CONST-057-style closure discipline).
func TestRound46_RoundTripAnchor(t *testing.T) {
	const truncatedMsg = "llm response: truncated due to max-tokens limit; Content contains partial output"
	const blockedMsg = "llm response: blocked by content-safety filter; Content may be empty or partial"
	assert.Equal(t, truncatedMsg, ErrResponseTruncated.Error())
	assert.Equal(t, blockedMsg, ErrResponseContentBlocked.Error())
	// Defensive: the sentinels carry stable JSON type labels.
	assert.Equal(t, llmErrTypeResponseTruncated, llmErrTypeForSentinel(ErrResponseTruncated))
	assert.Equal(t, llmErrTypeResponseContentBlocked, llmErrTypeForSentinel(ErrResponseContentBlocked))
	assert.Equal(t, llmErrTypeGeneric, llmErrTypeForSentinel(fmt.Errorf("some other err")))
}
