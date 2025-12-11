package types

import (
	"context"
	"fmt"
	"testing"
	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
// TestNewDebuggingAgent tests debugging agent creation
func TestNewDebuggingAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		config := &agent.AgentConfig{
			ID:   "debugging-1",
			Type: agent.AgentTypeDebugging,
			Name: "Test Debugging Agent",
		}
		provider := &MockLLMProvider{}
		registry, err := tools.NewToolRegistry(nil)
		require.NoError(t, err)
		debuggingAgent, err := NewDebuggingAgent(config, provider, registry)
		require.NotNil(t, debuggingAgent)
		assert.Equal(t, "debugging-1", debuggingAgent.ID())
		assert.Equal(t, agent.AgentTypeDebugging, debuggingAgent.Type())
	})
	t.Run("Nil provider", func(t *testing.T) {
		agent, err := NewDebuggingAgent(config, nil, registry)
		assert.Error(t, err)
		assert.Nil(t, agent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	t.Run("Nil tool registry", func(t *testing.T) {
		agent, err := NewDebuggingAgent(config, provider, nil)
		assert.Contains(t, err.Error(), "tool registry is required")
}
// TestDebuggingAgentInitialize tests agent initialization
func TestDebuggingAgentInitialize(t *testing.T) {
	config := &agent.AgentConfig{
		ID:   "debugging-1",
		Type: agent.AgentTypeDebugging,
		Name: "Test Debugging Agent",
	}
	provider := &MockLLMProvider{}
	registry, err := tools.NewToolRegistry(nil)
	require.NoError(t, err)
	debuggingAgent, err := NewDebuggingAgent(config, provider, registry)
	ctx := context.Background()
	err = debuggingAgent.Initialize(ctx, config)
	assert.Equal(t, agent.StatusIdle, debuggingAgent.Status())
// TestDebuggingAgentShutdown tests agent shutdown
func TestDebuggingAgentShutdown(t *testing.T) {
	err = debuggingAgent.Shutdown(ctx)
	assert.Equal(t, agent.StatusShutdown, debuggingAgent.Status())
// TestDebuggingAgentExecuteBasic tests basic error analysis
func TestDebuggingAgentExecuteBasic(t *testing.T) {
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"analysis": "Null pointer error", "root_cause": "Variable not initialized", "suggested_fixes": ["Initialize variable before use"]}`,
			}, nil
		},
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
	result, err := debuggingAgent.Execute(ctx, testTask)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "analysis")
	assert.Contains(t, result.Output, "root_cause")
	assert.Contains(t, result.Output, "suggested_fixes")
// TestDebuggingAgentExecuteMissingError tests error when error message is missing
func TestDebuggingAgentExecuteMissingError(t *testing.T) {
		"Test Task",
		"Test",
		task.PriorityNormal,
		"other_field": "value",
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "error message not found")
	health := debuggingAgent.Health()
	assert.Equal(t, 1, health.ErrorCount)
// TestDebuggingAgentExecuteLLMError tests LLM generation error
func TestDebuggingAgentExecuteLLMError(t *testing.T) {
		models: []llm.ModelInfo{},
		"error": "Some error occurred",
// TestDebuggingAgentCollaborate tests collaboration with testing agents
func TestDebuggingAgentCollaborate(t *testing.T) {
				Content: `{"analysis": "Bug fixed", "root_cause": "Logic error", "suggested_fixes": ["Fix applied"]}`,
	// Create a mock testing agent
	testingConfig := &agent.AgentConfig{
		ID:   "testing-1",
		Type: agent.AgentTypeTesting,
		Name: "Test Testing Agent",
	testingAgent := &MockCollabAgent{
		BaseAgent: agent.NewBaseAgent(testingConfig),
		"Debug Task",
		"Fix bug",
		"error": "Bug in code",
	result, err := debuggingAgent.Collaborate(ctx, []agent.Agent{testingAgent}, testTask)
	assert.Contains(t, result.Participants, debuggingAgent.ID())
	assert.NotNil(t, result.Consensus)
// TestDebuggingAgentDetermineDiagnosticCommands tests diagnostic command generation
func TestDebuggingAgentDetermineDiagnosticCommands(t *testing.T) {
	t.Run("Go file", func(t *testing.T) {
		commands := debuggingAgent.determineDiagnosticCommands("internal/api/handler.go", "error")
		assert.Contains(t, commands, "go_vet")
		assert.Contains(t, commands, "go_build")
		assert.Contains(t, commands, "go_test")
	t.Run("Non-Go file", func(t *testing.T) {
		commands := debuggingAgent.determineDiagnosticCommands("script.js", "error")
		assert.Empty(t, commands)
// TestDebuggingAgentMetrics tests metrics recording
func TestDebuggingAgentMetrics(t *testing.T) {
				Content: `{"analysis": "Analysis complete", "root_cause": "Bug identified", "suggested_fixes": ["Fix 1", "Fix 2"]}`,
		"Analyze error",
		"error": "Runtime error",
	assert.NotNil(t, result.Metrics)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
// TestDebuggingAgentReadFile tests the readFile helper function
func TestDebuggingAgentReadFile(t *testing.T) {
	t.Run("Successful file read", func(t *testing.T) {
		// Create mock registry with FSRead tool
		mockRegistry := CreateMockToolRegistry(
			func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return "file content here", nil
			},
			nil,
		)
		debuggingAgent, err := NewDebuggingAgent(config, provider, ConvertToToolRegistry(mockRegistry))
		ctx := context.Background()
		content, err := debuggingAgent.readFile(ctx, "/path/to/file.go")
		assert.Equal(t, "file content here", content)
	t.Run("Tool not found error", func(t *testing.T) {
		mockRegistry := NewMockToolRegistry() // Empty registry
		_, err = debuggingAgent.readFile(ctx, "/path/to/file.go")
		assert.Contains(t, err.Error(), "failed to get FSRead tool")
	t.Run("File read execution error", func(t *testing.T) {
				return nil, fmt.Errorf("file not found")
		_, err = debuggingAgent.readFile(ctx, "/nonexistent.go")
		assert.Contains(t, err.Error(), "failed to read file")
	t.Run("Unexpected output type", func(t *testing.T) {
				return 12345, nil // Return non-string
		assert.Contains(t, err.Error(), "unexpected output type")
// TestDebuggingAgentRunDiagnostics tests the runDiagnostics helper function
func TestDebuggingAgentRunDiagnostics(t *testing.T) {
	t.Run("Successful diagnostics", func(t *testing.T) {
				return "command output success", nil
		results, err := debuggingAgent.runDiagnostics(ctx, "test.go", "compile error")
		assert.NotNil(t, results)
		// Should have diagnostics for Go files
		assert.Contains(t, results, "go_vet")
		assert.Contains(t, results, "go_build")
		assert.Contains(t, results, "go_test")
		// Check success status
		vetResult := results["go_vet"].(map[string]interface{})
		assert.Equal(t, "success", vetResult["status"])
	t.Run("Shell tool not found", func(t *testing.T) {
		_, err = debuggingAgent.runDiagnostics(ctx, "test.go", "error")
		assert.Contains(t, err.Error(), "failed to get Shell tool")
	t.Run("Command execution failures recorded", func(t *testing.T) {
				// Fail some commands
				command := params["command"].(string)
				if command == "go build ." {
					return nil, fmt.Errorf("build failed")
				}
				return "success", nil
		results, err := debuggingAgent.runDiagnostics(ctx, "test.go", "error")
		// go_build should have failed status
		buildResult := results["go_build"].(map[string]interface{})
		assert.Equal(t, "failed", buildResult["status"])
		assert.Contains(t, buildResult["error"], "build failed")
	t.Run("Non-Go file no diagnostics", func(t *testing.T) {
				return "output", nil
		results, err := debuggingAgent.runDiagnostics(ctx, "script.js", "error")
		// Should have no diagnostic commands for non-Go files
		assert.Empty(t, results)
// TestDebuggingAgentApplyFix tests the applyFix helper function
func TestDebuggingAgentApplyFix(t *testing.T) {
	t.Run("Successful fix application", func(t *testing.T) {
		provider := &MockLLMProvider{
			generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
				return &llm.LLMResponse{
					Content: `{"fixed_code": "fixed code content"}`,
				}, nil
				return "original code", nil
				return nil, nil
		result, err := debuggingAgent.applyFix(ctx, "test.go", "fix the bug")
		assert.Equal(t, "success", result["status"])
		assert.Equal(t, "test.go", result["file_path"])
		assert.Equal(t, "fix the bug", result["fix_applied"])
	t.Run("Empty file path", func(t *testing.T) {
		mockRegistry := NewMockToolRegistry()
		result, err := debuggingAgent.applyFix(ctx, "", "fix")
		assert.Equal(t, "skipped", result["status"])
		assert.Contains(t, result["message"], "no file path")
	t.Run("Generate fixed code error", func(t *testing.T) {
			models: []llm.ModelInfo{}, // No models available
		_, err = debuggingAgent.applyFix(ctx, "test.go", "fix")
		assert.Contains(t, err.Error(), "failed to generate fixed code")
	t.Run("FSWrite tool not found", func(t *testing.T) {
					Content: `{"fixed_code": "fixed"}`,
		// Only FSRead, no FSWrite
				return "code", nil
			nil, // No FSWrite
		assert.Contains(t, err.Error(), "failed to get FSWrite tool")
// TestDebuggingAgentGenerateFixedCode tests the generateFixedCode helper function
func TestDebuggingAgentGenerateFixedCode(t *testing.T) {
	t.Run("Successful code generation", func(t *testing.T) {
					Content: `{"fixed_code": "corrected code"}`,
		fixedCode, err := debuggingAgent.generateFixedCode(ctx, "test.go", "apply fix")
		assert.Equal(t, "corrected code", fixedCode)
	t.Run("No models available", func(t *testing.T) {
			models: []llm.ModelInfo{}, // No models
		_, err = debuggingAgent.generateFixedCode(ctx, "test.go", "apply fix")
		assert.Contains(t, err.Error(), "no models available")
	t.Run("Read file error", func(t *testing.T) {
		assert.Contains(t, err.Error(), "failed to read current code")
	t.Run("LLM generation error", func(t *testing.T) {
				return nil, fmt.Errorf("LLM unavailable")
		_, err = debuggingAgent.generateFixedCode(ctx, "test.go", "fix")
	t.Run("Empty content from LLM", func(t *testing.T) {
					Content: "",
		assert.Contains(t, err.Error(), "no fixed code generated")
	t.Run("Invalid JSON response", func(t *testing.T) {
					Content: "not json",
		assert.Contains(t, err.Error(), "failed to parse fix response")
