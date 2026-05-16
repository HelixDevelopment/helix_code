// Package smartedit defines the foundational types, marker constants, size
// limits, and sentinel errors for HelixCode's SEARCH/REPLACE smart-edit tool
// (P1-F17).
//
// This file is type-only: every consumer (the block parser, the lenient
// applier, the diff wrapper, the agent tool, the `/edit` slash command, the
// integration test harness) imports the declarations here. Behaviour
// (parsing, applying, diffing, dispatch) lives in sibling files added by
// later T03-T07 tasks.
//
// The marker constants implement the claude-code / aider convention used by
// every modern SEARCH/REPLACE editing flow. Markers are matched as strict
// literal byte sequences: there is NO escape mechanism in v1, so a SEARCH
// section that itself contains a literal "<<<<<<< SEARCH" line is
// unrepresentable. Spec §5.3 documents the limitation and points users at
// the multiedit transactional tool as the work-around.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f17-smart-edit-tool-design.md
// Plan: docs/superpowers/plans/2026-05-06-p1-f17-smart-edit-tool.md
package smartedit

import (
	"errors"
	"time"
)

// SEARCH/REPLACE marker triplet — strict literal strings; matched at column 0
// by the parser. The exact bracket counts (7 of each) and the divider's seven
// equals signs are part of the on-disk contract; tests assert byte equality
// against the spec strings.
const (
	// MarkerSearch opens a SEARCH section.
	MarkerSearch = "<<<<<<< SEARCH"
	// MarkerDivider separates SEARCH from REPLACE within a single block.
	MarkerDivider = "======="
	// MarkerReplace closes a REPLACE section.
	MarkerReplace = ">>>>>>> REPLACE"
)

// Size limits — guard against pathological inputs at every layer of the
// pipeline. The parser checks `MaxPromptBytes`, `MaxBlocksPerPrompt`,
// `MaxSearchBytes`, and `MaxReplaceBytes`; the applier checks `MaxFileBytes`.
const (
	// MaxPromptBytes caps the total byte size of a single SEARCH/REPLACE
	// prompt fed to the parser. 1 MiB is generous for human-authored edits
	// while still bounding adversarial input.
	MaxPromptBytes = 1 << 20 // 1 MiB
	// MaxBlocksPerPrompt is a hard cap on the number of edit blocks one
	// prompt may carry. 100 is comfortably above the multi-file refactors
	// agents emit in practice.
	MaxBlocksPerPrompt = 100
	// MaxFileBytes is the maximum size the applier will read into memory
	// before refusing to edit. Larger files are rejected with
	// `OutcomeTooLarge` / `ErrFileTooLarge`.
	MaxFileBytes = 10 << 20 // 10 MiB
	// MaxSearchBytes caps a single SEARCH section. 64 KiB is far more than
	// any reasonable surrounding-context window.
	MaxSearchBytes = 1 << 16 // 64 KiB
	// MaxReplaceBytes caps a single REPLACE section. Same rationale as
	// MaxSearchBytes.
	MaxReplaceBytes = 1 << 16 // 64 KiB
)

// EditBlock represents a single SEARCH/REPLACE pair targeting a specific path.
//
// `LineStart` and `LineEnd` are 1-indexed line numbers in the SOURCE PROMPT
// (NOT in the target file). They exist so the parser can produce error
// messages that point back to the offending block when later validation
// fails partway through the prompt.
type EditBlock struct {
	Path      string // file path (relative or absolute) the block targets
	Search    string // the text to find — must match content literally
	Replace   string // the replacement text
	LineStart int    // 1-indexed start line in the source prompt
	LineEnd   int    // 1-indexed end line in the source prompt
}

// EditPlan is the parser's output: every block in source order plus a
// per-file index that downstream stages (T04 applier, T06 transaction)
// consume to batch reads and writes.
//
// `Blocks` and `PerFile` reference the same underlying EditBlock values; the
// PerFile index is purely a convenience grouping. `SourceBytes` records the
// total byte size of the prompt the parser consumed, enabling the
// MaxPromptBytes guard to be re-asserted by the applier as a defence in
// depth.
type EditPlan struct {
	Blocks      []EditBlock
	PerFile     map[string][]EditBlock
	SourceBytes int
}

// EditOutcome enumerates every terminal state for a single block application.
// String values are stable on-the-wire identifiers consumed by the
// integration-test harness and the `/edit` slash command's status output.
type EditOutcome string

const (
	// OutcomeApplied — SEARCH text was unique, REPLACE was applied, and the
	// post-write read-back confirmed the change.
	OutcomeApplied EditOutcome = "applied"
	// OutcomeNotFound — SEARCH text is absent from the current file content.
	OutcomeNotFound EditOutcome = "not-found"
	// OutcomeAmbiguous — SEARCH text appears more than once in the file;
	// the user must widen the SEARCH with surrounding context.
	OutcomeAmbiguous EditOutcome = "ambiguous"
	// OutcomeBinary — the file was detected as binary; smart-edit refuses
	// to operate on binary content.
	OutcomeBinary EditOutcome = "binary"
	// OutcomeReadFailed — `os.ReadFile` returned an error (missing file,
	// permission denied, …) before any edit attempt.
	OutcomeReadFailed EditOutcome = "read-failed"
	// OutcomeWriteFailed — the multiedit transaction failed during commit
	// (rename clash, disk full, fsync error, …); the file is unchanged.
	OutcomeWriteFailed EditOutcome = "write-failed"
	// OutcomeTooLarge — the file's size exceeds `MaxFileBytes` and the
	// applier refused to read it into memory.
	OutcomeTooLarge EditOutcome = "too-large"
)

// EditResult is the per-block outcome reported to callers. `Diff` is only
// populated when `Outcome == OutcomeApplied`; for failed outcomes it is
// omitted from JSON.
type EditResult struct {
	Block   EditBlock   `json:"block"`
	Outcome EditOutcome `json:"outcome"`
	Error   string      `json:"error,omitempty"`
	Diff    string      `json:"diff,omitempty"` // unified-diff text; only when applied
}

// SmartEditResult is the aggregate result for a whole prompt. `Diff` is the
// concatenation of every successful per-block diff in source order; callers
// can render it directly or split on the `--- ` header to per-file chunks.
//
// `Atomic` reports whether the multiedit whole-prompt commit succeeded.
// When `Atomic == false`, no file was modified on disk and `AtomicError`
// carries the underlying transaction failure.
type SmartEditResult struct {
	Results      []EditResult `json:"results"`
	AppliedCount int          `json:"applied_count"`
	FailedCount  int          `json:"failed_count"`
	Diff         string       `json:"diff"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  time.Time    `json:"completed_at"`
	Atomic       bool         `json:"atomic"`
	AtomicError  string       `json:"atomic_error,omitempty"`
}

// IsZero reports whether r is the zero value (no edits attempted, no timing
// recorded, no atomic state set). Used by the slash-command status renderer
// to distinguish "never ran" from "ran but produced no diffs".
//
// The check covers every field that could carry semantic content: a single
// non-zero field anywhere makes the result non-zero. Note that the zero
// `time.Time` is the canonical empty marker for `StartedAt` / `CompletedAt`.
func (r SmartEditResult) IsZero() bool {
	return len(r.Results) == 0 &&
		r.AppliedCount == 0 &&
		r.FailedCount == 0 &&
		r.Diff == "" &&
		r.StartedAt.IsZero() &&
		r.CompletedAt.IsZero() &&
		!r.Atomic &&
		r.AtomicError == ""
}

// Sentinel errors. Tests assert against these via errors.Is so each must
// carry a distinct, stable message.
var (
	// ErrInvalidBlockStructure — markers appeared in an order or shape
	// that does not parse to a SEARCH/REPLACE block (e.g. missing divider,
	// nested SEARCH, REPLACE before SEARCH, …).
	ErrInvalidBlockStructure = errors.New("invalid SEARCH/REPLACE block structure")
	// ErrSearchEmpty — the SEARCH section contained no bytes between the
	// opening marker and the divider. An empty SEARCH would match
	// everywhere; the parser rejects it.
	ErrSearchEmpty = errors.New("SEARCH section empty")
	// ErrSearchNotFound — applier could not locate the SEARCH text in the
	// current file content.
	ErrSearchNotFound = errors.New("SEARCH text not found in target file")
	// ErrSearchAmbiguous — applier found more than one occurrence of the
	// SEARCH text; the user must widen with surrounding context.
	ErrSearchAmbiguous = errors.New("SEARCH text matches multiple locations; widen with surrounding context")
	// ErrPromptTooLarge — prompt byte size exceeds MaxPromptBytes.
	ErrPromptTooLarge = errors.New("prompt exceeds MaxPromptBytes")
	// ErrTooManyBlocks — prompt declares more than MaxBlocksPerPrompt
	// edit blocks.
	ErrTooManyBlocks = errors.New("prompt has more than MaxBlocksPerPrompt edit blocks")
	// ErrFileTooLarge — target file size exceeds MaxFileBytes.
	ErrFileTooLarge = errors.New("file exceeds MaxFileBytes")
	// ErrSearchTooLarge — a single SEARCH section exceeds MaxSearchBytes.
	ErrSearchTooLarge = errors.New("SEARCH section exceeds MaxSearchBytes")
	// ErrReplaceTooLarge — a single REPLACE section exceeds MaxReplaceBytes.
	ErrReplaceTooLarge = errors.New("REPLACE section exceeds MaxReplaceBytes")
	// ErrBinaryFile — the file was detected as binary; smart-edit refuses
	// to operate on binary content.
	ErrBinaryFile = errors.New("file appears to be binary; refusing to edit")
	// ErrPathRequired — an edit block did not carry a target file path
	// (no preceding path-line and no path-stickiness inheritance available).
	ErrPathRequired = errors.New("edit block missing target file path")
)
