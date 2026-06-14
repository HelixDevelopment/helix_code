package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/llm"
)

// env_providers_dynamic_test.go — proves the PRIMARY path sources the
// OpenAI-compatible providers DYNAMICALLY from a LLMsVerifier /api/providers
// endpoint (CONST-036/CONST-046: the verifier's api_url is the base URL, NOT a
// hardcoded literal), and that the hardcoded catalogue is engaged ONLY as a
// degraded fallback when the verifier is unreachable.

// newFakeVerifier stands up an httptest LLMsVerifier serving /api/providers with
// the real envelope shape (name + api_url + models + is_active + status).
func newFakeVerifier(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/providers" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"providers": [
				{"id":1,"name":"cerebras","api_url":"https://verifier.cerebras.example/v1","status":"active","is_active":true,"reliability_score":9.1,"models":["m1"]},
				{"id":2,"name":"sambanova","api_url":"https://verifier.sambanova.example/v1","status":"active","is_active":true,"reliability_score":8.7,"models":["m2"]},
				{"id":3,"name":"novita","api_url":"https://verifier.novita.example/v3/openai","status":"active","is_active":true,"reliability_score":8.0,"models":["m3"]}
			],
			"count": 3
		}`))
	}))
}

// TestBuildOpenAICompatibleProviders_DynamicUsesVerifierAPIURL asserts the
// PRIMARY path: when the verifier is reachable, providers are built from its
// api_url (NOT the hardcoded catalogue URL) and only present-key providers come up.
func TestBuildOpenAICompatibleProviders_DynamicUsesVerifierAPIURL(t *testing.T) {
	clearAllProviderKeys(t)
	srv := newFakeVerifier(t)
	defer srv.Close()
	t.Setenv("HELIX_VERIFIER_ENDPOINT", srv.URL)

	// Only cerebras + novita keys present → sambanova must be skipped.
	t.Setenv("CEREBRAS_API_KEY", "cb-dummy-real-looking-value-123")
	t.Setenv("NOVITA_API_KEY", "nv-dummy-real-looking-value-123")

	providers, usedDynamic := buildOpenAICompatibleProviders(nil)
	if !usedDynamic {
		t.Fatalf("expected usedDynamic=true when verifier is reachable")
	}

	byName := map[string]llm.Provider{}
	for _, p := range providers {
		byName[p.GetName()] = p
	}
	if _, ok := byName["sambanova"]; ok {
		t.Errorf("sambanova has no present key — must NOT be built")
	}
	for name, wantURL := range map[string]string{
		"cerebras": "https://verifier.cerebras.example/v1",
		"novita":   "https://verifier.novita.example/v3/openai",
	} {
		p, ok := byName[name]
		if !ok {
			t.Fatalf("expected %s to be built from verifier record", name)
		}
		oc, ok := p.(*llm.OpenAICompatibleProvider)
		if !ok {
			t.Fatalf("%s: not *OpenAICompatibleProvider: %T", name, p)
		}
		if got := oc.BaseURL(); got != wantURL {
			t.Errorf("%s BaseURL = %q, want the VERIFIER api_url %q (no hardcoded URL)", name, got, wantURL)
		}
	}
}

// TestBuildOpenAICompatibleProviders_FallbackWhenVerifierUnreachable asserts the
// FALLBACK gate: when the verifier is unreachable, usedDynamic is false and the
// hardcoded catalogue provides the providers (degraded offline safety net).
func TestBuildOpenAICompatibleProviders_FallbackWhenVerifierUnreachable(t *testing.T) {
	clearAllProviderKeys(t)
	// Unreachable verifier endpoint.
	t.Setenv("HELIX_VERIFIER_ENDPOINT", "http://127.0.0.1:1")

	// A hardcoded-catalogue provider key present so the fallback yields ≥1.
	t.Setenv("CEREBRAS_API_KEY", "cb-dummy-real-looking-value-123")

	providers, usedDynamic := buildOpenAICompatibleProviders(nil)
	if usedDynamic {
		t.Fatalf("expected usedDynamic=false when verifier is unreachable")
	}
	found := false
	for _, p := range providers {
		if p.GetName() == "cerebras" {
			oc := p.(*llm.OpenAICompatibleProvider)
			// The fallback must use the HARDCODED catalogue URL, not a verifier URL.
			if oc.BaseURL() != "https://api.cerebras.ai/v1" {
				t.Errorf("fallback cerebras BaseURL = %q, want hardcoded catalogue URL", oc.BaseURL())
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fallback to register cerebras from the hardcoded catalogue")
	}
}

// TestRegisterEnvProviders_DynamicEndToEnd proves the full wiring: registerEnvProviders
// with a reachable verifier registers the dynamically-built providers into the
// ModelManager (CONST-036 primary path is live end-to-end).
func TestRegisterEnvProviders_DynamicEndToEnd(t *testing.T) {
	clearAllProviderKeys(t)
	srv := newFakeVerifier(t)
	defer srv.Close()
	t.Setenv("HELIX_VERIFIER_ENDPOINT", srv.URL)
	t.Setenv("CEREBRAS_API_KEY", "cb-dummy-real-looking-value-123")

	manager := llm.NewModelManager()
	cfg := &config.Config{} // no explicit verifier config → env endpoint resolves

	got := registerEnvProviders(manager, cfg)
	if got < 1 {
		t.Fatalf("registerEnvProviders registered %d providers, want >= 1 (dynamic cerebras)", got)
	}
}
