# Repomap Package

The `repomap` package provides semantic codebase mapping and symbol extraction for the HelixCode platform. It creates intelligent mappings of source code repositories using tree-sitter for accurate multi-language parsing and implements smart ranking algorithms to select the most relevant code context within token budgets for AI-powered development workflows.

## Overview

This package handles:
- Repository structure analysis and file discovery
- Multi-language code symbol extraction using tree-sitter
- Intelligent file ranking based on relevance and context
- Token-budget-aware context selection for LLM interactions
- Disk-based caching with TTL for parsed symbols
- Dependency and import relationship tracking

## Architecture

The repomap system consists of five core components:

1. **RepoMap** - Main orchestrator coordinating symbol extraction and context building
2. **TreeSitterParser** - Multi-language parsing using tree-sitter grammars
3. **TagExtractor** - Language-specific symbol extraction from syntax trees
4. **FileRanker** - Relevance scoring and file ranking algorithms
5. **RepoCache** - Disk-based caching with TTL and cleanup routines

## Key Types

### RepoMap

The main entry point for repository mapping:

```go
type RepoMap struct {
    rootPath string
    parser   *TreeSitterParser
    cache    *RepoCache
    ranker   *FileRanker
    config   RepoMapConfig
    mu       sync.RWMutex
}
```

### RepoMapConfig

Configuration controlling mapping behavior:

```go
type RepoMapConfig struct {
    MaxFiles           int           // Maximum files to include in context
    TokenBudget        int           // Token limit for LLM context
    CacheEnabled       bool          // Enable symbol caching
    LanguagesSupported []string      // Languages to process
    CacheTTL           time.Duration // Cache time-to-live
    MaxConcurrency     int           // Parallel processing limit
}
```

### Symbol

Represents a code symbol (function, class, method, etc.):

```go
type Symbol struct {
    Name      string      // Symbol name
    Type      SymbolType  // function, method, class, struct, etc.
    FilePath  string      // Source file path
    LineStart int         // Starting line number
    LineEnd   int         // Ending line number
    Signature string      // Full signature/definition
    Docstring string      // Documentation comment
    Parent    string      // Parent class/struct for methods
}
```

### SymbolType

Enumeration of supported symbol types:

```go
const (
    SymbolTypeFunction  SymbolType = "function"
    SymbolTypeMethod    SymbolType = "method"
    SymbolTypeClass     SymbolType = "class"
    SymbolTypeInterface SymbolType = "interface"
    SymbolTypeStruct    SymbolType = "struct"
    SymbolTypeEnum      SymbolType = "enum"
    SymbolTypeTrait     SymbolType = "trait"
    SymbolTypeModule    SymbolType = "module"
    SymbolTypeVariable  SymbolType = "variable"
    SymbolTypeConstant  SymbolType = "constant"
    SymbolTypeImport    SymbolType = "import"
    SymbolTypeExport    SymbolType = "export"
)
```

### FileContext

Contextual information about a file for LLM context:

```go
type FileContext struct {
    FilePath   string    // Path to the file
    Symbols    []Symbol  // Extracted symbols
    Content    string    // File content
    Relevance  float64   // Relevance score (0-1)
    TokenCount int       // Estimated token count
}
```

### FileScore

Relevance score for a file:

```go
type FileScore struct {
    FilePath string    // Path to the file
    Score    float64   // Relevance score
    Reasons  []string  // Reasons for the score (debugging)
}
```

## Supported Languages

The package supports symbol extraction for:

| Language | Extensions | Symbol Types |
|----------|------------|--------------|
| Go | `.go` | functions, methods, structs, interfaces |
| Python | `.py` | functions, classes, methods |
| JavaScript | `.js`, `.jsx` | functions, classes, methods, arrow functions |
| TypeScript | `.ts`, `.tsx` | functions, classes, methods, arrow functions |
| Java | `.java` | classes, methods, interfaces |
| C | `.c`, `.h` | functions, structs |
| C++ | `.cpp`, `.cc`, `.cxx`, `.hpp` | functions, classes, structs |
| Rust | `.rs` | functions, structs, enums, traits |
| Ruby | `.rb` | methods, classes, modules |

## Usage Examples

### Basic Repository Mapping

```go
import "dev.helix.code/internal/repomap"

config := repomap.DefaultConfig()
config.TokenBudget = 8000
config.MaxFiles = 100

rm, err := repomap.NewRepoMap("/path/to/project", config)
if err != nil {
    log.Fatal(err)
}

// Get optimal context for a query
contexts, err := rm.GetOptimalContext("implement user authentication", []string{})
if err != nil {
    log.Fatal(err)
}

for _, ctx := range contexts {
    fmt.Printf("File: %s (relevance: %.2f, tokens: %d)\n",
        ctx.FilePath, ctx.Relevance, ctx.TokenCount)
    for _, sym := range ctx.Symbols {
        fmt.Printf("  - %s: %s\n", sym.Type, sym.Name)
    }
}
```

### Context with Changed Files

```go
// Prioritize recently changed files
changedFiles := []string{
    "/path/to/project/auth/login.go",
    "/path/to/project/auth/session.go",
}

contexts, err := rm.GetOptimalContext("fix login bug", changedFiles)
if err != nil {
    log.Fatal(err)
}

// Changed files will have higher relevance scores
```

### Custom Configuration

```go
config := repomap.RepoMapConfig{
    MaxFiles:           50,
    TokenBudget:        4000,
    CacheEnabled:       true,
    LanguagesSupported: []string{"go", "python", "typescript"},
    CacheTTL:           12 * time.Hour,
    MaxConcurrency:     8,
}

rm, err := repomap.NewRepoMap(projectPath, config)
```

### Getting Repository Statistics

```go
stats, err := rm.GetStatistics()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Total files: %d\n", stats.TotalFiles)
fmt.Printf("Total symbols: %d\n", stats.TotalSymbols)
fmt.Printf("Cached files: %d\n", stats.CachedFiles)
fmt.Printf("Indexing duration: %v\n", stats.IndexingDuration)
fmt.Printf("Languages: %v\n", stats.Languages)
```

### Cache Management

```go
// Refresh entire cache
err := rm.RefreshCache()

// Invalidate specific file
err := rm.InvalidateFile("/path/to/changed/file.go")
```

## File Ranking

The `FileRanker` scores files based on multiple factors:

### Ranking Weights

```go
type RankingWeights struct {
    RecentlyChanged float64  // Weight for recently modified files (0.3)
    SymbolMatch     float64  // Weight for symbol name matches (0.4)
    ImportFrequency float64  // Weight for import frequency (0.1)
    DependencyDepth float64  // Weight for central dependencies (0.1)
    FileSize        float64  // Weight for optimal file size (0.05)
    SymbolDensity   float64  // Weight for symbol density (0.05)
}
```

### Ranking Factors

1. **Recently Changed** - Files modified recently get higher scores
   - Within 1 hour: 0.8
   - Within 24 hours: 0.5
   - Within 7 days: 0.2

2. **Symbol Match** - Symbols matching query terms score higher
   - Classes/interfaces: 1.0 weight
   - Functions: 0.8 weight
   - Methods: 0.7 weight
   - Variables: 0.2 weight

3. **Import Frequency** - Files imported by many others rank higher

4. **Dependency Depth** - Central files with many exported symbols rank higher

5. **File Size** - Files of optimal size (5KB-15KB) preferred
   - Very small (<5KB): 0.3
   - Optimal (5KB-15KB): 1.0
   - Large (15KB-50KB): 0.7
   - Very large (>50KB): 0.3

6. **Symbol Density** - Files with good symbol-to-line ratio preferred

### Custom Ranking

```go
weights := repomap.RankingWeights{
    RecentlyChanged: 0.5,  // Prioritize recent changes
    SymbolMatch:     0.3,
    ImportFrequency: 0.1,
    DependencyDepth: 0.05,
    FileSize:        0.03,
    SymbolDensity:   0.02,
}

ranker := repomap.NewFileRankerWithWeights(weights)
```

## Caching System

### RepoCache

Disk-based caching with automatic cleanup:

```go
type RepoCache struct {
    cacheDir string
    ttl      time.Duration
    mu       sync.RWMutex
    entries  map[string]*cacheEntry
}
```

### Cache Operations

```go
// Create cache
cache, err := repomap.NewRepoCache(cacheDir, 24*time.Hour)

// Set value
cache.Set(key, symbols)

// Get value
symbols, found := cache.Get(key)

// Check existence
exists := cache.Has(key)

// Compute if not cached
symbols, err := cache.GetOrCompute(key, func() (interface{}, error) {
    return extractSymbols(file)
})

// Invalidate
cache.Invalidate(key)
cache.InvalidateAll()

// Cleanup expired entries
removed := cache.Cleanup()

// Start background cleanup routine
stop := cache.StartCleanupRoutine(1 * time.Hour)
defer close(stop)
```

### Cache Statistics

```go
stats := cache.GetStats()
fmt.Printf("Total entries: %d\n", stats.TotalEntries)
fmt.Printf("Expired entries: %d\n", stats.ExpiredEntries)
fmt.Printf("Total size: %d bytes\n", stats.TotalSize)
```

### Cache Export/Import

```go
// Export cache for backup
exportFile, _ := os.Create("cache_backup.gob")
cache.Export(exportFile)
exportFile.Close()

// Import cache
importFile, _ := os.Open("cache_backup.gob")
cache.Import(importFile)
importFile.Close()
```

## Tree-Sitter Integration

### TreeSitterParser

Multi-language parsing with tree-sitter:

```go
parser := repomap.NewTreeSitterParser()

// Parse a file
tree, err := parser.ParseFile("/path/to/file.go", "go")

// Extract symbols
symbols, err := parser.ExtractSymbols(tree, filePath, "go")

// Get supported languages
langs := parser.SupportedLanguages()
// ["go", "python", "javascript", "typescript", "java", "c", "cpp", "rust", "ruby"]
```

### Language Queries

Tree-sitter queries for each language:

```go
queries := repomap.GetLanguageQueries("go")
// Returns queries for functions, methods, types, interfaces, structs
```

## Configuration

### Default Configuration

```go
func DefaultConfig() RepoMapConfig {
    return RepoMapConfig{
        MaxFiles:           100,
        TokenBudget:        8000,
        CacheEnabled:       true,
        LanguagesSupported: []string{
            "go", "python", "javascript", "typescript",
            "java", "c", "cpp", "rust", "ruby",
        },
        CacheTTL:       24 * time.Hour,
        MaxConcurrency: 4,
    }
}
```

### YAML Configuration

```yaml
repomap:
  enabled: true
  max_files: 100
  token_budget: 8000
  cache_enabled: true
  cache_ttl: "24h"
  max_concurrency: 4
  languages:
    - go
    - python
    - typescript
    - javascript
    - java
    - rust
  ignore_patterns:
    - "*_test.go"
    - "vendor/*"
    - "node_modules/*"
```

## Best Practices

### 1. Enable Caching for Large Repositories

Caching dramatically improves performance for repositories with many files:

```go
config.CacheEnabled = true
config.CacheTTL = 24 * time.Hour
```

### 2. Tune Token Budget for LLM Context

Match token budget to your LLM's context window:

```go
config.TokenBudget = 8000  // GPT-3.5 suitable
config.TokenBudget = 32000 // GPT-4 suitable
config.TokenBudget = 100000 // Claude suitable
```

### 3. Prioritize Changed Files

Always pass recently changed files for better context:

```go
changedFiles := getGitChangedFiles()
contexts, _ := rm.GetOptimalContext(query, changedFiles)
```

### 4. Use Selective Language Support

Limit languages to reduce processing time:

```go
config.LanguagesSupported = []string{"go", "typescript"}
```

### 5. Refresh Cache on Major Changes

Invalidate cache when significant changes occur:

```go
rm.RefreshCache()
```

## Integration Patterns

### With AI Agents

```go
func buildAIContext(query string, changedFiles []string) string {
    rm, _ := repomap.NewRepoMap(projectPath, repomap.DefaultConfig())
    contexts, _ := rm.GetOptimalContext(query, changedFiles)

    var builder strings.Builder
    for _, ctx := range contexts {
        builder.WriteString(fmt.Sprintf("// File: %s\n", ctx.FilePath))
        builder.WriteString(ctx.Content)
        builder.WriteString("\n\n")
    }
    return builder.String()
}
```

### With Code Analysis Tools

```go
func analyzeCodebase(path string) map[string]int {
    rm, _ := repomap.NewRepoMap(path, repomap.DefaultConfig())
    stats, _ := rm.GetStatistics()
    return stats.Languages
}
```

## File Discovery

The package automatically discovers source files while ignoring:
- `.git` directories
- `node_modules` directories
- `vendor` directories
- `.helix` directories
- `__pycache__` directories
- `dist`, `build`, `target` directories

## Performance Considerations

1. **Cache Hit Rate** - Monitor and optimize cache TTL for your workflow
2. **Token Budget** - Lower budgets mean faster processing but less context
3. **Concurrency** - Adjust `MaxConcurrency` based on available CPU cores
4. **Language Selection** - Fewer languages = faster discovery and parsing

## Thread Safety

All `RepoMap` operations are thread-safe through internal `sync.RWMutex` protection, allowing concurrent access from multiple goroutines. The cache also uses mutex protection for thread-safe operations.

## Testing

```bash
# Run all repomap tests
go test -v ./internal/repomap/...

# Run with coverage
go test -cover ./internal/repomap/...

# Run specific test
go test -v ./internal/repomap -run TestGetOptimalContext
```

## Notes

- Symbol extraction uses tree-sitter for accurate AST-based parsing
- Cache entries include file modification time for automatic invalidation
- Token count estimation uses 1 token per 4 characters approximation
- Exported symbols (uppercase in Go) are weighted higher for dependency analysis
