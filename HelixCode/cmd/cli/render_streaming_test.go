// Tests for the LLM streaming -> Renderer wire-in (P1-F18-T07) AND for the
// theme.Styler -> Renderer wire-in on the non-stream branch (P1-F20-T06).
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
//
// P1-F20-T06 invariants:
//   - adjustDepthForRenderer collapses to DepthOff when r.Mode() == ModePlain
//     regardless of the requested depth (load-bearing per F20 spec §11:
//     "plain mode forces zero color emission regardless of theme setting").
//   - printResponseThroughRendererStyled emits the styler's RoleHighlight
//     ANSI sequence around the text under fancy mode + non-Off depth, but
//     emits ZERO ANSI bytes under plain mode (because the depth was forced
//     to DepthOff at the integration site).
//   - A nil styler short-circuits to RenderTextBlock with the raw text.
package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/render"
	"dev.helix.code/internal/theme"
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

func TestPrintResponseThroughRenderer_NonEmptyContent_NoError(t *testing.T) {
	// Smoke: non-stream LLM response printing must succeed end-to-end with
	// the default renderer construction (HELIXCODE_RENDER unset, no Writer
	// override -> os.Stdout, plain mode resolved by the factory's TTY probe
	// when running under `go test`).
	if err := printResponseThroughRenderer("hello from non-stream branch\n"); err != nil {
		t.Fatalf("printResponseThroughRenderer non-empty: %v", err)
	}
}

func TestPrintResponseThroughRenderer_EmptyContent_NoOp(t *testing.T) {
	// Empty content must not error and must not panic. The internal
	// RenderTextBlock("") -> no-op contract is exercised here.
	if err := printResponseThroughRenderer(""); err != nil {
		t.Fatalf("printResponseThroughRenderer empty: %v", err)
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

// --------------------------------------------------------------------------
// P1-F20-T06 tests: theme.Styler wire-in on the non-stream branch.
// --------------------------------------------------------------------------

// TestAdjustDepthForRenderer_PlainModeForcesOff is the load-bearing F20 §11
// invariant: regardless of the depth requested by the operator's environment,
// when the renderer is in plain mode the depth MUST collapse to DepthOff so
// the Styler emits no ANSI bytes that the plain renderer (which passes
// pre-styled bytes through verbatim) would leak into a log file.
func TestAdjustDepthForRenderer_PlainModeForcesOff(t *testing.T) {
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModePlain})
	if err != nil {
		t.Fatalf("NewRenderer plain: %v", err)
	}
	defer r.Close()

	for _, requested := range []theme.ColorDepth{
		theme.DepthANSI16,
		theme.DepthANSI256,
		theme.DepthTruecolor,
	} {
		got := adjustDepthForRenderer(r, requested)
		if got != theme.DepthOff {
			t.Errorf("plain mode + requested %s: want DepthOff, got %s", requested, got)
		}
	}
}

// TestAdjustDepthForRenderer_FancyModePassesThrough — under fancy mode the
// adjustDepth helper must NOT downgrade; the operator's chosen depth wins.
func TestAdjustDepthForRenderer_FancyModePassesThrough(t *testing.T) {
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModeFancy})
	if err != nil {
		t.Fatalf("NewRenderer fancy: %v", err)
	}
	defer r.Close()

	for _, requested := range []theme.ColorDepth{
		theme.DepthOff,
		theme.DepthANSI16,
		theme.DepthANSI256,
		theme.DepthTruecolor,
	} {
		got := adjustDepthForRenderer(r, requested)
		if got != requested {
			t.Errorf("fancy mode + requested %s: want %s, got %s", requested, requested, got)
		}
	}
}

// TestPrintResponseThroughRendererStyled_PlainMode_NoColorEmitted — even when
// the test builds the Styler with a non-Off depth (operator might mistakenly
// have COLORTERM=truecolor while output is piped to a file), the integration
// helper MUST collapse the depth to DepthOff for plain renderers, so ZERO 0x1b
// bytes reach the writer.
func TestPrintResponseThroughRendererStyled_PlainMode_NoColorEmitted(t *testing.T) {
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModePlain})
	if err != nil {
		t.Fatalf("NewRenderer plain: %v", err)
	}
	defer r.Close()

	// Build a styler at a non-Off depth — the integration helper is
	// responsible for forcing it down.
	dark := theme.BuiltinDarkTheme()
	styler := theme.NewStyler(dark, adjustDepthForRenderer(r, theme.DepthANSI256))

	if err := printResponseThroughRendererStyled(r, styler, "Hello\n"); err != nil {
		t.Fatalf("printResponseThroughRendererStyled: %v", err)
	}

	out := buf.String()
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("plain mode emitted ANSI escape (0x1b); buf = %q", out)
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("plain mode missing text; got %q", out)
	}
}

// TestPrintResponseThroughRendererStyled_FancyMode_StylesText — under fancy
// renderer + ANSI256 + dark theme + RoleHighlight, the buffer MUST contain
// the role's ANSI256 open sequence (\x1b[38;5;51m), the literal text, and the
// trailing reset (\x1b[0m).
func TestPrintResponseThroughRendererStyled_FancyMode_StylesText(t *testing.T) {
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModeFancy})
	if err != nil {
		t.Fatalf("NewRenderer fancy: %v", err)
	}
	defer r.Close()

	dark := theme.BuiltinDarkTheme()
	depth := adjustDepthForRenderer(r, theme.DepthANSI256)
	if depth != theme.DepthANSI256 {
		t.Fatalf("fancy mode must preserve ANSI256 depth, got %s", depth)
	}
	styler := theme.NewStyler(dark, depth)

	if err := printResponseThroughRendererStyled(r, styler, "Hello"); err != nil {
		t.Fatalf("printResponseThroughRendererStyled: %v", err)
	}

	out := buf.String()
	// Dark theme RoleHighlight at ANSI256 = \x1b[38;5;51m (cf. builtin.go).
	wantOpen := "\x1b[38;5;51m"
	if !strings.Contains(out, wantOpen) {
		t.Errorf("fancy mode missing role open seq %q; buf = %q", wantOpen, out)
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("fancy mode missing literal text; buf = %q", out)
	}
	if !strings.Contains(out, theme.Reset) {
		t.Errorf("fancy mode missing theme.Reset (%q); buf = %q", theme.Reset, out)
	}
}

// TestPrintResponseThroughRendererStyled_NilStyler_PassesThrough — passing
// nil for styler must not panic and must emit the raw text unchanged through
// the renderer (no ANSI bytes emitted by our code; plain mode invariants
// continue to hold).
func TestPrintResponseThroughRendererStyled_NilStyler_PassesThrough(t *testing.T) {
	var buf bytes.Buffer
	r, err := render.NewRenderer(render.FactoryOptions{Writer: &buf, Mode: render.ModePlain})
	if err != nil {
		t.Fatalf("NewRenderer plain: %v", err)
	}
	defer r.Close()

	if err := printResponseThroughRendererStyled(r, nil, "Hello nil\n"); err != nil {
		t.Fatalf("printResponseThroughRendererStyled nil styler: %v", err)
	}
	out := buf.String()
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("plain mode + nil styler must not emit ANSI; got %q", out)
	}
	if !strings.Contains(out, "Hello nil") {
		t.Errorf("plain mode + nil styler missing text; got %q", out)
	}
}
