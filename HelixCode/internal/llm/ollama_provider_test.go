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

func TestNewOllamaProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Create mock server for model discovery
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				response := map[string]interface{}{
					"models": []map[string]interface{}{
						{
							"name":        "llama3:latest",
							"modified_at": time.Now().Format(time.RFC3339),
							"size":        4000000000,
							"digest":      "abc123",
							"details": map[string]interface{}{
								"format":             "gguf",
								"family":             "llama",
								"parameter_size":     "8B",
								"quantization_level": "Q4_0",
							},
						},
					},
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := OllamaConfig{
			BaseURL:       server.URL,
			DefaultModel:  "llama3:latest",
			Timeout:       30 * time.Second,
			StreamEnabled: false,
		}

		provider, err := NewOllamaProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})

	t.Run("WithoutModels", func(t *testing.T) {
		// Create mock server that returns empty models
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				response := map[string]interface{}{
					"models": []map[string]interface{}{},
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := OllamaConfig{
			BaseURL:       server.URL,
			DefaultModel:  "llama3:latest",
			Timeout:       30 * time.Second,
			StreamEnabled: false,
		}

		provider, err := NewOllamaProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

func TestOllamaProvider_GetType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"models": []map[string]interface{}{}}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := OllamaConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeLocal, provider.GetType())
}

func TestOllamaProvider_GetName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"models": []map[string]interface{}{}}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := OllamaConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "ollama", provider.GetName())
}

func TestOllamaProvider_GetModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			response := map[string]interface{}{
				"models": []map[string]interface{}{
					{
						"name":        "llama3:latest",
						"modified_at": time.Now().Format(time.RFC3339),
						"size":        4000000000,
						"details": map[string]interface{}{
							"format":         "gguf",
							"family":         "llama",
							"parameter_size": "8B",
						},
					},
					{
						"name":        "codellama:7b",
						"modified_at": time.Now().Format(time.RFC3339),
						"size":        3800000000,
						"details": map[string]interface{}{
							"format":         "gguf",
							"family":         "llama",
							"parameter_size": "7B",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := OllamaConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	require.NoError(t, err)

	models := provider.GetModels()
	assert.Len(t, models, 2)
}

func TestOllamaProvider_GetCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"models": []map[string]interface{}{}}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := OllamaConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	require.NoError(t, err)

	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, CapabilityTextGeneration)
	assert.Contains(t, capabilities, CapabilityCodeGeneration)
}

func TestOllamaProvider_Generate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				response := map[string]interface{}{
					"models": []map[string]interface{}{
						{"name": "llama3:latest"},
					},
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			if r.URL.Path == "/api/chat" {
				response := OllamaAPIResponse{
					Model:     "llama3:latest",
					CreatedAt: time.Now().Format(time.RFC3339),
					Response:  "Hello from Ollama!",
					Done:      true,
					EvalCount: 10,
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := OllamaConfig{
			BaseURL:      server.URL,
			DefaultModel: "llama3:latest",
			Timeout:      30 * time.Second,
		}

		provider, err := NewOllamaProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "llama3:latest",
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
		assert.Equal(t, "Hello from Ollama!", response.Content)
	})

	t.Run("APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				response := map[string]interface{}{"models": []map[string]interface{}{}}
				json.NewEncoder(w).Encode(response)
				return
			}
			if r.URL.Path == "/api/chat" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := OllamaConfig{
			BaseURL:      server.URL,
			DefaultModel: "llama3:latest",
			Timeout:      30 * time.Second,
		}

		provider, err := NewOllamaProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "llama3:latest",
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

func TestOllamaProvider_IsAvailable(t *testing.T) {
	t.Run("Available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				response := map[string]interface{}{
					"models": []map[string]interface{}{
						{"name": "llama3:latest"},
					},
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := OllamaConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewOllamaProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.True(t, available)
	})

	t.Run("Unavailable", func(t *testing.T) {
		// Use a server that immediately closes connections
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				response := map[string]interface{}{"models": []map[string]interface{}{}}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		serverURL := server.URL
		server.Close() // Close immediately to simulate unavailability

		config := OllamaConfig{
			BaseURL: serverURL,
			Timeout: 1 * time.Second,
		}

		provider, err := NewOllamaProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})
}

func TestOllamaProvider_GetHealth(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				response := map[string]interface{}{
					"models": []map[string]interface{}{
						{"name": "llama3:latest"},
						{"name": "codellama:7b"},
					},
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := OllamaConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
		}

		provider, err := NewOllamaProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "healthy", health.Status)
		assert.Equal(t, 2, health.ModelCount)
	})
}

func TestOllamaProvider_Close(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"models": []map[string]interface{}{}}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := OllamaConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
	assert.False(t, provider.isRunning)
}
