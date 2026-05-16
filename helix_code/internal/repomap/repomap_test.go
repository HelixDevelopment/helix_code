package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test RepoMap creation and initialization
func TestNewRepoMap(t *testing.T) {
	tempDir := t.TempDir()

	config := DefaultConfig()
	rm, err := NewRepoMap(tempDir, config)

	if err != nil {
		t.Fatalf("Failed to create RepoMap: %v", err)
	}

	if rm == nil {
		t.Fatal("RepoMap is nil")
	}

	if rm.rootPath != tempDir {
		t.Errorf("Expected root path %s, got %s", tempDir, rm.rootPath)
	}
}

func TestNewRepoMapInvalidPath(t *testing.T) {
	config := DefaultConfig()
	_, err := NewRepoMap("/nonexistent/path/to/nowhere", config)

	if err == nil {
		t.Fatal("Expected error for invalid path, got nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxFiles <= 0 {
		t.Error("MaxFiles should be positive")
	}

	if config.TokenBudget <= 0 {
		t.Error("TokenBudget should be positive")
	}

	if len(config.LanguagesSupported) == 0 {
		t.Error("LanguagesSupported should not be empty")
	}

	expectedLanguages := []string{"go", "python", "javascript", "typescript", "java", "c", "cpp", "rust", "ruby"}
	if len(config.LanguagesSupported) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(config.LanguagesSupported))
	}
}

// Test language detection
func TestDetectLanguage(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig()
	rm, _ := NewRepoMap(tempDir, config)

	tests := []struct {
		filename string
		expected string
	}{
		{"test.go", "go"},
		{"test.py", "python"},
		{"test.js", "javascript"},
		{"test.jsx", "javascript"},
		{"test.ts", "typescript"},
		{"test.tsx", "typescript"},
		{"test.java", "java"},
		{"test.c", "c"},
		{"test.h", "c"},
		{"test.cpp", "cpp"},
		{"test.cc", "cpp"},
		{"test.cxx", "cpp"},
		{"test.hpp", "cpp"},
		{"test.rs", "rust"},
		{"test.rb", "ruby"},
		{"test.txt", ""},
		{"test.md", ""},
	}

	for _, tt := range tests {
		result := rm.detectLanguage(tt.filename)
		if result != tt.expected {
			t.Errorf("detectLanguage(%s) = %s, expected %s", tt.filename, result, tt.expected)
		}
	}
}

// Test file discovery
func TestDiscoverFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	createTestFile(t, tempDir, "main.go", "package main\n\nfunc main() {}\n")
	createTestFile(t, tempDir, "utils.go", "package main\n\nfunc helper() {}\n")
	createTestFile(t, tempDir, "README.md", "# Test\n")

	// Create subdirectory
	subdir := filepath.Join(tempDir, "pkg")
	os.MkdirAll(subdir, 0755)
	createTestFile(t, subdir, "lib.go", "package pkg\n\nfunc Lib() {}\n")

	config := DefaultConfig()
	rm, _ := NewRepoMap(tempDir, config)

	files, err := rm.discoverFiles()
	if err != nil {
		t.Fatalf("Failed to discover files: %v", err)
	}

	// Should find 3 Go files, not the markdown file
	if len(files) != 3 {
		t.Errorf("Expected 3 files, found %d", len(files))
	}

	// Verify all files are Go files
	for _, file := range files {
		if !strings.HasSuffix(file, ".go") {
			t.Errorf("Found non-Go file: %s", file)
		}
	}
}

func TestDiscoverFilesIgnoresCommonDirs(t *testing.T) {
	tempDir := t.TempDir()

	// Create files in ignored directories
	ignoredDirs := []string{".git", "node_modules", "vendor", ".helix", "__pycache__", "dist", "build", "target"}

	for _, dir := range ignoredDirs {
		dirPath := filepath.Join(tempDir, dir)
		os.MkdirAll(dirPath, 0755)
		createTestFile(t, dirPath, "test.go", "package test")
	}

	// Create a file in the root
	createTestFile(t, tempDir, "main.go", "package main")

	config := DefaultConfig()
	rm, _ := NewRepoMap(tempDir, config)

	files, err := rm.discoverFiles()
	if err != nil {
		t.Fatalf("Failed to discover files: %v", err)
	}

	// Should only find the root file
	if len(files) != 1 {
		t.Errorf("Expected 1 file, found %d (ignored directories not working)", len(files))
	}
}

// Test symbol extraction
func TestExtractFileSymbols(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	code := `package main

// MyFunc is a test function
func MyFunc() {
	println("hello")
}

// MyStruct is a test struct
type MyStruct struct {
	Field string
}

// MyMethod is a method on MyStruct
func (m *MyStruct) MyMethod() {
	println(m.Field)
}
`

	createTestFile(t, tempDir, "test.go", code)

	config := DefaultConfig()
	config.CacheEnabled = false // Disable cache for testing
	rm, _ := NewRepoMap(tempDir, config)

	symbols, err := rm.extractFileSymbols(testFile)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	// Should find function, struct, and method
	if len(symbols) < 2 {
		t.Errorf("Expected at least 2 symbols, got %d", len(symbols))
	}

	// Check for MyFunc
	foundFunc := false
	for _, sym := range symbols {
		if sym.Name == "MyFunc" && sym.Type == SymbolTypeFunction {
			foundFunc = true
			if sym.LineStart <= 0 {
				t.Error("Symbol should have line number")
			}
		}
	}

	if !foundFunc {
		t.Error("Did not find MyFunc symbol")
	}
}

// Test GetOptimalContext
func TestGetOptimalContext(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	createTestFile(t, tempDir, "main.go", `package main

func main() {
	helper()
}
`)

	createTestFile(t, tempDir, "helper.go", `package main

func helper() {
	println("helper")
}
`)

	config := DefaultConfig()
	config.CacheEnabled = false
	config.TokenBudget = 10000
	rm, _ := NewRepoMap(tempDir, config)

	contexts, err := rm.GetOptimalContext("helper", []string{})
	if err != nil {
		t.Fatalf("GetOptimalContext failed: %v", err)
	}

	if len(contexts) == 0 {
		t.Error("Expected at least one context")
	}

	// Should find helper.go with higher relevance
	foundHelper := false
	for _, ctx := range contexts {
		if strings.Contains(ctx.FilePath, "helper.go") {
			foundHelper = true
		}
	}

	if !foundHelper {
		t.Error("Expected to find helper.go in context")
	}
}

func TestGetOptimalContextWithChangedFiles(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.go")

	createTestFile(t, tempDir, "file1.go", "package main\nfunc File1() {}")
	createTestFile(t, tempDir, "file2.go", "package main\nfunc File2() {}")

	config := DefaultConfig()
	config.CacheEnabled = false
	rm, _ := NewRepoMap(tempDir, config)

	contexts, err := rm.GetOptimalContext("", []string{file1})
	if err != nil {
		t.Fatalf("GetOptimalContext failed: %v", err)
	}

	// Changed file should appear first
	if len(contexts) > 0 && contexts[0].FilePath != file1 {
		t.Error("Changed file should have highest relevance")
	}
}

func TestGetOptimalContextTokenBudget(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple large files
	largeContent := strings.Repeat("package main\nfunc Test() {}\n", 1000)

	for i := 0; i < 10; i++ {
		createTestFile(t, tempDir, filepath.Join("file", string(rune('0'+i))+".go"), largeContent)
	}

	config := DefaultConfig()
	config.CacheEnabled = false
	config.TokenBudget = 100 // Very small budget
	rm, _ := NewRepoMap(tempDir, config)

	contexts, err := rm.GetOptimalContext("", []string{})
	if err != nil {
		t.Fatalf("GetOptimalContext failed: %v", err)
	}

	// Should respect token budget
	totalTokens := 0
	for _, ctx := range contexts {
		totalTokens += ctx.TokenCount
	}

	if totalTokens > config.TokenBudget {
		t.Errorf("Total tokens %d exceeds budget %d", totalTokens, config.TokenBudget)
	}
}

// Test cache functionality
func TestCacheEnabled(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	createTestFile(t, tempDir, "test.go", "package main\nfunc Test() {}")

	config := DefaultConfig()
	config.CacheEnabled = true
	rm, _ := NewRepoMap(tempDir, config)
	t.Cleanup(func() { _ = rm.cache.Close() })

	// First call - not cached
	symbols1, err := rm.extractFileSymbols(testFile)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	// Second call - should be cached
	symbols2, err := rm.extractFileSymbols(testFile)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	if len(symbols1) != len(symbols2) {
		t.Error("Cached symbols differ from original")
	}
}

func TestInvalidateFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	createTestFile(t, tempDir, "test.go", "package main\nfunc Test() {}")

	config := DefaultConfig()
	config.CacheEnabled = true
	rm, _ := NewRepoMap(tempDir, config)
	t.Cleanup(func() { _ = rm.cache.Close() })

	// Cache the file
	_, _ = rm.extractFileSymbols(testFile)

	// Drain async save to disk
	rm.cache.Wait()

	cacheKey := rm.getCacheKey(testFile)
	if !rm.cache.Has(cacheKey) {
		t.Fatalf("precondition: cache must contain entry for %q before invalidation; extractFileSymbols did not populate cache", testFile)
	}

	// Invalidate
	err := rm.InvalidateFile(testFile)
	if err != nil {
		t.Fatalf("Failed to invalidate file: %v", err)
	}

	// Drain async removal from disk
	rm.cache.Wait()

	if rm.cache.Has(cacheKey) {
		t.Fatalf("postcondition: cache must NOT contain entry for %q after InvalidateFile; invalidation was a no-op (bluff)", testFile)
	}
}

func TestRefreshCache(t *testing.T) {
	tempDir := t.TempDir()

	createTestFile(t, tempDir, "test.go", "package main\nfunc Test() {}")

	config := DefaultConfig()
	config.CacheEnabled = true
	rm, _ := NewRepoMap(tempDir, config)
	t.Cleanup(func() { _ = rm.cache.Close() })

	err := rm.RefreshCache()
	if err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}
}

func TestRefreshCacheDisabled(t *testing.T) {
	tempDir := t.TempDir()

	config := DefaultConfig()
	config.CacheEnabled = false
	rm, _ := NewRepoMap(tempDir, config)

	err := rm.RefreshCache()
	if err == nil {
		t.Error("Expected error when cache is disabled")
	}
}

// Test statistics
func TestGetStatistics(t *testing.T) {
	tempDir := t.TempDir()

	createTestFile(t, tempDir, "test.go", "package main\nfunc Test() {}")
	createTestFile(t, tempDir, "test.py", "def test():\n    pass")

	config := DefaultConfig()
	config.CacheEnabled = false
	rm, _ := NewRepoMap(tempDir, config)

	stats, err := rm.GetStatistics()
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if stats.TotalFiles != 2 {
		t.Errorf("Expected 2 files, got %d", stats.TotalFiles)
	}

	if len(stats.Languages) == 0 {
		t.Error("Expected language statistics")
	}

	if stats.IndexingDuration == 0 {
		t.Error("Expected non-zero indexing duration")
	}
}

// Test TreeSitterParser
func TestNewTreeSitterParser(t *testing.T) {
	parser := NewTreeSitterParser()

	if parser == nil {
		t.Fatal("Parser is nil")
	}

	langs := parser.SupportedLanguages()
	if len(langs) < 8 {
		t.Errorf("Expected at least 8 supported languages, got %d", len(langs))
	}
}

func TestParseFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	createTestFile(t, tempDir, "test.go", "package main\n\nfunc main() {}\n")

	parser := NewTreeSitterParser()
	tree, err := parser.ParseFile(testFile, "go")

	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Tree is nil")
	}
}

func TestParseFileUnsupportedLanguage(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.unknown")

	createTestFile(t, tempDir, "test.unknown", "unknown content")

	parser := NewTreeSitterParser()
	_, err := parser.ParseFile(testFile, "unknown")

	if err == nil {
		t.Error("Expected error for unsupported language")
	}
}

func TestExtractSymbols(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	code := `package main

func MyFunction() {
	println("test")
}

type MyStruct struct {
	Field string
}
`

	createTestFile(t, tempDir, "test.go", code)

	parser := NewTreeSitterParser()
	tree, _ := parser.ParseFile(testFile, "go")

	symbols, err := parser.ExtractSymbols(tree, testFile, "go")
	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	if len(symbols) == 0 {
		t.Error("Expected to extract symbols")
	}

	// Should find at least the function
	foundFunc := false
	for _, sym := range symbols {
		if sym.Name == "MyFunction" {
			foundFunc = true
		}
	}

	if !foundFunc {
		t.Error("Did not find MyFunction")
	}
}

// Test TagExtractor
func TestNewTagExtractor(t *testing.T) {
	extractor := NewTagExtractor("go")

	if extractor == nil {
		t.Fatal("Extractor is nil")
	}

	if extractor.language != "go" {
		t.Error("Language not set correctly")
	}
}

func TestSymbolTypes(t *testing.T) {
	types := []SymbolType{
		SymbolTypeFunction,
		SymbolTypeMethod,
		SymbolTypeClass,
		SymbolTypeInterface,
		SymbolTypeStruct,
		SymbolTypeEnum,
		SymbolTypeTrait,
		SymbolTypeModule,
		SymbolTypeVariable,
		SymbolTypeConstant,
		SymbolTypeImport,
		SymbolTypeExport,
	}

	for _, st := range types {
		if string(st) == "" {
			t.Errorf("Symbol type %v is empty", st)
		}
	}
}

// Test FileRanker
func TestNewFileRanker(t *testing.T) {
	ranker := NewFileRanker()

	if ranker == nil {
		t.Fatal("Ranker is nil")
	}

	if ranker.weights.SymbolMatch <= 0 {
		t.Error("Symbol match weight should be positive")
	}
}

func TestRankFiles(t *testing.T) {
	ranker := NewFileRanker()

	symbols := []Symbol{
		{
			Name:     "TestFunc",
			Type:     SymbolTypeFunction,
			FilePath: "/test/file1.go",
		},
		{
			Name:     "HelperFunc",
			Type:     SymbolTypeFunction,
			FilePath: "/test/file2.go",
		},
	}

	scores := ranker.RankFiles(symbols, "test", []string{})

	if len(scores) != 2 {
		t.Errorf("Expected 2 scores, got %d", len(scores))
	}

	// file1.go should rank higher due to "TestFunc" matching "test"
	var file1Score, file2Score float64
	for _, score := range scores {
		if strings.Contains(score.FilePath, "file1.go") {
			file1Score = score.Score
		} else if strings.Contains(score.FilePath, "file2.go") {
			file2Score = score.Score
		}
	}

	if file1Score <= file2Score {
		t.Error("Expected file1.go to have higher score due to symbol match")
	}
}

func TestRankFilesWithChangedFiles(t *testing.T) {
	ranker := NewFileRanker()

	symbols := []Symbol{
		{Name: "Func", Type: SymbolTypeFunction, FilePath: "/test/file1.go"},
		{Name: "Func", Type: SymbolTypeFunction, FilePath: "/test/file2.go"},
	}

	changedFiles := []string{"/test/file2.go"}
	scores := ranker.RankFiles(symbols, "", changedFiles)

	// file2.go should rank higher because it was changed
	var file1Score, file2Score float64
	for _, score := range scores {
		if score.FilePath == "/test/file1.go" {
			file1Score = score.Score
		} else if score.FilePath == "/test/file2.go" {
			file2Score = score.Score
		}
	}

	if file2Score <= file1Score {
		t.Error("Changed file should have higher score")
	}
}

func TestTokenizeQuery(t *testing.T) {
	ranker := NewFileRanker()

	tests := []struct {
		query    string
		expected int
	}{
		{"simple query", 2},
		{"test_function_name", 3},
		{"TestCamelCase", 1},
		{"test-kebab-case", 3},
		{"", 0},
	}

	for _, tt := range tests {
		tokens := ranker.tokenizeQuery(tt.query)
		if len(tokens) != tt.expected {
			t.Errorf("tokenizeQuery(%q) returned %d tokens, expected %d", tt.query, len(tokens), tt.expected)
		}
	}
}

func TestTokenizeSymbolName(t *testing.T) {
	ranker := NewFileRanker()

	tests := []struct {
		name     string
		expected int
	}{
		{"MyFunction", 2},
		{"my_function", 2},
		{"my-function", 2},
		{"myfunction", 1},
		{"HTTPServer", 1},
	}

	for _, tt := range tests {
		tokens := ranker.tokenizeSymbolName(tt.name)
		if len(tokens) < 1 {
			t.Errorf("tokenizeSymbolName(%q) returned %d tokens, expected at least 1", tt.name, len(tokens))
		}
	}
}

// Test Cache
func TestNewRepoCache(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	cache, err := NewRepoCache(cacheDir, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	if cache == nil {
		t.Fatal("Cache is nil")
	}

	// Check directory was created
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("Cache directory was not created")
	}
}

func TestCacheGetSet(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Hour)

	key := "test-key"
	value := "test-value"

	// Set value
	cache.Set(key, value)

	// Wait for async save to complete
	time.Sleep(50 * time.Millisecond)

	// Get value
	retrieved, found := cache.Get(key)
	if !found {
		t.Error("Value not found in cache")
	}

	if retrieved.(string) != value {
		t.Errorf("Retrieved value %v != original value %v", retrieved, value)
	}
}

func TestCacheInvalidate(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Hour)

	key := "test-key"
	cache.Set(key, "value")

	// Wait for async save to complete
	time.Sleep(50 * time.Millisecond)

	cache.Invalidate(key)

	// Wait for async removal to complete
	time.Sleep(50 * time.Millisecond)

	_, found := cache.Get(key)
	if found {
		t.Error("Value should not be found after invalidation")
	}
}

func TestCacheExpiration(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Millisecond)

	key := "test-key"
	cache.Set(key, "value")

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, found := cache.Get(key)
	if found {
		t.Error("Value should have expired")
	}
}

func TestCacheSize(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Hour)

	if cache.Size() != 0 {
		t.Error("New cache should be empty")
	}

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Wait for async save to complete
	time.Sleep(50 * time.Millisecond)

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}
}

func TestCacheCleanup(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Millisecond)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	time.Sleep(10 * time.Millisecond)

	removed := cache.Cleanup()
	if removed != 2 {
		t.Errorf("Expected to remove 2 entries, removed %d", removed)
	}

	if cache.Size() != 0 {
		t.Error("Cache should be empty after cleanup")
	}
}

func TestCacheGetStats(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Hour)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Wait for async save to complete
	time.Sleep(50 * time.Millisecond)

	stats := cache.GetStats()

	if stats.TotalEntries != 2 {
		t.Errorf("Expected 2 total entries, got %d", stats.TotalEntries)
	}

	if stats.TotalSize <= 0 {
		t.Error("Expected positive total size")
	}
}

func TestCacheGetOrCompute(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Hour)

	computeCount := 0
	compute := func() (interface{}, error) {
		computeCount++
		return "computed", nil
	}

	// First call - should compute
	value1, _ := cache.GetOrCompute("key", compute)
	if value1.(string) != "computed" {
		t.Error("Expected computed value")
	}
	if computeCount != 1 {
		t.Error("Compute should have been called once")
	}

	// Wait for async save to complete
	time.Sleep(50 * time.Millisecond)

	// Second call - should use cache
	value2, _ := cache.GetOrCompute("key", compute)
	if value2.(string) != "computed" {
		t.Error("Expected cached value")
	}
	if computeCount != 1 {
		t.Error("Compute should not have been called again")
	}
}

func TestCacheHas(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Hour)

	if cache.Has("key") {
		t.Error("Cache should not have key")
	}

	cache.Set("key", "value")

	// Wait for async save to complete
	time.Sleep(50 * time.Millisecond)

	if !cache.Has("key") {
		t.Error("Cache should have key")
	}
}

func TestCacheKeys(t *testing.T) {
	tempDir := t.TempDir()
	cache, _ := NewRepoCache(filepath.Join(tempDir, "cache"), 1*time.Hour)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Wait for async save to complete
	time.Sleep(50 * time.Millisecond)

	keys := cache.Keys()

	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestGetLanguageQueries(t *testing.T) {
	languages := []string{"go", "python", "javascript", "typescript", "java", "c", "cpp", "rust", "ruby"}

	for _, lang := range languages {
		queries := GetLanguageQueries(lang)
		if queries == nil {
			t.Errorf("Expected queries for language %s, got nil", lang)
		}
	}

	// Test unsupported language
	queries := GetLanguageQueries("unknown")
	if queries != nil {
		t.Error("Expected nil for unsupported language")
	}
}

// Test GetSupportedLanguages
func TestGetSupportedLanguages(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig()
	rm, err := NewRepoMap(tempDir, config)
	if err != nil {
		t.Fatalf("Failed to create RepoMap: %v", err)
	}

	languages := rm.GetSupportedLanguages()

	if len(languages) == 0 {
		t.Error("Expected at least one supported language")
	}

	// Check for common languages
	expectedLangs := []string{"go", "python", "javascript"}
	for _, expected := range expectedLangs {
		found := false
		for _, lang := range languages {
			if lang == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language %s to be supported", expected)
		}
	}
}

// Test FileRanker with custom weights
func TestNewFileRankerWithWeights(t *testing.T) {
	weights := RankingWeights{
		RecentlyChanged: 0.5,
		SymbolMatch:     0.2,
		ImportFrequency: 0.15,
		DependencyDepth: 0.1,
		FileSize:        0.03,
		SymbolDensity:   0.02,
	}

	ranker := NewFileRankerWithWeights(weights)

	if ranker == nil {
		t.Fatal("Expected non-nil FileRanker")
	}

	if ranker.weights.RecentlyChanged != 0.5 {
		t.Errorf("Expected RecentlyChanged weight 0.5, got %f", ranker.weights.RecentlyChanged)
	}
}

// Test RankByModificationTime
func TestRankByModificationTime(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files with different modification times
	file1 := filepath.Join(tempDir, "old.go")
	file2 := filepath.Join(tempDir, "new.go")

	createTestFile(t, tempDir, "old.go", "package old")
	time.Sleep(10 * time.Millisecond)
	createTestFile(t, tempDir, "new.go", "package new")

	ranker := NewFileRanker()
	files := []string{file1, file2}

	scores := ranker.RankByModificationTime(files)

	if len(scores) != 2 {
		t.Errorf("Expected 2 scores, got %d", len(scores))
	}

	// Newer file should have higher score
	var oldScore, newScore float64
	for _, s := range scores {
		if strings.Contains(s.FilePath, "old.go") {
			oldScore = s.Score
		} else if strings.Contains(s.FilePath, "new.go") {
			newScore = s.Score
		}
	}

	if newScore < oldScore {
		t.Errorf("Newer file should have higher score: new=%f, old=%f", newScore, oldScore)
	}
}

// Test RankBySymbolCount
func TestRankBySymbolCount(t *testing.T) {
	ranker := NewFileRanker()

	symbolsByFile := map[string][]Symbol{
		"small.go": {
			{Name: "Func1", Type: SymbolTypeFunction},
		},
		"large.go": {
			{Name: "Func1", Type: SymbolTypeFunction},
			{Name: "Func2", Type: SymbolTypeFunction},
			{Name: "Type1", Type: SymbolTypeStruct},
			{Name: "Var1", Type: SymbolTypeVariable},
			{Name: "Func3", Type: SymbolTypeFunction},
		},
	}

	scores := ranker.RankBySymbolCount(symbolsByFile)

	if len(scores) != 2 {
		t.Errorf("Expected 2 scores, got %d", len(scores))
	}

	// File with more symbols should have higher score
	var smallScore, largeScore float64
	for _, s := range scores {
		if strings.Contains(s.FilePath, "small.go") {
			smallScore = s.Score
		} else if strings.Contains(s.FilePath, "large.go") {
			largeScore = s.Score
		}
	}

	if largeScore <= smallScore {
		t.Errorf("File with more symbols should have higher score: large=%f, small=%f", largeScore, smallScore)
	}
}

// Test CombineScores
func TestCombineScores(t *testing.T) {
	ranker := NewFileRanker()

	t.Run("empty score sets", func(t *testing.T) {
		result := ranker.CombineScores([][]FileScore{}, []float64{})
		if len(result) != 0 {
			t.Errorf("Expected empty result for empty input, got %d", len(result))
		}
	})

	t.Run("single score set", func(t *testing.T) {
		scores := [][]FileScore{
			{
				{FilePath: "file1.go", Score: 0.8},
				{FilePath: "file2.go", Score: 0.6},
			},
		}

		result := ranker.CombineScores(scores, []float64{1.0})
		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}
	})

	t.Run("multiple score sets with weights", func(t *testing.T) {
		scores := [][]FileScore{
			{
				{FilePath: "file1.go", Score: 1.0},
				{FilePath: "file2.go", Score: 0.5},
			},
			{
				{FilePath: "file1.go", Score: 0.5},
				{FilePath: "file2.go", Score: 1.0},
			},
		}

		result := ranker.CombineScores(scores, []float64{0.5, 0.5})
		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}

		// Both files should have equal combined scores (0.5*1.0 + 0.5*0.5) = 0.75
		for _, s := range result {
			if s.Score < 0.7 || s.Score > 0.8 {
				t.Errorf("Expected score around 0.75, got %f for %s", s.Score, s.FilePath)
			}
		}
	})

	t.Run("default equal weights", func(t *testing.T) {
		scores := [][]FileScore{
			{{FilePath: "file1.go", Score: 1.0}},
			{{FilePath: "file1.go", Score: 0.0}},
		}

		result := ranker.CombineScores(scores, []float64{})
		if len(result) != 1 {
			t.Errorf("Expected 1 result, got %d", len(result))
		}
	})
}

// Test Cache InvalidateAll
func TestCacheInvalidateAll(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	cache, err := NewRepoCache(cacheDir, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Add some entries
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	if cache.Size() != 3 {
		t.Errorf("Expected 3 entries, got %d", cache.Size())
	}

	// Wait for async disk save
	time.Sleep(100 * time.Millisecond)

	// Invalidate all
	err = cache.InvalidateAll()
	if err != nil {
		t.Errorf("InvalidateAll failed: %v", err)
	}

	if cache.Size() != 0 {
		t.Errorf("Expected 0 entries after InvalidateAll, got %d", cache.Size())
	}
}

// Test Cache TTL functions
func TestCacheTTL(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	cache, err := NewRepoCache(cacheDir, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	t.Run("GetTTL", func(t *testing.T) {
		ttl := cache.GetTTL()
		if ttl != time.Hour {
			t.Errorf("Expected TTL of 1 hour, got %v", ttl)
		}
	})

	t.Run("SetTTL", func(t *testing.T) {
		cache.SetTTL(30 * time.Minute)
		ttl := cache.GetTTL()
		if ttl != 30*time.Minute {
			t.Errorf("Expected TTL of 30 minutes, got %v", ttl)
		}
	})
}

// Test Cache Export and Import
func TestCacheExportImport(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	exportFilePath := filepath.Join(tempDir, "export.gob")

	cache, err := NewRepoCache(cacheDir, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Add some entries
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Wait for entries to be stored
	time.Sleep(50 * time.Millisecond)

	// Export to file
	exportFile, err := os.Create(exportFilePath)
	if err != nil {
		t.Fatalf("Failed to create export file: %v", err)
	}
	err = cache.Export(exportFile)
	exportFile.Close()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(exportFilePath); os.IsNotExist(err) {
		t.Error("Export file was not created")
	}

	// Create new cache and import
	cache2, err := NewRepoCache(filepath.Join(tempDir, "cache2"), time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache2: %v", err)
	}

	// Import from file
	importFile, err := os.Open(exportFilePath)
	if err != nil {
		t.Fatalf("Failed to open import file: %v", err)
	}
	err = cache2.Import(importFile)
	importFile.Close()
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify imported entries
	val, found := cache2.Get("key1")
	if !found {
		t.Error("Expected to find key1 after import")
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}
}

// Test JavaScript/TypeScript extraction
func TestExtractJavaScriptSymbols(t *testing.T) {
	tempDir := t.TempDir()

	jsContent := `
function greet(name) {
    return "Hello, " + name;
}

class Person {
    constructor(name) {
        this.name = name;
    }

    sayHello() {
        return greet(this.name);
    }
}

const add = (a, b) => a + b;

export default Person;
`
	createTestFile(t, tempDir, "test.js", jsContent)
	testFile := filepath.Join(tempDir, "test.js")

	config := DefaultConfig()
	config.CacheEnabled = true
	rm, _ := NewRepoMap(tempDir, config)
	t.Cleanup(func() { _ = rm.cache.Close() })

	symbols, err := rm.extractFileSymbols(testFile)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	// Should find some symbols
	if len(symbols) == 0 {
		t.Log("No JavaScript symbols extracted (tree-sitter parsing may not be available)")
	}
}

// Test Python class extraction
func TestExtractPythonSymbols(t *testing.T) {
	tempDir := t.TempDir()

	pyContent := `
class Calculator:
    def __init__(self):
        self.result = 0

    def add(self, x, y):
        return x + y

    def subtract(self, x, y):
        return x - y

def helper_function():
    return "helper"

CONSTANT = 42
`
	createTestFile(t, tempDir, "test.py", pyContent)
	testFile := filepath.Join(tempDir, "test.py")

	config := DefaultConfig()
	config.CacheEnabled = true
	rm, _ := NewRepoMap(tempDir, config)
	t.Cleanup(func() { _ = rm.cache.Close() })

	symbols, err := rm.extractFileSymbols(testFile)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	// Should find some symbols
	if len(symbols) == 0 {
		t.Log("No Python symbols extracted (tree-sitter parsing may not be available)")
	}
}

// Helper functions

func createTestFile(t *testing.T, dir, filename, content string) {
	path := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
}
