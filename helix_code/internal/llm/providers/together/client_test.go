package together

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
// request-building (model defaulting, message conversion, max_tokens /
// temperature defaulting) and response-parsing (decoding the OpenAI-style
// choices[0].message.content payload) against a real httptest.Server.
func TestClient_Generate_RequestAndResponse(t *testing.T) {
	tests := []struct {
		name     string
		req      *llm.LLMRequest
		wantBody togetherRequest
	}{
		{
			name: "default model and defaults applied when unset",
			req: &llm.LLMRequest{
				Messages: []llm.Message{
					{Role: "user", Content: "Hi there"},
				},
			},
			wantBody: togetherRequest{
				Model: "mistralai/Mixtral-8x22B-Instruct-v0.1",
				Messages: []togetherMessage{
					{Role: "user", Content: "Hi there"},
				},
				MaxTokens:   4096,
				Temperature: 0.7,
			},
		},
		{
			name: "custom model, explicit max_tokens/temperature preserved, all messages converted in order",
			req: &llm.LLMRequest{
				Model: "meta-llama/Llama-3-70b-chat-hf",
				Messages: []llm.Message{
					{Role: "system", Content: "you are helpful"},
					{Role: "user", Content: "what is 2+2"},
				},
				MaxTokens:   256,
				Temperature: 0.2,
			},
			wantBody: togetherRequest{
				Model: "meta-llama/Llama-3-70b-chat-hf",
				Messages: []togetherMessage{
					{Role: "system", Content: "you are helpful"},
					{Role: "user", Content: "what is 2+2"},
				},
				MaxTokens:   256,
				Temperature: 0.2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const apiKey = "test-together-key"
			var (
				gotMethod  string
				gotAuth    string
				gotCT      string
				gotBody    togetherRequest
				handlerHit bool
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerHit = true
				gotMethod = r.Method
				gotAuth = r.Header.Get("Authorization")
				gotCT = r.Header.Get("Content-Type")

				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(raw, &gotBody))

				w.WriteHeader(http.StatusOK)
				resp := togetherResponse{
					Choices: []togetherChoice{
						{Message: togetherMessage{Role: "assistant", Content: "the answer is 4"}},
					},
				}
				require.NoError(t, json.NewEncoder(w).Encode(resp))
			}))
			defer server.Close()

			c := NewClient(apiKey)
			// White-box: point the real client at our test server instead of
			// the real Together endpoint.
			c.baseURL = server.URL

			resp, err := c.Generate(context.Background(), tt.req)
			require.NoError(t, err)
			require.True(t, handlerHit, "server handler must have been invoked by the real client")

			// (a) assert the REAL request the client built.
			assert.Equal(t, http.MethodPost, gotMethod)
			assert.Equal(t, "Bearer "+apiKey, gotAuth)
			assert.Equal(t, "application/json", gotCT)
			assert.Equal(t, tt.wantBody, gotBody)

			// (b) assert the REAL response parsing produced the right content.
			require.NotNil(t, resp)
			assert.Equal(t, "the answer is 4", resp.Content)
		})
	}
}

// TestClient_Generate_NonOKStatus asserts the real client wraps a non-200
// upstream status into an error rather than silently succeeding.
func TestClient_Generate_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	c := NewClient("key")
	c.baseURL = server.URL

	resp, err := c.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "together API error")
	assert.Contains(t, err.Error(), "429")
}

// TestClient_Generate_EmptyChoices asserts the real client treats a response
// with zero choices as an error.
func TestClient_Generate_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(togetherResponse{Choices: nil}))
	}))
	defer server.Close()

	c := NewClient("key")
	c.baseURL = server.URL

	resp, err := c.Generate(context.Background(), &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no choices in response")
}

// TestClient_Generate_MalformedJSON asserts the real client surfaces a
// decode error rather than panicking or returning a zero-value success.
func TestClient_Generate_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
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
	assert.Equal(t, TogetherBaseURL, c.baseURL)
	assert.NotNil(t, c.client)
}
