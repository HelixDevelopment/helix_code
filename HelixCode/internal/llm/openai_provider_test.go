package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		config := ProviderConfigEntry{
			Type:    ProviderTypeOpenAI,
			APIKey:  "test-api-key",
			Enabled: true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "https://api.openai.com/v1", provider.endpoint)
		assert.Equal(t, "test-api-key", provider.apiKey)
	})

	t.Run("CustomEndpoint", func(t *testing.T) {
		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "test-api-key",
			Endpoint: "https://custom.openai.com/v1",
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)
		assert.Equal(t, "https://custom.openai.com/v1", provider.endpoint)
	})

	t.Run("MissingAPIKey", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		config := ProviderConfigEntry{
			Type:    ProviderTypeOpenAI,
			Enabled: true,
		}

		provider, err := NewOpenAIProvider(config)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "API key is required")
	})
}

func TestOpenAIProvider_GetType(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeOpenAI, provider.GetType())
}

func TestOpenAIProvider_GetName(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "OpenAI", provider.GetName())
}

func TestOpenAIProvider_GetModels(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)

	models := provider.GetModels()
	assert.NotEmpty(t, models)

	// Check that common models are present
	modelNames := make([]string, len(models))
	for i, m := range models {
		modelNames[i] = m.Name
	}

	assert.Contains(t, modelNames, "gpt-4o")
	assert.Contains(t, modelNames, "gpt-4-turbo")
}

func TestOpenAIProvider_GetCapabilities(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)

	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, CapabilityTextGeneration)
	assert.Contains(t, capabilities, CapabilityCodeGeneration)
	assert.Contains(t, capabilities, CapabilityVision)
}

func TestOpenAIProvider_Generate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/chat/completions", r.URL.Path)
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			response := map[string]interface{}{
				"id":      "chatcmpl-test123",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello! How can I help you today?",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 8,
					"total_tokens":      18,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "gpt-4o",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			MaxTokens:   100,
			Temperature: 0.7,
		}

		ctx := context.Background()
		response, err := provider.Generate(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "Hello! How can I help you today?", response.Content)
		assert.Equal(t, 18, response.Usage.TotalTokens)
	})

	t.Run("APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			response := map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid API key",
					"type":    "invalid_api_key",
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "invalid-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "gpt-4o",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ctx := context.Background()
		response, err := provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "gpt-4o",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		response, err := provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Nil(t, response)
	})
}

func TestOpenAIProvider_IsAvailable(t *testing.T) {
	t.Run("Available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "gpt-4o"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.True(t, available)
	})

	t.Run("Unavailable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})
}

func TestOpenAIProvider_GetHealth(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "gpt-4o"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "healthy", health.Status)
	})

	t.Run("Unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		// GetHealth returns error and unhealthy status when health check fails
		assert.Error(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
	})
}

func TestOpenAIProvider_Close(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
}
