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
    ID                uuid.UUID              `json:"id"`
    Type              TaskType               `json:"type"`
    Data              map[string]interface{} `json:"data"`
    Status            TaskStatus             `json:"status"`
    Priority          TaskPriority           `json:"priority"`
    Criticality       TaskCriticality        `json:"criticality"`
    AssignedWorker    *uuid.UUID             `json:"assigned_worker"`
    OriginalWorker    *uuid.UUID             `json:"original_worker"`
    Dependencies      []uuid.UUID            `json:"dependencies"`
    RetryCount        int                    `json:"retry_count"`
    MaxRetries        int                    `json:"max_retries"`
    ErrorMessage      string                 `json:"error_message"`
    ResultData        map[string]interface{} `json:"result_data"`
    CheckpointData    map[string]interface{} `json:"checkpoint_data"`
    EstimatedDuration time.Duration          `json:"estimated_duration"`
    StartedAt         *time.Time             `json:"started_at"`
    CompletedAt       *time.Time             `json:"completed_at"`
    CreatedAt         time.Time              `json:"created_at"`
    UpdatedAt         time.Time              `json:"updated_at"`
}
```

### TaskType

```go
type TaskType string

const (
    TaskTypePlanning    TaskType = "planning"
    TaskTypeBuilding    TaskType = "building"
    TaskTypeTesting     TaskType = "testing"
    TaskTypeRefactoring TaskType = "refactoring"
    TaskTypeDebugging   TaskType = "debugging"
    TaskTypeDesign      TaskType = "design"
    TaskTypeDiagram     TaskType = "diagram"
    TaskTypeDeployment  TaskType = "deployment"
    TaskTypePorting     TaskType = "porting"
)
```

### TaskPriority

```go
type TaskPriority int

const (
    PriorityLow      TaskPriority = 1
    PriorityNormal   TaskPriority = 5
    PriorityHigh     TaskPriority = 10
    PriorityCritical TaskPriority = 20
)
```

### TaskStatus

```go
type TaskStatus string

const (
    TaskStatusPending          TaskStatus = "pending"
    TaskStatusAssigned         TaskStatus = "assigned"
    TaskStatusRunning          TaskStatus = "running"
    TaskStatusCompleted        TaskStatus = "completed"
    TaskStatusFailed           TaskStatus = "failed"
    TaskStatusPaused           TaskStatus = "paused"
    TaskStatusWaitingForWorker TaskStatus = "waiting_for_worker"
    TaskStatusWaitingForDeps   TaskStatus = "waiting_for_deps"
)
```

### Checkpoint

```go
type Checkpoint struct {
    ID             uuid.UUID              `json:"id"`
    CheckpointName string                 `json:"checkpoint_name"`
    CheckpointData map[string]interface{} `json:"checkpoint_data"`
    WorkerID       uuid.UUID              `json:"worker_id"`
    CreatedAt      time.Time              `json:"created_at"`
}
```

## Usage

### Creating Tasks

```go
import "dev.helix.code/internal/task"

manager := task.NewTaskManager(db, redisClient)

newTask, err := manager.CreateTask(
    task.TaskTypeBuilding,
    map[string]interface{}{
        "project_id": projectID,
        "target":     "build",
    },
    task.PriorityHigh,
    task.CriticalityNormal,
    nil, // no dependencies
)
```

### Task Dependencies

```go
// Create task with dependencies
testTask, err := manager.CreateTask(
    task.TaskTypeTesting,
    map[string]interface{}{"test_suite": "unit"},
    task.PriorityNormal,
    task.CriticalityNormal,
    []uuid.UUID{buildTaskID}, // Run after build completes
)
```

### Checkpointing

```go
// Manual checkpoint
err := manager.CreateCheckpoint(taskID, "step-1-completed", map[string]interface{}{
    "progress": 50,
    "status":   "processing",
})

// Get latest checkpoint
checkpoint, err := manager.checkpointMgr.GetLatestCheckpoint(taskID)

// Get all checkpoints
checkpoints, err := manager.checkpointMgr.GetCheckpoints(taskID)
```

### Queue Operations

```go
// Get next task from queue
task := manager.queue.GetNextTask()

// Get task by ID with caching
task, err := manager.GetTaskWithCache(ctx, taskID)
```

### Task Lifecycle

```go
// Assign task to worker
err := manager.AssignTask(taskID, workerID)

// Complete task
err := manager.CompleteTask(taskID, resultData)

// Fail task (auto-retries if retries remain)
err := manager.FailTask(taskID, "error message")
```

### Retry Logic

```go
// Tasks are created with MaxRetries = 3 by default
// Failed tasks are automatically retried if RetryCount < MaxRetries
```

## Configuration

```yaml
tasks:
  max_retries: 3
  queue_size: 1000
  cleanup_interval: 1h
```

## Checkpoint System

Checkpoints preserve work state for recovery:

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
              WaitingForDeps       Paused
```

## Testing

```bash
go test -v ./internal/task/...
```

## Notes

- Tasks with dependencies wait for dependencies to complete
- Use appropriate priority levels to prevent starvation
- Monitor queue depth for capacity planning
- Worker ID is tracked in checkpoints for audit purposes
