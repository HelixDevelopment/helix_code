package fastapply

import (
	"context"
	"strings"
)

// Speculative-edit strategy.
//
// The speculative-edit strategy (R2 #3 — Cursor speculative edits, 9–13×)
// exploits the fact that an edited file is overwhelmingly identical to the
// original. Instead of a model re-emitting every byte, the original file
// SEEDS the draft: the unchanged regions are reused verbatim and only the
// edited spans are produced fresh. A speculative-decoding backend then
// verifies the seeded draft token-by-token, accepting long unchanged runs
// in a single step.
//
// This file provides a pure-Go speculative apply that needs no model at
// all: it performs the exact same structural transformation as the
// reference apply but is the canonical "fast route" used to exercise and
// benchmark the fast-apply path end-to-end without a live provider. Because
// it produces byte-identical output to the reference apply by construction,
// it always passes the [Applier]'s mandatory byte verification — proving
// the verification + ship-fast path works. A real speculative-decoding
// provider plugs in via [SpeculativeFastEditFunc] returning a candidate
// that the Applier still byte-verifies.

// SpeculativeDraft is the seed for a speculative apply: the original file
// split into the unchanged prefix/suffix that bracket the edited region.
// A speculative-decoding backend accepts the prefix and suffix in bulk and
// only generates the changed span.
type SpeculativeDraft struct {
	// Prefix is the verbatim unchanged head of the file.
	Prefix string

	// Changed is the freshly-produced edited span.
	Changed string

	// Suffix is the verbatim unchanged tail of the file.
	Suffix string
}

// Bytes assembles the draft into the full file bytes.
func (d SpeculativeDraft) Bytes() []byte {
	return []byte(d.Prefix + d.Changed + d.Suffix)
}

// AcceptedRatio reports the fraction of the file reused verbatim from the
// original (prefix+suffix) — the speculative "acceptance rate". A high
// ratio is exactly why speculative edits are fast: most of the file is
// accepted without generation.
func (d SpeculativeDraft) AcceptedRatio() float64 {
	total := len(d.Prefix) + len(d.Changed) + len(d.Suffix)
	if total == 0 {
		return 1.0
	}
	return float64(len(d.Prefix)+len(d.Suffix)) / float64(total)
}

// SpeculativeApply performs a single-hunk speculative apply: it locates the
// edited span, reuses the unchanged prefix and suffix verbatim, and returns
// a [SpeculativeDraft]. For multi-hunk instructions it falls back to a
// sequential whole-content transform (each hunk seeded by the previous
// output). The result is byte-identical to [ReferenceApply] by
// construction.
//
// SpeculativeApply is deterministic and does no network I/O; it is the
// in-process fast route used to drive and benchmark the fast-apply path.
func SpeculativeApply(instr *Instruction, original []byte) (SpeculativeDraft, error) {
	if instr == nil {
		return SpeculativeDraft{}, ErrNilInstruction
	}
	if len(instr.Hunks) == 0 {
		return SpeculativeDraft{}, ErrNoHunks
	}

	// Single-hunk fast path: compute prefix / changed / suffix directly so
	// the speculative acceptance ratio is meaningful.
	if len(instr.Hunks) == 1 {
		return speculativeSingleHunk(string(original), instr.Hunks[0])
	}

	// Multi-hunk: apply sequentially, then express the final result as a
	// draft whose Changed span is the longest-common-prefix/suffix-trimmed
	// difference vs the original. Still byte-identical to ReferenceApply.
	final, err := ReferenceApply(instr, original)
	if err != nil {
		return SpeculativeDraft{}, err
	}
	return diffDraft(string(original), string(final)), nil
}

// speculativeSingleHunk computes the speculative draft for one hunk.
func speculativeSingleHunk(content string, h Hunk) (SpeculativeDraft, error) {
	switch h.Kind {
	case EditAppend:
		return SpeculativeDraft{Prefix: content, Changed: h.Replace, Suffix: ""}, nil
	case EditPrepend:
		return SpeculativeDraft{Prefix: "", Changed: h.Replace, Suffix: content}, nil
	case EditReplace, EditDelete:
		if h.Search == "" {
			return SpeculativeDraft{}, ErrSearchNotFound
		}
		idx := strings.Index(content, h.Search)
		if idx < 0 {
			return SpeculativeDraft{}, ErrSearchNotFound
		}
		if strings.Index(content[idx+len(h.Search):], h.Search) >= 0 {
			return SpeculativeDraft{}, ErrAmbiguousSearch
		}
		repl := h.Replace
		if h.Kind == EditDelete {
			repl = ""
		}
		return SpeculativeDraft{
			Prefix:  content[:idx],
			Changed: repl,
			Suffix:  content[idx+len(h.Search):],
		}, nil
	case EditInsertBefore, EditInsertAfter:
		if h.Anchor == "" {
			return SpeculativeDraft{}, ErrSearchNotFound
		}
		idx := strings.Index(content, h.Anchor)
		if idx < 0 {
			return SpeculativeDraft{}, ErrSearchNotFound
		}
		if strings.Index(content[idx+len(h.Anchor):], h.Anchor) >= 0 {
			return SpeculativeDraft{}, ErrAmbiguousSearch
		}
		if h.Kind == EditInsertBefore {
			return SpeculativeDraft{Prefix: content[:idx], Changed: h.Replace, Suffix: content[idx:]}, nil
		}
		end := idx + len(h.Anchor)
		return SpeculativeDraft{Prefix: content[:end], Changed: h.Replace, Suffix: content[end:]}, nil
	default:
		return SpeculativeDraft{}, ErrSearchNotFound
	}
}

// diffDraft expresses the transformation original→final as a
// [SpeculativeDraft] by trimming the longest common prefix and suffix.
// This is exactly the span a speculative-decoding backend would accept
// verbatim and skip generating.
func diffDraft(original, final string) SpeculativeDraft {
	// Longest common prefix.
	p := 0
	for p < len(original) && p < len(final) && original[p] == final[p] {
		p++
	}
	// Longest common suffix, not overlapping the prefix.
	s := 0
	for s < len(original)-p && s < len(final)-p &&
		original[len(original)-1-s] == final[len(final)-1-s] {
		s++
	}
	return SpeculativeDraft{
		Prefix:  final[:p],
		Changed: final[p : len(final)-s],
		Suffix:  final[len(final)-s:],
	}
}

// SpeculativeFastEditFunc returns a [FastEditFunc] backed by the in-process
// speculative apply. It is the canonical fast route for the fast-apply
// path: deterministic, model-free, and byte-identical to [ReferenceApply]
// by construction — so it always passes the [Applier]'s mandatory byte
// verification and drives the fast ship-path end-to-end.
//
// A real specialised-apply model or a speculative-decoding provider
// substitutes its own [FastEditFunc]; the [Applier] byte-verifies it the
// same way, so a hallucinating model can only ever trigger a fallback.
func SpeculativeFastEditFunc() FastEditFunc {
	return func(_ context.Context, _ string, instr *Instruction, original []byte) ([]byte, error) {
		draft, err := SpeculativeApply(instr, original)
		if err != nil {
			return nil, err
		}
		return draft.Bytes(), nil
	}
}
