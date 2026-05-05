// Package mcp also provides client-side support for the Model Context Protocol.
//
// The client surface lets HelixCode connect to external MCP servers across
// four transports (stdio, HTTP, SSE, WebSocket) and expose their tools to
// the agent. The client surface is orthogonal to the existing server-side
// MCPServer/MCPSession types in this package — they share JSON-RPC framing
// (MCPMessage, MCPError) but never call each other.
//
// Entry points:
//   - Manager: aggregates Clients across configured servers; consumed by
//     internal/tools/registry.go to register external tools.
//   - Client: one per server; owns a Transport and a state machine.
//   - Transport: abstracts stdio/HTTP/SSE/WS; one file per transport.
//
// Configuration is YAML-first (.helixcode/mcp.yml in project + user dirs).
// CLI commands (helixcode mcp add/remove) round-trip the YAML.
package mcp
