package theme

import "testing"

// envFunc builds a closure that returns values from a fixed map. Keys
// not present in the map yield "" — mimicking os.Getenv semantics for
// unset variables.
func envFunc(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

// ----------------------------------------------------------------------
// DetectThemeName
// ----------------------------------------------------------------------

func TestDetectThemeName_HELIXCODE_THEME_Dark(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{
		"HELIXCODE_THEME": "dark",
	}))
	if got != ThemeDark {
		t.Fatalf("want %q, got %q", ThemeDark, got)
	}
}

func TestDetectThemeName_HELIXCODE_THEME_Light(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{
		"HELIXCODE_THEME": "light",
	}))
	if got != ThemeLight {
		t.Fatalf("want %q, got %q", ThemeLight, got)
	}
}

func TestDetectThemeName_HELIXCODE_THEME_None(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{
		"HELIXCODE_THEME": "none",
	}))
	if got != ThemeNone {
		t.Fatalf("want %q, got %q", ThemeNone, got)
	}
}

func TestDetectThemeName_HELIXCODE_THEME_Garbage_FallsThrough(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{
		"HELIXCODE_THEME": "banana",
	}))
	if got != ThemeDark {
		t.Fatalf("garbage HELIXCODE_THEME with no other signal should default to %q, got %q",
			ThemeDark, got)
	}
}

func TestDetectThemeName_COLORFGBG_LightBg(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{
		"COLORFGBG": "0;15",
	}))
	if got != ThemeLight {
		t.Fatalf("COLORFGBG=0;15 (white bg) should yield %q, got %q", ThemeLight, got)
	}
}

func TestDetectThemeName_COLORFGBG_DarkBg(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{
		"COLORFGBG": "15;0",
	}))
	if got != ThemeDark {
		t.Fatalf("COLORFGBG=15;0 (black bg) should yield %q, got %q", ThemeDark, got)
	}
}

func TestDetectThemeName_COLORFGBG_Default_FallsThroughToDefault(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{
		"COLORFGBG": "default;default",
	}))
	if got != ThemeDark {
		t.Fatalf("COLORFGBG=default;default should fall through to default %q, got %q",
			ThemeDark, got)
	}
}

func TestDetectThemeName_AllUnset_DefaultDark(t *testing.T) {
	got := DetectThemeName(envFunc(map[string]string{}))
	if got != ThemeDark {
		t.Fatalf("all env unset should yield default %q, got %q", ThemeDark, got)
	}
}

func TestDetectThemeName_HELIXCODE_THEME_BeatsCOLORFGBG(t *testing.T) {
	// HELIXCODE_THEME=light; COLORFGBG=15;0 (would say dark) — operator
	// override wins.
	got := DetectThemeName(envFunc(map[string]string{
		"HELIXCODE_THEME": "light",
		"COLORFGBG":       "15;0",
	}))
	if got != ThemeLight {
		t.Fatalf("HELIXCODE_THEME must beat COLORFGBG; want %q, got %q", ThemeLight, got)
	}
}

// ----------------------------------------------------------------------
// DetectColorDepth
// ----------------------------------------------------------------------

func TestDetectColorDepth_NoColor_OverridesAll(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"NO_COLOR":  "1",
		"COLORTERM": "truecolor",
		"TERM":      "xterm-256color",
	}))
	if got != DepthOff {
		t.Fatalf("NO_COLOR set must override everything to %v, got %v", DepthOff, got)
	}
}

func TestDetectColorDepth_NoColor_EmptyDoesNotForce(t *testing.T) {
	// Per NO_COLOR.org: only set when value is non-empty.
	got := DetectColorDepth(envFunc(map[string]string{
		"NO_COLOR":  "",
		"COLORTERM": "truecolor",
		"TERM":      "xterm-256color",
	}))
	if got != DepthTruecolor {
		t.Fatalf("empty NO_COLOR must not force off; want %v, got %v", DepthTruecolor, got)
	}
}

func TestDetectColorDepth_COLORTERM_Truecolor(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"COLORTERM": "truecolor",
		"TERM":      "xterm-256color",
	}))
	if got != DepthTruecolor {
		t.Fatalf("COLORTERM=truecolor should yield %v, got %v", DepthTruecolor, got)
	}
}

func TestDetectColorDepth_COLORTERM_24bit(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"COLORTERM": "24bit",
		"TERM":      "xterm-256color",
	}))
	if got != DepthTruecolor {
		t.Fatalf("COLORTERM=24bit should yield %v, got %v", DepthTruecolor, got)
	}
}

func TestDetectColorDepth_COLORTERM_OtherValue_DoesNotForceTrue(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"COLORTERM": "rxvt",
		"TERM":      "xterm-256color",
	}))
	if got != DepthANSI256 {
		t.Fatalf("COLORTERM=rxvt + TERM=xterm-256color should yield %v, got %v",
			DepthANSI256, got)
	}
}

func TestDetectColorDepth_TERM_256color(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"TERM": "xterm-256color",
	}))
	if got != DepthANSI256 {
		t.Fatalf("TERM=xterm-256color should yield %v, got %v", DepthANSI256, got)
	}
}

func TestDetectColorDepth_TERM_xtermNo256(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"TERM": "xterm",
	}))
	if got != DepthANSI16 {
		t.Fatalf("TERM=xterm should yield %v, got %v", DepthANSI16, got)
	}
}

func TestDetectColorDepth_TERM_Dumb(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"TERM": "dumb",
	}))
	if got != DepthOff {
		t.Fatalf("TERM=dumb should yield %v, got %v", DepthOff, got)
	}
}

func TestDetectColorDepth_TERM_Empty(t *testing.T) {
	got := DetectColorDepth(envFunc(map[string]string{
		"TERM": "",
	}))
	if got != DepthOff {
		t.Fatalf("TERM=\"\" should yield %v, got %v", DepthOff, got)
	}
}

func TestDetectColorDepth_AllUnset(t *testing.T) {
	// No env at all: no TERM signal → DepthOff.
	got := DetectColorDepth(envFunc(map[string]string{}))
	if got != DepthOff {
		t.Fatalf("all env unset should yield %v, got %v", DepthOff, got)
	}
}

// ----------------------------------------------------------------------
// parseCOLORFGBG
// ----------------------------------------------------------------------

func TestParseCOLORFGBG_DarkBg(t *testing.T) {
	got := parseCOLORFGBG("15;0")
	if got != ThemeDark {
		t.Fatalf("15;0 should yield %q, got %q", ThemeDark, got)
	}
}

func TestParseCOLORFGBG_LightBg(t *testing.T) {
	got := parseCOLORFGBG("0;15")
	if got != ThemeLight {
		t.Fatalf("0;15 should yield %q, got %q", ThemeLight, got)
	}
}

func TestParseCOLORFGBG_HighIndexLight(t *testing.T) {
	got := parseCOLORFGBG("0;230")
	if got != ThemeLight {
		t.Fatalf("0;230 (high bg index) should yield %q, got %q", ThemeLight, got)
	}
}

func TestParseCOLORFGBG_Empty(t *testing.T) {
	got := parseCOLORFGBG("")
	if got != "" {
		t.Fatalf("empty should yield \"\", got %q", got)
	}
}

func TestParseCOLORFGBG_Garbage_NotNumeric(t *testing.T) {
	got := parseCOLORFGBG("fg;bg")
	if got != "" {
		t.Fatalf("non-numeric should yield \"\", got %q", got)
	}
}

func TestParseCOLORFGBG_MissingSemicolon(t *testing.T) {
	got := parseCOLORFGBG("0")
	if got != "" {
		t.Fatalf("missing semicolon should yield \"\", got %q", got)
	}
}

func TestParseCOLORFGBG_DefaultDefault(t *testing.T) {
	got := parseCOLORFGBG("default;default")
	if got != "" {
		t.Fatalf("default;default should yield \"\", got %q", got)
	}
}
