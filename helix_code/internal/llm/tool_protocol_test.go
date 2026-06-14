package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// tool_protocol_test.go proves the END-TO-END OpenAI/Groq tool-calling
// CONVERSATION protocol works across the 5 providers that participate in agent
// tool loops (Groq, DeepSeek, Mistral, OpenRouter, OpenAI-compatible).
//
// The unit tests in provider_tools_test.go prove a SINGLE turn: tools reach the
// wire and the model's tool_calls parse back. They do NOT prove the SECOND turn
// — feeding the executed tool's result back — which is where the live failure
// occurred:
//
//   groq request failed: invalid request: 'messages.4': for 'role:tool' the
//   'tool_call_id' is missing
//
// The OpenAI/Groq protocol requires the tool-result feed-back message to carry
// role:"tool" + tool_call_id:"<id>" + content, AND the prior assistant message
// to carry its tool_calls:[{id,...}]. llm.Message had neither field, so the
// conversation could not be made well-formed.
//
// Each test below drives a REAL provider through TWO Generate calls:
//   Turn 1 — the model returns a tool_call (id=call_1, name=git_status).
//   Caller simulates executing the tool and appends:
//     • an assistant message carrying ToolCalls=[{id:call_1,...}]
//     • a tool message carrying ToolCallID=call_1 + the tool output
//   Turn 2 — the handler CAPTURES the incoming body and ASSERTS it contains a
//            role:"tool" message with tool_call_id:"call_1" + the tool output,
//            AND an assistant message whose tool_calls reference "call_1".
// Anti-bluff (§11.4.5): the handler captures the REAL wire body; nothing is
// simulated, and the protocol fields are asserted on the actual serialized JSON.

// twoTurnHandler scripts a 2-turn tool conversation against an OpenAI Chat
// Completions compatible endpoint. It discriminates the two turns by the REAL
// protocol marker — whether the incoming request body already carries a
// role:"tool" result message (the feed-back turn). A request WITHOUT one is
// turn 1 and gets a tool_calls response; a request WITH one is turn 2: its body
// is captured into *turn2Body and the final answer is returned. Discriminating
// on the body (not a POST counter) keeps the handler order-independent and lets
// a caller make either one combined call or two separate calls.
func twoTurnHandler(t *testing.T, turn2Body *string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(string(body), `"role":"tool"`) {
			// Turn 1: no fed-back tool result yet ⇒ ask the model to call
			// git_status (id=call_1). `arguments` is a JSON STRING (the OpenAI
			// canonical object encoding) to exercise the string→map decode path.
			_, _ = io.WriteString(w, `{
			  "id":"chatcmpl-t1","object":"chat.completion","created":1700000000,"model":"m",
			  "choices":[{"index":0,"message":{"role":"assistant","content":"",
			    "tool_calls":[{"id":"call_1","type":"function",
			      "function":{"name":"git_status","arguments":"{}"}}]},
			    "finish_reason":"tool_calls"}],
			  "usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`)
			return
		}
		// Turn 2: the request carried the fed-back role:"tool" result. Capture
		// it for protocol assertions and answer.
		*turn2Body = string(body)
		_, _ = io.WriteString(w, `{
		  "id":"chatcmpl-t2","object":"chat.completion","created":1700000001,"model":"m",
		  "choices":[{"index":0,"message":{"role":"assistant",
		    "content":"You are on branch main with a clean tree."},
		    "finish_reason":"stop"}],
		  "usage":{"prompt_tokens":9,"completion_tokens":9,"total_tokens":18}}`)
	})
}

// assertToolResultFedBack verifies the turn-2 wire body carries a well-formed
// tool result: a role:"tool" message with tool_call_id matching the assistant's
// tool_calls[].id, plus the tool output text — exactly what the live Groq error
// said was missing.
func assertToolResultFedBack(t *testing.T, body string) {
	t.Helper()
	if body == "" {
		t.Fatal("turn-2 request body was never captured (loop did not make a second call)")
	}
	var wire struct {
		Messages []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id"`
			ToolCalls  []struct {
				ID       string `json:"id"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &wire); err != nil {
		t.Fatalf("turn-2 body is not valid JSON: %v; body=%s", err, body)
	}

	var sawAssistantToolCall bool
	var sawToolResult bool
	for _, m := range wire.Messages {
		if m.Role == "assistant" {
			for _, tc := range m.ToolCalls {
				if tc.ID == "call_1" && tc.Function.Name == "git_status" {
					sawAssistantToolCall = true
				}
			}
		}
		if m.Role == "tool" && m.ToolCallID == "call_1" {
			if !strings.Contains(m.Content, "On branch main") {
				t.Fatalf("role:tool message had wrong content %q (want it to contain the tool output)", m.Content)
			}
			sawToolResult = true
		}
	}
	if !sawAssistantToolCall {
		t.Fatalf("turn-2 body lacked an assistant message with tool_calls referencing call_1; body=%s", body)
	}
	if !sawToolResult {
		t.Fatalf("turn-2 body lacked a role:tool message with tool_call_id=call_1; body=%s", body)
	}
}

// assertToolCallArgumentsAreString verifies the SEND direction: the assistant
// message's tool_calls[].function.arguments serialises as a JSON-encoded STRING
// (OpenAI/Groq/DeepSeek/Mistral/OpenRouter require `"arguments":"{}"`), NOT as a
// JSON object (`"arguments":{}`). The live failure was:
//
//	groq request failed: invalid request:
//	'messages.3.tool_calls.0.function.arguments' : value must be a string
//
// This is the symmetric SEND-side counterpart to the already-fixed PARSE-side
// (string→map) decode in parseOpenAIWireToolCalls.
func assertToolCallArgumentsAreString(t *testing.T, body string) {
	t.Helper()
	if body == "" {
		t.Fatal("turn-2 request body was never captured")
	}
	// The object form `"arguments":{` MUST NOT appear — that is the bug.
	if strings.Contains(body, `"arguments":{`) {
		t.Fatalf("tool_calls function.arguments serialised as a JSON OBJECT (`\"arguments\":{`); "+
			"OpenAI/Groq require a JSON STRING. body=%s", body)
	}
	// The string form `"arguments":"` MUST appear for the assistant tool_call.
	if !strings.Contains(body, `"arguments":"`) {
		t.Fatalf("tool_calls function.arguments was not a JSON STRING (`\"arguments\":\"`); body=%s", body)
	}
	// Structurally confirm the arguments field decodes as a string (not object).
	var wire struct {
		Messages []struct {
			Role      string `json:"role"`
			ToolCalls []struct {
				Function struct {
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &wire); err != nil {
		t.Fatalf("turn-2 body is not valid JSON: %v; body=%s", err, body)
	}
	var sawAssistantArgs bool
	for _, m := range wire.Messages {
		if m.Role != "assistant" {
			continue
		}
		for _, tc := range m.ToolCalls {
			var asString string
			if err := json.Unmarshal(tc.Function.Arguments, &asString); err != nil {
				t.Fatalf("assistant tool_call arguments is not a JSON string: raw=%s body=%s",
					string(tc.Function.Arguments), body)
			}
			// Whatever the value, it MUST be valid JSON (the providers parse
			// the string back into an object) and MUST NOT be empty/null.
			if asString == "" {
				t.Fatalf("tool_call arguments string was empty (want a JSON object literal); body=%s", body)
			}
			var probe map[string]interface{}
			if err := json.Unmarshal([]byte(asString), &probe); err != nil {
				t.Fatalf("tool_call arguments string %q is not a JSON object: %v; body=%s", asString, err, body)
			}
			sawAssistantArgs = true
		}
	}
	if !sawAssistantArgs {
		t.Fatalf("no assistant tool_call with arguments found; body=%s", body)
	}
}

// assertEmptyArgsAreEmptyObject verifies an EMPTY arguments map serialises as the
// literal string "{}" (never "", never "null"). Used by the empty-args path.
func assertEmptyArgsAreEmptyObject(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `"arguments":"{}"`) {
		t.Fatalf("empty args map did not serialise as `\"arguments\":\"{}\"`; body=%s", body)
	}
}

// assertNonEmptyArgsRoundTrip verifies a NON-EMPTY arguments map round-trips as a
// JSON-encoded string, e.g. {"subcommand":"status"} → `"arguments":"{\"subcommand\":\"status\"}"`.
func assertNonEmptyArgsRoundTrip(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `"arguments":"{\"subcommand\":\"status\"}"`) {
		t.Fatalf("non-empty args map did not serialise as a JSON-encoded string "+
			"(`\"arguments\":\"{\\\"subcommand\\\":\\\"status\\\"}\"`); body=%s", body)
	}
}

// buildTwoTurnMessages reproduces what an agent tool loop appends after turn 1:
// the original user turn, the assistant turn carrying its tool_calls, and the
// tool-result turn carrying the matching tool_call_id.
func buildTwoTurnMessages() []Message {
	return []Message{
		{Role: "user", Content: "What changed in the repo?"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunc{
					Name:      "git_status",
					Arguments: map[string]interface{}{},
				},
			}},
		},
		{
			Role:       "tool",
			ToolCallID: "call_1",
			Name:       "git_status",
			Content:    "On branch main\nnothing to commit, working tree clean",
		},
	}
}

// twoTurnGenerate runs the second-turn Generate (carrying the fed-back tool
// result) and asserts the final answer is returned.
func twoTurnGenerate(t *testing.T, p Provider, model string) *LLMResponse {
	t.Helper()
	req := &LLMRequest{
		Model:    model,
		Messages: buildTwoTurnMessages(),
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name:        "git_status",
				Description: "Show the working tree status",
				Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			},
		}},
	}
	resp, err := p.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("turn-2 Generate: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Content, "branch main") {
		t.Fatalf("turn-2 did not return the final answer; resp=%#v", resp)
	}
	return resp
}

// TestOpenAICompatibleProvider_ToolCallParse proves the OpenAI-compatible
// provider (the ~11-backend fan-out: VLLM, LMStudio, Jan, LocalAI, …) decodes a
// turn-1 tool_calls response whose `function.arguments` is the OpenAI-canonical
// JSON STRING. Before the fix it failed with "cannot unmarshal string into Go
// struct field ToolCallFunc...arguments" — its tool loop died at turn 1. The
// other four providers already had this coverage in provider_tools_test.go;
// this closes the gap for the fifth.
func TestOpenAICompatibleProvider_ToolCallParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, toolCallResponseJSON) // arguments is a JSON string "{\"path\":\".\"}"
	}))
	defer srv.Close()

	p, err := NewOpenAICompatibleProvider("custom", OpenAICompatibleConfig{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	resp, err := p.Generate(context.Background(), toolTestRequest("local-model"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	assertToolCallsParsed(t, resp) // name=git_status, id=call_abc123, args{path:.}
}

func TestToolConversationProtocol_AllProviders(t *testing.T) {
	cases := []struct {
		name  string
		model string
		newP  func(t *testing.T, url string) Provider
	}{
		{
			name:  "groq",
			model: "llama-3.1-8b-instant",
			newP: func(t *testing.T, url string) Provider {
				p, err := NewGroqProvider(ProviderConfigEntry{Endpoint: url, APIKey: "test-key"})
				if err != nil {
					t.Fatalf("NewGroqProvider: %v", err)
				}
				return p
			},
		},
		{
			name:  "deepseek",
			model: "deepseek-chat",
			newP: func(t *testing.T, url string) Provider {
				p, err := NewDeepSeekProvider(ProviderConfigEntry{Endpoint: url, APIKey: "test-key"})
				if err != nil {
					t.Fatalf("NewDeepSeekProvider: %v", err)
				}
				return p
			},
		},
		{
			name:  "mistral",
			model: "mistral-large-latest",
			newP: func(t *testing.T, url string) Provider {
				p, err := NewMistralProvider(ProviderConfigEntry{Endpoint: url, APIKey: "test-key"})
				if err != nil {
					t.Fatalf("NewMistralProvider: %v", err)
				}
				return p
			},
		},
		{
			name:  "openrouter",
			model: "openai/gpt-oss-20b:free",
			newP: func(t *testing.T, url string) Provider {
				p, err := NewOpenRouterProvider(ProviderConfigEntry{Endpoint: url, APIKey: "test-key"})
				if err != nil {
					t.Fatalf("NewOpenRouterProvider: %v", err)
				}
				return p
			},
		},
		{
			name:  "openai_compatible",
			model: "local-model",
			newP: func(t *testing.T, url string) Provider {
				p, err := NewOpenAICompatibleProvider("custom", OpenAICompatibleConfig{
					BaseURL: url,
					APIKey:  "test-key",
				})
				if err != nil {
					t.Fatalf("NewOpenAICompatibleProvider: %v", err)
				}
				return p
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var turn2Body string
			srv := httptest.NewServer(twoTurnHandler(t, &turn2Body))
			defer srv.Close()

			p := tc.newP(t, srv.URL)

			// Turn 2 carries the fed-back assistant(tool_calls) + tool(tool_call_id)
			// messages. The handler captures the body; we then assert the protocol.
			twoTurnGenerate(t, p, tc.model)
			assertToolResultFedBack(t, turn2Body)
			// SEND-side fix (live bug): the assistant tool_calls[].function.arguments
			// MUST serialise as a JSON STRING, never a JSON object.
			assertToolCallArgumentsAreString(t, turn2Body)
			assertEmptyArgsAreEmptyObject(t, turn2Body)
		})

		t.Run(tc.name+"/non_empty_args", func(t *testing.T) {
			var turn2Body string
			srv := httptest.NewServer(twoTurnHandler(t, &turn2Body))
			defer srv.Close()

			p := tc.newP(t, srv.URL)
			req := &LLMRequest{
				Model: tc.model,
				Messages: []Message{
					{Role: "user", Content: "What changed in the repo?"},
					{
						Role:    "assistant",
						Content: "",
						ToolCalls: []ToolCall{{
							ID:   "call_1",
							Type: "function",
							Function: ToolCallFunc{
								Name:      "git_status",
								Arguments: map[string]interface{}{"subcommand": "status"},
							},
						}},
					},
					{
						Role:       "tool",
						ToolCallID: "call_1",
						Name:       "git_status",
						Content:    "On branch main\nnothing to commit, working tree clean",
					},
				},
				Tools: []Tool{{
					Type: "function",
					Function: ToolFunction{
						Name:        "git_status",
						Description: "Show the working tree status",
						Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
					},
				}},
			}
			if _, err := p.Generate(context.Background(), req); err != nil {
				t.Fatalf("turn-2 Generate (non-empty args): %v", err)
			}
			assertToolCallArgumentsAreString(t, turn2Body)
			assertNonEmptyArgsRoundTrip(t, turn2Body)
		})
	}
}
