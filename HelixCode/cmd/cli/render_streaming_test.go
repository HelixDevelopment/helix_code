// Tests for the LLM streaming -> Renderer wire-in (P1-F18-T07).
//
// The streaming hot path in handleGenerate is hard to unit-test directly
// because it depends on a real llm.Provider. We extract the inner loop into
// a small helper, streamToRenderer, and exercise that helper under both
// fancy (ansiRenderer) and plain (plainRenderer) modes plus an error path.
//
// Invariants asserted (cf. spec §11.6):
//   - Plain renderer: emits chunk text in order, ZERO 0x1b bytes (ANSI escapes),
//     ZERO 0x0d bytes (carriage returns), final transcript ends with \n.
//   - Fancy renderer: emits ANSI hide-cursor (\x1b[?25l), per-token
//     CR+clear-line (\r\x1b[K) sequence, and a final \n on Commit.
//   - Begin/Commit are balanced even when the producer goroutine returns an
//     error mid-stream: Commit MUST run, the final \n MUST be emitted.
package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/render"
)

// produceChunks ships the supplied content strings as LLMResponse values onto
// ch (one chunk per slice element) and then closes ch. Mirrors the surface
// area of llm.Provider.GenerateStream from the renderer's point of view.
func produceChunks(content []string) func(chan<- llm.LLMResponse) error {
	return func(ch chan<- llm.LLMResponse) error {
		for _, c := range content {
			ch <- llm.LLMResponse{Content: c}
		}
		close(ch)
		return nil
	}
}

// runStream is a thin test driver: spins up a goroutine that pumps chunks
// onto a channel via producer, then calls streamToRenderer with the channel.
// Returns whatever streamToRenderer returns.
func runStream(t *testing.T, r render.Renderer, blockID string, producer func(chan<- llm.LLMResponse) error) error {
	t.Helper()
	ch := make(chan llm.LLMResponse, 8)
	errCh := make(chan error, 1)
	go func() { errCh <- producer(ch) }()
	if err := streamToRenderer(context.Background(), ch, r, blockID); err != nil {
		<-errCh
		return err
	}
	return <-errCh
}

func TestStreamingThroughRenderer_PlainMode_NoANSI(t *testing.T) {
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModePlain})
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	defer r.Close()

	chunks := []string{"Hello", " ", "world", "!", " done"}
	if err := runStream(t, r, "test-plain", produceChunks(chunks)); err != nil {
		t.Fatalf("streamToRenderer: %v", err)
	}

	out := buf.String()

	// Zero-ANSI invariant: plain renderer never emits 0x1b.
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("plain mode emitted ANSI escape (0x1b); buf = %q", out)
	}
	// Zero-CR invariant: plain renderer never emits 0x0d.
	if strings.ContainsRune(out, 0x0d) {
		t.Errorf("plain mode emitted carriage return (0x0d); buf = %q", out)
	}
	// All chunk text must be present, in order.
	want := strings.Join(chunks, "")
	if !strings.Contains(out, want) {
		t.Errorf("plain mode missing concatenated chunk text; want substring %q, got %q", want, out)
	}
	// Plain renderer flushes a trailing \n on Commit when content was buffered.
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("plain mode transcript must end with newline; got %q", out)
	}
}

func TestStreamingThroughRenderer_FancyMode_HasANSIControl(t *testing.T) {
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModeFancy})
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	defer r.Close()

	chunks := []string{"foo", "bar", "baz"}
	if err := runStream(t, r, "test-fancy", produceChunks(chunks)); err != nil {
		t.Fatalf("streamToRenderer: %v", err)
	}

	out := buf.String()

	// Fancy renderer hides cursor on first Begin.
	if !strings.Contains(out, "\x1b[?25l") {
		t.Errorf("fancy mode missing hide-cursor (\\x1b[?25l); buf = %q", out)
	}
	// Each WriteToken with no embedded \n emits CR + ANSI clear-line + line.
	if !strings.Contains(out, "\r\x1b[K") {
		t.Errorf("fancy mode missing CR+clear-line (\\r\\x1b[K); buf = %q", out)
	}
	// Commit emits trailing \n for the in-progress line.
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("fancy mode transcript must end with newline; got %q", out)
	}
	// Concatenated chunk text must appear (the final CR+clear+line carries
	// the full accumulated streamingLine).
	want := strings.Join(chunks, "")
	if !strings.Contains(out, want) {
		t.Errorf("fancy mode missing concatenated chunk text; want substring %q, got %q", want, out)
	}
}

func TestStreamingThroughRenderer_BeginCommit_BalancedAcrossErrors(t *testing.T) {
	// Producer ships two chunks then closes the channel WITH an error
	// returned to its own caller. From streamToRenderer's perspective,
	// closing the channel is the signal to stop reading and Commit. The
	// invariant under test: even when the producer's goroutine reports an
	// error, the renderer-side Commit still runs and the final \n still
	// reaches the writer.
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModePlain})
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	defer r.Close()

	wantErr := errors.New("midstream provider failure")
	ch := make(chan llm.LLMResponse, 4)
	producerErrCh := make(chan error, 1)
	go func() {
		ch <- llm.LLMResponse{Content: "partial"}
		ch <- llm.LLMResponse{Content: "-output"}
		close(ch)
		producerErrCh <- wantErr
	}()
	// streamToRenderer reads the (closed) channel cleanly and returns nil;
	// the renderer's deferred Commit MUST still flush the buffered line.
	if err := streamToRenderer(context.Background(), ch, r, "test-err"); err != nil {
		t.Fatalf("streamToRenderer must succeed when channel closes cleanly: got %v", err)
	}
	if perr := <-producerErrCh; !errors.Is(perr, wantErr) {
		t.Fatalf("producer error mismatch: want %v, got %v", wantErr, perr)
	}

	out := buf.String()
	if !strings.Contains(out, "partial-output") {
		t.Errorf("expected buffered partial output; got %q", out)
	}
	// Commit ran -> trailing newline emitted -> transcript ends with \n.
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("Commit should emit trailing newline even on producer error; got %q", out)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("plain mode emitted ANSI escape; got %q", out)
	}
}
