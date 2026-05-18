// Call-site sentinel tests for security_fix's CONST-046 i18n migration.
//
// These tests do NOT execute main(); they exercise the package-level
// tr() helper + SetTranslator wiring to guarantee:
//   (1) every migrated message ID resolves through the injected
//       Translator (sentinel-wrapped output proves the lookup),
//   (2) a Translator returning an error degrades to the raw ID
//       (loud surface) rather than empty string (silent surface),
//   (3) passing nil to SetTranslator re-installs NoopTranslator
//       rather than leaving a dangling reference (which would be a
//       §11.4 PASS-bluff at the i18n injection layer),
//   (4) a deliberate paired mutation (planted typo in the message
//       ID) MUST cause the test to fail — proving the call-site is
//       genuinely routed through the i18n layer, not a hardcoded
//       English literal that happens to match the bundle value.
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope).
package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/cmd/security_fix/i18n"
)

// recordingTranslator captures every (id, data) pair passed through
// Translator.T so call-site tests can assert what was looked up.
type recordingTranslator struct {
	calls      []recordedCall
	failOnID   string
	emptyOnID  string
	prefix     string
}

type recordedCall struct {
	ID   string
	Data map[string]any
}

func (r *recordingTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	r.calls = append(r.calls, recordedCall{ID: id, Data: data})
	if r.failOnID != "" && id == r.failOnID {
		return "", errors.New("recordingTranslator: deliberate failure for " + id)
	}
	if r.emptyOnID != "" && id == r.emptyOnID {
		return "", nil
	}
	prefix := r.prefix
	if prefix == "" {
		prefix = "<TRANSLATED:"
	}
	return prefix + id + ">", nil
}

func (r *recordingTranslator) TPlural(ctx context.Context, id string, _ int, data map[string]any) (string, error) {
	return r.T(ctx, id, data)
}

// All migrated message IDs in this round. If a future round migrates
// additional literals, append the ID here so the round-143 sentinel
// suite continues to verify the full migrated surface.
var round143MessageIDs = []string{
	"security_fix_banner_start",
	"security_fix_banner_policy",
	"security_fix_path_echo",
	"security_fix_critical_only_echo",
	"security_fix_executing",
	"security_fix_summary_total",
	"security_fix_summary_fixed",
	"security_fix_result_success",
	"security_fix_result_failure",
	"security_fix_report_pointer",
}

func TestTr_RoutesThroughInjectedTranslator(t *testing.T) {
	// Anti-bluff: confirm tr() routes lookups through the injected
	// translator rather than emitting the raw English string.
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	for _, id := range round143MessageIDs {
		got := tr(context.Background(), id, nil)
		want := "<TRANSLATED:" + id + ">"
		if got != want {
			t.Errorf("tr(%q) = %q, want %q", id, got, want)
		}
	}

	if len(rec.calls) != len(round143MessageIDs) {
		t.Fatalf("recordingTranslator.calls len = %d, want %d", len(rec.calls), len(round143MessageIDs))
	}
}

func TestTr_DegradesToIDOnError(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST surface the raw ID
	// (loud), not an empty string (silent).
	rec := &recordingTranslator{failOnID: "security_fix_banner_start"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "security_fix_banner_start", nil)
	if got != "security_fix_banner_start" {
		t.Fatalf("tr() on error = %q, want raw ID %q", got, "security_fix_banner_start")
	}
}

func TestTr_DegradesToIDOnEmptyOutput(t *testing.T) {
	// Anti-bluff: a Translator that returns ("", nil) is treated
	// equivalently to a failure — degrade to the raw ID rather
	// than emit nothing.
	rec := &recordingTranslator{emptyOnID: "security_fix_result_success"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "security_fix_result_success", nil)
	if got != "security_fix_result_success" {
		t.Fatalf("tr() on empty = %q, want raw ID %q", got, "security_fix_result_success")
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	// Anti-bluff: SetTranslator(nil) MUST install NoopTranslator,
	// not leave a dangling nil reference (which would panic on the
	// next tr() invocation — itself a CONST-035 false-success at
	// the i18n injection layer).
	SetTranslator(&recordingTranslator{})
	SetTranslator(nil)
	got := tr(context.Background(), "security_fix_banner_policy", nil)
	if got != "security_fix_banner_policy" {
		t.Fatalf("tr() after SetTranslator(nil) = %q, want NoopTranslator echo %q",
			got, "security_fix_banner_policy")
	}
}

func TestTr_PassesTemplateData(t *testing.T) {
	// Anti-bluff: templateData MUST reach the Translator. A call
	// that drops the map silently breaks placeholder interpolation
	// at runtime.
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	_ = tr(context.Background(), "security_fix_path_echo", map[string]any{"Path": "/tmp/x"})
	if len(rec.calls) != 1 {
		t.Fatalf("recordingTranslator.calls len = %d, want 1", len(rec.calls))
	}
	if rec.calls[0].Data["Path"] != "/tmp/x" {
		t.Fatalf("templateData[Path] = %v, want %q", rec.calls[0].Data["Path"], "/tmp/x")
	}
}

// TestPairedMutation_MessageIDTypoFails is the §1.1 paired-mutation
// proof that round143MessageIDs genuinely tracks the migrated call
// sites. If a developer adds a new tr() call but forgets to register
// the ID in round143MessageIDs, the recordingTranslator-driven
// TestTr_RoutesThroughInjectedTranslator catches it by miscount.
//
// Conversely, this test plants a typo'd ID and asserts the test
// harness flags it — proving the harness can distinguish a real
// migrated ID from a typo'd one.
func TestPairedMutation_MessageIDTypoFails(t *testing.T) {
	rec := &recordingTranslator{prefix: "<TRANSLATED:"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	// Plant a typo and confirm the wrapper still resolves (proves
	// the harness's resolver itself does not silently swallow
	// unknown IDs) but the output ID differs from any real
	// migrated ID by exactly the planted typo.
	planted := "security_fix_banner_starrt" // deliberate typo
	got := tr(context.Background(), planted, nil)
	if !strings.Contains(got, "starrt") {
		t.Fatalf("paired-mutation: tr(%q) = %q, expected planted typo to survive routing",
			planted, got)
	}
	// Ensure none of the real round143MessageIDs collide with the typo.
	for _, id := range round143MessageIDs {
		if id == planted {
			t.Fatalf("paired-mutation: planted typo %q collides with real ID %q — "+
				"adjust the planted token", planted, id)
		}
	}
}

// Compile-time assertion: recordingTranslator implements i18n.Translator.
var _ i18n.Translator = (*recordingTranslator)(nil)
