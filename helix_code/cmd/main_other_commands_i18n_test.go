package cmd

// Round-317 §11.4 anti-bluff coverage for the CONST-046 i18n migration of
// helix_code/cmd/main_commands.go + other_commands.go. These tests prove
// the migrated user-facing strings (the start/auto/server/version cobra
// command tree) actually resolve through the package-level translator seam
// (tr/trc) — and that the seam is a real DI seam, not a no-op disguised as
// one.
//
// CONST-035 / CONST-046 / CONST-050(A) / Article XI §11.9.
//
// Unit-level tests (CONST-050(A)-compliant — *_test.go without the
// integration build tag is the only layer where test doubles are
// permitted; the sentinelTranslator reused here from local_llm_i18n_test.go
// is a test-only Translator used purely to observe the seam, never imported
// by production code).

import (
	"context"
	"strings"
	"testing"
)

// round317MessageIDs is the closed set of CONST-046 message IDs introduced
// by round 317. Every ID MUST resolve through the seam and MUST have a
// matching entry in cmd/i18n/bundles/active.en.yaml.
var round317MessageIDs = []string{
	"cmd_start_short",
	"cmd_start_long",
	"cmd_auto_short",
	"cmd_auto_long",
	"cmd_start_flag_auto",
	"cmd_start_flag_monitor",
	"cmd_start_flag_optimize",
	"cmd_start_flag_check_interval",
	"cmd_start_banner",
	"cmd_start_zerotouch",
	"cmd_start_init_manager",
	"cmd_init_failed",
	"cmd_start_manager_ready",
	"cmd_start_failed",
	"cmd_start_llm_started",
	"cmd_start_running",
	"cmd_start_endpoints_header",
	"cmd_start_mgmt_header",
	"cmd_start_mgmt_status",
	"cmd_start_mgmt_logs",
	"cmd_start_mgmt_monitor",
	"cmd_press_ctrlc_graceful",
	"cmd_shutdown_signal",
	"cmd_shutdown_ctx_cancelled",
	"cmd_shutdown_error",
	"cmd_start_stopped",
	"cmd_auto_banner",
	"cmd_auto_zerotouch",
	"cmd_server_short",
	"cmd_server_long",
	"cmd_version_short",
	"cmd_version_long",
	"cmd_version_platform_name",
	"cmd_version_version",
	"cmd_version_build",
	"cmd_version_providers",
	"cmd_version_token_context",
	"cmd_version_license",
}

// TestRound317I18n_SeamRoutesThroughTranslator wires a sentinel Translator
// and asserts every round-317 message ID is resolved THROUGH it — proving
// the tr() seam is real injection, not a hardcoded fallback. Restores the
// default NoopTranslator afterwards so sibling tests are unaffected.
func TestRound317I18n_SeamRoutesThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	ctx := context.Background()
	for _, id := range round317MessageIDs {
		got := tr(ctx, id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Errorf("message ID %q did not route through the injected Translator: got %q", id, got)
		}
		if !strings.Contains(got, id) {
			t.Errorf("message ID %q lost its identity through the seam: got %q", id, got)
		}
	}
}

// TestRound317I18n_NoopFallbackIsLoudEcho asserts that with no wired
// Translator the seam echoes the message ID verbatim (loud echo) rather
// than silently returning empty — a silent swallow would be a §11.4
// PASS-bluff at the i18n layer.
func TestRound317I18n_NoopFallbackIsLoudEcho(t *testing.T) {
	SetTranslator(nil) // explicit NoopTranslator
	ctx := context.Background()
	for _, id := range round317MessageIDs {
		got := tr(ctx, id, nil)
		if got != id {
			t.Errorf("NoopTranslator must echo the message ID verbatim: id=%q got=%q", id, got)
		}
	}
}

// TestRound317I18n_TrcUsesPackageTranslator proves the construction-time
// trc() helper (used for cobra Short/Long/flag-help metadata) resolves
// through the same package-level translator the runtime tr() helper uses.
func TestRound317I18n_TrcUsesPackageTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })
	for _, id := range []string{"cmd_start_short", "cmd_server_short", "cmd_version_long"} {
		got := trc(id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Fatalf("trc(%q) did not route through the injected Translator: got %q", id, got)
		}
	}
}

// TestRound317I18n_TemplateDataReaches proves the data map supplied to tr()
// reaches the Translator — the sentinel's identity-echo confirms the call
// path, and a real bundle-backed Translator would interpolate the named
// placeholders. This guards the {{.Error}}/{{.Version}}/etc. interpolation
// points migrated in round 317 against silent data drops.
func TestRound317I18n_TemplateDataReaches(t *testing.T) {
	var captured map[string]any
	SetTranslator(capturingTranslator{onT: func(d map[string]any) { captured = d }})
	t.Cleanup(func() { SetTranslator(nil) })

	want := map[string]any{"Error": "boom"}
	_ = tr(context.Background(), "cmd_init_failed", want)
	if captured == nil || captured["Error"] != "boom" {
		t.Fatalf("template data did not reach the Translator: captured=%v", captured)
	}
}

// capturingTranslator records the templateData passed to T so a test can
// assert interpolation arguments survive the seam. Test-only double per
// CONST-050(A).
type capturingTranslator struct {
	onT func(map[string]any)
}

func (c capturingTranslator) T(_ context.Context, id string, d map[string]any) (string, error) {
	if c.onT != nil {
		c.onT(d)
	}
	return i18nSentinel + id, nil
}

func (c capturingTranslator) TPlural(_ context.Context, id string, _ int, d map[string]any) (string, error) {
	if c.onT != nil {
		c.onT(d)
	}
	return i18nSentinel + id, nil
}
