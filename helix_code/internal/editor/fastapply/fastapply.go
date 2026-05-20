// Package fastapply implements the dedicated fast-apply path of the
// HelixCode speed programme (Phase 3, task P3-T03).
//
// # Why a fast-apply path
//
// Editing a file by having the frontier model re-emit the entire file is
// slow: the model must regenerate every unchanged byte at frontier-tier
// throughput. The industry answer (R2 #3) is a specialised apply route —
// Cursor's speculative edits (9–13×) and Morph's specialised apply model
// (4500–10,500 tok/s). A fast-apply path routes file application to a
// small/specialised model (via the [routing] package, P3-T01) and/or to a
// speculative-edit strategy where the original file seeds the draft, giving
// roughly an order-of-magnitude faster interactive file apply.
//
// # The non-negotiable correctness invariant
//
// Apply correctness is non-negotiable. A fast-apply that produces a wrong
// file is worse than no optimisation. This package therefore NEVER trusts
// the fast route blindly. Every fast-apply result is compared
// byte-for-byte against a deterministic reference apply. The fast bytes are
// returned ONLY when they are byte-identical to the reference; on ANY
// mismatch, error, or low-confidence signal the package falls back to the
// reference output. The user always receives a correct file.
//
// # Reference apply
//
// The reference apply is a fully deterministic, in-process transformation
// of (original file, [Instruction]) → edited bytes. It is the trusted
// oracle: it does no network I/O, never guesses, and either applies the
// instruction exactly or fails. See [ReferenceApply].
//
// # Fast apply
//
// The fast route is supplied by the caller as a [FastEditFunc] — typically
// a wrapper around a small/specialised apply model dispatched through the
// P3-T01 [routing.Router]. The fast route receives the original file plus
// the edit instruction and returns the candidate edited file. Because the
// candidate is always reference-verified, a buggy or hallucinating fast
// model can only ever cost a little wasted latency — never a wrong file.
//
// # Config gate (no-regression safety valve)
//
// An [Applier] built with [Config].Enabled == false performs the reference
// apply ONLY — the fast route is never invoked. That switch makes the
// fast-apply path a pure no-op relative to the established reference
// behaviour, satisfying the Med-High-risk no-regression constraint:
// disabling fast-apply yields byte-identical output to the pre-existing
// reference apply.
//
// # Decoupling (CONST-051(B))
//
// This package is project-not-aware and reusable. It depends only on the
// standard library and the sibling [routing] package; the concrete fast
// model, the verifier wiring, and the provider plumbing all live in the
// caller via the [FastEditFunc] / [routing.ModelResolver] seams.
package fastapply

import (
	"errors"
	"strings"
)

// EditKind classifies the structural shape of an [Instruction]. It drives
// the reference apply and lets benchmarks/tests categorise the edit corpus.
type EditKind int

const (
	// EditReplace replaces an exact occurrence of Search with Replace.
	EditReplace EditKind = iota

	// EditInsertBefore inserts Replace immediately before the first
	// occurrence of Anchor.
	EditInsertBefore

	// EditInsertAfter inserts Replace immediately after the first
	// occurrence of Anchor.
	EditInsertAfter

	// EditDelete deletes the first exact occurrence of Search.
	EditDelete

	// EditAppend appends Replace to the end of the file.
	EditAppend

	// EditPrepend prepends Replace to the start of the file.
	EditPrepend
)

// String renders an EditKind for logging and evidence capture.
func (k EditKind) String() string {
	switch k {
	case EditReplace:
		return "replace"
	case EditInsertBefore:
		return "insert_before"
	case EditInsertAfter:
		return "insert_after"
	case EditDelete:
		return "delete"
	case EditAppend:
		return "append"
	case EditPrepend:
		return "prepend"
	default:
		return "unknown"
	}
}

// Hunk is a single, unambiguous edit operation against a file. A multi-hunk
// edit is expressed as an ordered slice of Hunks applied in sequence — each
// hunk operates on the output of the previous one.
type Hunk struct {
	// Kind is the structural shape of this hunk.
	Kind EditKind

	// Search is the exact text the hunk locates (EditReplace, EditDelete).
	Search string

	// Anchor is the exact text an insertion is positioned relative to
	// (EditInsertBefore, EditInsertAfter).
	Anchor string

	// Replace is the new text (EditReplace, EditInsert*, EditAppend,
	// EditPrepend). Empty for EditDelete.
	Replace string
}

// Instruction is a complete, deterministic description of an edit to apply
// to one file. It is the input both the reference apply and the fast apply
// operate on — they MUST produce identical output for the same Instruction
// or the fast result is discarded.
type Instruction struct {
	// FilePath is the logical path of the file (informational; the bytes
	// to edit are passed explicitly to Apply).
	FilePath string

	// Hunks are the ordered edit operations. Applied in slice order.
	Hunks []Hunk

	// Prompt is an optional natural-language description of the edit. The
	// fast route may use it; the reference apply ignores it entirely
	// (the reference apply is purely structural and deterministic).
	Prompt string
}

// Sentinel errors for the fast-apply path.
var (
	// ErrNilInstruction is returned when Apply is given a nil Instruction.
	ErrNilInstruction = errors.New("fastapply: instruction must not be nil")

	// ErrNoHunks is returned when an Instruction carries no hunks.
	ErrNoHunks = errors.New("fastapply: instruction has no hunks")

	// ErrSearchNotFound is returned by the reference apply when a hunk's
	// Search/Anchor text is not present in the file. A fast route that
	// cannot satisfy the instruction reports this same error and the
	// Applier falls back to the (also-failing) reference apply so the
	// caller sees a single, honest error rather than a wrong file.
	ErrSearchNotFound = errors.New("fastapply: hunk search/anchor text not found in file")

	// ErrAmbiguousSearch is returned by the reference apply when a hunk's
	// Search text occurs more than once and the edit would be ambiguous.
	ErrAmbiguousSearch = errors.New("fastapply: hunk search text is ambiguous (multiple matches)")
)

// applyHunk applies a single hunk to content and returns the new content.
// It is the deterministic core shared by the reference apply. It never
// guesses: an unfindable or ambiguous target is a hard error.
func applyHunk(content string, h Hunk) (string, error) {
	switch h.Kind {
	case EditAppend:
		return content + h.Replace, nil
	case EditPrepend:
		return h.Replace + content, nil
	case EditReplace, EditDelete:
		if h.Search == "" {
			return "", ErrSearchNotFound
		}
		idx := strings.Index(content, h.Search)
		if idx < 0 {
			return "", ErrSearchNotFound
		}
		// Ambiguity guard: an edit whose target appears more than once
		// could land in the wrong place. Reject rather than guess.
		if strings.Index(content[idx+len(h.Search):], h.Search) >= 0 {
			return "", ErrAmbiguousSearch
		}
		repl := h.Replace
		if h.Kind == EditDelete {
			repl = ""
		}
		return content[:idx] + repl + content[idx+len(h.Search):], nil
	case EditInsertBefore, EditInsertAfter:
		if h.Anchor == "" {
			return "", ErrSearchNotFound
		}
		idx := strings.Index(content, h.Anchor)
		if idx < 0 {
			return "", ErrSearchNotFound
		}
		if strings.Index(content[idx+len(h.Anchor):], h.Anchor) >= 0 {
			return "", ErrAmbiguousSearch
		}
		if h.Kind == EditInsertBefore {
			return content[:idx] + h.Replace + content[idx:], nil
		}
		end := idx + len(h.Anchor)
		return content[:end] + h.Replace + content[end:], nil
	default:
		return "", ErrSearchNotFound
	}
}

// ReferenceApply applies an [Instruction] to original deterministically and
// returns the edited bytes. It is the trusted oracle every fast-apply
// result is verified against: pure, in-process, no network I/O, no
// guessing. A hunk whose target cannot be located unambiguously is a hard
// error — the reference apply never produces a wrong file silently.
//
// ReferenceApply is exported so callers can use the reference path directly
// (e.g. when fast-apply is config-disabled) and so tests can assert
// fast-apply byte-equality against it.
func ReferenceApply(instr *Instruction, original []byte) ([]byte, error) {
	if instr == nil {
		return nil, ErrNilInstruction
	}
	if len(instr.Hunks) == 0 {
		return nil, ErrNoHunks
	}
	content := string(original)
	for _, h := range instr.Hunks {
		next, err := applyHunk(content, h)
		if err != nil {
			return nil, err
		}
		content = next
	}
	return []byte(content), nil
}
