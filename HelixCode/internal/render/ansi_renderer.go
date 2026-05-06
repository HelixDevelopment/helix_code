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

	// frame-block state: per-blockID previous frame + cursor position
	// (lines below the frame's top edge after the most recent RenderFrame).
	framePrev      map[string]Frame
	frameCursorPos map[string]int
}

// NewANSIRenderer constructs a fancy-mode Renderer writing to w.
func NewANSIRenderer(w io.Writer) *ansiRenderer {
	return &ansiRenderer{
		w:              w,
		framePrev:      make(map[string]Frame),
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

	prev, hasPrev := r.framePrev[frame.BlockID]
	if !hasPrev {
		// First render: emit every line followed by \n.
		var b strings.Builder
		for _, line := range frame.Lines {
			b.WriteString(line)
			b.WriteString("\n")
		}
		if b.Len() > 0 {
			if _, err := io.WriteString(r.w, b.String()); err != nil {
				return err
			}
		}
		// Cursor now sits len(frame.Lines) lines below the frame's top.
		r.framePrev[frame.BlockID] = cloneFrame(frame)
		r.frameCursorPos[frame.BlockID] = len(frame.Lines)
		return nil
	}

	// Subsequent render: dirty-line diff.
	cursorPos := r.frameCursorPos[frame.BlockID]

	// Walk overlapping line slots.
	overlap := minInt(len(prev.Lines), len(frame.Lines))
	for i := 0; i < overlap; i++ {
		if prev.Lines[i] == frame.Lines[i] {
			continue
		}
		// Cursor is `cursorPos` lines below the top edge of the
		// frame; line i is `cursorPos - i` lines above the cursor.
		// (cursorPos > i because i < overlap <= len(prev.Lines) ==
		// initial cursorPos, and we don't decrement cursorPos in this
		// path.)
		up := cursorPos - i
		// up >= 1 always, since i in [0, overlap) and cursorPos >= overlap.
		seq := fmt.Sprintf(ansiCursorUpFmt, up) + ansiCRClearLine + frame.Lines[i] +
			fmt.Sprintf(ansiCursorDownFmt, up)
		if _, err := io.WriteString(r.w, seq); err != nil {
			return err
		}
	}

	// New frame longer than previous: append the extra lines at the bottom.
	if len(frame.Lines) > len(prev.Lines) {
		var b strings.Builder
		for i := len(prev.Lines); i < len(frame.Lines); i++ {
			b.WriteString(frame.Lines[i])
			b.WriteString("\n")
		}
		if _, err := io.WriteString(r.w, b.String()); err != nil {
			return err
		}
		cursorPos += len(frame.Lines) - len(prev.Lines)
	}
	// Note: when new frame is shorter than prev, trailing old lines are NOT
	// cleared (acceptable v1 per task scope).

	r.framePrev[frame.BlockID] = cloneFrame(frame)
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

// cloneFrame returns a deep copy of f so the renderer's recorded "previous
// frame" cannot be mutated by callers retaining the original slice.
func cloneFrame(f Frame) Frame {
	cp := Frame{BlockID: f.BlockID}
	if f.Lines != nil {
		cp.Lines = make([]string, len(f.Lines))
		copy(cp.Lines, f.Lines)
	}
	return cp
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
