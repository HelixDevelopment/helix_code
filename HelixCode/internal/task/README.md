# Task Package

The `task` package provides task management with checkpointing and dependency tracking for the HelixCode platform.

## Overview

This package handles:
- Task creation and lifecycle management
- Priority-based scheduling
- Dependency resolution
- Checkpoint system for work preservation
- Automatic retry and rollback
- Task queue management

## Key Types

### Task

Represents a development task:

```go
type Task struct {
    ID           string
    Type         TaskType
    Priority     Priority
    Status       Status
    Dependencies []string
    Checkpoint   *Checkpoint
    CreatedAt    time.Time
    StartedAt    *time.Time
    CompletedAt  *time.Time
}
```

### TaskType

```go
type TaskType string

const (
    TypePlanning    TaskType = "planning"
    TypeBuilding    TaskType = "building"
    TypeTesting     TaskType = "testing"
    TypeRefactoring TaskType = "refactoring"
    TypeDebugging   TaskType = "debugging"
    TypeDeployment  TaskType = "deployment"
)
```

### Priority

```go
type Priority int

const (
    PriorityLow      Priority = 1
    PriorityNormal   Priority = 2
    PriorityHigh     Priority = 3
    PriorityCritical Priority = 4
)
```

### Status

```go
type Status string

const (
    StatusPending   Status = "pending"
    StatusAssigned  Status = "assigned"
    StatusRunning   Status = "running"
    StatusCompleted Status = "completed"
    StatusFailed    Status = "failed"
    StatusCancelled Status = "cancelled"
)
```

## Usage

### Creating Tasks

```go
import "dev.helix.code/internal/task"

manager := task.NewManager(db, config)

task := &task.Task{
    Type:     task.TypeBuilding,
    Priority: task.PriorityHigh,
    Payload: &task.BuildPayload{
        ProjectID: projectID,
        Target:    "build",
    },
}

err := manager.CreateTask(ctx, task)
```

### Task Dependencies

```go
// Create task with dependencies
task := &task.Task{
    Type:         task.TypeTesting,
    Dependencies: []string{buildTaskID}, // Run after build completes
}

manager.CreateTask(ctx, task)
```

### Checkpointing

```go
// Checkpoints are saved automatically at configured intervals
// Default: every 300 seconds (5 minutes)

// Manual checkpoint
err := manager.Checkpoint(ctx, taskID)

// Restore from checkpoint
err := manager.RestoreFromCheckpoint(ctx, taskID)
```

### Queue Operations

```go
// Get next task from queue
task, err := manager.DequeueTask(ctx)

// Get pending tasks by priority
tasks, err := manager.GetPendingTasks(ctx, task.PriorityHigh)

// Get task by ID
task, err := manager.GetTask(ctx, taskID)
```

### Task Lifecycle

```go
// Start task
err := manager.StartTask(ctx, taskID, workerID)

// Complete task
err := manager.CompleteTask(ctx, taskID, result)

// Fail task
err := manager.FailTask(ctx, taskID, err)

// Cancel task
err := manager.CancelTask(ctx, taskID)
```

### Retry Logic

```go
// Configure retry behavior
config := &task.ManagerConfig{
    MaxRetries:   3,
    RetryBackoff: 5 * time.Second,
}

// Failed tasks are automatically retried if retries remain
```

## Configuration

```yaml
tasks:
  checkpoint_interval: 300s
  max_retries: 3
  retry_backoff: 5s
  queue_size: 1000
  cleanup_interval: 1h
```

## Checkpoint System

Checkpoints preserve work state for recovery:

```go
type Checkpoint struct {
    TaskID      string
    State       []byte
    Progress    float64
    CreatedAt   time.Time
    Metadata    map[string]interface{}
}
```

Benefits:
- Resume long-running tasks after failures
- Preserve progress across system restarts
- Enable task migration between workers

## Task Flow

```
Created -> Pending -> Assigned -> Running -> Completed
                  |                    |
                  |                    v
                  |                 Failed -> Retry -> Running
                  |                    |
                  v                    v
               Cancelled           Cancelled
```

## Testing

```bash
go test -v ./internal/task/...
```

## Notes

- Tasks with dependencies wait for dependencies to complete
- Checkpoints are compressed for storage efficiency
- Use appropriate priority levels to prevent starvation
- Monitor queue depth for capacity planning
