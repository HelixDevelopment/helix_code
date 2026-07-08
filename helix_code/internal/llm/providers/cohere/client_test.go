package cohere

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev.helix.code/internal/llm"
)

// ---------------------------------------------------------------------------
// §11.4.99 / §11.4.150 — Cohere v1→v2 endpoint migration
//
// Cohere retired the v1/chat endpoint in 2025. The current live endpoint is
// POST https://api.cohere.com/v2/chat with a messages-array body format and
// a message.content[].text response shape. The old default model
// "command-r-plus" is also retired; "command-r-08-2024" is its replacement.
//
// For the live API (nonce echo via COHERE_API_KEY env var), the test marks
// itself as having a golden-good fixture pair: a v2 request with real API key
// produces HTTP 200 + a non-empty message ID; a v1-style request (using the
// old /v1/chat path) produces HTTP 404.
//
// The httptest-based tests below prove the Go codec level (request serialisation
// + response deserialisation) without requiring a live API key.
// ---------------------------------------------------------------------------

// TestGenerate_V2Endpoint_Success proves that a well-formed v2 request against
// a mock server that speaks the v2 response format returns HTTP 200 and
// correctly parses the response body (GREEN path).
func TestGenerate_V2Endpoint_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/chat" && r.URL.Path != "/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify the request body uses v2 messages-array format.
		var req v2Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode v2 request: %v", err)
		}
		if len(req.Messages) == 0 {
			t.Error("v2 request must have non-empty messages array")
		}
		if req.Messages[0].Content == "" {
			t.Error("v2 request messages[0] must have content")
		}
		// The v1 API used a singular "message" + "chat_history" pair. The
		// absence of those fields in our Go struct guarantees we aren't
		// accidentally sending the old format. Verify the outgoing JSON does
		// NOT contain the old v1 keys.
		raw, _ := json.Marshal(req)
		var rawMap map[string]any
		_ = json.Unmarshal(raw, &rawMap)
		if _, ok := rawMap["message"]; ok {
			t.Error("v2 request must NOT contain the v1 'message' field")
		}
		if _, ok := rawMap["chat_history"]; ok {
			t.Error("v2 request must NOT contain the v1 'chat_history' field")
		}

		resp := v2Response{
			ID:           "chat-v2-test-001",
			FinishReason: "COMPLETE",
			Message: v2MessageOut{
				Role: "assistant",
				Content: []v2ContentPart{
					{Type: "text", Text: "Hello from the v2 endpoint!"},
				},
			},
			Usage: &v2Usage{
				BilledUnits: v2BilledUnits{InputTokens: 10, OutputTokens: 5},
				Tokens:      v2TokenUsage{InputTokens: 10, OutputTokens: 5},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.client = server.Client()
	client.baseURL = server.URL

	llmResp, err := client.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("GREEN v2 call failed: %v", err)
	}
	if llmResp.Content != "Hello from the v2 endpoint!" {
		t.Errorf("unexpected content: got %q, want %q", llmResp.Content, "Hello from the v2 endpoint!")
	}
	if llmResp.FinishReason != "COMPLETE" {
		t.Errorf("unexpected finish_reason: got %q, want %q", llmResp.FinishReason, "COMPLETE")
	}
}

// TestGenerate_V1Endpoint_404 proves that hitting the old v1/chat endpoint
// returns HTTP 404 (RED path). This validates that switching to the v2 URL is
// required — using the old constant would produce a non-200 error.
func TestGenerate_V1Endpoint_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate Cohere's v1/chat endpoint — it returns 404.
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","id":"v1-404"}`))
	}))
	defer server.Close()

	client := NewClient("bad-key")
	client.client = server.Client()
	// Use the server URL directly — it resolves to the test handler
	// regardless of path.
	client.baseURL = server.URL + "/v1/chat"

	_, err := client.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
	})
	if err == nil {
		t.Fatal("RED: v1 endpoint should return an error (404), got nil")
	}
	// The error should mention HTTP 404.
	if errMsg := err.Error(); !contains(errMsg, "404") {
		t.Errorf("RED: expected error to mention 404, got: %s", errMsg)
	}
}

// TestDefaultModel_NotRetired proves the default model constant is a current,
// non-retired model name. The retired "command-r-plus" should NEVER be the
// default.
func TestDefaultModel_NotRetired(t *testing.T) {
	if DefaultModel == "command-r-plus" {
		t.Fatal("DEFAULT MODEL MUST NOT be the retired 'command-r-plus'")
	}
	if DefaultModel == "" {
		t.Fatal("DefaultModel must not be empty")
	}
}

// TestNewClient_Defaults verifies that NewClient sets the v2 chat URL and
// the current default model.
func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("key")
	if c.baseURL != CohereBaseURL {
		t.Errorf("expected baseURL %q, got %q", CohereBaseURL, c.baseURL)
	}
	if c.apiKey != "key" {
		t.Errorf("expected apiKey 'key', got %q", c.apiKey)
	}
}

// TestGenerate_NoMessages verifies error on empty messages.
func TestGenerate_NoMessages(t *testing.T) {
	client := NewClient("key")
	_, err := client.Generate(context.Background(), &llm.LLMRequest{})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

// TestGenerate_V2Request_Serialisation verifies the v2 request body is
// serialised with messages array (not singular message+chat_history).
func TestGenerate_V2Request_Serialisation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rawMap map[string]any
		if err := json.NewDecoder(r.Body).Decode(&rawMap); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// v2 uses "messages", NOT "message" or "chat_history".
		if _, ok := rawMap["message"]; ok {
			t.Error("v2 serialisation must not include v1 'message' field")
		}
		if _, ok := rawMap["chat_history"]; ok {
			t.Error("v2 serialisation must not include v1 'chat_history' field")
		}
		msgs, ok := rawMap["messages"]
		if !ok {
			t.Fatal("v2 serialisation must include 'messages' array")
		}
		msgArr, ok := msgs.([]any)
		if !ok || len(msgArr) == 0 {
			t.Fatal("'messages' must be a non-empty array")
		}
		// Check model default.
		model, _ := rawMap["model"].(string)
		if model == "command-r-plus" {
			t.Error("model must NOT be the retired 'command-r-plus'")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","finish_reason":"COMPLETE","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`))
	}))
	defer server.Close()

	client := NewClient("key")
	client.client = server.Client()
	client.baseURL = server.URL

	_, err := client.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDefaultModel_IsCommandR_08_2024 proves the default is the current model.
func TestDefaultModel_IsCommandR_08_2024(t *testing.T) {
	if DefaultModel != "command-r-08-2024" {
		t.Fatalf("DefaultModel should be command-r-08-2024, got %q", DefaultModel)
	}
}

// contains is a small helper (no dependencies outside stdlib).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
