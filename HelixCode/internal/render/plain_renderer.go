// Plain-mode Renderer (P1-F18-T04).
//
// plainRenderer implements the Renderer contract from types.go for non-TTY
// destinations: log files, pipes, CI consoles, dumb terminals. Byte
// invariants (cf. spec §11.6):
//
//   - Zero-ANSI: this renderer NEVER emits a 0x1b (ESC) byte from its own
//     logic. Embedded ANSI bytes already present in caller-supplied text are
//     passed through verbatim (asymmetric with \r below) because the
//     "tool-output passthrough" clause of §11.6 lets a tool that legitimately
//     produces colour escapes (e.g. `ls --color`) reach the operator's log.
//
//   - Zero-CR: every 0x0d (CR) byte is silently stripped before buffering.
//     Streaming sources frequently inject \r for in-place spinners; in plain
//     mode those would smear the transcript, so they are dropped on the floor.
//
//   - Line-buffered tokens: WriteToken accumulates into streamBuf; complete
//     lines (terminated by \n) are flushed to the underlying writer; the
//     trailing fragment, if any, stays buffered until the next \n or Commit.
//
//   - No diffing for frames: RenderFrame re-prints every line on every call.
//     There is no in-place update to optimise in non-TTY mode, and re-printing
//     produces an honest append-only transcript that is easy to grep.
//
// Thread-safety: a single sync.Mutex serialises all state-mutating methods.
package render

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
)

// crStripper removes carriage returns from text before it is buffered. The
// zero-CR invariant is enforced at this single chokepoint.
var crStripper = strings.NewReplacer("\r", "")

// plainRenderer is the non-TTY Renderer impl.
type plainRenderer struct {
	mu          sync.Mutex
	w           io.Writer
	closed      bool
	streamingID string
	streamBuf   bytes.Buffer
}

// NewPlainRenderer constructs a plain-mode Renderer writing to w.
func NewPlainRenderer(w io.Writer) *plainRenderer {
	return &plainRenderer{w: w}
}

// Mode returns ModePlain.
func (r *plainRenderer) Mode() RenderMode { return ModePlain }

// commitStreamingLocked flushes any incomplete buffered line as a single
// terminated line. Caller must hold r.mu. Safe to call when no block is open.
func (r *plainRenderer) commitStreamingLocked() error {
	if r.streamBuf.Len() > 0 {
		// Append \n to whatever fragment is buffered and emit as one write.
		if _, err := io.WriteString(r.w, r.streamBuf.String()+"\n"); err != nil {
			return err
		}
		r.streamBuf.Reset()
	}
	r.streamingID = ""
	return nil
}

// Begin opens a streaming block. Idempotent for the same blockID; switching
// to a new blockID first commits the previous one's incomplete line.
func (r *plainRenderer) Begin(blockID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	if blockID == "" {
		return ErrEmptyBlockID
	}
	if r.streamingID == blockID {
		return nil
	}
	if r.streamingID != "" {
		if err := r.commitStreamingLocked(); err != nil {
			return err
		}
	}
	r.streamingID = blockID
	return nil
}

// WriteToken appends text to the current streaming block. \r bytes are
// stripped; complete lines (\n-terminated) are flushed; any trailing fragment
// stays buffered until the next \n or Commit.
func (r *plainRenderer) WriteToken(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	if r.streamingID == "" {
		// auto-Begin with synthetic ID so ad-hoc fmt-style writes still render
		r.streamingID = "_auto"
	}
	if text == "" {
		return nil
	}

	// Zero-CR invariant: strip \r before any further processing.
	stripped := crStripper.Replace(text)
	if stripped == "" {
		return nil
	}

	r.streamBuf.WriteString(stripped)

	// Flush every complete line currently in the buffer.
	for {
		buf := r.streamBuf.Bytes()
		idx := bytes.IndexByte(buf, '\n')
		if idx < 0 {
			break
		}
		// Emit "<line>\n" (idx+1 bytes including the \n).
		line := buf[:idx+1]
		if _, err := r.w.Write(line); err != nil {
			return err
		}
		// Drop the consumed prefix from streamBuf.
		remaining := append([]byte(nil), buf[idx+1:]...)
		r.streamBuf.Reset()
		r.streamBuf.Write(remaining)
	}
	return nil
}

// Commit closes the current streaming block. Any incomplete line is emitted
// with a trailing \n appended.
func (r *plainRenderer) Commit() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	return r.commitStreamingLocked()
}

// RenderFrame emits each line of the frame followed by \n. No diffing: every
// call re-prints all lines. (Acceptable for log capture / non-TTY where there
// is no in-place update to optimise.)
func (r *plainRenderer) RenderFrame(frame Frame) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRendererClosed
	}
	if frame.BlockID == "" {
		return ErrEmptyBlockID
	}
	for _, line := range frame.Lines {
		if _, err := fmt.Fprintln(r.w, line); err != nil {
			return err
		}
	}
	return nil
}

// Close finalises any in-progress streaming block. Idempotent.
func (r *plainRenderer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	if r.streamingID != "" {
		if err := r.commitStreamingLocked(); err != nil {
			return err
		}
	}
	r.closed = true
	return nil
}
