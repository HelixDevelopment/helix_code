package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// These tests prove OpenAI-compatible function-calling (tool-calling) support
// for the four providers that previously dropped tools on the request and/or
// the model's tool_calls on the response: Groq, DeepSeek, Mistral, OpenRouter.
//
// Each provider gets two assertions in the NON-STREAMING Generate path:
//   Test A (send):  the outgoing request body contains a "tools" array with the
//                   tool name (proves request.Tools reaches the wire).
//   Test B (parse): a Chat Completions response whose
//                   choices[0].message.tool_calls carries one entry is mapped
//                   into *LLMResponse.ToolCalls with the tool name AND its
//                   decoded arguments map (proves tool_calls are parsed back).
//
// All four are OpenAI Chat Completions compatible, so a single httptest.Server
// handler shape works for all of them. The provider base URL / endpoint is
// pointed at the test server via ProviderConfigEntry.Endpoint (and an APIKey so
// the constructor does not error out on the missing-key guard). Anti-bluff: the
// handler captures the REAL request body and returns a REAL Chat Completions
// JSON payload; nothing is simulated.

// toolTestRequest builds a Generate request carrying one function tool.
func toolTestRequest(model string) *LLMRequest {
	return &LLMRequest{
		ID:    uuid.New(),
		Model: model,
		Messages: []Message{
			{Role: "user", Content: "What changed in the repo?"},
		},
		MaxTokens: 256,
		Tools: []Tool{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "git_status",
					Description: "Show the working tree status",
					Parameters: map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			},
		},
		ToolChoice: "auto",
	}
}

// toolCallResponseJSON is a canonical OpenAI Chat Completions response whose
// single choice carries one tool_call. `arguments` is a JSON STRING (the
// OpenAI canonical encoding of a serialized object) to exercise the
// string→map decode path.
const toolCallResponseJSON = `{
  "id": "chatcmpl-tooltest",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "test-model",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "",
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "git_status",
              "arguments": "{\"path\":\".\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18}
}`

// toolTestServer stands up an httptest.Server that captures the raw request
// body into *capturedBody and replies with toolCallResponseJSON for any
// chat-completions POST (and an empty models list for any GET so health/catalog
// probes do not panic).
func toolTestServer(t *testing.T, capturedBody *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// /models style probe (catalog refresh / health)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		*capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, toolCallResponseJSON)
	}))
}

// assertToolsSent fails if the captured outgoing body does not contain a
// "tools" array referencing the tool name.
func assertToolsSent(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `"tools"`) {
		t.Fatalf("outgoing request body did not contain a \"tools\" array; body=%s", body)
	}
	if !strings.Contains(body, `"git_status"`) {
		t.Fatalf("outgoing request body did not contain the tool name git_status; body=%s", body)
	}
	// Confirm tools really is a JSON array carrying the function, not just a
	// substring coincidence.
	var wire struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
		ToolChoice interface{} `json:"tool_choice"`
	}
	if err := json.Unmarshal([]byte(body), &wire); err != nil {
		t.Fatalf("outgoing body is not valid JSON: %v; body=%s", err, body)
	}
	if len(wire.Tools) != 1 {
		t.Fatalf("expected exactly 1 tool on the wire, got %d; body=%s", len(wire.Tools), body)
	}
	if wire.Tools[0].Function.Name != "git_status" {
		t.Fatalf("expected tools[0].function.name=git_status, got %q", wire.Tools[0].Function.Name)
	}
	if wire.ToolChoice != "auto" {
		t.Fatalf("expected tool_choice=auto on the wire, got %v", wire.ToolChoice)
	}
}

// assertToolCallsParsed fails if the response did not yield exactly one parsed
// tool call named git_status with its arguments map decoded.
func assertToolCallsParsed(t *testing.T, resp *LLMResponse) {
	t.Helper()
	if resp == nil {
		t.Fatal("nil response")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 parsed tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Function.Name != "git_status" {
		t.Fatalf("expected parsed tool call name git_status, got %q", tc.Function.Name)
	}
	if tc.ID != "call_abc123" {
		t.Fatalf("expected tool call id call_abc123, got %q", tc.ID)
	}
	// arguments arrived as a JSON string "{\"path\":\".\"}" and must decode
	// into the map[string]interface{} downstream consumers expect.
	if got, ok := tc.Function.Arguments["path"]; !ok || got != "." {
		t.Fatalf("expected decoded arguments map {path:.}, got %#v", tc.Function.Arguments)
	}
}

func TestGroqProvider_ToolCalling(t *testing.T) {
	var captured string
	srv := toolTestServer(t, &captured)
	defer srv.Close()

	p, err := NewGroqProvider(ProviderConfigEntry{Endpoint: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewGroqProvider: %v", err)
	}

	resp, err := p.Generate(context.Background(), toolTestRequest("llama-3.1-8b-instant"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	assertToolsSent(t, captured)   // Test A: tools reach the wire
	assertToolCallsParsed(t, resp) // Test B: tool_calls parsed back
}

func TestDeepSeekProvider_ToolCalling(t *testing.T) {
	var captured string
	srv := toolTestServer(t, &captured)
	defer srv.Close()

	p, err := NewDeepSeekProvider(ProviderConfigEntry{Endpoint: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewDeepSeekProvider: %v", err)
	}

	resp, err := p.Generate(context.Background(), toolTestRequest("deepseek-chat"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	assertToolsSent(t, captured)
	assertToolCallsParsed(t, resp)
}

func TestMistralProvider_ToolCalling(t *testing.T) {
	var captured string
	srv := toolTestServer(t, &captured)
	defer srv.Close()

	p, err := NewMistralProvider(ProviderConfigEntry{Endpoint: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewMistralProvider: %v", err)
	}

	resp, err := p.Generate(context.Background(), toolTestRequest("mistral-large-latest"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	assertToolsSent(t, captured)
	assertToolCallsParsed(t, resp)
}

func TestOpenRouterProvider_ToolCalling(t *testing.T) {
	var captured string
	srv := toolTestServer(t, &captured)
	defer srv.Close()

	p, err := NewOpenRouterProvider(ProviderConfigEntry{Endpoint: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewOpenRouterProvider: %v", err)
	}

	resp, err := p.Generate(context.Background(), toolTestRequest("openai/gpt-oss-20b:free"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	assertToolsSent(t, captured)
	assertToolCallsParsed(t, resp)
}
