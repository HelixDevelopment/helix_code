package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// NewMCPServer creates a new MCP server
func NewMCPServer() *MCPServer {
	return &MCPServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, you should validate the origin
				return true
			},
		},
		sessions: make(map[uuid.UUID]*MCPSession),
		tools:    make(map[string]*Tool),
	}
}

// RegisterTool registers a new tool with the MCP server
func (s *MCPServer) RegisterTool(tool *Tool) error {
	s.toolMux.Lock()
	defer s.toolMux.Unlock()

	if _, exists := s.tools[tool.ID]; exists {
		return fmt.Errorf("%s", tr(context.Background(), "internal_mcp_server_tool_already_registered", map[string]any{"ToolID": tool.ID}))
	}

	s.tools[tool.ID] = tool
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
	s.toolMux.RLock()
	defer s.toolMux.RUnlock()

	tools := make([]map[string]interface{}, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		})
	}

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
