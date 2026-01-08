// Package tools provides a comprehensive tool ecosystem for AI-powered development workflows.
//
// The tools package implements a unified registry of development tools including
// filesystem operations, shell execution, web scraping, browser automation,
// codebase mapping, and multi-file editing. All tools implement a common interface
// with parameter validation, security controls, and audit logging.
//
// # Tool Registry
//
// ToolRegistry is the central coordinator for all tools:
//
//	config := tools.DefaultRegistryConfig()
//	registry, err := tools.NewToolRegistry(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer registry.Close()
//
//	// Execute a tool by name
//	ctx := context.Background()
//	result, err := registry.Execute(ctx, "fs_read", map[string]interface{}{
//	    "path": "/path/to/file.go",
//	})
//
// # Tool Interface
//
// All tools implement the Tool interface:
//
//	type Tool interface {
//	    Name() string                                              // Tool name
//	    Description() string                                       // Brief description
//	    Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
//	    Schema() ToolSchema                                        // JSON schema for params
//	    Category() ToolCategory                                    // Tool category
//	    Validate(params map[string]interface{}) error              // Param validation
//	}
//
// # Tool Categories
//
// Tools are organized into categories:
//
//	tools.CategoryFileSystem   // File operations (read, write, edit, glob, grep)
//	tools.CategoryShell        // Command execution
//	tools.CategoryWeb          // Web scraping and search
//	tools.CategoryBrowser      // Browser automation
//	tools.CategoryMapping      // Codebase analysis
//	tools.CategoryMultiEdit    // Transactional multi-file editing
//	tools.CategoryConfirmation // User interaction
//	tools.CategoryNotebook     // Jupyter notebook operations
//	tools.CategoryInteractive  // Interactive prompts
//
// # Available Tools
//
// FileSystem Tools:
//
//	fs_read      - Read file contents
//	fs_write     - Write file contents
//	fs_edit      - Edit file with string replacement
//	glob         - Find files by pattern
//	grep         - Search file contents
//
// Shell Tools:
//
//	shell            - Execute command and wait
//	shell_background - Execute command in background
//	shell_output     - Get background command output
//	shell_kill       - Kill background command
//
// Web Tools:
//
//	web_fetch  - Fetch and parse web page
//	web_search - Search the web
//
// Browser Tools:
//
//	browser_launch     - Launch browser instance
//	browser_navigate   - Navigate to URL
//	browser_screenshot - Take screenshot
//	browser_close      - Close browser
//
// Mapping Tools:
//
//	codebase_map     - Generate codebase map
//	file_definitions - Get symbol definitions
//
// Multi-Edit Tools:
//
//	multiedit_begin   - Start edit transaction
//	multiedit_add     - Add file to transaction
//	multiedit_preview - Preview changes
//	multiedit_commit  - Apply changes atomically
//
// Interactive Tools:
//
//	ask_user     - Prompt user for input
//	task_tracker - Track task progress
//
// Notebook Tools:
//
//	notebook_read - Read Jupyter notebook
//	notebook_edit - Edit notebook cell
//
// # FileSystem Operations
//
//	// Read a file
//	content, _ := registry.Execute(ctx, "fs_read", map[string]interface{}{
//	    "path": "main.go",
//	})
//
//	// Write a file
//	registry.Execute(ctx, "fs_write", map[string]interface{}{
//	    "path":    "output.txt",
//	    "content": "Hello, World!",
//	})
//
//	// Edit a file
//	registry.Execute(ctx, "fs_edit", map[string]interface{}{
//	    "path":       "config.go",
//	    "old_string": "localhost:8080",
//	    "new_string": "0.0.0.0:8080",
//	})
//
//	// Search files by pattern
//	matches, _ := registry.Execute(ctx, "glob", map[string]interface{}{
//	    "pattern": "**/*.go",
//	})
//
//	// Search file contents
//	results, _ := registry.Execute(ctx, "grep", map[string]interface{}{
//	    "pattern": "TODO:",
//	    "path":    ".",
//	})
//
// # Shell Execution
//
//	// Execute command
//	result, _ := registry.Execute(ctx, "shell", map[string]interface{}{
//	    "command": "go test ./...",
//	    "timeout": 300,
//	})
//
//	// Background execution
//	exec, _ := registry.Execute(ctx, "shell_background", map[string]interface{}{
//	    "command": "npm run build",
//	})
//
//	// Get output
//	output, _ := registry.Execute(ctx, "shell_output", map[string]interface{}{
//	    "execution_id": exec.ID,
//	})
//
// # Web Operations
//
//	// Fetch webpage
//	page, _ := registry.Execute(ctx, "web_fetch", map[string]interface{}{
//	    "url":            "https://docs.example.com",
//	    "parse_markdown": true,
//	})
//
//	// Web search
//	results, _ := registry.Execute(ctx, "web_search", map[string]interface{}{
//	    "query":       "golang error handling",
//	    "max_results": 10,
//	})
//
// # Multi-File Editing
//
//	// Begin transaction
//	tx, _ := registry.Execute(ctx, "multiedit_begin", map[string]interface{}{
//	    "description": "Refactor authentication",
//	})
//
//	// Add edits
//	registry.Execute(ctx, "multiedit_add", map[string]interface{}{
//	    "transaction_id": tx.ID,
//	    "file_path":      "internal/auth/handler.go",
//	    "operation":      "update",
//	    "new_content":    updatedContent,
//	})
//
//	// Preview changes
//	preview, _ := registry.Execute(ctx, "multiedit_preview", map[string]interface{}{
//	    "transaction_id": tx.ID,
//	})
//
//	// Commit atomically
//	registry.Execute(ctx, "multiedit_commit", map[string]interface{}{
//	    "transaction_id": tx.ID,
//	})
//
// # Tool Schema
//
// Get tool schemas for validation and documentation:
//
//	// Single tool schema
//	schema, err := registry.GetSchema("fs_read")
//
//	// All schemas
//	allSchemas := registry.GetAllSchemas()
//
//	// Export as JSON
//	jsonSchemas, _ := registry.ExportSchemas()
//
// # Configuration
//
// Customize tool behavior:
//
//	config := tools.DefaultRegistryConfig()
//
//	// FileSystem
//	config.FileSystemConfig.MaxFileSize = 100 * 1024 * 1024
//	config.FileSystemConfig.CacheEnabled = true
//
//	// Shell
//	config.ShellConfig.MaxConcurrent = 10
//	config.ShellConfig.DefaultTimeout = 30 * time.Second
//
//	// Web
//	config.WebConfig.RateLimitEnabled = true
//	config.WebConfig.MaxContentSize = 10 * 1024 * 1024
//
//	// Browser
//	config.BrowserConfig.MaxConcurrentBrowsers = 5
//
//	registry, _ := tools.NewToolRegistry(config)
//
// # Querying Tools
//
//	// Get specific tool
//	tool, err := registry.Get("fs_read")
//
//	// List all tools
//	allTools := registry.List()
//
//	// List by category
//	fsTools := registry.ListByCategory(tools.CategoryFileSystem)
//
//	// Register alias
//	registry.RegisterAlias("read", "fs_read")
//
// # Custom Tools
//
// Implement and register custom tools:
//
//	type MyTool struct {
//	    registry *tools.ToolRegistry
//	}
//
//	func (t *MyTool) Name() string { return "my_tool" }
//	func (t *MyTool) Description() string { return "Custom functionality" }
//	func (t *MyTool) Category() tools.ToolCategory { return tools.CategoryFileSystem }
//	func (t *MyTool) Schema() tools.ToolSchema { /* return schema */ }
//	func (t *MyTool) Validate(params map[string]interface{}) error { /* validate */ }
//	func (t *MyTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
//	    // Implementation
//	}
//
//	registry.Register(&MyTool{registry: registry})
//
// # Security
//
// All tools implement comprehensive security:
//   - Path validation and workspace boundaries
//   - Command blocklist for dangerous operations
//   - Resource limits (CPU, memory, processes)
//   - Timeout enforcement
//   - Audit logging
//   - Sandbox isolation
//   - Sensitive file detection
//
// # Sub-packages
//
// The tools package includes specialized sub-packages:
//   - filesystem: File operations with security controls
//   - shell: Command execution with sandboxing
//   - web: Web fetching with rate limiting
//   - browser: Browser automation with chromedp
//   - mapping: Codebase analysis with tree-sitter
//   - multiedit: Transactional multi-file editing
//   - confirmation: User interaction and prompts
//   - git: Git operations and commit automation
//   - voice: Voice input processing
//
// # Thread Safety
//
// The ToolRegistry is thread-safe for concurrent access from multiple goroutines.
//
// # Resource Cleanup
//
// Always close the registry to release resources:
//
//	defer registry.Close()
package tools
