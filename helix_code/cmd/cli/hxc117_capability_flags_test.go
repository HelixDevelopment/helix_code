// HXC-117 (Bug/High) — §11.4.115 RED->GREEN guard.
//
// The verifier decodes 6 CONST-040 capability flags (MCP/LSP/ACP/RAG/
// Skills/Plugins) onto verifier.VerifiedModel, but printVerifiedModels
// (the --list-models renderer) never displayed them. This test proves the
// defect (RED) and, after the fix, proves the flags reach the rendered
// CLI output sourced from the decoded VerifiedModel fields — never a
// hardcoded value (CONST-040 / BLUFF-002).
//
// RED_MODE=1 (default): the assertions below are expected to FAIL against
//
//	the pre-fix renderVerifiedModels/printVerifiedModels (no capability
//	line emitted). Captured manually before the fix landed (see
//	/tmp/hxc117_impl_report.md for the pre-fix failure transcript) since
//	RED_MODE only gates presence/absence assertions that would otherwise
//	need two code paths; here the guard body runs unconditionally and
//	simply demonstrates the fixed behaviour, matching the sibling D2 guard
//	style used elsewhere in this package for polarity-switch tests.
//
// RED_MODE=0: the GREEN guard — capability indicators for a MIXED
//
//	true/false fixture are all present and match the fixture (a constant
//	string could not pass this).
//
// Mocks/fakes NOT used — this exercises the real embedded i18n bundle.
// cmd/cli/i18n_boot_wire.go's package init() already wires a real
// clii18n.NewTranslator() into the package-level translator before any
// test runs (see TestI18nBootWire_ResolvesRealText), so this test relies
// on that boot-time wiring rather than calling SetTranslator itself --
// mutating global translator state here and restoring it to
// i18n.NoopTranslator{} in cleanup would regress every other test in this
// package that depends on the boot-wired real translator (verified: an
// earlier draft of this test did exactly that and broke
// TestI18nBootWire_ResolvesRealText).
package main

import (
	"strings"
	"testing"

	"dev.helix.code/internal/verifier"
)

// hxc117MixedCapabilityModel is a fixture with a MIXED true/false CONST-040
// capability profile so a hardcoded/constant renderer cannot pass this test.
func hxc117MixedCapabilityModel() *verifier.VerifiedModel {
	return &verifier.VerifiedModel{
		ID:                 "hxc117-model",
		Name:               "hxc117-model",
		DisplayName:        "HXC-117 Test Model",
		Provider:           "testprovider",
		Verified:           true,
		VerificationStatus: "verified",
		OverallScore:       9.1,
		ContextSize:        128000,
		SupportsMCP:        true,
		SupportsLSP:        false,
		SupportsACP:        true,
		SupportsRAG:        false,
		SupportsSkills:     true,
		SupportsPlugins:    false,
	}
}

// TestHXC117_CLI_PrintVerifiedModels_ShowsCapabilityFlags is the RED->GREEN
// guard for the CLI surface. GREEN asserts the rendered output contains a
// per-flag indicator matching the fixture's mixed true/false profile,
// sourced from the decoded VerifiedModel fields.
func TestHXC117_CLI_PrintVerifiedModels_ShowsCapabilityFlags(t *testing.T) {
	m := hxc117MixedCapabilityModel()
	out := renderVerifiedModels([]*verifier.VerifiedModel{m})

	if out == "" {
		t.Fatalf("renderVerifiedModels returned empty output for a working model")
	}

	// Sanity: the existing fields must still render (no regression).
	for _, want := range []string{m.ID, m.DisplayName, m.Provider} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered output missing pre-existing field %q; full output:\n%s", want, out)
		}
	}

	// The 6 CONST-040 capability flags, sourced from the fixture (mixed
	// true/false), must all be visible in the rendered output.
	cases := []struct {
		label     string
		supported bool
	}{
		{"MCP", m.SupportsMCP},
		{"LSP", m.SupportsLSP},
		{"ACP", m.SupportsACP},
		{"RAG", m.SupportsRAG},
		{"Skills", m.SupportsSkills},
		{"Plugins", m.SupportsPlugins},
	}
	for _, c := range cases {
		if !strings.Contains(out, c.label) {
			t.Fatalf("HXC-117 RED: rendered output does not mention capability label %q at all; full output:\n%s", c.label, out)
		}
	}

	// Exact-segment check: each flag's rendered "<label>:<indicator>"
	// segment must match the fixture's true/false value using the SAME
	// i18n-sourced indicator glyphs the renderer uses -- not merely "the
	// label is mentioned somewhere" (the loop above), and not a hardcoded
	// assumption about which glyph means supported/unsupported.
	supportedIndicator := trc("cli_capability_indicator_supported", nil)
	unsupportedIndicator := trc("cli_capability_indicator_unsupported", nil)
	skillsLabel := trc("cli_capability_label_skills", nil)
	pluginsLabel := trc("cli_capability_label_plugins", nil)
	segmentLabel := map[string]string{
		"MCP":     "MCP",
		"LSP":     "LSP",
		"ACP":     "ACP",
		"RAG":     "RAG",
		"Skills":  skillsLabel,
		"Plugins": pluginsLabel,
	}
	for _, c := range cases {
		indicator := unsupportedIndicator
		if c.supported {
			indicator = supportedIndicator
		}
		want := segmentLabel[c.label] + ":" + indicator
		if !strings.Contains(out, want) {
			t.Fatalf("HXC-117: expected capability segment %q not found; full output:\n%s", want, out)
		}
	}

	// Flip the fixture's polarity (ALL 6 flags at once) and confirm the
	// rendered text actually changes -- proves the renderer reads the live
	// fields rather than a hardcoded constant (CONST-040 / BLUFF-002
	// anti-bluff check).
	flipped := hxc117MixedCapabilityModel()
	flipped.SupportsMCP = !flipped.SupportsMCP
	flipped.SupportsLSP = !flipped.SupportsLSP
	flipped.SupportsACP = !flipped.SupportsACP
	flipped.SupportsRAG = !flipped.SupportsRAG
	flipped.SupportsSkills = !flipped.SupportsSkills
	flipped.SupportsPlugins = !flipped.SupportsPlugins
	flippedOut := renderVerifiedModels([]*verifier.VerifiedModel{flipped})
	if flippedOut == out {
		t.Fatalf("HXC-117: flipping every capability flag produced IDENTICAL output -- renderer is not sourcing from the decoded fields (hardcoded/BLUFF-002 pattern). output:\n%s", out)
	}

	// Per-flag anti-bluff (§11.4.125/§1.1 review finding): the whole-string
	// aggregate flip above is BLIND to a SINGLE hardcoded/miswired flag --
	// if e.g. LSP were hardcoded to always render "✗" while the other 5
	// flags flip correctly, the aggregate string still changes and the
	// check above still passes, silently hiding the LSP bug. Flip EACH
	// flag INDIVIDUALLY (leaving the other 5 at their mixed-fixture value)
	// and assert the output changes each time -- this makes a single
	// hardcoded/miswired flag fail the test on its own.
	for _, c := range cases {
		single := hxc117MixedCapabilityModel()
		switch c.label {
		case "MCP":
			single.SupportsMCP = !single.SupportsMCP
		case "LSP":
			single.SupportsLSP = !single.SupportsLSP
		case "ACP":
			single.SupportsACP = !single.SupportsACP
		case "RAG":
			single.SupportsRAG = !single.SupportsRAG
		case "Skills":
			single.SupportsSkills = !single.SupportsSkills
		case "Plugins":
			single.SupportsPlugins = !single.SupportsPlugins
		default:
			t.Fatalf("unhandled capability label %q in per-flag test table", c.label)
		}
		singleOut := renderVerifiedModels([]*verifier.VerifiedModel{single})
		if singleOut == out {
			t.Fatalf("HXC-117 §1.1: flipping ONLY %s produced IDENTICAL output to the mixed fixture -- that flag is not sourced from the decoded field (hardcoded/miswired), even though the aggregate all-flags-flipped check above passed", c.label)
		}

		// Also assert the flipped segment for THIS flag specifically
		// changed value (not just that *some* byte in the string differs).
		wantIndicator := supportedIndicator
		if c.supported {
			// c.supported was true in the base fixture; single flip makes
			// it false, so the flipped indicator is now "unsupported".
			wantIndicator = unsupportedIndicator
		}
		wantSegment := segmentLabel[c.label] + ":" + wantIndicator
		if !strings.Contains(singleOut, wantSegment) {
			t.Fatalf("HXC-117 §1.1: after flipping ONLY %s, expected segment %q not found in output; full output:\n%s", c.label, wantSegment, singleOut)
		}
	}
}

// TestHXC117_CLI_RenderFallbackModels_NoRawTemplateLeak is the RED->GREEN
// guard for the OTHER (untouched-by-the-original-fix) caller of the shared
// cli_model_info_verified i18n template: the offline-fallback path
// (Priority 3 in handleListModels, exercised when the verifier is
// unavailable -- CONST-035 constitutional fallback). The original HXC-117
// fix added a "Capabilities:" line to the shared template but only wired a
// value into the verified-models path, leaving this sibling call site
// rendering the Go text/template zero-value literal "<no value>" to real
// end users -- a §11.4.1 fix-A-breaks-B regression.
//
// RED (pre-fix, captured manually before the fallback fix landed): output
// contained the literal substring "<no value>".
// GREEN (post-fix): output never contains "<no value>" and instead carries
// the honest, i18n-sourced "capabilities unknown" indicator -- never a
// fabricated ✓/✗ (the verifier is unavailable on this path, so no per-flag
// CONST-040 data exists for fallback models).
func TestHXC117_CLI_RenderFallbackModels_NoRawTemplateLeak(t *testing.T) {
	models := []*verifier.VerifiedModel{
		{
			ID:           "hxc117-fallback-model",
			DisplayName:  "HXC-117 Fallback Test Model",
			Provider:     "testprovider",
			OverallScore: 5.0,
			ContextSize:  32000,
		},
	}
	out := renderFallbackModels(models)

	if out == "" {
		t.Fatalf("renderFallbackModels returned empty output for a fallback model")
	}
	if strings.Contains(out, "<no value>") {
		t.Fatalf("HXC-117 RED: fallback output leaks the raw Go text/template zero-value literal \"<no value>\" to end users (missing Capabilities template key); full output:\n%s", out)
	}

	wantUnknown := trc("cli_capability_flags_unknown", nil)
	if wantUnknown == "" || wantUnknown == "cli_capability_flags_unknown" {
		t.Fatalf("test setup: cli_capability_flags_unknown did not resolve through the real translator (got %q) -- cannot validate the fallback capabilities indicator", wantUnknown)
	}
	if !strings.Contains(out, wantUnknown) {
		t.Fatalf("HXC-117: fallback output missing the honest capabilities-unknown indicator %q; full output:\n%s", wantUnknown, out)
	}

	// Regression guard: pre-existing fields must still render.
	for _, want := range []string{models[0].ID, models[0].DisplayName, models[0].Provider, "unverified fallback"} {
		if !strings.Contains(out, want) {
			t.Fatalf("fallback output missing pre-existing field %q; full output:\n%s", want, out)
		}
	}
}
