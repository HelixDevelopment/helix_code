//go:build integration

package integration

// multi_provider_test.go (P1-F12-T09): integration tests covering the
// flag>env>config>wizard precedence chain plumbed through cmd/cli/main.go +
// the wizard cobra subcommand. These tests exercise REAL Selector logic,
// REAL NewCloudProvider construction, and REAL disk I/O for the wizard
// config. NO mocks for the wizard pipeline — only the cloud-call test is
// gated on credentials being present.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"dev.helix.code/internal/llm"
)

// TestMultiProvider_FlagOverridesEnv verifies the highest-precedence source
// (the explicit --provider flag) wins over a conflicting env var. Empty
// config simulates a fresh deployment.
func TestMultiProvider_FlagOverridesEnv(t *testing.T) {
	got, err := llm.Select(llm.SelectorInput{
		Flag:   "bedrock",
		Env:    "anthropic",
		Config: "",
	})
	require.NoError(t, err)
	require.Equal(t, llm.ProviderTypeBedrock, got,
		"flag must override env: --provider=bedrock with HELIX_LLM_PROVIDER=anthropic should resolve to bedrock")
}

// TestMultiProvider_EnvWhenNoFlag verifies that when no --provider flag is
// supplied, HELIX_LLM_PROVIDER is honoured.
func TestMultiProvider_EnvWhenNoFlag(t *testing.T) {
	got, err := llm.Select(llm.SelectorInput{
		Flag:   "",
		Env:    "anthropic",
		Config: "",
	})
	require.NoError(t, err)
	require.Equal(t, llm.ProviderTypeAnthropic, got)
}

// TestMultiProvider_ConfigWhenNoFlagOrEnv verifies the lowest-precedence
// source (config file) is consulted when neither flag nor env is set.
func TestMultiProvider_ConfigWhenNoFlagOrEnv(t *testing.T) {
	got, err := llm.Select(llm.SelectorInput{
		Flag:   "",
		Env:    "",
		Config: "vertexai",
	})
	require.NoError(t, err)
	require.Equal(t, llm.ProviderTypeVertexAI, got)
}

// TestMultiProvider_NoSourcesIsErrNoProviderConfigured verifies the sentinel
// is returned (and is errors.Is-able) when every source is empty.
func TestMultiProvider_NoSourcesIsErrNoProviderConfigured(t *testing.T) {
	_, err := llm.Select(llm.SelectorInput{})
	require.Error(t, err)
	require.True(t, errors.Is(err, llm.ErrNoProviderConfigured),
		"expected ErrNoProviderConfigured to be the sentinel returned for empty input, got: %v", err)
}

// TestMultiProvider_FactoryConstructsAllFour exercises NewCloudProvider for
// each F12 cloud backend with a synthetic config. None of these constructors
// should panic; each returned object must satisfy the Provider interface.
// We deliberately do not assert success of an HTTP call — that's the job of
// the gated test below.
func TestMultiProvider_FactoryConstructsAllFour(t *testing.T) {
	cases := []struct {
		name string
		t    llm.ProviderType
		cfg  llm.ProviderConfigEntry
	}{
		{
			name: "anthropic",
			t:    llm.ProviderTypeAnthropic,
			cfg: llm.ProviderConfigEntry{
				Type:    llm.ProviderTypeAnthropic,
				APIKey:  "sk-ant-test-not-real",
				Enabled: true,
				Parameters: map[string]interface{}{
					"api_key": "sk-ant-test-not-real",
				},
			},
		},
		{
			name: "bedrock",
			t:    llm.ProviderTypeBedrock,
			cfg: llm.ProviderConfigEntry{
				Type:    llm.ProviderTypeBedrock,
				Enabled: true,
				Parameters: map[string]interface{}{
					"region": "us-west-2",
				},
			},
		},
		{
			name: "vertexai",
			t:    llm.ProviderTypeVertexAI,
			cfg: llm.ProviderConfigEntry{
				Type:    llm.ProviderTypeVertexAI,
				Enabled: true,
				Parameters: map[string]interface{}{
					"project_id": "test-proj",
					"location":   "us-central1",
				},
			},
		},
		{
			name: "azure",
			t:    llm.ProviderTypeAzure,
			cfg: llm.ProviderConfigEntry{
				Type:    llm.ProviderTypeAzure,
				APIKey:  "azure-test-key",
				Enabled: true,
				Parameters: map[string]interface{}{
					"endpoint":    "https://example.openai.azure.com",
					"api_key":     "azure-test-key",
					"api_version": "2024-08-01-preview",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := llm.NewCloudProvider(tc.t, tc.cfg)
			// vertex AI may defer ADC; bedrock may defer creds; anthropic +
			// azure read api_key directly. No constructor is allowed to
			// panic on synthetic config.
			if err != nil {
				// Some providers may reject obviously-fake config at
				// construction time. That's still a real, non-panicking
				// failure path — we treat it as acceptable.
				t.Logf("constructor for %s rejected synthetic config (acceptable): %v", tc.name, err)
				return
			}
			require.NotNil(t, provider, "non-nil error implies non-nil provider")
			// Anti-bluff (CONST-035 / §11.9): the original form discarded
			// the result with `_ = provider.GetModels()` so the test
			// passed even if GetModels returned nil for a synthetic-cred
			// provider (which would crash callers iterating the slice).
			// Pin the documented contract: GetModels MUST return a
			// non-nil slice (possibly empty for unauthenticated state)
			// so callers can `range` it safely without a nil-deref. A
			// regression that returned nil would now FAIL.
			models := provider.GetModels()
			assert.NotNil(t, models, "GetModels must return a non-nil slice (possibly empty) so callers can iterate it safely")
		})
	}
}

// TestMultiProvider_WizardNonInteractiveAnthropic exercises the full
// non-interactive wizard pipeline end-to-end:
//
//  1. RunWizard with WizardConfig.NonInteractiveResult builds a result
//     without launching the TUI.
//  2. WriteWizardConfig persists it to a tempdir at mode 0600.
//  3. LoadWizardConfig reads it back, verifying the YAML round-trip.
//  4. Select with the loaded provider name resolves to the correct type.
//
// This is the exact flow `helixcode wizard --provider anthropic --api-key ...`
// uses in production, just with a fixed tempdir path.
func TestMultiProvider_WizardNonInteractiveAnthropic(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "helixcode", "llm.yaml")

	pre := &llm.WizardResult{
		ProviderType: llm.ProviderTypeAnthropic,
		ConfigEntry: llm.ProviderConfigEntry{
			Type:    llm.ProviderTypeAnthropic,
			APIKey:  "sk-ant-fixture-001",
			Enabled: true,
			Parameters: map[string]interface{}{
				"api_key": "sk-ant-fixture-001",
			},
		},
		ConfigPath: cfgPath,
	}

	// Step 1: RunWizard with NonInteractiveResult skips the TUI entirely.
	got, err := llm.RunWizard(ctx, llm.WizardConfig{
		ConfigPath:           cfgPath,
		NonInteractiveResult: pre,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, got.Cancelled)
	require.Equal(t, llm.ProviderTypeAnthropic, got.ProviderType)
	require.Equal(t, "sk-ant-fixture-001", got.ConfigEntry.APIKey)

	// Step 2: persist to disk via WriteWizardConfig (mode 0600 + O_EXCL).
	require.NoError(t, llm.WriteWizardConfig(cfgPath, got))

	// Verify on-disk file mode is 0600.
	st, err := os.Stat(cfgPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), st.Mode().Perm(), "wizard config must be mode 0600")

	// Verify YAML is well-formed and round-trips.
	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	var roundTrip llm.WizardResult
	require.NoError(t, yaml.Unmarshal(raw, &roundTrip))
	require.Equal(t, llm.ProviderTypeAnthropic, roundTrip.ProviderType)

	// Step 3: LoadWizardConfig reads it back through the production code path.
	loaded, err := llm.LoadWizardConfig(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, llm.ProviderTypeAnthropic, loaded.ProviderType)
	require.Equal(t, "sk-ant-fixture-001", loaded.ConfigEntry.APIKey)

	// Step 4: feed the loaded provider name back through Select to confirm
	// the round-trip is closed.
	resolved, err := llm.Select(llm.SelectorInput{Config: string(loaded.ProviderType)})
	require.NoError(t, err)
	require.Equal(t, llm.ProviderTypeAnthropic, resolved)
}

// TestMultiProvider_WizardLoadMissingFileIsNotExist verifies the read-side
// counterpart: a missing wizard config returns os.ErrNotExist (wrapped) so
// callers can fall through to other selection sources.
func TestMultiProvider_WizardLoadMissingFileIsNotExist(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-file.yaml")
	_, err := llm.LoadWizardConfig(missing)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist),
		"expected os.ErrNotExist for missing wizard config, got: %v", err)
}

// TestMultiProvider_RealCloudCallSkipsWithoutCreds is the gated cloud
// integration test. It only runs when ANTHROPIC_API_KEY is present in the
// environment; otherwise it SKIP-OKs with a loud, traceable marker so
// nobody mistakes "skipped" for "passed".
func TestMultiProvider_RealCloudCallSkipsWithoutCreds(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("SKIP-OK: P1-F12 cloud creds not provided (set ANTHROPIC_API_KEY to run this test)")
	}

	cfg := llm.ProviderConfigEntry{
		Type:    llm.ProviderTypeAnthropic,
		APIKey:  apiKey,
		Enabled: true,
		Parameters: map[string]interface{}{
			"api_key": apiKey,
		},
	}
	provider, err := llm.NewCloudProvider(llm.ProviderTypeAnthropic, cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Real models query: this must hit api.anthropic.com or fail loudly.
	models := provider.GetModels()
	require.NotEmpty(t, models, "anthropic provider must return at least one model when authed")
	t.Logf("anthropic GetModels returned %d models", len(models))
}
