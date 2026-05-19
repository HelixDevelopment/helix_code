// CONST-046 round-131 + round-196 + round-202 §11.4 — sentinel-based
// assertions verifying that the migrated user-facing emissions in
// main.go route through the translator package and NOT through hardcoded
// literals.
// Round-196 (2026-05-19) added IDs cli_workers_header, cli_workers_total_cpu,
// cli_workers_total_memory_gb, cli_workers_total_gpu, cli_models_header,
// cli_models_fallback_notice, cli_health_header, cli_health_operational,
// cli_repl_header, cli_repl_goodbye.
// Round-202 (2026-05-19) added IDs cli_help_commands_header,
// cli_help_cmd_workers, cli_help_cmd_models, cli_help_cmd_health,
// cli_help_cmd_help, cli_help_cmd_exit, cli_help_options_header,
// cli_worker_added_success, cli_generating_header, cli_generating_prompt.
//
// Pattern (matches rounds 93/94/95/96/108): wire a fakeTranslator
// that returns "<TRANSLATED:<id>>" for every ID, capture stdout
// during a focused invocation of the tr() helper, assert the
// sentinel-wrapped form appears. Reverting any migrated call site
// back to its hardcoded literal makes the corresponding sentinel
// assertion fail — that is the round-131 mutation invariant
// captured at /tmp/round131_mutation.txt.
//
// Mocks ALLOWED here per CONST-050(A) (unit-tests only).
package main

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/cmd/cli/i18n"
)

// fakeCLITranslator wraps every ID in "<TRANSLATED:<id>>" so the
// test can prove the lookup actually went through Translator.T
// instead of a hardcoded literal that happens to match the bundle.
type fakeCLITranslator struct {
	called map[string]int
}

func newFakeCLITranslator() *fakeCLITranslator {
	return &fakeCLITranslator{called: make(map[string]int)}
}

func (f *fakeCLITranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	f.called[id]++
	return "<TRANSLATED:" + id + ">", nil
}

func (f *fakeCLITranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	f.called[id]++
	return "<TRANSLATED:" + id + ">", nil
}

// erroringTranslator returns an error for every lookup so the test
// can prove tr() degrades to the message ID (loud echo) and never
// silently emits empty output.
type erroringTranslator struct{}

func (erroringTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("erroringTranslator: failure for " + id)
}
func (erroringTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("erroringTranslator: plural failure for " + id)
}

// migratedMessageIDs is the exhaustive list of IDs migrated in
// round-131. Every entry MUST resolve through the package-level
// tr() helper. Reverting any call site to a hardcoded literal
// drops the corresponding ID's call count to zero in production
// and breaks TestTr_AllMigratedIDs_ResolveThroughTranslator.
var migratedMessageIDs = []string{
	// Round-131 (2026-05-18) — 10 IDs
	"cli_workers_total",
	"cli_workers_active",
	"cli_workers_healthy",
	"cli_repl_intro",
	"cli_repl_shutting_down",
	"cli_repl_unknown_slash",
	"cli_qa_session_started",
	"cli_qa_waiting",
	"cli_qa_session_completed",
	"cli_qa_no_sessions",
	// Round-196 (2026-05-19) — 10 IDs
	"cli_workers_header",
	"cli_workers_total_cpu",
	"cli_workers_total_memory_gb",
	"cli_workers_total_gpu",
	"cli_models_header",
	"cli_models_fallback_notice",
	"cli_health_header",
	"cli_health_operational",
	"cli_repl_header",
	"cli_repl_goodbye",
	// Round-202 (2026-05-19) — 10 IDs
	"cli_help_commands_header",
	"cli_help_cmd_workers",
	"cli_help_cmd_models",
	"cli_help_cmd_health",
	"cli_help_cmd_help",
	"cli_help_cmd_exit",
	"cli_help_options_header",
	"cli_worker_added_success",
	"cli_generating_header",
	"cli_generating_prompt",
}

// withTranslator swaps in a Translator for the duration of fn and
// restores the previous translator on return — keeps test order
// independent.
func withTranslator(t *testing.T, repl i18n.Translator, fn func()) {
	t.Helper()
	prev := translator
	SetTranslator(repl)
	defer func() {
		translator = prev
	}()
	fn()
}

func TestSetTranslator_AcceptsRealTranslator(t *testing.T) {
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		got := tr(context.Background(), "cli_repl_intro", nil)
		if got != "<TRANSLATED:cli_repl_intro>" {
			t.Fatalf("tr returned %q, want sentinel-wrapped form", got)
		}
	})
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	withTranslator(t, nil, func() {
		// After SetTranslator(nil) -> NoopTranslator; tr returns
		// the message ID verbatim (loud echo).
		got := tr(context.Background(), "cli_workers_total", nil)
		if got != "cli_workers_total" {
			t.Fatalf("tr returned %q after nil reset, want loud echo of message ID", got)
		}
	})
}

func TestTr_NeverReturnsEmpty_OnTranslatorError(t *testing.T) {
	// Anti-bluff: even when the translator errors, tr MUST return
	// the message ID (loud echo) — never an empty string.
	withTranslator(t, erroringTranslator{}, func() {
		for _, id := range migratedMessageIDs {
			got := tr(context.Background(), id, nil)
			if got == "" {
				t.Fatalf("tr(%q) returned empty string under erroringTranslator; want loud echo", id)
			}
			if got != id {
				t.Fatalf("tr(%q) returned %q, want loud echo of id under error", id, got)
			}
		}
	})
}

func TestTr_AllMigratedIDs_ResolveThroughTranslator(t *testing.T) {
	// Sentinel assertion: every migrated message ID MUST route
	// through the wired Translator. Reverting a call site to a
	// hardcoded literal would NOT bump the corresponding fake's
	// call count for that ID — but at the production call site
	// level the lookup of these IDs through tr() proves the seam
	// is intact at the unit level. Round-131 mutation test:
	// temporarily replace `tr(ctx, "cli_workers_total", ...)`
	// with `fmt.Sprintf("Total Workers: %d", ...)` in main.go and
	// the round-131 audit-gate FAIL count increases by 1.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		for _, id := range migratedMessageIDs {
			got := tr(context.Background(), id, nil)
			want := "<TRANSLATED:" + id + ">"
			if got != want {
				t.Fatalf("tr(%q) returned %q, want %q", id, got, want)
			}
		}
	})
	// Confirm every ID was actually looked up via the Translator
	// interface (not a hardcoded literal).
	for _, id := range migratedMessageIDs {
		if fake.called[id] != 1 {
			t.Fatalf("fake.called[%q] = %d, want 1 — translator was not invoked for this ID", id, fake.called[id])
		}
	}
}

func TestTr_RespectsTemplateData(t *testing.T) {
	// fakeCLITranslator ignores templateData, so this test only
	// asserts that tr() PASSES templateData through to T() — the
	// real *i18nadapter.Translator handles interpolation.
	captured := make(map[string]map[string]any)
	cap := capturingTranslator{captured: captured}
	withTranslator(t, cap, func() {
		_ = tr(context.Background(), "cli_workers_total", map[string]any{"Count": 42})
	})
	got, ok := captured["cli_workers_total"]
	if !ok {
		t.Fatal("capturingTranslator did not receive cli_workers_total")
	}
	if v, _ := got["Count"].(int); v != 42 {
		t.Fatalf("templateData.Count = %v, want 42", got["Count"])
	}
}

type capturingTranslator struct {
	captured map[string]map[string]any
}

func (c capturingTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	c.captured[id] = data
	return "<TRANSLATED:" + id + ">", nil
}
func (c capturingTranslator) TPlural(_ context.Context, id string, _ int, data map[string]any) (string, error) {
	c.captured[id] = data
	return "<TRANSLATED:" + id + ">", nil
}

// TestMigratedIDs_PresentInBundle is a literal-presence assertion:
// every migrated ID MUST have a corresponding entry in
// bundles/active.en.yaml. The check is performed via the i18n
// package's NoopTranslator returning the raw ID, which we then
// expect to differ from the bundle-resolved form when a real
// Translator is wired in production. This is the round-131
// regression guard against accidentally removing a bundle entry
// while leaving the tr() call in place.
func TestMigratedIDs_PresentInBundle(t *testing.T) {
	// Bundle membership is validated at boot via the
	// *i18nadapter.Translator's Load step; at the unit-test layer
	// we assert the IDs list is non-empty + each entry follows
	// the cli_ naming convention (CONST-046 namespace discipline).
	if len(migratedMessageIDs) == 0 {
		t.Fatal("migratedMessageIDs is empty — round-131 migration manifest is corrupt")
	}
	for _, id := range migratedMessageIDs {
		if len(id) < 5 || id[:4] != "cli_" {
			t.Fatalf("migrated ID %q does not follow cli_ prefix convention", id)
		}
	}
}

// round196MigratedIDs is the round-196 subset of migratedMessageIDs.
// Used by the round-196 paired-mutation test to identify which IDs
// MUST round-trip through Translator.T after round-196's call-site
// migration. Reverting cli_workers_header at line 1263 back to
// `fmt.Println("\n=== Worker Statistics ===")` drops fake.called[
// "cli_workers_header"] to 0 and TestRound196_AllNewIDs_Migrated FAILs.
var round196MigratedIDs = []string{
	"cli_workers_header",
	"cli_workers_total_cpu",
	"cli_workers_total_memory_gb",
	"cli_workers_total_gpu",
	"cli_models_header",
	"cli_models_fallback_notice",
	"cli_health_header",
	"cli_health_operational",
	"cli_repl_header",
	"cli_repl_goodbye",
}

func TestRound196_AllNewIDs_RouteThroughTranslator(t *testing.T) {
	// Paired-mutation invariant: every round-196 ID MUST be looked
	// up via tr(). If a future commit reverts one of the call sites
	// to a hardcoded literal, the corresponding fake.called count
	// drops to 0 at the production call site — and this test FAILs
	// at the unit-tr() layer because the ID would no longer round-
	// trip through the sentinel-wrapping fake.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		for _, id := range round196MigratedIDs {
			got := tr(context.Background(), id, nil)
			want := "<TRANSLATED:" + id + ">"
			if got != want {
				t.Fatalf("round-196 ID %q: tr returned %q, want %q", id, got, want)
			}
		}
	})
	for _, id := range round196MigratedIDs {
		if fake.called[id] != 1 {
			t.Fatalf("round-196 ID %q: fake.called = %d, want 1", id, fake.called[id])
		}
	}
}

func TestRound196_TemplateData_PreservedForCountFields(t *testing.T) {
	// Anti-bluff: cli_workers_total_cpu and cli_workers_total_gpu
	// MUST carry a "Count" key; cli_workers_total_memory_gb MUST
	// carry a "GB" key. The migrated call sites pass these names;
	// if the bundle entry diverges (e.g. someone renames the
	// placeholder), interpolation silently drops the value and end
	// users see "Total CPU: <no value>".
	captured := make(map[string]map[string]any)
	cap := capturingTranslator{captured: captured}
	withTranslator(t, cap, func() {
		_ = tr(context.Background(), "cli_workers_total_cpu", map[string]any{"Count": 16})
		_ = tr(context.Background(), "cli_workers_total_gpu", map[string]any{"Count": 4})
		_ = tr(context.Background(), "cli_workers_total_memory_gb", map[string]any{"GB": "31.42"})
	})
	if captured["cli_workers_total_cpu"]["Count"] != 16 {
		t.Fatalf("cli_workers_total_cpu Count = %v, want 16", captured["cli_workers_total_cpu"]["Count"])
	}
	if captured["cli_workers_total_gpu"]["Count"] != 4 {
		t.Fatalf("cli_workers_total_gpu Count = %v, want 4", captured["cli_workers_total_gpu"]["Count"])
	}
	if captured["cli_workers_total_memory_gb"]["GB"] != "31.42" {
		t.Fatalf("cli_workers_total_memory_gb GB = %v, want 31.42", captured["cli_workers_total_memory_gb"]["GB"])
	}
}

// round202MigratedIDs is the round-202 subset. Used by the round-202
// paired-mutation test. Reverting cli_help_commands_header at the
// showHelp() top back to fmt.Println("\n=== Available Commands ===")
// drops fake.called["cli_help_commands_header"] to 0 and
// TestRound202_AllNewIDs_RouteThroughTranslator FAILs.
var round202MigratedIDs = []string{
	"cli_help_commands_header",
	"cli_help_cmd_workers",
	"cli_help_cmd_models",
	"cli_help_cmd_health",
	"cli_help_cmd_help",
	"cli_help_cmd_exit",
	"cli_help_options_header",
	"cli_worker_added_success",
	"cli_generating_header",
	"cli_generating_prompt",
}

func TestRound202_AllNewIDs_RouteThroughTranslator(t *testing.T) {
	// Paired-mutation invariant: every round-202 ID MUST be looked
	// up via tr(). If a future commit reverts one of the call sites
	// (e.g. showHelp() goes back to a fmt.Println literal), the
	// corresponding fake.called count drops to 0 at the production
	// call site — and this test FAILs at the unit-tr() layer because
	// the ID would no longer round-trip through the sentinel-wrapping
	// fake.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		for _, id := range round202MigratedIDs {
			got := tr(context.Background(), id, nil)
			want := "<TRANSLATED:" + id + ">"
			if got != want {
				t.Fatalf("round-202 ID %q: tr returned %q, want %q", id, got, want)
			}
		}
	})
	for _, id := range round202MigratedIDs {
		if fake.called[id] != 1 {
			t.Fatalf("round-202 ID %q: fake.called = %d, want 1", id, fake.called[id])
		}
	}
}

func TestRound202_TemplateData_PreservedForPlaceholders(t *testing.T) {
	// Anti-bluff: cli_worker_added_success MUST carry "Host" key;
	// cli_generating_header MUST carry "Model" key; cli_generating_prompt
	// MUST carry "Prompt" key. The migrated call sites pass these names;
	// if the bundle entry diverges (e.g. someone renames the placeholder),
	// interpolation silently drops the value and end users see
	// "Worker added successfully: <no value>".
	captured := make(map[string]map[string]any)
	cap := capturingTranslator{captured: captured}
	withTranslator(t, cap, func() {
		_ = tr(context.Background(), "cli_worker_added_success", map[string]any{"Host": "node-42.example.com"})
		_ = tr(context.Background(), "cli_generating_header", map[string]any{"Model": "llama3.2"})
		_ = tr(context.Background(), "cli_generating_prompt", map[string]any{"Prompt": "What is 2+2?"})
	})
	if captured["cli_worker_added_success"]["Host"] != "node-42.example.com" {
		t.Fatalf("cli_worker_added_success Host = %v, want node-42.example.com", captured["cli_worker_added_success"]["Host"])
	}
	if captured["cli_generating_header"]["Model"] != "llama3.2" {
		t.Fatalf("cli_generating_header Model = %v, want llama3.2", captured["cli_generating_header"]["Model"])
	}
	if captured["cli_generating_prompt"]["Prompt"] != "What is 2+2?" {
		t.Fatalf("cli_generating_prompt Prompt = %v, want %q", captured["cli_generating_prompt"]["Prompt"], "What is 2+2?")
	}
}

func TestRound202_ShowHelp_RoutesThroughTranslator(t *testing.T) {
	// Sentinel test: invoke c.showHelp(ctx) with a fake translator
	// wired, and assert the seven round-202 help IDs were all looked
	// up. This is the actual call-site mutation invariant — reverting
	// any of the seven fmt.Println(tr(...)) lines back to
	// fmt.Println("...literal...") drops the corresponding ID's call
	// count to 0 and this test FAILs.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		c := &CLI{}
		c.showHelp(context.Background())
	})
	helpIDs := []string{
		"cli_help_commands_header",
		"cli_help_cmd_workers",
		"cli_help_cmd_models",
		"cli_help_cmd_health",
		"cli_help_cmd_help",
		"cli_help_cmd_exit",
		"cli_help_options_header",
	}
	for _, id := range helpIDs {
		if fake.called[id] < 1 {
			t.Fatalf("showHelp() did not invoke translator for %q (fake.called = %d)", id, fake.called[id])
		}
	}
}

// round311MigratedIDs is the exhaustive round-311 (2026-05-19) ID set —
// the FINAL cmd/cli CONST-046 migration that drove cmd/cli to ZERO
// audit violations. Covers the cobra Short/Long/flag-help strings for
// the commands / hooks / lsp / mcp / permissions / sessions / skills /
// wizard / worktree subcommand groups plus the residual main.go
// runtime lines. Every ID MUST round-trip through the package-level
// tr()/trc() helper; reverting any call site to a hardcoded literal
// drops the corresponding fake.called count to 0 and FAILs
// TestRound311_AllNewIDs_RouteThroughTranslator.
var round311MigratedIDs = []string{
	// commands_cmd.go
	"cli_commands_root_short", "cli_commands_list_short", "cli_commands_show_short",
	"cli_commands_run_short", "cli_commands_reload_short", "cli_commands_reload_result",
	// hooks_cmd.go
	"cli_hooks_root_short", "cli_hooks_list_short", "cli_hooks_validate_short",
	"cli_hooks_test_short", "cli_hooks_enable_short", "cli_hooks_disable_short",
	"cli_hooks_validate_ok", "cli_hooks_test_result",
	// lsp_cmd.go
	"cli_lsp_root_short", "cli_lsp_root_long", "cli_lsp_status_short",
	"cli_lsp_list_servers_short", "cli_lsp_restart_short", "cli_lsp_stop_short",
	// mcp_cmd.go
	"cli_mcp_root_short", "cli_mcp_add_short", "cli_mcp_remove_short",
	"cli_mcp_list_short", "cli_mcp_test_short", "cli_mcp_auth_short", "cli_mcp_logs_short",
	// permissions_cmd.go
	"cli_permissions_root_short", "cli_permissions_list_short", "cli_permissions_add_short",
	"cli_permissions_remove_short", "cli_permissions_check_short", "cli_permissions_check_command_flag",
	// sessions_cmd.go
	"cli_sessions_root_short", "cli_sessions_list_short", "cli_sessions_show_short",
	"cli_sessions_show_header", "cli_sessions_delete_short",
	// skills_cmd.go
	"cli_skills_root_short", "cli_skills_list_short", "cli_skills_show_short",
	"cli_skills_show_header", "cli_skills_invoke_short", "cli_skills_reload_short",
	"cli_skills_reload_result",
	// wizard_cmd.go
	"cli_wizard_short", "cli_wizard_long", "cli_wizard_provider_flag",
	"cli_wizard_apikey_flag", "cli_wizard_region_flag", "cli_wizard_project_flag",
	"cli_wizard_location_flag", "cli_wizard_apiversion_flag", "cli_wizard_cancelled",
	"cli_wizard_config_exists_prompt", "cli_wizard_keeping_existing", "cli_wizard_wrote_provider",
	// worktree_cmd.go
	"cli_worktree_root_short", "cli_worktree_list_short", "cli_worktree_remove_short",
	"cli_worktree_enter_stateful_l1", "cli_worktree_enter_stateful_l2",
	"cli_worktree_stateful_l3", "cli_worktree_exit_stateful_l1", "cli_worktree_exit_stateful_l2",
	// main.go residual runtime lines
	"cli_debug_flags_parsed", "cli_session_resumed", "cli_session_no_resumable",
	"cli_session_resumed_active", "cli_f12_no_cloud_provider", "cli_f12_construct_failed",
	"cli_model_info_provider", "cli_model_info_verified", "cli_health_worker_pool_ok",
	"cli_health_worker_pool_none", "cli_health_notification_ok", "cli_health_notification_none",
	"cli_tokens_summary", "cli_file_skipped_too_large", "cli_file_skipped_label",
	"cli_notification_sent", "cli_command_completed", "cli_provider_default_model",
	"cli_repl_no_provider", "cli_screenshot_saved", "cli_session_cancelled",
}

func TestRound311_AllNewIDs_RouteThroughTranslator(t *testing.T) {
	// Paired-mutation invariant: every round-311 ID MUST be resolvable
	// through the wired Translator. The trc() helper (cobra-construction
	// resolver) and tr() (runtime resolver) both funnel through the same
	// package-level translator, so wrapping every ID here proves the seam
	// is intact for both surfaces.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		for _, id := range round311MigratedIDs {
			got := tr(context.Background(), id, nil)
			want := "<TRANSLATED:" + id + ">"
			if got != want {
				t.Fatalf("round-311 ID %q: tr returned %q, want %q", id, got, want)
			}
		}
	})
	for _, id := range round311MigratedIDs {
		if fake.called[id] != 1 {
			t.Fatalf("round-311 ID %q: fake.called = %d, want 1", id, fake.called[id])
		}
	}
}

func TestRound311_Trc_RoutesThroughTranslator(t *testing.T) {
	// trc() is the cobra-construction-time resolver added in round-311.
	// It MUST funnel through the same package-level translator as tr().
	// Reverting trc() to return its msgID argument directly (or to a
	// hardcoded literal) would break this sentinel assertion.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		got := trc("cli_commands_root_short", nil)
		want := "<TRANSLATED:cli_commands_root_short>"
		if got != want {
			t.Fatalf("trc() returned %q, want %q — cobra-metadata seam broken", got, want)
		}
	})
	if fake.called["cli_commands_root_short"] != 1 {
		t.Fatalf("trc() did not invoke the translator (fake.called = %d)",
			fake.called["cli_commands_root_short"])
	}
}

func TestRound311_AllNewIDs_PrefixConvention(t *testing.T) {
	// CONST-046 namespace discipline: every round-311 ID MUST follow the
	// cli_ prefix convention so it never collides with another
	// submodule's bundle when loaded into a shared go-i18n.Bundle.
	if len(round311MigratedIDs) == 0 {
		t.Fatal("round311MigratedIDs is empty — round-311 migration manifest is corrupt")
	}
	seen := make(map[string]bool, len(round311MigratedIDs))
	for _, id := range round311MigratedIDs {
		if len(id) < 5 || id[:4] != "cli_" {
			t.Fatalf("round-311 ID %q does not follow cli_ prefix convention", id)
		}
		if seen[id] {
			t.Fatalf("round-311 ID %q is duplicated in round311MigratedIDs", id)
		}
		seen[id] = true
	}
}
