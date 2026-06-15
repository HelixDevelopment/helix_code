// Standing regression guard (§11.4.135) for HXC-RENDER-TRUNC-CURSOR:
// the fancy ANSI renderer left frameCursorPos stale after a frame
// truncation (shrink), so the NEXT RenderFrame that changed a line
// computed its cursor-up distance against the OLD (larger) frame height
// and overshot the frame's top edge — rewriting the changed line ABOVE
// the block (corrupting unrelated terminal output) and leaving the
// cursor misplaced afterwards.
//
// §11.4.115 polarity switch via the RED_MODE env var:
//
//	RED_MODE=1 — reproduce the defect on a FAITHFUL pre-fix stand-in
//	             (staleCursorRenderFrame, a byte-for-byte copy of the old
//	             RenderFrame body WITHOUT the truncation cursorPos
//	             decrement). Asserts the defect is PRESENT (the overshoot
//	             cursor-up `\x1b[4A` is emitted and the stored cursorPos
//	             stays stale at 4). This proves the guard reproduces a real
//	             bug, not a synthetic one.
//
//	RED_MODE=0 (default) — drive the REAL, fixed *ansiRenderer and assert
//	             the defect is ABSENT (cursor-up is `\x1b[2A`, matching the
//	             live 2-line block, and stored cursorPos == live line count).
//
// Reproduction (pre-fix, captured 2026-06-15):
//
//	frame3 output: "\x1b[4A\r\x1b[KX\x1b[4B"   (overshoot: up 4 on a 2-line block)
//	cursorPos after = 4, vp.LineCount = 2       (stale)
//
// Post-fix:
//
//	frame3 output: "\x1b[2A\r\x1b[KX\x1b[2B"   (correct: up 2)
//	cursorPos after = 2, vp.LineCount = 2       (in sync)
package render

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// redMode reports whether the RED_MODE polarity switch is engaged.
func redModeTrunc() bool { return os.Getenv("RED_MODE") == "1" }

// staleCursorRenderFrame is a FAITHFUL stand-in for the pre-fix
// RenderFrame body: it reproduces the original Changed/Appended emission
// and the original behaviour of NOT decrementing cursorPos on truncation.
// It mutates the renderer's viewport + frameCursorPos exactly as the old
// code did so the RED_MODE=1 path exercises the genuine defect shape.
func staleCursorRenderFrame(r *ansiRenderer, frame Frame) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return "", ErrRendererClosed
	}
	if frame.BlockID == "" {
		return "", ErrEmptyBlockID
	}
	if err := r.ensureCursorHidden(); err != nil {
		return "", err
	}
	vp, ok := r.viewports[frame.BlockID]
	if !ok {
		vp = NewViewport(frame.BlockID)
		r.viewports[frame.BlockID] = vp
	}
	cursorPos := r.frameCursorPos[frame.BlockID]
	diff := vp.Apply(frame)
	if diff.IsNoChange() {
		return "", nil
	}
	var out strings.Builder
	for _, i := range diff.Changed {
		up := cursorPos - i
		out.WriteString(fmt.Sprintf(ansiCursorUpFmt, up) + ansiCRClearLine + frame.Lines[i] +
			fmt.Sprintf(ansiCursorDownFmt, up))
	}
	if len(diff.Appended) > 0 {
		for _, i := range diff.Appended {
			out.WriteString(frame.Lines[i])
			out.WriteString("\n")
		}
		cursorPos += len(diff.Appended)
	}
	// PRE-FIX: no `cursorPos -= diff.Truncated` here — this is the bug.
	r.frameCursorPos[frame.BlockID] = cursorPos
	return out.String(), nil
}

// TestANSIRenderer_TruncateThenChange_CursorNotStale is the standing guard.
func TestANSIRenderer_TruncateThenChange_CursorNotStale(t *testing.T) {
	const blockID = "guard"

	if redModeTrunc() {
		// RED_MODE=1: reproduce the defect on the faithful pre-fix stand-in.
		r := NewANSIRenderer(&bytes.Buffer{})
		// frame1: 4 lines.
		if _, err := staleCursorRenderFrame(r, Frame{BlockID: blockID, Lines: []string{"a", "b", "c", "d"}}); err != nil {
			t.Fatalf("RED frame1: %v", err)
		}
		// frame2: shrink to 2 lines (Truncated=2). Pre-fix leaves cursorPos=4.
		if _, err := staleCursorRenderFrame(r, Frame{BlockID: blockID, Lines: []string{"a", "b"}}); err != nil {
			t.Fatalf("RED frame2: %v", err)
		}
		if got := r.frameCursorPos[blockID]; got != 4 {
			t.Fatalf("RED expected stale cursorPos=4 (defect present), got %d", got)
		}
		// frame3: change line 0. Defect: up = 4 - 0 = 4 (overshoot past frame top).
		out, err := staleCursorRenderFrame(r, Frame{BlockID: blockID, Lines: []string{"X", "b"}})
		if err != nil {
			t.Fatalf("RED frame3: %v", err)
		}
		wantOvershoot := fmt.Sprintf(ansiCursorUpFmt, 4) // "\x1b[4A"
		if !strings.HasPrefix(out, wantOvershoot) {
			t.Fatalf("RED expected overshoot cursor-up %q (defect present), got %q", wantOvershoot, out)
		}
		// The down move must match the bad up move (also overshoot).
		if !strings.HasSuffix(out, fmt.Sprintf(ansiCursorDownFmt, 4)) {
			t.Fatalf("RED expected matching overshoot cursor-down up=4, got %q", out)
		}
		t.Logf("RED_MODE=1 reproduced HXC-RENDER-TRUNC-CURSOR: stale cursorPos=4, overshoot output=%q", out)
		return
	}

	// RED_MODE=0: drive the REAL fixed renderer and assert the defect is ABSENT.
	var buf bytes.Buffer
	r := NewANSIRenderer(&buf)
	if err := r.RenderFrame(Frame{BlockID: blockID, Lines: []string{"a", "b", "c", "d"}}); err != nil {
		t.Fatalf("frame1: %v", err)
	}
	if err := r.RenderFrame(Frame{BlockID: blockID, Lines: []string{"a", "b"}}); err != nil {
		t.Fatalf("frame2 (truncate): %v", err)
	}

	// After truncation the stored cursorPos MUST equal the live line count.
	if got, want := r.frameCursorPos[blockID], r.viewports[blockID].LineCount(); got != want {
		t.Fatalf("after truncation stored cursorPos=%d, want == live line count %d", got, want)
	}
	if got := r.frameCursorPos[blockID]; got != 2 {
		t.Fatalf("after truncation cursorPos=%d, want 2", got)
	}

	buf.Reset()
	if err := r.RenderFrame(Frame{BlockID: blockID, Lines: []string{"X", "b"}}); err != nil {
		t.Fatalf("frame3 (change after truncate): %v", err)
	}
	out := buf.String()

	// The changed line must move up by exactly the live block height (2),
	// never the stale old height (4).
	wantUp := fmt.Sprintf(ansiCursorUpFmt, 2) // "\x1b[2A"
	if !strings.HasPrefix(out, wantUp) {
		t.Fatalf("changed-after-truncate cursor-up = wrong; got %q, want prefix %q (no overshoot)", out, wantUp)
	}
	if !strings.HasSuffix(out, fmt.Sprintf(ansiCursorDownFmt, 2)) {
		t.Fatalf("changed-after-truncate cursor-down = wrong; got %q, want up=2 restore", out)
	}
	// The overshoot sequence MUST NOT appear.
	if strings.Contains(out, fmt.Sprintf(ansiCursorUpFmt, 4)) {
		t.Fatalf("overshoot cursor-up \\x1b[4A leaked into output: %q", out)
	}
	if !strings.Contains(out, ansiCRClearLine+"X") {
		t.Fatalf("changed line X not rewritten in place: %q", out)
	}
}
