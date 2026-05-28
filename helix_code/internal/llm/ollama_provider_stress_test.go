//go:build integration

package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 STRESS coverage for the REAL Ollama LLM provider in internal/llm
// against a LIVE Ollama server (no mocks — CONST-050: non-unit tests MUST hit
// real infrastructure; CONST-039 mandates Ollama as a supported provider). The
// unit under stress is the REAL *OllamaProvider (GetModels/GetHealth/IsAvailable
// → GET /api/tags fast paths, and Generate → POST /api/chat slow path) talking
// to a live Ollama at http://localhost:11434 with model qwen2.5:0.5b.
//
// IMPORTANT — CPU generation is slow. Ollama here runs on CPU, so each Generate
// call takes ~1-10+ seconds. The §11.4.85(A)(1) "N>=100" floor is applied ONLY
// to the FAST paths (GetModels / GetHealth — sub-second HTTP round-trips). For
// the SLOW generation path we deliberately use a BOUNDED real-generation sample
// (small concurrent batch, short prompts, small num_predict) to prove
// concurrency-safety + capture real latency without taking forever. Every
// generation PASS asserts a REAL non-empty model response came back (anti-bluff:
// a genuine Ollama completion, never empty/simulated).
//
// Connection convention mirrors the existing internal/llm/ollama_provider_test.go
// OllamaConfig{BaseURL, DefaultModel, Timeout}. BaseURL/model are overridable via
// OLLAMA_TEST_URL / OLLAMA_TEST_MODEL for portability; defaults point at the
// running podman instance.

const (
	defaultOllamaTestURL   = "http://localhost:11434"
	defaultOllamaTestModel = "qwen2.5:0.5b"
)

func ollamaTestURL() string {
	if v := os.Getenv("OLLAMA_TEST_URL"); v != "" {
		return v
	}
	return defaultOllamaTestURL
}

func ollamaTestModel() string {
	if v := os.Getenv("OLLAMA_TEST_MODEL"); v != "" {
		return v
	}
	return defaultOllamaTestModel
}

// liveOllamaProvider builds a REAL OllamaProvider pointed at the live server, or
// skips honestly (§11.4.3) ONLY if Ollama is genuinely unreachable. It never
// fakes a connection. Generous Timeout (90s) so a slow CPU generation does not
// false-fail.
func liveOllamaProvider(t *testing.T) *OllamaProvider {
	t.Helper()
	cfg := OllamaConfig{
		BaseURL:      ollamaTestURL(),
		DefaultModel: ollamaTestModel(),
		Timeout:      90 * time.Second,
	}
	p, err := NewOllamaProvider(cfg)
	if err != nil {
		t.Skipf("SKIP-OK: cannot construct Ollama provider for %s (%v) — §11.4.3 honest skip", cfg.BaseURL, err)
	}
	// Probe reachability with a real /api/tags round-trip; skip honestly if down.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if !p.IsAvailable(ctx) {
		t.Skipf("SKIP-OK: requires live Ollama at %s — unreachable; §11.4.3 honest skip, never a faked PASS", cfg.BaseURL)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// shortGenRequest builds a tiny generation request: short prompt, small
// num_predict (max-tokens) so each CPU generation stays fast. Each request
// carries a unique nonce so identical prompts can't be silently de-duplicated.
func shortGenRequest(model string, nonce int) *LLMRequest {
	return &LLMRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: fmt.Sprintf("Reply with a single short word. Nonce %d.", nonce)},
		},
		MaxTokens:   16,
		Temperature: 0.2,
		TopP:        0.9,
	}
}

// =============================================================================
// §11.4.85(A)(1) — Sustained load on the FAST paths (N>=100, p50/p95/p99)
// =============================================================================

// TestOllamaProvider_Stress_SustainedGetHealth drives N>=100 real GET /api/tags
// health round-trips against live Ollama. Each iteration asserts a healthy status
// and a model count > 0 — proving a real server answered, not a no-op. Latency
// p50/p95/p99 are captured as evidence.
func TestOllamaProvider_Stress_SustainedGetHealth(t *testing.T) {
	p := liveOllamaProvider(t)
	ctx := context.Background()

	var ok int64
	rep := stresschaos.RunSustainedLoad(t, "ollama_sustained_get_health",
		stresschaos.SustainedConfig{N: 150, MaxErrorRate: 0.0},
		func(i int) error {
			h, err := p.GetHealth(ctx)
			if err != nil {
				return fmt.Errorf("get health: %w", err)
			}
			if h == nil {
				return fmt.Errorf("nil health")
			}
			if h.Status != "healthy" {
				return fmt.Errorf("status = %q, want healthy", h.Status)
			}
			if h.ModelCount <= 0 {
				return fmt.Errorf("model count = %d, want > 0 (real server discovered no models)", h.ModelCount)
			}
			atomic.AddInt64(&ok, 1)
			return nil
		})

	if atomic.LoadInt64(&ok) == 0 {
		t.Fatal("ollama sustained health loop performed zero real round-trips — not real work")
	}
	t.Logf("ollama sustained GetHealth: %d real round-trips, p50=%.3fms p95=%.3fms p99=%.3fms",
		atomic.LoadInt64(&ok), rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestOllamaProvider_Stress_SustainedGetModels drives N>=100 real GetModels calls.
// After the first lazy discovery the list is cached, so this stresses the
// concurrent-safe read path. Asserts the live-discovered model set is non-empty
// and includes the configured test model.
func TestOllamaProvider_Stress_SustainedGetModels(t *testing.T) {
	p := liveOllamaProvider(t)
	model := ollamaTestModel()

	// Prime discovery once so the first (slow) /api/tags round-trip is not part
	// of the steady-state read measurement.
	if got := p.GetModels(); len(got) == 0 {
		t.Fatalf("live Ollama discovered zero models — expected at least %q", model)
	}

	rep := stresschaos.RunSustainedLoad(t, "ollama_sustained_get_models",
		stresschaos.SustainedConfig{N: 200, MaxErrorRate: 0.0},
		func(i int) error {
			models := p.GetModels()
			if len(models) == 0 {
				return fmt.Errorf("GetModels returned empty list")
			}
			return nil
		})

	// Anti-bluff: prove the configured test model is genuinely present.
	found := false
	for _, m := range p.GetModels() {
		if m.Name == model {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("configured test model %q not in live model list — discovery did not reflect real /api/tags", model)
	}
	t.Logf("ollama sustained GetModels: N=%d p50=%.3fms p95=%.3fms p99=%.3fms; model %q present",
		rep.N, rep.P50Ms, rep.P95Ms, rep.P99Ms, model)
}

// =============================================================================
// §11.4.85(A)(2) — Concurrent contention (N>=10 goroutines, no deadlock/leak)
// =============================================================================

// TestOllamaProvider_Stress_ConcurrentHealthAndModels hammers the REAL provider
// from N>=10 goroutines doing concurrent GetHealth + GetModels + IsAvailable.
// Run under -race this catches data races in the lazy-discovery / cached-models
// read path (sync.Once + p.models slice). Each goroutine asserts real positive
// results, so concurrent correctness is proven, not just survival.
func TestOllamaProvider_Stress_ConcurrentHealthAndModels(t *testing.T) {
	p := liveOllamaProvider(t)
	ctx := context.Background()

	stresschaos.RunConcurrent(t, "ollama_concurrent_health_models",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 25, Timeout: 60 * time.Second},
		func(g, iter int) error {
			h, err := p.GetHealth(ctx)
			if err != nil {
				return fmt.Errorf("g%d i%d get health: %w", g, iter, err)
			}
			if h == nil || h.Status != "healthy" {
				return fmt.Errorf("g%d i%d unhealthy: %+v", g, iter, h)
			}
			if models := p.GetModels(); len(models) == 0 {
				return fmt.Errorf("g%d i%d GetModels empty", g, iter)
			}
			if !p.IsAvailable(ctx) {
				return fmt.Errorf("g%d i%d IsAvailable=false on live server", g, iter)
			}
			return nil
		})
	t.Logf("ollama concurrent health/models: 16x25 real concurrent round-trips, no race/deadlock/leak")
}

// =============================================================================
// §11.4.85(A) — BOUNDED real-generation concurrency sample (slow CPU path)
// =============================================================================

// TestOllamaProvider_Stress_BoundedConcurrentGenerate proves the REAL Generate
// path is concurrency-safe and returns REAL non-empty model output under a small
// concurrent batch. CPU generation is the throughput limiter, so we deliberately
// do NOT run N>=100 here (that would take many minutes); instead 8 goroutines
// each do 1 short real generation (8 genuine completions total). Each goroutine
// asserts the returned Content is non-empty real text — anti-bluff: an empty or
// simulated response FAILS, it never passes. A real generated sample is captured
// to the test log as proof of non-simulated output.
func TestOllamaProvider_Stress_BoundedConcurrentGenerate(t *testing.T) {
	p := liveOllamaProvider(t)
	model := ollamaTestModel()

	const parallelism = 10     // §11.4.85(A)(2) concurrency floor; bounded because CPU generation is the throughput limiter
	var sampleOut atomic.Value // holds one real generated string for evidence
	var nonEmpty int64

	stresschaos.RunConcurrent(t, "ollama_bounded_concurrent_generate",
		stresschaos.ConcurrencyConfig{Parallelism: parallelism, IterationsPerGoroutine: 1, Timeout: 180 * time.Second},
		func(g, iter int) error {
			// Generous per-call timeout so a slow CPU generation never false-fails.
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			resp, err := p.Generate(ctx, shortGenRequest(model, g*1000+iter))
			if err != nil {
				return fmt.Errorf("g%d generate: %w", g, err)
			}
			if resp == nil {
				return fmt.Errorf("g%d nil response", g)
			}
			// ANTI-BLUFF (CONST-035 / §11.9): a real model completion is non-empty.
			// Empty content means generation did not actually work for the user —
			// that MUST fail, never pass.
			if strings.TrimSpace(resp.Content) == "" {
				return fmt.Errorf("g%d EMPTY generated content — real generation did not work (anti-bluff fail)", g)
			}
			// Real completions report token usage; a true round-trip produces > 0
			// completion tokens.
			if resp.Usage.CompletionTokens <= 0 {
				return fmt.Errorf("g%d zero completion tokens (usage=%+v) — not a real completion", g, resp.Usage)
			}
			atomic.AddInt64(&nonEmpty, 1)
			sampleOut.Store(resp.Content)
			return nil
		})

	if atomic.LoadInt64(&nonEmpty) != parallelism {
		t.Fatalf("expected %d non-empty real generations, got %d", parallelism, atomic.LoadInt64(&nonEmpty))
	}
	sample, _ := sampleOut.Load().(string)
	t.Logf("ollama bounded concurrent generate: %d/%d real non-empty completions; SAMPLE REAL OUTPUT: %q",
		atomic.LoadInt64(&nonEmpty), parallelism, strings.TrimSpace(sample))
}

// TestOllamaProvider_Stress_SmallSequentialGenerate runs a small explicit N of
// REAL sequential generations and captures generation latency. N is intentionally
// small (CPU-generation throughput is the limiter — see file header); N>=100 is
// NOT used for the slow path. Each iteration asserts real non-empty output.
func TestOllamaProvider_Stress_SmallSequentialGenerate(t *testing.T) {
	p := liveOllamaProvider(t)
	model := ollamaTestModel()

	// Small bounded N: CPU generation is slow, so ~5 real generations is enough to
	// capture representative latency without taking minutes. This is the §11.4.85
	// boundary-aware adaptation for an inherently slow sink (real-LLM-on-CPU).
	const n = 5
	var lat []float64
	var nonEmpty int
	var lastSample string

	for i := 0; i < n; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		start := time.Now()
		resp, err := p.Generate(ctx, shortGenRequest(model, i))
		elapsed := time.Since(start)
		cancel()
		if err != nil {
			t.Fatalf("sequential generate %d failed: %v", i, err)
		}
		if resp == nil || strings.TrimSpace(resp.Content) == "" {
			t.Fatalf("sequential generate %d returned EMPTY content — anti-bluff fail (real generation broken)", i)
		}
		nonEmpty++
		lastSample = resp.Content
		lat = append(lat, float64(elapsed.Milliseconds()))
	}

	if nonEmpty != n {
		t.Fatalf("expected %d non-empty generations, got %d", n, nonEmpty)
	}
	t.Logf("ollama small sequential generate: %d real completions, latencies(ms)=%v; SAMPLE REAL OUTPUT: %q",
		nonEmpty, lat, strings.TrimSpace(lastSample))
}
