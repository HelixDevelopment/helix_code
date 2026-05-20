package session

// Cache pre-warming at session start (speed programme P1-T06).
//
// When a session opens, the FIRST real user request would otherwise always pay
// the full cold-cache TTFT penalty: provider-side prompt caching only hits on
// the second-and-later request because the first request is what WRITES the
// cache entry. PreWarmer removes that penalty by firing one minimal,
// near-zero-completion request carrying the stable system+tools prefix the
// instant the session opens — before the user has typed anything. The
// provider caches the prefix as a side effect, so the user's first real
// request arrives as a cache HIT.
//
// # Hard no-regression contract
//
// Pre-warming is PURELY ADDITIVE and BEST-EFFORT. It MUST NOT:
//   - block session-open (dispatch is asynchronous on its own goroutine);
//   - change any user-visible behaviour;
//   - turn a provider error, a missing provider, or a provider without
//     caching into a session-open failure.
//
// If pre-warm cannot run or fails for ANY reason the session opens exactly as
// it does today and behaves identically. Pre-warm is an optimisation, never a
// dependency.

import (
	"context"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/promptcache"
)

// prewarmDispatchTimeout bounds a single pre-warm request. A pre-warm that
// hangs must never leak a goroutine for the session's lifetime, and a slow
// pre-warm provides no value (the user's first real request would arrive
// before the cache is written anyway). The timeout is generous enough for a
// normal provider round-trip but firmly bounded.
const prewarmDispatchTimeout = 20 * time.Second

// PreWarmProvider is the minimal slice of llm.Provider that the pre-warmer
// needs. Declaring it locally (rather than depending on the whole
// llm.Provider surface) keeps the pre-warm hook decoupled and trivially
// unit-testable with a tiny fake — the fake lives in the unit test, never in
// production code (CONST-050(A)).
type PreWarmProvider interface {
	// Generate dispatches a request to the provider. The pre-warmer uses it
	// for exactly one throwaway minimal request per session open.
	Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error)
	// IsAvailable reports whether the provider can currently serve requests.
	// A pre-warm against an unavailable provider is skipped silently.
	IsAvailable(ctx context.Context) bool
}

// PreWarmResult records the outcome of one pre-warm attempt. It exists for
// observability and tests; the session-open path does not depend on it (a
// pre-warm that produced a Skipped or Errored result still leaves the session
// fully functional).
type PreWarmResult struct {
	// Attempted is true when a warm request was actually dispatched (i.e.
	// pre-warm was enabled and a provider was available).
	Attempted bool
	// Skipped is true when pre-warm was a deliberate no-op — caching
	// disabled, nil provider, provider unavailable, or empty prefix.
	Skipped bool
	// SkipReason explains a Skipped result (empty otherwise).
	SkipReason string
	// CacheWritten is true when the warm response reported the provider
	// wrote cache-creation tokens (cache_creation_tokens > 0). This is the
	// positive runtime evidence that the warm-up actually primed the cache.
	CacheWritten bool
	// Err carries a non-fatal pre-warm dispatch error. It is recorded for
	// observability ONLY — it is never propagated to the session-open caller.
	Err error
}

// PreWarmConfig configures one pre-warm attempt.
type PreWarmConfig struct {
	// Enabled gates the whole mechanism. When false, PreWarm is a no-op that
	// returns immediately with a Skipped result. Callers wire this from the
	// active provider's prompt-cache setting, so a provider without caching
	// never triggers a pre-warm.
	Enabled bool
	// Model is the model id the warm request targets — it MUST be the same
	// model the session's real requests will use, so the provider keys the
	// cache entry the real requests will look up.
	Model string
	// Prefix is the stable cacheable prefix (system prompt + tool set) the
	// session's real requests will carry. The warm request carries exactly
	// this prefix so its bytes hash identically.
	Prefix promptcache.PrefixComponents
	// Provider dispatches the warm request. nil disables pre-warm silently.
	Provider PreWarmProvider
}

// warmRequestToLLM converts a provider-neutral promptcache.WarmRequest into a
// concrete llm.LLMRequest. The conversion lives here (not in promptcache)
// because promptcache must not import internal/llm — that would be an import
// cycle (anthropic_provider.go imports promptcache).
//
// The produced request:
//   - asks for MinWarmTokens completion tokens (near-zero — the request
//     exists to cache the prefix, not to generate);
//   - is non-streaming (a streamed single token has no value);
//   - carries the warm system prompt and the minimal warm user message;
//   - leaves Temperature at the zero value for determinism.
//
// Tool definitions: the warm request's Tools are the cache-prefix tool set,
// but llm.LLMRequest.Tools is a typed []llm.Tool. Because the prefix hash that
// matters for the cache is computed by the provider over the SERIALIZED system
// + tools blocks, and HelixCode's request-builder path (P1-T04) feeds the same
// promptcache prefix into both warm and real requests, the warm request
// carries the system prompt — the dominant prefix component — verbatim. Tool
// blocks are attached by the provider's own cache-prefix assembly using the
// session's frozen tool set, so the warm and real prefixes converge.
func warmRequestToLLM(model string, w promptcache.WarmRequest) *llm.LLMRequest {
	return &llm.LLMRequest{
		Model:     model,
		MaxTokens: w.MaxTokens,
		Stream:    false,
		Messages: []llm.Message{
			{Role: "system", Content: w.SystemPrompt},
			{Role: "user", Content: w.WarmUserMessage},
		},
	}
}

// responseCacheWritten reports whether an LLM response indicates the provider
// wrote a prompt-cache entry. The Anthropic provider surfaces
// cache_creation_tokens in ProviderMetadata; other providers surface
// equivalent keys. A positive value is the runtime proof that the warm-up
// actually primed the cache.
func responseCacheWritten(resp *llm.LLMResponse) bool {
	if resp == nil || resp.ProviderMetadata == nil {
		return false
	}
	for _, key := range []string{"cache_creation_tokens", "cache_creation_input_tokens"} {
		if v, ok := resp.ProviderMetadata[key]; ok && asPositiveInt(v) {
			return true
		}
	}
	return false
}

// responseCacheRead reports whether an LLM response was served from the
// provider's prompt cache (cache_read_tokens > 0). The session-open path does
// not use this — it is exposed for the integration test that proves the first
// real request after a pre-warm is a cache hit.
func responseCacheRead(resp *llm.LLMResponse) bool {
	if resp == nil || resp.ProviderMetadata == nil {
		return false
	}
	for _, key := range []string{"cache_read_tokens", "cache_read_input_tokens"} {
		if v, ok := resp.ProviderMetadata[key]; ok && asPositiveInt(v) {
			return true
		}
	}
	return false
}

// asPositiveInt reports whether v holds a strictly-positive integer-ish value.
// ProviderMetadata is map[string]interface{}, so the token count may arrive as
// int, int64, float64 (JSON-decoded), etc.
func asPositiveInt(v interface{}) bool {
	switch n := v.(type) {
	case int:
		return n > 0
	case int32:
		return n > 0
	case int64:
		return n > 0
	case float64:
		return n > 0
	case float32:
		return n > 0
	}
	return false
}

// PreWarm dispatches one best-effort pre-warm request for cfg, SYNCHRONOUSLY.
// It is the testable core of the mechanism; production session-open code calls
// PreWarmAsync, which wraps this on its own goroutine so session-open never
// blocks.
//
// PreWarm NEVER returns an error: every failure mode (caching disabled, nil
// provider, unavailable provider, empty prefix, dispatch error) is folded into
// the PreWarmResult. The caller may inspect the result for observability but
// MUST NOT treat it as a session-open precondition.
func PreWarm(ctx context.Context, cfg PreWarmConfig) PreWarmResult {
	if !cfg.Enabled {
		return PreWarmResult{Skipped: true, SkipReason: "prompt caching disabled for provider"}
	}
	if cfg.Provider == nil {
		return PreWarmResult{Skipped: true, SkipReason: "no provider configured"}
	}
	// An empty prefix has nothing to cache — warming it would be a pointless
	// billable call.
	if cfg.Prefix.SystemPrompt == "" && len(cfg.Prefix.Tools) == 0 {
		return PreWarmResult{Skipped: true, SkipReason: "empty cache prefix"}
	}
	if !cfg.Provider.IsAvailable(ctx) {
		return PreWarmResult{Skipped: true, SkipReason: "provider unavailable"}
	}

	warm := promptcache.BuildWarmRequest(cfg.Prefix)
	// Defensive: a warm request that is not minimal would generate a real
	// billable completion — never dispatch it.
	if !warm.IsMinimal() {
		return PreWarmResult{Skipped: true, SkipReason: "warm request not minimal"}
	}

	dctx, cancel := context.WithTimeout(ctx, prewarmDispatchTimeout)
	defer cancel()

	resp, err := cfg.Provider.Generate(dctx, warmRequestToLLM(cfg.Model, warm))
	if err != nil {
		// Best-effort: a failed pre-warm is swallowed. The session opened
		// fine; the first real request simply pays the cold-cache price as
		// it does today — no regression.
		return PreWarmResult{Attempted: true, Err: err}
	}
	return PreWarmResult{Attempted: true, CacheWritten: responseCacheWritten(resp)}
}

// PreWarmAsync fires PreWarm on a background goroutine so session-open returns
// immediately. This is the function the session-open path calls.
//
// The returned channel delivers exactly one PreWarmResult when the background
// pre-warm completes, then is closed. Production session-open code IGNORES the
// channel entirely (pre-warm is fire-and-forget); tests read it to assert the
// outcome. The goroutine cannot outlive prewarmDispatchTimeout, so no leak.
//
// A panic inside the background pre-warm (e.g. a misbehaving provider) is
// recovered so it can never crash the host process — pre-warm must never be
// able to take the session down.
func PreWarmAsync(ctx context.Context, cfg PreWarmConfig) <-chan PreWarmResult {
	done := make(chan PreWarmResult, 1)
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				done <- PreWarmResult{Attempted: true, Err: errorFromRecover(r)}
			}
		}()
		done <- PreWarm(ctx, cfg)
	}()
	return done
}

// errorFromRecover normalises a recovered panic value into an error so it can
// be stored in PreWarmResult.Err.
func errorFromRecover(r interface{}) error {
	if err, ok := r.(error); ok {
		return err
	}
	return &prewarmPanicError{value: r}
}

// prewarmPanicError wraps a non-error panic value recovered inside a
// background pre-warm goroutine.
type prewarmPanicError struct{ value interface{} }

func (e *prewarmPanicError) Error() string {
	return "prewarm: recovered panic in background pre-warm goroutine"
}
