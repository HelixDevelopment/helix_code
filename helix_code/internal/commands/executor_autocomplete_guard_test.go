package commands

// Standing regression guard (§11.4.135) for the Executor.Autocomplete
// exact-full-match defect, with §11.4.115 RED_MODE polarity.
//
// The defect (executor.go, pre-fix): the prefix test used
//
//	len(partial) < len(name) && name[:len(partial)] == partial
//
// Due to `<` (instead of `<=`), when the operator fully typed a registered
// command name (len(partial) == len(name)) the exact match was dropped — so
// `Autocomplete("/test")` with a registered "test" returned nothing,
// implying to the operator that the command does not exist. A name is a
// valid completion of itself; the fix uses `<=`.
//
// Polarity (§11.4.115):
//   - RED_MODE=1 : inline the unguarded PRE-FIX algorithm (`<`) inside the
//     test and assert it reproduces the defect (exact match dropped). This
//     proves the guard catches a real defect, not a synthetic one.
//   - RED_MODE=0 (default / no env) : drive the REAL fixed Executor.Autocomplete
//     and assert the safe outcome (exact full match IS offered). This is the
//     standing GREEN regression guard.
//
// Paired-mutation proof: revert executor.go's `<=` back to `<` and the
// default (GREEN) test FAILS; restore and it PASSES.

import (
	"os"
	"testing"
)

// autocompletePreFix is a faithful inline copy of the PRE-FIX Autocomplete
// name-matching loop (the `<` form). It exists ONLY so RED_MODE=1 can
// reproduce the historical defect against a faithful stand-in without
// reaching into production code. It must mirror the pre-fix algorithm
// exactly (leading '/' strip + `<` prefix test).
func autocompletePreFix(partial string, names []string) []string {
	if len(partial) == 0 || partial[0] != '/' {
		return nil
	}
	partial = partial[1:]
	matches := make([]string, 0)
	for _, name := range names {
		if partial == "" || len(partial) < len(name) && name[:len(partial)] == partial {
			matches = append(matches, "/"+name)
		}
	}
	return matches
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestAutocomplete_ExactFullMatch_Guard is the standing regression guard.
func TestAutocomplete_ExactFullMatch_Guard(t *testing.T) {
	const names0, names1 = "test", "task"

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the defect on the faithful pre-fix stand-in: a fully-typed
		// registered name "/test" is dropped (the bug). This run PASSES,
		// proving the defect is real and the guard targets it.
		got := autocompletePreFix("/test", []string{names0, names1})
		if containsStr(got, "/test") {
			t.Fatalf("RED_MODE: expected pre-fix algorithm to DROP exact match /test, but it was present: %v", got)
		}
		t.Logf("RED_MODE reproduced defect: pre-fix Autocomplete(/test) = %v (exact match dropped)", got)
		return
	}

	// Default GREEN guard: drive the REAL fixed Executor.Autocomplete.
	registry := NewRegistry()
	executor := NewExecutor(registry)
	if err := registry.Register(&MockCommand{name: names0}); err != nil {
		t.Fatalf("register %s: %v", names0, err)
	}
	if err := registry.Register(&MockCommand{name: names1}); err != nil {
		t.Fatalf("register %s: %v", names1, err)
	}

	// A fully-typed registered command name MUST be offered (a name is a
	// valid completion of itself).
	got := executor.Autocomplete("/test")
	if !containsStr(got, "/test") {
		t.Fatalf("exact full match /test must be offered by Autocomplete, got %v", got)
	}

	// Regression guard must not over-correct: a non-matching full-length token
	// must NOT be offered, and a proper prefix still works.
	if g := executor.Autocomplete("/te"); !containsStr(g, "/test") {
		t.Fatalf("prefix /te must still offer /test, got %v", g)
	}
	if g := executor.Autocomplete("/zzzz"); len(g) != 0 {
		t.Fatalf("non-matching /zzzz must offer nothing, got %v", g)
	}
}
