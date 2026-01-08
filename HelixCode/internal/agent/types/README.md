# Agent Types Package

The `types` package provides specialized agent implementations for HelixCode's multi-agent system. Each agent type is designed for specific development tasks and can collaborate with other agents to complete complex workflows.

## Overview

The agent types package includes five specialized agents:
- **CodingAgent**: Code generation and modification
- **PlanningAgent**: Requirements analysis and task planning
- **TestingAgent**: Test generation and execution
- **ReviewAgent**: Code review and quality analysis
- **DebuggingAgent**: Error analysis and bug fixing

All agents implement the common `agent.Agent` interface and can collaborate through the coordination framework.

## Key Types and Interfaces

### Common Agent Interface

All agent types implement this interface (from `internal/agent`):

```go
type Agent interface {
    ID() string
    Name() string
    Type() AgentType
    Status() AgentStatus

    Initialize(ctx context.Context, cfg *AgentConfig) error
    Execute(ctx context.Context, task *task.Task) (*task.Result, error)
    Collaborate(ctx context.Context, agents []Agent, task *task.Task) (*CollaborationResult, error)
    Shutdown(ctx context.Context) error
}
```

### Agent Types

```go
const (
    AgentTypePlanning   AgentType = "planning"
    AgentTypeCoding     AgentType = "coding"
    AgentTypeTesting    AgentType = "testing"
    AgentTypeReview     AgentType = "review"
    AgentTypeDebugging  AgentType = "debugging"
    AgentTypeRefactoring AgentType = "refactoring"
)
```

## CodingAgent

Specialized in code generation and modification using LLM providers.

### Structure

```go
type CodingAgent struct {
    *agent.BaseAgent
    llmProvider  llm.Provider
    toolRegistry *tools.ToolRegistry
}
```

### Creating a CodingAgent

```go
codingAgent, err := types.NewCodingAgent(
    &config.AgentConfig{
        Model:       "gpt-4",
        Temperature: 0.2,
    },
    llmProvider,
    toolRegistry,
)
if err != nil {
    log.Fatalf("Failed to create coding agent: %v", err)
}
```

### Usage

```go
// Initialize the agent
err := codingAgent.Initialize(ctx, agentConfig)

// Create a coding task
codeTask := task.NewTask(
    task.TaskTypeCodeGeneration,
    "Create User Service",
    "Implement CRUD operations for user management",
    task.PriorityNormal,
)
codeTask.Input = map[string]interface{}{
    "requirements": "REST API with user CRUD operations using Gin framework",
    "file_path":    "/src/services/user_service.go",
}

// Execute the task
result, err := codingAgent.Execute(ctx, codeTask)
if err != nil {
    log.Printf("Code generation failed: %v", err)
}

// Access results
code := result.Output["code"].(string)
explanation := result.Output["explanation"].(string)
```

### Task Input Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `requirements` | string | Yes | Code requirements/specifications |
| `file_path` | string | No | Target file path for generated code |
| `existing_code` | string | No | Existing code to modify (for edit operations) |

### Task Output

| Field | Type | Description |
|-------|------|-------------|
| `operation` | string | "create" or "edit" |
| `file_path` | string | Path to generated/modified file |
| `code` | string | Generated code content |
| `explanation` | string | Description of implementation/changes |
| `artifacts` | []Artifact | Generated file artifacts |

## PlanningAgent

Specialized in analyzing requirements and creating detailed task plans.

### Structure

```go
type PlanningAgent struct {
    *agent.BaseAgent
    llmProvider llm.Provider
}
```

### Creating a PlanningAgent

```go
planningAgent, err := types.NewPlanningAgent(
    &config.AgentConfig{
        Model:       "gpt-4",
        Temperature: 0.3,
    },
    llmProvider,
)
```

### Usage

```go
// Create a planning task
planTask := task.NewTask(
    task.TaskTypePlanning,
    "Plan Feature Implementation",
    "Create implementation plan for user authentication",
    task.PriorityHigh,
)
planTask.Input = map[string]interface{}{
    "requirements": `
        Implement user authentication with:
        - JWT tokens
        - Refresh token rotation
        - OAuth2 social login
        - Rate limiting
    `,
}

// Execute planning
result, err := planningAgent.Execute(ctx, planTask)

// Access the plan
plan := result.Output["plan"].(string)
subtasks := result.Output["subtasks"].([]*task.Task)
estimatedDuration := result.Output["estimated_duration"].(time.Duration)

// Subtasks are ready for execution by other agents
for _, subtask := range subtasks {
    fmt.Printf("Subtask: %s (Type: %s, Duration: %v)\n",
        subtask.Title, subtask.Type, subtask.EstimatedDuration)
}
```

### Task Output

| Field | Type | Description |
|-------|------|-------------|
| `plan` | string | Detailed technical plan |
| `subtasks` | []*task.Task | Structured list of subtasks |
| `total_tasks` | int | Number of generated subtasks |
| `estimated_duration` | time.Duration | Total estimated time |

## TestingAgent

Specialized in generating and executing tests.

### Structure

```go
type TestingAgent struct {
    *agent.BaseAgent
    llmProvider  llm.Provider
    toolRegistry *tools.ToolRegistry
}
```

### Creating a TestingAgent

```go
testingAgent, err := types.NewTestingAgent(
    &config.AgentConfig{
        Model:       "gpt-4",
        Temperature: 0.3,
    },
    llmProvider,
    toolRegistry,
)
```

### Usage

```go
// Create a testing task
testTask := task.NewTask(
    task.TaskTypeTesting,
    "Generate Unit Tests",
    "Create comprehensive unit tests for user service",
    task.PriorityNormal,
)
testTask.Input = map[string]interface{}{
    "code":           userServiceCode,
    "file_path":      "/src/services/user_service.go",
    "test_framework": "testing", // Go testing framework
    "execute_tests":  true,      // Run tests after generation
}

// Execute test generation
result, err := testingAgent.Execute(ctx, testTask)

// Access results
testCode := result.Output["test_code"].(string)
testCases := result.Output["test_cases"].([]string)
testResults := result.Output["test_results"].(map[string]interface{})

fmt.Printf("Generated %d test cases\n", len(testCases))
for _, tc := range testCases {
    fmt.Printf("  - %s\n", tc)
}
```

### Task Input Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `code` | string | Yes | Code to generate tests for |
| `file_path` | string | No | Source file path |
| `test_framework` | string | No | Testing framework (default: "testing") |
| `execute_tests` | bool | No | Whether to run generated tests |

### Task Output

| Field | Type | Description |
|-------|------|-------------|
| `test_file` | string | Path to generated test file |
| `test_code` | string | Generated test code |
| `test_cases` | []string | List of test case names |
| `test_results` | map | Test execution results (if executed) |
| `artifacts` | []Artifact | Generated test file artifacts |

## ReviewAgent

Specialized in code review and quality analysis.

### Structure

```go
type ReviewAgent struct {
    *agent.BaseAgent
    llmProvider  llm.Provider
    toolRegistry *tools.ToolRegistry
}
```

### Creating a ReviewAgent

```go
reviewAgent, err := types.NewReviewAgent(
    &config.AgentConfig{
        Model:       "gpt-4",
        Temperature: 0.3,
    },
    llmProvider,
    toolRegistry,
)
```

### Usage

```go
// Create a review task
reviewTask := task.NewTask(
    task.TaskTypeReview,
    "Code Review",
    "Review user service implementation",
    task.PriorityNormal,
)
reviewTask.Input = map[string]interface{}{
    "code":                 userServiceCode,
    "file_path":            "/src/services/user_service.go",
    "review_type":          "comprehensive", // or "security", "performance"
    "run_static_analysis":  true,
}

// Execute review
result, err := reviewAgent.Execute(ctx, reviewTask)

// Access review results
summary := result.Output["review_result"].(string)
issues := result.Output["issues"].([]map[string]interface{})
suggestions := result.Output["suggestions"].([]string)
metrics := result.Output["metrics"].(map[string]interface{})

// Process issues
for _, issue := range issues {
    fmt.Printf("[%s] %s: %s\n",
        issue["severity"], issue["type"], issue["description"])
    fmt.Printf("  Recommendation: %s\n", issue["recommendation"])
}
```

### Review Types

| Type | Focus Areas |
|------|-------------|
| `comprehensive` | Quality, maintainability, best practices, security, performance |
| `security` | SQL injection, XSS, auth issues, data exposure, crypto misuse |
| `performance` | Complexity, memory leaks, inefficient loops, caching |

### Task Output

| Field | Type | Description |
|-------|------|-------------|
| `review_type` | string | Type of review performed |
| `review_result` | string | Overall assessment summary |
| `issues` | []map | List of identified issues |
| `suggestions` | []string | Improvement suggestions |
| `metrics` | map | Quality scores (0-100) |
| `static_analysis_results` | map | Results from static analysis tools |

## DebuggingAgent

Specialized in analyzing and fixing errors.

### Structure

```go
type DebuggingAgent struct {
    *agent.BaseAgent
    llmProvider  llm.Provider
    toolRegistry *tools.ToolRegistry
}
```

### Creating a DebuggingAgent

```go
debugAgent, err := types.NewDebuggingAgent(
    &config.AgentConfig{
        Model:       "gpt-4",
        Temperature: 0.2,
    },
    llmProvider,
    toolRegistry,
)
```

### Usage

```go
// Create a debugging task
debugTask := task.NewTask(
    task.TaskTypeDebugging,
    "Fix Null Pointer Exception",
    "Analyze and fix NPE in user service",
    task.PriorityCritical,
)
debugTask.Input = map[string]interface{}{
    "error":           "panic: runtime error: invalid memory address or nil pointer dereference",
    "stack_trace":     stackTrace,
    "file_path":       "/src/services/user_service.go",
    "code_context":    relevantCode, // Optional, will be read from file if not provided
    "run_diagnostics": true,
    "auto_fix":        true,
}

// Execute debugging
result, err := debugAgent.Execute(ctx, debugTask)

// Access results
analysis := result.Output["analysis"].(string)
rootCause := result.Output["root_cause"].(string)
fixes := result.Output["suggested_fixes"].([]string)
fixResults := result.Output["fix_results"].(map[string]interface{})

fmt.Printf("Root Cause: %s\n", rootCause)
fmt.Printf("Suggested Fixes:\n")
for i, fix := range fixes {
    fmt.Printf("  %d. %s\n", i+1, fix)
}

if fixResults["status"] == "success" {
    fmt.Printf("Fix applied to: %s\n", fixResults["file_path"])
}
```

### Task Input Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `error` | string | Yes | Error message |
| `stack_trace` | string | No | Stack trace |
| `file_path` | string | No | File where error occurred |
| `code_context` | string | No | Code context around error |
| `run_diagnostics` | bool | No | Run diagnostic commands |
| `auto_fix` | bool | No | Automatically apply first suggested fix |

### Task Output

| Field | Type | Description |
|-------|------|-------------|
| `error_message` | string | Original error |
| `analysis` | string | Detailed error analysis |
| `root_cause` | string | Identified root cause |
| `suggested_fixes` | []string | Ordered list of fix suggestions |
| `diagnostic_results` | map | Results from diagnostic commands |
| `fix_results` | map | Results of auto-fix (if enabled) |

## Agent Collaboration

Agents can collaborate to complete complex tasks. Each agent's `Collaborate` method enables coordination with other agents.

### Example: Coding + Review Collaboration

```go
// CodingAgent automatically requests review after code generation
result, err := codingAgent.Collaborate(ctx, []agent.Agent{reviewAgent}, codeTask)

// Access results from all participants
for agentID, agentResult := range result.Results {
    fmt.Printf("Agent %s: Success=%v\n", agentID, agentResult.Success)
}

// Get final consensus result
consensus := result.Consensus
```

### Example: Debugging + Testing Collaboration

```go
// DebuggingAgent requests test verification after applying fix
result, err := debugAgent.Collaborate(ctx, []agent.Agent{testingAgent}, debugTask)

// Check if fix was verified by tests
testResult := result.Results[testingAgent.ID()]
if testResult.Output["test_results"].(map[string]interface{})["status"] == "passed" {
    fmt.Println("Fix verified by tests!")
}
```

### Collaboration Messages

```go
// Access collaboration messages
for _, msg := range result.Messages {
    fmt.Printf("[%s] %s -> %s: %s\n",
        msg.Timestamp.Format(time.RFC3339),
        msg.From, msg.To, msg.Content)
}
```

## Test Helpers

The package includes utilities for testing agent implementations:

### MockTool

```go
// Create a mock tool for testing
mockFSRead := types.NewMockTool("FSRead", func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    return "mock file content", nil
})
```

### MockToolRegistry

```go
// Create a mock tool registry
registry := types.CreateMockToolRegistry(
    // FSRead function
    func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        return "file content", nil
    },
    // FSWrite function
    func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        return nil, nil
    },
    // Shell function
    func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        return "command output", nil
    },
)

// Convert to tools.ToolRegistry for agent creation
toolRegistry := types.ConvertToToolRegistry(registry)
```

## Utility Functions

### countLines

Counts lines in generated code:

```go
lines := types.countLines(generatedCode)
```

### getTestFilePath

Converts source file path to test file path:

```go
testPath := getTestFilePath("/src/user_service.go")
// Returns: "/src/user_service_test.go"
```

## Best Practices

1. **LLM Temperature Settings**:
   - Code generation: 0.1-0.3 (more deterministic)
   - Planning: 0.3-0.5 (some creativity)
   - Review: 0.2-0.3 (consistent analysis)

2. **Error Handling**: Always check both the error return and `result.Success` flag

3. **Resource Management**: Call `agent.Shutdown(ctx)` when done

4. **Task Input Validation**: Agents validate required input parameters before execution

5. **Confidence Scores**: Use result confidence to determine if human review is needed

6. **Collaboration**: Use collaboration for complex tasks that benefit from multiple perspectives

## Agent Status Lifecycle

```
Creating -> Idle -> Busy -> Idle -> ... -> Shutdown
                     |
                     v
                   Error -> Idle (retry) or Shutdown
```

Status values:
- `StatusIdle`: Ready to accept tasks
- `StatusBusy`: Currently executing a task
- `StatusShutdown`: Agent is shut down
