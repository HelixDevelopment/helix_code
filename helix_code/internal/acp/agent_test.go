package acp

// HXC-119 Phase 1-3 scaffold test, extended for Phase 4 (Prompt turn
// generation wired to a real llm.Provider). Per §11.4.115 (RED-baseline-on-
// the-broken-artifact): the Phase 1-3 portion of this test was written
// BEFORE internal/acp/agent.go existed, and `go test ./internal/acp/...` at
// that point failed to compile (RED — no `NewAgent`/`Agent` symbols),
// proving the test genuinely exercises new code rather than passing
// vacuously. The Phase 4 additions below were, in turn, written against the
// Phase 1-3 Agent (whose Prompt always returned an honest "not wired"
// protocol error regardless of session validity) — running them against
// that implementation is a genuine RED (TestAgent_PromptDelegatesToRealGenerateStream
// fails: it asserts a successful PromptResponse carrying the fake
// provider's real streamed text, which the Phase 1-3 Prompt never
// produces). See docs/scratch/discovery/fixes/W4_119_evidence.md for the
// captured RED -> GREEN transcript.
//
// Anti-bluff (§11.4.6/§11.4.107): every test here drives a REAL ACP
// JSON-RPC round trip over a real io.Pipe transport using the
// coder/acp-go-sdk (github.com/coder/acp-go-sdk@v0.13.5) client-side
// connection talking to our real agent-side connection — never a synthetic
// in-process function call and never a hand-rolled fake protocol. The
// Phase 4 tests additionally prove Agent.Prompt GENUINELY delegates to
// llm.Provider.GenerateStream: the fake provider (§11.4.27(A) permits fakes
// ONLY in unit tests) emits content chosen by the TEST at run time, and the
// assertions check that EXACT content arrived over the real ACP
// session/update wire — a hardcoded string in Prompt could not pass this
// (see TestAgent_PromptDelegatesToRealGenerateStream). It is not the full
// external-client (Zed/JetBrains) E2E Challenge that HXC-119 Phase 6
// requires; that remains a documented later phase (DESIGN.md §5, HXC-119
// row 6).

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"

	"dev.helix.code/internal/llm"
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

// recordingClient embeds noopClient (for every peer-side ACP method this
// scaffold does not yet exercise) and overrides SessionUpdate to capture
// the real text of every agent_message_chunk HelixCode streams over the
// wire, so tests can assert on genuine, provider-produced content instead
// of merely "Prompt returned no error".
type recordingClient struct {
	noopClient
	mu     sync.Mutex
	chunks []string
}

func (c *recordingClient) SessionUpdate(_ context.Context, params acpsdk.SessionNotification) error {
	if params.Update.AgentMessageChunk != nil && params.Update.AgentMessageChunk.Content.Text != nil {
		c.mu.Lock()
		c.chunks = append(c.chunks, params.Update.AgentMessageChunk.Content.Text.Text)
		c.mu.Unlock()
	}
	return nil
}

func (c *recordingClient) received() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.chunks))
	copy(out, c.chunks)
	return out
}

var _ acpsdk.Client = (*recordingClient)(nil)

// fakeStreamingProvider is a unit-test-only llm.Provider double
// (§11.4.27(A) permits mocks/fakes ONLY in unit tests). It exists to prove
// Agent.Prompt GENUINELY delegates to Provider.GenerateStream: the content
// it streams is chosen by each test at construction time and is asserted
// against verbatim on the OTHER side of a real ACP wire round trip — a
// hardcoded string inside Prompt could not satisfy that assertion, so a
// pass here is real evidence of wiring, not a bluff.
type fakeStreamingProvider struct {
	mu     sync.Mutex
	chunks []string
	// chunkErr, when set, is emitted as a FINAL LLMResponse.Err-bearing
	// chunk (empty Content) — matching how real in-tree providers deliver a
	// round-46 partial-error sentinel (see
	// internal/llm/llamacpp_provider.go's GenerateStream: the sentinel
	// travels on ch, the function's own return value is
	// scanner.Err()/nil). This is intentionally NOT the function's return
	// value: conflating the two would mis-model every real provider this
	// fake stands in for.
	chunkErr    error
	block       <-chan struct{} // when non-nil, GenerateStream waits on this (or ctx.Done()) before streaming anything
	lastRequest *llm.LLMRequest
}

func (p *fakeStreamingProvider) GetType() llm.ProviderType { return llm.ProviderType("fake-test-only") }
func (p *fakeStreamingProvider) GetName() string           { return "fake-test-only" }
func (p *fakeStreamingProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "fake-model", Name: "fake-model"}}
}
func (p *fakeStreamingProvider) GetCapabilities() []llm.ModelCapability { return nil }

func (p *fakeStreamingProvider) Generate(_ context.Context, _ *llm.LLMRequest) (*llm.LLMResponse, error) {
	return nil, errors.New("fakeStreamingProvider: Generate is not used by ACP Prompt (streaming-only path)")
}

func (p *fakeStreamingProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	defer close(ch)

	p.mu.Lock()
	p.lastRequest = req
	chunks := append([]string(nil), p.chunks...)
	chunkErr := p.chunkErr
	block := p.block
	p.mu.Unlock()

	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	for _, c := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- llm.LLMResponse{ID: uuid.New(), RequestID: req.ID, Content: c, CreatedAt: time.Now()}:
		}
	}
	if chunkErr != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- llm.LLMResponse{ID: uuid.New(), RequestID: req.ID, CreatedAt: time.Now(), Err: chunkErr}:
		}
	}
	return nil
}

func (p *fakeStreamingProvider) IsAvailable(_ context.Context) bool { return true }
func (p *fakeStreamingProvider) GetHealth(_ context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "ok"}, nil
}
func (p *fakeStreamingProvider) Close() error                         { return nil }
func (p *fakeStreamingProvider) GetContextWindow() int                { return 4096 }
func (p *fakeStreamingProvider) CountTokens(text string) (int, error) { return len(text) / 4, nil }

func (p *fakeStreamingProvider) requestSeen() *llm.LLMRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastRequest
}

var _ llm.Provider = (*fakeStreamingProvider)(nil)

// wirePipesWithClient builds two connected io.Pipe pairs (client->agent and
// agent->client) and returns a real acpsdk.ClientSideConnection bound to
// client, plus the real HelixCode *Agent (constructed with provider and
// wired to its own real acpsdk.AgentSideConnection via SetConnection,
// exactly mirroring the ordering cmd/cli/acp_cmd.go's newACPCommand uses in
// production) — the same shape the coder/acp-go-sdk's own test suite
// (acp_test.go, e.g. TestConnectionHandlesInitialize) uses to prove a
// genuine round trip rather than an in-process fake.
func wirePipesWithClient(t *testing.T, provider llm.Provider, client acpsdk.Client) (*acpsdk.ClientSideConnection, *Agent, func()) {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()

	agent := NewAgent(provider)
	agentConn := acpsdk.NewAgentSideConnection(agent, a2cW, c2aR)
	agent.SetConnection(agentConn)
	clientConn := acpsdk.NewClientSideConnection(client, c2aW, a2cR)

	cleanup := func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	}
	return clientConn, agent, cleanup
}

// wirePipes is wirePipesWithClient with the honestly-empty noopClient as
// the peer — used by tests that only exercise the handshake / session
// lifecycle and do not need to observe session/update content.
func wirePipes(t *testing.T, provider llm.Provider) (*acpsdk.ClientSideConnection, *Agent, func()) {
	t.Helper()
	return wirePipesWithClient(t, provider, noopClient{})
}

// TestNewAgentSideConnection_WiresWithoutPanic is the minimum bar the
// dispatch instructions required: acpsdk.NewAgentSideConnection must wire a
// real HelixCode Agent over a real io.Pipe transport without panicking. No
// provider is needed for this assertion (the connection never calls
// Prompt), so nil is honest here rather than a throwaway fake.
func TestNewAgentSideConnection_WiresWithoutPanic(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	agent := NewAgent(nil)
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
	clientConn, _, cleanup := wirePipes(t, nil)
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

// TestAgent_NewSessionThenPromptHonestErrors proves session state is real
// (a session created via a real `session/new` round trip is genuinely
// tracked) and that Prompt is honest about missing preconditions rather
// than fabricating a completion: an unknown session id is genuinely
// rejected, and a tracked session with no LLM provider configured returns a
// real protocol error (never a fake success).
func TestAgent_NewSessionThenPromptHonestErrors(t *testing.T) {
	clientConn, agent, cleanup := wirePipes(t, nil)
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

	// No provider is wired in this test (wirePipes(t, nil)) — the honest
	// behavior is a real protocol-level error, never a fabricated
	// completion.
	_, promptErr := clientConn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: sessResp.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("hello")},
	})
	if promptErr == nil {
		t.Fatal("expected an honest no-provider-configured error from Prompt, got nil error")
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

// TestAgent_PromptDelegatesToRealGenerateStream is the HXC-119 Phase 4
// anti-bluff core: it proves Agent.Prompt genuinely delegates to
// llm.Provider.GenerateStream (the SAME interface method
// cmd/cli/main.go:handleGenerate's streaming branch calls against real
// providers), in both directions:
//
//  1. Forward (ACP prompt -> LLM request): the fakeStreamingProvider
//     records the *llm.LLMRequest it actually received, and this test
//     asserts its Messages[0].Content equals the exact text this test sent
//     as the ACP prompt's TextBlock — proving Prompt genuinely builds the
//     LLM request from the real ACP prompt content, not a canned request.
//  2. Backward (LLM stream -> ACP session/update): the fakeStreamingProvider
//     streams distinctive, test-chosen content, and this test asserts the
//     recordingClient on the OTHER end of a real ACP wire round trip
//     received EXACTLY that content as real session/update
//     agent_message_chunk notifications — a value Prompt could not have
//     hardcoded, because it was chosen by this test at run time.
//
// This is the test that is genuinely RED against the Phase 1-3 Agent.Prompt
// (which always returns an honest "not wired" protocol error regardless of
// session validity): it would fail both the PromptResponse.StopReason
// assertion and the recordingClient content assertion.
func TestAgent_PromptDelegatesToRealGenerateStream(t *testing.T) {
	const wantPromptText = "HXC-119-PHASE-4-FORWARD-PROBE-3f9a"
	wantChunks := []string{"REAL-STREAM-CHUNK-A-7c1", "REAL-STREAM-CHUNK-B-9d2", "REAL-STREAM-CHUNK-C-e01"}

	provider := &fakeStreamingProvider{chunks: wantChunks}
	client := &recordingClient{}
	clientConn, _, cleanup := wirePipesWithClient(t, provider, client)
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

	promptResp, err := clientConn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: sessResp.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(wantPromptText)},
	})
	if err != nil {
		t.Fatalf("real ACP prompt round-trip failed: %v", err)
	}
	if promptResp.StopReason != acpsdk.StopReasonEndTurn {
		t.Fatalf("StopReason = %q, want %q (real provider stream completed cleanly)", promptResp.StopReason, acpsdk.StopReasonEndTurn)
	}

	// Forward direction: the LLM request the fake provider actually
	// received must carry the EXACT text this test sent as the ACP prompt —
	// proving genuine translation, not a hardcoded/canned request.
	seen := provider.requestSeen()
	if seen == nil {
		t.Fatal("provider.GenerateStream was never called — Prompt did not delegate")
	}
	if len(seen.Messages) != 1 || seen.Messages[0].Content != wantPromptText {
		t.Fatalf("LLM request built from ACP prompt = %+v, want single user message with Content=%q", seen.Messages, wantPromptText)
	}
	if !seen.Stream {
		t.Fatal("expected Prompt to request a streaming completion (Stream=true)")
	}

	// Backward direction: the real ACP client on the other end of the wire
	// must have received EXACTLY the provider's real streamed chunks as
	// session/update agent_message_chunk notifications.
	got := client.received()
	if len(got) != len(wantChunks) {
		t.Fatalf("received %d session/update chunks over the real wire, want %d: got=%v", len(got), len(wantChunks), got)
	}
	for i, want := range wantChunks {
		if got[i] != want {
			t.Fatalf("session/update chunk[%d] = %q, want %q (real provider-streamed content)", i, got[i], want)
		}
	}
}

// TestAgent_PromptTruncatedStopReason proves a round-46
// llm.ErrResponseTruncated partial-error frame from the real provider is
// honestly reflected as StopReasonMaxTokens (never silently swallowed into
// a plain "end_turn" success) AND that the partial content the provider did
// produce still reaches the client — matching llm.LLMResponse.Err's own
// doc-comment contract ("Content may still hold a valid partial output").
func TestAgent_PromptTruncatedStopReason(t *testing.T) {
	provider := &fakeStreamingProvider{chunks: []string{"partial output"}, chunkErr: llm.ErrResponseTruncated}
	client := &recordingClient{}
	clientConn, _, cleanup := wirePipesWithClient(t, provider, client)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessResp, err := clientConn.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: "/tmp", McpServers: []acpsdk.McpServer{}})
	if err != nil {
		t.Fatalf("real session/new round-trip failed: %v", err)
	}

	promptResp, err := clientConn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: sessResp.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("hello")},
	})
	if err != nil {
		t.Fatalf("real ACP prompt round-trip failed: %v", err)
	}
	if promptResp.StopReason != acpsdk.StopReasonMaxTokens {
		t.Fatalf("StopReason = %q, want %q (truncated generation)", promptResp.StopReason, acpsdk.StopReasonMaxTokens)
	}
	if got := client.received(); len(got) != 1 || got[0] != "partial output" {
		t.Fatalf("expected the real partial content to still reach the client, got %v", got)
	}
}

// TestAgent_CancelStopsRealInFlightGeneration proves ACP `session/cancel` is
// wired to a REAL in-flight turn: it starts a Prompt whose fake provider
// blocks until either unblocked or its context is cancelled, sends a real
// ACP cancel notification while that Prompt call is genuinely still in
// flight, and asserts the pending Prompt call returns
// PromptResponse{StopReason: StopReasonCancelled} — proving Cancel actually
// stops real work rather than being a no-op that happens to return nil.
func TestAgent_CancelStopsRealInFlightGeneration(t *testing.T) {
	block := make(chan struct{}) // never closed — only ctx cancellation unblocks GenerateStream
	provider := &fakeStreamingProvider{chunks: []string{"should never be sent"}, block: block}
	client := &recordingClient{}
	clientConn, _, cleanup := wirePipesWithClient(t, provider, client)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessResp, err := clientConn.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: "/tmp", McpServers: []acpsdk.McpServer{}})
	if err != nil {
		t.Fatalf("real session/new round-trip failed: %v", err)
	}

	type promptResult struct {
		resp acpsdk.PromptResponse
		err  error
	}
	resultCh := make(chan promptResult, 1)
	go func() {
		resp, err := clientConn.Prompt(ctx, acpsdk.PromptRequest{
			SessionId: sessResp.SessionId,
			Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("hello")},
		})
		resultCh <- promptResult{resp: resp, err: err}
	}()

	// Give the Prompt call a moment to genuinely start (real goroutine
	// scheduling, not a fabricated delay used to fake concurrency).
	time.Sleep(50 * time.Millisecond)

	if err := clientConn.Cancel(ctx, acpsdk.CancelNotification{SessionId: sessResp.SessionId}); err != nil {
		t.Fatalf("real ACP session/cancel notification failed: %v", err)
	}

	select {
	case res := <-resultCh:
		if res.err != nil {
			t.Fatalf("Prompt returned an error instead of a cancelled StopReason: %v", res.err)
		}
		if res.resp.StopReason != acpsdk.StopReasonCancelled {
			t.Fatalf("StopReason = %q, want %q", res.resp.StopReason, acpsdk.StopReasonCancelled)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Prompt did not return after a real session/cancel notification — Cancel is not genuinely wired")
	}

	if got := client.received(); len(got) != 0 {
		t.Fatalf("expected zero session/update chunks for a cancelled-before-any-content turn, got %v", got)
	}
}
