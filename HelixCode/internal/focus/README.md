# Focus Package

The focus package provides a hierarchical attention management system for tracking development focus points during coding sessions. It helps HelixCode prioritize and organize work across files, tasks, and project components.

## Overview

Focus management enables the AI agent to maintain context awareness by tracking:
- Current file or directory being worked on
- Active tasks and features
- Error investigations
- Test cases
- Specific functions, classes, or packages

## Types

### FocusType

Defines what kind of entity is being focused on:

| Type | Description |
|------|-------------|
| `file` | Single file |
| `directory` | Directory |
| `task` | Task/feature |
| `error` | Error/bug investigation |
| `test` | Test case |
| `function` | Specific function |
| `class` | Class/struct |
| `package` | Package/module |
| `project` | Entire project |
| `custom` | Custom focus |

### FocusPriority

Priority levels for focuses:

| Priority | Value | Usage |
|----------|-------|-------|
| `PriorityLow` | 1 | Background tasks |
| `PriorityNormal` | 5 | Standard work |
| `PriorityHigh` | 10 | Important items |
| `PriorityCritical` | 20 | Urgent issues |

### Focus

The main struct representing a focus point:

```go
type Focus struct {
    ID          string                 // Unique identifier
    Type        FocusType              // Type of focus
    Target      string                 // Target (file path, task name, etc.)
    Description string                 // Human-readable description
    Priority    FocusPriority          // Priority level
    Context     map[string]interface{} // Additional context
    CreatedAt   time.Time              // When focus was created
    UpdatedAt   time.Time              // Last update time
    ExpiresAt   *time.Time             // Optional expiration
    Parent      *Focus                 // Parent focus (hierarchical)
    Children    []*Focus               // Child focuses
    Tags        []string               // Tags for categorization
    Metadata    map[string]string      // Custom metadata
}
```

## Usage

### Creating a Focus

```go
// Simple focus
focus := focus.NewFocus(focus.FocusTypeFile, "/path/to/file.go")

// With priority
focus := focus.NewFocusWithPriority(
    focus.FocusTypeError,
    "nil pointer in handler",
    focus.PriorityCritical,
)
```

### Building Focus Hierarchies

```go
// Create project focus
project := focus.NewFocus(focus.FocusTypeProject, "my-app")

// Add package focus
pkg := focus.NewFocus(focus.FocusTypePackage, "internal/auth")
project.AddChild(pkg)

// Add file focus
file := focus.NewFocus(focus.FocusTypeFile, "handler.go")
pkg.AddChild(file)
```

### Working with Context and Tags

```go
f := focus.NewFocus(focus.FocusTypeTask, "implement-auth")

// Add context
f.SetContext("related_files", []string{"auth.go", "token.go"})
f.SetContext("assignee", "ai-agent")

// Add tags
f.AddTag("security")
f.AddTag("high-priority")

// Check tags
if f.HasTag("security") {
    // Handle security-related focus
}
```

### Expiration and Lifecycle

```go
// Set expiration
f.SetExpiration(2 * time.Hour)

// Check if expired
if f.IsExpired() {
    // Clean up focus
}

// Update timestamp
f.Touch()
```

### Hierarchy Navigation

```go
// Get depth in hierarchy
depth := focus.GetDepth()

// Get root focus
root := focus.GetRoot()

// Get path from root to current
path := focus.GetPath()

// Find child by ID
child := focus.FindChild("focus-id-123")

// Count all descendants
count := focus.CountDescendants()
```

## Features

- **Hierarchical Structure**: Organize focuses in parent-child relationships
- **Priority Management**: Assign importance levels to focuses
- **Expiration**: Auto-expire focuses after a set duration
- **Tagging**: Categorize focuses with multiple tags
- **Context Storage**: Attach arbitrary data to focuses
- **Deep Cloning**: Clone entire focus hierarchies
- **Validation**: Ensure focus integrity with validation

## Integration

The focus system integrates with:
- **Task Manager**: Track which tasks are currently active
- **Context Builder**: Include relevant focuses in AI prompts
- **Agent Orchestration**: Direct agent attention to priority items
