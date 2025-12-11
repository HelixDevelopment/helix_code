# Fix Package

The `fix` package provides automated code fixing and repair for the HelixCode platform.

## Overview

This package handles:
- Automated bug fixes
- Code refactoring
- Lint fixes
- Style corrections
- Security patches

## Key Types

### Fixer

```go
type Fixer struct {
    llm       llm.Provider
    analyzers map[string]Analyzer
    config    *Config
}
```

### Fix

```go
type Fix struct {
    File        string
    Line        int
    Description string
    Original    string
    Fixed       string
    Confidence  float64
}
```

## Usage

### Fixing Code

```go
import "dev.helix.code/internal/fix"

fixer := fix.NewFixer(llmProvider, config)

// Fix single file
fixes, err := fixer.FixFile(ctx, "main.go")

// Fix project
fixes, err := fixer.FixProject(ctx, projectPath)
```

### Applying Fixes

```go
// Review fixes
for _, f := range fixes {
    fmt.Printf("File: %s, Line: %d\n", f.File, f.Line)
    fmt.Printf("Fix: %s\n", f.Description)
}

// Apply selected fixes
err := fixer.ApplyFixes(ctx, selectedFixes)
```

## Fix Types

- Syntax errors
- Lint issues
- Security vulnerabilities
- Performance issues
- Code style

## Configuration

```yaml
fix:
  auto_fix: false
  min_confidence: 0.8
  backup: true
```

## Testing

```bash
go test -v ./internal/fix/...
```
