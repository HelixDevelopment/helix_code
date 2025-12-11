# Rules Package

The `rules` package provides rule engine and validation for the HelixCode platform.

## Overview

This package handles:
- Business rule definition
- Rule evaluation
- Conditional logic
- Rule chaining
- Validation rules

## Key Types

### RuleEngine

```go
type RuleEngine struct {
    rules     []*Rule
    evaluator *Evaluator
    config    *Config
}
```

### Rule

```go
type Rule struct {
    Name      string
    Condition string
    Action    ActionHandler
    Priority  int
    Enabled   bool
}
```

## Usage

### Creating Rules

```go
import "dev.helix.code/internal/rules"

engine := rules.NewEngine(config)

engine.AddRule(&rules.Rule{
    Name:      "high-priority-task",
    Condition: "task.priority == 'critical' && task.status == 'pending'",
    Action: func(ctx context.Context, data interface{}) error {
        return notifyTeam(ctx, data)
    },
})
```

### Evaluating Rules

```go
results, err := engine.Evaluate(ctx, taskData)
```

## Condition Syntax

```
field == value
field != value
field > value
field contains value
field starts_with value
field && other_field
field || other_field
```

## Configuration

```yaml
rules:
  enabled: true
  evaluation_mode: "first_match"  # or "all"
  rules:
    - name: critical-alert
      condition: "severity == 'critical'"
      action: "notify_oncall"
```

## Testing

```bash
go test -v ./internal/rules/...
```
