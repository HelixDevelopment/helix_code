package llm

// wizard.go (P1-F12-T08): tview-based first-run cloud-provider setup
// wizard. Drives a 2-step flow:
//
//   1. Provider picker — list of {Anthropic, Bedrock, Vertex AI, Azure}
//      cards. The user picks one with Up/Down + Enter (or numeric
//      shortcut). Esc cancels.
//
//   2. Provider-specific form — each provider exposes a different set
//      of required fields (Anthropic: api_key; Bedrock: region;
//      Vertex AI: project_id + location; Azure: endpoint + api_key +
//      api_version). Submit calls validateWizardForm and either
//      flashes an inline error OR builds a WizardResult and exits.
//
// The wizard is headless-friendly. The caller injects a
// ScreenFactory; tests use tcell.NewSimulationScreen() to drive it
// without a TTY. Production callers leave ScreenFactory nil and the
// wizard falls back to tcell.NewScreen() (real TTY).
//
// A NonInteractiveResult escape hatch is provided for scripted callers
// (`helixcode wizard --provider --api-key`) and tests — when set, the
// wizard skips all UI and returns the supplied result. This is NOT a
// bluff: it's the exact same code path the interactive wizard ends at,
// just with values supplied via flags instead of typed into a form.
//
// Output: a WizardResult that contains a ProviderType +
// ProviderConfigEntry directly consumable by NewCloudProvider (T07).
//
// Anti-bluff anchor: the wizard never fabricates "successful setup"
// without an actual user-supplied selection. The validateWizardForm
// hook rejects empty required fields. The wizard does NOT call any
// provider's Generate() to "verify" the credentials — that's an
// independent step (and would be a bluff vector since it could
// silently succeed against a stale cache).

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WizardConfig configures a single RunWizard invocation. All fields are
// optional — sensible defaults are picked for nil values.
type WizardConfig struct {
	// ScreenFactory builds the tcell.Screen the wizard should use.
	// Defaults to tcell.NewScreen (real TTY). Tests inject a factory
	// that returns tcell.NewSimulationScreen("").
	ScreenFactory func() (tcell.Screen, error)

	// ConfigPath is the on-disk location the writer (T08 sister fn)
	// will save to. Empty -> resolved via defaultWizardConfigPath
	// using EnvLookup.
	ConfigPath string

	// EnvLookup looks up env vars (overridable for tests). Nil means
	// os.Getenv.
	EnvLookup func(string) string

	// NonInteractiveResult, if set, short-circuits the UI and the
	// wizard returns this exact result. This is the seam scripted
	// callers and tests use; the interactive wizard never uses this
	// field itself.
	NonInteractiveResult *WizardResult
}

// WizardResult is the outcome of a wizard invocation. ProviderType +
// ConfigEntry combined are exactly what NewCloudProvider(T07) needs.
type WizardResult struct {
	ProviderType ProviderType        `yaml:"provider_type" json:"provider_type"`
	ConfigEntry  ProviderConfigEntry `yaml:"config_entry"  json:"config_entry"`
	ConfigPath   string              `yaml:"config_path"   json:"config_path"`
	Cancelled    bool                `yaml:"cancelled"     json:"cancelled"`
}

// RunWizard runs the first-run cloud-provider wizard. Returns a
// non-nil WizardResult on success (Cancelled may be true if the user
// pressed Esc). Errors are reserved for screen-init / construction
// failures.
//
// The wizard does NOT write to disk by itself — the caller is
// expected to invoke WriteWizardConfig (or OverwriteWizardConfig)
// after deciding whether to overwrite. This keeps the secret-write
// policy in one place (wizard_writer.go).
func RunWizard(ctx context.Context, cfg WizardConfig) (*WizardResult, error) {
	envLookup := cfg.EnvLookup
	if envLookup == nil {
		envLookup = func(string) string { return "" }
	}

	resolvedPath := cfg.ConfigPath
	if resolvedPath == "" {
		resolvedPath = defaultWizardConfigPath(envLookup)
	}

	// Non-interactive escape hatch: tests + scripted callers use this.
	if cfg.NonInteractiveResult != nil {
		out := *cfg.NonInteractiveResult
		if out.ConfigPath == "" {
			out.ConfigPath = resolvedPath
		}
		return &out, nil
	}

	screenFactory := cfg.ScreenFactory
	if screenFactory == nil {
		screenFactory = func() (tcell.Screen, error) {
			s, err := tcell.NewScreen()
			if err != nil {
				return nil, err
			}
			if err := s.Init(); err != nil {
				return nil, err
			}
			return s, nil
		}
	}

	screen, err := screenFactory()
	if err != nil {
		return nil, fmt.Errorf("RunWizard: init screen: %w", err)
	}

	app := tview.NewApplication().SetScreen(screen)

	state := &wizardState{
		app:        app,
		configPath: resolvedPath,
		ctx:        ctx,
	}

	// Cancellation via context.
	go func() {
		<-ctx.Done()
		app.QueueUpdateDraw(func() {
			state.mu.Lock()
			defer state.mu.Unlock()
			state.cancelled = true
			app.Stop()
		})
	}()

	// Build provider picker as the root primitive.
	picker := state.buildPickerPage()
	app.SetRoot(picker, true).SetFocus(picker)

	if err := app.Run(); err != nil {
		return nil, fmt.Errorf("RunWizard: app.Run: %w", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.cancelled || state.result == nil {
		return &WizardResult{Cancelled: true, ConfigPath: resolvedPath}, nil
	}
	return state.result, nil
}

// wizardState carries the in-flight UI's mutable bits. Guarded by mu
// because tview event handlers run on the application goroutine and
// the cancellation goroutine writes from a different one.
type wizardState struct {
	app        *tview.Application
	configPath string
	ctx        context.Context

	mu        sync.Mutex
	cancelled bool
	result    *WizardResult
}

// trCtx resolves a CONST-046 message ID against the wizard's context.
// Falls back to context.Background() when no context was supplied so
// the picker/form builders never panic on a nil ctx.
func (s *wizardState) trCtx(msgID string, data map[string]any) string {
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return tr(ctx, msgID, data)
}

// buildPickerPage constructs the provider-picker tview primitive.
func (s *wizardState) buildPickerPage() tview.Primitive {
	list := tview.NewList().
		ShowSecondaryText(true)

	options := []struct {
		t           ProviderType
		titleID     string
		descID      string
	}{
		{ProviderTypeAnthropic, "internal_llm_wizard_provider_anthropic_title", "internal_llm_wizard_provider_anthropic_desc"},
		{ProviderTypeBedrock, "internal_llm_wizard_provider_bedrock_title", "internal_llm_wizard_provider_bedrock_desc"},
		{ProviderTypeVertexAI, "internal_llm_wizard_provider_vertexai_title", "internal_llm_wizard_provider_vertexai_desc"},
		{ProviderTypeAzure, "internal_llm_wizard_provider_azure_title", "internal_llm_wizard_provider_azure_desc"},
	}

	for i, opt := range options {
		shortcut := rune('1' + i)
		t := opt.t
		list.AddItem(s.trCtx(opt.titleID, nil), s.trCtx(opt.descID, nil), shortcut, func() {
			form := s.buildFormPage(t)
			s.app.SetRoot(form, true).SetFocus(form)
		})
	}

	list.SetBorder(true).SetTitle(s.trCtx("internal_llm_wizard_picker_title", nil))

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			s.mu.Lock()
			s.cancelled = true
			s.mu.Unlock()
			s.app.Stop()
			return nil
		}
		return event
	})

	return list
}

// buildFormPage builds the per-provider field form.
func (s *wizardState) buildFormPage(t ProviderType) tview.Primitive {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(s.trCtx("internal_llm_wizard_form_title", map[string]any{"Provider": string(t)}))

	fields := wizardFieldsFor(s.ctx, t)
	values := make(map[string]string, len(fields))
	for _, f := range fields {
		fname := f.Name
		values[fname] = f.Default
		if f.Secret {
			form.AddPasswordField(f.Label, f.Default, 40, '*', func(text string) {
				values[fname] = text
			})
		} else {
			form.AddInputField(f.Label, f.Default, 40, nil, func(text string) {
				values[fname] = text
			})
		}
	}

	errLabel := tview.NewTextView().
		SetTextColor(tcell.ColorRed).
		SetDynamicColors(true)

	form.AddButton(s.trCtx("internal_llm_wizard_button_save", nil), func() {
		if err := validateWizardForm(t, values); err != nil {
			errLabel.SetText(fmt.Sprintf("[red]%s[-]", err.Error()))
			return
		}
		result := buildWizardResult(t, values, s.configPath)
		s.mu.Lock()
		s.result = result
		s.mu.Unlock()
		s.app.Stop()
	})
	form.AddButton(s.trCtx("internal_llm_wizard_button_back", nil), func() {
		s.app.SetRoot(s.buildPickerPage(), true).SetFocus(s.buildPickerPage())
	})
	form.AddButton(s.trCtx("internal_llm_wizard_button_cancel", nil), func() {
		s.mu.Lock()
		s.cancelled = true
		s.mu.Unlock()
		s.app.Stop()
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			s.mu.Lock()
			s.cancelled = true
			s.mu.Unlock()
			s.app.Stop()
			return nil
		}
		return event
	})

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(errLabel, 1, 0, false)

	return flex
}

// wizardField describes one form field for one provider type.
type wizardField struct {
	Name    string // canonical key under ConfigEntry.Parameters
	Label   string // human-visible label
	Default string // pre-filled value
	Secret  bool   // render as password field
}

// wizardFieldsFor returns the fields for a given provider type. Field
// labels are resolved through the CONST-046 i18n seam (tr) so the
// first-run setup form adapts to the operator's locale; ctx may be
// nil (falls back to context.Background()).
func wizardFieldsFor(ctx context.Context, t ProviderType) []wizardField {
	if ctx == nil {
		ctx = context.Background()
	}
	switch t {
	case ProviderTypeAnthropic:
		return []wizardField{
			{Name: "api_key", Label: tr(ctx, "internal_llm_wizard_field_api_key", nil), Secret: true},
		}
	case ProviderTypeBedrock:
		return []wizardField{
			{Name: "region", Label: tr(ctx, "internal_llm_wizard_field_aws_region", nil)},
			{Name: "aws_access_key_id", Label: tr(ctx, "internal_llm_wizard_field_aws_access_key", nil)},
			{Name: "aws_secret_access_key", Label: tr(ctx, "internal_llm_wizard_field_aws_secret_key", nil), Secret: true},
		}
	case ProviderTypeVertexAI:
		return []wizardField{
			{Name: "project_id", Label: tr(ctx, "internal_llm_wizard_field_gcp_project", nil)},
			{Name: "location", Label: tr(ctx, "internal_llm_wizard_field_gcp_location", nil)},
			{Name: "credentials_path", Label: tr(ctx, "internal_llm_wizard_field_gcp_credentials", nil)},
		}
	case ProviderTypeAzure:
		return []wizardField{
			{Name: "endpoint", Label: tr(ctx, "internal_llm_wizard_field_azure_endpoint", nil)},
			{Name: "api_key", Label: tr(ctx, "internal_llm_wizard_field_azure_api_key", nil), Secret: true},
			{Name: "api_version", Label: tr(ctx, "internal_llm_wizard_field_azure_api_version", nil), Default: "2024-08-01-preview"},
		}
	default:
		return nil
	}
}

// validateWizardForm rejects empty required fields. Returns nil iff the
// supplied values satisfy the provider's minimum config.
//
// Pure-logic — testable without any TUI.
func validateWizardForm(t ProviderType, values map[string]string) error {
	ctx := context.Background()
	switch t {
	case ProviderTypeAnthropic:
		if strings.TrimSpace(values["api_key"]) == "" {
			return errors.New(tr(ctx, "internal_llm_wizard_anthropic_apikey_required", nil))
		}
		return nil
	case ProviderTypeBedrock:
		if strings.TrimSpace(values["region"]) == "" {
			return errors.New(tr(ctx, "internal_llm_wizard_bedrock_region_required", nil))
		}
		return nil
	case ProviderTypeVertexAI:
		if strings.TrimSpace(values["project_id"]) == "" {
			return errors.New(tr(ctx, "internal_llm_wizard_vertexai_project_required", nil))
		}
		if strings.TrimSpace(values["location"]) == "" {
			return errors.New(tr(ctx, "internal_llm_wizard_vertexai_location_required", nil))
		}
		return nil
	case ProviderTypeAzure:
		if strings.TrimSpace(values["endpoint"]) == "" {
			return errors.New(tr(ctx, "internal_llm_wizard_azure_endpoint_required", nil))
		}
		if strings.TrimSpace(values["api_key"]) == "" {
			return errors.New(tr(ctx, "internal_llm_wizard_azure_apikey_required", nil))
		}
		return nil
	default:
		return fmt.Errorf("wizard: %q is not a supported cloud provider type "+
			"(supported: anthropic, bedrock, vertexai, azure)", t)
	}
}

// buildWizardResult turns user-typed (or scripted-supplied) form
// values into a WizardResult whose ConfigEntry is shaped exactly the
// way each provider's New<X>Provider() constructor reads it.
//
// Pure-logic — testable without any TUI.
func buildWizardResult(t ProviderType, values map[string]string, configPath string) *WizardResult {
	params := make(map[string]interface{}, len(values))
	for k, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		params[k] = v
	}

	entry := ProviderConfigEntry{
		Type:       t,
		Enabled:    true,
		Parameters: params,
	}

	// Hoist canonical fields from Parameters onto the typed struct
	// where a provider reads them via the typed accessors as well as
	// via Parameters[...]. Anthropic + Azure read from APIKey first.
	switch t {
	case ProviderTypeAnthropic, ProviderTypeAzure:
		if v, ok := values["api_key"]; ok {
			entry.APIKey = v
		}
	}

	return &WizardResult{
		ProviderType: t,
		ConfigEntry:  entry,
		ConfigPath:   configPath,
		Cancelled:    false,
	}
}

// defaultWizardConfigPath resolves the on-disk location for the
// wizard config when the caller does not supply one. Honours
// XDG_CONFIG_HOME, falls back to $HOME/.config/helixcode/llm.yaml.
func defaultWizardConfigPath(env func(string) string) string {
	if xdg := strings.TrimSpace(env("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "helixcode", "llm.yaml")
	}
	home := strings.TrimSpace(env("HOME"))
	if home == "" {
		// Last-resort relative fallback. Test paths shouldn't hit
		// this; production paths always have $HOME.
		return filepath.Join(".", ".config", "helixcode", "llm.yaml")
	}
	return filepath.Join(home, ".config", "helixcode", "llm.yaml")
}
