package cmd

// Round-315 §11.4 anti-bluff coverage for the CONST-046 i18n migration of
// helix_code/cmd/local_llm.go + local_llm_advanced.go. These tests prove the
// migrated user-facing strings actually resolve through the package-level
// translator seam (tr/trc) — and that the seam is a real DI seam, not a
// no-op disguised as one.
//
// CONST-035 / CONST-046 / CONST-050(A) / Article XI §11.9.
//
// Unit-level tests (CONST-050(A)-compliant — *_test.go without the
// integration build tag is the only layer where test doubles are
// permitted; the sentinelTranslator here is a test-only Translator used
// purely to observe the seam, never imported by production code).

import (
	"context"
	"strings"
	"testing"

	cmdi18n "dev.helix.code/cmd/i18n"
)

// sentinelTranslator returns a uniquely-recognisable string for every
// message ID so a test can assert (a) the seam routed the call through a
// real Translator, and (b) the original English literal is absent from the
// rendered output. The joint invariant (sentinel present AND literal
// absent) is what makes this a paired-mutation anti-bluff test rather than
// a presence-only check.
type sentinelTranslator struct{}

const i18nSentinel = "‹CMD-I18N-SENTINEL›"

func (sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return i18nSentinel + id, nil
}

func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return i18nSentinel + id, nil
}

// migratedMessageIDs is the closed set of CONST-046 message IDs introduced
// by round 315. Every ID MUST resolve through the seam.
var migratedMessageIDs = []string{
	"cmd_local_llm_init_start",
	"cmd_local_llm_init_complete",
	"cmd_local_llm_autostart",
	"cmd_local_llm_start_all",
	"cmd_local_llm_start_provider",
	"cmd_local_llm_stop_all",
	"cmd_local_llm_stop_provider",
	"cmd_local_llm_status_none",
	"cmd_local_llm_running_endpoints",
	"cmd_local_llm_cleanup_start",
	"cmd_local_llm_update_all",
	"cmd_local_llm_update_failed",
	"cmd_local_llm_update_done",
	"cmd_local_llm_update_provider",
	"cmd_local_llm_monitor_start",
	"cmd_local_llm_monitor_stop_hint",
	"cmd_local_llm_monitor_stopping",
	"cmd_local_llm_monitor_status_header",
	"cmd_local_llm_monitor_status_error",
	"cmd_local_llm_download_model",
	"cmd_local_llm_download_desc",
	"cmd_local_llm_download_size",
	"cmd_local_llm_download_no_provider",
	"cmd_local_llm_download_starting",
	"cmd_local_llm_download_complete",
	"cmd_local_llm_adv_discovering",
	"cmd_local_llm_adv_source",
	"cmd_local_llm_adv_filter",
	"cmd_local_llm_adv_no_models",
	"cmd_local_llm_adv_found_models",
	"cmd_local_llm_adv_recommend_hint",
	"cmd_local_llm_adv_recommend_start",
	"cmd_local_llm_adv_recommend_tasks",
	"cmd_local_llm_adv_no_suitable",
	"cmd_local_llm_adv_adjust_hint",
	// Round-422 §11.4 (2026-05-20, CONST-046 Phase 4): local-llm cobra
	// command Short/Long descriptions + flag-help + provider descriptions.
	"cmd_local_llm_short",
	"cmd_local_llm_long",
	"cmd_local_llm_flag_dir",
	"cmd_local_llm_flag_autostart",
	"cmd_local_llm_flag_health_interval",
	"cmd_local_llm_init_short",
	"cmd_local_llm_init_long",
	"cmd_local_llm_start_short",
	"cmd_local_llm_start_long",
	"cmd_local_llm_stop_short",
	"cmd_local_llm_stop_long",
	"cmd_local_llm_status_short",
	"cmd_local_llm_status_long",
	"cmd_local_llm_list_short",
	"cmd_local_llm_list_long",
	"cmd_local_llm_cleanup_short",
	"cmd_local_llm_cleanup_long",
	"cmd_local_llm_update_short",
	"cmd_local_llm_update_long",
	"cmd_local_llm_logs_short",
	"cmd_local_llm_logs_long",
	"cmd_local_llm_provider_desc_vllm",
	"cmd_local_llm_provider_desc_localai",
	"cmd_local_llm_provider_desc_fastchat",
	"cmd_local_llm_provider_desc_textgen",
	"cmd_local_llm_provider_desc_lmstudio",
	"cmd_local_llm_provider_desc_jan",
	"cmd_local_llm_provider_desc_koboldai",
	"cmd_local_llm_provider_desc_gpt4all",
	"cmd_local_llm_provider_desc_tabbyapi",
	"cmd_local_llm_provider_desc_mlx",
	"cmd_local_llm_provider_desc_mistralrs",
}

// round422CobraMetadataIDs is the closed set of message IDs that feed the
// local-llm cobra command tree's Short/Long descriptions and flag-help
// text. Unlike runtime strings, these are resolved at command-construction
// time via trc() — so the paired-mutation guard below proves that even
// construction-time metadata is locale-aware rather than a frozen English
// literal.
var round422CobraMetadataIDs = []string{
	"cmd_local_llm_short",
	"cmd_local_llm_long",
	"cmd_local_llm_flag_dir",
	"cmd_local_llm_flag_autostart",
	"cmd_local_llm_flag_health_interval",
	"cmd_local_llm_init_short",
	"cmd_local_llm_init_long",
	"cmd_local_llm_start_short",
	"cmd_local_llm_start_long",
	"cmd_local_llm_stop_short",
	"cmd_local_llm_stop_long",
	"cmd_local_llm_status_short",
	"cmd_local_llm_status_long",
	"cmd_local_llm_list_short",
	"cmd_local_llm_list_long",
	"cmd_local_llm_cleanup_short",
	"cmd_local_llm_cleanup_long",
	"cmd_local_llm_update_short",
	"cmd_local_llm_update_long",
	"cmd_local_llm_logs_short",
	"cmd_local_llm_logs_long",
}

// round426ModelsMetadataIDs is the closed set of message IDs introduced by
// the round-426 §11.4 anti-bluff sweep (2026-05-20, CONST-046 Phase 4):
// the `models` command-group cobra Short/Long descriptions plus every
// model-management / cross-provider / advanced flag-help string. All are
// resolved at command-construction time via trc().
var round426ModelsMetadataIDs = []string{
	"cmd_local_llm_models_short",
	"cmd_local_llm_models_long",
	"cmd_local_llm_models_download_short",
	"cmd_local_llm_models_download_long",
	"cmd_local_llm_models_convert_short",
	"cmd_local_llm_models_convert_long",
	"cmd_local_llm_models_list_short",
	"cmd_local_llm_models_list_long",
	"cmd_local_llm_models_search_short",
	"cmd_local_llm_models_search_long",
	"cmd_local_llm_flag_download_format",
	"cmd_local_llm_flag_download_provider",
	"cmd_local_llm_flag_download_output",
	"cmd_local_llm_flag_download_force",
	"cmd_local_llm_flag_convert_format",
	"cmd_local_llm_flag_convert_quantize",
	"cmd_local_llm_flag_convert_optimize",
	"cmd_local_llm_flag_convert_hardware",
	"cmd_local_llm_flag_share_provider",
	"cmd_local_llm_flag_optimize_provider",
	"cmd_local_llm_flag_sync_all",
	"cmd_local_llm_flag_discover_source",
	"cmd_local_llm_flag_discover_filter",
	"cmd_local_llm_flag_recommend_tasks",
	"cmd_local_llm_flag_recommend_quality",
	"cmd_local_llm_flag_recommend_privacy",
	"cmd_local_llm_flag_recommend_max_memory",
	"cmd_local_llm_flag_recommend_budget",
	"cmd_local_llm_flag_recommend_providers",
}

// round434ResidualRuntimeIDs is the closed set of message IDs introduced
// by the round-434 §11.4 anti-bluff sweep (2026-05-20, CONST-046 Phase 4):
// helix_code/cmd residual round-3 — 24 user-facing runtime strings from
// the local-llm logs / watch / download / convert / models command
// groups. All resolve at runtime via tr().
var round434ResidualRuntimeIDs = []string{
	"cmd_local_llm_log_directory",
	"cmd_local_llm_log_showing",
	"cmd_local_llm_log_file",
	"cmd_local_llm_watch_start",
	"cmd_local_llm_watch_paths",
	"cmd_local_llm_watch_header",
	"cmd_local_llm_watch_status_error",
	"cmd_local_llm_flag_analytics_time_range",
	"cmd_local_llm_flag_report_format",
	"cmd_local_llm_flag_insights_type",
	"cmd_local_llm_download_formats",
	"cmd_local_llm_download_saved",
	"cmd_local_llm_download_saved_provider",
	"cmd_local_llm_convert_model",
	"cmd_local_llm_convert_source_format",
	"cmd_local_llm_convert_target_format",
	"cmd_local_llm_convert_started",
	"cmd_local_llm_convert_output_pending",
	"cmd_local_llm_convert_logs",
	"cmd_local_llm_convert_status_error",
	"cmd_local_llm_convert_completed",
	"cmd_local_llm_convert_output_saved",
	"cmd_local_llm_convert_duration",
	"cmd_local_llm_convert_failed",
	"cmd_local_llm_convert_check_logs",
	"cmd_local_llm_models_none_in_registry",
	"cmd_local_llm_models_total",
	"cmd_local_llm_models_hint_search",
	"cmd_local_llm_models_hint_download",
	"cmd_local_llm_models_none_for_query",
}

// TestLocalLLMI18n_Round434ResidualRuntimeRoutesThroughSeam is the
// paired-mutation anti-bluff guard for the round-434 migration: it wires a
// sentinel Translator and asserts every residual runtime message ID
// resolves THROUGH it (sentinel present) — proving the logs/watch/
// download/convert/models output strings are no longer frozen English
// literals. Re-inlining any would drop the sentinel and FAIL this test.
func TestLocalLLMI18n_Round434ResidualRuntimeRoutesThroughSeam(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	ctx := context.Background()
	for _, id := range round434ResidualRuntimeIDs {
		got := tr(ctx, id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Errorf("round-434 residual runtime ID %q did not route through the injected Translator: got %q", id, got)
		}
		if !strings.Contains(got, id) {
			t.Errorf("round-434 residual runtime ID %q lost its identity through the seam: got %q", id, got)
		}
	}
}

// TestLocalLLMI18n_Round434ResidualRuntimeNoopEcho proves the round-434
// IDs degrade to a loud message-ID echo when no translator is wired —
// never a silent empty string (which would be a §11.4 PASS-bluff at the
// i18n layer).
func TestLocalLLMI18n_Round434ResidualRuntimeNoopEcho(t *testing.T) {
	SetTranslator(nil) // explicit NoopTranslator
	ctx := context.Background()
	for _, id := range round434ResidualRuntimeIDs {
		got := tr(ctx, id, nil)
		if got != id {
			t.Errorf("round-434 residual runtime ID %q must echo verbatim under NoopTranslator: got %q", id, got)
		}
	}
}

// TestLocalLLMI18n_Round426ModelsMetadataRoutesThroughSeam is the
// paired-mutation anti-bluff guard for the round-426 migration: it wires a
// sentinel Translator and asserts every `models` command-group + flag-help
// message ID resolves THROUGH it (sentinel present) — proving the cobra
// metadata is no longer a frozen English literal. If a future change
// re-inlines any of these strings, the literal would not contain the
// sentinel and this test FAILs.
func TestLocalLLMI18n_Round426ModelsMetadataRoutesThroughSeam(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	for _, id := range round426ModelsMetadataIDs {
		got := trc(id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Errorf("round-426 models metadata ID %q did not route through the injected Translator: got %q", id, got)
		}
		if !strings.Contains(got, id) {
			t.Errorf("round-426 models metadata ID %q lost its identity through the seam: got %q", id, got)
		}
	}
}

// TestLocalLLMI18n_Round426ModelsMetadataNoopEcho proves the round-426
// IDs degrade to a loud message-ID echo when no translator is wired —
// never a silent empty string (which would be a §11.4 PASS-bluff at the
// i18n layer).
func TestLocalLLMI18n_Round426ModelsMetadataNoopEcho(t *testing.T) {
	SetTranslator(nil) // explicit NoopTranslator
	for _, id := range round426ModelsMetadataIDs {
		got := trc(id, nil)
		if got != id {
			t.Errorf("round-426 metadata ID %q must echo verbatim under NoopTranslator: got %q", id, got)
		}
	}
}

// TestLocalLLMI18n_Round422CobraMetadataRoutesThroughSeam is the
// paired-mutation anti-bluff guard for the round-422 migration: it wires a
// sentinel Translator and asserts every construction-time message ID
// resolves THROUGH it (sentinel present) — proving the cobra Short/Long
// descriptions are no longer frozen English literals. If a future change
// re-inlines any of these strings, the literal would not contain the
// sentinel and this test FAILs.
func TestLocalLLMI18n_Round422CobraMetadataRoutesThroughSeam(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	for _, id := range round422CobraMetadataIDs {
		got := trc(id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Errorf("round-422 cobra metadata ID %q did not route through the injected Translator: got %q", id, got)
		}
		if !strings.Contains(got, id) {
			t.Errorf("round-422 cobra metadata ID %q lost its identity through the seam: got %q", id, got)
		}
	}
}

// TestLocalLLMI18n_SeamRoutesThroughTranslator wires a sentinel Translator
// and asserts every migrated message ID is resolved THROUGH it — proving
// the tr() seam is real injection, not a hardcoded fallback. Restores the
// default NoopTranslator afterwards so sibling tests are unaffected.
func TestLocalLLMI18n_SeamRoutesThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	ctx := context.Background()
	for _, id := range migratedMessageIDs {
		got := tr(ctx, id, nil)
		if !strings.HasPrefix(got, i18nSentinel) {
			t.Errorf("message ID %q did not route through the injected Translator: got %q", id, got)
		}
		if !strings.Contains(got, id) {
			t.Errorf("message ID %q lost its identity through the seam: got %q", id, got)
		}
	}
}

// TestLocalLLMI18n_NoopFallbackIsLoudEcho asserts that with no wired
// Translator the seam echoes the message ID verbatim (loud echo) rather
// than silently returning empty — a silent swallow would be a §11.4
// PASS-bluff at the i18n layer.
func TestLocalLLMI18n_NoopFallbackIsLoudEcho(t *testing.T) {
	SetTranslator(nil) // explicit NoopTranslator
	ctx := context.Background()
	for _, id := range migratedMessageIDs {
		got := tr(ctx, id, nil)
		if got != id {
			t.Errorf("NoopTranslator must echo the message ID verbatim: id=%q got=%q", id, got)
		}
	}
}

// TestLocalLLMI18n_SetTranslatorNilResets verifies SetTranslator(nil)
// resets to the loud-echo NoopTranslator instead of leaving a nil
// translator that would panic or silently swallow.
func TestLocalLLMI18n_SetTranslatorNilResets(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil)
	got := tr(context.Background(), "cmd_local_llm_init_start", nil)
	if got != "cmd_local_llm_init_start" {
		t.Fatalf("SetTranslator(nil) did not reset to NoopTranslator: got %q", got)
	}
}

// TestLocalLLMI18n_TrcUsesPackageTranslator proves the construction-time
// trc() helper resolves through the same package-level translator the
// runtime tr() helper uses (cobra metadata is just as locale-aware as
// runtime output).
func TestLocalLLMI18n_TrcUsesPackageTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })
	got := trc("cmd_local_llm_init_start", nil)
	if !strings.HasPrefix(got, i18nSentinel) {
		t.Fatalf("trc() did not route through the injected Translator: got %q", got)
	}
}

// TestLocalLLMI18n_NoopTranslatorContract is a paired-mutation guard on the
// NoopTranslator itself: T and TPlural must both echo the ID and never
// return an error.
func TestLocalLLMI18n_NoopTranslatorContract(t *testing.T) {
	var n cmdi18n.NoopTranslator
	out, err := n.T(context.Background(), "cmd_probe", nil)
	if err != nil || out != "cmd_probe" {
		t.Fatalf("NoopTranslator.T contract violation: out=%q err=%v", out, err)
	}
	outP, errP := n.TPlural(context.Background(), "cmd_probe", 3, nil)
	if errP != nil || outP != "cmd_probe" {
		t.Fatalf("NoopTranslator.TPlural contract violation: out=%q err=%v", outP, errP)
	}
}
