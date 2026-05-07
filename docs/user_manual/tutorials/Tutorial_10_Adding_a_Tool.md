# Tutorial 10: Adding a New Tool via MCP SDK

**Duration**: 30 minutes | **Level**: Advanced

## Overview
Create a custom tool that integrates with HelixCode's MCP server.

## Prerequisites
- HelixCode running with MCP enabled
- Go 1.24+
- Understanding of JSON-RPC

## Step 1: Define Tool Schema
Create `internal/tools/mycalculator/tool.go`:

```go
package mycalculator

import (
    "context"
    "encoding/json"
    "fmt"
)

type CalculatorArgs struct {
    A float64 `json:"a"`
    B float64 `json:"b"`
}

func (t *CalculatorTool) Name() string { return "calculator" }
func (t *CalculatorTool) Description() string {
    return "Performs arithmetic operations on two numbers"
}

func (t *CalculatorTool) Schema() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "a": {"type": "number"},
            "b": {"type": "number"}
        },
        "required": ["a", "b"]
    }`)
}
```

## Step 2: Implement Execute

```go
func (t *CalculatorTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var parsed CalculatorArgs
    if err := json.Unmarshal(args, &parsed); err != nil {
        return "", fmt.Errorf("invalid args: %w", err)
    }
    result := parsed.A + parsed.B
    return fmt.Sprintf("%f", result), nil
}
```

## Step 3: Register with MCP
In `internal/mcp/registry.go`:

```go
import "dev.helix.code/tools/mycalculator"

func init() {
    RegisterTool(&mycalculator.CalculatorTool{})
}
```

Test: Start server, connect via WebSocket, send `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"calculator","arguments":{"a":5,"b":3}}}`. Expected response: `{"result": "8.000000"}`.

## Step 4: Tool Confirmation (Optional)
Implement `Confirmable` interface for dangerous tools:

```go
func (t *CalculatorTool) RequiresConfirmation() bool {
    return false  // calculator is safe
}
```
