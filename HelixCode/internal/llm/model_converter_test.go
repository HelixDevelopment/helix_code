package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModelConverter(t *testing.T) {
	t.Run("CreatesNewConverter", func(t *testing.T) {
		tempDir := t.TempDir()

		converter := NewModelConverter(tempDir)
		require.NotNil(t, converter)
		assert.Equal(t, tempDir, converter.baseDir)
		assert.NotNil(t, converter.conversionTools)
		assert.NotEmpty(t, converter.conversionTools)
		assert.NotNil(t, converter.httpClient)
	})

	t.Run("CreatesTempDirectory", func(t *testing.T) {
		tempDir := t.TempDir()

		converter := NewModelConverter(tempDir)
		require.NotNil(t, converter)

		expectedTempDir := filepath.Join(tempDir, "temp")
		_, err := os.Stat(expectedTempDir)
		assert.NoError(t, err)
	})
}

func TestModelConverter_GetSupportedConversions(t *testing.T) {
	t.Run("ReturnsSupportedConversions", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		conversions := converter.GetSupportedConversions()
		require.NotNil(t, conversions)
		assert.NotEmpty(t, conversions)

		// Should have HF as source format
		_, hasHF := conversions[string(FormatHF)]
		assert.True(t, hasHF)
	})
}

func TestModelConverter_GetInstalledConversionTools(t *testing.T) {
	t.Run("ReturnsToolStatus", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		installed := converter.GetInstalledConversionTools()
		require.NotNil(t, installed)
		// All tools use python, check if it's available
		for name := range converter.conversionTools {
			_, exists := installed[name]
			assert.True(t, exists, "Tool %s should have status", name)
		}
	})
}

func TestModelConverter_ListConversionJobs(t *testing.T) {
	t.Run("ReturnsEmptyListInitially", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		jobs, err := converter.ListConversionJobs()
		require.NoError(t, err)
		assert.Empty(t, jobs)
	})

	t.Run("ReturnsExistingJobs", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		// Create jobs directory and add a job
		jobDir := filepath.Join(tempDir, "jobs")
		err := os.MkdirAll(jobDir, 0755)
		require.NoError(t, err)

		job := &ConversionJob{
			ID:           "test_job_1",
			SourcePath:   "/source/model.bin",
			TargetPath:   "/target/model.gguf",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Status:       StatusCompleted,
			StartTime:    time.Now(),
		}

		data, err := json.Marshal(job)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(jobDir, "test_job_1.json"), data, 0644)
		require.NoError(t, err)

		jobs, err := converter.ListConversionJobs()
		require.NoError(t, err)
		assert.Len(t, jobs, 1)
		assert.Equal(t, "test_job_1", jobs[0].ID)
	})
}

func TestModelConverter_GetConversionStatus(t *testing.T) {
	t.Run("ReturnsJobStatus", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		// Create jobs directory and add a job
		jobDir := filepath.Join(tempDir, "jobs")
		err := os.MkdirAll(jobDir, 0755)
		require.NoError(t, err)

		startTime := time.Now()
		job := &ConversionJob{
			ID:           "status_test_job",
			SourcePath:   "/source/model.bin",
			TargetPath:   "/target/model.gguf",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Status:       StatusRunning,
			Progress:     0.5,
			StartTime:    startTime,
			CurrentStep:  "Converting layers",
		}

		data, err := json.Marshal(job)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(jobDir, "status_test_job.json"), data, 0644)
		require.NoError(t, err)

		result, err := converter.GetConversionStatus("status_test_job")
		require.NoError(t, err)
		assert.Equal(t, "status_test_job", result.ID)
		assert.Equal(t, StatusRunning, result.Status)
		assert.Equal(t, 0.5, result.Progress)
		assert.Equal(t, "Converting layers", result.CurrentStep)
	})

	t.Run("ErrorsForNonExistentJob", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		result, err := converter.GetConversionStatus("non_existent_job")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestModelConverter_CancelConversion(t *testing.T) {
	t.Run("CancelsRunningJob", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		// Create jobs directory and add a running job
		jobDir := filepath.Join(tempDir, "jobs")
		err := os.MkdirAll(jobDir, 0755)
		require.NoError(t, err)

		job := &ConversionJob{
			ID:           "cancel_test_job",
			SourcePath:   "/source/model.bin",
			TargetPath:   "/target/model.gguf",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Status:       StatusRunning,
			StartTime:    time.Now(),
		}

		data, err := json.Marshal(job)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(jobDir, "cancel_test_job.json"), data, 0644)
		require.NoError(t, err)

		err = converter.CancelConversion("cancel_test_job")
		require.NoError(t, err)

		// Verify job was cancelled
		result, err := converter.GetConversionStatus("cancel_test_job")
		require.NoError(t, err)
		assert.Equal(t, StatusCancelled, result.Status)
		assert.NotNil(t, result.EndTime)
	})

	t.Run("CancelsPendingJob", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		jobDir := filepath.Join(tempDir, "jobs")
		err := os.MkdirAll(jobDir, 0755)
		require.NoError(t, err)

		job := &ConversionJob{
			ID:           "pending_cancel_job",
			SourcePath:   "/source/model.bin",
			TargetPath:   "/target/model.gguf",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Status:       StatusPending,
			StartTime:    time.Now(),
		}

		data, err := json.Marshal(job)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(jobDir, "pending_cancel_job.json"), data, 0644)
		require.NoError(t, err)

		err = converter.CancelConversion("pending_cancel_job")
		require.NoError(t, err)
	})

	t.Run("ErrorsForCompletedJob", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		jobDir := filepath.Join(tempDir, "jobs")
		err := os.MkdirAll(jobDir, 0755)
		require.NoError(t, err)

		job := &ConversionJob{
			ID:           "completed_job",
			SourcePath:   "/source/model.bin",
			TargetPath:   "/target/model.gguf",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Status:       StatusCompleted,
			StartTime:    time.Now(),
		}

		data, err := json.Marshal(job)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(jobDir, "completed_job.json"), data, 0644)
		require.NoError(t, err)

		err = converter.CancelConversion("completed_job")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be cancelled")
	})

	t.Run("ErrorsForNonExistentJob", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		err := converter.CancelConversion("non_existent_job")
		assert.Error(t, err)
	})
}

func TestModelConverter_ValidateConversion(t *testing.T) {
	t.Run("ValidatesGGUFConversion", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		result, err := converter.ValidateConversion(FormatHF, FormatGGUF)
		require.NoError(t, err)
		// IsPossible depends on whether python is installed
		// But recommendations should always be populated for GGUF
		assert.NotEmpty(t, result.Recommendations)
		// If python is not installed, IsPossible will be false but there should be warnings
		if !result.IsPossible {
			assert.NotEmpty(t, result.Warnings)
		}
	})

	t.Run("ValidatesGPTQConversion", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		result, err := converter.ValidateConversion(FormatHF, FormatGPTQ)
		require.NoError(t, err)
		// Result should always be returned, even if tool not installed
		assert.NotNil(t, result)
	})

	t.Run("ValidatesAWQConversion", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		result, err := converter.ValidateConversion(FormatHF, FormatAWQ)
		require.NoError(t, err)
		// Result should always be returned, even if tool not installed
		assert.NotNil(t, result)
	})

	t.Run("ErrorsForUnsupportedConversion", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		result, err := converter.ValidateConversion(FormatGGUF, FormatGPTQ)
		assert.Error(t, err)
		assert.False(t, result.IsPossible)
	})
}

func TestModelConverter_GetConversionHistory(t *testing.T) {
	t.Run("ReturnsEmptyHistory", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		history, err := converter.GetConversionHistory()
		require.NoError(t, err)
		assert.Equal(t, 0, history.TotalConversions)
		assert.Equal(t, 0, history.SuccessfulConversions)
		assert.Equal(t, 0, history.FailedConversions)
	})

	t.Run("ReturnsHistoryWithJobs", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		// Create jobs directory
		jobDir := filepath.Join(tempDir, "jobs")
		err := os.MkdirAll(jobDir, 0755)
		require.NoError(t, err)

		startTime := time.Now().Add(-10 * time.Minute)
		endTime := time.Now()

		// Add completed job
		completedJob := &ConversionJob{
			ID:           "completed_job_1",
			SourcePath:   "/source/model1.bin",
			TargetPath:   "/target/model1.gguf",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Status:       StatusCompleted,
			StartTime:    startTime,
			EndTime:      &endTime,
		}
		data, _ := json.Marshal(completedJob)
		os.WriteFile(filepath.Join(jobDir, "completed_job_1.json"), data, 0644)

		// Add failed job
		failedJob := &ConversionJob{
			ID:           "failed_job_1",
			SourcePath:   "/source/model2.bin",
			TargetPath:   "/target/model2.gguf",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Status:       StatusFailed,
			StartTime:    startTime,
			Error:        "conversion failed",
		}
		data, _ = json.Marshal(failedJob)
		os.WriteFile(filepath.Join(jobDir, "failed_job_1.json"), data, 0644)

		history, err := converter.GetConversionHistory()
		require.NoError(t, err)
		assert.Equal(t, 2, history.TotalConversions)
		assert.Equal(t, 1, history.SuccessfulConversions)
		assert.Equal(t, 1, history.FailedConversions)
		assert.NotEmpty(t, history.RecentConversions)
	})
}

func TestModelConverter_ConvertModel(t *testing.T) {
	t.Run("CreatesConversionJob", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)
		t.Cleanup(converter.Shutdown)

		// Create source file
		sourceDir := filepath.Join(tempDir, "source")
		os.MkdirAll(sourceDir, 0755)
		sourcePath := filepath.Join(sourceDir, "model.safetensors")
		os.WriteFile(sourcePath, []byte("test model content"), 0644)

		// Create logs directory
		logsDir := filepath.Join(tempDir, "logs")
		os.MkdirAll(logsDir, 0755)

		config := ConversionConfig{
			SourcePath:   sourcePath,
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		job, err := converter.ConvertModel(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, job)
		assert.NotEmpty(t, job.ID)
		assert.Contains(t, job.ID, "conv_")
		assert.Equal(t, FormatHF, job.SourceFormat)
		assert.Equal(t, FormatGGUF, job.TargetFormat)
		assert.NotEmpty(t, job.TargetPath)
		assert.NotEmpty(t, job.LogPath)
		assert.NotEmpty(t, job.Command)

		// Wait briefly for job to start
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("GeneratesTargetPath", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)
		t.Cleanup(converter.Shutdown)

		sourceDir := filepath.Join(tempDir, "source")
		os.MkdirAll(sourceDir, 0755)
		sourcePath := filepath.Join(sourceDir, "llama-7b.safetensors")
		os.WriteFile(sourcePath, []byte("test"), 0644)

		logsDir := filepath.Join(tempDir, "logs")
		os.MkdirAll(logsDir, 0755)

		config := ConversionConfig{
			SourcePath:   sourcePath,
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		job, err := converter.ConvertModel(ctx, config)
		require.NoError(t, err)
		assert.Contains(t, job.TargetPath, "llama-7b")
		assert.Contains(t, job.TargetPath, string(FormatGGUF))
	})

	t.Run("ErrorsForUnsupportedConversion", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		config := ConversionConfig{
			SourcePath:   "/source/model.gguf",
			SourceFormat: FormatGGUF,
			TargetFormat: FormatGPTQ,
		}

		ctx := context.Background()
		job, err := converter.ConvertModel(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "no conversion tool found")
	})
}

func TestModelConverter_estimateConversionTime(t *testing.T) {
	t.Run("BaseEstimation", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		config := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatHF,
		}

		time := converter.estimateConversionTime(config)
		assert.Greater(t, time, int64(0))
	})

	t.Run("GGUFFormatAddsTime", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		baseConfig := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatHF,
		}
		baseTime := converter.estimateConversionTime(baseConfig)

		ggufConfig := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
		}
		ggufTime := converter.estimateConversionTime(ggufConfig)

		assert.Greater(t, ggufTime, baseTime)
	})

	t.Run("QuantizationAddsTime", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		withoutQuant := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
		}
		timeWithout := converter.estimateConversionTime(withoutQuant)

		withQuant := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Quantization: &QuantizationConfig{
				Method: "q4_k_m",
				Bits:   4,
			},
		}
		timeWith := converter.estimateConversionTime(withQuant)

		assert.Greater(t, timeWith, timeWithout)
	})

	t.Run("OptimizationAddsTime", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		withoutOpt := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
		}
		timeWithout := converter.estimateConversionTime(withoutOpt)

		withOpt := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			Optimization: &OptimizationConfig{
				RemoveUnusedLayers: true,
				FuseOperations:     true,
			},
		}
		timeWith := converter.estimateConversionTime(withOpt)

		assert.Greater(t, timeWith, timeWithout)
	})
}

func TestModelConverter_generateTargetPath(t *testing.T) {
	t.Run("GeneratesCorrectPath", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		path := converter.generateTargetPath("/models/llama-7b.safetensors", FormatGGUF)
		assert.Equal(t, "/models/llama-7b.gguf", path)
	})

	t.Run("HandlesMultipleExtensions", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		path := converter.generateTargetPath("/models/model.tar.gz", FormatGGUF)
		// Should only strip the last extension
		assert.Contains(t, path, "model.tar")
		assert.Contains(t, path, string(FormatGGUF))
	})
}

func TestModelConverter_buildCommand(t *testing.T) {
	t.Run("AddsInputOutput", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		tool := &ConversionTool{
			Name:    "test",
			Command: "python",
			Args:    []string{"-m", "converter"},
		}

		config := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			TargetPath:   "/target/model.gguf",
		}

		args := converter.buildCommand(tool, config)
		assert.Contains(t, args, "--input")
		assert.Contains(t, args, "/source/model.bin")
		assert.Contains(t, args, "--output")
		assert.Contains(t, args, "/target/model.gguf")
	})

	t.Run("AddsQuantizationOptions", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		tool := &ConversionTool{
			Name:    "test",
			Command: "python",
			Args:    []string{},
		}

		config := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			TargetPath:   "/target/model.gguf",
			Quantization: &QuantizationConfig{
				Method:     "q4_k_m",
				Bits:       4,
				UseExllama: true,
				UseFp16:    true,
			},
		}

		args := converter.buildCommand(tool, config)
		assert.Contains(t, args, "--quantize")
		assert.Contains(t, args, "q4_k_m")
		assert.Contains(t, args, "--bits")
		assert.Contains(t, args, "--exllama")
		assert.Contains(t, args, "--fp16")
	})

	t.Run("AddsOptimizationOptions", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		tool := &ConversionTool{
			Name:    "test",
			Command: "python",
			Args:    []string{},
		}

		config := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			TargetPath:   "/target/model.gguf",
			Optimization: &OptimizationConfig{
				RemoveUnusedLayers: true,
				FuseOperations:     true,
				OptimizeFor:        "gpu",
				TargetHardware:     "nvidia",
				DeviceMap:          "auto",
			},
		}

		args := converter.buildCommand(tool, config)
		assert.Contains(t, args, "--prune")
		assert.Contains(t, args, "--fuse")
		assert.Contains(t, args, "--optimize-for")
		assert.Contains(t, args, "gpu")
		assert.Contains(t, args, "--target-hardware")
		assert.Contains(t, args, "nvidia")
		assert.Contains(t, args, "--device-map")
		assert.Contains(t, args, "auto")
	})
}

func TestModelConverter_flattenEnvVars(t *testing.T) {
	t.Run("FlattensEnvironmentVariables", func(t *testing.T) {
		tempDir := t.TempDir()
		converter := NewModelConverter(tempDir)

		envVars := map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
		}

		flattened := converter.flattenEnvVars(envVars)
		assert.Len(t, flattened, 2)

		hasVar1 := false
		hasVar2 := false
		for _, env := range flattened {
			if env == "VAR1=value1" {
				hasVar1 = true
			}
			if env == "VAR2=value2" {
				hasVar2 = true
			}
		}
		assert.True(t, hasVar1)
		assert.True(t, hasVar2)
	})
}

func TestConversionJob_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		startTime := time.Now()
		endTime := time.Now().Add(10 * time.Minute)

		job := &ConversionJob{
			ID:            "conv_12345",
			SourcePath:    "/source/model.bin",
			TargetPath:    "/target/model.gguf",
			SourceFormat:  FormatHF,
			TargetFormat:  FormatGGUF,
			Progress:      0.75,
			Status:        StatusRunning,
			StartTime:     startTime,
			EndTime:       &endTime,
			Error:         "",
			LogPath:       "/logs/conversion.log",
			Command:       "python",
			Args:          []string{"-m", "converter"},
			EstimatedTime: 30,
			CurrentStep:   "Converting layers",
		}

		assert.Equal(t, "conv_12345", job.ID)
		assert.Equal(t, "/source/model.bin", job.SourcePath)
		assert.Equal(t, "/target/model.gguf", job.TargetPath)
		assert.Equal(t, FormatHF, job.SourceFormat)
		assert.Equal(t, FormatGGUF, job.TargetFormat)
		assert.Equal(t, 0.75, job.Progress)
		assert.Equal(t, StatusRunning, job.Status)
		assert.Equal(t, startTime, job.StartTime)
		assert.Equal(t, &endTime, job.EndTime)
		assert.Empty(t, job.Error)
		assert.Equal(t, "/logs/conversion.log", job.LogPath)
		assert.Equal(t, "python", job.Command)
		assert.Len(t, job.Args, 2)
		assert.Equal(t, int64(30), job.EstimatedTime)
		assert.Equal(t, "Converting layers", job.CurrentStep)
	})
}

func TestConversionConfig_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		config := ConversionConfig{
			SourcePath:   "/source/model.bin",
			SourceFormat: FormatHF,
			TargetFormat: FormatGGUF,
			TargetPath:   "/target/model.gguf",
			Quantization: &QuantizationConfig{
				Method:     "q4_k_m",
				Bits:       4,
				UseExllama: true,
				UseFp16:    false,
			},
			Optimization: &OptimizationConfig{
				RemoveUnusedLayers: true,
				FuseOperations:     true,
				OptimizeFor:        "gpu",
				TargetHardware:     "nvidia",
				DeviceMap:          "auto",
			},
			Environment: map[string]string{
				"CUDA_VISIBLE_DEVICES": "0",
			},
			Timeout: 60,
		}

		assert.Equal(t, "/source/model.bin", config.SourcePath)
		assert.Equal(t, FormatHF, config.SourceFormat)
		assert.Equal(t, FormatGGUF, config.TargetFormat)
		assert.Equal(t, "/target/model.gguf", config.TargetPath)
		assert.NotNil(t, config.Quantization)
		assert.NotNil(t, config.Optimization)
		assert.NotNil(t, config.Environment)
		assert.Equal(t, 60, config.Timeout)
	})
}

func TestQuantizationConfig_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		quant := QuantizationConfig{
			Method:     "q4_k_m",
			Bits:       4,
			UseExllama: true,
			UseFp16:    true,
		}

		assert.Equal(t, "q4_k_m", quant.Method)
		assert.Equal(t, 4, quant.Bits)
		assert.True(t, quant.UseExllama)
		assert.True(t, quant.UseFp16)
	})
}

func TestOptimizationConfig_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		opt := OptimizationConfig{
			RemoveUnusedLayers: true,
			FuseOperations:     true,
			OptimizeFor:        "gpu",
			TargetHardware:     "nvidia",
			DeviceMap:          "cuda:0",
		}

		assert.True(t, opt.RemoveUnusedLayers)
		assert.True(t, opt.FuseOperations)
		assert.Equal(t, "gpu", opt.OptimizeFor)
		assert.Equal(t, "nvidia", opt.TargetHardware)
		assert.Equal(t, "cuda:0", opt.DeviceMap)
	})
}

func TestConversionStatus(t *testing.T) {
	t.Run("StatusValues", func(t *testing.T) {
		assert.Equal(t, ConversionStatus("pending"), StatusPending)
		assert.Equal(t, ConversionStatus("running"), StatusRunning)
		assert.Equal(t, ConversionStatus("completed"), StatusCompleted)
		assert.Equal(t, ConversionStatus("failed"), StatusFailed)
		assert.Equal(t, ConversionStatus("cancelled"), StatusCancelled)
	})
}

func TestValidationResult_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		result := ValidationResult{
			IsPossible:      true,
			Confidence:      0.85,
			Warnings:        []string{"Large model size"},
			Recommendations: []string{"Use GPU acceleration"},
			EstimatedTime:   30,
			RequiredSpace:   "10GB",
		}

		assert.True(t, result.IsPossible)
		assert.Equal(t, 0.85, result.Confidence)
		assert.Len(t, result.Warnings, 1)
		assert.Len(t, result.Recommendations, 1)
		assert.Equal(t, int64(30), result.EstimatedTime)
		assert.Equal(t, "10GB", result.RequiredSpace)
	})
}

func TestConversionHistory_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		history := ConversionHistory{
			TotalConversions:      10,
			SuccessfulConversions: 8,
			FailedConversions:     2,
			AverageConversionTime: 15,
			RecentConversions:     []*ConversionJob{},
		}

		assert.Equal(t, 10, history.TotalConversions)
		assert.Equal(t, 8, history.SuccessfulConversions)
		assert.Equal(t, 2, history.FailedConversions)
		assert.Equal(t, int64(15), history.AverageConversionTime)
		assert.Empty(t, history.RecentConversions)
	})
}

func TestHTTPClient_Struct(t *testing.T) {
	t.Run("AllFields", func(t *testing.T) {
		client := HTTPClient{
			Timeout: 30 * time.Second,
		}

		assert.Equal(t, 30*time.Second, client.Timeout)
	})
}
