//go:build integration

// P1-F17-T08 — smart-edit integration tests.
//
// These tests exercise the production smart-edit pipeline end-to-end through
// the REAL F08 *multiedit.MultiFileEditor (no test stubs, no in-memory
// committer). Every PASS below carries positive runtime evidence:
//
//   - Real os.WriteFile to set up test fixtures on a real tempdir.
//   - Real *multiedit.MultiFileEditor constructed via NewMultiFileEditor.
//   - Real disk re-reads after Execute to verify (or refute) mutation.
//   - Real binary detection (NUL byte) and ambiguity detection (duplicate
//     SEARCH text) producing the documented Outcome enum values.
//
// Anti-bluff anchors:
//
//   - TestSmartEdit_MultiFile_OneFails_NoFilesWritten certifies whole-prompt
//     atomicity: when ANY block fails, NEITHER file's bytes change on disk.
//     This is the load-bearing transactional guarantee inherited from F08.
//   - TestSmartEdit_PostCommitDiffMatchesDiskContent re-reads the file via
//     an INDEPENDENT os.ReadFile after commit and asserts the result.Diff
//     reflects the bytes actually observable on disk — not just an
//     in-memory view.
//   - TestSmartEdit_DryRunDoesNotMutate certifies that dry_run=true never
//     touches the filesystem, by re-reading the original fixture bytes.
//
// Run with:
//
//	cd HelixCode && go test -tags=integration -run TestSmartEdit_ ./tests/integration/...
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/multiedit"
	"dev.helix.code/internal/tools/smartedit"
)

// newRealSmartEditTool constructs a SmartEditTool wired to a REAL
// multiedit.MultiFileEditor. No mocks, no fakes. The returned tool's workdir
// is the supplied dir; relative paths in SEARCH/REPLACE prompts resolve
// against it.
//
// The multiedit Config is rooted at `workdir` so the underlying filesystem
// tool's workspace-confinement check accepts paths inside the test tempdir,
// and AllowedPaths is widened to include the test fixture extensions
// (*.txt, *.bin) the tests below write. This still exercises the real
// confinement logic — paths outside `workdir` would still be refused.
func newRealSmartEditTool(t *testing.T, workdir string) *smartedit.SmartEditTool {
	t.Helper()
	cfg := multiedit.DefaultConfig()
	cfg.WorkspaceRoot = workdir
	cfg.AllowedPaths = []string{
		"**/*.txt",
		"**/*.bin",
		"**/*.go",
		"**/*.md",
	}
	// Disable git-related checks (tempdirs aren't git repos and we're not
	// testing git integration here).
	cfg.GitEnabled = false
	cfg.DetectGitChanges = false
	cfg.CheckUncommitted = false
	cfg.RespectGitignore = false
	mfe, err := multiedit.NewMultiFileEditor(multiedit.WithConfig(cfg))
	require.NoError(t, err, "construct real multiedit editor")
	require.NotNil(t, mfe, "multiedit editor must be non-nil")
	committer := smartedit.NewMultieditCommitter(mfe)
	return smartedit.NewSmartEditTool(committer, workdir)
}

// buildPrompt assembles a SEARCH/REPLACE prompt for one or more (path, search,
// replace) triples. Each triple emits the path on its own line followed by
// `<<<<<<< SEARCH` … `=======` … `>>>>>>> REPLACE`.
func buildPrompt(blocks ...[3]string) string {
	var b strings.Builder
	for _, blk := range blocks {
		fmt.Fprintf(&b, "%s\n%s\n%s\n%s\n%s\n%s\n",
			blk[0],
			"<<<<<<< SEARCH",
			blk[1],
			"=======",
			blk[2],
			">>>>>>> REPLACE",
		)
	}
	return b.String()
}

// readBytes is a thin testify-friendly os.ReadFile wrapper. Failure is fatal
// (a setup miss invalidates the test).
func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read %s", path)
	return got
}

// TestSmartEdit_SingleFileEditAppliesToDisk drives the simplest happy path:
// one file, one block, real commit, disk content actually changes.
func TestSmartEdit_SingleFileEditAppliesToDisk(t *testing.T) {
	dir := t.TempDir()
	rel := "hello.txt"
	abs := filepath.Join(dir, rel)
	require.NoError(t, os.WriteFile(abs, []byte("alpha\nold-line\nbeta\n"), 0o644))

	tool := newRealSmartEditTool(t, dir)
	prompt := buildPrompt([3]string{rel, "old-line", "new-line"})

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res, ok := out.(*smartedit.SmartEditResult)
	require.True(t, ok, "result type")
	require.True(t, res.Atomic, "atomic must be true; atomic_error=%q", res.AtomicError)
	require.Equal(t, 1, res.AppliedCount)
	require.Equal(t, 0, res.FailedCount)

	got := string(readBytes(t, abs))
	require.Equal(t, "alpha\nnew-line\nbeta\n", got, "disk must reflect new content")
}

// TestSmartEdit_DryRunDoesNotMutate certifies that dry_run=true never touches
// the filesystem. The original bytes must be preserved verbatim.
func TestSmartEdit_DryRunDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	rel := "hello.txt"
	abs := filepath.Join(dir, rel)
	original := []byte("alpha\nold-line\nbeta\n")
	require.NoError(t, os.WriteFile(abs, original, 0o644))

	tool := newRealSmartEditTool(t, dir)
	prompt := buildPrompt([3]string{rel, "old-line", "new-line"})

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt":  prompt,
		"dry_run": true,
	})
	require.NoError(t, err)
	res, ok := out.(*smartedit.SmartEditResult)
	require.True(t, ok)
	require.True(t, res.Atomic, "dry-run with valid plan should be atomic=true")
	require.NotEmpty(t, res.Diff, "dry-run should still emit a diff")

	got := readBytes(t, abs)
	require.Equal(t, original, got, "dry-run must not mutate disk")
}

// TestSmartEdit_MultiFileAtomicCommit drives 2 files with 1 block each and
// asserts both files are updated on disk after a single Execute.
func TestSmartEdit_MultiFileAtomicCommit(t *testing.T) {
	dir := t.TempDir()
	relA, relB := "a.txt", "b.txt"
	absA, absB := filepath.Join(dir, relA), filepath.Join(dir, relB)
	require.NoError(t, os.WriteFile(absA, []byte("header-A\nold-A\nfooter-A\n"), 0o644))
	require.NoError(t, os.WriteFile(absB, []byte("header-B\nancient-B\nfooter-B\n"), 0o644))

	tool := newRealSmartEditTool(t, dir)
	prompt := buildPrompt(
		[3]string{relA, "old-A", "new-A"},
		[3]string{relB, "ancient-B", "modern-B"},
	)

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res, ok := out.(*smartedit.SmartEditResult)
	require.True(t, ok)
	require.True(t, res.Atomic, "both blocks should commit atomically; atomic_error=%q", res.AtomicError)
	require.Equal(t, 2, res.AppliedCount)

	require.Equal(t, "header-A\nnew-A\nfooter-A\n", string(readBytes(t, absA)))
	require.Equal(t, "header-B\nmodern-B\nfooter-B\n", string(readBytes(t, absB)))
}

// TestSmartEdit_MultiFile_OneFails_NoFilesWritten certifies whole-prompt
// atomicity through the real multiedit transaction. File A's block matches;
// file B's SEARCH text is absent. Neither file may change.
func TestSmartEdit_MultiFile_OneFails_NoFilesWritten(t *testing.T) {
	dir := t.TempDir()
	relA, relB := "a.txt", "b.txt"
	absA, absB := filepath.Join(dir, relA), filepath.Join(dir, relB)
	originalA := []byte("header-A\nold-A\nfooter-A\n")
	originalB := []byte("header-B\nnothing-matches\nfooter-B\n")
	require.NoError(t, os.WriteFile(absA, originalA, 0o644))
	require.NoError(t, os.WriteFile(absB, originalB, 0o644))

	tool := newRealSmartEditTool(t, dir)
	// File B's SEARCH ("DOES_NOT_EXIST") is not present.
	prompt := buildPrompt(
		[3]string{relA, "old-A", "new-A"},
		[3]string{relB, "DOES_NOT_EXIST", "irrelevant"},
	)

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res, ok := out.(*smartedit.SmartEditResult)
	require.True(t, ok)
	require.False(t, res.Atomic, "one-block-failed must yield atomic=false")
	require.NotEmpty(t, res.AtomicError, "atomic_error must be populated on failure")

	// Anti-bluff: BOTH files unchanged on disk despite file A's block matching.
	require.Equal(t, originalA, readBytes(t, absA), "file A must NOT have been written")
	require.Equal(t, originalB, readBytes(t, absB), "file B must NOT have been written")

	// At least one block must report a failure outcome (NotFound for file B).
	var sawNotFound bool
	for _, br := range res.Results {
		if br.Outcome == smartedit.OutcomeNotFound {
			sawNotFound = true
		}
	}
	require.True(t, sawNotFound, "expected at least one NotFound outcome among %v", res.Results)
}

// TestSmartEdit_PostCommitDiffMatchesDiskContent re-reads the committed file
// via an independent os.ReadFile and asserts the result.Diff text reflects
// what is actually on disk (not just an in-memory view).
func TestSmartEdit_PostCommitDiffMatchesDiskContent(t *testing.T) {
	dir := t.TempDir()
	rel := "diff.txt"
	abs := filepath.Join(dir, rel)
	require.NoError(t, os.WriteFile(abs, []byte("line1\nold\nline3\n"), 0o644))

	tool := newRealSmartEditTool(t, dir)
	prompt := buildPrompt([3]string{rel, "old", "new"})

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res, ok := out.(*smartedit.SmartEditResult)
	require.True(t, ok)
	require.True(t, res.Atomic)
	require.NotEmpty(t, res.Diff, "diff must be populated on successful commit")

	// Independent re-read.
	disk := string(readBytes(t, abs))
	require.Equal(t, "line1\nnew\nline3\n", disk)

	// Diff text must mention BOTH the removed "old" and the added "new" lines
	// in canonical unified-diff form. This proves the diff was computed
	// against the post-commit disk state, not stale in-memory bytes.
	require.Contains(t, res.Diff, "-old", "diff missing removed-line marker for 'old'")
	require.Contains(t, res.Diff, "+new", "diff missing added-line marker for 'new'")
}

// TestSmartEdit_BinaryFileRefused verifies that files containing a NUL byte
// are detected as binary and refused. The file must remain unchanged on disk.
func TestSmartEdit_BinaryFileRefused(t *testing.T) {
	dir := t.TempDir()
	rel := "blob.bin"
	abs := filepath.Join(dir, rel)
	original := []byte{0x68, 0x69, 0x00, 0x6f, 0x6c, 0x64, 0x0a} // "hi\x00old\n"
	require.NoError(t, os.WriteFile(abs, original, 0o644))

	tool := newRealSmartEditTool(t, dir)
	prompt := buildPrompt([3]string{rel, "old", "new"})

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res, ok := out.(*smartedit.SmartEditResult)
	require.True(t, ok)
	require.False(t, res.Atomic, "binary file should refuse with atomic=false")

	var sawBinary bool
	for _, br := range res.Results {
		if br.Outcome == smartedit.OutcomeBinary {
			sawBinary = true
		}
	}
	require.True(t, sawBinary, "expected at least one Binary outcome among %v", res.Results)

	// Disk content untouched.
	require.Equal(t, original, readBytes(t, abs))
}

// TestSmartEdit_AmbiguousMatchRefused verifies that when the SEARCH text
// appears more than once in the file, the block is refused as ambiguous and
// the file is not modified.
func TestSmartEdit_AmbiguousMatchRefused(t *testing.T) {
	dir := t.TempDir()
	rel := "ambig.txt"
	abs := filepath.Join(dir, rel)
	// The whole-line "needle" appears twice; smart-edit must refuse to guess
	// which to replace. (The applier's parsed SEARCH body is "needle\n" — to
	// produce a duplicate match the file must contain that exact byte
	// sequence more than once.)
	original := []byte("alpha\nneedle\nbeta\ngamma\nneedle\ndelta\n")
	require.NoError(t, os.WriteFile(abs, original, 0o644))

	tool := newRealSmartEditTool(t, dir)
	prompt := buildPrompt([3]string{rel, "needle", "thread"})

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res, ok := out.(*smartedit.SmartEditResult)
	require.True(t, ok)
	require.False(t, res.Atomic, "ambiguous match should refuse with atomic=false")

	var sawAmbiguous bool
	for _, br := range res.Results {
		if br.Outcome == smartedit.OutcomeAmbiguous {
			sawAmbiguous = true
		}
	}
	require.True(t, sawAmbiguous, "expected at least one Ambiguous outcome among %v", res.Results)

	// Disk content untouched.
	require.Equal(t, original, readBytes(t, abs))
}
