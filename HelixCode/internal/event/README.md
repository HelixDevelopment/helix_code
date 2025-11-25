# Event Package

The `event` package provides event-driven architecture support for the HelixCode platform.

## Overview

This package handles:
- Event publishing and subscription
- Event bus implementation
- Asynchronous event handling
- Event persistence
- Event replay

## Key Types

### EventBus

```go
type EventBus struct {
    handlers  map[string][]EventHandler
    queue     chan *Event
    config    *Config
}
```

### Event

```go
type Event struct {
    ID        string
    Type      string
    Source    string
    Data      interface{}
    Timestamp time.Time
}
```

## Usage

### Creating Event Bus

```go
import "dev.helix.code/internal/event"

bus := event.NewBus(config)
bus.Start(ctx)
```

### Publishing Events

```go
event := &event.Event{
    Type:   "task.completed",
    Source: "task-manager",
    Data:   taskResult,
}

err := bus.Publish(ctx, event)
```

### Subscribing to Events

```go
bus.Subscribe("task.*", func(ctx context.Context, e *event.Event) error {
    log.Info("Task event received: %s", e.Type)
    return nil
})
```

## Event Types

- `task.created`, `task.completed`, `task.failed`
- `project.created`, `project.updated`
- `worker.joined`, `worker.left`
- `build.started`, `build.completed`

## Configuration

```yaml
event:
  queue_size: 1000
  workers: 4
  persistence: true
  storage: "redis"
```

## Testing

```bash
go test -v ./internal/event/...
```
