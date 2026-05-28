package template

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the template package.
//
// Stress classes exercised against the REAL *Manager / *Template (no fakes —
// real validation, real placeholder substitution, real mutex-guarded indexes):
//
//   - sustained load: >=100 real renders against a registered template, p50/p95/
//     p99 latency captured. Proves rendering does not degrade or error under
//     repeated invocation.
//   - concurrent contention: >=10 goroutines hammer Register / Get / GetByName /
//     GetByType / Render / Search / Count simultaneously. The RWMutex must
//     serialise so no panic / race / deadlock occurs (run under -race).
//   - boundary conditions: empty content, missing required keys, many templates,
//     huge content, off-by-one placeholder shapes — each categorised and asserted.

// newStressTemplate builds a real, valid template with a single required {{name}}
// placeholder, suitable for repeated rendering.
func newStressTemplate(id, name string) *Template {
	tpl := NewTemplate(name, "stress template", TypePrompt)
	tpl.SetContent("Hello {{name}}, welcome to {{place}}.")
	tpl.AddVariable(Variable{Name: "name", Required: true, Type: "string"})
	tpl.AddVariable(Variable{Name: "place", Required: false, DefaultValue: "HelixCode", Type: "string"})
	return tpl
}

// TestTemplate_Stress_SustainedRender renders a REAL registered template under
// sustained load (>=100 invocations). Every render must succeed and produce the
// substituted output — a non-zero error rate or a wrong substitution is a
// §11.4.85(A) failure. p50/p95/p99 latency is captured to latency.json.
func TestTemplate_Stress_SustainedRender(t *testing.T) {
	mgr := NewManager()
	tpl := newStressTemplate("sustained", "Sustained Render Template")
	if err := mgr.Register(tpl); err != nil {
		t.Fatalf("register template: %v", err)
	}

	rep := stresschaos.RunSustainedLoad(t, "template_sustained_render",
		stresschaos.SustainedConfig{N: 2000},
		func(i int) error {
			out, err := mgr.Render(tpl.ID, map[string]interface{}{
				"name":  fmt.Sprintf("user-%d", i),
				"place": fmt.Sprintf("zone-%d", i),
			})
			if err != nil {
				return err
			}
			// Verify the substitution actually happened (anti-bluff: a render that
			// silently returns the unsubstituted template is NOT a working render).
			want := fmt.Sprintf("Hello user-%d, welcome to zone-%d.", i, i)
			if out != want {
				return fmt.Errorf("render %d produced %q, want %q", i, out, want)
			}
			return nil
		})

	if rep.N < stresschaos.MinSustainedN {
		t.Fatalf("sustained render N=%d below floor", rep.N)
	}
	t.Logf("template sustained render: N=%d p50=%.3fms p95=%.3fms p99=%.3fms",
		rep.N, rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestTemplate_Stress_SustainedRenderSimple stresses the package-level
// RenderSimple path (no Template object) under sustained load. RenderSimple is a
// pure-substitution path used by callers that do not need validation.
func TestTemplate_Stress_SustainedRenderSimple(t *testing.T) {
	content := "{{a}}/{{b}}/{{c}}"
	rep := stresschaos.RunSustainedLoad(t, "template_sustained_render_simple",
		stresschaos.SustainedConfig{N: 5000},
		func(i int) error {
			out := RenderSimple(content, map[string]interface{}{
				"a": i, "b": i * 2, "c": i * 3,
			})
			want := fmt.Sprintf("%d/%d/%d", i, i*2, i*3)
			if out != want {
				return fmt.Errorf("RenderSimple %d produced %q, want %q", i, out, want)
			}
			return nil
		})
	t.Logf("RenderSimple sustained: N=%d p50=%.3fms p95=%.3fms p99=%.3fms",
		rep.N, rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestTemplate_Stress_ConcurrentRegisterRenderLookup hammers the REAL Manager
// from >=10 goroutines doing concurrent Register / Get / GetByName / GetByType /
// Render / Search / Count. Each goroutine registers its own uniquely-named
// templates (so Register never collides on the duplicate-name guard) then reads
// them back through every accessor. The RWMutex must serialise so there is no
// data race, panic, or deadlock. Run under -race.
func TestTemplate_Stress_ConcurrentRegisterRenderLookup(t *testing.T) {
	mgr := NewManager()

	stresschaos.RunConcurrent(t, "template_concurrent_register_render_lookup",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 80},
		func(g, it int) error {
			name := fmt.Sprintf("tpl-g%d-i%d", g, it)
			tpl := newStressTemplate(name, name)
			if err := mgr.Register(tpl); err != nil {
				return fmt.Errorf("g%d i%d register: %w", g, it, err)
			}

			// Read it back by ID.
			got, err := mgr.Get(tpl.ID)
			if err != nil {
				return fmt.Errorf("g%d i%d get: %w", g, it, err)
			}
			if got.Name != name {
				return fmt.Errorf("g%d i%d get returned wrong template %q", g, it, got.Name)
			}

			// Read by name.
			if _, err := mgr.GetByName(name); err != nil {
				return fmt.Errorf("g%d i%d getByName: %w", g, it, err)
			}

			// Render through the manager.
			out, err := mgr.Render(tpl.ID, map[string]interface{}{"name": name})
			if err != nil {
				return fmt.Errorf("g%d i%d render: %w", g, it, err)
			}
			if !strings.Contains(out, name) {
				return fmt.Errorf("g%d i%d render missing name: %q", g, it, out)
			}

			// Read-only accessors widen the RLock contention surface.
			_ = mgr.GetByType(TypePrompt)
			_ = mgr.Search("stress")
			_ = mgr.Count()
			_ = mgr.CountByType()
			_ = mgr.GetAll()
			return nil
		})
}

// TestTemplate_Stress_ConcurrentCallbackRegistration hammers the Manager's
// callback-registration API (OnCreate/OnUpdate/OnDelete) concurrently with
// Register/Update/Delete. Register reads the m.onCreate slice under the write
// lock while OnCreate appends to it; if OnCreate does not take the lock this is a
// data race (caught under -race). A working Manager must serialise both. The
// callbacks themselves do real atomic work so the dispatch path is exercised.
func TestTemplate_Stress_ConcurrentCallbackRegistration(t *testing.T) {
	mgr := NewManager()
	var created, updated, deleted int64

	stresschaos.RunConcurrent(t, "template_concurrent_callback_registration",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 60},
		func(g, it int) error {
			// Concurrently register callbacks — these append to the manager's
			// callback slices that Register/Update/Delete iterate under lock.
			if it%4 == 0 {
				mgr.OnCreate(func(_ *Template) { atomic.AddInt64(&created, 1) })
			}
			if it%4 == 1 {
				mgr.OnUpdate(func(_ *Template) { atomic.AddInt64(&updated, 1) })
			}
			if it%4 == 2 {
				mgr.OnDelete(func(_ *Template) { atomic.AddInt64(&deleted, 1) })
			}

			name := fmt.Sprintf("cb-g%d-i%d", g, it)
			tpl := newStressTemplate(name, name)
			if err := mgr.Register(tpl); err != nil {
				return fmt.Errorf("g%d i%d register: %w", g, it, err)
			}
			if err := mgr.Update(tpl.ID, func(tt *Template) { tt.Description = "updated" }); err != nil {
				return fmt.Errorf("g%d i%d update: %w", g, it, err)
			}
			if err := mgr.Delete(tpl.ID); err != nil {
				return fmt.Errorf("g%d i%d delete: %w", g, it, err)
			}
			return nil
		})

	t.Logf("callback dispatch totals: created=%d updated=%d deleted=%d",
		atomic.LoadInt64(&created), atomic.LoadInt64(&updated), atomic.LoadInt64(&deleted))
}

// TestTemplate_Stress_Boundaries exercises boundary-condition inputs against the
// REAL render / validate / extract paths. Each boundary is categorised: rejected
// (error) or accepted (clean output) — but NEVER a panic and NEVER a wrong result.
func TestTemplate_Stress_Boundaries(t *testing.T) {
	t.Run("empty_content_render", func(t *testing.T) {
		tpl := NewTemplate("empty", "", TypeCustom)
		tpl.SetContent("")
		// No placeholders, no required vars -> render of empty content is empty.
		out, err := tpl.Render(map[string]interface{}{})
		if err != nil {
			t.Fatalf("empty-content render errored: %v", err)
		}
		if out != "" {
			t.Fatalf("empty-content render produced %q, want empty", out)
		}
	})

	t.Run("missing_required_key_rejected", func(t *testing.T) {
		tpl := newStressTemplate("missing", "Missing Key")
		// {{name}} is required but not supplied -> must reject, not panic.
		_, err := tpl.Render(map[string]interface{}{"place": "x"})
		if err == nil {
			t.Fatal("render with missing required key must error, got nil")
		}
	})

	t.Run("unreplaced_placeholder_rejected", func(t *testing.T) {
		// A template whose content has an undeclared placeholder must be rejected
		// at render (hasUnreplacedPlaceholders), not silently emit "{{x}}".
		tpl := NewTemplate("unreplaced", "", TypeCustom)
		tpl.SetContent("value is {{undeclared}}")
		_, err := tpl.Render(map[string]interface{}{})
		if err == nil {
			t.Fatal("render with unreplaced placeholder must error, got nil")
		}
	})

	t.Run("many_templates_registered", func(t *testing.T) {
		mgr := NewManager()
		const n = 5000
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("many-%d", i)
			if err := mgr.Register(newStressTemplate(name, name)); err != nil {
				t.Fatalf("register %d: %v", i, err)
			}
		}
		if got := mgr.Count(); got != n {
			t.Fatalf("count=%d, want %d", got, n)
		}
		// Lookups across the large index must still work.
		if _, err := mgr.GetByName("many-4999"); err != nil {
			t.Fatalf("getByName on large index: %v", err)
		}
		if len(mgr.GetByType(TypePrompt)) != n {
			t.Fatalf("GetByType returned %d, want %d", len(mgr.GetByType(TypePrompt)), n)
		}
	})

	t.Run("huge_content_render", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < 100000; i++ {
			b.WriteString("x")
		}
		b.WriteString("{{name}}")
		tpl := NewTemplate("huge", "", TypeCustom)
		tpl.SetContent(b.String())
		tpl.AddVariable(Variable{Name: "name", Required: true, Type: "string"})
		out, err := tpl.Render(map[string]interface{}{"name": "END"})
		if err != nil {
			t.Fatalf("huge-content render errored: %v", err)
		}
		if !strings.HasSuffix(out, "END") {
			t.Fatal("huge-content render did not substitute trailing placeholder")
		}
	})

	t.Run("extract_variables_dedup", func(t *testing.T) {
		tpl := NewTemplate("extract", "", TypeCustom)
		tpl.SetContent("{{a}}{{b}}{{a}}{{c}}{{b}}")
		vars := tpl.ExtractVariables()
		if len(vars) != 3 {
			t.Fatalf("ExtractVariables returned %d vars (%v), want 3 deduped", len(vars), vars)
		}
	})

	_ = time.Now
}
