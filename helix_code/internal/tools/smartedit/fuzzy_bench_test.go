package smartedit

import (
	"fmt"
	"strings"
	"testing"
)

// representativeFile builds a synthetic source file of n functions — a
// stand-in for a real edit target. The benchmarks below measure how many
// output bytes/tokens the LLM must emit to make a one-line change with the
// whole-file (full-rewrite) format versus the diff (SEARCH/REPLACE) format.
func representativeFile(n int) string {
	var b strings.Builder
	b.WriteString("package widget\n\nimport \"fmt\"\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func process%d(x int) int {\n\ty := x * %d\n\treturn y + %d\n}\n\n", i, i+1, i)
	}
	return b.String()
}

// approxTokens is a deliberately simple, deterministic token estimate:
// whitespace-delimited words plus a per-4-chars factor — enough to compare
// two emission strategies without depending on a real tokenizer. The RATIO
// between the two strategies is what the speed claim rests on, and the ratio
// is stable under any monotonic estimator.
func approxTokens(s string) int {
	if s == "" {
		return 0
	}
	words := len(strings.Fields(s))
	// ~1 token per 4 chars is the common GPT/Claude rule-of-thumb floor.
	chars := len(s) / 4
	if words > chars {
		return words
	}
	return chars
}

// BenchmarkOutputTokens_FullRewriteVsDiff is the anti-bluff evidence for
// P3-T02: it reports, for a one-line change to a representative file, the
// output-token count the model must emit under each strategy and the
// resulting reduction. Run with:
//
//	go test -bench BenchmarkOutputTokens -benchmem ./internal/tools/smartedit/
//
// The b.ReportMetric lines surface full-rewrite tokens, diff tokens, and the
// percentage reduction directly in the benchmark output.
func BenchmarkOutputTokens_FullRewriteVsDiff(b *testing.B) {
	for _, n := range []int{20, 60, 120} {
		b.Run(fmt.Sprintf("funcs=%d", n), func(b *testing.B) {
			file := representativeFile(n)

			// Full-rewrite strategy: the model re-emits the ENTIRE file.
			fullRewriteOutput := file

			// Diff strategy: the model emits ONLY the changed line as a
			// SEARCH/REPLACE block — the changed-lines-only form of P3-T02.
			diffOutput := "widget.go\n" +
				MarkerSearch + "\n" +
				"\ty := x * 1\n" +
				MarkerDivider + "\n" +
				"\ty := x * 2\n" +
				MarkerReplace + "\n"

			fullTok := approxTokens(fullRewriteOutput)
			diffTok := approxTokens(diffOutput)
			reduction := 100.0 * float64(fullTok-diffTok) / float64(fullTok)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Exercise the real apply path so the benchmark also proves
				// the diff actually lands (not just that it is shorter).
				plan, err := Parse(diffOutput)
				if err != nil {
					b.Fatalf("diff failed to parse: %v", err)
				}
				_, _, ok := ApplyPlanToContentFuzzy([]byte(file), plan.Blocks)
				if !ok {
					b.Fatalf("representative diff did not apply")
				}
			}
			b.StopTimer()

			b.ReportMetric(float64(fullTok), "fullrewrite-tok")
			b.ReportMetric(float64(diffTok), "diff-tok")
			b.ReportMetric(reduction, "tok-reduction-%")
		})
	}
}

// BenchmarkApplyPlan_FuzzyVsStrict measures the per-apply cost overhead of the
// whitespace-tolerant applier versus the strict one on an exact-match block —
// proving fuzzy's tier-1 fast path is not a meaningful regression.
func BenchmarkApplyPlan_FuzzyVsStrict(b *testing.B) {
	file := []byte(representativeFile(60))
	blocks := []EditBlock{{
		Path:    "widget.go",
		Search:  "\ty := x * 1\n",
		Replace: "\ty := x * 2\n",
	}}

	b.Run("strict", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ApplyPlanToContent(file, blocks)
		}
	})
	b.Run("fuzzy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ApplyPlanToContentFuzzy(file, blocks)
		}
	})
}
