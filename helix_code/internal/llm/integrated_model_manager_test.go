package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIntegratedModelManager(t *testing.T) {
	t.Run("CreatesNewManager", func(t *testing.T) {
		tempDir := t.TempDir()

		manager := NewIntegratedModelManager(tempDir)
		require.NotNil(t, manager)
		assert.Equal(t, tempDir, manager.baseDir)
		assert.NotNil(t, manager.downloadManager)
		assert.NotNil(t, manager.converter)
		assert.NotNil(t, manager.registry)
		assert.NotNil(t, manager.localLLMManager)
		assert.NotNil(t, manager.hardwareDetector)
		assert.NotNil(t, manager.downloadEvents)
		assert.NotNil(t, manager.conversionEvents)
		assert.NotNil(t, manager.activeDownloads)
		assert.NotNil(t, manager.activeConversions)
	})
}

func TestIntegratedModelManager_validateRequest(t *testing.T) {
	t.Run("ValidRequest", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		// Add a provider to the registry
		manager.registry.providers["ollama"] = &ProviderInfo{
			Name: "Ollama",
			Type: "openai-compatible",
		}

		req := IntegratedModelRequest{
			ModelID:        "llama-7b",
			TargetProvider: "ollama",
		}

		err := manager.validateRequest(req)
		assert.NoError(t, err)
	})

	t.Run("MissingModelID", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		req := IntegratedModelRequest{
			TargetProvider: "ollama",
		}

		err := manager.validateRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model_id is required")
	})

	t.Run("MissingTargetProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		req := IntegratedModelRequest{
			ModelID: "llama-7b",
		}

		err := manager.validateRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target_provider is required")
	})

	t.Run("UnknownProvider", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)
		// Clear providers
		manager.registry.providers = make(map[string]*ProviderInfo)

		req := IntegratedModelRequest{
			ModelID:        "llama-7b",
			TargetProvider: "unknown-provider",
		}

		err := manager.validateRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
	})
}

func TestIntegratedModelManager_detectModelFormat(t *testing.T) {
	t.Run("GGUFFormat", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.detectModelFormat("/models/model.gguf")
		require.NoError(t, err)
		assert.Equal(t, FormatGGUF, format)
	})

	t.Run("SafetensorsFormat", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.detectModelFormat("/models/model.safetensors")
		require.NoError(t, err)
		assert.Equal(t, FormatHF, format)
	})

	t.Run("BinFormat", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.detectModelFormat("/models/model.bin")
		require.NoError(t, err)
		assert.Equal(t, FormatHF, format)
	})

	t.Run("UnknownFormat", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.detectModelFormat("/models/model.unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown model format")
		assert.Empty(t, format)
	})
}

func TestIntegratedModelManager_determineOptimalFormat(t *testing.T) {
	t.Run("OptimizeForMemory", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.determineOptimalFormat(FormatHF, "memory", nil)
		require.NoError(t, err)
		assert.Equal(t, FormatGGUF, format)
	})

	t.Run("OptimizeForPerformance", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.determineOptimalFormat(FormatHF, "performance", nil)
		require.NoError(t, err)
		assert.Equal(t, FormatGPTQ, format)
	})

	t.Run("OptimizeForCompatibility", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.determineOptimalFormat(FormatHF, "compatibility", nil)
		require.NoError(t, err)
		assert.Equal(t, FormatGGUF, format)
	})

	t.Run("DefaultKeepsSource", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		format, err := manager.determineOptimalFormat(FormatAWQ, "other", nil)
		require.NoError(t, err)
		assert.Equal(t, FormatAWQ, format)
	})
}

func TestIntegratedModelManager_modelMatchesCriteria(t *testing.T) {
	t.Run("MatchesWithNoCriteria", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:          "test-model",
			ContextSize: 4096,
		}

		criteria := ModelSelectionCriteria{}

		matches := manager.modelMatchesCriteria(model, criteria)
		assert.True(t, matches)
	})

	t.Run("MatchesContextSize", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:          "test-model",
			ContextSize: 8192,
		}

		criteria := ModelSelectionCriteria{
			MaxTokens: 4096,
		}

		matches := manager.modelMatchesCriteria(model, criteria)
		assert.True(t, matches)
	})

	t.Run("FailsContextSizeCriteria", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:          "test-model",
			ContextSize: 2048,
		}

		criteria := ModelSelectionCriteria{
			MaxTokens: 4096,
		}

		matches := manager.modelMatchesCriteria(model, criteria)
		assert.False(t, matches)
	})
}

func TestIntegratedModelManager_calculateModelScore(t *testing.T) {
	t.Run("QualityPreference70B", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:        "llama-70b",
			ModelSize: "70B",
		}

		criteria := ModelSelectionCriteria{
			QualityPreference: "quality",
		}

		score := manager.calculateModelScore(model, criteria)
		assert.Equal(t, 1.3, score)
	})

	t.Run("QualityPreference34B", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:        "llama-34b",
			ModelSize: "34B",
		}

		criteria := ModelSelectionCriteria{
			QualityPreference: "quality",
		}

		score := manager.calculateModelScore(model, criteria)
		assert.Equal(t, 1.2, score)
	})

	t.Run("FastPreference7B", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:        "llama-7b",
			ModelSize: "7B",
		}

		criteria := ModelSelectionCriteria{
			QualityPreference: "fast",
		}

		score := manager.calculateModelScore(model, criteria)
		assert.Equal(t, 1.2, score)
	})

	t.Run("DefaultScore", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:        "llama-13b",
			ModelSize: "13B",
		}

		criteria := ModelSelectionCriteria{}

		score := manager.calculateModelScore(model, criteria)
		assert.Equal(t, 1.0, score)
	})
}

func TestIntegratedModelManager_findCompatibleProviders(t *testing.T) {
	// Round-36 §11.4 anti-bluff: the previous assertion contained
	// "localai", a provider never initialised by the registry. The old
	// findCompatibleProviders body returned a hardcoded slice that
	// included it regardless of inputs — testing the bluff rather than
	// the contract. The post-fix implementation derives the result from
	// the registry's actual provider+format compatibility map, so this
	// test now asserts the providers that DO declare GGUF support
	// (vllm, llamacpp, ollama per initializeDefaultProviders) and
	// asserts that an unsupported format produces an EMPTY slice — the
	// canonical anti-bluff: "filter actually filters".
	t.Run("ReturnsGGUFCompatibleProviders", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		providers := manager.findCompatibleProviders("test-model", FormatGGUF)
		assert.Contains(t, providers, "vllm")
		assert.Contains(t, providers, "llamacpp")
		assert.Contains(t, providers, "ollama")
		// Round-36: do NOT assert "localai" — it was never a registered
		// provider; the old assertion was testing a discarded-parameter
		// bluff (the hardcoded return value), not the actual contract.
	})

	t.Run("FormatFilterActuallyFilters", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		// llamacpp and ollama declare ONLY GGUF; vllm declares
		// GPTQ/AWQ/HF/etc. So a format-specific query MUST shrink the
		// candidate set.
		providers := manager.findCompatibleProviders("test-model", FormatAWQ)
		assert.NotContains(t, providers, "llamacpp",
			"llamacpp does not declare AWQ support; presence here means format filter is being discarded")
		assert.NotContains(t, providers, "ollama",
			"ollama does not declare AWQ support; presence here means format filter is being discarded")
	})
}

// TestIntegratedModelManager_modelMatchesCriteria_TagsFilter is the
// round-36 regression test that catches the original A1-CRITICAL bluff
// where modelMatchesCriteria silently passed RequiredCapabilities + TaskType
// through empty if-branches. With the bluff in place, all three sub-tests
// here return TRUE (false-pass). With the fix in place, the tag-mismatched
// cases return FALSE — proving the criteria are honored.
func TestIntegratedModelManager_modelMatchesCriteria_TagsFilter(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewIntegratedModelManager(tempDir)

	codeModel := &DownloadableModelInfo{
		ID:          "code-llama-7b",
		ContextSize: 16384,
		Tags:        []string{"instruct", "code", "programming"},
	}
	chatModel := &DownloadableModelInfo{
		ID:          "general-chat-7b",
		ContextSize: 8192,
		Tags:        []string{"instruct", "chat", "general"},
	}

	t.Run("CodeCriteriaAcceptsCodeModel", func(t *testing.T) {
		criteria := ModelSelectionCriteria{
			RequiredCapabilities: []ModelCapability{CapabilityCodeGeneration},
		}
		assert.True(t, manager.modelMatchesCriteria(codeModel, criteria))
	})

	t.Run("CodeCriteriaRejectsChatModel", func(t *testing.T) {
		// Pre-fix: chatModel would have "matched" because the
		// RequiredCapabilities branch was empty. Post-fix: rejected
		// because "chat"/"general"/"instruct" tags do not satisfy
		// CapabilityCodeGeneration's keyword set.
		criteria := ModelSelectionCriteria{
			RequiredCapabilities: []ModelCapability{CapabilityCodeGeneration},
		}
		assert.False(t, manager.modelMatchesCriteria(chatModel, criteria),
			"chat model lacks code-related tags; must NOT match a code-generation criteria — A1 anti-bluff regression")
	})

	t.Run("TaskTypeDebuggingRejectsChatModel", func(t *testing.T) {
		criteria := ModelSelectionCriteria{
			TaskType: "debugging",
		}
		assert.False(t, manager.modelMatchesCriteria(chatModel, criteria),
			"chat model lacks debugging-related tags; must NOT match — A1 anti-bluff regression")
	})
}

// TestIntegratedModelManager_calculateModelScore_HonorsCapabilities is the
// round-36 regression test for the calculateModelScore A1 fix. Pre-fix,
// the function read only QualityPreference. Post-fix, RequiredCapabilities
// and MaxTokens influence the score. The test asserts the post-fix
// score is STRICTLY GREATER when a code model satisfies a code criteria,
// vs. a flat baseline — locking in that the additional inputs MOVE the
// score.
func TestIntegratedModelManager_calculateModelScore_HonorsCapabilities(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewIntegratedModelManager(tempDir)

	model := &DownloadableModelInfo{
		ID:          "code-llama-13b",
		ModelSize:   "13B",
		ContextSize: 16384,
		Tags:        []string{"instruct", "code", "programming"},
	}

	baseline := manager.calculateModelScore(model, ModelSelectionCriteria{})
	withCaps := manager.calculateModelScore(model, ModelSelectionCriteria{
		RequiredCapabilities: []ModelCapability{CapabilityCodeGeneration},
	})

	assert.Greater(t, withCaps, baseline,
		"RequiredCapabilities must increase the score when satisfied; equality means the parameter is being discarded — A1 anti-bluff regression")
}

// TestIntegratedModelManager_determineOptimalFormat_HonorsConstraints is the
// round-36 regression test for the determineOptimalFormat A1 fix.
// Pre-fix, constraints was discarded; cpu_only=true + optimize_for="performance"
// returned FormatGPTQ (GPU-only). Post-fix, cpu_only wins.
func TestIntegratedModelManager_determineOptimalFormat_HonorsConstraints(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewIntegratedModelManager(tempDir)

	t.Run("CPUOnlyOverridesPerformanceHint", func(t *testing.T) {
		constraints := map[string]interface{}{"cpu_only": true}
		format, err := manager.determineOptimalFormat(FormatHF, "performance", constraints)
		require.NoError(t, err)
		assert.Equal(t, FormatGGUF, format,
			"cpu_only constraint must override the performance hint; GPTQ means constraints are being discarded — A1 anti-bluff regression")
	})

	t.Run("PreferredFormatOverridesHint", func(t *testing.T) {
		constraints := map[string]interface{}{"preferred_format": string(FormatAWQ)}
		format, err := manager.determineOptimalFormat(FormatHF, "memory", constraints)
		require.NoError(t, err)
		assert.Equal(t, FormatAWQ, format,
			"preferred_format constraint must win; GGUF means constraints are being discarded")
	})
}

func TestIntegratedModelManager_getModelPath(t *testing.T) {
	t.Run("GeneratesCorrectPath", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		path := manager.getModelPath("ollama", "llama-7b", FormatGGUF)
		assert.Contains(t, path, tempDir)
		assert.Contains(t, path, "ollama")
		assert.Contains(t, path, "llama-7b")
		assert.Contains(t, path, "model.gguf")
	})
}

func TestIntegratedModelManager_GetModelStatus(t *testing.T) {
	t.Run("ModelNotDownloaded", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		status, err := manager.GetModelStatus("non-existent-model")
		require.NoError(t, err)
		assert.Equal(t, "non-existent-model", status.ModelID)
		assert.False(t, status.Available)
	})

	t.Run("ModelDownloaded", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		// Register a downloaded model
		model := &DownloadedModel{
			ModelID:             "llama-7b",
			Provider:            "ollama",
			Format:              FormatGGUF,
			Path:                "/models/llama-7b.gguf",
			Size:                4000000000,
			DownloadTime:        time.Now(),
			LastUsed:            time.Now(),
			UseCount:            5,
			CompatibleProviders: []string{"ollama", "llamacpp"},
		}
		err := manager.registry.RegisterDownloadedModel(model)
		require.NoError(t, err)

		status, err := manager.GetModelStatus("llama-7b")
		require.NoError(t, err)
		assert.Equal(t, "llama-7b", status.ModelID)
		assert.True(t, status.Available)
		assert.Equal(t, "ollama", status.Provider)
		assert.Equal(t, FormatGGUF, status.Format)
		assert.Equal(t, "/models/llama-7b.gguf", status.Path)
		assert.Equal(t, int64(4000000000), status.Size)
		assert.Equal(t, 5, status.UseCount)
		assert.Contains(t, status.CompatibleProviders, "ollama")
	})
}

func TestIntegratedModelManager_ListAvailableModels(t *testing.T) {
	t.Run("ReturnsEmptyListInitially", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		models, err := manager.ListAvailableModels()
		require.NoError(t, err)
		// May return empty or populated based on default models
		assert.NotNil(t, models)
	})
}

func TestIntegratedModelManager_AcquireModel(t *testing.T) {
	t.Run("InvalidRequest", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		req := IntegratedModelRequest{
			// Missing ModelID
			TargetProvider: "ollama",
		}

		statusChan, err := manager.AcquireModel(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, statusChan)
		assert.Contains(t, err.Error(), "model_id is required")
	})
}

func TestIntegratedModelManager_OptimizeModel(t *testing.T) {
	t.Run("StartsOptimization", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		statusChan, err := manager.OptimizeModel(ctx, "/models/test.gguf", "memory", nil)
		require.NoError(t, err)
		require.NotNil(t, statusChan)

		// Should receive at least one status update
		select {
		case status := <-statusChan:
			assert.NotEmpty(t, status.OperationID)
			assert.Equal(t, "optimization", status.Type)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for status update")
		}
	})
}

func TestIntegratedModelManager_FindBestModel(t *testing.T) {
	t.Run("NoSuitableModel", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		criteria := ModelSelectionCriteria{
			MaxTokens: 1000000, // Very large requirement
		}

		result, err := manager.FindBestModel(criteria)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no suitable model found")
	})
}

func TestIntegratedModelRequest_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		req := IntegratedModelRequest{
			ModelID:         "llama-7b",
			TargetProvider:  "ollama",
			TargetFormat:    FormatGGUF,
			SourceProvider:  "huggingface",
			ForceDownload:   true,
			ConvertIfNeeded: true,
			OptimizeFor:     "memory",
			Constraints: map[string]interface{}{
				"max_memory": "8GB",
			},
			AutoStart: true,
		}

		assert.Equal(t, "llama-7b", req.ModelID)
		assert.Equal(t, "ollama", req.TargetProvider)
		assert.Equal(t, FormatGGUF, req.TargetFormat)
		assert.Equal(t, "huggingface", req.SourceProvider)
		assert.True(t, req.ForceDownload)
		assert.True(t, req.ConvertIfNeeded)
		assert.Equal(t, "memory", req.OptimizeFor)
		assert.NotNil(t, req.Constraints)
		assert.True(t, req.AutoStart)
	})
}

func TestIntegratedModelResult_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		result := IntegratedModelResult{
			Success:             true,
			ModelID:             "llama-7b",
			Provider:            "ollama",
			Format:              FormatGGUF,
			Path:                "/models/llama-7b.gguf",
			Converted:           true,
			DownloadTime:        30 * time.Second,
			ConversionTime:      60 * time.Second,
			TotalTime:           90 * time.Second,
			Size:                4000000000,
			CompatibleProviders: []string{"ollama", "llamacpp"},
			Warnings:            []string{"Large model"},
			Recommendations:     []string{"Use GPU"},
			Error:               "",
		}

		assert.True(t, result.Success)
		assert.Equal(t, "llama-7b", result.ModelID)
		assert.Equal(t, "ollama", result.Provider)
		assert.Equal(t, FormatGGUF, result.Format)
		assert.Equal(t, "/models/llama-7b.gguf", result.Path)
		assert.True(t, result.Converted)
		assert.Equal(t, 30*time.Second, result.DownloadTime)
		assert.Equal(t, 60*time.Second, result.ConversionTime)
		assert.Equal(t, 90*time.Second, result.TotalTime)
		assert.Equal(t, int64(4000000000), result.Size)
		assert.Len(t, result.CompatibleProviders, 2)
		assert.Len(t, result.Warnings, 1)
		assert.Len(t, result.Recommendations, 1)
		assert.Empty(t, result.Error)
	})
}

func TestModelOperationStatus_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		startTime := time.Now()
		status := ModelOperationStatus{
			OperationID:  "op_12345",
			Type:         "download",
			ModelID:      "llama-7b",
			Progress:     0.5,
			Status:       "in_progress",
			StartTime:    startTime,
			EstimatedETA: 120,
			CurrentStep:  "downloading weights",
			Error:        "",
		}

		assert.Equal(t, "op_12345", status.OperationID)
		assert.Equal(t, "download", status.Type)
		assert.Equal(t, "llama-7b", status.ModelID)
		assert.Equal(t, 0.5, status.Progress)
		assert.Equal(t, "in_progress", status.Status)
		assert.Equal(t, startTime, status.StartTime)
		assert.Equal(t, int64(120), status.EstimatedETA)
		assert.Equal(t, "downloading weights", status.CurrentStep)
		assert.Empty(t, status.Error)
	})
}

func TestModelStatus_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		downloadTime := time.Now().Add(-24 * time.Hour)
		lastUsed := time.Now()

		status := ModelStatus{
			ModelID:             "llama-7b",
			Name:                "LLaMA 7B",
			Description:         "7 billion parameter model",
			Available:           true,
			Provider:            "ollama",
			Format:              FormatGGUF,
			Path:                "/models/llama-7b.gguf",
			Size:                4000000000,
			DownloadTime:        downloadTime,
			LastUsed:            lastUsed,
			UseCount:            10,
			AvailableFormats:    []ModelFormat{FormatGGUF, FormatGPTQ},
			CompatibleProviders: []string{"ollama", "llamacpp"},
			ModelSize:           "7B",
			ContextSize:         4096,
			Requirements: ModelRequirements{
				MinRAM:  "8GB",
				MinVRAM: "4GB",
			},
		}

		assert.Equal(t, "llama-7b", status.ModelID)
		assert.Equal(t, "LLaMA 7B", status.Name)
		assert.Equal(t, "7 billion parameter model", status.Description)
		assert.True(t, status.Available)
		assert.Equal(t, "ollama", status.Provider)
		assert.Equal(t, FormatGGUF, status.Format)
		assert.Equal(t, "/models/llama-7b.gguf", status.Path)
		assert.Equal(t, int64(4000000000), status.Size)
		assert.Equal(t, downloadTime, status.DownloadTime)
		assert.Equal(t, lastUsed, status.LastUsed)
		assert.Equal(t, 10, status.UseCount)
		assert.Len(t, status.AvailableFormats, 2)
		assert.Len(t, status.CompatibleProviders, 2)
		assert.Equal(t, "7B", status.ModelSize)
		assert.Equal(t, 4096, status.ContextSize)
		assert.Equal(t, "8GB", status.Requirements.MinRAM)
	})
}

func TestIntegratedModelInfo_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		info := IntegratedModelInfo{
			DownloadableModelInfo: DownloadableModelInfo{
				ID:          "llama-7b",
				Name:        "LLaMA 7B",
				Description: "7 billion parameter model",
				ModelSize:   "7B",
				ContextSize: 4096,
			},
			Downloaded:       true,
			DownloadedPath:   "/models/llama-7b.gguf",
			DownloadedFormat: FormatGGUF,
			Providers:        []string{"ollama", "llamacpp"},
		}

		assert.Equal(t, "llama-7b", info.ID)
		assert.Equal(t, "LLaMA 7B", info.Name)
		assert.True(t, info.Downloaded)
		assert.Equal(t, "/models/llama-7b.gguf", info.DownloadedPath)
		assert.Equal(t, FormatGGUF, info.DownloadedFormat)
		assert.Len(t, info.Providers, 2)
	})
}

func TestIntegratedModelManager_scoreModelForHardware(t *testing.T) {
	t.Run("WithoutGPU", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewIntegratedModelManager(tempDir)

		model := &DownloadableModelInfo{
			ID:        "llama-7b",
			ModelSize: "7B",
		}

		// Detect actual hardware
		hwInfo, err := manager.hardwareDetector.Detect()
		require.NoError(t, err)

		// Test internal scoring
		score, provider, format := manager.scoreModelForHardware(model, hwInfo)
		// Hardware detection should work and provide valid results
		assert.Greater(t, score, 0.0)
		assert.NotEmpty(t, provider)
		assert.NotEmpty(t, format)
	})
}
