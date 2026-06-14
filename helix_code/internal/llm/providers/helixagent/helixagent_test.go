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
