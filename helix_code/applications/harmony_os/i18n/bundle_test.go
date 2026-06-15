// Regression guard for HXC-097 (harmony_os nogui+GUI CLI leaked raw
// i18n message keys). Root cause: the standalone harmony_os binary
// never wired a real Translator — it fell back to NoopTranslator{}
// loud-echo, so every command (help/status/version/...) printed the
// bare message ID instead of resolved prose. bundle.go's
// NewTranslator() + main()'s SetTranslator wiring close the gap.
//
// This test exercises the REAL translator built from the embedded
// active.en.yaml bundle (no cgo/Fyne toolchain needed). It is the
// §11.4.115/§11.4.135 standing GREEN guard: a status/help message ID
// that does NOT resolve (echoes its own key) FAILs the test —
// reproducing exactly the HXC-097 defect on any regression.
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope only).
package i18n

import (
	"context"
	"strings"
	"testing"
)

// hxc097ScopeIDs is the closed set of nogui-CLI message IDs whose
// raw-key leakage was the HXC-097 defect (status header + every
// status-line template + the help body + unknown-command error).
// Each MUST resolve to a non-empty translation that is NOT the bare
// key itself.
var hxc097ScopeIDs = []string{
	"harmony_os_cli_help_body",
	"harmony_os_cli_status_header",
	"harmony_os_cli_status_platform",
	"harmony_os_cli_status_cpu_cores",
	"harmony_os_cli_status_go_version",
	"harmony_os_cli_status_workers",
	"harmony_os_cli_status_tasks",
	"harmony_os_cli_status_projects",
	"harmony_os_cli_status_sessions",
	"harmony_os_cli_status_llm_models",
	"harmony_os_cli_unknown_command",
}

// TestNewTranslator_ResolvesHXC097Scope proves the boot-time
// translator resolves every HXC-097-scope key to real prose — the
// regression guard for the raw-key leak.
func TestNewTranslator_ResolvesHXC097Scope(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator() failed: %v", err)
	}
	for _, id := range hxc097ScopeIDs {
		got, terr := tr.T(context.Background(), id, nil)
		if terr != nil {
			t.Fatalf("T(%q) returned error: %v", id, terr)
		}
		if got == "" {
			t.Fatalf("T(%q) returned empty string", id)
		}
		if got == id {
			t.Fatalf("T(%q) echoed the raw message ID — HXC-097 regression (translator not resolving)", id)
		}
	}
}

// TestNewTranslator_StatusTemplatesRenderNamedData proves the
// status-line templates carry the EXACT named placeholders the call
// sites in main_nogui.go pass data for, and that the real translator
// substitutes them (no `<no value>` and no residual `{{.Field}}`
// literal). The harmony_os bundle uses go-i18n named templates
// ({{.Total}}, {{.Active}}, ...) rather than Printf % verbs, so this
// renders each template with concrete data and asserts the values
// appear in the output. A template/call-site placeholder mismatch
// surfaces as `<no value>` (go-template) — caught here.
func TestNewTranslator_StatusTemplatesRenderNamedData(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator() failed: %v", err)
	}
	// id -> the named data the call site in main_nogui.go passes, with
	// distinctive sentinel values we then assert appear in the rendered
	// output. Keys/values mirror cmdStatus() exactly.
	cases := []struct {
		id   string
		data map[string]any
		want []string
	}{
		{"harmony_os_cli_status_platform", map[string]any{"OSName": "HarmonyOS", "OSArch": "arm64"}, []string{"HarmonyOS", "arm64"}},
		{"harmony_os_cli_status_cpu_cores", map[string]any{"Cores": 8}, []string{"8"}},
		{"harmony_os_cli_status_workers", map[string]any{"Total": 3, "Active": 2}, []string{"3", "2"}},
		{"harmony_os_cli_status_tasks", map[string]any{"Total": 10, "Running": 4, "Completed": 6}, []string{"10", "4", "6"}},
		{"harmony_os_cli_status_projects", map[string]any{"Total": 5, "Active": "demo"}, []string{"5", "demo"}},
		{"harmony_os_cli_status_sessions", map[string]any{"Total": 7, "Active": 1}, []string{"7", "1"}},
		{"harmony_os_cli_status_llm_models", map[string]any{"Count": 12}, []string{"12"}},
		{"harmony_os_cli_unknown_command", map[string]any{"Command": "frobnicate"}, []string{"frobnicate"}},
	}
	for _, c := range cases {
		got, terr := tr.T(context.Background(), c.id, c.data)
		if terr != nil {
			t.Fatalf("T(%q) returned error: %v", c.id, terr)
		}
		if got == c.id || got == "" {
			t.Fatalf("T(%q) did not resolve (got %q) — HXC-097 regression", c.id, got)
		}
		if strings.Contains(got, "<no value>") {
			t.Fatalf("T(%q) = %q contains <no value> — template/call-site named-placeholder mismatch", c.id, got)
		}
		if strings.Contains(got, "{{") {
			t.Fatalf("T(%q) = %q left an unrendered {{...}} template literal — bundle template not substituted", c.id, got)
		}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Fatalf("T(%q) = %q missing rendered value %q (named placeholder not substituted)", c.id, got, w)
			}
		}
	}
}
