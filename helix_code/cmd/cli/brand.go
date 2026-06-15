package main

// brand.go — HelixCode CLI brand identity.
//
// The palette is derived from assets/Logo.png (a lime-green → teal
// nautilus spiral). Colors are emitted as ANSI 24-bit (truecolor)
// foreground escapes so the wordmark + accents render in the exact
// brand hues on any truecolor-capable terminal. Color is suppressed
// when NO_COLOR is set (https://no-color.org) or when stdout is not a
// character device (pipes, files, non-TTYs) — see colorEnabled().
//
// FACT palette (hex → R;G;B):
//   PRIMARY   (lime) #A8DD22 → 168;221;34
//   SECONDARY (teal) #8FC9B8 → 143;201;184
//   FG_MUTED         #9DB0A0 → 157;176;160
//   ERROR            #E06A5A → 224;106;90

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI 24-bit foreground escape codes for the brand palette. Each is the
// payload of \x1b[38;2;<code>m. Kept as bare "R;G;B" strings so colorize
// can wrap them and tests can assert the exact triplet is present.
const (
	brandPrimary   = "38;2;168;221;34"  // lime  #A8DD22 — wordmark, success, primary headings, prompt marker
	brandSecondary = "38;2;143;201;184" // teal  #8FC9B8 — subtitle, info/secondary
	brandMuted     = "38;2;157;176;160" // muted #9DB0A0 — de-emphasised text
	brandError     = "38;2;224;106;90"  // red   #E06A5A — errors

	ansiReset = "\x1b[0m"
)

// colorStdoutIsTTY is the TTY check used by colorEnabled. It is a package
// variable so tests can drive both branches deterministically without an
// actual terminal attached.
var colorStdoutIsTTY = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// colorEnabled reports whether brand color should be emitted. Color is
// ON only when NO_COLOR is unset AND stdout is a real terminal (char
// device). This honors https://no-color.org and keeps piped/redirected
// output free of escape sequences.
func colorEnabled() bool {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}
	return colorStdoutIsTTY()
}

// colorize wraps s in the given ANSI 24-bit foreground escape + reset.
// When color is disabled (NO_COLOR set or non-TTY) it returns s
// unchanged — no escape bytes are emitted at all.
func colorize(code, s string) string {
	if code == "" || !colorEnabled() {
		return s
	}
	return "\x1b[" + code + "m" + s + ansiReset
}

// Brand accent helpers — semantic wrappers over colorize so call sites
// read by intent, not by raw color code.
func brandSuccess(s string) string   { return colorize(brandPrimary, s) }
func brandHeading(s string) string   { return colorize(brandPrimary, s) }
func brandInfo(s string) string      { return colorize(brandSecondary, s) }
func brandErrorText(s string) string { return colorize(brandError, s) }
func brandPrompt(s string) string    { return colorize(brandPrimary, s) }

// brandBanner returns the startup wordmark + tagline. The "HelixCode"
// wordmark is rendered in lime; an ASCII nautilus-spiral motif and the
// tagline are rendered in teal, evoking the Logo.png spiral. When color
// is disabled the same layout is returned with no escape sequences, so
// the banner remains a clean, readable wordmark in pipes/files.
//
// The literal "HelixCode" always appears in the output (color-enabled or
// not) so brand identity is never lost.
func brandBanner() string {
	// ASCII art evoking the lime→teal nautilus spiral of the logo.
	const spiral = "" +
		"        ___\n" +
		"      ,'   `.\n" +
		"     /  ,-.  \\\n" +
		"    |  ( @ )  |   ~ HelixCode\n" +
		"     \\  `-'  /\n" +
		"      `.___,'\n"

	var b strings.Builder
	b.WriteString(colorize(brandSecondary, spiral))
	b.WriteString(colorize(brandPrimary, "  HelixCode"))
	b.WriteString(colorize(brandMuted, " — "))
	b.WriteString(colorize(brandSecondary, "AI development, end to end"))
	b.WriteString("\n")
	return b.String()
}
