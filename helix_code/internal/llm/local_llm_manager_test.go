package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalLLMManager(t *testing.T) {
	t.Run("CreatesWithBaseDir", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		require.NotNil(t, manager)
		assert.Equal(t, tempDir, manager.baseDir)
		assert.Contains(t, manager.binaryDir, "bin")
		assert.Contains(t, manager.configDir, "config")
		assert.Contains(t, manager.dataDir, "data")
		assert.NotNil(t, manager.providers)
		assert.NotNil(t, manager.httpClient)
		assert.False(t, manager.isInitialized)
	})

	t.Run("CreatesWithEmptyBaseDir", func(t *testing.T) {
		manager := NewLocalLLMManager("")
		require.NotNil(t, manager)
		// Should use default home directory
		assert.Contains(t, manager.baseDir, "local-llm")
	})
}

func TestLocalLLMManager_GetBaseDir(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)
	assert.Equal(t, tempDir, manager.GetBaseDir())
}

func TestLocalLLMManager_SetSkipProviderInstall(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	assert.False(t, manager.skipProviderInstall)
	manager.SetSkipProviderInstall(true)
	assert.True(t, manager.skipProviderInstall)
}

func TestLocalLLMManager_CreateDirectories(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	err := manager.createDirectories()
	require.NoError(t, err)

	// Verify directories were created
	_, err = os.Stat(manager.baseDir)
	assert.NoError(t, err)
	_, err = os.Stat(manager.binaryDir)
	assert.NoError(t, err)
	_, err = os.Stat(manager.configDir)
	assert.NoError(t, err)
	_, err = os.Stat(manager.dataDir)
	assert.NoError(t, err)
}

func TestLocalLLMManager_Initialize(t *testing.T) {
	t.Run("InitializesWithSkipInstall", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		manager.SetSkipProviderInstall(true)

		ctx := context.Background()
		err := manager.Initialize(ctx)
		require.NoError(t, err)
		assert.True(t, manager.isInitialized)
		assert.NotEmpty(t, manager.providers)
	})

	t.Run("InitializationIsIdempotent", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		manager.SetSkipProviderInstall(true)

		ctx := context.Background()
		err := manager.Initialize(ctx)
		require.NoError(t, err)

		// Second init should return early
		err = manager.Initialize(ctx)
		require.NoError(t, err)
	})
}

func TestLocalLLMManager_StartProvider(t *testing.T) {
	t.Run("ErrorWhenProviderNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		manager.SetSkipProviderInstall(true)

		ctx := context.Background()
		err := manager.StartProvider(ctx, "nonexistent-provider")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ErrorWhenAlreadyRunning", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		manager.SetSkipProviderInstall(true)

		ctx := context.Background()
		manager.Initialize(ctx)

		// Set provider status to running
		if provider, exists := manager.providers["vllm"]; exists {
			provider.Status = "running"

			err := manager.StartProvider(ctx, "vllm")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "already running")
		}
	})
}

func TestLocalLLMManager_StopProvider(t *testing.T) {
	t.Run("ErrorWhenProviderNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)

		ctx := context.Background()
		err := manager.StopProvider(ctx, "nonexistent-provider")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ErrorWhenNotRunning", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		manager.SetSkipProviderInstall(true)

		ctx := context.Background()
		manager.Initialize(ctx)

		// Provider is not running by default
		err := manager.StopProvider(ctx, "vllm")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestLocalLLMManager_UpdateProvider(t *testing.T) {
	t.Run("ErrorWhenProviderNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)

		ctx := context.Background()
		err := manager.UpdateProvider(ctx, "nonexistent-provider")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestLocalLLMManager_GetProviderStatus(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)
	manager.SetSkipProviderInstall(true)

	ctx := context.Background()
	manager.Initialize(ctx)

	status := manager.GetProviderStatus(ctx)
	assert.NotEmpty(t, status)

	// All providers should have LastCheck updated
	for _, provider := range status {
		assert.False(t, provider.LastCheck.IsZero())
	}
}

func TestLocalLLMManager_GetRunningProviders(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)
	manager.SetSkipProviderInstall(true)

	ctx := context.Background()
	manager.Initialize(ctx)

	// No providers running initially
	running := manager.GetRunningProviders(ctx)
	assert.Empty(t, running)

	// Set one as running
	if provider, exists := manager.providers["vllm"]; exists {
		provider.Status = "running"
		// Note: GetRunningProviders checks health, so this won't actually appear
		// as running unless the health check passes
	}
}

func TestLocalLLMManager_DetectModelFormat(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	testCases := []struct {
		path     string
		expected ModelFormat
		hasError bool
	}{
		{"/path/to/model.gguf", FormatGGUF, false},
		{"/path/to/model.pt", FormatHF, false},
		{"/path/to/model.pth", FormatHF, false},
		{"/path/to/model.safetensors", FormatHF, false},
		{"/path/to/model.bin", FormatGPTQ, false},
		{"/path/to/model.unknown", "", true},
	}

	for _, tc := range testCases {
		t.Run(filepath.Ext(tc.path), func(t *testing.T) {
			format, err := manager.detectModelFormat(tc.path)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, format)
			}
		})
	}
}

func TestLocalLLMManager_IsFormatCompatibleWithProvider(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	testCases := []struct {
		format     ModelFormat
		provider   string
		compatible bool
	}{
		// GGUF is universal
		{FormatGGUF, "llamacpp", true},
		{FormatGGUF, "ollama", true},
		{FormatGGUF, "vllm", true},
		{FormatGGUF, "localai", true},
		{FormatGGUF, "koboldai", true},
		{FormatGGUF, "gpt4all", true},

		// GPTQ support
		{FormatGPTQ, "vllm", true},
		{FormatGPTQ, "localai", true},
		{FormatGPTQ, "llamacpp", false},
		{FormatGPTQ, "ollama", false},

		// HuggingFace format
		{FormatHF, "vllm", true},
		{FormatHF, "localai", true},
		{FormatHF, "llamacpp", false},
		{FormatHF, "ollama", false},

		// AWQ format
		{FormatAWQ, "vllm", true},
		{FormatAWQ, "localai", true},
		{FormatAWQ, "llamacpp", false},

		// FP16/BF16
		{FormatFP16, "vllm", true},
		{FormatBF16, "vllm", true},
		{FormatFP16, "mistralrs", true},
		{FormatBF16, "mistralrs", true},

		// Unknown provider should default to GGUF only
		{FormatGGUF, "unknown-provider", true},
		{FormatGPTQ, "unknown-provider", false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.format)+"-"+tc.provider, func(t *testing.T) {
			result := manager.isFormatCompatibleWithProvider(tc.format, tc.provider)
			assert.Equal(t, tc.compatible, result)
		})
	}
}

func TestLocalLLMManager_FindMostCompatibleFormat(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)
	manager.SetSkipProviderInstall(true)

	ctx := context.Background()
	manager.Initialize(ctx)

	// GGUF should be the most compatible format
	format := manager.findMostCompatibleFormat(FormatGPTQ)
	assert.Equal(t, FormatGGUF, format)
}

func TestLocalLLMManager_GetOptimalFormatForProvider(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	testCases := []struct {
		provider       string
		expectedFormat ModelFormat
	}{
		{"llamacpp", FormatGGUF},
		{"vllm", FormatGGUF},
		{"ollama", FormatGGUF},
		{"localai", FormatGGUF},
		{"mistralrs", FormatGGUF},
		{"unknown", FormatGGUF},
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			format := manager.getOptimalFormatForProvider(tc.provider)
			assert.Equal(t, tc.expectedFormat, format)
		})
	}
}

func TestLocalLLMManager_GetOptimizationTarget(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	testCases := []struct {
		provider string
		expected string
	}{
		{"vllm", "gpu"},
		{"llamacpp", "cpu"},
		{"mlx", "gpu"},
		{"mistralrs", "gpu"},
		{"unknown", "cpu"},
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			target := manager.getOptimizationTarget(tc.provider)
			assert.Equal(t, tc.expected, target)
		})
	}
}

func TestLocalLLMManager_GetTargetHardware(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	testCases := []struct {
		provider string
		expected string
	}{
		{"vllm", "nvidia"},
		{"mlx", "apple"},
		{"mistralrs", "nvidia"},
		{"llamacpp", "cpu"},
		{"unknown", "cpu"},
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			hardware := manager.getTargetHardware(tc.provider)
			assert.Equal(t, tc.expected, hardware)
		})
	}
}

func TestLocalLLMManager_CopyModel(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.gguf")
	content := []byte("test model content")
	err := os.WriteFile(srcPath, content, 0644)
	require.NoError(t, err)

	// Copy file
	dstPath := filepath.Join(tempDir, "dest.gguf")
	err = manager.copyModel(srcPath, dstPath)
	require.NoError(t, err)

	// Verify copy
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, content, dstContent)
}

func TestLocalLLMManager_CopyModel_SourceNotFound(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	srcPath := filepath.Join(tempDir, "nonexistent.gguf")
	dstPath := filepath.Join(tempDir, "dest.gguf")

	err := manager.copyModel(srcPath, dstPath)
	assert.Error(t, err)
}

func TestLocalLLMManager_GetSharedModels(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)
	manager.SetSkipProviderInstall(true)

	ctx := context.Background()
	manager.Initialize(ctx)

	// Create models directory for one provider and add a model file
	for _, provider := range manager.providers {
		modelsDir := filepath.Join(provider.DataPath, "models")
		os.MkdirAll(modelsDir, 0755)

		// Create a test model file
		modelPath := filepath.Join(modelsDir, "test-model.gguf")
		os.WriteFile(modelPath, []byte("test"), 0644)
		break // Only do it for one provider
	}

	shared, err := manager.GetSharedModels(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, shared)
}

func TestLocalLLMManager_ShareModelWithProviders(t *testing.T) {
	t.Run("NoCompatibleProviders", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		manager.SetSkipProviderInstall(true)

		ctx := context.Background()
		manager.Initialize(ctx)

		// Create a model with unknown extension
		modelPath := filepath.Join(tempDir, "model.unknown")
		os.WriteFile(modelPath, []byte("test"), 0644)

		err := manager.ShareModelWithProviders(ctx, modelPath, "test-model")
		assert.Error(t, err)
	})

	t.Run("ShareGGUFModel", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)
		manager.SetSkipProviderInstall(true)

		ctx := context.Background()
		manager.Initialize(ctx)

		// Create a GGUF model
		modelPath := filepath.Join(tempDir, "model.gguf")
		os.WriteFile(modelPath, []byte("test model content"), 0644)

		err := manager.ShareModelWithProviders(ctx, modelPath, "test-model")
		require.NoError(t, err)
	})
}

func TestLocalLLMManager_OptimizeModelForProvider(t *testing.T) {
	t.Run("ProviderNotFound", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewLocalLLMManager(tempDir)

		ctx := context.Background()
		err := manager.OptimizeModelForProvider(ctx, "/path/to/model.gguf", "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestLocalLLMManager_Cleanup(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)
	manager.SetSkipProviderInstall(true)

	ctx := context.Background()
	manager.Initialize(ctx)

	err := manager.Cleanup(ctx)
	require.NoError(t, err)
}

func TestLocalLLMProvider_Struct(t *testing.T) {
	provider := &LocalLLMProvider{
		Name:         "Test Provider",
		Repository:   "https://github.com/test/test.git",
		Version:      "main",
		Description:  "Test description",
		DefaultPort:  8080,
		BinaryPath:   "/path/to/binary",
		ConfigPath:   "/path/to/config",
		DataPath:     "/path/to/data",
		Status:       "installed",
		HealthURL:    "http://localhost:8080/health",
		Dependencies: []string{"git", "python3"},
		BuildScript:  "make build",
		StartupCmd:   []string{"./server"},
		Environment: map[string]string{
			"HOST": "127.0.0.1",
			"PORT": "8080",
		},
		LastCheck: time.Now(),
	}

	assert.Equal(t, "Test Provider", provider.Name)
	assert.Equal(t, "https://github.com/test/test.git", provider.Repository)
	assert.Equal(t, "main", provider.Version)
	assert.Equal(t, 8080, provider.DefaultPort)
	assert.Equal(t, "installed", provider.Status)
	assert.Len(t, provider.Dependencies, 2)
	assert.NotEmpty(t, provider.Environment)
	assert.False(t, provider.LastCheck.IsZero())
}

func TestProviderDefinitions(t *testing.T) {
	// Verify all expected providers are defined
	expectedProviders := []string{
		"vllm",
		"localai",
		"fastchat",
		"textgen",
		"lmstudio",
		"jan",
		"koboldai",
		"gpt4all",
		"tabbyapi",
		"mlx",
		"mistralrs",
	}

	for _, name := range expectedProviders {
		t.Run(name, func(t *testing.T) {
			provider, exists := providerDefinitions[name]
			assert.True(t, exists, "Provider %s should exist", name)
			if exists {
				assert.NotEmpty(t, provider.Name)
				assert.NotEmpty(t, provider.Repository)
				assert.Greater(t, provider.DefaultPort, 0)
				assert.NotEmpty(t, provider.Dependencies)
			}
		})
	}
}

func TestLocalLLMManager_CreateStartupScript(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)
	err := manager.createDirectories()
	require.NoError(t, err)

	provider := &LocalLLMProvider{
		Name:        "TestProvider",
		DefaultPort: 8080,
		StartupCmd:  []string{"python3", "-m", "server"},
		Environment: map[string]string{
			"HOST": "127.0.0.1",
			"PORT": "8080",
		},
	}

	err = manager.createStartupScript(provider)
	require.NoError(t, err)

	// Check script exists
	scriptPath := filepath.Join(manager.binaryDir, "testprovider.sh")
	_, err = os.Stat(scriptPath)
	assert.NoError(t, err)

	// Read and verify script content
	content, err := os.ReadFile(scriptPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "#!/bin/bash")
	assert.Contains(t, string(content), "HOST")
	assert.Contains(t, string(content), "PORT")
	assert.Contains(t, string(content), "python3 -m server")
}

func TestLocalLLMManager_IsProviderHealthy(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewLocalLLMManager(tempDir)

	provider := &LocalLLMProvider{
		Name:        "Test",
		DefaultPort: 99999, // Non-existent port
	}

	ctx := context.Background()
	// This should return false since no server is running
	healthy := manager.isProviderHealthy(ctx, provider)
	assert.False(t, healthy)
}
