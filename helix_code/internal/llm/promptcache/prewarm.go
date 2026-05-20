package promptcache

// Cache pre-warming (speed programme P1-T06).
//
// # Why pre-warming exists
//
// Provider-side prompt caching (Anthropic `cache_control` ephemeral
// breakpoints and equivalents) only pays off on the SECOND and later request
// in a session: the first request always pays the full cold-cache price
// because the provider has not yet written the prefix into its cache. The
// first real user request therefore eats the entire cold-cache TTFT penalty.
//
// Pre-warming removes that penalty: at session open, before the user has typed
// anything, HelixCode fires ONE minimal request that carries the stable
// system+tools prefix and asks for a near-zero number of completion tokens.
// The provider writes the prefix into its cache as a side effect. The user's
// FIRST real request then arrives as a cache HIT.
//
// # Design contract
//
// This file is provider-agnostic on purpose. `promptcache` is imported BY
// `internal/llm` (anthropic_provider.go), so it MUST NOT import `internal/llm`
// back — that would be an import cycle. The helper here therefore produces a
// neutral WarmRequest VALUE describing what the warm request must contain; the
// session-open hook in `internal/session` (which may import `internal/llm`)
// translates that value into a concrete provider `LLMRequest` and dispatches
// it. The translation layer lives in `internal/session/prewarm.go`.
//
// All functions here are pure and deterministic.

// MinWarmTokens is the completion-token budget for a pre-warm request. It is
// deliberately the smallest value every supported provider accepts (Anthropic
// and OpenAI both reject max_tokens=0, so 1 is the floor). The pre-warm
// request's purpose is to make the provider HASH AND CACHE the prefix, not to
// produce useful output — so the completion is squeezed to a single token.
const MinWarmTokens = 1

// WarmRequest is a provider-neutral description of the minimal request that
// pre-warms a provider's prompt cache. It is produced by BuildWarmRequest from
// a session's stable PrefixComponents and consumed by the session-open hook,
// which converts it into a concrete provider request.
//
// A WarmRequest carries exactly the stable cacheable prefix (system prompt +
// tool definitions) and nothing turn-specific: the whole point is that the
// bytes the provider hashes here are byte-identical to the bytes it will hash
// on the user's first real request, so that request hits the cache.
type WarmRequest struct {
	// SystemPrompt is the stable system instruction text — identical to the
	// SystemPrompt the real requests in this session will carry.
	SystemPrompt string

	// Tools is the ordered, stable tool-definition set — identical to the
	// tool set the real requests in this session will carry. Order matters
	// to the cache hash (see PrefixComponents.Tools).
	Tools []interface{}

	// MaxTokens is the completion-token budget — always MinWarmTokens. The
	// warm request wants the prefix cached, not a useful completion.
	MaxTokens int

	// WarmUserMessage is a minimal, content-free user turn. A provider
	// request needs at least one message; this is the smallest one that
	// still lets the provider accept the request and cache the prefix. It is
	// intentionally tiny and is NOT part of the cacheable prefix, so its
	// content does not affect the prefix hash the next request must match.
	WarmUserMessage string
}

// PrefixHash returns the hash of this warm request's cacheable prefix. It is
// byte-identical to PrefixComponents{SystemPrompt, Tools}.Hash() for the same
// prefix — so a caller can assert that the warm request and the first real
// request will hash the same prefix (and therefore the first real request
// will hit the cache the warm request created).
func (w WarmRequest) PrefixHash() (string, error) {
	return PrefixComponents{SystemPrompt: w.SystemPrompt, Tools: w.Tools}.Hash()
}

// minimalWarmUserMessage is the smallest non-empty user-turn content. It is a
// single dot: providers require a non-empty message, and a one-character body
// minimises the non-cached tail of the request. It is NOT user-facing text
// (CONST-046 does not apply — it is never rendered to a user; it exists only
// to satisfy the provider's "at least one message" requirement during a
// throwaway warm-up call).
const minimalWarmUserMessage = "."

// BuildWarmRequest constructs the minimal pre-warm request for the given
// stable prefix.
//
// Guarantees:
//   - MaxTokens is exactly MinWarmTokens (the request asks for near-zero
//     output — it exists to cache the prefix, not to generate);
//   - SystemPrompt and Tools are copied verbatim from p, so the warm request's
//     prefix hashes IDENTICALLY to every real request that carries the same
//     prefix — which is the whole mechanism: the warm call writes the cache
//     entry, the first real call reads it.
//
// BuildWarmRequest never fails: it is a pure field copy. Validity of the
// prefix (e.g. tool serializability) is checked lazily by PrefixHash.
func BuildWarmRequest(p PrefixComponents) WarmRequest {
	tools := make([]interface{}, len(p.Tools))
	copy(tools, p.Tools)
	return WarmRequest{
		SystemPrompt:    p.SystemPrompt,
		Tools:           tools,
		MaxTokens:       MinWarmTokens,
		WarmUserMessage: minimalWarmUserMessage,
	}
}

// IsMinimal reports whether w asks for the minimal completion budget. The
// session-open hook asserts this before dispatch so a mis-built warm request
// (one that would generate a real, billable completion) is caught rather than
// silently sent. A warm request that is not minimal is a defect — pre-warming
// must be cheap.
func (w WarmRequest) IsMinimal() bool {
	return w.MaxTokens == MinWarmTokens
}
