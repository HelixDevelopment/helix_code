# Event Package

The `event` package provides a comprehensive event-driven architecture implementation for the HelixCode platform, enabling loose coupling between components through a publish-subscribe pattern.

## Overview

The event package implements a thread-safe event bus that supports both synchronous and asynchronous event handling. It provides predefined event types for common platform operations (tasks, workflows, workers, users, and system events), severity levels for filtering and alerting, and comprehensive event metadata for tracking and auditing.

Key features include:
- Synchronous and asynchronous event processing
- Type-safe event categorization with predefined constants
- Severity levels for event prioritization
- Automatic event ID and timestamp generation
- Thread-safe concurrent access
- Error logging with automatic pruning
- Global singleton instance support
- Multiple handler subscription per event type

## Architecture

The package follows a simple but powerful architecture:

```
Publishers --> EventBus --> Subscribers
                  |
                  v
            Error Log (last 100 errors)
```

- **Publishers**: Any component that calls `Publish()` or `PublishAndWait()`
- **EventBus**: Central hub managing subscriptions and dispatching events
- **Subscribers**: Event handlers registered via `Subscribe()` or `SubscribeMultiple()`

## Key Types

### EventType

Predefined event type constants for categorizing events:

```go
type EventType string

const (
    // Task lifecycle events
    EventTaskCreated   EventType = "task.created"
    EventTaskAssigned  EventType = "task.assigned"
    EventTaskStarted   EventType = "task.started"
    EventTaskCompleted EventType = "task.completed"
    EventTaskFailed    EventType = "task.failed"
    EventTaskPaused    EventType = "task.paused"
    EventTaskResumed   EventType = "task.resumed"
    EventTaskCancelled EventType = "task.cancelled"

    // Workflow events
    EventWorkflowStarted   EventType = "workflow.started"
    EventWorkflowCompleted EventType = "workflow.completed"
    EventWorkflowFailed    EventType = "workflow.failed"
    EventStepCompleted     EventType = "step.completed"
    EventStepFailed        EventType = "step.failed"

    // Worker pool events
    EventWorkerConnected       EventType = "worker.connected"
    EventWorkerDisconnected    EventType = "worker.disconnected"
    EventWorkerHealthDegraded  EventType = "worker.health_degraded"
    EventWorkerHeartbeatMissed EventType = "worker.heartbeat_missed"
    EventWorkerTaskAssigned    EventType = "worker.task_assigned"
    EventWorkerTaskCompleted   EventType = "worker.task_completed"

    // User and API events
    EventUserRegistered EventType = "user.registered"
    EventUserLogin      EventType = "user.login"
    EventUserLogout     EventType = "user.logout"
    EventProjectCreated EventType = "project.created"
    EventProjectDeleted EventType = "project.deleted"
    EventAuthFailure    EventType = "auth.failure"

    // System events
    EventSystemStartup  EventType = "system.startup"
    EventSystemShutdown EventType = "system.shutdown"
    EventSystemError    EventType = "system.error"
)
```

### EventSeverity

Severity levels for prioritizing and filtering events:

```go
type EventSeverity string

const (
    SeverityInfo     EventSeverity = "info"     // Informational events
    SeverityWarning  EventSeverity = "warning"  // Warning conditions
    SeverityError    EventSeverity = "error"    // Error conditions
    SeverityCritical EventSeverity = "critical" // Critical issues
)
```

### Event

The core event structure containing all event metadata:

```go
type Event struct {
    ID        string                 `json:"id"`          // Auto-generated UUID
    Type      EventType              `json:"type"`        // Event category
    Timestamp time.Time              `json:"timestamp"`   // Auto-set if zero
    Source    string                 `json:"source"`      // Component identifier
    Severity  EventSeverity          `json:"severity"`    // Importance level
    Data      map[string]interface{} `json:"data"`        // Custom payload
    UserID    string                 `json:"user_id,omitempty"`
    ProjectID string                 `json:"project_id,omitempty"`
    TaskID    string                 `json:"task_id,omitempty"`
    WorkerID  string                 `json:"worker_id,omitempty"`
}
```

### EventHandler

Function signature for event handlers:

```go
type EventHandler func(ctx context.Context, event Event) error
```

### EventBus

The central event management component:

```go
type EventBus struct {
    subscribers map[EventType][]EventHandler
    mutex       sync.RWMutex
    async       bool
    errorLog    []error
    errorMutex  sync.Mutex
}
```

## Usage Examples

### Creating an Event Bus

```go
import "dev.helix.code/internal/event"

// Create async event bus (handlers run in goroutines)
asyncBus := event.NewEventBus(true)

// Create sync event bus (handlers run sequentially)
syncBus := event.NewEventBus(false)
```

### Subscribing to Events

```go
// Subscribe to a single event type
bus.Subscribe(event.EventTaskCompleted, func(ctx context.Context, e event.Event) error {
    log.Printf("Task %s completed by %s", e.TaskID, e.WorkerID)
    return nil
})

// Subscribe to multiple event types with one handler
bus.SubscribeMultiple([]event.EventType{
    event.EventTaskCreated,
    event.EventTaskStarted,
    event.EventTaskCompleted,
    event.EventTaskFailed,
}, func(ctx context.Context, e event.Event) error {
    log.Printf("Task event: %s for task %s", e.Type, e.TaskID)
    return nil
})

// Unsubscribe all handlers from an event type
bus.Unsubscribe(event.EventTaskCreated)
```

### Publishing Events

```go
// Create and publish an event
e := event.Event{
    Type:     event.EventTaskCompleted,
    Source:   "task-manager",
    Severity: event.SeverityInfo,
    TaskID:   "task-123",
    WorkerID: "worker-456",
    Data: map[string]interface{}{
        "duration":    "5m30s",
        "result":      "success",
        "output_size": 1024,
    },
}

// Async mode: returns immediately
err := bus.Publish(ctx, e)

// Async mode: wait for all handlers to complete
err := bus.PublishAndWait(ctx, e)

// Note: ID and Timestamp are auto-generated if not set
```

### Using the Global Event Bus

```go
// Get the global singleton (async by default)
globalBus := event.GetGlobalBus()

// Subscribe to events
globalBus.Subscribe(event.EventSystemError, func(ctx context.Context, e event.Event) error {
    alertOps(e.Data["error"])
    return nil
})

// Publish events
globalBus.Publish(ctx, event.Event{
    Type:     event.EventSystemError,
    Severity: event.SeverityCritical,
    Data:     map[string]interface{}{"error": "Database connection lost"},
})

// For testing: reset the global bus
event.ResetGlobalBus()

// For testing: set a custom global bus
event.SetGlobalBus(customBus)
```

### Error Handling

```go
// In sync mode, errors are collected and returned
err := bus.Publish(ctx, e)
if err != nil {
    log.Printf("Handler errors: %v", err)
}

// Get recent errors (up to last 100)
errors := bus.GetErrors()
for _, err := range errors {
    log.Printf("Event error: %v", err)
}

// Clear error log
bus.ClearErrors()
```

### Introspection

```go
// Get subscriber count for an event type
count := bus.GetSubscriberCount(event.EventTaskCompleted)

// Get total number of subscriptions
total := bus.GetTotalSubscribers()

// Get all event types with subscribers
types := bus.GetSubscribedEvents()
for _, t := range types {
    fmt.Printf("Event type: %s\n", t)
}

// Check if bus is in async mode
if bus.IsAsync() {
    fmt.Println("Running in async mode")
}
```

## Configuration Options

Configure event settings in `config/config.yaml`:

```yaml
event:
  # Size of the event queue for async processing
  queue_size: 1000

  # Number of worker goroutines for event processing
  workers: 4

  # Enable event persistence for replay
  persistence: true

  # Storage backend for persisted events
  storage: "redis"
```

## Best Practices

### Event Design

1. **Use Specific Event Types**: Prefer specific event types over generic ones for better filtering and handling.

2. **Include Relevant Context**: Always populate relevant ID fields (UserID, ProjectID, TaskID, WorkerID) for traceability.

3. **Keep Data Lightweight**: Store only essential information in the Data map; reference external storage for large payloads.

4. **Set Appropriate Severity**: Use severity levels consistently:
   - `Info`: Normal operations, status updates
   - `Warning`: Recoverable issues, deprecation notices
   - `Error`: Failed operations, exceptions
   - `Critical`: System-wide failures requiring immediate attention

### Handler Implementation

1. **Keep Handlers Fast**: Long-running handlers block other handlers in sync mode; use async mode or spawn goroutines for heavy work.

2. **Handle Errors Gracefully**: Return errors from handlers to enable proper error logging and monitoring.

3. **Use Context**: Respect context cancellation in handlers to enable graceful shutdown.

4. **Avoid Circular Events**: Be careful not to publish events that trigger handlers which publish the same events.

```go
// Good: Fast handler with async work
bus.Subscribe(event.EventTaskCompleted, func(ctx context.Context, e event.Event) error {
    // Quick synchronous work
    log.Printf("Task completed: %s", e.TaskID)

    // Heavy work in separate goroutine
    go func() {
        sendNotification(e)
        updateAnalytics(e)
    }()

    return nil
})
```

### Mode Selection

1. **Use Async Mode** for:
   - High-throughput scenarios
   - Non-critical event processing
   - When handler latency should not affect publishers

2. **Use Sync Mode** for:
   - Critical events requiring confirmation
   - When handler errors must be returned to publishers
   - Sequential processing requirements

## Integration Patterns

### With Task Manager

```go
func (tm *TaskManager) completeTask(task *Task) error {
    // Complete the task
    task.Status = StatusCompleted

    // Publish completion event
    event.GetGlobalBus().Publish(context.Background(), event.Event{
        Type:      event.EventTaskCompleted,
        Source:    "task-manager",
        Severity:  event.SeverityInfo,
        TaskID:    task.ID,
        ProjectID: task.ProjectID,
        Data: map[string]interface{}{
            "duration": task.Duration.String(),
            "output":   task.Output,
        },
    })

    return nil
}
```

### With Worker Pool

```go
func (wp *WorkerPool) onWorkerDisconnect(workerID string) {
    event.GetGlobalBus().Publish(context.Background(), event.Event{
        Type:     event.EventWorkerDisconnected,
        Source:   "worker-pool",
        Severity: event.SeverityWarning,
        WorkerID: workerID,
        Data: map[string]interface{}{
            "reason":     "heartbeat_timeout",
            "last_seen":  time.Now().Add(-30 * time.Second),
            "pool_size":  wp.Size() - 1,
        },
    })
}
```

### Monitoring and Alerting

```go
func setupAlerts(bus *event.EventBus) {
    // Alert on critical system errors
    bus.Subscribe(event.EventSystemError, func(ctx context.Context, e event.Event) error {
        if e.Severity == event.SeverityCritical {
            pagerduty.Alert(fmt.Sprintf("Critical: %v", e.Data["error"]))
        }
        return nil
    })

    // Track worker health
    bus.SubscribeMultiple([]event.EventType{
        event.EventWorkerHealthDegraded,
        event.EventWorkerHeartbeatMissed,
    }, func(ctx context.Context, e event.Event) error {
        metrics.Increment("worker_health_issues")
        return nil
    })
}
```

## Thread Safety

The EventBus is fully thread-safe:

- All operations use read-write mutex protection
- Concurrent `Subscribe()`, `Unsubscribe()`, and `Publish()` calls are safe
- Error logging uses a separate mutex to prevent contention
- Handler slices are copied during iteration to prevent modification during execution

## Performance Considerations

- **Async Mode**: Minimal publisher latency; handlers run in separate goroutines
- **Sync Mode**: Publisher waits for all handlers; better error reporting
- **Error Log**: Automatically trimmed to last 100 entries to prevent memory growth
- **Handler Copies**: Event types are copied during publish to allow concurrent modifications

## Testing

```bash
# Run all event package tests
go test -v ./internal/event/...

# Run benchmarks
go test -bench=. ./internal/event/...

# Run with race detector
go test -race ./internal/event/...
```

### Test Utilities

```go
// Create test bus
bus := event.NewEventBus(false) // sync for predictable testing

// Reset global bus between tests
event.ResetGlobalBus()

// Set custom global bus for testing
testBus := event.NewEventBus(false)
event.SetGlobalBus(testBus)
```

## Related Packages

- `internal/task`: Task management (publishes task events)
- `internal/worker`: Worker pool (publishes worker events)
- `internal/notification`: Notification delivery (subscribes to events)
- `internal/monitoring`: System monitoring (subscribes to all events)
