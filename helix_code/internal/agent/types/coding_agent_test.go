package types

import (
	"context"
	"testing"

	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMProvider implements llm.Provider for testing
type MockLLMProvider struct {
	generateFunc func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error)
	models       []llm.ModelInfo
	noModels     bool // Set to true to simulate no available models
}

func (m *MockLLMProvider) GetType() llm.ProviderType {
	return llm.ProviderType("mock")
}

func (m *MockLLMProvider) GetName() string {
	return "mock"
}

func (m *MockLLMProvider) Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, request)
	}
	return &llm.LLMResponse{
		Content: `{"code": "test code", "explanation": "test explanation"}`,
	}, nil
}

func (m *MockLLMProvider) GenerateStream(ctx context.Context, request *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	return nil
}

func (m *MockLLMProvider) GetModels() []llm.ModelInfo {
	if m.noModels {
		return []llm.ModelInfo{}
	}
	if len(m.models) > 0 {
		return m.models
	}
	return []llm.ModelInfo{{Name: "test-model"}}
}

func (m *MockLLMProvider) GetCapabilities() []llm.ModelCapability {
	return []llm.ModelCapability{}
}

func (m *MockLLMProvider) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *MockLLMProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "healthy"}, nil
}

func (m *MockLLMProvider) Close() error {
	return nil
}

// GetContextWindow returns a fixed context-window size for tests.
// Satisfies the llm.Provider interface (added in P1-F01-T02).
func (m *MockLLMProvider) GetContextWindow() int {
	return 4096
}

// CountTokens estimates token count using a simple whitespace-split heuristic.
// Satisfies the llm.Provider interface (added in P1-F01-T02).
func (m *MockLLMProvider) CountTokens(text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	count := 0
	inToken := false
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inToken = false
		} else {
			if !inToken {
				count++
				inToken = true
			}
		}
	}
	return count, nil
}

// TestNewCodingAgent tests coding agent creation
func TestNewCodingAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		codingAgent, err := NewCodingAgent(cfg, provider, registry)
		require.NoError(t, err)
		require.NotNil(t, codingAgent)
		assert.Equal(t, "coding-agent", codingAgent.ID())
	})

	t.Run("Nil provider", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		codingAgent, err := NewCodingAgent(cfg, nil, registry)
		assert.Error(t, err)
		assert.Nil(t, codingAgent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	})

	t.Run("Nil tool registry", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}

		codingAgent, err := NewCodingAgent(cfg, provider, nil)
		assert.Error(t, err)
		assert.Nil(t, codingAgent)
		assert.Contains(t, err.Error(), "tool registry is required")
	})
}

// TestCodingAgentInitialize tests agent initialization
func TestCodingAgentInitialize(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	codingAgent, err := NewCodingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = codingAgent.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusIdle, codingAgent.Status())
}

// TestCodingAgentShutdown tests agent shutdown
func TestCodingAgentShutdown(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	codingAgent, err := NewCodingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = codingAgent.Shutdown(ctx)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusShutdown, codingAgent.Status())
}

// TestCodingAgentExecuteCreate tests code creation
func TestCodingAgentExecuteCreate(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"code": "function hello() { return 'world'; }", "explanation": "Simple hello function"}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	codingAgent, err := NewCodingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeCodeGeneration,
		"Create Hello Function",
		"Create a simple hello function",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create a function that returns 'hello world'",
	}

	result, err := codingAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "code")
	assert.Contains(t, result.Output, "explanation")
	assert.Contains(t, result.Output, "operation")
	assert.Equal(t, "create", result.Output["operation"])
}

// TestCodingAgentExecuteMissingRequirements tests error when requirements missing
func TestCodingAgentExecuteMissingRequirements(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	codingAgent, err := NewCodingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeCodeGeneration,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"other_field": "value",
	}

	result, err := codingAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "requirements not found")
}

// TestCodingAgentExecuteLLMError tests LLM generation error
func TestCodingAgentExecuteLLMError(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		noModels: true, // No models available
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	codingAgent, err := NewCodingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeCodeGeneration,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create a function",
	}

	result, err := codingAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
}

// TestCodingAgentCollaborate tests collaboration with review agents
func TestCodingAgentCollaborate(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"code": "function test() {}", "explanation": "Test function"}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	codingAgent, err := NewCodingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeCodeGeneration,
		"Create Test Function",
		"Create a test function",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create a test function",
	}

	// Test collaboration without other agents
	result, err := codingAgent.Collaborate(ctx, []agent.Agent{}, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Participants, codingAgent.ID())
	assert.NotNil(t, result.Consensus)
}

// TestCodingAgentTaskMetrics tests task metrics recording
func TestCodingAgentTaskMetrics(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"code": "line1\nline2\nline3", "explanation": "Three lines"}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	codingAgent, err := NewCodingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeCodeGeneration,
		"Create Code",
		"Create code",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create code",
	}

	result, err := codingAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.NotNil(t, result.Metrics)
	assert.Greater(t, result.Metrics.LinesAdded, 0)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}
