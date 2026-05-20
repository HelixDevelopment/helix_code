// Tests for the desktop chat streaming wire-in (P1-T07, speed programme
// Phase 1).
//
// Before P1-T07 the desktop Send-Message handler called the buffered
// provider.Generate — the whole reply was assembled before a single byte
// reached the chat panel. consumeDesktopChatStream replaces that with
// provider.GenerateStream, invoking an onChunk callback per chunk so the
// (*widget.Entry) chat history grows token-by-token.
//
// This test file carries NO build tag (matching chat_stream.go) so it runs
// under both GUI and `-tags nogui` builds — no X11 display required.
//
// Invariants asserted:
//   - consumeDesktopChatStream invokes onChunk once per non-empty chunk, in
//     order (anti-buffer proof).
//   - The concatenation of onChunk arguments is the byte-exact chunk text
//     (no-regression proof: same final text as Generate).
//   - A provider error is returned, not swallowed.
//   - Timestamped render log: the first onChunk fires before stream
//     completion (CONST-035 anti-bluff streaming proof).
package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
)

// fakeDesktopStreamProvider is a unit-test-only llm.Provider whose
// GenerateStream emits a scripted chunk sequence with an optional per-chunk
// delay. CONST-050(A): fakes are permitted ONLY in *_test.go unit sources.
type fakeDesktopStreamProvider struct {
	chunks    []llm.LLMResponse
	perChunk  time.Duration
	streamErr error
	// noClose mirrors the Ollama / OpenAI-compatible provider contract:
	// GenerateStream returns WITHOUT closing the channel.
	noClose   bool
	emittedAt []time.Time
}

func (f *fakeDesktopStreamProvider) GetType() llm.ProviderType  { return llm.ProviderType("fake") }
func (f *fakeDesktopStreamProvider) GetName() string            { return "fake-stream" }
func (f *fakeDesktopStreamProvider) GetModels() []llm.ModelInfo { return []llm.ModelInfo{{Name: "fake-1"}} }
func (f *fakeDesktopStreamProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (f *fakeDesktopStreamProvider) IsAvailable(ctx context.Context) bool   { return true }
func (f *fakeDesktopStreamProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{}, nil
}
func (f *fakeDesktopStreamProvider) Close() error                         { return nil }
func (f *fakeDesktopStreamProvider) GetContextWindow() int                { return 8192 }
func (f *fakeDesktopStreamProvider) CountTokens(text string) (int, error) { return len(text) / 4, nil }

func (f *fakeDesktopStreamProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	var sb strings.Builder
	for _, c := range f.chunks {
		sb.WriteString(c.Content)
	}
	return &llm.LLMResponse{Content: sb.String()}, nil
}

func (f *fakeDesktopStreamProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
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

func TestConsumeDesktopChatStream_IncrementalCallbacks(t *testing.T) {
	chunks := []llm.LLMResponse{
		{Content: "Hi "},
		{Content: "from "},
		{Content: "desktop"},
	}
	provider := &fakeDesktopStreamProvider{chunks: chunks}

	var rendered []string
	err := consumeDesktopChatStream(context.Background(), provider,
		&llm.LLMRequest{Stream: true}, func(content string) {
			rendered = append(rendered, content)
		})
	if err != nil {
		t.Fatalf("consumeDesktopChatStream: %v", err)
	}
	if len(rendered) != len(chunks) {
		t.Fatalf("got %d onChunk callbacks, want %d", len(rendered), len(chunks))
	}
	for i, c := range chunks {
		if rendered[i] != c.Content {
			t.Errorf("callback %d = %q, want %q", i, rendered[i], c.Content)
		}
	}
	// No-regression: concatenation equals the buffered Generate result.
	if got := strings.Join(rendered, ""); got != "Hi from desktop" {
		t.Errorf("assembled = %q, want %q", got, "Hi from desktop")
	}
}

// TestConsumeDesktopChatStream_NonClosingProvider proves the desktop consumer
// does NOT deadlock when the provider returns without closing the channel (the
// Ollama / OpenAI-compatible contract).
func TestConsumeDesktopChatStream_NonClosingProvider(t *testing.T) {
	chunks := []llm.LLMResponse{{Content: "no"}, {Content: "-close"}}
	provider := &fakeDesktopStreamProvider{chunks: chunks, noClose: true}

	done := make(chan struct{})
	var rendered []string
	var err error
	go func() {
		err = consumeDesktopChatStream(context.Background(), provider,
			&llm.LLMRequest{Stream: true}, func(c string) { rendered = append(rendered, c) })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumeDesktopChatStream deadlocked against a non-closing provider")
	}
	if err != nil {
		t.Fatalf("consumeDesktopChatStream: %v", err)
	}
	if got := strings.Join(rendered, ""); got != "no-close" {
		t.Errorf("assembled = %q, want %q", got, "no-close")
	}
}

func TestConsumeDesktopChatStream_PropagatesError(t *testing.T) {
	wantErr := errors.New("desktop stream failed")
	provider := &fakeDesktopStreamProvider{
		chunks:    []llm.LLMResponse{{Content: "partial"}},
		streamErr: wantErr,
	}
	err := consumeDesktopChatStream(context.Background(), provider,
		&llm.LLMRequest{Stream: true}, func(string) {})
	if !errors.Is(err, wantErr) {
		t.Fatalf("consumeDesktopChatStream error = %v, want %v", err, wantErr)
	}
}

// TestConsumeDesktopChatStream_FirstChunkBeforeCompletion is the CONST-035
// anti-bluff streaming proof for the desktop surface: with a per-chunk delay,
// the first onChunk callback fires before the last chunk is emitted. A
// buffered consumer would deliver nothing until every chunk was assembled.
func TestConsumeDesktopChatStream_FirstChunkBeforeCompletion(t *testing.T) {
	const delay = 15 * time.Millisecond
	chunks := []llm.LLMResponse{
		{Content: "w"}, {Content: "x"}, {Content: "y"}, {Content: "z"},
	}
	provider := &fakeDesktopStreamProvider{chunks: chunks, perChunk: delay}

	start := time.Now()
	type renderEvent struct {
		at      time.Duration
		content string
	}
	var events []renderEvent
	err := consumeDesktopChatStream(context.Background(), provider,
		&llm.LLMRequest{Stream: true}, func(content string) {
			events = append(events, renderEvent{at: time.Since(start), content: content})
		})
	if err != nil {
		t.Fatalf("consumeDesktopChatStream: %v", err)
	}
	if len(events) != len(chunks) {
		t.Fatalf("got %d render events, want %d", len(events), len(chunks))
	}

	// Timestamped render log — captured anti-bluff evidence.
	t.Logf("desktop streaming render log (P1-T07 anti-bluff):")
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
