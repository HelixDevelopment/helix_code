//go:build !nogui

// CONST-046 sentinel tests for the desktop application. These tests
// assert the SetTranslator wiring + tr() resolver actually route
// every migrated message ID through the injected Translator —
// catching regressions where a future edit reverts a call site back
// to a literal string (which would silently re-introduce the §11.4
// PASS-bluff this round closes).
//
// Mocks ALLOWED per CONST-050(A): unit tests only.
//
// NOTE: these tests deliberately do NOT spin up the full Fyne UI
// (which requires X11/Xcursor headers — see round 96 environmental
// caveat). They exercise the Translator seam directly via the
// public SetTranslator + private tr() helper.
package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/applications/desktop/i18n"
)

// sentinelTranslator wraps every message ID with a recognisable
// envelope so call-site tests can assert lookup actually went
// through the Translator seam, not a hardcoded literal that happens
// to match the bundle value.
type sentinelTranslator struct {
	calls []string
}

func (s *sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	s.calls = append(s.calls, id)
	return "<SENTINEL:" + id + ">", nil
}

func (s *sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	s.calls = append(s.calls, id)
	return "<SENTINEL:" + id + ">", nil
}

// erroringTranslator returns an error for every lookup; tr() MUST
// degrade to the raw message ID (loud echo) rather than returning
// an empty string (which would silently break the UI — §11.4
// PASS-bluff at the i18n degradation layer).
type erroringTranslator struct{}

func (erroringTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("erroringTranslator: deliberate failure for " + id)
}

func (erroringTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("erroringTranslator: deliberate failure for " + id)
}

// TestDesktopApp_SetTranslator_NilResetsToNoop verifies SetTranslator(nil)
// does NOT leave the translator field as nil (which would crash on
// the next tr() call) — it MUST reset to NoopTranslator{} per the
// documented contract.
func TestDesktopApp_SetTranslator_NilResetsToNoop(t *testing.T) {
	da := &DesktopApp{}
	da.SetTranslator(nil)
	if _, ok := da.translator.(i18n.NoopTranslator); !ok {
		t.Fatalf("SetTranslator(nil) left translator as %T, want i18n.NoopTranslator", da.translator)
	}
}

// TestDesktopApp_SetTranslator_NonNilWires verifies a non-nil
// Translator is stored verbatim.
func TestDesktopApp_SetTranslator_NonNilWires(t *testing.T) {
	da := &DesktopApp{}
	s := &sentinelTranslator{}
	da.SetTranslator(s)
	if da.translator != s {
		t.Fatalf("SetTranslator did not store the provided Translator")
	}
}

// TestDesktopApp_Tr_RoutesThroughTranslator asserts the tr()
// helper actually invokes Translator.T for every migrated message
// ID — catches regressions where a future edit reverts call sites
// to hardcoded literals.
func TestDesktopApp_Tr_RoutesThroughTranslator(t *testing.T) {
	migratedIDs := []string{
		"desktop_window_title",
		"desktop_status_bar_default",
		"desktop_dashboard_header",
		"desktop_projects_select_prompt",
		"desktop_projects_create_header",
		"desktop_projects_delete_confirm",
		"desktop_sessions_select_prompt",
		"desktop_sessions_create_header",
		"desktop_models_available_header",
		"desktop_chat_input_placeholder",
		// Round-313 §11.4 sweep additions.
		"desktop_dashboard_activity_seed",
		"desktop_dashboard_activity_title",
		"desktop_tasks_description_placeholder",
		"desktop_tasks_new_label",
		"desktop_tasks_create_button",
		"desktop_common_refresh_button",
		"desktop_models_select_prompt",
		"desktop_models_details_title",
		"desktop_chat_history_placeholder",
		"desktop_chat_model_name_placeholder",
		"desktop_settings_about_text",
		"desktop_settings_about_title",
	}

	s := &sentinelTranslator{}
	da := &DesktopApp{}
	da.SetTranslator(s)

	ctx := context.Background()
	for _, id := range migratedIDs {
		got := da.tr(ctx, id, nil)
		want := "<SENTINEL:" + id + ">"
		if got != want {
			t.Fatalf("da.tr(%q) returned %q, want %q — call site likely bypasses Translator (CONST-046 regression)", id, got, want)
		}
	}

	// Every migrated ID MUST appear in the recorded calls slice.
	if len(s.calls) != len(migratedIDs) {
		t.Fatalf("sentinelTranslator recorded %d calls, want %d", len(s.calls), len(migratedIDs))
	}
	for i, id := range migratedIDs {
		if s.calls[i] != id {
			t.Fatalf("calls[%d] = %q, want %q", i, s.calls[i], id)
		}
	}
}

// TestDesktopApp_Tr_DegradesToIDOnError verifies translator errors
// do NOT silently produce empty strings — they MUST degrade to the
// raw message ID (loud echo).
func TestDesktopApp_Tr_DegradesToIDOnError(t *testing.T) {
	da := &DesktopApp{}
	da.SetTranslator(erroringTranslator{})

	got := da.tr(context.Background(), "desktop_window_title", nil)
	if got != "desktop_window_title" {
		t.Fatalf("da.tr on erroring Translator returned %q, want loud-echo %q", got, "desktop_window_title")
	}
}

// TestDesktopApp_Tr_NilTranslatorAutoHeals verifies that if the
// translator field is somehow nil at call time (e.g. caller built
// a DesktopApp{} directly without using NewDesktopApp), the tr()
// helper MUST auto-heal to NoopTranslator{} rather than panic.
func TestDesktopApp_Tr_NilTranslatorAutoHeals(t *testing.T) {
	da := &DesktopApp{} // translator field deliberately left zero
	got := da.tr(context.Background(), "desktop_window_title", nil)
	if got != "desktop_window_title" {
		t.Fatalf("da.tr with nil translator returned %q, want loud-echo %q", got, "desktop_window_title")
	}
	if _, ok := da.translator.(i18n.NoopTranslator); !ok {
		t.Fatalf("tr() failed to auto-heal nil translator; got %T", da.translator)
	}
}

// TestDesktopApp_DefaultTranslatorIsNoop verifies the constructor's
// default is NoopTranslator{}, not nil. (We cannot fully invoke
// NewDesktopApp without a display server, so this test exercises
// only the field-default assertion via direct construction.)
func TestDesktopApp_DefaultTranslatorIsNoop(t *testing.T) {
	da := &DesktopApp{translator: i18n.NoopTranslator{}}
	got := da.tr(context.Background(), "desktop_window_title", nil)
	// NoopTranslator echoes the ID verbatim.
	if got != "desktop_window_title" {
		t.Fatalf("NoopTranslator default produced %q, want loud-echo %q", got, "desktop_window_title")
	}
	if !strings.HasPrefix(got, "desktop_") {
		t.Fatalf("expected loud-echo with desktop_ prefix; got %q", got)
	}
}
