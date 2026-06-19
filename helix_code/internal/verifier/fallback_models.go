package verifier

// FallbackModels is the hardcoded fallback list used when the LLMsVerifier
// service is unavailable. This is the ONLY permitted hardcoded model list
// in the entire codebase (CONST-035 compliance).
//
// When the verifier is unreachable (circuit breaker open, network failure,
// or verifier not yet deployed), these 7 models provide a safe baseline
// so that HelixCode remains functional.
//
// Authority: CONST-035 — End-User Usability Mandate
//            CONST-037 — LLMsVerifier Single Source of Truth Mandate
//
// Rules:
//   - This list MUST NOT exceed 8 entries.
//   - Every entry MUST have a real, working public API endpoint.
//   - Scores MUST be realistic (range 0–10) and reflect known performance.
//   - Local models MUST be marked OpenSource=true.
//   - Updates require constitutional review (CONST amendment process).
var FallbackModels = []*VerifiedModel{
	{ID: "llama-3.2-3b", Name: "Llama 3.2 3B", DisplayName: "Llama 3.2 3B", Provider: "ollama", ContextSize: 131072, Source: "fallback", OverallScore: 6.0, Tier: 3, OpenSource: true},
	{ID: "gpt-4o", Name: "GPT-4o", DisplayName: "GPT-4o", Provider: "openai", ContextSize: 128000, Source: "fallback", OverallScore: 9.1, Tier: 1},
	{ID: "claude-3-5-sonnet", Name: "Claude 3.5 Sonnet", DisplayName: "Claude 3.5 Sonnet", Provider: "anthropic", ContextSize: 200000, Source: "fallback", OverallScore: 8.9, Tier: 1},
	{ID: "mistral-large", Name: "Mistral Large", DisplayName: "Mistral Large", Provider: "mistral", ContextSize: 128000, Source: "fallback", OverallScore: 7.8, Tier: 2},
	{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", DisplayName: "Gemini 2.5 Pro", Provider: "gemini", ContextSize: 1000000, Source: "fallback", OverallScore: 8.7, Tier: 1},
	{ID: "deepseek-chat", Name: "DeepSeek Chat", DisplayName: "DeepSeek Chat", Provider: "deepseek", ContextSize: 64000, Source: "fallback", OverallScore: 8.3, Tier: 2, OpenSource: true},
	{ID: "grok-3-fast-beta", Name: "Grok-3 Fast Beta", DisplayName: "Grok-3 Fast Beta", Provider: "xai", ContextSize: 131072, Source: "fallback", OverallScore: 8.0, Tier: 1},
	{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro", DisplayName: "Xiaomi MiMo V2.5 Pro", Provider: "xiaomi", ContextSize: 1000000, Source: "fallback", OverallScore: 8.1, Tier: 1},
}

// makeDefaultProviderScores returns the default provider score map used by
// the embedded verifier server and test fixtures.
func makeDefaultProviderScores() map[string]float64 {
	return map[string]float64{
		"openai":    9.1,
		"anthropic": 8.9,
		"ollama":    6.0,
		"mistral":   7.8,
		"gemini":    8.7,
		"deepseek":  8.3,
		"xai":       8.0,
		"xiaomi":    8.1,
	}
}
