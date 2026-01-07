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

// TestNewTestingAgent tests testing agent creation
func TestNewTestingAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		testingAgent, err := NewTestingAgent(cfg, provider, registry)
		require.NoError(t, err)
		require.NotNil(t, testingAgent)
		assert.Equal(t, "testing-agent", testingAgent.ID())
	})

	t.Run("Nil provider", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		testingAgent, err := NewTestingAgent(cfg, nil, registry)
		assert.Error(t, err)
		assert.Nil(t, testingAgent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	})

	t.Run("Nil tool registry", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}

		testingAgent, err := NewTestingAgent(cfg, provider, nil)
		assert.Error(t, err)
		assert.Nil(t, testingAgent)
		assert.Contains(t, err.Error(), "tool registry is required")
	})
}

// TestTestingAgentInitialize tests agent initialization
func TestTestingAgentInitialize(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	testingAgent, err := NewTestingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = testingAgent.Initialize(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusIdle, testingAgent.Status())
}

// TestTestingAgentShutdown tests agent shutdown
func TestTestingAgentShutdown(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	testingAgent, err := NewTestingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = testingAgent.Shutdown(ctx)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusShutdown, testingAgent.Status())
}

// TestTestingAgentExecuteMissingCode tests error when code is missing
func TestTestingAgentExecuteMissingCode(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	testingAgent, err := NewTestingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeTesting,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"other_field": "value",
	}

	result, err := testingAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "code not found")
}

// TestTestingAgentExecuteLLMError tests LLM generation error
func TestTestingAgentExecuteLLMError(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		noModels: true, // No models available
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	testingAgent, err := NewTestingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeTesting,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"code": "some code",
	}

	result, err := testingAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
}

// TestTestingAgentCollaborate tests collaboration with other agents
func TestTestingAgentCollaborate(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"test_code": "func TestExample(t *testing.T) {}", "test_cases": ["TestExample"]}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	mockRegistry.Register(NewMockTool("FSWrite", func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return nil, nil
	}))
	registry := ConvertToToolRegistry(mockRegistry)

	testingAgent, err := NewTestingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeTesting,
		"Testing Task",
		"Generate tests",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"code": "func example() {}",
	}

	// Test collaboration without other agents
	result, err := testingAgent.Collaborate(ctx, []agent.Agent{}, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Participants, testingAgent.ID())
	assert.NotNil(t, result.Consensus)
}

// TestGetTestFilePath tests test file path generation
func TestGetTestFilePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty path",
			input:    "",
			expected: "generated_test.go",
		},
		{
			name:     "Go file",
			input:    "handler.go",
			expected: "handler_test.go",
		},
		{
			name:     "Go file with path",
			input:    "internal/api/handler.go",
			expected: "internal/api/handler_test.go",
		},
		{
			name:     "Non-Go file",
			input:    "script.py",
			expected: "script.py_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTestFilePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
