// history_guard_test.go — standing regression guard (§11.4.135) for the
// History.Average() aggregate-boolean defect found by the §11.4.118
// discovery sweep (2026-06-15).
//
// DEFECT: Average() summed Overall/LintScore/TestPassRate/Security but
// NEVER set the Compilation or Passed booleans, so the averaged
// ScoreResult always carried Compilation=false, Passed=false — even for
// a history where EVERY entry compiled and passed. A consumer asking
// "did the averaged history compile / pass?" got a provably-wrong false.
//
// §11.4.115 RED polarity:
//   RED_MODE=1 → reproduce the defect on a faithful inline pre-fix
//               stand-in (averagePreFix) and assert the WRONG values it
//               produced (test PASSES, proving the defect was real).
//   RED_MODE=0 (default) → drive the REAL fixed History.Average() and
//               assert the CORRECT aggregate booleans (the standing
//               GREEN guard asserting the defect is ABSENT).
package quality

import (
	"os"
	"testing"
)

// averagePreFix is a byte-faithful copy of the pre-fix History.Average()
// body (the version that never set Compilation/Passed). It exists ONLY
// so RED_MODE=1 can reproduce the historical wrong values on demand.
func averagePreFix(entries []ScoreResult) ScoreResult {
	if len(entries) == 0 {
		return ScoreResult{}
	}
	var sum ScoreResult
	var securitySum float64
	count := float64(len(entries))
	for _, e := range entries {
		sum.Overall += e.Overall
		sum.LintScore += e.LintScore
		sum.TestPassRate += e.TestPassRate
		securitySum += float64(e.Security)
	}
	return ScoreResult{
		Overall:      sum.Overall / count,
		LintScore:    sum.LintScore / count,
		TestPassRate: sum.TestPassRate / count,
		Security:     int(securitySum / count),
		// NOTE: Compilation + Passed intentionally left zero-valued —
		// this is the bug being reproduced.
	}
}

func TestHistoryAverage_AggregateBooleans_Guard(t *testing.T) {
	// History of two entries that BOTH compiled and BOTH passed.
	entries := []ScoreResult{
		{Overall: 100, Compilation: true, Passed: true},
		{Overall: 100, Compilation: true, Passed: true},
	}

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the defect on the faithful pre-fix stand-in.
		got := averagePreFix(entries)
		if got.Compilation != false || got.Passed != false {
			t.Fatalf("RED_MODE: pre-fix stand-in expected to reproduce the "+
				"defect (Compilation=false, Passed=false) for an all-passing "+
				"history, got Compilation=%v Passed=%v — stand-in no longer "+
				"faithful", got.Compilation, got.Passed)
		}
		// Overall correctly averages even in the buggy version.
		if got.Overall != 100 {
			t.Fatalf("RED_MODE: pre-fix Overall = %v, want 100", got.Overall)
		}
		return
	}

	// GREEN guard: drive the real fixed code.
	h := NewHistory("")
	for _, e := range entries {
		if err := h.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got := h.Average()
	if got.Overall != 100 {
		t.Fatalf("Average().Overall = %v, want 100", got.Overall)
	}
	if !got.Compilation {
		t.Fatalf("Average().Compilation = false for an all-compiled history; "+
			"want true (defect regressed)")
	}
	if !got.Passed {
		t.Fatalf("Average().Passed = false for an all-passing history; want "+
			"true (defect regressed)")
	}
}

// TestHistoryAverage_NotAllPassed_Guard proves the fix is faithful in the
// negative direction: if ANY entry failed/did-not-compile, the aggregate
// booleans MUST be false (the fix must not blindly hardcode true).
func TestHistoryAverage_NotAllPassed_Guard(t *testing.T) {
	h := NewHistory("")
	mustAppend(t, h, ScoreResult{Overall: 100, Compilation: true, Passed: true})
	mustAppend(t, h, ScoreResult{Overall: 0, Compilation: false, Passed: false})

	got := h.Average()
	if got.Compilation {
		t.Fatalf("Average().Compilation = true though one entry did not "+
			"compile; want false")
	}
	if got.Passed {
		t.Fatalf("Average().Passed = true though one entry did not pass; "+
			"want false")
	}
	if got.Overall != 50 {
		t.Fatalf("Average().Overall = %v, want 50", got.Overall)
	}
}

func mustAppend(t *testing.T, h *History, r ScoreResult) {
	t.Helper()
	if err := h.Append(r); err != nil {
		t.Fatalf("Append: %v", err)
	}
}
