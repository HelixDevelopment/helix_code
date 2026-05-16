# Hooks Package

The `hooks` package provides a comprehensive lifecycle hooks and event handling system for the HelixCode platform, enabling extensible event-driven architecture with configurable priorities, async execution, and conditional triggering.

## Overview

The hooks package implements a flexible hook system that allows components to register callbacks that execute at specific points in the application lifecycle. It supports synchronous and asynchronous execution, priority-based ordering, conditional execution, timeouts, and comprehensive execution tracking.

Key features include:
- Multiple hook types for different lifecycle events (task, LLM, edit, build, test, etc.)
- Priority-based execution ordering (highest executes first)
- Synchronous and asynchronous hook execution
- Conditional execution based on event data
- Configurable timeouts per hook
- Execution result tracking with statistics
- Thread-safe concurrent operations
- Tag-based hook categorization
- Lifecycle callbacks for monitoring

## Architecture

The package follows a Manager-Executor-Hook pattern:

```
               +-------------+
               |   Manager   |
               |-------------|
               | - hooks     |
               | - executor  |
               | - callbacks |
               +-------------+
                     |
         +-----------+-----------+
         |                       |
    +---------+            +-----------+
    |  Hooks  |            |  Executor |
    |---------|            |-----------|
    | - type  |            | - results |
    | - func  |            | - stats   |
    | - prio  |            | - limits  |
    +---------+            +-----------+
```

- **Manager**: Central registry for hook registration, enabling/disabling, and triggering
- **Executor**: Handles hook execution with concurrency control and result tracking
- **Hook**: Individual hook with handler function, priority, and metadata

## Key Types

### HookType

Predefined hook types for different lifecycle events:

```go
type HookType string

const (
    HookTypeBeforeTask  HookType = "before_task"  // Before task execution
    HookTypeAfterTask   HookType = "after_task"   // After task execution
    HookTypeBeforeLLM   HookType = "before_llm"   // Before LLM API call
    HookTypeAfterLLM    HookType = "after_llm"    // After LLM API call
    HookTypeBeforeEdit  HookType = "before_edit"  // Before file edit
    HookTypeAfterEdit   HookType = "after_edit"   // After file edit
    HookTypeBeforeBuild HookType = "before_build" // Before build process
    HookTypeAfterBuild  HookType = "after_build"  // After build process
    HookTypeBeforeTest  HookType = "before_test"  // Before test run
    HookTypeAfterTest   HookType = "after_test"   // After test run
    HookTypeOnError     HookType = "on_error"     // On error occurrence
    HookTypeOnSuccess   HookType = "on_success"   // On success
    HookTypeCustom      HookType = "custom"       // Custom hook types
)
```

### HookPriority

Priority levels for execution ordering:

```go
type HookPriority int

const (
    PriorityLowest  HookPriority = 1    // Execute last
    PriorityLow     HookPriority = 25   // Low priority
    PriorityNormal  HookPriority = 50   // Default priority
    PriorityHigh    HookPriority = 75   // High priority
    PriorityHighest HookPriority = 100  // Execute first
)
```

Hooks are sorted by priority (highest first) before execution.

### HookStatus

Execution status tracking:

```go
type HookStatus string

const (
    StatusPending   HookStatus = "pending"   // Not yet executed
    StatusRunning   HookStatus = "running"   // Currently executing
    StatusCompleted HookStatus = "completed" // Successfully completed
    StatusFailed    HookStatus = "failed"    // Failed with error
    StatusCanceled  HookStatus = "canceled"  // Canceled before completion
    StatusSkipped   HookStatus = "skipped"   // Skipped due to conditions
)
```

### Hook

The core hook structure:

```go
type Hook struct {
    ID          string            // Unique identifier (auto-generated)
    Name        string            // Human-readable name
    Type        HookType          // Type of hook event
    Description string            // Hook description
    Handler     HookFunc          // Handler function
    Priority    HookPriority      // Execution priority (default: 50)
    Async       bool              // Execute asynchronously
    Timeout     time.Duration     // Execution timeout (0 = no timeout)
    Condition   func(*Event) bool // Optional condition check
    Tags        []string          // Tags for categorization
    Metadata    map[string]string // Custom metadata
    Enabled     bool              // Whether hook is enabled
    CreatedAt   time.Time         // When hook was created
}
```

### Event

Event structure that triggers hooks:

```go
type Event struct {
    Type      HookType               // Event type
    Data      map[string]interface{} // Event data payload
    Context   context.Context        // Context for execution
    Timestamp time.Time              // When event occurred
    Source    string                 // Source of the event
    Metadata  map[string]string      // Additional metadata
}
```

### ExecutionResult

Result of hook execution:

```go
type ExecutionResult struct {
    HookID      string        // ID of executed hook
    HookName    string        // Name of executed hook
    Status      HookStatus    // Execution status
    Error       error         // Error if failed
    Duration    time.Duration // Execution duration
    StartedAt   time.Time     // When execution started
    CompletedAt time.Time     // When execution completed
}
```

### Manager

Central hook management component:

```go
type Manager struct {
    hooks     map[HookType][]*Hook // Hooks organized by type
    hooksAll  map[string]*Hook     // All hooks by ID
    executor  *Executor            // Hook executor
    mu        sync.RWMutex         // Thread-safety
    onCreate  []HookCallback       // Callbacks on hook creation
    onRemove  []HookCallback       // Callbacks on hook removal
    onExecute []ExecuteCallback    // Callbacks on execution
}
```

### Executor

Hook execution engine:

```go
type Executor struct {
    maxConcurrent int                // Maximum concurrent async executions
    semaphore     chan struct{}      // Semaphore for concurrency control
    wg            sync.WaitGroup     // Wait group for async executions
    results       []*ExecutionResult // Results of recent executions
    maxResults    int                // Maximum results to keep (default: 100)
    onComplete    []ResultCallback   // Callbacks on completion
    onError       []ResultCallback   // Callbacks on error
}
```

## Usage Examples

### Creating and Registering Hooks

```go
import "dev.helix.code/internal/hooks"

// Create a manager
manager := hooks.NewManager()

// Create a simple hook
hook := hooks.NewHook("lint", hooks.HookTypeBeforeBuild, func(ctx context.Context, event *hooks.Event) error {
    return runLinter(ctx)
})

// Register the hook
err := manager.Register(hook)
if err != nil {
    log.Fatal(err)
}
```

### Creating Async Hooks

```go
// Create an async hook (executes in goroutine)
asyncHook := hooks.NewAsyncHook("notify", hooks.HookTypeAfterBuild, func(ctx context.Context, event *hooks.Event) error {
    return sendSlackNotification(event)
})

manager.Register(asyncHook)
```

### Creating Hooks with Priority

```go
// High priority hook (executes first)
validationHook := hooks.NewHookWithPriority(
    "validate",
    hooks.HookTypeBeforeBuild,
    validateConfig,
    hooks.PriorityHigh,
)

// Low priority hook (executes last)
cleanupHook := hooks.NewHookWithPriority(
    "cleanup",
    hooks.HookTypeAfterBuild,
    cleanupTempFiles,
    hooks.PriorityLow,
)

manager.Register(validationHook)
manager.Register(cleanupHook)
```

### Setting Hook Timeout

```go
hook := hooks.NewHook("slow-operation", hooks.HookTypeAfterTask, slowHandler)
hook.Timeout = 30 * time.Second // Will be canceled after 30 seconds

manager.Register(hook)
```

### Conditional Execution

```go
hook := hooks.NewHook("debug-only", hooks.HookTypeAfterBuild, debugHandler)

// Only execute in debug mode
hook.Condition = func(event *hooks.Event) bool {
    debug, ok := event.GetData("debug")
    return ok && debug == true
}

manager.Register(hook)
```

### Using Tags and Metadata

```go
hook := hooks.NewHook("deploy", hooks.HookTypeAfterBuild, deployHandler)

// Add tags for categorization
hook.AddTag("production")
hook.AddTag("deployment")

// Add metadata
hook.SetMetadata("environment", "staging")
hook.SetMetadata("version", "1.0.0")

manager.Register(hook)

// Find hooks by tag
productionHooks := manager.GetByTag("production")
```

### Triggering Hooks

```go
// Trigger by type (returns immediately for async hooks)
results := manager.Trigger(ctx, hooks.HookTypeBeforeBuild)

// Trigger and wait for all hooks (including async)
results := manager.TriggerAndWait(ctx, hooks.HookTypeBeforeBuild)

// Trigger synchronously (all hooks execute in sequence)
results := manager.TriggerSync(ctx, hooks.HookTypeBeforeBuild)
```

### Triggering with Event Data

```go
// Create event with data
event := hooks.NewEventWithContext(ctx, hooks.HookTypeBeforeBuild)
event.SetData("project", projectName)
event.SetData("commit", commitHash)
event.Source = "build-system"

// Trigger with event
results := manager.TriggerEvent(event)

// Or trigger and wait
results := manager.TriggerEventAndWait(event)
```

### Processing Execution Results

```go
results := manager.TriggerSync(ctx, hooks.HookTypeBeforeBuild)

for _, result := range results {
    fmt.Printf("Hook: %s\n", result.HookName)
    fmt.Printf("Status: %s\n", result.Status)
    fmt.Printf("Duration: %v\n", result.Duration)

    if result.Error != nil {
        fmt.Printf("Error: %v\n", result.Error)
    }
}
```

### Hook Lifecycle Callbacks

```go
// Called when a hook is registered
manager.OnCreate(func(hook *hooks.Hook) {
    log.Printf("Hook registered: %s (%s)", hook.Name, hook.Type)
})

// Called when a hook is removed
manager.OnRemove(func(hook *hooks.Hook) {
    log.Printf("Hook removed: %s", hook.Name)
})

// Called after hooks are executed
manager.OnExecute(func(event *hooks.Event, results []*hooks.ExecutionResult) {
    successful := 0
    for _, r := range results {
        if r.Status == hooks.StatusCompleted {
            successful++
        }
    }
    log.Printf("Executed %d/%d hooks for %s", successful, len(results), event.Type)
})
```

### Enabling and Disabling Hooks

```go
// Disable a specific hook
manager.Disable(hookID)

// Enable a specific hook
manager.Enable(hookID)

// Disable all hooks
manager.DisableAll()

// Enable all hooks
manager.EnableAll()

// Get only enabled hooks
enabled := manager.GetEnabled()
```

### Getting Statistics

```go
// Manager statistics
stats := manager.GetStatistics()
fmt.Printf("Total hooks: %d\n", stats.TotalHooks)
fmt.Printf("Enabled: %d\n", stats.EnabledHooks)
fmt.Printf("Disabled: %d\n", stats.DisabledHooks)

// Executor statistics
execStats := stats.ExecutorStats
fmt.Printf("Executions: %d\n", execStats.TotalExecutions)
fmt.Printf("Success rate: %.1f%%\n", execStats.SuccessRate*100)
fmt.Printf("Avg duration: %v\n", execStats.AverageDuration)
```

### Custom Executor Configuration

```go
// Create executor with concurrency limit
executor := hooks.NewExecutorWithLimit(5) // Max 5 concurrent async hooks
manager := hooks.NewManagerWithExecutor(executor)

// Configure result retention
executor.SetMaxResults(1000) // Keep last 1000 results

// Register error callbacks
executor.OnError(func(result *hooks.ExecutionResult) {
    alertTeam(result.HookName, result.Error)
})

// Get results by status
failed := executor.GetResultsByStatus(hooks.StatusFailed)
for _, f := range failed {
    log.Printf("Failed hook: %s - %v", f.HookName, f.Error)
}
```

## Configuration Options

Configure hooks settings in `config/config.yaml`:

```yaml
hooks:
  # Enable hooks system
  enabled: true

  # Default timeout for hooks without explicit timeout
  timeout: 5m

  # Continue executing remaining hooks if one fails
  continue_on_error: false

  # Maximum concurrent async hook executions
  max_concurrent: 10

  # Maximum number of results to keep in history
  max_results: 100

  # Pre-configured hooks from config
  hooks:
    pre_build:
      - name: lint
        command: "golangci-lint run"
        timeout: 2m
      - name: format-check
        command: "gofmt -l ."
    post_build:
      - name: notify
        command: "curl -X POST https://webhook.example.com"
        async: true
    on_error:
      - name: alert
        command: "send-alert.sh"
```

## Best Practices

### Hook Design

1. **Keep Hooks Focused**: Each hook should do one thing well. Split complex operations into multiple hooks.

2. **Use Appropriate Priorities**: Reserve high priorities for validation/security hooks; use low priorities for cleanup/notification.

3. **Handle Errors Gracefully**: Return errors from handlers to enable proper tracking and error callbacks.

4. **Respect Context**: Check context cancellation in long-running hooks to enable graceful shutdown.

```go
func longRunningHandler(ctx context.Context, event *hooks.Event) error {
    for i := 0; i < 100; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err() // Respect cancellation
        default:
            processItem(i)
        }
    }
    return nil
}
```

### Async vs Sync

1. **Use Async** for:
   - Non-critical operations (notifications, logging)
   - Long-running operations that shouldn't block
   - Independent operations that can run concurrently

2. **Use Sync** for:
   - Critical validation that must complete before proceeding
   - Operations with dependencies on previous hooks
   - Operations where order matters

### Timeout Management

1. **Set Reasonable Timeouts**: Always set timeouts for external operations.

2. **Consider Hook Type**: Build hooks may need longer timeouts than notification hooks.

3. **Handle Timeout Errors**: Check for context deadline exceeded errors.

```go
hook := hooks.NewHook("external-api", hooks.HookTypeAfterTask, apiHandler)
hook.Timeout = 10 * time.Second

// In handler
func apiHandler(ctx context.Context, event *hooks.Event) error {
    result, err := callExternalAPI(ctx)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return fmt.Errorf("API call timed out")
        }
        return err
    }
    return nil
}
```

### Conditional Execution

Use conditions to prevent unnecessary hook execution:

```go
// Only run in production
hook.Condition = func(event *hooks.Event) bool {
    env, _ := event.GetMetadata("environment")
    return env == "production"
}

// Only run for specific project types
hook.Condition = func(event *hooks.Event) bool {
    projectType, ok := event.GetData("project_type")
    return ok && projectType == "go"
}
```

## Integration Patterns

### With Task System

```go
func (tm *TaskManager) executeTask(task *Task) error {
    // Create event
    event := hooks.NewEventWithContext(task.Context, hooks.HookTypeBeforeTask)
    event.SetData("task_id", task.ID)
    event.SetData("task_type", task.Type)

    // Trigger pre-task hooks
    results := tm.hookManager.TriggerSync(task.Context, hooks.HookTypeBeforeTask)

    // Check for failures
    for _, r := range results {
        if r.Status == hooks.StatusFailed {
            return fmt.Errorf("pre-task hook failed: %s: %v", r.HookName, r.Error)
        }
    }

    // Execute task
    err := task.Execute()

    // Trigger post-task hooks
    if err != nil {
        tm.hookManager.Trigger(task.Context, hooks.HookTypeOnError)
    } else {
        tm.hookManager.Trigger(task.Context, hooks.HookTypeOnSuccess)
    }
    tm.hookManager.Trigger(task.Context, hooks.HookTypeAfterTask)

    return err
}
```

### With Build System

```go
func (bs *BuildSystem) build(project *Project) error {
    ctx := context.Background()

    // Pre-build hooks (sync - must complete before build)
    event := hooks.NewEventWithContext(ctx, hooks.HookTypeBeforeBuild)
    event.SetData("project", project.Name)
    event.SetData("version", project.Version)

    results := bs.hooks.TriggerSync(ctx, hooks.HookTypeBeforeBuild)
    if hasFailures(results) {
        return fmt.Errorf("pre-build validation failed")
    }

    // Perform build
    err := bs.performBuild(project)

    // Post-build hooks (can be async for notifications)
    postEvent := hooks.NewEventWithContext(ctx, hooks.HookTypeAfterBuild)
    postEvent.SetData("success", err == nil)
    postEvent.SetData("duration", time.Since(start))

    bs.hooks.TriggerEvent(postEvent) // Fire and forget

    return err
}
```

### With LLM Calls

```go
func (lm *LLMManager) generate(request *Request) (*Response, error) {
    ctx := request.Context

    // Before LLM hook - can modify request
    event := hooks.NewEventWithContext(ctx, hooks.HookTypeBeforeLLM)
    event.SetData("model", request.Model)
    event.SetData("prompt_tokens", countTokens(request.Prompt))

    lm.hooks.TriggerSync(ctx, hooks.HookTypeBeforeLLM)

    // Make LLM call
    response, err := lm.provider.Generate(ctx, request)

    // After LLM hook - can log/analyze response
    afterEvent := hooks.NewEventWithContext(ctx, hooks.HookTypeAfterLLM)
    afterEvent.SetData("response_tokens", countTokens(response.Content))
    afterEvent.SetData("latency", response.Latency)

    lm.hooks.Trigger(ctx, hooks.HookTypeAfterLLM)

    return response, err
}
```

## Thread Safety

The hooks package is fully thread-safe:

- **Manager**: Uses read-write mutex for all hook operations
- **Executor**: Uses semaphore for concurrency control, mutex for results
- **Hooks**: Hook slices are copied during iteration to allow concurrent modification

Concurrent operations supported:
- Registering/unregistering hooks while triggering
- Multiple simultaneous trigger calls
- Concurrent async hook execution

## Performance Considerations

- **Priority Sorting**: Hooks are sorted by priority before each execution
- **Async Execution**: Uses semaphore to limit concurrent goroutines
- **Result Retention**: Automatically trims results to configured limit
- **Handler Copies**: Hook lists are copied during trigger to prevent modification issues

For high-throughput scenarios:
- Use async hooks for non-critical operations
- Set appropriate concurrency limits
- Consider using conditions to skip unnecessary hooks

## Testing

```bash
# Run all hooks package tests
go test -v ./internal/hooks/...

# Run with coverage
go test -cover ./internal/hooks/...

# Run with race detector
go test -race ./internal/hooks/...
```

### Test Utilities

```go
func TestHookExecution(t *testing.T) {
    manager := hooks.NewManager()

    executed := false
    hook := hooks.NewHook("test", hooks.HookTypeBeforeTask, func(ctx context.Context, e *hooks.Event) error {
        executed = true
        return nil
    })

    manager.Register(hook)
    manager.TriggerSync(context.Background(), hooks.HookTypeBeforeTask)

    assert.True(t, executed)
}

func TestAsyncHook(t *testing.T) {
    manager := hooks.NewManager()

    hook := hooks.NewAsyncHook("async-test", hooks.HookTypeAfterTask, asyncHandler)
    manager.Register(hook)

    manager.Trigger(context.Background(), hooks.HookTypeAfterTask)
    manager.Wait() // Wait for async hooks to complete
}
```

## Related Packages

- `internal/task`: Task execution (triggers task hooks)
- `internal/workflow`: Workflow engine (triggers build/test hooks)
- `internal/llm`: LLM integration (triggers LLM hooks)
- `internal/editor`: File editing (triggers edit hooks)
- `internal/event`: Event bus (can be used alongside hooks for system-wide events)
