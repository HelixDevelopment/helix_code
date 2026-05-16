# Mapping Package

The `mapping` package provides comprehensive codebase analysis and mapping capabilities using tree-sitter parsers for semantic understanding of code structure across 30+ programming languages. It enables AI agents to understand code context, navigate codebases, and extract relevant definitions.

## Overview

This package enables:
- Semantic code parsing using tree-sitter for 30+ languages
- Definition extraction (functions, classes, methods, types, constants)
- Codebase mapping with intelligent caching
- Dependency graph analysis
- Token counting for LLM context management
- Symbol search and navigation

## Supported Languages

The mapping package supports parsing for the following languages:

| Language | Extensions | Parser |
|----------|-----------|--------|
| Go | .go | tree-sitter-go |
| JavaScript | .js, .jsx, .mjs | tree-sitter-javascript |
| TypeScript | .ts, .tsx | tree-sitter-typescript |
| Python | .py, .pyi | tree-sitter-python |
| Rust | .rs | tree-sitter-rust |
| Java | .java | tree-sitter-java |
| C | .c, .h | tree-sitter-c |
| C++ | .cpp, .cc, .hpp | tree-sitter-cpp |
| C# | .cs | tree-sitter-c-sharp |
| Ruby | .rb | tree-sitter-ruby |
| PHP | .php | tree-sitter-php |
| Swift | .swift | tree-sitter-swift |
| Kotlin | .kt, .kts | tree-sitter-kotlin |
| Scala | .scala | tree-sitter-scala |
| HTML | .html, .htm | tree-sitter-html |
| CSS | .css | tree-sitter-css |
| JSON | .json | tree-sitter-json |
| YAML | .yaml, .yml | tree-sitter-yaml |
| Markdown | .md | tree-sitter-markdown |
| Bash | .sh, .bash | tree-sitter-bash |
| SQL | .sql | tree-sitter-sql |
| And more... | | |

## Key Types

### Mapper

The main interface for codebase mapping operations.

```go
type Mapper interface {
    // MapDirectory maps all files in a directory
    MapDirectory(ctx context.Context, path string, opts *MapOptions) (*CodebaseMap, error)

    // MapFile maps a single file
    MapFile(ctx context.Context, path string) (*FileMap, error)

    // GetDefinitions extracts definitions from a file
    GetDefinitions(ctx context.Context, path string) ([]*Definition, error)

    // FindSymbol searches for a symbol across the codebase
    FindSymbol(ctx context.Context, name string, opts *FindOptions) ([]*Symbol, error)

    // GetDependencies returns dependencies for a file
    GetDependencies(ctx context.Context, path string) ([]*Dependency, error)

    // GetTokenCount estimates token count for content
    GetTokenCount(content string) int
}
```

### DefaultMapper

The primary implementation of the Mapper interface.

```go
type DefaultMapper struct {
    parsers     map[string]*sitter.Parser
    cache       *MapCache
    config      *MapperConfig
    mu          sync.RWMutex
}

type MapperConfig struct {
    CacheEnabled    bool          // Enable mapping cache
    CacheDir        string        // Cache directory (.helix.cache)
    CacheTTL        time.Duration // Cache entry TTL
    MaxFileSize     int64         // Max file size to parse
    MaxDepth        int           // Max directory depth
    IgnorePatterns  []string      // Patterns to ignore
    IncludePatterns []string      // Patterns to include
    ParallelParsing bool          // Enable parallel file parsing
    WorkerCount     int           // Number of parallel workers
}
```

### CodebaseMap

Represents a mapped codebase structure.

```go
type CodebaseMap struct {
    RootPath     string               // Root directory path
    Files        map[string]*FileMap  // Path -> FileMap
    Definitions  []*Definition        // All definitions
    Dependencies *DependencyGraph     // Dependency relationships
    Stats        *MapStats            // Mapping statistics
    Timestamp    time.Time            // Map creation time
}

type MapStats struct {
    TotalFiles      int
    TotalLines      int
    TotalDefinitions int
    TotalTokens     int
    ParseErrors     int
    Duration        time.Duration
    ByLanguage      map[string]LanguageStats
}

type LanguageStats struct {
    Files       int
    Lines       int
    Definitions int
    Tokens      int
}
```

### FileMap

Represents a mapped file.

```go
type FileMap struct {
    Path        string
    Language    string
    Size        int64
    Lines       int
    Tokens      int
    Definitions []*Definition
    Imports     []string
    Exports     []string
    Checksum    string
    ParsedAt    time.Time
    Error       error
}
```

### Definition

Represents a code definition (function, class, etc.).

```go
type Definition struct {
    Name        string
    Kind        DefinitionKind  // Function, Class, Method, Type, Constant, Variable
    Language    string
    FilePath    string
    StartLine   int
    EndLine     int
    StartCol    int
    EndCol      int
    Signature   string          // Full signature
    DocComment  string          // Documentation comment
    Visibility  Visibility      // Public, Private, Protected
    Parent      string          // Parent class/struct for methods
    Parameters  []*Parameter    // Function parameters
    ReturnType  string          // Return type
    Modifiers   []string        // async, static, abstract, etc.
}

type DefinitionKind int

const (
    KindFunction DefinitionKind = iota
    KindClass
    KindMethod
    KindType
    KindInterface
    KindStruct
    KindEnum
    KindConstant
    KindVariable
    KindProperty
    KindModule
)
```

### DependencyGraph

Represents file and symbol dependencies.

```go
type DependencyGraph struct {
    Nodes map[string]*DependencyNode
    Edges []*DependencyEdge
}

type DependencyNode struct {
    Path     string
    Type     string   // file, package, module
    Imports  []string
    Exports  []string
}

type DependencyEdge struct {
    From     string
    To       string
    Type     string  // import, extends, implements
    Symbols  []string
}
```

## Usage Examples

### Basic Codebase Mapping

```go
package main

import (
    "context"
    "fmt"

    "dev.helix.code/internal/tools/mapping"
)

func main() {
    // Create mapper with caching
    mapper, err := mapping.NewDefaultMapper(&mapping.MapperConfig{
        CacheEnabled:    true,
        CacheDir:        ".helix.cache",
        CacheTTL:        1 * time.Hour,
        ParallelParsing: true,
        WorkerCount:     4,
        IgnorePatterns:  []string{"node_modules/**", "vendor/**", ".git/**"},
    })
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    // Map entire directory
    codebaseMap, err := mapper.MapDirectory(ctx, "/path/to/project", &mapping.MapOptions{
        MaxDepth:        10,
        IncludeTests:    true,
        ExtractComments: true,
    })
    if err != nil {
        panic(err)
    }

    // Print statistics
    fmt.Printf("Mapped %d files\n", codebaseMap.Stats.TotalFiles)
    fmt.Printf("Found %d definitions\n", codebaseMap.Stats.TotalDefinitions)
    fmt.Printf("Total lines: %d\n", codebaseMap.Stats.TotalLines)
    fmt.Printf("Total tokens: %d\n", codebaseMap.Stats.TotalTokens)
}
```

### Extracting Definitions

```go
// Get definitions from a single file
definitions, err := mapper.GetDefinitions(ctx, "src/auth/handler.go")

for _, def := range definitions {
    fmt.Printf("[%s] %s\n", def.Kind, def.Name)
    fmt.Printf("  Location: %s:%d-%d\n", def.FilePath, def.StartLine, def.EndLine)
    fmt.Printf("  Signature: %s\n", def.Signature)
    if def.DocComment != "" {
        fmt.Printf("  Doc: %s\n", def.DocComment)
    }
}

// Filter by kind
functions := mapping.FilterDefinitions(definitions, mapping.KindFunction)
classes := mapping.FilterDefinitions(definitions, mapping.KindClass)

// Get public definitions only
publicDefs := mapping.FilterByVisibility(definitions, mapping.VisibilityPublic)
```

### Symbol Search

```go
// Search for a symbol by name
symbols, err := mapper.FindSymbol(ctx, "AuthenticateUser", &mapping.FindOptions{
    IncludeReferences: true,
    MaxResults:        20,
})

for _, sym := range symbols {
    fmt.Printf("Found: %s in %s\n", sym.Name, sym.FilePath)
    if sym.Definition != nil {
        fmt.Printf("  Definition at line %d\n", sym.Definition.StartLine)
    }
    fmt.Printf("  References: %d\n", len(sym.References))
}

// Search with pattern
symbols, err = mapper.FindSymbol(ctx, "Auth*", &mapping.FindOptions{
    UsePattern:   true,
    CaseSensitive: false,
})

// Search by kind
symbols, err = mapper.FindSymbol(ctx, "", &mapping.FindOptions{
    Kind: mapping.KindClass,
})
```

### Dependency Analysis

```go
// Get dependencies for a file
deps, err := mapper.GetDependencies(ctx, "src/main.go")

for _, dep := range deps {
    fmt.Printf("Imports: %s\n", dep.Target)
    fmt.Printf("  Type: %s\n", dep.Type)
    fmt.Printf("  Symbols: %v\n", dep.Symbols)
}

// Build full dependency graph
graph := codebaseMap.Dependencies

// Find files that depend on a specific file
dependents := graph.GetDependents("src/utils/helpers.go")
for _, d := range dependents {
    fmt.Printf("%s depends on helpers.go\n", d)
}

// Find circular dependencies
cycles := graph.FindCycles()
for _, cycle := range cycles {
    fmt.Printf("Circular dependency: %v\n", cycle)
}

// Get import tree
tree := graph.GetImportTree("src/main.go", 3) // 3 levels deep
```

### Token Counting

```go
// Count tokens in content
content := `func main() {
    fmt.Println("Hello, World!")
}`
tokens := mapper.GetTokenCount(content)
fmt.Printf("Estimated tokens: %d\n", tokens)

// Count tokens in file
fileMap, _ := mapper.MapFile(ctx, "src/main.go")
fmt.Printf("File tokens: %d\n", fileMap.Tokens)

// Get total tokens for a selection of files
totalTokens := 0
for _, path := range []string{"src/auth.go", "src/handlers.go"} {
    fm, _ := mapper.MapFile(ctx, path)
    totalTokens += fm.Tokens
}
fmt.Printf("Total tokens for selection: %d\n", totalTokens)
```

### Caching

```go
// Configure caching
mapper, _ := mapping.NewDefaultMapper(&mapping.MapperConfig{
    CacheEnabled: true,
    CacheDir:     ".helix.cache",
    CacheTTL:     2 * time.Hour,
})

// First mapping - will parse files
start := time.Now()
map1, _ := mapper.MapDirectory(ctx, projectPath, nil)
fmt.Printf("First map: %v\n", time.Since(start))

// Second mapping - uses cache
start = time.Now()
map2, _ := mapper.MapDirectory(ctx, projectPath, nil)
fmt.Printf("Cached map: %v\n", time.Since(start))

// Clear cache
mapper.ClearCache()

// Invalidate specific file
mapper.InvalidateCache("src/modified_file.go")

// Get cache statistics
stats := mapper.CacheStats()
fmt.Printf("Cache hits: %d, misses: %d\n", stats.Hits, stats.Misses)
```

### Language-Specific Parsing

```go
// Map only specific languages
codebaseMap, err := mapper.MapDirectory(ctx, projectPath, &mapping.MapOptions{
    Languages: []string{"go", "python", "javascript"},
})

// Get language statistics
for lang, stats := range codebaseMap.Stats.ByLanguage {
    fmt.Printf("%s: %d files, %d definitions\n", lang, stats.Files, stats.Definitions)
}

// Check if language is supported
if mapper.SupportsLanguage("rust") {
    fmt.Println("Rust parsing is supported")
}

// Get supported languages
languages := mapper.SupportedLanguages()
fmt.Printf("Supported: %v\n", languages)
```

### Generating Context for LLM

```go
// Generate context string for LLM
context := mapping.GenerateContext(codebaseMap, &mapping.ContextOptions{
    MaxTokens:       4000,
    IncludeStructure: true,
    IncludeSignatures: true,
    PrioritizeFiles:  []string{"src/main.go", "src/api/"},
})

// Generate file-specific context
fileContext := mapping.GenerateFileContext(fileMap, &mapping.ContextOptions{
    IncludeImports:    true,
    IncludeDefinitions: true,
    IncludeComments:   true,
})

// Generate symbol-focused context
symbolContext := mapping.GenerateSymbolContext(symbol, &mapping.ContextOptions{
    IncludeReferences: true,
    IncludeCallers:    true,
    IncludeCallees:    true,
})
```

## Configuration Options

### MapperConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `CacheEnabled` | bool | true | Enable mapping cache |
| `CacheDir` | string | .helix.cache | Cache directory |
| `CacheTTL` | time.Duration | 1h | Cache entry TTL |
| `MaxFileSize` | int64 | 1MB | Max file size to parse |
| `MaxDepth` | int | 50 | Max directory depth |
| `IgnorePatterns` | []string | [] | Glob patterns to ignore |
| `IncludePatterns` | []string | [] | Glob patterns to include |
| `ParallelParsing` | bool | true | Enable parallel parsing |
| `WorkerCount` | int | CPU count | Parallel worker count |

### MapOptions

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `MaxDepth` | int | 50 | Max directory depth |
| `IncludeTests` | bool | true | Include test files |
| `ExtractComments` | bool | true | Extract doc comments |
| `Languages` | []string | all | Languages to parse |
| `FollowSymlinks` | bool | false | Follow symbolic links |

### FindOptions

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `UsePattern` | bool | false | Enable glob pattern matching |
| `CaseSensitive` | bool | true | Case-sensitive search |
| `IncludeReferences` | bool | false | Include symbol references |
| `MaxResults` | int | 100 | Maximum results |
| `Kind` | DefinitionKind | any | Filter by definition kind |

## Security Considerations

1. **File Access**: The mapper respects workspace boundaries and won't parse files outside the configured root.

2. **Large Files**: Files exceeding `MaxFileSize` are skipped to prevent memory exhaustion.

3. **Symlinks**: Symlink following is disabled by default to prevent escaping workspace.

4. **Cache Security**: Cache files are stored with restricted permissions (0600).

5. **Ignore Patterns**: Always exclude sensitive directories like `.git`, `node_modules`, and credentials.

## Error Types

```go
var (
    ErrUnsupportedLanguage = errors.New("unsupported language")
    ErrFileTooLarge        = errors.New("file exceeds maximum size")
    ErrParseError          = errors.New("failed to parse file")
    ErrCacheCorrupted      = errors.New("cache data corrupted")
    ErrMaxDepthExceeded    = errors.New("maximum depth exceeded")
)
```

## Performance Tips

1. **Enable caching** for repeated mappings to avoid re-parsing unchanged files.

2. **Use parallel parsing** for large codebases to leverage multiple CPU cores.

3. **Configure ignore patterns** to skip irrelevant directories (node_modules, vendor, etc.).

4. **Set appropriate MaxDepth** to avoid traversing deeply nested directories.

5. **Limit languages** when you only need specific file types.
