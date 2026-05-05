// Package llm — P1-F12-T03 audit tests for AnthropicProvider.
//
// This file is the F12 conformance harness for the AnthropicProvider:
//
//  1. Compile-time + runtime confirmation that *AnthropicProvider satisfies
//     the unified Provider interface (missing_types.go:191).
//  2. ANTHROPIC_BASE_URL precedence — the wizard (T08) and selector (T07)
//     rely on a documented precedence order so users can override the API
//     endpoint via env var without rewriting their config:
//     explicit Config.Endpoint > ANTHROPIC_BASE_URL env > default
//     https://api.anthropic.com/v1/messages.
//  3. GetModels() routes through the verifier-priority branch in
//     EnrichModelInfo (verifier_bridge.go:36-63) when an enabled adapter is
//     installed via SetVerifierAdapter — i.e. the LLMsVerifier-as-single-
//     source-of-truth wiring (CONST-036/037) is real, not a stub.
//
// These tests run with NO credentials and NO network. They MUST NOT call
// Skip() — failure here is a real conformance regression that blocks
// T07–T11 (factory, wizard, main wiring, challenge, close-out).
package llm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/verifier"
)

// Compile-time conformance: *AnthropicProvider satisfies the unified
// Provider interface defined in missing_types.go. The file under test
// (anthropic_provider.go) is the canonical implementation; this assertion
// fails at build time if any method is dropped or has its signature broken.
var _ Provider = (*AnthropicProvider)(nil)

// anthropicEnvBaseURL is the env var the spec calls out for overriding the
// Anthropic API endpoint without touching config files. Mirroring the env
// var used by the official Anthropic SDKs and OpenCode/Codename Goose.
const anthropicEnvBaseURL = "ANTHROPIC_BASE_URL"

// withAnthropicBaseURLEnv sets ANTHROPIC_BASE_URL for the duration of the
// test and restores the prior value (or unset state) on cleanup.
func withAnthropicBaseURLEnv(t *testing.T, value string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(anthropicEnvBaseURL)
	require.NoError(t, os.Setenv(anthropicEnvBaseURL, value))
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(anthropicEnvBaseURL, prev)
		} else {
			_ = os.Unsetenv(anthropicEnvBaseURL)
		}
	})
}

// withoutAnthropicBaseURLEnv ensures ANTHROPIC_BASE_URL is unset for the
// duration of the test and restores the prior value on cleanup.
func withoutAnthropicBaseURLEnv(t *testing.T) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(anthropicEnvBaseURL)
	require.NoError(t, os.Unsetenv(anthropicEnvBaseURL))
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(anthropicEnvBaseURL, prev)
		}
	})
}

// TestAnthropicProvider_BaseURLPrecedence_ExplicitWinsOverEnv asserts that an
// explicit Endpoint passed to NewAnthropicProvider takes precedence over the
// ANTHROPIC_BASE_URL env var. This is the highest-priority path: a user who
// has wired a specific endpoint into their config must always reach it, even
// if they (or a sibling tool) export ANTHROPIC_BASE_URL globally.
func TestAnthropicProvider_BaseURLPrecedence_ExplicitWinsOverEnv(t *testing.T) {
	withAnthropicBaseURLEnv(t, "https://env.example.com/v1/messages")

	const explicit = "https://explicit.example.com/v1/messages"
	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:     ProviderTypeAnthropic,
		APIKey:   "test-key-not-used-for-network",
		Endpoint: explicit,
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, explicit, p.endpoint,
		"explicit Config.Endpoint MUST take precedence over ANTHROPIC_BASE_URL")
}

// TestAnthropicProvider_BaseURLPrecedence_EnvWinsOverDefault asserts that the
// ANTHROPIC_BASE_URL env var is honoured when no explicit Endpoint is set in
// the ProviderConfigEntry. Wizards and ad-hoc CLI invocations rely on this
// path to swap the endpoint (e.g. Anthropic-compatible proxies, on-prem
// gateways) without persisting a config change.
func TestAnthropicProvider_BaseURLPrecedence_EnvWinsOverDefault(t *testing.T) {
	const fromEnv = "https://env.example.com/v1/messages"
	withAnthropicBaseURLEnv(t, fromEnv)

	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:   ProviderTypeAnthropic,
		APIKey: "test-key-not-used-for-network",
		// Endpoint intentionally unset.
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, fromEnv, p.endpoint,
		"ANTHROPIC_BASE_URL MUST be used when Config.Endpoint is empty")
}

// TestAnthropicProvider_BaseURLPrecedence_DefaultWhenUnset asserts the
// fallback: when neither Config.Endpoint nor ANTHROPIC_BASE_URL is set,
// the provider points at the canonical Anthropic API endpoint. Regression
// guard: silently changing the default would route every default-configured
// install at the wrong endpoint.
func TestAnthropicProvider_BaseURLPrecedence_DefaultWhenUnset(t *testing.T) {
	withoutAnthropicBaseURLEnv(t)

	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:   ProviderTypeAnthropic,
		APIKey: "test-key-not-used-for-network",
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "https://api.anthropic.com/v1/messages", p.endpoint,
		"with no Config.Endpoint and no ANTHROPIC_BASE_URL, provider MUST "+
			"use the canonical Anthropic API URL")
}

// TestAnthropicProvider_BaseURLPrecedence_EmptyEnvFallsBackToDefault is a
// belt-and-braces guard against treating an empty ANTHROPIC_BASE_URL as a
// valid endpoint. An empty env var (e.g. `export ANTHROPIC_BASE_URL=`) must
// be treated as "unset" and yield the default — sending requests to "" would
// fail in non-obvious ways at the HTTP layer.
func TestAnthropicProvider_BaseURLPrecedence_EmptyEnvFallsBackToDefault(t *testing.T) {
	withAnthropicBaseURLEnv(t, "")

	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:   ProviderTypeAnthropic,
		APIKey: "test-key-not-used-for-network",
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "https://api.anthropic.com/v1/messages", p.endpoint,
		"empty ANTHROPIC_BASE_URL MUST be treated as unset and fall back to "+
			"the canonical Anthropic API URL")
}

// TestAnthropicProvider_GetModelsRoutesViaVerifier asserts that
// AnthropicProvider's model-list builder path (anthropic_provider.go:
// getAnthropicModels -> EnrichModelInfo) routes through the verifier-priority
// branch in verifier_bridge.go when an enabled adapter is installed.
//
// Detection mechanism (same as provider_audit_test.go:150 — the F12 audit
// idiom): the heuristic fallback (inferCapabilitiesFromModelID) always
// appends CapabilityTextGeneration; the verifier branch returns BEFORE that.
// So if any model in the Anthropic provider's list emerges WITHOUT
// CapabilityTextGeneration after construction with an enabled adapter
// installed, the verifier branch ran for that model — proving GetModels()
// is wired through the verifier.
//
// This is a real-routing assertion (verifier_bridge.go IS the integration
// point AnthropicProvider uses), not a mock-call assertion.
func TestAnthropicProvider_GetModelsRoutesViaVerifier(t *testing.T) {
	// Snapshot + restore the package-global so this test doesn't bleed
	// into sibling tests in the same package.
	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })

	enabled := verifier.NewAdapter(nil, nil, nil, &verifier.AdapterConfig{Enabled: true})
	require.True(t, enabled.IsEnabled(),
		"test fixture: enabled adapter must report IsEnabled()==true")
	SetVerifierAdapter(enabled)

	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:   ProviderTypeAnthropic,
		APIKey: "test-key-not-used-for-network",
	})
	require.NoError(t, err)

	models := p.GetModels()
	require.NotEmpty(t, models,
		"AnthropicProvider.GetModels() must return non-empty list at "+
			"construction (verifier-enriched)")

	for _, m := range models {
		// Anthropic model IDs (claude-3-*, claude-3-5-*, claude-3-7-*,
		// claude-4-*) all match the "claude-3" / "claude-4" heuristic in
		// inferCapabilitiesFromModelID, which would set SupportsVision=true
		// and append CapabilityTextGeneration. The verifier branch returns
		// before either of those side-effects fires. So the absence of the
		// heuristic-only marker on EVERY model proves the verifier path
		// was the one that ran.
		assert.NotContains(t, m.Capabilities, CapabilityTextGeneration,
			"model %q (%s): EnrichModelInfo must take the verifier-priority "+
				"branch (returns before heuristic appends CapabilityTextGeneration); "+
				"finding it here means the heuristic ran instead — GetModels() "+
				"is NOT wired through the verifier",
			m.ID, m.Name)
	}
}

// TestAnthropicProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic
// is the negative companion to the test above: with no adapter installed,
// EnrichModelInfo MUST take the heuristic branch — otherwise GetModels()
// returns models with no capabilities populated at all, breaking selectors
// and routers that depend on the inferred metadata as a default.
func TestAnthropicProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic(t *testing.T) {
	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })
	SetVerifierAdapter(nil)

	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:   ProviderTypeAnthropic,
		APIKey: "test-key-not-used-for-network",
	})
	require.NoError(t, err)

	models := p.GetModels()
	require.NotEmpty(t, models)

	for _, m := range models {
		assert.Contains(t, m.Capabilities, CapabilityTextGeneration,
			"model %q (%s): with no verifier adapter, EnrichModelInfo MUST "+
				"fall through to inferCapabilitiesFromModelID, which adds "+
				"CapabilityTextGeneration",
			m.ID, m.Name)
	}
}
