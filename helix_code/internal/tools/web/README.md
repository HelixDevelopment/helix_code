# Web Package

The `web` package provides comprehensive web tools for HelixCode, including multi-provider web search, URL fetching with caching, and HTML-to-markdown conversion. It enables AI agents to retrieve and process web content efficiently.

## Overview

This package enables:
- Multi-provider web search (Google Custom Search, Bing, DuckDuckGo)
- URL fetching with intelligent caching
- HTML-to-markdown conversion for LLM consumption
- Rate limiting and request throttling
- Configurable timeouts and retries
- Content extraction and summarization
- Mock implementations for testing

## Key Types

### WebTools

The main coordinator for web operations.

```go
type WebTools struct {
    searcher   *WebSearcher
    fetcher    *Fetcher
    config     *WebConfig
    mu         sync.RWMutex
}

type WebConfig struct {
    SearchProvider   SearchProvider // google, bing, duckduckgo
    GoogleAPIKey     string
    GoogleCX         string         // Custom Search Engine ID
    BingAPIKey       string
    CacheEnabled     bool
    CacheDir         string
    CacheTTL         time.Duration
    MaxCacheSize     int64
    Timeout          time.Duration
    MaxRetries       int
    RetryDelay       time.Duration
    RateLimitPerMin  int
    UserAgent        string
    ProxyURL         string
}
```

### WebSearcher

Handles web search across multiple providers.

```go
type WebSearcher struct {
    providers map[SearchProvider]SearchClient
    config    *SearchConfig
    cache     *SearchCache
    mu        sync.RWMutex
}

type SearchConfig struct {
    DefaultProvider SearchProvider
    MaxResults      int
    SafeSearch      bool
    Language        string
    Region          string
    DateRestrict    string // e.g., "d7" for last 7 days
    SiteRestrict    []string
    ExcludeSites    []string
    Timeout         time.Duration
}
```

### SearchResult

Represents a single search result.

```go
type SearchResult struct {
    Title       string
    URL         string
    Snippet     string
    DisplayURL  string
    CachedURL   string
    Source      string    // Provider name
    Position    int       // Rank in results
    Timestamp   time.Time
}

type SearchResponse struct {
    Query           string
    Results         []SearchResult
    TotalResults    int64
    SearchTime      time.Duration
    Provider        SearchProvider
    Page            int
    ResultsPerPage  int
}
```

### Fetcher

Handles URL fetching with caching.

```go
type Fetcher struct {
    httpClient  *http.Client
    cache       *FetchCache
    rateLimiter *RateLimiter
    config      *FetchConfig
    mu          sync.RWMutex
}

type FetchConfig struct {
    Timeout          time.Duration
    MaxRetries       int
    RetryDelay       time.Duration
    MaxContentSize   int64
    FollowRedirects  bool
    MaxRedirects     int
    UserAgent        string
    AcceptTypes      []string
    CacheEnabled     bool
    CacheTTL         time.Duration
    RateLimitPerMin  int
    ProxyURL         string
    SkipTLSVerify    bool  // Only for testing
}
```

### FetchResult

The result of fetching a URL.

```go
type FetchResult struct {
    URL          string
    FinalURL     string    // After redirects
    StatusCode   int
    ContentType  string
    Content      []byte
    Markdown     string    // HTML converted to markdown
    Title        string
    Description  string
    FetchTime    time.Duration
    FromCache    bool
    Timestamp    time.Time
    Headers      http.Header
    Error        error
}
```

### SearchProvider

Supported search providers.

```go
type SearchProvider string

const (
    ProviderGoogle     SearchProvider = "google"
    ProviderBing       SearchProvider = "bing"
    ProviderDuckDuckGo SearchProvider = "duckduckgo"
)
```

## Usage Examples

### Basic Web Search

```go
package main

import (
    "context"
    "fmt"

    "dev.helix.code/internal/tools/web"
)

func main() {
    // Create web tools with Google search
    tools, err := web.NewWebTools(&web.WebConfig{
        SearchProvider: web.ProviderGoogle,
        GoogleAPIKey:   "your-api-key",
        GoogleCX:       "your-search-engine-id",
        CacheEnabled:   true,
        CacheTTL:       1 * time.Hour,
    })
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    // Perform search
    response, err := tools.Search(ctx, "Go programming language", &web.SearchOptions{
        MaxResults: 10,
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Found %d results in %v\n", len(response.Results), response.SearchTime)
    for i, result := range response.Results {
        fmt.Printf("%d. %s\n   %s\n   %s\n\n",
            i+1, result.Title, result.URL, result.Snippet)
    }
}
```

### Multi-Provider Search

```go
// Create tools with multiple providers
tools, _ := web.NewWebTools(&web.WebConfig{
    SearchProvider: web.ProviderGoogle,
    GoogleAPIKey:   "google-key",
    GoogleCX:       "google-cx",
    BingAPIKey:     "bing-key",
})

// Search with specific provider
googleResults, _ := tools.SearchWith(ctx, web.ProviderGoogle, "Go programming", nil)
bingResults, _ := tools.SearchWith(ctx, web.ProviderBing, "Go programming", nil)

// Search with fallback (tries next provider on failure)
results, provider, err := tools.SearchWithFallback(ctx, "Go programming", nil)
fmt.Printf("Results from: %s\n", provider)
```

### Advanced Search Options

```go
// Search with filters
response, err := tools.Search(ctx, "machine learning", &web.SearchOptions{
    MaxResults:   20,
    SafeSearch:   true,
    Language:     "en",
    Region:       "us",
    DateRestrict: "m6",  // Last 6 months
    SiteRestrict: []string{"github.com", "stackoverflow.com"},
    ExcludeSites: []string{"pinterest.com"},
})

// Search for specific file types
response, err = tools.Search(ctx, "machine learning filetype:pdf", nil)

// Search within a specific site
response, err = tools.Search(ctx, "site:github.com Go testing framework", nil)
```

### URL Fetching

```go
// Fetch a URL
result, err := tools.Fetch(ctx, "https://example.com/article")
if err != nil {
    panic(err)
}

fmt.Printf("Title: %s\n", result.Title)
fmt.Printf("Status: %d\n", result.StatusCode)
fmt.Printf("Content Type: %s\n", result.ContentType)
fmt.Printf("From Cache: %v\n", result.FromCache)

// Get content as markdown (ideal for LLM)
fmt.Printf("Markdown:\n%s\n", result.Markdown)

// Get raw content
fmt.Printf("Raw HTML length: %d bytes\n", len(result.Content))
```

### Fetch with Options

```go
// Fetch with custom options
result, err := tools.Fetch(ctx, "https://api.example.com/data", &web.FetchOptions{
    Timeout:         10 * time.Second,
    MaxRetries:      3,
    FollowRedirects: true,
    MaxRedirects:    5,
    Headers: map[string]string{
        "Accept":        "application/json",
        "Authorization": "Bearer token",
    },
})

// Fetch without caching
result, err = tools.Fetch(ctx, "https://example.com", &web.FetchOptions{
    SkipCache: true,
})

// Fetch with custom user agent
result, err = tools.Fetch(ctx, "https://example.com", &web.FetchOptions{
    UserAgent: "CustomBot/1.0",
})
```

### HTML to Markdown Conversion

```go
// Fetch and convert to markdown automatically
result, _ := tools.Fetch(ctx, "https://example.com/article")
markdown := result.Markdown

// Convert HTML string to markdown
html := "<h1>Title</h1><p>Content with <strong>bold</strong> text.</p>"
markdown, err := tools.HTMLToMarkdown(html)
// Output: "# Title\n\nContent with **bold** text."

// Extract main content (removes nav, footer, ads)
mainContent, err := tools.ExtractMainContent(html)
```

### Rate Limiting

```go
// Configure rate limiting
tools, _ := web.NewWebTools(&web.WebConfig{
    RateLimitPerMin: 60, // 60 requests per minute
})

// Rate limiting is automatic
for i := 0; i < 100; i++ {
    result, err := tools.Search(ctx, fmt.Sprintf("query %d", i), nil)
    if err != nil {
        if errors.Is(err, web.ErrRateLimited) {
            fmt.Println("Rate limited, waiting...")
            time.Sleep(1 * time.Second)
            continue
        }
        panic(err)
    }
}

// Check rate limit status
status := tools.RateLimitStatus()
fmt.Printf("Remaining: %d/%d\n", status.Remaining, status.Limit)
fmt.Printf("Resets at: %s\n", status.ResetsAt)
```

### Caching

```go
// Configure caching
tools, _ := web.NewWebTools(&web.WebConfig{
    CacheEnabled: true,
    CacheDir:     ".helix/cache/web",
    CacheTTL:     24 * time.Hour,
    MaxCacheSize: 100 * 1024 * 1024, // 100MB
})

// First fetch - from network
result1, _ := tools.Fetch(ctx, "https://example.com")
fmt.Printf("From cache: %v\n", result1.FromCache) // false

// Second fetch - from cache
result2, _ := tools.Fetch(ctx, "https://example.com")
fmt.Printf("From cache: %v\n", result2.FromCache) // true

// Clear cache for specific URL
tools.InvalidateCache("https://example.com")

// Clear entire cache
tools.ClearCache()

// Get cache statistics
stats := tools.CacheStats()
fmt.Printf("Cache size: %d bytes\n", stats.Size)
fmt.Printf("Cache entries: %d\n", stats.Entries)
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate*100)
```

### Content Extraction

```go
// Extract structured content
result, _ := tools.Fetch(ctx, "https://example.com/article")

// Get metadata
fmt.Printf("Title: %s\n", result.Title)
fmt.Printf("Description: %s\n", result.Description)

// Extract all links from page
links, err := tools.ExtractLinks(result.Content, result.URL)
for _, link := range links {
    fmt.Printf("Link: %s -> %s\n", link.Text, link.URL)
}

// Extract images
images, err := tools.ExtractImages(result.Content, result.URL)
for _, img := range images {
    fmt.Printf("Image: %s (%s)\n", img.URL, img.Alt)
}

// Extract specific elements
headings, err := tools.ExtractElements(result.Content, "h1, h2, h3")
```

### Error Handling and Retries

```go
// Configure retries
tools, _ := web.NewWebTools(&web.WebConfig{
    MaxRetries: 3,
    RetryDelay: 1 * time.Second,
    Timeout:    30 * time.Second,
})

// Automatic retry on transient failures
result, err := tools.Fetch(ctx, "https://flaky-server.com")
if err != nil {
    switch {
    case errors.Is(err, web.ErrTimeout):
        fmt.Println("Request timed out after retries")
    case errors.Is(err, web.ErrRateLimited):
        fmt.Println("Rate limited by server")
    case errors.Is(err, web.ErrNotFound):
        fmt.Println("Page not found (404)")
    case errors.Is(err, web.ErrForbidden):
        fmt.Println("Access forbidden (403)")
    default:
        fmt.Printf("Unknown error: %v\n", err)
    }
}
```

### Mock Implementation for Testing

```go
// Create mock web tools for testing
mockTools := web.NewMockWebTools()

// Set mock search results
mockTools.SetSearchResults("test query", []web.SearchResult{
    {Title: "Test Result 1", URL: "https://example.com/1", Snippet: "..."},
    {Title: "Test Result 2", URL: "https://example.com/2", Snippet: "..."},
})

// Set mock fetch results
mockTools.SetFetchResult("https://example.com", &web.FetchResult{
    StatusCode:  200,
    ContentType: "text/html",
    Content:     []byte("<html>Mock content</html>"),
    Markdown:    "Mock content",
})

// Use in tests
results, _ := mockTools.Search(ctx, "test query", nil)
content, _ := mockTools.Fetch(ctx, "https://example.com")
```

## Configuration Options

### WebConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `SearchProvider` | SearchProvider | google | Default search provider |
| `GoogleAPIKey` | string | "" | Google Custom Search API key |
| `GoogleCX` | string | "" | Google Search Engine ID |
| `BingAPIKey` | string | "" | Bing Search API key |
| `CacheEnabled` | bool | true | Enable response caching |
| `CacheDir` | string | .helix/cache/web | Cache directory |
| `CacheTTL` | time.Duration | 1h | Cache entry TTL |
| `MaxCacheSize` | int64 | 100MB | Maximum cache size |
| `Timeout` | time.Duration | 30s | Request timeout |
| `MaxRetries` | int | 3 | Maximum retry attempts |
| `RetryDelay` | time.Duration | 1s | Delay between retries |
| `RateLimitPerMin` | int | 60 | Requests per minute limit |
| `UserAgent` | string | HelixCode/1.0 | HTTP User-Agent |
| `ProxyURL` | string | "" | Proxy server URL |

### SearchOptions

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `MaxResults` | int | 10 | Maximum results to return |
| `SafeSearch` | bool | true | Enable safe search filter |
| `Language` | string | "" | Result language (ISO 639-1) |
| `Region` | string | "" | Result region (ISO 3166-1) |
| `DateRestrict` | string | "" | Date restriction (d7, m6, y1) |
| `SiteRestrict` | []string | [] | Restrict to these sites |
| `ExcludeSites` | []string | [] | Exclude these sites |

### FetchOptions

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Timeout` | time.Duration | 30s | Request timeout |
| `MaxRetries` | int | 3 | Maximum retries |
| `FollowRedirects` | bool | true | Follow HTTP redirects |
| `MaxRedirects` | int | 10 | Maximum redirects |
| `Headers` | map[string]string | {} | Custom HTTP headers |
| `SkipCache` | bool | false | Skip cache lookup |
| `UserAgent` | string | "" | Override user agent |

## Security Considerations

1. **API Key Protection**: Store API keys securely. Never commit them to version control.

2. **TLS Verification**: Never disable TLS verification in production (`SkipTLSVerify`).

3. **Proxy Security**: When using proxies, ensure they are trusted and secure.

4. **Rate Limiting**: Respect rate limits to avoid IP bans and service disruptions.

5. **Content Validation**: Validate fetched content before processing to prevent injection attacks.

6. **Cache Security**: Secure the cache directory with appropriate permissions.

7. **User Agent**: Use an identifiable user agent to allow site operators to contact you.

## Error Types

```go
var (
    ErrTimeout       = errors.New("request timed out")
    ErrRateLimited   = errors.New("rate limit exceeded")
    ErrNotFound      = errors.New("resource not found (404)")
    ErrForbidden     = errors.New("access forbidden (403)")
    ErrServerError   = errors.New("server error (5xx)")
    ErrInvalidURL    = errors.New("invalid URL")
    ErrContentTooLarge = errors.New("content exceeds size limit")
    ErrUnsupportedType = errors.New("unsupported content type")
    ErrCacheMiss     = errors.New("cache miss")
    ErrProviderError = errors.New("search provider error")
)
```

## Best Practices

1. **Enable caching** to reduce API usage and improve performance.

2. **Configure rate limits** appropriately for your API quotas.

3. **Use search provider fallbacks** to handle provider outages.

4. **Set reasonable timeouts** to prevent hanging requests.

5. **Handle errors gracefully** with appropriate retry logic.

6. **Use markdown output** for LLM consumption instead of raw HTML.

7. **Implement cleanup policies** for the cache directory.
