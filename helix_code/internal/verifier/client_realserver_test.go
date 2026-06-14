package verifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realVerifierMux mimics the REAL LLMsVerifier server's HTTP surface and exact
// JSON response shapes as served by submodules/llms_verifier/llm-verifier/api
// (server.go + handlers.go):
//   - GET /api/health    → {"status":"healthy","timestamp":<unix int>,...}
//   - GET /api/models    → {"models":[{...,"status":...,"score":...}],"count":N}
//   - GET /api/providers → {"providers":[{...,"reliability_score":...}],"count":N}
//
// It deliberately does NOT register /api/scores — exactly like the real server —
// so the reconciliation (fall back to /api/providers) is exercised.
func realVerifierMux(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// timestamp is a Unix INTEGER, as the real HealthHandler emits.
		_, _ = w.Write([]byte(`{"status":"healthy","timestamp":1700000000,"database":"connected","database_status":"ok"}`))
	})

	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// Envelope shape with `models` + `count`, `status` (not
		// `verification_status`), and numeric `score`.
		_, _ = w.Write([]byte(`{
			"models": [
				{"id":"gpt-4o","model_id":"gpt-4o","name":"GPT-4o","provider":"openai","provider_id":1,"status":"verified","score":9.1,"capabilities":["text","vision"],"deprecated":false},
				{"id":"claude-3-5-sonnet","model_id":"claude-3-5-sonnet","name":"Claude 3.5 Sonnet","provider":"anthropic","provider_id":2,"status":"verified","score":8.9,"capabilities":["text"],"deprecated":false}
			],
			"count": 2
		}`))
	})

	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/providers", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"providers": [
				{"id":1,"name":"openai","status":"active","is_active":true,"models":12,"reliability_score":9.1},
				{"id":2,"name":"anthropic","status":"active","is_active":true,"models":8,"reliability_score":8.9}
			],
			"count": 2
		}`))
	})

	return mux
}

func TestClient_Health_RealServer_UnixTimestamp(t *testing.T) {
	server := httptest.NewServer(realVerifierMux(t))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	hr, err := c.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", hr.Status)
	// Unix integer 1700000000 must parse into a real time, not zero/error.
	assert.Equal(t, time.Unix(1700000000, 0).UTC(), hr.Timestamp)
}

func TestClient_GetModels_RealServer_Envelope(t *testing.T) {
	server := httptest.NewServer(realVerifierMux(t))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	models, err := c.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2, "must parse the {\"models\":[...]} envelope, not yield empty")

	assert.Equal(t, "gpt-4o", models[0].ID)
	assert.Equal(t, "openai", models[0].Provider)
	assert.Equal(t, 9.1, models[0].Score, "real server `score` must map onto VerifiedModel.Score")
	assert.Equal(t, "verified", models[0].VerificationStatus, "real server `status` must map onto VerificationStatus")
	assert.Equal(t, []string{"text", "vision"}, models[0].Capabilities)

	assert.Equal(t, "claude-3-5-sonnet", models[1].ID)
	assert.Equal(t, "anthropic", models[1].Provider)
	assert.Equal(t, 8.9, models[1].Score)
}

func TestClient_GetProviderScores_RealServer_FallsBackToProviders(t *testing.T) {
	server := httptest.NewServer(realVerifierMux(t))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	// /api/scores is absent on the real server (404) → must fall back to
	// /api/providers and derive the score map from reliability_score.
	scores, err := c.GetProviderScores(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 9.1, scores["openai"])
	assert.Equal(t, 8.9, scores["anthropic"])
}

// TestClient_GetModels_RealServer_EmptyEnvelope proves a legitimate "zero
// models" result from the real server — `{"models":[],"count":0}` — yields an
// empty slice and NO error. The earlier `len(envelope.Models) > 0` branch
// condition fell through to the bare-array path on an empty `models` array,
// which then tried to unmarshal the whole `{...}` object as a JSON array and
// returned a decode error for a perfectly valid empty result.
func TestClient_GetModels_RealServer_EmptyEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[],"count":0}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	models, err := c.GetModels(context.Background())
	require.NoError(t, err, "an empty {\"models\":[]} envelope must NOT error")
	assert.Empty(t, models, "an empty envelope must yield zero models")
}

// TestClient_GetModels_RealServer_EmptyEnvelopeVariants proves the
// envelope-vs-bare-array detection keys on "is the body an object" rather than
// "does the models array contain elements". An envelope object MUST take the
// envelope path even when `models` is empty, `null`, or absent — never fall
// through to the bare-array path which would try to unmarshal the whole `{...}`
// object as a JSON array and return a spurious decode error.
func TestClient_GetModels_RealServer_EmptyEnvelopeVariants(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty array", `{"models":[],"count":0}`},
		{"null models", `{"models":null,"count":0}`},
		{"absent models key", `{"count":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			models, err := decodeModels([]byte(tc.body))
			require.NoError(t, err, "object envelope %q must NOT error", tc.body)
			assert.Empty(t, models, "object envelope %q must yield zero models", tc.body)
		})
	}
}

// TestClient_GetModels_EmbeddedShape_StillWorks proves the reconciliation did
// NOT break the legacy bare-array shape served by the embedded server.
func TestClient_GetModels_EmbeddedShape_StillWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"llama-3.2-3b","name":"Llama 3.2 3B","provider":"ollama","overall_score":6.0}]`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	models, err := c.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "llama-3.2-3b", models[0].ID)
	assert.Equal(t, 6.0, models[0].OverallScore)
}
