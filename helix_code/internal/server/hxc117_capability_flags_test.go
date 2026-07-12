// HXC-117 (Bug/High) — §11.4.115 RED->GREEN guard for the server REST
// surface. verifier.VerifiedModel decodes 6 CONST-040 capability flags
// (MCP/LSP/ACP/RAG/Skills/Plugins), but verifiedModelToJSON (the
// /api/v1/llm/models JSON consumed by desktop/terminal-ui/mobile/external
// clients) never emitted them. This proves the flags reach the JSON
// payload sourced from the decoded VerifiedModel fields -- never a
// hardcoded value (CONST-040 / BLUFF-002).
//
// Mocks/fakes NOT used -- verifiedModelToJSON is a pure function over a
// *verifier.VerifiedModel value; no infrastructure required (CONST-050(A)
// unit test).
package server

import (
	"testing"

	"dev.helix.code/internal/verifier"
)

// hxc117MixedCapabilityModel is a fixture with a MIXED true/false CONST-040
// capability profile so a hardcoded/constant JSON emitter cannot pass this
// test.
func hxc117MixedCapabilityModel() *verifier.VerifiedModel {
	return &verifier.VerifiedModel{
		ID:                 "hxc117-server-model",
		DisplayName:        "HXC-117 Server Test Model",
		Provider:           "testprovider",
		Verified:           true,
		VerificationStatus: "verified",
		OverallScore:       8.4,
		ContextSize:        64000,
		MaxOutputTokens:    4096,
		Tier:               1,
		SupportsVision:     true,
		SupportsTools:      true,
		SupportsMCP:        true,
		SupportsLSP:        false,
		SupportsACP:        true,
		SupportsRAG:        false,
		SupportsSkills:     true,
		SupportsPlugins:    false,
	}
}

// TestHXC117_Server_VerifiedModelToJSON_IncludesCapabilityFlags is the
// RED->GREEN guard for the server JSON surface. It asserts the 6 CONST-040
// capability keys are present in the emitted gin.H map AND match the
// fixture's mixed true/false profile, and that pre-existing fields
// (supports_vision / supports_tools) still render (no regression).
func TestHXC117_Server_VerifiedModelToJSON_IncludesCapabilityFlags(t *testing.T) {
	m := hxc117MixedCapabilityModel()
	got := verifiedModelToJSON(m)

	// Regression guard: pre-existing fields must still be present.
	if v, ok := got["supports_vision"].(bool); !ok || v != m.SupportsVision {
		t.Fatalf("supports_vision missing or wrong: got %#v want %v", got["supports_vision"], m.SupportsVision)
	}
	if v, ok := got["supports_tools"].(bool); !ok || v != m.SupportsTools {
		t.Fatalf("supports_tools missing or wrong: got %#v want %v", got["supports_tools"], m.SupportsTools)
	}

	cases := []struct {
		key  string
		want bool
	}{
		{"supports_mcp", m.SupportsMCP},
		{"supports_lsp", m.SupportsLSP},
		{"supports_acp", m.SupportsACP},
		{"supports_rag", m.SupportsRAG},
		{"supports_skills", m.SupportsSkills},
		{"supports_plugins", m.SupportsPlugins},
	}
	for _, c := range cases {
		raw, present := got[c.key]
		if !present {
			t.Fatalf("HXC-117 RED: verifiedModelToJSON output is missing key %q entirely; full map: %#v", c.key, got)
		}
		v, ok := raw.(bool)
		if !ok {
			t.Fatalf("key %q is not a bool: %#v", c.key, raw)
		}
		if v != c.want {
			t.Fatalf("key %q = %v, want %v (fixture value) -- must be sourced from the decoded model field, not hardcoded", c.key, v, c.want)
		}
	}

	// Anti-bluff: flip every capability flag and confirm the JSON output
	// changes accordingly (proves the emitter reads the live field rather
	// than a hardcoded constant -- CONST-040 / BLUFF-002 check).
	flipped := hxc117MixedCapabilityModel()
	flipped.SupportsMCP = !flipped.SupportsMCP
	flipped.SupportsLSP = !flipped.SupportsLSP
	flipped.SupportsACP = !flipped.SupportsACP
	flipped.SupportsRAG = !flipped.SupportsRAG
	flipped.SupportsSkills = !flipped.SupportsSkills
	flipped.SupportsPlugins = !flipped.SupportsPlugins
	flippedGot := verifiedModelToJSON(flipped)
	for _, c := range cases {
		if flippedGot[c.key] == got[c.key] {
			t.Fatalf("HXC-117: key %q did not change after flipping the fixture's capability flag -- emitter is not sourcing from the decoded field (hardcoded/BLUFF-002 pattern)", c.key)
		}
	}
}
