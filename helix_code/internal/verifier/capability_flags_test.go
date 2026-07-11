package verifier

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerificationResult_CapabilityFields_DefaultFalse is the RED test for
// HXC-117 Phase 1 (§11.4.115 RED-baseline-on-the-broken-artifact): before the
// six CONST-040 capability fields exist on VerificationResult, this file does
// not even compile. Once the fields are added (additive, non-breaking per the
// design doc §2.2/§2.4), the zero-value struct MUST report every capability
// flag as false — "not verified as supporting", never "verified as NOT
// supporting" (see the field doc comment on VerificationResult).
func TestVerificationResult_CapabilityFields_DefaultFalse(t *testing.T) {
	var r VerificationResult

	assert.False(t, r.SupportsMCP, "zero-value VerificationResult.SupportsMCP must default false")
	assert.False(t, r.SupportsLSP, "zero-value VerificationResult.SupportsLSP must default false")
	assert.False(t, r.SupportsACP, "zero-value VerificationResult.SupportsACP must default false")
	assert.False(t, r.SupportsRAG, "zero-value VerificationResult.SupportsRAG must default false")
	assert.False(t, r.SupportsSkills, "zero-value VerificationResult.SupportsSkills must default false")
	assert.False(t, r.SupportsPlugins, "zero-value VerificationResult.SupportsPlugins must default false")
}

// TestVerifiedModel_CapabilityFields_DefaultFalse mirrors the above for
// VerifiedModel (the catalogue-listing type), per the design doc's mandate
// that VerifiedModel mirror the same six fields for consistency with the
// existing SupportsEmbeddings duplication pattern already present in both
// structs.
func TestVerifiedModel_CapabilityFields_DefaultFalse(t *testing.T) {
	var m VerifiedModel

	assert.False(t, m.SupportsMCP, "zero-value VerifiedModel.SupportsMCP must default false")
	assert.False(t, m.SupportsLSP, "zero-value VerifiedModel.SupportsLSP must default false")
	assert.False(t, m.SupportsACP, "zero-value VerifiedModel.SupportsACP must default false")
	assert.False(t, m.SupportsRAG, "zero-value VerifiedModel.SupportsRAG must default false")
	assert.False(t, m.SupportsSkills, "zero-value VerifiedModel.SupportsSkills must default false")
	assert.False(t, m.SupportsPlugins, "zero-value VerifiedModel.SupportsPlugins must default false")
}

// TestVerificationResult_CapabilityFields_JSONRoundTrip proves the six new
// fields (de)serialize through JSON exactly like the existing
// SupportsEmbeddings field (types.go), using the same snake_case json-tag
// convention, so a real LLMsVerifier HTTP response (see client.go's
// VerifyModel, which does json.Unmarshal directly into VerificationResult)
// can populate them without any additional plumbing.
func TestVerificationResult_CapabilityFields_JSONRoundTrip(t *testing.T) {
	src := VerificationResult{
		ModelID:         "test-model",
		SupportsMCP:     true,
		SupportsLSP:     true,
		SupportsACP:     true,
		SupportsRAG:     true,
		SupportsSkills:  true,
		SupportsPlugins: true,
	}

	raw, err := json.Marshal(src)
	require.NoError(t, err)

	// Assert the exact wire field names (snake_case, matching the
	// "supports_embeddings" pattern at types.go line 95).
	var asMap map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &asMap))
	for _, key := range []string{
		"supports_mcp",
		"supports_lsp",
		"supports_acp",
		"supports_rag",
		"supports_skills",
		"supports_plugins",
	} {
		v, ok := asMap[key]
		require.Truef(t, ok, "expected JSON key %q in marshaled VerificationResult, got: %s", key, raw)
		assert.Equal(t, true, v, "expected %q to round-trip as true", key)
	}

	var dst VerificationResult
	require.NoError(t, json.Unmarshal(raw, &dst))
	assert.Equal(t, src, dst, "VerificationResult must round-trip through JSON unchanged")
}

// TestVerifiedModel_CapabilityFields_JSONRoundTrip mirrors the round-trip
// proof for VerifiedModel.
func TestVerifiedModel_CapabilityFields_JSONRoundTrip(t *testing.T) {
	src := VerifiedModel{
		ID:              "test-model",
		SupportsMCP:     true,
		SupportsLSP:     true,
		SupportsACP:     true,
		SupportsRAG:     true,
		SupportsSkills:  true,
		SupportsPlugins: true,
	}

	raw, err := json.Marshal(src)
	require.NoError(t, err)

	var asMap map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &asMap))
	for _, key := range []string{
		"supports_mcp",
		"supports_lsp",
		"supports_acp",
		"supports_rag",
		"supports_skills",
		"supports_plugins",
	} {
		v, ok := asMap[key]
		require.Truef(t, ok, "expected JSON key %q in marshaled VerifiedModel, got: %s", key, raw)
		assert.Equal(t, true, v, "expected %q to round-trip as true", key)
	}

	var dst VerifiedModel
	require.NoError(t, json.Unmarshal(raw, &dst))
	assert.Equal(t, src, dst, "VerifiedModel must round-trip through JSON unchanged")
}
