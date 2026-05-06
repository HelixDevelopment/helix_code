// Package render defines the foundational types, sentinel errors, and the
// Renderer contract for HelixCode's terminal rendering layer (P1-F18).
//
// This file is type-only: every consumer (the ANSI fancy renderer in T03, the
// plain line-by-line renderer in T04, the diff helpers in T05, the factory in
// T06, and the agent/CLI call sites in T07-T09) imports the declarations here.
// Behaviour (cursor moves, dirty-line diffing, TTY detection, mode selection)
// lives in sibling files added by later tasks.
//
// Two distinct rendering flows share one Renderer interface:
//
//   - Token-streaming flow: Begin -> WriteToken... -> Commit. This is the
//     hot path for LLM token streams. In fancy mode each WriteToken updates
//     an in-progress line in place using CR + ANSI clear-line; in plain mode
//     output is buffered until a newline arrives so the visible result is
//     a clean line-per-line transcript with zero ANSI noise.
//
//   - Frame-based flow: RenderFrame for multi-line tool result blocks
//     identified by Frame.BlockID. Successive RenderFrame calls with the
//     same BlockID compute a dirty-line diff against the previous frame
//     and emit only the lines that changed (fancy); plain mode re-prints
//     all lines.
//
// Mode selection happens in the T06 factory, not in this file. The three
// RenderMode sentinels and the HELIXCODE_RENDER env-var name are the only
// configuration surface exposed at the type level.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f18-render-design.md
// Plan: docs/superpowers/plans/2026-05-06-p1-f18-render.md
package render

import (
	"errors"
	"io"
)

// RenderMode identifies which renderer is in use. The string values are also
// the legal values of the HELIXCODE_RENDER env var; comparison is exact and
// case-sensitive (no normalisation, no aliases) so that an operator who sets
// HELIXCODE_RENDER=Fancy gets a clear "invalid mode" error instead of a
// silent fallback.
type RenderMode string

const (
	// ModeFancy enables ANSI escape sequences and CR-based in-place line
	// updates. Suitable for interactive terminals (TTY).
	ModeFancy RenderMode = "fancy"
	// ModePlain emits line-by-line output via fmt.Fprintln, strips any
	// embedded \r or ANSI sequences, and never repositions the cursor.
	// Suitable for log files, pipes, CI consoles, and dumb terminals.
	ModePlain RenderMode = "plain"
	// ModeAuto defers the choice to the factory: fancy if the writer is a
	// real TTY, plain otherwise. ModeAuto MUST be resolved before reaching
	// a Renderer implementation; Renderer.Mode() never returns ModeAuto.
	ModeAuto RenderMode = "auto"
)

// IsValid reports whether m is one of the three documented RenderMode
// sentinels. Consumers (the T06 factory, the env-var parser) reject invalid
// modes with ErrInvalidMode rather than silently falling back to a default.
func (m RenderMode) IsValid() bool {
	switch m {
	case ModeFancy, ModePlain, ModeAuto:
		return true
	}
	return false
}

// EnvVarName is the configuration env var consulted by the factory:
// HELIXCODE_RENDER=plain|fancy|auto. Unset or empty falls back to ModeAuto.
const EnvVarName = "HELIXCODE_RENDER"

// Frame is a multi-line render block used by the frame-based rendering flow
// (tool result panels, status blocks, structured progress views).
//
// BlockID is a stable identifier so successive RenderFrame calls for "the
// same logical block" diff against the previous frame and emit only the
// changed lines (fancy mode) or just re-print all lines (plain mode). A
// Frame with an empty BlockID is rejected by the renderer with
// ErrEmptyBlockID.
//
// Lines are stored without trailing newlines: the renderer is responsible
// for inserting line terminators at emit time. This keeps diffing trivial
// (string equality per slot) and lets the same Frame value be passed to
// either mode without translation.
type Frame struct {
	BlockID string   // stable identifier for diff-against-previous-frame
	Lines   []string // line contents WITHOUT trailing newlines
}

// IsZero reports whether the frame carries no semantic content. A frame is
// zero when both fields are at their zero values: empty BlockID and either a
// nil or zero-length Lines slice. Used by the factory's idempotence checks
// and by tests to distinguish "never rendered" from "rendered an empty block".
func (f Frame) IsZero() bool {
	return f.BlockID == "" && len(f.Lines) == 0
}

// Renderer is the contract implemented by ansiRenderer (T03) and
// plainRenderer (T04). The two implementations share this interface so call
// sites are mode-agnostic; the factory picks the right concrete type.
//
// The interface is split across two flows that callers may interleave freely:
//
//   - Token-streaming flow (Begin -> WriteToken... -> Commit):
//     Begin starts a streaming block. Every subsequent WriteToken updates an
//     in-progress line in place (fancy: CR + clear-to-EOL + reprint) or
//     buffers until a newline flushes (plain). Commit ends the block,
//     ensuring the final line is terminated with \n.
//
//   - Frame-based flow (RenderFrame):
//     RenderFrame draws or updates a multi-line block identified by
//     frame.BlockID. Successive RenderFrame calls with the same BlockID
//     compute a dirty-line diff against the previous frame for that ID and
//     emit only the changed lines (fancy mode); plain mode re-prints all
//     lines for clarity in non-interactive transcripts.
//
// Implementations MUST be safe for sequential use by a single goroutine but
// are NOT required to be concurrent-safe; callers that interleave from
// multiple goroutines are responsible for synchronisation.
type Renderer interface {
	// Mode returns the active RenderMode. The returned value is always
	// ModeFancy or ModePlain - never ModeAuto, since auto is resolved by
	// the factory before the Renderer is constructed.
	Mode() RenderMode

	// Begin opens a streaming block with the given blockID. Idempotent:
	// calling Begin again with the same blockID is a no-op. Calling Begin
	// with a different blockID implicitly Commits the previous streaming
	// block before opening the new one. An empty blockID returns
	// ErrEmptyBlockID; calling Begin on a closed Renderer returns
	// ErrRendererClosed.
	Begin(blockID string) error

	// WriteToken appends text to the currently open streaming block. If no
	// block is open implementations MAY auto-Begin with a synthetic blockID
	// rather than erroring, so that ad-hoc fmt-style writes still render.
	// Returns ErrRendererClosed when the renderer has been Closed.
	WriteToken(text string) error

	// Commit closes the current streaming block, ensuring the last emitted
	// line is terminated with a newline. Calling Commit with no open block
	// is a no-op. Returns ErrRendererClosed when the renderer has been
	// Closed.
	Commit() error

	// RenderFrame draws or updates a frame block. The first call for a
	// given BlockID emits all lines; subsequent calls for the same BlockID
	// diff against the previous frame and emit only the changed lines
	// (fancy) or re-print all lines (plain). An empty BlockID returns
	// ErrEmptyBlockID; calling RenderFrame on a closed Renderer returns
	// ErrRendererClosed.
	RenderFrame(frame Frame) error

	// Close releases any resources held by the renderer (re-shows the
	// cursor in fancy mode, flushes pending plain-mode buffers). After
	// Close all other methods return ErrRendererClosed. Close is
	// idempotent: a second call after a successful Close returns nil.
	Close() error
}

// Sentinel errors. Tests compare via errors.Is so each must carry a distinct,
// stable message that survives refactors.
var (
	// ErrInvalidMode is returned by the env-var parser and the T06
	// factory when a RenderMode value fails IsValid().
	ErrInvalidMode = errors.New("invalid render mode")
	// ErrRendererClosed is returned by every Renderer method after the
	// renderer has been Closed.
	ErrRendererClosed = errors.New("renderer is closed")
	// ErrEmptyBlockID is returned by Begin and RenderFrame when the
	// supplied block identifier is the empty string.
	ErrEmptyBlockID = errors.New("frame block ID required")
)

// FactoryOptions configures the RendererFactory implemented in T06. The zero
// value is intentionally "everything defaults" (Writer = os.Stdout,
// Mode = ModeAuto, IsTTY = real terminal probe, EnvLookup = os.Getenv) so
// that callers who only want sane behaviour can pass FactoryOptions{}.
type FactoryOptions struct {
	// Writer is the destination stream. Defaults to os.Stdout when nil.
	Writer io.Writer
	// Mode is the requested render mode. Defaults to ModeAuto when empty.
	// ModeAuto resolves to ModeFancy if Writer is a TTY, ModePlain
	// otherwise.
	Mode RenderMode
	// IsTTY is the TTY probe used by ModeAuto resolution. Defaults to a
	// real term.IsTerminal probe on Writer when Writer is an *os.File;
	// defaults to a function that returns false for any other writer.
	IsTTY func() bool
	// EnvLookup is the env-var reader used to resolve HELIXCODE_RENDER.
	// Defaults to os.Getenv. Tests inject a deterministic lookup.
	EnvLookup func(string) string
}
