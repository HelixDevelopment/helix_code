package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the RED polarity switch is active (§11.4.115).
//
//	RED_MODE=1 (default): reproduce the DEFECT on the pre-fix code path and
//	            assert it is PRESENT (the unfiltered GetVerifiedModels leaks
//	            failed / sub-threshold / non-key-present models).
//	RED_MODE=0: the standing GREEN regression guard asserts the defect is
//	            ABSENT (GetWorkingModels filters them out).
func redMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "" || v == "1"
}

// newWorkingModelsServer serves a fixed catalog containing one healthy model,
// one verified-but-sub-threshold model, one failed model, and a verified model
// belonging to a provider the caller has NO key for.
func newWorkingModelsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		models := []*VerifiedModel{
			// (1) working: anthropic, verified, status verified, score 8.0 >= 6.0
			{ID: "claude-good", Provider: "anthropic", Verified: true,
				VerificationStatus: "verified", OverallScore: 8.0},
			// (2) sub-threshold: anthropic, verified but score 4.0 < 6.0
			{ID: "claude-lowscore", Provider: "anthropic", Verified: true,
				VerificationStatus: "verified", OverallScore: 4.0},
			// (3) failed: anthropic, not verified, status failed
			{ID: "claude-failed", Provider: "anthropic", Verified: false,
				VerificationStatus: "failed", OverallScore: 9.0},
			// (4) pending: anthropic, not verified
			{ID: "claude-pending", Provider: "anthropic", Verified: false,
				VerificationStatus: "pending", OverallScore: 9.0},
			// (5) no-key provider: openai, fully verified high score
			{ID: "gpt-good", Provider: "openai", Verified: true,
				VerificationStatus: "verified", OverallScore: 9.5},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models)
	}))
}

func newWorkingModelsAdapter(t *testing.T, url string) *Adapter {
	t.Helper()
	client := NewClient(url, "", 0)
	health := NewHealthMonitor(5, 3, 60*time.Second)
	cache := NewCache(5*time.Minute, nil)
	cfg := &AdapterConfig{Enabled: true} // default MinAcceptableScore -> 6.0
	return NewAdapter(client, cache, health, cfg)
}

// TestGetWorkingModels_VerifiedAndScore is the D-4 RED/GREEN guard: only
// Verified ∧ status=="verified" ∧ OverallScore>=GetMinAcceptableScore() models
// survive (D-4), AND only key-present providers (key-presence gate).
func TestGetWorkingModels_VerifiedAndScore(t *testing.T) {
	server := newWorkingModelsServer(t)
	defer server.Close()
	adapter := newWorkingModelsAdapter(t, server.URL)

	present := map[string]bool{"anthropic": true} // NO openai key

	if redMode() {
		// RED: prove the defect exists on the unfiltered path. The raw
		// GetVerifiedModels leaks failed / low-score / no-key models.
		raw, err := adapter.GetVerifiedModels(context.Background())
		require.NoError(t, err)
		ids := idSet(raw)
		assert.True(t, ids["claude-failed"],
			"RED: GetVerifiedModels leaks the failed model (the D-2/D-4 defect)")
		assert.True(t, ids["claude-lowscore"],
			"RED: GetVerifiedModels leaks the sub-threshold model (D-4 defect)")
		assert.True(t, ids["gpt-good"],
			"RED: GetVerifiedModels leaks a provider with no key (key-gate defect)")
		return
	}

	// GREEN: GetWorkingModels filters to the single working, key-present model.
	working, err := adapter.GetWorkingModels(context.Background(), present)
	require.NoError(t, err)
	ids := idSet(working)
	assert.True(t, ids["claude-good"], "the only working+key-present model must survive")
	assert.False(t, ids["claude-lowscore"], "sub-threshold model must be dropped (D-4)")
	assert.False(t, ids["claude-failed"], "failed model must be dropped (D-2/D-4)")
	assert.False(t, ids["claude-pending"], "pending model must be dropped (D-2/D-4)")
	assert.False(t, ids["gpt-good"], "no-key provider must be dropped (key-gate)")
	assert.Len(t, working, 1)
}

// TestGetWorkingModels_KeyPresenceGate isolates the key-presence predicate.
func TestGetWorkingModels_KeyPresenceGate(t *testing.T) {
	server := newWorkingModelsServer(t)
	defer server.Close()
	adapter := newWorkingModelsAdapter(t, server.URL)

	if redMode() {
		t.Skip("RED polarity covered by TestGetWorkingModels_VerifiedAndScore") // SKIP-OK: paired RED guard
	}

	// With BOTH keys present, the working high-score models from both providers
	// survive; the failed/low-score still drop.
	working, err := adapter.GetWorkingModels(context.Background(),
		map[string]bool{"anthropic": true, "openai": true})
	require.NoError(t, err)
	ids := idSet(working)
	assert.True(t, ids["claude-good"])
	assert.True(t, ids["gpt-good"], "openai now key-present, its working model survives")
	assert.False(t, ids["claude-failed"])
	assert.False(t, ids["claude-lowscore"])
}

func idSet(models []*VerifiedModel) map[string]bool {
	s := make(map[string]bool, len(models))
	for _, m := range models {
		s[m.ID] = true
	}
	return s
}
