// completions_guard_test.go — §11.4.135 standing regression guard for the
// HXC-continua CompletionEngine.Complete out-of-range line/column panic.
//
// Historical defect (reproduced before the fix): Complete indexed
// lines[line-1] and sliced lineText[:col-1] with no lower-bound guard.
//   line=0 -> lines[-1]      -> "index out of range [-1]"      (panic)
//   col=0  -> lineText[:-1]  -> "slice bounds out of range [:-1]" (panic)
// Both line/col flow from tool input, so a 0/negative value crashed the
// request goroutine.
//
// §11.4.115 RED_MODE polarity:
//   RED_MODE=1 — reproduce the panic on a faithful pre-fix stand-in
//     (completeUnguarded) and assert it panics, PROVING the guard catches the
//     real bug. Run with:
//       RED_MODE=1 go test -count=1 -run TestCompletionEngine_OutOfRangeColumn_NoPanic ./internal/continua/
//   RED_MODE unset / 0 (DEFAULT) — drive the REAL fixed CompletionEngine and
//     assert NO panic + a valid result for line=0, col=0, and negative inputs.
//
// Mocks ALLOWED here per CONST-050(A) — unit (*_test.go) file. The RED
// stand-in faithfully reproduces the pre-fix arithmetic; default mode drives
// the REAL Complete.
package continua

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// completeUnguarded is a faithful stand-in for the PRE-FIX Complete body —
// the exact unguarded indexing/slicing that panicked. Used only under
// RED_MODE=1 to prove the panic is real.
func completeUnguarded(src string, line, col int) (prefix string) {
	lines := strings.Split(src, "\n")
	if line-1 < len(lines) { // pre-fix guard: upper bound only
		if col-1 <= len(lines[line-1]) {
			prefix = lines[line-1][:col-1] // panics for col<=0 or line<=0
		}
	}
	return prefix
}

func writeProbeFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// outOfRangeInputs are the (line, col) pairs that crashed the pre-fix code.
func outOfRangeInputs() [][2]int {
	return [][2]int{
		{0, 1},   // line=0  -> index out of range [-1]
		{1, 0},   // col=0   -> slice bounds [:-1]
		{0, 0},   // both zero
		{-3, -7}, // negatives
	}
}

func TestCompletionEngine_OutOfRangeColumn_NoPanic(t *testing.T) {
	const body = "package p\nfunc main(){}\n"

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the historical panic on the faithful pre-fix stand-in.
		for _, in := range outOfRangeInputs() {
			line, col := in[0], in[1]
			func() {
				defer func() {
					if r := recover(); r == nil {
						t.Fatalf("RED stand-in did NOT panic for line=%d col=%d "+
							"(expected the historical out-of-range panic)", line, col)
					}
				}()
				_ = completeUnguarded(body, line, col)
			}()
		}
		return
	}

	// DEFAULT: drive the REAL fixed CompletionEngine — must NOT panic and
	// must return a valid result for every out-of-range input.
	path := writeProbeFile(t, body)
	e := NewCompletionEngine()
	for _, in := range outOfRangeInputs() {
		line, col := in[0], in[1]
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Complete panicked for line=%d col=%d: %v", line, col, r)
				}
			}()
			res, err := e.Complete(context.Background(), path, line, col)
			if err != nil {
				t.Fatalf("Complete(line=%d,col=%d) error: %v", line, col, err)
			}
			if res == nil {
				t.Fatalf("Complete(line=%d,col=%d) returned nil result", line, col)
			}
			// Echoes the requested coordinates back (existing contract).
			if res.Line != line || res.Column != col {
				t.Fatalf("Complete(line=%d,col=%d) returned Line=%d Column=%d",
					line, col, res.Line, res.Column)
			}
		}()
	}

	// Sanity: a valid in-range call still produces a non-empty suggestion,
	// proving the guard did not break the happy path.
	res, err := e.Complete(context.Background(), path, 2, 5)
	if err != nil {
		t.Fatalf("in-range Complete error: %v", err)
	}
	if res.Suggestion == "" {
		t.Fatalf("in-range Complete returned empty suggestion: %s", fmt.Sprintf("%+v", res))
	}
}
