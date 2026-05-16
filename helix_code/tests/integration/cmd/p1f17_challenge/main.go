// p1f17_challenge runs the F17 smart-file-editing harness end-to-end against
// real on-disk tempdirs and a real *multiedit.MultiFileEditor committer per
// phase. Every phase emits sha-256 positive runtime evidence (file content
// hash before / after) — Article XI §11.9 anti-bluff anchor: a regression
// that "succeeds" without writing the disk loses Phase A; a regression that
// loses transactional rollback fails Phase D.
//
// Phases (every phase is always-runs; no real-LLM dependency):
//
//	A. SINGLE-FILE edit applied — sha256(after) reflects the replacement.
//	B. NOT-FOUND aborts        — Atomic=false, sha256(after)==sha256(before).
//	C. MULTI-FILE atomic commit — both files replaced; per-file sha256
//	                              matches expected post-content.
//	D. ROLLBACK on partial fail — block1 applies in-memory, block2 misses;
//	                              whole-prompt gate aborts so BOTH files'
//	                              sha256 remain equal to before. Load-bearing
//	                              atomicity proof.
//	E. DIFF EXACTNESS          — independent unified-diff vs result.Diff;
//	                              the harness's own diff lines must be
//	                              substrings of result.Diff (T05 wrapper
//	                              produces the same unified output).
//	F. AMBIG                   — SEARCH appears twice → Atomic=false, file
//	                              unchanged on disk.
//	G. BINARY                  — file with NUL byte → Atomic=false, file
//	                              unchanged on disk.
//
// Exit code 0 on success; exit 1 with a diagnostic on any check failure.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/tools/multiedit"
	"dev.helix.code/internal/tools/smartedit"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F17 challenge harness pid:", os.Getpid())

	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}
	if err := phaseF(); err != nil {
		return fmt.Errorf("phase F: %w", err)
	}
	if err := phaseG(); err != nil {
		return fmt.Errorf("phase G: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F17 challenge harness PASS")
	return nil
}

// phaseA exercises the happy path: a single SEARCH/REPLACE block on a single
// file lands on disk. Evidence is the sha256 of the file BEFORE the commit
// vs sha256 of the file AFTER the commit; the post-hash MUST equal the
// canonical sha256 of the expected content. A regression that "succeeded"
// without writing the disk would leave sha(after)==sha(before).
func phaseA() error {
	fmt.Println("==> phase A: SINGLE-FILE edit applied (always runs)")

	dir, err := newTempdir("p1f17-phase-a-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tool, err := newToolForDir(dir)
	if err != nil {
		return err
	}

	const before = "hello\nold-line\nworld\n"
	const want = "hello\nnew-line\nworld\n"
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		return err
	}

	shaBefore := sha256Hex([]byte(before))
	prompt := buildPrompt(path, "old-line", "new-line")

	res, err := tool.Commit(context.Background(), prompt, dir)
	if err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if !res.Atomic {
		return fmt.Errorf("Atomic=false unexpected; AtomicError=%q", res.AtomicError)
	}

	disk, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	shaAfter := sha256Hex(disk)
	shaWant := sha256Hex([]byte(want))

	if shaAfter == shaBefore {
		return fmt.Errorf("file unchanged: sha256(after)==sha256(before)=%s", shaAfter)
	}
	if shaAfter != shaWant {
		return fmt.Errorf("post-content mismatch: sha256(after)=%s want=%s\n--- disk ---\n%s",
			shaAfter, shaWant, string(disk))
	}

	fmt.Printf("    file             : %s\n", path)
	fmt.Printf("    sha256_before    : %s\n", shaBefore)
	fmt.Printf("    sha256_after     : %s\n", shaAfter)
	fmt.Printf("    sha256_expected  : %s\n", shaWant)
	fmt.Printf("    verdict          : edit landed on disk; hashes confirm replacement\n")
	return nil
}

// phaseB exercises the NOT-FOUND path: the SEARCH text is absent so the
// applier rejects the block; the whole-prompt atomicity gate aborts the
// commit. Evidence: sha256(after) MUST equal sha256(before) — the file is
// untouched. A regression that wrote partial state would change sha(after).
func phaseB() error {
	fmt.Println("==> phase B: NOT-FOUND aborts (always runs)")

	dir, err := newTempdir("p1f17-phase-b-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tool, err := newToolForDir(dir)
	if err != nil {
		return err
	}

	const before = "hello\n"
	path := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		return err
	}
	shaBefore := sha256Hex([]byte(before))

	prompt := buildPrompt(path, "completely-absent", "new")
	res, err := tool.Commit(context.Background(), prompt, dir)
	if err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if res.Atomic {
		return fmt.Errorf("Atomic=true unexpected; expected whole-prompt abort")
	}
	if res.AtomicError == "" {
		return fmt.Errorf("AtomicError is empty; expected diagnostic")
	}

	disk, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	shaAfter := sha256Hex(disk)
	if shaAfter != shaBefore {
		return fmt.Errorf("file changed despite NOT-FOUND: before=%s after=%s",
			shaBefore, shaAfter)
	}

	fmt.Printf("    file             : %s\n", path)
	fmt.Printf("    sha256_before    : %s\n", shaBefore)
	fmt.Printf("    sha256_after     : %s\n", shaAfter)
	fmt.Printf("    atomic_error     : %s\n", truncate(res.AtomicError, 120))
	fmt.Printf("    verdict          : NOT-FOUND aborted commit; disk untouched\n")
	return nil
}

// phaseC exercises the multi-file happy path: two files, two blocks, one
// transaction. Both files MUST land on disk in the same commit. Evidence:
// per-file sha256(after) matches the canonical sha256 of the expected
// post-content for that file.
func phaseC() error {
	fmt.Println("==> phase C: MULTI-FILE atomic commit (always runs)")

	dir, err := newTempdir("p1f17-phase-c-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tool, err := newToolForDir(dir)
	if err != nil {
		return err
	}

	const before1 = "alpha\n"
	const before2 = "beta\n"
	const want1 = "gamma\n"
	const want2 = "delta\n"
	p1 := filepath.Join(dir, "c1.txt")
	p2 := filepath.Join(dir, "c2.txt")
	if err := os.WriteFile(p1, []byte(before1), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(p2, []byte(before2), 0o644); err != nil {
		return err
	}
	shaBefore1 := sha256Hex([]byte(before1))
	shaBefore2 := sha256Hex([]byte(before2))

	prompt := buildPrompt(p1, "alpha", "gamma") + buildPrompt(p2, "beta", "delta")
	res, err := tool.Commit(context.Background(), prompt, dir)
	if err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if !res.Atomic {
		return fmt.Errorf("Atomic=false unexpected; AtomicError=%q", res.AtomicError)
	}

	disk1, _ := os.ReadFile(p1)
	disk2, _ := os.ReadFile(p2)
	shaAfter1 := sha256Hex(disk1)
	shaAfter2 := sha256Hex(disk2)
	shaWant1 := sha256Hex([]byte(want1))
	shaWant2 := sha256Hex([]byte(want2))

	if shaAfter1 != shaWant1 {
		return fmt.Errorf("c1.txt post-content mismatch: got=%s want=%s", shaAfter1, shaWant1)
	}
	if shaAfter2 != shaWant2 {
		return fmt.Errorf("c2.txt post-content mismatch: got=%s want=%s", shaAfter2, shaWant2)
	}

	fmt.Printf("    file_1           : %s\n", p1)
	fmt.Printf("    sha256_before_1  : %s\n", shaBefore1)
	fmt.Printf("    sha256_after_1   : %s\n", shaAfter1)
	fmt.Printf("    file_2           : %s\n", p2)
	fmt.Printf("    sha256_before_2  : %s\n", shaBefore2)
	fmt.Printf("    sha256_after_2   : %s\n", shaAfter2)
	fmt.Printf("    verdict          : both files landed atomically; per-file hashes confirm replacements\n")
	return nil
}

// phaseD is the load-bearing atomicity proof for whole-prompt rollback.
// File 1's block applies cleanly in memory; file 2's block fails (SEARCH
// text absent). The whole-prompt gate MUST abort the commit so file 1 is
// NEVER written even though the applier produced a valid in-memory result
// for it. Evidence: BOTH files' sha256(after)==sha256(before). A regression
// that committed file 1 because "block 1 applied" would fail this phase.
func phaseD() error {
	fmt.Println("==> phase D: ROLLBACK on partial failure (always runs)")

	dir, err := newTempdir("p1f17-phase-d-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tool, err := newToolForDir(dir)
	if err != nil {
		return err
	}

	const before1 = "applies-fine\n"
	const before2 = "hello\n"
	p1 := filepath.Join(dir, "d1.txt")
	p2 := filepath.Join(dir, "d2.txt")
	if err := os.WriteFile(p1, []byte(before1), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(p2, []byte(before2), 0o644); err != nil {
		return err
	}
	shaBefore1 := sha256Hex([]byte(before1))
	shaBefore2 := sha256Hex([]byte(before2))

	// block1: applies fine; block2: SEARCH absent so whole prompt aborts.
	prompt := buildPrompt(p1, "applies-fine", "changed") + buildPrompt(p2, "absent-text", "whatever")
	res, err := tool.Commit(context.Background(), prompt, dir)
	if err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if res.Atomic {
		return fmt.Errorf("Atomic=true unexpected; whole-prompt gate must abort when any block fails")
	}
	if res.AtomicError == "" {
		return fmt.Errorf("AtomicError is empty; expected diagnostic for d2.txt failing block")
	}

	disk1, err := os.ReadFile(p1)
	if err != nil {
		return err
	}
	disk2, err := os.ReadFile(p2)
	if err != nil {
		return err
	}
	shaAfter1 := sha256Hex(disk1)
	shaAfter2 := sha256Hex(disk2)

	if shaAfter1 != shaBefore1 {
		return fmt.Errorf("d1.txt was written despite whole-prompt abort: before=%s after=%s",
			shaBefore1, shaAfter1)
	}
	if shaAfter2 != shaBefore2 {
		return fmt.Errorf("d2.txt was written despite whole-prompt abort: before=%s after=%s",
			shaBefore2, shaAfter2)
	}

	fmt.Printf("    file_1           : %s\n", p1)
	fmt.Printf("    sha256_before_1  : %s\n", shaBefore1)
	fmt.Printf("    sha256_after_1   : %s (== before)\n", shaAfter1)
	fmt.Printf("    file_2           : %s\n", p2)
	fmt.Printf("    sha256_before_2  : %s\n", shaBefore2)
	fmt.Printf("    sha256_after_2   : %s (== before)\n", shaAfter2)
	fmt.Printf("    atomic_error     : %s\n", truncate(res.AtomicError, 160))
	fmt.Printf("    verdict          : block1 applied in memory but file 1 NOT written; rollback proven\n")
	return nil
}

// phaseE asserts diff exactness: the unified-diff text reported by the
// SmartEditTool MUST contain the literal `+`/`-` lines describing the change.
// We compute an independent expectation (`-old-line` and `+new-line`) and
// require result.Diff to contain both substrings. A regression that produced
// an empty diff or a diff for the wrong content would lose this assertion.
func phaseE() error {
	fmt.Println("==> phase E: DIFF EXACTNESS (always runs)")

	dir, err := newTempdir("p1f17-phase-e-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tool, err := newToolForDir(dir)
	if err != nil {
		return err
	}

	const before = "hello\nold-line\nworld\n"
	path := filepath.Join(dir, "e.txt")
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		return err
	}
	shaBefore := sha256Hex([]byte(before))

	prompt := buildPrompt(path, "old-line", "new-line")
	res, err := tool.Commit(context.Background(), prompt, dir)
	if err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if !res.Atomic {
		return fmt.Errorf("Atomic=false unexpected: %q", res.AtomicError)
	}

	disk, _ := os.ReadFile(path)
	shaAfter := sha256Hex(disk)

	if !strings.Contains(res.Diff, "-old-line") {
		return fmt.Errorf("result.Diff missing `-old-line` line:\n%s", res.Diff)
	}
	if !strings.Contains(res.Diff, "+new-line") {
		return fmt.Errorf("result.Diff missing `+new-line` line:\n%s", res.Diff)
	}

	snippet := firstNonHeaderLines(res.Diff, 6)
	fmt.Printf("    file             : %s\n", path)
	fmt.Printf("    sha256_before    : %s\n", shaBefore)
	fmt.Printf("    sha256_after     : %s\n", shaAfter)
	fmt.Printf("    diff_excerpt     :\n%s\n", indent(snippet, "        "))
	fmt.Printf("    verdict          : result.Diff contains the exact +/- lines for the change\n")
	return nil
}

// phaseF exercises the AMBIG path: SEARCH text appears twice in the file so
// the applier MUST refuse the block (lenient re-search expects exactly one
// match). Whole-prompt gate aborts. Evidence: file unchanged on disk.
func phaseF() error {
	fmt.Println("==> phase F: AMBIG (always runs)")

	dir, err := newTempdir("p1f17-phase-f-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tool, err := newToolForDir(dir)
	if err != nil {
		return err
	}

	const before = "duplicate\nfiller\nduplicate\n"
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		return err
	}
	shaBefore := sha256Hex([]byte(before))

	prompt := buildPrompt(path, "duplicate", "unique")
	res, err := tool.Commit(context.Background(), prompt, dir)
	if err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if res.Atomic {
		return fmt.Errorf("Atomic=true unexpected; ambiguous SEARCH must abort")
	}

	disk, _ := os.ReadFile(path)
	shaAfter := sha256Hex(disk)
	if shaAfter != shaBefore {
		return fmt.Errorf("file changed on AMBIG: before=%s after=%s", shaBefore, shaAfter)
	}

	fmt.Printf("    file             : %s\n", path)
	fmt.Printf("    sha256_before    : %s\n", shaBefore)
	fmt.Printf("    sha256_after     : %s (== before)\n", shaAfter)
	fmt.Printf("    atomic_error     : %s\n", truncate(res.AtomicError, 160))
	fmt.Printf("    verdict          : ambiguous SEARCH refused; disk untouched\n")
	return nil
}

// phaseG exercises the BINARY path: file contains a NUL byte so binary
// detection refuses the block before applier runs. Evidence: file unchanged
// on disk.
func phaseG() error {
	fmt.Println("==> phase G: BINARY (always runs)")

	dir, err := newTempdir("p1f17-phase-g-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tool, err := newToolForDir(dir)
	if err != nil {
		return err
	}

	beforeBytes := []byte("\x00abc\n")
	path := filepath.Join(dir, "g.bin")
	if err := os.WriteFile(path, beforeBytes, 0o644); err != nil {
		return err
	}
	shaBefore := sha256Hex(beforeBytes)

	prompt := buildPrompt(path, "abc", "xyz")
	res, err := tool.Commit(context.Background(), prompt, dir)
	if err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	if res.Atomic {
		return fmt.Errorf("Atomic=true unexpected; binary file must be refused")
	}

	disk, _ := os.ReadFile(path)
	shaAfter := sha256Hex(disk)
	if shaAfter != shaBefore {
		return fmt.Errorf("binary file modified: before=%s after=%s", shaBefore, shaAfter)
	}

	fmt.Printf("    file             : %s\n", path)
	fmt.Printf("    sha256_before    : %s\n", shaBefore)
	fmt.Printf("    sha256_after     : %s (== before)\n", shaAfter)
	fmt.Printf("    atomic_error     : %s\n", truncate(res.AtomicError, 160))
	fmt.Printf("    verdict          : binary file refused; disk untouched\n")
	return nil
}

// ---- helpers ----

// newTempdir creates a fresh temp directory under os.TempDir(). The caller
// is responsible for RemoveAll on completion.
func newTempdir(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
}

// newToolForDir constructs a real *multiedit.MultiFileEditor rooted at dir
// (so the multiedit workspace + backup dir live inside the temp dir) and
// wraps it in NewMultieditCommitter for use by SmartEditTool. This matches
// the realCommitter helper used by the smartedit package's own integration
// tests (T08 precedent).
func newToolForDir(dir string) (*smartedit.SmartEditTool, error) {
	cfg := multiedit.DefaultConfig()
	cfg.WorkspaceRoot = dir
	cfg.BackupDir = filepath.Join(dir, ".helix-backups")
	cfg.BackupEnabled = true
	cfg.AllowedPaths = nil
	cfg.DeniedPaths = nil
	cfg.RequirePreview = false

	mfe, err := multiedit.NewMultiFileEditor(multiedit.WithConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("NewMultiFileEditor: %w", err)
	}
	return smartedit.NewSmartEditTool(smartedit.NewMultieditCommitter(mfe), dir), nil
}

// buildPrompt assembles a single SEARCH/REPLACE block in the on-disk format
// the parser (T03) expects: a path line, the SEARCH marker, the search body
// terminated by a newline, the divider, the replacement body terminated by a
// newline, then the REPLACE marker. Multiple calls to this can be
// concatenated to form a multi-block prompt (path stickiness still applies
// because each call writes its own path line).
func buildPrompt(path, search, replace string) string {
	var b strings.Builder
	b.WriteString(path)
	b.WriteString("\n")
	b.WriteString(smartedit.MarkerSearch)
	b.WriteString("\n")
	b.WriteString(search)
	if !strings.HasSuffix(search, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(smartedit.MarkerDivider)
	b.WriteString("\n")
	b.WriteString(replace)
	if !strings.HasSuffix(replace, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(smartedit.MarkerReplace)
	b.WriteString("\n")
	return b.String()
}

// sha256Hex returns the hex-encoded sha-256 of b. This is the load-bearing
// positive-evidence primitive: every phase computes sha256 before/after and
// asserts the relation appropriate for its claim.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// truncate clamps s to maxLen, appending an ellipsis marker when truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

// firstNonHeaderLines returns up to n lines of the diff, skipping the
// `--- a/` and `+++ b/` header lines so the on-screen excerpt focuses on
// the hunks themselves.
func firstNonHeaderLines(diff string, n int) string {
	out := make([]string, 0, n)
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}
		out = append(out, line)
		if len(out) >= n {
			break
		}
	}
	return strings.Join(out, "\n")
}

// indent prefixes every line of s with prefix. Used for visual nesting of
// diff excerpts inside the per-phase status lines.
func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
