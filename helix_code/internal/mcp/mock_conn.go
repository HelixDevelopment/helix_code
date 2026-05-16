package mcp

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MockConn is a mock WebSocket connection for testing.
// All fields are protected by mu so it is safe to use from concurrent goroutines
// (e.g. BroadcastNotification spawns a goroutine per session).
type MockConn struct {
	mu           sync.Mutex
	writeCalled  bool
	lastMessage  []byte
	closeCalled  bool
	readMessages []interface{}
}

// WriteCalled returns whether WriteJSON or WriteMessage has been called.
func (m *MockConn) WriteCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCalled
}

// LastMessage returns a copy of the last message written.
func (m *MockConn) LastMessage() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]byte, len(m.lastMessage))
	copy(out, m.lastMessage)
	return out
}

// CloseCalled returns whether Close has been called.
func (m *MockConn) CloseCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalled
}

// Ensure MockConn implements WebSocketConn interface
func (m *MockConn) ReadJSON(v interface{}) error {
	m.mu.Lock()
	if len(m.readMessages) == 0 {
		m.mu.Unlock()
		return &websocket.CloseError{Code: websocket.CloseNormalClosure}
	}
	// Pop the first message
	msg := m.readMessages[0]
	m.readMessages = m.readMessages[1:]
	m.mu.Unlock()

	// Convert message to JSON bytes and unmarshal
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, v)
}

func (m *MockConn) WriteJSON(v interface{}) error {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.writeCalled = true
	m.lastMessage = jsonBytes
	m.mu.Unlock()
	return nil
}

func (m *MockConn) Close() error {
	m.mu.Lock()
	m.closeCalled = true
	m.mu.Unlock()
	return nil
}

func (m *MockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) SetPongHandler(h func(string) error) {
	// No-op for testing
}

func (m *MockConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	m.writeCalled = true
	m.lastMessage = data
	m.mu.Unlock()
	return nil
}
