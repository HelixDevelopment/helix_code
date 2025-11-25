# MCP Package

The `mcp` package provides Model Context Protocol (MCP) implementation for the HelixCode platform.

## Overview

This package handles:
- MCP protocol implementation
- Multiple transport support (stdio, SSE)
- Tool registration and execution
- Resource management
- Prompt templates

## Key Types

### Server

The MCP server:

```go
type Server struct {
    transport Transport
    tools     map[string]*Tool
    resources map[string]*Resource
    prompts   map[string]*Prompt
}
```

### Transport

```go
type Transport interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Send(msg *Message) error
    Receive() <-chan *Message
}
```

### Tool

```go
type Tool struct {
    Name        string
    Description string
    InputSchema *Schema
    Handler     ToolHandler
}

type ToolHandler func(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
```

### Resource

```go
type Resource struct {
    URI         string
    Name        string
    Description string
    MimeType    string
}
```

## Usage

### Creating an MCP Server

```go
import "dev.helix.code/internal/mcp"

// Create with stdio transport
transport := mcp.NewStdioTransport()
server := mcp.NewServer(transport)

// Create with SSE transport
sseTransport := mcp.NewSSETransport(port)
server := mcp.NewServer(sseTransport)
```

### Registering Tools

```go
// Register a tool
server.RegisterTool(&mcp.Tool{
    Name:        "read_file",
    Description: "Read contents of a file",
    InputSchema: &mcp.Schema{
        Type: "object",
        Properties: map[string]*mcp.Schema{
            "path": {Type: "string", Description: "File path"},
        },
        Required: []string{"path"},
    },
    Handler: func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
        path := params["path"].(string)
        content, err := os.ReadFile(path)
        if err != nil {
            return nil, err
        }
        return &mcp.ToolResult{Content: string(content)}, nil
    },
})
```

### Registering Resources

```go
server.RegisterResource(&mcp.Resource{
    URI:         "file:///project/config.yaml",
    Name:        "Configuration",
    Description: "Project configuration file",
    MimeType:    "text/yaml",
})
```

### Registering Prompts

```go
server.RegisterPrompt(&mcp.Prompt{
    Name:        "code_review",
    Description: "Review code for issues",
    Arguments: []*mcp.Argument{
        {Name: "code", Description: "Code to review", Required: true},
    },
})
```

### Starting the Server

```go
ctx := context.Background()
err := server.Start(ctx)
if err != nil {
    log.Fatal(err)
}
```

## Protocol Messages

### Request Types

```go
// Tool call request
{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
        "name": "read_file",
        "arguments": {"path": "/path/to/file"}
    },
    "id": 1
}

// List tools request
{
    "jsonrpc": "2.0",
    "method": "tools/list",
    "id": 2
}

// List resources request
{
    "jsonrpc": "2.0",
    "method": "resources/list",
    "id": 3
}
```

### Response Types

```go
// Tool result response
{
    "jsonrpc": "2.0",
    "result": {
        "content": "file contents..."
    },
    "id": 1
}

// Error response
{
    "jsonrpc": "2.0",
    "error": {
        "code": -32600,
        "message": "Invalid request"
    },
    "id": 1
}
```

## Transport Types

### Stdio Transport

```go
// Used for CLI integrations
transport := mcp.NewStdioTransport()
```

### SSE Transport

```go
// Used for web integrations
transport := mcp.NewSSETransport(8081)
```

## Built-in Tools

The package includes standard tools:

- `fs_read` - Read file contents
- `fs_write` - Write file contents
- `fs_edit` - Edit file contents
- `glob` - Find files by pattern
- `grep` - Search file contents
- `exec` - Execute shell commands
- `web_fetch` - Fetch web content

## Configuration

```yaml
mcp:
  enabled: true
  transport: "stdio"  # or "sse"
  sse_port: 8081
  tools:
    enabled: true
    allowed: ["fs_read", "fs_write", "exec"]
```

## Testing

```bash
go test -v ./internal/mcp/...
```

## Notes

- Use stdio transport for CLI integrations
- Use SSE transport for web-based clients
- Validate all tool inputs for security
- Implement proper error handling
