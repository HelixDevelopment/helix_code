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

// TestNewPlanningAgent tests planning agent creation
func TestNewPlanningAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}

		planningAgent, err := NewPlanningAgent(cfg, provider)
		require.NoError(t, err)
		require.NotNil(t, planningAgent)
		assert.Equal(t, "planning-agent", planningAgent.ID())
	})

	t.Run("Nil provider", func(t *testing.T) {
		cfg := &config.AgentConfig{}

		planningAgent, err := NewPlanningAgent(cfg, nil)
		assert.Error(t, err)
		assert.Nil(t, planningAgent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	})
}

// TestPlanningAgentInitialize tests agent initialization
func TestPlanningAgentInitialize(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}

	planningAgent, err := NewPlanningAgent(cfg, provider)
	require.NoError(t, err)

	ctx := context.Background()
	err = planningAgent.Initialize(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusIdle, planningAgent.Status())
}

// TestPlanningAgentShutdown tests agent shutdown
func TestPlanningAgentShutdown(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}

	planningAgent, err := NewPlanningAgent(cfg, provider)
	require.NoError(t, err)

	ctx := context.Background()
	err = planningAgent.Shutdown(ctx)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusShutdown, planningAgent.Status())
}

// TestPlanningAgentExecuteBasic tests basic planning execution
func TestPlanningAgentExecuteBasic(t *testing.T) {
	cfg := &config.AgentConfig{}
	callCount := 0
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				// First call - generate plan
				return &llm.LLMResponse{
					Content: "1. Set up project structure\n2. Implement CRUD operations\n3. Add tests",
				}, nil
			}
			// Second call - parse subtasks
			return &llm.LLMResponse{
				Content: `[{"title": "Set up project", "description": "Create project structure", "type": "code_generation", "priority": 2, "estimated_duration_minutes": 30, "depends_on": [], "required_capabilities": []}, {"title": "Implement CRUD", "description": "Add CRUD operations", "type": "code_generation", "priority": 2, "estimated_duration_minutes": 60, "depends_on": ["Set up project"], "required_capabilities": []}]`,
			}, nil
		},
	}

	planningAgent, err := NewPlanningAgent(cfg, provider)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypePlanning,
		"Create Project Plan",
		"Create a development plan for new feature",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Build a REST API with CRUD operations",
	}

	result, err := planningAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "plan")
	assert.Contains(t, result.Output, "subtasks")
}

// TestPlanningAgentExecuteMissingRequirements tests error when requirements missing
func TestPlanningAgentExecuteMissingRequirements(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}

	planningAgent, err := NewPlanningAgent(cfg, provider)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypePlanning,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"other_field": "value",
	}

	result, err := planningAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "requirements not found")
}

// TestPlanningAgentExecuteLLMError tests LLM generation error
func TestPlanningAgentExecuteLLMError(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		noModels: true, // No models available
	}

	planningAgent, err := NewPlanningAgent(cfg, provider)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypePlanning,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create a plan",
	}

	result, err := planningAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
}

// TestPlanningAgentCollaborate tests collaboration with other agents
func TestPlanningAgentCollaborate(t *testing.T) {
	cfg := &config.AgentConfig{}
	callCount := 0
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				// First call - generate plan
				return &llm.LLMResponse{
					Content: "1. First task\n2. Second task",
				}, nil
			}
			// Second call - parse subtasks
			return &llm.LLMResponse{
				Content: `[{"title": "First task", "description": "Do first task", "type": "planning", "priority": 2, "estimated_duration_minutes": 15, "depends_on": [], "required_capabilities": []}]`,
			}, nil
		},
	}

	planningAgent, err := NewPlanningAgent(cfg, provider)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypePlanning,
		"Planning Task",
		"Create project plan",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create a project plan",
	}

	// Test collaboration without other agents
	result, err := planningAgent.Collaborate(ctx, []agent.Agent{}, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Participants, planningAgent.ID())
	assert.NotNil(t, result.Consensus)
}

// TestPlanningAgentTaskMetrics tests task metrics recording
func TestPlanningAgentTaskMetrics(t *testing.T) {
	cfg := &config.AgentConfig{}
	callCount := 0
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				// First call - generate plan
				return &llm.LLMResponse{
					Content: "1. Test task",
				}, nil
			}
			// Second call - parse subtasks
			return &llm.LLMResponse{
				Content: `[{"title": "Test task", "description": "Test task description", "type": "testing", "priority": 2, "estimated_duration_minutes": 10, "depends_on": [], "required_capabilities": []}]`,
			}, nil
		},
	}

	planningAgent, err := NewPlanningAgent(cfg, provider)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypePlanning,
		"Plan",
		"Create plan",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create plan",
	}

	result, err := planningAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}
