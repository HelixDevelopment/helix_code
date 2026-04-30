package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_NewClient_WithDefaults(t *testing.T) {
	c := NewClient("", "", 0)
	assert.Equal(t, "http://localhost:8081", c.baseURL)
	assert.Equal(t, 30*time.Second, c.timeout)
}

func TestClient_NewClient_WithCustomTimeout(t *testing.T) {
	c := NewClient("http://example.com", "key", 10*time.Second)
	assert.Equal(t, "http://example.com", c.baseURL)
	assert.Equal(t, "key", c.apiKey)
	assert.Equal(t, 10*time.Second, c.timeout)
}

func TestClient_Health_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "healthy", Version: "1.0.0"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	hr, err := c.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", hr.Status)
}

func TestClient_Health_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	_, err := c.Health(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 503")
}

func TestClient_GetModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/models", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		models := []*VerifiedModel{
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", OverallScore: 9.1},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models)
	}))
	defer server.Close()

	c := NewClient(server.URL, "secret", 0)
	models, err := c.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "gpt-4o", models[0].ID)
}

func TestClient_GetModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	_, err := c.GetModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestClient_GetModelByID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/models/missing", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	_, err := c.GetModelByID(context.Background(), "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestClient_GetProviderScores_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/scores", r.URL.Path)
		scores := map[string]float64{"openai": 9.1, "anthropic": 8.9}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(scores)
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	scores, err := c.GetProviderScores(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 9.1, scores["openai"])
}

func TestClient_VerifyModel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/models/gpt-4o/verify", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		result := VerificationResult{ModelID: "gpt-4o", Status: "completed", OverallScore: 9.1}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	result, err := c.VerifyModel(context.Background(), "gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", result.ModelID)
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	c := NewClient(server.URL, "", 0)
	_, err := c.GetModels(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestClient_AuthHeaderRedacted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sk-secret", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]*VerifiedModel{})
	}))
	defer server.Close()

	c := NewClient(server.URL, "sk-secret", 0)
	_, err := c.GetModels(context.Background())
	require.NoError(t, err)
}
