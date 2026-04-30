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

func TestAdapter_IsEnabled(t *testing.T) {
	a := NewAdapter(nil, nil, nil, &AdapterConfig{Enabled: true})
	assert.True(t, a.IsEnabled())

	a2 := NewAdapter(nil, nil, nil, &AdapterConfig{Enabled: false})
	assert.False(t, a2.IsEnabled())

	a3 := NewAdapter(nil, nil, nil, nil)
	assert.False(t, a3.IsEnabled())
}

func TestAdapter_GetVerifiedModels_Disabled(t *testing.T) {
	a := NewAdapter(nil, nil, nil, &AdapterConfig{Enabled: false})
	_, err := a.GetVerifiedModels(context.Background())
	require.ErrorIs(t, err, ErrVerifierDisabled)
}

func TestAdapter_GetVerifiedModels_FromVerifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		models := []*VerifiedModel{
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", OverallScore: 9.1},
			{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", OverallScore: 8.9},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", 0)
	health := NewHealthMonitor(5, 3, 60*time.Second)
	cache := NewCache(5*time.Minute, nil)
	cfg := &AdapterConfig{Enabled: true}
	adapter := NewAdapter(client, cache, health, cfg)

	models, err := adapter.GetVerifiedModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, "gpt-4o", models[0].ID)

	// Score maps should be populated
	score, ok := adapter.GetModelScore("gpt-4o")
	assert.True(t, ok)
	assert.Equal(t, 9.1, score)
}

func TestAdapter_GetVerifiedModels_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		models := []*VerifiedModel{{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", OverallScore: 9.1}}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", 0)
	health := NewHealthMonitor(5, 3, 60*time.Second)
	cache := NewCache(5*time.Minute, nil)
	cfg := &AdapterConfig{Enabled: true}
	adapter := NewAdapter(client, cache, health, cfg)

	// First call hits server
	_, err := adapter.GetVerifiedModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call should hit cache
	_, err = adapter.GetVerifiedModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "second call should be served from cache")
}

func TestAdapter_GetVerifiedModels_FallbackOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", 0)
	health := NewHealthMonitor(5, 3, 60*time.Second)
	cache := NewCache(5*time.Minute, nil)
	cfg := &AdapterConfig{Enabled: true}
	adapter := NewAdapter(client, cache, health, cfg)

	models, err := adapter.GetVerifiedModels(context.Background())
	require.ErrorIs(t, err, ErrUsingFallback)
	require.Len(t, models, 7, "should return fallback models")
	assert.Equal(t, "fallback", models[0].Source)
}

func TestAdapter_filterByProviderConfig(t *testing.T) {
	models := []*VerifiedModel{
		{ID: "a", Provider: "openai"},
		{ID: "b", Provider: "anthropic"},
		{ID: "c", Provider: "xai"},
	}
	cfg := &AdapterConfig{
		Enabled:   true,
		Providers: map[string]ProviderAdapterConfig{"xai": {Enabled: false}},
	}
	a := NewAdapter(nil, nil, nil, cfg)
	filtered := a.filterByProviderConfig(models)
	require.Len(t, filtered, 2)
	assert.Equal(t, "openai", filtered[0].Provider)
	assert.Equal(t, "anthropic", filtered[1].Provider)
}

func TestAdapter_GetMinAcceptableScore_Default(t *testing.T) {
	a := NewAdapter(nil, nil, nil, nil)
	assert.Equal(t, 6.0, a.GetMinAcceptableScore())
}

func TestAdapter_GetMinAcceptableScore_Configured(t *testing.T) {
	a := NewAdapter(nil, nil, nil, &AdapterConfig{
		Enabled: true,
		Scoring: ScoringAdapterConfig{MinAcceptableScore: 7.5},
	})
	assert.Equal(t, 7.5, a.GetMinAcceptableScore())
}

func TestAdapter_GetProviderStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		models := []*VerifiedModel{
			{ID: "gpt-4o", Provider: "openai", OverallScore: 9.1},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", 0)
	health := NewHealthMonitor(5, 3, 60*time.Second)
	cache := NewCache(5*time.Minute, nil)
	cfg := &AdapterConfig{Enabled: true}
	adapter := NewAdapter(client, cache, health, cfg)

	_, err := adapter.GetVerifiedModels(context.Background())
	require.NoError(t, err)

	status, ok := adapter.GetProviderStatus("openai")
	require.True(t, ok)
	assert.Equal(t, 9.1, status.Score)
	assert.True(t, status.Healthy)
}

func TestAdapter_ForceRefresh(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		models := []*VerifiedModel{{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", OverallScore: 9.1}}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", 0)
	health := NewHealthMonitor(5, 3, 60*time.Second)
	cache := NewCache(5*time.Minute, nil)
	cfg := &AdapterConfig{Enabled: true}
	adapter := NewAdapter(client, cache, health, cfg)

	_, _ = adapter.GetVerifiedModels(context.Background())
	assert.Equal(t, 1, callCount)

	err := adapter.ForceRefresh(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "force refresh should bypass cache")
}
