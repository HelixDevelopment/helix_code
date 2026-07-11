package acp

import (
	"context"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"

	"dev.helix.code/internal/version"
)

// AgentName is the ACP `agentInfo.name` HelixCode reports during
// initialize. It is a stable, protocol-facing identifier, distinct from any
// user-facing display name.
const AgentName = "helixcode"

// sessionState is the real (not fake) per-session state this scaffold
// tracks. It intentionally holds only what session/new, session/close, and
// the not-yet-wired Prompt honest-error path need in this phase; the fields
// a wired Prompt (HXC-119 Phase 4) will need (provider handle, conversation
// history, etc.) are added when that phase lands, not speculatively here.
type sessionState struct {
	cwd                   string
	additionalDirectories []string
}

// Agent implements github.com/coder/acp-go-sdk's acp.Agent interface for
// HelixCode. See package doc.go for the Phase 1-3 scope boundary: real
// handshake + real session tracking, honest (non-fabricated) Prompt
// rejection, no permission-request handling yet.
type Agent struct {
	mu       sync.Mutex
	sessions map[acpsdk.SessionId]*sessionState
}

var _ acpsdk.Agent = (*Agent)(nil)

// NewAgent constructs a HelixCode ACP agent with empty, real (not
// pre-seeded/fake) session state.
func NewAgent() *Agent {
	return &Agent{
		sessions: make(map[acpsdk.SessionId]*sessionState),
	}
}

// Initialize negotiates the ACP protocol version and reports HelixCode's
// real, current agent capabilities. No capability is advertised that this
// phase does not genuinely support (loadSession is false; prompt/session
// capabilities are left at their SDK zero-value defaults) — this is a
// deliberate honesty constraint, not an oversight: advertising a capability
// this scaffold cannot yet honor would itself be a bluff at the protocol
// layer.
func (a *Agent) Initialize(_ context.Context, params acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	protocolVersion := acpsdk.ProtocolVersionNumber
	if params.ProtocolVersion != 0 && int(params.ProtocolVersion) < protocolVersion {
		protocolVersion = int(params.ProtocolVersion)
	}
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersion(protocolVersion),
		AgentInfo: &acpsdk.Implementation{
			Name:    AgentName,
			Version: version.Version,
		},
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: false,
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

// Authenticate is not yet wired to any real HelixCode auth flow in this
// phase (no auth method is advertised by Initialize, so a compliant client
// should never call this) — returns an honest method-not-found error rather
// than fabricating a successful authentication.
func (a *Agent) Authenticate(_ context.Context, _ acpsdk.AuthenticateRequest) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, acpsdk.NewMethodNotFound("authenticate")
}

// Logout is a no-op that succeeds honestly: this scaffold does not yet
// track any authenticated state (see Authenticate), so there is nothing to
// tear down, and reporting success is accurate, not fabricated.
func (a *Agent) Logout(_ context.Context, _ acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

// NewSession creates and tracks REAL per-session state keyed by a
// cryptographically-random UUID (github.com/google/uuid), not an
// incrementing counter or fixed placeholder.
func (a *Agent) NewSession(_ context.Context, params acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	if err := params.Validate(); err != nil {
		return acpsdk.NewSessionResponse{}, acpsdk.NewInvalidParams(map[string]any{"error": err.Error()})
	}
	id := acpsdk.SessionId(uuid.NewString())
	a.mu.Lock()
	a.sessions[id] = &sessionState{
		cwd:                   params.Cwd,
		additionalDirectories: params.AdditionalDirectories,
	}
	a.mu.Unlock()
	return acpsdk.NewSessionResponse{SessionId: id}, nil
}

// Prompt looks up the real tracked session (rejecting unknown session ids
// exactly as a correct implementation must) but does not yet route the
// prompt through HelixCode's real LLM provider path — that wiring
// (session/update notifications over the existing GenerateStream call) is
// HXC-119 Phase 4 (docs/research/const040_capability_model_20260712/DESIGN.md
// §4.4.4), which also needs its own regression-call-graph review before it
// touches cmd/cli/main.go's real-generation handlers. Returning a real
// protocol error here — instead of a plausible-looking but unwired
// completion — is the anti-bluff-correct behavior for an unimplemented
// capability (CLAUDE.md §3.3 / Article XI §11.9): callers get an honest
// failure, never a response that looks like real model output but is not.
func (a *Agent) Prompt(_ context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	a.mu.Lock()
	_, ok := a.sessions[params.SessionId]
	a.mu.Unlock()
	if !ok {
		return acpsdk.PromptResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error":     "unknown session id",
			"sessionId": string(params.SessionId),
		})
	}
	return acpsdk.PromptResponse{}, acpsdk.NewInternalError(map[string]any{
		"reason": "ACP prompt turn generation is not wired in this HelixCode release (tracked: HXC-119 Phase 4)",
	})
}

// Cancel is a real, side-effect-honest no-op in this phase: since Prompt
// never starts a real long-running turn yet (see Prompt above), there is
// genuinely nothing in flight to cancel.
func (a *Agent) Cancel(_ context.Context, _ acpsdk.CancelNotification) error {
	return nil
}

// CloseSession removes the real tracked session state so a subsequent
// Prompt against the closed session id is genuinely rejected as unknown.
func (a *Agent) CloseSession(_ context.Context, params acpsdk.CloseSessionRequest) (acpsdk.CloseSessionResponse, error) {
	a.mu.Lock()
	delete(a.sessions, params.SessionId)
	a.mu.Unlock()
	return acpsdk.CloseSessionResponse{}, nil
}

// ListSessions is not yet wired (Initialize does not advertise
// sessionCapabilities.list) — honest method-not-found rather than an empty
// list dressed up as a real answer.
func (a *Agent) ListSessions(_ context.Context, _ acpsdk.ListSessionsRequest) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{}, acpsdk.NewMethodNotFound("session/list")
}

// ResumeSession is not yet wired (Initialize does not advertise
// sessionCapabilities.resume).
func (a *Agent) ResumeSession(_ context.Context, _ acpsdk.ResumeSessionRequest) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, acpsdk.NewMethodNotFound("session/resume")
}

// SetSessionConfigOption is not yet wired: this phase reports no config
// options from NewSession/ResumeSession, so there is nothing real to set.
func (a *Agent) SetSessionConfigOption(_ context.Context, _ acpsdk.SetSessionConfigOptionRequest) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{}, acpsdk.NewMethodNotFound("session/set_config_option")
}

// SetSessionMode is not yet wired: this phase reports no session modes.
func (a *Agent) SetSessionMode(_ context.Context, _ acpsdk.SetSessionModeRequest) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, acpsdk.NewMethodNotFound("session/set_mode")
}
