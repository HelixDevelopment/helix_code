package theme

// loader.go (P1-F20-T05): user-loaded YAML theme overrides + Styler.
//
// This file owns three public types:
//
//   ThemeRegistry — pre-populated with the three built-in themes
//   (dark/light/none from builtin.go). LoadFromFile may attach an
//   optional custom theme that merges colors over the dark baseline.
//   Get/Names/Custom expose the registry's contents to callers.
//
//   Styler — decorates text with role-coded ANSI sequences using a
//   particular Theme and ColorDepth. DepthOff produces a no-op styler;
//   none-theme produces a no-op styler regardless of depth.
//
//   DefaultThemePath — pure helper resolving the canonical config
//   path (XDG → HOME → empty fallback). Tests inject their own
//   envLookup; production callers pass os.Getenv.
//
// Anti-bluff anchor: LoadFromFile reads a real file on disk, parses real
// YAML through gopkg.in/yaml.v3, and merges over the real built-in dark
// theme returned by BuiltinDarkTheme(). There is no in-memory short-
// circuit and no fake parse path. Tests use t.TempDir() and
// os.WriteFile to feed real YAML bytes through the loader.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// ThemeFileEnvVar is the optional override for the theme YAML location.
// Callers (typically main()) may consult os.Getenv(ThemeFileEnvVar) to
// take a user-specified path before falling back to DefaultThemePath.
const ThemeFileEnvVar = "HELIXCODE_THEME_FILE"

// DefaultThemePath returns the canonical theme YAML config path:
//
//	$XDG_CONFIG_HOME/helixcode/theme.yaml
//
// falling back to:
//
//	$HOME/.config/helixcode/theme.yaml
//
// If neither env var is set, returns "" so callers know there is no
// default location to attempt (they then skip auto-load rather than
// reading a relative path that depends on cwd, which would be a footgun
// in production).
//
// Pure function; takes envLookup for testability. Production callers
// pass os.Getenv.
func DefaultThemePath(envLookup func(string) string) string {
	if xdg := envLookup("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "helixcode", "theme.yaml")
	}
	if home := envLookup("HOME"); home != "" {
		return filepath.Join(home, ".config", "helixcode", "theme.yaml")
	}
	return ""
}

// themeYAML is the on-disk schema for a user-loaded theme. Mirrors
// Theme but with the same yaml tags so unmarshalling is one step.
type themeYAML struct {
	Name   string           `yaml:"name"`
	Colors map[string]Color `yaml:"colors"`
}

// ThemeRegistry holds built-in themes plus an optional user-loaded YAML
// override. Thread-safe: all accessors take r.mu.
type ThemeRegistry struct {
	mu     sync.RWMutex
	themes map[ThemeName]Theme // built-ins (dark/light/none); never modified after construction
	custom *Theme              // optional user-loaded theme (any name); merged over dark baseline
}

// NewThemeRegistry constructs a registry pre-populated with the three
// built-in themes (dark, light, none). The Custom slot starts nil; call
// LoadFromFile to attach a user-defined theme.
func NewThemeRegistry() *ThemeRegistry {
	r := &ThemeRegistry{
		themes: make(map[ThemeName]Theme, 3),
	}
	for _, t := range AllBuiltinThemes() {
		r.themes[t.Name] = t
	}
	return r
}

// LoadFromFile reads YAML from path, merges colors over the dark
// baseline, and stores the result as the registry's custom theme.
//
// Behavior:
//   - Missing file (fs.ErrNotExist) → returns nil; registry retains
//     built-ins only and Custom() stays nil. This is the graceful path
//     for first-run users with no theme.yaml on disk.
//   - Parse error → returns an error wrapping ErrInvalidYAML.
//   - Other open/read errors → returned unwrapped (caller can decide).
//
// Merge semantics: starting from BuiltinDarkTheme().Colors as the
// baseline, every role present in the YAML's colors map overrides the
// baseline entry; roles absent from the YAML are inherited unchanged.
// The result is ALWAYS a full 5-role map (per spec §11). The custom
// theme's Name is taken verbatim from the YAML.
func (r *ThemeRegistry) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("theme: open %s: %w", path, err)
	}
	defer f.Close()

	var raw themeYAML
	dec := yaml.NewDecoder(f)
	dec.KnownFields(false) // tolerate forward-compatible extra keys
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidYAML, path, err)
	}

	merged := mergeOverDark(raw.Colors)
	custom := Theme{
		Name:   ThemeName(raw.Name),
		Colors: merged,
	}

	r.mu.Lock()
	r.custom = &custom
	r.mu.Unlock()
	return nil
}

// mergeOverDark builds a fresh 5-role Colors map starting from the dark
// baseline and overlaying any roles present in overrides. overrides
// keys are validated against the canonical role set; unknown keys are
// silently dropped (forward-compatible).
func mergeOverDark(overrides map[string]Color) map[Role]Color {
	dark := BuiltinDarkTheme().Colors
	out := make(map[Role]Color, len(dark))
	for r, c := range dark {
		out[r] = c
	}
	for k, v := range overrides {
		role := Role(k)
		if !role.IsValid() {
			continue
		}
		out[role] = v
	}
	return out
}

// Get returns the theme for the given name. Lookup order:
//  1. If a custom theme is loaded AND name matches its Name → custom.
//  2. Built-in (dark/light/none).
//  3. ErrThemeNotFound.
func (r *ThemeRegistry) Get(name ThemeName) (Theme, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.custom != nil && r.custom.Name == name {
		return *r.custom, nil
	}
	if t, ok := r.themes[name]; ok {
		return t, nil
	}
	return Theme{}, fmt.Errorf("%w: %q", ErrThemeNotFound, name)
}

// Names returns all available theme names: the three built-ins
// (dark, light, none) in canonical order, followed by the custom
// theme's Name if a custom theme is loaded.
func (r *ThemeRegistry) Names() []ThemeName {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []ThemeName{ThemeDark, ThemeLight, ThemeNone}
	if r.custom != nil {
		out = append(out, r.custom.Name)
	}
	return out
}

// Custom returns a pointer to the loaded custom theme, or nil if none
// has been loaded. The returned pointer aliases the registry's storage;
// callers MUST NOT mutate the pointee.
func (r *ThemeRegistry) Custom() *Theme {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.custom
}

// Styler decorates text with role-coded ANSI sequences. A Styler is
// immutable after construction (the embedded Theme's Colors map is
// read-only by convention; concurrent Stylize calls are safe because
// they only read).
type Styler struct {
	theme Theme
	depth ColorDepth
}

// NewStyler constructs a Styler bound to a particular Theme + depth.
// depth=DepthOff produces a no-op styler — Stylize returns text
// unchanged regardless of role. The none theme similarly degrades to a
// no-op because every role's Color slots are empty.
func NewStyler(theme Theme, depth ColorDepth) *Styler {
	return &Styler{theme: theme, depth: depth}
}

// Stylize wraps text with the role's ANSI escape and trailing Reset.
// Returns text unchanged when:
//   - depth == DepthOff (no-color mode), OR
//   - role is not mapped in the theme (e.g., custom role name), OR
//   - the role's Color has an empty Open slot at this depth.
//
// In every other case, returns Open(depth) + text + Reset.
func (s *Styler) Stylize(role Role, text string) string {
	if s.depth == DepthOff {
		return text
	}
	c := s.theme.ColorFor(role)
	open := c.Open(s.depth)
	if open == "" {
		return text
	}
	return open + text + Reset
}

// Theme returns the styler's bound theme.
func (s *Styler) Theme() Theme { return s.theme }

// Depth returns the styler's bound color depth.
func (s *Styler) Depth() ColorDepth { return s.depth }
