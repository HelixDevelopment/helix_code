package theme

import (
	"strconv"
	"strings"
)

// ThemeNameEnvVar is the operator-controlled override.
const ThemeNameEnvVar = "HELIXCODE_THEME"

// DetectThemeName resolves which theme to use:
//  1. envLookup(HELIXCODE_THEME) → if "dark"/"light"/"none", use it.
//  2. parseCOLORFGBG(envLookup("COLORFGBG")) → if parses to dark or
//     light, use it.
//  3. Default: ThemeDark.
//
// Pure function. envLookup is typically os.Getenv; tests inject a
// closure.
//
// Garbage HELIXCODE_THEME values (e.g., "banana") fall through to the
// next signal rather than erroring (graceful degrade per spec §5).
func DetectThemeName(envLookup func(string) string) ThemeName {
	// Layer 1 — operator override.
	if raw := envLookup(ThemeNameEnvVar); raw != "" {
		switch ThemeName(raw) {
		case ThemeDark, ThemeLight, ThemeNone:
			return ThemeName(raw)
		}
		// Unrecognised value → fall through (do NOT error).
	}

	// Layer 2 — opportunistic COLORFGBG parse.
	if name := parseCOLORFGBG(envLookup("COLORFGBG")); name != "" {
		return name
	}

	// Layer 3 — default.
	return ThemeDark
}

// DetectColorDepth resolves the terminal's color capability:
//  1. envLookup("NO_COLOR") set non-empty → DepthOff (NO_COLOR.org
//     standard).
//  2. envLookup("COLORTERM") in {"truecolor","24bit"} → DepthTruecolor.
//  3. envLookup("TERM") matches `*-256color` → DepthANSI256.
//  4. envLookup("TERM") set to "dumb" or empty → DepthOff.
//  5. envLookup("TERM") set otherwise → DepthANSI16.
func DetectColorDepth(envLookup func(string) string) ColorDepth {
	// Layer 1 — NO_COLOR overrides everything when non-empty.
	if envLookup("NO_COLOR") != "" {
		return DepthOff
	}

	// Layer 2 — COLORTERM truecolor signal.
	switch envLookup("COLORTERM") {
	case "truecolor", "24bit":
		return DepthTruecolor
	}

	// Layer 3+ — TERM-based decision.
	term := envLookup("TERM")
	if term == "" || term == "dumb" {
		return DepthOff
	}
	if strings.HasSuffix(term, "-256color") {
		return DepthANSI256
	}
	return DepthANSI16
}

// parseCOLORFGBG attempts to extract dark/light from the standard
// "fg;bg" format. Per spec §11, this is OPPORTUNISTIC — malformed
// values fall through silently.
//
// Convention: bg index >= 8 → light theme (light bg); bg index < 8 →
// dark. Common XTerm convention: e.g., "default;default" → unknown;
// "15;0" → dark; "0;15" → light.
//
// Returns "" (empty ThemeName) if cannot parse.
func parseCOLORFGBG(value string) ThemeName {
	if value == "" {
		return ""
	}

	parts := strings.Split(value, ";")
	if len(parts) < 2 {
		return ""
	}

	// Background index is the LAST field. Some terminals emit three
	// fields (fg;cursor;bg); the last is still the background.
	bgRaw := parts[len(parts)-1]
	bg, err := strconv.Atoi(bgRaw)
	if err != nil {
		return ""
	}

	if bg >= 8 {
		return ThemeLight
	}
	return ThemeDark
}
