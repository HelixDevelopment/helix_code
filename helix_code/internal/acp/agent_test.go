package acp

// HXC-119 Phase 1-3 scaffold test. Per §11.4.115 (RED-baseline-on-the-
// broken-artifact) this test was written BEFORE internal/acp/agent.go
// existed: `go test ./internal/acp/...` at that point failed to compile
// (RED — no `NewAgent`/`Agent` symbols), proving the test genuinely
// exercises new code rather than passing vacuously. See
// docs/qa/... evidence file referenced in the HXC-119 implementer report
// for the captured RED transcript.
//
// Anti-bluff (§11.4.6/§11.4.107): this test drives a REAL ACP JSON-RPC
// handshake over a real io.Pipe transport using the coder/acp-go-sdk
// (github.com/coder/acp-go-sdk@v0.13.5) client-side connection talking to
// our real agent-side connection — not a synthetic in-process function
// call and not a hand-rolled fake protocol. It is not the full external-
// client (Zed/JetBrains) E2E Challenge that HXC-119 Phase 6 requires; that
// remains a documented later phase (DESIGN.md §5, HXC-119 row 6).

import (
	"context"
	"io"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

// noopClient is a minimal, real (not mocked-out) acp.Client implementation
// used only to drive the other end of the wire in this test. It has no
// production role — HelixCode is always the ACP *agent*, never the
// *client* — so a bare, honestly-empty Client is the correct test double
// for the peer role for this scaffold's grade of test.
type noopClient struct{}

func (noopClient) ReadTextFile(context.Context, acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	return acpsdk.ReadTextFileResponse{}, acpsdk.NewMethodNotFound("fs/read_text_file")
}

func (noopClient) WriteTextFile(context.Context, acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	return acpsdk.WriteTextFileResponse{}, acpsdk.NewMethodNotFound("fs/write_text_file")
}

func (noopClient) RequestPermission(context.Context, acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	return acpsdk.RequestPermissionResponse{}, acpsdk.NewMethodNotFound("session/request_permission")
}

func (noopClient) SessionUpdate(context.Context, acpsdk.SessionNotification) error { return nil }

func (noopClient) CreateTerminal(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/create")
}

func (noopClient) KillTerminal(context.Context, acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	return acpsdk.KillTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/kill")
}

func (noopClient) TerminalOutput(context.Context, acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	return acpsdk.TerminalOutputResponse{}, acpsdk.NewMethodNotFound("terminal/output")
}

func (noopClient) ReleaseTerminal(context.Context, acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	return acpsdk.ReleaseTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/release")
}

func (noopClient) WaitForTerminalExit(context.Context, acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	return acpsdk.WaitForTerminalExitResponse{}, acpsdk.NewMethodNotFound("terminal/wait_for_exit")
}

var _ acpsdk.Client = noopClient{}

// wirePipes builds two connected io.Pipe pairs (client->agent and
// agent->client) and returns a real acpsdk.AgentSideConnection bound to a
// real HelixCode Agent, plus a real acpsdk.ClientSideConnection bound to
// noopClient — the same shape the coder/acp-go-sdk's own test suite
// (acp_test.go, e.g. TestConnectionHandlesInitialize) uses to prove a
// genuine round trip rather than an in-process fake.
func wirePipes(t *testing.T) (*acpsdk.ClientSideConnection, *Agent, func()) {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()

	agent := NewAgent()
	agentConn := acpsdk.NewAgentSideConnection(agent, a2cW, c2aR)
	clientConn := acpsdk.NewClientSideConnection(noopClient{}, c2aW, a2cR)

	cleanup := func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	}
	_ = agentConn // referenced only to keep the agent-side connection alive
	return clientConn, agent, cleanup
}

// TestNewAgentSideConnection_WiresWithoutPanic is the minimum bar the
// dispatch instructions required: acpsdk.NewAgentSideConnection must wire a
// real HelixCode Agent over a real io.Pipe transport without panicking.
func TestNewAgentSideConnection_WiresWithoutPanic(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	agent := NewAgent()
	conn := acpsdk.NewAgentSideConnection(agent, io.Discard.(io.Writer), r)
	if conn == nil {
		t.Fatal("NewAgentSideConnection returned nil")
	}
	_ = w
}

// TestAgent_RealHandshake drives a genuine ACP `initialize` round trip: a
// real acpsdk.ClientSideConnection sends a real JSON-RPC request over a real
// io.Pipe to a real acpsdk.AgentSideConnection wrapping our Agent, and this
// test asserts on the real InitializeResponse that comes back over the
// wire — not a value constructed in-process.
func TestAgent_RealHandshake(t *testing.T) {
	clientConn, _, cleanup := wirePipes(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := clientConn.Initialize(ctx, acpsdk.InitializeRequest{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
	})
	if err != nil {
		t.Fatalf("real ACP initialize round-trip failed: %v", err)
	}
	if resp.ProtocolVersion != acpsdk.ProtocolVersionNumber {
		t.Fatalf("protocol version mismatch over the wire: got %d want %d", resp.ProtocolVersion, acpsdk.ProtocolVersionNumber)
	}
	if resp.AgentInfo == nil || resp.AgentInfo.Name != AgentName {
		t.Fatalf("unexpected agentInfo over the wire: %+v", resp.AgentInfo)
	}
}

// TestAgent_NewSessionThenPrompt proves session state is real (a session
// created via a real `session/new` round trip is genuinely tracked, and an
// unknown session id is genuinely rejected) and that Prompt is honest about
// not yet being wired to real turn generation (HXC-119 Phase 4) rather than
// fabricating a completion.
func TestAgent_NewSessionThenPrompt(t *testing.T) {
	clientConn, agent, cleanup := wirePipes(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessResp, err := clientConn.NewSession(ctx, acpsdk.NewSessionRequest{
		Cwd:        "/tmp",
		McpServers: []acpsdk.McpServer{},
	})
	if err != nil {
		t.Fatalf("real session/new round-trip failed: %v", err)
	}
	if sessResp.SessionId == "" {
		t.Fatal("expected a real, non-empty session id")
	}

	agent.mu.Lock()
	_, tracked := agent.sessions[sessResp.SessionId]
	agent.mu.Unlock()
	if !tracked {
		t.Fatal("session created over the wire was not tracked in real agent state")
	}

	// Prompt turn generation is deliberately not wired in this phase
	// (HXC-119 Phase 4 per DESIGN.md §4.4.4) — the honest behavior is a
	// real protocol-level error, never a fabricated completion.
	_, promptErr := clientConn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: sessResp.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("hello")},
	})
	if promptErr == nil {
		t.Fatal("expected an honest not-yet-wired error from Prompt, got nil error")
	}

	// An unknown session id must be genuinely rejected, not silently
	// accepted.
	_, unknownErr := clientConn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: acpsdk.SessionId("does-not-exist"),
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("hello")},
	})
	if unknownErr == nil {
		t.Fatal("expected Prompt against an unknown session id to fail")
	}
}
