// Tool-result frame rendering helpers (P1-F18-T08).
//
// RenderTextBlock and RenderLines wrap the Renderer.RenderFrame contract so
// agent call sites that print multi-line tool output (LLM responses, LSP
// diagnostics, file diffs, etc.) flow through the renderer's frame pipeline
// instead of bypassing it via bare fmt.Println. This buys two things:
//
//  1. Stable BlockIDs (e.g. "lsp-diagnostics", "smart-edit-diff") let the
//     fancy renderer's dirty-line diff (T05 viewport + T03 ANSI cursor moves)
//     emit only the lines that changed when the same logical block is
//     re-rendered. Bare fmt.Println re-emits everything every time.
//
//  2. Plain mode (non-TTY destinations) gets the zero-ANSI / zero-CR
//     guarantees from plain_renderer.go automatically; callers don't need
//     to branch on isatty themselves.
//
// Pragmatic v1 scope (per plan T08): provide the helper FUNCTIONS only. We do
// NOT refactor every existing print site in the codebase. The non-stream
// branch of cmd/cli/main.go::handleGenerate is wired as the canonical
// example; future tasks (or future cleanup PRs) can migrate other call sites
// incrementally.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f18-render-design.md §4.2
// Plan: docs/superpowers/plans/2026-05-06-p1-f18-render.md T08
package render

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// oneshotCounter generates unique synthetic BlockIDs for one-shot calls
// (RenderTextBlock with blockID==""). atomic so concurrent ad-hoc calls from
// independent goroutines don't collide on the same synthetic ID; the
// renderer itself is not concurrent-safe (cf. types.go) but a global ID
// counter being safe is cheap and avoids surprising aliasing.
var oneshotCounter uint64

// nextOneshotID returns a fresh synthetic BlockID for one-shot output.
// Format: "oneshot-<n>" where n is a monotonically increasing counter.
// Justification for the prefix: it is reserved (no production code uses
// "oneshot-" as a stable BlockID) so the diff path can never accidentally
// alias against a caller-supplied ID.
func nextOneshotID() string {
	n := atomic.AddUint64(&oneshotCounter, 1)
	return fmt.Sprintf("oneshot-%d", n)
}

// RenderTextBlock prints text as a single Frame through r. text is split on
// "\n" into lines (a trailing empty line — i.e. a final "\n" terminator — is
// dropped so "a\nb\n" yields lines [a, b], not [a, b, ""]).
//
// blockID semantics:
//   - "": one-shot output. A fresh synthetic BlockID is generated so each
//     call is independent (no diff against any previous frame). Use this for
//     ad-hoc print sites where the same logical block will not be re-rendered.
//   - non-empty: stable BlockID. Successive RenderTextBlock calls with the
//     same blockID + same r compute a dirty-line diff (fancy mode) or
//     re-print all lines (plain mode). Use this for blocks that are updated
//     over time (e.g. "lsp-diagnostics", "smart-edit-diff").
//
// Documented contract: empty text is a no-op — the underlying writer
// receives ZERO bytes. Justification: a frame with no lines has no semantic
// content (cf. Frame.IsZero in types.go) and emitting a stray newline or
// hide-cursor sequence for "nothing to render" would clutter the transcript.
func RenderTextBlock(r Renderer, blockID, text string) error {
	if text == "" {
		return nil
	}

	// Drop a single trailing newline so "a\nb\n" -> [a, b]. This matches the
	// shape callers naturally pass when stitching a multi-line string from a
	// reader or buffer that includes the final terminator.
	trimmed := strings.TrimSuffix(text, "\n")
	lines := strings.Split(trimmed, "\n")
	return RenderLines(r, blockID, lines)
}

// RenderLines is the slice variant of RenderTextBlock: lines are passed
// pre-split. Same blockID semantics as RenderTextBlock (empty -> synthetic
// one-shot ID; non-empty -> stable diffable ID).
//
// Documented contract: empty/nil lines is a no-op — the underlying writer
// receives ZERO bytes. (Same justification as RenderTextBlock empty-text.)
func RenderLines(r Renderer, blockID string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	id := blockID
	if id == "" {
		id = nextOneshotID()
	}
	return r.RenderFrame(Frame{BlockID: id, Lines: lines})
}
