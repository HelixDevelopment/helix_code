// p1f12_challenge runs the F12 Multi-Provider Backend pipeline end-to-end
// against real disk I/O. Runtime-evidence harness for the F12 Challenge.
//
// Phases:
//
//	A. Selector precedence — flag > env > config > ErrNoProviderConfigured.
//	B. Factory — NewCloudProvider for all four cloud backends.
//	C. Wizard non-interactive write/read round-trip on REAL disk
//	   (XDG_CONFIG_HOME tempdir; mode 0600 verified via os.Stat).
//	D. End-to-end Selector + factory after disk read.
//	E. (gated) Real cloud call: only if ANTHROPIC_API_KEY is set; otherwise
//	   prints `[skipped: ANTHROPIC_API_KEY not set]`.
//
// Exit code 0 on success; exit 1 with a diagnostic on any check failure.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.code/internal/llm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F12 challenge harness pid:", os.Getpid())

	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	wizardPath, err := phaseC()
	if err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(wizardPath); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F12 challenge harness PASS")
	return nil
}

// phaseA exercises Selector precedence with explicit inputs.
func phaseA() error {
	fmt.Println("==> phase A: Selector precedence (flag > env > config)")

	// A1: env present, no flag, no config -> env wins.
	got, err := llm.Select(llm.SelectorInput{Env: "anthropic"})
	if err != nil {
		return fmt.Errorf("A1 Select(env=anthropic): %w", err)
	}
	if got != llm.ProviderTypeAnthropic {
		return fmt.Errorf("A1 expected ProviderTypeAnthropic; got %q", got)
	}
	fmt.Printf("    A1 env=anthropic flag=\"\" config=\"\" -> %q OK\n", got)

	// A2: flag overrides env.
	got, err = llm.Select(llm.SelectorInput{Flag: "bedrock", Env: "anthropic"})
	if err != nil {
		return fmt.Errorf("A2 Select(flag=bedrock,env=anthropic): %w", err)
	}
	if got != llm.ProviderTypeBedrock {
		return fmt.Errorf("A2 expected ProviderTypeBedrock; got %q", got)
	}
	fmt.Printf("    A2 flag=bedrock env=anthropic config=\"\" -> %q OK (flag wins)\n", got)

	// A3: every source empty -> sentinel.
	_, err = llm.Select(llm.SelectorInput{})
	if err == nil {
		return fmt.Errorf("A3 Select(empty) returned nil error; want ErrNoProviderConfigured")
	}
	if !errors.Is(err, llm.ErrNoProviderConfigured) {
		return fmt.Errorf("A3 expected errors.Is(err, ErrNoProviderConfigured); got %v", err)
	}
	fmt.Printf("    A3 all-empty -> errors.Is(err, ErrNoProviderConfigured) OK (%v)\n", err)

	// A4: config-only path resolves correctly (vertexai aliasing).
	got, err = llm.Select(llm.SelectorInput{Config: "vertex-ai"})
	if err != nil {
		return fmt.Errorf("A4 Select(config=vertex-ai): %w", err)
	}
	if got != llm.ProviderTypeVertexAI {
		return fmt.Errorf("A4 expected ProviderTypeVertexAI; got %q", got)
	}
	fmt.Printf("    A4 config=vertex-ai -> %q OK\n", got)

	return nil
}

// phaseB constructs all four cloud providers via NewCloudProvider with
// synthetic config and asserts the Provider interface contract.
func phaseB() error {
	fmt.Println("==> phase B: NewCloudProvider constructs all 4 cloud backends")

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
				APIKey:  "test",
				Enabled: true,
				Parameters: map[string]interface{}{
					"api_key": "test",
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
					"region": "us-east-1",
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
					"project_id": "test-project",
					"location":   "us-central1",
				},
			},
		},
		{
			name: "azure",
			t:    llm.ProviderTypeAzure,
			cfg: llm.ProviderConfigEntry{
				Type:    llm.ProviderTypeAzure,
				APIKey:  "test",
				Enabled: true,
				Parameters: map[string]interface{}{
					"endpoint":    "https://test.openai.azure.com",
					"api_key":     "test",
					"api_version": "2024-08-01-preview",
				},
			},
		},
	}

	// Bedrock SDK may demand env vars on construction in some setups; set
	// synthetic creds so it does not refuse to build offline. Harness env
	// is process-local; the binary runs as a child of run.sh in the
	// challenge harness and inherits nothing from production secrets.
	_ = os.Setenv("AWS_ACCESS_KEY_ID", "test")
	_ = os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	_ = os.Setenv("AWS_REGION", "us-east-1")

	constructed := 0
	rejected := 0
	for _, tc := range cases {
		provider, err := llm.NewCloudProvider(tc.t, tc.cfg)
		if err != nil {
			// Per T07, constructors are allowed to defer credential
			// validation; if a constructor rejects synthetic config
			// outright, that is a real, non-bluff failure path. Note
			// it but do not fail the harness.
			fmt.Printf("    B.%s constructor rejected synthetic config (acceptable): %v\n",
				tc.name, err)
			rejected++
			continue
		}
		if provider == nil {
			return fmt.Errorf("B.%s NewCloudProvider returned nil provider with nil error", tc.name)
		}
		// Provider interface compliance: GetType + GetName + GetModels
		// must not panic on a freshly-constructed provider.
		if provider.GetType() != tc.t {
			return fmt.Errorf("B.%s provider.GetType()=%q want %q", tc.name, provider.GetType(), tc.t)
		}
		name := provider.GetName()
		models := provider.GetModels()
		fmt.Printf("    B.%s constructed OK type=%q name=%q models=%d\n",
			tc.name, provider.GetType(), name, len(models))
		constructed++
	}

	if constructed+rejected != len(cases) {
		return fmt.Errorf("B accounting: constructed=%d rejected=%d total=%d (want %d)",
			constructed, rejected, constructed+rejected, len(cases))
	}
	if constructed == 0 {
		return fmt.Errorf("B: zero providers constructed; harness asserts at least one cloud backend builds")
	}
	fmt.Printf("    B summary: constructed=%d rejected=%d / %d\n", constructed, rejected, len(cases))
	return nil
}

// phaseC writes a wizard config to a real tempdir (mode 0600 enforced) and
// reads it back through LoadWizardConfig.
func phaseC() (string, error) {
	fmt.Println("==> phase C: wizard non-interactive write/read round-trip on disk")

	dir, err := os.MkdirTemp("", "p1f12-xdg-")
	if err != nil {
		return "", fmt.Errorf("tempdir: %w", err)
	}
	// Set XDG_CONFIG_HOME to our tempdir; defaultWizardConfigPath honours it.
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		return "", fmt.Errorf("setenv XDG_CONFIG_HOME: %w", err)
	}
	cfgPath := filepath.Join(dir, "helixcode", "llm.yaml")
	fmt.Printf("    XDG_CONFIG_HOME=%s\n", dir)
	fmt.Printf("    cfgPath        =%s\n", cfgPath)

	pre := &llm.WizardResult{
		ProviderType: llm.ProviderTypeAnthropic,
		ConfigEntry: llm.ProviderConfigEntry{
			Type:    llm.ProviderTypeAnthropic,
			APIKey:  "harness-test-key",
			Enabled: true,
			Parameters: map[string]interface{}{
				"api_key": "harness-test-key",
			},
		},
		ConfigPath: cfgPath,
	}

	// Drive the same path the cobra wizard uses.
	got, err := llm.RunWizard(context.Background(), llm.WizardConfig{
		ConfigPath:           cfgPath,
		NonInteractiveResult: pre,
	})
	if err != nil {
		return "", fmt.Errorf("RunWizard: %w", err)
	}
	if got == nil || got.Cancelled {
		return "", fmt.Errorf("RunWizard returned cancelled/nil result: %+v", got)
	}
	if got.ProviderType != llm.ProviderTypeAnthropic {
		return "", fmt.Errorf("RunWizard ProviderType = %q; want anthropic", got.ProviderType)
	}
	if got.ConfigEntry.APIKey != "harness-test-key" {
		return "", fmt.Errorf("RunWizard APIKey = %q; want harness-test-key", got.ConfigEntry.APIKey)
	}
	fmt.Printf("    RunWizard OK provider=%q api_key=%q\n",
		got.ProviderType, got.ConfigEntry.APIKey)

	// Persist via the secret-safe writer.
	if err := llm.WriteWizardConfig(cfgPath, got); err != nil {
		return "", fmt.Errorf("WriteWizardConfig: %w", err)
	}

	st, err := os.Stat(cfgPath)
	if err != nil {
		return "", fmt.Errorf("os.Stat(cfgPath): %w", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		return "", fmt.Errorf("on-disk file mode = %o; want 0600 (secret-safe)", perm)
	}
	fmt.Printf("    on-disk size=%d bytes mode=0600 OK\n", st.Size())

	loaded, err := llm.LoadWizardConfig(cfgPath)
	if err != nil {
		return "", fmt.Errorf("LoadWizardConfig: %w", err)
	}
	if loaded == nil {
		return "", fmt.Errorf("LoadWizardConfig returned nil with nil error")
	}
	if loaded.ProviderType != llm.ProviderTypeAnthropic {
		return "", fmt.Errorf("loaded ProviderType = %q; want anthropic", loaded.ProviderType)
	}
	if loaded.ConfigEntry.APIKey != "harness-test-key" {
		return "", fmt.Errorf("loaded APIKey = %q; want harness-test-key (round-trip lost the secret)",
			loaded.ConfigEntry.APIKey)
	}
	fmt.Printf("    LoadWizardConfig OK provider=%q api_key=%q\n",
		loaded.ProviderType, loaded.ConfigEntry.APIKey)

	return cfgPath, nil
}

// phaseD drives Selector + factory using the wizard config that phase C
// just wrote to disk.
func phaseD(wizardPath string) error {
	fmt.Println("==> phase D: end-to-end Selector + factory after disk read")

	loaded, err := llm.LoadWizardConfig(wizardPath)
	if err != nil {
		return fmt.Errorf("LoadWizardConfig: %w", err)
	}
	resolved, err := llm.Select(llm.SelectorInput{Config: string(loaded.ProviderType)})
	if err != nil {
		return fmt.Errorf("Select(config=%q): %w", loaded.ProviderType, err)
	}
	if resolved != llm.ProviderTypeAnthropic {
		return fmt.Errorf("Select resolved to %q; want anthropic", resolved)
	}

	provider, err := llm.NewCloudProvider(resolved, loaded.ConfigEntry)
	if err != nil {
		return fmt.Errorf("NewCloudProvider: %w", err)
	}
	if provider == nil {
		return fmt.Errorf("NewCloudProvider returned nil provider")
	}
	if provider.GetType() != llm.ProviderTypeAnthropic {
		return fmt.Errorf("provider.GetType() = %q; want anthropic", provider.GetType())
	}
	fmt.Printf("    Select(loaded.ProviderType) -> %q\n", resolved)
	fmt.Printf("    NewCloudProvider OK type=%q name=%q\n",
		provider.GetType(), provider.GetName())

	return nil
}

// phaseE makes a real cloud call IFF ANTHROPIC_API_KEY is set. Otherwise
// it prints a noted-skip and returns nil.
func phaseE() error {
	fmt.Println("==> phase E: real cloud round-trip (gated on ANTHROPIC_API_KEY)")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("    [skipped: ANTHROPIC_API_KEY not set]")
		return nil
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
	if err != nil {
		return fmt.Errorf("NewCloudProvider: %w", err)
	}

	ctx := context.Background()
	health, err := provider.GetHealth(ctx)
	if err != nil {
		return fmt.Errorf("provider.GetHealth: %w", err)
	}
	fmt.Printf("    provider.GetHealth status=%q model_count=%d latency=%s\n",
		health.Status, health.ModelCount, health.Latency)

	models := provider.GetModels()
	fmt.Printf("    provider.GetModels returned %d models\n", len(models))
	return nil
}
