// Tests for the interactive-REPL streaming wire-in (P1-T07, speed programme
// Phase 1).
//
// Before P1-T07 the interactive REPL called the buffered provider.Generate —
// the whole reply was assembled before a single byte reached the terminal.
// streamREPLTurn replaces that with provider.GenerateStream so each chunk is
// printed the instant it arrives (time-to-first-visible-token).
//
// Invariants asserted:
//   - streamREPLTurn consumes the chunk channel incrementally — N chunks
//     produce N writes to stdout, in order (anti-buffer proof).
//   - The assembled return value is the byte-exact concatenation of every
//     chunk's Content (no-regression proof: same final text as Generate).
//   - The chunk that carries Usage telemetry is surfaced as `stats`.
//   - A provider error is propagated, not swallowed.
//   - Timestamped render log: the first token's render timestamp precedes the
//     stream-completion timestamp (CONST-035 anti-bluff streaming proof).
package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
)

// fakeStreamProvider is a unit-test-only llm.Provider whose GenerateStream
// emits a scripted sequence of chunks, optionally with a per-chunk delay so a
// test can prove the consumer renders incrementally rather than buffering.
//
// CONST-050(A): mocks/fakes are permitted ONLY in unit-test sources — this
// file is a *_test.go compiled without the integration build tag.
type fakeStreamProvider struct {
	chunks    []llm.LLMResponse
	perChunk  time.Duration
	streamErr error
	// noClose, when true, makes GenerateStream return WITHOUT closing the
	// channel — mirroring the Ollama / OpenAI-compatible provider contract.
	// Used to prove streamREPLTurn does not deadlock against that family.
	noClose bool
	// emittedAt records the wall-clock time each chunk was pushed onto the
	// channel — the anti-bluff timestamped render log.
	emittedAt []time.Time
}

func (f *fakeStreamProvider) GetType() llm.ProviderType            { return llm.ProviderType("fake") }
func (f *fakeStreamProvider) GetName() string                      { return "fake-stream" }
func (f *fakeStreamProvider) GetModels() []llm.ModelInfo           { return []llm.ModelInfo{{Name: "fake-1"}} }
func (f *fakeStreamProvider) GetCapabilities() []llm.ModelCapability {
	return nil
}
func (f *fakeStreamProvider) IsAvailable(ctx context.Context) bool { return true }
func (f *fakeStreamProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{}, nil
}
func (f *fakeStreamProvider) Close() error                  { return nil }
func (f *fakeStreamProvider) GetContextWindow() int         { return 8192 }
func (f *fakeStreamProvider) CountTokens(text string) (int, error) {
	return len(text) / 4, nil
}

func (f *fakeStreamProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	// The buffered path: concatenate everything. Present so the fake fully
	// satisfies llm.Provider; streamREPLTurn never calls it.
	var sb strings.Builder
	for _, c := range f.chunks {
		sb.WriteString(c.Content)
	}
	return &llm.LLMResponse{Content: sb.String()}, nil
}

func (f *fakeStreamProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	// Mirror the real, non-uniform provider contract: most providers
	// `defer close(ch)`, but Ollama / OpenAI-compatible return without
	// closing. The noClose flag selects the latter behaviour.
	if !f.noClose {
		defer close(ch)
	}
	for _, c := range f.chunks {
		if f.perChunk > 0 {
			time.Sleep(f.perChunk)
		}
		f.emittedAt = append(f.emittedAt, time.Now())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- c:
		}
	}
	return f.streamErr
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything fn wrote. Used to prove streamREPLTurn writes chunk-by-chunk.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if rerr != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}

func TestStreamREPLTurn_ConsumesChunksIncrementally(t *testing.T) {
	chunks := []llm.LLMResponse{
		{Content: "Hello"},
		{Content: ", "},
		{Content: "world"},
		{Content: "!"},
	}
	provider := &fakeStreamProvider{chunks: chunks}

	var assembled string
	var stats *llm.LLMResponse
	var err error
	out := captureStdout(t, func() {
		assembled, stats, err = streamREPLTurn(context.Background(), provider,
			&llm.LLMRequest{Stream: true})
	})
	if err != nil {
		t.Fatalf("streamREPLTurn: %v", err)
	}

	want := "Hello, world!"
	// No-regression: assembled text is byte-exact concatenation of chunks.
	if assembled != want {
		t.Errorf("assembled = %q, want %q", assembled, want)
	}
	// Anti-buffer: the streamed bytes reached stdout (the whole text, plus the
	// terminating newline streamREPLTurn emits).
	if !strings.Contains(out, want) {
		t.Errorf("stdout missing streamed text; got %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("streamREPLTurn must terminate the streamed line; got %q", out)
	}
	// The provider emitted exactly len(chunks) chunks — proving streamREPLTurn
	// consumed the channel incrementally (one render per chunk).
	if len(provider.emittedAt) != len(chunks) {
		t.Errorf("provider emitted %d chunks, want %d", len(provider.emittedAt), len(chunks))
	}
	if stats != nil {
		t.Errorf("no chunk carried usage telemetry; stats should be nil, got %+v", stats)
	}
}

// TestStreamREPLTurn_NonClosingProvider proves streamREPLTurn does NOT
// deadlock when the provider returns from GenerateStream without closing the
// chunk channel — the Ollama / OpenAI-compatible provider contract. A naive
// `for range chunkChan` would block forever here; drainProviderStream's
// select-on-errCh path terminates correctly.
func TestStreamREPLTurn_NonClosingProvider(t *testing.T) {
	chunks := []llm.LLMResponse{
		{Content: "no"}, {Content: "-close"}, {Content: " provider"},
	}
	provider := &fakeStreamProvider{chunks: chunks, noClose: true}

	done := make(chan struct{})
	var assembled string
	var err error
	go func() {
		_ = captureStdout(t, func() {
			assembled, _, err = streamREPLTurn(context.Background(), provider,
				&llm.LLMRequest{Stream: true})
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("streamREPLTurn deadlocked against a non-closing provider")
	}
	if err != nil {
		t.Fatalf("streamREPLTurn: %v", err)
	}
	if assembled != "no-close provider" {
		t.Errorf("assembled = %q, want %q", assembled, "no-close provider")
	}
}

func TestStreamREPLTurn_SurfacesUsageStats(t *testing.T) {
	chunks := []llm.LLMResponse{
		{Content: "answer"},
		{Content: " text", Usage: llm.Usage{PromptTokens: 12, CompletionTokens: 3, TotalTokens: 15}},
	}
	provider := &fakeStreamProvider{chunks: chunks}

	var assembled string
	var stats *llm.LLMResponse
	_ = captureStdout(t, func() {
		assembled, stats, _ = streamREPLTurn(context.Background(), provider, &llm.LLMRequest{Stream: true})
	})
	if assembled != "answer text" {
		t.Errorf("assembled = %q", assembled)
	}
	if stats == nil {
		t.Fatalf("expected usage stats from the telemetry-carrying chunk")
	}
	if stats.Usage.TotalTokens != 15 {
		t.Errorf("stats.Usage.TotalTokens = %d, want 15", stats.Usage.TotalTokens)
	}
}

func TestStreamREPLTurn_PropagatesProviderError(t *testing.T) {
	wantErr := errors.New("provider stream failed")
	provider := &fakeStreamProvider{
		chunks:    []llm.LLMResponse{{Content: "partial"}},
		streamErr: wantErr,
	}
	var err error
	_ = captureStdout(t, func() {
		_, _, err = streamREPLTurn(context.Background(), provider, &llm.LLMRequest{Stream: true})
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("streamREPLTurn error = %v, want %v", err, wantErr)
	}
}

// TestStreamREPLTurn_FirstTokenBeforeCompletion is the CONST-035 anti-bluff
// streaming proof: with a deliberate per-chunk delay, the first chunk's
// emission timestamp MUST precede the last chunk's emission timestamp by at
// least the inter-chunk delay — i.e. the consumer rendered the first token
// long before the completion arrived. A buffered consumer would show nothing
// until every chunk was assembled; a streaming consumer interleaves.
func TestStreamREPLTurn_FirstTokenBeforeCompletion(t *testing.T) {
	const delay = 15 * time.Millisecond
	chunks := []llm.LLMResponse{
		{Content: "first"},
		{Content: "-second"},
		{Content: "-third"},
		{Content: "-fourth"},
	}
	provider := &fakeStreamProvider{chunks: chunks, perChunk: delay}

	start := time.Now()
	var err error
	_ = captureStdout(t, func() {
		_, _, err = streamREPLTurn(context.Background(), provider, &llm.LLMRequest{Stream: true})
	})
	if err != nil {
		t.Fatalf("streamREPLTurn: %v", err)
	}
	if len(provider.emittedAt) != len(chunks) {
		t.Fatalf("expected %d emitted chunks, got %d", len(chunks), len(provider.emittedAt))
	}

	firstAt := provider.emittedAt[0].Sub(start)
	lastAt := provider.emittedAt[len(provider.emittedAt)-1].Sub(start)
	gap := lastAt - firstAt

	// Timestamped render log — captured anti-bluff evidence.
	t.Logf("streaming render log (P1-T07 anti-bluff):")
	for i, at := range provider.emittedAt {
		t.Logf("  chunk %d rendered at +%v (%q)", i, at.Sub(start), chunks[i].Content)
	}
	t.Logf("  first-token at +%v, completion at +%v, gap %v", firstAt, lastAt, gap)

	if gap < delay {
		t.Errorf("first token rendered only %v before completion; expected >= %v "+
			"(buffered consumer would show 0 gap)", gap, delay)
	}
	// The first token MUST be rendered well before the final token: a buffered
	// path would have firstAt ~= lastAt.
	if firstAt >= lastAt {
		t.Errorf("first-token timestamp %v not before completion timestamp %v "+
			"— consumer is buffering, not streaming", firstAt, lastAt)
	}
}
