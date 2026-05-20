package llm

import (
	"encoding/json"
	"log"

	"dev.helix.code/internal/llm/promptcache"
)

// provider_cache.go — speed programme P1-T05.
//
// P1-T04 wired Anthropic `cache_control` ephemeral breakpoints + the
// promptcache prefix-stability detector. P1-T05 extends prompt-caching
// support to the remaining cache-capable providers:
//
//   - OpenAI   — implicit/automatic prompt caching. There is NO explicit
//                request flag; the provider transparently caches a prompt
//                prefix once it exceeds ~1024 tokens and reuses it when the
//                NEXT request shares a byte-identical prefix. The win is
//                therefore entirely a function of prefix stability — exactly
//                what the promptcache package guarantees. OpenAI reports a
//                cache hit in `usage.prompt_tokens_details.cached_tokens`.
//   - DeepSeek — context caching on disk, also implicit/automatic (no request
//                flag). DeepSeek reports the hit/miss split with its own
//                response fields `usage.prompt_cache_hit_tokens` and
//                `usage.prompt_cache_miss_tokens`.
//   - Gemini   — Gemini 2.5 performs implicit context caching automatically
//                for a stable prefix and reports it in
//                `usageMetadata.cachedContentTokenCount` (already parsed by
//                gemini_provider.go). A separate explicit `cachedContents`
//                resource API also exists but is a heavyweight,
//                lifecycle-managed resource out of scope for the per-request
//                fast path; implicit caching is the zero-config win.
//
// Providers with NO prompt-caching support are deliberately left untouched
// (see the cacheCapableProviders documentation below).
//
// CRITICAL no-regression invariant (CONST-035 / Article XI §11.9): NONE of
// the helpers in this file mutate an outbound request body, header, auth, or
// endpoint. Implicit-caching providers expose no request flag, so there is
// nothing to add to the wire. The only behaviours added are (a) parsing extra
// RESPONSE usage fields that the provider already sends, and (b) observing
// prefix drift. Both are read-only with respect to the request.

// cacheCapableProviders documents, for the record and for the P1-T05 tests,
// exactly which providers received prompt-caching wiring and which were
// intentionally left untouched because their API has no caching support.
//
// Wired (cache-capable):
//   - ProviderTypeAnthropic — explicit cache_control (done in P1-T04)
//   - ProviderTypeOpenAI    — implicit caching, cached_tokens metric
//   - ProviderTypeDeepSeek  — implicit context caching, hit/miss metric
//   - ProviderTypeGemini    — implicit context caching, cachedContent metric
//
// Left untouched (no provider-side prompt caching, or OpenAI-compatible
// shells whose upstream does not document caching): Ollama and Llama.cpp
// (local inference — no remote prompt cache), Groq, Cerebras, xAI/Grok,
// Mistral, OpenRouter, Copilot, Azure OpenAI, AWS Bedrock. These keep their
// request paths byte-identical — P1-T05 changes nothing for them. If any of
// these later documents prompt caching, extend this map and wire it then.
var cacheCapableProviders = map[ProviderType]bool{
	ProviderTypeAnthropic: true,
	ProviderTypeOpenAI:    true,
	ProviderTypeDeepSeek:  true,
	ProviderTypeGemini:    true,
}

// providerSupportsPromptCache reports whether the given provider type has any
// prompt-caching wiring under the speed programme. Used by tests and callers
// that want to branch on caching support without hardcoding the list.
func providerSupportsPromptCache(t ProviderType) bool {
	return cacheCapableProviders[t]
}

// openAICacheUsageFields is the superset of cache-related usage fields emitted
// by OpenAI and OpenAI-compatible caching providers. Embedding it into the
// shared OpenAIResponse usage struct lets a single response type carry both
// the OpenAI shape (`prompt_tokens_details.cached_tokens`) and the DeepSeek
// shape (`prompt_cache_hit_tokens` / `prompt_cache_miss_tokens`).
//
// Every field is `omitempty`-irrelevant because this struct only ever appears
// on the RESPONSE side — it is decoded, never marshalled into a request.
// A provider that does not send a given field simply leaves it zero.
type openAICacheUsageFields struct {
	// PromptTokensDetails is OpenAI's nested cache breakdown. cached_tokens
	// is the count of prompt tokens served from the implicit cache.
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`

	// PromptCacheHitTokens / PromptCacheMissTokens are DeepSeek's flat
	// cache-accounting fields (DeepSeek context caching on disk).
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens,omitempty"`
}

// cachedPromptTokens returns the number of prompt tokens served from the
// provider cache for this response, normalising across the OpenAI and DeepSeek
// field shapes. Returns 0 when neither shape reported a hit.
func (u openAICacheUsageFields) cachedPromptTokens() int {
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		return u.PromptTokensDetails.CachedTokens
	}
	return u.PromptCacheHitTokens
}

// cacheMetadata builds the ProviderMetadata map fragment describing a prompt
// cache hit for an OpenAI-compatible provider response. Returns nil when no
// cache hit was reported, so callers can leave ProviderMetadata unset (and the
// response stays byte-identical to the pre-P1-T05 behaviour for a cache miss).
func (u openAICacheUsageFields) cacheMetadata() map[string]interface{} {
	hit := u.cachedPromptTokens()
	if hit <= 0 {
		return nil
	}
	meta := map[string]interface{}{
		"cached_prompt_tokens": hit,
	}
	if u.PromptCacheMissTokens > 0 {
		meta["cache_miss_prompt_tokens"] = u.PromptCacheMissTokens
	}
	return meta
}

// trackPromptCachePrefixGeneric records or verifies the prompt-cache prefix
// for an implicit-caching provider's session, mirroring
// AnthropicProvider.trackPromptCachePrefix.
//
// On the first call it freezes the prefix (system prompt + tool definitions)
// as the session baseline; on later calls it checks the live prefix against
// the baseline and logs a "cache break" warning if it drifted — implicit
// caching silently misses for the rest of the session once the prefix moves.
//
// This is observability ONLY: it never mutates the request, so feature
// behaviour is unaffected for every provider and every caller. Errors are
// swallowed — prefix tracking is an optimization concern and MUST NEVER fail
// a request (CONST-035 / Article XI §11.9: a non-functional optimisation is
// preferable to a broken feature).
func trackPromptCachePrefixGeneric(
	detector *promptcache.CacheBreakDetector,
	providerName string,
	systemPrompt string,
	tools []Tool,
) {
	if detector == nil {
		return
	}
	prefixTools := make([]interface{}, len(tools))
	for i := range tools {
		// Canonicalize each tool definition so the prefix hash is byte-stable
		// regardless of Go map-iteration order (the same pitfall P1-T04
		// closed for Anthropic). A nil/failed canonicalization falls back to
		// the raw tool — still correct, just potentially cache-missing.
		prefixTools[i] = canonicalizeToolForPrefix(tools[i])
	}
	prefix := promptcache.PrefixComponents{
		SystemPrompt: systemPrompt,
		Tools:        prefixTools,
	}
	if !detector.IsFrozen() {
		if _, err := detector.Freeze(prefix); err != nil {
			log.Printf("%s: prompt-cache prefix freeze failed: %v", providerName, err)
		}
		return
	}
	res, err := detector.Check(prefix)
	if err != nil {
		log.Printf("%s: prompt-cache prefix check failed: %v", providerName, err)
		return
	}
	if res.Broken {
		log.Printf("%s: %s", providerName, res.Reason)
	}
}

// canonicalizeToolForPrefix returns a deterministically-ordered representation
// of a tool definition suitable for prompt-cache-prefix hashing. It normalises
// the JSON-Schema parameter map (sorted keys + sorted set-like string arrays)
// so two logically-identical tool sets hash equal regardless of how Go ranged
// over the underlying maps. On any error it returns the original tool — the
// prefix may then mis-hash and miss the cache, but the request is never broken.
func canonicalizeToolForPrefix(tool Tool) interface{} {
	if tool.Function.Parameters == nil {
		return tool
	}
	canon, err := promptcache.CanonicalJSONSorted(tool.Function.Parameters)
	if err != nil {
		return tool
	}
	var normalized map[string]interface{}
	if err := json.Unmarshal(canon, &normalized); err != nil {
		return tool
	}
	// Return a structurally-stable representation: the function name +
	// description (order-fixed by struct field order) plus the normalised
	// parameter schema.
	return map[string]interface{}{
		"name":        tool.Function.Name,
		"description": tool.Function.Description,
		"parameters":  normalized,
	}
}
