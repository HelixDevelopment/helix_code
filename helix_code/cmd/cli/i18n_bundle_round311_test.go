// CONST-046 round-311 §11.4 — anti-bluff parity guard. Asserts that the
// round311BundleText map (used by unit tests to wire a realistic
// Translator) is byte-for-byte consistent with the on-disk
// i18n/bundles/active.en.yaml `other:` values. Without this guard a
// future edit to the bundle would silently leave the test map stale and
// unit assertions would pass against text users never actually see.
//
// Mocks ALLOWED here per CONST-050(A) — unit-test-only file.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseBundleOther does a deliberately minimal YAML scan of
// active.en.yaml: it looks for a top-level `<id>:` line immediately
// followed by an `  other: "<value>"` line and records the unquoted
// value. It does NOT handle the block-scalar (`|-`) entries — those are
// not in round311BundleText, so the scan covers exactly the simple
// single-line `other:` entries the test map mirrors.
func parseBundleOther(t *testing.T) map[string]string {
	t.Helper()
	bundlePath := filepath.Join("i18n", "bundles", "active.en.yaml")
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read bundle %s: %v", bundlePath, err)
	}
	lines := strings.Split(string(raw), "\n")
	out := make(map[string]string)
	for i := 0; i+1 < len(lines); i++ {
		idLine := lines[i]
		// Top-level key: no leading whitespace, ends with ':'.
		if idLine == "" || idLine[0] == ' ' || idLine[0] == '#' {
			continue
		}
		if !strings.HasSuffix(strings.TrimRight(idLine, " "), ":") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimRight(idLine, " "), ":")
		next := lines[i+1]
		trimmed := strings.TrimSpace(next)
		if !strings.HasPrefix(trimmed, "other: \"") {
			continue
		}
		val := strings.TrimPrefix(trimmed, "other: \"")
		val = strings.TrimSuffix(val, "\"")
		out[id] = val
	}
	return out
}

func TestRound311Translator_MatchesBundle(t *testing.T) {
	bundle := parseBundleOther(t)
	if len(bundle) == 0 {
		t.Fatal("parsed zero single-line `other:` entries from active.en.yaml — parser is broken")
	}
	for id, want := range round311BundleText {
		got, ok := bundle[id]
		if !ok {
			t.Errorf("round311BundleText[%q] has no matching single-line entry in active.en.yaml", id)
			continue
		}
		if got != want {
			t.Errorf("round311BundleText[%q] drift:\n  test map: %q\n  bundle:   %q", id, want, got)
		}
	}
}

func TestRound311Translator_AllIDsResolveNonEmpty(t *testing.T) {
	// Anti-bluff: every ID in the test map MUST resolve through the
	// translator to a non-empty, non-ID string.
	tr := round311TestTranslator{}
	for id := range round311BundleText {
		got, err := tr.T(t.Context(), id, nil)
		if err != nil {
			t.Errorf("round311TestTranslator.T(%q) errored: %v", id, err)
			continue
		}
		if got == "" {
			t.Errorf("round311TestTranslator.T(%q) returned empty string", id)
		}
		if got == id {
			t.Errorf("round311TestTranslator.T(%q) echoed the raw ID — bundle text missing", id)
		}
	}
}
