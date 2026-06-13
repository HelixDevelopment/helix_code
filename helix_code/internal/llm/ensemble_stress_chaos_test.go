package llm

// ensemble_stress_chaos_test.go — §11.4.85 stress + chaos resilience coverage for
// the just-hardened Helix Agent ensemble (ensemble_provider.go). These are
// unit-test-only (CONST-050(A): stubs/fakes permitted ONLY in *_test.go invoked
// without the integration build tag — this file qualifies). They prove the
// ensemble cannot deadlock / leak goroutines / panic / race, and that it degrades
// correctly under adverse conditions (member failure, ctx cancel/timeout, an
// all-dead-then-recovering catalogue).
//
// Determinism (§11.4.50): NO wall-clock sleeps gate any assertion. Latency is
// injected via the existing stubs' bounded `delay` and gate channels; cancellation
// is driven by blocking gate channels, not by racing a timer against a sleep. The
// whole suite is run at `-count=3 -race` in the captured evidence to prove
// non-flake.
//
// Reuse (§11.4.74): the load/contention/dead-model scenarios drive the existing
// ensembleStubProvider / modelAwareStub / modelCountingStub from
// ensemble_provider_test.go. Two small purpose-built stubs are added here for the
// scenarios the existing stubs do not cover: a mid-run flipping member
// (flippingStub) and a gate-blocked slow member (gatedBlockingStub) for the
// cancellation/timeout chaos case.

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// settleGoroutines returns the goroutine count after giving the runtime a bounded
// window to reap goroutines that have returned but not yet been scheduled away.
// It is NOT a timing-gated assertion: it polls until the count stops decreasing
// (or a generous cap elapses), so a transiently-elevated count from a just-returned
// fan-out worker does not flake the leak check. The assertion is "no GROWTH beyond
// a small slack", which is deterministic regardless of scheduler timing.
func settleGoroutines(baseline int) int {
	best := runtime.NumGoroutine()
	for i := 0; i < 200; i++ {
		runtime.Gosched()
		n := runtime.NumGoroutine()
		if n < best {
			best = n
		}
		if best <= baseline {
			return best
		}
		time.Sleep(time.Millisecond)
	}
	return best
}

func ensembleReq(prompt string) *LLMRequest {
	return &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Messages: []Message{{Role: "user", Content: prompt}}}
}

// NOTE: percentile (fractional p in 0..1) is the package-level helper from
// groq_provider.go — reused here per §11.4.74 (no duplicate implementation).

// ---------------------------------------------------------------------------
// Purpose-built chaos stubs
// ---------------------------------------------------------------------------

// flippingStub flips deterministically between three behaviours across successive
// Generate calls, driven by an atomic counter (NOT a timer): the call index modulo
// the length of `cycle` selects success / error / empty. This injects member-failure
// chaos with zero timing flake. `succeedAfter` (when >0) forces every call at/after
// that index to succeed — used to prove a member that errored transiently recovers.
type flippingStub struct {
	ptype        ProviderType
	name         string
	content      string
	cycle        []string // each entry ∈ {"ok","err","empty"}
	succeedAfter int32    // 0 = disabled
	calls        int32
}

func (s *flippingStub) GetType() ProviderType { return s.ptype }
func (s *flippingStub) GetName() string        { return s.name }
func (s *flippingStub) GetModels() []ModelInfo {
	return []ModelInfo{{ID: string(s.ptype) + "-chat", Name: string(s.ptype) + "-chat", Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}}}
}
func (s *flippingStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *flippingStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	n := atomic.AddInt32(&s.calls, 1) - 1 // 0-based index of this call
	if s.succeedAfter > 0 && n >= s.succeedAfter {
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: s.content, FinishReason: "stop", CreatedAt: time.Now()}, nil
	}
	behaviour := s.cycle[int(n)%len(s.cycle)]
	switch behaviour {
	case "err":
		// A TRANSIENT-class error (rate limit) — must NOT poison the member.
		return nil, errors.New("429 Too Many Requests: rate limit exceeded")
	case "empty":
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "", FinishReason: "stop", CreatedAt: time.Now()}, nil
	default: // "ok"
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: s.content, FinishReason: "stop", CreatedAt: time.Now()}, nil
	}
}
func (s *flippingStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *flippingStub) IsAvailable(ctx context.Context) bool { return true }
func (s *flippingStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: 1}, nil
}
func (s *flippingStub) Close() error                      { return nil }
func (s *flippingStub) GetContextWindow() int             { return 8192 }
func (s *flippingStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }

// gatedBlockingStub blocks inside Generate until either its release gate is closed
// OR the context is cancelled — whichever comes first. It is the deterministic
// driver for the cancellation/timeout chaos: the test cancels the ctx (or sets a
// tiny timeout) while the member is parked on the gate, and the stub must observe
// ctx.Done() and return ctx.Err() (no hang). `released` (when non-nil) is closed
// when Generate returns, so the test can confirm the goroutine actually unwound.
type gatedBlockingStub struct {
	ptype    ProviderType
	name     string
	gate     chan struct{} // never closed in the cancel tests → forces ctx path
	entered  chan struct{} // signalled once Generate has parked on the gate
	released chan struct{} // closed once Generate returns
	calls    int32
}

func (s *gatedBlockingStub) GetType() ProviderType { return s.ptype }
func (s *gatedBlockingStub) GetName() string        { return s.name }
func (s *gatedBlockingStub) GetModels() []ModelInfo {
	return []ModelInfo{{ID: string(s.ptype) + "-chat", Name: string(s.ptype) + "-chat", Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}}}
}
func (s *gatedBlockingStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *gatedBlockingStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.entered != nil {
		select {
		case s.entered <- struct{}{}:
		default:
		}
	}
	defer func() {
		if s.released != nil {
			select {
			case <-s.released: // already closed
			default:
				close(s.released)
			}
		}
	}()
	select {
	case <-s.gate:
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "late answer from " + s.name, FinishReason: "stop", CreatedAt: time.Now()}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (s *gatedBlockingStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *gatedBlockingStub) IsAvailable(ctx context.Context) bool { return true }
func (s *gatedBlockingStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: 1}, nil
}
func (s *gatedBlockingStub) Close() error                      { return nil }
func (s *gatedBlockingStub) GetContextWindow() int             { return 8192 }
func (s *gatedBlockingStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }

// ---------------------------------------------------------------------------
// 1. STRESS — sustained load
// ---------------------------------------------------------------------------

// TestEnsembleProvider_Stress_SustainedLoad fires N≥100 sequential Generate calls
// on ONE ensemble of mixed-latency members. Every call MUST return non-empty voted
// content. It records p50/p95 timing and asserts NO goroutine leak across the run
// (the fan-out spawns 1 worker per member + 1 closer per call; all MUST be reaped).
func TestEnsembleProvider_Stress_SustainedLoad(t *testing.T) {
	const iterations = 120

	fast := &ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "Fast member answer, sufficiently long to score well.", finish: "stop", tokens: 10}
	mid := &ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", content: "Mid-latency member answer.", finish: "stop", tokens: 6, delay: 750 * time.Microsecond}
	slow := &ensembleStubProvider{ptype: ProviderTypeMistral, name: "Mistral", content: "Slow member answer, the most balanced and detailed of the three.", finish: "stop", tokens: 14, delay: 2 * time.Millisecond}

	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{fast, mid, slow}, Timeout: 10 * time.Second})

	// Settle then snapshot the pre-load baseline.
	baseline := settleGoroutines(runtime.NumGoroutine())

	timings := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, err := ens.Generate(context.Background(), ensembleReq("Reply OK"))
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("iteration %d: sustained-load Generate must not error, got: %v", i, err)
		}
		if resp == nil || strings.TrimSpace(resp.Content) == "" {
			t.Fatalf("iteration %d: expected non-empty voted content, got %+v", i, resp)
		}
		if n, _ := resp.ProviderMetadata["ensemble_successful_providers"].(int); n != 3 {
			t.Fatalf("iteration %d: all 3 members must succeed, got %v", i, resp.ProviderMetadata["ensemble_successful_providers"])
		}
		timings = append(timings, elapsed)
	}

	// No goroutine leak: after the whole run, the settled count must not exceed the
	// baseline by more than a small slack (test-harness noise).
	after := settleGoroutines(baseline)
	if after > baseline+2 {
		t.Fatalf("goroutine leak: baseline=%d after=%d (delta=%d) over %d iterations", baseline, after, after-baseline, iterations)
	}

	// Each member must have been called exactly once per iteration (real fan-out).
	if got := atomic.LoadInt32(&fast.calls); got != iterations {
		t.Fatalf("fast member called %d times, want %d", got, iterations)
	}

	p50 := percentile(timings, 0.50)
	p95 := percentile(timings, 0.95)
	t.Logf("STRESS sustained-load: iterations=%d p50=%s p95=%s goroutines baseline=%d after=%d (delta=%d)",
		iterations, p50, p95, baseline, after, after-baseline)
}

// ---------------------------------------------------------------------------
// 2. STRESS — concurrent contention (run under -race)
// ---------------------------------------------------------------------------

// TestEnsembleProvider_Stress_ConcurrentContention fires N≥20 concurrent Generate
// + WarmCache calls on the SAME ensemble instance. Under -race this proves no data
// race / no panic; every Generate must return a real voted answer; the
// dead-model + workingModel maps stay internally consistent.
func TestEnsembleProvider_Stress_ConcurrentContention(t *testing.T) {
	const workers = 32

	// modelAwareStub catalogue: a decommissioned lead model, an embedding model
	// (capability-filtered out), then the working chat model — exercising the dead /
	// working / cached maps concurrently.
	a := &modelAwareStub{ptype: ProviderTypeGroq, name: "Groq", ids: []string{"gemma-7b-it", "groq-embed-v1", "llama-3.1-8b-instant"}, goodModel: "llama-3.1-8b-instant", embedModel: "groq-embed-v1"}
	b := &modelCountingStub{ptype: ProviderTypeDeepSeek, name: "DeepSeek", ids: []string{"deepseek-dead", "deepseek-chat"}, goodModel: "deepseek-chat"}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{a, b}, Timeout: 10 * time.Second})

	baseline := settleGoroutines(runtime.NumGoroutine())

	var wg sync.WaitGroup
	var okCount int32
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%4 == 0 {
				// Interleave WarmCache calls to contend on the same maps.
				ens.WarmCache(context.Background())
			}
			resp, err := ens.Generate(context.Background(), ensembleReq("Reply OK"))
			if err != nil {
				errCh <- err
				return
			}
			if resp == nil || strings.TrimSpace(resp.Content) == "" {
				errCh <- errors.New("empty voted content under contention")
				return
			}
			if !strings.Contains(resp.Content, "Real answer") {
				errCh <- errors.New("expected a real member answer, got: " + resp.Content)
				return
			}
			atomic.AddInt32(&okCount, 1)
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent contention surfaced an error: %v", err)
		}
	}
	if got := atomic.LoadInt32(&okCount); got != workers {
		t.Fatalf("expected all %d concurrent calls to return a voted answer; got %d", workers, got)
	}

	// Map-consistency invariant: every dead model recorded for a member must NOT be
	// the member's cached working model (markDead drops a stale cache entry). And the
	// cached working model, when present, must be that member's goodModel.
	ens.mu.RLock()
	deadA := map[string]bool{}
	for id := range ens.deadModel["Groq"] {
		deadA[id] = true
	}
	cachedA := ens.workingModel["Groq"]
	cachedB := ens.workingModel["DeepSeek"]
	ens.mu.RUnlock()
	if cachedA != "" && deadA[cachedA] {
		t.Fatalf("map inconsistency: Groq cached working model %q is also marked dead", cachedA)
	}
	if cachedA != "" && cachedA != a.goodModel {
		t.Fatalf("Groq cached model %q must be the working model %q", cachedA, a.goodModel)
	}
	if cachedB != "" && cachedB != b.goodModel {
		t.Fatalf("DeepSeek cached model %q must be the working model %q", cachedB, b.goodModel)
	}

	after := settleGoroutines(baseline)
	if after > baseline+2 {
		t.Fatalf("goroutine leak under contention: baseline=%d after=%d (delta=%d)", baseline, after, after-baseline)
	}
	t.Logf("STRESS concurrent-contention: workers=%d all-ok=%d Groq-dead=%d Groq-cached=%q DeepSeek-cached=%q goroutines baseline=%d after=%d",
		workers, okCount, len(deadA), cachedA, cachedB, baseline, after)
}

// ---------------------------------------------------------------------------
// 3. CHAOS — member-failure injection
// ---------------------------------------------------------------------------

// TestEnsembleProvider_Chaos_MemberFailureInjection drives members that flip
// between success / error / empty across a run of prompts. The ensemble MUST keep
// returning content as long as ≥1 member yields content on a given prompt, and a
// transient error MUST NOT permanently kill the member (it is not recorded dead).
func TestEnsembleProvider_Chaos_MemberFailureInjection(t *testing.T) {
	const prompts = 60

	// Two members on complementary cycles so that on EVERY prompt at least one
	// yields content (a never both-down on the same index):
	//   a.cycle: ok, err, empty   → down on idx%3 ∈ {1,2}
	//   b.cycle: empty, ok, ok    → down on idx%3 ∈ {0}
	// idx%3==0: a ok, b empty → a up.  ==1: a err, b ok → b up.  ==2: a empty, b ok → b up.
	a := &flippingStub{ptype: ProviderTypeGroq, name: "Groq", content: "Groq content answer, long enough.", cycle: []string{"ok", "err", "empty"}}
	b := &flippingStub{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "DeepSeek content answer, long enough.", cycle: []string{"empty", "ok", "ok"}}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{a, b}, Timeout: 10 * time.Second})

	baseline := settleGoroutines(runtime.NumGoroutine())

	survived := 0
	for i := 0; i < prompts; i++ {
		resp, err := ens.Generate(context.Background(), ensembleReq("Reply OK"))
		if err != nil {
			t.Fatalf("prompt %d: ensemble must survive while ≥1 member is up, got: %v", i, err)
		}
		if resp == nil || strings.TrimSpace(resp.Content) == "" {
			t.Fatalf("prompt %d: expected content while a member is up, got %+v", i, resp)
		}
		survived++
	}
	if survived != prompts {
		t.Fatalf("ensemble survived %d/%d prompts", survived, prompts)
	}

	// Transient (429) errors from member A must NOT have marked it dead: these are
	// explicit-path members (concrete model "<type>-chat"), and a 429 is transient.
	if dc := ens.DeadModelCount()["Groq"]; dc != 0 {
		t.Fatalf("transient 429 must NOT mark Groq dead; DeadModelCount[Groq]=%d, want 0", dc)
	}

	// Now drive the ALL-FAIL case on a single prompt: both members down at once.
	// allDownA errors, allDownB returns empty → zero successes → ensemble MUST error.
	allDownA := &flippingStub{ptype: ProviderTypeGroq, name: "Groq", content: "x", cycle: []string{"err"}}
	allDownB := &flippingStub{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "x", cycle: []string{"empty"}}
	ensDown := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{allDownA, allDownB}, Timeout: 5 * time.Second})
	if _, err := ensDown.Generate(context.Background(), ensembleReq("Reply OK")); err == nil {
		t.Fatalf("ensemble MUST error when ALL members fail on a prompt")
	}

	after := settleGoroutines(baseline)
	if after > baseline+2 {
		t.Fatalf("goroutine leak under failure injection: baseline=%d after=%d", baseline, after)
	}
	t.Logf("CHAOS member-failure-injection: prompts=%d survived=%d (≥1-up) all-down-errored=true Groq-dead=%d goroutines baseline=%d after=%d",
		prompts, survived, ens.DeadModelCount()["Groq"], baseline, after)
}

// ---------------------------------------------------------------------------
// 4. CHAOS — context cancellation / timeout
// ---------------------------------------------------------------------------

// TestEnsembleProvider_Chaos_ContextCancellation cancels the ctx mid-fan-out while
// every member is parked on a never-closed gate. The ensemble MUST return promptly
// with an error (no hang), no panic, and every fan-out goroutine MUST unwind (no
// leak). Determinism: cancellation is the ONLY thing that releases the members —
// there is no timer/sleep race.
func TestEnsembleProvider_Chaos_ContextCancellation(t *testing.T) {
	gate := make(chan struct{}) // never closed → members only return via ctx.Done()
	enteredA := make(chan struct{}, 1)
	enteredB := make(chan struct{}, 1)
	relA := make(chan struct{})
	relB := make(chan struct{})
	a := &gatedBlockingStub{ptype: ProviderTypeGroq, name: "Groq", gate: gate, entered: enteredA, released: relA}
	b := &gatedBlockingStub{ptype: ProviderTypeDeepSeek, name: "DeepSeek", gate: gate, entered: enteredB, released: relB}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{a, b}, Timeout: 30 * time.Second})

	baseline := settleGoroutines(runtime.NumGoroutine())

	ctx, cancel := context.WithCancel(context.Background())
	// Explicit-model path so members are called directly (single attempt each),
	// parking on the gate deterministically.
	req := &LLMRequest{ID: uuid.New(), Model: "Groq-chat", Messages: []Message{{Role: "user", Content: "hi"}}}

	done := make(chan struct{})
	var genErr error
	go func() {
		_, genErr = ens.Generate(ctx, req)
		close(done)
	}()

	// Wait until BOTH members are parked on the gate, THEN cancel — proving the
	// cancellation interrupts an in-flight fan-out (not a pre-cancelled no-op).
	<-enteredA
	<-enteredB
	cancel()

	select {
	case <-done:
		// good — Generate returned promptly after cancel.
	case <-time.After(5 * time.Second):
		t.Fatalf("ensemble HUNG after ctx cancel — members never observed ctx.Done()")
	}
	if genErr == nil {
		t.Fatalf("expected an error after ctx cancel (all members cancelled), got nil")
	}

	// Both member goroutines must have unwound (released closed).
	for _, rel := range []chan struct{}{relA, relB} {
		select {
		case <-rel:
		case <-time.After(2 * time.Second):
			t.Fatalf("a member goroutine did not unwind after cancel — leak")
		}
	}

	after := settleGoroutines(baseline)
	if after > baseline+2 {
		t.Fatalf("goroutine leak after cancellation: baseline=%d after=%d (delta=%d)", baseline, after, after-baseline)
	}
	t.Logf("CHAOS context-cancellation: errored=%v both-unwound=true no-hang=true goroutines baseline=%d after=%d", genErr != nil, baseline, after)
}

// TestEnsembleProvider_Chaos_TinyTimeout uses a tiny ensemble Timeout with members
// parked on a never-closed gate: the ensemble's own fan-out timeout MUST fire,
// every member MUST observe the deadline, and Generate MUST return an error
// without hanging or leaking. This exercises the e.timeout path (vs the caller's
// ctx in the prior test).
func TestEnsembleProvider_Chaos_TinyTimeout(t *testing.T) {
	gate := make(chan struct{}) // never closed
	relA := make(chan struct{})
	a := &gatedBlockingStub{ptype: ProviderTypeGroq, name: "Groq", gate: gate, released: relA}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{a}, Timeout: 25 * time.Millisecond})

	baseline := settleGoroutines(runtime.NumGoroutine())

	done := make(chan struct{})
	var genErr error
	go func() {
		_, genErr = ens.Generate(context.Background(), &LLMRequest{ID: uuid.New(), Model: "Groq-chat", Messages: []Message{{Role: "user", Content: "hi"}}})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("ensemble HUNG — e.timeout did not bound the fan-out")
	}
	if genErr == nil {
		t.Fatalf("expected a timeout error from the bounded fan-out, got nil")
	}
	select {
	case <-relA:
	case <-time.After(2 * time.Second):
		t.Fatalf("member goroutine did not unwind after timeout — leak")
	}

	after := settleGoroutines(baseline)
	if after > baseline+2 {
		t.Fatalf("goroutine leak after timeout: baseline=%d after=%d", baseline, after)
	}
	t.Logf("CHAOS tiny-timeout: errored=%v unwound=true goroutines baseline=%d after=%d", genErr != nil, baseline, after)
}

// ---------------------------------------------------------------------------
// 5. CHAOS — all-members-decommissioned then one recovers; bounded probing
// ---------------------------------------------------------------------------

// TestEnsembleProvider_Chaos_DeadModelSetBoundsProbing feeds a catalogue whose lead
// models are decommissioned (definitive errors). The dead-model set MUST bound the
// probing so that across MANY prompts a member does NOT re-walk its dead models
// every call — the per-model call count for a dead id stays at exactly 1 over the
// whole run, while the working model is hit once per prompt. This is the chaos-load
// version of the dead-model guard: many prompts, dead-model burn happens once.
func TestEnsembleProvider_Chaos_DeadModelSetBoundsProbing(t *testing.T) {
	const prompts = 40

	// Catalogue: two decommissioned lead models, then the working model. A cold,
	// unbounded ensemble would re-walk both dead models on EVERY prompt (2*prompts
	// dead attempts + prompts good = 3*prompts calls). The bounded ensemble attempts
	// each dead model exactly ONCE (discovery on prompt 1) then skips them forever.
	stub := &modelCountingStub{
		ptype:     ProviderTypeGroq,
		name:      "Groq",
		ids:       []string{"gemma-7b-it", "mixtral-dead", "llama-3.1-8b-instant"},
		goodModel: "llama-3.1-8b-instant",
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 10 * time.Second})

	baseline := settleGoroutines(runtime.NumGoroutine())

	for i := 0; i < prompts; i++ {
		resp, err := ens.Generate(context.Background(), ensembleReq("Reply OK"))
		if err != nil {
			t.Fatalf("prompt %d: must reach the working model past decommissioned leads, got: %v", i, err)
		}
		if resp == nil || !strings.Contains(resp.Content, "Real answer from Groq") {
			t.Fatalf("prompt %d: expected working-model content, got %+v", i, resp)
		}
	}

	// The two dead models must each have been attempted EXACTLY ONCE across all
	// prompts — proving the dead-model set bounds (does not re-burn) the probing.
	for _, dead := range []string{"gemma-7b-it", "mixtral-dead"} {
		if c := stub.callsFor(dead); c != 1 {
			t.Fatalf("dead model %q must be attempted exactly once across %d prompts (no re-walk); got %d", dead, prompts, c)
		}
	}
	// The working model is called once per prompt.
	if good := stub.callsFor("llama-3.1-8b-instant"); good != prompts {
		t.Fatalf("working model must be called once per prompt (%d); got %d", prompts, good)
	}
	// Both dead models recorded in the dead set.
	if dc := ens.DeadModelCount()["Groq"]; dc != 2 {
		t.Fatalf("both decommissioned leads must be recorded dead; DeadModelCount[Groq]=%d, want 2", dc)
	}

	// Member-call total = bounded discovery (2 dead, once) + prompts (working) =
	// 2 + prompts, NOT 3*prompts. This is the unforgeable bounded-probing evidence.
	wantTotalCalls := 2 + prompts
	gotTotal := stub.callsFor("gemma-7b-it") + stub.callsFor("mixtral-dead") + stub.callsFor("llama-3.1-8b-instant")
	if gotTotal != wantTotalCalls {
		t.Fatalf("bounded probing violated: total member calls=%d, want %d (2 dead-once + %d working)", gotTotal, wantTotalCalls, prompts)
	}

	after := settleGoroutines(baseline)
	if after > baseline+2 {
		t.Fatalf("goroutine leak over dead-model run: baseline=%d after=%d", baseline, after)
	}
	t.Logf("CHAOS dead-model-bounds-probing: prompts=%d dead-recorded=%d total-member-calls=%d (want %d) goroutines baseline=%d after=%d",
		prompts, ens.DeadModelCount()["Groq"], gotTotal, wantTotalCalls, baseline, after)
}

// TestEnsembleProvider_Chaos_AllDeadThenRecovers proves the all-members-dead →
// recovery path: every lead model is decommissioned and the working model only
// "comes online" partway through the run. Before recovery the ensemble fails
// cleanly (no panic, error returned); after the working model starts serving
// content, the ensemble recovers WITHOUT re-walking the already-known-dead leads.
func TestEnsembleProvider_Chaos_AllDeadThenRecovers(t *testing.T) {
	stub := &recoveringStub{
		ptype:        ProviderTypeGroq,
		name:         "Groq",
		ids:          []string{"dead-lead-1", "dead-lead-2", "recover-model"},
		goodModel:    "recover-model",
		recoverAfter: 3, // the working model returns empty until the 4th time it is asked
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 10 * time.Second})

	// Drive prompts until recovery. Before recovery the ensemble errors (working
	// model returns empty, dead leads error); after recovery it returns content.
	recovered := false
	for i := 0; i < 12 && !recovered; i++ {
		resp, err := ens.Generate(context.Background(), ensembleReq("Reply OK"))
		if err == nil && resp != nil && strings.Contains(resp.Content, "recovered answer") {
			recovered = true
		}
	}
	if !recovered {
		t.Fatalf("ensemble never recovered after the working model came online")
	}

	// The two permanently-dead leads must each have been attempted EXACTLY ONCE for
	// the whole run — the dead-model set kept the recovery prompts from re-walking
	// them. (The working model was probed repeatedly while it returned empty, which
	// is correct: empty is NOT a definitive-dead error, so it is retried.)
	for _, dead := range []string{"dead-lead-1", "dead-lead-2"} {
		if c := stub.callsFor(dead); c != 1 {
			t.Fatalf("dead lead %q must be attempted exactly once across the whole recovery run; got %d", dead, c)
		}
	}
	if dc := ens.DeadModelCount()["Groq"]; dc != 2 {
		t.Fatalf("both dead leads must be recorded; DeadModelCount[Groq]=%d, want 2", dc)
	}
	t.Logf("CHAOS all-dead-then-recovers: recovered=true dead-leads-attempted-once=true dead-recorded=%d", ens.DeadModelCount()["Groq"])
}

// recoveringStub: lead ids return a definitive decommissioned error; the goodModel
// returns EMPTY content (a non-definitive, retryable outcome) until it has been
// asked `recoverAfter` times, after which it returns real content. This drives the
// all-dead-then-recovers chaos without any timer.
type recoveringStub struct {
	ptype        ProviderType
	name         string
	ids          []string
	goodModel    string
	recoverAfter int
	mu           sync.Mutex
	perModel     map[string]int
}

func (s *recoveringStub) GetType() ProviderType { return s.ptype }
func (s *recoveringStub) GetName() string        { return s.name }
func (s *recoveringStub) GetModels() []ModelInfo {
	out := make([]ModelInfo, 0, len(s.ids))
	for _, id := range s.ids {
		out = append(out, ModelInfo{ID: id, Name: id, Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}})
	}
	return out
}
func (s *recoveringStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *recoveringStub) callsFor(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.perModel[id]
}
func (s *recoveringStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	s.mu.Lock()
	if s.perModel == nil {
		s.perModel = map[string]int{}
	}
	s.perModel[request.Model]++
	goodCount := s.perModel[s.goodModel]
	s.mu.Unlock()
	if request.Model == s.goodModel {
		if goodCount > s.recoverAfter {
			return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "recovered answer from " + s.name + ".", FinishReason: "stop", CreatedAt: time.Now()}, nil
		}
		// Still "down": empty content (retryable, NOT definitive-dead).
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "", FinishReason: "stop", CreatedAt: time.Now()}, nil
	}
	return nil, errors.New("invalid request: The model `" + request.Model + "` has been decommissioned")
}
func (s *recoveringStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *recoveringStub) IsAvailable(ctx context.Context) bool { return true }
func (s *recoveringStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: len(s.ids)}, nil
}
func (s *recoveringStub) Close() error                      { return nil }
func (s *recoveringStub) GetContextWindow() int             { return 8192 }
func (s *recoveringStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }
