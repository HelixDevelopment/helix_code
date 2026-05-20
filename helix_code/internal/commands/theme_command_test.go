// Package commands — theme_command_test.go (P1-F20-T07).
//
// Unit tests for /theme slash command (status/list/show). Uses a fake
// ThemeInspector to keep the suite hermetic — we never touch disk or
// env vars here. The fake returns real *theme.Theme values built from the
// production builtin.go constructors so the styled-output assertions
// exercise the real Stylize() byte path (no fake Styler).
package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/theme"
)

// fakeThemeInspector is a hexagonal-seam impl of the ThemeInspector
// interface used by /theme tests. It records Get/Names/Custom calls so
// the tests can assert delegation, and lets each test override the
// returned values per-case.
type fakeThemeInspector struct {
	getFn    func(theme.ThemeName) (theme.Theme, error)
	namesFn  func() []theme.ThemeName
	customFn func() *theme.Theme
}

func (f *fakeThemeInspector) Get(n theme.ThemeName) (theme.Theme, error) {
	return f.getFn(n)
}
func (f *fakeThemeInspector) Names() []theme.ThemeName {
	return f.namesFn()
}
func (f *fakeThemeInspector) Custom() *theme.Theme {
	if f.customFn == nil {
		return nil
	}
	return f.customFn()
}

// newDarkOnlyInspector returns a fake registry that knows about the
// three built-in themes only (no custom). Mirrors the steady-state
// production behaviour when no theme.yaml is loaded.
func newDarkOnlyInspector() *fakeThemeInspector {
	return &fakeThemeInspector{
		getFn: func(n theme.ThemeName) (theme.Theme, error) {
			switch n {
			case theme.ThemeDark:
				return theme.BuiltinDarkTheme(), nil
			case theme.ThemeLight:
				return theme.BuiltinLightTheme(), nil
			case theme.ThemeNone:
				return theme.BuiltinNoneTheme(), nil
			}
			return theme.Theme{}, theme.ErrThemeNotFound
		},
		namesFn: func() []theme.ThemeName {
			return []theme.ThemeName{theme.ThemeDark, theme.ThemeLight, theme.ThemeNone}
		},
	}
}

func TestThemeCommand_Name(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceEnv, nil)
	assert.Equal(t, "theme", c.Name())
	// Description()/Usage() route through the CONST-046 tr() seam; the
	// default NoopTranslator echoes the message ID verbatim.
	assert.Equal(t, "internal_commands_theme_description", c.Description())
	assert.Equal(t, "internal_commands_theme_usage", c.Usage())
	assert.Nil(t, c.Aliases())
}

func TestThemeCommand_DefaultIsStatus(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceEnv, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "internal_commands_theme_status_header")
	assert.Contains(t, res.Output, "internal_commands_theme_label_name")
	assert.Contains(t, res.Output, "dark")
}

func TestThemeCommand_StatusShowsNameAndDepth(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeLight, theme.DepthANSI256, ThemeSourceCOLORFGBG, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "light")
	assert.Contains(t, res.Output, "ansi256")
}

func TestThemeCommand_StatusShowsSource(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"env", ThemeSourceEnv},
		{"colorfgbg", ThemeSourceCOLORFGBG},
		{"default", ThemeSourceDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, tc.source, nil)
			res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
			require.NoError(t, err)
			assert.Contains(t, res.Output, tc.source)
		})
	}
}

func TestThemeCommand_StatusShowsCustomNone(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "internal_commands_theme_label_custom")
	assert.Contains(t, res.Output, "internal_commands_theme_custom_none")
}

func TestThemeCommand_ListShowsAllNames(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "internal_commands_theme_list_header")
	assert.Contains(t, res.Output, "dark")
	assert.Contains(t, res.Output, "light")
	assert.Contains(t, res.Output, "none")
	assert.Contains(t, res.Output, "internal_commands_theme_tag_builtin")
}

func TestThemeCommand_ListShowsCustom_WhenLoaded(t *testing.T) {
	customTheme := theme.Theme{
		Name:   theme.ThemeName("my-custom"),
		Colors: theme.BuiltinDarkTheme().Colors,
	}
	insp := newDarkOnlyInspector()
	insp.customFn = func() *theme.Theme { return &customTheme }
	insp.namesFn = func() []theme.ThemeName {
		return []theme.ThemeName{theme.ThemeDark, theme.ThemeLight, theme.ThemeNone, customTheme.Name}
	}
	insp.getFn = func(n theme.ThemeName) (theme.Theme, error) {
		if n == customTheme.Name {
			return customTheme, nil
		}
		return newDarkOnlyInspector().getFn(n)
	}

	c := NewThemeCommand(insp, theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "my-custom")
	assert.Contains(t, res.Output, "internal_commands_theme_tag_user_loaded")
}

func TestThemeCommand_Show_RendersAllRoles(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "dark"}})
	require.NoError(t, err)
	for _, role := range []string{"info", "warn", "error", "highlight", "dim"} {
		assert.Contains(t, res.Output, role)
	}
	// The sample line routes through the CONST-046 tr() seam; the
	// default NoopTranslator echoes the message ID verbatim.
	assert.Contains(t, res.Output, "internal_commands_theme_sample_text")
	// Truecolor dark theme: every role's Stylize MUST emit a real ANSI
	// open + Reset around the sample text. Anti-bluff: assert at least
	// one role produced the truecolor open prefix \x1b[38;2; — proving
	// the styler actually ran (no fake-output path).
	assert.Contains(t, res.Output, "\x1b[38;2;", "truecolor open sequence missing — Stylize() did not run")
	assert.Contains(t, res.Output, "\x1b[0m", "ANSI reset missing — Stylize() did not close the styled region")
}

func TestThemeCommand_Show_DepthOff_NoANSI(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthOff, ThemeSourceDefault, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "dark"}})
	require.NoError(t, err)
	// All five roles still listed, but no ANSI bytes (DepthOff -> no-op).
	assert.NotContains(t, res.Output, "\x1b[", "depth=off must produce zero ANSI bytes")
}

func TestThemeCommand_Show_UnknownThemeErrors(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "banana"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "theme not found")
}

func TestThemeCommand_Show_MissingNameArgErrors(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show"}})
	require.Error(t, err)
	// Error message routes through the CONST-046 i18n seam; NoopTranslator
	// echoes the message ID verbatim.
	assert.Contains(t, err.Error(), "internal_commands_theme_err_show_missing_name")
}

func TestThemeCommand_UnknownSubcommandErrors(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"flush"}})
	require.Error(t, err)
	// Error message routes through the CONST-046 i18n seam; NoopTranslator
	// echoes the message ID verbatim.
	assert.Contains(t, err.Error(), "internal_commands_theme_err_unknown_subcommand")
}

func TestResolveThemeSource_EnvOverride(t *testing.T) {
	env := func(k string) string {
		if k == theme.ThemeNameEnvVar {
			return "light"
		}
		return ""
	}
	assert.Equal(t, ThemeSourceEnv, ResolveThemeSource(env))
}

func TestResolveThemeSource_GarbageEnvFallsThroughToDefault(t *testing.T) {
	env := func(k string) string {
		if k == theme.ThemeNameEnvVar {
			return "banana"
		}
		return ""
	}
	// Garbage HELIXCODE_THEME doesn't drive the choice; with no COLORFGBG
	// either, the source is the default.
	assert.Equal(t, ThemeSourceDefault, ResolveThemeSource(env))
}

func TestResolveThemeSource_COLORFGBG(t *testing.T) {
	env := func(k string) string {
		if k == "COLORFGBG" {
			return "0;15"
		}
		return ""
	}
	assert.Equal(t, ThemeSourceCOLORFGBG, ResolveThemeSource(env))
}

func TestResolveThemeSource_AllUnset(t *testing.T) {
	env := func(string) string { return "" }
	assert.Equal(t, ThemeSourceDefault, ResolveThemeSource(env))
}

// Smoke check that the test file's role list matches theme.AllRoles().
// Guard against drift if a future change adds a sixth role.
func TestThemeCommand_Show_RoleListMatchesAllRoles(t *testing.T) {
	c := NewThemeCommand(newDarkOnlyInspector(), theme.ThemeDark, theme.DepthTruecolor, ThemeSourceDefault, nil)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "dark"}})
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(res.Output), "\n")
	// 1 header line + 5 role lines = 6.
	require.Len(t, lines, 1+len(theme.AllRoles()))
	for i, role := range theme.AllRoles() {
		assert.Contains(t, lines[i+1], string(role))
	}
}
