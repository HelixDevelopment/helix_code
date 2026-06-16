package sessionrules

// §11.4.85 stress + chaos coverage for the session-scoped permission Store.
//
// Why this matters (investigated, not guessed): Store gates LIVE permission
// decisions — Decide() is consulted on the hot path before a tool call runs, and
// Add/Remove mutate the rule set concurrently from the interactive command loop
// while background tool dispatch calls Decide/Rules/Has on the SAME *Store. A torn
// read or a lost mutation here is a real security-relevant defect (a stale-or-torn
// rule set could allow a call the operator just denied, or deny one they just
// allowed). The store.go doc explicitly promises "safe for concurrent use by
// multiple goroutines"; this suite proves that promise under §11.4.85 load + fault.
//
// The pre-existing store_test.go has a 50-goroutine smoke (TestStore_ConcurrentAccess);
// this file EXTENDS it to the full §11.4.85 floor: sustained load with captured
// p50/p95/p99 latency, ≥10-goroutine contention with deadlock/leak guards, boundary
// conditions, and four chaos fault classes (state-corruption, input-corruption,
// resource-pressure, goroutine-death-mid-op). Every PASS writes a captured-evidence
// artefact via the tests/stresschaos harness (latency.json / concurrency_report.json
// / recovery_trace.{json,log}).
//
// No fakes (§11.4.85 / CONST-050): the REAL *Store (real sync.RWMutex, real maps,
// real Decide→NewRuleEngine→Evaluate path) is exercised end-to-end. Run under -race
// to make the race detector the evidence for "no data race". No sleeps are used for
// synchronization — concurrency is driven by the harness's WaitGroup + start-gate.

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
	"dev.helix.code/tests/stresschaos"
)

// patternFor returns a deterministic, well-formed Bash rule pattern for index i.
// The arg-glob varies so distinct i produce distinct patterns (real upsert keys),
// while a bounded modulo keeps the working set finite for contention.
//
// The glob is `cmd<i>*` (trailing star, no `:`) so it genuinely matches the
// space-separated leaf commands the Decide assertions feed in (verified: the
// permissions matcher treats `:` as a literal, so `cmd0:*` does NOT match
// "cmd0 --flag" — it would fall through to Ask, masking the safety property
// these tests exercise).
func patternFor(i int) string {
	return fmt.Sprintf("Bash(cmd%d*)", i)
}

// cmdFor returns a leaf command that the pattern from patternFor(i) matches.
func cmdFor(i int) string {
	return fmt.Sprintf("cmd%d --run", i)
}

// ---------------------------------------------------------------------------
// STRESS — §11.4.85(A)
// ---------------------------------------------------------------------------

// TestStore_Stress_SustainedAddRemoveDecide drives Add→Has→Rules→Decide→Remove in
// a sustained loop (≥100 iters) and captures p50/p95/p99 latency. The full live
// gate cycle runs each iteration against the REAL store, so the latency report
// covers the actual permission-decision hot path, not a microbenchmark of one op.
func TestStore_Stress_SustainedAddRemoveDecide(t *testing.T) {
	s := New()
	const sess = "stress-sustained"

	rep := stresschaos.RunSustainedLoad(t, "store_sustained_add_remove_decide",
		stresschaos.SustainedConfig{N: 2000}, func(i int) error {
			pat := patternFor(i)
			s.Add(sess, permissions.Rule{Pattern: pat, Action: confirmation.ActionDeny, Priority: i % 100})
			if !s.Has(sess, pat) {
				return fmt.Errorf("iter %d: Has reported false immediately after Add(%q)", i, pat)
			}
			_ = s.Rules(sess)
			// The just-added deny rule MUST be honoured by a real Decide.
			d := s.Decide(sess, "Bash", cmdFor(i))
			if d.Action != confirmation.ActionDeny {
				return fmt.Errorf("iter %d: Decide returned %v, want Deny for just-added rule %q", i, d.Action, pat)
			}
			if !s.Remove(sess, pat) {
				return fmt.Errorf("iter %d: Remove(%q) reported false for a rule that was just added", i, pat)
			}
			return nil
		})

	// Sanity on the captured evidence itself (§11.4.5 — the report must be real).
	if rep.N < stresschaos.MinSustainedN {
		t.Fatalf("sustained run N=%d below §11.4.85 floor", rep.N)
	}
	if rep.ErrorRate != 0 {
		t.Fatalf("sustained run error rate %.4f != 0 — a live gate cycle failed", rep.ErrorRate)
	}
}

// TestStore_Stress_ConcurrentContention hammers the store from ≥10 goroutines that
// MIX Add/Remove/Rules/Has/Decide on BOTH a shared session key (max contention on
// one map) AND a per-goroutine distinct key (isolation under load). The harness
// guards against deadlock (timeout) and goroutine leak; -race catches data races.
func TestStore_Stress_ConcurrentContention(t *testing.T) {
	s := New()
	const shared = "shared-session"

	rep := stresschaos.RunConcurrent(t, "store_concurrent_contention",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 200},
		func(g, it int) error {
			distinct := fmt.Sprintf("g%d", g)
			pat := patternFor(g*1000 + it%64)

			// Writer side on both shared + distinct keys.
			s.Add(shared, permissions.Rule{Pattern: pat, Action: confirmation.ActionDeny, Priority: it % 50})
			s.Add(distinct, permissions.Rule{Pattern: pat, Action: confirmation.ActionAllow, Priority: 1})

			// Reader side: none of these may panic / tear under concurrent writes.
			_ = s.Rules(shared)
			_ = s.Has(shared, pat)
			_ = s.Decide(shared, "Bash", fmt.Sprintf("cmd%d run", g*1000+it%64))
			_ = s.Decide(distinct, "Bash", "ls -la")

			// Distinct-key isolation invariant: a goroutine's own distinct session
			// holds exactly the rule it just added (no other goroutine writes it),
			// so the just-added pattern MUST be present right after the Add above.
			if !s.Has(distinct, pat) {
				return fmt.Errorf("g%d it%d: distinct session lost its own just-added rule %q (cross-session leak / torn write)", g, it, pat)
			}

			s.Remove(shared, pat)
			return nil
		})

	if rep.Deadlock {
		t.Fatalf("contention run deadlocked")
	}
	if rep.TotalCalls < stresschaos.MinParallelism*50 {
		t.Fatalf("contention run only %d calls — below meaningful load", rep.TotalCalls)
	}
}

// TestStore_Stress_BoundaryConditions exercises the §11.4.85(A)(3) boundary cases:
// empty store, empty session key, empty pattern, duplicate add (upsert), remove of
// a non-existent rule, and a large rule set (max-ish) on one session.
func TestStore_Stress_BoundaryConditions(t *testing.T) {
	s := New()

	t.Run("empty_store_decide_is_ask", func(t *testing.T) {
		d := s.Decide("never-seen", "Bash", "anything")
		if d.Action != confirmation.ActionAsk {
			t.Fatalf("empty store Decide = %v, want Ask (fall-through)", d.Action)
		}
		if got := s.Rules("never-seen"); len(got) != 0 {
			t.Fatalf("empty store Rules = %d, want 0", len(got))
		}
		if s.Has("never-seen", "Bash(x:*)") {
			t.Fatal("empty store Has = true, want false")
		}
		if s.Remove("never-seen", "Bash(x:*)") {
			t.Fatal("empty store Remove = true, want false (no bluff success)")
		}
	})

	t.Run("empty_session_key", func(t *testing.T) {
		// An empty session key is a valid (if unusual) map key — it must behave
		// like any other isolated session, not panic or alias a different one.
		s.Add("", permissions.Rule{Pattern: "Edit(*)", Action: confirmation.ActionAllow})
		if !s.Has("", "Edit(*)") {
			t.Fatal("empty-key session lost its rule")
		}
		if s.Has("other", "Edit(*)") {
			t.Fatal("empty-key session aliased a non-empty key")
		}
		s.Remove("", "Edit(*)")
	})

	t.Run("duplicate_add_upserts", func(t *testing.T) {
		const sess = "dup"
		for i := 0; i < 100; i++ {
			s.Add(sess, permissions.Rule{Pattern: "Bash(git push:*)", Action: confirmation.ActionDeny, Priority: i})
		}
		got := s.Rules(sess)
		if len(got) != 1 {
			t.Fatalf("100 adds of one pattern produced %d rules, want 1 (upsert)", len(got))
		}
		if got[0].Priority != 99 {
			t.Fatalf("upsert kept stale priority %d, want last-write 99", got[0].Priority)
		}
		s.Remove(sess, "Bash(git push:*)")
	})

	t.Run("large_rule_set", func(t *testing.T) {
		const sess = "big"
		const n = 5000
		for i := 0; i < n; i++ {
			s.Add(sess, permissions.Rule{Pattern: patternFor(i), Action: confirmation.ActionAsk, Priority: i % 10})
		}
		got := s.Rules(sess)
		if len(got) != n {
			t.Fatalf("large set: Rules returned %d, want %d", len(got), n)
		}
		// Sorted descending priority then pattern — verify the ordering invariant
		// holds on a large set (Rules promises a deterministic order).
		for i := 1; i < len(got); i++ {
			if got[i-1].Priority < got[i].Priority {
				t.Fatalf("large set: priority order violated at %d (%d < %d)", i, got[i-1].Priority, got[i].Priority)
			}
		}
		// Decide still resolves against a large rule set without error.
		_ = s.Decide(sess, "Bash", "cmd1 go")
	})
}

// ---------------------------------------------------------------------------
// CHAOS — §11.4.85(B)
// ---------------------------------------------------------------------------

// TestStore_Chaos_StateCorruptionAddDuringDecide injects the §11.4.85(B)(5)
// state-corruption fault: a swarm of writers hammers Add/Remove on the SAME session
// while a reader swarm calls Decide concurrently. The invariant under test is the
// store.go promise that Decide takes a CONSISTENT snapshot (Rules() copies under
// RLock) — a torn read would yield a Decision neither all-old nor all-new, or
// panic. We assert every concurrent Decide returns a well-formed Decision (one of
// the three closed actions) and that the run is race-clean (under -race).
func TestStore_Chaos_StateCorruptionAddDuringDecide(t *testing.T) {
	s := New()
	const sess = "chaos-state"
	rec := stresschaos.NewChaosRecorder(t, "store_state_corruption_add_during_decide", "state-corruption")

	// Seed a baseline rule so Decide has something to match from the start.
	// The glob `seed*` genuinely matches the "seed now" leaf fed below.
	s.Add(sess, permissions.Rule{Pattern: "Bash(seed*)", Action: confirmation.ActionDeny, Priority: 100})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	var decideCalls int64
	var torn int64

	// Writer swarm: continuously mutate the rule set.
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; ctx.Err() == nil; i++ {
				pat := patternFor(id*100 + i%32)
				s.Add(sess, permissions.Rule{Pattern: pat, Action: confirmation.ActionAllow, Priority: i % 7})
				if i%2 == 0 {
					s.Remove(sess, pat)
				}
			}
		}(w)
	}

	// Reader swarm: Decide concurrently; assert each decision is well-formed.
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				d := s.Decide(sess, "Bash", "seed now")
				atomic.AddInt64(&decideCalls, 1)
				switch d.Action {
				case confirmation.ActionAllow, confirmation.ActionDeny, confirmation.ActionAsk:
					// well-formed
				default:
					atomic.AddInt64(&torn, 1)
				}
			}
		}()
	}

	wg.Wait()

	calls := atomic.LoadInt64(&decideCalls)
	rec.Record(stresschaos.Recovered, fmt.Sprintf("%d concurrent Decide calls during Add/Remove storm", calls))
	if t := atomic.LoadInt64(&torn); t > 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("%d Decide calls returned a malformed Action (torn read)", t))
	} else {
		rec.Record(stresschaos.Recovered, "every concurrent Decide returned a well-formed closed-set Action (no torn read)")
	}
	if calls < 100 {
		t.Fatalf("only %d Decide calls in 5s window — workload did not run", calls)
	}
	rec.AssertNoFatal()
}

// TestStore_Chaos_CorruptPatternFailsClosed injects the §11.4.85(B)(3)
// input-corruption fault at the Decide layer: malformed / hostile patterns are
// stored and then a real Decide is run. The store.go contract is FAIL-CLOSED — a
// corrupt session rule set must DENY, never silently Allow. This is the load-bearing
// safety property the §1.1 mutation below targets.
func TestStore_Chaos_CorruptPatternFailsClosed(t *testing.T) {
	// Patterns that ParsePattern rejects as MALFORMED (no ToolName(arg) shape) —
	// these force Store.Decide down the fail-closed branch (corrupt engine → Deny).
	// Verified malformed (store.go Decide + rule_engine.ParsePattern): an embedded
	// NUL inside a well-formed "Bash(...)" shape is NOT malformed (it parses), so it
	// is deliberately excluded here — this set is the genuinely-corrupt subset that
	// MUST hit the fail-closed branch.
	malformed := []string{
		"(((((",                // unbalanced parens
		"NotAToolNameNoParens", // no ToolName(arg) shape
		"Bash(",                // truncated
		")(Bash",               // garbage
		"\x00\x01\x02",         // NUL/control bytes — no parens shape
		"%s%s%s%n",             // format-string payload
	}

	// Drive the recorder DIRECTLY (not ChaosCorruptInputDuring) so an ALLOW outcome
	// is recorded as FATAL — a corrupt rule set that allows is a security defect,
	// NOT a graceful rejection. This is what makes the test catch a fail-closed →
	// fail-open mutation of store.go (proven by the §1.1 paired mutation: flipping
	// the Decide corrupt-engine branch from Deny to Allow MUST turn this RED).
	rec := stresschaos.NewChaosRecorder(t, "store_corrupt_pattern_fails_closed", "input-corruption")

	for _, pat := range malformed {
		func(pattern string) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("Decide PANICKED on malformed pattern %q: %v", pattern, p))
				}
			}()
			s := New()
			const sess = "corrupt"
			// Storing a malformed pattern must not panic (Add does no validation).
			s.Add(sess, permissions.Rule{Pattern: pattern, Action: confirmation.ActionAllow, Priority: 1})
			// The corrupt rule set MUST make Decide FAIL CLOSED — a request that would
			// otherwise be denied/asked must NEVER be silently allowed.
			d := s.Decide(sess, "Bash", "rm -rf /")
			switch d.Action {
			case confirmation.ActionDeny:
				rec.Record(stresschaos.Degraded, fmt.Sprintf("malformed pattern %q -> fail-closed Deny (correct)", pattern))
			case confirmation.ActionAllow:
				rec.Record(stresschaos.Fatal, fmt.Sprintf("malformed pattern %q -> ALLOW — fail-closed VIOLATED (security defect)", pattern))
			default:
				// A malformed pattern MUST reach the corrupt-engine Deny branch; an
				// Ask here would mean the pattern was not actually treated as corrupt,
				// weakening the test's premise.
				rec.Record(stresschaos.Fatal, fmt.Sprintf("malformed pattern %q -> %v (expected fail-closed Deny; not treated as corrupt)", pattern, d.Action))
			}
		}(pat)
	}

	tr := rec.AssertNoFatal()
	if tr.Degraded != len(malformed) {
		t.Fatalf("expected all %d malformed patterns to fail-closed to Deny; got %d Deny outcomes (events: %v)", len(malformed), tr.Degraded, tr.Events)
	}
}

// TestStore_Chaos_ResourcePressureManySessionsManyRules injects the §11.4.85(B)(4)
// resource-exhaustion fault: under bounded memory pressure, fill many sessions ×
// many rules and keep operating (Add/Decide/Remove). The store must keep functioning
// rather than crash.
func TestStore_Chaos_ResourcePressureManySessionsManyRules(t *testing.T) {
	s := New()
	stresschaos.ChaosResourcePressureDuring(t, "store_resource_pressure_many_sessions", 32,
		func(rec *stresschaos.ChaosRecorder) {
			const sessions = 200
			const rulesPer = 100
			for sIdx := 0; sIdx < sessions; sIdx++ {
				sess := fmt.Sprintf("sess-%d", sIdx)
				for rIdx := 0; rIdx < rulesPer; rIdx++ {
					s.Add(sess, permissions.Rule{Pattern: patternFor(rIdx), Action: confirmation.ActionAsk, Priority: rIdx})
				}
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("filled %d sessions × %d rules under memory pressure", sessions, rulesPer))
			// Still usable after the fill: decide + remove across sessions.
			for sIdx := 0; sIdx < sessions; sIdx += 25 {
				sess := fmt.Sprintf("sess-%d", sIdx)
				_ = s.Decide(sess, "Bash", "cmd1 x")
				if got := s.Rules(sess); len(got) != rulesPer {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("session %s lost rules under pressure: %d != %d", sess, len(got), rulesPer))
				}
			}
			rec.Record(stresschaos.Recovered, "store stayed functional + self-consistent under pressure")
		})
}

// TestStore_Chaos_GoroutineDeathMidOp injects the §11.4.85(B)(1) process-death
// fault: a worker churning Add/Remove/Decide on the store is cancelled mid-flight.
// The shared store must unwind cleanly and remain usable afterward.
func TestStore_Chaos_GoroutineDeathMidOp(t *testing.T) {
	s := New()
	const sess = "chaos-death"

	stresschaos.ChaosKillDuring(t, "store_goroutine_death_mid_op", 100*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			for i := 0; ctx.Err() == nil; i++ {
				pat := patternFor(i % 128)
				s.Add(sess, permissions.Rule{Pattern: pat, Action: confirmation.ActionDeny, Priority: i % 10})
				_ = s.Decide(sess, "Bash", "cmd1 go")
				s.Remove(sess, pat)
			}
			rec.Record(stresschaos.Recovered, "churn worker observed cancellation and stopped")
		})

	// Post-cancellation the store must still be fully usable (no held lock, no
	// corrupted map): a fresh Add/Has/Decide/Remove cycle must succeed. The glob
	// `after*` genuinely matches the "after now" leaf below.
	s.Add(sess, permissions.Rule{Pattern: "Bash(after*)", Action: confirmation.ActionDeny, Priority: 1})
	if !s.Has(sess, "Bash(after*)") {
		t.Fatal("store unusable after goroutine-death chaos: Has failed")
	}
	if d := s.Decide(sess, "Bash", "after now"); d.Action != confirmation.ActionDeny {
		t.Fatalf("store unusable after goroutine-death chaos: Decide = %v, want Deny", d.Action)
	}
	if !s.Remove(sess, "Bash(after*)") {
		t.Fatal("store unusable after goroutine-death chaos: Remove failed")
	}
}
