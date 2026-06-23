package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1: WebTools creation with default config
func TestNewWebTools_DefaultConfig(t *testing.T) {
	wt, err := NewWebTools(nil)
	require.NoError(t, err)
	assert.NotNil(t, wt)
	assert.NotNil(t, wt.config)
	assert.NotNil(t, wt.httpClient)
	assert.NotNil(t, wt.searchEngine)
	assert.NotNil(t, wt.fetcher)
	assert.NotNil(t, wt.parser)

	defer wt.Close()
}

// Test 2: WebTools creation with custom config
func TestNewWebTools_CustomConfig(t *testing.T) {
	config := &Config{
		FetchTimeout:   10 * time.Second,
		MaxContentSize: 5 * 1024 * 1024,
		CacheEnabled:   false,
	}

	wt, err := NewWebTools(config)
	require.NoError(t, err)
	assert.NotNil(t, wt)
	assert.Equal(t, 10*time.Second, wt.config.FetchTimeout)
	assert.Equal(t, int64(5*1024*1024), wt.config.MaxContentSize)

	defer wt.Close()
}

// Test 3: HTTP fetch with mock server - success
func TestFetcher_Fetch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body><h1>Test Page</h1><p>Test content</p></body></html>"))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.CacheEnabled = false
	config.AllowPrivateIPs = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	result, err := wt.Fetch(context.Background(), server.URL, FetchOptions{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.Status)
	assert.Contains(t, string(result.Content), "Test Page")
	assert.Equal(t, "text/html", result.ContentType)
}

// Test 4: HTTP fetch - 404 error
func TestFetcher_Fetch_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.CacheEnabled = false
	config.AllowPrivateIPs = true
	config.AllowPrivateIPs = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	_, err = wt.Fetch(context.Background(), server.URL, FetchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// Test 5: HTTP fetch - redirect handling
func TestFetcher_Fetch_Redirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Final destination"))
	}))
	defer target.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.CacheEnabled = false
	config.AllowPrivateIPs = true
	config.FollowRedirects = false
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	result, err := wt.Fetch(context.Background(), server.URL, FetchOptions{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, result.Status)
	assert.NotEmpty(t, result.Redirect)
}

// Test 6: HTTP fetch - timeout
func TestFetcher_Fetch_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("Slow response"))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.FetchTimeout = 100 * time.Millisecond
	config.CacheEnabled = false
	config.AllowPrivateIPs = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	_, err = wt.Fetch(context.Background(), server.URL, FetchOptions{})
	assert.Error(t, err)
}

// Test 7: HTML to markdown conversion
func TestParser_Parse_HTMLToMarkdown(t *testing.T) {
	config := DefaultConfig()
	parser := NewParser(config)

	html := []byte(`
		<html>
		<head>
			<title>Test Page</title>
			<meta name="description" content="Test description">
		</head>
		<body>
			<h1>Hello World</h1>
			<p>This is a <strong>test</strong> paragraph.</p>
			<ul>
				<li>Item 1</li>
				<li>Item 2</li>
			</ul>
		</body>
		</html>
	`)

	parsed, err := parser.Parse(html, "https://example.com")
	require.NoError(t, err)
	assert.Contains(t, parsed.Markdown, "# Hello World")
	assert.Contains(t, parsed.Markdown, "**test**")
	assert.Contains(t, parsed.Markdown, "- Item 1")
	assert.Equal(t, "Test Page", parsed.Metadata.Title)
	assert.Equal(t, "Test description", parsed.Metadata.Description)
}

// Test 8: Metadata extraction
func TestParser_ExtractMetadata(t *testing.T) {
	config := DefaultConfig()
	config.ExtractMetadata = true
	parser := NewParser(config)

	html := []byte(`
		<html lang="en">
		<head>
			<title>Test Title</title>
			<meta name="description" content="Test Description">
			<meta name="author" content="Test Author">
			<meta name="keywords" content="test, keywords, example">
			<meta property="og:image" content="https://example.com/image.jpg">
		</head>
		<body>Content</body>
		</html>
	`)

	parsed, err := parser.Parse(html, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, "Test Title", parsed.Metadata.Title)
	assert.Equal(t, "Test Description", parsed.Metadata.Description)
	assert.Equal(t, "Test Author", parsed.Metadata.Author)
	assert.Equal(t, "https://example.com/image.jpg", parsed.Metadata.Image)
	assert.Equal(t, "en", parsed.Metadata.Language)
	assert.Len(t, parsed.Metadata.Keywords, 3)
}

// Test 9: Cache hit and miss
func TestCacheManager_HitAndMiss(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)

	// Cache miss
	_, ok := cm.Get("test-key")
	assert.False(t, ok)
	assert.Equal(t, int64(1), cm.stats.Misses.Load())

	// Set value
	testData := []byte("test data")
	cm.Set("test-key", testData)

	// Cache hit
	data, ok := cm.Get("test-key")
	assert.True(t, ok)
	assert.Equal(t, testData, data)
	assert.Equal(t, int64(1), cm.stats.Hits.Load())

	// Clean up
	cm.Close()
}

// Test 10: Cache expiration
func TestCacheManager_Expiration(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCacheManager(tmpDir, 100*time.Millisecond, 100*1024*1024)

	// Set value
	cm.Set("test-key", []byte("test data"))

	// Should exist
	_, ok := cm.Get("test-key")
	assert.True(t, ok)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, ok = cm.Get("test-key")
	assert.False(t, ok)

	// Clean up
	cm.Close()
}

// Test 11: Rate limiter - basic functionality
func TestRateLimiter_Basic(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("test", RateLimit{
		RequestsPerSecond: 10,
		Burst:             1,
	})

	ctx := context.Background()

	// First request should succeed immediately
	err := rl.Wait(ctx, "test")
	assert.NoError(t, err)

	// Second request should wait
	start := time.Now()
	err = rl.Wait(ctx, "test")
	assert.NoError(t, err)
	duration := time.Since(start)
	assert.Greater(t, duration, 50*time.Millisecond)
}

// Test 12: Rate limiter - allow check
func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetLimit("test", RateLimit{
		RequestsPerSecond: 10,
		Burst:             2,
	})

	// First two should be allowed
	assert.True(t, rl.Allow("test"))
	assert.True(t, rl.Allow("test"))

	// Third should be denied
	assert.False(t, rl.Allow("test"))
}

// Test 13: URL validation - valid URLs
func TestFetcher_ValidateURL_Valid(t *testing.T) {
	config := DefaultConfig()
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	validURLs := []string{
		"https://example.com",
		"http://example.com",
		"https://example.com/path",
		"https://example.com:8080/path",
	}

	for _, url := range validURLs {
		err := wt.fetcher.validateURL(url)
		assert.NoError(t, err, "URL should be valid: %s", url)
	}
}

// Test 14: URL validation - invalid URLs
func TestFetcher_ValidateURL_Invalid(t *testing.T) {
	config := DefaultConfig()
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	invalidURLs := []string{
		"ftp://example.com", // Invalid scheme
		"https://",          // Missing host
		"not-a-url",         // Invalid URL
		"https://localhost", // Private IP (when disabled)
		"https://127.0.0.1", // Loopback
	}

	for _, url := range invalidURLs {
		err := wt.fetcher.validateURL(url)
		assert.Error(t, err, "URL should be invalid: %s", url)
	}
}

// Test 15: Fetch and parse integration
func TestWebTools_FetchAndParse_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		html := `
			<html>
			<head>
				<title>Integration Test</title>
				<meta name="description" content="Integration test page">
			</head>
			<body>
				<h1>Integration Test</h1>
				<p>This is an <strong>integration</strong> test.</p>
			</body>
			</html>
		`
		w.Write([]byte(html))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.CacheEnabled = false
	config.AllowPrivateIPs = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	markdown, metadata, err := wt.FetchAndParse(context.Background(), server.URL)
	require.NoError(t, err)
	assert.Contains(t, markdown, "# Integration Test")
	assert.Contains(t, markdown, "**integration**")
	assert.Equal(t, "Integration Test", metadata.Title)
	assert.Equal(t, "Integration test page", metadata.Description)
}

// Test 16: Caching with fetch
func TestWebTools_Fetch_WithCache(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("<html><body>Request %d</body></html>", requestCount)))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.CacheEnabled = true
	config.CacheDir = tmpDir
	config.CacheTTL = 1 * time.Minute
	config.AllowPrivateIPs = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	// First fetch - should hit server
	result1, err := wt.Fetch(context.Background(), server.URL, FetchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, requestCount)
	assert.Contains(t, string(result1.Content), "Request 1")

	// Second fetch - should hit cache
	result2, err := wt.Fetch(context.Background(), server.URL, FetchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, requestCount) // Server not hit again
	assert.Contains(t, string(result2.Content), "Request 1")
}

// Test 17: Remove scripts and styles
func TestParser_RemoveScriptsAndStyles(t *testing.T) {
	config := DefaultConfig()
	config.RemoveScripts = true
	config.RemoveStyles = true
	parser := NewParser(config)

	html := []byte(`
		<html>
		<head>
			<style>body { color: red; }</style>
		</head>
		<body>
			<p>Visible content</p>
			<script>alert('test');</script>
		</body>
		</html>
	`)

	parsed, err := parser.Parse(html, "https://example.com")
	require.NoError(t, err)
	assert.Contains(t, parsed.Markdown, "Visible content")
	assert.NotContains(t, parsed.Markdown, "alert")
	assert.NotContains(t, parsed.Markdown, "color: red")
}

// Test 18: Multiple concurrent fetches
func TestFetcher_FetchMultiple(t *testing.T) {
	servers := make([]*httptest.Server, 3)
	for i := range servers {
		index := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("Response %d", index)))
		}))
		defer servers[i].Close()
	}

	config := DefaultConfig()
	config.CacheEnabled = false
	config.AllowPrivateIPs = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	urls := make([]string, len(servers))
	for i, s := range servers {
		urls[i] = s.URL
	}

	// Establish a SEQUENTIAL baseline on the SAME servers/host so the
	// invariant is a RATIO (concurrent vs sequential), not an absolute
	// wall-clock cap. Each server sleeps ~10ms, so sequential ≈ N*10ms while
	// concurrent ≈ max(individual) ≈ 10ms. A ratio is immune to host speed,
	// GC pauses, scheduler jitter, and -race overhead.
	seqStart := time.Now()
	for _, u := range urls {
		_, err := wt.fetcher.Fetch(context.Background(), u, FetchOptions{})
		require.NoError(t, err)
	}
	sequentialDuration := time.Since(seqStart)

	concStart := time.Now()
	results, err := wt.fetcher.FetchMultiple(context.Background(), urls, FetchOptions{})
	concurrentDuration := time.Since(concStart)

	require.NoError(t, err)
	assert.Len(t, results, 3)

	// RELATIVE INVARIANT: concurrent fetch of N URLs must be meaningfully
	// faster than fetching them one-by-one. The real property of
	// FetchMultiple is concurrency, which means total time tracks
	// max(individual) rather than sum(individual). We require concurrent to
	// be at most 70% of sequential — a generous margin that still proves
	// genuine parallelism (a serial regression would land at ~100%) without
	// flaking when both runs are slowed equally by host load.
	assert.Less(t, concurrentDuration, sequentialDuration,
		"concurrent FetchMultiple was not faster than sequential fetching — concurrency appears broken")
	assert.Less(t, concurrentDuration, (sequentialDuration*7)/10,
		"concurrent FetchMultiple (%v) was not meaningfully faster than sequential (%v) — expected it to track max(individual), not sum",
		concurrentDuration, sequentialDuration)

	for i, result := range results {
		assert.Contains(t, string(result.Content), fmt.Sprintf("Response %d", i))
	}
}

// Test 19: Parser links and images
func TestParser_LinksAndImages(t *testing.T) {
	config := DefaultConfig()
	parser := NewParser(config)

	html := []byte(`
		<html>
		<body>
			<p>Check out <a href="https://example.com">this link</a></p>
			<img src="https://example.com/image.jpg" alt="Test Image">
		</body>
		</html>
	`)

	parsed, err := parser.Parse(html, "https://example.com")
	require.NoError(t, err)
	assert.Contains(t, parsed.Markdown, "[this link](https://example.com)")
	assert.Contains(t, parsed.Markdown, "![Test Image](https://example.com/image.jpg)")
}

// Test 20: Cache clear functionality
func TestCacheManager_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)

	// Set multiple values
	cm.Set("key1", []byte("value1"))
	cm.Set("key2", []byte("value2"))
	cm.Set("key3", []byte("value3"))

	// Verify they exist
	_, ok := cm.Get("key1")
	assert.True(t, ok)

	// Clear cache
	err := cm.Clear()
	require.NoError(t, err)

	// Verify they're gone
	_, ok = cm.Get("key1")
	assert.False(t, ok)
	_, ok = cm.Get("key2")
	assert.False(t, ok)

	cm.Close()
}

// Test 21: Disk cache persistence
func TestDiskCache_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	cm1 := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)

	// Set value
	cm1.Set("persistent-key", []byte("persistent-value"))
	cm1.Close()

	// Create new cache manager with same directory
	cm2 := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)

	// Value should still be there
	data, ok := cm2.Get("persistent-key")
	assert.True(t, ok)
	assert.Equal(t, []byte("persistent-value"), data)

	cm2.Close()
}

// Test 22: Empty search query error
func TestWebTools_Search_EmptyQuery(t *testing.T) {
	config := DefaultConfig()
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	_, err = wt.Search(context.Background(), "", SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// Test 23: Empty URL error
func TestWebTools_Fetch_EmptyURL(t *testing.T) {
	config := DefaultConfig()
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	_, err = wt.Fetch(context.Background(), "", FetchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// Test 24: Extract plain text
func TestParser_ExtractText(t *testing.T) {
	config := DefaultConfig()
	parser := NewParser(config)

	html := []byte(`
		<html>
		<head><title>Test</title></head>
		<body>
			<h1>Header</h1>
			<p>Paragraph text</p>
			<script>alert('test');</script>
		</body>
		</html>
	`)

	text, err := parser.ExtractText(html)
	require.NoError(t, err)
	assert.Contains(t, text, "Test")
	assert.Contains(t, text, "Header")
	assert.Contains(t, text, "Paragraph text")
	assert.Contains(t, text, "alert") // ExtractText doesn't remove scripts
}

// Test 25: Cache stats
func TestCacheManager_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)

	// Initial stats
	stats := cm.GetStats()
	assert.NotNil(t, stats)

	// Cause some misses
	cm.Get("nonexistent1")
	cm.Get("nonexistent2")
	assert.Equal(t, int64(2), stats.Misses.Load())

	// Set and get
	cm.Set("key1", []byte("value1"))
	cm.Get("key1")
	assert.Equal(t, int64(1), stats.Hits.Load())

	cm.Close()
}

// Benchmark tests
func BenchmarkFetch(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>Benchmark</body></html>"))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.CacheEnabled = false
	config.AllowPrivateIPs = true
	wt, _ := NewWebTools(config)
	defer wt.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wt.Fetch(context.Background(), server.URL, FetchOptions{})
	}
}

func BenchmarkParse(b *testing.B) {
	config := DefaultConfig()
	parser := NewParser(config)

	html := []byte(`
		<html>
		<head><title>Benchmark</title></head>
		<body>
			<h1>Title</h1>
			<p>Paragraph 1</p>
			<p>Paragraph 2</p>
		</body>
		</html>
	`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(html, "https://example.com")
	}
}

func BenchmarkCacheGetHit(b *testing.B) {
	tmpDir := b.TempDir()
	cm := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)
	cm.Set("bench-key", []byte("bench-value"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.Get("bench-key")
	}
}

// Test 26: RateLimiter Reserve function
func TestRateLimiter_Reserve(t *testing.T) {
	rl := NewRateLimiter()

	reservation := rl.Reserve("google")
	assert.NotNil(t, reservation)
	assert.True(t, reservation.OK())
}

// Test 27: RateLimiter GetLimit function
func TestRateLimiter_GetLimit(t *testing.T) {
	rl := NewRateLimiter()

	t.Run("get known limit", func(t *testing.T) {
		limit := rl.GetLimit("google")
		assert.Equal(t, float64(10), limit.RequestsPerSecond)
		assert.Equal(t, 20, limit.Burst)
	})

	t.Run("get unknown limit returns default", func(t *testing.T) {
		limit := rl.GetLimit("unknown-provider")
		assert.Equal(t, float64(1), limit.RequestsPerSecond)
		assert.Equal(t, 3, limit.Burst)
	})
}

// Test 28: RateLimiter RemoveLimit function
func TestRateLimiter_RemoveLimit(t *testing.T) {
	rl := NewRateLimiter()

	// Set a custom limit
	rl.SetLimit("custom", RateLimit{RequestsPerSecond: 100, Burst: 200})

	// Verify it exists
	limit := rl.GetLimit("custom")
	assert.Equal(t, float64(100), limit.RequestsPerSecond)

	// Remove the limit
	rl.RemoveLimit("custom")

	// Should now get default
	limit = rl.GetLimit("custom")
	assert.Equal(t, float64(1), limit.RequestsPerSecond)
}

// Test 29: RateLimiter Clear function
func TestRateLimiter_Clear(t *testing.T) {
	rl := NewRateLimiter()

	// Use some limiters
	rl.Allow("google")
	rl.Allow("bing")

	// Clear all
	rl.Clear()

	// Operations should still work after clear
	assert.True(t, rl.Allow("google"))
}

// Test 30: SearchProvider String function
func TestSearchProvider_String(t *testing.T) {
	tests := []struct {
		provider SearchProvider
		expected string
	}{
		{ProviderGoogle, "google"},
		{ProviderBing, "bing"},
		{ProviderDuckDuckGo, "duckduckgo"},
		{SearchProvider(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.provider.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test 31: WebTools ClearCache function
func TestWebTools_ClearCache(t *testing.T) {
	config := DefaultConfig()
	config.CacheEnabled = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	// Add something to cache via cache manager
	wt.cacheManager.Set("test-key", []byte("test-value"))

	// Verify it exists
	_, ok := wt.cacheManager.Get("test-key")
	assert.True(t, ok)

	// Clear cache
	err = wt.ClearCache()
	require.NoError(t, err)

	// Verify it's gone
	_, ok = wt.cacheManager.Get("test-key")
	assert.False(t, ok)
}

// Test 32: WebTools GetCacheStats function
func TestWebTools_GetCacheStats(t *testing.T) {
	config := DefaultConfig()
	config.CacheEnabled = true
	wt, err := NewWebTools(config)
	require.NoError(t, err)
	defer wt.Close()

	stats := wt.GetCacheStats()
	assert.NotNil(t, stats)
}

// Test 33: CacheManager Remove function
func TestCacheManager_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)
	defer cm.Close()

	// Set a value
	cm.Set("remove-key", []byte("remove-value"))

	// Verify it exists
	_, ok := cm.Get("remove-key")
	assert.True(t, ok)

	// Remove it
	cm.Remove("remove-key")

	// Verify it's gone
	_, ok = cm.Get("remove-key")
	assert.False(t, ok)
}

// Test 34: CacheManager Cleanup function
func TestCacheManager_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	// Use very short TTL
	cm := NewCacheManager(tmpDir, 1*time.Millisecond, 100*1024*1024)
	defer cm.Close()

	// Set values
	cm.Set("cleanup-key1", []byte("value1"))
	cm.Set("cleanup-key2", []byte("value2"))

	// Wait for TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Run cleanup - it returns error, not count
	err := cm.Cleanup()

	// Should not error
	assert.NoError(t, err)
}

// Test 35: DiskCache Remove function
func TestDiskCache_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)
	defer cm.Close()

	// Set value in disk cache using proper CacheEntry
	entry := &CacheEntry{
		Key:       "disk-key",
		Value:     []byte("disk-value"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Size:      10,
	}
	err := cm.diskCache.Set("disk-key", entry)
	require.NoError(t, err)

	// Verify it exists
	_, ok := cm.diskCache.Get("disk-key")
	assert.True(t, ok)

	// Remove it
	err = cm.diskCache.Remove("disk-key")
	require.NoError(t, err)

	// Verify it's gone
	_, ok = cm.diskCache.Get("disk-key")
	assert.False(t, ok)
}

// Test 36: DiskCache Clear function
func TestDiskCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCacheManager(tmpDir, 15*time.Minute, 100*1024*1024)
	defer cm.Close()

	// Set values using proper CacheEntry
	entry1 := &CacheEntry{
		Key:       "clear-key1",
		Value:     []byte("value1"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Size:      6,
	}
	entry2 := &CacheEntry{
		Key:       "clear-key2",
		Value:     []byte("value2"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Size:      6,
	}
	cm.diskCache.Set("clear-key1", entry1)
	cm.diskCache.Set("clear-key2", entry2)

	// Clear
	err := cm.diskCache.Clear()
	require.NoError(t, err)

	// Verify entries are gone
	_, ok := cm.diskCache.Get("clear-key1")
	assert.False(t, ok)
}

// Test 37: DiskCache Cleanup function
func TestDiskCache_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	// Short TTL
	cm := NewCacheManager(tmpDir, 1*time.Millisecond, 100*1024*1024)
	defer cm.Close()

	// Set value using proper CacheEntry
	entry := &CacheEntry{
		Key:       "expire-key",
		Value:     []byte("value"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Millisecond),
		Size:      5,
	}
	cm.diskCache.Set("expire-key", entry)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Run cleanup with TTL
	err := cm.diskCache.Cleanup(1 * time.Millisecond)
	assert.NoError(t, err)
}
