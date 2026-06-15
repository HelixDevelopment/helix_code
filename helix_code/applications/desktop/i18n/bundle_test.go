// Regression guard for HXC-095 (desktop nogui CLI leaked raw i18n
// message keys + emitted %!(EXTRA ...) noise). Root cause: the
// standalone desktop binary never wired a real Translator — it fell
// back to NoopTranslator{} loud-echo, so format-string call sites
// printed the bare message ID (no % verbs) and Go reported the
// passed args as EXTRA. bundle.go's NewTranslator() closes the gap.
//
// This test exercises the REAL translator built from the embedded
// active.en.yaml bundle (no cgo/Fyne toolchain needed). It is the
// §11.4.115/§11.4.135 standing GREEN guard: a status/help message ID
// that does NOT resolve (echoes its own key) FAILs the test —
// reproducing exactly the HXC-095 defect on any regression.
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope only).
package i18n

import (
	"context"
	"strings"
	"testing"
)

// hxc095ScopeIDs is the closed set of nogui-CLI message IDs whose
// raw-key leakage was the HXC-095 defect (status header + every
// status-line format string + the help body). Each MUST resolve to a
// non-empty translation that is NOT the bare key itself.
var hxc095ScopeIDs = []string{
	"desktop_cli_help_body",
	"desktop_cli_status_header",
	"desktop_cli_status_workers",
	"desktop_cli_status_tasks",
	"desktop_cli_status_projects",
	"desktop_cli_status_sessions",
	"desktop_cli_status_llm_models",
}

// TestNewTranslator_ResolvesHXC095Scope proves the boot-time
// translator resolves every HXC-095-scope key to real prose — the
// regression guard for the raw-key leak.
func TestNewTranslator_ResolvesHXC095Scope(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator() failed: %v", err)
	}
	for _, id := range hxc095ScopeIDs {
		got, terr := tr.T(context.Background(), id, nil)
		if terr != nil {
			t.Fatalf("T(%q) returned error: %v", id, terr)
		}
		if got == "" {
			t.Fatalf("T(%q) returned empty string", id)
		}
		if got == id {
			t.Fatalf("T(%q) echoed the raw message ID — HXC-095 regression (translator not resolving)", id)
		}
	}
}

// TestNewTranslator_StatusFormatVerbsMatch proves the status-line
// format strings carry the EXACT % verb count the call sites in
// main_nogui.go pass args for. A verb-count mismatch is what
// produced the %!(EXTRA ...) noise once the key resolved. This pins
// the bundle's format strings to the call-site arity.
func TestNewTranslator_StatusFormatVerbsMatch(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator() failed: %v", err)
	}
	// id -> expected number of format verbs (matches main_nogui.go args).
	want := map[string]int{
		"desktop_cli_status_workers":    2, // len(workers), activeWorkers
		"desktop_cli_status_tasks":      3, // total, running, completed
		"desktop_cli_status_projects":   2, // len(projects), activeProjectName
		"desktop_cli_status_sessions":   2, // len(sessions), activeSessions
		"desktop_cli_status_llm_models": 1, // len(models)
	}
	for id, n := range want {
		got, terr := tr.T(context.Background(), id, nil)
		if terr != nil {
			t.Fatalf("T(%q) returned error: %v", id, terr)
		}
		// Count `%` verbs excluding literal `%%` escapes.
		verbs := strings.Count(got, "%") - 2*strings.Count(got, "%%")
		if verbs != n {
			t.Fatalf("T(%q) = %q has %d format verbs, want %d (call-site arity mismatch → %%!(EXTRA) noise)", id, got, verbs, n)
		}
	}
}
