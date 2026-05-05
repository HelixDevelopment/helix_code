// Package llm — P1-F12-T06 audit tests for AzureProvider.
//
// This file is the F12 conformance harness for the AzureProvider:
//
//  1. Compile-time + runtime confirmation that *AzureProvider satisfies
//     the unified Provider interface (missing_types.go:191).
//  2. Endpoint precedence — explicit Parameters["endpoint"] wins over
//     AZURE_OPENAI_ENDPOINT env, which is the only env-var fallback;
//     when neither is set, NewAzureProvider MUST return a clear error
//     (Azure has no public default endpoint — every tenant has its own
//     resource URL).
//  3. API version precedence — explicit Parameters["api_version"] wins
//     over AZURE_OPENAI_API_VERSION env (with AZURE_API_VERSION accepted
//     as a legacy fallback for backward compatibility), which in turn
//     wins over the documented default (2025-04-01-preview).
//  4. API key precedence — explicit ProviderConfigEntry.APIKey wins over
//     AZURE_OPENAI_API_KEY env. (The Azure SDK / Entra ID branch is a
//     parallel auth mode opted into via use_entra_id=true; these tests
//     pin only the API-key path.)
//  5. GetModels() routes through the verifier-priority branch in
//     EnrichModelInfo (verifier_bridge.go:36-63) when an enabled adapter
//     is installed via SetVerifierAdapter — i.e. the LLMsVerifier-as-
//     single-source-of-truth wiring (CONST-036/037) is real, not a stub.
//
// These tests run with NO real Azure credentials and NO network. A
// synthetic endpoint and api_key are used purely to satisfy the
// constructor's required-field checks; no HTTP call is made.
//
// The tests MUST NOT call Skip() — failure here is a real conformance
// regression that blocks T07–T11 (factory, wizard, main wiring,
// challenge, close-out).
package llm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/verifier"
)

// Compile-time conformance: *AzureProvider satisfies the unified
// Provider interface defined in missing_types.go. The file under test
// (azure_provider.go) is the canonical implementation; this assertion
// fails at build time if any method is dropped or has its signature broken.
var _ Provider = (*AzureProvider)(nil)

// azureEnvEndpoint is the canonical Azure SDK env var for the OpenAI
// resource endpoint. The wizard (T08) and selector (T07) rely on this
// name so users with a standard Azure CLI environment do not have to
// learn a Helix-specific variable.
const azureEnvEndpoint = "AZURE_OPENAI_ENDPOINT"

// azureEnvAPIKey is the canonical Azure SDK env var for the OpenAI
// resource key. Same reasoning as azureEnvEndpoint.
const azureEnvAPIKey = "AZURE_OPENAI_API_KEY"

// azureEnvAPIVersion is the canonical Azure SDK env var for the
// data-plane API version. T06 adds it as the primary env-var fallback;
// the legacy AZURE_API_VERSION (which the older code honoured) is kept
// as a secondary fallback for backward compatibility.
const azureEnvAPIVersion = "AZURE_OPENAI_API_VERSION"

// azureEnvAPIVersionLegacy is the long-standing Helix-specific env var
// for the API version. Kept as a secondary fallback for compatibility
// with already-deployed configs.
const azureEnvAPIVersionLegacy = "AZURE_API_VERSION"

// azureDefaultAPIVersion is the documented fallback baked into
// NewAzureProvider when neither config nor env var supplies an API
// version. Encoded here as a regression guard: silently changing the
// default would route every default-configured install at the wrong
// data-plane version.
const azureDefaultAPIVersion = "2025-04-01-preview"

// azureTestEndpoint is a synthetic Azure resource URL used purely to
// satisfy the constructor's required-field check in tests.
const azureTestEndpoint = "https://test.openai.azure.com"

// withAzureEnv sets one of the Azure env vars for the duration of the
// test and restores the prior value (or unset state) on cleanup.
func withAzureEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	require.NoError(t, os.Setenv(key, value))
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// withoutAzureEnv ensures the named Azure env var is unset for the
// duration of the test, restoring prior state on cleanup.
func withoutAzureEnv(t *testing.T, key string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(key, prev)
		}
	})
}

// withoutAzureAPIVersionEnvs clears BOTH the canonical and legacy API-
// version env vars so default-fallback tests are hermetic.
func withoutAzureAPIVersionEnvs(t *testing.T) {
	t.Helper()
	withoutAzureEnv(t, azureEnvAPIVersion)
	withoutAzureEnv(t, azureEnvAPIVersionLegacy)
}

// TestAzureProvider_EndpointPrecedence_ExplicitWinsOverEnv asserts that
// an explicit Parameters["endpoint"] entry takes precedence over the
// AZURE_OPENAI_ENDPOINT env var. This is the highest-priority path: a
// user who has wired a specific endpoint into their config must always
// reach it, even if AZURE_OPENAI_ENDPOINT is exported globally.
func TestAzureProvider_EndpointPrecedence_ExplicitWinsOverEnv(t *testing.T) {
	withAzureEnv(t, azureEnvEndpoint, "https://env.openai.azure.com")
	withoutAzureAPIVersionEnvs(t)

	const explicit = "https://explicit.openai.azure.com"
	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		Parameters: map[string]interface{}{
			"endpoint": explicit,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, explicit, p.endpoint,
		"explicit Parameters[\"endpoint\"] MUST take precedence over AZURE_OPENAI_ENDPOINT")
}

// TestAzureProvider_EndpointPrecedence_EnvWinsOverError asserts that the
// AZURE_OPENAI_ENDPOINT env var is honoured when no explicit endpoint
// is set in the ProviderConfigEntry. Wizards and ad-hoc CLI invocations
// rely on this path to point at a different Azure resource without
// persisting a config change.
func TestAzureProvider_EndpointPrecedence_EnvWinsOverError(t *testing.T) {
	const fromEnv = "https://env.openai.azure.com"
	withAzureEnv(t, azureEnvEndpoint, fromEnv)
	withoutAzureAPIVersionEnvs(t)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		// Parameters intentionally unset — exercise the env fallback.
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, fromEnv, p.endpoint,
		"AZURE_OPENAI_ENDPOINT MUST be used when no Parameters[\"endpoint\"] is provided")
}

// TestAzureProvider_EndpointPrecedence_MissingErrors asserts that with
// neither an explicit endpoint nor AZURE_OPENAI_ENDPOINT set,
// NewAzureProvider returns a clear error rather than silently
// constructing with an empty/invalid endpoint. Azure has no public
// default — every tenant has its own resource URL — so a missing
// endpoint MUST fail loudly at construction.
func TestAzureProvider_EndpointPrecedence_MissingErrors(t *testing.T) {
	withoutAzureEnv(t, azureEnvEndpoint)
	withoutAzureAPIVersionEnvs(t)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		// No Parameters — exercise the missing-endpoint path.
	})
	assert.Nil(t, p,
		"NewAzureProvider MUST return nil provider when no endpoint is configured")
	require.Error(t, err,
		"NewAzureProvider MUST return an error when no endpoint is configured")
	assert.Contains(t, err.Error(), "endpoint",
		"error MUST mention 'endpoint' so the user knows what to fix; got: %v", err)
}

// TestAzureProvider_APIKeyPrecedence_ExplicitWinsOverEnv asserts that an
// explicit ProviderConfigEntry.APIKey takes precedence over the
// AZURE_OPENAI_API_KEY env var. This is the highest-priority auth path
// for the API-key mode (Entra ID is a separate opt-in branch tested
// elsewhere).
func TestAzureProvider_APIKeyPrecedence_ExplicitWinsOverEnv(t *testing.T) {
	withAzureEnv(t, azureEnvAPIKey, "key-from-env")
	withoutAzureAPIVersionEnvs(t)

	const explicit = "key-from-config"
	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: explicit,
		Parameters: map[string]interface{}{
			"endpoint": azureTestEndpoint,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, explicit, p.apiKey,
		"explicit ProviderConfigEntry.APIKey MUST take precedence over AZURE_OPENAI_API_KEY")
}

// TestAzureProvider_APIKeyPrecedence_EnvWinsOverError asserts that
// AZURE_OPENAI_API_KEY is honoured when no explicit APIKey is set in
// the ProviderConfigEntry. Without it (and without the Entra ID
// opt-in), the constructor must error.
func TestAzureProvider_APIKeyPrecedence_EnvWinsOverError(t *testing.T) {
	const fromEnv = "key-from-env-only"
	withAzureEnv(t, azureEnvAPIKey, fromEnv)
	withoutAzureAPIVersionEnvs(t)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type: ProviderTypeAzure,
		// APIKey intentionally empty.
		Parameters: map[string]interface{}{
			"endpoint": azureTestEndpoint,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, fromEnv, p.apiKey,
		"AZURE_OPENAI_API_KEY MUST be used when ProviderConfigEntry.APIKey is empty")
}

// TestAzureProvider_APIVersionPrecedence_ExplicitWinsOverEnv asserts
// that an explicit Parameters["api_version"] entry takes precedence
// over the AZURE_OPENAI_API_VERSION env var.
func TestAzureProvider_APIVersionPrecedence_ExplicitWinsOverEnv(t *testing.T) {
	withAzureEnv(t, azureEnvAPIVersion, "2024-01-01-from-env")
	withoutAzureEnv(t, azureEnvAPIVersionLegacy)

	const explicit = "2024-06-01-from-config"
	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		Parameters: map[string]interface{}{
			"endpoint":    azureTestEndpoint,
			"api_version": explicit,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, explicit, p.apiVersion,
		"explicit Parameters[\"api_version\"] MUST take precedence over AZURE_OPENAI_API_VERSION")
}

// TestAzureProvider_APIVersionPrecedence_EnvWinsOverDefault asserts
// that AZURE_OPENAI_API_VERSION (canonical Azure SDK env var name) is
// used when no explicit api_version is set in the ProviderConfigEntry.
func TestAzureProvider_APIVersionPrecedence_EnvWinsOverDefault(t *testing.T) {
	const fromEnv = "2024-06-01-from-env"
	withAzureEnv(t, azureEnvAPIVersion, fromEnv)
	withoutAzureEnv(t, azureEnvAPIVersionLegacy)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		Parameters: map[string]interface{}{
			"endpoint": azureTestEndpoint,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, fromEnv, p.apiVersion,
		"AZURE_OPENAI_API_VERSION MUST be used when no Parameters[\"api_version\"] is provided")
}

// TestAzureProvider_APIVersionPrecedence_LegacyEnvAcceptedAsFallback
// asserts that the legacy AZURE_API_VERSION env var is still honoured
// when neither an explicit api_version nor AZURE_OPENAI_API_VERSION is
// set. This preserves backward compatibility for already-deployed
// configs that predate the canonical-env-var alignment in T06.
func TestAzureProvider_APIVersionPrecedence_LegacyEnvAcceptedAsFallback(t *testing.T) {
	withoutAzureEnv(t, azureEnvAPIVersion)
	const fromLegacyEnv = "2023-12-01-from-legacy-env"
	withAzureEnv(t, azureEnvAPIVersionLegacy, fromLegacyEnv)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		Parameters: map[string]interface{}{
			"endpoint": azureTestEndpoint,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, fromLegacyEnv, p.apiVersion,
		"AZURE_API_VERSION (legacy) MUST be honoured when AZURE_OPENAI_API_VERSION is unset")
}

// TestAzureProvider_APIVersionPrecedence_DefaultWhenUnset asserts the
// fallback: when neither Parameters["api_version"] nor any env var is
// set, the provider uses the documented default. Regression guard:
// silently changing the default would route every default-configured
// install at a stale data-plane version.
func TestAzureProvider_APIVersionPrecedence_DefaultWhenUnset(t *testing.T) {
	withoutAzureAPIVersionEnvs(t)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		Parameters: map[string]interface{}{
			"endpoint": azureTestEndpoint,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, azureDefaultAPIVersion, p.apiVersion,
		"with no Parameters[\"api_version\"] and no env var, AzureProvider MUST "+
			"fall back to the documented default (%s)", azureDefaultAPIVersion)
}

// TestAzureProvider_GetModelsRoutesViaVerifier asserts that
// AzureProvider's model-list builder path (azure_provider.go:
// getAzureModels -> EnrichModelInfo at line 439) routes through the
// verifier-priority branch in verifier_bridge.go when an enabled
// adapter is installed.
//
// Detection mechanism (same idiom as anthropic/bedrock/vertexai audit
// tests): the heuristic fallback (inferCapabilitiesFromModelID) always
// appends CapabilityTextGeneration; the verifier branch returns BEFORE
// that. So if every model in the Azure provider's list emerges WITHOUT
// CapabilityTextGeneration after construction with an enabled adapter
// installed, the verifier branch ran for every model — proving
// GetModels() is wired through the verifier.
func TestAzureProvider_GetModelsRoutesViaVerifier(t *testing.T) {
	withoutAzureAPIVersionEnvs(t)

	// Snapshot + restore the package-global so this test doesn't bleed
	// into sibling tests in the same package.
	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })

	enabled := verifier.NewAdapter(nil, nil, nil, &verifier.AdapterConfig{Enabled: true})
	require.True(t, enabled.IsEnabled(),
		"test fixture: enabled adapter must report IsEnabled()==true")
	SetVerifierAdapter(enabled)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		Parameters: map[string]interface{}{
			"endpoint": azureTestEndpoint,
		},
	})
	require.NoError(t, err)

	models := p.GetModels()
	require.NotEmpty(t, models,
		"AzureProvider.GetModels() must return non-empty list at "+
			"construction (verifier-enriched)")

	for _, m := range models {
		// Azure model IDs include "gpt-4*", "gpt-35-turbo*", "o1-*", and
		// embedding model names — all of which match the OpenAI-family
		// heuristics in inferCapabilitiesFromModelID, which would append
		// CapabilityTextGeneration. The verifier branch returns BEFORE
		// that side-effect fires. So the absence of the heuristic-only
		// marker on EVERY model proves the verifier path was the one
		// that ran.
		assert.NotContains(t, m.Capabilities, CapabilityTextGeneration,
			"model %q (%s): EnrichModelInfo must take the verifier-priority "+
				"branch (returns before heuristic appends CapabilityTextGeneration); "+
				"finding it here means the heuristic ran instead — GetModels() "+
				"is NOT wired through the verifier",
			m.ID, m.Name)
	}
}

// TestAzureProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic
// is the negative companion to the test above: with no adapter installed,
// EnrichModelInfo MUST take the heuristic branch — otherwise GetModels()
// returns models with no capabilities populated at all, breaking selectors
// and routers that depend on the inferred metadata as a default.
func TestAzureProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic(t *testing.T) {
	withoutAzureAPIVersionEnvs(t)

	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })
	SetVerifierAdapter(nil)

	p, err := NewAzureProvider(ProviderConfigEntry{
		Type:   ProviderTypeAzure,
		APIKey: "test-key-not-used-for-network",
		Parameters: map[string]interface{}{
			"endpoint": azureTestEndpoint,
		},
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
