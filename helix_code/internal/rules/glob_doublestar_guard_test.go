package rules

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Standing regression guard (§11.4.135) for the trailing/bare "**" glob
// fail-open coverage gap.
//
// Defect (reproduced 2026-06-15): matchGlobPath only translated the "**/"
// (double-star-slash) token. A bare or trailing "**" — e.g. a security rule
// scoped to "secrets/**" — fell through to the single-"*" → "[^/]*" rule,
// collapsing "secrets/**" to "^secrets/[^/]*$". That matched the top-level
// "secrets/aws.key" but SILENTLY EXCLUDED every nested file such as
// "secrets/prod/aws.key". A `category: security` rule therefore failed to
// apply to nested credentials — a fail-open rule-coverage gap.
//
// §11.4.115 polarity switch via the RED_MODE env var:
//   RED_MODE=1 : inline the PRE-FIX translation and assert the WRONG verdict
//                (nested file NOT matched) — reproduces the defect on the
//                broken logic and PASSES, proving the guard is real.
//   RED_MODE=0 : drive the REAL fixed matchGlobPath and assert the CORRECT
//                verdict (nested file IS matched) — the standing GREEN guard.

// brokenMatchGlobPath is the verbatim PRE-FIX translation, preserved here ONLY
// so the RED_MODE=1 reproduction can demonstrate the defect on the original
// logic. It is never used by production code.
func brokenMatchGlobPath(pattern, path string) bool {
	regexPattern := regexp.QuoteMeta(pattern)
	regexPattern = strings.ReplaceAll(regexPattern, `\*\*/`, `<<<DOUBLESTAR>>>`)
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, `[^/]*`)
	regexPattern = strings.ReplaceAll(regexPattern, `<<<DOUBLESTAR>>>`, `(?:.*/)?`)
	regexPattern = strings.ReplaceAll(regexPattern, `\?`, `.`)
	regexPattern = "^" + regexPattern + "$"
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

func TestGlobTrailingDoubleStar_FailOpenGuard(t *testing.T) {
	red := os.Getenv("RED_MODE") == "1"

	// Concrete counterexample: a security rule scoped to a directory tree must
	// cover files nested arbitrarily deep, not just the top level.
	const pattern = "secrets/**"
	const nestedSecret = "secrets/prod/aws.key" // arbitrarily deep
	const topLevelSecret = "secrets/aws.key"    // top level (matched even pre-fix)

	if red {
		// Reproduce the defect on the PRE-FIX logic: the nested secret is NOT
		// matched (fail-open). Assert the WRONG verdict — this PASSES on broken
		// logic, proving the guard genuinely reproduces the bug.
		if brokenMatchGlobPath(pattern, nestedSecret) {
			t.Fatalf("RED_MODE: pre-fix logic unexpectedly matched nested %q for %q; "+
				"the defect did not reproduce", nestedSecret, pattern)
		}
		if !brokenMatchGlobPath(pattern, topLevelSecret) {
			t.Fatalf("RED_MODE: pre-fix logic should still match top-level %q for %q",
				topLevelSecret, pattern)
		}
		t.Logf("RED_MODE: reproduced fail-open — pre-fix %q does NOT match nested %q",
			pattern, nestedSecret)
		return
	}

	// GREEN guard: the REAL fixed code MUST match nested files under a "**" tree.
	if !matchGlobPath(pattern, nestedSecret) {
		t.Errorf("fail-open regression: security rule %q does NOT match nested %q",
			pattern, nestedSecret)
	}
	if !matchGlobPath(pattern, topLevelSecret) {
		t.Errorf("rule %q does NOT match top-level %q", pattern, topLevelSecret)
	}

	// End-to-end through the public parser + Matches path (fully reachable).
	rs, err := ParseString("[secrets-rule]\npattern: secrets/**\ncategory: security\nNo credentials here")
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if got := rs.Rules[0].PatternType; got != PatternTypeGlob {
		t.Fatalf("expected glob pattern type for %q, got %q", pattern, got)
	}
	if !rs.Rules[0].Matches(nestedSecret) {
		t.Errorf("end-to-end fail-open: parsed security rule does NOT match nested %q", nestedSecret)
	}
}

// TestGlobDoubleStar_Semantics is the broader standing guard covering the full
// "**" / "*" semantic matrix the fix established. "**" crosses directory
// separators (any depth); single "*" never does.
func TestGlobDoubleStar_Semantics(t *testing.T) {
	cases := []struct {
		pat, path string
		want      bool
	}{
		{"secrets/**", "secrets/prod/aws.key", true},      // trailing ** crosses dirs
		{"secrets/**", "secrets/aws.key", true},           // trailing ** at top level
		{"docs/**", "docs/a/b/c.md", true},                // arbitrarily deep
		{"src/**/*.go", "src/internal/auth/jwt.go", true}, // mid ** (regression)
		{"src/**/*.go", "src/c.go", true},                 // mid ** zero dirs (regression)
		{"**/*_test.go", "internal/rules/rule_test.go", true},
		{"**/test/**/*.go", "a/test/b/c.go", true}, // ** on both sides
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", false},  // single * must NOT cross /
		{"src/*.go", "src/a/b.go", false}, // single * must NOT cross /
		{"a/?/b", "a/x/b", true},          // ? matches exactly one non-sep char
		{"a/?/b", "a/ab/b", false},        // ? matches exactly ONE char, not two
		{"sec?ets/key", "sec/ets/key", false}, // ? must NOT cross / (fail-open guard)
	}
	for _, c := range cases {
		if got := matchGlobPath(c.pat, c.path); got != c.want {
			t.Errorf("matchGlobPath(%q,%q)=%v want=%v", c.pat, c.path, got, c.want)
		}
	}
}
