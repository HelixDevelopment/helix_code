package types

import (
	"context"
	"testing"
	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
// TestNewCodingAgent tests coding agent creation
func TestNewCodingAgent(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		config := &agent.AgentConfig{
			ID:   "coding-1",
			Type: agent.AgentTypeCoding,
			Name: "Test Coding Agent",
		}
		provider := &MockLLMProvider{}
		registry, err := tools.NewToolRegistry(nil)
		require.NoError(t, err)
		codingAgent, err := NewCodingAgent(config, provider, registry)
		require.NotNil(t, codingAgent)
		assert.Equal(t, "coding-1", codingAgent.ID())
		assert.Equal(t, agent.AgentTypeCoding, codingAgent.Type())
	})
	t.Run("Nil provider", func(t *testing.T) {
		agent, err := NewCodingAgent(config, nil, registry)
		assert.Error(t, err)
		assert.Nil(t, agent)
		assert.Contains(t, err.Error(), "LLM provider is required")
	t.Run("Nil tool registry", func(t *testing.T) {
		agent, err := NewCodingAgent(config, provider, nil)
		assert.Contains(t, err.Error(), "tool registry is required")
}
// TestCodingAgentInitialize tests agent initialization
func TestCodingAgentInitialize(t *testing.T) {
	config := &agent.AgentConfig{
		ID:   "coding-1",
		Type: agent.AgentTypeCoding,
		Name: "Test Coding Agent",
	}
	provider := &MockLLMProvider{}
	registry, err := tools.NewToolRegistry(nil)
	require.NoError(t, err)
	codingAgent, err := NewCodingAgent(config, provider, registry)
	ctx := context.Background()
	err = codingAgent.Initialize(ctx, config)
	assert.Equal(t, agent.StatusIdle, codingAgent.Status())
// TestCodingAgentShutdown tests agent shutdown
func TestCodingAgentShutdown(t *testing.T) {
	err = codingAgent.Shutdown(ctx)
	assert.Equal(t, agent.StatusShutdown, codingAgent.Status())
// TestCodingAgentExecuteCreate tests code creation
func TestCodingAgentExecuteCreate(t *testing.T) {
	provider := &MockLLMProvider{
		generateFunc: func(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content: `{"code": "function hello() { return 'world'; }", "explanation": "Simple hello function"}`,
			}, nil
		},
	testTask := task.NewTask(
		task.TaskTypeCodeGeneration,
		"Create Hello Function",
		"Create a simple hello function",
		task.PriorityNormal,
	)
	testTask.Input = map[string]interface{}{
		"requirements": "Create a function that returns 'hello world'",
	result, err := codingAgent.Execute(ctx, testTask)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "code")
	assert.Contains(t, result.Output, "explanation")
	assert.Contains(t, result.Output, "operation")
	assert.Equal(t, "create", result.Output["operation"])
// TestCodingAgentExecuteEdit tests code editing
func TestCodingAgentExecuteEdit(t *testing.T) {
				Content: `{"code": "function hello() { return 'world!'; }", "explanation": "Added exclamation"}`,
		"Edit Hello Function",
		"Add exclamation to hello function",
		"requirements":  "Add exclamation mark",
		"existing_code": "function hello() { return 'world'; }",
	assert.Equal(t, "edit", result.Output["operation"])
// TestCodingAgentExecuteMissingRequirements tests error when requirements missing
func TestCodingAgentExecuteMissingRequirements(t *testing.T) {
		"Test Task",
		"Test",
		"other_field": "value",
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "requirements not found")
	health := codingAgent.Health()
	assert.Equal(t, 1, health.ErrorCount)
// TestCodingAgentExecuteLLMError tests LLM generation error
func TestCodingAgentExecuteLLMError(t *testing.T) {
		models: []llm.ModelInfo{},
		"requirements": "Create a function",
// TestCodingAgentCollaborate tests collaboration with review agents
func TestCodingAgentCollaborate(t *testing.T) {
				Content: `{"code": "function test() {}", "explanation": "Test function"}`,
	// Create a mock review agent
	reviewConfig := &agent.AgentConfig{
		ID:   "review-1",
		Type: agent.AgentTypeReview,
		Name: "Test Review Agent",
	reviewAgent := &MockCollabAgent{
		BaseAgent: agent.NewBaseAgent(reviewConfig),
		"requirements": "Create a test function",
	result, err := codingAgent.Collaborate(ctx, []agent.Agent{reviewAgent}, testTask)
	assert.Contains(t, result.Participants, codingAgent.ID())
	assert.Contains(t, result.Participants, reviewAgent.ID())
	assert.NotNil(t, result.Consensus)
// TestCodingAgentTaskMetrics tests task metrics recording
func TestCodingAgentTaskMetrics(t *testing.T) {
				Content: `{"code": "line1\nline2\nline3", "explanation": "Three lines"}`,
		"requirements": "Create code",
	assert.NotNil(t, result.Metrics)
	assert.Greater(t, result.Metrics.LinesAdded, 0)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
// MockCollabAgent for collaboration testing
type MockCollabAgent struct {
	*agent.BaseAgent
func (m *MockCollabAgent) Initialize(ctx context.Context, config *config.AgentConfig) error {
	return nil
func (m *MockCollabAgent) Execute(ctx context.Context, t *task.Task) (*task.Result, error) {
	result := task.NewResult(t.ID, m.ID())
	result.SetSuccess(map[string]interface{}{"review": "approved"}, 0.9)
	return result, nil
func (m *MockCollabAgent) Collaborate(ctx context.Context, agents []agent.Agent, t *task.Task) (*agent.CollaborationResult, error) {
	return &agent.CollaborationResult{Success: true}, nil
func (m *MockCollabAgent) Shutdown(ctx context.Context) error {
