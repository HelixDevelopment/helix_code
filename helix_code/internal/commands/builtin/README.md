# Builtin Commands

This package provides built-in slash commands for HelixCode.

## Overview

Builtin commands are pre-registered commands available to all users. They provide core functionality like conversation management, task creation, planning workflows, and code analysis.

## Available Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `/condense` | `/smol`, `/compact`, `/summarize` | Summarize and condense conversation history to save tokens |
| `/deepplanning` | - | Initiate deep planning mode for complex tasks |
| `/newrule` | - | Create a new rule in the rules system |
| `/newtask` | - | Create a new task for workers |

## Command Interface

All builtin commands implement the `commands.Command` interface:

```go
type Command interface {
    Name() string
    Aliases() []string
    Description() string
    Usage() string
    Execute(ctx context.Context, args []string) (*CommandResult, error)
}
```

## Usage Examples

### Condense Command

```
/condense                    # Use defaults
/condense --keep-last 5      # Keep last 5 messages uncompressed
/condense --preserve-code    # Keep all code blocks intact
/condense --preserve-errors  # Keep all error messages intact
/condense --ratio 0.3        # Target 30% compression
```

### Deep Planning Command

```
/deepplanning               # Start interactive planning session
/deepplanning analyze       # Analyze current task complexity
/deepplanning breakdown     # Break task into subtasks
```

### New Task Command

```
/newtask "Implement feature X"
/newtask --priority high --type building "Fix critical bug"
/newtask --assign worker-1 "Review pull request"
```

### New Rule Command

```
/newrule "Use tabs for indentation"
/newrule --scope project "Always write tests first"
```

## Registration

Builtin commands are registered with the command manager at startup:

```go
import "dev.helix.code/internal/commands/builtin"

mgr := commands.NewManager()
mgr.Register(builtin.NewCondenseCommand())
mgr.Register(builtin.NewDeepPlanningCommand())
mgr.Register(builtin.NewNewTaskCommand())
mgr.Register(builtin.NewNewRuleCommand())
```

## Adding New Commands

1. Create a new file (e.g., `mycommand.go`)
2. Implement the `commands.Command` interface
3. Register the command in the manager

```go
package builtin

type MyCommand struct{}

func NewMyCommand() *MyCommand {
    return &MyCommand{}
}

func (c *MyCommand) Name() string { return "mycommand" }
func (c *MyCommand) Aliases() []string { return []string{"mc", "my"} }
func (c *MyCommand) Description() string { return "Does something useful" }
func (c *MyCommand) Usage() string { return "/mycommand [options]" }

func (c *MyCommand) Execute(ctx context.Context, args []string) (*commands.CommandResult, error) {
    // Implementation
    return &commands.CommandResult{Success: true}, nil
}
```

## Testing

```bash
go test -v ./internal/commands/builtin/...
```

## See Also

- `internal/commands/` - Command management
- `internal/task/` - Task system
- `internal/rules/` - Rules system
- `internal/workflow/` - Workflow execution
