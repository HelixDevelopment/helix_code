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
// TestNewTestingAgent tests testing agent creation
func TestNewTestingAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		config := &agent.AgentConfig{
			ID:   "testing-1",
			Type: agent.AgentTypeTesting,
			Name: "Test Testing Agent",
		}
		provider := &MockLLMProvider{}
		registry, err := tools.NewToolRegistry(nil)
		require.NoError(t, err)
		testingAgent, err := NewTestingAgent(config, provider, registry)
		require.NotNil(t, testingAgent)
		assert.Equal(t, "testing-1", testingAgent.ID())
		assert.Equal(t, agent.AgentTypeTesting, testingAgent.Type())
	})
	t.Run("Nil provider", func(t *testing.T) {
		agent, err := NewTestingAgent(config, nil, registry)
		assert.Error(t, err)
		assert.Nil(t, agent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	t.Run("Nil tool registry", func(t *testing.T) {
		agent, err := NewTestingAgent(config, provider, nil)
		assert.Contains(t, err.Error(), "tool registry is required")
}
// TestTestingAgentInitialize tests agent initialization
func TestTestingAgentInitialize(t *testing.T) {
	config := &agent.AgentConfig{
		ID:   "testing-1",
		Type: agent.AgentTypeTesting,
		Name: "Test Testing Agent",
	}
	provider := &MockLLMProvider{}
	registry, err := tools.NewToolRegistry(nil)
	require.NoError(t, err)
	testingAgent, err := NewTestingAgent(config, provider, registry)
	ctx := context.Background()
	err = testingAgent.Initialize(ctx, config)
	assert.Equal(t, agent.StatusIdle, testingAgent.Status())
// TestTestingAgentShutdown tests agent shutdown
func TestTestingAgentShutdown(t *testing.T) {
	err = testingAgent.Shutdown(ctx)
	assert.Equal(t, agent.StatusShutdown, testingAgent.Status())
// TestTestingAgentExecuteGenerate tests test generation without execution
func TestTestingAgentExecuteGenerate(t *testing.T) {
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"test_code": "func TestHello(t *testing.T) { }", "test_cases": ["TestHello"]}`,
			}, nil
		},
	testTask := task.NewTask(
		task.TaskTypeTesting,
		"Generate Tests",
		"Generate tests for hello function",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"code": "function hello() { return 'world'; }",
	_, err = testingAgent.Execute(ctx, testTask)
	// Note: Will fail due to FSWrite tool not registered, which is expected in unit tests
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FSWrite")
// TestTestingAgentExecuteWithFilePath tests test generation with file path
func TestTestingAgentExecuteWithFilePath(t *testing.T) {
				Content: `{"test_code": "func TestAdd(t *testing.T) { }", "test_cases": ["TestAdd"]}`,
		"Generate tests for math functions",
		"code":      "func Add(a, b int) int { return a + b }",
		"file_path": "math.go",
// TestTestingAgentExecuteWithFramework tests test generation with custom framework
func TestTestingAgentExecuteWithFramework(t *testing.T) {
			// Verify framework is mentioned in prompt
			assert.Contains(t, request.Messages[0].Content, "testify")
				Content: `{"test_code": "func TestWithTestify(t *testing.T) { }", "test_cases": ["TestWithTestify"]}`,
		"Generate tests with testify",
		"code":           "func Process() error { return nil }",
		"test_framework": "testify",
// TestTestingAgentExecuteMissingCode tests error when code is missing
func TestTestingAgentExecuteMissingCode(t *testing.T) {
		"Test Task",
		"Test",
		"other_field": "value",
	result, err := testingAgent.Execute(ctx, testTask)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "code not found")
	health := testingAgent.Health()
	assert.Equal(t, 1, health.ErrorCount)
// TestTestingAgentExecuteLLMError tests LLM generation error
func TestTestingAgentExecuteLLMError(t *testing.T) {
		models: []llm.ModelInfo{},
		"code": "func Hello() string { return \"world\" }",
// TestTestingAgentCollaborate tests collaboration with coding agents
func TestTestingAgentCollaborate(t *testing.T) {
				Content: `{"test_code": "func TestFunc(t *testing.T) {}", "test_cases": ["TestFunc"]}`,
	// Create a mock coding agent
	codingConfig := &agent.AgentConfig{
		ID:   "coding-1",
		Type: agent.AgentTypeCoding,
		Name: "Test Coding Agent",
	codingAgent := &MockCollabAgent{
		BaseAgent: agent.NewBaseAgent(codingConfig),
		"code": "func Example() {}",
	_, err = testingAgent.Collaborate(ctx, []agent.Agent{codingAgent}, testTask)
// TestTestingAgentTaskMetrics tests task metrics recording
func TestTestingAgentTaskMetrics(t *testing.T) {
				Content: `{"test_code": "func TestOne(t *testing.T) {}\nfunc TestTwo(t *testing.T) {}", "test_cases": ["TestOne", "TestTwo"]}`,
		"code": "func Process() {}",
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
			name:     "Go file",
			input:    "handler.go",
			expected: "handler_test.go",
			name:     "Go file with path",
			input:    "internal/api/handler.go",
			expected: "internal/api/handler_test.go",
			name:     "Non-Go file",
			input:    "script.js",
			expected: "script.js_test",
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTestFilePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
// TestGetTestDirectory tests test directory extraction
func TestGetTestDirectory(t *testing.T) {
			name:     "No directory",
			input:    "test.go",
			expected: ".",
			name:     "Simple directory",
			input:    "pkg/test.go",
			expected: "pkg",
			name:     "Nested directory",
			input:    "internal/api/handler_test.go",
			expected: "internal/api",
			name:     "Root directory",
			input:    "/test.go",
			expected: "",
			result := getTestDirectory(tt.input)
// TestTestingAgentExecuteTests tests the executeTests helper function
func TestTestingAgentExecuteTests(t *testing.T) {
	t.Run("Successful test execution", func(t *testing.T) {
		mockRegistry := CreateMockToolRegistry(
			nil,
			func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return "PASS: TestExample\nok\tpackage\t0.123s", nil
			},
		)
		testingAgent, err := NewTestingAgent(config, provider, ConvertToToolRegistry(mockRegistry))
		ctx := context.Background()
		results, err := testingAgent.executeTests(ctx, "/path/to/test.go")
		assert.NotNil(t, results)
		assert.Equal(t, "completed", results["status"])
		assert.Contains(t, results["raw_output"], "PASS")
	t.Run("Shell tool not found", func(t *testing.T) {
		mockRegistry := NewMockToolRegistry() // Empty registry
		_, err = testingAgent.executeTests(ctx, "/path/to/test.go")
		assert.Contains(t, err.Error(), "failed to get Shell tool")
	t.Run("Test execution failure", func(t *testing.T) {
				return nil, fmt.Errorf("test failed: compilation error")
		assert.Contains(t, err.Error(), "failed to execute tests")
	t.Run("Test directory extraction", func(t *testing.T) {
		// Track which command was executed
		var executedCommand string
				executedCommand = params["command"].(string)
				return "test output", nil
		_, err = testingAgent.executeTests(ctx, "/path/to/package/file_test.go")
		// Verify the correct directory was used in the command
		assert.Contains(t, executedCommand, "/path/to/package")
		assert.Contains(t, executedCommand, "go test")
