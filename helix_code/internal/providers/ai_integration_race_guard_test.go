package providers

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"dev.helix.code/internal/llm"
	memproviders "dev.helix.code/internal/memory/providers"
)

// §11.4.135 standing regression guards for two reproduced concurrency defects in
// internal/providers/ai_integration.go:
//
//   DEFECT A — AIIntegration.GetStats / HealthCheck iterated the ai.providers map
//              (and called ai.vector/ai.memory stats) WITHOUT holding ai.mu, racing
//              Initialize's ai.mu.Lock()-guarded writes. Snapshot-getter race class.
//   DEFECT B — LLMProviderAdapter.lastCostInfo was mutated by GenerateText /
//              GenerateChat and read by GetCostInfo with NO synchronization, and
//              GetCostInfo handed back the live stored pointer.
//
// §11.4.115 polarity switch via the RED_MODE env var:
//
//   RED_MODE=1 — reproduce the defect on a FAITHFUL pre-fix stand-in. The
//                stand-in replicates the exact unsynchronized access pattern the
//                production code had before the fix. Under `go test -race` the
//                race detector trips on the stand-in, proving the guard targets a
//                real race (the -race trip IS the reproduction). The test PASSES
//                in this mode (the trip is the expected, observed reproduction).
//   RED_MODE=0 — DEFAULT. Drive the REAL, fixed production code concurrently and
//                assert it runs clean. Under `-race` a clean run proves the race
//                is absent. This is the standing GREEN regression guard.
//
// These guards MUST be run under -race; the snapshot/lock fixes are invisible
// without the race detector.

func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// ---------------------------------------------------------------------------
// DEFECT A guard — GetStats / HealthCheck vs Initialize provider-map race.
// ---------------------------------------------------------------------------

// newStatsRaceIntegration builds a real AIIntegration whose provider map is
// populated by the production factory with in-process NotImplemented (Cohere)
// providers — no network. DefaultLLM is deliberately unregistered so the
// compression coordinator is skipped and the test targets only the provider-map
// iteration paths.
func newStatsRaceIntegration() *AIIntegration {
	cfg := &AIConfig{
		DefaultLLM:    "__unregistered__",
		DefaultMemory: "",
		Providers:     map[string]*AIProviderConfig{},
	}
	for i := 0; i < 8; i++ {
		cfg.Providers[fmt.Sprintf("p%d", i)] = &AIProviderConfig{
			Type:    memproviders.ProviderTypeCohere,
			Enabled: true,
			Model:   "m",
		}
	}
	return NewAIIntegration(cfg)
}

// redUnlockedStatsIteration is a FAITHFUL pre-fix stand-in for the old GetStats /
// HealthCheck bodies: it iterates ai.providers with NO lock, exactly as the
// production code did before snapshotProviders() was introduced. It exists ONLY
// to reproduce the race under RED_MODE=1.
func redUnlockedStatsIteration(ai *AIIntegration) int {
	count := 0
	for range ai.providers { // unguarded map iteration — the reproduced defect
		count++
	}
	return count
}

func TestGuard_AIIntegration_StatsHealth_NoRaceWithInitialize(t *testing.T) {
	ai := newStatsRaceIntegration()
	ctx := context.Background()

	// Realistic concurrency contract: a single Initialize populates the provider
	// map under ai.mu.Lock() concurrently with many GetStats/HealthCheck readers
	// (both are public methods a consumer may call from any goroutine). The
	// writer is a single Initialize so the test does not manufacture an
	// unrealistic Initialize-vs-Initialize race; it races readers against the
	// guarded map writes — exactly the reproduced DEFECT A.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ai.Initialize(ctx)
	}()
	for k := 0; k < 80; k++ {
		wg.Add(1)
		// Reader: in RED mode use the unguarded stand-in (reproduces the race);
		// in GREEN mode use the REAL, fixed GetStats/HealthCheck (must be clean).
		go func() {
			defer wg.Done()
			if redMode() {
				_ = redUnlockedStatsIteration(ai)
				return
			}
			if _, err := ai.GetStats(ctx); err != nil {
				t.Errorf("GetStats returned error: %v", err)
			}
			if _, err := ai.HealthCheck(ctx); err != nil {
				t.Errorf("HealthCheck returned error: %v", err)
			}
		}()
	}
	wg.Wait()

	// Post-condition: the fixed getters still report every registered provider.
	if !redMode() {
		stats, err := ai.GetStats(ctx)
		if err != nil {
			t.Fatalf("final GetStats error: %v", err)
		}
		if stats == nil || stats.Providers == nil {
			t.Fatalf("final GetStats returned nil stats")
		}
	}
}

// ---------------------------------------------------------------------------
// DEFECT B guard — LLMProviderAdapter cost-info concurrent read/write race.
// ---------------------------------------------------------------------------

// guardFakeLLMProvider is a minimal, in-process llm.Provider (unit-test only,
// no network) whose Generate returns fixed token usage so the adapter's cost
// update path runs deterministically.
type guardFakeLLMProvider struct{}

func (p *guardFakeLLMProvider) GetType() llm.ProviderType              { return llm.ProviderTypeOpenAI }
func (p *guardFakeLLMProvider) GetName() string                        { return "guard-fake" }
func (p *guardFakeLLMProvider) GetModels() []llm.ModelInfo             { return nil }
func (p *guardFakeLLMProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (p *guardFakeLLMProvider) Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{
		Content: "ok",
		Usage:   llm.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}, nil
}
func (p *guardFakeLLMProvider) GenerateStream(ctx context.Context, request *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	close(ch)
	return nil
}
func (p *guardFakeLLMProvider) IsAvailable(ctx context.Context) bool { return true }
func (p *guardFakeLLMProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "ok"}, nil
}
func (p *guardFakeLLMProvider) Close() error                         { return nil }
func (p *guardFakeLLMProvider) GetContextWindow() int                { return 4096 }
func (p *guardFakeLLMProvider) CountTokens(text string) (int, error) { return len(text) / 4, nil }

// redUnlockedCostWrite is a FAITHFUL pre-fix stand-in for the old cost-update
// path: it mutates a.lastCostInfo fields directly with NO lock, exactly as
// GenerateText/GenerateChat did before updateCostInfo() was introduced.
func redUnlockedCostWrite(a *LLMProviderAdapter) {
	a.lastCostInfo.InputTokens = 1
	a.lastCostInfo.OutputTokens = 2
	a.lastCostInfo.TotalTokens = 3
}

// redUnlockedCostRead is the FAITHFUL pre-fix stand-in for the old GetCostInfo:
// it reads a.lastCostInfo fields directly with NO lock.
func redUnlockedCostRead(a *LLMProviderAdapter) int {
	return a.lastCostInfo.InputTokens + a.lastCostInfo.OutputTokens + a.lastCostInfo.TotalTokens
}

func TestGuard_LLMProviderAdapter_CostInfo_NoConcurrentRace(t *testing.T) {
	a := NewLLMProviderAdapter(&guardFakeLLMProvider{}, "guard")
	ctx := context.Background()

	var wg sync.WaitGroup
	for k := 0; k < 100; k++ {
		wg.Add(2)
		// Writer.
		go func() {
			defer wg.Done()
			if redMode() {
				redUnlockedCostWrite(a) // unguarded write — the reproduced defect
				return
			}
			if _, err := a.GenerateText(ctx, "hi", &GenerationOptions{MaxTokens: 5}); err != nil {
				t.Errorf("GenerateText error: %v", err)
			}
		}()
		// Reader.
		go func() {
			defer wg.Done()
			if redMode() {
				_ = redUnlockedCostRead(a) // unguarded read — the reproduced defect
				return
			}
			ci := a.GetCostInfo()
			if ci == nil {
				t.Errorf("GetCostInfo returned nil")
			}
		}()
	}
	wg.Wait()

	// Post-condition: the fixed adapter reports the deterministic fake usage and
	// GetCostInfo returns a COPY (mutating it must not corrupt the stored value).
	if !redMode() {
		ci := a.GetCostInfo()
		if ci.TotalTokens != 3 {
			t.Fatalf("expected TotalTokens=3 after generations, got %d", ci.TotalTokens)
		}
		ci.TotalTokens = 999 // mutate the returned copy
		if again := a.GetCostInfo(); again.TotalTokens != 3 {
			t.Fatalf("GetCostInfo leaked the live pointer: stored value mutated to %d", again.TotalTokens)
		}
	}
}

// ---------------------------------------------------------------------------
// DEFECT C guard — VectorIntegration.GetVectorStats vs Initialize race.
//
//   VectorIntegration.GetVectorStats read vi.manager (assigned by Initialize
//   under vi.mu.Lock()) WITHOUT taking vi.mu, racing the guarded write.
// ---------------------------------------------------------------------------

// redUnlockedVectorManagerRead is the FAITHFUL pre-fix stand-in for the old
// GetVectorStats: it reads vi.manager directly with NO lock.
func redUnlockedVectorManagerRead(vi *VectorIntegration) bool {
	return vi.manager == nil // unguarded read of a field Initialize writes
}

func TestGuard_VectorIntegration_Stats_NoRaceWithInitialize(t *testing.T) {
	vi := NewVectorIntegration(nil)
	ctx := context.Background()

	// Single Initialize (guarded write of vi.manager) concurrent with many
	// GetVectorStats readers — the reproduced DEFECT C.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = vi.Initialize(ctx)
	}()
	for k := 0; k < 80; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if redMode() {
				_ = redUnlockedVectorManagerRead(vi)
				return
			}
			// The functional result is irrelevant to this guard (an empty
			// VectorConfig yields a manager with no active provider, which is a
			// benign "no active provider" error). What matters is that the
			// concurrent vi.manager READ here does NOT race Initialize's guarded
			// write — verified by `go test -race` reporting no DATA RACE.
			_, _ = vi.GetVectorStats(ctx)
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// DEFECT C (extended) guards — the OTHER 11 VectorIntegration methods that read
// vi.manager outside Initialize/Stop had the SAME guarded-write/unguarded-read
// race as GetVectorStats. The review found GetVectorStats alone was an
// incomplete fix. These guards drive a representative spread of those methods
// (a writer-path StoreVectorInProvider, a read-path SearchVectors, and the
// VectorIntegration.HealthCheck aggregator) concurrently with Initialize's
// guarded vi.manager write.
//
//   RED_MODE=1 — the faithful unguarded stand-in below reads vi.manager with NO
//                lock (exactly the pre-fix bodies), tripping -race.
//   RED_MODE=0 — DEFAULT. Drive the REAL, fixed methods; a clean -race run proves
//                the snapshot-under-RLock fix removed the race.
// ---------------------------------------------------------------------------

// redUnlockedVectorManagerUse is the FAITHFUL pre-fix stand-in for the 11
// extended methods: it reads vi.manager directly with NO lock and uses it,
// exactly as StoreVectorInProvider / SearchVectors / HealthCheck / … did before
// the snapshot-under-RLock fix.
func redUnlockedVectorManagerUse(vi *VectorIntegration) bool {
	m := vi.manager // unguarded read of a field Initialize writes
	return m == nil
}

// raceVectorInitialize spins a single Initialize (the guarded vi.manager writer)
// concurrently with `readers` goroutines each invoking `drive`. In RED mode the
// driver is the unguarded stand-in (reproduces the race); in GREEN mode it is the
// REAL fixed method (must be clean under -race). The functional result of each
// driver call is intentionally ignored: an empty VectorConfig yields a manager
// with no active provider, so the methods return benign "no active provider"
// errors — what this guard asserts is the ABSENCE of a DATA RACE on the
// vi.manager read, reported by `go test -race`.
func raceVectorInitialize(drive func(*VectorIntegration)) {
	vi := NewVectorIntegration(nil)
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = vi.Initialize(ctx)
	}()
	for k := 0; k < 80; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if redMode() {
				_ = redUnlockedVectorManagerUse(vi)
				return
			}
			drive(vi)
		}()
	}
	wg.Wait()
}

func TestGuard_VectorIntegration_StoreVectorInProvider_NoRaceWithInitialize(t *testing.T) {
	raceVectorInitialize(func(vi *VectorIntegration) {
		_ = vi.StoreVectorInProvider(context.Background(), "p", &VectorData{
			ID:        "v1",
			Embedding: []float64{0.1, 0.2, 0.3},
		})
	})
}

func TestGuard_VectorIntegration_SearchVectors_NoRaceWithInitialize(t *testing.T) {
	raceVectorInitialize(func(vi *VectorIntegration) {
		_, _ = vi.SearchVectors(context.Background(), &VectorSearchQuery{
			Embedding: []float64{0.1, 0.2, 0.3},
			K:         5,
		})
	})
}

func TestGuard_VectorIntegration_HealthCheck_NoRaceWithInitialize(t *testing.T) {
	raceVectorInitialize(func(vi *VectorIntegration) {
		_, _ = vi.HealthCheck(context.Background())
	})
}
