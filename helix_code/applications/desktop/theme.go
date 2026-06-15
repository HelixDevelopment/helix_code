//go:build !nogui

package main

import (
	"fmt"
	"image/color"
	"os"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// HelixCode brand palette (FACT hex values, derived from assets/Logo.png — a
// lime-green→teal nautilus spiral on near-black). These are the canonical brand
// colors applied to the Fyne desktop client. The brand theme is always dark.
const (
	hxcBGBase     = "#0E1310" // window background — near-black, faint green tint
	hxcBGSurface  = "#18201A" // panels / inputs
	hxcBGRaised   = "#202A22" // buttons / raised surfaces
	hxcBorder     = "#2A352C" // separators / borders / shadows
	hxcPrimary    = "#A8DD22" // lime — primary accent: focus, primary buttons, selection
	hxcPrimaryDim = "#7FA81B" // dimmed lime — selection background, pressed
	hxcSecondary  = "#8FC9B8" // teal — hyperlinks, secondary/info accents
	hxcFGText     = "#ECF3E8" // foreground text
	hxcFGMuted    = "#9DB0A0" // disabled / placeholder text
	hxcSuccess    = "#A8DD22" // success (lime)
	hxcError      = "#E06A5A" // error
	hxcWarning    = "#E0C040" // warning
)

// Theme represents a UI theme
type Theme struct {
	Name       string
	IsDark     bool
	Primary    string
	Secondary  string
	Accent     string
	Text       string
	Background string
	Border     string
	Success    string
	Warning    string
	Error      string
	Info       string

	// HelixCode brand extension. When IsBrand is true, the CustomTheme.Color
	// mapping uses the full brand palette (distinct surface/raised/muted/dim
	// tones) instead of the legacy 6-field approximation, and forces the dark
	// Fyne variant regardless of the host OS appearance setting.
	IsBrand      bool
	BGSurface    string // panel / input background
	BGRaised     string // button / raised-surface background
	PrimaryDim   string // selection background / pressed accent
	TextMuted    string // disabled / placeholder text
}

// Available themes
var (
	DarkTheme = Theme{
		Name:       "Dark",
		IsDark:     true,
		Primary:    "#2E86AB",
		Secondary:  "#A23B72",
		Accent:     "#F18F01",
		Text:       "#FFFFFF",
		Background: "#1E1E1E",
		Border:     "#404040",
		Success:    "#4CAF50",
		Warning:    "#FF9800",
		Error:      "#F44336",
		Info:       "#2196F3",
	}

	LightTheme = Theme{
		Name:       "Light",
		IsDark:     false,
		Primary:    "#1976D2",
		Secondary:  "#7B1FA2",
		Accent:     "#FF6F00",
		Text:       "#212121",
		Background: "#FFFFFF",
		Border:     "#BDBDBD",
		Success:    "#4CAF50",
		Warning:    "#FF9800",
		Error:      "#F44336",
		Info:       "#2196F3",
	}

	// HelixTheme is the HelixCode brand dark theme derived from assets/Logo.png
	// (the lime-green→teal nautilus spiral). It is always dark and uses the full
	// brand palette (IsBrand=true) so the CustomTheme.Color mapping resolves the
	// distinct surface / raised / muted / dim brand tones.
	HelixTheme = Theme{
		Name:       "Helix",
		IsDark:     true,
		IsBrand:    true,
		Primary:    hxcPrimary,
		Secondary:  hxcSecondary,
		Accent:     hxcPrimary,
		Text:       hxcFGText,
		Background: hxcBGBase,
		Border:     hxcBorder,
		Success:    hxcSuccess,
		Warning:    hxcWarning,
		Error:      hxcError,
		Info:       hxcSecondary,
		BGSurface:  hxcBGSurface,
		BGRaised:   hxcBGRaised,
		PrimaryDim: hxcPrimaryDim,
		TextMuted:  hxcFGMuted,
	}
)

// ThemeManager manages UI themes
type ThemeManager struct {
	currentTheme *Theme
	themes       map[string]*Theme
}

// NewThemeManager creates a new theme manager
func NewThemeManager() *ThemeManager {
	tm := &ThemeManager{
		themes: make(map[string]*Theme),
	}

	// Register themes
	tm.themes["dark"] = &DarkTheme
	tm.themes["light"] = &LightTheme
	tm.themes["helix"] = &HelixTheme

	// Set default theme based on system preference
	tm.currentTheme = tm.detectSystemTheme()

	return tm
}

// detectSystemTheme detects the system's preferred theme
func (tm *ThemeManager) detectSystemTheme() *Theme {
	// Check environment variables
	if theme := os.Getenv("HELIX_THEME"); theme != "" {
		if t, exists := tm.themes[strings.ToLower(theme)]; exists {
			return t
		}
	}

	// HelixCode brand default: the brand dark theme (derived from
	// assets/Logo.png) is applied on EVERY platform regardless of the host OS
	// appearance setting — the Fyne client renders in the HelixCode brand
	// identity, not the system light/dark preference. HELIX_THEME env var
	// (checked above) remains the operator escape hatch to select "dark" or
	// "light" explicitly. runtime is still imported for callers that branch on
	// GOOS elsewhere; the brand theme intentionally ignores it here so macOS,
	// Linux, and Windows all show the same brand identity.
	_ = runtime.GOOS
	return &HelixTheme
}

// GetCurrentTheme returns the current theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.currentTheme
}

// SetTheme sets the current theme
func (tm *ThemeManager) SetTheme(themeName string) bool {
	if theme, exists := tm.themes[strings.ToLower(themeName)]; exists {
		tm.currentTheme = theme
		return true
	}
	return false
}

// GetAvailableThemes returns list of available theme names
func (tm *ThemeManager) GetAvailableThemes() []string {
	names := make([]string, 0, len(tm.themes))
	for name := range tm.themes {
		names = append(names, name)
	}
	return names
}

// GetColor returns a color code for the given type
func (tm *ThemeManager) GetColor(colorType string) string {
	theme := tm.currentTheme
	switch strings.ToLower(colorType) {
	case "primary":
		return theme.Primary
	case "secondary":
		return theme.Secondary
	case "accent":
		return theme.Accent
	case "text":
		return theme.Text
	case "background":
		return theme.Background
	case "border":
		return theme.Border
	case "success":
		return theme.Success
	case "warning":
		return theme.Warning
	case "error":
		return theme.Error
	case "info":
		return theme.Info
	default:
		return theme.Text
	}
}

// CustomTheme implements fyne.Theme for HelixCode
type CustomTheme struct {
	currentTheme *Theme
}

// NewCustomTheme creates a new custom theme
func NewCustomTheme() *CustomTheme {
	tm := NewThemeManager()
	return &CustomTheme{
		currentTheme: tm.GetCurrentTheme(),
	}
}

// SetTheme sets the current theme
func (ct *CustomTheme) SetTheme(themeName string) {
	tm := NewThemeManager()
	if tm.SetTheme(themeName) {
		ct.currentTheme = tm.GetCurrentTheme()
	}
}

// PrimaryColor returns the primary color
func (ct *CustomTheme) PrimaryColor() color.Color {
	return parseHexColor(ct.currentTheme.Primary)
}

// HyperlinkColor returns the hyperlink color
func (ct *CustomTheme) HyperlinkColor() color.Color {
	return parseHexColor(ct.currentTheme.Accent)
}

// TextColor returns the text color
func (ct *CustomTheme) TextColor() color.Color {
	return parseHexColor(ct.currentTheme.Text)
}

// BackgroundColor returns the background color
func (ct *CustomTheme) BackgroundColor() color.Color {
	return parseHexColor(ct.currentTheme.Background)
}

// ButtonColor returns the button color
func (ct *CustomTheme) ButtonColor() color.Color {
	return parseHexColor(ct.currentTheme.Secondary)
}

// DisabledButtonColor returns the disabled button color
func (ct *CustomTheme) DisabledButtonColor() color.Color {
	return parseHexColor(ct.currentTheme.Border)
}

// DisabledTextColor returns the disabled text color
func (ct *CustomTheme) DisabledTextColor() color.Color {
	return parseHexColor(ct.currentTheme.Border)
}

// IconColor returns the icon color
func (ct *CustomTheme) IconColor() color.Color {
	return parseHexColor(ct.currentTheme.Text)
}

// DisabledIconColor returns the disabled icon color
func (ct *CustomTheme) DisabledIconColor() color.Color {
	return parseHexColor(ct.currentTheme.Border)
}

// PrimaryBorderColor returns the active theme's Border color (round-33
// §11.4 doc-comment correction — the prior "returns the placeholder
// color" comment was mechanically wrong, the function returns the real
// configured border color; CONST-035 / Article XI §11.9).
func (ct *CustomTheme) PrimaryBorderColor() color.Color {
	return parseHexColor(ct.currentTheme.Border)
}

// PrimaryTextColor returns the primary text color
func (ct *CustomTheme) PrimaryTextColor() color.Color {
	return parseHexColor(ct.currentTheme.Text)
}

// FocusColor returns the focus color
func (ct *CustomTheme) FocusColor() color.Color {
	return parseHexColor(ct.currentTheme.Accent)
}

// ScrollBarColor returns the scrollbar color
func (ct *CustomTheme) ScrollBarColor() color.Color {
	return parseHexColor(ct.currentTheme.Border)
}

// ShadowColor returns the shadow color
func (ct *CustomTheme) ShadowColor() color.Color {
	return color.RGBA{0, 0, 0, 100}
}

// TextSize returns the text size
func (ct *CustomTheme) TextSize() float32 {
	return 14
}

// TextFont returns the text font
func (ct *CustomTheme) TextFont() fyne.Resource {
	return theme.DefaultTextFont()
}

// TextBoldFont returns the bold text font
func (ct *CustomTheme) TextBoldFont() fyne.Resource {
	return theme.DefaultTextBoldFont()
}

// TextItalicFont returns the italic text font
func (ct *CustomTheme) TextItalicFont() fyne.Resource {
	return theme.DefaultTextItalicFont()
}

// TextBoldItalicFont returns the bold italic text font
func (ct *CustomTheme) TextBoldItalicFont() fyne.Resource {
	return theme.DefaultTextBoldItalicFont()
}

// TextMonospaceFont returns the monospace text font
func (ct *CustomTheme) TextMonospaceFont() fyne.Resource {
	return theme.DefaultTextMonospaceFont()
}

// HeadingTextSize returns the heading text size
func (ct *CustomTheme) HeadingTextSize() float32 {
	return 24
}

// Padding returns the padding
func (ct *CustomTheme) Padding() float32 {
	return 4
}

// IconInlineSize returns the inline icon size
func (ct *CustomTheme) IconInlineSize() float32 {
	return 20
}

// ScrollBarSize returns the scrollbar size
func (ct *CustomTheme) ScrollBarSize() float32 {
	return 16
}

// ScrollBarSmallSize returns the small scrollbar size
func (ct *CustomTheme) ScrollBarSmallSize() float32 {
	return 12
}

// parseHexColor parses a hex color string to color.Color
func parseHexColor(hex string) color.Color {
	// Remove # if present
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}

	// Parse hex color (RRGGBB or RRGGBBAA)
	if len(hex) == 6 || len(hex) == 8 {
		var r, g, b uint8
		a := uint8(255)
		if n, err := parseHexPair(hex[0:2]); err == nil {
			r = n
		}
		if n, err := parseHexPair(hex[2:4]); err == nil {
			g = n
		}
		if n, err := parseHexPair(hex[4:6]); err == nil {
			b = n
		}
		if len(hex) == 8 {
			if n, err := parseHexPair(hex[6:8]); err == nil {
				a = n
			}
		}
		return color.NRGBA{R: r, G: g, B: b, A: a}
	}

	// Default to black if parsing fails
	return color.Black
}

// Color returns the color for the given theme color name
func (ct *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// HXC: nil-guard — Fyne queries Color() during canvas creation; a
	// CustomTheme built without NewCustomTheme (zero value) has a nil
	// currentTheme and would nil-deref (desktop GUI launch SIGSEGV). Lazily
	// initialise so every construction path is crash-safe.
	if ct.currentTheme == nil {
		ct.currentTheme = NewThemeManager().GetCurrentTheme()
	}

	// HelixCode brand mapping: when the active theme is the brand theme, map
	// EVERY Fyne ColorName onto the full brand palette so the rendered GUI
	// carries the brand identity end-to-end. The brand theme is always dark, so
	// the `variant` argument is intentionally ignored — the brand colors apply
	// regardless of the host OS light/dark setting.
	t := ct.currentTheme
	if t.IsBrand {
		switch name {
		case theme.ColorNamePrimary:
			return parseHexColor(t.Primary) // lime
		case theme.ColorNameBackground:
			return parseHexColor(t.Background) // BG_BASE
		case theme.ColorNameForeground:
			return parseHexColor(t.Text) // FG_TEXT
		case theme.ColorNameButton:
			return parseHexColor(t.BGRaised) // BG_RAISED
		case theme.ColorNameDisabledButton:
			return parseHexColor(t.BGSurface)
		case theme.ColorNameDisabled:
			return parseHexColor(t.TextMuted) // FG_MUTED
		case theme.ColorNamePlaceHolder:
			return parseHexColor(t.TextMuted) // FG_MUTED
		case theme.ColorNameInputBackground:
			return parseHexColor(t.BGSurface) // BG_SURFACE
		case theme.ColorNameHyperlink:
			return parseHexColor(t.Secondary) // teal
		case theme.ColorNameError:
			return parseHexColor(t.Error)
		case theme.ColorNameSuccess:
			return parseHexColor(t.Success)
		case theme.ColorNameWarning:
			return parseHexColor(t.Warning)
		case theme.ColorNameFocus:
			return parseHexColor(t.Primary) // lime focus ring
		case theme.ColorNamePressed:
			return parseHexColor(t.PrimaryDim)
		case theme.ColorNameSelection:
			return parseHexColor(t.PrimaryDim) // PRIMARY_DIM
		case theme.ColorNameSeparator:
			return parseHexColor(t.Border) // BORDER
		case theme.ColorNameShadow:
			return parseHexColor(t.Border) // BORDER
		case theme.ColorNameScrollBar:
			return parseHexColor(t.Border)
		case theme.ColorNameMenuBackground:
			return parseHexColor(t.BGSurface)
		case theme.ColorNameOverlayBackground:
			return parseHexColor(t.BGBaseOverlay())
		case theme.ColorNameInputBorder:
			return parseHexColor(t.Border)
		case theme.ColorNameHeaderBackground:
			return parseHexColor(t.BGSurface)
		case theme.ColorNameHover:
			return parseHexColor(t.BGRaised)
		case theme.ColorNameScrollBarBackground:
			return parseHexColor(t.BGSurface)
		// On bright brand accents (lime primary/success, amber warning) the
		// legible foreground is the near-black base, not the light FG_TEXT.
		case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
			return parseHexColor(t.Background)
		case theme.ColorNameForegroundOnError:
			return parseHexColor(t.Text)
		default:
			return parseHexColor(t.Text)
		}
	}

	switch name {
	case theme.ColorNamePrimary:
		return parseHexColor(t.Primary)
	case theme.ColorNameBackground:
		return parseHexColor(t.Background)
	case theme.ColorNameButton:
		return parseHexColor(t.Secondary)
	case theme.ColorNameDisabledButton:
		return parseHexColor(t.Border)
	case theme.ColorNameError:
		return parseHexColor(t.Error)
	case theme.ColorNameFocus:
		return parseHexColor(t.Accent)
	case theme.ColorNameForeground:
		return parseHexColor(t.Text)
	case theme.ColorNameDisabled:
		return parseHexColor(t.Border)
	case theme.ColorNamePlaceHolder:
		return parseHexColor(t.Border)
	case theme.ColorNamePressed:
		return parseHexColor(t.Accent)
	case theme.ColorNameScrollBar:
		return parseHexColor(t.Border)
	case theme.ColorNameShadow:
		return color.RGBA{0, 0, 0, 100}
	case theme.ColorNameInputBackground:
		if t.IsDark {
			return parseHexColor("#2A2A2A")
		}
		return parseHexColor("#F5F5F5")
	case theme.ColorNameMenuBackground:
		return parseHexColor(t.Background)
	case theme.ColorNameOverlayBackground:
		return color.RGBA{0, 0, 0, 150}
	case theme.ColorNameSeparator:
		return parseHexColor(t.Border)
	default:
		return parseHexColor(t.Text)
	}
}

// BGBaseOverlay returns a translucent variant of the brand base background used
// for modal/overlay scrims. It composes the brand base color with ~80% alpha so
// dialogs dim the content behind them while staying on-brand.
func (th *Theme) BGBaseOverlay() string {
	return th.Background + "CC" // RRGGBB + AA(0xCC ≈ 80%)
}

// Font returns the font for the given theme font style
func (ct *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return theme.DefaultTextMonospaceFont()
	}
	if style.Bold {
		if style.Italic {
			return theme.DefaultTextBoldItalicFont()
		}
		return theme.DefaultTextBoldFont()
	}
	if style.Italic {
		return theme.DefaultTextItalicFont()
	}
	return theme.DefaultTextFont()
}

// Size returns the size for the given theme size name
func (ct *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameScrollBar:
		return 16
	case theme.SizeNameScrollBarSmall:
		return 12
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInlineIcon:
		return 20
	default:
		return 14
	}
}

// Icon returns the icon for the given theme icon name
func (ct *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// parseHexPair parses a hex pair to uint8
func parseHexPair(s string) (uint8, error) {
	var result uint8
	for _, c := range s {
		result <<= 4
		switch {
		case '0' <= c && c <= '9':
			result |= uint8(c - '0')
		case 'a' <= c && c <= 'f':
			result |= uint8(c - 'a' + 10)
		case 'A' <= c && c <= 'F':
			result |= uint8(c - 'A' + 10)
		default:
			return 0, fmt.Errorf("invalid hex character: %c", c)
		}
	}
	return result, nil
}
