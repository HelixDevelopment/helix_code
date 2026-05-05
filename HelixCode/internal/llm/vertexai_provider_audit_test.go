// Package llm — P1-F12-T05 audit tests for VertexAIProvider.
//
// This file is the F12 conformance harness for the VertexAIProvider:
//
//  1. Compile-time + runtime confirmation that *VertexAIProvider satisfies
//     the unified Provider interface (missing_types.go:191).
//  2. GCP project + location precedence — explicit
//     Parameters["project_id"] / Parameters["location"] win over their env-var
//     counterparts, which in turn win over the documented default (location
//     fallback us-central1; project has no static default and must error
//     when entirely unset). Downstream wizards/selectors rely on this
//     contract.
//  3. GetModels() routes through the verifier-priority branch in
//     EnrichModelInfo (verifier_bridge.go:36-63) when an enabled adapter
//     is installed via SetVerifierAdapter — i.e. the LLMsVerifier-as-
//     single-source-of-truth wiring (CONST-036/037) is real, not a stub.
//
// These tests run with NO real GCP credentials and NO network.
// GOOGLE_APPLICATION_CREDENTIALS is intentionally cleared and
// FindDefaultCredentials must NOT be required at construction — credential
// resolution is deferred to the first API call. The tests MUST NOT call
// Skip() — failure here is a real conformance regression that blocks
// T07–T11.
//
// Note on env-var convention: the existing VertexAIProvider reads
// VERTEXAI_PROJECT / VERTEXAI_LOCATION (and GCP_PROJECT as a secondary
// fallback). T05 preserves that established convention and adds the
// canonical GCP-standard GOOGLE_CLOUD_PROJECT / GOOGLE_CLOUD_LOCATION
// env vars as additional fallbacks (in that order, after the
// VERTEXAI_*  names). The precedence tests below pin every link in that
// chain.
package llm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/verifier"
)

// Compile-time conformance: *VertexAIProvider satisfies the unified
// Provider interface defined in missing_types.go. The file under test
// (vertexai_provider.go) is the canonical implementation; this assertion
// fails at build time if any method is dropped or has its signature broken.
var _ Provider = (*VertexAIProvider)(nil)

// vertexEnvProjectPrimary is the env var the existing VertexAIProvider has
// honoured since inception. T05 pins it as the highest-priority env-var
// override, ahead of the GCP-canonical GOOGLE_CLOUD_PROJECT.
const vertexEnvProjectPrimary = "VERTEXAI_PROJECT"

// vertexEnvProjectGCP is the canonical GCP-standard env var for the project
// ID. T05 adds it as a secondary fallback (after VERTEXAI_PROJECT and
// before GCP_PROJECT) so users with a standard GCP CLI environment do not
// have to set a Vertex-specific variable.
const vertexEnvProjectGCP = "GOOGLE_CLOUD_PROJECT"

// vertexEnvProjectLegacy is the long-standing GCP-CLI env var. Kept as the
// last-resort project fallback for compatibility with the existing test
// fixtures.
const vertexEnvProjectLegacy = "GCP_PROJECT"

// vertexEnvLocationPrimary is the env var the existing VertexAIProvider has
// honoured since inception for the model-serving location/region.
const vertexEnvLocationPrimary = "VERTEXAI_LOCATION"

// vertexEnvLocationGCP is the canonical GCP-standard env var for the
// model-serving location/region. T05 adds it as a fallback after
// VERTEXAI_LOCATION.
const vertexEnvLocationGCP = "GOOGLE_CLOUD_LOCATION"

// vertexDefaultLocation is the documented fallback baked into
// NewVertexAIProvider when neither config nor env var supplies a location.
// Encoded here as a regression guard: silently changing the default would
// route every default-configured install at the wrong Vertex AI region.
const vertexDefaultLocation = "us-central1"

// vertexEnvCredentialsFile is the env var the gcloud SDK and the Vertex
// provider's credential loader honour. We unset it for these tests so
// construction never tries to resolve real credentials.
const vertexEnvCredentialsFile = "GOOGLE_APPLICATION_CREDENTIALS"

// withVertexProjectEnv sets a project env var for the duration of the test
// and restores the prior value (or unset state) on cleanup.
func withVertexProjectEnv(t *testing.T, key, value string) {
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

// withVertexLocationEnv sets a location env var for the duration of the
// test and restores the prior value (or unset state) on cleanup.
func withVertexLocationEnv(t *testing.T, key, value string) {
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

// withoutVertexProjectEnv ensures every project env var the provider reads
// is unset for the duration of the test, restoring prior state on cleanup.
func withoutVertexProjectEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{vertexEnvProjectPrimary, vertexEnvProjectGCP, vertexEnvProjectLegacy} {
		prev, hadPrev := os.LookupEnv(key)
		require.NoError(t, os.Unsetenv(key))
		t.Cleanup(func() {
			if hadPrev {
				_ = os.Setenv(key, prev)
			}
		})
	}
}

// withoutVertexLocationEnv ensures every location env var the provider reads
// is unset for the duration of the test, restoring prior state on cleanup.
func withoutVertexLocationEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{vertexEnvLocationPrimary, vertexEnvLocationGCP} {
		prev, hadPrev := os.LookupEnv(key)
		require.NoError(t, os.Unsetenv(key))
		t.Cleanup(func() {
			if hadPrev {
				_ = os.Setenv(key, prev)
			}
		})
	}
}

// withoutVertexCredentialsEnv unsets GOOGLE_APPLICATION_CREDENTIALS so
// FindDefaultCredentials cannot accidentally find a real GCP service
// account file the developer happens to have on their host. Restored
// on cleanup.
func withoutVertexCredentialsEnv(t *testing.T) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(vertexEnvCredentialsFile)
	require.NoError(t, os.Unsetenv(vertexEnvCredentialsFile))
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(vertexEnvCredentialsFile, prev)
		}
	})
}

// TestVertexAIProvider_ProjectPrecedence_ExplicitWinsOverEnv asserts that an
// explicit Parameters["project_id"] entry takes precedence over every
// project env var. This is the highest-priority path: a user who has wired
// a specific project into their config must always reach it, even if
// VERTEXAI_PROJECT / GOOGLE_CLOUD_PROJECT / GCP_PROJECT are exported
// globally on the host.
func TestVertexAIProvider_ProjectPrecedence_ExplicitWinsOverEnv(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexLocationEnv(t)
	withoutVertexProjectEnv(t)
	withVertexProjectEnv(t, vertexEnvProjectPrimary, "env-primary-project")
	withVertexProjectEnv(t, vertexEnvProjectGCP, "env-gcp-project")
	withVertexProjectEnv(t, vertexEnvProjectLegacy, "env-legacy-project")

	const explicit = "explicit-project"
	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type: ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			"project_id": explicit,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, explicit, p.projectID,
		"explicit Parameters[\"project_id\"] MUST take precedence over every project env var")
}

// TestVertexAIProvider_ProjectPrecedence_EnvWinsOverEmpty asserts that the
// VERTEXAI_PROJECT env var is honoured when no explicit project_id is set
// in the ProviderConfigEntry. This is the ordering wizards and ad-hoc CLI
// invocations rely on to swap the project without persisting a config
// change. VERTEXAI_PROJECT MUST also win over GOOGLE_CLOUD_PROJECT and
// GCP_PROJECT to preserve the existing convention.
func TestVertexAIProvider_ProjectPrecedence_EnvWinsOverEmpty(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexLocationEnv(t)
	withoutVertexProjectEnv(t)
	withVertexProjectEnv(t, vertexEnvProjectPrimary, "env-primary-project")
	withVertexProjectEnv(t, vertexEnvProjectGCP, "env-gcp-project")
	withVertexProjectEnv(t, vertexEnvProjectLegacy, "env-legacy-project")

	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type:       ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			// project_id intentionally unset.
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "env-primary-project", p.projectID,
		"VERTEXAI_PROJECT MUST be used when no Parameters[\"project_id\"] is provided "+
			"and MUST win over GOOGLE_CLOUD_PROJECT / GCP_PROJECT")
}

// TestVertexAIProvider_ProjectPrecedence_GCPEnvWinsOverLegacyOnly asserts
// the secondary fallback: when neither explicit Parameters["project_id"]
// nor VERTEXAI_PROJECT is set, the canonical GCP-standard
// GOOGLE_CLOUD_PROJECT env var is used (and wins over the older GCP_PROJECT
// fallback).
func TestVertexAIProvider_ProjectPrecedence_GCPEnvWinsOverLegacyOnly(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexLocationEnv(t)
	withoutVertexProjectEnv(t)
	withVertexProjectEnv(t, vertexEnvProjectGCP, "env-gcp-project")
	withVertexProjectEnv(t, vertexEnvProjectLegacy, "env-legacy-project")

	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type:       ProviderTypeVertexAI,
		Parameters: map[string]interface{}{},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "env-gcp-project", p.projectID,
		"with no Parameters[\"project_id\"] and no VERTEXAI_PROJECT, "+
			"GOOGLE_CLOUD_PROJECT MUST be honoured and MUST win over GCP_PROJECT")
}

// TestVertexAIProvider_LocationPrecedence_ExplicitWinsOverEnv asserts that
// an explicit Parameters["location"] entry takes precedence over the
// VERTEXAI_LOCATION / GOOGLE_CLOUD_LOCATION env vars.
func TestVertexAIProvider_LocationPrecedence_ExplicitWinsOverEnv(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexProjectEnv(t)
	withoutVertexLocationEnv(t)
	withVertexLocationEnv(t, vertexEnvLocationPrimary, "europe-west4")
	withVertexLocationEnv(t, vertexEnvLocationGCP, "europe-west1")

	const explicit = "us-west1"
	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type: ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			"project_id": "test-project",
			"location":   explicit,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, explicit, p.location,
		"explicit Parameters[\"location\"] MUST take precedence over location env vars")
}

// TestVertexAIProvider_LocationPrecedence_EnvWinsOverDefault asserts that
// VERTEXAI_LOCATION is honoured when no explicit location is set, and wins
// over GOOGLE_CLOUD_LOCATION (preserves the existing convention).
func TestVertexAIProvider_LocationPrecedence_EnvWinsOverDefault(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexProjectEnv(t)
	withoutVertexLocationEnv(t)
	withVertexLocationEnv(t, vertexEnvLocationPrimary, "europe-west4")
	withVertexLocationEnv(t, vertexEnvLocationGCP, "europe-west1")

	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type: ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			"project_id": "test-project",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "europe-west4", p.location,
		"VERTEXAI_LOCATION MUST be used when no Parameters[\"location\"] is "+
			"provided and MUST win over GOOGLE_CLOUD_LOCATION")
}

// TestVertexAIProvider_LocationPrecedence_DefaultWhenUnset asserts the
// fallback: when neither Parameters["location"] nor any location env var is
// set, the provider points at the documented default (us-central1).
// Regression guard: silently changing the default would route every
// default-configured install at the wrong Vertex AI region.
func TestVertexAIProvider_LocationPrecedence_DefaultWhenUnset(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexProjectEnv(t)
	withoutVertexLocationEnv(t)

	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type: ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			"project_id": "test-project",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, vertexDefaultLocation, p.location,
		"with no explicit location and no location env var, VertexAIProvider "+
			"MUST fall back to the documented default location (us-central1)")
}

// TestVertexAIProvider_ConstructionWithoutAmbientCredentials asserts that
// when GOOGLE_APPLICATION_CREDENTIALS is unset and no credentials_path is
// provided, NewVertexAIProvider does NOT panic and does NOT return an
// error solely on the basis of missing credentials. Credential resolution
// must be deferred to the first API call so that helixcode can construct a
// provider, list verifier-backed models, and run the wizard on a host with
// no GCP creds at all. This is the precondition that makes the project
// precedence tests above work without ambient creds.
func TestVertexAIProvider_ConstructionWithoutAmbientCredentials(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexProjectEnv(t)
	withoutVertexLocationEnv(t)

	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type: ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			"project_id": "test-project",
		},
	})
	require.NoError(t, err,
		"NewVertexAIProvider MUST construct successfully when project_id is "+
			"set and GOOGLE_APPLICATION_CREDENTIALS is unset; credential "+
			"resolution MUST be deferred to first API call")
	require.NotNil(t, p)
	assert.Equal(t, "test-project", p.projectID)
}

// TestVertexAIProvider_GetModelsRoutesViaVerifier asserts that
// VertexAIProvider's model-list builder path (vertexai_provider.go:
// getVertexAIModels -> EnrichModelInfo at line 415) routes through the
// verifier-priority branch in verifier_bridge.go when an enabled adapter
// is installed.
//
// Detection mechanism (same as anthropic_provider_audit_test.go:168 and
// bedrock_provider_audit_test.go:203 — the F12 audit idiom): the heuristic
// fallback (inferCapabilitiesFromModelID) always appends
// CapabilityTextGeneration; the verifier branch returns BEFORE that. So if
// any model in the Vertex AI provider's list emerges WITHOUT
// CapabilityTextGeneration after construction with an enabled adapter
// installed, the verifier branch ran for that model — proving GetModels()
// is wired through the verifier.
//
// This is a real-routing assertion (verifier_bridge.go IS the integration
// point VertexAIProvider uses), not a mock-call assertion.
func TestVertexAIProvider_GetModelsRoutesViaVerifier(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexProjectEnv(t)
	withoutVertexLocationEnv(t)

	// Snapshot + restore the package-global so this test doesn't bleed
	// into sibling tests in the same package.
	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })

	enabled := verifier.NewAdapter(nil, nil, nil, &verifier.AdapterConfig{Enabled: true})
	require.True(t, enabled.IsEnabled(),
		"test fixture: enabled adapter must report IsEnabled()==true")
	SetVerifierAdapter(enabled)

	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type: ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			"project_id": "test-project",
		},
	})
	require.NoError(t, err)

	models := p.GetModels()
	require.NotEmpty(t, models,
		"VertexAIProvider.GetModels() must return non-empty list at "+
			"construction (verifier-enriched)")

	for _, m := range models {
		// Vertex model IDs include "gemini-1.5-pro", "gemini-2.0-flash",
		// "gemini-2.5-flash" and "claude-*-sonnet@..." which match the
		// "gemini-1.5" / "gemini-2" / "claude-3" heuristics in
		// inferCapabilitiesFromModelID — that path appends
		// CapabilityTextGeneration to every model. The verifier branch
		// returns BEFORE that side-effect fires. So the absence of the
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

// TestVertexAIProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic
// is the negative companion to the test above: with no adapter installed,
// EnrichModelInfo MUST take the heuristic branch — otherwise GetModels()
// returns models with no capabilities populated at all, breaking selectors
// and routers that depend on the inferred metadata as a default.
func TestVertexAIProvider_GetModelsRoutesViaVerifier_DisabledFallsBackToHeuristic(t *testing.T) {
	withoutVertexCredentialsEnv(t)
	withoutVertexProjectEnv(t)
	withoutVertexLocationEnv(t)

	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })
	SetVerifierAdapter(nil)

	p, err := NewVertexAIProvider(ProviderConfigEntry{
		Type: ProviderTypeVertexAI,
		Parameters: map[string]interface{}{
			"project_id": "test-project",
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
