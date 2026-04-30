//go:build integration

package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_BootstrapAndFetchModels verifies the full verifier
// initialization and model fetch cycle against a real HTTP server.
func TestIntegration_BootstrapAndFetchModels(t *testing.T) {
	// Start a mock LLMsVerifier server
	mockServer := newMockVerifierServer()
	defer mockServer.Close()

	cfg := &config.VerifierConfig{
		Enabled:         true,
		Mode:            "remote",
		Endpoint:        mockServer.GetURL(),
		Timeout:         5 * time.Second,
		CacheTTL:        1 * time.Minute,
		PollingInterval: 10 * time.Second,
		Scoring: config.VerifierScoringConfig{
			Weights: config.ScoringWeights{
				CodeCapability:   0.40,
				Responsiveness:   0.20,
				Reliability:      0.20,
				FeatureRichness:  0.15,
				ValueProposition: 0.05,
			},
			MinAcceptableScore: 6.0,
		},
		Health: config.VerifierHealthConfig{
			FailureThreshold:  3,
			RecoveryThreshold: 2,
			CircuitBreaker: config.CircuitBreakerConfig{
				Enabled:         true,
				HalfOpenTimeout: 30 * time.Second,
			},
		},
		Events: config.VerifierEventsConfig{
			Enabled: true,
		},
	}

	// Bootstrap the verifier subsystem
	result, err := Bootstrap(cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	defer result.Shutdown()

	assert.NotNil(t, result.Client)
	assert.NotNil(t, result.Adapter)
	assert.True(t, result.Adapter.IsEnabled())

	// Fetch models through the adapter
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := result.Adapter.GetVerifiedModels(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, models)

	// Verify we got the mock models
	assert.GreaterOrEqual(t, len(models), 2)
	assert.Equal(t, "gpt-4o", models[0].ID)
	assert.Equal(t, "openai", models[0].Provider)

	// Verify score lookup works
	score, found := result.Adapter.GetModelScore("gpt-4o")
	assert.True(t, found)
	assert.Equal(t, 9.1, score)
}

// TestIntegration_CacheHit verifies that repeated calls use the cache.
func TestIntegration_CacheHit(t *testing.T) {
	mockServer := newMockVerifierServer()
	defer mockServer.Close()

	cfg := &config.VerifierConfig{
		Enabled:  true,
		Mode:     "remote",
		Endpoint: mockServer.GetURL(),
		Timeout:  5 * time.Second,
		CacheTTL: 5 * time.Minute,
		Scoring:  config.VerifierScoringConfig{MinAcceptableScore: 6.0},
		Health:   config.VerifierHealthConfig{CircuitBreaker: config.CircuitBreakerConfig{Enabled: true, HalfOpenTimeout: 30 * time.Second}},
		Events:   config.VerifierEventsConfig{Enabled: false},
	}

	result, err := Bootstrap(cfg)
	require.NoError(t, err)
	defer result.Shutdown()

	ctx := context.Background()

	// First call hits the server
	_, err = result.Adapter.GetVerifiedModels(ctx)
	require.NoError(t, err)
	requestCountAfterFirst := mockServer.RequestCount()
	assert.GreaterOrEqual(t, requestCountAfterFirst, 1)

	// Second call should hit cache
	_, err = result.Adapter.GetVerifiedModels(ctx)
	require.NoError(t, err)
	requestCountAfterSecond := mockServer.RequestCount()
	assert.Equal(t, requestCountAfterFirst, requestCountAfterSecond,
		"second request should not hit the server (cache hit)")
}

// TestIntegration_FallbackOnServerDown verifies fallback when verifier is unreachable.
func TestIntegration_FallbackOnServerDown(t *testing.T) {
	cfg := &config.VerifierConfig{
		Enabled:  true,
		Mode:     "remote",
		Endpoint: "http://localhost:1", // guaranteed to fail
		Timeout:  1 * time.Second,
		CacheTTL: 1 * time.Minute,
		Scoring:  config.VerifierScoringConfig{MinAcceptableScore: 6.0},
		Health:   config.VerifierHealthConfig{FailureThreshold: 1, RecoveryThreshold: 1, CircuitBreaker: config.CircuitBreakerConfig{Enabled: true, HalfOpenTimeout: 1 * time.Second}},
		Events:   config.VerifierEventsConfig{Enabled: false},
	}

	result, err := Bootstrap(cfg)
	require.NoError(t, err) // bootstrap should not fail even if server is down
	defer result.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	models, err := result.Adapter.GetVerifiedModels(ctx)
	// Fallback is expected when verifier is unreachable
	assert.Equal(t, ErrUsingFallback, err)
	require.NotNil(t, models)
	assert.Equal(t, len(FallbackModels), len(models),
		"when verifier is down, should return exactly the fallback models")
	assert.Equal(t, "fallback", models[0].Source)
}

// ---------------------------------------------------------------------------
// Mock LLMsVerifier Server
// ---------------------------------------------------------------------------

type mockVerifierServer struct {
	Server       *httptest.Server
	requestCount int
}

func newMockVerifierServer() *mockVerifierServer {
	m := &mockVerifierServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", m.handleHealth)
	mux.HandleFunc("/api/models", m.handleModels)
	mux.HandleFunc("/api/models/gpt-4o", m.handleModelDetail)
	mux.HandleFunc("/api/scores", m.handleProviderScores)
	m.Server = httptest.NewServer(mux)
	return m
}

func (m *mockVerifierServer) Close() {
	m.Server.Close()
}

func (m *mockVerifierServer) GetURL() string {
	return m.Server.URL
}

func (m *mockVerifierServer) RequestCount() int {
	return m.requestCount
}

func (m *mockVerifierServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	m.requestCount++
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (m *mockVerifierServer) handleModels(w http.ResponseWriter, r *http.Request) {
	m.requestCount++
	models := []*VerifiedModel{
		{
			ID: "gpt-4o", Name: "GPT-4o", DisplayName: "GPT-4o", Provider: "openai",
			ContextSize: 128000, MaxOutputTokens: 4096, Source: "verifier",
			OverallScore: 9.1, Tier: 1, Verified: true, VerificationStatus: "verified",
			SupportsCode: true, SupportsStreaming: true, SupportsTools: true,
			SupportsVision: true, SupportsReasoning: true,
		},
		{
			ID: "claude-3-5-sonnet", Name: "Claude 3.5 Sonnet", DisplayName: "Claude 3.5 Sonnet",
			Provider: "anthropic", ContextSize: 200000, MaxOutputTokens: 8192,
			Source: "verifier", OverallScore: 8.9, Tier: 1, Verified: true,
			VerificationStatus: "verified", SupportsCode: true, SupportsStreaming: true,
			SupportsTools: true, SupportsVision: true, SupportsReasoning: true,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func (m *mockVerifierServer) handleModelDetail(w http.ResponseWriter, r *http.Request) {
	m.requestCount++
	result := &VerificationResult{
		ModelID: "gpt-4o", OverallScore: 9.1,
		CodeCapabilityScore: 9.5, ReliabilityScore: 9.0,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (m *mockVerifierServer) handleProviderScores(w http.ResponseWriter, r *http.Request) {
	m.requestCount++
	scores := map[string]float64{
		"openai":    9.1,
		"anthropic": 8.9,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}
