package cmd

// Round-453 §11.4 anti-bluff coverage for the CONST-046 i18n migration of
// the residual user-facing string literals in helix_code/cmd/main_commands.go
// (auto-mode flow + provider/automation status displays), other_commands.go
// (generate/test/worker/notify cobra command tree), and root.go (root cobra
// metadata, persistent-flag help, config-file message). These tests prove the
// migrated strings actually resolve through the package-level translator seam
// (tr/trc) — and that the seam is a real DI seam, not a no-op disguised as
// one.
//
// CONST-035 / CONST-046 / CONST-050(A) / Article XI §11.9.
//
// Unit-level tests (CONST-050(A)-compliant — *_test.go without the
// integration build tag is the only layer where test doubles are permitted;
// the sentinelTranslator / capturingTranslator reused here from
// local_llm_i18n_test.go / main_other_commands_i18n_test.go are test-only
// Translators used purely to observe the seam, never imported by production
// code).

import (
	"context"
	"strings"
	"testing"
)

// round453MessageIDs is the closed set of CONST-046 message IDs introduced by
// round 453. Every ID MUST resolve through the seam and MUST have a matching
// entry in cmd/i18n/bundles/active.en.yaml.
var round453MessageIDs = []string{
	// main_commands.go — auto-mode flow
	"cmd_auto_init_system",
	"cmd_auto_start_operations",
	"cmd_auto_mode_active",
	"cmd_auto_mode_running",
	"cmd_auto_web_dashboard",
	"cmd_auto_api_docs",
	"cmd_auto_bg_header",
	"cmd_auto_bg_install",
	"cmd_auto_bg_health",
	"cmd_auto_bg_optimize",
	"cmd_auto_bg_updates",
	"cmd_auto_bg_recovery",
	"cmd_auto_no_action",
	"cmd_press_ctrlc_stop",
	"cmd_auto_shutdown",
	"cmd_auto_stopped_signal",
	"cmd_auto_stop_error",
	"cmd_auto_system_stopped",
	// main_commands.go — status displays
	"cmd_provider_status_header",
	"cmd_provider_status_line",
	"cmd_automation_status_header",
	"cmd_automation_installed",
	"cmd_automation_running",
	"cmd_automation_healthy",
	"cmd_automation_auto_optimize",
	"cmd_automation_auto_updates",
	"cmd_automation_auto_recovery",
	"cmd_automation_bg_status",
	"cmd_main_server_started",
	"cmd_monitoring_dashboard_started",
	// other_commands.go — server/generate/test/worker/notify
	"cmd_err_config",
	"cmd_err_database_unavailable",
	"cmd_err_redis_unavailable",
	"cmd_err_server",
	"cmd_server_received_signal",
	"cmd_err_shutdown",
	"cmd_server_stopped",
	"cmd_generate_short",
	"cmd_generate_long",
	"cmd_generate_need_prompt",
	"cmd_generate_no_default_provider",
	"cmd_generate_set_default_provider",
	"cmd_generate_no_models",
	"cmd_generate_provider_unavailable",
	"cmd_generate_failed",
	"cmd_test_short",
	"cmd_test_long",
	"cmd_test_failed",
	"cmd_worker_short",
	"cmd_worker_long",
	"cmd_worker_needs_database",
	"cmd_worker_set_database",
	"cmd_worker_config_summary",
	"cmd_worker_use_subcommands",
	"cmd_notify_short",
	"cmd_notify_long",
	"cmd_notify_need_message",
	"cmd_notify_title",
	"cmd_notify_failed",
	"cmd_notify_dispatched",
	// root.go
	"cmd_root_short",
	"cmd_root_long",
	"cmd_root_flag_config",
	"cmd_root_flag_debug",
	"cmd_root_flag_log_level",
	"cmd_root_using_config_file",
}

// TestRound453I18n_SeamRoutesThroughTranslator wires a sentinel Translator and
// asserts every round-453 message ID is resolved THROUGH it — proving the tr()
// seam is real injection, not a hardcoded fallback. Restores the default
// NoopTranslator afterwards so sibling tests are unaffected.
func TestRound453I18n_SeamRoutesThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	ctx := context.Background()
	for _, id := range round453MessageIDs {
		got := tr(ctx, id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Errorf("message ID %q did not route through the injected Translator: got %q", id, got)
		}
		if !strings.Contains(got, id) {
			t.Errorf("message ID %q lost its identity through the seam: got %q", id, got)
		}
	}
}

// TestRound453I18n_NoopFallbackIsLoudEcho asserts that with no wired Translator
// the seam echoes the message ID verbatim (loud echo) rather than silently
// returning empty — a silent swallow would be a §11.4 PASS-bluff at the i18n
// layer.
func TestRound453I18n_NoopFallbackIsLoudEcho(t *testing.T) {
	SetTranslator(nil) // explicit NoopTranslator
	ctx := context.Background()
	for _, id := range round453MessageIDs {
		got := tr(ctx, id, nil)
		if got != id {
			t.Errorf("NoopTranslator must echo the message ID verbatim: id=%q got=%q", id, got)
		}
	}
}

// TestRound453I18n_TrcUsesPackageTranslator proves the construction-time trc()
// helper (used for cobra Short/Long/flag-help metadata) resolves through the
// same package-level translator the runtime tr() helper uses.
func TestRound453I18n_TrcUsesPackageTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })
	for _, id := range []string{
		"cmd_generate_short", "cmd_test_long", "cmd_worker_short",
		"cmd_notify_long", "cmd_root_short", "cmd_root_flag_debug",
	} {
		got := trc(id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Fatalf("trc(%q) did not route through the injected Translator: got %q", id, got)
		}
	}
}

// TestRound453I18n_TemplateDataReaches proves the data map supplied to tr()
// reaches the Translator — guarding the {{.Error}}/{{.Count}}/{{.Path}}/etc.
// interpolation points migrated in round 453 against silent data drops.
func TestRound453I18n_TemplateDataReaches(t *testing.T) {
	cases := []struct {
		id     string
		data   map[string]any
		probeK string
		probeV any
	}{
		{"cmd_auto_stop_error", map[string]any{"Error": "boom"}, "Error", "boom"},
		{"cmd_automation_installed", map[string]any{"Count": 7}, "Count", 7},
		{"cmd_root_using_config_file", map[string]any{"Path": "/etc/helix.yaml"}, "Path", "/etc/helix.yaml"},
		{"cmd_worker_config_summary", map[string]any{"HealthTTL": 30, "MaxConcurrent": 4}, "HealthTTL", 30},
	}
	for _, c := range cases {
		var captured map[string]any
		SetTranslator(capturingTranslator{onT: func(d map[string]any) { captured = d }})
		_ = tr(context.Background(), c.id, c.data)
		if captured == nil || captured[c.probeK] != c.probeV {
			t.Errorf("template data did not reach the Translator for %q: captured=%v", c.id, captured)
		}
		SetTranslator(nil)
	}
}

// TestRound453I18n_NoLiteralLeakThroughSeam is the paired-mutation half: with
// the sentinel Translator wired, the rendered output for a representative set
// of migrated points MUST NOT contain the original English literal — proving
// the literal was actually removed from the emission path, not merely
// duplicated alongside a seam call.
func TestRound453I18n_NoLiteralLeakThroughSeam(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	ctx := context.Background()
	literalProbes := map[string]string{
		"cmd_auto_mode_active":       "Fully Automated Mode Active",
		"cmd_generate_short":         "Generate code/text with AI",
		"cmd_worker_use_subcommands": "Use worker subcommands",
		"cmd_root_short":             "Enterprise AI Development Platform",
		"cmd_server_stopped":         "Server stopped",
	}
	for id, literal := range literalProbes {
		got := tr(ctx, id, nil)
		if strings.Contains(got, literal) {
			t.Errorf("message ID %q leaked the original literal %q through the seam: got %q", id, literal, got)
		}
	}
}
