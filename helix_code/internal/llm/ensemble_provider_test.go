package llm

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ensembleStubProvider is a unit-test-only in-memory llm.Provider used to drive
// the EnsembleProvider deterministically WITHOUT real network calls. Per
// CONST-050(A) mocks/stubs are permitted ONLY in *_test.go unit sources — this
// file qualifies. The integration-grade proof (a real ensemble fanning a prompt
// to >1 live cloud provider) lives in the gated probe + the TUI evidence
// captured in the task report, not here.
type ensembleStubProvider struct {
	ptype    ProviderType
	name     string
	content  string
	finish   string
	err      error
	delay    time.Duration
	calls    int32
	tokens   int
}

func (s *ensembleStubProvider) GetType() ProviderType { return s.ptype }
func (s *ensembleStubProvider) GetName() string       { return s.name }
func (s *ensembleStubProvider) GetModels() []ModelInfo {
	return []ModelInfo{{ID: string(s.ptype) + "-m", Name: string(s.ptype) + "-m", Provider: s.ptype}}
}
func (s *ensembleStubProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *ensembleStubProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return &LLMResponse{
		ID:           uuid.New(),
		RequestID:    request.ID,
		Content:      s.content,
		FinishReason: s.finish,
		Usage:        Usage{CompletionTokens: s.tokens, TotalTokens: s.tokens},
		CreatedAt:    time.Now(),
	}, nil
}
func (s *ensembleStubProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	resp, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *resp
	close(ch)
	return nil
}
func (s *ensembleStubProvider) IsAvailable(ctx context.Context) bool { return s.err == nil }
func (s *ensembleStubProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: 1}, nil
}
func (s *ensembleStubProvider) Close() error            { return nil }
func (s *ensembleStubProvider) GetContextWindow() int   { return 8192 }
func (s *ensembleStubProvider) CountTokens(t string) (int, error) {
	return len(t) / 4, nil
}

func TestEnsembleProvider_Generate_OrchestratesMultipleProviders(t *testing.T) {
	a := &ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "The answer is OK from A.", finish: "stop", tokens: 12}
	b := &ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", content: "OK", finish: "stop", tokens: 1}
	c := &ensembleStubProvider{ptype: ProviderTypeMistral, name: "Mistral", content: "The answer is OK from C, a balanced reply.", finish: "stop", tokens: 14}

	ens := NewEnsembleProvider(EnsembleProviderConfig{
		Members:  []Provider{a, b, c},
		Strategy: "confidence_weighted",
		Timeout:  10 * time.Second,
	})

	req := &LLMRequest{ID: uuid.New(), Messages: []Message{{Role: "user", Content: "Reply OK"}}}
	resp, err := ens.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp == nil || resp.Content == "" {
		t.Fatalf("expected non-empty combined response, got %+v", resp)
	}

	// Anti-bluff: every member MUST have actually been called (real orchestration,
	// not a single-provider pass-through dressed up as an ensemble).
	if got := atomic.LoadInt32(&a.calls); got != 1 {
		t.Errorf("member A called %d times, want 1", got)
	}
	if got := atomic.LoadInt32(&b.calls); got != 1 {
		t.Errorf("member B called %d times, want 1", got)
	}
	if got := atomic.LoadInt32(&c.calls); got != 1 {
		t.Errorf("member C called %d times, want 1", got)
	}

	// The metadata MUST record that >1 provider participated and which one won.
	if resp.ProviderMetadata == nil {
		t.Fatalf("expected ensemble metadata, got nil")
	}
	if n, _ := resp.ProviderMetadata["ensemble_total_providers"].(int); n != 3 {
		t.Errorf("ensemble_total_providers = %v, want 3", resp.ProviderMetadata["ensemble_total_providers"])
	}
	if n, _ := resp.ProviderMetadata["ensemble_successful_providers"].(int); n != 3 {
		t.Errorf("ensemble_successful_providers = %v, want 3", resp.ProviderMetadata["ensemble_successful_providers"])
	}
	parts, ok := resp.ProviderMetadata["ensemble_participants"].([]string)
	if !ok || len(parts) != 3 {
		t.Fatalf("ensemble_participants = %v, want 3 named members", resp.ProviderMetadata["ensemble_participants"])
	}
	if sel, _ := resp.ProviderMetadata["ensemble_selected_provider"].(string); sel == "" {
		t.Errorf("ensemble_selected_provider must name the winning member, got empty")
	}
}

func TestEnsembleProvider_Generate_SurvivesPartialFailure(t *testing.T) {
	good := &ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "OK from the healthy provider.", finish: "stop", tokens: 8}
	bad := &ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", err: errors.New("429 rate limited")}

	ens := NewEnsembleProvider(EnsembleProviderConfig{
		Members: []Provider{good, bad},
		Timeout: 5 * time.Second,
	})

	resp, err := ens.Generate(context.Background(), &LLMRequest{ID: uuid.New(), Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("ensemble must survive one failed member; got err: %v", err)
	}
	if !strings.Contains(resp.Content, "healthy") {
		t.Errorf("expected the healthy provider's content to win, got %q", resp.Content)
	}
	if n, _ := resp.ProviderMetadata["ensemble_successful_providers"].(int); n != 1 {
		t.Errorf("ensemble_successful_providers = %v, want 1", resp.ProviderMetadata["ensemble_successful_providers"])
	}
	if n, _ := resp.ProviderMetadata["ensemble_failed_providers"].(int); n != 1 {
		t.Errorf("ensemble_failed_providers = %v, want 1", resp.ProviderMetadata["ensemble_failed_providers"])
	}
}

func TestEnsembleProvider_Generate_AllFailReturnsError(t *testing.T) {
	bad1 := &ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", err: errors.New("boom1")}
	bad2 := &ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", err: errors.New("boom2")}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{bad1, bad2}, Timeout: 3 * time.Second})

	_, err := ens.Generate(context.Background(), &LLMRequest{ID: uuid.New(), Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatalf("expected error when every member fails, got nil")
	}
}

func TestEnsembleProvider_GetType_GetName_GetModels(t *testing.T) {
	ens := NewEnsembleProvider(EnsembleProviderConfig{
		Members: []Provider{&ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek"}},
	})
	if ens.GetType() != ProviderTypeEnsemble {
		t.Errorf("GetType = %q, want %q", ens.GetType(), ProviderTypeEnsemble)
	}
	models := ens.GetModels()
	if len(models) == 0 {
		t.Fatalf("ensemble must expose at least one selectable model")
	}
	if models[0].Name != EnsembleModelName {
		t.Errorf("model name = %q, want %q", models[0].Name, EnsembleModelName)
	}
	if models[0].Provider != ProviderTypeEnsemble {
		t.Errorf("model provider = %q, want %q", models[0].Provider, ProviderTypeEnsemble)
	}
}

func TestEnsembleProvider_NoMembersIsUnavailable(t *testing.T) {
	ens := NewEnsembleProvider(EnsembleProviderConfig{})
	if ens.IsAvailable(context.Background()) {
		t.Errorf("ensemble with zero members must be unavailable")
	}
	if _, err := ens.Generate(context.Background(), &LLMRequest{ID: uuid.New()}); err == nil {
		t.Errorf("Generate with zero members must error")
	}
}

// ---------------------------------------------------------------------------
// §11.4.43 / §11.4.115 regression guard for the "ensemble forwards a dead
// models[0]" defect found while recording the Helix Agent ensemble TUI video.
// Forensic FACT: defaultModelFor returned p.GetModels()[0], which per real
// provider is unusable (groq gemma-7b-it = decommissioned; openrouter[0] = 402
// paid; mistral[0] = embedding; deepseek[0] = empty content). Every member
// failed → ensemble returned "all 4 member(s) failed". The fix: try chat-capable
// candidates until one returns non-empty content.
//
// modelAwareStub is a per-model stub: only goodModel returns content; everything
// else errors (decommissioned/paid) or returns empty (the deepseek case).
type modelAwareStub struct {
	ptype      ProviderType
	name       string
	ids        []string // catalogue order (models[0] first, as the real bug)
	goodModel  string   // the only id that returns real content
	embedModel string   // id the catalogue declares embedding/non-chat (excluded)
	emptyIDs   map[string]bool
	calls      int32
	// gate (when non-nil) blocks a goodModel Generate until closed; entered (when
	// non-nil) signals a goodModel probe has begun. Used to deterministically
	// test the WarmCache once-guard while a probe is in-flight.
	gate    chan struct{}
	entered chan struct{}
}

func (s *modelAwareStub) GetType() ProviderType { return s.ptype }
func (s *modelAwareStub) GetName() string        { return s.name }
func (s *modelAwareStub) GetModels() []ModelInfo {
	out := make([]ModelInfo, 0, len(s.ids))
	for _, id := range s.ids {
		mi := ModelInfo{ID: id, Name: id, Provider: s.ptype}
		// Mirror a real enriched catalogue: each entry declares its
		// capabilities. The embedding entry declares the vision capability with
		// no text-generation capability so the capability-driven (NOT
		// name-driven) fallback excludes it. Chat entries declare text gen.
		if id == s.embedModel {
			mi.Capabilities = []ModelCapability{CapabilityVision}
		} else {
			mi.Capabilities = []ModelCapability{CapabilityTextGeneration}
		}
		out = append(out, mi)
	}
	return out
}
func (s *modelAwareStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *modelAwareStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	atomic.AddInt32(&s.calls, 1)
	if request.Model == s.goodModel {
		if s.entered != nil {
			select {
			case s.entered <- struct{}{}:
			default:
			}
		}
		if s.gate != nil {
			<-s.gate
		}
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "Real answer from " + s.name + ".", FinishReason: "stop", CreatedAt: time.Now()}, nil
	}
	if s.emptyIDs[request.Model] {
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "", FinishReason: "stop", CreatedAt: time.Now()}, nil
	}
	return nil, errors.New("invalid request: The model `" + request.Model + "` has been decommissioned")
}
func (s *modelAwareStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *modelAwareStub) IsAvailable(ctx context.Context) bool { return true }
func (s *modelAwareStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: len(s.ids)}, nil
}
func (s *modelAwareStub) Close() error                        { return nil }
func (s *modelAwareStub) GetContextWindow() int               { return 8192 }
func (s *modelAwareStub) CountTokens(t string) (int, error)   { return len(t) / 4, nil }

// TestEnsembleProvider_ResilientModelResolution is the standing GREEN guard
// (RED_MODE=0, default). With RED_MODE=1 it reproduces the defect on the
// pre-fix path (models[0] only) and asserts that path FAILs — positive
// evidence the defect is real (§11.4.115).
func TestEnsembleProvider_ResilientModelResolution(t *testing.T) {
	// Catalogue mirrors the real shape: dead model first, an embedding model
	// (must be filtered), then the working chat model.
	stub := &modelAwareStub{
		ptype:      ProviderTypeGroq,
		name:       "Groq",
		ids:        []string{"gemma-7b-it", "groq-embed-v1", "llama-3.1-8b-instant"},
		goodModel:  "llama-3.1-8b-instant",
		embedModel: "groq-embed-v1",
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 10 * time.Second})
	req := &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Messages: []Message{{Role: "user", Content: "Reply OK"}}}

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the defect: the pre-fix path forwarded only models[0]
		// ("gemma-7b-it"), which errors → member fails → ensemble all-fail.
		dead := defaultModelForLegacy(stub)
		if _, err := stub.Generate(context.Background(), &LLMRequest{ID: uuid.New(), Model: dead, Messages: req.Messages}); err == nil {
			t.Fatalf("RED expected models[0]=%q to fail (decommissioned), but it succeeded", dead)
		}
		t.Logf("RED reproduced: legacy models[0]=%q fails as decommissioned", dead)
		return
	}

	// GREEN guard: the resilient ensemble must walk past the dead+embed models
	// and return the working model's content.
	resp, err := ens.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("resilient ensemble must succeed past dead models[0], got: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Content, "Real answer from Groq") {
		t.Fatalf("expected real content from the working model, got %+v", resp)
	}
	// The embedding model must never be attempted. Note the catalogue filter is
	// now capability-driven: the stub declares per-model capabilities so the
	// embedding entry is excluded by its declared capabilities, not by a
	// hardcoded name marker.
	for _, c := range catalogueChatCandidatesFor(stub) {
		if strings.Contains(c, "embed") {
			t.Fatalf("embedding model %q must be filtered out of chat candidates", c)
		}
	}
}

// TestEnsembleProvider_WarmCache_PreventsColdStartStorm proves WarmCache:
//
//  1. populates the member's working-model cache to the model that actually
//     returns content (skipping the decommissioned models[0] and the filtered
//     embedding model); AND
//  2. makes a SUBSEQUENT real Generate hit the cached model in exactly ONE call
//     to that member — no re-probing of the dead models (the cold-start
//     discovery storm that caused "all N member(s) failed" in the TUI).
//
// The stub's atomic `calls` counter is the unforgeable evidence: after WarmCache,
// the next Generate must add exactly 1 to the counter (the cached working model),
// not the 2+ a cold re-probe would add.
//
// Paired §1.1 mutation: a build that skips caching (e.g. WarmCache returns
// immediately / rememberWorkingModel is a no-op) leaves the cache cold, so the
// post-warm Generate re-probes (gemma-7b-it error → llama good = 2 calls) and the
// "exactly 1 call" assertion FAILs — proving the assertion genuinely catches the
// regression.
func TestEnsembleProvider_WarmCache_PreventsColdStartStorm(t *testing.T) {
	stub := &modelAwareStub{
		ptype:      ProviderTypeGroq,
		name:       "Groq",
		ids:        []string{"gemma-7b-it", "groq-embed-v1", "llama-3.1-8b-instant"},
		goodModel:  "llama-3.1-8b-instant",
		embedModel: "groq-embed-v1",
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 10 * time.Second})

	// Cold cache: WarmCache must probe past the dead models[0] and cache the
	// working model. With models[0]=gemma (error) → llama (good), warming costs
	// exactly 2 calls.
	ens.WarmCache(context.Background())

	warmCalls := atomic.LoadInt32(&stub.calls)
	if warmCalls == 0 {
		t.Fatalf("WarmCache must probe the member at least once; got 0 calls")
	}

	// The cache must now hold the WORKING model, not the dead models[0].
	ens.mu.RLock()
	cached := ens.workingModel[stub.GetName()]
	ens.mu.RUnlock()
	if cached != stub.goodModel {
		t.Fatalf("WarmCache must cache the working model %q; cached %q", stub.goodModel, cached)
	}

	// Now a real prompt. Because the cache is warm, the member must be called
	// EXACTLY ONCE (the cached working model) — proving no cold-start re-probe.
	req := &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Messages: []Message{{Role: "user", Content: "Reply OK"}}}
	resp, err := ens.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("warm ensemble Generate must succeed, got: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Content, "Real answer from Groq") {
		t.Fatalf("expected real content from the cached working model, got %+v", resp)
	}

	afterCalls := atomic.LoadInt32(&stub.calls)
	delta := afterCalls - warmCalls
	if delta != 1 {
		t.Fatalf("post-warm Generate must hit the cached model in exactly 1 call (no re-probe); got %d calls (warm=%d after=%d)", delta, warmCalls, afterCalls)
	}
}

// TestEnsembleProvider_WarmCache_IdempotentAndConcurrencySafe proves repeated and
// concurrent WarmCache calls are safe (sync.Once-style guard) and do not re-probe
// once the cache is warm.
func TestEnsembleProvider_WarmCache_IdempotentAndConcurrencySafe(t *testing.T) {
	stub := &modelAwareStub{
		ptype:      ProviderTypeGroq,
		name:       "Groq",
		ids:        []string{"gemma-7b-it", "llama-3.1-8b-instant"},
		goodModel:  "llama-3.1-8b-instant",
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 10 * time.Second})

	// Fire many concurrent WarmCache calls; the guard must let the fan-out run at
	// most once.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); ens.WarmCache(context.Background()) }()
	}
	wg.Wait()

	// Second-phase warm calls must be no-ops (cache already warm + guard tripped).
	before := atomic.LoadInt32(&stub.calls)
	ens.WarmCache(context.Background())
	ens.WarmCache(context.Background())
	after := atomic.LoadInt32(&stub.calls)
	if after != before {
		t.Fatalf("repeated WarmCache after warm must not re-probe; before=%d after=%d", before, after)
	}

	ens.mu.RLock()
	cached := ens.workingModel[stub.GetName()]
	ens.mu.RUnlock()
	if cached != stub.goodModel {
		t.Fatalf("WarmCache must cache the working model %q; cached %q", stub.goodModel, cached)
	}
}

// defaultModelForLegacy reproduces the pre-fix models[0] selection for the RED
// reproduction above (the bug the fix removed).
func defaultModelForLegacy(p Provider) string {
	m := p.GetModels()
	if len(m) == 0 {
		return ""
	}
	return m[0].ID
}

// TestEnsembleProvider_WarmCache_OnceGuardPreventsDoubleFanout proves the
// warmStarted once-guard (NOT merely the per-member cache) prevents a second
// concurrent WarmCache from fanning out while the first probe is still in
// flight — nothing is cached yet, so only the guard can stop the double probe.
// Paired §1.1: removing `e.warmStarted = true` in WarmCache makes both calls
// fan out → calls == 2 → this test FAILs.
func TestEnsembleProvider_WarmCache_OnceGuardPreventsDoubleFanout(t *testing.T) {
	gate := make(chan struct{})
	entered := make(chan struct{}, 1)
	stub := &modelAwareStub{
		ptype:     ProviderTypeGroq,
		name:      "Groq",
		ids:       []string{"m-good"},
		goodModel: "m-good",
		gate:      gate,
		entered:   entered,
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 5 * time.Second})

	done := make(chan struct{})
	go func() { ens.WarmCache(context.Background()); close(done) }()
	<-entered // first WarmCache's probe is now in-flight (blocked on gate, nothing cached)

	// Second concurrent WarmCache MUST return immediately via the once-guard.
	ens.WarmCache(context.Background())

	close(gate) // unblock the first probe
	<-done

	if c := atomic.LoadInt32(&stub.calls); c != 1 {
		t.Fatalf("once-guard: member probed %d times during concurrent WarmCache, want exactly 1", c)
	}
}
