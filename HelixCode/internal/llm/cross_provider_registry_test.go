package llm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCrossProviderRegistry(t *testing.T) {
	t.Run("CreatesNewRegistry", func(t *testing.T) {
		tempDir := t.TempDir()

		registry := NewCrossProviderRegistry(tempDir)
		require.NotNil(t, registry)
		assert.Equal(t, tempDir, registry.baseDir)
		assert.NotNil(t, registry.compatibility)
		assert.NotNil(t, registry.providers)
		assert.NotNil(t, registry.downloadedModels)
	})

	t.Run("InitializesDefaultProviders", func(t *testing.T) {
		tempDir := t.TempDir()

		registry := NewCrossProviderRegistry(tempDir)
		require.NotNil(t, registry)

		// Should have default providers initialized
		assert.NotEmpty(t, registry.compatibility)
	})

	t.Run("CreatesRegistryDirectory", func(t *testing.T) {
		tempDir := t.TempDir()
		registryDir := filepath.Join(tempDir, "subdir", "registry")

		registry := NewCrossProviderRegistry(registryDir)
		require.NotNil(t, registry)

		// Directory should exist
		_, err := os.Stat(registryDir)
		assert.NoError(t, err)
	})
}

func TestCrossProviderRegistry_GetCompatibleFormats(t *testing.T) {
	t.Run("ReturnsFormatsForKnownProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Add a test provider
		registry.compatibility["ollama"] = &ProviderCompatibility{
			Provider:         "ollama",
			SupportedFormats: []ModelFormat{FormatGGUF, FormatGPTQ},
			PreferredFormats: []ModelFormat{FormatGGUF},
		}

		formats, err := registry.GetCompatibleFormats("ollama")
		require.NoError(t, err)
		assert.Contains(t, formats, FormatGGUF)
		assert.Contains(t, formats, FormatGPTQ)
	})

	t.Run("ErrorsForUnknownProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		formats, err := registry.GetCompatibleFormats("unknown-provider")
		assert.Error(t, err)
		assert.Nil(t, formats)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestCrossProviderRegistry_CheckCompatibility(t *testing.T) {
	t.Run("PreferredFormat", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Add a test provider
		registry.compatibility["ollama"] = &ProviderCompatibility{
			Provider:         "ollama",
			SupportedFormats: []ModelFormat{FormatGGUF, FormatGPTQ},
			PreferredFormats: []ModelFormat{FormatGGUF},
		}

		query := ModelCompatibilityQuery{
			ModelID:        "llama-7b",
			SourceFormat:   FormatGGUF,
			TargetProvider: "ollama",
			TargetFormat:   FormatGGUF,
		}

		result, err := registry.CheckCompatibility(query)
		require.NoError(t, err)
		assert.True(t, result.IsCompatible)
		assert.Equal(t, 1.0, result.Confidence)
		assert.False(t, result.ConversionRequired)
	})

	t.Run("SupportedButNotPreferred", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		registry.compatibility["ollama"] = &ProviderCompatibility{
			Provider:         "ollama",
			SupportedFormats: []ModelFormat{FormatGGUF, FormatGPTQ},
			PreferredFormats: []ModelFormat{FormatGGUF},
		}

		query := ModelCompatibilityQuery{
			ModelID:        "llama-7b",
			SourceFormat:   FormatGPTQ,
			TargetProvider: "ollama",
			TargetFormat:   FormatGPTQ,
		}

		result, err := registry.CheckCompatibility(query)
		require.NoError(t, err)
		assert.True(t, result.IsCompatible)
		assert.GreaterOrEqual(t, result.Confidence, 0.8)
	})

	t.Run("UnknownProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		query := ModelCompatibilityQuery{
			ModelID:        "llama-7b",
			SourceFormat:   FormatGGUF,
			TargetProvider: "unknown-provider",
		}

		result, err := registry.CheckCompatibility(query)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsCompatible)
	})
}

func TestCrossProviderRegistry_RegisterDownloadedModel(t *testing.T) {
	t.Run("RegistersModel", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		model := &DownloadedModel{
			ModelID:      "llama-7b",
			Provider:     "huggingface",
			Format:       FormatGGUF,
			Path:         "/models/llama-7b.gguf",
			Size:         4000000000,
			Checksum:     "abc123",
			DownloadTime: time.Now(),
			LastUsed:     time.Now(),
			UseCount:     0,
			Tags:         []string{"llama", "7b"},
			Metadata:     map[string]string{"version": "1.0"},
		}

		err := registry.RegisterDownloadedModel(model)
		require.NoError(t, err)

		// Verify model is registered
		models := registry.GetDownloadedModels()
		assert.Len(t, models, 1)
		assert.Equal(t, "llama-7b", models[0].ModelID)
	})

	t.Run("UpdatesCompatibleProviders", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		model := &DownloadedModel{
			ModelID:  "test-model",
			Provider: "test",
			Format:   FormatGGUF,
			Path:     "/models/test.gguf",
			Size:     1000000000,
		}

		err := registry.RegisterDownloadedModel(model)
		require.NoError(t, err)

		// CompatibleProviders should be populated
		models := registry.GetDownloadedModels()
		assert.Len(t, models, 1)
		// The model should have compatible providers set (might be empty if no matches)
		assert.NotNil(t, models[0].CompatibleProviders)
	})
}

func TestCrossProviderRegistry_GetDownloadedModels(t *testing.T) {
	t.Run("ReturnsEmptyListInitially", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		models := registry.GetDownloadedModels()
		assert.Empty(t, models)
	})

	t.Run("ReturnsAllModels", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Register multiple models
		for i := 0; i < 3; i++ {
			model := &DownloadedModel{
				ModelID:  "model-" + string(rune('a'+i)),
				Provider: "test",
				Format:   FormatGGUF,
				Path:     "/models/model.gguf",
				Size:     1000000000,
			}
			err := registry.RegisterDownloadedModel(model)
			require.NoError(t, err)
		}

		models := registry.GetDownloadedModels()
		assert.Len(t, models, 3)
	})
}

func TestCrossProviderRegistry_FindModelsForProvider(t *testing.T) {
	t.Run("FindsCompatibleModels", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Register a model with compatible providers
		model := &DownloadedModel{
			ModelID:             "llama-7b",
			Provider:            "huggingface",
			Format:              FormatGGUF,
			Path:                "/models/llama.gguf",
			Size:                4000000000,
			CompatibleProviders: []string{"ollama", "llamacpp"},
		}

		// Directly add to map to set CompatibleProviders manually
		registry.mu.Lock()
		registry.downloadedModels["huggingface:llama-7b:gguf"] = model
		registry.mu.Unlock()

		models, err := registry.FindModelsForProvider("ollama")
		require.NoError(t, err)
		assert.Len(t, models, 1)
		assert.Equal(t, "llama-7b", models[0].ModelID)
	})

	t.Run("ReturnsEmptyForNoMatches", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		model := &DownloadedModel{
			ModelID:             "test-model",
			Provider:            "test",
			Format:              FormatGGUF,
			Path:                "/models/test.gguf",
			Size:                1000000000,
			CompatibleProviders: []string{"other-provider"},
		}

		registry.mu.Lock()
		registry.downloadedModels["test:test-model:gguf"] = model
		registry.mu.Unlock()

		models, err := registry.FindModelsForProvider("ollama")
		require.NoError(t, err)
		assert.Empty(t, models)
	})
}

func TestCrossProviderRegistry_FindOptimalProvider(t *testing.T) {
	t.Run("FindsProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Add a provider with GGUF support
		registry.compatibility["ollama"] = &ProviderCompatibility{
			Provider:         "ollama",
			SupportedFormats: []ModelFormat{FormatGGUF},
			PreferredFormats: []ModelFormat{FormatGGUF},
			Performance: ProviderPerformance{
				Throughput:  "high",
				Latency:     "low",
				MemoryUsage: "medium",
			},
		}
		registry.providers["ollama"] = &ProviderInfo{
			Name:        "Ollama",
			Type:        "openai-compatible",
			DefaultPort: 11434,
		}

		provider, err := registry.FindOptimalProvider("llama-7b", FormatGGUF, nil)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		// A provider should be found (could be ollama or default VLLM)
		assert.NotEmpty(t, provider.Name)
	})

	t.Run("NoCompatibleProviders", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Clear default providers
		registry.compatibility = make(map[string]*ProviderCompatibility)
		registry.providers = make(map[string]*ProviderInfo)

		provider, err := registry.FindOptimalProvider("model", FormatGGUF, nil)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "no compatible providers")
	})
}

func TestCrossProviderRegistry_SaveAndLoad(t *testing.T) {
	t.Run("PersistsRegistry", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create registry and add a model
		registry1 := NewCrossProviderRegistry(tempDir)
		model := &DownloadedModel{
			ModelID:  "persist-test",
			Provider: "test",
			Format:   FormatGGUF,
			Path:     "/models/test.gguf",
			Size:     1000000000,
		}
		err := registry1.RegisterDownloadedModel(model)
		require.NoError(t, err)

		// Create new registry from same directory
		registry2 := NewCrossProviderRegistry(tempDir)

		// Should load the saved model
		models := registry2.GetDownloadedModels()
		assert.Len(t, models, 1)
		assert.Equal(t, "persist-test", models[0].ModelID)
	})
}

func TestProviderCompatibility(t *testing.T) {
	t.Run("StructFields", func(t *testing.T) {
		compat := &ProviderCompatibility{
			Provider:         "ollama",
			SupportedFormats: []ModelFormat{FormatGGUF},
			PreferredFormats: []ModelFormat{FormatGGUF},
			ConversionPaths:  map[string][]string{"safetensors": {"gguf"}},
			Requirements: ProviderRequirements{
				MinRAM:      "8GB",
				MinVRAM:     "4GB",
				GPURequired: false,
				CPUOnly:     true,
			},
			Performance: ProviderPerformance{
				Throughput:  "high",
				Latency:     "low",
				MemoryUsage: "medium",
				BatchSize:   1,
				Parallelism: 4,
			},
			LastUpdated: time.Now(),
		}

		assert.Equal(t, "ollama", compat.Provider)
		assert.Len(t, compat.SupportedFormats, 1)
		assert.Equal(t, "8GB", compat.Requirements.MinRAM)
		assert.Equal(t, "high", compat.Performance.Throughput)
	})
}

func TestProviderInfo(t *testing.T) {
	t.Run("StructFields", func(t *testing.T) {
		info := &ProviderInfo{
			Name:        "Ollama",
			Type:        "openai-compatible",
			Endpoint:    "http://localhost:11434",
			Version:     "0.1.0",
			Repository:  "https://github.com/ollama/ollama",
			Description: "Local LLM runner",
			Website:     "https://ollama.ai",
			DefaultPort: 11434,
			License:     "MIT",
			Tags:        []string{"local", "llm"},
		}

		assert.Equal(t, "Ollama", info.Name)
		assert.Equal(t, "openai-compatible", info.Type)
		assert.Equal(t, 11434, info.DefaultPort)
		assert.Contains(t, info.Tags, "local")
	})
}

func TestDownloadedModel(t *testing.T) {
	t.Run("StructFields", func(t *testing.T) {
		model := &DownloadedModel{
			ModelID:             "llama-7b-gguf",
			Provider:            "huggingface",
			Format:              FormatGGUF,
			Path:                "/models/llama-7b.gguf",
			Size:                4294967296,
			Checksum:            "sha256:abc123",
			DownloadTime:        time.Now(),
			LastUsed:            time.Now(),
			UseCount:            10,
			Tags:                []string{"llama", "7b", "gguf"},
			Metadata:            map[string]string{"quantization": "Q4_K_M"},
			CompatibleProviders: []string{"ollama", "llamacpp"},
		}

		assert.Equal(t, "llama-7b-gguf", model.ModelID)
		assert.Equal(t, FormatGGUF, model.Format)
		assert.Equal(t, int64(4294967296), model.Size)
		assert.Equal(t, 10, model.UseCount)
		assert.Len(t, model.CompatibleProviders, 2)
	})
}

func TestCompatibilityResult(t *testing.T) {
	t.Run("StructFields", func(t *testing.T) {
		result := &CompatibilityResult{
			IsCompatible:         true,
			Confidence:           0.95,
			ConversionRequired:   true,
			ConversionPath:       []string{"safetensors", "gguf"},
			EstimatedTime:        30,
			EstimatedSize:        4000000000,
			Warnings:             []string{"Large model size"},
			Recommendations:      []string{"Use GPU acceleration"},
			AlternativeProviders: []string{"vllm", "llamacpp"},
		}

		assert.True(t, result.IsCompatible)
		assert.Equal(t, 0.95, result.Confidence)
		assert.True(t, result.ConversionRequired)
		assert.Len(t, result.ConversionPath, 2)
		assert.Equal(t, int64(30), result.EstimatedTime)
		assert.Len(t, result.Warnings, 1)
		assert.Len(t, result.AlternativeProviders, 2)
	})
}

func TestModelCompatibilityQuery(t *testing.T) {
	t.Run("StructFields", func(t *testing.T) {
		query := ModelCompatibilityQuery{
			ModelID:        "llama-7b",
			SourceFormat:   FormatHF,
			TargetProvider: "ollama",
			TargetFormat:   FormatGGUF,
			Constraints: map[string]interface{}{
				"max_memory": "8GB",
				"gpu":        true,
			},
		}

		assert.Equal(t, "llama-7b", query.ModelID)
		assert.Equal(t, FormatHF, query.SourceFormat)
		assert.Equal(t, "ollama", query.TargetProvider)
		assert.Equal(t, FormatGGUF, query.TargetFormat)
		assert.NotNil(t, query.Constraints)
	})
}

func TestCrossProviderRegistry_findAlternativeProviders(t *testing.T) {
	t.Run("FindsProvidersWithMatchingFormat", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Add providers with GGUF support
		registry.compatibility["ollama"] = &ProviderCompatibility{
			Provider:         "ollama",
			SupportedFormats: []ModelFormat{FormatGGUF},
		}
		registry.compatibility["llamacpp"] = &ProviderCompatibility{
			Provider:         "llamacpp",
			SupportedFormats: []ModelFormat{FormatGGUF},
		}

		alternatives := registry.findAlternativeProviders("test-model", FormatGGUF)
		assert.Contains(t, alternatives, "ollama")
		assert.Contains(t, alternatives, "llamacpp")
	})

	t.Run("ReturnsEmptyForNoMatches", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Clear default providers
		registry.compatibility = make(map[string]*ProviderCompatibility)

		alternatives := registry.findAlternativeProviders("test-model", FormatGGUF)
		assert.Empty(t, alternatives)
	})
}

func TestCrossProviderRegistry_findCompatibleProvidersForModel(t *testing.T) {
	t.Run("FindsProvidersWithMatchingFormat", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Add providers with specific format support
		registry.compatibility["vllm"] = &ProviderCompatibility{
			Provider:         "vllm",
			SupportedFormats: []ModelFormat{FormatGPTQ, FormatAWQ},
		}
		registry.compatibility["ollama"] = &ProviderCompatibility{
			Provider:         "ollama",
			SupportedFormats: []ModelFormat{FormatGGUF},
		}

		compatible := registry.findCompatibleProvidersForModel("test-model", FormatGPTQ)
		assert.Contains(t, compatible, "vllm")
		assert.NotContains(t, compatible, "ollama")
	})
}

func TestCrossProviderRegistry_estimateConversionTime(t *testing.T) {
	t.Run("ReturnsEstimateForKnownFormats", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Estimate conversion from HF to GGUF
		estimate := registry.estimateConversionTime(FormatHF, FormatGGUF)
		assert.Greater(t, estimate, int64(0))
	})
}

func TestCrossProviderRegistry_GetProviderInfo(t *testing.T) {
	t.Run("ReturnsInfoForKnownProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)

		// Add a provider
		registry.providers["test-provider"] = &ProviderInfo{
			Name:        "Test Provider",
			Type:        "openai-compatible",
			DefaultPort: 8080,
		}

		info, err := registry.GetProviderInfo("test-provider")
		require.NoError(t, err)
		assert.Equal(t, "Test Provider", info.Name)
		assert.Equal(t, 8080, info.DefaultPort)
	})

	t.Run("ErrorsForUnknownProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		registry := NewCrossProviderRegistry(tempDir)
		registry.providers = make(map[string]*ProviderInfo)

		info, err := registry.GetProviderInfo("unknown")
		assert.Error(t, err)
		assert.Nil(t, info)
	})
}

