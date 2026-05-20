//go:build nogui

// CONST-046 sentinel tests for the desktop application's nogui-mode
// CLI (CLIApp). Round-365 §11.4 anti-bluff sweep added the
// Translator seam (SetTranslator + t() helper) to CLIApp, mirroring
// the aurora_os nogui pattern from round-327.
//
// These tests assert the seam actually routes every migrated
// message ID through the injected Translator — catching regressions
// where a future edit reverts a call site back to a literal string
// (which would silently re-introduce the §11.4 PASS-bluff this
// round closes).
//
// Mocks ALLOWED per CONST-050(A): unit tests only.
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/applications/desktop/i18n"
	"gopkg.in/yaml.v3"
)

// cliFakeTranslator is a unit-test-only translator (CONST-050(A):
// fakes permitted in *_test.go). calls captures the message IDs the
// CLI resolves so the paired-mutation tests below can assert the
// seam actually routes through Translator.T rather than echoing a
// literal.
type cliFakeTranslator struct {
	prefix string
	fail   bool
	calls  []string
}

func (f *cliFakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	f.calls = append(f.calls, id)
	if f.fail {
		return "", errors.New("translate failed")
	}
	return f.prefix + id, nil
}

func (f *cliFakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	f.calls = append(f.calls, id)
	if f.fail {
		return "", errors.New("translate failed")
	}
	return f.prefix + id, nil
}

// TestCLIAppTranslatorDefault verifies NewCLIApp installs a non-nil
// NoopTranslator (loud-echo safety net) per CONST-046 round-365.
func TestCLIAppTranslatorDefault(t *testing.T) {
	app := NewCLIApp()
	if app.translator == nil {
		t.Fatal("NewCLIApp left translator nil; want NoopTranslator default")
	}
	if got := app.t("desktop_cli_status_header"); got != "desktop_cli_status_header" {
		t.Fatalf("NoopTranslator default produced %q, want loud-echo %q",
			got, "desktop_cli_status_header")
	}
}

// TestCLIAppSetTranslator is the positive case: a wired translator
// IS consulted and its output replaces the message ID.
func TestCLIAppSetTranslator(t *testing.T) {
	app := NewCLIApp()
	ft := &cliFakeTranslator{prefix: "XL:"}
	app.SetTranslator(ft)

	got := app.t("desktop_cli_help_body")
	if got != "XL:desktop_cli_help_body" {
		t.Fatalf("app.t returned %q, want %q", got, "XL:desktop_cli_help_body")
	}
	if len(ft.calls) != 1 || ft.calls[0] != "desktop_cli_help_body" {
		t.Fatalf("Translator.T not consulted; calls=%v", ft.calls)
	}
}

// TestCLIAppSetTranslatorNilNoop is the paired-mutation guard:
// passing nil MUST NOT clear the NoopTranslator default — the
// loud-echo safety net must never disappear silently.
func TestCLIAppSetTranslatorNilNoop(t *testing.T) {
	app := NewCLIApp()
	ft := &cliFakeTranslator{prefix: "XL:"}
	app.SetTranslator(ft)
	app.SetTranslator(nil) // no-op — must NOT wipe ft

	got := app.t("desktop_cli_status_header")
	if got != "XL:desktop_cli_status_header" {
		t.Fatalf("SetTranslator(nil) must be a no-op, not a reset; got %q", got)
	}
}

// TestCLIAppTranslatorFallbackOnError is the paired-mutation guard
// for the error path: when Translator.T returns an error the helper
// MUST fall back to the literal message ID (loud echo), never an
// empty string.
func TestCLIAppTranslatorFallbackOnError(t *testing.T) {
	app := NewCLIApp()
	app.SetTranslator(&cliFakeTranslator{fail: true})

	got := app.t("desktop_cli_no_projects")
	if got != "desktop_cli_no_projects" {
		t.Fatalf("on translate error the helper must echo the message ID; got %q", got)
	}
}

// TestCLIAppTranslatorNoopType confirms i18n.NoopTranslator
// satisfies the i18n.Translator contract used by SetTranslator.
func TestCLIAppTranslatorNoopType(t *testing.T) {
	var _ i18n.Translator = i18n.NoopTranslator{}
	app := NewCLIApp()
	app.SetTranslator(i18n.NoopTranslator{})
	if got := app.t("desktop_cli_sessions_header"); got != "desktop_cli_sessions_header" {
		t.Fatalf("NoopTranslator must echo the ID; got %q", got)
	}
}

// round365IDs is the closed set of message IDs migrated in the
// round-365 §11.4 sweep of desktop/main_nogui.go.
var round365IDs = []string{
	"desktop_cli_unknown_command",
	"desktop_cli_unknown_subcommand",
	"desktop_cli_help_body",
	"desktop_cli_status_header",
	"desktop_cli_status_workers",
	"desktop_cli_status_tasks",
	"desktop_cli_status_projects",
	"desktop_cli_status_sessions",
	"desktop_cli_status_llm_models",
	"desktop_cli_projects_header",
	"desktop_cli_no_projects",
	"desktop_cli_err_name_path_required",
	"desktop_cli_created_project",
	"desktop_cli_err_project_id_required",
	"desktop_cli_set_active_project",
	"desktop_cli_deleted_project",
	"desktop_cli_sessions_header",
	"desktop_cli_no_sessions",
	"desktop_cli_err_name_project_required",
	"desktop_cli_created_session",
	"desktop_cli_err_session_id_required",
	"desktop_cli_started_session",
	"desktop_cli_paused_session",
	"desktop_cli_completed_session",
}

// TestCLIAppRound365IDsResolveThroughTranslator is the positive
// case for the round-365 §11.4 residual migration: every newly
// migrated message ID MUST route through Translator.T (not echo a
// literal).
func TestCLIAppRound365IDsResolveThroughTranslator(t *testing.T) {
	app := NewCLIApp()
	ft := &cliFakeTranslator{prefix: "R365:"}
	app.SetTranslator(ft)

	for _, id := range round365IDs {
		got := app.t(id)
		if got != "R365:"+id {
			t.Fatalf("id %q must route through Translator.T; got %q", id, got)
		}
	}
	if len(ft.calls) != len(round365IDs) {
		t.Fatalf("Translator consulted %d times, want %d", len(ft.calls), len(round365IDs))
	}
	for i, id := range round365IDs {
		if ft.calls[i] != id {
			t.Fatalf("calls[%d]=%q, want %q", i, ft.calls[i], id)
		}
	}
}

// TestCLIAppRound365FallbackOnError is the paired-mutation guard for
// the round-365 IDs: on translate error the helper MUST echo the
// literal message ID (loud echo), never an empty string.
func TestCLIAppRound365FallbackOnError(t *testing.T) {
	app := NewCLIApp()
	app.SetTranslator(&cliFakeTranslator{fail: true})

	for _, id := range round365IDs {
		if got := app.t(id); got != id {
			t.Fatalf("on translate error id %q must echo verbatim; got %q", id, got)
		}
	}
}

// TestCLIAppNilTranslatorAutoHeals verifies that if the translator
// field is somehow nil at call time (e.g. caller built CLIApp{}
// directly without NewCLIApp), the t() helper MUST auto-heal to
// NoopTranslator{} rather than panic.
func TestCLIAppNilTranslatorAutoHeals(t *testing.T) {
	app := &CLIApp{} // translator field deliberately left zero
	got := app.t("desktop_cli_status_header")
	if got != "desktop_cli_status_header" {
		t.Fatalf("nil-translator t() returned %q, want loud-echo", got)
	}
	if _, ok := app.translator.(i18n.NoopTranslator); !ok {
		t.Fatalf("t() failed to auto-heal nil translator; got %T", app.translator)
	}
}

// round446IDs is the closed set of message IDs migrated in the
// round-446 §11.4 sweep of desktop/main_nogui.go (tasks/workers/llm
// subcommand surfaces + interactive-mode banner/prompts). Completes
// the desktop nogui CLI hardcoded-content migration begun in
// round-365.
var round446IDs = []string{
	"desktop_cli_tasks_header",
	"desktop_cli_no_tasks",
	"desktop_cli_err_desc_required",
	"desktop_cli_created_task",
	"desktop_cli_err_task_id_required",
	"desktop_cli_cancelled_task",
	"desktop_cli_workers_header",
	"desktop_cli_no_workers",
	"desktop_cli_err_host_required",
	"desktop_cli_added_worker",
	"desktop_cli_err_worker_id_required",
	"desktop_cli_removed_worker",
	"desktop_cli_llm_providers_header",
	"desktop_cli_no_providers",
	"desktop_cli_available_models_header",
	"desktop_cli_no_models",
	"desktop_cli_chat_requires_provider",
	"desktop_cli_chat_configure_hint",
	"desktop_cli_interactive_header",
	"desktop_cli_interactive_hint",
	"desktop_cli_exiting",
	"desktop_cli_goodbye",
	"desktop_cli_error_prefix",
}

// TestCLIAppRound446IDsResolveThroughTranslator is the positive case
// for the round-446 §11.4 residual migration: every newly migrated
// message ID MUST route through Translator.T (not echo a literal).
func TestCLIAppRound446IDsResolveThroughTranslator(t *testing.T) {
	app := NewCLIApp()
	ft := &cliFakeTranslator{prefix: "R446:"}
	app.SetTranslator(ft)

	for _, id := range round446IDs {
		got := app.t(id)
		if got != "R446:"+id {
			t.Fatalf("id %q must route through Translator.T; got %q", id, got)
		}
	}
	if len(ft.calls) != len(round446IDs) {
		t.Fatalf("Translator consulted %d times, want %d", len(ft.calls), len(round446IDs))
	}
}

// TestCLIAppRound446FallbackOnError is the paired-mutation guard for
// the round-446 IDs: on translate error the helper MUST echo the
// literal message ID (loud echo), never an empty string.
func TestCLIAppRound446FallbackOnError(t *testing.T) {
	app := NewCLIApp()
	app.SetTranslator(&cliFakeTranslator{fail: true})

	for _, id := range round446IDs {
		if got := app.t(id); got != id {
			t.Fatalf("on translate error id %q must echo verbatim; got %q", id, got)
		}
	}
}

// TestRound446BundleEntriesPresent parses the active.en.yaml bundle
// directly (no X11/Fyne toolchain required) and asserts every
// round-446 message ID has a non-empty "other" translation. A
// missing entry would make the i18nadapter-wired Translator fall
// back to a loud echo at runtime — a degraded user experience that
// the CONST-046 migration exists to prevent.
func TestRound446BundleEntriesPresent(t *testing.T) {
	bundlePath := filepath.Join("i18n", "bundles", "active.en.yaml")
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read bundle %s: %v", bundlePath, err)
	}

	var entries map[string]struct {
		Other string `yaml:"other"`
	}
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("parse bundle YAML: %v", err)
	}

	for _, id := range round446IDs {
		e, ok := entries[id]
		if !ok {
			t.Errorf("round-446 message ID %q missing from %s", id, bundlePath)
			continue
		}
		if strings.TrimSpace(e.Other) == "" {
			t.Errorf("round-446 message ID %q has empty 'other' translation", id)
		}
	}
}

// TestRound446BundleParityWithCallSites is the paired-mutation guard
// at the source layer: it asserts every round-446 message ID is
// actually referenced in main_nogui.go. If a future edit reverts a
// call site to a hardcoded literal, the ID drops out of the source
// and this test FAILs — catching the §11.4 PASS-bluff regression.
func TestRound446BundleParityWithCallSites(t *testing.T) {
	src, err := os.ReadFile("main_nogui.go")
	if err != nil {
		t.Fatalf("read main_nogui.go: %v", err)
	}
	body := string(src)
	for _, id := range round446IDs {
		if !strings.Contains(body, `"`+id+`"`) {
			t.Errorf("round-446 message ID %q no longer referenced in main_nogui.go "+
				"(call site reverted to a hardcoded literal?)", id)
		}
	}
}
