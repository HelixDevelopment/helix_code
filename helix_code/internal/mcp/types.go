package mcp

import (
	"errors"
	"fmt"
	"sync/atomic"
)

// TransportType enumerates supported client-side transports.
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportHTTP  TransportType = "http"
	TransportSSE   TransportType = "sse"
	TransportWS    TransportType = "ws"
)

func (t TransportType) Validate() error {
	switch t {
	case TransportStdio, TransportHTTP, TransportSSE, TransportWS:
		return nil
	default:
		return fmt.Errorf("mcp: invalid transport %q (want stdio|http|sse|ws)", string(t))
	}
}

// ClientState is the high-level lifecycle state for a client connection.
type ClientState int32

const (
	StateDisconnected ClientState = iota
	StateConnecting
	StateInitializing
	StateReady
	StateReconnecting
	StateClosed
)

func (s ClientState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateInitializing:
		return "initializing"
	case StateReady:
		return "ready"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return fmt.Sprintf("unknown(%d)", int32(s))
	}
}

// loadState atomically reads the state from a *atomic.Int32 backing store.
func loadState(p *atomic.Int32) ClientState {
	return ClientState(p.Load())
}

// storeState atomically stores the state.
func storeState(p *atomic.Int32, s ClientState) {
	p.Store(int32(s))
}

// Event represents a lifecycle event emitted by a Client.
type Event struct {
	Server string
	State  ClientState
	Err    error
	Msg    string
}

// Client-side error sentinels.
var (
	ErrServerNotFound  = errors.New("mcp: server not found")
	ErrNotReady        = errors.New("mcp: client not ready")
	ErrReconnect       = errors.New("mcp: transport reconnecting")
	ErrInitFailed      = errors.New("mcp: initialize handshake failed")
	ErrTransportClosed = errors.New("mcp: transport closed")
	ErrOAuthRequired   = errors.New("mcp: oauth token missing or invalid; run 'helixcode mcp auth'")
	ErrToolNotFound    = errors.New("mcp: tool not found on server")
	ErrProtocol        = errors.New("mcp: protocol violation")
	ErrTooManyPending  = errors.New("mcp: too many pending requests")
)
