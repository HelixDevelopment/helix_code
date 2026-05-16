package theme

import (
	"reflect"
	"strings"
	"testing"
)

func TestReset_IsCSIZero(t *testing.T) {
	if Reset != "\x1b[0m" {
		t.Fatalf("Reset = %q, want %q", Reset, "\x1b[0m")
	}
}

func TestRole_IsValid_AcceptsKnown(t *testing.T) {
	roles := []Role{RoleInfo, RoleWarn, RoleError, RoleHighlight, RoleDim}
	for _, r := range roles {
		if !r.IsValid() {
			t.Errorf("Role(%q).IsValid() = false, want true", r)
		}
	}
}

func TestRole_IsValid_RejectsUnknown(t *testing.T) {
	bad := []Role{Role(""), Role("banana"), Role("INFO"), Role("info ")}
	for _, r := range bad {
		if r.IsValid() {
			t.Errorf("Role(%q).IsValid() = true, want false", r)
		}
	}
}

func TestAllRoles_ReturnsAllFive(t *testing.T) {
	got := AllRoles()
	want := []Role{RoleInfo, RoleWarn, RoleError, RoleHighlight, RoleDim}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllRoles() = %v, want %v", got, want)
	}
	if len(got) != 5 {
		t.Fatalf("AllRoles() length = %d, want 5", len(got))
	}
}

func TestColorDepth_String_Values(t *testing.T) {
	tests := []struct {
		d    ColorDepth
		want string
	}{
		{DepthOff, "off"},
		{DepthANSI16, "ansi16"},
		{DepthANSI256, "ansi256"},
		{DepthTruecolor, "truecolor"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("ColorDepth(%d).String() = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestColorDepth_IsValid_AcceptsKnown(t *testing.T) {
	depths := []ColorDepth{DepthOff, DepthANSI16, DepthANSI256, DepthTruecolor}
	for _, d := range depths {
		if !d.IsValid() {
			t.Errorf("ColorDepth(%d).IsValid() = false, want true", d)
		}
	}
}

func TestColorDepth_IsValid_RejectsUnknown(t *testing.T) {
	bad := []ColorDepth{99, -1, 4, 100}
	for _, d := range bad {
		if d.IsValid() {
			t.Errorf("ColorDepth(%d).IsValid() = true, want false", d)
		}
	}
}

func TestColor_ZeroIsZero(t *testing.T) {
	c := Color{}
	if !c.IsZero() {
		t.Fatalf("Color{}.IsZero() = false, want true")
	}
}

func TestColor_NonZero(t *testing.T) {
	cases := []Color{
		{OpenANSI16: "\x1b[31m"},
		{OpenANSI256: "\x1b[38;5;196m"},
		{OpenTruecolor: "\x1b[38;2;255;64;64m"},
		{OpenANSI16: "\x1b[31m", OpenANSI256: "\x1b[38;5;196m"},
	}
	for i, c := range cases {
		if c.IsZero() {
			t.Errorf("case %d: Color%+v.IsZero() = true, want false", i, c)
		}
	}
}

func TestColor_Open_DepthOff_AlwaysEmpty(t *testing.T) {
	c := Color{
		OpenANSI16:    "\x1b[31m",
		OpenANSI256:   "\x1b[38;5;196m",
		OpenTruecolor: "\x1b[38;2;255;64;64m",
	}
	if got := c.Open(DepthOff); got != "" {
		t.Fatalf("Color.Open(DepthOff) = %q, want \"\"", got)
	}
}

func TestColor_Open_DepthANSI16_ReturnsANSI16Slot(t *testing.T) {
	c := Color{
		OpenANSI16:    "\x1b[31m",
		OpenANSI256:   "\x1b[38;5;196m",
		OpenTruecolor: "\x1b[38;2;255;64;64m",
	}
	if got := c.Open(DepthANSI16); got != "\x1b[31m" {
		t.Fatalf("Color.Open(DepthANSI16) = %q, want %q", got, "\x1b[31m")
	}
}

func TestColor_Open_DepthANSI256_ReturnsANSI256Slot(t *testing.T) {
	c := Color{
		OpenANSI16:    "\x1b[31m",
		OpenANSI256:   "\x1b[38;5;196m",
		OpenTruecolor: "\x1b[38;2;255;64;64m",
	}
	if got := c.Open(DepthANSI256); got != "\x1b[38;5;196m" {
		t.Fatalf("Color.Open(DepthANSI256) = %q, want %q", got, "\x1b[38;5;196m")
	}
}

func TestColor_Open_DepthTruecolor_ReturnsTruecolorSlot(t *testing.T) {
	c := Color{
		OpenANSI16:    "\x1b[31m",
		OpenANSI256:   "\x1b[38;5;196m",
		OpenTruecolor: "\x1b[38;2;255;64;64m",
	}
	if got := c.Open(DepthTruecolor); got != "\x1b[38;2;255;64;64m" {
		t.Fatalf("Color.Open(DepthTruecolor) = %q, want %q", got, "\x1b[38;2;255;64;64m")
	}
}

func TestColor_Open_EmptySlotReturnsEmpty(t *testing.T) {
	// Color with only ANSI16 set; Open(DepthTruecolor) MUST return ""
	// (no fallback to ANSI16).
	c := Color{OpenANSI16: "\x1b[31m"}
	if got := c.Open(DepthTruecolor); got != "" {
		t.Fatalf("Color.Open(DepthTruecolor) = %q, want \"\" (no fallback)", got)
	}
	if got := c.Open(DepthANSI256); got != "" {
		t.Fatalf("Color.Open(DepthANSI256) = %q, want \"\" (no fallback)", got)
	}
	// And the populated slot still works
	if got := c.Open(DepthANSI16); got != "\x1b[31m" {
		t.Fatalf("Color.Open(DepthANSI16) = %q, want %q", got, "\x1b[31m")
	}
}

func TestThemeName_IsValid_AcceptsKnown(t *testing.T) {
	names := []ThemeName{ThemeDark, ThemeLight, ThemeNone}
	for _, n := range names {
		if !n.IsValid() {
			t.Errorf("ThemeName(%q).IsValid() = false, want true", n)
		}
	}
}

func TestThemeName_IsValid_RejectsUnknown(t *testing.T) {
	bad := []ThemeName{ThemeName(""), ThemeName("solarized"), ThemeName("DARK")}
	for _, n := range bad {
		if n.IsValid() {
			t.Errorf("ThemeName(%q).IsValid() = true, want false", n)
		}
	}
}

func TestTheme_ZeroIsZero(t *testing.T) {
	tt := Theme{}
	if !tt.IsZero() {
		t.Fatalf("Theme{}.IsZero() = false, want true")
	}
}

func TestTheme_NonZero(t *testing.T) {
	cases := []Theme{
		{Name: ThemeDark},
		{Colors: map[Role]Color{RoleInfo: {OpenANSI16: "\x1b[34m"}}},
		{Name: ThemeLight, Colors: map[Role]Color{RoleError: {OpenANSI16: "\x1b[31m"}}},
	}
	for i, c := range cases {
		if c.IsZero() {
			t.Errorf("case %d: Theme%+v.IsZero() = true, want false", i, c)
		}
	}
}

func TestTheme_ColorFor_KnownRole(t *testing.T) {
	want := Color{OpenANSI16: "\x1b[31m", OpenANSI256: "\x1b[38;5;196m"}
	th := Theme{
		Name: ThemeDark,
		Colors: map[Role]Color{
			RoleError: want,
		},
	}
	got := th.ColorFor(RoleError)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Theme.ColorFor(RoleError) = %+v, want %+v", got, want)
	}
}

func TestTheme_ColorFor_UnknownRole_ReturnsZero(t *testing.T) {
	th := Theme{
		Name: ThemeDark,
		Colors: map[Role]Color{
			RoleError: {OpenANSI16: "\x1b[31m"},
		},
	}
	got := th.ColorFor(RoleInfo) // not mapped
	if !got.IsZero() {
		t.Fatalf("Theme.ColorFor(unmapped) = %+v, want Color{}", got)
	}
}

func TestTheme_NilColorsMap_ColorForReturnsZero(t *testing.T) {
	th := Theme{Name: ThemeDark, Colors: nil}
	got := th.ColorFor(RoleError)
	if !got.IsZero() {
		t.Fatalf("Theme{Colors:nil}.ColorFor() = %+v, want Color{}", got)
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	errs := map[string]error{
		"ErrInvalidRole":       ErrInvalidRole,
		"ErrInvalidColorDepth": ErrInvalidColorDepth,
		"ErrInvalidThemeName":  ErrInvalidThemeName,
		"ErrThemeNotFound":     ErrThemeNotFound,
		"ErrInvalidYAML":       ErrInvalidYAML,
	}
	seen := map[string]string{}
	for name, err := range errs {
		if err == nil {
			t.Errorf("%s is nil", name)
			continue
		}
		msg := err.Error()
		if msg == "" {
			t.Errorf("%s has empty message", name)
		}
		if prev, ok := seen[msg]; ok {
			t.Errorf("%s and %s share message %q", prev, name, msg)
		}
		seen[msg] = name
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 distinct sentinel messages, got %d", len(seen))
	}
}

func TestColor_YAMLTagsExist(t *testing.T) {
	rt := reflect.TypeOf(Color{})
	wantTags := map[string]string{
		"OpenANSI16":    "ansi16",
		"OpenANSI256":   "ansi256",
		"OpenTruecolor": "truecolor",
	}
	for fieldName, wantSubstr := range wantTags {
		f, ok := rt.FieldByName(fieldName)
		if !ok {
			t.Fatalf("Color has no field %s", fieldName)
		}
		yamlTag := f.Tag.Get("yaml")
		if !strings.Contains(yamlTag, wantSubstr) {
			t.Errorf("Color.%s yaml tag = %q, want it to contain %q",
				fieldName, yamlTag, wantSubstr)
		}
	}
}
