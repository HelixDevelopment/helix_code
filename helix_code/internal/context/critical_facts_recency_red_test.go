package context

// §11.4.115 RED-on-broken-artifact + polarity-switch regression guard for
// DEFECT C1: FilesTouched recency-cap regression.
//
// Root cause (pre-fix): ExtractCriticalFacts built FilesTouched as
//   capFacts(sortedUnique(files))
// sortedUnique sorts ALPHABETICALLY, then capFacts keeps the TAIL (last 24).
// On an alphabetically-sorted list the tail is the alpha-last 24 entries, NOT
// the most-recently-touched 24. So when >24 distinct files are touched, the
// most-recently-touched (active-work) file — if its name sorts early
// alphabetically — is silently DROPPED from the condensed summary, violating
// the package's documented recency contract ("the most recent matches win when
// the cap is hit") and the no-regression guarantee.
//
// Polarity switch RED_MODE:
//   - RED_MODE=1  → reproduce-and-assert-defect-PRESENT (run against the pre-fix
//                   artifact to prove the guard genuinely catches the bug).
//   - unset/other → standing GREEN regression guard asserting the defect is
//                   ABSENT on the fixed artifact (the default, so the full
//                   suite stays green per §11.4.135).

import (
	"os"
	"strings"
	"testing"
)

// buildRecencyTurns produces turns touching >maxFactsPerCategory distinct files
// where the MOST-RECENTLY-touched file has a name that sorts EARLY
// alphabetically (so an alpha-sort-then-tail-cap drops it). Returns the turns
// and the name of the most-recently-touched file.
func buildRecencyTurns() ([]HistoryTurn, string) {
	// 30 older files whose names all sort AFTER the active file (they start
	// with "z"), so an alpha sort places them all in the tail and the active
	// file in the head — exactly the slice the tail-cap discards.
	turns := make([]HistoryTurn, 0, maxFactsPerCategory+10)
	for i := 0; i < maxFactsPerCategory+6; i++ {
		// e.g. zfile_00.go .. zfile_29.go — all sort after "active_work.go".
		turns = append(turns, HistoryTurn{
			Role:    "tool",
			Content: "edited zfile_" + twoDigit(i) + ".go",
		})
	}
	// The active-work file is touched LAST (most recent) but sorts FIRST.
	const activeFile = "active_work.go"
	turns = append(turns, HistoryTurn{
		Role:    "assistant",
		Content: "now editing " + activeFile + " for the active task",
	})
	return turns, activeFile
}

func twoDigit(i int) string {
	const digits = "0123456789"
	return string([]byte{digits[(i/10)%10], digits[i%10]})
}

// TestCriticalFacts_FilesTouched_RecencyCap_C1 is the C1 regression guard.
func TestCriticalFacts_FilesTouched_RecencyCap_C1(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1" // default OFF → standing GREEN guard

	turns, activeFile := buildRecencyTurns()
	facts := ExtractCriticalFacts(turns)

	// Sanity: the cap is actually exceeded (otherwise the test proves nothing).
	if len(facts.FilesTouched) != maxFactsPerCategory {
		t.Fatalf("precondition: expected FilesTouched capped to %d, got %d",
			maxFactsPerCategory, len(facts.FilesTouched))
	}

	present := false
	for _, f := range facts.FilesTouched {
		if f == activeFile {
			present = true
			break
		}
	}

	// Also assert it survives into the rendered condensed summary, since that
	// is what the user actually sees.
	summary := renderStructuredSummary(turns, facts)
	inSummary := strings.Contains(summary, activeFile)

	if redMode {
		// On the BROKEN artifact the most-recently-touched file is dropped.
		if present || inSummary {
			t.Fatalf("RED_MODE: expected the broken artifact to DROP the most-recent file %q "+
				"(present=%v inSummary=%v) — defect not reproduced; the bug may already be fixed, "+
				"re-run with RED_MODE=0", activeFile, present, inSummary)
		}
		t.Logf("RED_MODE: defect reproduced — most-recent file %q dropped from cap (present=%v inSummary=%v)",
			activeFile, present, inSummary)
		return
	}

	// GREEN guard (RED_MODE=0): the most-recently-touched file MUST survive.
	if !present {
		t.Fatalf("GREEN: most-recently-touched file %q was dropped from FilesTouched %v",
			activeFile, facts.FilesTouched)
	}
	if !inSummary {
		t.Fatalf("GREEN: most-recently-touched file %q absent from rendered condensed summary:\n%s",
			activeFile, summary)
	}
}
