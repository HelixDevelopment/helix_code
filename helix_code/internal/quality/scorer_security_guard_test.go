// scorer_security_guard_test.go — standing regression guard (§11.4.135)
// for the computeOverall() inverted-security-credit defect found by the
// §11.4.118 discovery sweep (2026-06-15).
//
// DEFECT: computeOverall awarded the +10 security credit with
// `if r.Security > 0`. Security is a COUNT of security findings (lower
// is better). The condition was inverted on two counts:
//   1. It rewarded results that RECORDED findings (Security>0) and
//      PENALISED clean results (Security==0) — a perverse incentive.
//   2. Both real scoring paths (Score / ScoreWithTools) hardcode
//      Security=0, so `Security > 0` was ALWAYS false → the +10 was dead
//      code and a clean build's documented 100-point ceiling was
//      structurally unreachable, capping at 90.
//
// §11.4.115 RED polarity:
//   RED_MODE=1 → reproduce the defect on a faithful inline pre-fix
//               stand-in (computeOverallPreFix) and assert the WRONG
//               numbers (clean build = 90; with-findings build = 100).
//   RED_MODE=0 (default) → drive the REAL fixed computeOverall via the
//               public Score() path and assert the CORRECT numbers
//               (clean build reaches 100; a result with findings does
//               NOT get the credit).
package quality

import (
	"os"
	"testing"
)

// computeOverallPreFix is a byte-faithful copy of the pre-fix
// computeOverall body (the `if r.Security > 0` version). It exists ONLY
// so RED_MODE=1 can reproduce the historical wrong scores on demand.
func computeOverallPreFix(r *ScoreResult) float64 {
	score := 0.0
	if r.Compilation {
		score += 40.0
	}
	score += r.TestPassRate * 30.0
	score += r.LintScore * 0.2
	if r.Security > 0 { // the inverted condition being reproduced
		score += 10.0
	}
	return score
}

func TestComputeOverall_SecurityCreditDirection_Guard(t *testing.T) {
	clean := &ScoreResult{Compilation: true, TestPassRate: 1.0, LintScore: 100.0, Security: 0}
	withFindings := &ScoreResult{Compilation: true, TestPassRate: 1.0, LintScore: 100.0, Security: 3}

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the defect on the faithful pre-fix stand-in.
		gotClean := computeOverallPreFix(clean)
		gotFindings := computeOverallPreFix(withFindings)
		if gotClean != 90.0 {
			t.Fatalf("RED_MODE: pre-fix clean-build score = %v, want 90 "+
				"(clean build wrongly capped at 90 by the inverted credit); "+
				"stand-in no longer faithful", gotClean)
		}
		if gotFindings != 100.0 {
			t.Fatalf("RED_MODE: pre-fix with-findings score = %v, want 100 "+
				"(findings wrongly rewarded with the +10 credit); stand-in "+
				"no longer faithful", gotFindings)
		}
		// The perverse incentive captured numerically: a build WITH
		// security findings scored HIGHER than an identical clean build.
		if !(gotFindings > gotClean) {
			t.Fatalf("RED_MODE: expected with-findings (%v) > clean (%v) "+
				"under the defect", gotFindings, gotClean)
		}
		return
	}

	// GREEN guard: drive the real fixed computeOverall.
	s := NewScorer()
	gotClean := s.computeOverall(clean)
	gotFindings := s.computeOverall(withFindings)

	if gotClean != 100.0 {
		t.Fatalf("computeOverall(clean) = %v, want 100 — clean build must "+
			"reach the documented ceiling (defect regressed)", gotClean)
	}
	if gotFindings != 90.0 {
		t.Fatalf("computeOverall(withFindings) = %v, want 90 — a result "+
			"with security findings must NOT receive the credit (defect "+
			"regressed)", gotFindings)
	}
	if !(gotClean > gotFindings) {
		t.Fatalf("clean (%v) must score strictly higher than with-findings "+
			"(%v) — security incentive still inverted", gotClean, gotFindings)
	}
}

// TestScore_CleanBuildReachesCeiling_Guard proves the fix end-to-end
// through the public Score() API: a clean, compiling program now reaches
// the 100-point ceiling (was capped at 90 under the defect) and is
// reported Passed.
func TestScore_CleanBuildReachesCeiling_Guard(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// Under the defect, the Score path's result (Security=0,
		// Compilation=true, TestPassRate=1, LintScore=100) scored 90 via
		// computeOverallPreFix. Reproduce that exact number.
		r := &ScoreResult{Compilation: true, TestPassRate: 1.0, LintScore: 100.0, Security: 0}
		if got := computeOverallPreFix(r); got != 90.0 {
			t.Fatalf("RED_MODE: pre-fix Score-path overall = %v, want 90", got)
		}
		return
	}

	s := NewScorer()
	const cleanProgram = "package main\n\nfunc main() {}\n"
	res, err := s.Score(t.Context(), cleanProgram, "")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if !res.Compilation {
		t.Skipf("toolchain could not compile the trivial program in this "+
			"environment (build_error=%q) — ceiling assertion not applicable",
			res.Details["build_error"])
	}
	if res.Overall != 100.0 {
		t.Fatalf("Score(clean).Overall = %v, want 100 (clean build must "+
			"reach the ceiling)", res.Overall)
	}
	if !res.Passed {
		t.Fatalf("Score(clean).Passed = false, want true at Overall=100")
	}
}
