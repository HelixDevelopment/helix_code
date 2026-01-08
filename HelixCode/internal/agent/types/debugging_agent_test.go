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

// TestNewDebuggingAgent tests debugging agent creation
func TestNewDebuggingAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
		require.NoError(t, err)
		require.NotNil(t, debuggingAgent)
		assert.Equal(t, "debugging-agent", debuggingAgent.ID())
	})

	t.Run("Nil provider", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		mockRegistry := NewMockToolRegistry()
		registry := ConvertToToolRegistry(mockRegistry)

		debuggingAgent, err := NewDebuggingAgent(cfg, nil, registry)
		assert.Error(t, err)
		assert.Nil(t, debuggingAgent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	})

	t.Run("Nil tool registry", func(t *testing.T) {
		cfg := &config.AgentConfig{}
		provider := &MockLLMProvider{}

		debuggingAgent, err := NewDebuggingAgent(cfg, provider, nil)
		assert.Error(t, err)
		assert.Nil(t, debuggingAgent)
		assert.Contains(t, err.Error(), "tool registry is required")
	})
}

// TestDebuggingAgentInitialize tests agent initialization
func TestDebuggingAgentInitialize(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = debuggingAgent.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusIdle, debuggingAgent.Status())
}

// TestDebuggingAgentShutdown tests agent shutdown
func TestDebuggingAgentShutdown(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	err = debuggingAgent.Shutdown(ctx)
	require.NoError(t, err)
	assert.Equal(t, agent.StatusShutdown, debuggingAgent.Status())
}

// TestDebuggingAgentExecuteBasic tests basic error analysis
func TestDebuggingAgentExecuteBasic(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"analysis": "Null pointer error", "root_cause": "Variable not initialized", "suggested_fixes": ["Initialize variable before use"]}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeDebugging,
		"Debug Error",
		"Analyze null pointer error",
		task.PriorityHigh,
	)
	testTask.Input = map[string]interface{}{
		"error":        "NullPointerException at line 42",
		"stack_trace":  "at main.go:42\nat app.go:15",
		"code_context": "var x *int\nfmt.Println(*x)",
	}

	result, err := debuggingAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "analysis")
	assert.Contains(t, result.Output, "root_cause")
	assert.Contains(t, result.Output, "suggested_fixes")
}

// TestDebuggingAgentExecuteMissingError tests error when error message is missing
func TestDebuggingAgentExecuteMissingError(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeDebugging,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"other_field": "value",
	}

	result, err := debuggingAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "error message not found")
}

// TestDebuggingAgentExecuteLLMError tests LLM generation error
func TestDebuggingAgentExecuteLLMError(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		noModels: true, // No models available
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeDebugging,
		"Test Task",
		"Test",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"error": "Some error occurred",
	}

	result, err := debuggingAgent.Execute(ctx, testTask)
	assert.Error(t, err)
	assert.False(t, result.Success)
}

// TestDebuggingAgentCollaborate tests collaboration with other agents
func TestDebuggingAgentCollaborate(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"analysis": "Bug fixed", "root_cause": "Logic error", "suggested_fixes": ["Fix applied"]}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeDebugging,
		"Debug Task",
		"Fix bug",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"error": "Bug in code",
	}

	// Test collaboration without other agents
	result, err := debuggingAgent.Collaborate(ctx, []agent.Agent{}, testTask)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Participants, debuggingAgent.ID())
	assert.NotNil(t, result.Consensus)
}

// TestDebuggingAgentTaskMetrics tests task metrics recording
func TestDebuggingAgentTaskMetrics(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"analysis": "Analysis complete", "root_cause": "Bug identified", "suggested_fixes": ["Fix 1", "Fix 2"]}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeDebugging,
		"Debug",
		"Analyze error",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"error": "Runtime error",
	}

	result, err := debuggingAgent.Execute(ctx, testTask)
	require.NoError(t, err)
	assert.NotNil(t, result.Metrics)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}

// TestDebuggingAgentReadFile tests the readFile function
func TestDebuggingAgentReadFile(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}

	// Use CreateMockToolRegistry with FSRead function
	mockRegistry := CreateMockToolRegistry(
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return "package main\n\nfunc main() {}", nil
		},
		nil,
		nil,
	)

	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	content, err := debuggingAgent.readFile(ctx, "/test/main.go")

	// Should succeed since FSRead tool is configured
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "package main")
}

// TestDebuggingAgentRunDiagnostics tests the runDiagnostics function
func TestDebuggingAgentRunDiagnostics(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}

	// Use CreateMockToolRegistry with Shell function
	mockRegistry := CreateMockToolRegistry(
		nil,
		nil,
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return "diagnostic output", nil
		},
	)

	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	results, err := debuggingAgent.runDiagnostics(ctx, "/test/main.go", "null pointer error")

	require.NoError(t, err)
	assert.NotNil(t, results)
	// Should have go_vet, go_build, go_test results
	assert.Contains(t, results, "go_vet")
}

// TestDebuggingAgentDetermineDiagnosticCommands tests command determination
func TestDebuggingAgentDetermineDiagnosticCommands(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	t.Run("Go file", func(t *testing.T) {
		commands := debuggingAgent.determineDiagnosticCommands("/test/main.go", "error")
		assert.Contains(t, commands, "go_vet")
		assert.Contains(t, commands, "go_build")
		assert.Contains(t, commands, "go_test")
	})

	t.Run("Non-Go file", func(t *testing.T) {
		commands := debuggingAgent.determineDiagnosticCommands("/test/script.py", "error")
		// Should return empty or different commands for non-Go files
		assert.NotNil(t, commands)
	})
}

// TestDebuggingAgentApplyFix tests the applyFix function
func TestDebuggingAgentApplyFix(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"fixed_code": "package main\n\nfunc main() {\n\tvar x int = 0\n\tfmt.Println(x)\n}"}`,
			}, nil
		},
	}

	// Use CreateMockToolRegistry with FSRead and FSWrite functions
	mockRegistry := CreateMockToolRegistry(
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return "package main\n\nfunc main() {\n\tvar x *int\n\tfmt.Println(*x)\n}", nil
		},
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return "ok", nil
		},
		nil,
	)

	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Empty file path", func(t *testing.T) {
		result, err := debuggingAgent.applyFix(ctx, "", "Initialize variable")
		require.NoError(t, err)
		assert.Equal(t, "skipped", result["status"])
	})

	t.Run("Valid file path", func(t *testing.T) {
		result, err := debuggingAgent.applyFix(ctx, "/test/main.go", "Initialize variable before use")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "success", result["status"])
	})
}

// TestDebuggingAgentGenerateFixedCode tests the generateFixedCode function
func TestDebuggingAgentGenerateFixedCode(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"fixed_code": "package main\n\nfunc main() {\n\tvar x int = 0\n\tfmt.Println(x)\n}"}`,
			}, nil
		},
	}

	// Use CreateMockToolRegistry with FSRead function
	mockRegistry := CreateMockToolRegistry(
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return "package main\n\nfunc main() {\n\tvar x *int\n\tfmt.Println(*x)\n}", nil
		},
		nil,
		nil,
	)

	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	fixedCode, err := debuggingAgent.generateFixedCode(ctx, "/test/main.go", "Initialize x as int instead of *int")

	require.NoError(t, err)
	assert.NotEmpty(t, fixedCode)
}

// TestDebuggingAgentCollaborateWithTestingAgent tests collaboration with testing agent
func TestDebuggingAgentCollaborateWithTestingAgent(t *testing.T) {
	cfg := &config.AgentConfig{}
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"analysis": "Bug found", "root_cause": "Logic error", "suggested_fixes": ["Fix applied"]}`,
			}, nil
		},
	}
	mockRegistry := NewMockToolRegistry()
	registry := ConvertToToolRegistry(mockRegistry)

	debuggingAgent, err := NewDebuggingAgent(cfg, provider, registry)
	require.NoError(t, err)

	// Create a mock testing agent
	testingAgent, err := NewTestingAgent(cfg, provider, registry)
	require.NoError(t, err)

	ctx := context.Background()
	testTask := task.NewTask(
		task.TaskTypeDebugging,
		"Debug and Test",
		"Fix bug and verify",
		task.PriorityHigh,
	)
	testTask.Input = map[string]interface{}{
		"error": "Logic error in calculation",
	}

	// Collaborate with testing agent
	result, err := debuggingAgent.Collaborate(ctx, []agent.Agent{testingAgent}, testTask)
	require.NoError(t, err)
	assert.Contains(t, result.Participants, debuggingAgent.ID())
}
