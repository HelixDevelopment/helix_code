package render

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestPlainRenderer_Mode(t *testing.T) {
	r := NewPlainRenderer(&bytes.Buffer{})
	if got := r.Mode(); got != ModePlain {
		t.Fatalf("Mode() = %q, want %q", got, ModePlain)
	}
}

func TestPlainRenderer_Compiles_AsRenderer(t *testing.T) {
	var _ Renderer = NewPlainRenderer(&bytes.Buffer{})
}

func TestPlainRenderer_WriteToken_NoNewline_BuffersOnly(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := r.WriteToken("partial"); err != nil {
		t.Fatalf("WriteToken: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer (no newline yet), got %q", buf.String())
	}
}

func TestPlainRenderer_WriteToken_WithNewline_Flushes(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("hello\n"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "hello\n" {
		t.Fatalf("buffer = %q, want %q", got, "hello\n")
	}
}

func TestPlainRenderer_WriteToken_MultipleLines(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("a\nb\nc\n"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "a\nb\nc\n" {
		t.Fatalf("buffer = %q, want %q", got, "a\nb\nc\n")
	}
}

func TestPlainRenderer_WriteToken_PartialThenComplete(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("a"); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer after partial, got %q", buf.String())
	}
	if err := r.WriteToken("bc\n"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "abc\n" {
		t.Fatalf("buffer = %q, want %q", got, "abc\n")
	}
}

func TestPlainRenderer_StripsCarriageReturn(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("hi\rworld\n"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "hiworld\n" {
		t.Fatalf("buffer = %q, want %q", got, "hiworld\n")
	}
	// Hard invariant: no \r in output.
	if strings.ContainsRune(buf.String(), '\r') {
		t.Fatalf("buffer must not contain \\r, got %q", buf.String())
	}
}

func TestPlainRenderer_PassesEmbeddedANSI(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	in := "\x1b[31mred\x1b[0m\n"
	if err := r.WriteToken(in); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if got != in {
		t.Fatalf("buffer = %q, want %q (embedded ANSI must pass through verbatim)", got, in)
	}
	// Sanity: the ESC bytes are present.
	if !strings.Contains(got, "\x1b[31m") || !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("expected embedded ANSI bytes preserved, got %q", got)
	}
}

func TestPlainRenderer_Commit_FlushesIncompleteLine(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("partial"); err != nil {
		t.Fatal(err)
	}
	if err := r.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := buf.String(); got != "partial\n" {
		t.Fatalf("buffer = %q, want %q", got, "partial\n")
	}
}

func TestPlainRenderer_Commit_NoOpIfNothingBuffered(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer, got %q", buf.String())
	}
}

func TestPlainRenderer_Begin_DifferentID_CommitsPrevious(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("partial"); err != nil {
		t.Fatal(err)
	}
	// Switching block ID must commit the previous incomplete line first.
	if err := r.Begin("b"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "partial\n" {
		t.Fatalf("after switch, buffer = %q, want %q", got, "partial\n")
	}
	if err := r.WriteToken("more\n"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "partial\nmore\n" {
		t.Fatalf("buffer = %q, want %q", got, "partial\nmore\n")
	}
}

func TestPlainRenderer_Begin_EmptyBlockID(t *testing.T) {
	r := NewPlainRenderer(&bytes.Buffer{})
	if err := r.Begin(""); !errors.Is(err, ErrEmptyBlockID) {
		t.Fatalf("Begin(\"\") err=%v, want ErrEmptyBlockID", err)
	}
}

func TestPlainRenderer_RenderFrame_AllLinesPrinted(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	frame := Frame{BlockID: "f1", Lines: []string{"alpha", "beta", "gamma"}}
	if err := r.RenderFrame(frame); err != nil {
		t.Fatal(err)
	}
	want := "alpha\nbeta\ngamma\n"
	if got := buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestPlainRenderer_RenderFrame_SecondCall_ReprintsAllLines(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	frame := Frame{BlockID: "f1", Lines: []string{"x", "y"}}
	if err := r.RenderFrame(frame); err != nil {
		t.Fatal(err)
	}
	first := buf.String()
	if first != "x\ny\n" {
		t.Fatalf("first render = %q, want %q", first, "x\ny\n")
	}
	if err := r.RenderFrame(frame); err != nil {
		t.Fatal(err)
	}
	// Plain mode does not diff: second call doubles the size.
	if got := buf.String(); got != "x\ny\nx\ny\n" {
		t.Fatalf("after second render, buffer = %q, want %q", got, "x\ny\nx\ny\n")
	}
}

func TestPlainRenderer_RenderFrame_EmptyBlockID(t *testing.T) {
	r := NewPlainRenderer(&bytes.Buffer{})
	err := r.RenderFrame(Frame{BlockID: "", Lines: []string{"x"}})
	if !errors.Is(err, ErrEmptyBlockID) {
		t.Fatalf("RenderFrame empty BlockID err=%v, want ErrEmptyBlockID", err)
	}
}

func TestPlainRenderer_PostCloseWriteToken_ReturnsError(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("x\n"); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("WriteToken after Close err=%v, want ErrRendererClosed", err)
	}
	if err := r.Begin("a"); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("Begin after Close err=%v, want ErrRendererClosed", err)
	}
	if err := r.Commit(); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("Commit after Close err=%v, want ErrRendererClosed", err)
	}
	if err := r.RenderFrame(Frame{BlockID: "x", Lines: []string{"y"}}); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("RenderFrame after Close err=%v, want ErrRendererClosed", err)
	}
}

func TestPlainRenderer_Close_Idempotent(t *testing.T) {
	r := NewPlainRenderer(&bytes.Buffer{})
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// Load-bearing invariant: with plain text input (no embedded ANSI), the
// renderer's output MUST contain ZERO 0x1b bytes AND ZERO 0x0d bytes.
func TestPlainRenderer_ZeroANSIInvariant(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	// Mix of plain text, partial lines, multi-line, and \r noise.
	inputs := []string{
		"hello",
		" world\n",
		"second line\rwith CR mid-stream\n",
		"a\nb\nc",
		"\rmore\rCR\rnoise\n",
	}
	for _, in := range inputs {
		if err := r.WriteToken(in); err != nil {
			t.Fatalf("WriteToken(%q): %v", in, err)
		}
	}
	if err := r.Commit(); err != nil {
		t.Fatal(err)
	}
	frame := Frame{BlockID: "f", Lines: []string{"line one", "line two", "line three"}}
	if err := r.RenderFrame(frame); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	for i, b := range out {
		if b == 0x1b {
			t.Fatalf("byte at index %d is 0x1b (ANSI ESC); plain renderer emitted ANSI for plain input. Output=%q", i, string(out))
		}
		if b == 0x0d {
			t.Fatalf("byte at index %d is 0x0d (CR); plain renderer emitted CR. Output=%q", i, string(out))
		}
	}
}

func TestPlainRenderer_ConcurrentSafe(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewPlainRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	const N = 50
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each goroutine writes a complete line so output is
			// deterministic and free of mid-line interleaving.
			_ = r.WriteToken("line\n")
		}(i)
	}
	wg.Wait()
	if err := r.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(buf.String(), "line\n"); got != N {
		t.Fatalf("expected %d 'line\\n' occurrences, got %d in %q", N, got, buf.String())
	}
	// Hard invariant still holds under concurrency.
	if strings.ContainsRune(buf.String(), '\r') || strings.ContainsRune(buf.String(), 0x1b) {
		t.Fatalf("concurrent output contained \\r or ESC: %q", buf.String())
	}
}
