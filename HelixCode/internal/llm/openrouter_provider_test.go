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

func TestNewOpenRouterProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		config := ProviderConfigEntry{
			Type:    ProviderTypeOpenRouter,
			APIKey:  "test-api-key",
			Enabled: true,
		}

		provider, err := NewOpenRouterProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "https://openrouter.ai/api/v1", provider.endpoint)
		assert.Equal(t, "test-api-key", provider.apiKey)
	})

	t.Run("CustomEndpoint", func(t *testing.T) {
		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenRouter,
			APIKey:   "test-api-key",
			Endpoint: "https://custom.openrouter.ai/v1",
			Enabled:  true,
		}

		provider, err := NewOpenRouterProvider(config)
		require.NoError(t, err)
		assert.Equal(t, "https://custom.openrouter.ai/v1", provider.endpoint)
	})

	t.Run("MissingAPIKey", func(t *testing.T) {
		config := ProviderConfigEntry{
			Type:    ProviderTypeOpenRouter,
			Enabled: true,
		}

		provider, err := NewOpenRouterProvider(config)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "API key is required")
	})
}

func TestOpenRouterProvider_GetType(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenRouter,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenRouterProvider(config)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeOpenRouter, provider.GetType())
}

func TestOpenRouterProvider_GetName(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenRouter,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenRouterProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "OpenRouter", provider.GetName())
}

func TestOpenRouterProvider_GetModels(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenRouter,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenRouterProvider(config)
	require.NoError(t, err)

	models := provider.GetModels()
	assert.NotEmpty(t, models)

	// Check that models have proper structure
	for _, model := range models {
		assert.NotEmpty(t, model.Name)
	}
}

func TestOpenRouterProvider_GetCapabilities(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenRouter,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenRouterProvider(config)
	require.NoError(t, err)

	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, CapabilityTextGeneration)
	assert.Contains(t, capabilities, CapabilityCodeGeneration)
}

func TestOpenRouterProvider_Generate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/chat/completions")
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			response := map[string]interface{}{
				"id":      "chatcmpl-test123",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "anthropic/claude-3-opus",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello from OpenRouter!",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenRouter,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenRouterProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "anthropic/claude-3-opus",
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
		assert.Equal(t, "Hello from OpenRouter!", response.Content)
		assert.Equal(t, 15, response.Usage.TotalTokens)
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
			Type:     ProviderTypeOpenRouter,
			APIKey:   "invalid-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenRouterProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "anthropic/claude-3-opus",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ctx := context.Background()
		response, err := provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Nil(t, response)
	})
}

func TestOpenRouterProvider_IsAvailable(t *testing.T) {
	t.Run("Available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "anthropic/claude-3-opus"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenRouter,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenRouterProvider(config)
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
			Type:     ProviderTypeOpenRouter,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenRouterProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})
}

func TestOpenRouterProvider_GetHealth(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "anthropic/claude-3-opus"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeOpenRouter,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenRouterProvider(config)
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
			Type:     ProviderTypeOpenRouter,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewOpenRouterProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		assert.Error(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
	})
}

func TestOpenRouterProvider_Close(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeOpenRouter,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewOpenRouterProvider(config)
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
}
