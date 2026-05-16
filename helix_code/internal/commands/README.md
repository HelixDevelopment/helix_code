# Commands Package

The `commands` package provides a comprehensive slash command system for the HelixCode platform, enabling users to invoke special actions through a command-line style interface. It supports command registration, parsing, execution, aliases, autocompletion, and follow-up actions.

## Overview

This package implements a complete slash command framework that handles:

- **Command Registration**: Central registry for managing commands and aliases
- **Command Parsing**: Robust parsing of command name, arguments, and flags
- **Command Execution**: Context-aware execution with result handling
- **Autocompletion**: Command name suggestions for partial input
- **Built-in Commands**: Standard commands for common operations
- **Action System**: Follow-up actions returned by commands for further processing

## Key Components

### Command Interface

The contract that all commands must implement:

```go
type Command interface {
    // Name returns the command name (without /)
    Name() string

    // Aliases returns alternative names for the command
    Aliases() []string

    // Description returns a short description
    Description() string

    // Usage returns usage information
    Usage() string

    // Execute runs the command
    Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error)
}
```

### Registry

Central storage for registered commands and their aliases:

```go
type Registry struct {
    commands map[string]Command
    aliases  map[string]string
    mutex    sync.RWMutex
}
```

### Parser

Parses user input into command components:

```go
type Parser struct {
    commandRegex *regexp.Regexp
}
```

### Executor

Orchestrates command parsing and execution:

```go
type Executor struct {
    registry *Registry
    parser   *Parser
}
```

## Key Types

### CommandContext

Provides execution context for commands:

```go
type CommandContext struct {
    UserID      string                 `json:"user_id"`
    SessionID   string                 `json:"session_id"`
    ProjectID   string                 `json:"project_id"`
    Args        []string               `json:"args"`
    Flags       map[string]string      `json:"flags"`
    RawInput    string                 `json:"raw_input"`
    ChatHistory []ChatMessage          `json:"chat_history"`
    WorkingDir  string                 `json:"working_dir"`
    Metadata    map[string]interface{} `json:"metadata"`
}
```

### CommandResult

Result returned by command execution:

```go
type CommandResult struct {
    Success     bool                   `json:"success"`
    Message     string                 `json:"message"`
    Data        interface{}            `json:"data,omitempty"`
    Actions     []Action               `json:"actions,omitempty"`
    ShouldReply bool                   `json:"should_reply"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
```

### Action

Follow-up action to be processed after command execution:

```go
type Action struct {
    Type string                 `json:"type"`
    Data map[string]interface{} `json:"data"`
}
```

### ChatMessage

Represents a message in chat history:

```go
type ChatMessage struct {
    Role      string    `json:"role"`    // user, assistant, system
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}
```

### CommandError

Structured error for command failures:

```go
type CommandError struct {
    Command string
    Message string
    Err     error
}

func (e *CommandError) Error() string {
    if e.Err != nil {
        return e.Command + ": " + e.Message + ": " + e.Err.Error()
    }
    return e.Command + ": " + e.Message
}
```

## Usage Examples

### Setting Up the Command System

```go
import "dev.helix.code/internal/commands"

// Create registry and executor
registry := commands.NewRegistry()
executor := commands.NewExecutor(registry)

// Register built-in commands
builtin.RegisterBuiltinCommands(registry)

// Register custom commands
registry.Register(&MyCustomCommand{})
```

### Creating a Custom Command

```go
type MyCommand struct{}

func (c *MyCommand) Name() string {
    return "mycommand"
}

func (c *MyCommand) Aliases() []string {
    return []string{"mc", "my"}
}

func (c *MyCommand) Description() string {
    return "Does something useful"
}

func (c *MyCommand) Usage() string {
    return `/mycommand <argument> [--flag=value]

Description of what the command does.

Examples:
  /mycommand hello
  /mycommand "multi word arg" --verbose
  /mycommand --dry-run

Flags:
  --verbose: Enable verbose output
  --dry-run: Preview without executing`
}

func (c *MyCommand) Execute(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
    // Access arguments
    if len(cmdCtx.Args) == 0 {
        return &commands.CommandResult{
            Success: false,
            Message: "Argument required",
        }, nil
    }
    arg := cmdCtx.Args[0]

    // Access flags
    verbose := cmdCtx.Flags["verbose"] == "true"
    dryRun := cmdCtx.Flags["dry-run"] == "true"

    // Access context
    userID := cmdCtx.UserID
    projectID := cmdCtx.ProjectID
    workingDir := cmdCtx.WorkingDir

    // Perform command logic
    result := doSomething(arg, verbose, dryRun)

    // Return result with optional actions
    return &commands.CommandResult{
        Success:     true,
        Message:     fmt.Sprintf("Processed: %s", arg),
        Data:        result,
        ShouldReply: true,
        Actions: []commands.Action{
            {
                Type: "update_context",
                Data: map[string]interface{}{
                    "last_command": c.Name(),
                },
            },
        },
        Metadata: map[string]interface{}{
            "verbose":  verbose,
            "dry_run":  dryRun,
            "user_id":  userID,
        },
    }, nil
}
```

### Executing Commands

```go
// Create command context
cmdCtx := &commands.CommandContext{
    UserID:     "user-123",
    SessionID:  "session-456",
    ProjectID:  "project-789",
    WorkingDir: "/path/to/project",
    Metadata:   map[string]interface{}{},
}

// Execute a command
result, err := executor.Execute(ctx, "/mycommand hello --verbose", cmdCtx)
if err != nil {
    log.Printf("Command error: %v", err)
    return
}

if result.Success {
    fmt.Printf("Result: %s\n", result.Message)

    // Process actions
    for _, action := range result.Actions {
        processAction(action)
    }
}

// Execute with default context
result, err = executor.ExecuteWithDefault(ctx, "/help")

// Check if input is a valid command
if executor.CanExecute("/mycommand arg") {
    // Input is a valid command
}
```

### Using the Parser Directly

```go
parser := commands.NewParser()

// Parse command input
name, args, flags, isCommand := parser.Parse("/deploy prod --force --target=k8s")
// name: "deploy"
// args: ["prod"]
// flags: {"force": "true", "target": "k8s"}
// isCommand: true

// Check if input is a command
if parser.IsCommand("/help") {
    fmt.Println("This is a command")
}

// Extract just the command name
name := parser.ExtractCommandName("/build --verbose")
// name: "build"
```

### Registry Operations

```go
// Register a command
err := registry.Register(&MyCommand{})
if err != nil {
    log.Fatal(err)
}

// Unregister a command
registry.Unregister("mycommand")

// Get a command by name or alias
cmd, exists := registry.Get("mycommand")
cmd, exists = registry.Get("mc")  // Using alias

// List all commands
commands := registry.List()
for _, cmd := range commands {
    fmt.Printf("/%s - %s\n", cmd.Name(), cmd.Description())
}

// List command names
names := registry.ListNames()

// Get help for a command
help := registry.GetHelp("mycommand")
fmt.Println(help)

// Get help for all commands
allHelp := registry.GetAllHelp()
fmt.Println(allHelp)

// Get command count
count := registry.Count()
```

### Autocompletion

```go
// Get command suggestions for partial input
matches := executor.Autocomplete("/dep")
// Returns: ["/deploy", "/dependencies", ...]

// Empty partial returns all commands
matches = executor.Autocomplete("/")
```

### Context Validation

```go
// Validate required context fields
err := executor.ValidateContext(cmdCtx, []string{"user_id", "project_id", "working_dir"})
if err != nil {
    return fmt.Errorf("missing required context: %w", err)
}
```

## Built-in Commands

The `builtin` subpackage provides standard commands:

### /newtask

Create a new task with preserved context:

```
/newtask <description>
/newtask "Implement authentication" --priority high
/newtask --link-previous Fix the bug discussed above
/newtask "Refactor" --transfer-files

Flags:
  --link-previous: Link to current task
  --priority: Set priority (low, normal, high, critical)
  --transfer-files: Transfer file references to new task
```

### /condense

Summarize and condense conversation history:

```
/condense
/condense --keep-last 5
/condense --preserve-code
/condense --ratio 0.3

Flags:
  --keep-last N: Keep last N messages uncompressed
  --preserve-code: Keep all code blocks intact
  --preserve-errors: Keep all error messages intact
  --ratio: Target compression ratio (default: 0.5)
```

### /deepplanning

Enter extended planning mode:

```
/deepplanning "new authentication system"
/deepplanning --depth 3 --output plan.md
/deepplanning microservices --include-diagrams
/deepplanning --resume plan-123

Planning Phases:
  1. Requirements Analysis
  2. Architecture Design
  3. Technology Selection
  4. Implementation Planning
  5. Risk Assessment
  6. Resource Estimation

Flags:
  --depth: Planning depth (1-5, default: 3)
  --output: Save plan to file
  --include-diagrams: Generate diagrams
  --resume: Resume previous session
  --focus: Focus areas (comma-separated)
  --constraints: Specify constraints
```

### /newrule

Create a new project rule:

```
/newrule <description>
/newrule "Always use error wrapping"
```

### /reportbug

Report a bug or issue:

```
/reportbug <description>
/reportbug "Crash when loading large files"
```

### /workflows

Manage workflows:

```
/workflows
/workflows list
/workflows run <name>
```

### Registering Built-in Commands

```go
import (
    "dev.helix.code/internal/commands"
    "dev.helix.code/internal/commands/builtin"
)

registry := commands.NewRegistry()
builtin.RegisterBuiltinCommands(registry)

// Get list of built-in command names
names := builtin.GetBuiltinCommandNames()

// Get built-in aliases
aliases := builtin.GetBuiltinCommandAliases()
```

## Command Parsing Syntax

The parser handles various command syntaxes:

### Basic Commands

```
/help
/status
```

### Commands with Arguments

```
/newtask "Implement feature"
/deploy production
/build main.go
```

### Commands with Flags

```
/build --verbose
/deploy --target=kubernetes
/run --timeout 30s
```

### Combined Syntax

```
/deploy production --force --dry-run
/newtask "Fix bug" --priority=high --link-previous
```

### Quote Handling

```
/task "Multi word description"
/search 'exact phrase'
/command "arg with spaces" --flag="value with spaces"
```

### Flag Formats

```
--flag=value       # Explicit value assignment
--flag value       # Space-separated value
--flag             # Boolean flag (value = "true")
```

## Action Types

Commands can return actions for follow-up processing:

```go
// Create a task
actions = append(actions, commands.Action{
    Type: "create_task",
    Data: map[string]interface{}{
        "description": "Task description",
        "priority":    "high",
    },
})

// Switch mode
actions = append(actions, commands.Action{
    Type: "switch_mode",
    Data: map[string]interface{}{
        "mode": "planning",
    },
})

// Update context
actions = append(actions, commands.Action{
    Type: "update_context",
    Data: map[string]interface{}{
        "key": "value",
    },
})

// Start deep planning
actions = append(actions, commands.Action{
    Type: "start_deep_planning",
    Data: map[string]interface{}{
        "topic": "System architecture",
        "depth": 3,
    },
})

// Condense history
actions = append(actions, commands.Action{
    Type: "condense_history",
    Data: map[string]interface{}{
        "keep_last": 5,
        "ratio":     0.5,
    },
})

// Link tasks
actions = append(actions, commands.Action{
    Type: "link_tasks",
    Data: map[string]interface{}{
        "type": "continuation",
    },
})
```

## Configuration Options

### YAML Configuration

```yaml
commands:
  history_size: 1000
  aliases:
    b: build
    t: test
    d: deploy
    s: status

  builtin:
    newtask:
      default_priority: normal
    condense:
      default_keep_last: 5
      default_ratio: 0.5
    deepplanning:
      default_depth: 3
```

## Best Practices

### Command Design

```go
// Keep command names short and memorable
func (c *MyCommand) Name() string {
    return "build"  // Good: short, clear
}

// Provide meaningful aliases
func (c *MyCommand) Aliases() []string {
    return []string{"b", "compile"}  // Short and descriptive
}

// Write comprehensive usage documentation
func (c *MyCommand) Usage() string {
    return `/build [target] [flags]

Builds the project for the specified target.

Examples:
  /build                  Build for current platform
  /build linux           Build for Linux
  /build --release       Build with optimizations

Flags:
  --release: Enable release optimizations
  --verbose: Show detailed output
  --target: Specify build target`
}
```

### Error Handling

```go
func (c *MyCommand) Execute(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
    // Validate required arguments
    if len(cmdCtx.Args) == 0 {
        return &commands.CommandResult{
            Success: false,
            Message: "Argument required. Usage: /mycommand <arg>",
        }, nil
    }

    // Validate context
    if cmdCtx.ProjectID == "" {
        return &commands.CommandResult{
            Success: false,
            Message: "No project selected",
        }, nil
    }

    // Return errors for system failures
    result, err := doOperation()
    if err != nil {
        return nil, &commands.CommandError{
            Command: c.Name(),
            Message: "operation failed",
            Err:     err,
        }
    }

    return &commands.CommandResult{
        Success: true,
        Message: "Operation completed",
        Data:    result,
    }, nil
}
```

### Context Usage

```go
func (c *MyCommand) Execute(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
    // Use context for timeouts and cancellation
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Access user and session info
    log.Printf("User %s executing command in session %s",
        cmdCtx.UserID, cmdCtx.SessionID)

    // Use project context
    if cmdCtx.ProjectID != "" {
        loadProject(cmdCtx.ProjectID)
    }

    // Use working directory
    if cmdCtx.WorkingDir != "" {
        os.Chdir(cmdCtx.WorkingDir)
    }

    // Access chat history for context
    for _, msg := range cmdCtx.ChatHistory {
        if msg.Role == "user" {
            extractContext(msg.Content)
        }
    }

    // Use metadata for custom data
    if taskID, ok := cmdCtx.Metadata["current_task_id"].(string); ok {
        linkToTask(taskID)
    }

    return &commands.CommandResult{
        Success: true,
        Message: "Done",
    }, nil
}
```

## Integration Patterns

### With Session Management

```go
func handleUserInput(session *Session, input string) {
    cmdCtx := &commands.CommandContext{
        UserID:      session.UserID,
        SessionID:   session.ID,
        ProjectID:   session.CurrentProject,
        ChatHistory: session.GetHistory(),
        WorkingDir:  session.WorkingDir,
        Metadata:    session.Metadata,
    }

    if executor.CanExecute(input) {
        result, err := executor.Execute(ctx, input, cmdCtx)
        if err != nil {
            session.SendError(err.Error())
            return
        }

        // Process actions
        for _, action := range result.Actions {
            session.ProcessAction(action)
        }

        if result.ShouldReply {
            session.SendMessage(result.Message)
        }
    } else {
        // Not a command, treat as chat
        session.HandleChat(input)
    }
}
```

### With Task Management

```go
func processActions(taskManager *TaskManager, actions []commands.Action) {
    for _, action := range actions {
        switch action.Type {
        case "create_task":
            taskManager.Create(action.Data)
        case "link_tasks":
            taskManager.Link(action.Data)
        case "switch_mode":
            taskManager.SetMode(action.Data["mode"].(string))
        }
    }
}
```

## Thread Safety

The `Registry` is safe for concurrent use:

- Uses `sync.RWMutex` for thread-safe operations
- Commands should be registered before processing starts
- Command execution is stateless and safe to call concurrently

```go
// Safe to call concurrently
go func() {
    cmd, _ := registry.Get("build")
    result, _ := cmd.Execute(ctx, cmdCtx)
}()
```

## Testing

```bash
# Run all command tests
go test -v ./internal/commands/...

# Run specific test
go test -v ./internal/commands -run TestParser

# Run builtin command tests
go test -v ./internal/commands/builtin/...

# Run with coverage
go test -cover ./internal/commands/...

# Run benchmarks
go test -bench=. ./internal/commands/...
```

### Testing Commands

```go
func TestMyCommand(t *testing.T) {
    registry := commands.NewRegistry()
    registry.Register(&MyCommand{})

    executor := commands.NewExecutor(registry)

    cmdCtx := &commands.CommandContext{
        UserID:     "test-user",
        ProjectID:  "test-project",
        WorkingDir: "/tmp/test",
    }

    result, err := executor.Execute(context.Background(), "/mycommand arg1 --flag", cmdCtx)

    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.Equal(t, "Expected message", result.Message)
}
```
