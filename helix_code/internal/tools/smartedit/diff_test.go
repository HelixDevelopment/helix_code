package smartedit

import (
	"strings"
	"sync"
	"testing"
)

func TestNewDiffer_NotNil(t *testing.T) {
	d := NewDiffer()
	if d == nil {
		t.Fatal("NewDiffer() returned nil")
	}
}

func TestFileDiff_NoChange_EmptyOutput(t *testing.T) {
	d := NewDiffer()
	content := []byte("alpha\nbeta\ngamma\n")
	out, err := d.FileDiff("foo.txt", content, content)
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty diff for unchanged content, got %q", out)
	}
}

func TestFileDiff_AddLine_HasUnifiedFormat(t *testing.T) {
	d := NewDiffer()
	old := []byte("alpha\nbeta\n")
	nw := []byte("alpha\nbeta\ngamma\n")
	out, err := d.FileDiff("foo.txt", old, nw)
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty diff for added line")
	}
	if !strings.Contains(out, "+gamma") {
		t.Fatalf("expected '+gamma' marker in diff, got:\n%s", out)
	}
	if !strings.Contains(out, "@@") {
		t.Fatalf("expected hunk header '@@' in unified diff, got:\n%s", out)
	}
}

func TestFileDiff_RemoveLine_HasMinusMarker(t *testing.T) {
	d := NewDiffer()
	old := []byte("alpha\nbeta\ngamma\n")
	nw := []byte("alpha\ngamma\n")
	out, err := d.FileDiff("foo.txt", old, nw)
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty diff for removed line")
	}
	if !strings.Contains(out, "-beta") {
		t.Fatalf("expected '-beta' marker in diff, got:\n%s", out)
	}
}

func TestFileDiff_ModifyLine_HasBothMarkers(t *testing.T) {
	d := NewDiffer()
	old := []byte("alpha\nbeta\ngamma\n")
	nw := []byte("alpha\nBETA\ngamma\n")
	out, err := d.FileDiff("foo.txt", old, nw)
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}
	if !strings.Contains(out, "-beta") {
		t.Fatalf("expected '-beta' marker, got:\n%s", out)
	}
	if !strings.Contains(out, "+BETA") {
		t.Fatalf("expected '+BETA' marker, got:\n%s", out)
	}
}

func TestFileDiff_PathInHeader(t *testing.T) {
	d := NewDiffer()
	old := []byte("a\n")
	nw := []byte("b\n")
	const path = "pkg/sub/example.go"
	out, err := d.FileDiff(path, old, nw)
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}
	if !strings.Contains(out, path) {
		t.Fatalf("expected path %q in diff header, got:\n%s", path, out)
	}
	if !strings.Contains(out, "--- ") || !strings.Contains(out, "+++ ") {
		t.Fatalf("expected --- / +++ headers in unified diff, got:\n%s", out)
	}
}

func TestFileDiff_MultilineChanges(t *testing.T) {
	d := NewDiffer()
	old := []byte("one\ntwo\nthree\nfour\nfive\n")
	nw := []byte("one\nTWO\nthree\nFOUR\nfive\nsix\n")
	out, err := d.FileDiff("multi.txt", old, nw)
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}
	for _, want := range []string{"-two", "+TWO", "-four", "+FOUR", "+six"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in diff, got:\n%s", want, out)
		}
	}
}

func TestCombinedDiff_EmptyMap(t *testing.T) {
	d := NewDiffer()
	got := d.CombinedDiff(map[string]string{})
	if got != "" {
		t.Fatalf("expected empty CombinedDiff for empty map, got %q", got)
	}
}

func TestCombinedDiff_AllEmptyValues(t *testing.T) {
	d := NewDiffer()
	got := d.CombinedDiff(map[string]string{
		"a.txt": "",
		"b.txt": "",
	})
	if got != "" {
		t.Fatalf("expected empty CombinedDiff when all values empty, got %q", got)
	}
}

func TestCombinedDiff_TwoFiles(t *testing.T) {
	d := NewDiffer()
	d1, err := d.FileDiff("alpha.txt", []byte("a\n"), []byte("b\n"))
	if err != nil {
		t.Fatalf("FileDiff alpha: %v", err)
	}
	d2, err := d.FileDiff("beta.txt", []byte("x\n"), []byte("y\n"))
	if err != nil {
		t.Fatalf("FileDiff beta: %v", err)
	}
	combined := d.CombinedDiff(map[string]string{
		"alpha.txt": d1,
		"beta.txt":  d2,
	})
	if !strings.Contains(combined, "alpha.txt") {
		t.Errorf("expected alpha.txt in combined output, got:\n%s", combined)
	}
	if !strings.Contains(combined, "beta.txt") {
		t.Errorf("expected beta.txt in combined output, got:\n%s", combined)
	}
	if !strings.Contains(combined, "-a") || !strings.Contains(combined, "+b") {
		t.Errorf("expected alpha diff body in combined output, got:\n%s", combined)
	}
	if !strings.Contains(combined, "-x") || !strings.Contains(combined, "+y") {
		t.Errorf("expected beta diff body in combined output, got:\n%s", combined)
	}
}

func TestCombinedDiff_DeterministicOrder(t *testing.T) {
	d := NewDiffer()
	d1, err := d.FileDiff("alpha.txt", []byte("a\n"), []byte("b\n"))
	if err != nil {
		t.Fatalf("FileDiff alpha: %v", err)
	}
	d2, err := d.FileDiff("beta.txt", []byte("x\n"), []byte("y\n"))
	if err != nil {
		t.Fatalf("FileDiff beta: %v", err)
	}
	d3, err := d.FileDiff("gamma.txt", []byte("p\n"), []byte("q\n"))
	if err != nil {
		t.Fatalf("FileDiff gamma: %v", err)
	}
	input := map[string]string{
		"gamma.txt": d3,
		"alpha.txt": d1,
		"beta.txt":  d2,
	}
	first := d.CombinedDiff(input)
	for i := 0; i < 25; i++ {
		got := d.CombinedDiff(input)
		if got != first {
			t.Fatalf("CombinedDiff not deterministic across calls\nfirst:\n%s\ngot:\n%s", first, got)
		}
	}
	// Sanity: alpha appears before beta which appears before gamma in the
	// sorted-by-key output.
	idxA := strings.Index(first, "alpha.txt")
	idxB := strings.Index(first, "beta.txt")
	idxG := strings.Index(first, "gamma.txt")
	if !(idxA >= 0 && idxB > idxA && idxG > idxB) {
		t.Fatalf("expected sorted-key order alpha < beta < gamma, idxA=%d idxB=%d idxG=%d\n%s", idxA, idxB, idxG, first)
	}
}

func TestDiffer_ConcurrentSafe(t *testing.T) {
	d := NewDiffer()
	const goroutines = 10
	const iterations = 25
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines*iterations)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			old := []byte("alpha\nbeta\ngamma\n")
			nw := []byte("alpha\nBETA\ngamma\ndelta\n")
			for i := 0; i < iterations; i++ {
				out, err := d.FileDiff("concurrent.txt", old, nw)
				if err != nil {
					errCh <- err
					return
				}
				if out == "" {
					errCh <- err
					return
				}
				if !strings.Contains(out, "+BETA") || !strings.Contains(out, "+delta") {
					errCh <- err
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent FileDiff error: %v", err)
		}
	}
}
