# Context Builder

This package provides a fluent API for building LLM conversation context.

## Overview

The Context Builder assembles and prioritizes context items from multiple sources to create optimal prompts for LLM calls. It manages context size limits, caching, and priority-based content selection.

## Key Components

### Builder

The main builder that assembles context from multiple sources:

```go
builder := builder.NewBuilder(sessionMgr, focusMgr)
builder.SetMaxSize(128 * 1024)  // 128KB limit
builder.SetMaxTokens(8000)       // ~8000 tokens
```

### Context Items

Individual pieces of context with priority levels:

```go
type ContextItem struct {
    Type     SourceType        // session, focus, file, git, project, error, log, custom
    Priority Priority          // low (1), normal (5), high (10), critical (20)
    Title    string
    Content  string
    Metadata map[string]string
    Size     int
}
```

### Sources

Pluggable sources that provide context:

| Source Type | Description |
|-------------|-------------|
| `session` | Conversation history |
| `focus` | Currently focused files/code |
| `file` | File contents |
| `git` | Git changes, commits, blame |
| `project` | Project metadata |
| `error` | Error messages, stack traces |
| `log` | Log output |
| `custom` | User-defined content |

## Usage

### Basic Context Building

```go
builder := builder.NewBuilder(sessionMgr, focusMgr)

ctx, err := builder.
    AddSession().
    AddFocus().
    AddFile("/path/to/file.go", builder.PriorityHigh).
    AddGitChanges().
    Build()
```

### Priority-Based Selection

When context exceeds size limits, lower priority items are trimmed:

```go
builder.
    Add("Critical requirement", builder.PriorityCritical).
    Add("Main task details", builder.PriorityHigh).
    Add("Background info", builder.PriorityNormal).
    Add("Nice to have", builder.PriorityLow)
```

### Template-Based Building

Use predefined templates for common scenarios:

```go
builder.UseTemplate("code-review").
    WithFile(filePath).
    WithGitDiff().
    Build()
```

### Caching

Enable caching for repeated context builds:

```go
builder.EnableCache(5 * time.Minute)
```

## Templates

Pre-defined templates in `templates.go`:

| Template | Description |
|----------|-------------|
| `code-review` | Review code changes with git context |
| `bug-fix` | Debug with error logs and stack traces |
| `new-feature` | Add feature with project context |
| `refactor` | Restructure code with full file context |
| `documentation` | Document code with examples |

## API Reference

### Builder Methods

```go
// Source additions
AddSession() *Builder
AddFocus() *Builder
AddFile(path string, priority Priority) *Builder
AddGitChanges() *Builder
AddGitCommit(hash string) *Builder
AddProject() *Builder
AddError(err error) *Builder
AddLog(lines int) *Builder
Add(content string, priority Priority) *Builder

// Configuration
SetMaxSize(bytes int) *Builder
SetMaxTokens(tokens int) *Builder
UseTemplate(name string) *Builder
EnableCache(ttl time.Duration) *Builder

// Build
Build() (*Context, error)
```

## Testing

```bash
go test -v ./internal/context/builder/...
```

## See Also

- `internal/context/` - Main context package
- `internal/context/mentions/` - Mention handling
- `internal/session/` - Session management
- `internal/focus/` - Focus management
