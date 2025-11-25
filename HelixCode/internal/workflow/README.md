# Workflow Package

The `workflow` package provides workflow execution engine with step dependencies for the HelixCode platform.

## Overview

This package handles:
- Workflow definition and execution
- Step dependencies (DAG execution)
- Step types and actions
- Workflow templates
- Execution monitoring

## Key Types

### Workflow

```go
type Workflow struct {
    ID          string
    Name        string
    Description string
    Steps       []*Step
    Status      Status
    CreatedAt   time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
}
```

### Step

```go
type Step struct {
    ID           string
    Type         StepType
    Action       Action
    Dependencies []string
    Config       map[string]interface{}
    Status       Status
    Output       interface{}
}
```

### StepType

```go
type StepType string

const (
    StepTypeAnalysis    StepType = "analysis"
    StepTypeGeneration  StepType = "generation"
    StepTypeExecution   StepType = "execution"
    StepTypeValidation  StepType = "validation"
    StepTypeDeployment  StepType = "deployment"
)
```

### Action

```go
type Action string

const (
    ActionAnalyzeCode     Action = "analyze_code"
    ActionGenerateCode    Action = "generate_code"
    ActionRunTests        Action = "run_tests"
    ActionBuild           Action = "build"
    ActionDeploy          Action = "deploy"
    ActionLint            Action = "lint"
    ActionFormat          Action = "format"
)
```

## Usage

### Creating a Workflow

```go
import "dev.helix.code/internal/workflow"

engine := workflow.NewEngine(config, taskManager, llmProvider)

wf := &workflow.Workflow{
    Name:        "Build and Test",
    Description: "Build project and run tests",
    Steps: []*workflow.Step{
        {
            ID:     "lint",
            Type:   workflow.StepTypeAnalysis,
            Action: workflow.ActionLint,
        },
        {
            ID:           "build",
            Type:         workflow.StepTypeExecution,
            Action:       workflow.ActionBuild,
            Dependencies: []string{"lint"},
        },
        {
            ID:           "test",
            Type:         workflow.StepTypeExecution,
            Action:       workflow.ActionRunTests,
            Dependencies: []string{"build"},
        },
    },
}

err := engine.CreateWorkflow(ctx, wf)
```

### Executing a Workflow

```go
// Start workflow execution
err := engine.ExecuteWorkflow(ctx, workflowID)

// Execute with parameters
params := map[string]interface{}{
    "target":  "production",
    "verbose": true,
}
err := engine.ExecuteWorkflowWithParams(ctx, workflowID, params)
```

### Monitoring Execution

```go
// Get workflow status
status, err := engine.GetWorkflowStatus(ctx, workflowID)

// Get step status
stepStatus, err := engine.GetStepStatus(ctx, workflowID, stepID)

// Subscribe to updates
updates := engine.Subscribe(workflowID)
for update := range updates {
    log.Info("Step %s: %s", update.StepID, update.Status)
}
```

### Built-in Workflows

```go
// Development workflow
devWorkflow := workflow.NewDevWorkflow()

// CI/CD workflow
ciWorkflow := workflow.NewCIWorkflow()

// Deployment workflow
deployWorkflow := workflow.NewDeployWorkflow()
```

## Step Dependencies

Steps form a DAG (Directed Acyclic Graph):

```
         ┌─────────┐
         │  lint   │
         └────┬────┘
              │
         ┌────▼────┐
         │  build  │
         └────┬────┘
              │
    ┌─────────┴─────────┐
    │                   │
┌───▼───┐         ┌─────▼─────┐
│ test  │         │  analyze  │
└───┬───┘         └─────┬─────┘
    │                   │
    └─────────┬─────────┘
              │
         ┌────▼────┐
         │ deploy  │
         └─────────┘
```

```go
steps := []*workflow.Step{
    {ID: "lint", Type: StepTypeAnalysis, Action: ActionLint},
    {ID: "build", Type: StepTypeExecution, Action: ActionBuild, Dependencies: []string{"lint"}},
    {ID: "test", Type: StepTypeExecution, Action: ActionRunTests, Dependencies: []string{"build"}},
    {ID: "analyze", Type: StepTypeAnalysis, Action: ActionAnalyzeCode, Dependencies: []string{"build"}},
    {ID: "deploy", Type: StepTypeDeployment, Action: ActionDeploy, Dependencies: []string{"test", "analyze"}},
}
```

## Configuration

```yaml
workflow:
  parallel_steps: 4
  step_timeout: 30m
  retry_failed: true
  max_retries: 2

  templates:
    build:
      steps:
        - lint
        - build
        - test

    deploy:
      steps:
        - lint
        - build
        - test
        - deploy
```

## Workflow Status

| Status | Description |
|--------|-------------|
| `pending` | Workflow not started |
| `running` | Workflow executing |
| `paused` | Workflow paused |
| `completed` | All steps completed |
| `failed` | Step failed |
| `cancelled` | Workflow cancelled |

## Error Handling

```go
// Retry failed step
err := engine.RetryStep(ctx, workflowID, stepID)

// Skip failed step
err := engine.SkipStep(ctx, workflowID, stepID)

// Cancel workflow
err := engine.CancelWorkflow(ctx, workflowID)
```

## Testing

```bash
go test -v ./internal/workflow/...
```

## Notes

- Steps execute in parallel when dependencies allow
- Failed steps can be retried or skipped
- Use step outputs as inputs for downstream steps
- Monitor execution for long-running workflows
