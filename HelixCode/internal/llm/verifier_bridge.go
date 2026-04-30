package llm

import (
	"strings"

	"dev.helix.code/internal/verifier"
)

// verifierAdapter is a package-level reference to the verifier subsystem.
// It is set during application startup (server or CLI) via SetVerifierAdapter.
var verifierAdapter *verifier.Adapter

// SetVerifierAdapter injects the verifier adapter into the llm package.
// This MUST be called during application initialization before any provider
// GetModels() calls are made.
func SetVerifierAdapter(adapter *verifier.Adapter) {
	verifierAdapter = adapter
}

// EnrichModelInfo populates model capabilities, vision support, and tool
// support from the verifier when available. If the verifier is disabled or
// unreachable, it falls back to capability inference from the model ID.
//
// This function is the single point where model metadata transitions from
// provider-defined to verifier-authoritative (CONST-036 / CONST-041).
func EnrichModelInfo(mi *ModelInfo) {
	if mi == nil {
		return
	}

	// Ensure ID is set (some providers only set Name)
	if mi.ID == "" && mi.Name != "" {
		mi.ID = mi.Name
	}

	// Priority 1: LLMsVerifier data
	if verifierAdapter != nil && verifierAdapter.IsEnabled() {
		score, found := verifierAdapter.GetModelScore(mi.ID)
		if found {
			if mi.Metadata == nil {
				mi.Metadata = make(map[string]interface{})
			}
			mi.Metadata["verifier_score"] = score
		}

		codeScore, hasCode := verifierAdapter.GetModelCodeCapabilityScore(mi.ID)
		if hasCode && codeScore > 5.0 {
			mi.Capabilities = ensureCapability(mi.Capabilities, CapabilityCodeGeneration)
			mi.Capabilities = ensureCapability(mi.Capabilities, CapabilityCodeAnalysis)
		}

		relScore, hasRel := verifierAdapter.GetModelReliabilityScore(mi.ID)
		if hasRel && relScore > 5.0 {
			mi.Capabilities = ensureCapability(mi.Capabilities, CapabilityReasoning)
		}

		// Tool support from verifier score heuristic
		if score > 7.0 {
			mi.SupportsTools = true
		}

		return
	}

	// Priority 2: Capability inference from model ID (fallback when verifier off)
	inferCapabilitiesFromModelID(mi)
}

// inferCapabilitiesFromModelID adds capabilities based on model name heuristics.
// This is ONLY used when the verifier is unavailable and MUST NOT be used
// as a substitute for verifier data in production.
func inferCapabilitiesFromModelID(mi *ModelInfo) {
	name := mi.ID + " " + mi.Name
	lower := strings.ToLower(name)

	if strings.Contains(lower, "vision") || strings.Contains(lower, "4o") ||
		strings.Contains(lower, "claude-3") || strings.Contains(lower, "gemini-pro-vision") {
		mi.SupportsVision = true
	}

	if strings.Contains(lower, "code") || strings.Contains(lower, "coder") {
		mi.Capabilities = ensureCapability(mi.Capabilities, CapabilityCodeGeneration)
		mi.Capabilities = ensureCapability(mi.Capabilities, CapabilityCodeAnalysis)
	}

	if strings.Contains(lower, "reason") {
		mi.Capabilities = ensureCapability(mi.Capabilities, CapabilityReasoning)
	}

	// Tool support inference from known tool-capable model families
	if strings.Contains(lower, "gpt-4") || strings.Contains(lower, "claude-3") ||
		strings.Contains(lower, "gemini-1.5") || strings.Contains(lower, "gemini-2") ||
		strings.Contains(lower, "mistral-large") || strings.Contains(lower, "command-r-plus") ||
		strings.Contains(lower, "llama-3.3") || strings.Contains(lower, "qwen2.5") {
		mi.SupportsTools = true
	}

	// All models support text generation at minimum
	mi.Capabilities = ensureCapability(mi.Capabilities, CapabilityTextGeneration)
}

func ensureCapability(caps []ModelCapability, cap ModelCapability) []ModelCapability {
	for _, c := range caps {
		if c == cap {
			return caps
		}
	}
	return append(caps, cap)
}
