package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the rules package.
//
// Chaos classes exercised against the REAL *Manager / *Parser / *Rule (no fakes —
// real on-disk files, real parsing, real mutex-guarded store):
//
//   - input-corruption: structurally hostile .clinerules content + malformed
//     in-memory rule definitions (invalid regex pattern, triple-star glob, empty
//     name/content, unclosed brackets, binary garbage). Parse + Validate + match
//     MUST reject cleanly without crashing — a panic on malformed input is a
//     §11.4.85(B) Fatal.
//   - state-corruption under contention: a single Manager is concurrently
//     Add/Remove/Match/Clear'd from many goroutines mid-flight. The RWMutex must
//     serialise so the store never panics or races and ends self-consistent.
//   - process-death: a long parse/match loop is cancelled mid-operation; it must
//     observe the cancellation and unwind cleanly without leaking a goroutine.
//   - resource-pressure: matching a large rule store proceeds under bounded
//     memory pressure without OOM-crash.

// TestRules_Chaos_CorruptRuleFiles feeds structurally hostile .clinerules content
// to the REAL Parser. Parsing malformed files must either reject with an error or
// normalise gracefully — never panic. A crash on malformed input is fatal.
func TestRules_Chaos_CorruptRuleFiles(t *testing.T) {
	dir := t.TempDir()

	corrupt := [][]byte{
		[]byte("[unclosed-bracket\npattern: *.go\ncontent here"),                    // 0: unterminated rule header
		[]byte("[bad-regex]\npattern: /[unterminated(/\nNever happens."),            // 1: invalid regex pattern value
		[]byte("[triple]\npattern: ***.go\nTriple star glob (invalid)."),            // 2: *** glob rejected by Validate
		[]byte("[empty-content]\npattern: *.go\n"),                                  // 3: no content -> Validate fails
		[]byte("[]\npattern: *.go\nEmpty name."),                                    // 4: empty rule name -> Validate fails
		[]byte("\x00\x01\x02\xff\xfe garbage \x00 not a rules file at all"),         // 5: binary garbage, no rule header
		[]byte("[dup]\npattern: *.go\nfirst\n[dup]\npattern: *.ts\nsecond"),         // 6: duplicate IDs -> AddRule rejects
		[]byte("priority: notanumber\n[x]\npattern: *.go\npriority: alsonotanumber\nbody"), // 7: bad priority values
	}
	// Persist each corrupt payload as a real file so the real os.Open + scanner
	// path is exercised (not just ParseString).
	payloads := make([][]byte, len(corrupt))
	for i, c := range corrupt {
		path := filepath.Join(dir, fmt.Sprintf("corrupt-%d.clinerules", i))
		if err := os.WriteFile(path, c, 0o644); err != nil {
			t.Fatalf("write corrupt file %d: %v", i, err)
		}
		payloads[i] = []byte(path) // feed() receives the on-disk path
	}

	stresschaos.ChaosCorruptInputDuring(t, "rules_corrupt_rule_files", payloads,
		func(input []byte) error {
			p := NewParser(string(input))
			rs, err := p.Parse()
			if err != nil {
				return err // graceful rejection (Degraded) — desired
			}
			// Accepted: must hand back a usable, non-panicking RuleSet. Touch it to
			// flow the parsed data through (mirrors a real consumer).
			_ = rs.Count()
			_ = rs.GetMatchingRules("foo.go")
			return nil
		})
}

// TestRules_Chaos_InvalidRuleDefinitions feeds hostile in-memory *Rule values to
// the REAL AddProjectRule / Validate path. Invalid rules must be rejected cleanly
// (error, not panic) and must NOT enter the store. A non-existent rule pattern
// that crashes matching is a §11.4.85(B) Fatal.
func TestRules_Chaos_InvalidRuleDefinitions(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "rules_invalid_rule_definitions", "input-corruption")
	mgr := NewManager()

	bad := []*Rule{
		{ID: "no-name", Pattern: "*.go", PatternType: PatternTypeGlob, Content: "x"},      // empty Name
		{ID: "no-content", Name: "nc", Pattern: "*.go", PatternType: PatternTypeGlob},     // empty Content
		{ID: "no-pattern", Name: "np", PatternType: PatternTypeGlob, Content: "x"},        // empty Pattern (glob)
		{ID: "bad-regex", Name: "br", Pattern: "[unterminated(", PatternType: PatternTypeRegex, Content: "x"}, // invalid regex
		{ID: "triple", Name: "ts", Pattern: "***", PatternType: PatternTypeGlob, Content: "x"},                // triple star
	}

	for i, r := range bad {
		func(idx int, rule *Rule) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("bad[%d] AddProjectRule panicked: %v", idx, p))
				}
			}()
			if err := mgr.AddProjectRule(rule); err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("bad[%d] rejected cleanly: %v", idx, err))
			} else {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("bad[%d] invalid rule accepted into store", idx))
			}
		}(i, r)
	}

	// Even with the invalid-regex rule rejected, matching a file must not panic.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("matching after invalid adds panicked: %v", p))
			}
		}()
		_ = mgr.GetRulesForFile("x.go")
		rec.Record(stresschaos.Recovered, "matching survived invalid-rule rejections")
	}()

	// The store must contain none of the invalid rules.
	if mgr.Count() != 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("store accepted %d invalid rules", mgr.Count()))
	} else {
		rec.Record(stresschaos.Recovered, "no invalid rule entered the store")
	}

	rec.AssertNoFatal()
	t.Log("rules survived invalid-rule-definition injection")
}

// TestRules_Chaos_ConcurrentChurnWithClear hammers the SAME Manager with
// concurrent AddProjectRule / AddWorkspaceRule / RemoveRule / GetRulesForFile /
// GetAllRules / Clear from many goroutines. The Manager.mu must serialise the
// store mutations so the manager never panics or races and stays self-consistent.
// Clear mid-flight (full store wipe) races against concurrent appends/reads —
// the harshest state-corruption surface. Run under -race.
func TestRules_Chaos_ConcurrentChurnWithClear(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "rules_concurrent_churn_with_clear", "state-corruption")
	mgr := NewManager()

	const goroutines = 12
	const iters = 250
	var wg sync.WaitGroup
	var adds, removes, clears, reads int64

	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				switch (id + it) % 5 {
				case 0:
					_ = mgr.AddProjectRule(newValidRule(fmt.Sprintf("p-%d-%d", id, it), "*.go"))
					atomic.AddInt64(&adds, 1)
				case 1:
					_ = mgr.AddWorkspaceRule(newValidRule(fmt.Sprintf("w-%d-%d", id, it), "*.ts"))
					atomic.AddInt64(&adds, 1)
				case 2:
					mgr.RemoveRule(fmt.Sprintf("p-%d-%d", id, it-1))
					atomic.AddInt64(&removes, 1)
				case 3:
					// Clear wipes the whole store while others append/read.
					if it%50 == 0 {
						mgr.Clear()
						atomic.AddInt64(&clears, 1)
					} else {
						_ = mgr.GetRulesForFile("a/b/c.go")
						atomic.AddInt64(&reads, 1)
					}
				default:
					_ = mgr.GetAllRules()
					_ = mgr.Count()
					_ = mgr.Export()
					atomic.AddInt64(&reads, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived churn+clear: %d adds, %d removes, %d clears, %d reads, no panic/race",
		atomic.LoadInt64(&adds), atomic.LoadInt64(&removes), atomic.LoadInt64(&clears), atomic.LoadInt64(&reads)))

	// Final state must be coherent and the store must still work: a fresh add must
	// be matchable, proving the slices/map were not left torn after a Clear race.
	if c := mgr.Count(); c < 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("rule count went negative: %d", c))
	}
	if err := mgr.AddProjectRule(newValidRule("final", "*.go")); err != nil {
		rec.Record(stresschaos.Degraded, "final add errored: "+err.Error())
	}
	if got := mgr.GetRulesForFile("final.go"); len(got) == 0 {
		rec.Record(stresschaos.Fatal, "store did not match a fresh rule after churn — corrupted")
	} else {
		rec.Record(stresschaos.Recovered, "store matches correctly after churn — self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("rules churn: adds=%d removes=%d clears=%d reads=%d final-count=%d",
		atomic.LoadInt64(&adds), atomic.LoadInt64(&removes), atomic.LoadInt64(&clears), atomic.LoadInt64(&reads), mgr.Count())
}

// TestRules_Chaos_CancelDuringParseLoop injects a process-death fault: a long
// parse+match loop honours a cancellable context and must unwind cleanly when the
// context is cancelled mid-flight, without leaking the worker goroutine.
func TestRules_Chaos_CancelDuringParseLoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".clinerules")
	content := "[loop]\npattern: *.go\ndescription: loop rule\nBody content for the loop rule.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rules file: %v", err)
	}

	stresschaos.ChaosKillDuring(t, "rules_cancel_during_parse_loop", 40*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			mgr := NewManager()
			iterations := 0
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Recovered, fmt.Sprintf("parse loop observed cancellation after %d iterations", iterations))
					return
				default:
				}
				p := NewParser(path)
				rs, err := p.Parse()
				if err != nil {
					rec.Record(stresschaos.Degraded, "parse errored mid-loop: "+err.Error())
					return
				}
				// Feed parsed rules into a real manager and match — real work.
				for _, r := range rs.Rules {
					_ = mgr.AddProjectRule(r.Clone())
				}
				_ = mgr.GetRulesForFile("x.go")
				iterations++
			}
		})
}

// TestRules_Chaos_MatchUnderMemoryPressure asserts matching a large rule store
// proceeds under bounded memory pressure without OOM-crash (§11.4.85(B)(4)).
func TestRules_Chaos_MatchUnderMemoryPressure(t *testing.T) {
	mgr := NewManager()
	const n = 500
	for i := 0; i < n; i++ {
		if err := mgr.AddProjectRule(newValidRule(fmt.Sprintf("mem-%d", i), "*.go")); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}

	stresschaos.ChaosResourcePressureDuring(t, "rules_match_under_memory_pressure", 32,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 200; i++ {
				matches := mgr.GetRulesForFile(fmt.Sprintf("pkg/file_%d.go", i))
				if len(matches) != n {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("match %d returned %d/%d rules under pressure", i, len(matches), n))
					return
				}
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("matched %d-rule store 200x under memory pressure", n))
		})
}
