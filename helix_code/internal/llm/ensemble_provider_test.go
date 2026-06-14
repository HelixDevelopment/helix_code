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

// TestEnsembleProvider_Generate_PopulatesPerMemberModels asserts the
// per-member model-visibility metadata: ProviderMetadata["ensemble_models"] maps
// each successful participant's provider name → the model id it actually served
// (resolved via the resilient sentinel path — verifier/cache/catalogue). This is
// the LOCAL half of the "operator sees which model each ensemble member used"
// feature; the TUI render (FormatEnsemblePanel) consumes this exact map.
func TestEnsembleProvider_Generate_PopulatesPerMemberModels(t *testing.T) {
	a := &ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "The answer is OK from A.", finish: "stop", tokens: 12}
	b := &ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", content: "The answer is OK from B too.", finish: "stop", tokens: 10}

	ens := NewEnsembleProvider(EnsembleProviderConfig{
		Members:  []Provider{a, b},
		Strategy: "confidence_weighted",
		Timeout:  10 * time.Second,
	})

	// Sentinel model ⇒ each member resolves its own served model via the
	// resilient resolver (catalogue fallback here: "<ptype>-m").
	req := &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Messages: []Message{{Role: "user", Content: "Reply OK"}}}
	resp, err := ens.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	models, ok := resp.ProviderMetadata["ensemble_models"].(map[string]string)
	if !ok {
		t.Fatalf("ensemble_models metadata missing or wrong type: %T", resp.ProviderMetadata["ensemble_models"])
	}
	// BOTH successful participants MUST carry a non-empty model id.
	if len(models) != 2 {
		t.Fatalf("ensemble_models = %v, want 2 entries (one per successful participant)", models)
	}
	wantA := string(ProviderTypeDeepSeek) + "-m"
	wantB := string(ProviderTypeGroq) + "-m"
	if models["DeepSeek"] != wantA {
		t.Errorf("ensemble_models[DeepSeek] = %q, want %q", models["DeepSeek"], wantA)
	}
	if models["Groq"] != wantB {
		t.Errorf("ensemble_models[Groq] = %q, want %q", models["Groq"], wantB)
	}
}

// TestEnsembleProvider_Generate_PerMemberModels_NonSentinel asserts that when
// the caller passes an explicit (non-sentinel) model, the per-member model
// metadata records THAT model for each participant (the single-attempt path).
func TestEnsembleProvider_Generate_PerMemberModels_NonSentinel(t *testing.T) {
	a := &ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "The answer is OK from A.", finish: "stop", tokens: 12}
	b := &ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", content: "The answer is OK from B.", finish: "stop", tokens: 10}

	ens := NewEnsembleProvider(EnsembleProviderConfig{
		Members: []Provider{a, b},
		Timeout: 10 * time.Second,
	})

	const explicit = "explicit-model-xyz"
	req := &LLMRequest{ID: uuid.New(), Model: explicit, Messages: []Message{{Role: "user", Content: "Reply OK"}}}
	resp, err := ens.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	models, ok := resp.ProviderMetadata["ensemble_models"].(map[string]string)
	if !ok || len(models) != 2 {
		t.Fatalf("ensemble_models = %v, want 2 entries", resp.ProviderMetadata["ensemble_models"])
	}
	if models["DeepSeek"] != explicit || models["Groq"] != explicit {
		t.Errorf("ensemble_models = %v, want both members to report %q", models, explicit)
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

// modelCountingStub records, per model id, how many times Generate was called
// with that id — the unforgeable evidence that a dead model is NOT re-walked on
// later prompts. Its lead id ("gemma-7b-it") returns a real decommissioned-class
// error; a later id ("llama-3.1-8b-instant") returns content.
type modelCountingStub struct {
	ptype     ProviderType
	name      string
	ids       []string
	goodModel string
	mu        sync.Mutex
	perModel  map[string]int
}

func (s *modelCountingStub) GetType() ProviderType { return s.ptype }
func (s *modelCountingStub) GetName() string        { return s.name }
func (s *modelCountingStub) GetModels() []ModelInfo {
	out := make([]ModelInfo, 0, len(s.ids))
	for _, id := range s.ids {
		out = append(out, ModelInfo{ID: id, Name: id, Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}})
	}
	return out
}
func (s *modelCountingStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *modelCountingStub) callsFor(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.perModel[id]
}
func (s *modelCountingStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	s.mu.Lock()
	if s.perModel == nil {
		s.perModel = map[string]int{}
	}
	s.perModel[request.Model]++
	s.mu.Unlock()
	if request.Model == s.goodModel {
		return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "Real answer from " + s.name + ".", FinishReason: "stop", CreatedAt: time.Now()}, nil
	}
	// Decommissioned-class definitive error (the live groq gemma-7b-it message).
	return nil, errors.New("invalid request: The model `" + request.Model + "` has been decommissioned")
}
func (s *modelCountingStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *modelCountingStub) IsAvailable(ctx context.Context) bool { return true }
func (s *modelCountingStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: len(s.ids)}, nil
}
func (s *modelCountingStub) Close() error                      { return nil }
func (s *modelCountingStub) GetContextWindow() int             { return 8192 }
func (s *modelCountingStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }

// TestEnsembleProvider_DeadModelSkippedOnSecondGenerate is the core dead-model
// regression guard. Prompt 1 must discover the decommissioned lead model
// ("gemma-7b-it"), record it dead, and reach the working model. Prompt 2 must
// NOT re-attempt the dead lead model — proven by the per-model call counter: the
// dead id is called EXACTLY ONCE across both prompts (discovered on prompt 1,
// skipped on prompt 2), while the good model is called on each prompt.
//
// This is the live-TUI defect's unit-level mirror: cold prompts re-walking the
// catalogue's leading dead model every time produced "all N member(s) failed".
//
// Paired §1.1 mutation: if markDead is a no-op OR orderedCandidates does not skip
// dead models, prompt 2 re-walks the dead lead model → its call count becomes 2 →
// the "exactly 1" assertion FAILs, proving the assertion genuinely catches the
// regression.
func TestEnsembleProvider_DeadModelSkippedOnSecondGenerate(t *testing.T) {
	stub := &modelCountingStub{
		ptype:     ProviderTypeGroq,
		name:      "Groq",
		ids:       []string{"gemma-7b-it", "llama-3.1-8b-instant"}, // dead lead, then working
		goodModel: "llama-3.1-8b-instant",
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 10 * time.Second})
	req := func() *LLMRequest {
		return &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Messages: []Message{{Role: "user", Content: "Reply OK"}}}
	}

	// Prompt 1: discovers the dead lead model, records it dead, reaches the good one.
	resp1, err := ens.Generate(context.Background(), req())
	if err != nil {
		t.Fatalf("prompt 1 must succeed past the dead lead model, got: %v", err)
	}
	if resp1 == nil || !strings.Contains(resp1.Content, "Real answer from Groq") {
		t.Fatalf("prompt 1 expected working-model content, got %+v", resp1)
	}
	if dc := ens.DeadModelCount()["Groq"]; dc != 1 {
		t.Fatalf("after prompt 1 the dead lead model must be recorded; DeadModelCount[Groq]=%d, want 1", dc)
	}
	deadCallsAfter1 := stub.callsFor("gemma-7b-it")
	if deadCallsAfter1 != 1 {
		t.Fatalf("prompt 1 should attempt the dead model exactly once; got %d", deadCallsAfter1)
	}

	// Prompt 2: MUST skip the now-known-dead lead model entirely.
	resp2, err := ens.Generate(context.Background(), req())
	if err != nil {
		t.Fatalf("prompt 2 must succeed, got: %v", err)
	}
	if resp2 == nil || !strings.Contains(resp2.Content, "Real answer from Groq") {
		t.Fatalf("prompt 2 expected working-model content, got %+v", resp2)
	}

	// The dead model's lifetime call count MUST still be 1 — proving prompt 2 did
	// NOT re-walk it (the cross-prompt persistence that stops the per-prompt
	// re-burn). The working model is called once per prompt.
	if dead := stub.callsFor("gemma-7b-it"); dead != 1 {
		t.Fatalf("dead model must be attempted EXACTLY ONCE across both prompts (no re-walk); got %d", dead)
	}
	if good := stub.callsFor("llama-3.1-8b-instant"); good != 2 {
		t.Fatalf("working model must be called once per prompt (2 total); got %d", good)
	}
}

// TestEnsembleProvider_TransientErrorDoesNotMarkDead proves a TRANSIENT failure
// (rate-limit / timeout) does NOT poison a model: a model that 429s on prompt 1
// is retried on prompt 2 (NOT recorded dead). Distinguishing DEAD (permanent)
// from TRANSIENT (retryable) is mandatory per the task.
//
// Paired §1.1 mutation: if isDefinitiveModelError mis-classified a 429 as
// definitive, the model would be recorded dead and prompt 2 would skip it →
// DeadModelCount would be 1 and the model's retry would not happen → assertions
// FAIL.
func TestEnsembleProvider_TransientErrorDoesNotMarkDead(t *testing.T) {
	// A single-model member that returns a 429 on the first attempt, content after.
	stub := &transientThenOKStub{ptype: ProviderTypeGroq, name: "Groq", id: "llama-3.1-8b-instant", failFirst: 1}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 10 * time.Second})
	req := func() *LLMRequest {
		return &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Messages: []Message{{Role: "user", Content: "Reply OK"}}}
	}

	// Prompt 1: the only candidate 429s → ensemble fails this prompt, but the model
	// must NOT be recorded dead (transient).
	_, _ = ens.Generate(context.Background(), req())
	if dc := ens.DeadModelCount()["Groq"]; dc != 0 {
		t.Fatalf("a 429/transient failure must NOT mark the model dead; DeadModelCount[Groq]=%d, want 0", dc)
	}

	// Prompt 2: the same model now returns content (transient cleared) — proving it
	// was retried, not permanently skipped.
	resp, err := ens.Generate(context.Background(), req())
	if err != nil {
		t.Fatalf("prompt 2 must retry the transiently-failed model and succeed, got: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Content, "Real answer") {
		t.Fatalf("prompt 2 expected content after transient cleared, got %+v", resp)
	}
}

// transientThenOKStub fails its first `failFirst` calls with a 429 (transient),
// then returns content — to prove transient failures are retried, not marked dead.
type transientThenOKStub struct {
	ptype     ProviderType
	name      string
	id        string
	mu        sync.Mutex
	calls     int
	failFirst int
}

func (s *transientThenOKStub) GetType() ProviderType { return s.ptype }
func (s *transientThenOKStub) GetName() string        { return s.name }
func (s *transientThenOKStub) GetModels() []ModelInfo {
	return []ModelInfo{{ID: s.id, Name: s.id, Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}}}
}
func (s *transientThenOKStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *transientThenOKStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	s.mu.Lock()
	s.calls++
	fail := s.calls <= s.failFirst
	s.mu.Unlock()
	if fail {
		return nil, errors.New("429 Too Many Requests: rate limit exceeded")
	}
	return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "Real answer from " + s.name + ".", FinishReason: "stop", CreatedAt: time.Now()}, nil
}
func (s *transientThenOKStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *transientThenOKStub) IsAvailable(ctx context.Context) bool { return true }
func (s *transientThenOKStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: 1}, nil
}
func (s *transientThenOKStub) Close() error                      { return nil }
func (s *transientThenOKStub) GetContextWindow() int             { return 8192 }
func (s *transientThenOKStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }

// TestIsDefinitiveModelError_ClassifiesByErrorClass proves the DEAD/TRANSIENT
// classifier keys off the error CLASS (not a hardcoded model name): real
// provider decommissioned/not-found messages classify DEAD; real rate-limit /
// timeout / network messages classify TRANSIENT.
func TestIsDefinitiveModelError_ClassifiesByErrorClass(t *testing.T) {
	dead := []string{
		"invalid request: The model `gemma-7b-it` has been decommissioned",
		"the model `gpt-x` does not exist",
		"error code: model_not_found",
		"openrouter: no endpoints found for foo/bar",
		"model not found",
		"unsupported model: whatever",
		"is not a valid model id",
	}
	for _, m := range dead {
		if !isDefinitiveModelError(errors.New(m)) {
			t.Errorf("expected DEAD classification for %q", m)
		}
	}
	transient := []string{
		"429 Too Many Requests",
		"rate limit exceeded",
		"context deadline exceeded",
		"dial tcp: i/o timeout",
		"503 service unavailable",
		"connection refused",
		"the server is overloaded, please try again",
		"429 rate limit: model not found for your tier",
		"503 service unavailable: model is not a valid model id right now",
	}
	for _, m := range transient {
		if isDefinitiveModelError(errors.New(m)) {
			t.Errorf("expected TRANSIENT (not dead) classification for %q", m)
		}
	}
}

// streamSensitiveStub mirrors a real OpenAI-compatible cloud provider's
// non-streaming Generate: when the request carries Stream=true, the provider
// asks the API for an SSE stream ("stream":true) but then tries to JSON-decode
// the single body — choking on the leading `data:` chunk with the exact live
// error `invalid character 'd' looking for beginning of value`. When Stream is
// false it returns real content. This is the unit-level mirror of the live-TUI
// "all 4 member(s) failed" defect: the TUI sets Stream=true and the ensemble
// forwarded it into each member's buffered Generate.
type streamSensitiveStub struct {
	ptype ProviderType
	name  string
	calls int32
}

func (s *streamSensitiveStub) GetType() ProviderType { return s.ptype }
func (s *streamSensitiveStub) GetName() string        { return s.name }
func (s *streamSensitiveStub) GetModels() []ModelInfo {
	return []ModelInfo{{ID: string(s.ptype) + "-chat", Name: string(s.ptype) + "-chat", Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}}}
}
func (s *streamSensitiveStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *streamSensitiveStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	atomic.AddInt32(&s.calls, 1)
	if request.Stream {
		// A buffered Generate that received Stream=true gets an SSE body it cannot
		// JSON-decode — the exact live failure mode.
		return nil, errors.New(string(s.ptype) + " request failed: invalid character 'd' looking for beginning of value")
	}
	return &LLMResponse{ID: uuid.New(), RequestID: request.ID, Content: "Real buffered answer from " + s.name + ".", FinishReason: "stop", CreatedAt: time.Now()}, nil
}
func (s *streamSensitiveStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *streamSensitiveStub) IsAvailable(ctx context.Context) bool { return true }
func (s *streamSensitiveStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: 1}, nil
}
func (s *streamSensitiveStub) Close() error                      { return nil }
func (s *streamSensitiveStub) GetContextWindow() int             { return 8192 }
func (s *streamSensitiveStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }

// TestEnsembleProvider_DoesNotForwardStreamToMembers is the regression guard for
// the live-TUI "all N member(s) failed (... invalid character 'd' ...)" defect.
// The TUI drives the ensemble via GenerateStream with Stream=true on the request;
// the ensemble must call each member's buffered Generate with Stream=FALSE (it
// votes on complete responses). If it forwards Stream=true, every member's
// buffered Generate fails to decode the SSE body and the ensemble all-fails.
//
// Paired §1.1 mutation: removing the single `base.Stream = false` (the resilient
// path's `reqCopy := base` inherits it) makes the stub return the
// SSE-decode error for every member → the ensemble returns an error → this test
// FAILs, proving the assertion catches the regression.
func TestEnsembleProvider_DoesNotForwardStreamToMembers(t *testing.T) {
	a := &streamSensitiveStub{ptype: ProviderTypeDeepSeek, name: "DeepSeek"}
	b := &streamSensitiveStub{ptype: ProviderTypeGroq, name: "Groq"}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{a, b}, Timeout: 10 * time.Second})

	// (1) Explicit-model path: caller passes a concrete model + Stream=true.
	req := &LLMRequest{ID: uuid.New(), Model: "DeepSeek-chat", Stream: true, Messages: []Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}}}
	resp, err := ens.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("ensemble must strip Stream before calling members; got error: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Content, "Real buffered answer") {
		t.Fatalf("expected a buffered member answer, got %+v", resp)
	}

	// (2) Sentinel/resilient path: model is the ensemble sentinel + Stream=true.
	sreq := &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Stream: true, Messages: []Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}}}
	sresp, serr := ens.Generate(context.Background(), sreq)
	if serr != nil {
		t.Fatalf("sentinel+Stream=true path must also strip Stream; got error: %v", serr)
	}
	if sresp == nil || !strings.Contains(sresp.Content, "Real buffered answer") {
		t.Fatalf("sentinel path expected a buffered member answer, got %+v", sresp)
	}

	// (3) Full streaming path the TUI uses: GenerateStream with Stream=true must
	// emit the voted content as a chunk and close the channel — never error.
	ch := make(chan LLMResponse, 4)
	gerr := ens.GenerateStream(context.Background(), &LLMRequest{ID: uuid.New(), Model: EnsembleModelName, Stream: true, Messages: []Message{{Role: "user", Content: "hi"}}}, ch)
	if gerr != nil {
		t.Fatalf("GenerateStream(Stream=true) must succeed (TUI path), got: %v", gerr)
	}
	var streamed string
	for c := range ch {
		streamed += c.Content
	}
	if !strings.Contains(streamed, "Real buffered answer") {
		t.Fatalf("streamed ensemble content expected a real buffered answer, got %q", streamed)
	}
}

// TestEnsembleProvider_GenerateStream_CarriesProviderMetadata proves the chunk
// the ensemble emits over GenerateStream carries the ensemble panel metadata
// (ensemble / ensemble_participants / ensemble_selected_provider / ...), NOT a
// bare content-only chunk. The streaming tool-loop path (RunToolLoopStream)
// captures result.FinalMetadata from the streamed chunk, so a metadata-less
// stream chunk would silently drop the ensemble panel data. The ensemble emits
// the full voted *resp (which Generate populated with the metadata), so the
// emitted chunk carries it — this test makes that guarantee load-bearing.
func TestEnsembleProvider_GenerateStream_CarriesProviderMetadata(t *testing.T) {
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{
		&ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", content: "answer A", finish: "stop", tokens: 2},
		&ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "answer B", finish: "stop", tokens: 2},
	}})

	ch := make(chan LLMResponse, 8)
	err := ens.GenerateStream(context.Background(), &LLMRequest{ID: uuid.New(), Messages: []Message{{Role: "user", Content: "hi"}}}, ch)
	if err != nil {
		t.Fatalf("GenerateStream must succeed, got: %v", err)
	}

	// Find the chunk carrying ProviderMetadata.
	var meta map[string]interface{}
	for c := range ch {
		if c.ProviderMetadata != nil {
			meta = c.ProviderMetadata
		}
	}
	if meta == nil {
		t.Fatal("streamed ensemble chunk MUST carry ProviderMetadata (ensemble panel data), got nil")
	}
	if ok, _ := meta["ensemble"].(bool); !ok {
		t.Errorf("streamed chunk metadata[ensemble] = %v, want true", meta["ensemble"])
	}
	if parts, ok := meta["ensemble_participants"].([]string); !ok || len(parts) != 2 {
		t.Errorf("streamed chunk metadata[ensemble_participants] = %v, want 2 named members", meta["ensemble_participants"])
	}
	if sel, _ := meta["ensemble_selected_provider"].(string); sel == "" {
		t.Error("streamed chunk metadata[ensemble_selected_provider] must be non-empty")
	}
}

// toolCallStub is a unit-test-only member provider that returns a tool-call
// request (non-empty ToolCalls, EMPTY Content) — the shape a real provider
// produces on a tool-calling turn. Pre-fix the ensemble treated empty Content as
// a failure; this stub proves the tool-request turn is now a valid participant.
type toolCallStub struct {
	ptype     ProviderType
	name      string
	toolName  string
	toolArgs  map[string]interface{}
	calls     int32
}

func (s *toolCallStub) GetType() ProviderType { return s.ptype }
func (s *toolCallStub) GetName() string        { return s.name }
func (s *toolCallStub) GetModels() []ModelInfo {
	return []ModelInfo{{ID: string(s.ptype) + "-chat", Name: string(s.ptype) + "-chat", Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}}}
}
func (s *toolCallStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *toolCallStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	atomic.AddInt32(&s.calls, 1)
	return &LLMResponse{
		ID:        uuid.New(),
		RequestID: request.ID,
		Content:   "", // tool-calling turn: empty content is VALID, not a failure
		ToolCalls: []ToolCall{{
			ID:       "call-" + s.name,
			Type:     "function",
			Function: ToolCallFunc{Name: s.toolName, Arguments: s.toolArgs},
		}},
		FinishReason: "tool_calls",
		CreatedAt:    time.Now(),
	}, nil
}
func (s *toolCallStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *toolCallStub) IsAvailable(ctx context.Context) bool { return true }
func (s *toolCallStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: 1}, nil
}
func (s *toolCallStub) Close() error                      { return nil }
func (s *toolCallStub) GetContextWindow() int             { return 8192 }
func (s *toolCallStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }

// TestEnsemble_ToolCallPassthrough proves the ensemble is tool-loop compatible:
//
//  1. Members that return ToolCalls with EMPTY Content are SUCCESS/participants,
//     NOT failures — so the ensemble does NOT return "all N member(s) failed".
//  2. The returned response carries non-empty ToolCalls (so a caller's tool loop
//     can execute them), chosen DETERMINISTICALLY (provider name sorts first).
//  3. The existing no-tools voting path is UNCHANGED: content-only members still
//     return the voted winner with NO tool calls.
//
// Paired §1.1 mutation: reverting the success condition to
// `strings.TrimSpace(oc.resp.Content) == ""` makes both tool-calling members
// count as failures → the ensemble returns the "all N member(s) failed" error →
// this test FAILs, proving the assertion catches the regression.
func TestEnsemble_ToolCallPassthrough(t *testing.T) {
	// (A) Tool-request turn: both members ask to call a tool (empty content).
	// "Apple" sorts before "Zebra", so the deterministic pick is Apple's tool call.
	apple := &toolCallStub{ptype: ProviderTypeDeepSeek, name: "Apple", toolName: "read_file", toolArgs: map[string]interface{}{"path": "a.go"}}
	zebra := &toolCallStub{ptype: ProviderTypeGroq, name: "Zebra", toolName: "list_dir", toolArgs: map[string]interface{}{"path": "."}}

	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{zebra, apple}, Timeout: 10 * time.Second})
	resp, err := ens.Generate(context.Background(), &LLMRequest{ID: uuid.New(), Model: "DeepSeek-chat", Messages: []Message{{Role: "user", Content: "open a.go"}}})
	if err != nil {
		t.Fatalf("a tool-calling turn must NOT be an all-failed error; got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response carrying tool calls, got nil")
	}
	// The response MUST carry tool calls for the caller's loop to execute.
	if len(resp.ToolCalls) == 0 {
		t.Fatalf("ensemble must surface ToolCalls on a tool-request turn, got none (content=%q)", resp.Content)
	}
	// Deterministic selection: Apple's provider name sorts first → its tool call.
	if resp.ToolCalls[0].Function.Name != "read_file" {
		t.Fatalf("deterministic tool-call selection expected Apple's %q, got %q", "read_file", resp.ToolCalls[0].Function.Name)
	}
	// Both members participated; none counted as a failure.
	if n, _ := resp.ProviderMetadata["ensemble_successful_providers"].(int); n != 2 {
		t.Fatalf("ensemble_successful_providers = %v, want 2 (tool-calling members are participants)", resp.ProviderMetadata["ensemble_successful_providers"])
	}
	if n, _ := resp.ProviderMetadata["ensemble_failed_providers"].(int); n != 0 {
		t.Fatalf("ensemble_failed_providers = %v, want 0", resp.ProviderMetadata["ensemble_failed_providers"])
	}
	parts, _ := resp.ProviderMetadata["ensemble_participants"].([]string)
	if len(parts) != 2 {
		t.Fatalf("expected 2 participants, got %v", resp.ProviderMetadata["ensemble_participants"])
	}
	// The tool-calling member's excerpt names the requested tool.
	exc, _ := resp.ProviderMetadata["ensemble_excerpts"].(map[string]string)
	if !strings.Contains(exc["Apple"], "[tool: read_file]") {
		t.Fatalf("tool-calling member excerpt must name the tool, got %q", exc["Apple"])
	}

	// (B) The existing no-tools content-voting path is UNCHANGED: content-only
	// members still return the voted winner with NO tool calls.
	good := &ensembleStubProvider{ptype: ProviderTypeDeepSeek, name: "DeepSeek", content: "The answer is a clear, well-formed reply.", finish: "stop", tokens: 10}
	short := &ensembleStubProvider{ptype: ProviderTypeGroq, name: "Groq", content: "ok", finish: "stop", tokens: 1}
	ens2 := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{good, short}, Timeout: 10 * time.Second})
	cresp, cerr := ens2.Generate(context.Background(), &LLMRequest{ID: uuid.New(), Messages: []Message{{Role: "user", Content: "hi"}}})
	if cerr != nil {
		t.Fatalf("content-only voting path must still work, got: %v", cerr)
	}
	if len(cresp.ToolCalls) != 0 {
		t.Fatalf("no-tools path must return NO tool calls, got %d", len(cresp.ToolCalls))
	}
	if strings.TrimSpace(cresp.Content) == "" {
		t.Fatalf("no-tools path must return the voted content winner, got empty")
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

// TestEnsembleProvider_OrderedCandidatesSkipsDeadModel is the W1 independent
// guard: it asserts the dead-model skip in orderedCandidates is load-bearing
// ON ITS OWN, with NO cached working model to mask it. (The
// DeadModelSkippedOnSecondGenerate test is satisfied by the working-model cache
// being tried first, so it does not exercise the orderedCandidates skip.)
// Paired §1.1: removing the `dead[id]` skip in orderedCandidates makes the dead
// model reappear in the candidate list -> this FAILs.
func TestEnsembleProvider_OrderedCandidatesSkipsDeadModel(t *testing.T) {
	stub := &modelAwareStub{ptype: ProviderTypeGroq, name: "Groq", ids: []string{"dead-1", "good-2"}, goodModel: "good-2"}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}, Timeout: 5 * time.Second})

	// Mark the lead model dead WITHOUT ever caching a working model — so only
	// the orderedCandidates dead-skip can exclude it.
	ens.markDead("Groq", "dead-1")

	cands := ens.orderedCandidates(context.Background(), stub)
	for _, c := range cands {
		if c == "dead-1" {
			t.Fatalf("orderedCandidates MUST skip the dead model; got %v", cands)
		}
	}
	found := false
	for _, c := range cands {
		if c == "good-2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("orderedCandidates dropped a live model; got %v", cands)
	}
}

// deadThenToolCallStub is a unit-test-only member provider whose catalogue LEADS
// with a decommissioned model and is FOLLOWED by a tool-capable model that
// returns a tool-call turn (EMPTY Content + non-empty ToolCalls — the real shape
// a provider produces when the model decides to call a tool). It is the unit
// mirror of the live root cause: on the sentinel+Tools resolution path, the
// resolver must (1) skip the dead lead model AND (2) ACCEPT the working model's
// tool-call response as success — even though its Content is empty.
type deadThenToolCallStub struct {
	ptype     ProviderType
	name      string
	deadID    string
	goodID    string
	toolName  string
	mu        sync.Mutex
	perModel  map[string]int
}

func (s *deadThenToolCallStub) GetType() ProviderType { return s.ptype }
func (s *deadThenToolCallStub) GetName() string        { return s.name }
func (s *deadThenToolCallStub) GetModels() []ModelInfo {
	return []ModelInfo{
		{ID: s.deadID, Name: s.deadID, Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}},
		{ID: s.goodID, Name: s.goodID, Provider: s.ptype, Capabilities: []ModelCapability{CapabilityTextGeneration}},
	}
}
func (s *deadThenToolCallStub) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityTextGeneration}
}
func (s *deadThenToolCallStub) callsFor(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.perModel[id]
}
func (s *deadThenToolCallStub) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	s.mu.Lock()
	if s.perModel == nil {
		s.perModel = map[string]int{}
	}
	s.perModel[request.Model]++
	s.mu.Unlock()
	if request.Model == s.goodID {
		// Working, tool-capable model: a tool-calling turn — EMPTY Content +
		// non-empty ToolCalls. This is the exact live behaviour proven by the
		// per-member DIRECT probe (content="" toolcalls=1 finish="tool_calls").
		return &LLMResponse{
			ID:        uuid.New(),
			RequestID: request.ID,
			Content:   "",
			ToolCalls: []ToolCall{{
				ID:       "call-" + s.name,
				Type:     "function",
				Function: ToolCallFunc{Name: s.toolName, Arguments: map[string]interface{}{}},
			}},
			FinishReason: "tool_calls",
			CreatedAt:    time.Now(),
		}, nil
	}
	// Decommissioned-class definitive error (the live groq gemma-7b-it message).
	return nil, errors.New("invalid request: The model `" + request.Model + "` has been decommissioned")
}
func (s *deadThenToolCallStub) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	r, err := s.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *r
	close(ch)
	return nil
}
func (s *deadThenToolCallStub) IsAvailable(ctx context.Context) bool { return true }
func (s *deadThenToolCallStub) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return &ProviderHealth{Status: "healthy", ModelCount: 2}, nil
}
func (s *deadThenToolCallStub) Close() error                      { return nil }
func (s *deadThenToolCallStub) GetContextWindow() int             { return 8192 }
func (s *deadThenToolCallStub) CountTokens(t string) (int, error) { return len(t) / 4, nil }

// TestEnsemble_SentinelToolResolution_AcceptsToolCallModel is the RED→GREEN guard
// for the live root cause: on the sentinel ("helix-agent-ensemble") + Tools path,
// the resilient resolver (generateMemberResilient) must ACCEPT a tool-capable
// model whose response is a tool-call turn (EMPTY Content + non-empty ToolCalls)
// as SUCCESS — instead of rejecting it for empty Content and walking the rest of
// the catalogue until every candidate is exhausted ("all N member(s) failed").
//
// Construction mirrors the live failure precisely:
//   - the catalogue LEADS with a decommissioned model (skipped/dead) and the
//     ONLY working model is NOT first — so resolution genuinely runs;
//   - the request carries the ensemble sentinel model AND Tools — the exact TUI
//     tool-loop request shape — so generateMemberResilient is exercised (a
//     non-sentinel model would bypass it).
//
// Paired §1.1 mutation: reverting generateMemberResilient's success predicate to
// `strings.TrimSpace(r.Content) != ""` (ignoring tool calls) makes the working
// tool-call model be rejected → the resolver returns the "no chat-capable model
// returned content" error → the ensemble returns "all N member(s) failed" →
// this test FAILs, proving the assertion genuinely catches the regression.
func TestEnsemble_SentinelToolResolution_AcceptsToolCallModel(t *testing.T) {
	stub := &deadThenToolCallStub{
		ptype:    ProviderTypeGroq,
		name:     "Groq",
		deadID:   "gemma-7b-it",          // decommissioned lead — must be skipped
		goodID:   "llama-3.3-70b-versatile", // working, tool-capable, NOT first
		toolName: "git_status",
	}
	// A second tool-capable member so the ensemble has >1 participant on a tool turn.
	stub2 := &deadThenToolCallStub{
		ptype:    ProviderTypeDeepSeek,
		name:     "DeepSeek",
		deadID:   "deepseek-dead",
		goodID:   "deepseek-chat",
		toolName: "git_status",
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub, stub2}, Timeout: 10 * time.Second})

	req := &LLMRequest{
		ID:    uuid.New(),
		Model: EnsembleModelName, // SENTINEL — forces generateMemberResilient
		Messages: []Message{
			{Role: "system", Content: "Call git_status to inspect the repo."},
			{Role: "user", Content: "What is the git status?"},
		},
		Tools: []Tool{{
			Type:     "function",
			Function: ToolFunction{Name: "git_status", Description: "show status", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		}},
		ToolChoice: "auto",
	}

	resp, err := ens.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("sentinel+tools resolution must succeed (tool-call model is a working model); got error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response carrying tool calls, got nil")
	}
	// The ensemble MUST surface the tool call so the caller's tool loop can run it.
	if len(resp.ToolCalls) == 0 {
		t.Fatalf("ensemble must surface ToolCalls on a sentinel+tools turn, got none (content=%q)", resp.Content)
	}
	if resp.ToolCalls[0].Function.Name != "git_status" {
		t.Fatalf("expected git_status tool call, got %q", resp.ToolCalls[0].Function.Name)
	}
	// Both members participated (each resolved its working tool-call model).
	if n, _ := resp.ProviderMetadata["ensemble_successful_providers"].(int); n != 2 {
		t.Fatalf("ensemble_successful_providers = %v, want 2", resp.ProviderMetadata["ensemble_successful_providers"])
	}
	// The dead lead model was attempted (and recorded dead), the working model won.
	if stub.callsFor("gemma-7b-it") < 1 {
		t.Fatalf("dead lead model should have been attempted once during resolution")
	}
	if stub.callsFor("llama-3.3-70b-versatile") < 1 {
		t.Fatalf("working tool-call model must have been reached during resolution")
	}
	// The working tool-call model must be CACHED so the next prompt is 1 call/member.
	ens.mu.RLock()
	cached := ens.workingModel["Groq"]
	ens.mu.RUnlock()
	if cached != "llama-3.3-70b-versatile" {
		t.Fatalf("working tool-call model must be cached for reuse; cached=%q", cached)
	}
}
