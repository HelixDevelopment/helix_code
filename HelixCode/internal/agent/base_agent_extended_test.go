package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
)

// MockLLMProvider implements llm.Provider for testing
type MockLLMProvider struct {
	models       []llm.ModelInfo
	generateFunc func(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error)
	available    bool
}

func NewMockLLMProvider() *MockLLMProvider {
	return &MockLLMProvider{
		models: []llm.ModelInfo{
			{
				ID:          "mock-model",
				Name:        "mock-model",
				Provider:    llm.ProviderTypeLocal,
				ContextSize: 4096,
				MaxTokens:   2048,
			},
		},
		available: true,
	}
}

func (m *MockLLMProvider) GetType() llm.ProviderType {
	return llm.ProviderTypeLocal
}

func (m *MockLLMProvider) GetName() string {
	return "MockProvider"
}

func (m *MockLLMProvider) GetModels() []llm.ModelInfo {
	return m.models
}

func (m *MockLLMProvider) GetCapabilities() []llm.ModelCapability {
	return []llm.ModelCapability{llm.CapabilityCodeGeneration, llm.CapabilityPlanning}
}

func (m *MockLLMProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req)
	}

	// Default mock response
	return &llm.LLMResponse{
		Content: `{"result": "mock response", "code": "func main() {}", "analysis": "test analysis"}`,
		Usage: llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

func (m *MockLLMProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	response, err := m.Generate(ctx, req)
	if err != nil {
		return err
	}
	ch <- *response
	close(ch)
	return nil
}

func (m *MockLLMProvider) IsAvailable(ctx context.Context) bool {
	return m.available
}

func (m *MockLLMProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{
		Status:     "healthy",
		LastCheck:  time.Now(),
		ModelCount: len(m.models),
	}, nil
}

func (m *MockLLMProvider) Close() error {
	return nil
}

// GetContextWindow returns a fixed context-window size for tests.
// Satisfies the llm.Provider interface (added in P1-F01-T02).
func (m *MockLLMProvider) GetContextWindow() int {
	return 4096
}

// CountTokens estimates token count using a simple word-split heuristic.
// Satisfies the llm.Provider interface (added in P1-F01-T02).
func (m *MockLLMProvider) CountTokens(text string) (int, error) {
	// Minimal whitespace-split estimate — good enough for unit tests.
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

// MockToolRegistry creates a mock tool registry for testing
type MockToolRegistry struct {
	tools map[string]tools.Tool
}

func NewMockToolRegistry() *MockToolRegistry {
	return &MockToolRegistry{
		tools: make(map[string]tools.Tool),
	}
}

func (m *MockToolRegistry) Get(name string) (tools.Tool, error) {
	tool, ok := m.tools[name]
	if !ok {
		return nil, errors.New("tool not found: " + name)
	}
	return tool, nil
}

// MockTool implements tools.Tool for testing
type MockTool struct {
	approval.DefaultLevelEdit
	name        string
	executeFunc func(ctx context.Context, params map[string]interface{}) (interface{}, error)
}

func (t *MockTool) Name() string {
	return t.name
}

func (t *MockTool) Description() string {
	return "Mock tool for testing"
}

func (t *MockTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, params)
	}
	return "mock result", nil
}

func (t *MockTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{Type: "object"}
}

func (t *MockTool) Category() tools.ToolCategory {
	return tools.CategoryShell
}

func (t *MockTool) Validate(params map[string]interface{}) error {
	return nil
}

// Tests for executeTask without LLM (basic execution)
func TestBaseAgentExecuteTaskBasic(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
		Capabilities: []Capability{
			CapabilityPlanning,
		},
	})

	ctx := context.Background()

	// Test planning task without LLM
	planningTask := task.NewTask(task.TaskTypePlanning, "Plan Test", "Test planning", task.PriorityNormal)
	planningTask.Input = map[string]interface{}{
		"requirements": "Build a simple web server",
	}

	result, err := agent.executeTaskBasic(ctx, planningTask)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if _, exists := resultMap["plan"]; !exists {
		t.Error("Expected plan in result")
	}
}

func TestBaseAgentExecuteTaskBasicAnalysis(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
	})

	ctx := context.Background()

	// Test analysis task without LLM
	analysisTask := task.NewTask(task.TaskTypeAnalysis, "Analyze", "Test analysis", task.PriorityNormal)
	analysisTask.Input = map[string]interface{}{
		"content": "line1\nline2\nline3",
	}

	result, err := agent.executeTaskBasic(ctx, analysisTask)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	lineCount, ok := resultMap["line_count"].(int)
	if !ok {
		t.Fatal("Expected line_count to be an int")
	}
	if lineCount != 3 {
		t.Errorf("Expected line_count 3, got %d", lineCount)
	}
}

func TestBaseAgentExecuteTaskBasicRequiresLLM(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Test Agent",
	})

	ctx := context.Background()

	// Test code generation requires LLM
	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "Generate", "Test code gen", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{
		"requirements": "Create a hello world function",
	}

	_, err := agent.executeTaskBasic(ctx, codeTask)
	if err == nil {
		t.Error("Expected error for code generation without LLM")
	}
}

func TestBaseAgentExecuteTaskBasicMissingInput(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
	})

	ctx := context.Background()

	// Test planning task without requirements
	planningTask := task.NewTask(task.TaskTypePlanning, "Plan Test", "Test planning", task.PriorityNormal)
	planningTask.Input = map[string]interface{}{} // No requirements

	_, err := agent.executeTaskBasic(ctx, planningTask)
	if err == nil {
		t.Error("Expected error for missing requirements")
	}
}

// Tests for executeTask with LLM
func TestBaseAgentExecuteTaskWithLLM(t *testing.T) {
	mockProvider := NewMockLLMProvider()
	mockProvider.generateFunc = func(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
		return &llm.LLMResponse{
			Content: `{"code": "func hello() { println(\"Hello\") }", "explanation": "A simple hello function"}`,
			Usage:   llm.Usage{TotalTokens: 100},
		}, nil
	}

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Test Agent",
		Capabilities: []Capability{
			CapabilityCodeGeneration,
		},
	})
	agent.SetLLMProvider(mockProvider)

	ctx := context.Background()

	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "Generate", "Test code gen", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{
		"requirements": "Create a hello world function",
	}

	result, err := agent.executeTaskWithLLM(ctx, codeTask)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["code"] == nil {
		t.Error("Expected code in result")
	}
}

func TestBaseAgentExecuteTaskWithLLMNoModels(t *testing.T) {
	mockProvider := &MockLLMProvider{
		models:    []llm.ModelInfo{}, // No models
		available: true,
	}

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Test Agent",
	})
	agent.SetLLMProvider(mockProvider)

	ctx := context.Background()

	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "Generate", "Test code gen", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{
		"requirements": "Create a hello world function",
	}

	_, err := agent.executeTaskWithLLM(ctx, codeTask)
	if err == nil {
		t.Error("Expected error for no models available")
	}
}

func TestBaseAgentExecuteTaskWithLLMGenerateError(t *testing.T) {
	mockProvider := NewMockLLMProvider()
	mockProvider.generateFunc = func(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
		return nil, errors.New("LLM generation failed")
	}

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Test Agent",
	})
	agent.SetLLMProvider(mockProvider)

	ctx := context.Background()

	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "Generate", "Test code gen", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{
		"requirements": "Create a hello world function",
	}

	_, err := agent.executeTaskWithLLM(ctx, codeTask)
	if err == nil {
		t.Error("Expected error when LLM fails")
	}
}

// Tests for prompt building
func TestBaseAgentBuildPromptForTask(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
	})

	tests := []struct {
		name      string
		taskType  task.TaskType
		input     map[string]interface{}
		wantError bool
	}{
		{
			name:     "Planning",
			taskType: task.TaskTypePlanning,
			input: map[string]interface{}{
				"requirements": "Build a web API",
			},
			wantError: false,
		},
		{
			name:     "CodeGeneration",
			taskType: task.TaskTypeCodeGeneration,
			input: map[string]interface{}{
				"requirements": "Create a function",
			},
			wantError: false,
		},
		{
			name:     "CodeEdit",
			taskType: task.TaskTypeCodeEdit,
			input: map[string]interface{}{
				"requirements":  "Add error handling",
				"existing_code": "func main() {}",
			},
			wantError: false,
		},
		{
			name:     "CodeEditMissingCode",
			taskType: task.TaskTypeCodeEdit,
			input: map[string]interface{}{
				"requirements": "Add error handling",
			},
			wantError: true,
		},
		{
			name:     "Debugging",
			taskType: task.TaskTypeDebugging,
			input: map[string]interface{}{
				"error":       "nil pointer dereference",
				"stack_trace": "at main.go:10",
			},
			wantError: false,
		},
		{
			name:      "DebuggingMissingError",
			taskType:  task.TaskTypeDebugging,
			input:     map[string]interface{}{},
			wantError: true,
		},
		{
			name:     "Review",
			taskType: task.TaskTypeReview,
			input: map[string]interface{}{
				"code": "func main() {}",
			},
			wantError: false,
		},
		{
			name:     "Refactoring",
			taskType: task.TaskTypeRefactoring,
			input: map[string]interface{}{
				"code":  "func main() {}",
				"goals": "Improve readability",
			},
			wantError: false,
		},
		{
			name:     "Documentation",
			taskType: task.TaskTypeDocumentation,
			input: map[string]interface{}{
				"code": "func main() {}",
			},
			wantError: false,
		},
		{
			name:     "Testing",
			taskType: task.TaskTypeTesting,
			input: map[string]interface{}{
				"code": "func Add(a, b int) int { return a + b }",
			},
			wantError: false,
		},
		{
			name:     "Analysis",
			taskType: task.TaskTypeAnalysis,
			input: map[string]interface{}{
				"content": "Some content to analyze",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTask := task.NewTask(tt.taskType, tt.name, "Test", task.PriorityNormal)
			testTask.Input = tt.input

			prompt, err := agent.buildPromptForTask(testTask)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if prompt == "" {
					t.Error("Expected non-empty prompt")
				}
			}
		})
	}
}

// Tests for Execute method (full Agent interface)
func TestBaseAgentExecute(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
		Capabilities: []Capability{
			CapabilityPlanning,
		},
	})

	ctx := context.Background()

	planningTask := task.NewTask(task.TaskTypePlanning, "Plan", "Test", task.PriorityNormal)
	planningTask.Input = map[string]interface{}{
		"requirements": "Build a web server",
	}

	result, err := agent.Execute(ctx, planningTask)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	if result.TaskID != planningTask.ID {
		t.Error("Result task ID mismatch")
	}

	if result.AgentID != agent.ID() {
		t.Error("Result agent ID mismatch")
	}

	// Check statistics were updated
	health := agent.Health()
	if health.TaskCount != 1 {
		t.Errorf("Expected task count 1, got %d", health.TaskCount)
	}
}

func TestBaseAgentExecuteWithLLM(t *testing.T) {
	mockProvider := NewMockLLMProvider()

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Test Agent",
		Capabilities: []Capability{
			CapabilityCodeGeneration,
		},
	})
	agent.SetLLMProvider(mockProvider)

	ctx := context.Background()

	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "Generate", "Test", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{
		"requirements": "Create a function",
	}

	result, err := agent.Execute(ctx, codeTask)
	if err != nil {
		t.Errorf("Execute with LLM failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful result")
	}

	// With LLM, confidence should be higher
	if result.Confidence < 0.8 {
		t.Errorf("Expected confidence >= 0.8 with LLM, got %f", result.Confidence)
	}
}

func TestBaseAgentExecuteFailure(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Test Agent",
	})
	// No LLM provider, so code generation will fail

	ctx := context.Background()

	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "Generate", "Test", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{
		"requirements": "Create a function",
	}

	result, err := agent.Execute(ctx, codeTask)
	if err == nil {
		t.Error("Expected error for code generation without LLM")
	}

	if result.Success {
		t.Error("Expected failed result")
	}

	// Check that failed task was counted
	health := agent.Health()
	if health.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", health.ErrorCount)
	}
}

// Tests for Collaborate method
func TestBaseAgentCollaborate(t *testing.T) {
	// Create primary agent
	primaryAgent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "coding-agent",
		Type: AgentTypeCoding,
		Name: "Coding Agent",
		Capabilities: []Capability{
			CapabilityCodeGeneration,
		},
	})

	// Create a mock review agent
	reviewAgent := &MockAgent{
		BaseAgent: NewBaseAgentFromConfig(&AgentConfig{
			ID:   "review-agent",
			Type: AgentTypeReview,
			Name: "Review Agent",
			Capabilities: []Capability{
				CapabilityCodeReview,
			},
		}),
		executeFunc: func(ctx context.Context, t *task.Task) (*task.Result, error) {
			result := task.NewResult(t.ID, "review-agent")
			result.SetSuccess(map[string]interface{}{
				"review_summary": "Code looks good",
				"issues":         []map[string]interface{}{},
			}, 0.9)
			return result, nil
		},
	}

	// Set up mock LLM for coding agent
	mockProvider := NewMockLLMProvider()
	mockProvider.generateFunc = func(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
		return &llm.LLMResponse{
			Content: `{"code": "func hello() {}", "explanation": "Hello function"}`,
			Usage:   llm.Usage{TotalTokens: 100},
		}, nil
	}
	primaryAgent.SetLLMProvider(mockProvider)

	ctx := context.Background()

	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "Generate", "Test", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{
		"requirements": "Create a hello function",
	}

	agents := []Agent{reviewAgent}
	result, err := primaryAgent.Collaborate(ctx, agents, codeTask)

	if err != nil {
		t.Errorf("Collaborate failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful collaboration")
	}

	// Should have at least the primary agent's result
	if len(result.Results) < 1 {
		t.Error("Expected at least one result")
	}

	// Check messages were recorded
	if len(result.Messages) == 0 {
		t.Error("Expected collaboration messages")
	}
}

func TestBaseAgentCollaborateNoOtherAgents(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
		Capabilities: []Capability{
			CapabilityPlanning,
		},
	})

	ctx := context.Background()

	planningTask := task.NewTask(task.TaskTypePlanning, "Plan", "Test", task.PriorityNormal)
	planningTask.Input = map[string]interface{}{
		"requirements": "Build something",
	}

	// No other agents
	result, err := agent.Collaborate(ctx, []Agent{}, planningTask)

	if err != nil {
		t.Errorf("Collaborate with no agents failed: %v", err)
	}

	// Should still succeed with just the primary result
	if len(result.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(result.Results))
	}

	if len(result.Participants) != 1 {
		t.Errorf("Expected 1 participant, got %d", len(result.Participants))
	}
}

// Tests for shouldCollaborateWith
func TestBaseAgentShouldCollaborateWith(t *testing.T) {
	tests := []struct {
		name           string
		agentType      AgentType
		otherType      AgentType
		shouldCollab   bool
		collaborateFor string
	}{
		{"CodingWithReview", AgentTypeCoding, AgentTypeReview, true, "review"},
		{"CodingWithTesting", AgentTypeCoding, AgentTypeTesting, true, "testing"},
		{"CodingWithPlanning", AgentTypeCoding, AgentTypePlanning, false, ""},
		{"PlanningWithPlanning", AgentTypePlanning, AgentTypePlanning, true, "consensus"},
		{"DebuggingWithTesting", AgentTypeDebugging, AgentTypeTesting, true, "verification"},
		{"ReviewWithRefactoring", AgentTypeReview, AgentTypeRefactoring, true, "refactoring"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := NewBaseAgentFromConfig(&AgentConfig{
				ID:   "agent",
				Type: tt.agentType,
				Name: "Agent",
			})

			other := NewBaseAgentFromConfig(&AgentConfig{
				ID:   "other",
				Type: tt.otherType,
				Name: "Other",
			})

			shouldCollab, collabType := agent.shouldCollaborateWith(other, nil)

			if shouldCollab != tt.shouldCollab {
				t.Errorf("Expected shouldCollaborate=%v, got %v", tt.shouldCollab, shouldCollab)
			}

			if tt.shouldCollab && collabType != tt.collaborateFor {
				t.Errorf("Expected collaborationType=%s, got %s", tt.collaborateFor, collabType)
			}
		})
	}
}

// Tests for Initialize and Shutdown
func TestBaseAgentInitialize(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
	})

	ctx := context.Background()

	// Initialize with new config
	newConfig := &AgentConfig{
		ID:   "updated-agent",
		Name: "Updated Agent",
		Type: AgentTypeCoding,
		Capabilities: []Capability{
			CapabilityCodeGeneration,
		},
	}

	err := agent.Initialize(ctx, newConfig)
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}

	if agent.ID() != "updated-agent" {
		t.Errorf("Expected ID updated-agent, got %s", agent.ID())
	}

	if agent.Name() != "Updated Agent" {
		t.Errorf("Expected name Updated Agent, got %s", agent.Name())
	}

	if agent.Type() != AgentTypeCoding {
		t.Errorf("Expected type coding, got %s", agent.Type())
	}

	caps := agent.Capabilities()
	if len(caps) != 1 || caps[0] != CapabilityCodeGeneration {
		t.Error("Capabilities not updated correctly")
	}
}

func TestBaseAgentShutdown(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
	})

	ctx := context.Background()

	err := agent.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	if agent.Status() != StatusShutdown {
		t.Errorf("Expected shutdown status, got %s", agent.Status())
	}
}

// Tests for LLM and Tool Registry setters/getters
func TestBaseAgentSetGetLLMProvider(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Test Agent",
	})

	// Initially nil
	if agent.GetLLMProvider() != nil {
		t.Error("Expected nil LLM provider initially")
	}

	// Set provider
	mockProvider := NewMockLLMProvider()
	agent.SetLLMProvider(mockProvider)

	if agent.GetLLMProvider() == nil {
		t.Error("Expected LLM provider to be set")
	}

	if agent.GetLLMProvider().GetName() != "MockProvider" {
		t.Errorf("Expected MockProvider, got %s", agent.GetLLMProvider().GetName())
	}
}

func TestBaseAgentSetGetToolRegistry(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeTesting,
		Name: "Test Agent",
	})

	// Initially nil
	if agent.GetToolRegistry() != nil {
		t.Error("Expected nil tool registry initially")
	}

	// Note: We can't easily create a real ToolRegistry without dependencies,
	// but we can test the setter/getter work with nil
	agent.SetToolRegistry(nil)
	if agent.GetToolRegistry() != nil {
		t.Error("Expected nil tool registry after setting nil")
	}
}

// Tests for countLinesInString helper
func TestCountLinesInString(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello\n", 2},
		{"line1\nline2", 2},
		{"line1\nline2\nline3", 3},
		{"\n\n\n", 4},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := countLinesInString(tt.input)
			if result != tt.expected {
				t.Errorf("countLinesInString(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// Tests for getSystemPrompt
func TestBaseAgentGetSystemPrompt(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypeCoding,
		Name: "Code Bot",
		Capabilities: []Capability{
			CapabilityCodeGeneration,
			CapabilityCodeAnalysis,
		},
	})

	prompt := agent.getSystemPrompt()

	if prompt == "" {
		t.Error("Expected non-empty system prompt")
	}

	// Check it contains agent info
	if !containsString(prompt, "coding") {
		t.Error("Expected system prompt to contain agent type")
	}

	if !containsString(prompt, "Code Bot") {
		t.Error("Expected system prompt to contain agent name")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Concurrent execution tests
func TestBaseAgentConcurrentExecute(t *testing.T) {
	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID:   "test-agent",
		Type: AgentTypePlanning,
		Name: "Test Agent",
		Capabilities: []Capability{
			CapabilityPlanning,
		},
	})

	ctx := context.Background()

	// Run multiple executions concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			planningTask := task.NewTask(task.TaskTypePlanning, "Plan", "Test", task.PriorityNormal)
			planningTask.Input = map[string]interface{}{
				"requirements": "Build something",
			}

			_, _ = agent.Execute(ctx, planningTask)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check task count
	health := agent.Health()
	if health.TaskCount != 10 {
		t.Errorf("Expected task count 10, got %d", health.TaskCount)
	}
}
