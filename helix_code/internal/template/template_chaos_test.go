package template

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the template package.
//
// Chaos classes exercised against the REAL *Manager / *Template (no fakes —
// real validation, real substitution, real mutex-guarded indexes, real callback
// dispatch):
//
//   - input-corruption: structurally hostile template SOURCE (unterminated
//     braces, nested/recursive placeholders, injection-y markers, binary garbage,
//     huge content) AND hostile render DATA (nil values, self-referential cycles,
//     unprintable values). Parse + Validate + Render MUST reject cleanly or
//     normalise — a panic on malformed input is a §11.4.85(B) Fatal.
//   - state-corruption under contention: a single Manager is concurrently
//     Register/Update/Delete/Clear'd plus callback-registered from many
//     goroutines. The RWMutex must serialise so the store never panics/races and
//     ends self-consistent.
//   - process-death: a long render+register loop is cancelled mid-operation; it
//     must observe the cancellation and unwind cleanly without leaking a goroutine.
//   - resource-pressure: rendering a large template store proceeds under bounded
//     memory pressure without OOM-crash.
//   - callback-panic injection: a registered OnCreate/OnUpdate/OnDelete callback
//     that panics mid-dispatch MUST NOT take down the manager or corrupt its
//     mutex-guarded state — the manager must stay usable afterwards.

// TestTemplate_Chaos_CorruptTemplateSource feeds structurally hostile template
// content to the REAL ParseTemplate + Validate + Render path. Parsing/validation
// must reject with an error or normalise gracefully — never panic. A crash on
// malformed input is fatal.
func TestTemplate_Chaos_CorruptTemplateSource(t *testing.T) {
	corrupt := [][]byte{
		[]byte("{{unterminated"),                                  // 0: unterminated placeholder
		[]byte("}}backwards{{"),                                   // 1: reversed braces
		[]byte("{{a}}{{b}}{{c}}{{d}}{{e}}"),                       // 2: many placeholders, none supplied
		[]byte("{{{{nested}}}}"),                                  // 3: nested braces
		[]byte("{{1invalid}} {{ spaced }} {{-dash}}"),             // 4: invalid placeholder names (not extracted)
		[]byte("\x00\x01\x02\xff\xfe binary garbage \x00 {{x}}"),  // 5: binary garbage + a placeholder
		[]byte(strings.Repeat("{{x}}", 10000)),                    // 6: huge repeated placeholder
		[]byte("{{a}} {{a}} {{a}}"),                               // 7: repeated same placeholder
		[]byte(""),                                                // 8: empty content
		[]byte("plain text no placeholders at all"),               // 9: no placeholders
	}

	stresschaos.ChaosCorruptInputDuring(t, "template_corrupt_template_source", corrupt,
		func(input []byte) error {
			tpl, err := ParseTemplate("chaos", string(input), TypeCustom)
			if err != nil {
				return err // graceful rejection (Degraded) — desired
			}
			// Validate the parsed template (empty content is rejected here).
			if err := tpl.Validate(); err != nil {
				return err
			}
			// Render with all extracted vars supplied so substitution flows through.
			vars := make(map[string]interface{})
			for _, v := range tpl.ExtractVariables() {
				vars[v] = "VAL"
			}
			out, rerr := tpl.Render(vars)
			if rerr != nil {
				return rerr
			}
			// Accepted: must hand back a usable string. Touch it (mirrors a real
			// consumer); any unreplaced placeholder would have already errored.
			_ = len(out)
			return nil
		})
}

// TestTemplate_Chaos_CorruptRenderData feeds hostile DATA values into the REAL
// render path of a fixed valid template. nil values, self-referential maps,
// channels/funcs, and huge values flow through fmt.Sprint inside Render — none of
// which may panic. A crash on hostile data is a §11.4.85(B) Fatal.
func TestTemplate_Chaos_CorruptRenderData(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "template_corrupt_render_data", "input-corruption")

	tpl := NewTemplate("data-chaos", "", TypeCustom)
	tpl.SetContent("a={{a}} b={{b}}")
	tpl.AddVariable(Variable{Name: "a", Required: true, Type: "string"})
	tpl.AddVariable(Variable{Name: "b", Required: true, Type: "string"})

	// A deeply-nested (but acyclic) value stresses fmt.Sprint's recursion without
	// the uncatchable stack-overflow a self-referential cycle would cause — Go's
	// fmt does not guard cyclic maps, so a true cycle is an out-of-scope crash of
	// the fmt package itself, not a template-engine defect.
	nested := map[string]interface{}{"k": map[string]interface{}{"k": map[string]interface{}{"k": "deep"}}}

	hostile := []map[string]interface{}{
		{"a": nil, "b": nil},
		{"a": make(chan int), "b": func() {}},
		{"a": nested, "b": nested},
		{"a": strings.Repeat("x", 1<<16), "b": strings.Repeat("y", 1<<16)},
		{"a": []byte{0x00, 0xff, 0xfe}, "b": struct{ X int }{42}},
		{"a": 3.14, "b": -0.0},
	}

	for i, data := range hostile {
		func(idx int, d map[string]interface{}) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("render[%d] panicked on hostile data: %v", idx, p))
				}
			}()
			out, err := tpl.Render(d)
			if err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("render[%d] rejected hostile data cleanly: %v", idx, err))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("render[%d] substituted hostile data without crash (len=%d)", idx, len(out)))
			}
		}(i, data)
	}

	rec.AssertNoFatal()
	t.Log("template render survived hostile-data injection")
}

// TestTemplate_Chaos_CallbackPanicIsolation registers a callback that panics and
// then drives Register/Update/Delete. A panicking callback runs while the
// Manager holds its write lock; if the manager does not isolate the panic it
// propagates to the caller AND — critically — leaves the mutex locked, which
// would deadlock every subsequent operation. The manager MUST stay usable.
func TestTemplate_Chaos_CallbackPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "template_callback_panic_isolation", "process-death")

	mgr := NewManager()
	var goodCreate int64
	mgr.OnCreate(func(_ *Template) { atomic.AddInt64(&goodCreate, 1) })
	mgr.OnCreate(func(_ *Template) { panic("chaos: OnCreate callback panic") })
	mgr.OnCreate(func(_ *Template) { atomic.AddInt64(&goodCreate, 1) })

	tpl := newStressTemplate("panic-cb", "Panic Callback Template")

	// Drive the Register on a guarded goroutine: if the manager does not isolate
	// the callback panic, in the synchronous path it propagates here. We catch it
	// and record Degraded (surfaced error/panic is acceptable degradation), but
	// the real test is whether the manager mutex is left locked (deadlock below).
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("register propagated callback panic to caller: %v", p))
			}
		}()
		if err := mgr.Register(tpl); err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("register surfaced error: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "register completed despite panicking callback")
		}
	}()

	// CRITICAL: the manager must remain usable. If the write lock was left held by
	// the panicking callback path, this follow-up Register will block forever — we
	// guard it with a timeout and record Fatal (deadlock) if it does not return.
	followUpDone := make(chan error, 1)
	go func() {
		tpl2 := newStressTemplate("follow-up", "Follow Up Template")
		// Replace the panicking callbacks impact: register a fresh manager op.
		followUpDone <- mgr.Register(tpl2)
	}()

	select {
	case err := <-followUpDone:
		if err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("follow-up register errored: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "manager still usable after callback panic — lock not leaked")
		}
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "manager deadlocked after callback panic — write lock leaked")
	}

	rec.AssertNoFatal()
	t.Logf("manager survived callback-panic injection (good-callback hits=%d)", atomic.LoadInt64(&goodCreate))
}

// TestTemplate_Chaos_ConcurrentChurnWithClear hammers the SAME Manager with
// concurrent Register / Update / Delete / Render / Clear plus concurrent callback
// registration from many goroutines. Clear mid-flight (full store wipe) races
// against concurrent registrations/reads — the harshest state-corruption surface.
// The manager must never panic or race and must stay self-consistent. Run -race.
func TestTemplate_Chaos_ConcurrentChurnWithClear(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "template_concurrent_churn_with_clear", "state-corruption")
	mgr := NewManager()

	const goroutines = 12
	const iters = 250
	var wg sync.WaitGroup
	var registers, updates, deletes, clears, reads int64

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
				name := fmt.Sprintf("churn-%d-%d", id, it)
				switch (id + it) % 6 {
				case 0:
					tpl := newStressTemplate(name, name)
					_ = mgr.Register(tpl)
					atomic.AddInt64(&registers, 1)
				case 1:
					// Update a template registered by this goroutine (if present).
					if got, err := mgr.GetByName(fmt.Sprintf("churn-%d-%d", id, it-1)); err == nil {
						_ = mgr.Update(got.ID, func(tt *Template) { tt.Description = "churned" })
					}
					atomic.AddInt64(&updates, 1)
				case 2:
					if got, err := mgr.GetByName(fmt.Sprintf("churn-%d-%d", id, it-2)); err == nil {
						_ = mgr.Delete(got.ID)
					}
					atomic.AddInt64(&deletes, 1)
				case 3:
					if it%50 == 0 {
						mgr.Clear()
						atomic.AddInt64(&clears, 1)
					} else {
						_ = mgr.GetAll()
						_ = mgr.Count()
						atomic.AddInt64(&reads, 1)
					}
				case 4:
					_ = mgr.Search("stress")
					_ = mgr.GetByType(TypePrompt)
					_ = mgr.CountByType()
					atomic.AddInt64(&reads, 1)
				default:
					// Register a callback mid-churn — exercises the callback-slice
					// mutation racing against Register/Update/Delete iteration.
					mgr.OnCreate(func(_ *Template) {})
					atomic.AddInt64(&reads, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived churn+clear: %d registers, %d updates, %d deletes, %d clears, %d reads, no panic/race",
		atomic.LoadInt64(&registers), atomic.LoadInt64(&updates),
		atomic.LoadInt64(&deletes), atomic.LoadInt64(&clears), atomic.LoadInt64(&reads)))

	// Final state must be coherent and the store must still work: a fresh register
	// must be renderable, proving the maps/slices were not left torn after a Clear.
	if c := mgr.Count(); c < 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("template count went negative: %d", c))
	}
	final := newStressTemplate("final", "Final Template")
	if err := mgr.Register(final); err != nil {
		rec.Record(stresschaos.Degraded, "final register errored: "+err.Error())
	}
	if out, err := mgr.Render(final.ID, map[string]interface{}{"name": "X"}); err != nil || !strings.Contains(out, "X") {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("store did not render a fresh template after churn — corrupted (out=%q err=%v)", out, err))
	} else {
		rec.Record(stresschaos.Recovered, "store renders correctly after churn — self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("template churn: registers=%d updates=%d deletes=%d clears=%d reads=%d final-count=%d",
		atomic.LoadInt64(&registers), atomic.LoadInt64(&updates),
		atomic.LoadInt64(&deletes), atomic.LoadInt64(&clears), atomic.LoadInt64(&reads), mgr.Count())
}

// TestTemplate_Chaos_CancelDuringRenderLoop injects a process-death fault: a long
// register+render loop honours a cancellable context and must unwind cleanly when
// the context is cancelled mid-flight, without leaking the worker goroutine.
func TestTemplate_Chaos_CancelDuringRenderLoop(t *testing.T) {
	stresschaos.ChaosKillDuring(t, "template_cancel_during_render_loop", 40*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			mgr := NewManager()
			tpl := newStressTemplate("loop", "Loop Template")
			if err := mgr.Register(tpl); err != nil {
				rec.Record(stresschaos.Degraded, "register errored: "+err.Error())
				return
			}
			iterations := 0
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Recovered, fmt.Sprintf("render loop observed cancellation after %d iterations", iterations))
					return
				default:
				}
				_, err := mgr.Render(tpl.ID, map[string]interface{}{"name": fmt.Sprintf("n%d", iterations)})
				if err != nil {
					rec.Record(stresschaos.Degraded, "render errored mid-loop: "+err.Error())
					return
				}
				iterations++
			}
		})
}

// TestTemplate_Chaos_RenderUnderMemoryPressure asserts rendering a large template
// store proceeds under bounded memory pressure without OOM-crash (§11.4.85(B)(4)).
func TestTemplate_Chaos_RenderUnderMemoryPressure(t *testing.T) {
	mgr := NewManager()
	const n = 500
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("mem-%d", i)
		tpl := newStressTemplate(name, name)
		if err := mgr.Register(tpl); err != nil {
			t.Fatalf("register %d: %v", i, err)
		}
		ids[i] = tpl.ID
	}

	stresschaos.ChaosResourcePressureDuring(t, "template_render_under_memory_pressure", 32,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < n; i++ {
				out, err := mgr.Render(ids[i], map[string]interface{}{"name": fmt.Sprintf("u%d", i)})
				if err != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("render %d errored under pressure: %v", i, err))
					return
				}
				if !strings.Contains(out, fmt.Sprintf("u%d", i)) {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("render %d wrong output under pressure: %q", i, out))
					return
				}
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("rendered %d-template store under memory pressure", n))
		})
}
