# Hooks Package

The `hooks` package provides lifecycle hooks and event handling for the HelixCode platform.

## Overview

This package handles:
- Pre/post operation hooks
- Event-driven callbacks
- Custom hook registration
- Hook execution order
- Error handling in hooks

## Key Types

### HookManager

```go
type HookManager struct {
    hooks    map[HookType][]*Hook
    executor *Executor
    config   *Config
}
```

### Hook

```go
type Hook struct {
    Name     string
    Type     HookType
    Handler  HookHandler
    Priority int
    Enabled  bool
}

type HookHandler func(ctx context.Context, event *Event) error
```

### HookType

```go
type HookType string

const (
    HookPreBuild    HookType = "pre_build"
    HookPostBuild   HookType = "post_build"
    HookPreTest     HookType = "pre_test"
    HookPostTest    HookType = "post_test"
    HookPreDeploy   HookType = "pre_deploy"
    HookPostDeploy  HookType = "post_deploy"
)
```

## Usage

### Creating the Manager

```go
import "dev.helix.code/internal/hooks"

manager := hooks.NewManager(config)
```

### Registering Hooks

```go
// Register pre-build hook
manager.Register(&hooks.Hook{
    Name:     "lint",
    Type:     hooks.HookPreBuild,
    Priority: 10,
    Handler: func(ctx context.Context, event *hooks.Event) error {
        return runLinter(ctx)
    },
})
```

### Triggering Hooks

```go
// Trigger all hooks of a type
err := manager.Trigger(ctx, hooks.HookPreBuild, &hooks.Event{
    Name: "build_started",
    Data: buildConfig,
})
```

## Configuration

```yaml
hooks:
  enabled: true
  timeout: 5m
  continue_on_error: false
  hooks:
    pre_build:
      - name: lint
        command: "golangci-lint run"
    post_build:
      - name: notify
        command: "curl -X POST https://webhook.example.com"
```

## Testing

```bash
go test -v ./internal/hooks/...
```
