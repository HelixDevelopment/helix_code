package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLlamaCPPProvider(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath:     "/path/to/model.gguf",
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
		assert.Equal(t, config.ModelPath, provider.config.ModelPath)
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
		ModelPath:   "/path/to/model.gguf",
		ContextSize: 4096,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeLocal, provider.GetType())
}

func TestLlamaCPPProvider_GetName(t *testing.T) {
	config := LlamaConfig{
		ModelPath:   "/path/to/model.gguf",
		ContextSize: 4096,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "llama-cpp", provider.GetName())
}

func TestLlamaCPPProvider_GetModels(t *testing.T) {
	config := LlamaConfig{
		ModelPath:   "/path/to/llama-7b.gguf",
		ContextSize: 4096,
	}

	provider, err := NewLlamaCPPProvider(config)
	require.NoError(t, err)

	models := provider.GetModels()
	require.Len(t, models, 1)

	model := models[0]
	assert.Equal(t, "/path/to/llama-7b.gguf", model.Name)
	assert.Equal(t, ProviderTypeLocal, model.Provider)
	assert.Equal(t, 4096, model.ContextSize)
	assert.Equal(t, 4096, model.MaxTokens)
	assert.False(t, model.SupportsTools)
	assert.False(t, model.SupportsVision)
	assert.Contains(t, model.Capabilities, CapabilityTextGeneration)
	assert.Contains(t, model.Capabilities, CapabilityCodeGeneration)
	assert.Contains(t, model.Capabilities, CapabilityCodeAnalysis)
}

func TestLlamaCPPProvider_GetCapabilities(t *testing.T) {
	config := LlamaConfig{
		ModelPath:   "/path/to/model.gguf",
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
	t.Run("Success", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath:   "/path/to/model.gguf",
			ContextSize: 4096,
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
		assert.Contains(t, response.Content, "simulated response")
		assert.Equal(t, 150, response.Usage.TotalTokens)
		assert.NotZero(t, response.ProcessingTime)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath: "/path/to/model.gguf",
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
	t.Run("Success", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath:   "/path/to/model.gguf",
			ContextSize: 4096,
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
			err := provider.GenerateStream(ctx, request, ch)
			assert.NoError(t, err)
			close(ch)
		}()

		var chunks []string
		for response := range ch {
			chunks = append(chunks, response.Content)
		}

		assert.NotEmpty(t, chunks)
		assert.Len(t, chunks, 5) // "This", " is", " a", " streaming", " response"
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath: "/path/to/model.gguf",
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

		// Cancel immediately
		cancel()

		err = provider.GenerateStream(ctx, request, ch)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("ProviderStopped", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath: "/path/to/model.gguf",
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
			ModelPath: "/path/to/model.gguf",
		}

		provider, err := NewLlamaCPPProvider(config)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.True(t, available)
	})

	t.Run("Unavailable", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath: "/path/to/model.gguf",
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
			ModelPath:   "/path/to/model.gguf",
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
		assert.Equal(t, 1, health.ModelCount)
	})

	t.Run("Unhealthy", func(t *testing.T) {
		config := LlamaConfig{
			ModelPath: "/path/to/model.gguf",
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
		ModelPath: "/path/to/model.gguf",
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
		ModelPath:     "/path/to/model.gguf",
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

	assert.Equal(t, "/path/to/model.gguf", provider.config.ModelPath)
	assert.Equal(t, 8192, provider.config.ContextSize)
	assert.True(t, provider.config.GPUEnabled)
	assert.Equal(t, 40, provider.config.GPULayers)
	assert.Equal(t, 16, provider.config.Threads)
	assert.Equal(t, "127.0.0.1", provider.config.ServerHost)
	assert.Equal(t, 9090, provider.config.ServerPort)
	assert.Equal(t, 120*time.Second, provider.config.ServerTimeout)
}
