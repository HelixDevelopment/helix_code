package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/hooks"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/compression"
	"dev.helix.code/internal/llm/compressioniface"
	"dev.helix.code/internal/telemetry"
	"dev.helix.code/internal/tools"
)

// Task represents a task that can be executed by an agent
type Task = task.Task

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID      string                 `json:"task_id"`
	Success     bool                   `json:"success"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	CompletedAt time.Time              `json:"completed_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BaseAgent provides a basic implementation of the Agent interface
type BaseAgent struct {
	id           string
	name         string
	agentType    AgentType
	status       AgentStatus
	capabilities []Capability
	taskQueue    chan *Task
	resultChan   chan *TaskResult
	stopChan     chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex

	// Statistics
	tasksProcessed int
	tasksSucceeded int
	tasksFailed    int
	totalDuration  time.Duration
	lastActivity   time.Time
	startTime      time.Time

	// Configuration
	maxConcurrency int
	timeout        time.Duration
	retryCount     int

	// LLM and Tools integration (optional - can be nil for simple agents)
	llmProvider  llm.Provider
	toolRegistry *tools.ToolRegistry

	// autoCompactor handles claude-code-style 80%-window auto-compaction.
	// Optional: nil = compaction is disabled (graceful degradation).
	// Per Feature 1 (claude-code auto-compaction port), P1-F01-T07.
	autoCompactor *compression.AutoCompactor

	// hooksManager fires lifecycle hook events (OnError, OnPlanApproval, etc.).
	// Optional: nil = hook firing is disabled (graceful degradation).
	// Per Feature 5 (hook-based extensibility), P1-F05-T08.
	hooksManager *hooks.Manager

	// telemetry is the optional OTel-based span+metric helper for the agent
	// loop's per-iteration boundary. nil = no telemetry (graceful degradation).
	// Per Feature 16 (telemetry), P1-F16-T08.
	telemetry *telemetry.AgentInstrumentation
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(id, name string, agentType AgentType, config *config.AgentConfig) *BaseAgent {
	maxConcurrency := 1
	timeout := 30 * time.Second
	retryCount := 3

	if config != nil {
		if config.MaxConcurrency > 0 {
			maxConcurrency = config.MaxConcurrency
		}
		if config.Timeout > 0 {
			timeout = time.Duration(config.Timeout) * time.Second
		}
		if config.RetryCount >= 0 {
			retryCount = config.RetryCount
		}
	}

	now := time.Now()
	return &BaseAgent{
		id:             id,
		name:           name,
		agentType:      agentType,
		status:         StatusIdle,
		capabilities:   []Capability{},
		taskQueue:      make(chan *Task, 100),
		resultChan:     make(chan *TaskResult, 100),
		stopChan:       make(chan struct{}),
		maxConcurrency: maxConcurrency,
		timeout:        timeout,
		retryCount:     retryCount,
		lastActivity:   now,
		startTime:      now,
	}
}

// ID returns the agent ID
func (a *BaseAgent) ID() string {
	return a.id
}

// Name returns the agent name
func (a *BaseAgent) Name() string {
	return a.name
}

// Type returns the agent type
func (a *BaseAgent) Type() AgentType {
	return a.agentType
}

// Status returns the current agent status
func (a *BaseAgent) Status() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// SetStatus sets the agent status
func (a *BaseAgent) SetStatus(status AgentStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
	a.lastActivity = time.Now()
}

// Capabilities returns the agent capabilities
func (a *BaseAgent) Capabilities() []Capability {
	a.mu.RLock()
	defer a.mu.RUnlock()
	caps := make([]Capability, len(a.capabilities))
	copy(caps, a.capabilities)
	return caps
}

// AddCapability adds a capability to the agent
func (a *BaseAgent) AddCapability(capability Capability) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.capabilities = append(a.capabilities, capability)
}

// RemoveCapability removes a capability from the agent
func (a *BaseAgent) RemoveCapability(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, cap := range a.capabilities {
		if string(cap) == name {
			a.capabilities = append(a.capabilities[:i], a.capabilities[i+1:]...)
			break
		}
	}
}

// CanHandleTaskType checks if the agent can handle a specific task type
func (a *BaseAgent) CanHandleTaskType(taskType string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, cap := range a.capabilities {
		if string(cap) == taskType {
			return true
		}
	}
	return false
}

// SubmitTask submits a task for execution
func (a *BaseAgent) SubmitTask(ctx context.Context, task *Task) (*TaskResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	// Check if agent can handle this task type
	if !a.CanHandleTaskType(string(task.Type)) {
		return nil, fmt.Errorf("agent cannot handle task type: %s", task.Type)
	}

	// Set status to busy
	a.SetStatus(StatusBusy)

	// Create a context with timeout
	taskCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Execute the task
	startTime := time.Now()
	result, err := a.executeTask(taskCtx, task)
	duration := time.Since(startTime)

	// Update statistics
	a.mu.Lock()
	a.tasksProcessed++
	a.totalDuration += duration
	if err != nil {
		a.tasksFailed++
	} else {
		a.tasksSucceeded++
	}
	a.mu.Unlock()

	// Set status back to idle
	a.SetStatus(StatusIdle)

	if err != nil {
		return &TaskResult{
			TaskID:      task.ID,
			Success:     false,
			Error:       err.Error(),
			Duration:    duration,
			CompletedAt: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:      task.ID,
		Success:     true,
		Result:      result,
		Duration:    duration,
		CompletedAt: time.Now(),
	}, nil
}

// executeTask executes a task using the configured LLM provider and tools
func (a *BaseAgent) executeTask(ctx context.Context, t *Task) (interface{}, error) {
	// If no LLM provider is configured, return basic execution result
	if a.llmProvider == nil {
		return a.executeTaskBasic(ctx, t)
	}

	// Execute with LLM assistance
	return a.executeTaskWithLLM(ctx, t)
}

// executeTaskBasic provides basic task execution without LLM
func (a *BaseAgent) executeTaskBasic(ctx context.Context, t *Task) (interface{}, error) {
	// For basic execution without LLM, just process based on task type
	switch t.Type {
	case task.TaskTypePlanning:
		return a.basicPlanning(t)
	case task.TaskTypeAnalysis:
		return a.basicAnalysis(t)
	case task.TaskTypeCodeGeneration, task.TaskTypeCodeEdit:
		return nil, fmt.Errorf("code tasks require LLM provider to be configured")
	case task.TaskTypeTesting:
		return a.basicTesting(ctx, t)
	case task.TaskTypeDebugging:
		return nil, fmt.Errorf("debugging tasks require LLM provider to be configured")
	case task.TaskTypeReview:
		return nil, fmt.Errorf("review tasks require LLM provider to be configured")
	case task.TaskTypeRefactoring:
		return nil, fmt.Errorf("refactoring tasks require LLM provider to be configured")
	case task.TaskTypeDocumentation:
		return nil, fmt.Errorf("documentation tasks require LLM provider to be configured")
	default:
		return map[string]interface{}{
			"message":   "Task processed",
			"task_id":   t.ID,
			"task_type": t.Type,
			"status":    "completed",
		}, nil
	}
}

// basicPlanning provides basic planning without LLM
func (a *BaseAgent) basicPlanning(t *Task) (interface{}, error) {
	requirements, _ := t.Input["requirements"].(string)
	if requirements == "" {
		return nil, fmt.Errorf("requirements not found in task input")
	}

	// Return a basic plan structure
	return map[string]interface{}{
		"plan":        "Basic plan generated without LLM assistance",
		"subtasks":    []map[string]interface{}{},
		"total_tasks": 0,
		"note":        "For detailed planning, configure an LLM provider",
	}, nil
}

// basicAnalysis provides basic analysis without LLM
func (a *BaseAgent) basicAnalysis(t *Task) (interface{}, error) {
	content, _ := t.Input["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content not found in task input")
	}

	// Return basic analysis metrics
	lineCount := countLinesInString(content)
	return map[string]interface{}{
		"analysis":   "Basic analysis without LLM",
		"line_count": lineCount,
		"char_count": len(content),
	}, nil
}

// basicTesting provides basic test execution using shell tools
func (a *BaseAgent) basicTesting(ctx context.Context, t *Task) (interface{}, error) {
	if a.toolRegistry == nil {
		return nil, fmt.Errorf("tool registry required for test execution")
	}

	// Get test command from input or use default
	testCmd, ok := t.Input["test_command"].(string)
	if !ok {
		// Default to go test
		testDir, _ := t.Input["test_directory"].(string)
		if testDir == "" {
			testDir = "."
		}
		testCmd = fmt.Sprintf("go test -v %s", testDir)
	}

	// Execute test using shell tool
	shellTool, err := a.toolRegistry.Get("Shell")
	if err != nil {
		return nil, fmt.Errorf("shell tool not available: %w", err)
	}

	result, err := shellTool.Execute(ctx, map[string]interface{}{
		"command": testCmd,
		"timeout": 60000, // 60 second timeout
	})
	if err != nil {
		return map[string]interface{}{
			"status":  "failed",
			"error":   err.Error(),
			"command": testCmd,
		}, err
	}

	return map[string]interface{}{
		"status":  "completed",
		"output":  result,
		"command": testCmd,
	}, nil
}

// executeTaskWithLLM executes a task using the LLM provider
func (a *BaseAgent) executeTaskWithLLM(ctx context.Context, t *Task) (result interface{}, iterErr error) {
	// Wrap iteration body in a telemetry span when wired. The LLM-driven
	// path here represents one agent loop iteration (build prompt → call
	// LLM → process response). nil telemetry = graceful no-op.
	// Per Feature 16 (telemetry), P1-F16-T08.
	if a.telemetry != nil {
		var finish func(error)
		ctx, finish = a.telemetry.BeginIteration(ctx, 0, t.ID)
		defer func() { finish(iterErr) }()
	}

	// Build the prompt based on task type
	prompt, err := a.buildPromptForTask(t)
	if err != nil {
		iterErr = fmt.Errorf("failed to build prompt: %w", err)
		return nil, iterErr
	}

	// Get available models
	models := a.llmProvider.GetModels()
	if len(models) == 0 {
		iterErr = fmt.Errorf("no models available from provider")
		return nil, iterErr
	}

	// Create LLM request
	request := &llm.LLMRequest{
		Model: models[0].Name,
		Messages: []llm.Message{
			{Role: "system", Content: a.getSystemPrompt()},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   4000,
		Temperature: 0.3, // Low temperature for consistent results
	}

	// Run auto-compaction on the request messages if an AutoCompactor is wired.
	// Nil autoCompactor = graceful no-op (backwards-compatible). Per P1-F01-T07.
	if a.autoCompactor != nil {
		conv := buildConversationForCompaction(request.Messages)
		if _, err := a.autoCompactor.MaybeCompact(ctx, conv); err != nil {
			iterErr = fmt.Errorf("auto-compaction: %w", err)
			return nil, iterErr
		}
	}

	// Execute LLM request
	response, err := a.llmProvider.Generate(ctx, request)
	if err != nil {
		llmErr := fmt.Errorf("LLM generation failed: %w", err)
		a.dispatchOnError(ctx, llmErr, "llm")
		iterErr = llmErr
		return nil, iterErr
	}

	// Parse and process the response based on task type
	processed, err := a.processLLMResponse(ctx, t, response)
	if err != nil {
		llmRespErr := fmt.Errorf("failed to process LLM response: %w", err)
		a.dispatchOnError(ctx, llmRespErr, "llm")
		iterErr = llmRespErr
		return nil, iterErr
	}

	return processed, nil
}

// buildPromptForTask builds an appropriate prompt for the task type
func (a *BaseAgent) buildPromptForTask(t *Task) (string, error) {
	switch t.Type {
	case task.TaskTypePlanning:
		requirements, _ := t.Input["requirements"].(string)
		if requirements == "" {
			return "", fmt.Errorf("requirements not found in task input")
		}
		return fmt.Sprintf(`Analyze the following requirements and create a detailed technical plan:

Requirements:
%s

Provide:
1. Brief analysis of requirements
2. Key technical decisions
3. Breakdown of subtasks with type, description, priority, and dependencies
4. Potential risks and mitigations

Format as JSON:
{
  "analysis": "requirement analysis",
  "decisions": ["decision 1", "decision 2"],
  "subtasks": [{"title": "...", "type": "...", "priority": 1-4, "depends_on": []}],
  "risks": [{"risk": "...", "mitigation": "..."}]
}`, requirements), nil

	case task.TaskTypeCodeGeneration:
		requirements, _ := t.Input["requirements"].(string)
		if requirements == "" {
			return "", fmt.Errorf("requirements not found in task input")
		}
		return fmt.Sprintf(`Generate code according to these requirements:

Requirements:
%s

Format response as JSON:
{
  "code": "the complete code",
  "explanation": "explanation of implementation"
}`, requirements), nil

	case task.TaskTypeCodeEdit:
		requirements, _ := t.Input["requirements"].(string)
		existingCode, _ := t.Input["existing_code"].(string)
		if requirements == "" || existingCode == "" {
			return "", fmt.Errorf("requirements and existing_code required for code editing")
		}
		return fmt.Sprintf(`Modify the following code according to requirements:

Requirements:
%s

Existing Code:
%s

Format response as JSON:
{
  "code": "the complete modified code",
  "explanation": "explanation of changes"
}`, requirements, existingCode), nil

	case task.TaskTypeDebugging:
		errorMsg, _ := t.Input["error"].(string)
		stackTrace, _ := t.Input["stack_trace"].(string)
		codeContext, _ := t.Input["code_context"].(string)
		if errorMsg == "" {
			return "", fmt.Errorf("error message required for debugging")
		}
		return fmt.Sprintf(`Analyze this error and identify the root cause:

Error:
%s

Stack Trace:
%s

Code Context:
%s

Format response as JSON:
{
  "analysis": "detailed analysis",
  "root_cause": "root cause",
  "suggested_fixes": ["fix 1", "fix 2"]
}`, errorMsg, stackTrace, codeContext), nil

	case task.TaskTypeReview:
		code, _ := t.Input["code"].(string)
		if code == "" {
			return "", fmt.Errorf("code required for review")
		}
		return fmt.Sprintf(`Review this code for quality, security, and best practices:

Code:
%s

Format response as JSON:
{
  "review_summary": "overall assessment",
  "issues": [{"severity": "low/medium/high/critical", "description": "...", "recommendation": "..."}],
  "suggestions": ["suggestion 1", "suggestion 2"],
  "metrics": {"quality_score": 0-100}
}`, code), nil

	case task.TaskTypeRefactoring:
		code, _ := t.Input["code"].(string)
		goals, _ := t.Input["goals"].(string)
		if code == "" {
			return "", fmt.Errorf("code required for refactoring")
		}
		return fmt.Sprintf(`Refactor this code according to the goals:

Code:
%s

Refactoring Goals:
%s

Format response as JSON:
{
  "refactored_code": "the refactored code",
  "changes": ["change 1", "change 2"],
  "improvements": ["improvement 1", "improvement 2"]
}`, code, goals), nil

	case task.TaskTypeDocumentation:
		code, _ := t.Input["code"].(string)
		if code == "" {
			return "", fmt.Errorf("code required for documentation")
		}
		return fmt.Sprintf(`Generate documentation for this code:

Code:
%s

Format response as JSON:
{
  "documentation": "the generated documentation in markdown",
  "summary": "brief summary",
  "functions": [{"name": "...", "description": "...", "params": [], "returns": "..."}]
}`, code), nil

	case task.TaskTypeTesting:
		code, _ := t.Input["code"].(string)
		if code == "" {
			return "", fmt.Errorf("code required for test generation")
		}
		return fmt.Sprintf(`Generate comprehensive tests for this code:

Code:
%s

Format response as JSON:
{
  "test_code": "complete test code",
  "test_cases": ["TestCase1", "TestCase2"],
  "coverage_notes": "notes about test coverage"
}`, code), nil

	case task.TaskTypeAnalysis:
		content, _ := t.Input["content"].(string)
		if content == "" {
			return "", fmt.Errorf("content required for analysis")
		}
		return fmt.Sprintf(`Analyze the following content:

Content:
%s

Format response as JSON:
{
  "analysis": "detailed analysis",
  "findings": ["finding 1", "finding 2"],
  "recommendations": ["recommendation 1", "recommendation 2"]
}`, content), nil

	default:
		// Generic task handling
		description := t.Description
		if description == "" {
			description = t.Title
		}
		return fmt.Sprintf(`Execute the following task:

Task: %s
Description: %s
Input: %v

Format response as JSON:
{
  "result": "task result",
  "notes": "any relevant notes"
}`, t.Title, description, t.Input), nil
	}
}

// getSystemPrompt returns the system prompt for the agent
func (a *BaseAgent) getSystemPrompt() string {
	return fmt.Sprintf(`You are a %s agent named %s. Your capabilities include: %v.

You are part of a multi-agent system for software development. Your responses should be:
1. Precise and actionable
2. Formatted as JSON as requested
3. Focused on your area of expertise

Tool output handling: when a tool produces output exceeding 50,000 characters, the runtime persists the raw content to disk. The tool result you receive will contain a "persistedOutputPath" pointing to a file under .helix/tool-results/. To read the full content, invoke the Read tool with that path. Treat the path as a regular workspace file.

Always respond with valid JSON only, no additional text.`, a.agentType, a.name, a.capabilities)
}

// processLLMResponse processes the LLM response based on task type
func (a *BaseAgent) processLLMResponse(ctx context.Context, t *Task, response *llm.LLMResponse) (interface{}, error) {
	if response.Content == "" {
		return nil, fmt.Errorf("empty response from LLM")
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		// If JSON parsing fails, wrap the content
		result = map[string]interface{}{
			"raw_response": response.Content,
			"parse_error":  err.Error(),
		}
	}

	// Add metadata
	result["task_id"] = t.ID
	result["task_type"] = string(t.Type)
	result["agent_id"] = a.id
	result["tokens_used"] = response.Usage.TotalTokens

	// For code generation/edit tasks, optionally apply changes using tools
	if a.toolRegistry != nil {
		switch t.Type {
		case task.TaskTypeCodeGeneration, task.TaskTypeCodeEdit:
			if err := a.applyCodeChangesIfRequested(ctx, t, result); err != nil {
				result["apply_error"] = err.Error()
			}
		}
	}

	return result, nil
}

// applyCodeChangesIfRequested applies generated code to files if requested
func (a *BaseAgent) applyCodeChangesIfRequested(ctx context.Context, t *Task, result map[string]interface{}) error {
	// Check if auto_apply is requested
	autoApply, _ := t.Input["auto_apply"].(bool)
	if !autoApply {
		return nil
	}

	filePath, _ := t.Input["file_path"].(string)
	if filePath == "" {
		return nil // No file path specified
	}

	code, _ := result["code"].(string)
	if code == "" {
		return fmt.Errorf("no code in result to apply")
	}

	// Get the write tool
	writeTool, err := a.toolRegistry.Get("FSWrite")
	if err != nil {
		return fmt.Errorf("FSWrite tool not available: %w", err)
	}

	// Write the code
	_, err = writeTool.Execute(ctx, map[string]interface{}{
		"path":    filePath,
		"content": code,
	})
	if err != nil {
		return fmt.Errorf("failed to write code: %w", err)
	}

	result["file_written"] = filePath
	return nil
}

// countLinesInString counts lines in a string
func countLinesInString(s string) int {
	if s == "" {
		return 0
	}
	lines := 1
	for _, c := range s {
		if c == '\n' {
			lines++
		}
	}
	return lines
}

// SetLLMProvider sets the LLM provider for the agent
func (a *BaseAgent) SetLLMProvider(provider llm.Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.llmProvider = provider
}

// GetLLMProvider returns the LLM provider
func (a *BaseAgent) GetLLMProvider() llm.Provider {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.llmProvider
}

// SetToolRegistry sets the tool registry for the agent
func (a *BaseAgent) SetToolRegistry(registry *tools.ToolRegistry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.toolRegistry = registry
}

// GetToolRegistry returns the tool registry
func (a *BaseAgent) GetToolRegistry() *tools.ToolRegistry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.toolRegistry
}

// SetAutoCompactor enables auto-compaction on this agent. Pass nil to disable.
// When non-nil, MaybeCompact() runs before each LLM Generate call.
// Per Feature 1 (claude-code auto-compaction port), P1-F01-T07.
func (a *BaseAgent) SetAutoCompactor(ac *compression.AutoCompactor) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.autoCompactor = ac
}

// GetAutoCompactor returns the current AutoCompactor (may be nil).
func (a *BaseAgent) GetAutoCompactor() *compression.AutoCompactor {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.autoCompactor
}

// SetTelemetryInstrumentation wires an *telemetry.AgentInstrumentation so the
// agent loop can emit a span + iteration counter + latency histogram per
// LLM-driven iteration. Pass nil to disable. When non-nil, executeTaskWithLLM
// brackets its body with BeginIteration / finish closure.
// Per Feature 16 (telemetry), P1-F16-T08.
func (a *BaseAgent) SetTelemetryInstrumentation(ti *telemetry.AgentInstrumentation) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.telemetry = ti
}

// GetTelemetryInstrumentation returns the current AgentInstrumentation (may be nil).
func (a *BaseAgent) GetTelemetryInstrumentation() *telemetry.AgentInstrumentation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.telemetry
}

// SetHooksManager wires a hooks.Manager so the agent's lifecycle code can
// fire OnError (on tool/LLM errors in the message loop) and OnPlanApproval
// (when the plan-mode approval gate calls RequestPlanApproval). A nil
// manager disables hook firing.
// Per Feature 5 (hook-based extensibility), P1-F05-T08.
func (a *BaseAgent) SetHooksManager(m *hooks.Manager) {
	a.hooksManager = m
}

// dispatchOnError fires HookTypeOnError synchronously with a payload of
// {error_message, error_type}. Sync (TriggerEventAndWait) is used so test
// observation is deterministic; the returned blockers are deliberately
// IGNORED — the error has already happened and the agent loop's job is
// to report it, not retry.
// Per Feature 5 (hook-based extensibility), P1-F05-T08.
func (a *BaseAgent) dispatchOnError(ctx context.Context, err error, errorType string) {
	if a.hooksManager == nil || err == nil {
		return
	}
	event := hooks.NewEventWithContext(ctx, hooks.HookTypeOnError)
	event.Source = "agent"
	event.SetData("error_message", err.Error())
	event.SetData("error_type", errorType)
	_ = a.hooksManager.TriggerEventAndWait(event)
}

// RequestPlanApproval fires HookTypeOnPlanApproval synchronously. A
// blocking hook surfaces as a returned error; nil error means all hooks
// allow the plan. F05 ships this method but does NOT call it in the agent
// message loop — F08 (Plan Mode) wires it into the actual approval gate.
// Per Feature 5 (hook-based extensibility), P1-F05-T08.
func (a *BaseAgent) RequestPlanApproval(ctx context.Context, plan string) error {
	if a.hooksManager == nil {
		return nil
	}
	event := hooks.NewEventWithContext(ctx, hooks.HookTypeOnPlanApproval)
	event.Source = "agent"
	event.SetData("plan_text", plan)
	results := a.hooksManager.TriggerEventAndWait(event)
	if blockers := hooks.Blockers(results); len(blockers) > 0 {
		return fmt.Errorf("plan approval blocked: %v", blockers[0])
	}
	return nil
}

// buildConversationForCompaction converts []llm.Message (the agent's per-request
// message slice) into a *compressioniface.Conversation that AutoCompactor can
// operate on. Only Role and Content are projected; token counting and metadata
// are handled inside the compression layer. Per P1-F01-T07.
func buildConversationForCompaction(msgs []llm.Message) *compressioniface.Conversation {
	ciMsgs := make([]*compressioniface.Message, 0, len(msgs))
	for _, m := range msgs {
		ciMsgs = append(ciMsgs, &compressioniface.Message{
			Role:    compressioniface.MessageRole(m.Role),
			Content: m.Content,
		})
	}
	return &compressioniface.Conversation{Messages: ciMsgs}
}

// Execute implements the Agent interface Execute method
// This method executes a task and returns a task.Result
func (a *BaseAgent) Execute(ctx context.Context, t *task.Task) (*task.Result, error) {
	a.SetStatus(StatusBusy)
	defer a.SetStatus(StatusIdle)

	startTime := time.Now()
	result := task.NewResult(t.ID, a.ID())

	// Execute the task
	output, err := a.executeTask(ctx, t)
	result.Duration = time.Since(startTime)

	if err != nil {
		// Dispatch OnError hook synchronously so subscribers can react to the
		// task-level error. Blockers are ignored — the error has already occurred.
		// Per Feature 5 (hook-based extensibility), P1-F05-T08.
		a.dispatchOnError(ctx, err, "tool")
		result.SetFailure(err)
		a.mu.Lock()
		a.tasksFailed++
		a.tasksProcessed++
		a.totalDuration += result.Duration
		a.lastActivity = time.Now()
		a.mu.Unlock()
		return result, err
	}

	// Convert output to map if possible
	outputMap, ok := output.(map[string]interface{})
	if !ok {
		outputMap = map[string]interface{}{
			"result": output,
		}
	}

	// Determine confidence based on whether LLM was used
	confidence := 0.7 // Default confidence for basic execution
	if a.llmProvider != nil {
		confidence = 0.85 // Higher confidence with LLM assistance
	}

	result.SetSuccess(outputMap, confidence)

	// Update statistics
	a.mu.Lock()
	a.tasksSucceeded++
	a.tasksProcessed++
	a.totalDuration += result.Duration
	a.lastActivity = time.Now()
	a.mu.Unlock()

	return result, nil
}

// Collaborate implements the Agent interface Collaborate method
// This allows the agent to work with other agents on a task
func (a *BaseAgent) Collaborate(ctx context.Context, agents []Agent, t *task.Task) (*CollaborationResult, error) {
	startTime := time.Now()

	collaborationResult := &CollaborationResult{
		Success:      true,
		Results:      make(map[string]*task.Result),
		Participants: []string{a.ID()},
		Messages:     []*CollaborationMessage{},
	}

	// First, execute our own task
	myResult, err := a.Execute(ctx, t)
	if err != nil {
		collaborationResult.Success = false
	}
	collaborationResult.Results[a.ID()] = myResult

	// Identify agents that can help based on task requirements
	for _, other := range agents {
		if other.ID() == a.ID() {
			continue // Skip self
		}

		// Check if the other agent can contribute
		shouldCollaborate, collaborationType := a.shouldCollaborateWith(other, t)
		if !shouldCollaborate {
			continue
		}

		collaborationResult.Participants = append(collaborationResult.Participants, other.ID())

		// Create appropriate sub-task based on collaboration type
		subTask := a.createCollaborationTask(t, collaborationType, myResult)
		if subTask == nil {
			continue
		}

		// Send collaboration message
		msg := &CollaborationMessage{
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			From:      a.ID(),
			To:        other.ID(),
			Type:      MessageTypeRequest,
			Content:   fmt.Sprintf("Requesting %s collaboration on task: %s", collaborationType, t.Title),
			Timestamp: time.Now(),
		}
		collaborationResult.Messages = append(collaborationResult.Messages, msg)

		// Execute sub-task with the other agent
		otherResult, err := other.Execute(ctx, subTask)
		if err != nil {
			// Record the failure but continue with other agents
			responseMsg := &CollaborationMessage{
				ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
				From:      other.ID(),
				To:        a.ID(),
				Type:      MessageTypeResponse,
				Content:   fmt.Sprintf("Collaboration failed: %s", err.Error()),
				Timestamp: time.Now(),
			}
			collaborationResult.Messages = append(collaborationResult.Messages, responseMsg)
			continue
		}

		collaborationResult.Results[other.ID()] = otherResult

		// Record successful response
		responseMsg := &CollaborationMessage{
			ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			From:      other.ID(),
			To:        a.ID(),
			Type:      MessageTypeResponse,
			Content:   "Collaboration completed successfully",
			Timestamp: time.Now(),
		}
		collaborationResult.Messages = append(collaborationResult.Messages, responseMsg)
	}

	// Use our result as the consensus (could be enhanced with voting)
	collaborationResult.Consensus = myResult
	collaborationResult.Duration = time.Since(startTime)

	return collaborationResult, nil
}

// shouldCollaborateWith determines if we should collaborate with another agent
func (a *BaseAgent) shouldCollaborateWith(other Agent, t *task.Task) (bool, string) {
	otherType := other.Type()

	switch a.agentType {
	case AgentTypeCoding:
		// Coding agents benefit from review and testing
		if otherType == AgentTypeReview {
			return true, "review"
		}
		if otherType == AgentTypeTesting {
			return true, "testing"
		}

	case AgentTypePlanning:
		// Planning agents can consult other planning agents
		if otherType == AgentTypePlanning {
			return true, "consensus"
		}

	case AgentTypeDebugging:
		// Debugging agents benefit from testing to verify fixes
		if otherType == AgentTypeTesting {
			return true, "verification"
		}

	case AgentTypeReview:
		// Review agents can request refactoring for critical issues
		if otherType == AgentTypeRefactoring {
			return true, "refactoring"
		}
	}

	return false, ""
}

// createCollaborationTask creates a sub-task for collaboration
func (a *BaseAgent) createCollaborationTask(originalTask *task.Task, collaborationType string, myResult *task.Result) *task.Task {
	switch collaborationType {
	case "review":
		// Create a code review task
		code, _ := myResult.Output["code"].(string)
		if code == "" {
			return nil
		}
		subTask := task.NewTask(
			task.TaskTypeReview,
			"Code Review",
			"Review the generated code",
			task.PriorityNormal,
		)
		subTask.Input = map[string]interface{}{
			"code":      code,
			"file_path": myResult.Output["file_path"],
		}
		return subTask

	case "testing":
		// Create a testing task
		code, _ := myResult.Output["code"].(string)
		if code == "" {
			return nil
		}
		subTask := task.NewTask(
			task.TaskTypeTesting,
			"Generate Tests",
			"Generate tests for the code",
			task.PriorityNormal,
		)
		subTask.Input = map[string]interface{}{
			"code":      code,
			"file_path": myResult.Output["file_path"],
		}
		return subTask

	case "verification":
		// Create a test execution task
		subTask := task.NewTask(
			task.TaskTypeTesting,
			"Verify Fix",
			"Verify that the fix works",
			task.PriorityHigh,
		)
		subTask.Input = map[string]interface{}{
			"file_path":     myResult.Output["file_path"],
			"execute_tests": true,
		}
		return subTask

	case "refactoring":
		// Create a refactoring task for review issues
		issues, _ := myResult.Output["issues"].([]map[string]interface{})
		if len(issues) == 0 {
			return nil
		}
		subTask := task.NewTask(
			task.TaskTypeRefactoring,
			"Address Review Issues",
			"Refactor code to address review findings",
			task.PriorityHigh,
		)
		subTask.Input = map[string]interface{}{
			"file_path": myResult.Output["file_path"],
			"issues":    issues,
		}
		return subTask

	case "consensus":
		// For consensus, just pass the same task
		return originalTask

	default:
		return nil
	}
}

// Initialize implements the Agent interface Initialize method
func (a *BaseAgent) Initialize(ctx context.Context, cfg *AgentConfig) error {
	if cfg != nil {
		// Update agent configuration
		if cfg.ID != "" {
			a.id = cfg.ID
		}
		if cfg.Name != "" {
			a.name = cfg.Name
		}
		if cfg.Type != "" {
			a.agentType = cfg.Type
		}
		if len(cfg.Capabilities) > 0 {
			a.capabilities = make([]Capability, len(cfg.Capabilities))
			copy(a.capabilities, cfg.Capabilities)
		}
	}

	a.SetStatus(StatusIdle)
	return nil
}

// Shutdown implements the Agent interface Shutdown method
func (a *BaseAgent) Shutdown(ctx context.Context) error {
	a.SetStatus(StatusShutdown)
	a.Stop()
	return nil
}

// Start starts the agent
func (a *BaseAgent) Start(ctx context.Context) error {
	a.wg.Add(1)
	go a.processTasks(ctx)
	return nil
}

// Stop stops the agent
func (a *BaseAgent) Stop() {
	close(a.stopChan)
	a.wg.Wait()
}

// processTasks processes tasks from the queue
func (a *BaseAgent) processTasks(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopChan:
			return
		case task := <-a.taskQueue:
			a.processTask(ctx, task)
		}
	}
}

// processTask processes a single task
func (a *BaseAgent) processTask(ctx context.Context, task *Task) {
	result, err := a.SubmitTask(ctx, task)

	select {
	case a.resultChan <- result:
	default:
		// Channel is full, log error
		log.Printf("Agent %s: result channel full, dropping result for task %s", a.id, task.ID)
	}

	if err != nil {
		log.Printf("Agent %s: error processing task %s: %v", a.id, task.ID, err)
	}
}

// Health returns the agent health status
func (a *BaseAgent) Health() *HealthCheck {
	a.mu.RLock()
	defer a.mu.RUnlock()

	errorRate := float64(0)
	if a.tasksProcessed > 0 {
		errorRate = float64(a.tasksFailed) / float64(a.tasksProcessed)
	}

	// Determine if healthy based on status and error rate
	healthy := a.status != StatusError && a.status != StatusShutdown && errorRate < 0.5

	return &HealthCheck{
		AgentID:    a.id,
		Healthy:    healthy,
		Status:     a.status,
		Uptime:     time.Since(a.startTime),
		TaskCount:  a.tasksProcessed,
		ErrorCount: a.tasksFailed,
		ErrorRate:  errorRate,
		Timestamp:  time.Now(),
	}
}

// HealthMap returns the agent health status as a map (for backward compatibility)
func (a *BaseAgent) HealthMap() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalTasks := a.tasksProcessed
	successRate := float64(0)
	if totalTasks > 0 {
		successRate = float64(a.tasksSucceeded) / float64(totalTasks) * 100
	}

	avgDuration := time.Duration(0)
	if a.tasksProcessed > 0 {
		avgDuration = a.totalDuration / time.Duration(a.tasksProcessed)
	}

	return map[string]interface{}{
		"id":              a.id,
		"name":            a.name,
		"status":          string(a.status),
		"capabilities":    len(a.capabilities),
		"tasks_processed": a.tasksProcessed,
		"tasks_succeeded": a.tasksSucceeded,
		"tasks_failed":    a.tasksFailed,
		"success_rate":    successRate,
		"avg_duration":    avgDuration.String(),
		"last_activity":   a.lastActivity.Format(time.RFC3339),
		"queue_size":      len(a.taskQueue),
	}
}

// GetStatistics returns agent statistics
func (a *BaseAgent) GetStatistics() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"tasks_processed": a.tasksProcessed,
		"tasks_succeeded": a.tasksSucceeded,
		"tasks_failed":    a.tasksFailed,
		"total_duration":  a.totalDuration.String(),
		"average_duration": func() string {
			if a.tasksProcessed == 0 {
				return "0s"
			}
			return (a.totalDuration / time.Duration(a.tasksProcessed)).String()
		}(),
		"success_rate": func() float64 {
			if a.tasksProcessed == 0 {
				return 0.0
			}
			return float64(a.tasksSucceeded) / float64(a.tasksProcessed) * 100
		}(),
		"last_activity": a.lastActivity.Format(time.RFC3339),
	}
}

// ResetStatistics resets the agent statistics
func (a *BaseAgent) ResetStatistics() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tasksProcessed = 0
	a.tasksSucceeded = 0
	a.tasksFailed = 0
	a.totalDuration = 0
}

// IsHealthy checks if the agent is healthy
func (a *BaseAgent) IsHealthy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Agent is healthy if it's not in error status and has been active recently
	if a.status == StatusError {
		return false
	}

	// Consider agent unhealthy if no activity for more than 5 minutes
	if time.Since(a.lastActivity) > 5*time.Minute {
		return false
	}

	return true
}

// UpdateLastActivity updates the last activity timestamp
func (a *BaseAgent) UpdateLastActivity() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = time.Now()
}

// GetTaskQueueSize returns the current task queue size
func (a *BaseAgent) GetTaskQueueSize() int {
	return len(a.taskQueue)
}

// GetMaxConcurrency returns the maximum concurrency
func (a *BaseAgent) GetMaxConcurrency() int {
	return a.maxConcurrency
}

// SetMaxConcurrency sets the maximum concurrency
func (a *BaseAgent) SetMaxConcurrency(concurrency int) {
	if concurrency > 0 {
		a.maxConcurrency = concurrency
	}
}

// GetTimeout returns the task timeout
func (a *BaseAgent) GetTimeout() time.Duration {
	return a.timeout
}

// SetTimeout sets the task timeout
func (a *BaseAgent) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		a.timeout = timeout
	}
}

// GetRetryCount returns the retry count
func (a *BaseAgent) GetRetryCount() int {
	return a.retryCount
}

// SetRetryCount sets the retry count
func (a *BaseAgent) SetRetryCount(count int) {
	if count >= 0 {
		a.retryCount = count
	}
}

// IncrementTaskCount increments the task counter
func (a *BaseAgent) IncrementTaskCount() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tasksProcessed++
	a.lastActivity = time.Now()
}

// IncrementErrorCount increments the error counter
func (a *BaseAgent) IncrementErrorCount() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tasksFailed++
	a.lastActivity = time.Now()
}

// NewBaseAgentFromConfig creates a new base agent from an AgentConfig (backward compatible constructor)
func NewBaseAgentFromConfig(agentConfig *AgentConfig) *BaseAgent {
	if agentConfig == nil {
		return nil
	}

	now := time.Now()
	agent := &BaseAgent{
		id:             agentConfig.ID,
		name:           agentConfig.Name,
		agentType:      agentConfig.Type,
		status:         StatusIdle,
		capabilities:   make([]Capability, len(agentConfig.Capabilities)),
		taskQueue:      make(chan *Task, 100),
		resultChan:     make(chan *TaskResult, 100),
		stopChan:       make(chan struct{}),
		maxConcurrency: 1,
		timeout:        30 * time.Second,
		retryCount:     3,
		lastActivity:   now,
		startTime:      now,
	}

	// Copy capabilities
	copy(agent.capabilities, agentConfig.Capabilities)

	return agent
}
