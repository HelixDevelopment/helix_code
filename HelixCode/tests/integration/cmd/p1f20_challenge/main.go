// p1f20_challenge runs the F20 theme-system harness end-to-end against the
// real registry, real built-in palettes, real DetectColorDepth env logic, and
// a real on-disk YAML override processed through gopkg.in/yaml.v3. Article XI
// 11.9 anti-bluff anchor: a regression that "PASSes" by stubbing the registry
// or hardcoding role bytes will trip one of the byte-equality, zero-ANSI, or
// 5/5-roles invariants captured per phase.
//
// Phases (all five always run; no SKIPs):
//
//	A. BUILT-IN-DARK         - registry.Get(ThemeDark) + Styler{Truecolor}
//	                           must wrap each role with the canonical
//	                           truecolor open code (spec 3.4 dark row)
//	                           and the package-level Reset.
//	B. BUILT-IN-LIGHT        - same shape against ThemeLight; the bytes
//	                           differ from dark per spec 3.4 (e.g. light
//	                           error 175;0;0 vs dark error 255;64;64).
//	C. PLAIN-ZERO-COLOR      - depth=DepthOff -> Stylize must return the
//	                           input string byte-equal with ZERO 0x1B
//	                           bytes anywhere in the output. Load-bearing
//	                           invariant for NO_COLOR / dumb terminals.
//	D. DEPTH-DETECT          - DetectColorDepth via synthesised envLookup
//	                           closures across six branches: NO_COLOR
//	                           override, COLORTERM=truecolor, TERM=*-256
//	                           color, TERM=xterm (ANSI16), TERM=dumb,
//	                           and all-unset. Each must return the
//	                           expected ColorDepth.
//	E. YAML-MERGE            - tempdir theme.yaml overrides only ROLE=
//	                           error; LoadFromFile then Custom() must
//	                           expose all 5 roles where error is the
//	                           override and info/warn/highlight/dim are
//	                           byte-equal to the dark baseline.
//
// Exit code 0 on PASS; exit 1 with a diagnostic on any check failure.
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/theme"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F20 challenge harness pid:", os.Getpid())

	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F20 challenge harness PASS")
	return nil
}

// expectedDarkTruecolor maps Role -> the spec 3.4 truecolor opening sequence
// for the dark theme. These bytes are the contract; a regression that drifts
// any single sequence trips phaseA's byte-equality assertion.
func expectedDarkTruecolor() map[theme.Role]string {
	return map[theme.Role]string{
		theme.RoleInfo:      "\x1b[38;2;220;220;220m",
		theme.RoleWarn:      "\x1b[38;2;255;176;0m",
		theme.RoleError:     "\x1b[38;2;255;64;64m",
		theme.RoleHighlight: "\x1b[38;2;0;200;220m",
		theme.RoleDim:       "\x1b[38;2;128;128;128m",
	}
}

// expectedLightTruecolor maps Role -> the spec 3.4 truecolor opening sequence
// for the light theme. Light error in particular (175;0;0) is the load-
// bearing distinguisher from dark error (255;64;64).
func expectedLightTruecolor() map[theme.Role]string {
	return map[theme.Role]string{
		theme.RoleInfo:      "\x1b[38;2;40;40;40m",
		theme.RoleWarn:      "\x1b[38;2;175;95;0m",
		theme.RoleError:     "\x1b[38;2;175;0;0m",
		theme.RoleHighlight: "\x1b[38;2;0;95;175m",
		theme.RoleDim:       "\x1b[38;2;138;138;138m",
	}
}

// phaseA: dark theme + truecolor depth -> each role is wrapped in the
// expected open code + Reset. Bytes are compared exactly.
func phaseA() error {
	fmt.Println("==> phase A: BUILT-IN-DARK (always runs)")

	reg := theme.NewThemeRegistry()
	dark, err := reg.Get(theme.ThemeDark)
	if err != nil {
		return fmt.Errorf("registry Get dark: %w", err)
	}
	if dark.Name != theme.ThemeDark {
		return fmt.Errorf("dark theme Name mismatch: got %q want %q", dark.Name, theme.ThemeDark)
	}

	st := theme.NewStyler(dark, theme.DepthTruecolor)
	want := expectedDarkTruecolor()

	for _, role := range theme.AllRoles() {
		styled := st.Stylize(role, "X")
		expectOpen := want[role]
		expected := expectOpen + "X" + theme.Reset
		if styled != expected {
			return fmt.Errorf("dark role=%s styled bytes mismatch:\n got  %q\n want %q",
				role, styled, expected)
		}
		if !strings.HasPrefix(styled, expectOpen) {
			return fmt.Errorf("dark role=%s missing open prefix %q", role, expectOpen)
		}
		if !strings.HasSuffix(styled, theme.Reset) {
			return fmt.Errorf("dark role=%s missing Reset suffix", role)
		}
		fmt.Printf("    phaseA: role=%-9s open-bytes=%s total-bytes=%d\n",
			role, hex.EncodeToString([]byte(expectOpen)), len(styled))
	}
	fmt.Printf("    verdict: dark theme rendered all 5 roles with pinned truecolor opens + Reset\n")
	return nil
}

// phaseB: light theme + truecolor depth -> each role is wrapped in the spec
// 3.4 light bytes; light error specifically must be 175;0;0 (NOT the dark
// 255;64;64).
func phaseB() error {
	fmt.Println("==> phase B: BUILT-IN-LIGHT (always runs)")

	reg := theme.NewThemeRegistry()
	light, err := reg.Get(theme.ThemeLight)
	if err != nil {
		return fmt.Errorf("registry Get light: %w", err)
	}
	if light.Name != theme.ThemeLight {
		return fmt.Errorf("light theme Name mismatch: got %q want %q", light.Name, theme.ThemeLight)
	}

	st := theme.NewStyler(light, theme.DepthTruecolor)
	want := expectedLightTruecolor()

	for _, role := range theme.AllRoles() {
		styled := st.Stylize(role, "X")
		expectOpen := want[role]
		expected := expectOpen + "X" + theme.Reset
		if styled != expected {
			return fmt.Errorf("light role=%s styled bytes mismatch:\n got  %q\n want %q",
				role, styled, expected)
		}
		fmt.Printf("    phaseB: role=%-9s open-bytes=%s total-bytes=%d\n",
			role, hex.EncodeToString([]byte(expectOpen)), len(styled))
	}

	// Cross-theme distinguisher: light error must NOT match dark error bytes.
	if want[theme.RoleError] == expectedDarkTruecolor()[theme.RoleError] {
		return fmt.Errorf("light/dark error truecolor bytes collided; spec 3.4 says they differ")
	}
	fmt.Printf("    verdict: light theme rendered all 5 roles; light-error %q != dark-error %q\n",
		want[theme.RoleError], expectedDarkTruecolor()[theme.RoleError])
	return nil
}

// phaseC: zero-color invariant. depth=DepthOff -> Stylize returns the input
// string byte-equal AND contains zero 0x1B bytes regardless of role/theme.
func phaseC() error {
	fmt.Println("==> phase C: PLAIN-ZERO-COLOR (always runs)")

	reg := theme.NewThemeRegistry()
	dark, err := reg.Get(theme.ThemeDark)
	if err != nil {
		return fmt.Errorf("registry Get dark: %w", err)
	}
	st := theme.NewStyler(dark, theme.DepthOff)

	const input = "X"
	for _, role := range theme.AllRoles() {
		out := st.Stylize(role, input)
		if out != input {
			return fmt.Errorf("zero-color role=%s: got %q want %q (byte-equal)", role, out, input)
		}
		if strings.ContainsRune(out, 0x1b) {
			return fmt.Errorf("zero-color role=%s: output contains ESC byte: %q", role, out)
		}
	}
	fmt.Printf("    phaseC: all 5 roles returned plain text (zero ANSI bytes)\n")
	fmt.Printf("    verdict: DepthOff invariant holds across every role; NO_COLOR / dumb path safe\n")
	return nil
}

// envFor returns an envLookup closure backed by the supplied map.
func envFor(env map[string]string) func(string) string {
	return func(k string) string { return env[k] }
}

// phaseD: six DetectColorDepth branches. Each input is a deterministic env
// map fed through a closure; each output is asserted against the spec.
func phaseD() error {
	fmt.Println("==> phase D: DEPTH-DETECT (always runs)")

	cases := []struct {
		name string
		env  map[string]string
		want theme.ColorDepth
	}{
		{
			// NO_COLOR overrides everything per NO_COLOR.org standard.
			name: "no-color-overrides-truecolor",
			env:  map[string]string{"NO_COLOR": "1", "COLORTERM": "truecolor", "TERM": "xterm-256color"},
			want: theme.DepthOff,
		},
		{
			name: "colorterm-truecolor",
			env:  map[string]string{"COLORTERM": "truecolor", "TERM": "xterm-256color"},
			want: theme.DepthTruecolor,
		},
		{
			name: "term-256color",
			env:  map[string]string{"TERM": "xterm-256color"},
			want: theme.DepthANSI256,
		},
		{
			name: "term-xterm-ansi16",
			env:  map[string]string{"TERM": "xterm"},
			want: theme.DepthANSI16,
		},
		{
			name: "term-dumb",
			env:  map[string]string{"TERM": "dumb"},
			want: theme.DepthOff,
		},
		{
			name: "all-unset",
			env:  map[string]string{},
			want: theme.DepthOff,
		},
	}

	for _, tc := range cases {
		got := theme.DetectColorDepth(envFor(tc.env))
		if got != tc.want {
			return fmt.Errorf("DetectColorDepth %s: got %s want %s (env=%v)",
				tc.name, got, tc.want, tc.env)
		}
	}
	fmt.Printf("    phaseD: 6 depth branches verified (truecolor, ansi256, ansi16, off x3)\n")
	fmt.Printf("    verdict: all branches returned the spec-mandated depth\n")
	return nil
}

// phaseE: write a real YAML on disk overriding only error. LoadFromFile must
// merge over the dark baseline so the resulting Custom() theme exposes 5
// roles where error is the override and the other four are byte-equal to
// dark.
func phaseE() error {
	fmt.Println("==> phase E: YAML-MERGE (always runs)")

	dir, err := os.MkdirTemp("", "p1f20-challenge-")
	if err != nil {
		return fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(dir)

	yamlPath := filepath.Join(dir, "theme.yaml")
	yamlBody := "name: my-custom\ncolors:\n  error:\n    ansi256: \"\\x1b[38;5;201m\"\n    truecolor: \"\\x1b[38;2;255;0;255m\"\n"
	if err := os.WriteFile(yamlPath, []byte(yamlBody), 0o600); err != nil {
		return fmt.Errorf("write yaml: %w", err)
	}

	reg := theme.NewThemeRegistry()
	if err := reg.LoadFromFile(yamlPath); err != nil {
		return fmt.Errorf("LoadFromFile: %w", err)
	}

	custom := reg.Custom()
	if custom == nil {
		return fmt.Errorf("Custom() returned nil after LoadFromFile")
	}
	if custom.Name != theme.ThemeName("my-custom") {
		return fmt.Errorf("custom Name mismatch: got %q want %q", custom.Name, "my-custom")
	}
	if got, want := len(custom.Colors), 5; got != want {
		return fmt.Errorf("custom Colors length: got %d want %d (full 5-role merge)", got, want)
	}

	// Override role: bytes must equal what the YAML carried.
	const wantErrorOpen = "\x1b[38;2;255;0;255m"
	const wantErrorANSI256 = "\x1b[38;5;201m"
	gotErr := custom.Colors[theme.RoleError]
	if gotErr.OpenTruecolor != wantErrorOpen {
		return fmt.Errorf("error truecolor: got %q want %q", gotErr.OpenTruecolor, wantErrorOpen)
	}
	if gotErr.OpenANSI256 != wantErrorANSI256 {
		return fmt.Errorf("error ansi256: got %q want %q", gotErr.OpenANSI256, wantErrorANSI256)
	}

	// Inheritance roles: bytes must equal the dark baseline byte-for-byte.
	dark := theme.BuiltinDarkTheme()
	for _, role := range []theme.Role{theme.RoleInfo, theme.RoleWarn, theme.RoleHighlight, theme.RoleDim} {
		darkColor := dark.Colors[role]
		gotColor := custom.Colors[role]
		if gotColor != darkColor {
			return fmt.Errorf("role=%s did not inherit dark baseline:\n got  %+v\n want %+v",
				role, gotColor, darkColor)
		}
	}

	// Crucially, error MUST differ from dark error to prove the override
	// actually merged (not silently dropped).
	if custom.Colors[theme.RoleError].OpenTruecolor == dark.Colors[theme.RoleError].OpenTruecolor {
		return fmt.Errorf("error override collapsed to dark baseline; YAML merge bluffed")
	}

	fmt.Printf("    phaseE: YAML override merged - error=custom, info/warn/highlight/dim=dark-baseline; 5/5 roles present\n")
	fmt.Printf("    verdict: real YAML on disk parsed, real merge over dark baseline, custom theme retrievable\n")
	return nil
}
