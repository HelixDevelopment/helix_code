package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/version"
)

// AgentName is the ACP `agentInfo.name` HelixCode reports during
// initialize. It is a stable, protocol-facing identifier, distinct from any
// user-facing display name.
const AgentName = "helixcode"

// sessionState is the real (not fake) per-session state this scaffold
// tracks. cancel (HXC-119 Phase 4) holds the CancelFunc for whatever turn is
// currently in flight for this session, so a real ACP `session/cancel`
// notification can actually stop real generation — nil when no turn is in
// flight, exactly mirroring the reference agent shape in
// github.com/coder/acp-go-sdk's own example/agent/main.go.
type sessionState struct {
	cwd                   string
	additionalDirectories []string
	cancel                context.CancelFunc
}

// Agent implements github.com/coder/acp-go-sdk's acp.Agent interface for
// HelixCode. See package doc.go for the current scope boundary: real
// handshake + real session tracking + Prompt turn generation wired to
// HelixCode's real LLM provider (HXC-119 Phase 4) + permission-request
// handling mapped onto internal/tools/permissions (HXC-119 Phase 5,
// Option B — tool-execution gate).
type Agent struct {
	mu       sync.Mutex
	sessions map[acpsdk.SessionId]*sessionState
	// provider is the REAL LLM provider Prompt delegates generation to (the
	// same llm.Provider interface cmd/cli/main.go:handleGenerate's streaming
	// branch calls via provider.GenerateStream — see Prompt below). Injected
	// by the caller (cmd/cli/acp_cmd.go's newACPCommand in production, a
	// test double in unit tests per §11.4.27) rather than constructed here,
	// so this package stays decoupled from provider-selection policy.
	provider llm.Provider
	// conn is the live agent-side connection Prompt uses to emit real
	// `session/update` notifications as generation streams in. It is nil
	// until SetConnection is called (the caller wires it immediately after
	// acpsdk.NewAgentSideConnection, mirroring the SDK's own
	// example/agent/main.go SetAgentConnection convention) — Prompt reports
	// an honest error rather than silently dropping stream output if a
	// caller forgets this step.
	conn *acpsdk.AgentSideConnection
	// perm bridges ACP's session/request_permission onto HelixCode's
	// internal/tools/permissions engine (HXC-119 Phase 5, Option B).
	// Injected by the caller; nil → all requests default to ActionAsk
	// (fail-closed, never auto-approve).
	perm *PermissionAdapter
}

var _ acpsdk.Agent = (*Agent)(nil)

// NewAgent constructs a HelixCode ACP agent with empty, real (not
// pre-seeded/fake) session state, wired to generate real completions via
// provider. provider must be a real dev.helix.code/internal/llm.Provider
// (a real cloud/local provider in production; a unit-test-only double,
// per §11.4.27, is acceptable ONLY inside *_test.go files) — Prompt never
// fabricates output regardless of what provider does internally.
func NewAgent(provider llm.Provider) *Agent {
	return &Agent{
		sessions: make(map[acpsdk.SessionId]*sessionState),
		provider: provider,
	}
}

// SetConnection wires the live agent-side connection Prompt uses to emit
// `session/update` notifications. The caller (cmd/cli/acp_cmd.go's
// newACPCommand) MUST call this immediately after
// acpsdk.NewAgentSideConnection returns, before the connection starts
// dispatching incoming requests — the same ordering the SDK's own
// example/agent/main.go uses via its SetAgentConnection method.
func (a *Agent) SetConnection(conn *acpsdk.AgentSideConnection) {
	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()
}

// SetPermissionAdapter wires the ACP permission adapter (HXC-119 Phase 5).
// If not called, all permission requests default to ActionAsk (fail-closed).
func (a *Agent) SetPermissionAdapter(perm *PermissionAdapter) {
	a.mu.Lock()
	a.perm = perm
	a.mu.Unlock()
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
// exactly as a correct implementation must) and routes the prompt through
// HelixCode's REAL LLM provider path: it builds an llm.LLMRequest from the
// prompt's text content blocks and calls provider.GenerateStream — the
// SAME Provider.GenerateStream method cmd/cli/main.go:handleGenerate's
// streaming branch calls against real providers (BLUFF-001-clean: no
// simulation, no fabricated tokens). Each non-empty streamed chunk is
// forwarded to the ACP client as a real `session/update`
// agent_message_chunk notification via a.conn.SessionUpdate, so a
// connected editor sees genuine incremental model output, not a
// batched-then-replayed illusion of streaming.
//
// Permission-request handling (mapping ACP's `session/request_permission`
// onto HelixCode's internal/approval / internal/tools/permissions) remains
// deliberately unimplemented — that is HXC-119 Phase 5
// (docs/research/const040_capability_model_20260712/DESIGN.md §4.4.5),
// explicitly flagged medium-high risk and scoped to its own security-focused
// review pass. This method never needs to request permission because it
// does not yet perform any tool-call/file-write action on the client's
// behalf — it only streams model text.
func (a *Agent) Prompt(ctx context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	a.mu.Lock()
	_, ok := a.sessions[params.SessionId]
	conn := a.conn
	provider := a.provider
	a.mu.Unlock()
	if !ok {
		return acpsdk.PromptResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error":     "unknown session id",
			"sessionId": string(params.SessionId),
		})
	}
	if provider == nil {
		return acpsdk.PromptResponse{}, acpsdk.NewInternalError(map[string]any{
			"reason": "ACP agent has no LLM provider configured",
		})
	}
	if conn == nil {
		return acpsdk.PromptResponse{}, acpsdk.NewInternalError(map[string]any{
			"reason": "ACP agent-side connection is not wired (SetConnection was never called)",
		})
	}

	promptText := flattenPromptText(params.Prompt)
	if promptText == "" {
		return acpsdk.PromptResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error": "prompt contained no text (or resource_link) content this agent can act on",
		})
	}

	modelName := ""
	if models := provider.GetModels(); len(models) > 0 {
		modelName = models[0].Name
	}

	turnCtx, cancel := context.WithCancel(ctx)
	a.mu.Lock()
	if st, tracked := a.sessions[params.SessionId]; tracked {
		st.cancel = cancel
	}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		if st, tracked := a.sessions[params.SessionId]; tracked && st.cancel != nil {
			st.cancel = nil
		}
		a.mu.Unlock()
		cancel()
	}()

	req := &llm.LLMRequest{
		ID:     uuid.New(),
		Model:  modelName,
		Stream: true,
		Messages: []llm.Message{
			{Role: "user", Content: promptText},
		},
	}

	chunkChan := make(chan llm.LLMResponse, 100)
	errCh := make(chan error, 1)
	go func() {
		errCh <- provider.GenerateStream(turnCtx, req, chunkChan)
	}()

	var streamWarning error
	for chunk := range chunkChan {
		if chunk.Err != nil {
			// A non-nil chunk.Err (round-46 partial-error frame, e.g.
			// truncation or a content-safety block) is NOT itself a
			// protocol failure — Content may still carry real partial
			// output that MUST reach the client (see llm.LLMResponse.Err's
			// doc comment). Remember the most-recent one so the final
			// PromptResponse.StopReason can honestly reflect it.
			streamWarning = chunk.Err
		}
		if chunk.Content == "" {
			continue
		}
		if updErr := conn.SessionUpdate(turnCtx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(chunk.Content),
		}); updErr != nil {
			return acpsdk.PromptResponse{}, acpsdk.NewInternalError(map[string]any{
				"reason": fmt.Sprintf("failed to emit session/update: %v", updErr),
			})
		}
	}

	genErr := <-errCh
	if genErr != nil {
		if turnCtx.Err() != nil {
			// The context was cancelled — either by a real Cancel
			// notification (see Cancel below) or by the caller's ctx. This
			// is the honest StopReasonCancelled path, not a fabricated
			// success.
			return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonCancelled}, nil
		}
		return acpsdk.PromptResponse{}, acpsdk.NewInternalError(map[string]any{
			"reason": fmt.Sprintf("generation failed: %v", genErr),
		})
	}

	switch {
	case errors.Is(streamWarning, llm.ErrResponseTruncated):
		return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonMaxTokens}, nil
	case errors.Is(streamWarning, llm.ErrResponseContentBlocked):
		return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonRefusal}, nil
	default:
		return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
	}
}

// flattenPromptText extracts the real text content HelixCode can act on
// from an ACP prompt's content blocks. Per the protocol baseline (see
// acpsdk.PromptRequest's doc comment) every agent MUST support
// ContentBlock::Text and ContentBlock::ResourceLink; this scaffold does not
// yet resolve resource_link targets into real file content (no RAG/context
// pipeline is wired — see HXC-118), so a resource_link is honestly
// represented as a bracketed reference rather than silently dropped or
// fabricated. Image/Audio/Resource(embedded) blocks are skipped: Initialize
// does not advertise the corresponding PromptCapabilities, so a compliant
// client MUST NOT send them, and this scaffold has no vision/audio
// generation path to honor them if one did.
func flattenPromptText(blocks []acpsdk.ContentBlock) string {
	var b strings.Builder
	for _, block := range blocks {
		switch {
		case block.Text != nil:
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(block.Text.Text)
		case block.ResourceLink != nil:
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "[resource: %s (%s)]", block.ResourceLink.Name, block.ResourceLink.Uri)
		}
	}
	return b.String()
}

// Cancel stops a REAL in-flight turn: if params.SessionId names a session
// with a Prompt call currently streaming (see Prompt's turnCtx wiring
// above), Cancel invokes that turn's real context.CancelFunc, which
// unblocks provider.GenerateStream via ctx.Done() and causes the pending
// Prompt call to return PromptResponse{StopReason: StopReasonCancelled}.
// Cancel is a genuine no-op (not a bluff) only when there is truly nothing
// in flight for the session — an unknown session id or a session with no
// active turn.
func (a *Agent) Cancel(_ context.Context, params acpsdk.CancelNotification) error {
	a.mu.Lock()
	st, ok := a.sessions[params.SessionId]
	a.mu.Unlock()
	if !ok || st == nil {
		return nil
	}
	a.mu.Lock()
	cancel := st.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
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
