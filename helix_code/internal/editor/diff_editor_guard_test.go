package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiffEditor_TrailingNewlineDiffApplies is the standing §11.4.135 regression
// guard for the trailing-newline-diff-silently-rejected defect.
//
// DEFECT (pre-fix, diff_editor.go parseDiff): a unified diff string that ends in
// the conventional trailing newline (the normal case for every real diff) was
// split by strings.Split into lines whose final element is a spurious empty
// string "". parseDiff treated that zero-length element as an empty *context*
// line and appended a phantom row to the hunk. applyHunks then ran out of
// original-file lines on that phantom context row and returned
// "hunk context mismatch: unexpected end of file" — so a perfectly valid edit
// was rejected and the target file left UNCHANGED. This is a data-correctness /
// §11.4 PASS-bluff surface: the editor reports an error and silently drops a
// valid edit.
//
// Concrete byte-level counterexample:
//
//	file before : "a\nb\nc\n"
//	diff        : "@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"   (note trailing "\n")
//	want after  : "a\nB\nc\n"
//	pre-fix got : "a\nb\nc\n"  + error "hunk context mismatch: unexpected end of file"
//
// §11.4.115 polarity switch:
//
//	RED_MODE=1 : inline the pre-fix parseDiff (phantom empty-context row) and
//	             ASSERT the corrupted/wrong outcome (file unchanged + mismatch
//	             error) is REPRODUCED — proves the guard catches a real defect.
//	RED_MODE=0 : (default, no env) drive the REAL fixed DiffEditor and ASSERT the
//	             correct outcome ("a\nB\nc\n", no error).
func TestDiffEditor_TrailingNewlineDiffApplies(t *testing.T) {
	const orig = "a\nb\nc\n"
	const diff = "@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n" // trailing newline = the normal case
	const want = "a\nB\nc\n"

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the pre-fix behaviour with a local copy of the BROKEN
		// parseDiff (zero-length line -> phantom empty context row), then run
		// the unchanged applyHunks and assert the defect is observed.
		hunks := parseDiffBROKEN(diff)
		de := NewDiffEditor()
		_, err := de.applyHunks([]string{"a", "b", "c"}, hunks)
		if err == nil {
			t.Fatalf("RED_MODE: expected pre-fix defect (hunk context mismatch) but applyHunks succeeded — defect not reproduced")
		}
		if !strings.Contains(err.Error(), "context mismatch") {
			t.Fatalf("RED_MODE: expected a context-mismatch error reproducing the defect, got %v", err)
		}
		t.Logf("RED_MODE: pre-fix defect reproduced: %v (valid edit would have been silently dropped)", err)
		return
	}

	// GREEN guard (default): the real fixed code applies the trailing-newline diff.
	dir := t.TempDir()
	fp := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(fp, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	de := NewDiffEditor()
	if err := de.Apply(Edit{FilePath: fp, Format: EditFormatDiff, Content: diff}); err != nil {
		t.Fatalf("fixed DiffEditor rejected a valid trailing-newline diff: %v", err)
	}
	got, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("trailing-newline diff applied wrong:\n got=%q\nwant=%q", string(got), want)
	}
}

// TestDiffEditor_MultiHunkTrailingNewline guards the multi-hunk variant: two
// independent hunks in one trailing-newline-terminated diff must both apply,
// preserving intervening unmodified lines.
func TestDiffEditor_MultiHunkTrailingNewline(t *testing.T) {
	const orig = "l1\nl2\nl3\nl4\nl5\nl6\n"
	const diff = "@@ -1,1 +1,1 @@\n-l1\n+X1\n@@ -4,1 +4,1 @@\n-l4\n+X4\n"
	const want = "X1\nl2\nl3\nX4\nl5\nl6\n"

	if os.Getenv("RED_MODE") == "1" {
		hunks := parseDiffBROKEN(diff)
		de := NewDiffEditor()
		_, err := de.applyHunks([]string{"l1", "l2", "l3", "l4", "l5", "l6"}, hunks)
		if err == nil {
			t.Fatalf("RED_MODE: expected pre-fix multi-hunk defect but applyHunks succeeded — defect not reproduced")
		}
		t.Logf("RED_MODE: pre-fix multi-hunk defect reproduced: %v", err)
		return
	}

	dir := t.TempDir()
	fp := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(fp, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	de := NewDiffEditor()
	if err := de.Apply(Edit{FilePath: fp, Format: EditFormatDiff, Content: diff}); err != nil {
		t.Fatalf("fixed DiffEditor rejected a valid multi-hunk trailing-newline diff: %v", err)
	}
	got, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("multi-hunk trailing-newline diff applied wrong:\n got=%q\nwant=%q", string(got), want)
	}
}

// TestDiffEditor_LegitimateEmptyContextLinePreserved guards against the fix
// over-correcting: a REAL empty context line (encoded in a unified diff as a
// single space " ") must still be honoured, not dropped.
func TestDiffEditor_LegitimateEmptyContextLinePreserved(t *testing.T) {
	const orig = "a\n\nc\n" // middle line is genuinely empty
	const diff = "@@ -1,3 +1,3 @@\n-a\n+A\n \n c\n"
	const want = "A\n\nc\n"

	dir := t.TempDir()
	fp := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(fp, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	de := NewDiffEditor()
	if err := de.Apply(Edit{FilePath: fp, Format: EditFormatDiff, Content: diff}); err != nil {
		t.Fatalf("fixed DiffEditor rejected a diff with a legitimate empty context line: %v", err)
	}
	got, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("legitimate empty context line not preserved:\n got=%q\nwant=%q", string(got), want)
	}
}

// parseDiffBROKEN is a verbatim copy of the pre-fix parseDiff body, preserved
// ONLY for the §11.4.115 RED_MODE reproduction above. It treats a zero-length
// line as an empty context line — the defect. It is a test-only fixture (unit
// test file), never reachable from production code.
func parseDiffBROKEN(diffContent string) []DiffHunk {
	de := NewDiffEditor()
	var hunks []DiffHunk
	lines := strings.Split(diffContent, "\n")

	var currentHunk *DiffHunk
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			hunk, err := de.parseHunkHeader(line)
			if err != nil {
				return hunks
			}
			currentHunk = &hunk
			continue
		}

		if currentHunk != nil {
			if len(line) == 0 {
				// THE DEFECT: phantom empty context row.
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: ' ', Content: ""})
			} else if line[0] == '+' || line[0] == '-' || line[0] == ' ' {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: line[0], Content: line[1:]})
			}
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}
	return hunks
}
