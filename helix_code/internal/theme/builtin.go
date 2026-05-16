package theme

// Built-in themes.
//
// The byte sequences returned by these constructors are pinned by spec
// §3.4 (docs/superpowers/specs/2026-05-06-p1-f20-theme-system-design.md).
// They are NOT auto-derived between depth tiers — a theme designer hand-
// picks codes per (role × depth) so a Truecolor palette is not quantised
// from ANSI16 (which would lose information) or warped from RGB (which
// would shift warm/cool tones). Spec §3.3 records this decision.
//
// Every constructor returns a FRESH Theme value with a freshly-allocated
// Colors map. Callers may mutate the returned theme without affecting
// subsequent calls.

// BuiltinDarkTheme returns the dark-mode palette. Bytes pinned to spec
// §3.4 (dark theme table — biased toward bright tones that read on dark
// terminals).
func BuiltinDarkTheme() Theme {
	return Theme{
		Name: ThemeDark,
		Colors: map[Role]Color{
			RoleInfo: {
				OpenANSI16:    "\x1b[37m",
				OpenANSI256:   "\x1b[38;5;250m",
				OpenTruecolor: "\x1b[38;2;220;220;220m",
			},
			RoleWarn: {
				OpenANSI16:    "\x1b[33m",
				OpenANSI256:   "\x1b[38;5;214m",
				OpenTruecolor: "\x1b[38;2;255;176;0m",
			},
			RoleError: {
				OpenANSI16:    "\x1b[31m",
				OpenANSI256:   "\x1b[38;5;196m",
				OpenTruecolor: "\x1b[38;2;255;64;64m",
			},
			RoleHighlight: {
				OpenANSI16:    "\x1b[36m",
				OpenANSI256:   "\x1b[38;5;51m",
				OpenTruecolor: "\x1b[38;2;0;200;220m",
			},
			RoleDim: {
				OpenANSI16:    "\x1b[90m",
				OpenANSI256:   "\x1b[38;5;243m",
				OpenTruecolor: "\x1b[38;2;128;128;128m",
			},
		},
	}
}

// BuiltinLightTheme returns the light-mode palette. Bytes pinned to spec
// §3.4 (light theme table — biased toward darker tones that read on light
// terminals).
func BuiltinLightTheme() Theme {
	return Theme{
		Name: ThemeLight,
		Colors: map[Role]Color{
			RoleInfo: {
				OpenANSI16:    "\x1b[30m",
				OpenANSI256:   "\x1b[38;5;235m",
				OpenTruecolor: "\x1b[38;2;40;40;40m",
			},
			RoleWarn: {
				OpenANSI16:    "\x1b[33m",
				OpenANSI256:   "\x1b[38;5;130m",
				OpenTruecolor: "\x1b[38;2;175;95;0m",
			},
			RoleError: {
				OpenANSI16:    "\x1b[31m",
				OpenANSI256:   "\x1b[38;5;124m",
				OpenTruecolor: "\x1b[38;2;175;0;0m",
			},
			RoleHighlight: {
				OpenANSI16:    "\x1b[34m",
				OpenANSI256:   "\x1b[38;5;25m",
				OpenTruecolor: "\x1b[38;2;0;95;175m",
			},
			RoleDim: {
				OpenANSI16:    "\x1b[37m",
				OpenANSI256:   "\x1b[38;5;245m",
				OpenTruecolor: "\x1b[38;2;138;138;138m",
			},
		},
	}
}

// BuiltinNoneTheme returns the no-color theme. Per spec §3.4, every role's
// Color has empty Open slots at every depth, so Stylize(role, text)
// returns text unchanged regardless of ColorDepth. Distinct from
// DepthOff: "none" means "TTY but no color" (e.g. piping through less -R
// is fine, but the user prefers monochrome); DepthOff means "the
// terminal can't render ANSI".
//
// Implementation choice: returns Colors as an empty (but non-nil) map
// rather than nil. Theme.ColorFor handles both, but a non-nil empty map
// makes range-over-Colors loops behave identically across all three
// built-ins and avoids special-casing in callers that introspect the
// map.
func BuiltinNoneTheme() Theme {
	return Theme{
		Name:   ThemeNone,
		Colors: map[Role]Color{},
	}
}

// AllBuiltinThemes returns the three built-in themes in canonical order:
// dark, light, none. The returned slice and each Theme's Colors map are
// fresh on every call; callers may mutate them freely.
func AllBuiltinThemes() []Theme {
	return []Theme{
		BuiltinDarkTheme(),
		BuiltinLightTheme(),
		BuiltinNoneTheme(),
	}
}
