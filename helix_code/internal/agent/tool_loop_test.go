package agent

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// writeTool — a REAL WRITE tool (RequiresApproval() == LevelEdit) with a REAL,
// observable side effect: it flips an *int32 sentinel via atomic store. Used to
// prove the read-only-only guard genuinely prevents a write/shell-class tool
// from EXECUTING (the side effect must NOT occur) while still letting the loop
// continue. Not a loop stub — Execute really runs and really mutates.
// ---------------------------------------------------------------------------

type writeTool struct {
	sideEffect *int32 // set to 1 by Execute; the test asserts it stays 0 under the guard
	level      approval.ApprovalLevel
}

func (w *writeTool) Name() string                             { return "fs_write_test" }
func (w *writeTool) Description() string                      { return "writes a sentinel (real side effect)" }
func (w *writeTool) Category() tools.ToolCategory             { return tools.CategoryFileSystem }
func (w *writeTool) Validate(map[string]interface{}) error    { return nil }
func (w *writeTool) RequiresApproval() approval.ApprovalLevel { return w.level }

func (w *writeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"content": map[string]interface{}{"type": "string"},
		},
		Required:    []string{"content"},
		Description: "writes a sentinel",
	}
}

func (w *writeTool) Execute(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	if w.sideEffect != nil {
		atomic.StoreInt32(w.sideEffect, 1) // REAL side effect — must NOT happen under the guard
	}
	return "wrote sentinel", nil
}

// Anti-bluff RunToolLoop unit tests (CONST-050(A): fakes permitted in *_test.go
// invoked without the integration build tag). The fake here is the LLM provider
// only — the tool-execution loop under test is REAL, and the tool it drives is a
// REAL tool (echoTool actually executes, capturing and returning its arguments).

// ---------------------------------------------------------------------------
// echoTool — a REAL tool (not a loop stub). It genuinely executes: it reads its
// "text" parameter and returns it, recording the call so the test can assert the
// loop dispatched a real execution whose real output flowed back into the trace.
// ---------------------------------------------------------------------------

type echoTool struct {
	calls *int32
}

func (e *echoTool) Name() string                             { return "echo" }
func (e *echoTool) Description() string                      { return "echoes the text argument back" }
func (e *echoTool) Category() tools.ToolCategory             { return tools.CategoryFileSystem }
func (e *echoTool) Validate(map[string]interface{}) error    { return nil }
func (e *echoTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (e *echoTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"text": map[string]interface{}{"type": "string"},
		},
		Required:    []string{"text"},
		Description: "echoes the text argument back",
	}
}

func (e *echoTool) Execute(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if e.calls != nil {
		atomic.AddInt32(e.calls, 1)
	}
	text, _ := params["text"].(string)
	return "echoed: " + text, nil
}

// ---------------------------------------------------------------------------
// scriptedProvider — a fake llm.Provider that returns a pre-scripted sequence of
// LLMResponses, one per Generate call. Only Generate / GetModels are exercised
// by RunToolLoop; the rest satisfy the interface.
// ---------------------------------------------------------------------------

type scriptedProvider struct {
	responses   []llm.LLMResponse
	calls       int32
	lastReqHad  bool            // whether the last request carried Tools
	lastTools   []string        // names of the tools offered on the LAST request (offer-filter evidence)
	reqMessages [][]llm.Message // a snapshot of req.Messages captured per Generate call (turn N == index N)
}

func (p *scriptedProvider) Generate(_ context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	p.lastReqHad = len(req.Tools) > 0
	names := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		names = append(names, t.Function.Name)
	}
	p.lastTools = names
	// Capture a snapshot of the messages this turn saw (the model-facing
	// conversation), so a test can assert what the loop fed back from a prior
	// tool result. Copy the slice so later mutation of convo can't alias it.
	snap := make([]llm.Message, len(req.Messages))
	copy(snap, req.Messages)
	p.reqMessages = append(p.reqMessages, snap)
	i := int(atomic.AddInt32(&p.calls, 1)) - 1
	if i >= len(p.responses) {
		return nil, fmt.Errorf("scriptedProvider: no response scripted for Generate call #%d", i+1)
	}
	resp := p.responses[i]
	return &resp, nil
}

func (p *scriptedProvider) GetType() llm.ProviderType { return llm.ProviderTypeLocal }
func (p *scriptedProvider) GetName() string           { return "scripted" }
func (p *scriptedProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "scripted-1", Name: "scripted-model-1", SupportsTools: true}}
}
func (p *scriptedProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (p *scriptedProvider) GenerateStream(context.Context, *llm.LLMRequest, chan<- llm.LLMResponse) error {
	return nil
}
func (p *scriptedProvider) IsAvailable(context.Context) bool { return true }
func (p *scriptedProvider) GetHealth(context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "ok"}, nil
}
func (p *scriptedProvider) Close() error                    { return nil }
func (p *scriptedProvider) GetContextWindow() int           { return 8192 }
func (p *scriptedProvider) CountTokens(string) (int, error) { return 0, nil }

func newEchoRegistry(t *testing.T, calls *int32) *tools.ToolRegistry {
	t.Helper()
	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	r.Register(&echoTool{calls: calls})
	return r
}

// Test 1: tool-call turn then final answer.
func TestRunToolLoop_ToolThenFinalAnswer(t *testing.T) {
	var calls int32
	registry := newEchoRegistry(t, &calls)

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		// Turn 1: model asks to call the echo tool.
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
		// Turn 2: model gives the final text answer (no tool calls).
		{ID: uuid.New(), Content: "The tool said hello world."},
	}}

	res, err := RunToolLoop(context.Background(), provider, registry, []llm.Message{
		{Role: "user", Content: "Echo hello world"},
	}, ToolLoopOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, "The tool said hello world.", res.FinalContent)
	require.Equal(t, 2, res.Turns)
	require.Len(t, res.Trace, 1)

	entry := res.Trace[0]
	require.Equal(t, "echo", entry.ToolName)
	require.Equal(t, "hello world", entry.Arguments["text"])
	require.Empty(t, entry.Err)
	// REAL output of the REAL tool flowed back into the trace.
	require.Contains(t, entry.Output, "echoed: hello world")
	// The real tool genuinely executed exactly once.
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
	// The provider was offered the registry's tool schemas.
	require.True(t, provider.lastReqHad)
}

// Test 1b: FinalMetadata is carried through from the FINAL (tool-call-free)
// Generate response. The final answer turn sets ProviderMetadata{"ensemble":
// true, ...}; RunToolLoop must surface exactly that map as res.FinalMetadata so
// a caller can render ensemble member info from the response that produced the
// answer. The EARLIER tool-call turn carries DIFFERENT metadata to prove the
// final turn's metadata (not the tool turn's) is the one returned.
func TestRunToolLoop_FinalMetadataFromFinalResponse(t *testing.T) {
	var calls int32
	registry := newEchoRegistry(t, &calls)

	finalMeta := map[string]interface{}{
		"ensemble":                   true,
		"ensemble_selected_provider": "DeepSeek",
		"ensemble_participants":      []string{"DeepSeek", "Groq"},
	}

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		// Turn 1: tool-call turn, carries its OWN (different) metadata.
		{
			ID:               uuid.New(),
			ProviderMetadata: map[string]interface{}{"ensemble": true, "turn": "tool"},
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "echo",
					Arguments: map[string]interface{}{"text": "x"},
				},
			}},
		},
		// Turn 2: final answer turn carries the ensemble metadata to surface.
		{ID: uuid.New(), Content: "done", ProviderMetadata: finalMeta},
	}}

	res, err := RunToolLoop(context.Background(), provider, registry, []llm.Message{
		{Role: "user", Content: "echo x"},
	}, ToolLoopOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, "done", res.FinalContent)
	require.Equal(t, 2, res.Turns)
	// FinalMetadata MUST be the FINAL response's metadata (the ensemble map),
	// not the tool-call turn's metadata.
	require.NotNil(t, res.FinalMetadata)
	require.Equal(t, true, res.FinalMetadata["ensemble"])
	require.Equal(t, "DeepSeek", res.FinalMetadata["ensemble_selected_provider"])
	require.Equal(t, []string{"DeepSeek", "Groq"}, res.FinalMetadata["ensemble_participants"])
	// Prove it is NOT the tool turn's metadata.
	require.NotContains(t, res.FinalMetadata, "turn")
}

// Test 2: final answer immediately, no tool calls.
func TestRunToolLoop_FinalAnswerImmediately(t *testing.T) {
	var calls int32
	registry := newEchoRegistry(t, &calls)

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		{ID: uuid.New(), Content: "42"},
	}}

	res, err := RunToolLoop(context.Background(), provider, registry, []llm.Message{
		{Role: "user", Content: "What is 6 times 7?"},
	}, ToolLoopOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, "42", res.FinalContent)
	require.Equal(t, 1, res.Turns)
	require.Empty(t, res.Trace)
	require.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

// Test 3: nil registry degrades to a single plain Generate, no panic, no tools.
func TestRunToolLoop_NilRegistry(t *testing.T) {
	provider := &scriptedProvider{responses: []llm.LLMResponse{
		{ID: uuid.New(), Content: "plain answer"},
	}}

	res, err := RunToolLoop(context.Background(), provider, nil, []llm.Message{
		{Role: "user", Content: "Just answer"},
	}, ToolLoopOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, "plain answer", res.FinalContent)
	require.Equal(t, 1, res.Turns)
	require.Empty(t, res.Trace)
	// No tools were offered to the provider.
	require.False(t, provider.lastReqHad)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// newMixedRegistry builds a REAL registry holding BOTH a real read-only tool
// (echo, LevelReadOnly) and a real WRITE tool (fs_write_test, LevelEdit) whose
// Execute has a genuine observable side effect (flips the sentinel).
func newMixedRegistry(t *testing.T, echoCalls *int32, writeSentinel *int32, writeLevel approval.ApprovalLevel) *tools.ToolRegistry {
	t.Helper()
	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	r.Register(&echoTool{calls: echoCalls})
	r.Register(&writeTool{sideEffect: writeSentinel, level: writeLevel})
	return r
}

// Test 4 — SAFETY-CRITICAL (§11.4.133 target safety + defense-in-depth).
//
// With ReadOnlyOnly:true the loop MUST refuse a write/shell-class tool at BOTH
// layers: the write tool is NOT offered to the model (offer-filter), and even
// when the model requests it anyway, the EXECUTE step refuses it — the write
// tool's REAL side effect (sentinel flip) must NOT occur, its trace entry must
// carry a "not permitted" Err, and the loop must still return the final answer.
func TestRunToolLoop_ReadOnlyOnly_RefusesWriteTool(t *testing.T) {
	var echoCalls, writeSentinel int32
	registry := newMixedRegistry(t, &echoCalls, &writeSentinel, approval.LevelEdit)

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		// Turn 1: the model (maliciously or by mistake) asks to call the WRITE tool.
		{
			ID: uuid.New(),
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "fs_write_test",
					Arguments: map[string]interface{}{"content": "destroy repo"},
				},
			}},
		},
		// Turn 2: final text answer (no tool calls).
		{ID: uuid.New(), Content: "I cannot write in read-only mode."},
	}}

	res, err := RunToolLoop(context.Background(), provider, registry, []llm.Message{
		{Role: "user", Content: "Write a file"},
	}, ToolLoopOptions{ReadOnlyOnly: true})
	require.NoError(t, err)
	require.NotNil(t, res)

	// Loop still returns the final answer.
	require.Equal(t, "I cannot write in read-only mode.", res.FinalContent)
	require.Equal(t, 2, res.Turns)

	// EXECUTE-layer guard: the write tool's REAL side effect did NOT occur.
	require.Equal(t, int32(0), atomic.LoadInt32(&writeSentinel),
		"write tool must NOT execute under ReadOnlyOnly — its side effect must not happen")

	// The trace records the refusal with a "not permitted" Err.
	require.Len(t, res.Trace, 1)
	require.Equal(t, "fs_write_test", res.Trace[0].ToolName)
	require.Contains(t, res.Trace[0].Err, "not permitted in read-only mode")

	// OFFER-layer guard: the write tool was NOT offered to the provider; the
	// read-only echo tool WAS.
	require.NotContains(t, provider.lastTools, "fs_write_test",
		"write tool must NOT be offered to the model under ReadOnlyOnly")
	require.Contains(t, provider.lastTools, "echo",
		"read-only tool must still be offered under ReadOnlyOnly")
}

// Test 4b — load-bearing-flag guard (§1.1-style mutation guard). The SAME write
// tool, with ReadOnlyOnly:false (the default), MUST be offered AND MUST execute
// (its real side effect occurs). This proves the ReadOnlyOnly flag is genuinely
// load-bearing — Test 4's refusal is caused by the flag, not by a tautology.
func TestRunToolLoop_DefaultMode_OffersAndExecutesWriteTool(t *testing.T) {
	var echoCalls, writeSentinel int32
	registry := newMixedRegistry(t, &echoCalls, &writeSentinel, approval.LevelEdit)

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		{
			ID: uuid.New(),
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "fs_write_test",
					Arguments: map[string]interface{}{"content": "ok"},
				},
			}},
		},
		{ID: uuid.New(), Content: "wrote it"},
	}}

	res, err := RunToolLoop(context.Background(), provider, registry, []llm.Message{
		{Role: "user", Content: "Write a file"},
	}, ToolLoopOptions{ReadOnlyOnly: false})
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, "wrote it", res.FinalContent)
	// Default mode: the write tool WAS offered ...
	require.Contains(t, provider.lastTools, "fs_write_test",
		"write tool must be offered in default (non-read-only) mode")
	// ... and DID execute (real side effect occurred).
	require.Equal(t, int32(1), atomic.LoadInt32(&writeSentinel),
		"write tool must execute in default mode — proves the ReadOnlyOnly flag is load-bearing")
	require.Len(t, res.Trace, 1)
	require.Empty(t, res.Trace[0].Err)
}

// ---------------------------------------------------------------------------
// bigOutputTool — a REAL read-only tool that returns a LARGE output (size runes
// of 'A'). Models the real-world case of fs_read of a big governance file or a
// long grep/git-log whose full output would otherwise accumulate across turns
// and overflow the model's context window. Not a loop stub — Execute really runs
// and really returns the big string.
// ---------------------------------------------------------------------------

type bigOutputTool struct {
	size int
}

func (b *bigOutputTool) Name() string                             { return "big_read" }
func (b *bigOutputTool) Description() string                      { return "returns a large output" }
func (b *bigOutputTool) Category() tools.ToolCategory             { return tools.CategoryFileSystem }
func (b *bigOutputTool) Validate(map[string]interface{}) error    { return nil }
func (b *bigOutputTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (b *bigOutputTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Properties:  map[string]interface{}{},
		Description: "returns a large output",
	}
}

func (b *bigOutputTool) Execute(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	r := make([]rune, b.size)
	for i := range r {
		r[i] = 'A'
	}
	return string(r), nil
}

// Test 5 — context-bounding (the multi-turn context-overflow fix).
//
// A multi-turn loop where turn 1 calls a real tool returning a 20,000-char
// output, then turn 2 produces the final answer. The model-facing role:"tool"
// message fed back into turn 2 MUST be truncated to MaxToolResultChars (default
// 4000) WITH the truncation marker — NOT the full 20,000 chars — so accumulated
// multi-turn context stays bounded regardless of tool/model/file size. The
// ToolTraceEntry keeps its own independent short excerpt (~300 chars).
//
// §1.1 mutation note: removing the truncateForModel call in tool_loop.go (feeding
// back the full output) makes the turn-2 tool message carry all 20,000 chars,
// failing the length + non-contains assertions below.
func TestRunToolLoop_BoundsToolResultContext(t *testing.T) {
	const bigSize = 20000

	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	r.Register(&bigOutputTool{size: bigSize})

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		// Turn 1: model asks to call the big-output tool.
		{
			ID: uuid.New(),
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "big_read",
					Arguments: map[string]interface{}{},
				},
			}},
		},
		// Turn 2: final text answer (no tool calls).
		{ID: uuid.New(), Content: "read it"},
	}}

	res, err := RunToolLoop(context.Background(), provider, r, []llm.Message{
		{Role: "user", Content: "read the big file"},
	}, ToolLoopOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "read it", res.FinalContent)
	require.Equal(t, 2, res.Turns)

	// Capture the messages the provider saw on TURN 2 (index 1).
	require.Len(t, provider.reqMessages, 2)
	turn2 := provider.reqMessages[1]

	// Find the role:"tool" message carrying the big_read result.
	var toolMsg *llm.Message
	for i := range turn2 {
		if turn2[i].Role == "tool" && turn2[i].Name == "big_read" {
			toolMsg = &turn2[i]
			break
		}
	}
	require.NotNil(t, toolMsg, "turn-2 conversation must contain the role:tool big_read result")

	// The fed-back tool result MUST be bounded to MaxToolResultChars (default
	// 4000) plus the truncation marker — NOT the full 20,000 chars.
	feedRunes := []rune(toolMsg.Content)
	require.Less(t, len(feedRunes), bigSize,
		"fed-back tool result must be bounded, not the full 20,000 chars")
	require.LessOrEqual(t, len(feedRunes), 4000+200,
		"fed-back tool result must be ~MaxToolResultChars + a short marker")
	require.Contains(t, toolMsg.Content, "[truncated",
		"truncated tool result must carry the truncation marker")
	// The marker MUST report the original size so the model knows it was cut.
	require.Contains(t, toolMsg.Content, fmt.Sprintf("of %d", bigSize))

	// The ToolTraceEntry keeps its OWN independent short excerpt (~300-char cap),
	// distinct from the model-facing feedback bound.
	require.Len(t, res.Trace, 1)
	traceRunes := []rune(res.Trace[0].Output)
	require.LessOrEqual(t, len(traceRunes), traceOutputMaxLen+1,
		"trace excerpt keeps its own ~300-char cap")
}

// Test 5b — explicit MaxToolResultChars override is honored (load-bearing).
func TestRunToolLoop_MaxToolResultCharsOverride(t *testing.T) {
	const bigSize = 20000

	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	r.Register(&bigOutputTool{size: bigSize})

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		{
			ID: uuid.New(),
			ToolCalls: []llm.ToolCall{{
				ID:       "call-1",
				Type:     "function",
				Function: llm.ToolCallFunc{Name: "big_read", Arguments: map[string]interface{}{}},
			}},
		},
		{ID: uuid.New(), Content: "ok"},
	}}

	res, err := RunToolLoop(context.Background(), provider, r, []llm.Message{
		{Role: "user", Content: "read"},
	}, ToolLoopOptions{MaxToolResultChars: 500})
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Len(t, provider.reqMessages, 2)
	turn2 := provider.reqMessages[1]
	var toolMsg *llm.Message
	for i := range turn2 {
		if turn2[i].Role == "tool" && turn2[i].Name == "big_read" {
			toolMsg = &turn2[i]
			break
		}
	}
	require.NotNil(t, toolMsg)
	require.LessOrEqual(t, len([]rune(toolMsg.Content)), 500+200,
		"override bound (500) must be honored")
	require.Contains(t, toolMsg.Content, "[truncated")
}

// ---------------------------------------------------------------------------
// Empty-final-content guard (bug: HelixAgent agentic loop produced an empty
// final answer for the first prompt; the empty assistant turn then violated
// HelixAgent's strict "assistant message must have content or tool_calls"
// validation on the NEXT request, 400ing the whole conversation).
//
// RunToolLoop MUST NEVER return an empty FinalContent. When the model's final
// turn (no tool calls) carries empty content, the loop must synthesize a
// non-empty answer (e.g. from the trace) so the caller never feeds an empty
// assistant message back to a strict provider.
// ---------------------------------------------------------------------------

// Test: final turn has empty content AND no tool calls — after running a tool
// in an earlier turn. FinalContent MUST be non-empty.
func TestRunToolLoop_EmptyFinalAfterToolNeverEmpty(t *testing.T) {
	var calls int32
	reg := newEchoRegistry(t, &calls)

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		// Turn 1: ask for a tool call.
		{ToolCalls: []llm.ToolCall{{ID: "c1", Function: llm.ToolCallFunc{
			Name: "echo", Arguments: map[string]interface{}{"text": "hi"}}}}},
		// Turn 2: FINAL turn — no tool calls, EMPTY content (the bug trigger).
		{Content: "", FinishReason: "stop"},
	}}

	res, err := RunToolLoop(context.Background(), provider, reg,
		[]llm.Message{{Role: "user", Content: "Do you see my codebase?"}},
		ToolLoopOptions{Model: "scripted-model-1", MaxTurns: 6})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.FinalContent,
		"FinalContent MUST NOT be empty: an empty assistant turn breaks the next provider request")
	require.NotEqual(t, "", strings.TrimSpace(res.FinalContent),
		"FinalContent MUST NOT be whitespace-only")
}

// Test: final turn empty content + no tool calls AND no prior tools (the loop's
// very first response is an empty stop). Still must never be empty.
func TestRunToolLoop_EmptyFinalNoToolsNeverEmpty(t *testing.T) {
	var calls int32
	reg := newEchoRegistry(t, &calls)

	provider := &scriptedProvider{responses: []llm.LLMResponse{
		{Content: "", FinishReason: "stop"}, // first + final turn, empty.
	}}

	res, err := RunToolLoop(context.Background(), provider, reg,
		[]llm.Message{{Role: "user", Content: "Anything?"}},
		ToolLoopOptions{Model: "scripted-model-1", MaxTurns: 6})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, strings.TrimSpace(res.FinalContent),
		"FinalContent MUST NOT be empty/whitespace even when the only turn was an empty stop")
}
