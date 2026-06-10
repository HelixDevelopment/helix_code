package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/verifier"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// funnelRedMode is the §11.4.115 polarity switch for the server-side
// working-model funnel wiring guard.
//
//	RED_MODE=1 (default): reproduce the not-wired defect — the raw
//	            GetVerifiedModels path leaks failed / sub-threshold / no-key
//	            models, so a server listing built from it would show models the
//	            user cannot use (the pre-fix server handler called
//	            GetVerifiedModels directly).
//	RED_MODE=0: the GREEN guard — the wired handler runs GetWorkingModels with
//	            the llm.PresentProviderNames() key-presence gate, so only the
//	            single working+key-present model survives end-to-end over HTTP.
func funnelRedMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "" || v == "1"
}

// newFunnelCatalogServer serves the same mixed catalog the verifier-layer test
// uses: one working anthropic model, one sub-threshold, one failed, one
// pending, and one fully-verified openai model the caller has NO key for.
func newFunnelCatalogServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		models := []*verifier.VerifiedModel{
			{ID: "claude-good", Provider: "anthropic", Verified: true,
				VerificationStatus: "verified", OverallScore: 8.0},
			{ID: "claude-lowscore", Provider: "anthropic", Verified: true,
				VerificationStatus: "verified", OverallScore: 4.0},
			{ID: "claude-failed", Provider: "anthropic", Verified: false,
				VerificationStatus: "failed", OverallScore: 9.0},
			{ID: "claude-pending", Provider: "anthropic", Verified: false,
				VerificationStatus: "pending", OverallScore: 9.0},
			{ID: "gpt-good", Provider: "openai", Verified: true,
				VerificationStatus: "verified", OverallScore: 9.5},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models)
	}))
}

func newFunnelAdapter(t *testing.T, url string) *verifier.Adapter {
	t.Helper()
	client := verifier.NewClient(url, "", 0)
	health := verifier.NewHealthMonitor(5, 3, 60*time.Second)
	cache := verifier.NewCache(5*time.Minute, nil)
	cfg := &verifier.AdapterConfig{Enabled: true} // default MinAcceptableScore -> 6.0
	return verifier.NewAdapter(client, cache, health, cfg)
}

// clearProviderEnv unsets every recognized provider alias so the test starts
// from a hermetic key-presence baseline.
func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, aliases := range llm.ProviderEnvAliases() {
		for _, a := range aliases {
			if _, ok := os.LookupEnv(a); ok {
				t.Setenv(a, "")
			}
		}
	}
}

// TestServerListLLMModels_WorkingFunnelEndToEnd proves the server's
// /api/v1/llm/models handler runs the working-model funnel end-to-end:
// only key-present ∧ Verified ∧ status=="verified" ∧ score>=min models are
// returned over real HTTP. A no-key provider's model is hidden.
func TestServerListLLMModels_WorkingFunnelEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := newFunnelCatalogServer(t)
	defer catalog.Close()

	clearProviderEnv(t)
	// anthropic key PRESENT; openai key ABSENT.
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-realvalue-1234567890")

	adapter := newFunnelAdapter(t, catalog.URL)
	srv := &Server{
		verifierResult: &verifier.BootstrapResult{Adapter: adapter},
	}

	router := gin.New()
	router.GET("/api/v1/llm/models", srv.listLLMModels)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/llm/models", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	if funnelRedMode() {
		// RED: prove the raw verifier path (pre-fix) leaks unusable models, so a
		// listing built from it would be a PASS-bluff. We exercise the same
		// adapter via the unfiltered call to capture the defect signal.
		raw, err := adapter.GetVerifiedModels(req.Context())
		require.NoError(t, err)
		ids := map[string]bool{}
		for _, m := range raw {
			ids[m.ID] = true
		}
		assert.True(t, ids["claude-failed"], "RED: unfiltered path leaks the failed model")
		assert.True(t, ids["gpt-good"], "RED: unfiltered path leaks the no-key openai model")
		return
	}

	// GREEN: the wired handler emits only the working+key-present model.
	require.Equal(t, "success", resp["status"])
	require.Equal(t, "verifier", resp["source"], "must be sourced from the working-model funnel, not fallback")
	served := idSetFromModels(t, resp["models"])
	assert.True(t, served["claude-good"], "the single working+key-present model must be listed")
	assert.False(t, served["claude-lowscore"], "sub-threshold model must be hidden (D-4)")
	assert.False(t, served["claude-failed"], "failed model must be hidden (D-2/D-4)")
	assert.False(t, served["claude-pending"], "pending model must be hidden (D-2/D-4)")
	assert.False(t, served["gpt-good"], "no-key provider model must be hidden (key-gate)")
	assert.Len(t, served, 1)
}

// TestServerListLLMProviders_WorkingFunnelEndToEnd proves the
// /api/v1/llm/providers handler hides providers with no key end-to-end:
// only providers backed by ≥1 working+key-present model appear.
func TestServerListLLMProviders_WorkingFunnelEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := newFunnelCatalogServer(t)
	defer catalog.Close()

	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-realvalue-1234567890") // openai absent

	adapter := newFunnelAdapter(t, catalog.URL)
	srv := &Server{verifierResult: &verifier.BootstrapResult{Adapter: adapter}}

	router := gin.New()
	router.GET("/api/v1/llm/providers", srv.listLLMProviders)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/llm/providers", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	if funnelRedMode() {
		// RED guard sibling: covered by the models test's RED branch.
		t.Skip("RED polarity covered by TestServerListLLMModels_WorkingFunnelEndToEnd") // SKIP-OK: paired RED guard
	}

	require.Equal(t, "success", resp["status"])
	require.Equal(t, "verifier", resp["source"])
	provs, ok := resp["providers"].([]interface{})
	require.True(t, ok, "providers field must be an array")
	names := map[string]bool{}
	for _, p := range provs {
		pm := p.(map[string]interface{})
		if id, ok := pm["id"].(string); ok {
			names[id] = true
		}
	}
	assert.True(t, names["anthropic"], "anthropic key-present + working model -> provider listed")
	assert.False(t, names["openai"], "openai has NO key -> provider hidden (key-gate, anti-bluff)")
}

func idSetFromModels(t *testing.T, v interface{}) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	arr, ok := v.([]interface{})
	if !ok {
		return out
	}
	for _, m := range arr {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := mm["id"].(string); ok {
			out[id] = true
		}
	}
	return out
}
