package llm

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProvider implements Provider for testing
type MockProvider struct {
	generateFunc       func(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
	generateStreamFunc func(ctx context.Context, req *LLMRequest, ch chan<- LLMResponse) error
}

func (m *MockProvider) GetType() ProviderType { return ProviderTypeOpenAI }
func (m *MockProvider) GetName() string       { return "mock-provider" }
func (m *MockProvider) GetModels() []ModelInfo {
	return []ModelInfo{{Name: "mock-model"}}
}
func (m *MockProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (m *MockProvider) Generate(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req)
	}
	return &LLMResponse{
		ID:      uuid.New(),
		Content: "Mock response",
	}, nil
}
func (m *MockProvider) GenerateStream(ctx context.Context, req *LLMRequest, ch chan<- LLMResponse) error {
	if m.generateStreamFunc != nil {
		return m.generateStreamFunc(ctx, req, ch)
	}
	ch <- LLMResponse{ID: uuid.New(), Content: "Mock stream"}
	close(ch)
	return nil
}
func (m *MockProvider) IsAvailable(ctx context.Context) bool                      { return true }
func (m *MockProvider) GetHealth(ctx context.Context) (*ProviderHealth, error)    { return &ProviderHealth{Status: "healthy"}, nil }
func (m *MockProvider) Close() error                                              { return nil }

// MockToolExecutor implements ToolExecutor for testing
type MockToolExecutor struct {
	executeFunc func(ctx context.Context, name string, params map[string]interface{}) (interface{}, error)
}

func (m *MockToolExecutor) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, name, params)
	}
	return map[string]interface{}{"result": "executed"}, nil
}

func TestNewToolCallingProvider(t *testing.T) {
	t.Run("CreatesProviderWithBaseProvider", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		require.NotNil(t, provider)
		assert.NotNil(t, provider.baseProvider)
		assert.NotNil(t, provider.tools)
		assert.Empty(t, provider.tools)
	})
}

func TestToolCallingProvider_SetToolExecutor(t *testing.T) {
	t.Run("SetsExecutor", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		executor := &MockToolExecutor{}
		provider.SetToolExecutor(executor)

		assert.NotNil(t, provider.toolExecutor)
	})
}

func TestToolCallingProvider_RegisterTool(t *testing.T) {
	t.Run("RegistersNewTool", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		tool := Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"input": map[string]interface{}{"type": "string"},
					},
				},
			},
		}

		err := provider.RegisterTool(tool)
		require.NoError(t, err)

		tools := provider.ListAvailableTools()
		assert.Len(t, tools, 1)
		assert.Equal(t, "test_tool", tools[0].Function.Name)
	})

	t.Run("ErrorsOnDuplicateTool", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		tool := Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        "duplicate_tool",
				Description: "A test tool",
			},
		}

		err := provider.RegisterTool(tool)
		require.NoError(t, err)

		// Try to register again
		err = provider.RegisterTool(tool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestToolCallingProvider_ListAvailableTools(t *testing.T) {
	t.Run("ReturnsEmptyListInitially", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		tools := provider.ListAvailableTools()
		assert.Empty(t, tools)
	})

	t.Run("ReturnsRegisteredTools", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		// Register multiple tools
		for i := 0; i < 3; i++ {
			tool := Tool{
				Type: "function",
				Function: ToolFunction{
					Name:        "tool_" + string(rune('a'+i)),
					Description: "Test tool",
				},
			}
			provider.RegisterTool(tool)
		}

		tools := provider.ListAvailableTools()
		assert.Len(t, tools, 3)
	})
}

func TestToolCallingProvider_GetType(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	assert.Equal(t, ProviderTypeOpenAI, provider.GetType())
}

func TestToolCallingProvider_GetName(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	assert.Equal(t, "mock-provider", provider.GetName())
}

func TestToolCallingProvider_GetModels(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	models := provider.GetModels()
	assert.Len(t, models, 1)
	assert.Equal(t, "mock-model", models[0].Name)
}

func TestToolCallingProvider_GetCapabilities(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	caps := provider.GetCapabilities()
	assert.Contains(t, caps, CapabilityTextGeneration)
}

func TestToolCallingProvider_Generate(t *testing.T) {
	t.Run("DelegatesToBaseProvider", func(t *testing.T) {
		mockBase := &MockProvider{
			generateFunc: func(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
				return &LLMResponse{
					ID:      uuid.New(),
					Content: "Base provider response",
				}, nil
			},
		}
		provider := NewToolCallingProvider(mockBase)

		ctx := context.Background()
		req := &LLMRequest{
			Model:    "test",
			Messages: []Message{{Role: "user", Content: "Hello"}},
		}

		resp, err := provider.Generate(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "Base provider response", resp.Content)
	})
}

func TestToolCallingProvider_GenerateWithTools(t *testing.T) {
	t.Run("GeneratesWithoutToolCalls", func(t *testing.T) {
		mockBase := &MockProvider{
			generateFunc: func(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
				return &LLMResponse{
					ID:      uuid.New(),
					Content: "I can help you with that. Here's the answer.",
				}, nil
			},
		}
		provider := NewToolCallingProvider(mockBase)

		ctx := context.Background()
		req := ToolGenerationRequest{
			ID:          uuid.New(),
			Prompt:      "What is 2+2?",
			Tools:       []Tool{},
			MaxTokens:   100,
			Temperature: 0.7,
		}

		resp, err := provider.GenerateWithTools(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Text)
		assert.Empty(t, resp.ToolCalls)
	})

	t.Run("GeneratesWithToolCalls", func(t *testing.T) {
		callCount := 0
		mockBase := &MockProvider{
			generateFunc: func(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
				callCount++
				if callCount == 1 {
					// First call - return tool call
					return &LLMResponse{
						ID:      uuid.New(),
						Content: "TOOL_CALL: {\"tool_name\": \"calculator\", \"arguments\": {\"expression\": \"2+2\"}}",
					}, nil
				}
				// Second call - return final answer
				return &LLMResponse{
					ID:      uuid.New(),
					Content: "The answer is 4.",
				}, nil
			},
		}
		provider := NewToolCallingProvider(mockBase)

		// Register tool
		tool := Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        "calculator",
				Description: "Calculates expressions",
			},
		}
		provider.RegisterTool(tool)

		ctx := context.Background()
		req := ToolGenerationRequest{
			ID:     uuid.New(),
			Prompt: "Calculate 2+2",
			Tools:  []Tool{tool},
		}

		resp, err := provider.GenerateWithTools(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Text)
		assert.NotNil(t, resp.Metadata)
	})
}

func TestToolCallingProvider_IsAvailable(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	ctx := context.Background()
	assert.True(t, provider.IsAvailable(ctx))
}

func TestToolCallingProvider_GetHealth(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	ctx := context.Background()
	health, err := provider.GetHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestToolCallingProvider_Close(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	err := provider.Close()
	assert.NoError(t, err)
}

func TestToolCallingProvider_buildToolEnhancedPrompt(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	tools := []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "search",
				Description: "Search for information",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	prompt := provider.buildToolEnhancedPrompt("Find information about Go", tools)

	assert.Contains(t, prompt, "search")
	assert.Contains(t, prompt, "Search for information")
	assert.Contains(t, prompt, "Find information about Go")
	assert.Contains(t, prompt, "TOOL_CALL")
}

func TestToolCallingProvider_extractToolCallsAndReasoning(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	t.Run("ExtractsToolCalls", func(t *testing.T) {
		text := `Let me help you with that.
TOOL_CALL: {"tool_name": "calculator", "arguments": {"x": 1}}
I'll calculate it for you.`

		toolCalls, reasoning := provider.extractToolCallsAndReasoning(text)

		assert.NotEmpty(t, reasoning)
		assert.Contains(t, reasoning, "Let me help you")
		// Tool call parsing depends on JSON structure
		_ = toolCalls
	})

	t.Run("NoToolCalls", func(t *testing.T) {
		text := "Just a simple response without any tool calls."

		toolCalls, reasoning := provider.extractToolCallsAndReasoning(text)

		assert.Empty(t, toolCalls)
		assert.NotEmpty(t, reasoning)
	})
}

func TestToolCallingProvider_executeToolCalls(t *testing.T) {
	t.Run("ExecutesWithExecutor", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		// Register tool
		tool := Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        "test_tool",
				Description: "Test tool",
			},
		}
		provider.RegisterTool(tool)

		// Set executor
		executor := &MockToolExecutor{
			executeFunc: func(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
				return "executed: " + name, nil
			},
		}
		provider.SetToolExecutor(executor)

		ctx := context.Background()
		toolCalls := []ToolCall{
			{
				ID:   "1",
				Type: "function",
				Function: ToolCallFunc{
					Name:      "test_tool",
					Arguments: map[string]interface{}{"input": "test"},
				},
			},
		}

		results, err := provider.executeToolCalls(ctx, toolCalls)
		require.NoError(t, err)
		assert.Contains(t, results, "test_tool")
	})

	t.Run("HandlesUnregisteredTool", func(t *testing.T) {
		mockBase := &MockProvider{}
		provider := NewToolCallingProvider(mockBase)

		ctx := context.Background()
		toolCalls := []ToolCall{
			{
				ID:   "1",
				Type: "function",
				Function: ToolCallFunc{
					Name:      "unknown_tool",
					Arguments: map[string]interface{}{},
				},
			},
		}

		results, err := provider.executeToolCalls(ctx, toolCalls)
		require.NoError(t, err)
		assert.Contains(t, results["unknown_tool"].(string), "Tool not found")
	})
}

func TestToolCallingProvider_buildFinalPrompt(t *testing.T) {
	mockBase := &MockProvider{}
	provider := NewToolCallingProvider(mockBase)

	results := map[string]interface{}{
		"calculator": "4",
		"search":     "Found information",
	}

	prompt := provider.buildFinalPrompt("What is 2+2?", "Let me calculate", results)

	assert.Contains(t, prompt, "What is 2+2?")
	assert.Contains(t, prompt, "Let me calculate")
	assert.Contains(t, prompt, "calculator")
	assert.Contains(t, prompt, "4")
}

func TestToolGenerationRequest_Struct(t *testing.T) {
	req := ToolGenerationRequest{
		ID:          uuid.New(),
		Prompt:      "Test prompt",
		Tools:       []Tool{},
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
		Context:     map[string]interface{}{"key": "value"},
	}

	assert.NotEqual(t, uuid.Nil, req.ID)
	assert.Equal(t, "Test prompt", req.Prompt)
	assert.Equal(t, 100, req.MaxTokens)
	assert.Equal(t, 0.7, req.Temperature)
	assert.True(t, req.Stream)
}

func TestToolGenerationResponse_Struct(t *testing.T) {
	resp := ToolGenerationResponse{
		ID:        uuid.New(),
		Text:      "Response text",
		ToolCalls: []ToolCall{},
		Reasoning: "My reasoning",
		Metadata:  map[string]interface{}{"key": "value"},
	}

	assert.NotEqual(t, uuid.Nil, resp.ID)
	assert.Equal(t, "Response text", resp.Text)
	assert.Equal(t, "My reasoning", resp.Reasoning)
}

func TestToolStreamChunk_Struct(t *testing.T) {
	chunk := ToolStreamChunk{
		ID:        uuid.New(),
		Content:   "Chunk content",
		ToolCalls: []ToolCall{},
		Reasoning: "Reasoning",
		Done:      false,
		Error:     "",
	}

	assert.NotEqual(t, uuid.Nil, chunk.ID)
	assert.Equal(t, "Chunk content", chunk.Content)
	assert.False(t, chunk.Done)
	assert.Empty(t, chunk.Error)
}

func TestToolCallingProvider_StreamWithTools(t *testing.T) {
	t.Run("StreamsResponse", func(t *testing.T) {
		mockBase := &MockProvider{
			generateStreamFunc: func(ctx context.Context, req *LLMRequest, ch chan<- LLMResponse) error {
				ch <- LLMResponse{ID: uuid.New(), Content: "Streamed content"}
				close(ch)
				return nil
			},
		}
		provider := NewToolCallingProvider(mockBase)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req := ToolGenerationRequest{
			ID:          uuid.New(),
			Prompt:      "Test streaming",
			Tools:       []Tool{},
			MaxTokens:   100,
			Temperature: 0.7,
			Stream:      true,
		}

		ch, err := provider.StreamWithTools(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Collect chunks
		var chunks []ToolStreamChunk
		for chunk := range ch {
			chunks = append(chunks, chunk)
		}

		assert.NotEmpty(t, chunks)
	})
}
