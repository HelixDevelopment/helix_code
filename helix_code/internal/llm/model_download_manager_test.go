package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModelDownloadManager(t *testing.T) {
	t.Run("CreatesManager", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewModelDownloadManager(tempDir)
		require.NotNil(t, manager)
		assert.Equal(t, tempDir, manager.baseDir)
		assert.NotNil(t, manager.httpClient)
		assert.NotNil(t, manager.availableModels)
		assert.NotNil(t, manager.sources)
		assert.NotNil(t, manager.conversionTools)
		assert.NotNil(t, manager.downloads)
	})

	t.Run("LoadsModelRegistry", func(t *testing.T) {
		tempDir := t.TempDir()
		manager := NewModelDownloadManager(tempDir)
		require.NotNil(t, manager)
		// Should have some default models loaded
		assert.NotEmpty(t, manager.availableModels)
	})
}

func TestModelFormat_Constants(t *testing.T) {
	assert.Equal(t, ModelFormat("gguf"), FormatGGUF)
	assert.Equal(t, ModelFormat("gptq"), FormatGPTQ)
	assert.Equal(t, ModelFormat("awq"), FormatAWQ)
	assert.Equal(t, ModelFormat("bf16"), FormatBF16)
	assert.Equal(t, ModelFormat("fp16"), FormatFP16)
	assert.Equal(t, ModelFormat("int8"), FormatINT8)
	assert.Equal(t, ModelFormat("int4"), FormatINT4)
	assert.Equal(t, ModelFormat("hf"), FormatHF)
}

func TestModelDownloadManager_GetAvailableModels(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	models := manager.GetAvailableModels()
	assert.NotEmpty(t, models)

	// Check that models have required fields
	for _, model := range models {
		assert.NotEmpty(t, model.ID)
		assert.NotEmpty(t, model.Name)
		assert.NotEmpty(t, model.AvailableFormats)
	}
}

func TestModelDownloadManager_SearchModels(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	t.Run("SearchByName", func(t *testing.T) {
		results := manager.SearchModels("llama")
		assert.NotEmpty(t, results)
		for _, model := range results {
			assert.Contains(t, model.Name+model.Description, "Llama")
		}
	})

	t.Run("SearchByTag", func(t *testing.T) {
		results := manager.SearchModels("code")
		assert.NotEmpty(t, results)
	})

	t.Run("SearchNoResults", func(t *testing.T) {
		results := manager.SearchModels("nonexistent-xyz-12345")
		assert.Empty(t, results)
	})
}

func TestModelDownloadManager_GetModelByID(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	t.Run("ExistingModel", func(t *testing.T) {
		model, err := manager.GetModelByID("llama-3-8b-instruct")
		require.NoError(t, err)
		assert.Equal(t, "llama-3-8b-instruct", model.ID)
		assert.Equal(t, "Llama 3 8B Instruct", model.Name)
	})

	t.Run("NonExistingModel", func(t *testing.T) {
		model, err := manager.GetModelByID("nonexistent-model")
		assert.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestModelDownloadManager_GetCompatibleFormats(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	t.Run("OllamaCompatibility", func(t *testing.T) {
		formats, err := manager.GetCompatibleFormats("ollama", "llama-3-8b-instruct")
		require.NoError(t, err)
		assert.Contains(t, formats, FormatGGUF)
	})

	t.Run("VLLMCompatibility", func(t *testing.T) {
		formats, err := manager.GetCompatibleFormats("vllm", "llama-3-8b-instruct")
		require.NoError(t, err)
		assert.Contains(t, formats, FormatGGUF)
		assert.Contains(t, formats, FormatGPTQ)
	})

	t.Run("NonExistingModel", func(t *testing.T) {
		formats, err := manager.GetCompatibleFormats("ollama", "nonexistent-model")
		assert.Error(t, err)
		assert.Nil(t, formats)
	})
}

func TestModelDownloadManager_GetProviderSupportedFormats(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	testCases := []struct {
		provider       string
		expectedFormat ModelFormat
	}{
		{"llamacpp", FormatGGUF},
		{"ollama", FormatGGUF},
		{"vllm", FormatGGUF},
		{"localai", FormatGGUF},
		{"textgen", FormatGGUF},
		{"mlx", FormatGGUF},
		{"mistralrs", FormatGGUF},
		{"koboldai", FormatGGUF},
		{"gpt4all", FormatGGUF},
		{"unknown-provider", FormatGGUF}, // Default
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			formats := manager.getProviderSupportedFormats(tc.provider)
			assert.Contains(t, formats, tc.expectedFormat)
		})
	}
}

func TestModelDownloadManager_IsFormatAvailableDirectly(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	model, _ := manager.GetModelByID("llama-3-8b-instruct")

	t.Run("AvailableFormat", func(t *testing.T) {
		available := manager.isFormatAvailableDirectly(model, FormatGGUF)
		assert.True(t, available)
	})

	t.Run("UnavailableFormat", func(t *testing.T) {
		available := manager.isFormatAvailableDirectly(model, FormatINT8)
		assert.False(t, available)
	})
}

func TestModelDownloadManager_GetModelPath(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	path := manager.getModelPath("ollama", "llama-7b", FormatGGUF)
	assert.Contains(t, path, tempDir)
	assert.Contains(t, path, "ollama")
	assert.Contains(t, path, "llama-7b")
	assert.Contains(t, path, "model.gguf")
}

func TestModelDownloadManager_GetDownloadURL(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	model, _ := manager.GetModelByID("llama-3-8b-instruct")

	t.Run("DirectURL", func(t *testing.T) {
		url, err := manager.getDownloadURL(model, FormatGGUF)
		require.NoError(t, err)
		assert.NotEmpty(t, url)
		assert.Contains(t, url, "huggingface")
	})

	t.Run("UnavailableFormat", func(t *testing.T) {
		url, err := manager.getDownloadURL(model, FormatINT8)
		assert.Error(t, err)
		assert.Empty(t, url)
	})
}

func TestModelDownloadManager_DownloadModel(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a model download
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake model content for testing purposes"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	// Add a test model with mock URL
	manager.availableModels["test-model"] = &DownloadableModelInfo{
		ID:               "test-model",
		Name:             "Test Model",
		AvailableFormats: []ModelFormat{FormatGGUF},
		DefaultFormat:    FormatGGUF,
		DownloadURLs: map[ModelFormat]string{
			FormatGGUF: server.URL + "/test-model.gguf",
		},
	}

	t.Run("FormatNotAvailable", func(t *testing.T) {
		req := ModelDownloadRequest{
			ModelID:        "test-model",
			Format:         FormatINT8, // Not available
			TargetProvider: "ollama",
		}

		ctx := context.Background()
		_, err := manager.DownloadModel(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no conversion path found")
	})

	t.Run("ModelNotFound", func(t *testing.T) {
		req := ModelDownloadRequest{
			ModelID:        "nonexistent-model",
			Format:         FormatGGUF,
			TargetProvider: "ollama",
		}

		ctx := context.Background()
		_, err := manager.DownloadModel(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestModelDownloadManager_DownloadFile(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := "test model content"
		w.Header().Set("Content-Length", "18")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	t.Run("SuccessfulDownload", func(t *testing.T) {
		targetPath := filepath.Join(tempDir, "test.gguf")
		progress := &ModelDownloadProgress{}

		ctx := context.Background()
		err := manager.downloadFile(ctx, server.URL+"/test.gguf", targetPath, progress)
		require.NoError(t, err)

		// Check file was created
		_, err = os.Stat(targetPath)
		assert.NoError(t, err)
	})

	t.Run("CancelledContext", func(t *testing.T) {
		targetPath := filepath.Join(tempDir, "cancelled.gguf")
		progress := &ModelDownloadProgress{}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := manager.downloadFile(ctx, server.URL+"/test.gguf", targetPath, progress)
		assert.Error(t, err)
	})

	t.Run("BadURL", func(t *testing.T) {
		targetPath := filepath.Join(tempDir, "bad.gguf")
		progress := &ModelDownloadProgress{}

		ctx := context.Background()
		err := manager.downloadFile(ctx, "http://localhost:99999/nonexistent", targetPath, progress)
		assert.Error(t, err)
	})
}

func TestDownloadableModelInfo_Struct(t *testing.T) {
	model := &DownloadableModelInfo{
		ID:               "test-model",
		Name:             "Test Model",
		Description:      "A test model",
		Provider:         "test-provider",
		AvailableFormats: []ModelFormat{FormatGGUF, FormatGPTQ},
		DefaultFormat:    FormatGGUF,
		Sources: []ModelDownloadSource{
			{Name: "HuggingFace", URL: "https://huggingface.co"},
		},
		ModelSize:   "7B",
		ContextSize: 4096,
		Requirements: ModelRequirements{
			MinRAM:      "8GB",
			MinVRAM:     "6GB",
			GPURequired: false,
			CPUOnly:     true,
		},
		DownloadURLs: map[ModelFormat]string{
			FormatGGUF: "https://example.com/model.gguf",
		},
		LastUpdated: time.Now(),
		Tags:        []string{"test", "model"},
	}

	assert.Equal(t, "test-model", model.ID)
	assert.Equal(t, "Test Model", model.Name)
	assert.Equal(t, "7B", model.ModelSize)
	assert.Equal(t, 4096, model.ContextSize)
	assert.Contains(t, model.AvailableFormats, FormatGGUF)
	assert.Contains(t, model.Tags, "test")
}

func TestModelDownloadSource_Struct(t *testing.T) {
	source := &ModelDownloadSource{
		Name:        "HuggingFace",
		URL:         "https://huggingface.co",
		Formats:     []ModelFormat{FormatGGUF, FormatGPTQ},
		Headers:     map[string]string{"Authorization": "Bearer token"},
		Description: "Main model hub",
		Priority:    1,
	}

	assert.Equal(t, "HuggingFace", source.Name)
	assert.Equal(t, "https://huggingface.co", source.URL)
	assert.Equal(t, 1, source.Priority)
	assert.Contains(t, source.Formats, FormatGGUF)
}

func TestModelRequirements_Struct(t *testing.T) {
	reqs := &ModelRequirements{
		MinRAM:          "8GB",
		MinVRAM:         "6GB",
		RecommendedVRAM: "12GB",
		SupportedOS:     []string{"linux", "darwin", "windows"},
		GPURequired:     false,
		CPUOnly:         true,
	}

	assert.Equal(t, "8GB", reqs.MinRAM)
	assert.Equal(t, "6GB", reqs.MinVRAM)
	assert.False(t, reqs.GPURequired)
	assert.True(t, reqs.CPUOnly)
	assert.Len(t, reqs.SupportedOS, 3)
}

func TestModelDownloadRequest_Struct(t *testing.T) {
	req := &ModelDownloadRequest{
		ModelID:        "llama-7b",
		Format:         FormatGGUF,
		TargetProvider: "ollama",
		TargetPath:     "/models/llama-7b.gguf",
		ForceDownload:  true,
	}

	assert.Equal(t, "llama-7b", req.ModelID)
	assert.Equal(t, FormatGGUF, req.Format)
	assert.Equal(t, "ollama", req.TargetProvider)
	assert.True(t, req.ForceDownload)
}

func TestModelDownloadProgress_Struct(t *testing.T) {
	progress := &ModelDownloadProgress{
		ModelID:   "llama-7b",
		Format:    FormatGGUF,
		Progress:  0.75,
		Speed:     5000000, // 5MB/s
		ETA:       60,      // 60 seconds
		StartTime: time.Now(),
		Error:     "",
	}

	assert.Equal(t, "llama-7b", progress.ModelID)
	assert.Equal(t, FormatGGUF, progress.Format)
	assert.Equal(t, 0.75, progress.Progress)
	assert.Equal(t, int64(5000000), progress.Speed)
	assert.Equal(t, int64(60), progress.ETA)
}

func TestConversionTool_Struct(t *testing.T) {
	tool := &ConversionTool{
		Name:          "llama.cpp",
		Command:       "python",
		Args:          []string{"-m", "llama_cpp.convert", "{input}", "{output}"},
		SourceFormats: []ModelFormat{FormatHF, FormatFP16},
		TargetFormat:  FormatGGUF,
		EnvVars: map[string]string{
			"HF_HUB_DISABLE_TELEMETRY": "1",
		},
	}

	assert.Equal(t, "llama.cpp", tool.Name)
	assert.Equal(t, "python", tool.Command)
	assert.Contains(t, tool.SourceFormats, FormatHF)
	assert.Equal(t, FormatGGUF, tool.TargetFormat)
}

func TestInitializeDownloadSources(t *testing.T) {
	sources := initializeDownloadSources()
	assert.NotEmpty(t, sources)

	// Check for expected sources
	sourceNames := make(map[string]bool)
	for _, source := range sources {
		sourceNames[source.Name] = true
		assert.NotEmpty(t, source.URL)
		assert.NotEmpty(t, source.Formats)
	}

	assert.True(t, sourceNames["HuggingFace"])
	assert.True(t, sourceNames["TheBloke"])
	assert.True(t, sourceNames["Bartowski"])
}

func TestInitializeConversionTools(t *testing.T) {
	tools := initializeConversionTools()
	assert.NotEmpty(t, tools)

	// Check GGUF conversion tool
	ggufTool, exists := tools[FormatGGUF]
	assert.True(t, exists)
	assert.Equal(t, "llama.cpp", ggufTool.Name)
	assert.Equal(t, FormatGGUF, ggufTool.TargetFormat)

	// Check GPTQ conversion tool
	gptqTool, exists := tools[FormatGPTQ]
	assert.True(t, exists)
	assert.Equal(t, "AutoGPTQ", gptqTool.Name)
	assert.Equal(t, FormatGPTQ, gptqTool.TargetFormat)

	// Check AWQ conversion tool
	awqTool, exists := tools[FormatAWQ]
	assert.True(t, exists)
	assert.Equal(t, "AutoAWQ", awqTool.Name)
	assert.Equal(t, FormatAWQ, awqTool.TargetFormat)
}

func TestModelDownloadManager_ConvertModel(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	progressChan := make(chan ModelDownloadProgress, 100)

	t.Run("NoConversionTool", func(t *testing.T) {
		// Clear conversion tools
		originalTools := manager.conversionTools
		manager.conversionTools = make(map[ModelFormat]*ConversionTool)

		progress := manager.convertModel("/path/to/model.hf", FormatGGUF, progressChan)
		assert.Contains(t, progress.Error, "no conversion tool found")

		// Restore tools
		manager.conversionTools = originalTools
	})

	t.Run("InputFileNotFound", func(t *testing.T) {
		progress := manager.convertModel("/nonexistent/path/model.hf", FormatGGUF, progressChan)
		assert.Contains(t, progress.Error, "input file not found")
	})
}

func TestModelDownloadManager_LoadModelRegistry(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewModelDownloadManager(tempDir)

	// Should have loaded some default models
	assert.NotEmpty(t, manager.availableModels)

	// Check specific models are loaded
	expectedModels := []string{
		"llama-3-8b-instruct",
		"mistral-7b-instruct",
		"codellama-7b-instruct",
	}

	for _, modelID := range expectedModels {
		t.Run(modelID, func(t *testing.T) {
			model, exists := manager.availableModels[modelID]
			assert.True(t, exists, "Model %s should exist", modelID)
			if exists {
				assert.NotEmpty(t, model.Name)
				assert.NotEmpty(t, model.AvailableFormats)
				assert.NotEmpty(t, model.DownloadURLs)
			}
		})
	}
}
