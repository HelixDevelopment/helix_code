# Task Package

The `task` package provides the core data structures for representing units of work to be performed by agents in HelixCode. It defines tasks, their lifecycle, priorities, dependencies, and execution results.

## Overview

The task package is fundamental to the agent system, providing:
- Task definition with rich metadata
- Priority-based scheduling support
- Dependency tracking between tasks
- Task lifecycle management (pending, in-progress, completed, failed)
- Execution results with artifacts and metrics
- Support for task coordination and orchestration

## Key Types and Interfaces

### Task

The primary structure representing a unit of work:

```go
type Task struct {
    ID          string     // Unique identifier (UUID)
    Type        TaskType   // Category of work
    Title       string     // Human-readable title
    Description string     // Detailed description
    Priority    Priority   // Execution priority
    Status      TaskStatus // Current state

    // Requirements
    RequiredCapabilities []string      // Agent capabilities needed
    EstimatedDuration    time.Duration // Expected completion time
    Deadline             *time.Time    // Optional deadline

    // Dependencies
    DependsOn []string // Task IDs this depends on
    BlockedBy []string // Task IDs blocking this

    // Input/Output
    Input  map[string]interface{} // Task parameters
    Output map[string]interface{} // Execution results

    // Execution tracking
    AssignedTo  string        // Agent ID
    StartedAt   *time.Time    // Execution start
    CompletedAt *time.Time    // Execution end
    Duration    time.Duration // Actual duration

    // Metadata
    CreatedAt time.Time              // Creation timestamp
    UpdatedAt time.Time              // Last update
    CreatedBy string                 // Creator (agent ID or "user")
    Tags      []string               // Custom tags
    Metadata  map[string]interface{} // Additional data
}
```

### TaskType

Defines the category of work:

```go
const (
    TaskTypePlanning       TaskType = "planning"        // Requirements analysis
    TaskTypeAnalysis       TaskType = "analysis"        // Code/data analysis
    TaskTypeCodeGeneration TaskType = "code_generation" // New code creation
    TaskTypeCodeEdit       TaskType = "code_edit"       // Modify existing code
    TaskTypeRefactoring    TaskType = "refactoring"     // Code restructuring
    TaskTypeTesting        TaskType = "testing"         // Test generation/execution
    TaskTypeDebugging      TaskType = "debugging"       // Error analysis/fixing
    TaskTypeReview         TaskType = "review"          // Code review
    TaskTypeDocumentation  TaskType = "documentation"   // Documentation
    TaskTypeResearch       TaskType = "research"        // Information gathering
)
```

### Priority

Defines execution priority levels:

```go
const (
    PriorityLow      Priority = 1 // Background tasks
    PriorityNormal   Priority = 2 // Standard tasks
    PriorityHigh     Priority = 3 // Important tasks
    PriorityCritical Priority = 4 // Urgent tasks
)
```

### TaskStatus

Represents the current state in the task lifecycle:

```go
const (
    StatusPending    TaskStatus = "pending"     // Not yet ready
    StatusReady      TaskStatus = "ready"       // Ready to execute
    StatusAssigned   TaskStatus = "assigned"    // Assigned to agent
    StatusInProgress TaskStatus = "in_progress" // Currently executing
    StatusBlocked    TaskStatus = "blocked"     // Blocked by dependencies
    StatusCompleted  TaskStatus = "completed"   // Successfully finished
    StatusFailed     TaskStatus = "failed"      // Execution failed
    StatusCancelled  TaskStatus = "cancelled"   // Cancelled by user/system
)
```

### Result

Contains the outcome of task execution:

```go
type Result struct {
    TaskID     string                 // Associated task
    AgentID    string                 // Executing agent
    Success    bool                   // Execution success
    Output     map[string]interface{} // Result data
    Error      string                 // Error message if failed
    Duration   time.Duration          // Execution time
    Confidence float64                // Result confidence (0.0-1.0)
    Artifacts  []Artifact             // Generated artifacts
    Metrics    *TaskMetrics           // Execution metrics
    Timestamp  time.Time              // Completion time
}
```

### Artifact

Represents files or resources created by a task:

```go
type Artifact struct {
    ID        string    // Unique identifier
    Type      string    // "code", "test", "doc", "config"
    Path      string    // File path
    Content   string    // File content
    Size      int64     // Size in bytes
    Checksum  string    // Content hash
    CreatedAt time.Time // Creation time
}
```

### TaskMetrics

Contains metrics about task execution:

```go
type TaskMetrics struct {
    TokensUsed     int           // LLM tokens consumed
    LLMCalls       int           // Number of LLM calls
    ToolCalls      int           // Number of tool invocations
    FilesModified  int           // Files changed
    LinesAdded     int           // Lines of code added
    LinesRemoved   int           // Lines of code removed
    TestsGenerated int           // Tests created
    ExecutionTime  time.Duration // Total execution time
}
```

## Usage Examples

### Creating Tasks

```go
// Create a basic task
codeTask := task.NewTask(
    task.TaskTypeCodeGeneration,
    "Implement User Service",
    "Create a user service with CRUD operations",
    task.PriorityNormal,
)

// Add input parameters
codeTask.Input = map[string]interface{}{
    "requirements": "User service with Create, Read, Update, Delete operations",
    "language":     "go",
    "framework":    "gin",
}

// Set required capabilities
codeTask.RequiredCapabilities = []string{"code_generation", "go"}

// Set estimated duration
codeTask.EstimatedDuration = 30 * time.Minute

// Add tags for organization
codeTask.Tags = []string{"backend", "user-module", "sprint-1"}
```

### Setting Task Dependencies

```go
// Create dependent tasks
analysisTask := task.NewTask(task.TaskTypeAnalysis, "Analyze requirements", "...", task.PriorityHigh)
implementTask := task.NewTask(task.TaskTypeCodeGeneration, "Implement feature", "...", task.PriorityNormal)
testTask := task.NewTask(task.TaskTypeTesting, "Write tests", "...", task.PriorityNormal)

// Set dependencies
implementTask.DependsOn = []string{analysisTask.ID}
testTask.DependsOn = []string{implementTask.ID}
```

### Task Lifecycle Management

```go
// Check if task is ready to execute
completedTasks := map[string]bool{
    "dep-task-1": true,
    "dep-task-2": true,
}

if task.IsReady(completedTasks) {
    fmt.Println("Task is ready for execution")
}

// Start task execution
if task.CanStart() {
    task.Start("coding-agent-1")
    fmt.Printf("Task started by: %s\n", task.AssignedTo)
}

// Complete task with output
task.Complete(map[string]interface{}{
    "code":       generatedCode,
    "file_path":  "/path/to/file.go",
    "tests":      testCode,
})

// Handle task failure
task.Fail("LLM provider unavailable")

// Block task
task.Block("waiting for database migration", []string{"migration-task-id"})

// Unblock task
task.Unblock()

// Cancel task
task.Cancel("user requested cancellation")
```

### Checking Task State

```go
// Check various states
if task.IsCompleted() {
    fmt.Printf("Task completed in %v\n", task.Duration)
}

if task.IsFailed() {
    reason := task.Metadata["failure_reason"].(string)
    fmt.Printf("Task failed: %s\n", reason)
}

if task.IsActive() {
    fmt.Println("Task is currently being executed")
}
```

### Working with Results

```go
// Create a result
result := task.NewResult(myTask.ID, "coding-agent-1")

// Set success with output
result.SetSuccess(
    map[string]interface{}{
        "code": generatedCode,
        "explanation": "Implemented using repository pattern",
    },
    0.85, // 85% confidence
)

// Or set failure
result.SetFailure(fmt.Errorf("failed to generate code: timeout"))

// Add artifacts
result.AddArtifact(task.Artifact{
    ID:        "artifact-1",
    Type:      "code",
    Path:      "/src/user_service.go",
    Content:   generatedCode,
    Size:      int64(len(generatedCode)),
    CreatedAt: time.Now(),
})

// Set metrics
result.Metrics = &task.TaskMetrics{
    TokensUsed:    1500,
    LLMCalls:      3,
    ToolCalls:     5,
    FilesModified: 2,
    LinesAdded:    150,
    ExecutionTime: 45 * time.Second,
}
```

## Integration Patterns

### With Agent System

```go
// Agent executing a task
func (agent *CodingAgent) Execute(ctx context.Context, t *task.Task) (*task.Result, error) {
    result := task.NewResult(t.ID, agent.ID())

    // Extract requirements
    requirements, ok := t.Input["requirements"].(string)
    if !ok {
        err := fmt.Errorf("requirements not found")
        result.SetFailure(err)
        return result, err
    }

    // Generate code...
    code, err := agent.generateCode(ctx, requirements)
    if err != nil {
        result.SetFailure(err)
        return result, err
    }

    // Set success
    result.SetSuccess(map[string]interface{}{
        "code": code,
    }, 0.9)

    return result, nil
}
```

### With Task Queue

```go
// Priority-based task queue
type TaskQueue struct {
    tasks []*task.Task
    mu    sync.Mutex
}

func (q *TaskQueue) Push(t *task.Task) {
    q.mu.Lock()
    defer q.mu.Unlock()

    // Insert by priority (higher priority first)
    index := sort.Search(len(q.tasks), func(i int) bool {
        return q.tasks[i].Priority < t.Priority
    })

    q.tasks = append(q.tasks[:index], append([]*task.Task{t}, q.tasks[index:]...)...)
}

func (q *TaskQueue) Pop() *task.Task {
    q.mu.Lock()
    defer q.mu.Unlock()

    if len(q.tasks) == 0 {
        return nil
    }

    t := q.tasks[0]
    q.tasks = q.tasks[1:]
    return t
}
```

### With Dependency Resolution

```go
// Find tasks that are ready to execute
func findReadyTasks(tasks []*task.Task) []*task.Task {
    completedTasks := make(map[string]bool)

    // Build completed task map
    for _, t := range tasks {
        if t.IsCompleted() {
            completedTasks[t.ID] = true
        }
    }

    // Find ready tasks
    ready := make([]*task.Task, 0)
    for _, t := range tasks {
        if t.IsReady(completedTasks) {
            ready = append(ready, t)
        }
    }

    return ready
}
```

### With Workflow Steps

```go
// Convert workflow step to task
func stepToTask(step *WorkflowStep) *task.Task {
    t := task.NewTask(
        determineTaskType(step.Action),
        step.Name,
        step.Description,
        determinePriority(step.Critical),
    )

    t.Input = step.Parameters
    t.EstimatedDuration = step.Timeout
    t.Metadata["workflow_id"] = step.WorkflowID
    t.Metadata["step_number"] = step.Number

    return t
}
```

## Task Type Guidelines

| Task Type | Use Case | Typical Agent |
|-----------|----------|---------------|
| `planning` | Analyzing requirements, creating execution plans | PlanningAgent |
| `analysis` | Code analysis, data analysis, dependency analysis | AnalysisAgent |
| `code_generation` | Creating new code files, functions, classes | CodingAgent |
| `code_edit` | Modifying existing code | CodingAgent |
| `refactoring` | Restructuring code without changing behavior | RefactoringAgent |
| `testing` | Generating and executing tests | TestingAgent |
| `debugging` | Analyzing errors, fixing bugs | DebuggingAgent |
| `review` | Code review, quality assessment | ReviewAgent |
| `documentation` | Creating docs, comments, README files | DocumentationAgent |
| `research` | Gathering information, searching codebases | ResearchAgent |

## Best Practices

1. **Clear Descriptions**: Provide detailed descriptions for tasks to help agents understand requirements
2. **Appropriate Priorities**: Use priority levels consistently across the system
3. **Capability Matching**: Set `RequiredCapabilities` to ensure tasks are assigned to capable agents
4. **Dependency Management**: Keep dependency chains manageable to avoid blocking
5. **Metadata Usage**: Store additional context in `Metadata` for debugging and auditing
6. **Result Confidence**: Set realistic confidence values to indicate result reliability
7. **Artifact Tracking**: Record all generated files as artifacts for traceability

## Error Handling

Tasks support comprehensive error tracking:

```go
// On failure, store details in metadata
task.Fail("Network timeout while calling LLM")
// Metadata now contains:
// - failure_reason: "Network timeout while calling LLM"
// - failed_at: time.Time

// On block, store blocking info
task.Block("Waiting for database", []string{"db-migration-task"})
// Metadata now contains:
// - block_reason: "Waiting for database"
// BlockedBy is set to ["db-migration-task"]

// On cancellation, store reason
task.Cancel("User requested cancellation")
// Metadata now contains:
// - cancellation_reason: "User requested cancellation"
```

## Thread Safety Notes

The `Task` struct itself is not thread-safe. When using tasks in concurrent scenarios:

```go
// Use mutex for concurrent access
type SafeTask struct {
    task *task.Task
    mu   sync.RWMutex
}

func (st *SafeTask) GetStatus() task.TaskStatus {
    st.mu.RLock()
    defer st.mu.RUnlock()
    return st.task.Status
}

func (st *SafeTask) Start(agentID string) {
    st.mu.Lock()
    defer st.mu.Unlock()
    st.task.Start(agentID)
}
```
