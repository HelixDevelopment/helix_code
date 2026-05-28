//go:build integration

package llm

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 CHAOS coverage for the REAL Ollama LLM provider against a LIVE Ollama
// server (no mocks — CONST-050). The provider parses HTTP/JSON responses from an
// external server and makes long-running generation calls, so the chaos surface
// is: (1) cancel mid-generation (context timeout — critical for a slow LLM call);
// (2) malformed/hostile prompts (huge prompt, control bytes, empty); (3)
// connection chaos (point at a closed port → clean error, never panic/crash).
// The provider MUST degrade cleanly (controlled error) and NEVER crash, deadlock,
// or leak. Every PASS cites a recovery_trace artefact per §11.4.5/§11.4.69.

// =============================================================================
// §11.4.85(B)(1) — Process-death / cancel-mid-generation injection
// =============================================================================

// TestOllamaProvider_Chaos_CancelMidGenerate cancels a real, in-flight generation
// (a SLOW CPU call) mid-operation via context cancellation, simulating a worker
// killed while busy. The provider must observe the cancellation and unwind
// cleanly — return a controlled error, never panic or hang. The op forces a long
// generation (large num_predict) so cancellation reliably lands mid-flight.
func TestOllamaProvider_Chaos_CancelMidGenerate(t *testing.T) {
	p := liveOllamaProvider(t)
	model := ollamaTestModel()

	stresschaos.ChaosKillDuring(t, "ollama_cancel_mid_generate", 300*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			// Long generation so the cancel (fired ~300ms in) lands mid-flight.
			req := &LLMRequest{
				Model: model,
				Messages: []Message{
					{Role: "user", Content: "Write a long detailed essay about the history of computing, at length."},
				},
				MaxTokens:   2048, // large so generation runs long enough to be cancelled
				Temperature: 0.7,
			}
			resp, err := p.Generate(ctx, req)
			if err != nil {
				// Expected graceful path: the cancelled context surfaces as a
				// controlled error (context.Canceled / DeadlineExceeded wrapped).
				rec.Record(stresschaos.Degraded, fmt.Sprintf("generation returned controlled error after cancel: %v", err))
				return
			}
			// If it completed before the cancel landed, that is also a clean outcome.
			if resp != nil {
				rec.Record(stresschaos.Recovered, "generation completed before cancel landed (clean)")
			}
		})
	t.Logf("ollama cancel-mid-generate chaos: provider unwound cleanly on mid-flight cancellation")
}

// TestOllamaProvider_Chaos_DeadlineExceededGenerate uses a deliberately tiny
// context deadline against a SLOW generation so the deadline fires before the
// CPU model can finish. The provider must surface a controlled error, never panic
// or leak the in-flight request.
func TestOllamaProvider_Chaos_DeadlineExceededGenerate(t *testing.T) {
	p := liveOllamaProvider(t)
	model := ollamaTestModel()

	rec := stresschaos.NewChaosRecorder(t, "ollama_deadline_exceeded_generate", "process-death")
	func() {
		defer func() {
			if r := recover(); r != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("Generate panicked on deadline: %v", r))
			}
		}()
		// 50ms deadline — far shorter than a CPU generation, so it must trip.
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		req := &LLMRequest{
			Model:       model,
			Messages:    []Message{{Role: "user", Content: "Explain quantum mechanics in great detail."}},
			MaxTokens:   2048,
			Temperature: 0.7,
		}
		_, err := p.Generate(ctx, req)
		if err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("deadline surfaced as controlled error: %v", err))
		} else {
			// Extremely unlikely on CPU, but a clean completion is still non-fatal.
			rec.Record(stresschaos.Recovered, "generation beat the 50ms deadline (clean completion)")
		}
	}()
	rec.AssertNoFatal()
	t.Logf("ollama deadline-exceeded chaos: provider surfaced controlled error, no panic")
}

// =============================================================================
// §11.4.85(B)(3) — Malformed / hostile prompt (input-corruption) injection
// =============================================================================

// TestOllamaProvider_Chaos_HostilePrompts feeds a battery of malformed/hostile
// prompts at the REAL provider: empty prompt, control bytes, invalid UTF-8, and a
// very large prompt. The provider must either reject cleanly or normalise — never
// crash. Because each real generation is slow on CPU, the hostile inputs use a
// tiny num_predict (max-tokens=8) so the battery completes quickly.
func TestOllamaProvider_Chaos_HostilePrompts(t *testing.T) {
	p := liveOllamaProvider(t)
	model := ollamaTestModel()

	// Each payload is the prompt content bytes; control/binary/UTF-8-invalid bytes
	// and boundary sizes exercise the JSON-marshal + HTTP + response-parse path.
	hostile := [][]byte{
		[]byte(""),                              // empty
		[]byte("\x00\x01\x02\x03control bytes"), // control bytes
		[]byte("\xff\xfe\xfa invalid utf8"),     // invalid UTF-8 leading bytes
		[]byte(strings.Repeat("A", 8*1024)),     // 8 KiB large prompt (boundary; kept modest so the tiny 0.5b CPU model is not OOM-crashed — chaos must stress the PROVIDER's parse/marshal path, not kill the shared fixture)
		[]byte("\n\n\n\t\t  "),                  // whitespace-only
	}

	stresschaos.ChaosCorruptInputDuring(t, "ollama_hostile_prompts", hostile,
		func(input []byte) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			req := &LLMRequest{
				Model:       model,
				Messages:    []Message{{Role: "user", Content: string(input)}},
				MaxTokens:   8, // tiny so the slow CPU path stays fast for the battery
				Temperature: 0.2,
			}
			// A clean error (server rejects) OR a successful normalisation (no crash)
			// are both acceptable §11.4.85 outcomes — the harness records the path.
			// What matters: NO panic / crash on hostile external-bound input.
			_, err := p.Generate(ctx, req)
			return err
		})
	t.Logf("ollama hostile-prompt chaos: provider handled empty/control/invalid-utf8/large/whitespace prompts without crash")
}

// =============================================================================
// §11.4.85(B) — Connection chaos: point at a closed port → clean error
// =============================================================================

// TestOllamaProvider_Chaos_ClosedPort points the provider at a port with nothing
// listening and exercises every network-touching path (GetHealth, IsAvailable,
// GetModels, Generate). The provider must return a CLEAN controlled error / false
// status — never panic, never crash on the unreachable endpoint or on the absent
// HTTP response it tries to parse.
func TestOllamaProvider_Chaos_ClosedPort(t *testing.T) {
	// 127.0.0.1:1 is the reserved tcpmux port — effectively always refused.
	cfg := OllamaConfig{
		BaseURL:      "http://127.0.0.1:1",
		DefaultModel: ollamaTestModel(),
		Timeout:      3 * time.Second,
	}
	p, err := NewOllamaProvider(cfg)
	if err != nil {
		t.Fatalf("constructor must not fail (it does zero I/O): %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	rec := stresschaos.NewChaosRecorder(t, "ollama_closed_port", "network-fault")

	// IsAvailable on a closed port → false, no panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("IsAvailable panicked on closed port: %v", r))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if p.IsAvailable(ctx) {
			rec.Record(stresschaos.Fatal, "IsAvailable=true against a closed port — false positive")
		} else {
			rec.Record(stresschaos.Degraded, "IsAvailable=false on closed port (clean)")
		}
	}()

	// GetHealth on a closed port → degraded status, no panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("GetHealth panicked on closed port: %v", r))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		h, err := p.GetHealth(ctx)
		if err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("GetHealth controlled error on closed port: %v", err))
		} else if h != nil && h.Status != "healthy" {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("GetHealth reported non-healthy on closed port: %q (clean)", h.Status))
		} else {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("GetHealth reported healthy against a closed port: %+v", h))
		}
	}()

	// Generate on a closed port → controlled error, no panic / nil-deref on the
	// absent response (the bug-pattern: crash-on-malformed/absent HTTP response).
	func() {
		defer func() {
			if r := recover(); r != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("Generate panicked on closed port: %v", r))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		resp, err := p.Generate(ctx, shortGenRequest(ollamaTestModel(), 0))
		if err == nil {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("Generate succeeded against a closed port (resp=%+v)", resp))
		} else {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("Generate controlled error on closed port: %v", err))
		}
	}()

	rec.AssertNoFatal()
	t.Logf("ollama closed-port chaos: every network path returned a clean error/false, no panic/crash")
}
