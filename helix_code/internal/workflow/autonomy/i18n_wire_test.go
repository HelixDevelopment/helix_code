// i18n_wire_test.go — HXC-036 Phase 4 anti-bluff acceptance test for the
// internal/workflow/autonomy CONST-046 wiring.
//
// This test exercises the REAL render path (AutonomyMode.String(), which
// resolves through the package-level tr() seam) AFTER injecting the real
// translator built from the embedded internal/workflow/i18n bundle — the
// exact construction i18nwiring.WireAll performs. It asserts that the
// rendered label is the RESOLVED English text, NOT the raw message-ID key.
// Without the bundle.go constructor + wiring this would echo
// "internal_workflow_mode_full_auto_label" — the §11.4 PASS-bluff this
// phase closes.
package autonomy

import (
	"testing"

	workflowi18n "dev.helix.code/internal/workflow/i18n"
)

func TestAutonomyMode_String_ResolvesRealBundleText(t *testing.T) {
	// Build + inject the real translator exactly as i18nwiring.WireAll does.
	tr, err := workflowi18n.NewTranslator()
	if err != nil {
		t.Fatalf("workflowi18n.NewTranslator() failed: %v", err)
	}
	SetTranslator(tr)
	t.Cleanup(func() { SetTranslator(nil) }) // restore NoopTranslator

	cases := []struct {
		mode AutonomyMode
		want string
	}{
		{ModeNone, "None (Manual Control)"},
		{ModeBasic, "Basic (Manual Steps)"},
		{ModeBasicPlus, "Basic Plus (Smart Semi-Automation)"},
		{ModeSemiAuto, "Semi Auto (Automated with Approval)"},
		{ModeFullAuto, "Full Auto (Fully Autonomous)"},
	}
	for _, c := range cases {
		got := c.mode.String()
		if got != c.want {
			t.Errorf("AutonomyMode(%s).String() = %q, want resolved %q", string(c.mode), got, c.want)
		}
		// Anti-bluff: the raw message-ID key must NOT leak through.
		if got == "internal_workflow_mode_none_label" ||
			got == "internal_workflow_mode_full_auto_label" {
			t.Errorf("AutonomyMode(%s).String() returned raw message-ID key %q — translator not wired", string(c.mode), got)
		}
	}

	// Capture one resolved label for evidence.
	t.Logf("RESOLVED autonomy label: ModeFullAuto.String() = %q", ModeFullAuto.String())
}
