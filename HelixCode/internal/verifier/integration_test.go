//go:build integration

package verifier

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_BootstrapAndFetchModels verifies the full verifier
// initialization and model fetch cycle against a REAL in-process HTTP server.
func TestIntegration_BootstrapAndFetchModels(t *testing.T) {
	// Start a REAL in-process verifier server (binds to random TCP port)
	server, err := NewTestServer()
	require.NoError(t, err, "REAL TestServer must start successfully")
	defer server.Shutdown()

	cfg := &config.VerifierConfig{
		Enabled:         true,
		Mode:            "remote",
		Endpoint:        server.URL(),
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

	// Fetch models through the adapter over REAL TCP
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := result.Adapter.GetVerifiedModels(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, models)

	// Verify we got REAL server models
	assert.GreaterOrEqual(t, len(models), 2)
	assert.Equal(t, "gpt-4o", models[0].ID)
	assert.Equal(t, "openai", models[0].Provider)

	// Verify score lookup works with REAL data
	score, found := result.Adapter.GetModelScore("gpt-4o")
	assert.True(t, found)
	assert.Equal(t, 9.1, score)
}

// TestIntegration_CacheHit verifies that repeated calls use the cache.
func TestIntegration_CacheHit(t *testing.T) {
	server, err := NewTestServer()
	require.NoError(t, err, "REAL TestServer must start")
	defer server.Shutdown()

	cfg := &config.VerifierConfig{
		Enabled:  true,
		Mode:     "remote",
		Endpoint: server.URL(),
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

	// First call hits the REAL server
	models1, err := result.Adapter.GetVerifiedModels(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, models1)

	// Second call should hit cache (no new TCP connection needed)
	models2, err := result.Adapter.GetVerifiedModels(ctx)
	require.NoError(t, err)
	require.Equal(t, len(models1), len(models2))

	// Verify cached data is identical
	assert.Equal(t, models1[0].ID, models2[0].ID)
	assert.Equal(t, models1[0].OverallScore, models2[0].OverallScore)
}

// TestIntegration_FallbackOnServerDown verifies fallback when verifier is unreachable.
func TestIntegration_FallbackOnServerDown(t *testing.T) {
	cfg := &config.VerifierConfig{
		Enabled:  true,
		Mode:     "remote",
		Endpoint: "http://localhost:1", // guaranteed to fail (no server on port 1)
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

// TestIntegration_RealServerModelDetail verifies that model detail lookups
// work against the REAL in-process server.
func TestIntegration_RealServerModelDetail(t *testing.T) {
	server, err := NewTestServer()
	require.NoError(t, err, "REAL TestServer must start")
	defer server.Shutdown()

	client := NewClient(server.URL(), "", 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.VerifyModel(ctx, "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "gpt-4o", result.ModelID)
	assert.Equal(t, 9.1, result.OverallScore)
	assert.Equal(t, 9.5, result.CodeCapabilityScore)
}
