package theme

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// ----------------------------------------------------------------------------
// DefaultThemePath
// ----------------------------------------------------------------------------

func TestDefaultThemePath_XDGSet(t *testing.T) {
	got := DefaultThemePath(func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return "/foo"
		}
		return ""
	})
	want := filepath.Join("/foo", "helixcode", "theme.yaml")
	if got != want {
		t.Fatalf("DefaultThemePath: got %q, want %q", got, want)
	}
}

func TestDefaultThemePath_XDGUnset_HOMESet(t *testing.T) {
	got := DefaultThemePath(func(k string) string {
		if k == "HOME" {
			return "/home/u"
		}
		return ""
	})
	want := filepath.Join("/home/u", ".config", "helixcode", "theme.yaml")
	if got != want {
		t.Fatalf("DefaultThemePath: got %q, want %q", got, want)
	}
}

// Both XDG_CONFIG_HOME and HOME unset: implementation choice — return ""
// (caller treats empty as "no default path available, skip load").
// This is documented in DefaultThemePath's docstring.
func TestDefaultThemePath_NeitherSet_FallsBackTo_DotConfig(t *testing.T) {
	got := DefaultThemePath(func(k string) string { return "" })
	if got != "" {
		t.Fatalf("DefaultThemePath: got %q, want empty string when neither XDG nor HOME is set", got)
	}
}

// ----------------------------------------------------------------------------
// ThemeRegistry — built-ins
// ----------------------------------------------------------------------------

func TestThemeRegistry_GetBuiltinDark(t *testing.T) {
	r := NewThemeRegistry()
	got, err := r.Get(ThemeDark)
	if err != nil {
		t.Fatalf("Get(dark): unexpected error: %v", err)
	}
	if got.Name != ThemeDark {
		t.Fatalf("Get(dark): got name %q, want %q", got.Name, ThemeDark)
	}
	// Validate at least one role's bytes match the canonical dark palette.
	want := BuiltinDarkTheme().Colors[RoleError].OpenANSI16
	if got.Colors[RoleError].OpenANSI16 != want {
		t.Fatalf("Get(dark): error.ansi16 = %q, want %q", got.Colors[RoleError].OpenANSI16, want)
	}
}

func TestThemeRegistry_GetBuiltinLight(t *testing.T) {
	r := NewThemeRegistry()
	got, err := r.Get(ThemeLight)
	if err != nil {
		t.Fatalf("Get(light): unexpected error: %v", err)
	}
	if got.Name != ThemeLight {
		t.Fatalf("Get(light): got name %q, want %q", got.Name, ThemeLight)
	}
}

func TestThemeRegistry_GetBuiltinNone(t *testing.T) {
	r := NewThemeRegistry()
	got, err := r.Get(ThemeNone)
	if err != nil {
		t.Fatalf("Get(none): unexpected error: %v", err)
	}
	if got.Name != ThemeNone {
		t.Fatalf("Get(none): got name %q, want %q", got.Name, ThemeNone)
	}
}

func TestThemeRegistry_GetUnknown_Errors(t *testing.T) {
	r := NewThemeRegistry()
	_, err := r.Get(ThemeName("banana"))
	if err == nil {
		t.Fatalf("Get(banana): expected error, got nil")
	}
	if !errors.Is(err, ErrThemeNotFound) {
		t.Fatalf("Get(banana): expected ErrThemeNotFound, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// ThemeRegistry — LoadFromFile
// ----------------------------------------------------------------------------

func TestThemeRegistry_LoadFromFile_MissingFileNoError(t *testing.T) {
	r := NewThemeRegistry()
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	if err := r.LoadFromFile(missing); err != nil {
		t.Fatalf("LoadFromFile(missing): expected nil error, got %v", err)
	}
	if c := r.Custom(); c != nil {
		t.Fatalf("LoadFromFile(missing): expected Custom() nil, got %+v", c)
	}
}

func TestThemeRegistry_LoadFromFile_HappyPath(t *testing.T) {
	r := NewThemeRegistry()
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	body := `name: my-custom
colors:
  info:
    ansi16: "\x1b[37m"
    ansi256: "\x1b[38;5;250m"
    truecolor: "\x1b[38;2;220;220;220m"
  warn:
    ansi16: "\x1b[33m"
    ansi256: "\x1b[38;5;214m"
    truecolor: "\x1b[38;2;255;176;0m"
  error:
    ansi16: "\x1b[35m"
    ansi256: "\x1b[38;5;201m"
    truecolor: "\x1b[38;2;200;0;200m"
  highlight:
    ansi16: "\x1b[36m"
    ansi256: "\x1b[38;5;51m"
    truecolor: "\x1b[38;2;0;200;220m"
  dim:
    ansi16: "\x1b[90m"
    ansi256: "\x1b[38;5;243m"
    truecolor: "\x1b[38;2;128;128;128m"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := r.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: unexpected error: %v", err)
	}
	cust := r.Custom()
	if cust == nil {
		t.Fatalf("LoadFromFile: expected Custom() non-nil")
	}
	if cust.Name != ThemeName("my-custom") {
		t.Fatalf("LoadFromFile: got name %q, want %q", cust.Name, "my-custom")
	}
	if got := cust.Colors[RoleError].OpenANSI16; got != "\x1b[35m" {
		t.Fatalf("LoadFromFile: error.ansi16 = %q, want %q", got, "\x1b[35m")
	}
}

func TestThemeRegistry_LoadFromFile_PartialOverride_MergesWithDark(t *testing.T) {
	r := NewThemeRegistry()
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	// Only `error` specified → other roles must inherit from dark baseline.
	body := `name: partial
colors:
  error:
    ansi16: "\x1b[35m"
    ansi256: "\x1b[38;5;201m"
    truecolor: "\x1b[38;2;200;0;200m"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := r.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: unexpected error: %v", err)
	}
	cust := r.Custom()
	if cust == nil {
		t.Fatalf("LoadFromFile: expected Custom() non-nil")
	}
	dark := BuiltinDarkTheme()

	// Overridden role: error → from YAML.
	if got := cust.Colors[RoleError].OpenANSI16; got != "\x1b[35m" {
		t.Fatalf("merge: error.ansi16 = %q, want %q (yaml override)", got, "\x1b[35m")
	}

	// Inherited roles: must equal dark baseline.
	for _, role := range []Role{RoleInfo, RoleWarn, RoleHighlight, RoleDim} {
		want := dark.Colors[role]
		got := cust.Colors[role]
		if got != want {
			t.Fatalf("merge: role %q expected dark baseline %+v, got %+v", role, want, got)
		}
	}

	// Sanity: full 5-role map.
	if len(cust.Colors) != 5 {
		t.Fatalf("merge: expected 5 roles in Colors map, got %d (%+v)", len(cust.Colors), cust.Colors)
	}
}

func TestThemeRegistry_LoadFromFile_BadYAML_Errors(t *testing.T) {
	r := NewThemeRegistry()
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	if err := os.WriteFile(path, []byte("not: valid: yaml: ::: garbage\n  - mismatched\n"), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	err := r.LoadFromFile(path)
	if err == nil {
		t.Fatalf("LoadFromFile(garbage): expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidYAML) {
		t.Fatalf("LoadFromFile(garbage): expected ErrInvalidYAML, got %v", err)
	}
}

func TestThemeRegistry_GetCustomByName_AfterLoad(t *testing.T) {
	r := NewThemeRegistry()
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	body := `name: my-custom
colors:
  error:
    ansi16: "\x1b[35m"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := r.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	got, err := r.Get(ThemeName("my-custom"))
	if err != nil {
		t.Fatalf("Get(my-custom): %v", err)
	}
	if got.Name != ThemeName("my-custom") {
		t.Fatalf("Get(my-custom): got name %q, want my-custom", got.Name)
	}
}

func TestThemeRegistry_Names_IncludesCustom(t *testing.T) {
	r := NewThemeRegistry()
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	body := `name: my-custom
colors:
  error:
    ansi16: "\x1b[35m"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := r.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	names := r.Names()
	have := map[ThemeName]bool{}
	for _, n := range names {
		have[n] = true
	}
	for _, want := range []ThemeName{ThemeDark, ThemeLight, ThemeNone, ThemeName("my-custom")} {
		if !have[want] {
			t.Fatalf("Names() missing %q; got %v", want, names)
		}
	}
}

// ----------------------------------------------------------------------------
// Styler
// ----------------------------------------------------------------------------

func TestStyler_DepthOff_NoOp(t *testing.T) {
	s := NewStyler(BuiltinDarkTheme(), DepthOff)
	for _, role := range AllRoles() {
		if got := s.Stylize(role, "hi"); got != "hi" {
			t.Fatalf("Stylize(%q): got %q, want %q (DepthOff must be no-op)", role, got, "hi")
		}
	}
}

func TestStyler_Stylize_DepthANSI16_WrapsWithReset(t *testing.T) {
	s := NewStyler(BuiltinDarkTheme(), DepthANSI16)
	got := s.Stylize(RoleError, "hi")
	want := "\x1b[31mhi\x1b[0m"
	if got != want {
		t.Fatalf("Stylize(error, hi): got %q, want %q", got, want)
	}
}

func TestStyler_Stylize_DepthANSI256_WrapsWithReset(t *testing.T) {
	s := NewStyler(BuiltinDarkTheme(), DepthANSI256)
	got := s.Stylize(RoleError, "hi")
	want := "\x1b[38;5;196mhi\x1b[0m"
	if got != want {
		t.Fatalf("Stylize(error, hi): got %q, want %q", got, want)
	}
}

func TestStyler_Stylize_DepthTruecolor_WrapsWithReset(t *testing.T) {
	s := NewStyler(BuiltinDarkTheme(), DepthTruecolor)
	got := s.Stylize(RoleError, "hi")
	want := "\x1b[38;2;255;64;64mhi\x1b[0m"
	if got != want {
		t.Fatalf("Stylize(error, hi): got %q, want %q", got, want)
	}
}

func TestStyler_Stylize_UnknownRole_NoOp(t *testing.T) {
	s := NewStyler(BuiltinDarkTheme(), DepthTruecolor)
	if got := s.Stylize(Role("banana"), "hi"); got != "hi" {
		t.Fatalf("Stylize(banana): got %q, want %q (unknown role must be no-op)", got, "hi")
	}
}

func TestStyler_Stylize_NoneTheme_NoOp(t *testing.T) {
	none := BuiltinNoneTheme()
	for _, depth := range []ColorDepth{DepthOff, DepthANSI16, DepthANSI256, DepthTruecolor} {
		s := NewStyler(none, depth)
		for _, role := range AllRoles() {
			if got := s.Stylize(role, "hi"); got != "hi" {
				t.Fatalf("Stylize(%q) [depth=%v]: got %q, want %q (none theme must be no-op)", role, depth, got, "hi")
			}
		}
	}
}

func TestStyler_ConcurrentSafe(t *testing.T) {
	s := NewStyler(BuiltinDarkTheme(), DepthTruecolor)
	const goroutines = 64
	const itersPer = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < itersPer; j++ {
				_ = s.Stylize(RoleInfo, "x")
				_ = s.Stylize(RoleError, "y")
				_ = s.Theme()
				_ = s.Depth()
			}
		}()
	}
	wg.Wait()
}

func TestStyler_ThemeAndDepthAccessors(t *testing.T) {
	th := BuiltinDarkTheme()
	s := NewStyler(th, DepthANSI256)
	if got := s.Theme().Name; got != ThemeDark {
		t.Fatalf("Theme().Name = %q, want %q", got, ThemeDark)
	}
	if got := s.Depth(); got != DepthANSI256 {
		t.Fatalf("Depth() = %v, want %v", got, DepthANSI256)
	}
}
