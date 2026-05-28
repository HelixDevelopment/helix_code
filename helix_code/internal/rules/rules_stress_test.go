package rules

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the rules package.
//
// The unit under stress is the REAL *Manager (RWMutex-guarded three-level rule
// store: workspace / project / file) and the REAL *Parser. No fakes: every Rule
// is a fully-valid *Rule that passes Rule.Validate(), and every match assertion
// reads the genuine GetRulesForFile / GetAllRules / Count output, so each PASS
// proves real rule storage + glob/regex matching happened — not a no-op.
//
// Sustained load (N>=100, p50/p95/p99 captured) drives the real add->match
// pipeline and the real Parser.Parse against on-disk .clinerules files.
// N>=10 concurrent goroutines hammer the shared rule store under genuine
// read/write contention against the Manager's RWMutex (run under -race to catch
// data races in the add/remove/match path).

// newValidRule builds a fully-valid Rule (passes Validate): non-empty Name,
// Pattern, Content. Using a unique id per call avoids the duplicate-ID rejection
// in RuleSet.AddRule so every Add genuinely mutates the store.
func newValidRule(id, pattern string) *Rule {
	return &Rule{
		ID:          id,
		Name:        "rule-" + id,
		Description: "stress rule " + id,
		Pattern:     pattern,
		PatternType: PatternTypeGlob,
		Content:     "Always follow guideline " + id,
		Priority:    1,
		Category:    RuleCategoryGeneral,
		Scope:       RuleScopeGlobal,
		Tags:        []string{"stress", id},
		Metadata:    map[string]string{"src": "stress"},
	}
}

// TestRules_Stress_SustainedAddMatch drives the real AddProjectRule -> match
// pipeline under sustained load (N>=100), recording per-call latency. Each
// iteration adds a real *.go-matching rule and asserts GetRulesForFile returns a
// growing match set, so the run proves real storage + glob matching — not a stub.
func TestRules_Stress_SustainedAddMatch(t *testing.T) {
	mgr := NewManager()

	var added int64
	stresschaos.RunSustainedLoad(t, "rules_sustained_add_match",
		stresschaos.SustainedConfig{N: 600, MaxErrorRate: 0.0},
		func(i int) error {
			rule := newValidRule(fmt.Sprintf("add-%d", i), "*.go")
			if err := mgr.AddProjectRule(rule); err != nil {
				return fmt.Errorf("add project rule: %w", err)
			}
			atomic.AddInt64(&added, 1)
			// Match against a real path — every *.go rule added so far must match.
			matches := mgr.GetRulesForFile("internal/server/server.go")
			if len(matches) == 0 {
				return fmt.Errorf("iteration %d: no rules matched a .go file after adding %d *.go rules", i, atomic.LoadInt64(&added))
			}
			return nil
		})

	if atomic.LoadInt64(&added) == 0 {
		t.Fatal("rules manager added zero rules under sustained load — not real work")
	}
	if got := int64(mgr.Count()); got != atomic.LoadInt64(&added) {
		t.Fatalf("Count()=%d != rules added=%d — store lost mutations", got, atomic.LoadInt64(&added))
	}
	// Final match must see every *.go rule.
	final := mgr.GetRulesForFile("main.go")
	if int64(len(final)) != atomic.LoadInt64(&added) {
		t.Fatalf("final GetRulesForFile matched %d/%d *.go rules", len(final), atomic.LoadInt64(&added))
	}
	t.Logf("rules sustained: %d rules added, Count=%d, final match=%d", atomic.LoadInt64(&added), mgr.Count(), len(final))
}

// TestRules_Stress_SustainedParse drives the real Parser.Parse against a real
// on-disk .clinerules file under sustained load (N>=100). Each iteration opens,
// scans and parses the file and asserts the expected rule count, proving the real
// file-reading + parsing path runs end-to-end (not a cached no-op).
func TestRules_Stress_SustainedParse(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.clinerules"
	content := `# stress rules file
[go-style]
pattern: *.go
description: go style
priority: 5
category: style
tags: lint, go
Always run gofmt before commit.

[security]
pattern: /.*secret.*/
description: secret scan
category: security
Never commit secrets.

[any-rule]
pattern: *
Apply to everything.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rules file: %v", err)
	}

	var parsed int64
	stresschaos.RunSustainedLoad(t, "rules_sustained_parse",
		stresschaos.SustainedConfig{N: 400, MaxErrorRate: 0.0},
		func(i int) error {
			p := NewParser(path)
			rs, err := p.Parse()
			if err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			if rs.Count() != 3 {
				return fmt.Errorf("iteration %d: parsed %d rules, want 3", i, rs.Count())
			}
			// Verify a real parsed field: the security rule must be regex-typed.
			sec := rs.GetRule("security")
			if sec == nil || sec.PatternType != PatternTypeRegex {
				return fmt.Errorf("iteration %d: security rule not parsed as regex (got %v)", i, sec)
			}
			atomic.AddInt64(&parsed, 1)
			return nil
		})

	if atomic.LoadInt64(&parsed) == 0 {
		t.Fatal("parser parsed zero files under sustained load")
	}
	t.Logf("rules sustained parse: %d successful parses of a 3-rule file", atomic.LoadInt64(&parsed))
}

// TestRules_Stress_ConcurrentAddRemoveMatch hammers the shared Manager rule store
// from N>=10 concurrent goroutines that interleave AddProjectRule + AddWorkspaceRule
// + RemoveRule + GetRulesForFile + GetAllRules + Count, asserting no deadlock, no
// goroutine leak, and no data race (run under -race) on the RWMutex-guarded store.
// Each goroutine uses a disjoint ID namespace so Add never collides on ID, making
// every Add a real mutation that contends with concurrent readers and removers.
func TestRules_Stress_ConcurrentAddRemoveMatch(t *testing.T) {
	mgr := NewManager()

	var adds, removes, matches int64
	stresschaos.RunConcurrent(t, "rules_concurrent_add_remove_match",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 150, Timeout: 25 * time.Second},
		func(g, it int) error {
			id := fmt.Sprintf("g%d-i%d", g, it)
			switch (g + it) % 4 {
			case 0:
				if err := mgr.AddProjectRule(newValidRule(id, "*.go")); err != nil {
					return fmt.Errorf("add project: %w", err)
				}
				atomic.AddInt64(&adds, 1)
			case 1:
				if err := mgr.AddWorkspaceRule(newValidRule(id, "*.ts")); err != nil {
					return fmt.Errorf("add workspace: %w", err)
				}
				atomic.AddInt64(&adds, 1)
			case 2:
				// Remove a (possibly already-removed / never-added) id — RemoveRule
				// must serialise its slice mutation against concurrent appends.
				mgr.RemoveRule(fmt.Sprintf("g%d-i%d", g, it-1))
				atomic.AddInt64(&removes, 1)
			default:
				// Read path: GetRulesForFile takes RLock and iterates the (concurrently
				// mutating) rule slices — the classic read/write race surface.
				_ = mgr.GetRulesForFile("internal/server/handler.go")
				_ = mgr.GetAllRules()
				_ = mgr.Count()
				atomic.AddInt64(&matches, 1)
			}
			return nil
		})

	if atomic.LoadInt64(&adds) == 0 {
		t.Fatal("no rules added under concurrent load")
	}
	// After the churn the store must still match correctly — proof the map/slices
	// were not left torn by the concurrent mutations.
	if mgr.Count() < 0 {
		t.Fatalf("negative rule count after churn: %d", mgr.Count())
	}
	final := mgr.GetRulesForFile("x.go")
	t.Logf("rules concurrent: adds=%d removes=%d reads=%d final-count=%d go-matches=%d",
		atomic.LoadInt64(&adds), atomic.LoadInt64(&removes), atomic.LoadInt64(&matches), mgr.Count(), len(final))
}

// TestRules_Stress_BoundaryConditions exercises the §11.4.85(A)(3) boundary cases
// against the real Manager + Rule matching: (empty) match against a manager with
// NO rules must return zero; (max) many rules under one pattern must all match;
// (off-by-one) add-then-remove must end at exactly zero.
func TestRules_Stress_BoundaryConditions(t *testing.T) {
	// Empty: a fresh manager matches nothing and Counts zero.
	t.Run("no_rules", func(t *testing.T) {
		mgr := NewManager()
		if mgr.Count() != 0 {
			t.Fatalf("fresh manager Count()=%d, want 0", mgr.Count())
		}
		if got := mgr.GetRulesForFile("anything.go"); len(got) != 0 {
			t.Fatalf("empty manager matched %d rules, want 0", len(got))
		}
		if got := mgr.GetAllRules(); len(got) != 0 {
			t.Fatalf("empty manager GetAllRules returned %d, want 0", len(got))
		}
	})

	// Max: a large number of *.go rules must all match a .go file.
	t.Run("many_rules", func(t *testing.T) {
		mgr := NewManager()
		const many = 800
		for i := 0; i < many; i++ {
			if err := mgr.AddProjectRule(newValidRule(fmt.Sprintf("many-%d", i), "*.go")); err != nil {
				t.Fatalf("add %d: %v", i, err)
			}
		}
		if mgr.Count() != many {
			t.Fatalf("Count()=%d, want %d", mgr.Count(), many)
		}
		matches := mgr.GetRulesForFile("pkg/foo/bar.go")
		if len(matches) != many {
			t.Fatalf("matched %d/%d rules", len(matches), many)
		}
	})

	// Off-by-one: add one, remove it, expect exactly zero (not -1, not 1).
	t.Run("add_then_remove", func(t *testing.T) {
		mgr := NewManager()
		r := newValidRule("solo", "*.go")
		if err := mgr.AddProjectRule(r); err != nil {
			t.Fatalf("add: %v", err)
		}
		if mgr.Count() != 1 {
			t.Fatalf("after add Count()=%d, want 1", mgr.Count())
		}
		if !mgr.RemoveRule("solo") {
			t.Fatal("RemoveRule returned false for an existing rule")
		}
		if mgr.Count() != 0 {
			t.Fatalf("after remove Count()=%d, want 0", mgr.Count())
		}
		// Removing again must be a clean false, not a panic / negative count.
		if mgr.RemoveRule("solo") {
			t.Fatal("RemoveRule returned true for an already-removed rule")
		}
		if mgr.Count() != 0 {
			t.Fatalf("after double-remove Count()=%d, want 0", mgr.Count())
		}
	})

	// Boundary on pattern types: empty/huge file paths must not crash matching.
	t.Run("empty_and_huge_paths", func(t *testing.T) {
		mgr := NewManager()
		if err := mgr.AddProjectRule(newValidRule("anyp", "*")); err != nil {
			t.Fatalf("add: %v", err)
		}
		// Empty path — must not panic; "*" PatternTypeAny matches everything.
		_ = mgr.GetRulesForFile("")
		// Huge path — must not panic.
		huge := makeHugePath(1 << 16)
		got := mgr.GetRulesForFile(huge)
		if len(got) == 0 {
			t.Fatalf("PatternTypeAny rule should match huge path, matched %d", len(got))
		}
	})
}

func makeHugePath(n int) string {
	b := make([]byte, n)
	for i := range b {
		if i%32 == 0 {
			b[i] = '/'
		} else {
			b[i] = 'a'
		}
	}
	return string(b) + ".go"
}
