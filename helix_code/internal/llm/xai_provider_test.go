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

func TestNewXAIProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		config := ProviderConfigEntry{
			Type:    ProviderTypeXAI,
			APIKey:  "test-api-key",
			Enabled: true,
		}

		provider, err := NewXAIProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "https://api.x.ai/v1", provider.endpoint)
		assert.Equal(t, "test-api-key", provider.apiKey)
	})

	t.Run("CustomEndpoint", func(t *testing.T) {
		config := ProviderConfigEntry{
			Type:     ProviderTypeXAI,
			APIKey:   "test-api-key",
			Endpoint: "https://custom.xai.com/v1",
			Enabled:  true,
		}

		provider, err := NewXAIProvider(config)
		require.NoError(t, err)
		assert.Equal(t, "https://custom.xai.com/v1", provider.endpoint)
	})

	t.Run("MissingAPIKey", func(t *testing.T) {
		t.Setenv("XAI_API_KEY", "")
		config := ProviderConfigEntry{
			Type:    ProviderTypeXAI,
			Enabled: true,
		}

		provider, err := NewXAIProvider(config)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "API key is required")
	})
}

func TestXAIProvider_GetType(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewXAIProvider(config)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeXAI, provider.GetType())
}

func TestXAIProvider_GetName(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewXAIProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "XAI (Grok)", provider.GetName())
}

func TestXAIProvider_GetModels(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewXAIProvider(config)
	require.NoError(t, err)

	models := provider.GetModels()
	assert.NotEmpty(t, models)

	// Check that models have proper structure
	for _, model := range models {
		assert.NotEmpty(t, model.Name)
		assert.Equal(t, ProviderTypeXAI, model.Provider)
	}
}

func TestXAIProvider_GetCapabilities(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewXAIProvider(config)
	require.NoError(t, err)

	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, CapabilityTextGeneration)
	assert.Contains(t, capabilities, CapabilityCodeGeneration)
}

func TestXAIProvider_Generate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/chat/completions", r.URL.Path)
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			response := map[string]interface{}{
				"id":      "chatcmpl-test123",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "grok-2",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello from Grok!",
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
			Type:     ProviderTypeXAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewXAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "grok-2",
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
		assert.Equal(t, "Hello from Grok!", response.Content)
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
			Type:     ProviderTypeXAI,
			APIKey:   "invalid-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewXAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "grok-2",
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

func TestXAIProvider_IsAvailable(t *testing.T) {
	t.Run("Available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "grok-2"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeXAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewXAIProvider(config)
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
			Type:     ProviderTypeXAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewXAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})
}

func TestXAIProvider_GetHealth(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "grok-2"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := ProviderConfigEntry{
			Type:     ProviderTypeXAI,
			APIKey:   "test-api-key",
			Endpoint: server.URL,
			Enabled:  true,
		}

		provider, err := NewXAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "healthy", health.Status)
	})
}

func TestXAIProvider_Close(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXAI,
		APIKey:  "test-api-key",
		Enabled: true,
	}

	provider, err := NewXAIProvider(config)
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
}
