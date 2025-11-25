# Commands Package

The `commands` package provides CLI command handling for the HelixCode platform.

## Overview

This package handles:
- Command registration and parsing
- Interactive command mode
- Command history
- Tab completion
- Help system

## Key Types

### CommandRegistry

```go
type CommandRegistry struct {
    commands map[string]*Command
    aliases  map[string]string
}
```

### Command

```go
type Command struct {
    Name        string
    Description string
    Usage       string
    Flags       []*Flag
    Handler     CommandHandler
    SubCommands []*Command
}
```

## Usage

### Registering Commands

```go
import "dev.helix.code/internal/commands"

registry := commands.NewRegistry()

registry.Register(&commands.Command{
    Name:        "build",
    Description: "Build the project",
    Handler: func(ctx context.Context, args []string) error {
        return buildProject(ctx)
    },
})
```

### Executing Commands

```go
err := registry.Execute(ctx, "build", args)
```

## Built-in Commands

- `build` - Build project
- `test` - Run tests
- `deploy` - Deploy project
- `status` - Show status
- `help` - Show help

## Configuration

```yaml
commands:
  history_size: 1000
  aliases:
    b: build
    t: test
```

## Testing

```bash
go test -v ./internal/commands/...
```
