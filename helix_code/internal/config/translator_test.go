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

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(context.Background(), "internal_config_validate_version_required", nil)
	if got != "internal_config_validate_version_required" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
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

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_config_warn_no_config_file_using_defaults", nil)
	if got != "internal_config_warn_no_config_file_using_defaults" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
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
		Auth:        AuthConfig{JWTSecret: "real-secret-not-default"},
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

// TestValidateConfig_RawTextEmittedByDefault asserts that with no
// translator wired (NoopTranslator), validateConfig emits the bundle
// message ID — confirming the migration didn't accidentally pass an
// empty string or a different literal.
func TestValidateConfig_RawTextEmittedByDefault(t *testing.T) {
	resetTranslator(t)

	cfg := minimallyValidConfig()
	cfg.Version = ""
	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("validateConfig(version=\"\") returned no error")
	}
	if !strings.Contains(err.Error(), "internal_config_validate_version_required") {
		t.Fatalf("validateConfig error = %q, want raw message ID (Noop echo)", err.Error())
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
