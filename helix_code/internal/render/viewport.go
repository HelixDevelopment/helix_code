// Viewport: frame-buffer + dirty-line tracking + pure-Go Diff (P1-F18-T05).
//
// A Viewport tracks the lines most recently rendered for one logical block
// (identified by Frame.BlockID) and computes the minimal LineDiff needed to
// transform the prior frame into a new one. Renderers (the ANSI fancy
// renderer in T03, future renderers in subsequent tasks) consume the
// LineDiff to decide which lines to redraw on the terminal.
//
// State machine:
//   - Initially: lines == nil.
//   - After Apply(frame): lines == defensive-copy(frame.Lines).
//
// The standalone Diff function is intentionally exposed and pure: it reads
// no global state and mutates neither input slice. Renderers and tests can
// inspect a diff without constructing a Viewport.
//
// Algorithm (per task scope, sufficient for terminal frames where lines
// are checked independently): O(min(len)) pairwise compare.
//   - For i < min(len(old), len(new)): if old[i] != new[i] -> Changed += i.
//   - For i in [len(old), len(new))    -> Appended += i.
//   - If len(old) > len(new)           -> Truncated = len(old) - len(new).
//
// Anything more sophisticated (Myers, etc.) is overkill for v1.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f18-render-design.md §3
// Plan: docs/superpowers/plans/2026-05-06-p1-f18-render.md T05
package render

// LineDiff describes the minimal set of changes between two frames.
//
// Indices in Changed and Appended are 0-based and refer to positions in the
// NEW frame. Truncated reports the count of trailing lines present in the
// OLD frame but absent from the NEW one. Renderers may choose to visually
// clear truncated lines; the v1 ANSI renderer does not (documented).
type LineDiff struct {
	// Changed lists the indices that differ between OLD[i] and NEW[i] for
	// i < min(len(OLD), len(NEW)).
	Changed []int

	// Appended lists indices that exist in NEW but not in OLD (i.e. the
	// new frame is longer). These trail the bottom of the frame.
	Appended []int

	// Truncated is the count of trailing lines that were in OLD but are
	// not in NEW.
	Truncated int
}

// IsNoChange reports whether the diff is empty (no Changed, no Appended,
// no Truncated).
func (d LineDiff) IsNoChange() bool {
	return len(d.Changed) == 0 && len(d.Appended) == 0 && d.Truncated == 0
}

// Viewport tracks the lines of a single frame block and computes minimal
// diffs against incoming new frames. A Viewport is NOT safe for concurrent
// use; renderers that hold per-blockID viewports must serialise access.
type Viewport struct {
	blockID string
	lines   []string
}

// NewViewport constructs an empty viewport for the given blockID.
func NewViewport(blockID string) *Viewport {
	return &Viewport{blockID: blockID}
}

// BlockID returns the viewport's block identifier.
func (v *Viewport) BlockID() string { return v.blockID }

// LineCount returns the current number of buffered lines.
func (v *Viewport) LineCount() int { return len(v.lines) }

// Lines returns a defensive copy of the current lines. Mutating the
// returned slice does not affect the viewport's state.
func (v *Viewport) Lines() []string {
	if v.lines == nil {
		return nil
	}
	out := make([]string, len(v.lines))
	copy(out, v.lines)
	return out
}

// Apply replaces the viewport's buffered lines with a defensive copy of
// frame.Lines and returns the LineDiff that was needed to transform the
// previous buffer into the new one.
func (v *Viewport) Apply(frame Frame) LineDiff {
	d := Diff(v.lines, frame.Lines)
	// Defensive copy so subsequent caller mutations of frame.Lines do not
	// affect the viewport's recorded state.
	if frame.Lines == nil {
		v.lines = nil
	} else {
		next := make([]string, len(frame.Lines))
		copy(next, frame.Lines)
		v.lines = next
	}
	return d
}

// Diff is a pure function that computes the LineDiff between two slices.
// Used by Viewport.Apply but exposed for direct use and inspection.
//
// Diff is pure in the sense that it reads no package state, mutates
// neither input, and always returns the same LineDiff for the same inputs.
func Diff(oldLines, newLines []string) LineDiff {
	var d LineDiff
	overlap := len(oldLines)
	if len(newLines) < overlap {
		overlap = len(newLines)
	}
	for i := 0; i < overlap; i++ {
		if oldLines[i] != newLines[i] {
			d.Changed = append(d.Changed, i)
		}
	}
	if len(newLines) > len(oldLines) {
		for i := len(oldLines); i < len(newLines); i++ {
			d.Appended = append(d.Appended, i)
		}
	}
	if len(oldLines) > len(newLines) {
		d.Truncated = len(oldLines) - len(newLines)
	}
	return d
}
