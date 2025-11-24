package config

// PlatformAdapter represents a platform-specific UI adapter
type PlatformAdapter struct {
	platformType string
	features     []string
	themes       map[string]PlatformTheme
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
			"default_dpi":  96,
			"scale_factors": []float64{1.0, 1.25, 1.5, 2.0},
			"min_width":    800,
			"min_height":   600,
		}
	case "web":
		return map[string]interface{}{
			"default_dpi":  96,
			"scale_factors": []float64{1.0, 1.5, 2.0},
			"min_width":    320,
			"min_height":   568,
		}
	case "mobile":
		return map[string]interface{}{
			"default_dpi":  160,
			"scale_factors": []float64{1.0, 2.0, 3.0},
			"min_width":    320,
			"min_height":   480,
		}
	default:
		return map[string]interface{}{
			"default_dpi":  96,
			"scale_factors": []float64{1.0},
			"min_width":    800,
			"min_height":   600,
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
	ID        string
	Label     string
	Type      string
	Icon      string
	Shortcut  string
	Default   bool
	Cancel    bool
	Position  string
}

// WebConfigForm represents web configuration form
type WebConfigForm struct {
	ID         string
	Title      string
	Type       string
	Layout     string
	Responsive bool
	JavaScript []string
	CSS        []string
	Sections   []WebConfigSection
	Actions    []WebConfigAction
}

// WebConfigSection represents a section in web config form
type WebConfigSection struct {
	ID          string
	Title       string
	Icon        string
	Type        string
	Expanded    bool
	Fields      []WebConfigField
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
	ID          string
	Title       string
	Icon        string
	Type        string
	Expanded    bool
	Fields      []MobileConfigField
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

// TUIAdapter represents terminal UI adapter
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
		"terminal_shortcuts",
		"resize_handling",
	}
	return &TUIAdapter{
		PlatformAdapter: base,
	}
}

// TUIConfigForm represents TUI configuration form
type TUIConfigForm struct {
	ID           string
	Title        string
	Type         string
	Layout       string
	Theme        string
	Features     []string
	KeyBindings  map[string]string
	Sections     []TUIConfigSection
	Actions      []TUIConfigAction
}

// TUIConfigSection represents a section in TUI config form
type TUIConfigSection struct {
	ID          string
	Title       string
	Icon        string
	Type        string
	Expanded    bool
	Fields      []TUIConfigField
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
func (a *TUIAdapter) RenderConfigForm(configUI *ConfigUI) (interface{}, error) {
	form := TUIConfigForm{
		ID:          "helix_config_form",
		Title:       "HelixCode Configuration",
		Type:        "tui_screens",
		Layout:      "menu_driven",
		Theme:       "terminal",
		Features:    []string{"colors", "unicode", "mouse"},
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
				Title:    "Application",
				Icon:     "🚀",
				Type:     "screen",
				Expanded: true,
				Fields: []TUIConfigField{
					{
						ID:          "name",
						Label:       "Application Name",
						Type:        "text",
						Class:       "form-input",
						Value:       configUI.config.Application.Name,
						Required:    true,
						Placeholder: "Enter application name",
						Help:        "The name of HelixCode application",
					},
				},
			},
		},
		Actions: []TUIConfigAction{
			{
				ID:      "save",
				Label:   "Save",
				Type:    "primary",
				Icon:    "💾",
				Default: true,
			},
		},
	}
	
	return form, nil
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

// NewWebUIAdapter creates a new web UI adapter
func NewWebUIAdapter() *WebPlatformAdapter {
	return NewWebPlatformAdapter()
}

// NewWebPlatformAdapter creates a new web platform adapter
func NewWebPlatformAdapter() *WebPlatformAdapter {
	base := NewBasePlatformAdapter("web")
	return &WebPlatformAdapter{
		PlatformAdapter: base,
	}
}

// NewMobileUIAdapter creates a new mobile UI adapter
func NewMobileUIAdapter() *MobilePlatformAdapter {
	return NewMobilePlatformAdapter()
}

// NewMobilePlatformAdapter creates a new mobile platform adapter
func NewMobilePlatformAdapter() *MobilePlatformAdapter {
	base := NewBasePlatformAdapter("mobile")
	return &MobilePlatformAdapter{
		PlatformAdapter: base,
	}
}

// GetNativeMenuBarInfo returns native menu bar information
func (a *DesktopPlatformAdapter) GetNativeMenuBarInfo() map[string]interface{} {
	return map[string]interface{}{
		"enabled":         true,
		"menu_shortcuts":  true,
		"global_menu":     true,
		"system_tray":     true,
		"dock_integration": true,
	}
}

// RenderConfigForm renders configuration form for desktop platform
func (a *DesktopPlatformAdapter) RenderConfigForm(configUI *ConfigUI) (interface{}, error) {
	form := DesktopConfigForm{
		ID:           "helix_config_form",
		Title:        "HelixCode Configuration",
		Type:         "native_window",
		Layout:       "tabs",
		Modal:        true,
		Resizable:    true,
		MinWidth:     800,
		MinHeight:    600,
		DefaultWidth:  1200,
		DefaultHeight: 800,
		CenterScreen:   true,
		Sections: []DesktopConfigSection{
			{
				ID:       "application",
				Title:    "Application",
				Icon:     "🚀",
				Type:     "tab_page",
				Expanded: true,
				Fields: []DesktopConfigField{
					{
						ID:          "name",
						Label:       "Application Name",
						Type:        "text",
						Class:       "form-control",
						Value:       configUI.config.Application.Name,
						Required:    true,
						Keyboard:    "text",
						Placeholder: "Enter application name",
						Help:        "The name of the HelixCode application",
					},
					{
						ID:          "version",
						Label:       "Version",
						Type:        "text",
						Class:       "form-control",
						Value:       configUI.config.Application.Version,
						Required:    false,
						Placeholder: "1.0.0",
						Help:        "The version of the application",
					},
				},
			},
		},
		Actions: []DesktopConfigAction{
			{
				ID:       "save",
				Label:    "Save",
				Type:     "primary",
				Icon:     "💾",
				Shortcut: "Ctrl+S",
				Default:  true,
				Cancel:   false,
				Position: "right",
			},
			{
				ID:       "cancel",
				Label:    "Cancel",
				Type:     "secondary",
				Icon:     "✖",
				Shortcut: "Escape",
				Default:  false,
				Cancel:   true,
				Position: "left",
			},
		},
	}
	
	return form, nil
}

// WebPlatformAdapter represents web-specific adapter
type WebPlatformAdapter struct {
	*PlatformAdapter
}

// GetBrowserInfo returns browser-specific information
func (a *WebPlatformAdapter) GetBrowserInfo() map[string]interface{} {
	return map[string]interface{}{
		"local_storage":     true,
		"service_workers":   true,
		"web_workers":       true,
		"notifications":     true,
		"fullscreen":        true,
		"drag_and_drop":     true,
		"file_api":         true,
		"webgl":            true,
		"websocket":        true,
	}
}

// RenderConfigForm renders configuration form for web platform
func (a *WebPlatformAdapter) RenderConfigForm(configUI *ConfigUI) (interface{}, error) {
	form := WebConfigForm{
		ID:         "helix_config_form",
		Title:      "HelixCode Configuration",
		Type:       "spa_component",
		Layout:     "responsive_tabs",
		Responsive: true,
		JavaScript: []string{"config.js", "validation.js"},
		CSS:        []string{"config.css", "themes.css"},
		Sections: []WebConfigSection{
			{
				ID:       "application",
				Title:    "Application",
				Icon:     "🚀",
				Type:     "tab_page",
				Expanded: true,
				Fields: []WebConfigField{
					{
						ID:          "name",
						Label:       "Application Name",
						Type:        "text",
						Class:       "form-control",
						Value:       configUI.config.Application.Name,
						Required:    true,
						Keyboard:    "text",
						Placeholder: "Enter application name",
						Help:        "The name of the HelixCode application",
					},
					{
						ID:          "version",
						Label:       "Version",
						Type:        "text",
						Class:       "form-control",
						Value:       configUI.config.Application.Version,
						Required:    false,
						Placeholder: "1.0.0",
						Help:        "The version of the application",
					},
				},
			},
		},
		Actions: []WebConfigAction{
			{
				ID:      "save",
				Label:   "Save",
				Type:    "primary",
				Icon:    "💾",
				Default: true,
			},
		},
	}
	
	return form, nil
}

// MobilePlatformAdapter represents mobile-specific adapter
type MobilePlatformAdapter struct {
	*PlatformAdapter
}

// GetMobileInfo returns mobile-specific information
func (a *MobilePlatformAdapter) GetMobileInfo() map[string]interface{} {
	return map[string]interface{}{
		"touch":            true,
		"accelerometer":     true,
		"geolocation":      true,
		"camera":           true,
		"vibration":        true,
		"offline":          true,
		"app_cache":        true,
		"native_bridge":    true,
	}
}

// RenderConfigForm renders configuration form for mobile platform
func (a *MobilePlatformAdapter) RenderConfigForm(configUI *ConfigUI) (interface{}, error) {
	form := MobileConfigForm{
		ID:         "helix_config_form",
		Title:      "HelixCode Configuration",
		Type:       "mobile_screens",
		Layout:     "carousel",
		Responsive: true,
		Gestures:   []string{"swipe", "tap", "double_tap", "pinch", "long_press"},
		Sections: []MobileConfigSection{
			{
				ID:       "application",
				Title:    "Application",
				Icon:     "🚀",
				Type:     "screen",
				Expanded: true,
				Fields: []MobileConfigField{
					{
						ID:          "name",
						Label:       "Application Name",
						Type:        "text",
						Class:       "mobile-input",
						Value:       configUI.config.Application.Name,
						Required:    true,
						Placeholder: "Enter application name",
						Help:        "The name of HelixCode application",
					},
					{
						ID:          "version",
						Label:       "Version",
						Type:        "text",
						Class:       "mobile-input",
						Value:       configUI.config.Application.Version,
						Required:    false,
						Placeholder: "1.0.0",
						Help:        "The version of the application",
					},
				},
			},
		},
		Actions: []MobileConfigAction{
			{
				ID:      "save",
				Label:   "Save",
				Type:    "primary",
				Icon:    "💾",
				Default: true,
			},
		},
	}
	
	return form, nil
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

// RenderConfigForm renders a configuration form for the desktop platform
func (a *DesktopPlatformAdapter) RenderConfigForm(formType string) interface{} {
	return DesktopConfigForm{
		ID:            formType,
		Title:         "Desktop Configuration",
		Type:          "desktop",
		Layout:        "tabbed",
		Modal:         true,
		Resizable:     true,
		MinWidth:      600,
		MinHeight:     400,
		DefaultWidth:  800,
		DefaultHeight: 600,
		CenterScreen:  true,
		Sections:      []DesktopConfigSection{},
		Actions:       []DesktopConfigAction{},
	}
}

// RenderConfigForm renders a configuration form for the web platform
func (a *WebPlatformAdapter) RenderConfigForm(formType string) interface{} {
	return WebConfigForm{
		ID:         formType,
		Title:      "Web Configuration",
		Type:       "web",
		Layout:     "responsive",
		Responsive: true,
		JavaScript: []string{"jquery.min.js", "config.min.js"},
		CSS:        []string{"config.min.css"},
		Sections:   []WebConfigSection{},
		Actions:    []WebConfigAction{},
	}
}

// RenderConfigForm renders a configuration form for the mobile platform
func (a *MobilePlatformAdapter) RenderConfigForm(formType string) interface{} {
	return MobileConfigForm{
		ID:         formType,
		Title:      "Mobile Configuration",
		Type:       "mobile",
		Layout:     "scrollable",
		Responsive: true,
		Gestures:   []string{"tap", "swipe", "pinch"},
		Sections:   []MobileConfigSection{},
		Actions:    []MobileConfigAction{},
	}
}

// RenderConfigForm renders a configuration form for the TUI platform
func (a *TUIAdapter) RenderConfigForm(formType string) interface{} {
	return TUIConfigForm{
		ID:          formType,
		Title:       "TUI Configuration",
		Type:        "tui",
		Layout:      "vertical",
		BorderStyle: "single",
		KeyBindings: map[string]string{
			"save":     "Ctrl+S",
			"cancel":   "Esc",
			"next":     "Tab",
			"prev":     "Shift+Tab",
			"help":     "F1",
			"submit":   "Enter",
			"quit":     "Ctrl+Q",
		},
		Sections: []TUIConfigSection{},
		Actions:  []TUIConfigAction{},
	}
}

// GetPlatformType returns the platform type for desktop
func (a *DesktopPlatformAdapter) GetPlatformType() string {
	return "desktop"
}

// GetPlatformType returns the platform type for web
func (a *WebPlatformAdapter) GetPlatformType() string {
	return "web"
}

// GetPlatformType returns the platform type for mobile
func (a *MobilePlatformAdapter) GetPlatformType() string {
	return "mobile"
}

// GetPlatformType returns the platform type for TUI
func (a *TUIAdapter) GetPlatformType() string {
	return "tui"
}

// PlatformAdapterInterface defines the interface for platform adapters
type PlatformAdapterInterface interface {
	RenderConfigForm(formType string) interface{}
	GetPlatformType() string
}
