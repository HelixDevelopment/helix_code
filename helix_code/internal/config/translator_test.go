// Unit tests for the internal/config package-level translator + tr()
// helper (CONST-046 round-150 §11.4 anti-bluff sweep, 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package config

import (
	"context"
	"errors"
	"strings"
	"testing"

	configi18n "dev.helix.code/internal/config/i18n"
	"dev.helix.code/internal/database"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

// TestTr_DefaultsToRealBundleTranslator — §11.4.120 reconcile (HXC-097,
// 2026-06-15). Previously asserted the Noop key-echo default
// (raw "internal_config_validate_version_required"). The package default
// is now the real embedded-bundle translator installed at init(), so the
// default-path resolves to the bundle PROSE. This is the regression guard
// for HXC-097: if the default ever reverts to NoopTranslator{} (or the
// embed fails to load), this test FAILs because it sees the raw key.
func TestTr_DefaultsToRealBundleTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(context.Background(), "internal_config_validate_version_required", nil)
	if got != "version is required" {
		t.Fatalf("tr default = %q, want resolved bundle prose %q "+
			"(HXC-097: default must be the real embedded-bundle translator, not Noop)",
			got, "version is required")
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_config_validate_jwt_secret_must_be_set", nil)
	if got != "<TR:internal_config_validate_jwt_secret_must_be_set>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade to
	// the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_config_validate_database_host_required", nil)
	if got != "internal_config_validate_database_host_required" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

// TestSetTranslator_NilRestoresDefault — §11.4.120 reconcile (HXC-097,
// 2026-06-15). SetTranslator(nil) now restores the package DEFAULT (the
// real embedded-bundle translator), not NoopTranslator{}. So after wiring
// a sentinel and then nil-resetting, tr() resolves to bundle prose — never
// reverting a correctly-loaded binary to raw-key echo.
func TestSetTranslator_NilRestoresDefault(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // restore default
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_config_warn_no_config_file_using_defaults", nil)
	want := "WARN: No config file found, using defaults and environment variables"
	if got != want {
		t.Fatalf("tr after nil-reset = %q, want resolved bundle prose %q (default restored)", got, want)
	}
}

// validateConfigCase represents one migrated validateConfig() branch.
// Test cases construct a minimally valid Config then mutate a single
// field so only the targeted branch fires — proving the migrated
// literal flows through tr() and not a stray hardcoded fallback.
type validateConfigCase struct {
	name        string
	mutate      func(*Config)
	wantSentID  string
	wantRawText string
}

// minimallyValidConfig builds a Config that satisfies every
// validateConfig() branch so a single-field mutation triggers exactly
// one error — isolating the call-site we want to assert against.
func minimallyValidConfig() *Config {
	return &Config{
		Version:     "1.0.0",
		Application: ApplicationConfig{Name: "helix"},
		Server:      ServerConfig{Port: 8080},
		Database:    database.Config{Host: "localhost", DBName: "helix"},
		Auth:        AuthConfig{JWTSecret: "real-secret-not-default", BcryptCost: 12},
		Workers:     WorkersConfig{HealthCheckInterval: 30, MaxConcurrentTasks: 10},
		Tasks:       TasksConfig{MaxRetries: 3},
		LLM:         LLMConfig{MaxTokens: 1000, Temperature: 0.7},
	}
}

// TestValidateConfig_AllMigratedLiterals_GoThroughTranslator is the
// call-site paired-mutation: with a sentinel translator wired, every
// migrated fmt.Errorf path MUST surface the sentinel-wrapped message
// ID — proving the literal was NOT hardcoded anywhere on the path.
// If a future refactor inlines any string, the matching case fails.
func TestValidateConfig_AllMigratedLiterals_GoThroughTranslator(t *testing.T) {
	cases := []validateConfigCase{
		{
			name:       "version_required",
			mutate:     func(c *Config) { c.Version = "" },
			wantSentID: "<TR:internal_config_validate_version_required>",
		},
		{
			name:       "application_name_required",
			mutate:     func(c *Config) { c.Application.Name = "" },
			wantSentID: "<TR:internal_config_validate_application_name_required>",
		},
		{
			name:       "server_port_out_of_range",
			mutate:     func(c *Config) { c.Server.Port = 0 },
			wantSentID: "<TR:internal_config_validate_server_port_out_of_range>",
		},
		{
			name:       "database_host_required",
			mutate:     func(c *Config) { c.Database.Host = "" },
			wantSentID: "<TR:internal_config_validate_database_host_required>",
		},
		{
			name:       "database_name_required",
			mutate:     func(c *Config) { c.Database.DBName = "" },
			wantSentID: "<TR:internal_config_validate_database_name_required>",
		},
		{
			name:       "jwt_secret_must_be_set",
			mutate:     func(c *Config) { c.Auth.JWTSecret = "" },
			wantSentID: "<TR:internal_config_validate_jwt_secret_must_be_set>",
		},
		{
			name:       "jwt_secret_default_rejected",
			mutate:     func(c *Config) { c.Auth.JWTSecret = "default-secret-change-in-production" },
			wantSentID: "<TR:internal_config_validate_jwt_secret_must_be_set>",
		},
		{
			name:       "health_check_interval_positive",
			mutate:     func(c *Config) { c.Workers.HealthCheckInterval = 0 },
			wantSentID: "<TR:internal_config_validate_health_check_interval_positive>",
		},
		{
			name:       "max_concurrent_tasks_positive",
			mutate:     func(c *Config) { c.Workers.MaxConcurrentTasks = 0 },
			wantSentID: "<TR:internal_config_validate_max_concurrent_tasks_positive>",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetTranslator(t)
			SetTranslator(sentinelTranslator{})
			defer resetTranslator(t)

			cfg := minimallyValidConfig()
			tc.mutate(cfg)
			err := validateConfig(cfg)
			if err == nil {
				t.Fatalf("validateConfig(%s) returned no error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSentID) {
				t.Fatalf("validateConfig(%s) error = %q, want %q — call site bypassed tr()",
					tc.name, err.Error(), tc.wantSentID)
			}
		})
	}
}

// TestValidateConfig_ResolvedProseByDefault — §11.4.120 reconcile
// (HXC-097, 2026-06-15). Previously asserted the raw message ID (Noop
// echo). With the real embedded-bundle translator as the package default,
// validateConfig now surfaces the resolved bundle PROSE — the actual text
// an end user sees. This is an HXC-097 regression guard at the
// validateConfig call site: a revert to Noop would re-surface the raw key
// and FAIL here.
func TestValidateConfig_ResolvedProseByDefault(t *testing.T) {
	resetTranslator(t)

	cfg := minimallyValidConfig()
	cfg.Version = ""
	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("validateConfig(version=\"\") returned no error")
	}
	if !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("validateConfig error = %q, want resolved bundle prose %q (real-bundle default)",
			err.Error(), "version is required")
	}
	if strings.Contains(err.Error(), "internal_config_validate_version_required") {
		t.Fatalf("validateConfig error = %q still contains the raw message-ID key "+
			"(HXC-097 regression: package fell back to NoopTranslator)", err.Error())
	}
}

// TestSetTranslator_AcceptsNoopExplicit confirms the public API
// allows an explicit NoopTranslator (used by tests + ad-hoc tools)
// without unexpected behaviour.
func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(configi18n.NoopTranslator{})
	got := tr(context.Background(), "internal_config_info_using_config_file", nil)
	if got != "internal_config_info_using_config_file" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// --- Round-444 §11.4 paired-mutation: platform_ui_adapters.go +
// config_api.go genuine-UI literals (CONST-046 genuine-UI residual
// round-23). With a sentinel translator wired, every migrated call
// site MUST emit the sentinel-wrapped message ID; with the default
// Noop translator it MUST emit the raw message ID. A regression that
// reintroduces a hardcoded literal fails both halves.

// renderFormTitle extracts the Title field from a RenderConfigForm
// result regardless of which concrete *ConfigForm type was returned.
func renderFormTitle(t *testing.T, form interface{}) string {
	t.Helper()
	switch f := form.(type) {
	case TUIConfigForm:
		return f.Title
	case DesktopConfigForm:
		return f.Title
	case WebConfigForm:
		return f.Title
	case MobileConfigForm:
		return f.Title
	default:
		t.Fatalf("renderFormTitle: unexpected form type %T", form)
		return ""
	}
}

func TestRenderConfigForm_Round444_TitleGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	want := "<TR:internal_config_ui_form_title>"
	adapters := map[string]PlatformAdapterInterface{
		"tui":     NewTUIAdapter(),
		"desktop": NewDesktopPlatformAdapter(),
		"web":     NewWebPlatformAdapter(),
		"mobile":  NewMobilePlatformAdapter(),
	}
	for name, a := range adapters {
		got := renderFormTitle(t, a.RenderConfigForm(""))
		if got != want {
			t.Fatalf("%s RenderConfigForm Title = %q, want %q — call site bypassed translator", name, got, want)
		}
	}
}

// TestRenderConfigForm_Round444_ResolvedTitleByDefault — §11.4.120
// reconcile (HXC-097, 2026-06-15). Default path now resolves the form
// title to bundle prose, not the raw key.
func TestRenderConfigForm_Round444_ResolvedTitleByDefault(t *testing.T) {
	resetTranslator(t)

	got := renderFormTitle(t, NewWebPlatformAdapter().RenderConfigForm(""))
	if got != "HelixCode Configuration" {
		t.Fatalf("web RenderConfigForm Title = %q, want resolved bundle prose %q (real-bundle default)",
			got, "HelixCode Configuration")
	}
}

func TestRenderConfigForm_Round444_TUIFieldsGoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	form, ok := NewTUIAdapter().RenderConfigForm("").(TUIConfigForm)
	if !ok {
		t.Fatalf("TUI RenderConfigForm did not return TUIConfigForm")
	}
	if len(form.Sections) == 0 {
		t.Fatal("TUI form has no sections")
	}
	sec := form.Sections[0]
	if sec.Title != "<TR:internal_config_ui_section_application>" {
		t.Fatalf("section Title = %q, want sentinel-wrapped ID", sec.Title)
	}
	wantLabels := map[string]string{
		"<TR:internal_config_ui_field_app_name_label>":    "<TR:internal_config_ui_field_app_name_help>",
		"<TR:internal_config_ui_field_app_version_label>": "<TR:internal_config_ui_field_app_version_help>",
	}
	got := map[string]string{}
	for _, fld := range sec.Fields {
		got[fld.Label] = fld.HelpText
	}
	for label, help := range wantLabels {
		if got[label] != help {
			t.Fatalf("field %q HelpText = %q, want %q — call site bypassed translator", label, got[label], help)
		}
	}
}

func TestRenderConfigForm_Round444_WebSaveMessagesGoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	form, ok := NewWebPlatformAdapter().RenderConfigForm("").(WebConfigForm)
	if !ok {
		t.Fatalf("web RenderConfigForm did not return WebConfigForm")
	}
	if form.SubmitAction.Success != "<TR:internal_config_ui_save_success>" {
		t.Fatalf("save Success = %q, want sentinel-wrapped ID", form.SubmitAction.Success)
	}
	if form.SubmitAction.Error != "<TR:internal_config_ui_save_failure>" {
		t.Fatalf("save Error = %q, want sentinel-wrapped ID", form.SubmitAction.Error)
	}
}

// TestRenderConfigForm_Round444_WebSaveMessagesResolvedByDefault —
// §11.4.120 reconcile (HXC-097, 2026-06-15). Default path now resolves
// the web save-success banner to bundle prose, not the raw key.
func TestRenderConfigForm_Round444_WebSaveMessagesResolvedByDefault(t *testing.T) {
	resetTranslator(t)

	form, ok := NewWebPlatformAdapter().RenderConfigForm("").(WebConfigForm)
	if !ok {
		t.Fatalf("web RenderConfigForm did not return WebConfigForm")
	}
	if form.SubmitAction.Success != "Configuration saved successfully!" {
		t.Fatalf("save Success = %q, want resolved bundle prose %q (real-bundle default)",
			form.SubmitAction.Success, "Configuration saved successfully!")
	}
}

// TestConfigAPI_Round444_ErrorMessageBundleKeys confirms the
// config_api.go HTTP-error message IDs are wired into the active
// bundle — paired-mutation: tr() of each migrated ID returns the
// sentinel wrapper, never a hardcoded English literal.
func TestConfigAPI_Round444_ErrorMessageBundleKeys(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	ids := []string{
		"internal_config_api_update_failed",
		"internal_config_api_reload_failed",
		"internal_config_api_reset_failed",
		"internal_config_api_restore_failed",
		"internal_config_api_invalid_restore_request",
	}
	for _, id := range ids {
		got := tr(context.Background(), id, map[string]any{"Error": "boom"})
		if got != "<TR:"+id+">" {
			t.Fatalf("tr(%q) = %q, want sentinel-wrapped ID", id, got)
		}
	}
}
