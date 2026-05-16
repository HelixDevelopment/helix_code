// Package repomap provides semantic codebase mapping and symbol extraction for AI context optimization.
//
// The repomap package creates intelligent mappings of source code repositories,
// extracting symbols, relationships, and structural information to provide
// optimal context for AI-powered development workflows. It uses tree-sitter
// for accurate multi-language parsing and implements smart ranking algorithms
// to select the most relevant code context within token budgets.
//
// # Key Components
//
// RepoMap is the main entry point that coordinates symbol extraction and context building:
//
//	config := repomap.DefaultConfig()
//	config.TokenBudget = 8000
//	config.MaxFiles = 100
//
//	rm, err := repomap.NewRepoMap("/path/to/project", config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get optimal context for a query
//	contexts, err := rm.GetOptimalContext("implement user authentication", changedFiles)
//	for _, ctx := range contexts {
//	    fmt.Printf("File: %s (relevance: %.2f)\n", ctx.FilePath, ctx.Relevance)
//	    for _, sym := range ctx.Symbols {
//	        fmt.Printf("  - %s: %s\n", sym.Type, sym.Name)
//	    }
//	}
//
// # Symbol Extraction
//
// The TagExtractor component extracts symbols from source code using tree-sitter:
//
//	extractor := repomap.NewTagExtractor("go")
//	symbols := extractor.Extract(tree, filePath)
//
//	for _, symbol := range symbols {
//	    fmt.Printf("%s %s at line %d\n", symbol.Type, symbol.Name, symbol.LineStart)
//	}
//
// Supported symbol types include:
//   - Functions and methods
//   - Classes, structs, and interfaces
//   - Enums and traits
//   - Modules and constants
//
// # Supported Languages
//
// The package supports multiple programming languages through tree-sitter:
//   - Go
//   - Python
//   - JavaScript/TypeScript
//   - Java
//   - C/C++
//   - Rust
//   - Ruby
//
// # File Ranking
//
// The FileRanker scores files based on relevance to queries and recent changes:
//
//	ranker := repomap.NewFileRanker()
//	fileScores := ranker.RankFiles(allSymbols, query, changedFiles)
//
//	// Files are scored based on:
//	// - Symbol name matches
//	// - Documentation content
//	// - Recent modification
//	// - Import relationships
//
// # Caching
//
// The RepoCache provides efficient caching of parsed symbols:
//
//	cache, err := repomap.NewRepoCache(cacheDir, 24*time.Hour)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Cache entries are keyed by file path and modification time
//	cache.Set(cacheKey, symbols)
//	cachedSymbols, found := cache.Get(cacheKey)
//
// # Configuration
//
// RepoMapConfig controls mapping behavior:
//
//	config := repomap.RepoMapConfig{
//	    MaxFiles:           100,           // Maximum files to include
//	    TokenBudget:        8000,          // Token limit for context
//	    CacheEnabled:       true,          // Enable symbol caching
//	    LanguagesSupported: []string{"go", "python", "typescript"},
//	    CacheTTL:           24 * time.Hour,
//	    MaxConcurrency:     4,             // Parallel processing
//	}
//
// # Statistics
//
// The package provides statistics about repository mapping:
//
//	stats, err := rm.GetStatistics()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Files: %d, Symbols: %d, Cached: %d\n",
//	    stats.TotalFiles, stats.TotalSymbols, stats.CachedFiles)
//	fmt.Printf("Languages: %v\n", stats.Languages)
//
// # Thread Safety
//
// RepoMap operations are thread-safe through internal mutex protection,
// allowing concurrent access from multiple goroutines.
//
// # Performance Considerations
//
// For large repositories:
//   - Enable caching to avoid re-parsing unchanged files
//   - Tune MaxConcurrency based on available CPU cores
//   - Use RefreshCache selectively for frequently changing directories
//   - Adjust TokenBudget based on LLM context window size
package repomap
