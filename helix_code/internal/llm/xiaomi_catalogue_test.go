package llm

import (
	"testing"
)

func TestXiaomiInHostedCatalogue(t *testing.T) {
	catalogue := HostedOpenAICompatibleCatalogue()
	found := false
	for _, h := range catalogue {
		if h.Name == "xiaomi" {
			found = true
			if h.BaseURL != "https://api.xiaomimimo.com/v1" {
				t.Errorf("BaseURL = %q, want %q", h.BaseURL, "https://api.xiaomimimo.com/v1")
			}
			expectedAliases := []string{"XIAOMI_MIMO_API_KEY", "ApiKey_Xiaomi_MiMo"}
			if len(h.KeyEnvAliases) != len(expectedAliases) {
				t.Fatalf("expected %d aliases, got %d", len(expectedAliases), len(h.KeyEnvAliases))
			}
			for i, a := range expectedAliases {
				if h.KeyEnvAliases[i] != a {
					t.Errorf("alias[%d] = %q, want %q", i, h.KeyEnvAliases[i], a)
				}
			}
			if h.ModelEndpoint != "/models" {
				t.Errorf("ModelEndpoint = %q, want %q", h.ModelEndpoint, "/models")
			}
			if h.ChatEndpoint != "/chat/completions" {
				t.Errorf("ChatEndpoint = %q, want %q", h.ChatEndpoint, "/chat/completions")
			}
			expectedModelsURL := "https://api.xiaomimimo.com/v1/models"
			if h.ComposedModelsURL() != expectedModelsURL {
				t.Errorf("ComposedModelsURL() = %q, want %q", h.ComposedModelsURL(), expectedModelsURL)
			}
			break
		}
	}
	if !found {
		t.Fatal("xiaomi not found in HostedOpenAICompatibleCatalogue")
	}
}
