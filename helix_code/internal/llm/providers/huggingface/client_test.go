package huggingface

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/llm"
)

// TestClient_Generate_RequestAndResponse exercises the REAL client.Generate
// request-building (URL/model routing, headers, body) and response-parsing
// (decoding the HF array-of-objects payload into llm.LLMResponse) against a
// real httptest.Server. No reimplementation of the client's logic — the
// server merely observes what the real code sent and returns canned bytes.
func TestClient_Generate_RequestAndResponse(t *testing.T) {
	tests := []struct {
		name       string
		req        *llm.LLMRequest
		wantModel  string // expected model segment in the request path
		wantPrompt string // expected "inputs" field the real client should build
	}{
		{
			name: "default model used when request has no model, prompt is last message",
			req: &llm.LLMRequest{
				Messages: []llm.Message{
					{Role: "user", Content: "Hello there"},
				},
			},
			wantModel:  "mistralai/Mistral-7B-Instruct-v0.2",
			wantPrompt: "Hello there",
		},
		{
			name: "custom model honored, last of multiple messages used as prompt",
			req: &llm.LLMRequest{
				Model: "gpt2",
				Messages: []llm.Message{
					{Role: "user", Content: "first message"},
					{Role: "assistant", Content: "second message"},
					{Role: "user", Content: "third message wins"},
				},
			},
			wantModel:  "gpt2",
			wantPrompt: "third message wins",
		},
		{
			name: "no messages produces empty prompt but still calls default model",
			req: &llm.LLMRequest{
				Model: "some/custom-model",
			},
			wantModel:  "some/custom-model",
			wantPrompt: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const apiKey = "test-api-key-123"
			var (
				gotMethod  string
				gotPath    string
				gotAuth    string
				gotCT      string
				gotBody    hfRequest
				handlerHit bool
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerHit = true
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotAuth = r.Header.Get("Authorization")
				gotCT = r.Header.Get("Content-Type")

				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(raw, &gotBody))

				w.WriteHeader(http.StatusOK)
				resp := []hfResponse{
					{GeneratedText: "generated-for: " + gotBody.Inputs},
				}
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			}))
			defer server.Close()

			c := NewClient(apiKey)
			// White-box: point the real client at our test server instead of
			// the real HF endpoint, exactly like the constructor would if
			// configured with a custom base URL.
			c.baseURL = server.URL

			resp, err := c.Generate(context.Background(), tt.req)
			require.NoError(t, err)
			require.True(t, handlerHit, "server handler must have been invoked by the real client")

			// (a) assert the REAL request the client built.
			assert.Equal(t, http.MethodPost, gotMethod)
			assert.Equal(t, "/"+tt.wantModel, gotPath)
			assert.Equal(t, "Bearer "+apiKey, gotAuth)
			assert.Equal(t, "application/json", gotCT)
			assert.Equal(t, tt.wantPrompt, gotBody.Inputs)

			// (b) assert the REAL response parsing produced the right content.
			require.NotNil(t, resp)
			assert.Equal(t, "generated-for: "+tt.wantPrompt, resp.Content)
		})
	}
}

// TestClient_Generate_NonOKStatus asserts the real client wraps a non-200
// upstream status into an error rather than silently succeeding.
func TestClient_Generate_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	c := NewClient("key")
	c.baseURL = server.URL

	resp, err := c.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "huggingface API error")
	assert.Contains(t, err.Error(), "500")
}

// TestClient_Generate_EmptyResultsArray asserts the real client treats an
// empty JSON array response as an error (no generated text available).
func TestClient_Generate_EmptyResultsArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := NewClient("key")
	c.baseURL = server.URL

	resp, err := c.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "empty response from huggingface")
}

// TestClient_Generate_MalformedJSON asserts the real client surfaces a
// decode error rather than panicking or returning a zero-value success.
func TestClient_Generate_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json{`))
	}))
	defer server.Close()

	c := NewClient("key")
	c.baseURL = server.URL

	resp, err := c.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "decode response")
}

// TestNewClient asserts the constructor wires the API key and default base
// URL that the rest of the package relies on.
func TestNewClient(t *testing.T) {
	c := NewClient("my-secret-key")
	require.NotNil(t, c)
	assert.Equal(t, "my-secret-key", c.apiKey)
	assert.Equal(t, BaseURL, c.baseURL)
	assert.NotNil(t, c.client)
}
