// Tests for the TUI chat streaming wire-in (P1-T07, speed programme Phase 1).
//
// Before P1-T07 sendChatMessage called the buffered provider.Generate — the
// whole reply was assembled before a single byte reached the chat panel, and
// the call blocked the tview event loop. consumeChatStream replaces that with
// provider.GenerateStream, invoking an onChunk callback per chunk so the panel
// grows token-by-token.
//
// Invariants asserted:
//   - consumeChatStream invokes onChunk once per non-empty chunk, in order
//     (anti-buffer proof: N chunks -> N callbacks).
//   - The concatenation of every onChunk argument is the byte-exact text of
//     the chunks (no-regression proof: same final text as Generate).
//   - The telemetry-carrying chunk's TotalTokens is returned.
//   - A provider error is returned, not swallowed.
//   - Timestamped render log: the first onChunk fires before the stream
//     completes (CONST-035 anti-bluff streaming proof).
package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
)

// fakeStreamProvider is a unit-test-only llm.Provider whose GenerateStream
// emits a scripted chunk sequence with an optional per-chunk delay.
// CONST-050(A): fakes are permitted ONLY in *_test.go unit sources.
type fakeStreamProvider struct {
	chunks    []llm.LLMResponse
	perChunk  time.Duration
	streamErr error
	// noClose mirrors the Ollama / OpenAI-compatible provider contract:
	// GenerateStream returns WITHOUT closing the channel.
	noClose   bool
	emittedAt []time.Time
}

func (f *fakeStreamProvider) GetType() llm.ProviderType              { return llm.ProviderType("fake") }
func (f *fakeStreamProvider) GetName() string                        { return "fake-stream" }
func (f *fakeStreamProvider) GetModels() []llm.ModelInfo             { return []llm.ModelInfo{{Name: "fake-1"}} }
func (f *fakeStreamProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (f *fakeStreamProvider) IsAvailable(ctx context.Context) bool   { return true }
func (f *fakeStreamProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{}, nil
}
func (f *fakeStreamProvider) Close() error                          { return nil }
func (f *fakeStreamProvider) GetContextWindow() int                 { return 8192 }
func (f *fakeStreamProvider) CountTokens(text string) (int, error)  { return len(text) / 4, nil }

func (f *fakeStreamProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	var sb strings.Builder
	for _, c := range f.chunks {
		sb.WriteString(c.Content)
	}
	return &llm.LLMResponse{Content: sb.String()}, nil
}

func (f *fakeStreamProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
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

func TestConsumeChatStream_IncrementalCallbacks(t *testing.T) {
	chunks := []llm.LLMResponse{
		{Content: "Hel"},
		{Content: "lo "},
		{Content: "TUI"},
	}
	provider := &fakeStreamProvider{chunks: chunks}

	var rendered []string
	streamErr, totalTokens := consumeChatStream(context.Background(), provider,
		&llm.LLMRequest{Stream: true}, func(content string) {
			rendered = append(rendered, content)
		})
	if streamErr != nil {
		t.Fatalf("consumeChatStream: %v", streamErr)
	}
	// Anti-buffer: one callback per non-empty chunk, in order.
	if len(rendered) != len(chunks) {
		t.Fatalf("got %d onChunk callbacks, want %d", len(rendered), len(chunks))
	}
	for i, c := range chunks {
		if rendered[i] != c.Content {
			t.Errorf("callback %d = %q, want %q", i, rendered[i], c.Content)
		}
	}
	// No-regression: concatenation equals the buffered Generate result.
	if got := strings.Join(rendered, ""); got != "Hello TUI" {
		t.Errorf("assembled = %q, want %q", got, "Hello TUI")
	}
	if totalTokens != 0 {
		t.Errorf("no chunk carried token telemetry; totalTokens = %d, want 0", totalTokens)
	}
}

func TestConsumeChatStream_ReturnsTokenCount(t *testing.T) {
	chunks := []llm.LLMResponse{
		{Content: "reply"},
		{Content: " done", Usage: llm.Usage{TotalTokens: 42}},
	}
	provider := &fakeStreamProvider{chunks: chunks}
	_, totalTokens := consumeChatStream(context.Background(), provider,
		&llm.LLMRequest{Stream: true}, func(string) {})
	if totalTokens != 42 {
		t.Errorf("totalTokens = %d, want 42", totalTokens)
	}
}

// TestConsumeChatStream_NonClosingProvider proves consumeChatStream does NOT
// deadlock when the provider returns without closing the channel (the Ollama /
// OpenAI-compatible contract).
func TestConsumeChatStream_NonClosingProvider(t *testing.T) {
	chunks := []llm.LLMResponse{{Content: "no"}, {Content: "-close"}}
	provider := &fakeStreamProvider{chunks: chunks, noClose: true}

	done := make(chan struct{})
	var rendered []string
	var streamErr error
	go func() {
		streamErr, _ = consumeChatStream(context.Background(), provider,
			&llm.LLMRequest{Stream: true}, func(c string) { rendered = append(rendered, c) })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumeChatStream deadlocked against a non-closing provider")
	}
	if streamErr != nil {
		t.Fatalf("consumeChatStream: %v", streamErr)
	}
	if got := strings.Join(rendered, ""); got != "no-close" {
		t.Errorf("assembled = %q, want %q", got, "no-close")
	}
}

func TestConsumeChatStream_PropagatesError(t *testing.T) {
	wantErr := errors.New("tui stream failed")
	provider := &fakeStreamProvider{
		chunks:    []llm.LLMResponse{{Content: "partial"}},
		streamErr: wantErr,
	}
	streamErr, _ := consumeChatStream(context.Background(), provider,
		&llm.LLMRequest{Stream: true}, func(string) {})
	if !errors.Is(streamErr, wantErr) {
		t.Fatalf("consumeChatStream error = %v, want %v", streamErr, wantErr)
	}
}

// TestConsumeChatStream_FirstChunkBeforeCompletion is the CONST-035 anti-bluff
// streaming proof for the TUI: with a per-chunk delay, the first onChunk
// callback fires well before the last chunk is emitted. A buffered consumer
// would deliver nothing until every chunk was assembled.
func TestConsumeChatStream_FirstChunkBeforeCompletion(t *testing.T) {
	const delay = 15 * time.Millisecond
	chunks := []llm.LLMResponse{
		{Content: "a"}, {Content: "b"}, {Content: "c"}, {Content: "d"},
	}
	provider := &fakeStreamProvider{chunks: chunks, perChunk: delay}

	start := time.Now()
	type renderEvent struct {
		at      time.Duration
		content string
	}
	var events []renderEvent
	streamErr, _ := consumeChatStream(context.Background(), provider,
		&llm.LLMRequest{Stream: true}, func(content string) {
			events = append(events, renderEvent{at: time.Since(start), content: content})
		})
	if streamErr != nil {
		t.Fatalf("consumeChatStream: %v", streamErr)
	}
	if len(events) != len(chunks) {
		t.Fatalf("got %d render events, want %d", len(events), len(chunks))
	}

	// Timestamped render log — captured anti-bluff evidence.
	t.Logf("TUI streaming render log (P1-T07 anti-bluff):")
	for i, e := range events {
		t.Logf("  chunk %d rendered at +%v (%q)", i, e.at, e.content)
	}
	completionAt := provider.emittedAt[len(provider.emittedAt)-1].Sub(start)
	t.Logf("  first-chunk rendered at +%v, stream completion at +%v", events[0].at, completionAt)

	if events[0].at >= completionAt {
		t.Errorf("first chunk rendered at +%v, not before completion at +%v "+
			"— consumer is buffering, not streaming", events[0].at, completionAt)
	}
	if gap := events[len(events)-1].at - events[0].at; gap < delay {
		t.Errorf("first-to-last render gap %v < inter-chunk delay %v — chunks not "+
			"rendered incrementally", gap, delay)
	}
}
