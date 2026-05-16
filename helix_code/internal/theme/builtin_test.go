package theme

import (
	"testing"
)

// Spec §3.4 byte tables — pinned canonical values. If these tests need to
// change, the spec must change first (the tests follow the spec, not
// vice versa).

func TestBuiltinDarkTheme_Name(t *testing.T) {
	got := BuiltinDarkTheme()
	if got.Name != ThemeDark {
		t.Fatalf("Name = %q; want %q", got.Name, ThemeDark)
	}
}

func TestBuiltinDarkTheme_Info_ByteTable(t *testing.T) {
	c := BuiltinDarkTheme().ColorFor(RoleInfo)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[37m"},
		{DepthANSI256, "\x1b[38;5;250m"},
		{DepthTruecolor, "\x1b[38;2;220;220;220m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("dark Info @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinDarkTheme_Warn_ByteTable(t *testing.T) {
	c := BuiltinDarkTheme().ColorFor(RoleWarn)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[33m"},
		{DepthANSI256, "\x1b[38;5;214m"},
		{DepthTruecolor, "\x1b[38;2;255;176;0m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("dark Warn @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinDarkTheme_Error_ByteTable(t *testing.T) {
	c := BuiltinDarkTheme().ColorFor(RoleError)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[31m"},
		{DepthANSI256, "\x1b[38;5;196m"},
		{DepthTruecolor, "\x1b[38;2;255;64;64m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("dark Error @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinDarkTheme_Highlight_ByteTable(t *testing.T) {
	c := BuiltinDarkTheme().ColorFor(RoleHighlight)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[36m"},
		{DepthANSI256, "\x1b[38;5;51m"},
		{DepthTruecolor, "\x1b[38;2;0;200;220m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("dark Highlight @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinDarkTheme_Dim_ByteTable(t *testing.T) {
	c := BuiltinDarkTheme().ColorFor(RoleDim)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[90m"},
		{DepthANSI256, "\x1b[38;5;243m"},
		{DepthTruecolor, "\x1b[38;2;128;128;128m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("dark Dim @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinLightTheme_Name(t *testing.T) {
	got := BuiltinLightTheme()
	if got.Name != ThemeLight {
		t.Fatalf("Name = %q; want %q", got.Name, ThemeLight)
	}
}

func TestBuiltinLightTheme_Info_ByteTable(t *testing.T) {
	c := BuiltinLightTheme().ColorFor(RoleInfo)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[30m"},
		{DepthANSI256, "\x1b[38;5;235m"},
		{DepthTruecolor, "\x1b[38;2;40;40;40m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("light Info @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinLightTheme_Warn_ByteTable(t *testing.T) {
	c := BuiltinLightTheme().ColorFor(RoleWarn)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[33m"},
		{DepthANSI256, "\x1b[38;5;130m"},
		{DepthTruecolor, "\x1b[38;2;175;95;0m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("light Warn @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinLightTheme_Error_ByteTable(t *testing.T) {
	c := BuiltinLightTheme().ColorFor(RoleError)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[31m"},
		{DepthANSI256, "\x1b[38;5;124m"},
		{DepthTruecolor, "\x1b[38;2;175;0;0m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("light Error @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinLightTheme_Highlight_ByteTable(t *testing.T) {
	c := BuiltinLightTheme().ColorFor(RoleHighlight)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[34m"},
		{DepthANSI256, "\x1b[38;5;25m"},
		{DepthTruecolor, "\x1b[38;2;0;95;175m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("light Highlight @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinLightTheme_Dim_ByteTable(t *testing.T) {
	c := BuiltinLightTheme().ColorFor(RoleDim)
	cases := []struct {
		depth ColorDepth
		want  string
	}{
		{DepthANSI16, "\x1b[37m"},
		{DepthANSI256, "\x1b[38;5;245m"},
		{DepthTruecolor, "\x1b[38;2;138;138;138m"},
	}
	for _, tc := range cases {
		if got := c.Open(tc.depth); got != tc.want {
			t.Errorf("light Dim @ %s: Open = %q; want %q", tc.depth, got, tc.want)
		}
	}
}

func TestBuiltinNoneTheme_Name(t *testing.T) {
	got := BuiltinNoneTheme()
	if got.Name != ThemeNone {
		t.Fatalf("Name = %q; want %q", got.Name, ThemeNone)
	}
}

func TestBuiltinNoneTheme_AllRolesReturnZeroColor(t *testing.T) {
	th := BuiltinNoneTheme()
	for _, r := range AllRoles() {
		c := th.ColorFor(r)
		if !c.IsZero() {
			t.Errorf("none theme: ColorFor(%q) = %+v; want zero Color", r, c)
		}
	}
}

func TestBuiltinNoneTheme_Open_AllDepthsAllRolesEmpty(t *testing.T) {
	th := BuiltinNoneTheme()
	depths := []ColorDepth{DepthANSI16, DepthANSI256, DepthTruecolor}
	for _, r := range AllRoles() {
		c := th.ColorFor(r)
		for _, d := range depths {
			if got := c.Open(d); got != "" {
				t.Errorf("none theme: ColorFor(%q).Open(%s) = %q; want empty", r, d, got)
			}
		}
	}
}

func TestAllBuiltinThemes_ContainsThree(t *testing.T) {
	got := AllBuiltinThemes()
	if len(got) != 3 {
		t.Fatalf("len(AllBuiltinThemes()) = %d; want 3", len(got))
	}
	wantNames := []ThemeName{ThemeDark, ThemeLight, ThemeNone}
	for i, want := range wantNames {
		if got[i].Name != want {
			t.Errorf("AllBuiltinThemes()[%d].Name = %q; want %q", i, got[i].Name, want)
		}
	}
}

func TestBuiltinThemes_FreshPerCall(t *testing.T) {
	a := BuiltinDarkTheme()
	// Mutate a's Colors map by replacing the entry for RoleInfo.
	if a.Colors == nil {
		t.Fatal("BuiltinDarkTheme().Colors is nil; expected populated map")
	}
	a.Colors[RoleInfo] = Color{OpenANSI16: "\x1b[99m"}

	b := BuiltinDarkTheme()
	bInfo := b.ColorFor(RoleInfo)
	if bInfo.OpenANSI16 != "\x1b[37m" {
		t.Errorf("after mutating first call, second call's Info.OpenANSI16 = %q; want %q (fresh allocation expected)",
			bInfo.OpenANSI16, "\x1b[37m")
	}
}

func TestBuiltinThemes_AllRolesPopulated(t *testing.T) {
	for _, th := range []Theme{BuiltinDarkTheme(), BuiltinLightTheme()} {
		for _, r := range AllRoles() {
			if _, ok := th.Colors[r]; !ok {
				t.Errorf("theme %q: missing role %q in Colors map", th.Name, r)
			}
		}
	}
}
