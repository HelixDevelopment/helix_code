// Package mcp provides Model Context Protocol (MCP) server implementation for the HelixCode platform.
//
// The mcp package implements the Model Context Protocol, enabling AI models to interact
// with external tools, resources, and services through a standardized JSON-RPC interface.
// It supports WebSocket connections for real-time bidirectional communication.
//
// # Model Context Protocol
//
// MCP is a protocol that allows AI assistants to:
//   - Execute registered tools with parameters
//   - Access external resources (files, APIs, databases)
//   - Receive notifications and updates
//   - Maintain session context across interactions
//
// # Architecture
//
// The package follows a session-based architecture:
//   - MCPServer: Central server managing sessions and tools
//   - MCPSession: Individual client session with context
//   - Tool: Registered tool with handler function
//   - MCPMessage: JSON-RPC 2.0 protocol message
//
// # Basic Usage
//
// Creating and configuring an MCP server:
//
//	server := mcp.NewMCPServer()
//
//	// Register tools
//	server.RegisterTool(&mcp.Tool{
//	    ID:          "read_file",
//	    Name:        "read_file",
//	    Description: "Read contents of a file",
//	    Parameters: map[string]interface{}{
//	        "type": "object",
//	        "properties": map[string]interface{}{
//	            "path": map[string]interface{}{
//	                "type":        "string",
//	                "description": "File path to read",
//	            },
//	        },
//	        "required": []string{"path"},
//	    },
//	    Handler: func(ctx context.Context, session *mcp.MCPSession, args map[string]interface{}) (interface{}, error) {
//	        path := args["path"].(string)
//	        content, err := os.ReadFile(path)
//	        return string(content), err
//	    },
//	})
//
//	// Serve WebSocket connections
//	http.HandleFunc("/mcp", server.HandleWebSocket)
//
// # Protocol Messages
//
// The protocol uses JSON-RPC 2.0 format:
//
// Request:
//
//	{
//	    "jsonrpc": "2.0",
//	    "method": "tools/call",
//	    "params": {
//	        "name": "read_file",
//	        "arguments": {"path": "/path/to/file"}
//	    },
//	    "id": 1
//	}
//
// Response:
//
//	{
//	    "jsonrpc": "2.0",
//	    "result": {
//	        "content": [{"type": "text", "text": "file contents..."}]
//	    },
//	    "id": 1
//	}
//
// # Supported Methods
//
// The server handles these protocol methods:
//
//   - initialize: Initialize session and exchange capabilities
//   - tools/list: List all registered tools
//   - tools/call: Execute a tool with arguments
//   - notifications/capabilities: Handle capability notifications
//   - ping: Health check ping/pong
//
// # Tool Registration
//
// Tools are defined with JSON Schema parameters:
//
//	tool := &mcp.Tool{
//	    ID:          "execute_command",
//	    Name:        "execute_command",
//	    Description: "Execute a shell command",
//	    Parameters: map[string]interface{}{
//	        "type": "object",
//	        "properties": map[string]interface{}{
//	            "command": map[string]interface{}{
//	                "type":        "string",
//	                "description": "Command to execute",
//	            },
//	            "timeout": map[string]interface{}{
//	                "type":        "integer",
//	                "description": "Timeout in seconds",
//	            },
//	        },
//	        "required": []string{"command"},
//	    },
//	    Permissions: []string{"shell:execute"},
//	    Handler:     executeCommandHandler,
//	}
//
// # Session Management
//
// Sessions track client connections and context:
//
//	// Get active session count
//	count := server.GetSessionCount()
//
//	// Close a specific session
//	server.CloseSession(sessionID)
//
//	// Close all sessions
//	server.CloseAllSessions()
//
// # Broadcasting Notifications
//
// Send notifications to all connected clients:
//
//	server.BroadcastNotification("progress", map[string]interface{}{
//	    "task":     "building",
//	    "progress": 75,
//	    "message":  "Compiling modules...",
//	})
//
// # Session Context
//
// Sessions maintain context for stateful interactions:
//
//	// Within a tool handler
//	func myHandler(ctx context.Context, session *mcp.MCPSession, args map[string]interface{}) (interface{}, error) {
//	    // Access session context
//	    if projectPath, ok := session.Context["project_path"]; ok {
//	        // Use project path
//	    }
//
//	    // Store context for later use
//	    session.Context["last_operation"] = "read_file"
//
//	    return result, nil
//	}
//
// # Error Handling
//
// The server uses standard JSON-RPC error codes:
//
//   - -32700: Parse error
//   - -32600: Invalid request
//   - -32601: Method not found
//   - -32602: Invalid params
//   - -32603: Internal error
//   - -32000: Tool execution failed
//
// Error response format:
//
//	{
//	    "jsonrpc": "2.0",
//	    "error": {
//	        "code": -32601,
//	        "message": "Method not found",
//	        "data": null
//	    },
//	    "id": 1
//	}
//
// # Server Capabilities
//
// The server advertises its capabilities during initialization:
//
//	{
//	    "protocolVersion": "2024-11-05",
//	    "capabilities": {
//	        "roots": {"listChanged": true},
//	        "sampling": {"enabled": true}
//	    },
//	    "serverInfo": {
//	        "name": "HelixCode MCP Server",
//	        "version": "1.0.0"
//	    }
//	}
//
// # Thread Safety
//
// The MCPServer is thread-safe. Multiple WebSocket connections can be handled
// concurrently, and tool handlers may be invoked in parallel.
package mcp
