# Context Mentions

This package handles @-mentions in user prompts, resolving references to files, folders, URLs, and other resources.

## Overview

Mentions allow users to reference resources directly in their prompts using @-syntax:

```
Can you review @file.go and fix the bug?
Check @git-changes for conflicts
```

## Mention Types

| Type | Syntax | Example |
|------|--------|---------|
| File | `@filename` or `@path/to/file` | `@main.go`, `@internal/auth/auth.go` |
| Folder | `@folder/` | `@internal/`, `@tests/` |
| URL | `@http://...` or `@https://...` | `@https://github.com/...` |
| Git Changes | `@git-changes` | Show uncommitted changes |
| Commit | `@commit:abc123` | Reference specific commit |
| Terminal | `@terminal` | Recent terminal output |
| Problems | `@problems` | Current diagnostics/errors |

## Usage

### Processing Mentions

```go
processor := mentions.NewProcessor()

result, err := processor.Process(ctx, "Review @main.go and @tests/")
if err != nil {
    log.Fatal(err)
}

// Access resolved contexts
for _, context := range result.Contexts {
    fmt.Printf("Type: %s, Content: %s\n", context.Type, context.Content)
}

// Use processed text
fmt.Println(result.ProcessedText)
```

### Custom Handlers

Register custom mention handlers:

```go
type MyHandler struct{}

func (h *MyHandler) Type() mentions.MentionType {
    return "custom"
}

func (h *MyHandler) CanHandle(mention string) bool {
    return strings.HasPrefix(mention, "@custom:")
}

func (h *MyHandler) Resolve(ctx context.Context, mention string, options map[string]string) (*mentions.MentionContext, error) {
    // Custom resolution logic
    return &mentions.MentionContext{
        Type:    "custom",
        Target:  mention,
        Content: "resolved content",
    }, nil
}

processor.RegisterHandler(&MyHandler{})
```

### Fuzzy Search

Enable fuzzy file matching:

```go
processor := mentions.NewProcessor(mentions.WithFuzzySearch(true))

// @auth.go will find internal/auth/auth.go
result, _ := processor.Process(ctx, "Fix @auth.go")
```

## Built-in Handlers

### FileMentionHandler

Resolves file paths with optional fuzzy matching:

```go
handler := mentions.NewFileMentionHandler(projectRoot)
handler.SetFuzzyThreshold(0.7) // 70% match threshold
```

### FolderMentionHandler

Resolves folder paths and lists contents:

```go
handler := mentions.NewFolderMentionHandler(projectRoot)
handler.SetMaxDepth(3) // Maximum directory depth
```

### GitMentionHandler

Resolves git references:

```go
handler := mentions.NewGitMentionHandler(repoPath)
// Supports: @git-changes, @commit:hash, @branch:name
```

## MentionContext Structure

```go
type MentionContext struct {
    Type       MentionType            // file, folder, url, git-changes, etc.
    Target     string                 // Original mention target
    Content    string                 // Resolved content
    TokenCount int                    // Approximate token count
    Metadata   map[string]interface{} // Additional metadata
    ResolvedAt time.Time              // Resolution timestamp
}
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithFuzzySearch` | Enable fuzzy file matching | `false` |
| `WithMaxFileSize` | Maximum file size to include | `1MB` |
| `WithTokenLimit` | Maximum tokens per mention | `4000` |
| `WithCaching` | Cache resolved mentions | `true` |

## Testing

```bash
go test -v ./internal/context/mentions/...
```

## See Also

- `internal/context/` - Main context package
- `internal/context/builder/` - Context builder
- `internal/focus/` - Focus management
