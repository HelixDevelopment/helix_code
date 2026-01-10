package llm

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAliasManager_AddAlias tests adding aliases
func TestAliasManager_AddAlias(t *testing.T) {
	manager := NewAliasManager(0.7)

	alias := &ModelAlias{
		Alias:       "gpt4",
		TargetModel: "gpt-4",
		Provider:    "openai",
		Description: "GPT-4 model",
		Tags:        []string{"openai", "gpt"},
	}

	// Add alias
	err := manager.AddAlias(alias)
	if err != nil {
		t.Fatalf("AddAlias() error = %v", err)
	}

	// Verify it was added
	if manager.Count() != 1 {
		t.Errorf("Count() = %d, want 1", manager.Count())
	}

	// Try to add duplicate
	err = manager.AddAlias(alias)
	if err == nil {
		t.Error("AddAlias() should error on duplicate")
	}
}

// TestAliasManager_AddAlias_Validation tests alias validation
func TestAliasManager_AddAlias_Validation(t *testing.T) {
	manager := NewAliasManager(0.7)

	tests := []struct {
		name      string
		alias     *ModelAlias
		shouldErr bool
	}{
		{
			name: "valid alias",
			alias: &ModelAlias{
				Alias:       "test",
				TargetModel: "test-model",
				Provider:    "test-provider",
			},
			shouldErr: false,
		},
		{
			name: "empty alias name",
			alias: &ModelAlias{
				Alias:       "",
				TargetModel: "test-model",
			},
			shouldErr: true,
		},
		{
			name: "empty target model",
			alias: &ModelAlias{
				Alias:       "test",
				TargetModel: "",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AddAlias(tt.alias)
			if (err != nil) != tt.shouldErr {
				t.Errorf("AddAlias() error = %v, shouldErr = %v", err, tt.shouldErr)
			}
		})
	}
}

// TestAliasManager_Resolve tests alias resolution
func TestAliasManager_Resolve(t *testing.T) {
	manager := NewAliasManager(0.7)

	// Add test aliases
	manager.AddAlias(&ModelAlias{
		Alias:       "gpt4",
		TargetModel: "gpt-4",
		Provider:    "openai",
	})
	manager.AddAlias(&ModelAlias{
		Alias:       "claude",
		TargetModel: "claude-3-opus-20240229",
		Provider:    "anthropic",
	})

	tests := []struct {
		name         string
		input        string
		wantModel    string
		wantProvider string
		wantResolved bool
	}{
		{
			name:         "exact match lowercase",
			input:        "gpt4",
			wantModel:    "gpt-4",
			wantProvider: "openai",
			wantResolved: true,
		},
		{
			name:         "exact match uppercase",
			input:        "GPT4",
			wantModel:    "gpt-4",
			wantProvider: "openai",
			wantResolved: true,
		},
		{
			name:         "exact match mixed case",
			input:        "Claude",
			wantModel:    "claude-3-opus-20240229",
			wantProvider: "anthropic",
			wantResolved: true,
		},
		{
			name:         "not an alias",
			input:        "gpt-3.5-turbo",
			wantModel:    "gpt-3.5-turbo",
			wantProvider: "",
			wantResolved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, provider, resolved := manager.Resolve(tt.input)

			if model != tt.wantModel {
				t.Errorf("Resolve() model = %v, want %v", model, tt.wantModel)
			}
			if provider != tt.wantProvider {
				t.Errorf("Resolve() provider = %v, want %v", provider, tt.wantProvider)
			}
			if resolved != tt.wantResolved {
				t.Errorf("Resolve() resolved = %v, want %v", resolved, tt.wantResolved)
			}
		})
	}
}

// TestAliasManager_FuzzyMatch tests fuzzy matching
func TestAliasManager_FuzzyMatch(t *testing.T) {
	manager := NewAliasManager(0.6) // Lower threshold for testing

	manager.AddAlias(&ModelAlias{
		Alias:       "gpt-4-turbo",
		TargetModel: "gpt-4-turbo-preview",
		Provider:    "openai",
	})

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		wantModel   string
	}{
		{
			name:        "exact match",
			input:       "gpt-4-turbo",
			shouldMatch: true,
			wantModel:   "gpt-4-turbo-preview",
		},
		{
			name:        "partial match",
			input:       "gpt4turbo",
			shouldMatch: true,
			wantModel:   "gpt-4-turbo-preview",
		},
		{
			name:        "typo",
			input:       "gpt-4-turb",
			shouldMatch: true,
			wantModel:   "gpt-4-turbo-preview",
		},
		{
			name:        "too different",
			input:       "claude",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, _, resolved := manager.Resolve(tt.input)

			if resolved != tt.shouldMatch {
				t.Errorf("Resolve() resolved = %v, want %v", resolved, tt.shouldMatch)
			}

			if tt.shouldMatch && model != tt.wantModel {
				t.Errorf("Resolve() model = %v, want %v", model, tt.wantModel)
			}
		})
	}
}

// TestAliasManager_ResolveWithProvider tests provider-specific resolution
func TestAliasManager_ResolveWithProvider(t *testing.T) {
	manager := NewAliasManager(0.7)

	// Add aliases with different names for different providers
	manager.AddAlias(&ModelAlias{
		Alias:       "fast-openai",
		TargetModel: "gpt-3.5-turbo",
		Provider:    "openai",
	})
	manager.AddAlias(&ModelAlias{
		Alias:       "fast-anthropic",
		TargetModel: "claude-3-haiku-20240307",
		Provider:    "anthropic",
	})

	tests := []struct {
		name         string
		alias        string
		provider     string
		wantModel    string
		wantProvider string
	}{
		{
			name:         "resolve with openai provider",
			alias:        "fast-openai",
			provider:     "openai",
			wantModel:    "gpt-3.5-turbo",
			wantProvider: "openai",
		},
		{
			name:         "resolve with anthropic provider",
			alias:        "fast-anthropic",
			provider:     "anthropic",
			wantModel:    "claude-3-haiku-20240307",
			wantProvider: "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, provider, _ := manager.ResolveWithProvider(tt.alias, tt.provider)

			if model != tt.wantModel {
				t.Errorf("ResolveWithProvider() model = %v, want %v", model, tt.wantModel)
			}
			if provider != tt.wantProvider {
				t.Errorf("ResolveWithProvider() provider = %v, want %v", provider, tt.wantProvider)
			}
		})
	}
}

// TestAliasManager_RemoveAlias tests removing aliases
func TestAliasManager_RemoveAlias(t *testing.T) {
	manager := NewAliasManager(0.7)

	alias := &ModelAlias{
		Alias:       "test",
		TargetModel: "test-model",
		Provider:    "test-provider",
	}

	manager.AddAlias(alias)

	// Verify it exists
	if manager.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", manager.Count())
	}

	// Remove it
	err := manager.RemoveAlias("test")
	if err != nil {
		t.Errorf("RemoveAlias() error = %v", err)
	}

	// Verify it's gone
	if manager.Count() != 0 {
		t.Errorf("Count() = %d, want 0", manager.Count())
	}

	// Try to remove again
	err = manager.RemoveAlias("test")
	if err == nil {
		t.Error("RemoveAlias() should error when alias doesn't exist")
	}
}

// TestAliasManager_ListAliases tests listing aliases
func TestAliasManager_ListAliases(t *testing.T) {
	manager := NewAliasManager(0.7)

	aliases := []*ModelAlias{
		{Alias: "gpt4", TargetModel: "gpt-4", Provider: "openai"},
		{Alias: "claude", TargetModel: "claude-3-opus", Provider: "anthropic"},
		{Alias: "gemini", TargetModel: "gemini-pro", Provider: "gemini"},
	}

	for _, alias := range aliases {
		manager.AddAlias(alias)
	}

	listed := manager.ListAliases()
	if len(listed) != 3 {
		t.Errorf("ListAliases() length = %d, want 3", len(listed))
	}
}

// TestAliasManager_ListAliasesByProvider tests listing by provider
func TestAliasManager_ListAliasesByProvider(t *testing.T) {
	manager := NewAliasManager(0.7)

	manager.AddAlias(&ModelAlias{Alias: "gpt4", TargetModel: "gpt-4", Provider: "openai"})
	manager.AddAlias(&ModelAlias{Alias: "gpt3", TargetModel: "gpt-3.5-turbo", Provider: "openai"})
	manager.AddAlias(&ModelAlias{Alias: "claude", TargetModel: "claude-3-opus", Provider: "anthropic"})

	openaiAliases := manager.ListAliasesByProvider("openai")
	if len(openaiAliases) != 2 {
		t.Errorf("ListAliasesByProvider(openai) length = %d, want 2", len(openaiAliases))
	}

	anthropicAliases := manager.ListAliasesByProvider("anthropic")
	if len(anthropicAliases) != 1 {
		t.Errorf("ListAliasesByProvider(anthropic) length = %d, want 1", len(anthropicAliases))
	}
}

// TestAliasManager_SearchAliases tests searching aliases
func TestAliasManager_SearchAliases(t *testing.T) {
	manager := NewAliasManager(0.7)

	manager.AddAlias(&ModelAlias{
		Alias:       "gpt4",
		TargetModel: "gpt-4",
		Provider:    "openai",
		Description: "Fast and capable model",
		Tags:        []string{"fast", "smart"},
	})
	manager.AddAlias(&ModelAlias{
		Alias:       "claude",
		TargetModel: "claude-3-opus",
		Provider:    "anthropic",
		Description: "Large context model",
		Tags:        []string{"large", "context"},
	})

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{
			name:      "search by tag",
			query:     "fast",
			wantCount: 1,
		},
		{
			name:      "search by description",
			query:     "context",
			wantCount: 1,
		},
		{
			name:      "no matches",
			query:     "nonexistent",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := manager.SearchAliases(tt.query)
			if len(results) != tt.wantCount {
				t.Errorf("SearchAliases() count = %d, want %d", len(results), tt.wantCount)
			}
		})
	}
}

// TestAliasManager_Autocomplete tests autocomplete
func TestAliasManager_Autocomplete(t *testing.T) {
	manager := NewAliasManager(0.7)

	manager.AddAlias(&ModelAlias{Alias: "gpt4", TargetModel: "gpt-4"})
	manager.AddAlias(&ModelAlias{Alias: "gpt3", TargetModel: "gpt-3.5-turbo"})
	manager.AddAlias(&ModelAlias{Alias: "claude", TargetModel: "claude-3-opus"})

	tests := []struct {
		name      string
		partial   string
		wantCount int
		wantMatch string
	}{
		{
			name:      "prefix match",
			partial:   "gpt",
			wantCount: 2,
		},
		{
			name:      "exact prefix",
			partial:   "claude",
			wantCount: 1,
			wantMatch: "claude",
		},
		{
			name:      "no matches",
			partial:   "xyz",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := manager.Autocomplete(tt.partial)
			if len(matches) != tt.wantCount {
				t.Errorf("Autocomplete() count = %d, want %d", len(matches), tt.wantCount)
			}

			if tt.wantMatch != "" {
				found := false
				for _, match := range matches {
					if match == tt.wantMatch {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Autocomplete() missing expected match: %s", tt.wantMatch)
				}
			}
		})
	}
}

// TestAliasManager_SetFuzzyThreshold tests threshold setting
func TestAliasManager_SetFuzzyThreshold(t *testing.T) {
	manager := NewAliasManager(0.7)

	tests := []struct {
		name      string
		threshold float64
		shouldErr bool
	}{
		{
			name:      "valid threshold",
			threshold: 0.8,
			shouldErr: false,
		},
		{
			name:      "invalid threshold too low",
			threshold: 0,
			shouldErr: true,
		},
		{
			name:      "invalid threshold too high",
			threshold: 1.5,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SetFuzzyThreshold(tt.threshold)
			if (err != nil) != tt.shouldErr {
				t.Errorf("SetFuzzyThreshold() error = %v, shouldErr = %v", err, tt.shouldErr)
			}

			if !tt.shouldErr {
				if manager.GetFuzzyThreshold() != tt.threshold {
					t.Errorf("GetFuzzyThreshold() = %v, want %v", manager.GetFuzzyThreshold(), tt.threshold)
				}
			}
		})
	}
}

// TestAliasManager_ImportExport tests import/export
func TestAliasManager_ImportExport(t *testing.T) {
	manager := NewAliasManager(0.7)

	// Add some aliases
	aliases := []*ModelAlias{
		{Alias: "gpt4", TargetModel: "gpt-4", Provider: "openai"},
		{Alias: "claude", TargetModel: "claude-3-opus", Provider: "anthropic"},
	}

	for _, alias := range aliases {
		manager.AddAlias(alias)
	}

	// Export
	exported := manager.ExportAliases()
	if len(exported) != 2 {
		t.Errorf("ExportAliases() length = %d, want 2", len(exported))
	}

	// Create new manager and import
	newManager := NewAliasManager(0.7)
	err := newManager.ImportAliases(exported, false)
	if err != nil {
		t.Errorf("ImportAliases() error = %v", err)
	}

	if newManager.Count() != 2 {
		t.Errorf("ImportAliases() count = %d, want 2", newManager.Count())
	}
}

// TestAliasManager_Clear tests clearing all aliases
func TestAliasManager_Clear(t *testing.T) {
	manager := NewAliasManager(0.7)

	manager.AddAlias(&ModelAlias{Alias: "test", TargetModel: "test-model"})

	if manager.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", manager.Count())
	}

	manager.Clear()

	if manager.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", manager.Count())
	}
}

// TestLoadAliasConfig tests loading config from file
func TestLoadAliasConfig(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "aliases.yaml")

	// Create test config
	config := &AliasConfig{
		Version:        "1.0",
		FuzzyThreshold: 0.8,
		Aliases: []*ModelAlias{
			{Alias: "gpt4", TargetModel: "gpt-4", Provider: "openai"},
		},
	}

	// Save config
	err := SaveAliasConfig(config, configPath)
	if err != nil {
		t.Fatalf("SaveAliasConfig() error = %v", err)
	}

	// Load config
	loaded, err := LoadAliasConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAliasConfig() error = %v", err)
	}

	if loaded.Version != "1.0" {
		t.Errorf("Version = %v, want 1.0", loaded.Version)
	}

	if loaded.FuzzyThreshold != 0.8 {
		t.Errorf("FuzzyThreshold = %v, want 0.8", loaded.FuzzyThreshold)
	}

	if len(loaded.Aliases) != 1 {
		t.Errorf("Aliases length = %d, want 1", len(loaded.Aliases))
	}
}

// TestLoadAliasConfig_NonExistent tests loading non-existent config
func TestLoadAliasConfig_NonExistent(t *testing.T) {
	config, err := LoadAliasConfig("/nonexistent/path/aliases.yaml")
	if err != nil {
		t.Errorf("LoadAliasConfig() should return default config, got error: %v", err)
	}

	if config == nil {
		t.Error("LoadAliasConfig() returned nil config")
	}

	// Should return default config with some aliases
	if len(config.Aliases) == 0 {
		t.Error("Default config should have aliases")
	}
}

// TestMergeAliasConfigs tests merging multiple configs
func TestMergeAliasConfigs(t *testing.T) {
	config1 := &AliasConfig{
		Version:        "1.0",
		FuzzyThreshold: 0.7,
		Aliases: []*ModelAlias{
			{Alias: "gpt4", TargetModel: "gpt-4", Provider: "openai"},
		},
	}

	config2 := &AliasConfig{
		Version:        "1.0",
		FuzzyThreshold: 0.8,
		Aliases: []*ModelAlias{
			{Alias: "claude", TargetModel: "claude-3-opus", Provider: "anthropic"},
			{Alias: "gpt4", TargetModel: "gpt-4-turbo", Provider: "openai"}, // Override
		},
	}

	merged := MergeAliasConfigs(config1, config2)

	// Should have both aliases
	if len(merged.Aliases) != 2 {
		t.Errorf("Merged config has %d aliases, want 2", len(merged.Aliases))
	}

	// gpt4 should be overridden by config2
	manager := NewAliasManager(merged.FuzzyThreshold)
	for _, alias := range merged.Aliases {
		manager.AddAlias(alias)
	}

	model, _, _ := manager.Resolve("gpt4")
	if model != "gpt-4-turbo" {
		t.Errorf("gpt4 resolved to %v, want gpt-4-turbo (should be overridden)", model)
	}
}

// TestAliasManager_GetAlias tests getting a specific alias
func TestAliasManager_GetAlias(t *testing.T) {
	manager := NewAliasManager(0.7)

	// Add test alias
	alias := &ModelAlias{
		Alias:       "gpt4",
		TargetModel: "gpt-4",
		Provider:    "openai",
		Description: "GPT-4 model",
	}
	manager.AddAlias(alias)

	tests := []struct {
		name       string
		aliasName  string
		wantExists bool
		wantModel  string
	}{
		{
			name:       "existing alias lowercase",
			aliasName:  "gpt4",
			wantExists: true,
			wantModel:  "gpt-4",
		},
		{
			name:       "existing alias uppercase",
			aliasName:  "GPT4",
			wantExists: true,
			wantModel:  "gpt-4",
		},
		{
			name:       "non-existing alias",
			aliasName:  "nonexistent",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, exists := manager.GetAlias(tt.aliasName)
			if exists != tt.wantExists {
				t.Errorf("GetAlias() exists = %v, want %v", exists, tt.wantExists)
			}
			if tt.wantExists && got.TargetModel != tt.wantModel {
				t.Errorf("GetAlias() model = %v, want %v", got.TargetModel, tt.wantModel)
			}
		})
	}
}

// TestLevenshteinDistance tests the Levenshtein distance function
func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1   string
		s2   string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "def", 3},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			got := levenshteinDistance(tt.s1, tt.s2)
			if got != tt.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.want)
			}
		})
	}
}

// TestLoadAliasManagerFromConfig tests loading alias manager from config file
func TestLoadAliasManagerFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-aliases.yaml")

	// Create test config
	config := &AliasConfig{
		Version:        "1.0",
		FuzzyThreshold: 0.75,
		Aliases: []*ModelAlias{
			{Alias: "gpt4", TargetModel: "gpt-4", Provider: "openai"},
			{Alias: "claude", TargetModel: "claude-3-opus", Provider: "anthropic"},
		},
	}

	// Save config
	err := SaveAliasConfig(config, configPath)
	if err != nil {
		t.Fatalf("SaveAliasConfig() error = %v", err)
	}

	// Load manager from config
	manager, err := LoadAliasManagerFromConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAliasManagerFromConfig() error = %v", err)
	}

	if manager == nil {
		t.Fatal("LoadAliasManagerFromConfig() returned nil manager")
	}

	if manager.Count() != 2 {
		t.Errorf("LoadAliasManagerFromConfig() count = %d, want 2", manager.Count())
	}

	if manager.GetFuzzyThreshold() != 0.75 {
		t.Errorf("LoadAliasManagerFromConfig() threshold = %v, want 0.75", manager.GetFuzzyThreshold())
	}

	// Verify aliases are loaded
	model, _, resolved := manager.Resolve("gpt4")
	if !resolved || model != "gpt-4" {
		t.Errorf("Resolve(gpt4) = %v, %v, want gpt-4, true", model, resolved)
	}
}

// TestSaveAliasManagerToConfig tests saving alias manager to config file
func TestSaveAliasManagerToConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "save-test.yaml")

	// Create manager with aliases
	manager := NewAliasManager(0.8)
	manager.AddAlias(&ModelAlias{Alias: "test1", TargetModel: "model1", Provider: "provider1"})
	manager.AddAlias(&ModelAlias{Alias: "test2", TargetModel: "model2", Provider: "provider2"})

	// Save manager to config
	err := SaveAliasManagerToConfig(manager, configPath)
	if err != nil {
		t.Fatalf("SaveAliasManagerToConfig() error = %v", err)
	}

	// Load config and verify
	config, err := LoadAliasConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAliasConfig() error = %v", err)
	}

	if config.FuzzyThreshold != 0.8 {
		t.Errorf("Saved config threshold = %v, want 0.8", config.FuzzyThreshold)
	}

	if len(config.Aliases) != 2 {
		t.Errorf("Saved config aliases = %d, want 2", len(config.Aliases))
	}
}

// TestGetConfigPaths tests getting standard config paths
func TestGetConfigPaths(t *testing.T) {
	paths := GetConfigPaths()

	if len(paths) == 0 {
		t.Error("GetConfigPaths() returned empty slice")
	}

	// Should include workspace, user, and system paths
	if len(paths) < 3 {
		t.Errorf("GetConfigPaths() returned %d paths, expected at least 3", len(paths))
	}

	// First path should be workspace-relative
	if paths[0] != ".helix/model-aliases.yaml" {
		t.Errorf("First path = %v, want .helix/model-aliases.yaml", paths[0])
	}
}

// TestLoadAliasManagerFromStandardPaths tests loading from standard paths
func TestLoadAliasManagerFromStandardPaths(t *testing.T) {
	// This test loads from standard paths - should return default config
	// when no config files exist
	manager, err := LoadAliasManagerFromStandardPaths()
	if err != nil {
		t.Fatalf("LoadAliasManagerFromStandardPaths() error = %v", err)
	}

	if manager == nil {
		t.Fatal("LoadAliasManagerFromStandardPaths() returned nil manager")
	}

	// Should have default aliases loaded
	if manager.Count() == 0 {
		t.Error("LoadAliasManagerFromStandardPaths() returned empty manager")
	}
}

// TestLoadAliasConfig_InvalidYAML tests loading invalid YAML config
func TestLoadAliasConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	invalidYAML := "{{invalid yaml content::"
	if err := writeTestFile(configPath, invalidYAML); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadAliasConfig(configPath)
	if err == nil {
		t.Error("LoadAliasConfig() should error on invalid YAML")
	}
}

// TestLoadAliasConfig_DefaultsForMissingFields tests defaults for missing fields
func TestLoadAliasConfig_DefaultsForMissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "minimal.yaml")

	// Write YAML with missing fields
	minimalYAML := `aliases:
  - alias: test
    target_model: test-model
`
	if err := writeTestFile(configPath, minimalYAML); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config, err := LoadAliasConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAliasConfig() error = %v", err)
	}

	// Should set default version
	if config.Version != "1.0" {
		t.Errorf("Version = %v, want 1.0 (default)", config.Version)
	}

	// Should set default threshold
	if config.FuzzyThreshold != 0.7 {
		t.Errorf("FuzzyThreshold = %v, want 0.7 (default)", config.FuzzyThreshold)
	}
}

// TestMergeAliasConfigs_Empty tests merging with empty input
func TestMergeAliasConfigs_Empty(t *testing.T) {
	merged := MergeAliasConfigs()

	// Should return default config
	if merged == nil {
		t.Fatal("MergeAliasConfigs() returned nil")
	}

	// Should have default aliases
	if len(merged.Aliases) == 0 {
		t.Error("MergeAliasConfigs() with no args should return default aliases")
	}
}

// TestMergeAliasConfigs_WithNil tests merging with nil configs
func TestMergeAliasConfigs_WithNil(t *testing.T) {
	config1 := &AliasConfig{
		FuzzyThreshold: 0.8,
		Aliases: []*ModelAlias{
			{Alias: "test", TargetModel: "test-model"},
		},
	}

	merged := MergeAliasConfigs(nil, config1, nil)

	if merged == nil {
		t.Fatal("MergeAliasConfigs() returned nil")
	}

	if len(merged.Aliases) != 1 {
		t.Errorf("Merged aliases = %d, want 1", len(merged.Aliases))
	}
}

// TestNormalizeKey tests the normalizeKey method via case-insensitive alias resolution
func TestNormalizeKey(t *testing.T) {
	manager := NewAliasManager(0.7)
	manager.AddAlias(&ModelAlias{
		Alias:       "test-alias",
		TargetModel: "test-model",
		Provider:    "test-provider",
	})

	// These should all resolve to the same alias (case-insensitive)
	tests := []struct {
		input string
	}{
		{"test-alias"},
		{"TEST-ALIAS"},
		{"Test-Alias"},
		{"  test-alias  "},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			model, _, resolved := manager.Resolve(tt.input)
			if !resolved {
				t.Errorf("Resolve(%q) should resolve", tt.input)
			}
			if model != "test-model" {
				t.Errorf("Resolve(%q) = %v, want test-model", tt.input, model)
			}
		})
	}
}

// TestNewAliasManager_Threshold tests NewAliasManager with different thresholds
func TestNewAliasManager_Threshold(t *testing.T) {
	tests := []struct {
		threshold float64
		expected  float64
	}{
		{0.5, 0.5},
		{0.7, 0.7},
		{0.9, 0.9},
		{0.0, 0.7}, // Default
		{-0.1, 0.7}, // Invalid, should use default
		{1.5, 0.7},  // Invalid, should use default
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			manager := NewAliasManager(tt.threshold)
			got := manager.GetFuzzyThreshold()
			if got != tt.expected {
				t.Errorf("NewAliasManager(%v).GetFuzzyThreshold() = %v, want %v", tt.threshold, got, tt.expected)
			}
		})
	}
}

// TestImportAliases_Override tests importing with override option
func TestImportAliases_Override(t *testing.T) {
	manager := NewAliasManager(0.7)

	// Add initial alias
	manager.AddAlias(&ModelAlias{
		Alias:       "test",
		TargetModel: "original-model",
		Provider:    "provider1",
	})

	// Import with override = true
	aliases := []*ModelAlias{
		{Alias: "test", TargetModel: "new-model", Provider: "provider2"},
	}

	err := manager.ImportAliases(aliases, true)
	if err != nil {
		t.Fatalf("ImportAliases() error = %v", err)
	}

	// Should be overridden
	model, _, _ := manager.Resolve("test")
	if model != "new-model" {
		t.Errorf("After override, model = %v, want new-model", model)
	}
}

// TestImportAliases_NoOverride tests importing without override option
func TestImportAliases_NoOverride(t *testing.T) {
	manager := NewAliasManager(0.7)

	// Add initial alias
	manager.AddAlias(&ModelAlias{
		Alias:       "test",
		TargetModel: "original-model",
		Provider:    "provider1",
	})

	// Import with override = false (should skip existing)
	aliases := []*ModelAlias{
		{Alias: "test", TargetModel: "new-model", Provider: "provider2"},
		{Alias: "new", TargetModel: "another-model", Provider: "provider3"},
	}

	err := manager.ImportAliases(aliases, false)
	if err != nil {
		t.Fatalf("ImportAliases() error = %v", err)
	}

	// Original should remain
	model, _, _ := manager.Resolve("test")
	if model != "original-model" {
		t.Errorf("Without override, model = %v, want original-model", model)
	}

	// New alias should be added
	model2, _, resolved := manager.Resolve("new")
	if !resolved || model2 != "another-model" {
		t.Errorf("New alias not added correctly: model = %v, resolved = %v", model2, resolved)
	}
}

// TestFuzzyMatching tests the fuzzy matching capability
func TestFuzzyMatching(t *testing.T) {
	manager := NewAliasManager(0.6) // Set low threshold for fuzzy matching

	// Add some aliases
	manager.AddAlias(&ModelAlias{
		Alias:       "gpt4-turbo",
		TargetModel: "gpt-4-turbo-preview",
		Provider:    "openai",
	})
	manager.AddAlias(&ModelAlias{
		Alias:       "claude-opus",
		TargetModel: "claude-3-opus",
		Provider:    "anthropic",
	})

	// Test fuzzy search
	results := manager.SearchAliases("gpt4")
	if len(results) == 0 {
		t.Error("SearchAliases('gpt4') should find similar aliases")
	}

	results2 := manager.SearchAliases("claude")
	if len(results2) == 0 {
		t.Error("SearchAliases('claude') should find similar aliases")
	}

	// Autocomplete test
	completions := manager.Autocomplete("gpt")
	if len(completions) == 0 {
		t.Error("Autocomplete('gpt') should find completions")
	}
}

// TestResolveWithProvider tests resolving alias with provider
func TestResolveWithProvider(t *testing.T) {
	manager := NewAliasManager(0.7)

	// Add aliases for same name but different providers
	manager.AddAlias(&ModelAlias{
		Alias:       "gpt4-openai",
		TargetModel: "gpt-4",
		Provider:    "openai",
	})
	manager.AddAlias(&ModelAlias{
		Alias:       "gpt4-azure",
		TargetModel: "gpt-4",
		Provider:    "azure",
	})

	// Test resolve with specific provider
	model, provider, resolved := manager.ResolveWithProvider("gpt4-azure", "azure")
	if !resolved {
		t.Error("ResolveWithProvider() should resolve")
	}
	if model != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", model)
	}
	if provider != "azure" {
		t.Errorf("provider = %v, want azure", provider)
	}

	// Test with non-matching provider (should still resolve via alias)
	model2, provider2, resolved2 := manager.ResolveWithProvider("gpt4-openai", "openai")
	if !resolved2 {
		t.Error("ResolveWithProvider() should resolve with matching provider")
	}
	if provider2 != "openai" {
		t.Errorf("provider = %v, want openai", provider2)
	}
	_ = model2
}

// Helper function to write test files
func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// TestDefaultAliasConfig tests the default configuration
func TestDefaultAliasConfig(t *testing.T) {
	config := DefaultAliasConfig()

	if config.Version != "1.0" {
		t.Errorf("Version = %v, want 1.0", config.Version)
	}

	if config.FuzzyThreshold != 0.7 {
		t.Errorf("FuzzyThreshold = %v, want 0.7", config.FuzzyThreshold)
	}

	if len(config.Aliases) == 0 {
		t.Error("Default config should have aliases")
	}

	// Check for some expected default aliases
	hasGPT4 := false
	hasClaude := false
	for _, alias := range config.Aliases {
		if alias.Alias == "gpt4" {
			hasGPT4 = true
		}
		if alias.Alias == "claude" {
			hasClaude = true
		}
	}

	if !hasGPT4 {
		t.Error("Default config should have gpt4 alias")
	}
	if !hasClaude {
		t.Error("Default config should have claude alias")
	}
}
