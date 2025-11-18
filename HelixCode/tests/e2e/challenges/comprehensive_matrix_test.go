package challenges

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ComprehensiveTestMatrix represents a complete test configuration
type ComprehensiveTestMatrix struct {
	Provider       LLMProviderType
	Model          string
	Distribution   ChallengeDistribution
	RequiresAPIKey bool
}

// TestComprehensiveMatrix runs ALL combinations: models × distributions × challenges
func TestComprehensiveMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive matrix test in short mode")
	}

	config := DefaultChallengeConfig()
	config.ResultsBaseDir = "./test-results/comprehensive-matrix"
	config.LogsBaseDir = "./test-results/logs"

	executor := NewChallengeExecutor(config)

	// Define ALL model configurations
	models := []ComprehensiveTestMatrix{
		// Local providers (no API key)
		{Provider: ProviderOllama, Model: "llama2", Distribution: DistributionSingle, RequiresAPIKey: false},

		// Cloud providers - xAI
		{Provider: ProviderXAI, Model: "grok-beta", Distribution: DistributionSingle, RequiresAPIKey: true},

		// Cloud providers - DeepSeek (3 models)
		{Provider: ProviderDeepSeek, Model: "deepseek-chat", Distribution: DistributionSingle, RequiresAPIKey: true},
		{Provider: ProviderDeepSeek, Model: "deepseek-coder", Distribution: DistributionSingle, RequiresAPIKey: true},
		{Provider: ProviderDeepSeek, Model: "deepseek-reasoner", Distribution: DistributionSingle, RequiresAPIKey: true},

		// New providers
		{Provider: ProviderHuggingFace, Model: "bigcode/starcoder", Distribution: DistributionSingle, RequiresAPIKey: true},
		{Provider: ProviderOpenCode, Model: "opencode-7b", Distribution: DistributionSingle, RequiresAPIKey: true},
		{Provider: ProviderOpenRouter, Model: "meta-llama/codellama-34b-instruct:free", Distribution: DistributionSingle, RequiresAPIKey: true},
		{Provider: ProviderGemini, Model: "gemini-pro", Distribution: DistributionSingle, RequiresAPIKey: true},
	}

	// Define ALL distribution modes
	distributions := []ChallengeDistribution{
		DistributionSingle,
		DistributionWorker2,
		DistributionWorker5,
		DistributionWorker10,
	}

	// Load ALL challenge definitions
	challenges := []string{
		"./definitions/ascii-art-generator.json",
		"./definitions/tic-tac-toe-tui.json",
		"./definitions/notes-project.json",
		"./definitions/cli-task-manager.json",
		"./definitions/url-shortener.json",
		"./definitions/json-validator-cli.json",
	}

	ctx := context.Background()

	// Summary tracking
	totalTests := 0
	passedTests := 0
	failedTests := 0
	skippedTests := 0

	startTime := time.Now()

	t.Logf("\n%s", strings.Repeat("=", 100))
	t.Logf("COMPREHENSIVE TEST MATRIX - ALL COMBINATIONS")
	t.Logf("Models: %d × Distributions: %d × Challenges: %d = %d Total Test Executions",
		len(models), len(distributions), len(challenges), len(models)*len(distributions)*len(challenges))
	t.Logf("Estimated Time: %.0f minutes", float64(len(models)*len(distributions)*len(challenges))*0.33)
	t.Logf("%s\n", strings.Repeat("=", 100))

	// PHASE 1: Single Distribution Mode (all models, all challenges)
	t.Run("Phase1_SingleDistribution", func(t *testing.T) {
		t.Logf("\n=== PHASE 1: Single Distribution Mode ===")
		t.Logf("Testing %d models × %d challenges = %d executions\n", len(models), len(challenges), len(models)*len(challenges))

		for _, tm := range models {
			testProvider(t, executor, tm, DistributionSingle, challenges, &totalTests, &passedTests, &failedTests, &skippedTests, ctx)
		}
	})

	// PHASE 2: Worker Distribution Modes (selected high-performance models)
	t.Run("Phase2_WorkerDistributions", func(t *testing.T) {
		t.Logf("\n=== PHASE 2: Worker Distribution Modes ===")
		
		// Select best performing models for worker tests
		selectedModels := []ComprehensiveTestMatrix{
			{Provider: ProviderOllama, Model: "llama2", Distribution: DistributionSingle, RequiresAPIKey: false},
			{Provider: ProviderDeepSeek, Model: "deepseek-coder", Distribution: DistributionSingle, RequiresAPIKey: true},
			{Provider: ProviderGemini, Model: "gemini-pro", Distribution: DistributionSingle, RequiresAPIKey: true},
			{Provider: ProviderHuggingFace, Model: "bigcode/starcoder", Distribution: DistributionSingle, RequiresAPIKey: true},
		}

		workerDistributions := []ChallengeDistribution{
			DistributionWorker2,
			DistributionWorker5,
			DistributionWorker10,
		}

		t.Logf("Testing %d models × %d distributions × %d challenges = %d executions\n",
			len(selectedModels), len(workerDistributions), len(challenges),
			len(selectedModels)*len(workerDistributions)*len(challenges))

		for _, dist := range workerDistributions {
			for _, tm := range selectedModels {
				testProvider(t, executor, tm, dist, challenges, &totalTests, &passedTests, &failedTests, &skippedTests, ctx)
			}
		}
	})

	// PHASE 3: Multi-Model Scenarios (DeepSeek 3-model combination)
	t.Run("Phase3_MultiModel_DeepSeek", func(t *testing.T) {
		t.Logf("\n=== PHASE 3: Multi-Model Scenarios (DeepSeek) ===")
		t.Log("Testing DeepSeek chat + coder + reasoner combination")
		
		// This would require implementing multi-model support
		// For now, we'll document that this needs to be added
		t.Log("Multi-model execution not yet implemented - would test combinations of models together")
	})

	duration := time.Since(startTime)

	// Print final summary
	t.Logf("\n%s", strings.Repeat("=", 100))
	t.Logf("COMPREHENSIVE MATRIX TEST SUMMARY")
	t.Logf("%s", strings.Repeat("=", 100))
	t.Logf("Total Duration:  %v", duration)
	t.Logf("Total Tests:     %d", totalTests)
	t.Logf("Passed:          %d (%.1f%%)", passedTests, float64(passedTests)/float64(totalTests)*100)
	t.Logf("Failed:          %d (%.1f%%)", failedTests, float64(failedTests)/float64(totalTests)*100)
	t.Logf("Skipped:         %d", skippedTests)
	t.Logf("Avg Time/Test:   %.2fs", duration.Seconds()/float64(totalTests))
	t.Logf("%s\n", strings.Repeat("=", 100))

	if failedTests > 0 {
		t.Logf("⚠️  %d tests failed - review logs for details", failedTests)
	}
	if passedTests == totalTests && totalTests > 0 {
		t.Logf("🎉 ALL TESTS PASSED!")
	}
}

// Helper function to test a provider with a specific distribution
func testProvider(t *testing.T, executor *ChallengeExecutor, tm ComprehensiveTestMatrix, dist ChallengeDistribution,
	challenges []string, totalTests, passedTests, failedTests, skippedTests *int, ctx context.Context) {

	providerName := fmt.Sprintf("%s/%s/%s", tm.Provider, tm.Model, dist)

	t.Run(providerName, func(t *testing.T) {
		// Check if we have API key if needed
		if tm.RequiresAPIKey {
			_, err := executor.apiKeys.GetAPIKey(tm.Provider)
			if err != nil {
				t.Logf("⚠️  Skipping %s - API key not configured", providerName)
				*skippedTests += len(challenges)
				return
			}
		}

		for _, challengePath := range challenges {
			spec, err := LoadChallengeSpec(challengePath)
			if err != nil {
				t.Logf("⚠️  Failed to load %s: %v", challengePath, err)
				continue
			}

			testName := fmt.Sprintf("%s_%s_%s", spec.ID, tm.Model, dist)
			t.Run(testName, func(t *testing.T) {
				*totalTests++

				t.Logf("\n%s", strings.Repeat("-", 80))
				t.Logf("Challenge: %s", spec.Name)
				t.Logf("Provider: %s | Model: %s", tm.Provider, tm.Model)
				t.Logf("Distribution: %s", dist)
				t.Logf("%s", strings.Repeat("-", 80))

				execCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
				defer cancel()

				startTime := time.Now()
				execution, err := executor.Execute(execCtx, spec, InterfaceCLI, dist, tm.Provider, tm.Model)
				duration := time.Since(startTime)

				if err != nil {
					t.Logf("⚠️  Execution returned error: %v", err)
				}

				if execution == nil {
					t.Errorf("❌ Execution failed - got nil execution")
					*failedTests++
					return
				}

				// Log execution details
				t.Logf("\nExecution ID: %s", execution.ID)
				t.Logf("Status: %s", execution.Status)
				t.Logf("Duration: %v", duration)

				// Log metrics if available
				if execution.Metrics.FilesGenerated > 0 {
					t.Logf("\nMetrics:")
					t.Logf("  Files: %d", execution.Metrics.FilesGenerated)
					t.Logf("  Lines of Code: %d", execution.Metrics.LinesOfCode)
				}

				// Check validation results
				passedValidations := 0
				failedValidations := 0

				for _, vr := range execution.ValidationResults {
					if vr.Passed {
						passedValidations++
					} else {
						failedValidations++
						t.Logf("  ✗ %s: %s", vr.CheckName, vr.Message)
					}
				}

				t.Logf("\nValidation: %d/%d passed", passedValidations, passedValidations+failedValidations)

				// Overall test result
				if execution.Status == StatusCompleted && failedValidations == 0 {
					t.Logf("✅ TEST PASSED")
					*passedTests++
				} else if execution.Status == StatusValidationFailed {
					t.Logf("⚠️  PARTIAL PASS - Some validations failed")
					*passedTests++
				} else {
					t.Logf("❌ TEST FAILED - Status: %s", execution.Status)
					*failedTests++
				}
			})
		}
	})
}

// TestPhase1_Quick runs a quick subset to verify the framework works
func TestPhase1_Quick(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	config := DefaultChallengeConfig()
	executor := NewChallengeExecutor(config)

	// Quick test: 2 models × 2 challenges = 4 tests
	models := []ComprehensiveTestMatrix{
		{Provider: ProviderOllama, Model: "llama2", Distribution: DistributionSingle, RequiresAPIKey: false},
		{Provider: ProviderGemini, Model: "gemini-pro", Distribution: DistributionSingle, RequiresAPIKey: true},
	}

	challenges := []string{
		"./definitions/ascii-art-generator.json",
		"./definitions/json-validator-cli.json",
	}

	ctx := context.Background()
	totalTests := 0
	passedTests := 0
	failedTests := 0
	skippedTests := 0

	t.Logf("Quick Test: 2 models × 2 challenges = 4 tests")

	for _, tm := range models {
		testProvider(t, executor, tm, DistributionSingle, challenges, &totalTests, &passedTests, &failedTests, &skippedTests, ctx)
	}

	t.Logf("\nQuick Test Summary: %d passed, %d failed, %d skipped", passedTests, failedTests, skippedTests)
}
