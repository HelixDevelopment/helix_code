package render

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// helper: count occurrences of a substring in s
func countSub(s, sub string) int {
	if sub == "" {
		return 0
	}
	return strings.Count(s, sub)
}

// helper: count cursor-up sequences "\x1b[<n>A" (digits + 'A')
var cursorUpRe = regexp.MustCompile(`\x1b\[\d+A`)

func countCursorUp(s string) int {
	return len(cursorUpRe.FindAllString(s, -1))
}

func TestANSIRenderer_Mode(t *testing.T) {
	r := NewANSIRenderer(&bytes.Buffer{})
	if got := r.Mode(); got != ModeFancy {
		t.Fatalf("Mode() = %q, want %q", got, ModeFancy)
	}
}

func TestANSIRenderer_Compiles_AsRenderer(t *testing.T) {
	var _ Renderer = NewANSIRenderer(&bytes.Buffer{})
}

func TestANSIRenderer_Begin_FirstCallEmitsHideCursor(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if !strings.Contains(buf.String(), ansiHideCursor) {
		t.Fatalf("expected hide-cursor sequence in buffer, got %q", buf.String())
	}
}

func TestANSIRenderer_Begin_SubsequentDoesNotReHide(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Begin("b"); err != nil {
		t.Fatal(err)
	}
	if got := countSub(buf.String(), ansiHideCursor); got != 1 {
		t.Fatalf("expected hide-cursor once, got %d in %q", got, buf.String())
	}
}

func TestANSIRenderer_Begin_EmptyBlockID(t *testing.T) {
	r := NewANSIRenderer(&bytes.Buffer{})
	if err := r.Begin(""); !errors.Is(err, ErrEmptyBlockID) {
		t.Fatalf("Begin(\"\") err=%v, want ErrEmptyBlockID", err)
	}
}

func TestANSIRenderer_WriteToken_NoNewline_EmitsCRClearLine(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), ansiCRClearLine+"hello") {
		t.Fatalf("expected CR+clear+hello in buffer, got %q", buf.String())
	}
}

func TestANSIRenderer_WriteToken_AppendsAccumulates(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("foo"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("bar"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), ansiCRClearLine+"foobar") {
		t.Fatalf("expected accumulated %q in buffer, got %q", ansiCRClearLine+"foobar", buf.String())
	}
}

func TestANSIRenderer_WriteToken_WithNewline_FinalizesLine(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("first\n"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "first\n") {
		t.Fatalf("expected first\\n in buffer, got %q", out)
	}
	// snapshot length, then write more — the previous "first" line must remain finalised.
	prefixLen := buf.Len()
	if err := r.WriteToken("second"); err != nil {
		t.Fatal(err)
	}
	delta := buf.String()[prefixLen:]
	if !strings.Contains(delta, ansiCRClearLine+"second") {
		t.Fatalf("expected fresh CR+clear+second after newline, delta=%q", delta)
	}
	if strings.Contains(delta, "first") {
		t.Fatalf("delta must not re-emit \"first\", delta=%q", delta)
	}
}

func TestANSIRenderer_WriteToken_MultiLineToken(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("a\nb\nc"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "a\n") {
		t.Fatalf("missing finalised \"a\\n\", got %q", out)
	}
	if !strings.Contains(out, "b\n") {
		t.Fatalf("missing finalised \"b\\n\", got %q", out)
	}
	if !strings.Contains(out, ansiCRClearLine+"c") {
		t.Fatalf("missing in-place \"c\" update, got %q", out)
	}
}

func TestANSIRenderer_Commit_AddsTrailingNewline(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	_ = r.Begin("a")
	_ = r.WriteToken("partial")
	if err := r.Commit(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("expected trailing newline after commit, got %q", buf.String())
	}
}

func TestANSIRenderer_Commit_NoExtraNewlineIfAlreadyTerminated(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	_ = r.Begin("a")
	_ = r.WriteToken("done\n")
	prevLen := buf.Len()
	if err := r.Commit(); err != nil {
		t.Fatal(err)
	}
	delta := buf.String()[prevLen:]
	if strings.Contains(delta, "\n") {
		t.Fatalf("commit emitted extra newline; delta=%q", delta)
	}
}

func TestANSIRenderer_Commit_NoOpenBlock(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Commit(); err != nil {
		t.Fatalf("Commit no-op: %v", err)
	}
}

func TestANSIRenderer_RenderFrame_FirstRenderEmitsAllLines(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	f := Frame{BlockID: "blk", Lines: []string{"alpha", "beta", "gamma"}}
	if err := r.RenderFrame(f); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"alpha\n", "beta\n", "gamma\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("first render missing %q, got %q", want, out)
		}
	}
}

func TestANSIRenderer_RenderFrame_SecondRender_NoChange_NoOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	f := Frame{BlockID: "blk", Lines: []string{"a", "b"}}
	if err := r.RenderFrame(f); err != nil {
		t.Fatal(err)
	}
	prevLen := buf.Len()
	if err := r.RenderFrame(f); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != prevLen {
		t.Fatalf("identical second render emitted %d bytes; expected 0; delta=%q",
			buf.Len()-prevLen, buf.String()[prevLen:])
	}
}

func TestANSIRenderer_RenderFrame_DirtyDiff_OneLineChange(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	// Lines are realistic-length so the dirty-diff delta (which contains
	// only the changed slot + a few cursor-control bytes) is strictly
	// smaller than the first render (which contains every line).
	f1 := Frame{BlockID: "blk", Lines: []string{
		"alpha-line-one--------",
		"beta-line-two---------",
		"gamma-line-three------",
	}}
	if err := r.RenderFrame(f1); err != nil {
		t.Fatal(err)
	}
	firstLen := buf.Len()

	f2 := Frame{BlockID: "blk", Lines: []string{
		"alpha-line-one--------",
		"X",
		"gamma-line-three------",
	}}
	if err := r.RenderFrame(f2); err != nil {
		t.Fatal(err)
	}
	delta := buf.String()[firstLen:]

	if len(delta) >= firstLen {
		t.Fatalf("dirty-diff delta (%d) must be strictly smaller than first render (%d); delta=%q",
			len(delta), firstLen, delta)
	}
	if up := countCursorUp(delta); up != 1 {
		t.Fatalf("expected exactly one cursor-up sequence in delta, got %d; delta=%q", up, delta)
	}
	if !strings.Contains(delta, "\x1b[K") {
		t.Fatalf("expected clear-line sequence \\x1b[K in delta, got %q", delta)
	}
	if !strings.Contains(delta, "X") {
		t.Fatalf("delta missing changed content \"X\"; delta=%q", delta)
	}
	// Must not re-emit unchanged lines verbatim. They contain unique
	// substrings ("alpha", "gamma") that must NOT appear in the delta.
	if strings.Contains(delta, "alpha") {
		t.Fatalf("delta unexpectedly re-emitted unchanged \"alpha\" line; delta=%q", delta)
	}
	if strings.Contains(delta, "gamma") {
		t.Fatalf("delta unexpectedly re-emitted unchanged \"gamma\" line; delta=%q", delta)
	}
}

func TestANSIRenderer_RenderFrame_AppendsLinesWhenLonger(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	f1 := Frame{BlockID: "blk", Lines: []string{"a"}}
	if err := r.RenderFrame(f1); err != nil {
		t.Fatal(err)
	}
	prevLen := buf.Len()

	f2 := Frame{BlockID: "blk", Lines: []string{"a", "b"}}
	if err := r.RenderFrame(f2); err != nil {
		t.Fatal(err)
	}
	delta := buf.String()[prevLen:]
	if !strings.Contains(delta, "b") {
		t.Fatalf("expected new line \"b\" in delta, got %q", delta)
	}
	// Must not redraw "a" as a finalised line.
	if strings.Contains(delta, "a\n") {
		t.Fatalf("delta unexpectedly re-emitted \"a\\n\"; delta=%q", delta)
	}
}

func TestANSIRenderer_RenderFrame_EmptyBlockID(t *testing.T) {
	r := NewANSIRenderer(&bytes.Buffer{})
	if err := r.RenderFrame(Frame{BlockID: "", Lines: []string{"x"}}); !errors.Is(err, ErrEmptyBlockID) {
		t.Fatalf("RenderFrame empty BlockID err=%v, want ErrEmptyBlockID", err)
	}
}

func TestANSIRenderer_BlockIDRequired_ForRenderFrame(t *testing.T) {
	r := NewANSIRenderer(&bytes.Buffer{})
	err := r.RenderFrame(Frame{})
	if !errors.Is(err, ErrEmptyBlockID) {
		t.Fatalf("expected ErrEmptyBlockID, got %v", err)
	}
}

func TestANSIRenderer_Close_EmitsShowCursor(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	_ = r.Begin("a")
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), ansiShowCursor) {
		t.Fatalf("expected show-cursor on Close, got %q", buf.String())
	}
}

func TestANSIRenderer_Close_Idempotent(t *testing.T) {
	r := NewANSIRenderer(&bytes.Buffer{})
	_ = r.Begin("a")
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v (want nil per Renderer contract)", err)
	}
}

func TestANSIRenderer_Close_NoShowCursorIfNeverHidden(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), ansiShowCursor) {
		t.Fatalf("show-cursor emitted without prior hide; buf=%q", buf.String())
	}
}

func TestANSIRenderer_PostCloseWriteToken_ReturnsError(t *testing.T) {
	r := NewANSIRenderer(&bytes.Buffer{})
	_ = r.Close()
	if err := r.WriteToken("x"); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("WriteToken post-Close err=%v, want ErrRendererClosed", err)
	}
	if err := r.Begin("a"); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("Begin post-Close err=%v, want ErrRendererClosed", err)
	}
	if err := r.Commit(); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("Commit post-Close err=%v, want ErrRendererClosed", err)
	}
	if err := r.RenderFrame(Frame{BlockID: "x", Lines: []string{"y"}}); !errors.Is(err, ErrRendererClosed) {
		t.Fatalf("RenderFrame post-Close err=%v, want ErrRendererClosed", err)
	}
}

func TestANSIRenderer_Begin_DifferentID_CommitsPrevious(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteToken("partial"); err != nil {
		t.Fatal(err)
	}
	prevLen := buf.Len()
	if err := r.Begin("b"); err != nil {
		t.Fatal(err)
	}
	delta := buf.String()[prevLen:]
	// The implicit commit of block "a" must have emitted at least one \n
	// before block "b" begins writing. Block "b" hasn't written anything
	// yet, so any \n in delta is from the implicit commit.
	if !strings.Contains(delta, "\n") {
		t.Fatalf("expected implicit commit newline in delta, got %q", delta)
	}
}

func TestANSIRenderer_WriteToken_AutoBegins(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.WriteToken("auto"); err != nil {
		t.Fatalf("auto-Begin should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "auto") {
		t.Fatalf("expected \"auto\" in buffer, got %q", buf.String())
	}
}

func TestANSIRenderer_ConcurrentSafe(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewANSIRenderer(buf)
	if err := r.Begin("a"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = r.WriteToken(fmt.Sprintf("g%d-j%d ", i, j))
			}
		}(i)
	}
	wg.Wait()
	if err := r.Commit(); err != nil {
		t.Fatalf("Commit after concurrent writes: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close after concurrent writes: %v", err)
	}
	// no panic, no race => success
	if buf.Len() == 0 {
		t.Fatalf("expected some output after concurrent writes")
	}
}
