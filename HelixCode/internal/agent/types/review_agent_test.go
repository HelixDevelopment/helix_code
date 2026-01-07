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

// TestNewReviewAgent tests review agent creation
func TestNewReviewAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		reviewAgent, err := NewReviewAgent(cfg, provider, registry)
		require.NoError(t, err)
		require.NotNil(t, reviewAgent)
		assert.Equal(t, "review-agent", reviewAgent.ID())
	})

	t.Run("Nil provider", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		reviewAgent, err := NewReviewAgent(cfg, nil, registry)
		assert.Error(t, err)
		assert.Nil(t, reviewAgent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	})

	t.Run("Nil tool registry", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}

		reviewAgent, err := NewReviewAgent(cfg, provider, nil)
		assert.Error(t, err)
		assert.Nil(t, reviewAgent)
		assert.Contains(t, err.Error(), "tool registry is required")
	})
}

// TestReviewAgentInitialize tests agent initialization
func TestReviewAgentInitialize(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	reviewAgent, err := NewReviewAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = reviewAgent.Initialize(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusIdle, reviewAgent.Status())
}

// TestReviewAgentShutdown tests agent shutdown
func TestReviewAgentShutdown(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	reviewAgent, err := NewReviewAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = reviewAgent.Shutdown(ctx)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusShutdown, reviewAgent.Status())
}

// TestReviewAgentExecuteWithCode tests basic code review with inline code
func TestReviewAgentExecuteWithCode(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"review_summary": "Code looks good", "issues": [{"severity": "low", "type": "style", "description": "Consider adding comments", "line_number": 1, "recommendation": "Add comments"}], "suggestions": ["Add unit tests"], "metrics": {"overall_score": 85}}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	reviewAgent, err := NewReviewAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeReview,
		"Review Code",
		"Review function implementation",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"code": "func add(a, b int) int { return a + b }",
	}

	result, err := reviewAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "review_result")
	assert.Contains(t, result.Output, "issues")
	assert.Contains(t, result.Output, "suggestions")
}

// TestReviewAgentExecuteMissingInput tests error when code is missing
func TestReviewAgentExecuteMissingInput(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	reviewAgent, err := NewReviewAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeReview,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"other_field": "value",
	}

	result, err := reviewAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "code or file_path not found")
}

// TestReviewAgentExecuteLLMError tests LLM generation error
func TestReviewAgentExecuteLLMError(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		noModels: true, // No models available
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	reviewAgent, err := NewReviewAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeReview,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"code": "some code",
	}

	result, err := reviewAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
}

// TestReviewAgentCollaborate tests collaboration with other agents
func TestReviewAgentCollaborate(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"review_summary": "Review complete", "issues": [], "suggestions": [], "metrics": {"overall_score": 90}}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	reviewAgent, err := NewReviewAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeReview,
		"Review Task",
		"Review code",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"code": "func test() {}",
	}

	// Test collaboration without other agents
	result, err := reviewAgent.Collaborate(ctx, []agent.Agent{}, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Participants, reviewAgent.ID())
	assert.NotNil(t, result.Consensus)
}

// TestReviewAgentTaskMetrics tests task metrics recording
func TestReviewAgentTaskMetrics(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"review_summary": "Review done", "issues": [], "suggestions": [], "metrics": {"overall_score": 95}}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	reviewAgent, err := NewReviewAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeReview,
		"Review",
		"Review code",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"code": "func test() {}",
	}

	result, err := reviewAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.NotNil(t, result.Metrics)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}
