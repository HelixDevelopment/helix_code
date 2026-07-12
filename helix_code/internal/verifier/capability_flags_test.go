package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// ---------------------------------------------------------------------------
// HXC-117 Phase 2 — real-data-path population tests.
//
// Phase 1 (above) proved the six fields exist and round-trip on the isolated
// struct. Phase 2 proves the REAL client decode path (client.go's
// unmarshalModelArray / GetModelByID / VerifyModel — the exact functions
// poller.go and adapter.go call to obtain VerifiedModel/VerificationResult
// from the actual LLMsVerifier HTTP service) genuinely populates these fields
// end-to-end, for BOTH the singular CONST-040 doc.go convention HelixCode's
// own struct tags use ("supports_mcp"/"supports_lsp"/"supports_acp") and the
// plural convention the REAL LLMsVerifier service's own DB-backed wire schema
// uses ("supports_mcps"/"supports_lsps"/"supports_acps" — confirmed by direct
// inspection of submodules/llms_verifier/llm-verifier/database/database.go
// this session; RAG/Skills/Plugins already match exactly on both sides). See
// the capabilityAliasFields doc comment in client.go for the full citation.
// ---------------------------------------------------------------------------

// TestClient_GetModels_CapabilityFlags_RealServerPluralKeys proves
// unmarshalModelArray (via Client.GetModels) promotes the real LLMsVerifier
// service's plural-keyed MCP/LSP/ACP flags onto VerifiedModel's singular-tag
// fields, and decodes the (already-matching) RAG/Skills/Plugins keys directly.
func TestClient_GetModels_CapabilityFlags_RealServerPluralKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"models": [
				{"id":"gpt-4o","model_id":"gpt-4o","name":"GPT-4o","provider":"openai","status":"verified","score":9.1,
				 "supports_mcps":true,"supports_lsps":true,"supports_acps":true,
				 "supports_rag":true,"supports_skills":true,"supports_plugins":true}
			],
			"count": 1
		}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	models, err := c.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)

	m := models[0]
	assert.True(t, m.SupportsMCP, "supports_mcps (real-server plural key) must promote onto SupportsMCP")
	assert.True(t, m.SupportsLSP, "supports_lsps (real-server plural key) must promote onto SupportsLSP")
	assert.True(t, m.SupportsACP, "supports_acps (real-server plural key) must promote onto SupportsACP")
	assert.True(t, m.SupportsRAG, "supports_rag decodes directly — no alias needed")
	assert.True(t, m.SupportsSkills, "supports_skills decodes directly — no alias needed")
	assert.True(t, m.SupportsPlugins, "supports_plugins decodes directly — no alias needed")
}

// TestClient_GetModels_CapabilityFlags_SingularKeys proves the SAME decode
// path also honors HelixCode's own singular CONST-040 doc.go convention
// directly via the struct's existing json tags (no alias plumbing needed for
// this shape — it was already structurally true since Phase 1, but never
// actually exercised through the real client decode functions until now).
func TestClient_GetModels_CapabilityFlags_SingularKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"claude-3-5-sonnet","name":"Claude 3.5 Sonnet","provider":"anthropic",
			 "supports_mcp":true,"supports_lsp":true,"supports_acp":true,
			 "supports_rag":true,"supports_skills":true,"supports_plugins":true}
		]`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	models, err := c.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)

	m := models[0]
	assert.True(t, m.SupportsMCP)
	assert.True(t, m.SupportsLSP)
	assert.True(t, m.SupportsACP)
	assert.True(t, m.SupportsRAG)
	assert.True(t, m.SupportsSkills)
	assert.True(t, m.SupportsPlugins)
}

// TestClient_GetModels_CapabilityFlags_HonestFalse_WhenAbsent proves the
// decode path never fabricates a capability: a response shaped exactly like
// the REAL LLMsVerifier service's CURRENT live ListModelsHandler output
// (confirmed by direct inspection of
// submodules/llms_verifier/llm-verifier/api/handlers.go this session — it
// does not emit ANY of the six CONST-040 capability keys today) must leave
// every flag at its honest zero value, never true.
func TestClient_GetModels_CapabilityFlags_HonestFalse_WhenAbsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Exact shape of the real ListModelsHandler's map[string]any response
		// today: id/model_id/name/provider/provider_id/status/score/
		// capabilities/description/version/deprecated/created_at/updated_at —
		// no supports_mcp(s)/lsp(s)/acp(s)/rag/skills/plugins key at all.
		_, _ = w.Write([]byte(`{
			"models": [
				{"id":1,"model_id":"gpt-4o","name":"GPT-4o","provider":"openai","provider_id":1,
				 "status":"verified","score":9.1,"capabilities":["text","vision"],"deprecated":false}
			],
			"count": 1
		}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	models, err := c.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)

	m := models[0]
	assert.False(t, m.SupportsMCP, "must NOT fabricate a capability the real server did not report")
	assert.False(t, m.SupportsLSP)
	assert.False(t, m.SupportsACP)
	assert.False(t, m.SupportsRAG)
	assert.False(t, m.SupportsSkills)
	assert.False(t, m.SupportsPlugins)
}

// TestClient_GetModelByID_CapabilityFlags_RealServerPluralKeys proves
// GetModelByID applies the same plural-key reconciliation as GetModels.
func TestClient_GetModelByID_CapabilityFlags_RealServerPluralKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"gpt-4o","name":"GPT-4o","provider":"openai",
			"supports_mcps":true,"supports_lsps":true,"supports_acps":true,
			"supports_rag":true,"supports_skills":true,"supports_plugins":true}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	m, err := c.GetModelByID(context.Background(), "gpt-4o")
	require.NoError(t, err)
	assert.True(t, m.SupportsMCP)
	assert.True(t, m.SupportsLSP)
	assert.True(t, m.SupportsACP)
	assert.True(t, m.SupportsRAG)
	assert.True(t, m.SupportsSkills)
	assert.True(t, m.SupportsPlugins)
}

// TestClient_VerifyModel_CapabilityFlags_RealServerPluralKeys proves
// VerifyModel applies the same plural-key reconciliation for
// VerificationResult (the on-demand-verification return type).
func TestClient_VerifyModel_CapabilityFlags_RealServerPluralKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/models/gpt-4o/verify", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model_id":"gpt-4o","status":"completed",
			"supports_mcps":true,"supports_lsps":true,"supports_acps":true,
			"supports_rag":true,"supports_skills":true,"supports_plugins":true}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	result, err := c.VerifyModel(context.Background(), "gpt-4o")
	require.NoError(t, err)
	assert.True(t, result.SupportsMCP)
	assert.True(t, result.SupportsLSP)
	assert.True(t, result.SupportsACP)
	assert.True(t, result.SupportsRAG)
	assert.True(t, result.SupportsSkills)
	assert.True(t, result.SupportsPlugins)
}

// TestClient_VerifyModel_CapabilityFlags_HonestFalse_WhenAbsent proves
// VerifyModel never fabricates a capability either: a response shaped exactly
// like the REAL LLMsVerifier service's CURRENT live VerifyModelHandler output
// (confirmed by direct inspection of
// submodules/llms_verifier/llm-verifier/api/handlers.go this session — its
// hand-rolled map[string]any response carries only
// status/model_id/model_name/verification_status/score/message/job_id/
// verification_id/started_at/completed_at, none of the six CONST-040
// capability keys) must leave every flag false.
func TestClient_VerifyModel_CapabilityFlags_HonestFalse_WhenAbsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// NOTE: the real VerifyModelHandler actually emits "model_id" as a
		// numeric int64 (api/handlers.go line ~349: `"model_id": modelID`),
		// which VerificationResult.ModelID (string, types.go) cannot decode —
		// a genuine PRE-EXISTING mismatch discovered this session, unrelated
		// to the six CONST-040 capability fields and out of scope for
		// HXC-117 Phase 2 (tracked for a future fix, not silently worked
		// around here). This test omits model_id entirely so it exercises
		// ONLY the capability-flag-absence behavior it is named for.
		_, _ = w.Write([]byte(`{"status":"completed","model_name":"GPT-4o",
			"verification_status":"verified","score":9.1,"message":"ok","job_id":42,
			"verification_id":42}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", 0)
	result, err := c.VerifyModel(context.Background(), "gpt-4o")
	require.NoError(t, err)
	assert.False(t, result.SupportsMCP, "must NOT fabricate a capability the real server did not report")
	assert.False(t, result.SupportsLSP)
	assert.False(t, result.SupportsACP)
	assert.False(t, result.SupportsRAG)
	assert.False(t, result.SupportsSkills)
	assert.False(t, result.SupportsPlugins)
}

// TestCapabilityAliasFields_ApplyTo_NeverDemotesTrueToFalse proves the alias
// promotion helper is a one-way OR: an absent/false alias never clears an
// already-true primary field decoded from the singular key, and a true alias
// never gets ignored just because the primary field happened to decode false.
func TestCapabilityAliasFields_ApplyTo_NeverDemotesTrueToFalse(t *testing.T) {
	trueVal := true
	falseVal := false

	// Case 1: primary already true, alias absent (nil) -> stays true.
	mcp, lsp, acp := true, true, true
	capabilityAliasFields{}.applyTo(&mcp, &lsp, &acp)
	assert.True(t, mcp)
	assert.True(t, lsp)
	assert.True(t, acp)

	// Case 2: primary already true, alias explicitly false -> stays true
	// (never demotes a verified "true" back to "false").
	mcp, lsp, acp = true, true, true
	capabilityAliasFields{
		SupportsMCPAlias: &falseVal,
		SupportsLSPAlias: &falseVal,
		SupportsACPAlias: &falseVal,
	}.applyTo(&mcp, &lsp, &acp)
	assert.True(t, mcp)
	assert.True(t, lsp)
	assert.True(t, acp)

	// Case 3: primary false, alias true -> promotes to true.
	mcp, lsp, acp = false, false, false
	capabilityAliasFields{
		SupportsMCPAlias: &trueVal,
		SupportsLSPAlias: &trueVal,
		SupportsACPAlias: &trueVal,
	}.applyTo(&mcp, &lsp, &acp)
	assert.True(t, mcp)
	assert.True(t, lsp)
	assert.True(t, acp)

	// Case 4: both false/absent -> stays honest false.
	mcp, lsp, acp = false, false, false
	capabilityAliasFields{}.applyTo(&mcp, &lsp, &acp)
	assert.False(t, mcp)
	assert.False(t, lsp)
	assert.False(t, acp)
}
