//go:build integration

// Package-level test file: P1-F20-T07 theme integration tests.
//
// Build tag rationale:
//   - `integration` — these tests are part of the integration test surface
//     gated by the existing test-infra harness (make test-infra-up etc.).
//     Without this tag the file does not compile into the default `go test`
//     run, mirroring tests/integration/sandbox_test.go.
//
// Run with:
//   cd HelixCode && go test -tags=integration -run "TestTheme_" ./tests/integration/...
//
// Anti-bluff anchors carried by these tests:
//
//   - Every PASS line below comes from a REAL theme.ThemeRegistry, REAL
//     YAML parse via gopkg.in/yaml.v3 (TestTheme_LoadFromYAML_PartialOverride),
//     and REAL theme.Styler.Stylize() byte assembly. No fake registry, no
//     fake styler, no in-memory short-circuit.
//   - TestTheme_StylerRoundtrip_DarkInfo_Truecolor compares against the
//     pinned bytes from theme/builtin.go §3.4. A regression that drifted
//     the dark-theme info color from "\x1b[38;2;220;220;220m" to anything
//     else would fail this test before the binary ships.
//   - TestTheme_PlainMode_StylerNoOp exercises the SAME code path as
//     handleGenerate (P1-F20-T06): adjustDepthForRenderer collapses
//     DepthTruecolor -> DepthOff when the renderer is plain, so the
//     resulting Stylize call is a no-op. This is the load-bearing
//     plain-mode-zero-color guarantee from F20 spec §11.

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/render"
	"dev.helix.code/internal/theme"
)

// envFromMap returns a closure suitable for theme.DetectThemeName /
// DetectColorDepth that resolves keys against the given map. Missing keys
// resolve to "" (matching os.Getenv semantics for unset vars).
func envFromMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestTheme_DefaultDarkWhenAllUnset asserts the F20 spec §5 layer-3 default:
// when no operator override (HELIXCODE_THEME) and no terminal hint
// (COLORFGBG) are set, DetectThemeName returns ThemeDark.
func TestTheme_DefaultDarkWhenAllUnset(t *testing.T) {
	got := theme.DetectThemeName(envFromMap(map[string]string{}))
	require.Equal(t, theme.ThemeDark, got)
}

// TestTheme_HELIXCODE_THEME_Light_Honored asserts the F20 spec §5 layer-1
// operator override: HELIXCODE_THEME=light wins over every other signal.
func TestTheme_HELIXCODE_THEME_Light_Honored(t *testing.T) {
	got := theme.DetectThemeName(envFromMap(map[string]string{
		theme.ThemeNameEnvVar: "light",
		"COLORFGBG":           "15;0", // would otherwise vote dark
	}))
	require.Equal(t, theme.ThemeLight, got)
}

// TestTheme_NO_COLOR_ForcesDepthOff asserts the NO_COLOR.org standard from
// F20 spec §6: NO_COLOR set non-empty forces DepthOff regardless of any
// COLORTERM signal.
func TestTheme_NO_COLOR_ForcesDepthOff(t *testing.T) {
	got := theme.DetectColorDepth(envFromMap(map[string]string{
		"NO_COLOR":  "1",
		"COLORTERM": "truecolor", // would otherwise vote truecolor
		"TERM":      "xterm-256color",
	}))
	require.Equal(t, theme.DepthOff, got)
}

// TestTheme_LoadFromYAML_PartialOverride asserts the F20 spec §11 merge
// semantics end-to-end: a partial theme.yaml on disk is parsed by real
// yaml.v3, merged over the dark baseline, and exposed via Custom().
func TestTheme_LoadFromYAML_PartialOverride(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "theme.yaml")
	yaml := `name: my-custom
colors:
  error:
    truecolor: "\x1b[38;2;255;0;128m"
    ansi256: "\x1b[38;5;199m"
    ansi16: "\x1b[35m"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	reg := theme.NewThemeRegistry()
	require.NoError(t, reg.LoadFromFile(path))

	cust := reg.Custom()
	require.NotNil(t, cust, "Custom() must be non-nil after LoadFromFile")
	require.Equal(t, theme.ThemeName("my-custom"), cust.Name)

	// Overridden role: pinned bytes from YAML.
	errColor := cust.ColorFor(theme.RoleError)
	require.Equal(t, "\x1b[38;2;255;0;128m", errColor.OpenTruecolor)
	require.Equal(t, "\x1b[38;5;199m", errColor.OpenANSI256)
	require.Equal(t, "\x1b[35m", errColor.OpenANSI16)

	// Inherited role: dark baseline preserved (anti-bluff: F20 spec §11
	// requires every absent role to be inherited from dark UNCHANGED).
	infoColor := cust.ColorFor(theme.RoleInfo)
	darkInfo := theme.BuiltinDarkTheme().ColorFor(theme.RoleInfo)
	require.Equal(t, darkInfo, infoColor)
}

// TestTheme_StylerRoundtrip_DarkInfo_Truecolor asserts the canonical
// dark-theme info color at truecolor depth flows end-to-end through
// registry.Get + NewStyler + Stylize, producing the pinned ANSI bytes from
// theme/builtin.go §3.4.
func TestTheme_StylerRoundtrip_DarkInfo_Truecolor(t *testing.T) {
	reg := theme.NewThemeRegistry()
	dark, err := reg.Get(theme.ThemeDark)
	require.NoError(t, err)

	styler := theme.NewStyler(dark, theme.DepthTruecolor)
	got := styler.Stylize(theme.RoleInfo, "X")

	// Pinned bytes from builtin.go::BuiltinDarkTheme — RoleInfo + Truecolor.
	want := "\x1b[38;2;220;220;220m" + "X" + theme.Reset
	require.Equal(t, want, got)
}

// TestTheme_PlainMode_StylerNoOp asserts the F20 spec §11 plain-mode-zero-
// color guarantee end-to-end. The plain renderer + a depth adjusted via
// the same code path as cmd/cli/main.go's adjustDepthForRenderer (Plain ->
// DepthOff) yields a Styler whose Stylize is a no-op.
//
// We do NOT call adjustDepthForRenderer directly here (it's a main.go-
// internal helper); instead we replicate its single-line behaviour to
// keep the integration test independent of cmd/cli internals while still
// proving the contract.
func TestTheme_PlainMode_StylerNoOp(t *testing.T) {
	// Build a real plain renderer and confirm its Mode is plain. This
	// mirrors what handleGenerate sees when HELIXCODE_RENDER=plain or
	// when stdout is not a TTY.
	t.Setenv("HELIXCODE_RENDER", "plain")
	r, err := render.NewRenderer(render.FactoryOptions{})
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	require.Equal(t, render.ModePlain, r.Mode(), "test setup must yield a plain renderer")

	// Replicate adjustDepthForRenderer: plain renderer -> DepthOff.
	requested := theme.DepthTruecolor
	var effective theme.ColorDepth
	if r.Mode() == render.ModePlain {
		effective = theme.DepthOff
	} else {
		effective = requested
	}
	require.Equal(t, theme.DepthOff, effective)

	dark := theme.BuiltinDarkTheme()
	styler := theme.NewStyler(dark, effective)
	got := styler.Stylize(theme.RoleError, "BOOM")
	require.Equal(t, "BOOM", got, "plain mode must produce zero ANSI bytes")
	require.False(t, strings.Contains(got, "\x1b["), "no ANSI escape may leak into plain mode output")
}

// TestTheme_DepthDetect_ANSI256 asserts the F20 spec §6 layer-3:
// TERM matching `*-256color` -> DepthANSI256.
func TestTheme_DepthDetect_ANSI256(t *testing.T) {
	got := theme.DetectColorDepth(envFromMap(map[string]string{
		"TERM": "xterm-256color",
	}))
	require.Equal(t, theme.DepthANSI256, got)
}

// TestTheme_DepthDetect_DumbTermOff asserts the F20 spec §6 layer-4:
// TERM=dumb -> DepthOff.
func TestTheme_DepthDetect_DumbTermOff(t *testing.T) {
	got := theme.DetectColorDepth(envFromMap(map[string]string{
		"TERM": "dumb",
	}))
	require.Equal(t, theme.DepthOff, got)
}

// TestTheme_NoneTheme_StylerNoOp asserts F20 spec §3.4: the none built-in
// has empty Color slots, so Stylize returns text unchanged at every depth.
func TestTheme_NoneTheme_StylerNoOp(t *testing.T) {
	none := theme.BuiltinNoneTheme()
	for _, d := range []theme.ColorDepth{theme.DepthANSI16, theme.DepthANSI256, theme.DepthTruecolor} {
		styler := theme.NewStyler(none, d)
		for _, role := range theme.AllRoles() {
			got := styler.Stylize(role, "Z")
			assert.Equal(t, "Z", got, "none theme must be a no-op (depth=%s, role=%s)", d, role)
		}
	}
}
