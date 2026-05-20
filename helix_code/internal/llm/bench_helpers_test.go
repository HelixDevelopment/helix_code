package llm

import (
	"context"
	"testing"
)

// bench_helpers_test.go — speed-programme Phase 0 task P0-T02.
//
// Unit tests asserting the bench_test.go helpers/fixtures build correctly, so a
// broken benchmark fails loudly in `go test` rather than silently producing
// meaningless numbers (CONST-050 — the benchmarks are the perf-test layer;
// these unit tests guard them). Mocks would be permitted here but none are
// needed — the httptest server is a real local HTTP round-trip.

// TestBenchHelper_OpenAIProvider asserts newBenchOpenAIProvider yields a working
// provider whose Generate() returns the fixture response over the local server.
func TestBenchHelper_OpenAIProvider(t *testing.T) {
	provider, cleanup := newBenchOpenAIProvider(t)
	defer cleanup()

	if provider == nil {
		t.Fatal("newBenchOpenAIProvider returned nil provider")
	}

	resp, err := provider.Generate(context.Background(), benchLLMRequest())
	if err != nil {
		t.Fatalf("benchmark helper Generate failed: %v", err)
	}
	if resp.Content != "4" {
		t.Fatalf("benchmark helper expected response content %q, got %q", "4", resp.Content)
	}
}

// TestBenchHelper_RequestBuild asserts the request-conversion path the
// RequestBuild benchmark exercises actually succeeds.
func TestBenchHelper_RequestBuild(t *testing.T) {
	provider, cleanup := newBenchOpenAIProvider(t)
	defer cleanup()

	out, err := provider.convertToOpenAIRequest(benchLLMRequest())
	if err != nil {
		t.Fatalf("convertToOpenAIRequest failed: %v", err)
	}
	if out.Model == "" || len(out.Messages) != 2 {
		t.Fatalf("converted request malformed: model=%q messages=%d", out.Model, len(out.Messages))
	}
}
