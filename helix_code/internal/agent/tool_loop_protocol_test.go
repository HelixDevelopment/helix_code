package agent

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// tool_loop_protocol_test.go is the belt-and-suspenders LOOP-LAYER proof that
// RunToolLoop feeds the OpenAI/Groq tool-conversation protocol back correctly:
// after a tool-call turn, the messages handed to the NEXT Generate must contain
// (a) an assistant message carrying the model's ToolCalls, and (b) one
// role:"tool" message per executed call carrying a ToolCallID that matches the
// assistant tool_call's id. A dangling tool_call (no matching tool result) or a
// role:"tool" message without a tool_call_id is exactly what the live Groq
// request rejected ("for 'role:tool' the 'tool_call_id' is missing").
//
// recordingProvider is a fake llm.Provider (CONST-050(A): fakes permitted in
// *_test.go without the integration tag) that records the messages each
// Generate received, so the test can assert the SECOND turn's conversation
// shape directly at the loop boundary — independent of any provider wire code.

type recordingProvider struct {
	responses []llm.LLMResponse
	calls     int32
	// turnMessages[i] is a copy of the messages handed to Generate call i+1.
	turnMessages [][]llm.Message
}

func (p *recordingProvider) Generate(_ context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	snapshot := make([]llm.Message, len(req.Messages))
	copy(snapshot, req.Messages)
	p.turnMessages = append(p.turnMessages, snapshot)
	i := int(atomic.AddInt32(&p.calls, 1)) - 1
	resp := p.responses[i]
	return &resp, nil
}

func (p *recordingProvider) GetType() llm.ProviderType { return llm.ProviderTypeLocal }
func (p *recordingProvider) GetName() string           { return "recording" }
func (p *recordingProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "rec-1", Name: "recording-model-1", SupportsTools: true}}
}
func (p *recordingProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (p *recordingProvider) GenerateStream(context.Context, *llm.LLMRequest, chan<- llm.LLMResponse) error {
	return nil
}
func (p *recordingProvider) IsAvailable(context.Context) bool { return true }
func (p *recordingProvider) GetHealth(context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "ok"}, nil
}
func (p *recordingProvider) Close() error                  { return nil }
func (p *recordingProvider) GetContextWindow() int         { return 8192 }
func (p *recordingProvider) CountTokens(string) (int, error) { return 0, nil }

// findToolConversation walks msgs and returns (assistant-with-matching-toolcall
// present, tool-result-with-matching-id present) for the given call id + tool
// output substring.
func findToolConversation(msgs []llm.Message, callID, toolName, outputContains string) (asstOK, toolOK bool) {
	for _, m := range msgs {
		if m.Role == "assistant" {
			for _, tc := range m.ToolCalls {
				if tc.ID == callID && tc.Function.Name == toolName {
					asstOK = true
				}
			}
		}
		if m.Role == "tool" && m.ToolCallID == callID && strings.Contains(m.Content, outputContains) {
			toolOK = true
		}
	}
	return
}

// TestRunToolLoop_FeedsToolCallIDIntoSecondTurn proves the loop assembles a
// protocol-valid second-turn conversation: assistant(tool_calls[call-1]) then
// tool(tool_call_id=call-1, content=<real echo output>).
func TestRunToolLoop_FeedsToolCallIDIntoSecondTurn(t *testing.T) {
	var calls int32
	registry := newEchoRegistry(t, &calls)

	provider := &recordingProvider{responses: []llm.LLMResponse{
		// Turn 1: the model asks to call echo (id=call-1).
		{
			ID: uuid.New(),
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "echo",
					Arguments: map[string]interface{}{"text": "hello world"},
				},
			}},
		},
		// Turn 2: final answer.
		{ID: uuid.New(), Content: "done"},
	}}

	res, err := RunToolLoop(context.Background(), provider, registry, []llm.Message{
		{Role: "user", Content: "Echo hello world"},
	}, ToolLoopOptions{})
	require.NoError(t, err)
	require.Equal(t, "done", res.FinalContent)
	require.Equal(t, 2, res.Turns)

	// The SECOND Generate must have seen the assistant(tool_calls) + the
	// matching tool(tool_call_id) messages.
	require.Len(t, provider.turnMessages, 2)
	second := provider.turnMessages[1]
	asstOK, toolOK := findToolConversation(second, "call-1", "echo", "echoed: hello world")
	require.True(t, asstOK, "second turn must carry an assistant message with tool_calls referencing call-1; got %#v", second)
	require.True(t, toolOK, "second turn must carry a role:tool message with tool_call_id=call-1 and the real echo output; got %#v", second)
}

// TestRunToolLoop_RefusedCallStillCarriesToolCallID proves a read-only-refused
// call is NOT left as a dangling assistant tool_call: it still gets a role:tool
// message with the matching tool_call_id + "not permitted" content, so the
// conversation the next Generate sees is well-formed.
func TestRunToolLoop_RefusedCallStillCarriesToolCallID(t *testing.T) {
	var writeSentinel int32
	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	r.Register(&writeTool{sideEffect: &writeSentinel, level: approval.LevelEdit})

	provider := &recordingProvider{responses: []llm.LLMResponse{
		// Turn 1: the model asks to call the WRITE tool (id=call-w).
		{
			ID: uuid.New(),
			ToolCalls: []llm.ToolCall{{
				ID:   "call-w",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "fs_write_test",
					Arguments: map[string]interface{}{"content": "x"},
				},
			}},
		},
		// Turn 2: final answer.
		{ID: uuid.New(), Content: "ok"},
	}}

	res, err := RunToolLoop(context.Background(), provider, r, []llm.Message{
		{Role: "user", Content: "Write a file"},
	}, ToolLoopOptions{ReadOnlyOnly: true})
	require.NoError(t, err)
	require.Equal(t, "ok", res.FinalContent)
	// The write tool must NOT have executed under the read-only guard.
	require.Equal(t, int32(0), atomic.LoadInt32(&writeSentinel))

	require.Len(t, provider.turnMessages, 2)
	second := provider.turnMessages[1]
	asstOK, toolOK := findToolConversation(second, "call-w", "fs_write_test", "not permitted")
	require.True(t, asstOK, "refused call must still appear as an assistant tool_call (no dangling-call); got %#v", second)
	require.True(t, toolOK, "refused call must still get a role:tool message with tool_call_id=call-w + not-permitted content; got %#v", second)
}
