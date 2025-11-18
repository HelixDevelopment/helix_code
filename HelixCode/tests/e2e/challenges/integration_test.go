package challenges

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAllChallenges runs all challenges with Ollama (local provider)
func TestAllChallenges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultChallengeConfig()
	config.ResultsBaseDir = "./test-results/challenges"
	config.LogsBaseDir = "./test-results/logs"

	executor := NewChallengeExecutor(config)

	// Load challenge definitions
	definitionsDir := "./definitions"
	challengeFiles, err := filepath.Glob(filepath.Join(definitionsDir, "*.json"))
	if err != nil {
		t.Fatalf("Failed to find challenge definitions: %v", err)
	}

	if len(challengeFiles) == 0 {
		t.Skip("No challenge definitions found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	for _, file := range challengeFiles {
		spec, err := LoadChallengeSpec(file)
		if err != nil {
			t.Logf("Warning: Failed to load challenge spec %s: %v", file, err)
			continue
		}

		t.Run(spec.ID, func(t *testing.T) {
			t.Logf("Running challenge: %s (%s)", spec.Name, spec.ID)

			execution, err := executor.Execute(ctx, spec, InterfaceCLI, DistributionSingle, ProviderOllama, "llama2")

			if err != nil {
				t.Logf("Challenge execution returned error: %v", err)
			}

			if execution == nil {
				t.Fatal("Expected non-nil execution")
			}

			// Check status
			t.Logf("Execution status: %s", execution.Status)
			t.Logf("Duration: %v", execution.Duration)
			t.Logf("Result directory: %s", execution.ResultDir)

			// Verify result directory exists
			if _, err := os.Stat(execution.ResultDir); os.IsNotExist(err) {
				t.Errorf("Result directory does not exist: %s", execution.ResultDir)
			}

			// Verify logs exist
			if execution.LogFile != "" {
				if _, err := os.Stat(execution.LogFile); os.IsNotExist(err) {
					t.Errorf("Log file does not exist: %s", execution.LogFile)
				}
			}

			// Check validation results
			passedCount := 0
			failedCount := 0
			for _, vr := range execution.ValidationResults {
				if vr.Passed {
					passedCount++
					t.Logf("  ✓ %s: %s", vr.CheckName, vr.Message)
				} else {
					failedCount++
					t.Logf("  ✗ %s: %s", vr.CheckName, vr.Message)
					if vr.Error != "" {
						t.Logf("    Error: %s", vr.Error)
					}
				}
			}

			t.Logf("Validation: %d passed, %d failed", passedCount, failedCount)

			// Don't fail the test if validations fail - just log them
			// This allows us to see all results even if some challenges fail
		})
	}
}

// TestASCIIArtGenerator specifically tests the ASCII art generator challenge
func TestASCIIArtGenerator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultChallengeConfig()
	config.ResultsBaseDir = "./test-results/challenges"
	config.LogsBaseDir = "./test-results/logs"

	executor := NewChallengeExecutor(config)

	spec, err := LoadChallengeSpec("./definitions/ascii-art-generator.json")
	if err != nil {
		t.Fatalf("Failed to load ASCII art challenge spec: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	execution, err := executor.Execute(ctx, spec, InterfaceCLI, DistributionSingle, ProviderOllama, "llama2")

	if err != nil {
		t.Logf("Execution returned error: %v", err)
	}

	if execution == nil {
		t.Fatal("Expected non-nil execution")
	}

	t.Logf("Status: %s", execution.Status)
	t.Logf("Duration: %v", execution.Duration)
	t.Logf("Result directory: %s", execution.ResultDir)

	// Log all validation results
	for _, vr := range execution.ValidationResults {
		if vr.Passed {
			t.Logf("✓ %s", vr.CheckName)
		} else {
			t.Logf("✗ %s: %s", vr.CheckName, vr.Message)
		}
	}
}

// TestTicTacToeGame specifically tests the tic-tac-toe challenge
func TestTicTacToeGame(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultChallengeConfig()
	config.ResultsBaseDir = "./test-results/challenges"
	config.LogsBaseDir = "./test-results/logs"

	executor := NewChallengeExecutor(config)

	spec, err := LoadChallengeSpec("./definitions/tic-tac-toe-tui.json")
	if err != nil {
		t.Fatalf("Failed to load tic-tac-toe challenge spec: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	execution, err := executor.Execute(ctx, spec, InterfaceCLI, DistributionSingle, ProviderOllama, "llama2")

	if err != nil {
		t.Logf("Execution returned error: %v", err)
	}

	if execution == nil {
		t.Fatal("Expected non-nil execution")
	}

	t.Logf("Status: %s", execution.Status)
	t.Logf("Duration: %v", execution.Duration)

	for _, vr := range execution.ValidationResults {
		if vr.Passed {
			t.Logf("✓ %s", vr.CheckName)
		} else {
			t.Logf("✗ %s: %s", vr.CheckName, vr.Message)
		}
	}
}

// TestNotesAPI specifically tests the notes API challenge
func TestNotesAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultChallengeConfig()
	config.ResultsBaseDir = "./test-results/challenges"
	config.LogsBaseDir = "./test-results/logs"

	executor := NewChallengeExecutor(config)

	spec, err := LoadChallengeSpec("./definitions/notes-project.json")
	if err != nil {
		t.Fatalf("Failed to load notes project challenge spec: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	execution, err := executor.Execute(ctx, spec, InterfaceCLI, DistributionSingle, ProviderOllama, "llama2")

	if err != nil {
		t.Logf("Execution returned error: %v", err)
	}

	if execution == nil {
		t.Fatal("Expected non-nil execution")
	}

	t.Logf("Status: %s", execution.Status)
	t.Logf("Duration: %v", execution.Duration)

	for _, vr := range execution.ValidationResults {
		if vr.Passed {
			t.Logf("✓ %s", vr.CheckName)
		} else {
			t.Logf("✗ %s: %s", vr.CheckName, vr.Message)
		}
	}
}

// LoadChallengeSpec loads a challenge specification from a JSON file
func LoadChallengeSpec(path string) (*ChallengeSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var spec ChallengeSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}

	return &spec, nil
}
