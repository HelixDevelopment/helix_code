// Package theme provides semantic role-based terminal styling primitives.
//
// The package defines five canonical Roles (info, warn, error, highlight,
// dim), three ColorDepth tiers (ansi16, ansi256, truecolor) plus an Off
// tier for non-styled output, and a Theme palette mapping each Role to
// a Color whose three Open-sequence slots correspond to the three
// non-Off depths. Renderers select the appropriate Open slot at runtime
// based on terminal capability.
package theme

import (
	"errors"
)

// Reset is the ANSI sequence that returns text to default color/style.
// Callers append this immediately after styled text to terminate the
// styling region.
const Reset = "\x1b[0m"

// Role is one of 5 semantic styling roles. It identifies WHAT the styled
// text means (informational, warning, error, highlighted, dimmed) rather
// than WHICH color renders it; the actual color comes from the active
// Theme.
type Role string

// Canonical role values. These five constants are the complete set of
// Roles recognised by the theme system.
const (
	RoleInfo      Role = "info"
	RoleWarn      Role = "warn"
	RoleError     Role = "error"
	RoleHighlight Role = "highlight"
	RoleDim       Role = "dim"
)

// IsValid reports whether r is one of the five canonical roles.
func (r Role) IsValid() bool {
	switch r {
	case RoleInfo, RoleWarn, RoleError, RoleHighlight, RoleDim:
		return true
	default:
		return false
	}
}

// AllRoles returns the five roles in canonical order
// (info, warn, error, highlight, dim). The returned slice is fresh on
// every call; callers may mutate it freely.
func AllRoles() []Role {
	return []Role{RoleInfo, RoleWarn, RoleError, RoleHighlight, RoleDim}
}

// ColorDepth determines which color encoding the renderer can support
// for a given terminal session. Higher depths supersede lower depths
// in expressiveness but require corresponding terminal capability.
type ColorDepth int

// Supported color depth tiers.
const (
	DepthOff       ColorDepth = 0 // no color emission (plain mode / dumb terminal)
	DepthANSI16    ColorDepth = 1 // 16-color ANSI (\x1b[30-37m, \x1b[90-97m)
	DepthANSI256   ColorDepth = 2 // 256-color ANSI (\x1b[38;5;Nm)
	DepthTruecolor ColorDepth = 3 // 24-bit truecolor (\x1b[38;2;R;G;Bm)
)

// String returns the lowercase canonical name of the depth:
// "off", "ansi16", "ansi256", or "truecolor". Unknown depths return
// "unknown".
func (d ColorDepth) String() string {
	switch d {
	case DepthOff:
		return "off"
	case DepthANSI16:
		return "ansi16"
	case DepthANSI256:
		return "ansi256"
	case DepthTruecolor:
		return "truecolor"
	default:
		return "unknown"
	}
}

// IsValid reports whether d is one of the four supported depth tiers.
func (d ColorDepth) IsValid() bool {
	switch d {
	case DepthOff, DepthANSI16, DepthANSI256, DepthTruecolor:
		return true
	default:
		return false
	}
}

// Color holds the OPENING ANSI escape sequence for one color slot, one
// per non-Off depth tier. The matching Reset is implicit; callers append
// the package-level Reset constant after the styled text.
//
// The three depth tiers per Color allow the same Theme to render
// correctly on terminals of any capability. An empty Open at a
// particular depth keeps that slot UNSTYLED at that depth (renderer
// emits no opening escape and no Reset for that slot); Open does NOT
// fall back to a different depth's code.
type Color struct {
	OpenANSI16    string `yaml:"ansi16,omitempty"`    // e.g., "\x1b[31m"
	OpenANSI256   string `yaml:"ansi256,omitempty"`   // e.g., "\x1b[38;5;196m"
	OpenTruecolor string `yaml:"truecolor,omitempty"` // e.g., "\x1b[38;2;255;64;64m"
}

// IsZero reports whether all three Open slots are empty.
func (c Color) IsZero() bool {
	return c.OpenANSI16 == "" && c.OpenANSI256 == "" && c.OpenTruecolor == ""
}

// Open returns the appropriate opening sequence for the given depth.
// Returns "" if the slot for that depth is empty (caller emits no
// styling for that role at this depth — there is no fallback to a
// different depth's code). Returns "" unconditionally for DepthOff,
// regardless of whether other slots are populated.
func (c Color) Open(d ColorDepth) string {
	switch d {
	case DepthANSI16:
		return c.OpenANSI16
	case DepthANSI256:
		return c.OpenANSI256
	case DepthTruecolor:
		return c.OpenTruecolor
	default:
		// DepthOff and any unknown depth: no styling.
		return ""
	}
}

// ThemeName identifies a built-in or user-loaded theme.
type ThemeName string

// Built-in theme names.
const (
	ThemeDark  ThemeName = "dark"
	ThemeLight ThemeName = "light"
	ThemeNone  ThemeName = "none"
)

// IsValid reports whether n is a known built-in theme name. User-loaded
// custom themes can have arbitrary names; this check is for built-ins
// only.
func (n ThemeName) IsValid() bool {
	switch n {
	case ThemeDark, ThemeLight, ThemeNone:
		return true
	default:
		return false
	}
}

// Theme is the palette that maps each Role to a 3-tier Color.
type Theme struct {
	Name   ThemeName      `yaml:"name"`
	Colors map[Role]Color `yaml:"colors"`
}

// IsZero reports whether the theme has no name and no colors.
func (t Theme) IsZero() bool {
	return t.Name == "" && len(t.Colors) == 0
}

// ColorFor returns the Color for the given role, or a zero Color if
// the role is unmapped (or the Colors map is nil).
func (t Theme) ColorFor(r Role) Color {
	if t.Colors == nil {
		return Color{}
	}
	return t.Colors[r]
}

// Sentinel errors. Callers use errors.Is to test against these.
var (
	ErrInvalidRole       = errors.New("invalid role")
	ErrInvalidColorDepth = errors.New("invalid color depth")
	ErrInvalidThemeName  = errors.New("invalid theme name")
	ErrThemeNotFound     = errors.New("theme not found")
	ErrInvalidYAML       = errors.New("invalid theme YAML")
)
