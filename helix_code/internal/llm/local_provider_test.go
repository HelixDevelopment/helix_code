package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalProvider(t *testing.T) {
	t.Run("CreatesProviderWithCustomEndpoint", func(t *testing.T) {
		config := ProviderConfigEntry{
			Endpoint: "http://localhost:11435",
		}

		provider, err := NewLocalProvider(config)
		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "http://localhost:11435", provider.endpoint)
		assert.NotNil(t, provider.httpClient)
		assert.NotNil(t, provider.lastHealth)
	})

	t.Run("DefaultsToStandardEndpoint", func(t *testing.T) {
		config := ProviderConfigEntry{}

		provider, err := NewLocalProvider(config)
		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "http://localhost:11434", provider.endpoint)
	})
}

func TestLocalProvider_GetType(t *testing.T) {
	provider := &LocalProvider{}
	assert.Equal(t, ProviderTypeLocal, provider.GetType())
}

func TestLocalProvider_GetName(t *testing.T) {
	provider := &LocalProvider{}
	assert.Equal(t, "Local LLama.cpp", provider.GetName())
}

func TestLocalProvider_GetModels(t *testing.T) {
	t.Run("ReturnsInitializedModels", func(t *testing.T) {
		provider := &LocalProvider{
			models: []ModelInfo{
				{Name: "llama-7b"},
				{Name: "codellama-7b"},
			},
		}

		models := provider.GetModels()
		assert.Len(t, models, 2)
		assert.Equal(t, "llama-7b", models[0].Name)
	})

	t.Run("ReturnsEmptyForUninitializedProvider", func(t *testing.T) {
		provider := &LocalProvider{}
		models := provider.GetModels()
		assert.Empty(t, models)
	})
}

func TestLocalProvider_GetCapabilities(t *testing.T) {
	provider := &LocalProvider{}
	capabilities := provider.GetCapabilities()

	assert.Contains(t, capabilities, CapabilityTextGeneration)
	assert.Contains(t, capabilities, CapabilityCodeGeneration)
	assert.Contains(t, capabilities, CapabilityCodeAnalysis)
	assert.Contains(t, capabilities, CapabilityPlanning)
	assert.Contains(t, capabilities, CapabilityDebugging)
	assert.Contains(t, capabilities, CapabilityRefactoring)
	assert.Contains(t, capabilities, CapabilityTesting)
}

func TestLocalProvider_GetHealth(t *testing.T) {
	t.Run("ReturnsHealthyWithWorkingServer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"models": []map[string]interface{}{
						{"name": "llama-7b"},
						{"name": "codellama-7b"},
					},
				})
			}
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{Status: "unknown"},
		}

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, "healthy", health.Status)
		assert.Equal(t, 2, health.ModelCount)
		assert.Equal(t, 0, health.ErrorCount)
	})

	t.Run("ReturnsUnhealthyWithFailingServer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{Status: "unknown", ErrorCount: 0},
		}

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		assert.Error(t, err)
		assert.Equal(t, "unhealthy", health.Status)
		assert.Equal(t, 1, health.ErrorCount)
	})

	t.Run("ReturnsDegradedWithBadJson", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{Status: "unknown"},
		}

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, "degraded", health.Status)
	})
}

func TestLocalProvider_IsAvailable(t *testing.T) {
	t.Run("TrueWhenHealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{Status: "unknown"},
		}

		ctx := context.Background()
		assert.True(t, provider.IsAvailable(ctx))
	})

	t.Run("FalseWhenUnhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{Status: "unknown"},
		}

		ctx := context.Background()
		assert.False(t, provider.IsAvailable(ctx))
	})
}

func TestLocalProvider_Close(t *testing.T) {
	provider := &LocalProvider{
		httpClient: &http.Client{},
	}

	err := provider.Close()
	assert.NoError(t, err)
}

func TestLocalProvider_Generate(t *testing.T) {
	t.Run("SuccessfulGeneration", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/generate" {
				json.NewEncoder(w).Encode(OllamaResponse{
					Model:           "llama-7b",
					Response:        "Hello, how can I help you?",
					Done:            true,
					PromptEvalCount: 10,
					EvalCount:       20,
				})
			}
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{},
		}

		ctx := context.Background()
		request := &LLMRequest{
			ID:          uuid.New(),
			Model:       "llama-7b",
			Messages:    []Message{{Role: "user", Content: "Hello"}},
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   100,
		}

		response, err := provider.Generate(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, "Hello, how can I help you?", response.Content)
		assert.Equal(t, 10, response.Usage.PromptTokens)
		assert.Equal(t, 20, response.Usage.CompletionTokens)
		assert.Equal(t, 30, response.Usage.TotalTokens)
	})

	t.Run("FailsWithServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{},
		}

		ctx := context.Background()
		request := &LLMRequest{
			ID:       uuid.New(),
			Model:    "llama-7b",
			Messages: []Message{{Role: "user", Content: "Hello"}},
		}

		response, err := provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "ollama request failed")
	})
}

func TestLocalProvider_GenerateStream(t *testing.T) {
	t.Run("StreamsResponses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/generate" {
				// Send multiple stream responses
				encoder := json.NewEncoder(w)
				encoder.Encode(OllamaStreamResponse{Response: "Hello", Done: false})
				encoder.Encode(OllamaStreamResponse{Response: " World", Done: false})
				encoder.Encode(OllamaStreamResponse{Response: "!", Done: true})
			}
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{},
		}

		ctx := context.Background()
		request := &LLMRequest{
			ID:       uuid.New(),
			Model:    "llama-7b",
			Messages: []Message{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		ch := make(chan LLMResponse, 10)
		go func() {
			err := provider.GenerateStream(ctx, request, ch)
			assert.NoError(t, err)
		}()

		var responses []string
		for response := range ch {
			responses = append(responses, response.Content)
		}

		assert.Len(t, responses, 3)
	})
}

func TestLocalProvider_convertToOllamaRequest(t *testing.T) {
	provider := &LocalProvider{}

	t.Run("ConvertsMessages", func(t *testing.T) {
		request := &LLMRequest{
			Model: "llama-7b",
			Messages: []Message{
				{Role: "system", Content: "You are a helpful assistant"},
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			Temperature: 0.7,
			TopP:        0.9,
			MaxTokens:   100,
			Stream:      false,
		}

		ollamaReq, err := provider.convertToOllamaRequest(request)
		require.NoError(t, err)
		assert.Equal(t, "llama-7b", ollamaReq.Model)
		assert.Contains(t, ollamaReq.Prompt, "System: You are a helpful assistant")
		assert.Contains(t, ollamaReq.Prompt, "User: Hello")
		assert.Contains(t, ollamaReq.Prompt, "Assistant: Hi there!")
		assert.Equal(t, 0.7, ollamaReq.Options["temperature"])
		assert.Equal(t, 0.9, ollamaReq.Options["top_p"])
		assert.Equal(t, 100, ollamaReq.Options["num_predict"])
	})
}

func TestLocalProvider_convertFromOllamaResponse(t *testing.T) {
	provider := &LocalProvider{}

	ollamaResp := &OllamaResponse{
		Response:        "Generated content",
		PromptEvalCount: 50,
		EvalCount:       100,
	}

	requestID := uuid.New()
	processingTime := 500 * time.Millisecond

	llmResp := provider.convertFromOllamaResponse(ollamaResp, requestID, processingTime)

	assert.Equal(t, requestID, llmResp.RequestID)
	assert.Equal(t, "Generated content", llmResp.Content)
	assert.Equal(t, 50, llmResp.Usage.PromptTokens)
	assert.Equal(t, 100, llmResp.Usage.CompletionTokens)
	assert.Equal(t, 150, llmResp.Usage.TotalTokens)
	assert.Equal(t, "stop", llmResp.FinishReason)
	assert.Equal(t, processingTime, llmResp.ProcessingTime)
}

func TestLocalProvider_updateHealth(t *testing.T) {
	provider := &LocalProvider{
		lastHealth: &ProviderHealth{},
	}

	latency := 100 * time.Millisecond
	provider.updateHealth("healthy", latency, 0)

	assert.Equal(t, "healthy", provider.lastHealth.Status)
	assert.Equal(t, latency, provider.lastHealth.Latency)
	assert.Equal(t, 0, provider.lastHealth.ErrorCount)
	assert.False(t, provider.lastHealth.LastCheck.IsZero())
}

func TestLocalProvider_initializeModels(t *testing.T) {
	t.Run("InitializesFromServer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"models": []map[string]interface{}{
						{"name": "llama-7b", "size": 4000000000},
						{"name": "codellama-7b-vision", "size": 4500000000},
					},
				})
			}
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{},
		}

		err := provider.initializeModels()
		require.NoError(t, err)
		assert.Len(t, provider.models, 2)

		// Check vision model detection
		for _, model := range provider.models {
			if model.Name == "codellama-7b-vision" {
				assert.True(t, model.SupportsVision)
			}
		}
	})

	t.Run("FailsWithServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		provider := &LocalProvider{
			endpoint:   server.URL,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			lastHealth: &ProviderHealth{},
		}

		err := provider.initializeModels()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get models")
	})
}

func TestOllamaRequest_Struct(t *testing.T) {
	req := OllamaRequest{
		Model:  "llama-7b",
		Prompt: "Hello",
		Options: map[string]interface{}{
			"temperature": 0.7,
		},
		Stream: true,
	}

	assert.Equal(t, "llama-7b", req.Model)
	assert.Equal(t, "Hello", req.Prompt)
	assert.True(t, req.Stream)
}

func TestOllamaResponse_Struct(t *testing.T) {
	resp := OllamaResponse{
		Model:           "llama-7b",
		CreatedAt:       "2025-01-01T00:00:00Z",
		Response:        "Hello",
		Done:            true,
		TotalDuration:   1000000,
		PromptEvalCount: 10,
		EvalCount:       20,
	}

	assert.Equal(t, "llama-7b", resp.Model)
	assert.Equal(t, "Hello", resp.Response)
	assert.True(t, resp.Done)
	assert.Equal(t, 10, resp.PromptEvalCount)
	assert.Equal(t, 20, resp.EvalCount)
}

func TestOllamaStreamResponse_Struct(t *testing.T) {
	resp := OllamaStreamResponse{
		Model:     "llama-7b",
		CreatedAt: "2025-01-01T00:00:00Z",
		Response:  "Chunk",
		Done:      false,
	}

	assert.Equal(t, "llama-7b", resp.Model)
	assert.Equal(t, "Chunk", resp.Response)
	assert.False(t, resp.Done)
}
