# Workflow Package

The `workflow` package provides workflow execution engine with step dependencies for the HelixCode platform.

## Overview

This package handles:
- Pre-built development workflows (planning, building, testing, refactoring)
- Step dependencies (DAG execution)
- Step types and actions
- LLM-powered analysis and code generation
- Execution monitoring and metrics

## Key Types

### Workflow

```go
type Workflow struct {
    ID          string         `json:"id"`
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Mode        string         `json:"mode"`
    Steps       []Step         `json:"steps"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    Status      WorkflowStatus `json:"status"`
}
```

### Step

```go
type Step struct {
    ID           string     `json:"id"`
    Name         string     `json:"name"`
    Description  string     `json:"description"`
    Type         StepType   `json:"type"`
    Action       StepAction `json:"action"`
    Dependencies []string   `json:"dependencies"`
    Status       StepStatus `json:"status"`
    Error        string     `json:"error,omitempty"`
}
```

### StepType

```go
type StepType string

const (
    StepTypeAnalysis   StepType = "analysis"
    StepTypeGeneration StepType = "generation"
    StepTypeExecution  StepType = "execution"
    StepTypeValidation StepType = "validation"
)
```

### StepAction

```go
type StepAction string

const (
    StepActionAnalyzeCode    StepAction = "analyze_code"
    StepActionGenerateCode   StepAction = "generate_code"
    StepActionExecuteCommand StepAction = "execute_command"
    StepActionRunTests       StepAction = "run_tests"
    StepActionLintCode       StepAction = "lint_code"
    StepActionBuildProject   StepAction = "build_project"
)
```

## Usage

### Creating an Executor

```go
import "dev.helix.code/internal/workflow"

// Basic executor (no LLM)
executor := workflow.NewExecutor(projectManager)

// Executor with LLM support
config := &workflow.ExecutorConfig{
    MaxConcurrentSteps: 4,
    StepTimeout:        10 * time.Minute,
    EnableLLM:          true,
    EnableMetrics:      true,
}
executor := workflow.NewExecutorWithLLM(projectManager, llmProvider, config)
```

### Pre-built Workflows

The executor provides pre-built workflows for common development tasks:

```go
// Planning workflow - generates architecture and design
wf, err := executor.ExecutePlanningWorkflow(ctx, projectID)

// Building workflow - compiles and builds the project
wf, err := executor.ExecuteBuildingWorkflow(ctx, projectID)

// Testing workflow - runs unit and integration tests
wf, err := executor.ExecuteTestingWorkflow(ctx, projectID)

// Refactoring workflow - analyzes and improves code quality
wf, err := executor.ExecuteRefactoringWorkflow(ctx, projectID)
```

### Monitoring Execution

```go
// Get a specific workflow by ID
wf, exists := executor.GetWorkflow(workflowID)
if exists {
    fmt.Printf("Workflow status: %s\n", wf.Status)
}

// Get all active workflows
activeWorkflows := executor.GetActiveWorkflows()
for _, w := range activeWorkflows {
    fmt.Printf("Active: %s - %s\n", w.ID, w.Name)
}

// Cancel a running workflow
err := executor.CancelWorkflow(workflowID)

// Get execution metrics
metrics := executor.GetMetrics()
fmt.Printf("LLM calls: %d, Tokens used: %d\n",
    metrics.LLMCalls, metrics.LLMTokensUsed)
```

### LLM Integration

When an LLM provider is configured, the executor uses it for:
- Code analysis with contextual insights
- Intelligent code generation
- Architecture recommendations

Without LLM, the executor falls back to:
- Static analysis (file structure, dependencies)
- Scaffold template generation (see below)

```go
// Enable/disable LLM at runtime
executor.SetLLMProvider(llmProvider)
```

### Scaffold Templates (No LLM Mode)

When LLM is not configured, the code generation step produces **scaffold templates** - production-ready starting points with best practices built in:

| Language | Features Included |
|----------|-------------------|
| Go | Context cancellation, signal handling, structured logging |
| Node.js | Async/await pattern, graceful shutdown, error handling |
| Python | Logging module, signal handlers, main guard |
| Rust | Error propagation, ctrlc handling, Result types |

Each scaffold includes:
- Task description embedded in comments
- Signal handling for graceful shutdown (SIGINT/SIGTERM)
- Proper error handling and exit codes
- Clear guidance to enable LLM for AI-powered generation

To enable AI-powered code generation, configure an LLM provider:

```yaml
llm:
  providers:
    openai:
      type: openai
      enabled: true
      parameters:
        api_key: "${OPENAI_API_KEY}"
```

## Step Dependencies

Steps form a DAG (Directed Acyclic Graph):

```
         +-------------+
         |   analyze   |
         +------+------+
                |
         +------v------+
         |   generate  |
         +------+------+
                |
    +-----------+-----------+
    |                       |
+---v---+             +-----v-----+
| test  |             |   lint    |
+---+---+             +-----+-----+
    |                       |
    +-----------+-----------+
                |
         +------v------+
         |   build     |
         +-------------+
```

Steps with satisfied dependencies execute in order. Failed steps halt dependent steps.

## Workflow Status

| Status | Description |
|--------|-------------|
| `pending` | Workflow not started |
| `running` | Workflow executing |
| `completed` | All steps completed |
| `failed` | Step failed |

## Step Status

| Status | Description |
|--------|-------------|
| `pending` | Step not started |
| `running` | Step executing |
| `completed` | Step finished successfully |
| `failed` | Step encountered error |
| `skipped` | Step skipped (dependencies not met) |

## Configuration

```go
type ExecutorConfig struct {
    MaxConcurrentSteps int           // Default: 4
    StepTimeout        time.Duration // Default: 10 minutes
    EnableLLM          bool          // Default: true
    EnableMetrics      bool          // Default: true
}
```

## Testing

```bash
go test -v ./internal/workflow/...
```

## Notes

- Workflows execute asynchronously (returns immediately after starting)
- Steps execute sequentially respecting dependencies
- LLM analysis provides deeper insights when available
- Template code generation is a fallback when LLM is unavailable
- Metrics track workflow and LLM usage statistics
