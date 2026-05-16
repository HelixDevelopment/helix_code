// Package smartedit — diff wrapper.
//
// This file thin-wraps F08's `multiedit.DiffManager` so smart-edit can emit
// unified-diff text without duplicating the line-comparison/grouping logic
// already shipped in `internal/tools/multiedit/diff.go`.
//
// Spec rationale (§3.4 — diff representation as plain string; §11.1 —
// decision rationale): smart-edit returns diffs as a single unified-diff
// string. The aggregate `SmartEditResult.Diff` is the source-order
// concatenation of every per-block successful diff. Per `EditResult.Diff`,
// each per-block diff is itself the unified-diff text for the file.
//
// Multiedit's API surface used here:
//
//	multiedit.NewDiffManager(format multiedit.DiffFormat) *multiedit.DiffManager
//	(*multiedit.DiffManager).GenerateDiff(oldContent, newContent []byte, filePath string) (*multiedit.Diff, error)
//	multiedit.Diff{ ..., Unified string, ... }
//
// We use `FormatUnified` exclusively — the spec is explicit that the
// on-the-wire shape is unified diff. We never reach into the multiedit
// `Hunks` struct from this package: the wrapper is a string-in / string-out
// boundary, which keeps the smartedit ↔ multiedit coupling minimal.
package smartedit

import (
	"bytes"
	"sort"
	"sync"

	"dev.helix.code/internal/tools/multiedit"
)

// Differ wraps multiedit's DiffManager and produces plain-string unified-diff
// output for smart-edit responses.
//
// Multiedit's DiffManager is documented as carrying per-call configuration
// (format) only and not internal mutable state, but we still serialise calls
// behind a mutex: (a) the wrapper is the canonical entry point shared across
// concurrent block-application goroutines, (b) it costs essentially nothing
// for the diff-of-a-single-file workloads the tool emits, and (c) the
// `TestDiffer_ConcurrentSafe` race-detector test enforces the invariant.
type Differ struct {
	inner *multiedit.DiffManager
	mu    sync.Mutex
}

// NewDiffer constructs a Differ over a fresh multiedit.DiffManager configured
// for unified-diff output.
func NewDiffer() *Differ {
	return &Differ{
		inner: multiedit.NewDiffManager(multiedit.FormatUnified),
	}
}

// FileDiff returns the unified-diff text comparing oldContent to newContent
// for filePath.
//
// Returns an empty string (and a nil error) if oldContent == newContent —
// this is the contract `EditResult.Diff` and `SmartEditResult.Diff` rely on
// when concatenating per-block diffs: empty diffs contribute nothing to the
// aggregate. Equality is checked at the byte level, so trailing-newline
// differences DO produce a diff (multiedit handles those at the line layer).
//
// The returned text is suitable for inclusion in `EditResult.Diff` and for
// concatenation into `SmartEditResult.Diff` via `CombinedDiff`.
func (d *Differ) FileDiff(filePath string, oldContent, newContent []byte) (string, error) {
	if bytes.Equal(oldContent, newContent) {
		return "", nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	diff, err := d.inner.GenerateDiff(oldContent, newContent, filePath)
	if err != nil {
		return "", err
	}
	if diff == nil {
		return "", nil
	}
	return diff.Unified, nil
}

// CombinedDiff joins multiple per-file diffs into a single string suitable
// for `SmartEditResult.Diff`. Files are emitted in deterministic key-sorted
// order so callers (and tests) get byte-identical output for byte-identical
// inputs regardless of map iteration order.
//
// Empty values are skipped so unchanged files do not pollute the aggregate.
// Returns an empty string if every value is empty (or the map itself is
// empty). The output preserves each per-file diff verbatim — including its
// `--- path` / `+++ path` header — so a downstream consumer can re-split on
// `--- ` to recover per-file chunks.
func (d *Differ) CombinedDiff(perFile map[string]string) string {
	if len(perFile) == 0 {
		return ""
	}

	keys := make([]string, 0, len(perFile))
	for k, v := range perFile {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(perFile[k])
	}
	return buf.String()
}
