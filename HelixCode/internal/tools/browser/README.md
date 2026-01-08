# Browser Package

The `browser` package provides comprehensive browser automation capabilities for HelixCode using the Chrome DevTools Protocol (CDP). It enables AI agents to interact with web pages, capture screenshots, monitor console output, and perform complex browser-based tasks.

## Overview

This package integrates with Chrome/Chromium browsers through CDP to provide:
- Automated browser instance management with configurable options
- Page navigation and interaction (click, type, scroll, hover)
- Screenshot capture with full-page and element-specific modes
- Console log monitoring and JavaScript error capture
- Tab management for multi-page workflows
- Browser discovery for finding available Chrome installations

## Key Types

### BrowserController

The main orchestrator for browser automation operations.

```go
type BrowserController struct {
    config     *BrowserConfig
    ctx        context.Context
    cancel     context.CancelFunc
    allocCtx   context.Context
    browserCtx context.Context
    tabs       map[string]*Tab
    console    *ConsoleMonitor
    mu         sync.RWMutex
}
```

### BrowserConfig

Configuration options for browser behavior.

```go
type BrowserConfig struct {
    Headless        bool          // Run browser without visible window
    Timeout         time.Duration // Default operation timeout
    UserDataDir     string        // Chrome user data directory
    ScreenshotDir   string        // Directory for saving screenshots
    WindowWidth     int           // Browser window width
    WindowHeight    int           // Browser window height
    IgnoreCertErrors bool         // Skip certificate validation
    ProxyServer     string        // HTTP/SOCKS proxy address
    UserAgent       string        // Custom user agent string
    ExecPath        string        // Path to Chrome executable
}
```

### Tab

Represents a browser tab with its own context.

```go
type Tab struct {
    ID       string
    URL      string
    Title    string
    ctx      context.Context
    cancel   context.CancelFunc
    isActive bool
}
```

### ConsoleMonitor

Captures and stores browser console output.

```go
type ConsoleMonitor struct {
    entries []ConsoleEntry
    maxSize int
    mu      sync.RWMutex
}

type ConsoleEntry struct {
    Level     string    // "log", "warn", "error", "info"
    Message   string
    Timestamp time.Time
    URL       string
    Line      int
}
```

### ScreenshotOptions

Configuration for screenshot capture.

```go
type ScreenshotOptions struct {
    FullPage   bool   // Capture entire scrollable page
    Selector   string // CSS selector for element capture
    Format     string // "png" or "jpeg"
    Quality    int    // JPEG quality (1-100)
    Scale      float64 // Device scale factor
    OutputPath string // File path for saving
}
```

## Usage Examples

### Basic Browser Automation

```go
package main

import (
    "context"
    "fmt"
    "time"

    "dev.helix.code/internal/tools/browser"
)

func main() {
    config := &browser.BrowserConfig{
        Headless:     true,
        Timeout:      30 * time.Second,
        WindowWidth:  1920,
        WindowHeight: 1080,
    }

    controller, err := browser.NewBrowserController(config)
    if err != nil {
        panic(err)
    }
    defer controller.Close()

    ctx := context.Background()

    // Navigate to a page
    if err := controller.Navigate(ctx, "https://example.com"); err != nil {
        panic(err)
    }

    // Get page title
    title, err := controller.GetTitle(ctx)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Page title: %s\n", title)
}
```

### Page Interaction

```go
// Click on an element
err := controller.Click(ctx, "#submit-button")

// Type text into an input field
err = controller.Type(ctx, "#search-input", "search query")

// Wait for an element to appear
err = controller.WaitForElement(ctx, ".results-container", 10*time.Second)

// Scroll to an element
err = controller.ScrollToElement(ctx, "#footer")

// Hover over an element
err = controller.Hover(ctx, ".dropdown-menu")

// Execute JavaScript
result, err := controller.Evaluate(ctx, "document.title")
```

### Screenshot Capture

```go
// Full page screenshot
screenshotPath, err := controller.Screenshot(ctx, &browser.ScreenshotOptions{
    FullPage:   true,
    Format:     "png",
    OutputPath: "/tmp/fullpage.png",
})

// Element-specific screenshot
screenshotPath, err = controller.Screenshot(ctx, &browser.ScreenshotOptions{
    Selector:   "#main-content",
    Format:     "jpeg",
    Quality:    85,
    OutputPath: "/tmp/element.jpg",
})

// Screenshot with custom scale
screenshotPath, err = controller.Screenshot(ctx, &browser.ScreenshotOptions{
    Scale:      2.0, // Retina-like quality
    OutputPath: "/tmp/highres.png",
})
```

### Tab Management

```go
// Create a new tab
tab, err := controller.NewTab(ctx)

// Navigate in the new tab
err = controller.NavigateInTab(ctx, tab.ID, "https://example.org")

// Switch between tabs
err = controller.SwitchToTab(ctx, tab.ID)

// Close a tab
err = controller.CloseTab(ctx, tab.ID)

// List all tabs
tabs := controller.ListTabs()
for _, t := range tabs {
    fmt.Printf("Tab %s: %s (%s)\n", t.ID, t.Title, t.URL)
}
```

### Console Monitoring

```go
// Start monitoring console output
controller.StartConsoleMonitor()

// Navigate and interact with pages...
err := controller.Navigate(ctx, "https://example.com")

// Get captured console entries
entries := controller.GetConsoleEntries()
for _, entry := range entries {
    fmt.Printf("[%s] %s: %s\n", entry.Timestamp, entry.Level, entry.Message)
}

// Filter for errors only
errors := controller.GetConsoleErrors()

// Clear console buffer
controller.ClearConsole()
```

### Browser Discovery

```go
// Find Chrome installations on the system
installations, err := browser.DiscoverBrowsers()
for _, install := range installations {
    fmt.Printf("Found: %s at %s (version %s)\n",
        install.Name, install.Path, install.Version)
}

// Use a specific Chrome installation
config := &browser.BrowserConfig{
    ExecPath: installations[0].Path,
    Headless: true,
}
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Headless` | bool | true | Run browser without visible window |
| `Timeout` | time.Duration | 30s | Default timeout for operations |
| `UserDataDir` | string | temp dir | Chrome user data directory |
| `ScreenshotDir` | string | temp dir | Directory for screenshots |
| `WindowWidth` | int | 1920 | Browser window width in pixels |
| `WindowHeight` | int | 1080 | Browser window height in pixels |
| `IgnoreCertErrors` | bool | false | Skip SSL certificate validation |
| `ProxyServer` | string | "" | Proxy server URL |
| `UserAgent` | string | default | Custom user agent string |
| `ExecPath` | string | auto | Path to Chrome executable |

## Security Considerations

1. **Certificate Validation**: The `IgnoreCertErrors` option should only be used in development environments. Never disable certificate validation in production.

2. **User Data Isolation**: Each browser session should use an isolated user data directory to prevent data leakage between sessions.

3. **Proxy Configuration**: When using proxies, ensure they are trusted. Malicious proxies can intercept sensitive data.

4. **JavaScript Execution**: The `Evaluate` method executes arbitrary JavaScript. Only run trusted code and validate inputs.

5. **Screenshot Privacy**: Screenshots may capture sensitive information. Ensure proper access controls on screenshot directories.

6. **Resource Cleanup**: Always call `Close()` on the controller to properly terminate browser processes and release resources.

## Error Handling

The package defines several error types for specific failure scenarios:

```go
var (
    ErrBrowserNotFound    = errors.New("browser executable not found")
    ErrNavigationFailed   = errors.New("page navigation failed")
    ErrElementNotFound    = errors.New("element not found")
    ErrTimeout            = errors.New("operation timed out")
    ErrTabNotFound        = errors.New("tab not found")
    ErrScreenshotFailed   = errors.New("screenshot capture failed")
    ErrJavaScriptError    = errors.New("JavaScript execution error")
)
```

## Platform Support

The browser package supports:
- **Linux**: Uses system Chrome/Chromium or Chromium from package managers
- **macOS**: Supports Chrome, Chrome Canary, and Chromium
- **Windows**: Supports Chrome installations from Program Files

Browser discovery automatically searches common installation paths for each platform.

## Dependencies

- `github.com/chromedp/chromedp`: Chrome DevTools Protocol client
- `github.com/chromedp/cdproto`: CDP type definitions
