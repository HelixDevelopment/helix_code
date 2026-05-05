// Package llm — P1-F12-T02 audit tests.
//
// This file is the F12 conformance harness for the unified Provider interface
// (defined in missing_types.go) and its LLMsVerifier integration path
// (verifier_bridge.go::EnrichModelInfo, verifier_integration.go::VerifierModelSource).
//
// The audit asserts:
//   1. The unified Provider interface exposes the 11 methods F12 builds on.
//   2. All four F12 cloud provider types statically satisfy Provider
//      (compile-time guarantee via blank-identifier assertions in this file).
//   3. EnrichModelInfo routes through the package-global verifierAdapter
//      when one is set via SetVerifierAdapter — i.e. the LLMsVerifier-as-
//      single-source-of-truth wiring (CONST-036/037) is real, not a stub.
//
// These tests run with NO credentials. They MUST NOT call Skip() — failure
// here is a real conformance regression that blocks T03–T06.
package llm

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/verifier"
)

// Compile-time conformance: every F12 cloud provider type satisfies the
// unified Provider interface defined in missing_types.go. If any method is
// missing or has a wrong signature, the package will fail to compile and
// this file will surface the gap immediately.
var (
	_ Provider = (*AnthropicProvider)(nil)
	_ Provider = (*BedrockProvider)(nil)
	_ Provider = (*AzureProvider)(nil)
	_ Provider = (*VertexAIProvider)(nil)
)

// TestProvider_InterfaceMethods asserts the unified Provider interface
// surface that F12 (selector + wizard + factory) builds on. Method names
// and signatures are anchored here; future refactors that drop or rename
// methods will fail this test rather than silently break callers.
func TestProvider_InterfaceMethods(t *testing.T) {
	providerType := reflect.TypeOf((*Provider)(nil)).Elem()

	expected := []struct {
		name string
		// numIn excludes the receiver (interfaces don't have one in NumIn).
		numIn  int
		numOut int
	}{
		{"GetType", 0, 1},
		{"GetName", 0, 1},
		{"GetModels", 0, 1},
		{"GetCapabilities", 0, 1},
		{"Generate", 2, 2},       // ctx, *LLMRequest -> *LLMResponse, error
		{"GenerateStream", 3, 1}, // ctx, *LLMRequest, chan<- LLMResponse -> error
		{"IsAvailable", 1, 1},    // ctx -> bool
		{"GetHealth", 1, 2},      // ctx -> *ProviderHealth, error
		{"Close", 0, 1},
		{"GetContextWindow", 0, 1},
		{"CountTokens", 1, 2}, // text -> int, error
	}

	for _, exp := range expected {
		t.Run(exp.name, func(t *testing.T) {
			m, ok := providerType.MethodByName(exp.name)
			require.True(t, ok, "Provider interface missing required method %q", exp.name)
			assert.Equal(t, exp.numIn, m.Type.NumIn(),
				"method %s: unexpected parameter count", exp.name)
			assert.Equal(t, exp.numOut, m.Type.NumOut(),
				"method %s: unexpected return count", exp.name)
		})
	}
}

// TestProvider_AllFourCloudTypes_AreInProviderType asserts the four F12
// cloud ProviderType constants are declared and distinct. The selector +
// NewCloudProvider helper (T07) switches on these exact values.
func TestProvider_AllFourCloudTypes_AreInProviderType(t *testing.T) {
	cloudTypes := []ProviderType{
		ProviderTypeAnthropic,
		ProviderTypeBedrock,
		ProviderTypeVertexAI,
		ProviderTypeAzure,
	}
	seen := make(map[ProviderType]bool, len(cloudTypes))
	for _, pt := range cloudTypes {
		assert.NotEmpty(t, string(pt), "ProviderType constant must be non-empty")
		assert.False(t, seen[pt], "duplicate ProviderType constant: %s", pt)
		seen[pt] = true
	}
	assert.Equal(t, "anthropic", string(ProviderTypeAnthropic))
	assert.Equal(t, "bedrock", string(ProviderTypeBedrock))
	assert.Equal(t, "vertexai", string(ProviderTypeVertexAI))
	assert.Equal(t, "azure", string(ProviderTypeAzure))
}

// TestProvider_AnthropicConstructable_NoCreds confirms the Anthropic provider
// constructs successfully with a synthetic API key (no network), which is
// the surface the selector + wizard need to guarantee at first-run time.
//
// Bedrock / Vertex / Azure constructors initialise real cloud SDKs and may
// require ambient credentials; their conformance is asserted via the
// compile-time `var _ Provider = ...` checks above plus the per-provider
// audits in T03–T06.
func TestProvider_AnthropicConstructable_NoCreds(t *testing.T) {
	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:     ProviderTypeAnthropic,
		APIKey:   "test-key-not-used-for-network",
		Endpoint: "https://api.anthropic.com/v1/messages",
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	// Provider interface conformance, runtime check.
	var iface Provider = p
	assert.Equal(t, ProviderTypeAnthropic, iface.GetType())
	assert.NotEmpty(t, iface.GetName())
	assert.NotEmpty(t, iface.GetModels(),
		"Anthropic.GetModels() must return non-empty list at construction")
	assert.GreaterOrEqual(t, iface.GetContextWindow(), 0)

	// CountTokens fallback: empty -> 0, non-empty -> >0.
	zero, err := iface.CountTokens("")
	require.NoError(t, err)
	assert.Equal(t, 0, zero)
	some, err := iface.CountTokens("hello world")
	require.NoError(t, err)
	assert.Greater(t, some, 0)
}

// TestProvider_GetModels_RoutesViaVerifier asserts the LLMsVerifier
// integration path: when SetVerifierAdapter installs an enabled adapter,
// EnrichModelInfo (called from every cloud provider's model list builder
// — see anthropic_provider.go:251, bedrock_provider.go:404,
// azure_provider.go:439, vertexai_provider.go:415) takes the
// verifier-priority branch and skips the heuristic inference path.
//
// Heuristic inference (verifier_bridge.go::inferCapabilitiesFromModelID)
// always appends CapabilityTextGeneration. The verifier branch returns
// before that. So we can detect which path ran by inspecting whether the
// model emerges with CapabilityTextGeneration appended for an ID that
// matches no other heuristic keyword.
//
// This is a real-routing assertion, not a mock-call assertion: it certifies
// the F12 wiring is present and active end-to-end through the Anthropic
// provider's GetModels() chain.
func TestProvider_GetModels_RoutesViaVerifier(t *testing.T) {
	// Snapshot + restore the package-global so this test doesn't bleed.
	prev := verifierAdapter
	t.Cleanup(func() { SetVerifierAdapter(prev) })

	// --- Sub-case A: verifier disabled -> heuristic path runs.
	SetVerifierAdapter(nil)
	miA := &ModelInfo{ID: "obscure-model-xyz-no-keyword"}
	EnrichModelInfo(miA)
	assert.Contains(t, miA.Capabilities, CapabilityTextGeneration,
		"with no verifier adapter, EnrichModelInfo MUST fall through to "+
			"inferCapabilitiesFromModelID, which adds CapabilityTextGeneration")

	// --- Sub-case B: verifier enabled -> verifier-priority path runs.
	enabledAdapter := verifier.NewAdapter(nil, nil, nil, &verifier.AdapterConfig{Enabled: true})
	require.True(t, enabledAdapter.IsEnabled(),
		"test fixture: enabled adapter must report IsEnabled()==true")
	SetVerifierAdapter(enabledAdapter)

	miB := &ModelInfo{ID: "obscure-model-xyz-no-keyword"}
	EnrichModelInfo(miB)
	assert.NotContains(t, miB.Capabilities, CapabilityTextGeneration,
		"with verifier adapter enabled, EnrichModelInfo MUST take the "+
			"verifier-priority branch and return BEFORE the heuristic path "+
			"appends CapabilityTextGeneration (see verifier_bridge.go:62-63)")

	// --- Sub-case C: cloud provider GetModels() runs the verifier-priority
	// branch on every model in its list. We construct an Anthropic provider
	// (the only cloud type constructable without ambient credentials) and
	// confirm none of the returned models picked up the heuristic-only
	// CapabilityTextGeneration marker, proving the verifier path was taken
	// for every model.
	p, err := NewAnthropicProvider(ProviderConfigEntry{
		Type:   ProviderTypeAnthropic,
		APIKey: "test-key-not-used-for-network",
	})
	require.NoError(t, err)
	models := p.GetModels()
	require.NotEmpty(t, models)
	for _, m := range models {
		assert.NotContains(t, m.Capabilities, CapabilityTextGeneration,
			"model %q (%s): verifier-priority branch must run; finding "+
				"CapabilityTextGeneration would mean the heuristic ran instead",
			m.ID, m.Name)
	}
}

// TestProvider_VerifierModelSource_Surface confirms VerifierModelSource —
// the dedicated bridge type the wizard (T08) and selector (T07) will use
// to populate provider/model dropdowns — exposes the FetchModels(ctx) +
// IsAvailable() surface the spec §3 calls out.
func TestProvider_VerifierModelSource_Surface(t *testing.T) {
	src := NewVerifierModelSource(nil)
	require.NotNil(t, src)
	// nil adapter -> not available, no panic.
	assert.False(t, src.IsAvailable())

	disabled := verifier.NewAdapter(nil, nil, nil, &verifier.AdapterConfig{Enabled: false})
	src2 := NewVerifierModelSource(disabled)
	assert.False(t, src2.IsAvailable(),
		"disabled adapter must report not-available so wizard can show fallback banner")
}
