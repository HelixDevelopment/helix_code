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

func TestNewLlamaCPPProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		config := LlamaConfig{
			Model:     "/path/to/model.gguf",
			ContextSize:   4096,
			GPUEnabled:    true,
			GPULayers:     35,
			Threads:       8,
			ServerHost:    "localhost",
			ServerPort:    8080,
			ServerTimeout: 60 * time.Second,
		}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.True(t, provider.isRunning)
		assert.Equal(t, config.Model, provider.config.Model)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		config := LlamaConfig{}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.True(t, provider.isRunning)
	})
}

func TestLlamaCPPProvider_GetType(t *testing.T) {
	config := LlamaConfig{
		Model:   "/path/to/model.gguf",
		ContextSize: 4096,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeLocal, provider.GetType())
}

func TestLlamaCPPProvider_GetName(t *testing.T) {
	config := LlamaConfig{
		Model:   "/path/to/model.gguf",
		ContextSize: 4096,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "llama-cpp", provider.GetName())
}

func TestLlamaCPPProvider_GetModels(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LlamaCPP test in short mode (SKIP-OK: #short-mode)")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/models", r.URL.Path)
		response := map[string]interface{}{
			"models": []map[string]interface{}{
				{"name": "llama-7b"},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := LlamaConfig{
		Model:      "/path/to/llama-7b.gguf",
		ContextSize: 4096,
		ServerHost: server.URL,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	models := provider.GetModels()
	require.Len(t, models, 1)

	model := models[0]
	assert.Equal(t, "llama-7b", model.Name)
	assert.Equal(t, ProviderTypeLocal, model.Provider)
}

func TestLlamaCPPProvider_GetCapabilities(t *testing.T) {
	config := LlamaConfig{
		Model:   "/path/to/model.gguf",
		ContextSize: 4096,
	}

	provider, err := NewLlamaCPPProvider(config)
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

func TestLlamaCPPProvider_Generate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LlamaCPP test in short mode (SKIP-OK: #short-mode)")
	}
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/v1/completions", r.URL.Path)
			response := map[string]interface{}{
				"content": "Hello! How can I help you today?",
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 20,
					"total_tokens":      30,
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := LlamaConfig{
			Model:      "/path/to/model.gguf",
			ContextSize: 4096,
			ServerHost: server.URL,
		}

		provider, err := NewLlamaCPPProvider(config)
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
		assert.Contains(t, response.Content, "Hello!")
		assert.Equal(t, 30, response.Usage.TotalTokens)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		config := LlamaConfig{
			Model: "/path/to/model.gguf",
		}

		provider, err := NewLlamaCPPProvider(config)
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

func TestLlamaCPPProvider_GenerateStream(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LlamaCPP test in short mode (SKIP-OK: #short-mode)")
	}
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/completion", r.URL.Path)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			require.True(t, ok)
			w.Write([]byte("data: {\"content\": \"Hello\"}\n\n"))
			flusher.Flush()
			w.Write([]byte("data: {\"content\": \" world\"}\n\n"))
			flusher.Flush()
			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
		}))
		defer server.Close()

		config := LlamaConfig{
			Model:      "/path/to/model.gguf",
			ContextSize: 4096,
			ServerHost: server.URL,
		}

		provider, err := NewLlamaCPPProvider(config)
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

		go func() {
			// Channel-ownership contract (Provider.GenerateStream): the provider
			// is the SOLE closer of ch. The test must NOT close ch here — doing so
			// would be a double-close panic now that LlamaCPPProvider correctly
			// closes the channel itself (server defect #5 fix). The range loop
			// below terminates on the provider's own close.
			err := provider.GenerateStream(ctx, request, ch)
			assert.NoError(t, err)
		}()

		var chunks []string
		for response := range ch {
			chunks = append(chunks, response.Content)
		}

		assert.NotEmpty(t, chunks)
		assert.Equal(t, []string{"Hello", " world"}, chunks)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			// Send one chunk then wait forever
			w.Write([]byte("data: {\"content\": \"Hello\"}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			<-r.Context().Done()
		}))
		defer server.Close()

		config := LlamaConfig{
			Model:      "/path/to/model.gguf",
			ServerHost: server.URL,
		}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "llama-7b",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ch := make(chan LLMResponse, 10)
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			// Cancel after a short delay to let the request start
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err = provider.GenerateStream(ctx, request, ch)
		assert.Error(t, err)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		config := LlamaConfig{
			Model: "/path/to/model.gguf",
		}

		provider, err := NewLlamaCPPProvider(config)
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

func TestLlamaCPPProvider_IsAvailable(t *testing.T) {
	t.Run("Available", func(t *testing.T) {
		config := LlamaConfig{
			Model: "/path/to/model.gguf",
		}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.True(t, available)
	})

	t.Run("Unavailable", func(t *testing.T) {
		config := LlamaConfig{
			Model: "/path/to/model.gguf",
		}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)

		provider.Close()

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})
}

func TestLlamaCPPProvider_GetHealth(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		config := LlamaConfig{
			Model:   "/path/to/model.gguf",
			ContextSize: 4096,
		}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "healthy", health.Status)
		assert.Equal(t, 0, health.ErrorCount)
		// ModelCount from GetModels() - depends on server response
	})

	t.Run("Unhealthy", func(t *testing.T) {
		config := LlamaConfig{
			Model: "/path/to/model.gguf",
		}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)

		provider.Close()

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err) // GetHealth doesn't return error when stopped
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
		assert.Equal(t, 1, health.ErrorCount)
	})
}

func TestLlamaCPPProvider_Close(t *testing.T) {
	config := LlamaConfig{
		Model: "/path/to/model.gguf",
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)
	assert.True(t, provider.isRunning)

	err = provider.Close()
	assert.NoError(t, err)
	assert.False(t, provider.isRunning)
}

func TestLlamaCPPProvider_ConfigFields(t *testing.T) {
	config := LlamaConfig{
		Model:     "/path/to/model.gguf",
		ContextSize:   8192,
		GPUEnabled:    true,
		GPULayers:     40,
		Threads:       16,
		ServerHost:    "127.0.0.1",
		ServerPort:    9090,
		ServerTimeout: 120 * time.Second,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "/path/to/model.gguf", provider.config.Model)
	assert.Equal(t, 8192, provider.config.ContextSize)
	assert.True(t, provider.config.GPUEnabled)
	assert.Equal(t, 40, provider.config.GPULayers)
	assert.Equal(t, 16, provider.config.Threads)
	assert.Equal(t, "127.0.0.1", provider.config.ServerHost)
	assert.Equal(t, 9090, provider.config.ServerPort)
	assert.Equal(t, 120*time.Second, provider.config.ServerTimeout)
}
