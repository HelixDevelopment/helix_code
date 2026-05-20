package config

import "context"

// PlatformAdapter represents a platform-specific UI adapter
type PlatformAdapter struct {
	platformType string
	features     []string
	themes       map[string]PlatformTheme
	currentTheme string
}

// PlatformTheme represents a platform theme
type PlatformTheme struct {
	Name        string
	Description string
	Primary     string
	Secondary   string
	Background  string
	Foreground  string
}

// NewBasePlatformAdapter creates a new base platform adapter
func NewBasePlatformAdapter(platformType string) *PlatformAdapter {
	return &PlatformAdapter{
		platformType: platformType,
		features:     []string{"native_menus", "system_tray", "file_dialogs", "notifications"},
		themes: map[string]PlatformTheme{
			"system": {
				Name:        "System",
				Description: "Matches system appearance",
				Primary:     "",
				Secondary:   "",
				Background:  "",
				Foreground:  "",
			},
			"light": {
				Name:        "Light",
				Description: "Light theme with bright background",
				Primary:     "#007bff",
				Secondary:   "#6c757d",
				Background:  "#ffffff",
				Foreground:  "#000000",
			},
			"dark": {
				Name:        "Dark",
				Description: "Dark theme with dark background",
				Primary:     "#0d6efd",
				Secondary:   "#6c757d",
				Background:  "#1a1a1a",
				Foreground:  "#ffffff",
			},
		},
		currentTheme: "system",
	}
}

// GetPlatformType returns the platform type
func (a *PlatformAdapter) GetPlatformType() string {
	return a.platformType
}

// GetPlatformFeatures returns the platform features
func (a *PlatformAdapter) GetPlatformFeatures() []string {
	return a.features
}

// GetPlatformThemes returns the platform themes
func (a *PlatformAdapter) GetPlatformThemes() map[string]PlatformTheme {
	return a.themes
}

// GetDefaultTheme returns the default theme for the platform
func (a *PlatformAdapter) GetDefaultTheme() string {
	switch a.platformType {
	case "desktop":
		return "system"
	case "web":
		return "light"
	case "mobile":
		return "system"
	default:
		return "light"
	}
}

// GetSupportedFormats returns the supported formats for the platform
func (a *PlatformAdapter) GetSupportedFormats() []string {
	switch a.platformType {
	case "desktop":
		return []string{"png", "jpg", "svg", "ico"}
	case "web":
		return []string{"png", "svg", "webp"}
	case "mobile":
		return []string{"png", "svg"}
	default:
		return []string{"png"}
	}
}

// GetResolutionInfo returns resolution information for the platform
func (a *PlatformAdapter) GetResolutionInfo() map[string]interface{} {
	switch a.platformType {
	case "desktop":
		return map[string]interface{}{
			"default_dpi":   96,
			"scale_factors": []float64{1.0, 1.25, 1.5, 2.0},
			"min_width":     800,
			"min_height":    600,
		}
	case "web":
		return map[string]interface{}{
			"default_dpi":   96,
			"scale_factors": []float64{1.0, 1.5, 2.0},
			"min_width":     320,
			"min_height":    568,
		}
	case "mobile":
		return map[string]interface{}{
			"default_dpi":   160,
			"scale_factors": []float64{1.0, 2.0, 3.0},
			"min_width":     320,
			"min_height":    480,
		}
	default:
		return map[string]interface{}{
			"default_dpi":   96,
			"scale_factors": []float64{1.0},
			"min_width":     800,
			"min_height":    600,
		}
	}
}

// ConfigUI represents configuration UI interface
type ConfigUI struct {
	configPath string
	config     *Config
}

// DesktopConfigForm represents desktop configuration form
type DesktopConfigForm struct {
	ID            string
	Title         string
	Type          string
	Layout        string
	Modal         bool
	Resizable     bool
	MinWidth      int
	MinHeight     int
	DefaultWidth  int
	DefaultHeight int
	CenterScreen  bool
	Sections      []DesktopConfigSection
	Actions       []DesktopConfigAction
}

// DesktopConfigSection represents a section in desktop config form
type DesktopConfigSection struct {
	ID       string
	Title    string
	Icon     string
	Type     string
	Expanded bool
	Fields   []DesktopConfigField
}

// DesktopConfigField represents a field in desktop config section
type DesktopConfigField struct {
	ID          string
	Label       string
	Type        string
	Class       string
	Disabled    bool
	Visible     bool
	Keyboard    string
	Value       interface{}
	Required    bool
	Placeholder string
	Help        string
	HelpText    string
	Options     []string
}

// DesktopConfigAction represents an action in desktop config form
type DesktopConfigAction struct {
	ID       string
	Label    string
	Type     string
	Icon     string
	Shortcut string
	Default  bool
	Cancel   bool
	Position string
}

// WebConfigForm represents web configuration form
type WebConfigForm struct {
	ID           string
	Title        string
	Type         string
	Layout       string
	Responsive   bool
	JavaScript   []string
	CSS          []string
	Validation   WebConfigValidation
	SubmitAction WebConfigSubmit
	Sections     []WebConfigSection
	Actions      []WebConfigAction
}

// WebConfigValidation represents validation configuration
type WebConfigValidation struct {
	Enabled  bool
	OnSubmit bool
	OnBlur   bool
	Realtime bool
	Custom   []string
}

// WebConfigSubmit represents submit configuration
type WebConfigSubmit struct {
	URL     string
	Method  string
	Headers map[string]string
	Success string
	Error   string
}

// WebConfigSection represents a section in web config form
type WebConfigSection struct {
	ID       string
	Title    string
	Icon     string
	Type     string
	Expanded bool
	Fields   []WebConfigField
}

// WebConfigField represents a field in web config section
type WebConfigField struct {
	ID          string
	Label       string
	Type        string
	Class       string
	Disabled    bool
	Visible     bool
	Keyboard    string
	Value       interface{}
	Required    bool
	Placeholder string
	Help        string
	HelpText    string
	Options     []string
}

// WebConfigAction represents an action in web config form
type WebConfigAction struct {
	ID      string
	Label   string
	Type    string
	Icon    string
	Default bool
}

// MobileConfigForm represents mobile configuration form
type MobileConfigForm struct {
	ID         string
	Title      string
	Type       string
	Layout     string
	Responsive bool
	Gestures   []string
	Sections   []MobileConfigSection
	Actions    []MobileConfigAction
}

// MobileConfigSection represents a section in mobile config form
type MobileConfigSection struct {
	ID       string
	Title    string
	Icon     string
	Type     string
	Expanded bool
	Fields   []MobileConfigField
}

// MobileConfigField represents a field in mobile config section
type MobileConfigField struct {
	ID          string
	Label       string
	Type        string
	Class       string
	Disabled    bool
	Visible     bool
	Keyboard    string
	Value       interface{}
	Required    bool
	Placeholder string
	Help        string
	HelpText    string
	Options     []string
}

// MobileConfigAction represents an action in mobile config form
type MobileConfigAction struct {
	ID      string
	Label   string
	Type    string
	Icon    string
	Default bool
}

// TUIConfigForm represents TUI configuration form
type TUIConfigForm struct {
	ID          string
	Title       string
	Type        string
	Layout      string
	BorderStyle string
	Colors      TUIColors
	KeyBindings map[string]string
	Sections    []TUIConfigSection
	Actions     []TUIConfigAction
}

// TUIColors represents color configuration for TUI
type TUIColors struct {
	Title     string
	Highlight string
	Text      string
	Border    string
	Error     string
	Success   string
}

// TUIConfigSection represents a section in TUI config form
type TUIConfigSection struct {
	ID       string
	Title    string
	Icon     string
	Type     string
	Expanded bool
	Fields   []TUIConfigField
}

// TUIConfigField represents a field in TUI config section
type TUIConfigField struct {
	ID          string
	Label       string
	Type        string
	Class       string
	Disabled    bool
	Visible     bool
	Keyboard    string
	Value       interface{}
	Required    bool
	Placeholder string
	Help        string
	HelpText    string
	Options     []string
}

// TUIConfigAction represents an action in TUI config form
type TUIConfigAction struct {
	ID      string
	Label   string
	Type    string
	Icon    string
	Default bool
}

// RenderConfigForm renders configuration form for TUI platform
func (a *TUIAdapter) RenderConfigForm(formType string) interface{} {
	ctx := context.Background()
	form := TUIConfigForm{
		ID:          "helix_config_form",
		Title:       tr(ctx, "internal_config_ui_form_title", nil),
		Type:        "tui_screens",
		Layout:      "menu_driven",
		BorderStyle: "single",
		Colors: TUIColors{
			Title:     "yellow",
			Highlight: "cyan",
			Text:      "white",
			Border:    "blue",
			Error:     "red",
			Success:   "green",
		},
		KeyBindings: map[string]string{
			"save":       "Ctrl+S",
			"reset":      "Ctrl+R",
			"quit":       "Ctrl+Q",
			"next_field": "Tab",
			"prev_field": "Shift+Tab",
			"select":     "Enter",
			"cancel":     "Esc",
		},
		Sections: []TUIConfigSection{
			{
				ID:       "application",
				Title:    tr(ctx, "internal_config_ui_section_application", nil),
				Type:     "form",
				Expanded: true,
				Fields: []TUIConfigField{
					{
						ID:          "app_name",
						Type:        "text",
						Label:       tr(ctx, "internal_config_ui_field_app_name_label", nil),
						HelpText:    tr(ctx, "internal_config_ui_field_app_name_help", nil),
						Required:    true,
						Placeholder: "My Application",
					},
					{
						ID:          "app_version",
						Type:        "text",
						Label:       tr(ctx, "internal_config_ui_field_app_version_label", nil),
						HelpText:    tr(ctx, "internal_config_ui_field_app_version_help", nil),
						Required:    true,
						Placeholder: "1.0.0",
					},
				},
			},
		},
		Actions: []TUIConfigAction{},
	}
	return form
}

// NewConfigUI creates a new configuration UI
func NewConfigUI(configPath string) (*ConfigUI, error) {
	return &ConfigUI{
		configPath: configPath,
		config:     &Config{},
	}, nil
}

// DesktopPlatformAdapter represents desktop-specific adapter
type DesktopPlatformAdapter struct {
	*PlatformAdapter
}

// NewDesktopUIAdapter creates a new desktop UI adapter
func NewDesktopUIAdapter() *DesktopPlatformAdapter {
	return NewDesktopPlatformAdapter()
}

// NewDesktopPlatformAdapter creates a new desktop platform adapter
func NewDesktopPlatformAdapter() *DesktopPlatformAdapter {
	base := NewBasePlatformAdapter("desktop")
	base.features = []string{
		"native_menus",
		"system_tray",
		"file_dialogs",
		"notifications",
		"native_fonts",
		"keyboard_shortcuts",
		"drag_drop",
		"context_menus",
		"system_notifications",
		"auto_update",
		"window_management",
	}
	return &DesktopPlatformAdapter{
		PlatformAdapter: base,
	}
}

// WebPlatformAdapter represents web-specific adapter
type WebPlatformAdapter struct {
	*PlatformAdapter
}

// NewWebUIAdapter creates a new web UI adapter
func NewWebUIAdapter() *WebPlatformAdapter {
	return NewWebPlatformAdapter()
}

// NewWebPlatformAdapter creates a new web platform adapter
func NewWebPlatformAdapter() *WebPlatformAdapter {
	base := NewBasePlatformAdapter("web")
	base.features = []string{
		"responsive_design",
		"pwa",
		"websockets",
		"offline_support",
		"touch_support",
		"browser_storage",
		"push_notifications",
		"service_worker",
		"css_animations",
		"local_storage",
		"service_workers",
		"web_workers",
		"push_api",
		"geolocation",
		"camera_api",
		"microphone_api",
	}
	return &WebPlatformAdapter{
		PlatformAdapter: base,
	}
}

// MobilePlatformAdapter represents mobile-specific adapter
type MobilePlatformAdapter struct {
	*PlatformAdapter
}

// NewMobileUIAdapter creates a new mobile UI adapter
func NewMobileUIAdapter() *MobilePlatformAdapter {
	return NewMobilePlatformAdapter()
}

// NewMobilePlatformAdapter creates a new mobile platform adapter
func NewMobilePlatformAdapter() *MobilePlatformAdapter {
	base := NewBasePlatformAdapter("mobile")
	base.features = []string{
		"touch_gestures",
		"biometric_auth",
		"push_notifications",
		"offline_first",
		"app_lifecycle",
		"camera_access",
		"location_services",
		"device_orientation",
		"native_plugins",
		"gps_location",
		"accelerometer",
		"gyroscope",
		"vibration",
		"offline_mode",
		"app_store_integration",
	}
	// Customize themes for mobile
	base.themes = map[string]PlatformTheme{
		"system": {
			Name:        "System",
			Description: "Matches system appearance",
			Primary:     "",
			Secondary:   "",
			Background:  "",
			Foreground:  "",
		},
		"light": {
			Name:        "Light",
			Description: "Light theme with bright background",
			Primary:     "#007bff",
			Secondary:   "#6c757d",
			Background:  "#ffffff",
			Foreground:  "#000000",
		},
		"dark": {
			Name:        "Dark",
			Description: "Dark theme with dark background",
			Primary:     "#0d6efd",
			Secondary:   "#6c757d",
			Background:  "#1a1a1a",
			Foreground:  "#ffffff",
		},
		"mobile_light": {
			Name:        "Mobile Light",
			Description: "Optimized light theme for mobile",
			Primary:     "#007AFF",
			Secondary:   "#5856D6",
			Background:  "#F2F2F7",
			Foreground:  "#000000",
		},
		"mobile_dark": {
			Name:        "Mobile Dark",
			Description: "Optimized dark theme for mobile",
			Primary:     "#0A84FF",
			Secondary:   "#5E5CE6",
			Background:  "#000000",
			Foreground:  "#FFFFFF",
		},
	}
	base.currentTheme = "system"
	return &MobilePlatformAdapter{
		PlatformAdapter: base,
	}
}

// TUIAdapter represents TUI-specific adapter
type TUIAdapter struct {
	*PlatformAdapter
}

// NewTUIAdapter creates a new TUI adapter
func NewTUIAdapter() *TUIAdapter {
	base := NewBasePlatformAdapter("tui")
	base.features = []string{
		"terminal_colors",
		"keyboard_navigation",
		"mouse_support",
		"terminal_fonts",
		"unicode_support",
		"screen_reader",
		"clipboard_access",
		"resize_support",
		"cursor_styles",
		"terminal_shortcuts",
		"resize_handling",
	}
	// Customize themes for TUI
	base.themes = map[string]PlatformTheme{
		"system": {
			Name:        "System",
			Description: "Matches system appearance",
			Primary:     "#00ff00",
			Secondary:   "#008000",
			Background:  "#000000",
			Foreground:  "#00ff00",
		},
		"light": {
			Name:        "Light",
			Description: "Light theme with bright background",
			Primary:     "#000000",
			Secondary:   "#666666",
			Background:  "#ffffff",
			Foreground:  "#000000",
		},
		"dark": {
			Name:        "Dark",
			Description: "Dark theme with dark background",
			Primary:     "#00ff00",
			Secondary:   "#008000",
			Background:  "#1a1a1a",
			Foreground:  "#00ff00",
		},
		"terminal": {
			Name:        "Terminal",
			Description: "Classic terminal theme",
			Primary:     "#00ff00",
			Secondary:   "#00ff00",
			Background:  "#000000",
			Foreground:  "#00ff00",
		},
	}
	base.currentTheme = "dark"
	return &TUIAdapter{
		PlatformAdapter: base,
	}
}

// GetNativeMenuBarInfo returns native menu bar information
func (a *DesktopPlatformAdapter) GetNativeMenuBarInfo() map[string]interface{} {
	return map[string]interface{}{
		"enabled":          true,
		"menu_shortcuts":   true,
		"global_menu":      true,
		"system_tray":      true,
		"dock_integration": true,
	}
}

// RenderConfigForm renders a configuration form for the desktop platform
func (a *DesktopPlatformAdapter) RenderConfigForm(formType string) interface{} {
	form := DesktopConfigForm{
		ID:            "helix_config_form",
		Title:         tr(context.Background(), "internal_config_ui_form_title", nil),
		Type:          "native_window",
		Layout:        "tabs",
		Modal:         true,
		Resizable:     true,
		MinWidth:      800,
		MinHeight:     600,
		DefaultWidth:  1200,
		DefaultHeight: 800,
		CenterScreen:  true,
		Sections:      []DesktopConfigSection{},
		Actions:       []DesktopConfigAction{},
	}
	return form
}

// GetBrowserInfo returns browser-specific information
func (a *WebPlatformAdapter) GetBrowserInfo() map[string]interface{} {
	return map[string]interface{}{
		"local_storage":   true,
		"service_workers": true,
		"web_workers":     true,
		"notifications":   true,
		"fullscreen":      true,
		"drag_and_drop":   true,
		"file_api":        true,
		"webgl":           true,
		"websocket":       true,
	}
}

// RenderConfigForm renders a configuration form for the web platform
func (a *WebPlatformAdapter) RenderConfigForm(formType string) interface{} {
	ctx := context.Background()
	form := WebConfigForm{
		ID:         "helix_config_form",
		Title:      tr(ctx, "internal_config_ui_form_title", nil),
		Type:       "spa_component",
		Layout:     "responsive_tabs",
		Responsive: true,
		JavaScript: []string{"config.js", "validation.js"},
		CSS:        []string{"config.css", "themes.css"},
		Validation: WebConfigValidation{
			Enabled:  true,
			OnSubmit: true,
			OnBlur:   false,
			Realtime: false,
			Custom:   []string{"emailValidator", "portValidator"},
		},
		SubmitAction: WebConfigSubmit{
			URL:     "/api/config/save",
			Method:  "POST",
			Headers: map[string]string{"Content-Type": "application/json"},
			Success: tr(ctx, "internal_config_ui_save_success", nil),
			Error:   tr(ctx, "internal_config_ui_save_failure", nil),
		},
		Sections: []WebConfigSection{},
		Actions:  []WebConfigAction{},
	}
	return form
}

// GetMobileInfo returns mobile-specific information
func (a *MobilePlatformAdapter) GetMobileInfo() map[string]interface{} {
	return map[string]interface{}{
		"touch":         true,
		"accelerometer": true,
		"geolocation":   true,
		"camera":        true,
		"vibration":     true,
		"offline":       true,
		"app_cache":     true,
		"native_bridge": true,
	}
}

// RenderConfigForm renders a configuration form for the mobile platform
func (a *MobilePlatformAdapter) RenderConfigForm(formType string) interface{} {
	form := MobileConfigForm{
		ID:         "helix_config_form",
		Title:      tr(context.Background(), "internal_config_ui_form_title", nil),
		Type:       "mobile_screens",
		Layout:     "carousel",
		Responsive: true,
		Gestures:   []string{"swipe", "tap", "double_tap", "pinch", "long_press"},
		Sections:   []MobileConfigSection{},
		Actions:    []MobileConfigAction{},
	}
	return form
}

// GetPlatformUIAdapter returns the appropriate UI adapter for the given platform
func GetPlatformUIAdapter(platformType string) PlatformAdapterInterface {
	switch platformType {
	case "desktop":
		return NewDesktopPlatformAdapter()
	case "web":
		return NewWebPlatformAdapter()
	case "mobile":
		return NewMobilePlatformAdapter()
	case "tui":
		return NewTUIAdapter()
	default:
		return NewDesktopPlatformAdapter()
	}
}

// PlatformAdapterInterface defines the interface for platform adapters
type PlatformAdapterInterface interface {
	RenderConfigForm(formType string) interface{}
	GetPlatformType() string
	GetPlatformFeatures() []string
	GetPlatformThemes() map[string]PlatformTheme
}

// Additional helper functions for platform adapters

// SetTheme sets the theme for the platform adapter
func (p *PlatformAdapter) SetTheme(themeName string) {
	if _, exists := p.themes[themeName]; exists {
		p.currentTheme = themeName
	}
}

// GetTheme returns the current theme
func (p *PlatformAdapter) GetTheme() PlatformTheme {
	if p.currentTheme == "" {
		p.currentTheme = "system"
	}
	return p.themes[p.currentTheme]
}

// AddFeature adds a feature to the platform adapter
func (p *PlatformAdapter) AddFeature(feature string) {
	for _, f := range p.features {
		if f == feature {
			return // Feature already exists
		}
	}
	p.features = append(p.features, feature)
}

// HasFeature checks if the platform adapter has a specific feature
func (p *PlatformAdapter) HasFeature(feature string) bool {
	for _, f := range p.features {
		if f == feature {
			return true
		}
	}
	return false
}

// SetCurrentTheme sets the current theme
func (p *PlatformAdapter) SetCurrentTheme(theme string) {
	p.currentTheme = theme
}

// GetCurrentTheme returns the current theme
func (p *PlatformAdapter) GetCurrentTheme() string {
	return p.currentTheme
}
