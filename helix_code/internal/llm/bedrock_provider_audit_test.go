// Package llm — P1-F12-T04 audit tests for BedrockProvider.
//
// This file is the F12 conformance harness for the BedrockProvider:
//
//  1. Compile-time + runtime confirmation that *BedrockProvider satisfies
//     the unified Provider interface (missing_types.go:191).
//  2. AWS region precedence — explicit Config.Parameters["region"] wins
//     over AWS_REGION env var, which in turn wins over the documented
//     default (us-east-1). This is the contract getBedrockRegion is
//     supposed to honour and that downstream wizards/selectors rely on.
//  3. GetModels() routes through the verifier-priority branch in
//     EnrichModelInfo (verifier_bridge.go:36-63) when an enabled adapter
//     is installed via SetVerifierAdapter — i.e. the LLMsVerifier-as-
//     single-source-of-truth wiring (CONST-036/037) is real, not a stub.
//
// These tests run with NO real AWS credentials and NO network. Dummy
// credentials are exported into the env so the AWS SDK config loader
// is happy at construction time; the actual credential validation
// (and any network call) is deferred to first API invocation, which
// these tests never trigger. The tests MUST NOT call Skip() — failure
// here is a real conformance regression that blocks T07–T11.
//
// Note on ProviderConfigEntry shape: the F12 plan refers to
// "Config.Region" as the explicit per-provider override. The shared
// ProviderConfigEntry struct (missing_types.go:94) has no Region
// field — providers use Parameters["region"] for that knob (mirroring
// the canonical config schema and the existing TestNewBedrockProvider
// table cases). These audit tests therefore drive precedence through
// Parameters["region"], which is the actual code path
// getBedrockRegion() reads.
package llm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/verifier"
)

// Compile-time conformance: *BedrockProvider satisfies the unified
// Provider interface defined in missing_types.go. The file under test
// (bedrock_provider.go) is the canonical implementation; this assertion
// fails at build time if any method is dropped or has its signature broken.
var _ Provider = (*BedrockProvider)(nil)

// awsEnvRegion is the env var the AWS SDK and getBedrockRegion both
// honour when no explicit region is configured.
const awsEnvRegion = "AWS_REGION"

// awsEnvDefaultRegion is the secondary env var getBedrockRegion checks
// after AWS_REGION. We control it in tests to keep precedence cases
// hermetic.
const awsEnvDefaultRegion = "AWS_DEFAULT_REGION"

// bedrockDefaultRegion is the documented fallback baked into
// getBedrockRegion when neither config nor env var supplies a region.
// Encoded here as a regression guard: silently changing the default
// would route every default-configured install at the wrong region.
const bedrockDefaultRegion = "us-east-1"

// withBedrockTestAWSCreds installs dummy AWS credentials in the env so
// the AWS SDK's default config loader can construct without error.
// These credentials are syntactically valid but never actually used —
// these tests do not make AWS API calls.
func withBedrockTestAWSCreds(t *testing.T) {
	t.Helper()
	for _, kv := range [][2]string{
		{"AWS_ACCESS_KEY_ID", "AKIA-TEST-NOT-REAL"},
		{"AWS_SECRET_ACCESS_KEY", "test-secret-not-real"},
	} {
		key, value := kv[0], kv[1]
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
}

// withAWSRegionEnv sets AWS_REGION for the duration of the test and
// restores the prior value (or unset state) on cleanup.
func withAWSRegionEnv(t *testing.T, value string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(awsEnvRegion)
	require.NoError(t, os.Setenv(awsEnvRegion, value))
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(awsEnvRegion, prev)
		} else {
			_ = os.Unsetenv(awsEnvRegion)
		}
	})
}

// withoutAWSRegionEnv ensures AWS_REGION (and the secondary
// AWS_DEFAULT_REGION) are unset for the duration of the test, restoring
// prior state on cleanup. Both must be cleared because getBedrockRegion
// falls through to AWS_DEFAULT_REGION when AWS_REGION is empty.
func withoutAWSRegionEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{awsEnvRegion, awsEnvDefaultRegion} {
		prev, hadPrev := os.LookupEnv(key)
		require.NoError(t, os.Unsetenv(key))
		t.Cleanup(func() {
			if hadPrev {
				_ = os.Setenv(key, prev)
			}
		})
	}
}

// TestBedrockProvider_RegionPrecedence_ExplicitWinsOverEnv asserts that an
// explicit Parameters["region"] entry takes precedence over the AWS_REGION
// env var. This is the highest-priority path: a user who has wired a
// specific region into their config must always reach it, even if AWS_REGION
// is exported globally on the host.
func TestBedrockProvider_RegionPrecedence_ExplicitWinsOverEnv(t *testing.T) {
	withBedrockTestAWSCreds(t)
	withAWSRegionEnv(t, "eu-west-1")

	const explicit = "us-west-2"
	p, err := NewBedrockProvider(ProviderConfigEntry{
		Type:    ProviderTypeBedrock,
		Enabled: true,
		Parameters: map[string]interface{}{
			"region": explicit,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, explicit, p.region,
		"explicit Parameters[\"region\"] MUST take precedence over AWS_REGION")
}

// TestBedrockProvider_RegionPrecedence_EnvWinsOverDefault asserts that the
// AWS_REGION env var is honoured when no explicit region is set in the
// ProviderConfigEntry. Wizards and ad-hoc CLI invocations rely on this
// path to switch regions without persisting a config change.
func TestBedrockProvider_RegionPrecedence_EnvWinsOverDefault(t *testing.T) {
	withBedrockTestAWSCreds(t)

	const fromEnv = "eu-west-1"
	withAWSRegionEnv(t, fromEnv)

	p, err := NewBedrockProvider(ProviderConfigEntry{
		Type:    ProviderTypeBedrock,
		Enabled: true,
		// Parameters intentionally unset.
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, fromEnv, p.region,
		"AWS_REGION MUST be used when no Parameters[\"region\"] is provided")
}

// TestBedrockProvider_RegionPrecedence_DefaultWhenUnset asserts the
// fallback: when neither Parameters["region"] nor AWS_REGION (nor
// AWS_DEFAULT_REGION) is set, the provider points at the documented
// default region (us-east-1). Regression guard: silently changing the
// default would route every default-configured install at the wrong
// AWS endpoint.
func TestBedrockProvider_RegionPrecedence_DefaultWhenUnset(t *testing.T) {
	withBedrockTestAWSCreds(t)
	withoutAWSRegionEnv(t)

	p, err := NewBedrockProvider(ProviderConfigEntry{
		Type:    ProviderTypeBedrock,
		Enabled: true,
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, bedrockDefaultRegion, p.region,
		"with no explicit region and no AWS_REGION env var, BedrockProvider "+
			"MUST fall back to the documented default region (us-east-1)")
}

// TestBedrockProvider_GetModelsRoutesViaVerifier asserts that
// BedrockProvider's model-list builder path (bedrock_provider.go:
// getBedrockModels -> EnrichModelInfo at line 404) routes through the
// verifier-priority branch in verifier_bridge.go when an enabled adapter
// is installed.
//
// Detection mechanism (same as anthropic_provider_audit_test.go:168 — the
// F12 audit idiom): the heuristic fallback (inferCapabilitiesFromModelID)
// always appends CapabilityTextGeneration; the verifier branch returns
// BEFORE that. So if any model in the Bedrock provider's list emerges
// WITHOUT CapabilityTextGeneration after construction with an enabled
// adapter installed, the verifier branch ran for that model — proving
// GetModels() is wired through the verifier.
//
// This is a real-routing assertion (verifier_bridge.go IS the integration
// point BedrockProvider uses), not a mock-call assertion.
func TestBedrockProvider_GetModelsRoutesViaVerifier(t *testing.T) {
	withBedrockTestAWSCreds(t)
	withoutAWSRegionEnv(t)

	// Snapshot + restore the package-global so this test doesn't bleed
	// into sibling tests in the same package.
	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })

	enabled := verifier.NewAdapter(nil, nil, nil, &verifier.AdapterConfig{Enabled: true})
	require.True(t, enabled.IsEnabled(),
		"test fixture: enabled adapter must report IsEnabled()==true")
	SetVerifierAdapter(enabled)

	p, err := NewBedrockProvider(ProviderConfigEntry{
		Type:    ProviderTypeBedrock,
		Enabled: true,
	})
	require.NoError(t, err)

	models := p.GetModels()
	require.NotEmpty(t, models,
		"BedrockProvider.GetModels() must return non-empty list at "+
			"construction (verifier-enriched)")

	for _, m := range models {
		// Bedrock model IDs include "claude-3-*", "claude-4-*",
		// "claude-3-5-*" (Anthropic via Bedrock) which match the
		// "claude-3" / "code"-family heuristics in
		// inferCapabilitiesFromModelID — that path appends
		// CapabilityTextGeneration to every model. The verifier branch
		// returns BEFORE that side-effect fires. So the absence of the
		// heuristic-only marker on EVERY model proves the verifier
		// path was the one that ran.
		assert.NotContains(t, m.Capabilities, CapabilityTextGeneration,
			"model %q (%s): EnrichModelInfo must take the verifier-priority "+
				"branch (returns before heuristic appends CapabilityTextGeneration); "+
				"finding it here means the heuristic ran instead — GetModels() "+
				"is NOT wired through the verifier",
			m.ID, m.Name)
	}
}

// TestBedrockProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic
// is the negative companion to the test above: with no adapter installed,
// EnrichModelInfo MUST take the heuristic branch — otherwise GetModels()
// returns models with no capabilities populated at all, breaking selectors
// and routers that depend on the inferred metadata as a default.
func TestBedrockProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic(t *testing.T) {
	withBedrockTestAWSCreds(t)
	withoutAWSRegionEnv(t)

	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })
	SetVerifierAdapter(nil)

	p, err := NewBedrockProvider(ProviderConfigEntry{
		Type:    ProviderTypeBedrock,
		Enabled: true,
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
