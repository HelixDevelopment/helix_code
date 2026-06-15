package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMCPServer(t *testing.T) {
	server := NewMCPServer()

	assert.NotNil(t, server)
	assert.NotNil(t, server.sessions)
	assert.NotNil(t, server.tools)
	assert.Equal(t, 0, server.GetSessionCount())
	assert.Equal(t, 0, server.GetToolCount())
}

func TestRegisterTool(t *testing.T) {
	server := NewMCPServer()

	tool := &Tool{
		ID:          "test-tool",
		Name:        "Test Tool",
		Description: "A test tool",
		Parameters:  map[string]interface{}{},
		Handler: func(ctx context.Context, session *MCPSession, args map[string]interface{}) (interface{}, error) {
			return "test result", nil
		},
	}

	err := server.RegisterTool(tool)
	assert.NoError(t, err)
	assert.Equal(t, 1, server.GetToolCount())

	// Try to register the same tool again
	err = server.RegisterTool(tool)
	assert.Error(t, err)
	assert.Equal(t, 1, server.GetToolCount())
}

func TestCloseAllSessions(t *testing.T) {
	server := NewMCPServer()

	// Since we can't easily create real sessions without WebSocket, just test the method doesn't panic
	server.CloseAllSessions()
	assert.Equal(t, 0, server.GetSessionCount())
}

func TestHandleInitialize(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	t.Run("ValidInitialize", func(t *testing.T) {
		params := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		}
		paramsJSON, _ := json.Marshal(params)

		message := &MCPMessage{
			ID:     "test-1",
			Type:   "request",
			Method: "initialize",
			Params: paramsJSON,
		}

		server.handleInitialize(session, message)

		// Check that a response was sent
		assert.True(t, mockConn.writeCalled)
		assert.NotNil(t, mockConn.lastMessage)

		var response MCPMessage
		err := json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-1", response.ID)
		assert.Equal(t, "response", response.Type)
		assert.Nil(t, response.Error)

		result, ok := response.Result.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "2024-11-05", result["protocolVersion"])
		assert.Contains(t, result, "capabilities")
		assert.Contains(t, result, "serverInfo")
	})

	t.Run("InvalidJSONParams", func(t *testing.T) {
		mockConn := &MockConn{}
		session := &MCPSession{
			ID:           uuid.New(),
			Conn:         mockConn,
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Context:      make(map[string]interface{}),
		}

		message := &MCPMessage{
			ID:     "test-2",
			Type:   "request",
			Method: "initialize",
			Params: json.RawMessage(`invalid json`),
		}

		server.handleInitialize(session, message)

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err := json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-2", response.ID)
		assert.NotNil(t, response.Error)
		assert.Equal(t, -32700, response.Error.Code) // Parse error
	})
}

func TestHandleListTools(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	t.Run("ListToolsWithRegisteredTools", func(t *testing.T) {
		// Register some tools
		tool1 := &Tool{
			ID:          "tool1",
			Name:        "Tool One",
			Description: "First test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]interface{}{"type": "string"},
				},
			},
		}
		tool2 := &Tool{
			ID:          "tool2",
			Name:        "Tool Two",
			Description: "Second test tool",
			Parameters:  map[string]interface{}{},
		}

		err := server.RegisterTool(tool1)
		assert.NoError(t, err)
		err = server.RegisterTool(tool2)
		assert.NoError(t, err)

		message := &MCPMessage{
			ID:     "list-1",
			Type:   "request",
			Method: "tools/list",
		}

		server.handleListTools(session, message)

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err = json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.Equal(t, "list-1", response.ID)
		assert.Equal(t, "response", response.Type)

		result, ok := response.Result.(map[string]interface{})
		assert.True(t, ok)
		tools, ok := result["tools"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, tools, 2)
	})

	t.Run("ListToolsEmpty", func(t *testing.T) {
		// Create a new server with no tools
		emptyServer := NewMCPServer()
		mockConn := &MockConn{}
		session := &MCPSession{
			ID:           uuid.New(),
			Conn:         mockConn,
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Context:      make(map[string]interface{}),
		}

		message := &MCPMessage{
			ID:     "list-2",
			Type:   "request",
			Method: "tools/list",
		}

		emptyServer.handleListTools(session, message)

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err := json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)

		result, ok := response.Result.(map[string]interface{})
		assert.True(t, ok)
		tools, ok := result["tools"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, tools, 0)
	})
}

func TestHandleCallTool(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	t.Run("CallToolSuccess", func(t *testing.T) {
		// Register a tool
		tool := &Tool{
			ID:          "echo-tool",
			Name:        "Echo Tool",
			Description: "Echoes back the input",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{"type": "string"},
				},
			},
			Handler: func(ctx context.Context, session *MCPSession, args map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{
					"echo":       args["message"],
					"session_id": session.ID.String(),
				}, nil
			},
		}

		err := server.RegisterTool(tool)
		assert.NoError(t, err)

		params := map[string]interface{}{
			// Spec clients call by the advertised tool Name (from tools/list),
			// not by the internal ID.
			"name":      "Echo Tool",
			"arguments": map[string]interface{}{"message": "hello world"},
		}
		paramsJSON, _ := json.Marshal(params)

		message := &MCPMessage{
			ID:     "call-1",
			Type:   "request",
			Method: "tools/call",
			Params: paramsJSON,
		}

		server.handleCallTool(context.Background(), session, message)

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err = json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.Equal(t, "call-1", response.ID)
		assert.Equal(t, "response", response.Type)
		assert.Nil(t, response.Error)

		result, ok := response.Result.(map[string]interface{})
		assert.True(t, ok)
		content, ok := result["content"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, content, 1)
	})

	t.Run("CallToolNotFound", func(t *testing.T) {
		params := map[string]interface{}{
			"name":      "nonexistent-tool",
			"arguments": map[string]interface{}{},
		}
		paramsJSON, _ := json.Marshal(params)

		message := &MCPMessage{
			ID:     "call-2",
			Type:   "request",
			Method: "tools/call",
			Params: paramsJSON,
		}

		server.handleCallTool(context.Background(), session, message)

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err := json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.Equal(t, "call-2", response.ID)
		assert.NotNil(t, response.Error)
		assert.Equal(t, -32601, response.Error.Code) // Method not found
	})

	t.Run("CallToolExecutionError", func(t *testing.T) {
		// Register a tool that fails
		failingTool := &Tool{
			ID:          "failing-tool",
			Name:        "Failing Tool",
			Description: "Always fails",
			Handler: func(ctx context.Context, session *MCPSession, args map[string]interface{}) (interface{}, error) {
				return nil, assert.AnError
			},
		}

		err := server.RegisterTool(failingTool)
		assert.NoError(t, err)

		params := map[string]interface{}{
			"name":      "Failing Tool",
			"arguments": map[string]interface{}{},
		}
		paramsJSON, _ := json.Marshal(params)

		message := &MCPMessage{
			ID:     "call-3",
			Type:   "request",
			Method: "tools/call",
			Params: paramsJSON,
		}

		server.handleCallTool(context.Background(), session, message)

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err = json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.Equal(t, "call-3", response.ID)
		assert.NotNil(t, response.Error)
		assert.Equal(t, -32000, response.Error.Code) // Tool execution failed
	})
}

func TestHandleCapabilities(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	message := &MCPMessage{
		ID:     "cap-1",
		Type:   "request",
		Method: "notifications/capabilities",
	}

	server.handleCapabilities(session, message)

	assert.True(t, mockConn.writeCalled)
	var response MCPMessage
	err := json.Unmarshal(mockConn.lastMessage, &response)
	assert.NoError(t, err)
	assert.Equal(t, "cap-1", response.ID)
	assert.Equal(t, "response", response.Type)
	assert.Nil(t, response.Error)

	result, ok := response.Result.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, result, "capabilities")
}

func TestHandlePing(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	message := &MCPMessage{
		ID:     "ping-1",
		Type:   "request",
		Method: "ping",
	}

	server.handlePing(session, message)

	assert.True(t, mockConn.writeCalled)
	var response MCPMessage
	err := json.Unmarshal(mockConn.lastMessage, &response)
	assert.NoError(t, err)
	assert.Equal(t, "ping-1", response.ID)
	assert.Equal(t, "response", response.Type)
	assert.Nil(t, response.Error)

	result, ok := response.Result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, true, result["pong"])
}

func TestHandleMessage(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	t.Run("InitializeMessage", func(t *testing.T) {
		params := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		}
		paramsJSON, _ := json.Marshal(params)

		message := &MCPMessage{
			ID:     "msg-1",
			Type:   "request",
			Method: "initialize",
			Params: paramsJSON,
		}

		server.handleMessage(session, message)

		assert.True(t, mockConn.writeCalled)
	})

	t.Run("UnknownMethod", func(t *testing.T) {
		mockConn := &MockConn{}
		session := &MCPSession{
			ID:           uuid.New(),
			Conn:         mockConn,
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Context:      make(map[string]interface{}),
		}

		message := &MCPMessage{
			ID:     "msg-2",
			Type:   "request",
			Method: "unknown-method",
		}

		server.handleMessage(session, message)

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err := json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.NotNil(t, response.Error)
		assert.Equal(t, -32601, response.Error.Code) // Method not found
	})
}

func TestSendMessageAndSendError(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	t.Run("SendMessage", func(t *testing.T) {
		message := &MCPMessage{
			ID:   "send-1",
			Type: "response",
			Result: map[string]interface{}{
				"test": "data",
			},
		}

		err := server.sendMessage(session, message)
		assert.NoError(t, err)
		assert.True(t, mockConn.writeCalled)
		assert.NotNil(t, mockConn.lastMessage)
	})

	t.Run("SendError", func(t *testing.T) {
		server.sendError(session, "err-1", -32000, "Test error", "additional data")

		assert.True(t, mockConn.writeCalled)
		var response MCPMessage
		err := json.Unmarshal(mockConn.lastMessage, &response)
		assert.NoError(t, err)
		assert.Equal(t, "err-1", response.ID)
		assert.NotNil(t, response.Error)
		assert.Equal(t, -32000, response.Error.Code)
		assert.Equal(t, "Test error", response.Error.Message)
		assert.Equal(t, "additional data", response.Error.Data)
	})
}

func TestBroadcastNotification(t *testing.T) {
	server := NewMCPServer()

	// Create mock sessions
	mockConn1 := &MockConn{}
	session1 := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn1,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	mockConn2 := &MockConn{}
	session2 := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn2,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	// Manually add sessions (normally done by HandleWebSocket)
	server.sessionMux.Lock()
	server.sessions[session1.ID] = session1
	server.sessions[session2.ID] = session2
	server.sessionMux.Unlock()

	server.BroadcastNotification("test.event", map[string]interface{}{
		"message": "test notification",
	})

	// Wait for the broadcast goroutines to deliver to both sessions.
	require.Eventually(t, func() bool {
		return mockConn1.WriteCalled() && mockConn2.WriteCalled()
	}, 5*time.Second, 5*time.Millisecond, "broadcast goroutines did not complete in time")

	// Both connections should have received the notification.
	assert.True(t, mockConn1.WriteCalled())
	assert.True(t, mockConn2.WriteCalled())

	// Verify the notification content (simplified check).
	assert.NotEmpty(t, mockConn1.LastMessage())
	assert.NotEmpty(t, mockConn2.LastMessage())

	// Check that both messages contain the expected method.
	assert.True(t, strings.Contains(string(mockConn1.LastMessage()), "test.event"))
	assert.True(t, strings.Contains(string(mockConn2.LastMessage()), "test.event"))
}

func TestCloseSession(t *testing.T) {
	server := NewMCPServer()
	mockConn := &MockConn{}
	sessionID := uuid.New()
	session := &MCPSession{
		ID:           sessionID,
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	// Manually add session
	server.sessionMux.Lock()
	server.sessions[sessionID] = session
	server.sessionMux.Unlock()

	assert.Equal(t, 1, server.GetSessionCount())

	server.CloseSession(sessionID)

	assert.Equal(t, 0, server.GetSessionCount())
	assert.True(t, mockConn.CloseCalled())
}
