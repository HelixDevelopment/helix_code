package main

import (
	"os"
	"testing"

	"dev.helix.code/internal/llm"
)

// clearAllProviderKeys unsets every credential env var the auto-registration
// path recognises, so a test starts from a known no-key baseline regardless of
// the developer's shell environment. t.Setenv restores the prior values on
// cleanup.
func clearAllProviderKeys(t *testing.T) {
	t.Helper()
	for _, aliases := range llm.ProviderEnvAliases() {
		for _, alias := range aliases {
			if _, ok := os.LookupEnv(alias); ok {
				t.Setenv(alias, "")
			}
		}
	}
	// Also clear the hosted OpenAI-compatible catalogue's key aliases — the
	// operator's shell exports the full ~/api_keys.sh set, so without this the
	// catalogue providers would register from the inherited env and the
	// no-key-path assertions would not hold. Stays auto-synced with the catalogue.
	for _, h := range llm.HostedOpenAICompatibleCatalogue() {
		for _, alias := range h.KeyEnvAliases {
			if _, ok := os.LookupEnv(alias); ok {
				t.Setenv(alias, "")
			}
		}
	}
}

// TestRegisterEnvProviders_RegistersWhenKeyPresent proves that, when a provider
// credential env var is set, registerEnvProviders registers that provider and
// the manager's GetAvailableModels() returns a non-empty list.
//
// §1.1 paired-mutation framing: the GREEN assertion is count > 0 / models > 0.
// If registerEnvProviders were mutated to skip the RegisterProvider call (the
// "empty ModelManager" bug this fixes), the count would stay 0 and BOTH
// assertions below would FAIL — the test cannot pass on the broken behaviour.
func TestRegisterEnvProviders_RegistersWhenKeyPresent(t *testing.T) {
	clearAllProviderKeys(t)
	// A syntactically-valid, non-placeholder fake key. The provider constructs
	// from it (no network at construction — the seed model list is offline);
	// we assert on GetAvailableModels(), which returns the seed catalogue
	// without making a live call.
	t.Setenv("DEEPSEEK_API_KEY", "sk-test-deepseek-real-looking-credential-0123456789")

	manager := llm.NewModelManager()
	got := registerEnvProviders(manager)

	if got < 1 {
		t.Fatalf("registerEnvProviders registered %d providers, want >= 1 when DEEPSEEK_API_KEY is set", got)
	}

	models := manager.GetAvailableModels()
	if len(models) == 0 {
		t.Fatalf("GetAvailableModels() returned 0 models after registering DeepSeek; want > 0 (the empty-ModelManager bug)")
	}

	// Anti-bluff: the registered models must actually belong to the provider we
	// registered — not a phantom/hardcoded entry from an unrelated source.
	foundDeepSeek := false
	for _, m := range models {
		if m.Provider == llm.ProviderTypeDeepSeek {
			foundDeepSeek = true
			break
		}
	}
	if !foundDeepSeek {
		t.Fatalf("GetAvailableModels() returned %d models but none from DeepSeek; provider was not really wired", len(models))
	}
}

// TestRegisterEnvProviders_RegistersNoneWhenUnset proves the honest no-key path:
// with every recognised credential env var cleared, registerEnvProviders
// registers ZERO providers and the manager reports ZERO available models — the
// picker honestly shows "no models available" rather than a fabricated list.
func TestRegisterEnvProviders_RegistersNoneWhenUnset(t *testing.T) {
	clearAllProviderKeys(t)

	manager := llm.NewModelManager()
	got := registerEnvProviders(manager)

	if got != 0 {
		t.Fatalf("registerEnvProviders registered %d providers with all keys cleared, want 0", got)
	}
	if n := len(manager.GetAvailableModels()); n != 0 {
		t.Fatalf("GetAvailableModels() returned %d models with all keys cleared, want 0", n)
	}
}

// TestRegisterEnvProviders_NilManagerSafe guards the defensive nil path.
func TestRegisterEnvProviders_NilManagerSafe(t *testing.T) {
	if got := registerEnvProviders(nil); got != 0 {
		t.Fatalf("registerEnvProviders(nil) = %d, want 0", got)
	}
}
