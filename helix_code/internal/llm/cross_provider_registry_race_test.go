package llm

import (
	"fmt"
	"sync"
	"testing"
)

// §11.4.169 race-detector coverage sweep (2026-07-11): CrossProviderRegistry
// (internal/llm/cross_provider_registry.go) guards three shared maps
// (compatibility, providers, downloadedModels) with a single sync.RWMutex —
// one writer method (RegisterDownloadedModel, mu.Lock()) and seven reader
// methods (GetCompatibleFormats, CheckCompatibility, GetDownloadedModels,
// FindModelsForProvider, FindOptimalProvider, GetProviderInfo, ListProviders,
// all mu.RLock()). Before this file, cross_provider_registry_test.go had ZERO
// goroutine-based concurrency coverage (verified: no `go func`/`WaitGroup` in
// that file) despite being a textbook shared-manager-with-RWMutex path
// exercised from multiple goroutines in production (model download workers +
// concurrent compatibility/provider lookups from request-handling code).
//
// This guard hammers the real, production RegisterDownloadedModel writer
// concurrently with all seven real reader methods and asserts the run is
// -race-clean. A paired §1.1 mutation (recorded in
// docs/qa/race_sweep_20260711_142529/RESULTS.md — NOT committed to source)
// temporarily strips the mu.Lock()/mu.Unlock() pair from
// RegisterDownloadedModel and re-runs this exact test under -race to prove a
// real DATA RACE is reported, then reverts the source — demonstrating the
// guard is load-bearing (it actually detects the absence of the lock, not
// just "no race today by luck").
func TestGuard_CrossProviderRegistry_ConcurrentAccess_NoRace(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewCrossProviderRegistry(tempDir)

	const writers = 20
	const readersPerKind = 15

	var wg sync.WaitGroup

	// Writers: concurrent RegisterDownloadedModel calls, each registering a
	// distinct model so the map genuinely grows under concurrent writers
	// (not just re-writing the same key).
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			model := &DownloadedModel{
				ModelID:  fmt.Sprintf("model-%d", idx),
				Provider: "ollama",
				Format:   FormatGGUF,
				Path:     fmt.Sprintf("/tmp/model-%d.gguf", idx),
			}
			if err := registry.RegisterDownloadedModel(model); err != nil {
				t.Errorf("RegisterDownloadedModel(%d) returned error: %v", idx, err)
			}
		}(w)
	}

	// Readers: every exported read method driven concurrently against the
	// SAME registry instance while writers are in flight. Errors like
	// "provider not found" are expected/benign for some inputs; the guard
	// only asserts the ABSENCE of a data race (verified by `go test -race`
	// reporting no DATA RACE), not that every read call succeeds.
	for r := 0; r < readersPerKind; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = registry.GetCompatibleFormats("ollama")
		}()

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = registry.CheckCompatibility(ModelCompatibilityQuery{
				ModelID:        fmt.Sprintf("model-%d", idx),
				SourceFormat:   FormatGGUF,
				TargetProvider: "ollama",
			})
		}(r)

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.GetDownloadedModels()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = registry.FindModelsForProvider("ollama")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = registry.FindOptimalProvider("model-x", FormatGGUF, nil)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = registry.GetProviderInfo("ollama")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.ListProviders()
		}()

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = registry.findCompatibleProvidersForModel(fmt.Sprintf("model-%d", idx), FormatGGUF)
		}(r)
	}

	wg.Wait()

	// Post-condition: every writer's model landed (the registry is
	// functionally correct, not just race-silent).
	models := registry.GetDownloadedModels()
	if len(models) != writers {
		t.Fatalf("expected %d downloaded models after concurrent registration, got %d", writers, len(models))
	}
}
