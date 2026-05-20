package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// bench_test.go — speed-programme Phase 0 task P0-T02.
//
// Baseline benchmarks for the LLM provider dispatch hot path (R1 bottleneck B03
// — LLM dispatch / per-call HTTP transport overhead). Real-network calls are
// deliberately excluded: a local `httptest` server provides a deterministic,
// zero-latency round-trip so the benchmark isolates HelixCode's own cost —
// request conversion, JSON marshal/unmarshal, HTTP client dispatch and response
// conversion — which is exactly what P1-T01 (shared tuned HTTP transport) must
// move. No production code is changed by this file.
//
// Run: go test -bench=. -benchmem -run=^$ ./internal/llm/

// openAIChatResponse is the minimal valid OpenAI chat-completions JSON body the
// provider's makeOpenAIRequest decoder accepts. Kept as a const so the httptest
// handler does no per-request allocation that would pollute the measurement.
const openAIChatResponse = `{"id":"chatcmpl-bench","object":"chat.completion","created":1,` +
	`"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant",` +
	`"content":"4"},"finish_reason":"stop"}],` +
	`"usage":{"prompt_tokens":8,"completion_tokens":1,"total_tokens":9}}`

// newBenchOpenAIProvider builds an OpenAIProvider wired to a local httptest
// server. Returned cleanup closes the server. Shared by the benchmarks and by
// bench_helpers_test.go's unit test so a broken helper fails loudly.
func newBenchOpenAIProvider(tb testing.TB) (*OpenAIProvider, func()) {
	tb.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(openAIChatResponse))
	}))
	provider, err := NewOpenAIProvider(ProviderConfigEntry{
		Type:     ProviderTypeOpenAI,
		Endpoint: srv.URL,
		APIKey:   "bench-key-not-a-real-secret",
		Enabled:  true,
	})
	if err != nil {
		srv.Close()
		tb.Fatalf("NewOpenAIProvider: %v", err)
	}
	return provider, srv.Close
}

// benchLLMRequest builds a representative chat request (system + user turn).
func benchLLMRequest() *LLMRequest {
	return &LLMRequest{
		ID:    uuid.New(),
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What is 2+2?"},
		},
		MaxTokens:   16,
		Temperature: 0.0,
	}
}

// BenchmarkLLMDispatch_RequestBuild measures only the request-construction cost
// — convert HelixCode's LLMRequest into the provider wire format. This is the
// pure CPU/alloc slice of dispatch, isolated from any I/O.
func BenchmarkLLMDispatch_RequestBuild(b *testing.B) {
	provider, cleanup := newBenchOpenAIProvider(b)
	defer cleanup()
	req := benchLLMRequest()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.convertToOpenAIRequest(req); err != nil {
			b.Fatalf("convertToOpenAIRequest: %v", err)
		}
	}
}

// BenchmarkLLMDispatch_RoundTrip measures a full Generate() call against the
// local httptest server: convert request -> marshal JSON -> HTTP dispatch ->
// decode response -> convert response. Network latency is ~0 so the number is
// HelixCode's own per-call overhead — the B03 hot path P1-T01 targets.
func BenchmarkLLMDispatch_RoundTrip(b *testing.B) {
	provider, cleanup := newBenchOpenAIProvider(b)
	defer cleanup()
	req := benchLLMRequest()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := provider.Generate(ctx, req)
		if err != nil {
			b.Fatalf("Generate: %v", err)
		}
		if resp.Content == "" {
			b.Fatal("empty response content")
		}
	}
}

// BenchmarkLLMDispatch_RoundTripParallel measures concurrent dispatch — surfaces
// the per-call TLS/connection cost the shared tuned transport (P1-T01) removes
// on bursts of rapid-fire requests.
func BenchmarkLLMDispatch_RoundTripParallel(b *testing.B) {
	provider, cleanup := newBenchOpenAIProvider(b)
	defer cleanup()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := benchLLMRequest()
		for pb.Next() {
			if _, err := provider.Generate(ctx, req); err != nil {
				b.Fatalf("Generate: %v", err)
			}
		}
	})
}
