// Anti-bluff unit tests for the HelixAgent llm.Provider adapter.
//
// CONST-035 / CONST-050(A): these tests exercise the REAL provider HTTP code
// path (real JSON encode/decode, real http.Client, real LLMResponse
// construction) against an httptest server that mimics the running HelixAgent
// REST surface at the transport boundary. No internal helpers are mocked — the
// only faked component is the remote HelixAgent server, which is the canonical
// pattern for provider-layer unit tests. A LIVE build-tagged test
// (helixagent_live_test.go) proves the adapter against the actual running
// agent on :7061 when reachable.
package helixagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dev.helix.code/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const knownContent = "HelixAgent engine replied: 2+2 is 4."

// newFakeHelixAgent returns an httptest server mimicking the three HelixAgent
// REST endpoints the adapter consumes.
func newFakeHelixAgent(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var req chatRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotEmpty(t, req.Model, "model must be populated (defaulted when empty)")
		require.NotEmpty(t, req.Messages, "messages must be forwarded")

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id":    "chatcmpl-test",
			"model": req.Model,
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]string{"role": "assistant", "content": knownContent},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens": 11, "completion_tokens": 9, "total_tokens": 20,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]string{
				{"id": "helixagent-llm"},
				{"id": "helixagent-ensemble"},
			},
		})
	})

	mux.HandleFunc("/v1/providers", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		// Object-wrapped roster of 25 providers.
		providers := make([]map[string]string, 0, 25)
		for i := 0; i < 25; i++ {
			providers = append(providers, map[string]string{"name": "p"})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"providers": providers})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNew_DefaultsBaseURLWhenEmpty(t *testing.T) {
	p := New("")
	assert.Equal(t, DefaultBaseURL, p.baseURL)

	p2 := New("http://example.test:7061/")
	assert.Equal(t, "http://example.test:7061", p2.baseURL, "trailing slash trimmed")
}

func TestInterfaceMethods_StaticShape(t *testing.T) {
	p := New("http://unused")
	assert.Equal(t, llm.ProviderType("helixagent"), p.GetType())
	assert.Equal(t, "HelixAgent", p.GetName())
	assert.NotEmpty(t, p.GetCapabilities())
	assert.Equal(t, 200_000, p.GetContextWindow())

	n, err := p.CountTokens("12345678") // 8 chars → 2 tokens
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	z, err := p.CountTokens("")
	require.NoError(t, err)
	assert.Equal(t, 0, z)

	require.NoError(t, p.Close())
}

func TestGenerate_ReturnsEngineContent(t *testing.T) {
	srv := newFakeHelixAgent(t)
	p := New(srv.URL)

	req := &llm.LLMRequest{
		ID:       uuid.New(),
		Messages: []llm.Message{{Role: "user", Content: "What is 2+2?"}},
	}
	resp, err := p.Generate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, knownContent, resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, req.ID, resp.RequestID)
	assert.Equal(t, 11, resp.Usage.PromptTokens)
	assert.Equal(t, 9, resp.Usage.CompletionTokens)
	assert.Equal(t, 20, resp.Usage.TotalTokens)
	// Empty model must be defaulted to helixagent-llm and echoed in metadata.
	assert.Equal(t, DefaultModel, resp.ProviderMetadata["helixagent_model"])
}

func TestGenerate_HonorsExplicitEnsembleModel(t *testing.T) {
	srv := newFakeHelixAgent(t)
	p := New(srv.URL)

	req := &llm.LLMRequest{
		ID:       uuid.New(),
		Model:    "helixagent-ensemble",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}
	resp, err := p.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "helixagent-ensemble", resp.ProviderMetadata["helixagent_model"])
}

func TestGenerate_HTTPErrorSurfaced(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New(srv.URL)
	_, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID:       uuid.New(),
		Messages: []llm.Message{{Role: "user", Content: "x"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestGetModels_FromLiveAPI(t *testing.T) {
	srv := newFakeHelixAgent(t)
	p := New(srv.URL)

	models := p.GetModels()
	require.Len(t, models, 2)

	ids := map[string]bool{}
	for _, m := range models {
		ids[m.ID] = true
		assert.Equal(t, llm.ProviderType("helixagent"), m.Provider)
		assert.Equal(t, m.ID, m.Name)
	}
	assert.True(t, ids["helixagent-llm"], "logical model helixagent-llm present")
	assert.True(t, ids["helixagent-ensemble"], "logical model helixagent-ensemble present")
}

func TestGetModels_UnreachableReturnsNil(t *testing.T) {
	p := New("http://127.0.0.1:1") // closed port
	assert.Nil(t, p.GetModels())
}

func TestIsAvailableAndHealth(t *testing.T) {
	srv := newFakeHelixAgent(t)
	p := New(srv.URL)

	assert.True(t, p.IsAvailable(context.Background()))

	h, err := p.GetHealth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", h.Status)
	assert.Equal(t, 2, h.ModelCount)
	assert.Contains(t, h.Message, "25 providers")
}

func TestIsAvailable_FalseWhenUnreachable(t *testing.T) {
	p := New("http://127.0.0.1:1")
	assert.False(t, p.IsAvailable(context.Background()))
	_, err := p.GetHealth(context.Background())
	require.Error(t, err)
}

// TestGenerate_ToolCalling_ForwardsToolsAndParsesToolCalls exercises the full
// OpenAI tool-calling protocol through the HelixAgent adapter, end-to-end across
// two turns, exactly the way internal/agent.RunToolLoop drives it:
//
//	Turn 1 — caller sends a user prompt + a `tools` array. The fake engine
//	         replies with an assistant turn carrying tool_calls (empty content +
//	         a git_status call). The adapter MUST surface ToolCalls len 1.
//	Turn 2 — caller feeds back the assistant tool_calls turn AND a role:"tool"
//	         result message (tool_call_id:"call_1"). The fake engine captures the
//	         RAW request body and asserts:
//	           (a) the `tools` array was forwarded,
//	           (b) a role:"tool" message with tool_call_id:"call_1" is present,
//	           (c) the assistant tool_calls[].function.arguments is a JSON STRING
//	               on the wire (`"arguments":"`), NOT an object (`"arguments":{`).
//
// This is the anti-bluff contract: a green PASS means the TUI agentic tool loop
// genuinely works through HelixAgent (tools forwarded + tool_calls parsed +
// string-encoded arguments on the wire), not merely that the code compiles.
func TestGenerate_ToolCalling_ForwardsToolsAndParsesToolCalls(t *testing.T) {
	var turn int
	var turn2Body []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		turn++

		w.Header().Set("Content-Type", "application/json")
		if turn == 1 {
			// Turn 1: the tools array must already be forwarded on the wire.
			assert.Contains(t, string(raw), `"tools"`, "turn-1 request must forward the tools array")
			assert.Contains(t, string(raw), `"git_status"`, "turn-1 request must forward the git_status tool definition")
			// Reply with an assistant tool_calls turn (empty content is valid).
			resp := map[string]interface{}{
				"id":    "chatcmpl-tc-1",
				"model": "helixagent-llm",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"id":   "call_1",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "git_status",
										"arguments": "{}",
									},
								},
							},
						},
						"finish_reason": "tool_calls",
					},
				},
				"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Turn 2: capture the fed-back conversation for assertion below.
		turn2Body = raw
		resp := map[string]interface{}{
			"id":    "chatcmpl-tc-2",
			"model": "helixagent-llm",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]string{"role": "assistant", "content": "On branch main, nothing to commit."},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{"prompt_tokens": 12, "completion_tokens": 7, "total_tokens": 19},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New(srv.URL)

	tools := []llm.Tool{{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "git_status",
			Description: "Show the working tree status",
			Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
	}}

	// ---- Turn 1: prompt + tools → expect tool_calls back ----
	reqID := uuid.New()
	resp1, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID:         reqID,
		Messages:   []llm.Message{{Role: "user", Content: "What is the git status?"}},
		Tools:      tools,
		ToolChoice: "auto",
	})
	require.NoError(t, err)
	require.NotNil(t, resp1)
	require.Len(t, resp1.ToolCalls, 1, "Generate must surface the engine's tool_calls")
	assert.Equal(t, "call_1", resp1.ToolCalls[0].ID)
	assert.Equal(t, "git_status", resp1.ToolCalls[0].Function.Name)
	assert.Equal(t, "function", resp1.ToolCalls[0].Type)
	assert.NotNil(t, resp1.ToolCalls[0].Function.Arguments, "arguments decoded into a map (even if empty)")
	assert.Equal(t, "tool_calls", resp1.FinishReason)

	// ---- Turn 2: feed back the assistant tool_calls turn + tool result ----
	resp2, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID: reqID,
		Messages: []llm.Message{
			{Role: "user", Content: "What is the git status?"},
			{Role: "assistant", Content: "", ToolCalls: resp1.ToolCalls},
			{Role: "tool", Content: "On branch main\nnothing to commit, working tree clean", ToolCallID: "call_1"},
		},
		Tools: tools,
	})
	require.NoError(t, err)
	require.NotNil(t, resp2)
	assert.Equal(t, "On branch main, nothing to commit.", resp2.Content)

	// ---- Anti-bluff wire assertions on the turn-2 request body ----
	require.NotEmpty(t, turn2Body, "turn-2 request body must have been captured")
	body := string(turn2Body)

	assert.Contains(t, body, `"tools"`, "turn-2 request must still forward the tools array")
	assert.Contains(t, body, `"role":"tool"`, "turn-2 request must include the role:tool result message")
	assert.Contains(t, body, `"tool_call_id":"call_1"`, "tool-result message must carry tool_call_id:call_1")

	// CRITICAL: arguments MUST be a JSON STRING on the wire, never an object.
	assert.Contains(t, body, `"arguments":"`, "assistant tool_calls[].function.arguments MUST be a JSON STRING")
	assert.NotContains(t, body, `"arguments":{`, "arguments MUST NOT be serialised as a JSON object")

	// Decode the wire body and assert the structured tool-protocol shape too.
	var decoded chatRequest
	require.NoError(t, json.Unmarshal(turn2Body, &decoded))
	require.Len(t, decoded.Tools, 1, "tools array decoded len 1")
	assert.Equal(t, "git_status", decoded.Tools[0].Function.Name)

	var sawToolResult, sawAssistantToolCall bool
	for _, m := range decoded.Messages {
		if m.Role == "tool" && m.ToolCallID == "call_1" {
			sawToolResult = true
		}
		if m.Role == "assistant" && len(m.ToolCalls) == 1 {
			sawAssistantToolCall = true
			assert.Equal(t, "call_1", m.ToolCalls[0].ID)
			assert.Equal(t, "git_status", m.ToolCalls[0].Function.Name)
			assert.Equal(t, "{}", m.ToolCalls[0].Function.Arguments, "wire arguments is the string \"{}\"")
		}
	}
	assert.True(t, sawToolResult, "decoded request must contain the role:tool result with tool_call_id call_1")
	assert.True(t, sawAssistantToolCall, "decoded request must contain the assistant tool_calls turn")
}

// TestGenerate_NoTools_OmitsToolsOnWire is the conservative negative: a plain
// chat request (no Tools) must NOT emit a `tools` key on the wire, so the
// pre-tool-loop wire stays byte-identical.
func TestGenerate_NoTools_OmitsToolsOnWire(t *testing.T) {
	var body []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "x",
			"model": "helixagent-llm",
			"choices": []map[string]interface{}{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "hi"}, "finish_reason": "stop"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New(srv.URL)
	resp, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID:       uuid.New(),
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Nil(t, resp.ToolCalls, "plain chat response has no tool_calls")
	assert.NotContains(t, string(body), `"tools"`, "plain chat request must omit the tools key")
	assert.NotContains(t, string(body), `"tool_choice"`, "plain chat request must omit tool_choice")
}

// TestGenerate_SanitizesEmptyAssistant proves the adapter NEVER sends an
// assistant message with empty content AND no tool_calls to the wire. HelixAgent
// rejects such a message ("messages[N]: assistant message must have content or
// tool_calls", HTTP 400), which broke the second prompt of every multi-prompt
// TUI conversation. RED before the wire-layer sanitiser: the body contains
// `"role":"assistant","content":""`. GREEN after: it carries a single-space
// placeholder so the strict server accepts it.
func TestGenerate_SanitizesEmptyAssistant(t *testing.T) {
	var body []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		// Mimic HelixAgent's strict validation: reject an assistant message with
		// no content and no tool_calls BEFORE producing a reply.
		var parsed struct {
			Messages []struct {
				Role      string            `json:"role"`
				Content   string            `json:"content"`
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &parsed)
		for i, m := range parsed.Messages {
			// Mirror the REAL HelixAgent rule (verified live): an assistant
			// message is rejected only when content is the EMPTY string and there
			// are no tool_calls. A single-space content is accepted (it satisfies
			// "has content"), so the fake must NOT TrimSpace here.
			if m.Role == "assistant" && m.Content == "" && len(m.ToolCalls) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"code":    400,
						"message": fmt.Sprintf("messages[%d]: assistant message must have content or tool_calls", i),
						"type":    "invalid_request",
					},
				})
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "x",
			"model": "helixagent-llm",
			"choices": []map[string]interface{}{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New(srv.URL)
	// The exact multi-prompt history shape: [user, assistant(""), user]. The
	// empty assistant turn (no content, no tool_calls) is what 400s a strict
	// server when it is replayed in the next request's history.
	resp, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(),
		Messages: []llm.Message{
			{Role: "user", Content: "Do you see my codebase?"},
			{Role: "assistant", Content: ""},
			{Role: "user", Content: "Do you need an AGENTS.md?"},
		},
	})
	require.NoError(t, err, "the strict server must not 400 — the empty assistant was sanitized")
	require.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Content)
	// The wire body must NOT carry an empty-content assistant; it gets a
	// single-space placeholder instead.
	assert.NotContains(t, string(body), `"role":"assistant","content":""`,
		"empty assistant content must be sanitized on the wire")
	assert.Contains(t, string(body), `"role":"assistant","content":" "`,
		"empty assistant content must become a single-space placeholder")
}

// TestGenerate_KeepsEmptyContentWhenToolCallsPresent proves the sanitiser does
// NOT touch an assistant turn that legitimately has empty content paired with
// tool_calls (the canonical tool-request turn) — that turn already satisfies the
// "content or tool_calls" rule, so its empty content is preserved verbatim.
func TestGenerate_KeepsEmptyContentWhenToolCallsPresent(t *testing.T) {
	var body []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "x",
			"model": "helixagent-llm",
			"choices": []map[string]interface{}{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New(srv.URL)
	_, err := p.Generate(context.Background(), &llm.LLMRequest{
		ID: uuid.New(),
		Messages: []llm.Message{
			{Role: "user", Content: "list files"},
			{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{{
				ID: "call_1", Type: "function",
				Function: llm.ToolCallFunc{Name: "glob", Arguments: map[string]interface{}{"pattern": "*"}},
			}}},
			{Role: "tool", ToolCallID: "call_1", Name: "glob", Content: "a.go"},
		},
	})
	require.NoError(t, err)
	// The tool-request assistant turn keeps empty content (it has tool_calls).
	assert.Contains(t, string(body), `"role":"assistant","content":""`,
		"assistant with tool_calls keeps its empty content (not placeholder-substituted)")
	// The tool result message keeps its empty/non-empty content untouched too.
	assert.Contains(t, string(body), `"tool_call_id":"call_1"`)
}

func TestGenerateStream_EmitsDeltasAndDone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`{"choices":[{"delta":{"content":"He"},"finish_reason":""}]}`,
			`{"choices":[{"delta":{"content":"llo"},"finish_reason":""}]}`,
			`{"choices":[{"delta":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		}
		for _, c := range chunks {
			_, _ = w.Write([]byte("data: " + c + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New(srv.URL)
	ch := make(chan llm.LLMResponse, 16)
	err := p.GenerateStream(context.Background(), &llm.LLMRequest{
		ID:       uuid.New(),
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, ch)
	require.NoError(t, err)

	var content strings.Builder
	var finish string
	var usageTotal int
	for resp := range ch {
		content.WriteString(resp.Content)
		if resp.FinishReason != "" {
			finish = resp.FinishReason
			usageTotal = resp.Usage.TotalTokens
		}
	}
	assert.Equal(t, "Hello", content.String())
	assert.Equal(t, "stop", finish)
	assert.Equal(t, 5, usageTotal)
}
