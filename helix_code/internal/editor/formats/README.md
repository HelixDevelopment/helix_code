# Editor Formats

This package defines different edit format strategies for applying code changes from LLM responses.

## Overview

Different LLM models and use cases benefit from different edit formats. This package provides a pluggable format system supporting whole file replacement, unified diffs, search/replace patterns, and line-based editing.

## Format Types

| Format | Use Case | Token Efficiency | Precision |
|--------|----------|------------------|-----------|
| `whole` | Small files, complete rewrites | Low | High |
| `diff` | Standard changes with context | High | High |
| `udiff` | Git-compatible patches | High | High |
| `search-replace` | Targeted string replacements | Very High | Medium |
| `editor` | Line-based modifications | High | Very High |
| `line-number` | Direct line addressing | High | Very High |
| `architect` | High-level structural changes | Very High | Medium |
| `ask` | Confirmation/clarification | - | - |

## Usage

### Format Registry

```go
registry := formats.NewFormatRegistry()

// Register formats
registry.Register(formats.NewDiffFormat())
registry.Register(formats.NewWholeFormat())
registry.Register(formats.NewSearchReplaceFormat())
registry.Register(formats.NewEditorFormat())
```

### Auto-Detection

```go
format, err := registry.DetectFormat(llmResponse)
if err != nil {
    log.Fatal(err)
}

edits, err := format.Parse(ctx, llmResponse)
```

### Manual Selection

```go
diffFormat := registry.Get("diff")
edits, err := diffFormat.Parse(ctx, content)
```

## Format Details

### Diff Format

Standard unified diff format:

```diff
--- a/file.go
+++ b/file.go
@@ -10,3 +10,4 @@
 func main() {
+    fmt.Println("Hello")
 }
```

### Search/Replace Format

Pattern-based replacements:

```
<<<< SEARCH
old code here
====
new code here
>>>> REPLACE
```

### Editor Format

Line-based editing with operations:

```
FILE: main.go
LINE 15: DELETE
LINE 16-20: REPLACE
    new content
    goes here
LINE 25: INSERT
    inserted line
```

### Line Number Format

Direct line addressing:

```
main.go:15 -> fmt.Println("updated")
main.go:20-25 -> {
    newBlock()
}
```

## EditFormat Interface

```go
type EditFormat interface {
    Type() FormatType
    Name() string
    Description() string
    CanHandle(content string) bool
    Parse(ctx context.Context, content string) ([]*FileEdit, error)
    Format(edits []*FileEdit) (string, error)
    PromptTemplate() string
    Validate(content string) error
}
```

## FileEdit Structure

```go
type FileEdit struct {
    FilePath      string                 // Target file
    Operation     EditOperation          // create, update, delete, rename
    OldContent    string                 // Original content (validation)
    NewContent    string                 // New content
    LineNumber    int                    // Starting line (line-based)
    LineCount     int                    // Lines affected
    SearchPattern string                 // Pattern (search/replace)
    ReplaceWith   string                 // Replacement (search/replace)
    Metadata      map[string]interface{} // Additional data
}
```

## Model-Specific Format Selection

The editor package selects optimal formats per LLM model:

| Model | Recommended Format |
|-------|-------------------|
| GPT-4 | diff, search-replace |
| Claude | diff, editor |
| Llama | whole, search-replace |
| Gemini | diff, line-number |
| Local (small) | whole |

## Prompt Templates

Each format includes prompt templates for instructing LLMs:

```go
template := format.PromptTemplate()
prompt := fmt.Sprintf(template, fileContent, instructions)
```

## Validation

Validate edit content before applying:

```go
if err := format.Validate(content); err != nil {
    log.Printf("Invalid format: %v", err)
}
```

## Testing

```bash
go test -v ./internal/editor/formats/...
```

## See Also

- `internal/editor/` - Main editor package with 276+ tests
- `internal/editor/README.md` - Comprehensive editor documentation
- `internal/llm/` - LLM integration
