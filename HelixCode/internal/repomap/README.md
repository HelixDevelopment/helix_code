# Repomap Package

The `repomap` package provides codebase mapping and analysis for the HelixCode platform.

## Overview

This package handles:
- Repository structure analysis
- Code symbol extraction
- Dependency mapping
- File relationship tracking
- Codebase visualization

## Key Types

### RepoMapper

```go
type RepoMapper struct {
    symbols      *SymbolTable
    dependencies *DependencyGraph
    files        *FileIndex
    config       *Config
}
```

### Symbol

```go
type Symbol struct {
    Name       string
    Type       SymbolType
    File       string
    Line       int
    References []Reference
}
```

## Usage

### Mapping a Repository

```go
import "dev.helix.code/internal/repomap"

mapper := repomap.NewMapper(config)
repoMap, err := mapper.Map(ctx, "/path/to/repo")
```

### Finding Symbols

```go
// Find function definition
symbol, err := repoMap.FindSymbol("MyFunction")

// Find all references
refs := repoMap.FindReferences("MyStruct")
```

### Dependency Analysis

```go
// Get file dependencies
deps := repoMap.GetDependencies("main.go")

// Get reverse dependencies
rdeps := repoMap.GetDependents("utils.go")
```

### Generating Map Output

```go
// Generate tree structure
tree := repoMap.GenerateTree()

// Generate JSON map
jsonMap, err := repoMap.ToJSON()
```

## Supported Languages

- Go
- Python
- JavaScript/TypeScript
- Rust
- Java
- C/C++

## Configuration

```yaml
repomap:
  enabled: true
  max_depth: 10
  include_patterns: ["*.go", "*.py", "*.ts"]
  exclude_patterns: ["*_test.go", "vendor/*"]
  extract_symbols: true
  analyze_dependencies: true
```

## Testing

```bash
go test -v ./internal/repomap/...
```
