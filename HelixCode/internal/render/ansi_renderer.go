// Fancy-mode Renderer (P1-F18-T03).
//
// ansiRenderer implements the Renderer contract from types.go using ANSI
// escape sequences for in-place line updates and dirty-region multi-line
// frame diffs. Byte invariants (cf. spec §11.6):
//
//   - First Begin emits \x1b[?25l (hide cursor).
//   - Each WriteToken with no embedded newline emits \r\x1b[K<accumulated line>
//     so the visible terminal column collapses to "the latest token state".
//   - Each newline inside WriteToken finalises the current line (\n) and
//     resets the in-progress accumulator.
//   - RenderFrame's first call for a BlockID prints every line followed by \n.
//     Subsequent calls compute a per-line diff against the previous frame and
//     emit only the changed lines: cursor-up to the changed slot via
//     \x1b[<n>A, \r\x1b[K<new line>, then cursor-down via \x1b[<n>B to restore
//     the post-frame cursor position. Equal frames produce zero output.
//   - Close emits \x1b[?25h (show cursor) iff hide-cursor was emitted earlier.
//
// Thread-safety: a single sync.Mutex serialises all state-mutating methods so
// concurrent callers cannot interleave bytes mid-sequence.
package render

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// ANSI control sequence constants. Kept as package-level vars so tests can
// reference the exact byte strings without re-typing escape codes.
const (
	ansiCRClearLine   = "\r\x1b[K"  // CR + clear current line to end
	ansiHideCursor    = "\x1b[?25l" // hide cursor
	ansiShowCursor    = "\x1b[?25h" // show cursor
	ansiCursorUpFmt   = "\x1b[%dA"  // move cursor up N lines
	ansiCursorDownFmt = "\x1b[%dB"  // move cursor down N lines
)

// ansiRenderer is the fancy-mode Renderer impl.
type ansiRenderer struct {
	mu           sync.Mutex
	w            io.Writer
	closed       bool
	cursorHidden bool

	// streaming-block state.
	streamingID   string // "" if no active streaming block
	streamingLine string // accumulated text for the current line (no trailing \n)

	// frame-block state: per-blockID Viewport (T05) + cursor position
	// (lines below the frame's top edge after the most recent RenderFrame).
	viewports      map[string]*Viewport
	frameCursorPos map[string]int
}

// NewANSIRenderer constructs a fancy-mode Renderer writing to w.
func NewANSIRenderer(w io.Writer) *ansiRenderer {
	return &ansiRenderer{
		w:              w,
		viewports:      make(map[string]*Viewport),
		frameCursorPos: make(map[string]int),
	}
}

// Mode returns ModeFancy.
func (r *ansiRenderer) Mode() RenderMode { return ModeFancy }

// ensureCursorHidden writes the hide-cursor sequence on first use and records
// that the cursor must be re-shown at Close. Caller must hold r.mu.
func (r *ansiRenderer) ensureCursorHidden() error {
	if r.cursorHidden {
		return nil
	}
	if _, err := io.WriteString(r.w, ansiHideCursor); err != nil {
		return err
	}
	r.cursorHidden = true
	return nil
}

// commitStreamingLocked finalises the current streaming line by emitting a
// trailing \n if the in-progress line is non-empty. Caller must hold r.mu.
func (r *ansiRenderer) commitStreamingLocked() error {
	if r.streamingID == "" {
		return nil
	}
	if r.streamingLine != "" {
		if _, err := io.WriteString(r.w, "\n"); err != nil {
			return err
		}
	}
	r.streamingID = ""
	r.streamingLine = ""
	return nil
}

// Begin opens a streaming block.
func (r *ansiRenderer) Begin(blockID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	if blockID == "" {
		return ErrEmptyBlockID
	}
	if r.streamingID == blockID {
		// idempotent
		return nil
	}
	if r.streamingID != "" {
		if err := r.commitStreamingLocked(); err != nil {
			return err
		}
	}
	if err := r.ensureCursorHidden(); err != nil {
		return err
	}
	r.streamingID = blockID
	r.streamingLine = ""
	return nil
}

// WriteToken appends text to the current streaming block.
func (r *ansiRenderer) WriteToken(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	if r.streamingID == "" {
		// auto-Begin with synthetic ID
		if err := r.ensureCursorHidden(); err != nil {
			return err
		}
		r.streamingID = "_auto"
		r.streamingLine = ""
	}
	if text == "" {
		return nil
	}

	// Split on \n. Every newline finalises r.streamingLine + accumulated
	// segment, and emits "<segment>\n". The trailing fragment (if any)
	// becomes the new in-progress line and is written via CR+clear+line.
	parts := strings.Split(text, "\n")
	// parts has len = number of \n + 1; e.g. "a\nb\nc" -> [a b c]
	for i := 0; i < len(parts)-1; i++ {
		// Finalise: r.streamingLine + parts[i] + "\n"
		// We want the visible terminal line (already showing
		// streamingLine via CR+clear) to be replaced by the full
		// finalised line, then advance via "\n".
		full := r.streamingLine + parts[i]
		// Emit CR+clear+full+"\n" as a single Write so the line on
		// screen ends up being "full" and the cursor advances to the
		// next row.
		if _, err := io.WriteString(r.w, ansiCRClearLine+full+"\n"); err != nil {
			return err
		}
		r.streamingLine = ""
	}
	// Trailing fragment: append to streamingLine and re-paint via CR+clear.
	tail := parts[len(parts)-1]
	if tail != "" {
		r.streamingLine += tail
		if _, err := io.WriteString(r.w, ansiCRClearLine+r.streamingLine); err != nil {
			return err
		}
	}
	return nil
}

// Commit closes the current streaming block.
func (r *ansiRenderer) Commit() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	return r.commitStreamingLocked()
}

// RenderFrame draws or updates a multi-line frame.
//
// Diff logic is delegated to the Viewport type (T05): we obtain the
// per-blockID Viewport, call Apply (which returns a LineDiff), and emit
// ANSI for the Changed and Appended slots. First-frame rendering is the
// degenerate case where the prior viewport was empty -- LineDiff carries
// every index in Appended, so the same code path emits every line.
func (r *ansiRenderer) RenderFrame(frame Frame) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	if frame.BlockID == "" {
		return ErrEmptyBlockID
	}
	if err := r.ensureCursorHidden(); err != nil {
		return err
	}

	vp, ok := r.viewports[frame.BlockID]
	if !ok {
		vp = NewViewport(frame.BlockID)
		r.viewports[frame.BlockID] = vp
	}
	prevLineCount := vp.LineCount()
	cursorPos := r.frameCursorPos[frame.BlockID]

	diff := vp.Apply(frame)
	if diff.IsNoChange() {
		return nil
	}

	// Changed slots: in-place rewrite via cursor-up + CR+clear + new line +
	// cursor-down. Indices refer to positions in the new frame, identical
	// to the old frame's positions (since they fall in the overlap region).
	// Cursor position is `cursorPos` lines below the top of the frame;
	// line i is `cursorPos - i` lines above the cursor.
	for _, i := range diff.Changed {
		up := cursorPos - i
		// up >= 1 always: i < min(prev,new) <= prevLineCount == cursorPos,
		// so cursorPos - i >= 1.
		seq := fmt.Sprintf(ansiCursorUpFmt, up) + ansiCRClearLine + frame.Lines[i] +
			fmt.Sprintf(ansiCursorDownFmt, up)
		if _, err := io.WriteString(r.w, seq); err != nil {
			return err
		}
	}

	// Appended slots: emit the new trailing lines, each followed by \n.
	if len(diff.Appended) > 0 {
		var b strings.Builder
		for _, i := range diff.Appended {
			b.WriteString(frame.Lines[i])
			b.WriteString("\n")
		}
		if _, err := io.WriteString(r.w, b.String()); err != nil {
			return err
		}
		cursorPos += len(diff.Appended)
	}
	// Truncated: when the new frame is shorter than the previous, trailing
	// old lines are NOT cleared from the terminal (acceptable v1 per task
	// scope; documented). We still need to update cursorPos to reflect the
	// new logical line count, but since cursor is below the lines, no
	// movement is needed for this v1.
	_ = prevLineCount // retained for clarity; debug builds may inspect.

	r.frameCursorPos[frame.BlockID] = cursorPos
	return nil
}

// Close finalises any in-progress streaming block and re-shows the cursor.
func (r *ansiRenderer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	// commit any in-progress streaming line so the terminal isn't left
	// halfway through a CR-rewritten line.
	if r.streamingID != "" {
		if err := r.commitStreamingLocked(); err != nil {
			return err
		}
	}
	if r.cursorHidden {
		if _, err := io.WriteString(r.w, ansiShowCursor); err != nil {
			return err
		}
		r.cursorHidden = false
	}
	r.closed = true
	return nil
}

// Note: defensive copying of Frame.Lines now lives in Viewport.Apply (T05).
// minInt was previously used by the inline diff loop; the diff is now
// computed by Diff in viewport.go.
