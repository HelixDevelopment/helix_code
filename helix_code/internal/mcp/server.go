package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// MCPServer implements the Model Context Protocol server
type MCPServer struct {
	upgrader   websocket.Upgrader
	sessions   map[uuid.UUID]*MCPSession
	sessionMux sync.RWMutex
	tools      map[string]*Tool
	toolMux    sync.RWMutex
}

// WebSocketConn defines the interface for WebSocket connections
type WebSocketConn interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	Close() error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetPongHandler(h func(string) error)
	WriteMessage(messageType int, data []byte) error
}

// MCPSession represents an MCP session
type MCPSession struct {
	ID           uuid.UUID
	Conn         WebSocketConn
	CreatedAt    time.Time
	LastActivity time.Time
	UserID       uuid.UUID
	Context      map[string]interface{}
}

// Tool represents an MCP tool
type Tool struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Handler     ToolHandler            `json:"-"`
	Permissions []string               `json:"permissions"`
}

// ToolHandler is the function signature for tool execution
type ToolHandler func(ctx context.Context, session *MCPSession, args map[string]interface{}) (interface{}, error)

// MCPMessage represents an MCP protocol message.
// ID is interface{} to accept both string and numeric JSON-RPC identifiers.
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc,omitempty"`
	ID      interface{} `json:"id,omitempty"`
	Type    string      `json:"type,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// unmarshalParams decodes the Params field (which may be json.RawMessage,
// map[string]any, or nil) into dst.
func unmarshalParams(params interface{}, dst interface{}) error {
	if params == nil {
		return nil
	}
	var raw []byte
	switch v := params.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		raw = v
	default:
		var err error
		raw, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}
	return json.Unmarshal(raw, dst)
}

// newOriginChecker builds a websocket.Upgrader.CheckOrigin function that
// validates the handshake request's Origin header against an explicit
// allowlist, per OWASP's WebSocket Security Cheat Sheet primary CSWSH
// (Cross-Site WebSocket Hijacking) defense: "Use an allowlist, not a
// denylist. Avoid wildcards or substring matching." (see the deep-research
// citation in
// docs/research/07.2026/05_mcp_acp_protocols/WS_ENDPOINT_AUTH_DESIGN.md §5.3).
//
// Requests with NO Origin header are allowed through: the Origin header is a
// browser-only artefact of the Fetch/WebSocket spec — a malicious web page's
// browser ALWAYS attaches it, so its absence identifies a non-browser client
// (Go/Python/Node MCP SDKs, curl-style harnesses, gorilla/websocket dialers)
// rather than a spoofing attempt; those clients are gated separately by
// server.go's wsAuthMiddleware() Bearer/x-api-key check.
//
// Same-origin (Origin host == request Host) and localhost/127.0.0.1/::1
// origins are always allowed. extraAllowed (operator-configured via
// cfg.Auth.WSAllowedOrigins / HELIX_WS_ALLOWED_ORIGINS) extends the
// allowlist for legitimate cross-origin browser deployments; anything not
// covered is rejected — never a wildcard "return true".
func newOriginChecker(extraAllowed []string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}

		u, err := url.Parse(origin)
		if err != nil {
			return false
		}

		if strings.EqualFold(u.Host, r.Host) {
			return true
		}

		switch strings.ToLower(u.Hostname()) {
		case "localhost", "127.0.0.1", "::1":
			return true
		}

		for _, allowed := range extraAllowed {
			allowed = strings.TrimSpace(allowed)
			if allowed != "" && strings.EqualFold(allowed, origin) {
				return true
			}
		}

		return false
	}
}

// NewMCPServer creates a new MCP server. The WebSocket upgrader's Origin
// allowlist defaults to same-origin/localhost/no-Origin-header only (see
// newOriginChecker) — call SetAllowedOrigins to extend it with
// operator-configured extra origins before the server starts accepting
// connections.
func NewMCPServer() *MCPServer {
	return &MCPServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: newOriginChecker(nil),
		},
		sessions: make(map[uuid.UUID]*MCPSession),
		tools:    make(map[string]*Tool),
	}
}

// SetAllowedOrigins overrides the WebSocket upgrader's Origin allowlist with
// operator-configured extra origins (cfg.Auth.WSAllowedOrigins), layered on
// top of the always-allowed same-origin/localhost/no-Origin-header defaults
// baked into newOriginChecker. Intended to be called once, at construction
// time, before the server starts accepting connections (server.New does
// this immediately after mcp.NewMCPServer()); not safe for concurrent use
// with an in-flight Upgrade() call.
func (s *MCPServer) SetAllowedOrigins(extraAllowed []string) {
	s.upgrader.CheckOrigin = newOriginChecker(extraAllowed)
}

// RegisterTool registers a new tool with the MCP server.
//
// The tool map is keyed by tool.Name — the SAME identifier that handleListTools
// advertises and handleCallTool dispatches on. Per the MCP spec a client lists
// tools (receiving their Name) then calls them by that Name, so registration,
// advertisement, and dispatch MUST all agree on Name; keying by tool.ID instead
// made any tool whose ID differs from its Name uncallable (-32601 Tool not
// found) by a spec-conformant client.
func (s *MCPServer) RegisterTool(tool *Tool) error {
	// Mutate the map under the write-lock, then RELEASE it BEFORE logging.
	// log.Print serialises on the standard logger's process-global mutex and
	// writes to stderr; performing that I/O while holding toolMux turns a
	// microsecond map insert into a critical section gated by global log +
	// stderr contention, blocking every concurrent reader (handleListTools /
	// handleCallTool / GetToolCount) for the duration of the write.
	s.toolMux.Lock()
	if _, exists := s.tools[tool.Name]; exists {
		s.toolMux.Unlock()
		return fmt.Errorf("%s", tr(context.Background(), "internal_mcp_server_tool_already_registered", map[string]any{"ToolID": tool.Name}))
	}
	s.tools[tool.Name] = tool
	s.toolMux.Unlock()

	log.Print(tr(context.Background(), "internal_mcp_server_tool_registered", map[string]any{"ToolName": tool.Name, "ToolID": tool.ID}))
	return nil
}

// HandleWebSocket handles WebSocket connections for MCP
func (s *MCPServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(tr(context.Background(), "internal_mcp_server_websocket_upgrade_failed", map[string]any{"Error": err}))
		return
	}

	// Create new session
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         conn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	// Store session
	s.sessionMux.Lock()
	s.sessions[session.ID] = session
	s.sessionMux.Unlock()

	log.Print(tr(context.Background(), "internal_mcp_server_session_started", map[string]any{"SessionID": session.ID}))

	// Handle session
	go s.handleSession(session)
}

// handleSession handles an individual MCP session
func (s *MCPServer) handleSession(session *MCPSession) {
	defer func() {
		session.Conn.Close()
		s.sessionMux.Lock()
		delete(s.sessions, session.ID)
		s.sessionMux.Unlock()
		log.Print(tr(context.Background(), "internal_mcp_server_session_ended", map[string]any{"SessionID": session.ID}))
	}()

	for {
		var message MCPMessage
		err := session.Conn.ReadJSON(&message)
		if err != nil {
			log.Print(tr(context.Background(), "internal_mcp_server_read_message_failed", map[string]any{"Error": err}))
			break
		}

		session.LastActivity = time.Now()

		// Handle message
		go s.handleMessage(session, &message)
	}
}

// handleMessage handles an individual MCP message
func (s *MCPServer) handleMessage(session *MCPSession, message *MCPMessage) {
	ctx := context.Background()

	switch message.Method {
	case "initialize":
		s.handleInitialize(session, message)
	case "tools/list":
		s.handleListTools(session, message)
	case "tools/call":
		s.handleCallTool(ctx, session, message)
	case "notifications/capabilities":
		s.handleCapabilities(session, message)
	case "ping":
		s.handlePing(session, message)
	default:
		s.sendError(session, message.ID, -32601, tr(context.Background(), "internal_mcp_server_method_not_found", nil), nil)
	}
}

// handleInitialize handles the initialize method
func (s *MCPServer) handleInitialize(session *MCPSession, message *MCPMessage) {
	var params struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		Capabilities    map[string]interface{} `json:"capabilities"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}

	if err := unmarshalParams(message.Params, &params); err != nil {
		s.sendError(session, message.ID, -32700, "Parse error", nil)
		return
	}

	response := MCPMessage{
		ID:   message.ID,
		Type: "response",
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
				"sampling": map[string]interface{}{
					"enabled": true,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    tr(context.Background(), "internal_mcp_server_info_name", nil),
				"version": "1.0.0",
			},
		},
	}

	s.sendMessage(session, &response)
}

// handleListTools handles the tools/list method
func (s *MCPServer) handleListTools(session *MCPSession, message *MCPMessage) {
	// Build the tool snapshot under the read-lock, then RELEASE the lock BEFORE
	// any I/O. Holding toolMux across sendMessage/WriteJSON (which JSON-marshals
	// the whole tools array and takes the connection's write-mutex) needlessly
	// widens the critical section: under concurrent RegisterTool load, Go's
	// writer-preferring RWMutex makes queued writers block, which then blocks
	// every subsequent reader, serialising all callers behind the slowest List.
	// handleCallTool already follows this copy-under-lock-then-act discipline.
	s.toolMux.RLock()
	tools := make([]map[string]interface{}, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		})
	}
	s.toolMux.RUnlock()

	response := MCPMessage{
		ID:   message.ID,
		Type: "response",
		Result: map[string]interface{}{
			"tools": tools,
		},
	}

	s.sendMessage(session, &response)
}

// handleCallTool handles the tools/call method
func (s *MCPServer) handleCallTool(ctx context.Context, session *MCPSession, message *MCPMessage) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := unmarshalParams(message.Params, &params); err != nil {
		s.sendError(session, message.ID, -32700, "Parse error", nil)
		return
	}

	s.toolMux.RLock()
	tool, exists := s.tools[params.Name]
	s.toolMux.RUnlock()

	if !exists {
		s.sendError(session, message.ID, -32601, "Tool not found", nil)
		return
	}

	// Execute tool. The handler is third-party/user-supplied code; a panic in
	// it MUST NOT crash the server process (handleMessage dispatches each
	// message in its own goroutine, so an unrecovered panic here would kill the
	// whole process and every other session). Isolate it and surface a clean
	// JSON-RPC error instead. (§11.4.85(B) tool-handler-panic isolation.)
	result, err := s.invokeToolHandler(ctx, tool, session, params.Arguments)
	if err != nil {
		s.sendError(session, message.ID, -32000, tr(context.Background(), "internal_mcp_server_tool_execution_failed", nil), err.Error())
		return
	}

	response := MCPMessage{
		ID:   message.ID,
		Type: "response",
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
		},
	}

	s.sendMessage(session, &response)
}

// invokeToolHandler runs a tool's handler with panic isolation. A panic in the
// (third-party / user-supplied) handler is converted into a returned error so
// the per-message dispatch goroutine — and thus the whole server process —
// never crashes. The recovered panic is surfaced as a normal tool-execution
// error so the caller's existing error-response path applies unchanged.
func (s *MCPServer) invokeToolHandler(ctx context.Context, tool *Tool, session *MCPSession, args map[string]interface{}) (result interface{}, err error) {
	defer func() {
		if p := recover(); p != nil {
			result = nil
			err = fmt.Errorf("%s", tr(context.Background(), "internal_mcp_server_tool_handler_panicked",
				map[string]any{"ToolName": tool.Name, "Panic": p}))
		}
	}()
	return tool.Handler(ctx, session, args)
}

// handleCapabilities handles the capabilities notification
func (s *MCPServer) handleCapabilities(session *MCPSession, message *MCPMessage) {
	// Acknowledge capabilities notification
	response := MCPMessage{
		ID:   message.ID,
		Type: "response",
		Result: map[string]interface{}{
			"capabilities": map[string]interface{}{
				"experimental": map[string]interface{}{},
			},
		},
	}

	s.sendMessage(session, &response)
}

// handlePing handles the ping method
func (s *MCPServer) handlePing(session *MCPSession, message *MCPMessage) {
	response := MCPMessage{
		ID:   message.ID,
		Type: "response",
		Result: map[string]interface{}{
			"pong": true,
		},
	}

	s.sendMessage(session, &response)
}

// sendMessage sends a message to a session
func (s *MCPServer) sendMessage(session *MCPSession, message *MCPMessage) error {
	session.Conn.WriteJSON(message)
	return nil
}

// sendError sends an error response
func (s *MCPServer) sendError(session *MCPSession, id interface{}, code int, message string, data interface{}) {
	errorResponse := MCPMessage{
		ID:   id,
		Type: "response",
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	s.sendMessage(session, &errorResponse)
}

// BroadcastNotification broadcasts a notification to all sessions
func (s *MCPServer) BroadcastNotification(method string, params interface{}) {
	s.sessionMux.RLock()
	defer s.sessionMux.RUnlock()

	notification := MCPMessage{
		ID:     uuid.New().String(),
		Type:   "notification",
		Method: method,
		Params: params,
	}

	for _, session := range s.sessions {
		go s.sendMessage(session, &notification)
	}
}

// GetSessionCount returns the number of active sessions
func (s *MCPServer) GetSessionCount() int {
	s.sessionMux.RLock()
	defer s.sessionMux.RUnlock()
	return len(s.sessions)
}

// GetToolCount returns the number of registered tools
func (s *MCPServer) GetToolCount() int {
	s.toolMux.RLock()
	defer s.toolMux.RUnlock()
	return len(s.tools)
}

// CloseSession closes a specific session
func (s *MCPServer) CloseSession(sessionID uuid.UUID) {
	s.sessionMux.Lock()
	defer s.sessionMux.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.Conn.Close()
		delete(s.sessions, sessionID)
	}
}

// CloseAllSessions closes all active sessions
func (s *MCPServer) CloseAllSessions() {
	s.sessionMux.Lock()
	defer s.sessionMux.Unlock()

	for _, session := range s.sessions {
		session.Conn.Close()
	}
	s.sessions = make(map[uuid.UUID]*MCPSession)
}
