package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestHelixThemeAppliesBrandPalette asserts that initHelixTheme() forces the
// HelixCode brand dark palette onto the global tview.Styles. tview widgets
// read their default colors from tview.Styles at construction time, so this
// is the load-bearing guarantee that the whole TUI inherits the brand.
//
// RED-then-GREEN: before initHelixTheme() existed (or if it stops setting the
// palette) PrimaryTextColor would be tview's stock white, not the brand
// FG_TEXT #ECF3E8 — and this test fails.
func TestHelixThemeAppliesBrandPalette(t *testing.T) {
	// Reset to a known non-brand state so the assertion proves initHelixTheme
	// actually performed the mutation rather than relying on package init order.
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor: tcell.ColorBlack,
		PrimaryTextColor:         tcell.ColorWhite,
		BorderColor:              tcell.ColorWhite,
		TitleColor:               tcell.ColorWhite,
	}

	initHelixTheme()

	cases := []struct {
		name string
		got  tcell.Color
		want tcell.Color
	}{
		{"PrimaryTextColor == FG_TEXT", tview.Styles.PrimaryTextColor, tcell.GetColor("#ECF3E8")},
		{"PrimitiveBackgroundColor == BG_BASE", tview.Styles.PrimitiveBackgroundColor, tcell.GetColor("#0E1310")},
		{"ContrastBackgroundColor == BG_SURFACE", tview.Styles.ContrastBackgroundColor, tcell.GetColor("#18201A")},
		{"BorderColor == BORDER", tview.Styles.BorderColor, tcell.GetColor("#2A352C")},
		{"TitleColor == PRIMARY (lime)", tview.Styles.TitleColor, tcell.GetColor("#A8DD22")},
		{"GraphicsColor == SECONDARY (teal)", tview.Styles.GraphicsColor, tcell.GetColor("#8FC9B8")},
		{"SecondaryTextColor == FG_MUTED", tview.Styles.SecondaryTextColor, tcell.GetColor("#9DB0A0")},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v (#%06X), want %v (#%06X)",
				c.name, c.got, c.got.Hex(), c.want, c.want.Hex())
		}
	}
}

// TestHelixThemeStructCarriesBrandPalette guards the ThemeManager-served brand
// theme (HelixTheme) — the value returned by detectSystemTheme() on non-macOS
// and read by ThemeManager.GetColor/FormatColor. It MUST mirror the FACT brand
// constants so ThemeManager-driven colors stay consistent with the tview.Styles
// initHelixTheme() applies. RED-then-GREEN: the pre-brand struct carried
// "#C2E95B"/"#1A1A1A" and this test fails on any such regression.
func TestHelixThemeStructCarriesBrandPalette(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"Primary == lime", HelixTheme.Primary, hxcPrimaryHex},
		{"Secondary == teal", HelixTheme.Secondary, hxcSecondaryHex},
		{"Text == FG_TEXT", HelixTheme.Text, hxcFgTextHex},
		{"Background == BG_BASE", HelixTheme.Background, hxcBgBaseHex},
		{"Border == BORDER", HelixTheme.Border, hxcBorderHex},
		{"Error == ERROR", HelixTheme.Error, hxcErrorHex},
	}
	if !HelixTheme.IsDark {
		t.Error("HelixTheme.IsDark must be true (brand theme is dark)")
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("HelixTheme.%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestHelixPaletteConstantsResolve guards the FACT palette: each hex literal
// must resolve to the documented RGB triple via tcell so the brand hues are
// exact on true-color terminals.
func TestHelixPaletteConstantsResolve(t *testing.T) {
	pairs := []struct {
		name      string
		resolved  tcell.Color
		hex       string
		wantR     int32
		wantG     int32
		wantB     int32
	}{
		{"PRIMARY", hxcPrimary, hxcPrimaryHex, 0xA8, 0xDD, 0x22},
		{"SECONDARY", hxcSecondary, hxcSecondaryHex, 0x8F, 0xC9, 0xB8},
		{"FG_TEXT", hxcFgText, hxcFgTextHex, 0xEC, 0xF3, 0xE8},
		{"ERROR", hxcError, hxcErrorHex, 0xE0, 0x6A, 0x5A},
	}
	for _, p := range pairs {
		r, g, b := p.resolved.RGB()
		if r != p.wantR || g != p.wantG || b != p.wantB {
			t.Errorf("%s (%s): got rgb(%d,%d,%d), want rgb(%d,%d,%d)",
				p.name, p.hex, r, g, b, p.wantR, p.wantG, p.wantB)
		}
	}
}
