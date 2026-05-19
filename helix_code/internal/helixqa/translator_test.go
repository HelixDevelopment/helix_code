// Unit tests for the internal/helixqa package-level translator +
// tr() helper (CONST-046 round-159 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package helixqa

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/internal/config"
	helixqai18n "dev.helix.code/internal/helixqa/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests
// can assert tr() actually went through Translator.T rather than
// returning a hardcoded literal that happened to match the bundle
// value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_helixqa_qa_disabled", nil)
	if got != "internal_helixqa_qa_disabled" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_helixqa_session_id_required", nil)
	if got != "<TR:internal_helixqa_session_id_required>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade
	// to the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_helixqa_no_report_available", nil)
	if got != "internal_helixqa_no_report_available" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_helixqa_bank_not_found", map[string]any{"Bank": "/foo"})
	if got != "internal_helixqa_bank_not_found" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(helixqai18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_helixqa_qa_disabled", nil)
	if got != "internal_helixqa_qa_disabled" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestStartSession_QADisabled_GoesThroughTranslator asserts the
// "QA is disabled" guard error returned from StartSession surfaces
// through tr() — proving the literal is NOT hardcoded on the path.
func TestStartSession_QADisabled_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cfg := &config.Config{QA: config.QAConfig{Enabled: false}}
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine(disabled) returned err: %v", err)
	}
	_, err = engine.StartSession(stdctx.Background(), "s", nil, nil, false)
	if err == nil {
		t.Fatal("StartSession on disabled engine returned no error")
	}
	want := "<TR:internal_helixqa_qa_disabled>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("StartSession err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}

// TestStartSession_QADisabled_RawTextByDefault asserts the
// Noop-default surface for the disabled-guard path.
func TestStartSession_QADisabled_RawTextByDefault(t *testing.T) {
	resetTranslator(t)

	cfg := &config.Config{QA: config.QAConfig{Enabled: false}}
	engine, _ := NewEngine(cfg)
	_, err := engine.StartSession(stdctx.Background(), "s", nil, nil, false)
	if err == nil {
		t.Fatal("StartSession on disabled engine returned no error")
	}
	if !strings.Contains(err.Error(), "internal_helixqa_qa_disabled") {
		t.Fatalf("StartSession err = %q, want raw message ID (Noop echo)", err.Error())
	}
}

// TestCancelSession_NotFound_GoesThroughTranslator covers the
// "session %s not found" path on CancelSession. With sentinel wired,
// the surfaced error MUST contain the sentinel-wrapped message ID.
func TestCancelSession_NotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	tmpDir := t.TempDir()
	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine returned err: %v", err)
	}
	err = engine.CancelSession("nonexistent-session")
	if err == nil {
		t.Fatal("CancelSession on missing session returned no error")
	}
	want := "<TR:internal_helixqa_session_not_found>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("CancelSession err = %q, want contain %q — call site bypassed tr()", err.Error(), want)
	}
}
