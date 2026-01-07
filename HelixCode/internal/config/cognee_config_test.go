package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCogneeConfig(t *testing.T) {
	config := DefaultCogneeConfig()

	assert.NotNil(t, config)
	assert.True(t, config.Enabled)
	assert.True(t, config.AutoStart)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8000, config.Port)
	assert.Equal(t, "local", config.Mode)
	assert.True(t, config.DynamicConfig)

	// Check sub-configs
	assert.NotNil(t, config.RemoteAPI)
	assert.NotNil(t, config.Optimization)
	assert.NotNil(t, config.Features)
	assert.NotNil(t, config.Providers)
	assert.NotNil(t, config.API)
	assert.NotNil(t, config.Performance)
	assert.NotNil(t, config.Cache)
	assert.NotNil(t, config.Monitoring)

	// Check default values
	assert.Equal(t, "https://api.cognee.ai", config.RemoteAPI.ServiceEndpoint)
	assert.Equal(t, 30*time.Second, config.RemoteAPI.Timeout)
	assert.True(t, config.Optimization.HostAware)
	assert.True(t, config.Features.KnowledgeGraph)
	assert.True(t, config.API.Enabled)
	assert.Equal(t, 4, config.Performance.Workers)
	assert.True(t, config.Cache.Enabled)
	assert.Equal(t, "redis", config.Cache.Type)
	assert.True(t, config.Monitoring.Enabled)
}

func TestLoadCogneeConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Test loading when file doesn't exist (should return default)
	configPath := filepath.Join(tempDir, "nonexistent.json")
	config, err := LoadCogneeConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.True(t, config.Enabled) // Should be default config

	// Test loading valid config file
	validConfig := &CogneeConfig{
		Enabled:   false,
		AutoStart: false,
		Host:      "example.com",
		Port:      9000,
		Mode:      "cloud",
		RemoteAPI: &CogneeRemoteAPIConfig{
			ServiceEndpoint: "https://api.example.com",
			APIKey:          "test-key",
			Timeout:         60 * time.Second,
		},
		Features: &CogneeFeatureConfig{
			KnowledgeGraph:     false,
			SemanticSearch:     true,
			RealTimeProcessing: false,
		},
	}

	validConfigPath := filepath.Join(tempDir, "valid_config.json")
	configData, err := json.MarshalIndent(validConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(validConfigPath, configData, 0644)
	require.NoError(t, err)

	loadedConfig, err := LoadCogneeConfig(validConfigPath)
	require.NoError(t, err)
	assert.False(t, loadedConfig.Enabled)
	assert.False(t, loadedConfig.AutoStart)
	assert.Equal(t, "example.com", loadedConfig.Host)
	assert.Equal(t, 9000, loadedConfig.Port)
	assert.Equal(t, "cloud", loadedConfig.Mode)
	assert.Equal(t, "https://api.example.com", loadedConfig.RemoteAPI.ServiceEndpoint)
	assert.Equal(t, "test-key", loadedConfig.RemoteAPI.APIKey)
	assert.Equal(t, 60*time.Second, loadedConfig.RemoteAPI.Timeout)
	assert.False(t, loadedConfig.Features.KnowledgeGraph)
	assert.True(t, loadedConfig.Features.SemanticSearch)
	assert.False(t, loadedConfig.Features.RealTimeProcessing)

	// Test loading invalid JSON file
	invalidConfigPath := filepath.Join(tempDir, "invalid_config.json")
	err = os.WriteFile(invalidConfigPath, []byte("{ invalid json }"), 0644)
	require.NoError(t, err)

	_, err = LoadCogneeConfig(invalidConfigPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse Cognee config")
}

func TestSaveCogneeConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	config := &CogneeConfig{
		Enabled:   true,
		AutoStart: false,
		Host:      "test.example.com",
		Port:      7777,
		Mode:      "hybrid",
		RemoteAPI: &CogneeRemoteAPIConfig{
			ServiceEndpoint: "https://api.test.com",
			APIKey:          "test-api-key",
			Timeout:         45 * time.Second,
		},
		Optimization: &CogneeOptimizationConfig{
			HostAware:          false,
			CPUOptimization:    true,
			GPUOptimization:    false,
			MemoryOptimization: true,
		},
		Features: &CogneeFeatureConfig{
			KnowledgeGraph:     true,
			SemanticSearch:     false,
			RealTimeProcessing: true,
			MultiModalSupport:  false,
			GraphAnalytics:     true,
		},
		API: &CogneeServerConfig{
			Enabled:      true,
			Host:         "api.example.com",
			Port:         8080,
			AuthRequired: true,
			RateLimit:    500,
			CORS:         false,
			Timeout:      20 * time.Second,
		},
	}

	err := SaveCogneeConfig(config, configPath)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Verify content
	loadedConfig, err := LoadCogneeConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, config.Enabled, loadedConfig.Enabled)
	assert.Equal(t, config.AutoStart, loadedConfig.AutoStart)
	assert.Equal(t, config.Host, loadedConfig.Host)
	assert.Equal(t, config.Port, loadedConfig.Port)
	assert.Equal(t, config.Mode, loadedConfig.Mode)
	assert.Equal(t, config.RemoteAPI.ServiceEndpoint, loadedConfig.RemoteAPI.ServiceEndpoint)
	assert.Equal(t, config.RemoteAPI.APIKey, loadedConfig.RemoteAPI.APIKey)
	assert.Equal(t, config.Optimization.HostAware, loadedConfig.Optimization.HostAware)
	assert.Equal(t, config.Features.KnowledgeGraph, loadedConfig.Features.KnowledgeGraph)
	assert.Equal(t, config.API.Enabled, loadedConfig.API.Enabled)
	assert.Equal(t, config.API.Host, loadedConfig.API.Host)
	assert.Equal(t, config.API.Port, loadedConfig.API.Port)

	// Test saving to nested directory (should create directory)
	nestedPath := filepath.Join(tempDir, "nested", "dir", "config.json")
	err = SaveCogneeConfig(config, nestedPath)
	require.NoError(t, err)

	_, err = os.Stat(nestedPath)
	assert.NoError(t, err)
}

func TestCogneeConfigApplyDefaults(t *testing.T) {
	config := &CogneeConfig{
		// Only set some fields
		Enabled: true,
		Host:    "custom.host",
		Port:    3000,
		// Leave most fields nil to test defaults
	}

	applied := config.applyDefaults()

	// Should preserve set values
	assert.True(t, applied.Enabled)
	assert.Equal(t, "custom.host", applied.Host)
	assert.Equal(t, 3000, applied.Port)

	// Should apply defaults for nil fields
	assert.NotNil(t, applied.RemoteAPI)
	assert.NotNil(t, applied.Optimization)
	assert.NotNil(t, applied.Features)
	assert.NotNil(t, applied.API)
	assert.NotNil(t, applied.Performance)
	assert.NotNil(t, applied.Cache)
	assert.NotNil(t, applied.Monitoring)

	// Check specific defaults
	assert.Equal(t, "local", applied.Mode) // Should be set if empty
	assert.Equal(t, "https://api.cognee.ai", applied.RemoteAPI.ServiceEndpoint)
	assert.True(t, applied.Optimization.HostAware)
	assert.True(t, applied.Features.KnowledgeGraph)
	assert.Equal(t, 4, applied.Performance.Workers)
	assert.Equal(t, "redis", applied.Cache.Type)
	assert.Equal(t, 9090, applied.Monitoring.MetricsPort)
}

func TestCogneeConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *CogneeConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &CogneeConfig{
				Host: "localhost",
				Port: 8000,
				Mode: "local",
				API: &CogneeServerConfig{
					Host:    "localhost",
					Port:    8000,
					Timeout: 30 * time.Second,
				},
				Performance: &CogneePerformanceConfig{
					Workers:       4,
					QueueSize:     1000,
					BatchSize:     32,
					FlushInterval: 5 * time.Second,
				},
				Cache: &CogneeCacheConfig{
					Type: "redis",
					Port: 6379,
					TTL:  1 * time.Hour,
				},
			},
			expectError: false,
		},
		{
			name: "invalid port - too low",
			config: &CogneeConfig{
				Host: "localhost",
				Port: 0,
			},
			expectError: true,
			errorMsg:    "invalid port: 0",
		},
		{
			name: "invalid port - too high",
			config: &CogneeConfig{
				Host: "localhost",
				Port: 70000,
			},
			expectError: true,
			errorMsg:    "invalid port: 70000",
		},
		{
			name: "empty host",
			config: &CogneeConfig{
				Host: "",
				Port: 8000,
			},
			expectError: true,
			errorMsg:    "host cannot be empty",
		},
		{
			name: "invalid API port",
			config: &CogneeConfig{
				Host: "localhost",
				Port: 8000,
				API: &CogneeServerConfig{
					Host: "localhost",
					Port: 0, // Invalid
				},
			},
			expectError: true,
			errorMsg:    "invalid API port: 0",
		},
		{
			name: "valid config with defaults applied",
			config: &CogneeConfig{
				Host: "localhost",
				Port: 8000,
				// Mode empty - should be set to default
				RemoteAPI: &CogneeRemoteAPIConfig{
					ServiceEndpoint: "", // Should be set to default
					Timeout:         0,  // Should be set to default
				},
				Optimization: &CogneeOptimizationConfig{
					// HostSpecific nil - should be initialized
				},
				Performance: &CogneePerformanceConfig{
					Workers:   0, // Should be set to default
					QueueSize: 0, // Should be set to default
					BatchSize: 0, // Should be set to default
				},
				Cache: &CogneeCacheConfig{
					Type: "redis",
					Port: 0, // Should be set to default 6379
					TTL:  0, // Should be set to default
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)

				// For valid configs, check that defaults were applied
				if tt.config.Mode == "" {
					assert.Equal(t, "local", tt.config.Mode)
				}
				if tt.config.RemoteAPI != nil && tt.config.RemoteAPI.ServiceEndpoint == "" {
					assert.Equal(t, "https://api.cognee.ai", tt.config.RemoteAPI.ServiceEndpoint)
				}
				if tt.config.Performance != nil && tt.config.Performance.Workers == 0 {
					assert.Equal(t, 4, tt.config.Performance.Workers)
				}
			}
		})
	}
}

func TestCogneeConfigToJSON(t *testing.T) {
	config := &CogneeConfig{
		Enabled:   true,
		AutoStart: false,
		Host:      "test.example.com",
		Port:      9000,
		Mode:      "cloud",
		RemoteAPI: &CogneeRemoteAPIConfig{
			ServiceEndpoint: "https://api.test.com",
			APIKey:          "test-key",
			Timeout:         60 * time.Second,
		},
	}

	// Test ToJSON
	jsonData, err := config.ToJSON()
	require.NoError(t, err)
	assert.NotNil(t, jsonData)

	// Verify it's valid JSON
	var parsedConfig CogneeConfig
	err = json.Unmarshal(jsonData, &parsedConfig)
	require.NoError(t, err)
	assert.Equal(t, config.Enabled, parsedConfig.Enabled)
	assert.Equal(t, config.Host, parsedConfig.Host)
	assert.Equal(t, config.Port, parsedConfig.Port)
	assert.Equal(t, config.Mode, parsedConfig.Mode)
}

func TestCogneeConfigToYAML(t *testing.T) {
	config := &CogneeConfig{
		Enabled:   true,
		AutoStart: false,
		Host:      "test.example.com",
		Port:      9000,
		Mode:      "cloud",
	}

	// Test ToYAML
	yamlData, err := config.ToYAML()
	require.NoError(t, err)
	assert.NotNil(t, yamlData)

	// Verify it contains expected content
	yamlStr := string(yamlData)
	assert.Contains(t, yamlStr, "enabled: true")
	assert.Contains(t, yamlStr, "host: test.example.com")
	assert.Contains(t, yamlStr, "port: 9000")
	assert.Contains(t, yamlStr, "mode: cloud")
}

func TestCogneeConfigClone(t *testing.T) {
	original := &CogneeConfig{
		Enabled:   true,
		AutoStart: false,
		Host:      "test.example.com",
		Port:      9000,
		Mode:      "cloud",
		RemoteAPI: &CogneeRemoteAPIConfig{
			ServiceEndpoint: "https://api.test.com",
			APIKey:          "test-key",
			Timeout:         60 * time.Second,
		},
		Optimization: &CogneeOptimizationConfig{
			HostAware:          true,
			CPUOptimization:    false,
			GPUOptimization:    true,
			MemoryOptimization: false,
		},
		Features: &CogneeFeatureConfig{
			KnowledgeGraph:     true,
			SemanticSearch:     false,
			RealTimeProcessing: true,
		},
	}

	// Test Clone
	cloned := original.Clone()

	// Should be equal but different instances
	assert.Equal(t, original.Enabled, cloned.Enabled)
	assert.Equal(t, original.Host, cloned.Host)
	assert.Equal(t, original.Port, cloned.Port)
	assert.Equal(t, original.Mode, cloned.Mode)
	assert.Equal(t, original.RemoteAPI.ServiceEndpoint, cloned.RemoteAPI.ServiceEndpoint)
	assert.Equal(t, original.Optimization.HostAware, cloned.Optimization.HostAware)
	assert.Equal(t, original.Features.KnowledgeGraph, cloned.Features.KnowledgeGraph)

	// Verify they're different objects
	cloned.Enabled = false
	cloned.Host = "changed.com"
	cloned.RemoteAPI.ServiceEndpoint = "https://changed.com"

	assert.True(t, original.Enabled)  // Original should be unchanged
	assert.False(t, cloned.Enabled)   // Clone should be changed
	assert.NotEqual(t, original.Host, cloned.Host)
	assert.NotEqual(t, original.RemoteAPI.ServiceEndpoint, cloned.RemoteAPI.ServiceEndpoint)
}

func TestCogneeConfigMerge(t *testing.T) {
	base := &CogneeConfig{
		Enabled:   true,
		AutoStart: false,
		Host:      "base.example.com",
		Port:      8000,
		Mode:      "local",
		RemoteAPI: &CogneeRemoteAPIConfig{
			ServiceEndpoint: "https://base.api.com",
			APIKey:          "base-key",
			Timeout:         30 * time.Second,
		},
		Features: &CogneeFeatureConfig{
			KnowledgeGraph:     true,
			SemanticSearch:     false,
			RealTimeProcessing: true,
			MultiModalSupport:  false,
		},
	}

	overlay := &CogneeConfig{
		Enabled: false, // Should override
		// AutoStart not set - should keep base value
		Host:  "", // Empty - should not override
		Port:  9000, // Should override
		Mode:  "cloud", // Should override
		RemoteAPI: &CogneeRemoteAPIConfig{
			ServiceEndpoint: "", // Empty - should not override
			APIKey:          "overlay-key", // Should override
			// Timeout not set - should keep base value
		},
		Features: &CogneeFeatureConfig{
			KnowledgeGraph: false, // Should override
			// SemanticSearch not set - should keep base value
			RealTimeProcessing: false, // Should override
			MultiModalSupport:  true,  // Should override
			GraphAnalytics:     true,  // New field - should be added
		},
		Optimization: &CogneeOptimizationConfig{
			HostAware: true, // New field - should be added
		},
	}

	// Test Merge
	base.Merge(overlay)

	// Verify merge results
	assert.False(t, base.Enabled) // Overridden
	assert.False(t, base.AutoStart) // Kept from base
	assert.Equal(t, "base.example.com", base.Host) // Kept (overlay was empty)
	assert.Equal(t, 9000, base.Port) // Overridden
	assert.Equal(t, "cloud", base.Mode) // Overridden

	// RemoteAPI merge
	assert.Equal(t, "https://base.api.com", base.RemoteAPI.ServiceEndpoint) // Kept
	assert.Equal(t, "overlay-key", base.RemoteAPI.APIKey) // Overridden
	assert.Equal(t, 30*time.Second, base.RemoteAPI.Timeout) // Kept

	// Features merge
	assert.False(t, base.Features.KnowledgeGraph) // Overridden
	assert.False(t, base.Features.SemanticSearch) // Kept from base
	assert.False(t, base.Features.RealTimeProcessing) // Overridden
	assert.True(t, base.Features.MultiModalSupport) // Overridden
	assert.True(t, base.Features.GraphAnalytics) // Added from overlay

	// New fields should be added
	assert.NotNil(t, base.Optimization)
	assert.True(t, base.Optimization.HostAware)
}

func TestCogneeConfigMergeScenarios(t *testing.T) {
	// Test merging with nil overlay
	base := &CogneeConfig{
		Enabled: true,
		Host:    "test.com",
		Port:    8000,
	}

	originalEnabled := base.Enabled
	originalHost := base.Host
	originalPort := base.Port

	base.Merge(nil)
	assert.Equal(t, originalEnabled, base.Enabled)
	assert.Equal(t, originalHost, base.Host)
	assert.Equal(t, originalPort, base.Port)

	// Test merging with nil fields in overlay
	overlay := &CogneeConfig{
		Enabled: false,
		// Other fields nil
	}

	originalHost = base.Host
	originalPort = base.Port
	
	base.Merge(overlay)
	assert.False(t, base.Enabled) // Should override
	assert.Equal(t, originalHost, base.Host) // Should keep base
	assert.Equal(t, originalPort, base.Port) // Should keep base

	// Test merging both configs having different structures
	baseComplex := &CogneeConfig{
		Enabled: true,
		API: &CogneeServerConfig{
			Enabled:    true,
			Host:       "api.base.com",
			Port:       8080,
			RateLimit:  1000,
			DocsEnabled: true,
		},
		Performance: &CogneePerformanceConfig{
			Workers:   4,
			QueueSize: 1000,
			BatchSize: 32,
		},
	}

	overlayComplex := &CogneeConfig{
		Enabled: false,
		API: &CogneeServerConfig{
			Enabled: false, // Override
			Host:    "api.overlay.com", // Override
			// Port not set - should keep base
			RateLimit: 500, // Override
			// DocsEnabled not set - should keep base
			Timeout: 60 * time.Second, // New field
		},
		Cache: &CogneeCacheConfig{
			Type: "redis",
			Port: 6379,
		},
	}

	baseComplex.Merge(overlayComplex)
	assert.False(t, baseComplex.Enabled)
	assert.Equal(t, "api.overlay.com", baseComplex.API.Host)
	assert.Equal(t, 8080, baseComplex.API.Port) // Kept from base
	assert.Equal(t, 500, baseComplex.API.RateLimit) // Overridden
	assert.True(t, baseComplex.API.DocsEnabled) // Kept from base
	assert.Equal(t, 60*time.Second, baseComplex.API.Timeout) // Added
	assert.NotNil(t, baseComplex.Cache) // Added from overlay
	assert.Equal(t, "redis", baseComplex.Cache.Type)
	assert.Equal(t, 4, baseComplex.Performance.Workers) // Kept from base
}

func TestCogneeConfigConstants(t *testing.T) {
	// Test mode constants
	assert.Equal(t, "local", CogneeModeLocal)
	assert.Equal(t, "cloud", CogneeModeCloud)
	assert.Equal(t, "hybrid", CogneeModeHybrid)
}

func TestCogneeConfigEdgeCases(t *testing.T) {
	// Test empty config validation - port is validated before host
	emptyConfig := &CogneeConfig{}
	err := emptyConfig.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port")

	// Test config with minimal valid fields
	minimalConfig := &CogneeConfig{
		Host: "localhost",
		Port: 8000,
	}
	err = minimalConfig.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "local", minimalConfig.Mode) // Should be set to default

	// Test config with all optional fields as nil
	configWithNils := &CogneeConfig{
		Host: "localhost",
		Port: 8000,
		// All other fields nil
	}
	err = configWithNils.Validate()
	assert.NoError(t, err)
	// Validate doesn't create nested configs - they remain nil
	// To get defaults, use WithDefaults() instead
	assert.Nil(t, configWithNils.RemoteAPI)
	assert.Nil(t, configWithNils.Optimization)
	assert.Nil(t, configWithNils.Features)
	assert.Nil(t, configWithNils.API)
	assert.Nil(t, configWithNils.Performance)
	assert.Nil(t, configWithNils.Cache)
	assert.Nil(t, configWithNils.Monitoring)
}

func TestCogneeConfigProviderIntegration(t *testing.T) {
	config := &CogneeConfig{
		Providers: make(map[string]*CogneeProviderConfig),
	}

	// Add some providers
	config.Providers["openai"] = &CogneeProviderConfig{
		Enabled:     true,
		Integration: "api",
		Priority:    1,
		Features:    []string{"embeddings", "completion"},
	}

	config.Providers["anthropic"] = &CogneeProviderConfig{
		Enabled:     false,
		Integration: "api",
		Priority:    2,
		Features:    []string{"completion"},
	}

	// Test that providers are preserved through operations
	err := SaveCogneeConfig(config, "/tmp/test_providers.json")
	require.NoError(t, err)

	loaded, err := LoadCogneeConfig("/tmp/test_providers.json")
	require.NoError(t, err)

	assert.Len(t, loaded.Providers, 2)
	assert.True(t, loaded.Providers["openai"].Enabled)
	assert.False(t, loaded.Providers["anthropic"].Enabled)
	assert.Equal(t, 1, loaded.Providers["openai"].Priority)
	assert.Equal(t, []string{"embeddings", "completion"}, loaded.Providers["openai"].Features)

	// Cleanup
	os.Remove("/tmp/test_providers.json")
}