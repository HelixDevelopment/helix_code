// Tests for the tool-result frame rendering helpers (P1-F18-T08).
//
// Helper surface under test:
//   - RenderTextBlock(r, blockID, text): split text on \n into lines, then
//     RenderFrame{BlockID, Lines}. Empty blockID auto-generates a unique ID
//     (so each call is independent). Trailing empty line dropped if present.
//   - RenderLines(r, blockID, lines): slice variant; passes through directly.
//
// Invariants asserted (cf. spec §4.2):
//   - Plain mode: every line emitted in order, separated by \n.
//   - Fancy mode: first frame emits all lines + cursor-hide; second frame for
//     the same blockID with one changed middle line emits a strictly smaller
//     delta than the first frame (load-bearing dirty-diff).
//   - Empty blockID: two calls emit independently (unique synthetic IDs); the
//     plain mode buffer contains both blocks' content concatenated.
//   - Empty text: no Write to underlying buffer (fast path; documented).
//   - Trailing newline: "a\nb\n" -> lines = [a, b] (not [a, b, ""]).
//   - RenderLines: identical buffer output to RenderTextBlock with joined input.
package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTextBlock_PlainMode_LinesPrinted(t *testing.T) {
	var buf bytes.Buffer
	r := NewPlainRenderer(&buf)
	defer r.Close()

	if err := RenderTextBlock(r, "test", "line1\nline2\nline3"); err != nil {
		t.Fatalf("RenderTextBlock: %v", err)
	}

	want := "line1\nline2\nline3\n"
	if got := buf.String(); got != want {
		t.Errorf("plain mode output mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestRenderTextBlock_FancyMode_FirstCall_AllLinesEmitted(t *testing.T) {
	var buf bytes.Buffer
	r := NewANSIRenderer(&buf)
	defer r.Close()

	if err := RenderTextBlock(r, "first-frame", "alpha\nbeta\ngamma"); err != nil {
		t.Fatalf("RenderTextBlock: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(out, want) {
			t.Errorf("fancy mode first frame missing %q; got %q", want, out)
		}
	}
	// Cursor-hide is the canonical first-Begin/RenderFrame side effect.
	if !strings.Contains(out, "\x1b[?25l") {
		t.Errorf("fancy mode first frame missing hide-cursor (\\x1b[?25l); got %q", out)
	}
}

func TestRenderTextBlock_FancyMode_SecondCall_OneLineChange_DirtyDiff(t *testing.T) {
	// Load-bearing dirty-diff invariant: rendering an unchanged frame followed
	// by a frame with one line changed must emit STRICTLY LESS output than the
	// first full frame (where every line is emitted as Appended).
	var buf bytes.Buffer
	r := NewANSIRenderer(&buf)
	defer r.Close()

	// First call: full frame emits all 3 lines + hide-cursor + per-line \n.
	if err := RenderTextBlock(r, "diff-block", "alpha\nbeta\ngamma"); err != nil {
		t.Fatalf("RenderTextBlock #1: %v", err)
	}
	firstDelta := buf.Len()
	if firstDelta == 0 {
		t.Fatalf("first call must emit output; buf is empty")
	}

	// Second call: same BlockID, only line[1] changes. Diff path emits a
	// single cursor-up + CR+clear + new line + cursor-down sequence — no
	// Appended slots, no re-emit of unchanged lines.
	bufBefore := buf.Len()
	if err := RenderTextBlock(r, "diff-block", "alpha\nBETA\ngamma"); err != nil {
		t.Fatalf("RenderTextBlock #2: %v", err)
	}
	secondDelta := buf.Len() - bufBefore

	if secondDelta == 0 {
		t.Errorf("second call with one-line change must emit something; got 0 bytes")
	}
	if secondDelta >= firstDelta {
		t.Errorf("dirty-diff invariant violated: second-call delta %d should be < first-call delta %d", secondDelta, firstDelta)
	}
	// The new line content must reach the writer.
	if !strings.Contains(buf.String(), "BETA") {
		t.Errorf("changed line content %q missing from buffer; got %q", "BETA", buf.String())
	}
}

func TestRenderTextBlock_EmptyBlockID_GeneratesUniqueID(t *testing.T) {
	// Two calls with blockID="" must each emit independently (fresh synthetic
	// IDs each time). In plain mode the underlying buffer simply accumulates
	// both blocks; the property under test is "each call produced output".
	var buf bytes.Buffer
	r := NewPlainRenderer(&buf)
	defer r.Close()

	if err := RenderTextBlock(r, "", "first"); err != nil {
		t.Fatalf("RenderTextBlock #1: %v", err)
	}
	afterFirst := buf.Len()
	if afterFirst == 0 {
		t.Fatalf("first call with empty blockID emitted nothing")
	}

	if err := RenderTextBlock(r, "", "second"); err != nil {
		t.Fatalf("RenderTextBlock #2: %v", err)
	}
	if buf.Len() <= afterFirst {
		t.Errorf("second call with empty blockID did not emit additional output; before=%d after=%d", afterFirst, buf.Len())
	}
	out := buf.String()
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Errorf("expected both block contents in buffer; got %q", out)
	}
}

func TestRenderTextBlock_EmptyText_NoOp(t *testing.T) {
	// Documented contract: empty text is a no-op; the underlying writer
	// receives ZERO bytes. Justification: a frame with no lines carries no
	// semantic content (cf. Frame.IsZero) and should not produce ANSI clutter
	// or empty newlines in the transcript.
	var buf bytes.Buffer
	r := NewPlainRenderer(&buf)
	defer r.Close()

	if err := RenderTextBlock(r, "empty", ""); err != nil {
		t.Fatalf("RenderTextBlock: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty text must be a no-op; got %d bytes: %q", buf.Len(), buf.String())
	}
}

func TestRenderTextBlock_TrailingNewlineDropped(t *testing.T) {
	// "a\nb\n" -> lines = [a, b], NOT [a, b, ""]. The trailing newline is a
	// terminator on line "b", not the start of a third (empty) line. This
	// matches the natural transcript shape callers expect when they pass a
	// reader-style multi-line string.
	var buf bytes.Buffer
	r := NewPlainRenderer(&buf)
	defer r.Close()

	if err := RenderTextBlock(r, "trail", "a\nb\n"); err != nil {
		t.Fatalf("RenderTextBlock: %v", err)
	}
	want := "a\nb\n"
	if got := buf.String(); got != want {
		t.Errorf("trailing-newline handling mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestRenderLines_PassThrough(t *testing.T) {
	// RenderLines is the slice variant: skip the split step. Buffer output
	// for RenderLines(r,"id",[a,b]) must match RenderTextBlock with "a\nb".
	var bufA, bufB bytes.Buffer
	rA := NewPlainRenderer(&bufA)
	rB := NewPlainRenderer(&bufB)
	defer rA.Close()
	defer rB.Close()

	if err := RenderLines(rA, "id", []string{"a", "b"}); err != nil {
		t.Fatalf("RenderLines: %v", err)
	}
	if err := RenderTextBlock(rB, "id", "a\nb"); err != nil {
		t.Fatalf("RenderTextBlock: %v", err)
	}

	if bufA.String() != bufB.String() {
		t.Errorf("RenderLines and RenderTextBlock output differ:\nlines=%q\ntext =%q", bufA.String(), bufB.String())
	}
}

func TestRenderLines_NilLines_NoOp(t *testing.T) {
	// Empty/nil lines slice: documented no-op (mirror of RenderTextBlock
	// empty-text contract). Justification: a Frame with zero lines is a
	// "zero frame" per types.go Frame.IsZero and produces no output.
	var buf bytes.Buffer
	r := NewPlainRenderer(&buf)
	defer r.Close()

	if err := RenderLines(r, "id", nil); err != nil {
		t.Fatalf("RenderLines(nil): %v", err)
	}
	if err := RenderLines(r, "id", []string{}); err != nil {
		t.Fatalf("RenderLines(empty): %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("nil/empty lines must be a no-op; got %d bytes: %q", buf.Len(), buf.String())
	}
}

func TestRenderTextBlock_PropagatesRendererError(t *testing.T) {
	// If the underlying renderer is closed, RenderTextBlock must surface the
	// ErrRendererClosed sentinel rather than silently dropping the frame.
	var buf bytes.Buffer
	r := NewPlainRenderer(&buf)
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := RenderTextBlock(r, "x", "data")
	if err == nil {
		t.Fatalf("expected error from closed renderer; got nil")
	}
}
