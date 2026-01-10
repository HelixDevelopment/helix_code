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

func setupKoboldAITestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/model":
			// Model discovery endpoint
			response := map[string]interface{}{
				"result": "llama-7b",
			}
			json.NewEncoder(w).Encode(response)

		case "/api/v1/generate":
			// Generation endpoint
			assert.Equal(t, "POST", r.Method)

			response := KoboldAIResponse{
				Results: []KoboldAIResult{
					{
						Text:      "Hello from KoboldAI!",
						Generated: true,
					},
				},
			}
			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestNewKoboldAIProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL:      server.URL,
			DefaultModel: "llama-7b",
			Timeout:      30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.True(t, provider.isRunning)
	})

	t.Run("WithAPIKey", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify API key header
			if r.Header.Get("Authorization") != "Bearer test-api-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			response := map[string]interface{}{
				"result": "llama-7b",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL:      server.URL,
			APIKey:       "test-api-key",
			DefaultModel: "llama-7b",
			Timeout:      30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})

	t.Run("CustomHeaders", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify custom header
			if r.Header.Get("X-Custom-Header") != "custom-value" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			response := map[string]interface{}{
				"result": "llama-7b",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL:      server.URL,
			DefaultModel: "llama-7b",
			Timeout:      30 * time.Second,
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

func TestKoboldAIProvider_GetType(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeKoboldAI, provider.GetType())
}

func TestKoboldAIProvider_GetName(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "koboldai", provider.GetName())
}

func TestKoboldAIProvider_GetModels(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)

	models := provider.GetModels()
	assert.NotEmpty(t, models)

	// Verify model has correct structure
	for _, model := range models {
		assert.NotEmpty(t, model.Name)
		assert.Equal(t, ProviderTypeKoboldAI, model.Provider)
	}
}

func TestKoboldAIProvider_GetCapabilities(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)

	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, CapabilityTextGeneration)
	assert.Contains(t, capabilities, CapabilityCodeGeneration)
	assert.Contains(t, capabilities, CapabilityCodeAnalysis)
	assert.Contains(t, capabilities, CapabilityPlanning)
	assert.Contains(t, capabilities, CapabilityDebugging)
	assert.Contains(t, capabilities, CapabilityRefactoring)
	assert.Contains(t, capabilities, CapabilityTesting)
}

func TestKoboldAIProvider_Generate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL:      server.URL,
			DefaultModel: "llama-7b",
			Timeout:      30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "llama-7b",
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
		assert.Equal(t, "Hello from KoboldAI!", response.Content)
	})

	t.Run("APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/model" {
				response := map[string]interface{}{"result": "llama-7b"}
				json.NewEncoder(w).Encode(response)
				return
			}
			if r.URL.Path == "/api/v1/generate" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL:      server.URL,
			DefaultModel: "llama-7b",
			Timeout:      30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "llama-7b",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ctx := context.Background()
		response, err := provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		// Stop the provider
		provider.Close()

		request := &LLMRequest{
			Model: "llama-7b",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ctx := context.Background()
		response, err := provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Equal(t, ErrProviderUnavailable, err)
	})
}

func TestKoboldAIProvider_GenerateStream(t *testing.T) {
	t.Run("FallbackToNonStreaming", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL:          server.URL,
			DefaultModel:     "llama-7b",
			Timeout:          30 * time.Second,
			StreamingSupport: false, // Streaming disabled
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "llama-7b",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			MaxTokens: 100,
		}

		ch := make(chan LLMResponse, 10)
		ctx := context.Background()

		err = provider.GenerateStream(ctx, request, ch)
		require.NoError(t, err)

		// Should receive the response
		response := <-ch
		assert.Equal(t, "Hello from KoboldAI!", response.Content)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		provider.Close()

		request := &LLMRequest{
			Model: "llama-7b",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ch := make(chan LLMResponse, 10)
		ctx := context.Background()

		err = provider.GenerateStream(ctx, request, ch)
		assert.Error(t, err)
		assert.Equal(t, ErrProviderUnavailable, err)
	})
}

func TestKoboldAIProvider_IsAvailable(t *testing.T) {
	t.Run("Available", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
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

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		provider.Close()

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})
}

func TestKoboldAIProvider_GetHealth(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
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

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		assert.Error(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		provider.Close()

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err) // GetHealth doesn't return error when stopped
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
	})
}

func TestKoboldAIProvider_Close(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)
	assert.True(t, provider.isRunning)

	err = provider.Close()
	assert.NoError(t, err)
	assert.False(t, provider.isRunning)
}

func TestKoboldAIProvider_ConvertMessagesToPrompt(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	prompt := provider.convertMessagesToPrompt(messages)

	assert.Contains(t, prompt, "System: You are a helpful assistant.")
	assert.Contains(t, prompt, "User: Hello")
	assert.Contains(t, prompt, "Assistant: Hi there!")
	assert.Contains(t, prompt, "User: How are you?")
	assert.True(t, len(prompt) > 0)
}

func TestKoboldAIProvider_GetAPIURL(t *testing.T) {
	t.Run("WithBaseURL", func(t *testing.T) {
		server := setupKoboldAITestServer(t)
		defer server.Close()

		config := KoboldAIConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewKoboldAIProvider(config)
		require.NoError(t, err)

		url := provider.getAPIURL("/api/v1/model")
		assert.Equal(t, server.URL+"/api/v1/model", url)
	})

	t.Run("WithTrailingSlash", func(t *testing.T) {
		provider := &KoboldAIProvider{
			config: KoboldAIConfig{
				BaseURL: "http://localhost:5001/",
			},
		}

		url := provider.getAPIURL("/api/v1/model")
		assert.Equal(t, "http://localhost:5001/api/v1/model", url)
	})

	t.Run("DefaultBaseURL", func(t *testing.T) {
		provider := &KoboldAIProvider{
			config: KoboldAIConfig{},
		}

		url := provider.getAPIURL("/api/v1/model")
		assert.Equal(t, "http://localhost:5001/api/v1/model", url)
	})
}

func TestKoboldAIProvider_GetModelName(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL:      server.URL,
		DefaultModel: "default-model",
		Timeout:      30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)

	// Test with requested model
	name := provider.getModelName("requested-model")
	assert.Equal(t, "requested-model", name)

	// Test with empty model (should use default)
	name = provider.getModelName("")
	assert.Equal(t, "default-model", name)
}

func TestKoboldAIProvider_UpdateHealth(t *testing.T) {
	server := setupKoboldAITestServer(t)
	defer server.Close()

	config := KoboldAIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewKoboldAIProvider(config)
	require.NoError(t, err)

	// Test healthy update
	provider.updateHealth("healthy", 50*time.Millisecond, 0)
	assert.Equal(t, "healthy", provider.lastHealth.Status)
	assert.Equal(t, 50*time.Millisecond, provider.lastHealth.Latency)
	assert.Equal(t, 0, provider.lastHealth.ErrorCount)

	// Test unhealthy update
	provider.updateHealth("unhealthy", 100*time.Millisecond, 5)
	assert.Equal(t, "unhealthy", provider.lastHealth.Status)
	assert.Equal(t, 100*time.Millisecond, provider.lastHealth.Latency)
	assert.Equal(t, 5, provider.lastHealth.ErrorCount)
}
