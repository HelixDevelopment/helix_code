// render_injection_security_test.go — standing regression guard (§11.4.135)
// for the template-injection / map-iteration-order-nondeterminism defect in
// Template.Render / RenderSimple.
//
// Defect (reproduced 2026-06-15): substitution iterated over the variable map
// applying strings.ReplaceAll per key, so a variable VALUE that contained a
// "{{other}}" sequence was re-scanned and the unrelated variable expanded into
// it — a template-injection. Because Go randomises map-iteration order, the SAME
// inputs produced different outcomes run-to-run (≈78% injection-expanded,
// ≈22% spurious unreplaced-placeholder error), a §11.4.85(B) non-determinism +
// §11.4.50 reproducibility violation.
//
// Fix: substitutePlaceholders scans the content exactly once, left to right, so
// already-written values are never re-examined — injection is impossible and the
// output is order-independent.
//
// §11.4.115 polarity: RED_MODE=1 inlines a faithful stand-in of the UNGUARDED
// pre-fix substitution (iterate map + ReplaceAll) and asserts the injection
// SUCCEEDS at least once across many trials (proving the defect is real on the
// stand-in). RED_MODE=0 drives the REAL fixed Render/RenderSimple and asserts the
// safe, deterministic outcome.
package template

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// redMode reports whether the RED reproduction polarity is active.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// unguardedSubstitute is a faithful reproduction of the pre-fix substitution loop
// from Template.Render / RenderSimple: iterate the variable map and ReplaceAll each
// "{{key}}" placeholder. Used ONLY by the RED_MODE=1 reproduction to demonstrate the
// historical injection defect; the production code no longer contains this pattern.
func unguardedSubstitute(content string, vars map[string]interface{}) string {
	result := content
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(value))
	}
	return result
}

func TestRenderInjection_VariableValuePlaceholderNotExpanded(t *testing.T) {
	const content = "A={{a}} B={{b}}"
	vars := map[string]interface{}{"a": "{{b}}", "b": "SECRET"}

	if redMode() {
		// RED: the unguarded pre-fix loop expands the injected {{b}} into a's
		// slot for at least some map orderings. Run enough trials to hit it.
		injected := false
		for i := 0; i < 2000; i++ {
			out := unguardedSubstitute(content, vars)
			if out == "A=SECRET B=SECRET" {
				injected = true
				break
			}
		}
		if !injected {
			t.Fatalf("RED_MODE: expected the unguarded substitution to expand the injected {{b}} at least once, but it never did — reproduction stand-in is not faithful")
		}
		t.Logf("RED_MODE: confirmed unguarded substitution expands injected placeholder (template injection)")
		return
	}

	// GREEN: the real fixed code emits a's value verbatim — the injected {{b}}
	// is NOT expanded — and the surviving literal placeholder makes Render error
	// via the unreplaced-placeholder check. Outcome is identical across runs.
	tpl := NewTemplate("inj", "", TypeCustom)
	tpl.SetContent(content)
	tpl.AddVariable(Variable{Name: "a", Required: true, Type: "string"})
	tpl.AddVariable(Variable{Name: "b", Required: true, Type: "string"})

	var firstErr string
	for i := 0; i < 500; i++ {
		out, err := tpl.Render(vars)
		if err == nil {
			t.Fatalf("iter %d: expected error (literal {{b}} from a's value is unreplaced), got output %q — injection not prevented", i, out)
		}
		if i == 0 {
			firstErr = err.Error()
		} else if err.Error() != firstErr {
			t.Fatalf("iter %d: non-deterministic error %q vs first %q", i, err.Error(), firstErr)
		}
	}
}

func TestRenderSimpleInjection_ValueVerbatim(t *testing.T) {
	const content = "X={{x}}|Y={{y}}"
	vars := map[string]interface{}{"x": "{{y}}", "y": "PWNED"}

	if redMode() {
		injected := false
		for i := 0; i < 2000; i++ {
			if unguardedSubstitute(content, vars) == "X=PWNED|Y=PWNED" {
				injected = true
				break
			}
		}
		if !injected {
			t.Fatalf("RED_MODE: expected unguarded RenderSimple stand-in to inject at least once")
		}
		return
	}

	// GREEN: deterministic — x's value is emitted verbatim, never expanded.
	want := "X={{y}}|Y=PWNED"
	for i := 0; i < 500; i++ {
		if got := RenderSimple(content, vars); got != want {
			t.Fatalf("iter %d: got %q want %q — injection or non-determinism", i, got, want)
		}
	}
}

// TestRenderInjection_NoInjectionWithUnknownVar proves the fix does not break the
// legitimate case where a value contains a placeholder for a variable that is not
// declared/provided: the value is emitted verbatim (no panic, no expansion).
func TestRenderInjection_BenignValuePreserved(t *testing.T) {
	// A value that contains literal "{{ }}"-like text but no matching var name.
	got := RenderSimple("greeting: {{msg}}", map[string]interface{}{
		"msg": "use {{handlebars}} syntax in docs",
	})
	want := "greeting: use {{handlebars}} syntax in docs"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
