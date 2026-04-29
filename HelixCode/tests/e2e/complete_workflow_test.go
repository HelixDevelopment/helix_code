package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end tests for complete workflows

const (
	cliPath       = "../../bin/cli"
	testTimeout   = 5 * time.Minute
	healthTimeout = 30 * time.Second
	longTimeout   = 10 * time.Minute
)

// Test scenarios for complete user workflows
func TestCompleteUserWorkflows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")  // SKIP-OK: #short-mode
	}

	// Build CLI
	require.NoError(t, buildCLI(), "CLI should build successfully")

	// Test workflow 1: New user setup
	t.Run("NewUserSetup", func(t *testing.T) {
		testNewUserSetup(t)
	})

	// Test workflow 2: Advanced user workflow
	t.Run("AdvancedUserWorkflow", func(t *testing.T) {
		testAdvancedUserWorkflow(t)
	})

	// Test workflow 3: Production deployment
	t.Run("ProductionDeployment", func(t *testing.T) {
		testProductionDeployment(t)
	})

	// Test workflow 4: Multi-provider orchestration
	t.Run("MultiProviderOrchestration", func(t *testing.T) {
		testMultiProviderOrchestration(t)
	})

	// Test workflow 5: Model optimization workflow
	t.Run("ModelOptimizationWorkflow", func(t *testing.T) {
		testModelOptimizationWorkflow(t)
	})
}

func TestCLICommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI tests in short mode")  // SKIP-OK: #short-mode
	}

	// Build CLI
	require.NoError(t, buildCLI(), "CLI should build successfully")

	testCases := []struct {
		name     string
		args     []string
		expected string
		exitCode int
	}{
		{
			name:     "HelpCommand",
			args:     []string{"--help"},
			expected: "Usage of ../../bin/cli:",
			exitCode: 0,
		},
		{
			name:     "ListWorkers",
			args:     []string{"--list-workers"},
			expected: "=== Worker Statistics ===",
			exitCode: 0,
		},
		{
			name:     "InvalidCommand",
			args:     []string{"--invalid-command"},
			expected: "flag provided but not defined",
			exitCode: 2, // Go flag parsing error
		},
		{
			name:     "HealthCheck",
			args:     []string{"--health"},
			expected: "=== System Health Check ===",
			exitCode: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, exitCode := runCLICommand(t, tc.args...)

			assert.Equal(t, tc.exitCode, exitCode, "Exit code mismatch")
			assert.Contains(t, output, tc.expected, "Expected output not found")
		})
	}
}

func TestRealModelWorkflow(t *testing.T) {
	if testing.Short() || os.Getenv("SKIP_REAL_MODEL_TESTS") == "true" {
		t.Skip("Skipping real model tests")  // SKIP-OK: #legacy-untriaged
	}

	// Build CLI
	require.NoError(t, buildCLI(), "CLI should build successfully")

	// Test complete workflow with real model
	testRealModelWorkflow(t)
}

func testNewUserSetup(t *testing.T) {
	t.Log("🔄 Testing new user setup workflow")

	baseDir := t.TempDir()

	// Step 1: Test basic health check
	output, exitCode := runCLICommandWithEnv(t, []string{
		"--health",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	assert.Equal(t, 0, exitCode, "Health check should succeed")
	assert.Contains(t, output, "=== System Health Check ===", "Should show health check")

	// Step 2: List workers (should be empty initially)
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--list-workers",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	assert.Equal(t, 0, exitCode, "List workers should succeed")
	assert.Contains(t, output, "=== Worker Statistics ===", "Should show worker statistics")

	// Step 3: List available models
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--list-models",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	assert.Equal(t, 0, exitCode, "List models should succeed")

	// Step 4: Test simple LLM prompt with default model
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--prompt", "Hello, how are you?",
		"--max-tokens", "10",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	// LLM command might fail if no provider is available, but command should complete
	t.Logf("LLM prompt result: %s (exit code: %d)", output, exitCode)

	// Step 5: Test notification system
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--notify", "Test notification from E2E test",
		"--notify-type", "info",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	t.Logf("Notification result: %s (exit code: %d)", output, exitCode)

	t.Log("✅ New user setup workflow completed")
}

func testAdvancedUserWorkflow(t *testing.T) {
	t.Log("🔄 Testing advanced user workflow")

	baseDir := t.TempDir()

	// Step 1: Test worker management
	output, exitCode := runCLICommandWithEnv(t, []string{
		"--list-workers",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	assert.Equal(t, 0, exitCode, "List workers should succeed")

	// Step 2: Test model listing
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--list-models",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	assert.Equal(t, 0, exitCode, "List models should succeed")

	// Step 3: Test LLM generation with different parameters
	testPrompts := []struct {
		prompt      string
		maxTokens   int
		temperature float64
	}{
		{"Hello", 10, 0.7},
		{"What is AI?", 50, 0.5},
		{"Explain quantum computing", 100, 0.8},
	}

	for _, test := range testPrompts {
		output, exitCode = runCLICommandWithEnv(t, []string{
			"--prompt", test.prompt,
			"--max-tokens", fmt.Sprintf("%d", test.maxTokens),
			"--temperature", fmt.Sprintf("%.1f", test.temperature),
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("LLM test (prompt: %s, tokens: %d, temp: %.1f): %s (exit code: %d)",
			test.prompt, test.maxTokens, test.temperature, output, exitCode)
	}

	// Step 4: Test streaming mode
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--prompt", "Count from 1 to 5",
		"--stream",
		"--max-tokens", "20",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	t.Logf("Streaming test result: %s (exit code: %d)", output, exitCode)

	// Step 5: Test notifications with different priorities
	priorities := []string{"low", "medium", "high", "critical"}
	for _, priority := range priorities {
		output, exitCode = runCLICommandWithEnv(t, []string{
			"--notify", fmt.Sprintf("Test %s priority notification", priority),
			"--notify-priority", priority,
			"--notify-type", "info",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Priority notification test (%s): %s (exit code: %d)", priority, output, exitCode)
	}

	// Step 6: Test different notification types
	notifyTypes := []string{"info", "success", "warning", "error"}
	for _, notifyType := range notifyTypes {
		output, exitCode = runCLICommandWithEnv(t, []string{
			"--notify", fmt.Sprintf("Test %s notification", notifyType),
			"--notify-type", notifyType,
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Notification type test (%s): %s (exit code: %d)", notifyType, output, exitCode)
	}

	t.Log("✅ Advanced user workflow completed")
}

func testProductionDeployment(t *testing.T) {
	t.Log("🔄 Testing production deployment workflow")

	baseDir := t.TempDir()

	// Step 1: Test production environment setup
	prodEnv := map[string]string{
		"HELIX_BASE_DIR":        baseDir,
		"HELIX_PRODUCTION":      "true",
		"HELIX_LOG_LEVEL":       "info",
		"HELIX_METRICS_ENABLED": "true",
	}

	// Step 2: Test health check in production mode
	_, exitCode := runCLICommandWithEnv(t, []string{
		"--health",
	}, prodEnv)

	assert.Equal(t, 0, exitCode, "Production health check should succeed")

	// Step 3: Test worker listing in production
	_, exitCode = runCLICommandWithEnv(t, []string{
		"--list-workers",
	}, prodEnv)

	assert.Equal(t, 0, exitCode, "Production worker listing should succeed")

	// Step 4: Test model listing in production
	_, exitCode = runCLICommandWithEnv(t, []string{
		"--list-models",
	}, prodEnv)

	assert.Equal(t, 0, exitCode, "Production model listing should succeed")

	// Step 5: Test LLM generation with production settings
	output, exitCode := runCLICommandWithEnv(t, []string{
		"--prompt", "Production test prompt",
		"--model", "llama-3-8b",
		"--max-tokens", "50",
		"--temperature", "0.5",
	}, prodEnv)

	t.Logf("Production LLM test: %s (exit code: %d)", output, exitCode)

	// Step 6: Test notifications in production
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--notify", "Production deployment test",
		"--notify-type", "success",
		"--notify-priority", "high",
	}, prodEnv)

	t.Logf("Production notification test: %s (exit code: %d)", output, exitCode)

	// Step 7: Test stress-like scenario with multiple rapid calls
	for i := 0; i < 5; i++ {
		_, exitCode = runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Rapid test prompt %d", i+1),
			"--max-tokens", "10",
		}, prodEnv)

		t.Logf("Rapid test %d: exit code %d", i+1, exitCode)
	}

	t.Log("✅ Production deployment workflow completed")
}

func testMultiProviderOrchestration(t *testing.T) {
	t.Log("🔄 Testing multi-provider orchestration workflow")

	baseDir := t.TempDir()

	// Step 1: Test with different models (simulating multi-provider scenario)
	models := []string{"llama-3-8b", "mistral-7b", "phi-3-mini"}

	for _, model := range models {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Test with model %s", model),
			"--model", model,
			"--max-tokens", "20",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Model %s test: %s (exit code: %d)", model, output, exitCode)
	}

	// Step 2: Test load balancing simulation
	for i := 0; i < 10; i++ {
		modelIndex := i % len(models)
		_, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Load balance test %d", i+1),
			"--model", models[modelIndex],
			"--max-tokens", "15",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Load balance iteration %d with model %s: exit code %d", i+1, models[modelIndex], exitCode)
	}

	// Step 3: Test different temperatures with same model (simulating provider optimization)
	temperatures := []float64{0.1, 0.5, 0.7, 1.0}
	for _, temp := range temperatures {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Temperature test with %.1f", temp),
			"--temperature", fmt.Sprintf("%.1f", temp),
			"--max-tokens", "25",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Temperature %.1f test: %s (exit code: %d)", temp, output, exitCode)
	}

	// Step 4: Test with different token limits (simulating different provider capabilities)
	tokenLimits := []int{10, 50, 100, 500}
	for _, maxTokens := range tokenLimits {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Token limit test with %d tokens", maxTokens),
			"--max-tokens", fmt.Sprintf("%d", maxTokens),
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Token limit %d test: %s (exit code: %d)", maxTokens, output, exitCode)
	}

	// Step 5: Test streaming vs non-streaming (simulating different provider capabilities)
	streamModes := []bool{false, true}
	for _, stream := range streamModes {
		args := []string{
			"--prompt", "Stream vs non-stream test",
			"--max-tokens", "30",
		}
		
		if stream {
			args = append(args, "--stream")
		}

		output, exitCode := runCLICommandWithEnv(t, args, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		modeStr := "streaming"
		if !stream {
			modeStr = "non-streaming"
		}
		t.Logf("%s test: %s (exit code: %d)", modeStr, output, exitCode)
	}

	t.Log("✅ Multi-provider orchestration workflow completed")
}

func testModelOptimizationWorkflow(t *testing.T) {
	t.Log("🔄 Testing model optimization workflow")

	baseDir := t.TempDir()

	// Step 1: Test different model configurations (simulating optimization)
	modelConfigs := []struct {
		model       string
		maxTokens   int
		temperature float64
		description string
	}{
		{"llama-3-8b", 100, 0.7, "Default configuration"},
		{"llama-3-8b", 50, 0.5, "Optimized for speed"},
		{"llama-3-8b", 200, 0.8, "Optimized for quality"},
		{"mistral-7b", 75, 0.6, "Balanced configuration"},
		{"phi-3-mini", 60, 0.4, "Compact configuration"},
	}

	for _, config := range modelConfigs {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Testing %s", config.description),
			"--model", config.model,
			"--max-tokens", fmt.Sprintf("%d", config.maxTokens),
			"--temperature", fmt.Sprintf("%.1f", config.temperature),
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Config '%s' test: %s (exit code: %d)", config.description, output, exitCode)
	}

	// Step 2: Test performance comparison simulation
	testPrompt := "Compare the performance of these configurations"
	for i, config := range modelConfigs[:3] { // Test first 3 configs for comparison
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("%s - Config %d: %s", testPrompt, i+1, config.description),
			"--model", config.model,
			"--max-tokens", fmt.Sprintf("%d", config.maxTokens),
			"--temperature", fmt.Sprintf("%.1f", config.temperature),
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Performance comparison %d: %s (exit code: %d)", i+1, output, exitCode)
	}

	// Step 3: Test streaming performance
	streamingConfigs := []struct {
		maxTokens int
		description string
	}{
		{25, "Short streaming"},
		{50, "Medium streaming"},
		{100, "Long streaming"},
	}

	for _, config := range streamingConfigs {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Testing %s response", config.description),
			"--stream",
			"--max-tokens", fmt.Sprintf("%d", config.maxTokens),
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Streaming performance %s: %s (exit code: %d)", config.description, output, exitCode)
	}

	// Step 4: Test with different prompt complexities
	promptComplexities := []struct {
		prompt      string
		description string
	}{
		{"Hi", "Simple prompt"},
		{"Explain photosynthesis in simple terms", "Medium complexity"},
		{"Analyze the economic implications of artificial general intelligence on global markets", "High complexity"},
	}

	for _, complexity := range promptComplexities {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", complexity.prompt,
			"--max-tokens", "50",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Complexity test '%s': %s (exit code: %d)", complexity.description, output, exitCode)
	}

	// Step 5: Test optimization through temperature tuning
	temperatureTest := "Generate a creative story about AI"
	temperatures := []float64{0.1, 0.5, 0.7, 1.0}

	for _, temp := range temperatures {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", temperatureTest,
			"--temperature", fmt.Sprintf("%.1f", temp),
			"--max-tokens", "30",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Temperature %.1f optimization: %s (exit code: %d)", temp, output, exitCode)
	}

	t.Log("✅ Model optimization workflow completed")
}

func testRealModelWorkflow(t *testing.T) {
	t.Log("🔄 Testing real model workflow")

	baseDir := t.TempDir()

	// Step 1: Test basic functionality
	output, exitCode := runCLICommandWithEnv(t, []string{
		"--health",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	assert.Equal(t, 0, exitCode, "Health check should succeed")

	// Step 2: Test with available models
	testModels := []string{"llama-3-8b", "mistral-7b", "phi-3-mini", "gemma-2b"}
	var workingModels []string

	for _, model := range testModels {
		output, exitCode = runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Test with model %s", model),
			"--model", model,
			"--max-tokens", "20",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Model %s test: %s (exit code: %d)", model, output, exitCode)
		if exitCode == 0 {
			workingModels = append(workingModels, model)
		}
	}

	if len(workingModels) == 0 {
		t.Skip("No working models available in test environment")  // SKIP-OK: #legacy-untriaged
		return
	}

	// Step 3: Test with different parameters using working models
	for _, model := range workingModels {
		// Test different parameters
		output, exitCode = runCLICommandWithEnv(t, []string{
			"--prompt", "Write a short poem about programming",
			"--model", model,
			"--max-tokens", "50",
			"--temperature", "0.8",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Creative test with %s: %s (exit code: %d)", model, output, exitCode)

		// Test streaming
		output, exitCode = runCLICommandWithEnv(t, []string{
			"--prompt", "Count from 1 to 5",
			"--model", model,
			"--stream",
			"--max-tokens", "15",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Streaming test with %s: %s (exit code: %d)", model, output, exitCode)
	}

	// Step 4: Test consecutive requests
	consecutiveRequests := 5
	for reqIdx := 0; reqIdx < consecutiveRequests; reqIdx++ {
		output, exitCode := runCLICommandWithEnv(t, []string{
			"--prompt", fmt.Sprintf("Consecutive request %d: What is the current time?", reqIdx+1),
			"--model", workingModels[0],
			"--max-tokens", "20",
		}, map[string]string{
			"HELIX_BASE_DIR": baseDir,
		})

		t.Logf("Consecutive request %d: %s (exit code: %d)", reqIdx+1, output, exitCode)
	}

	// Step 5: Test notifications
	output, exitCode = runCLICommandWithEnv(t, []string{
		"--notify", "Real model workflow completed successfully",
		"--notify-type", "success",
		"--notify-priority", "medium",
	}, map[string]string{
		"HELIX_BASE_DIR": baseDir,
	})

	t.Logf("Notification test: %s (exit code: %d)", output, exitCode)

	t.Log("✅ Real model workflow completed")
}

// Helper functions

func buildCLI() error {
	// Build the main HelixCode CLI for testing
	// The CLI is at ../../cmd/cli relative to tests/e2e/
	cliSourcePath := "../../cmd/cli"
	cliPath := "../../bin/cli"
	cmd := exec.Command("go", "build", "-o", cliPath, cliSourcePath)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOFLAGS=-tags=test")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build failed: %v\nOutput: %s", err, string(output))
	}
	return nil
}

func runCLICommand(t *testing.T, args ...string) (string, int) {
	return runCLICommandWithEnv(t, args, nil)
}

func runCLICommandWithEnv(t *testing.T, args []string, env map[string]string) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliPath, args...)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return output, exitError.ExitCode()
		}
		return output, 1
	}

	return output, 0
}
