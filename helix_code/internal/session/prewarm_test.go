package session

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/promptcache"
)

// ---------------------------------------------------------------------------
// Unit-test fakes. CONST-050(A): mocks/fakes are permitted ONLY in unit tests
// (this *_test.go file invoked without the integration build tag). The
// integration test below uses the REAL llm.AnthropicProvider against a
// controlled HTTP shim — no fakes there.
// ---------------------------------------------------------------------------

// fakePreWarmProvider is a minimal in-memory PreWarmProvider for unit tests.
type fakePreWarmProvider struct {
	available  bool
	generateFn func(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error)
	calls      int32
	lastReq    *llm.LLMRequest
}

func (f *fakePreWarmProvider) IsAvailable(ctx context.Context) bool { return f.available }

func (f *fakePreWarmProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	f.lastReq = req
	return f.generateFn(ctx, req)
}

func samplePrefix() promptcache.PrefixComponents {
	return promptcache.PrefixComponents{
		SystemPrompt: "You are HelixCode, an enterprise AI development platform.",
		Tools:        []interface{}{map[string]interface{}{"name": "read_file"}},
	}
}

// TestPreWarm_DispatchesMinimalRequest proves the warm request actually sent
// to the provider is max_tokens-minimal and carries the stable prefix —
// the P1-T06 "warm request is max_tokens-minimal" unit invariant.
func TestPreWarm_DispatchesMinimalRequest(t *testing.T) {
	prov := &fakePreWarmProvider{
		available: true,
		generateFn: func(_ context.Context, _ *llm.LLMRequest) (*llm.LLMResponse, error) {
			return &llm.LLMResponse{
				Content:          "x",
				ProviderMetadata: map[string]interface{}{"cache_creation_tokens": 512},
			}, nil
		},
	}
	prefix := samplePrefix()
	res := PreWarm(context.Background(), PreWarmConfig{
		Enabled:  true,
		Model:    "claude-sonnet-4",
		Prefix:   prefix,
		Provider: prov,
	})

	if !res.Attempted || res.Skipped {
		t.Fatalf("expected an attempted pre-warm, got %+v", res)
	}
	if atomic.LoadInt32(&prov.calls) != 1 {
		t.Fatalf("provider Generate called %d times, want exactly 1", prov.calls)
	}
	if prov.lastReq.MaxTokens != promptcache.MinWarmTokens {
		t.Fatalf("warm request MaxTokens = %d, want minimal (%d)", prov.lastReq.MaxTokens, promptcache.MinWarmTokens)
	}
	if prov.lastReq.Stream {
		t.Fatal("warm request must be non-streaming")
	}
	// The warm request must carry the stable system prompt verbatim so the
	// provider hashes the same prefix the first real request will hash.
	var sawSystem bool
	for _, m := range prov.lastReq.Messages {
		if m.Role == "system" && m.Content == prefix.SystemPrompt {
			sawSystem = true
		}
	}
	if !sawSystem {
		t.Fatal("warm request did not carry the stable system prompt")
	}
	if !res.CacheWritten {
		t.Fatal("expected CacheWritten=true from cache_creation_tokens>0")
	}
}

// TestPreWarm_FailureSwallowed proves a provider error during pre-warm is
// swallowed: PreWarm returns a result (never an error / panic), so session-
// open still succeeds. The P1-T06 no-regression invariant.
func TestPreWarm_FailureSwallowed(t *testing.T) {
	prov := &fakePreWarmProvider{
		available: true,
		generateFn: func(_ context.Context, _ *llm.LLMRequest) (*llm.LLMResponse, error) {
			return nil, errors.New("network unreachable")
		},
	}
	res := PreWarm(context.Background(), PreWarmConfig{
		Enabled: true, Model: "m", Prefix: samplePrefix(), Provider: prov,
	})
	if res.Err == nil {
		t.Fatal("expected the dispatch error to be recorded in PreWarmResult.Err")
	}
	if !res.Attempted {
		t.Fatal("a failed dispatch should still report Attempted=true")
	}
	// Crucially: PreWarm returned a value — it did not panic or abort. A
	// caller that ignores the result (production session-open) is unaffected.
}

// TestPreWarm_SkipsWhenDisabled proves pre-warm is a clean no-op when caching
// is disabled — no provider call, session-open behaves exactly as today.
func TestPreWarm_SkipsWhenDisabled(t *testing.T) {
	prov := &fakePreWarmProvider{available: true}
	res := PreWarm(context.Background(), PreWarmConfig{
		Enabled: false, Model: "m", Prefix: samplePrefix(), Provider: prov,
	})
	if !res.Skipped || res.Attempted {
		t.Fatalf("disabled pre-warm should be Skipped, got %+v", res)
	}
	if atomic.LoadInt32(&prov.calls) != 0 {
		t.Fatal("disabled pre-warm must not call the provider")
	}
}

// TestPreWarm_SkipsWhenProviderNil proves a nil provider is a silent skip,
// never a panic.
func TestPreWarm_SkipsWhenProviderNil(t *testing.T) {
	res := PreWarm(context.Background(), PreWarmConfig{
		Enabled: true, Model: "m", Prefix: samplePrefix(), Provider: nil,
	})
	if !res.Skipped {
		t.Fatalf("nil provider should be Skipped, got %+v", res)
	}
}

// TestPreWarm_SkipsWhenProviderUnavailable proves an unavailable provider is a
// silent skip — no dispatch, no error.
func TestPreWarm_SkipsWhenProviderUnavailable(t *testing.T) {
	prov := &fakePreWarmProvider{available: false}
	res := PreWarm(context.Background(), PreWarmConfig{
		Enabled: true, Model: "m", Prefix: samplePrefix(), Provider: prov,
	})
	if !res.Skipped || atomic.LoadInt32(&prov.calls) != 0 {
		t.Fatalf("unavailable provider should skip without dispatch, got %+v", res)
	}
}

// TestPreWarm_SkipsEmptyPrefix proves an empty prefix is not warmed — there is
// nothing to cache, so a billable call would be wasteful.
func TestPreWarm_SkipsEmptyPrefix(t *testing.T) {
	prov := &fakePreWarmProvider{available: true}
	res := PreWarm(context.Background(), PreWarmConfig{
		Enabled: true, Model: "m", Prefix: promptcache.PrefixComponents{}, Provider: prov,
	})
	if !res.Skipped || atomic.LoadInt32(&prov.calls) != 0 {
		t.Fatalf("empty prefix should skip, got %+v", res)
	}
}

// TestPreWarmAsync_DoesNotBlock proves PreWarmAsync returns immediately — the
// session-open path is never blocked by pre-warm. The provider here sleeps;
// PreWarmAsync must return before the sleep elapses.
func TestPreWarmAsync_DoesNotBlock(t *testing.T) {
	const providerDelay = 200 * time.Millisecond
	prov := &fakePreWarmProvider{
		available: true,
		generateFn: func(_ context.Context, _ *llm.LLMRequest) (*llm.LLMResponse, error) {
			time.Sleep(providerDelay)
			return &llm.LLMResponse{Content: "x"}, nil
		},
	}

	start := time.Now()
	done := PreWarmAsync(context.Background(), PreWarmConfig{
		Enabled: true, Model: "m", Prefix: samplePrefix(), Provider: prov,
	})
	returnLatency := time.Since(start)

	if returnLatency >= providerDelay {
		t.Fatalf("PreWarmAsync blocked for %v (provider delay %v) — session-open would stall", returnLatency, providerDelay)
	}

	// The background pre-warm still completes eventually.
	select {
	case res := <-done:
		if !res.Attempted {
			t.Fatalf("background pre-warm did not attempt: %+v", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("background pre-warm never completed")
	}
}

// TestPreWarmAsync_PanicRecovered proves a panic inside the background
// pre-warm goroutine is recovered — pre-warm can never crash the host process.
func TestPreWarmAsync_PanicRecovered(t *testing.T) {
	prov := &fakePreWarmProvider{
		available: true,
		generateFn: func(_ context.Context, _ *llm.LLMRequest) (*llm.LLMResponse, error) {
			panic("provider blew up")
		},
	}
	done := PreWarmAsync(context.Background(), PreWarmConfig{
		Enabled: true, Model: "m", Prefix: samplePrefix(), Provider: prov,
	})
	select {
	case res := <-done:
		if res.Err == nil {
			t.Fatal("expected the recovered panic to be recorded in PreWarmResult.Err")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PreWarmAsync did not recover the panic / never completed")
	}
}

// BenchmarkFirstTurnTTFT_PreWarmVsCold measures USER-PERCEIVED first-turn
// time-to-first-token with vs without a preceding pre-warm.
//
// Pre-warming happens at SESSION OPEN — before the user types anything — so
// the user-perceived first-turn latency is the latency of the FIRST REAL
// REQUEST alone, not pre-warm + request. The benchmark therefore times ONLY
// the first real request:
//   - "cold_no_prewarm": no pre-warm ran; the first real request pays the
//     full cold-cache latency (the provider hashes the whole prefix);
//   - "prewarmed": a pre-warm ran during setup (outside the timed region,
//     because it overlaps idle session-open time the user never waits on);
//     the timed first real request is now a cache HIT at warm latency.
//
// The delta between the two is the P1-T06 speedup claim: pre-warming removes
// the first-request cold-cache penalty. Latencies are modelled by the fake
// provider so the benchmark is hermetic and deterministic; the real-API metric
// is captured by the integration test when an API key is present (see
// prewarm_integration_test.go).
func BenchmarkFirstTurnTTFT_PreWarmVsCold(b *testing.B) {
	const (
		coldLatency = 8 * time.Millisecond // cold-cache: provider hashes full prefix
		warmLatency = 1 * time.Millisecond // cache hit: prefix already cached
	)
	prefix := samplePrefix()

	newProvider := func() *fakePreWarmProvider {
		var warmed atomic.Bool
		return &fakePreWarmProvider{
			available: true,
			generateFn: func(_ context.Context, _ *llm.LLMRequest) (*llm.LLMResponse, error) {
				if warmed.Swap(true) {
					time.Sleep(warmLatency)
					return &llm.LLMResponse{
						Content:          "x",
						ProviderMetadata: map[string]interface{}{"cache_read_tokens": 512},
					}, nil
				}
				time.Sleep(coldLatency)
				return &llm.LLMResponse{
					Content:          "x",
					ProviderMetadata: map[string]interface{}{"cache_creation_tokens": 512},
				}, nil
			},
		}
	}

	realRequest := func(prov *fakePreWarmProvider) {
		_, _ = prov.Generate(context.Background(), &llm.LLMRequest{
			Model: "m", MaxTokens: 1024,
			Messages: []llm.Message{{Role: "system", Content: prefix.SystemPrompt}, {Role: "user", Content: "hi"}},
		})
	}

	// cold: time the first real request with NO preceding pre-warm.
	b.Run("cold_no_prewarm", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			prov := newProvider()
			b.StartTimer()
			realRequest(prov) // first turn pays cold-cache latency
		}
	})

	// prewarmed: pre-warm in untimed setup (it overlaps session-open idle
	// time the user never perceives), then time ONLY the first real request,
	// which is now a cache hit.
	b.Run("prewarmed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			prov := newProvider()
			_ = PreWarm(context.Background(), PreWarmConfig{
				Enabled: true, Model: "m", Prefix: prefix, Provider: prov,
			})
			b.StartTimer()
			realRequest(prov) // first real turn now hits the warmed cache
		}
	})
}
