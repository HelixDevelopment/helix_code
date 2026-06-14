package verifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// providers_test.go — RED-first coverage for GetProviders, which exposes the
// REAL LLMsVerifier /api/providers envelope (name + api_url + models +
// is_active/status) so HelixCode can build its OpenAI-compatible providers
// FULLY DYNAMICALLY (CONST-036: the verifier is the single source of truth for
// the provider base URL — NO hardcoded api_url anywhere on the primary path).

// TestClient_GetProviders_RealServerShape asserts the real /api/providers
// envelope — including the `api_url` field that carries each provider's base URL
// — is parsed into VerifierProvider records. The api_url is the load-bearing
// field: it is what the dynamic catalogue consumes INSTEAD of a hardcoded URL.
func TestClient_GetProviders_RealServerShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/providers", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		// Real-server envelope: name + api_url + endpoint + models (list) +
		// is_active + status + reliability_score, wrapped in {providers,count}.
		_, _ = w.Write([]byte(`{
			"providers": [
				{"id":1,"name":"cerebras","api_url":"https://api.cerebras.ai/v1","endpoint":"/chat/completions","status":"active","is_active":true,"reliability_score":9.1,"models":["llama-3.3-70b","qwen-3-32b"]},
				{"id":2,"name":"groq","api_url":"https://api.groq.com/openai/v1","endpoint":"/chat/completions","status":"active","is_active":true,"reliability_score":8.9,"models":["llama-3.1-8b-instant"]}
			],
			"count": 2
		}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "secret", 0)
	providers, err := c.GetProviders(context.Background())
	require.NoError(t, err)
	require.Len(t, providers, 2)

	assert.Equal(t, "cerebras", providers[0].Name)
	assert.Equal(t, "https://api.cerebras.ai/v1", providers[0].APIURL)
	assert.True(t, providers[0].IsActive)
	assert.Equal(t, "active", providers[0].Status)
	assert.Equal(t, []string{"llama-3.3-70b", "qwen-3-32b"}, providers[0].Models)

	assert.Equal(t, "groq", providers[1].Name)
	assert.Equal(t, "https://api.groq.com/openai/v1", providers[1].APIURL)
	assert.Equal(t, []string{"llama-3.1-8b-instant"}, providers[1].Models)
}

// TestClient_GetProviders_ModelsAsCount proves robustness: some real-server
// builds emit `models` as an integer COUNT (e.g. `"models":12`) rather than a
// list. That MUST NOT error — Models is left empty and the record still parses
// (api_url remains the load-bearing field).
func TestClient_GetProviders_ModelsAsCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"providers": [
				{"id":1,"name":"openai","api_url":"https://api.openai.com/v1","status":"active","is_active":true,"models":12,"reliability_score":9.1}
			],
			"count": 1
		}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	providers, err := c.GetProviders(context.Background())
	require.NoError(t, err)
	require.Len(t, providers, 1)
	assert.Equal(t, "openai", providers[0].Name)
	assert.Equal(t, "https://api.openai.com/v1", providers[0].APIURL)
	assert.Empty(t, providers[0].Models, "integer `models` count must not populate the Models slice")
}

// TestClient_GetProviders_Unreachable asserts an unreachable verifier surfaces
// an error (so the TUI can gate the fallback on it), never a silent empty slice.
func TestClient_GetProviders_Unreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "", 0) // port 1 — connection refused
	_, err := c.GetProviders(context.Background())
	require.Error(t, err)
}

// TestClient_GetProviders_HTTPError asserts a non-200 status is an error.
func TestClient_GetProviders_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	_, err := c.GetProviders(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}
