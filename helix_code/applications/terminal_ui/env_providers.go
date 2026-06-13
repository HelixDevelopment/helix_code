package main

import (
	"log"

	"dev.helix.code/internal/llm"
)

// env_providers.go — wires cloud LLM providers discovered from environment API
// keys into the TUI's ModelManager so the chat model selector lists REAL models.
//
// Anti-bluff (CONST-035 / CONST-036 / Article XI §11.9, BLUFF-002): this file
// registers a provider ONLY when its credential env var is actually present and
// non-placeholder. It never hardcodes a model list — each registered provider's
// GetModels() queries the provider's own (live-refreshed) catalogue. A user with
// no keys configured registers zero providers and the picker honestly shows
// "no models available" rather than a fake list.
//
// Decoupling (CONST-051(B)): the provider→env-var recognition table lives in
// internal/llm (ProviderEnvAliases / IsProviderKeyPresent); this file only
// consumes it. No project-specific context is injected into the llm package.

// envProviderCandidate is one cloud provider that can be constructed from an API
// key alone (no extra endpoint/region/project wiring). These map 1:1 onto
// llm.NewProvider switch arms whose New<X>Provider reads config.APIKey (falling
// back to the same env var) and needs nothing else to come up.
type envProviderCandidate struct {
	providerType llm.ProviderType
}

// envProviderCandidates is the ordered, curated set of cloud providers the TUI
// auto-registers from environment keys. Order is deterministic so the model
// picker's digit shortcuts are stable across runs (the ModelManager itself
// stores models in a Go map, but registration order is preserved here for the
// provider-presence scan and for the captured-evidence probe).
//
// Scope note: only key-only constructible providers are listed. Bedrock,
// VertexAI, and Azure are intentionally excluded — they require region /
// project / deployment wiring beyond a bare API key, so auto-registering them
// from a single env var would produce a half-configured provider (an anti-bluff
// violation). Those remain reachable via explicit config.yaml / the server path.
var envProviderCandidates = []envProviderCandidate{
	{llm.ProviderTypeDeepSeek},
	{llm.ProviderTypeMistral},
	{llm.ProviderTypeGroq},
	{llm.ProviderTypeOpenRouter},
	{llm.ProviderTypeOpenAI},
	{llm.ProviderTypeAnthropic},
	{llm.ProviderTypeXAI},
	{llm.ProviderTypeQwen},
}

// registerEnvProviders registers, into manager, every cloud provider in
// envProviderCandidates whose API key is present in the environment (looked up
// via llm.IsProviderKeyPresent, which honours the multi-alias table and rejects
// placeholder values). It returns the number of providers successfully
// registered.
//
// Each registration goes through llm.NewProvider with an Enabled config entry;
// the New<X>Provider constructor reads its key from the same env var, so an
// empty APIKey in the entry is intentional — it lets the provider self-resolve
// the credential. A construction or registration error for one provider is
// logged and skipped (it never aborts the others), so a single misbehaving key
// cannot take the whole chat surface down.
func registerEnvProviders(manager *llm.ModelManager) int {
	if manager == nil {
		return 0
	}

	registered := 0
	for _, cand := range envProviderCandidates {
		if !llm.IsProviderKeyPresent(cand.providerType) {
			continue
		}

		// Use NewCloudProvider, not NewProvider: the catch-all NewProvider
		// switch in factory.go has no DeepSeek/Mistral arms (it returns
		// "unsupported provider type" for them), whereas NewCloudProvider in
		// provider_factory.go covers the full direct-cloud quartet+ that these
		// candidates draw from. The constructor reads its key from the same env
		// var, so the empty APIKey here is intentional self-resolution.
		provider, err := llm.NewCloudProvider(cand.providerType, llm.ProviderConfigEntry{
			Type:    cand.providerType,
			Enabled: true,
		})
		if err != nil {
			log.Printf("⚠️  TUI: skipping provider %s (construction failed: %v)", cand.providerType, err)
			continue
		}

		if err := manager.RegisterProvider(provider); err != nil {
			log.Printf("⚠️  TUI: skipping provider %s (registration failed: %v)", cand.providerType, err)
			continue
		}

		registered++
	}

	return registered
}
