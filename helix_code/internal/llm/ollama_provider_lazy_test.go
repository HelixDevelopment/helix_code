package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// ollama_provider_lazy_test.go — speed programme P1-T02 coverage.
//
// Task P1-T02 moved the blocking /api/tags discovery round-trip OUT of
// NewOllamaProvider's constructor and behind a sync.Once-guarded lazy
// path (ensureModelsDiscovered), triggered on the first real use
// (GetModels / GetHealth / getModelName). These tests prove:
//
//   - the constructor performs ZERO network I/O (no /api/tags hit);
//   - the first GetModels() triggers discovery exactly once;
//   - subsequent GetModels() calls reuse the result (no re-discovery);
//   - discovery against a real local httptest Ollama-shim still yields
//     the models — no behavioural regression vs the old eager path;
//   - a constructor benchmark proves the round-trip is gone.
//
// CONST-050(A): the httptest.Server below is a REAL local HTTP server on
// loopback — real network I/O, not a mock. The atomic call counter is a
// recording layer inside that real server, which is permitted in unit
// tests AND is itself a real measurement, not a fake implementation.

// newCountingOllamaShim returns a real loopback HTTP server that answers
// /api/tags like Ollama and increments tagsHits on every /api/tags
// request, so a test can assert exactly how many discovery round-trips
// the provider performed.
func newCountingOllamaShim(t *testing.T, tagsHits *int64, models []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			atomic.AddInt64(tagsHits, 1)
			modelList := make([]map[string]interface{}, 0, len(models))
			for _, name := range models {
				modelList = append(modelList, map[string]interface{}{
					"name":        name,
					"modified_at": time.Now().Format(time.RFC3339),
					"size":        4000000000,
					"digest":      "deadbeef",
					"details": map[string]interface{}{
						"format":         "gguf",
						"family":         "llama",
						"parameter_size": "8B",
					},
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"models": modelList})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestNewOllamaProvider_ConstructorPerformsNoNetworkIO proves the P1-T02
// invariant: NewOllamaProvider does ZERO network I/O. The recording shim
// asserts /api/tags was hit zero times after construction.
func TestNewOllamaProvider_ConstructorPerformsNoNetworkIO(t *testing.T) {
	var tagsHits int64
	srv := newCountingOllamaShim(t, &tagsHits, []string{"llama3:latest"})

	config := OllamaConfig{
		BaseURL:      srv.URL,
		DefaultModel: "llama3:latest",
		Timeout:      30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider returned error: %v", err)
	}
	if provider == nil {
		t.Fatal("NewOllamaProvider returned nil provider")
	}

	if got := atomic.LoadInt64(&tagsHits); got != 0 {
		t.Fatalf("constructor performed network I/O: /api/tags hit %d times, want 0 "+
			"(P1-T02: constructor MUST do zero network I/O)", got)
	}
}

// TestOllamaProvider_DiscoveryIsLazyAndRunsOnce proves discovery is
// triggered by the first GetModels() call and runs exactly once — the
// sync.Once guard means subsequent calls reuse the cached result.
func TestOllamaProvider_DiscoveryIsLazyAndRunsOnce(t *testing.T) {
	var tagsHits int64
	srv := newCountingOllamaShim(t, &tagsHits, []string{"llama3:latest", "codellama:7b"})

	config := OllamaConfig{
		BaseURL: srv.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider returned error: %v", err)
	}

	// Still zero before first use.
	if got := atomic.LoadInt64(&tagsHits); got != 0 {
		t.Fatalf("discovery ran before first use: /api/tags hit %d times, want 0", got)
	}

	// First GetModels() triggers discovery.
	first := provider.GetModels()
	if got := atomic.LoadInt64(&tagsHits); got != 1 {
		t.Fatalf("first GetModels() triggered %d discovery round-trips, want exactly 1", got)
	}
	if len(first) != 2 {
		t.Fatalf("first GetModels() returned %d models, want 2", len(first))
	}

	// Subsequent calls reuse the result — no further /api/tags hits.
	for i := 0; i < 5; i++ {
		again := provider.GetModels()
		if len(again) != 2 {
			t.Fatalf("repeat GetModels() returned %d models, want 2", len(again))
		}
	}
	if got := atomic.LoadInt64(&tagsHits); got != 1 {
		t.Fatalf("discovery re-ran on subsequent calls: /api/tags hit %d times total, "+
			"want exactly 1 (sync.Once must guarantee single discovery)", got)
	}

	// GetHealth also relies on the discovered model list; it must reuse
	// the already-discovered result without a fresh DISCOVERY round-trip.
	// Note: GetHealth additionally performs its own independent liveness
	// probe against /api/tags (a pre-existing health check, distinct from
	// model discovery) — so the total /api/tags hit count rises by 1 for
	// that probe, but ModelCount must come from the already-cached list.
	before := atomic.LoadInt64(&tagsHits)
	health, err := provider.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth returned error: %v", err)
	}
	if health.ModelCount != 2 {
		t.Fatalf("GetHealth ModelCount = %d, want 2 (discovered list must be reused)", health.ModelCount)
	}
	// GetHealth's liveness probe accounts for at most ONE extra /api/tags
	// hit; discovery itself must NOT re-run (sync.Once already fired).
	if after := atomic.LoadInt64(&tagsHits); after-before > 1 {
		t.Fatalf("GetHealth caused %d extra /api/tags hits; at most 1 (the liveness "+
			"probe) is allowed — discovery must not re-run", after-before)
	}
}

// TestOllamaProvider_LazyDiscoveryYieldsRealModels is the integration-
// style check: discovery against a real local httptest Ollama-shim still
// yields the exact model list — proving no behavioural regression vs the
// pre-P1-T02 eager-discovery path.
func TestOllamaProvider_LazyDiscoveryYieldsRealModels(t *testing.T) {
	var tagsHits int64
	want := []string{"llama3.2:latest", "qwen2.5-coder:7b", "deepseek-r1:8b"}
	srv := newCountingOllamaShim(t, &tagsHits, want)

	config := OllamaConfig{
		BaseURL: srv.URL,
		Timeout: 30 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider returned error: %v", err)
	}

	models := provider.GetModels()
	if len(models) != len(want) {
		t.Fatalf("GetModels() returned %d models, want %d", len(models), len(want))
	}
	got := make(map[string]bool, len(models))
	for _, m := range models {
		got[m.Name] = true
		if m.Provider != ProviderTypeLocal {
			t.Errorf("model %q has Provider=%v, want ProviderTypeLocal", m.Name, m.Provider)
		}
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("expected model %q missing from lazy-discovered list", name)
		}
	}
}

// TestOllamaProvider_DiscoveryFailureIsBestEffort proves error handling
// is unchanged: a discovery failure leaves the provider usable with an
// empty model list (the old eager path only log.Printf'd the warning),
// and the failed sync.Once does not re-attempt.
func TestOllamaProvider_DiscoveryFailureIsBestEffort(t *testing.T) {
	// Point at a closed server so /api/tags fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	config := OllamaConfig{
		BaseURL: closedURL,
		Timeout: 1 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider must not fail even when Ollama is down: %v", err)
	}

	models := provider.GetModels()
	if len(models) != 0 {
		t.Fatalf("GetModels() with unreachable Ollama returned %d models, want 0", len(models))
	}
	// A second call must not panic and must stay empty (sync.Once fired).
	if again := provider.GetModels(); len(again) != 0 {
		t.Fatalf("repeat GetModels() returned %d models, want 0", len(again))
	}
}

// BenchmarkNewOllamaProvider measures constructor wall-clock. Post-P1-T02
// this benchmark performs ZERO network I/O — the delta vs the pre-change
// eager constructor (which paid one /api/tags HTTP round-trip every call)
// is the speedup the task claims. Run with:
//
//	go test -bench BenchmarkNewOllamaProvider -benchmem ./internal/llm/
func BenchmarkNewOllamaProvider(b *testing.B) {
	config := OllamaConfig{
		BaseURL:      "http://localhost:11434",
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := NewOllamaProvider(config)
		if err != nil {
			b.Fatalf("NewOllamaProvider error: %v", err)
		}
		_ = p
	}
}

// BenchmarkNewOllamaProvider_EagerDiscoveryBaseline reproduces the
// PRE-P1-T02 cost: a constructor that pays one /api/tags round-trip. It
// constructs the provider and immediately forces discovery, so the
// per-op cost equals the old eager constructor. The delta between this
// benchmark and BenchmarkNewOllamaProvider IS the P1-T02 speedup the
// task must prove (CONST-035 Rule 9 — pasted before/after numbers).
func BenchmarkNewOllamaProvider_EagerDiscoveryBaseline(b *testing.B) {
	var tagsHits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			atomic.AddInt64(&tagsHits, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3.2:latest"}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	config := OllamaConfig{
		BaseURL:      srv.URL,
		DefaultModel: "llama3.2",
		Timeout:      30 * time.Second,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := NewOllamaProvider(config)
		if err != nil {
			b.Fatalf("NewOllamaProvider error: %v", err)
		}
		// Force discovery to reproduce the old eager-constructor cost.
		_ = p.GetModels()
	}
}
