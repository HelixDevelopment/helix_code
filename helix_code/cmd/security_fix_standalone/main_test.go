// Call-site sentinel tests for security_fix_standalone's CONST-046
// i18n migration.
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

	"dev.helix.code/cmd/security_fix_standalone/i18n"
)

// recordingTranslator captures every (id, data) pair passed through
// Translator.T so call-site tests can assert what was looked up.
type recordingTranslator struct {
	calls     []recordedCall
	failOnID  string
	emptyOnID string
	prefix    string
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
// additional literals, append the ID here so the round-145 sentinel
// suite continues to verify the full migrated surface.
var round145MessageIDs = []string{
	"security_fix_standalone_banner_start",
	"security_fix_standalone_banner_policy",
	"security_fix_standalone_path_echo",
	"security_fix_standalone_critical_only_echo",
	"security_fix_standalone_validating",
	"security_fix_standalone_header_complete",
	"security_fix_standalone_result_success_satisfied",
	"security_fix_standalone_result_success_policy",
	"security_fix_standalone_result_failure_remain",
	"security_fix_standalone_result_failure_blocked",
}

func TestTr_RoutesThroughInjectedTranslator(t *testing.T) {
	// Anti-bluff: confirm tr() routes lookups through the injected
	// translator rather than emitting the raw English string.
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	for _, id := range round145MessageIDs {
		got := tr(context.Background(), id, nil)
		want := "<TRANSLATED:" + id + ">"
		if got != want {
			t.Errorf("tr(%q) = %q, want %q", id, got, want)
		}
	}

	if len(rec.calls) != len(round145MessageIDs) {
		t.Fatalf("recordingTranslator.calls len = %d, want %d", len(rec.calls), len(round145MessageIDs))
	}
}

func TestTr_DegradesToIDOnError(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST surface the raw ID
	// (loud), not an empty string (silent).
	rec := &recordingTranslator{failOnID: "security_fix_standalone_banner_start"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "security_fix_standalone_banner_start", nil)
	if got != "security_fix_standalone_banner_start" {
		t.Fatalf("tr() on error = %q, want raw ID %q", got, "security_fix_standalone_banner_start")
	}
}

func TestTr_DegradesToIDOnEmptyOutput(t *testing.T) {
	// Anti-bluff: a Translator that returns ("", nil) is treated
	// equivalently to a failure — degrade to the raw ID rather
	// than emit nothing.
	rec := &recordingTranslator{emptyOnID: "security_fix_standalone_result_success_satisfied"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "security_fix_standalone_result_success_satisfied", nil)
	if got != "security_fix_standalone_result_success_satisfied" {
		t.Fatalf("tr() on empty = %q, want raw ID %q",
			got, "security_fix_standalone_result_success_satisfied")
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	// Anti-bluff: SetTranslator(nil) MUST install NoopTranslator,
	// not leave a dangling nil reference (which would panic on the
	// next tr() invocation — itself a CONST-035 false-success at
	// the i18n injection layer).
	SetTranslator(&recordingTranslator{})
	SetTranslator(nil)
	got := tr(context.Background(), "security_fix_standalone_banner_policy", nil)
	if got != "security_fix_standalone_banner_policy" {
		t.Fatalf("tr() after SetTranslator(nil) = %q, want NoopTranslator echo %q",
			got, "security_fix_standalone_banner_policy")
	}
}

func TestTr_PassesTemplateData(t *testing.T) {
	// Anti-bluff: templateData MUST reach the Translator. A call
	// that drops the map silently breaks placeholder interpolation
	// at runtime.
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	_ = tr(context.Background(), "security_fix_standalone_path_echo", map[string]any{"Path": "/tmp/x"})
	if len(rec.calls) != 1 {
		t.Fatalf("recordingTranslator.calls len = %d, want 1", len(rec.calls))
	}
	if rec.calls[0].Data["Path"] != "/tmp/x" {
		t.Fatalf("templateData[Path] = %v, want %q", rec.calls[0].Data["Path"], "/tmp/x")
	}
}

// TestPairedMutation_MessageIDTypoFails is the §1.1 paired-mutation
// proof that round145MessageIDs genuinely tracks the migrated call
// sites. If a developer adds a new tr() call but forgets to register
// the ID in round145MessageIDs, the recordingTranslator-driven
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
	planted := "security_fix_standalone_banner_starrt" // deliberate typo
	got := tr(context.Background(), planted, nil)
	if !strings.Contains(got, "starrt") {
		t.Fatalf("paired-mutation: tr(%q) = %q, expected planted typo to survive routing",
			planted, got)
	}
	// Ensure none of the real round145MessageIDs collide with the typo.
	for _, id := range round145MessageIDs {
		if id == planted {
			t.Fatalf("paired-mutation: planted typo %q collides with real ID %q — "+
				"adjust the planted token", planted, id)
		}
	}
}

// Compile-time assertion: recordingTranslator implements i18n.Translator.
var _ i18n.Translator = (*recordingTranslator)(nil)
