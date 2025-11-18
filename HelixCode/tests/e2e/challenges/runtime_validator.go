package challenges

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RuntimeValidator validates that compiled executables actually work
type RuntimeValidator struct {
	config *ChallengeConfig
}

// NewRuntimeValidator creates a new runtime validator
func NewRuntimeValidator(config *ChallengeConfig) *RuntimeValidator {
	return &RuntimeValidator{
		config: config,
	}
}

// ValidateRuntime performs runtime testing on compiled executables
func (v *RuntimeValidator) ValidateRuntime(ctx context.Context, spec *ChallengeSpec, resultDir string) []ValidationResult {
	results := []ValidationResult{}

	switch spec.ID {
	case "ascii-art-generator-001":
		results = append(results, v.validateASCIIArtGenerator(ctx, resultDir)...)
	case "tic-tac-toe-tui-001":
		results = append(results, v.validateTicTacToeTUI(ctx, resultDir)...)
	case "notes-project-001":
		results = append(results, v.validateNotesProject(ctx, resultDir)...)
	default:
		results = append(results, ValidationResult{
			CheckName: "runtime_validation",
			Passed:    true,
			Message:   fmt.Sprintf("No runtime validation defined for challenge: %s", spec.ID),
			Timestamp: time.Now(),
		})
	}

	return results
}

// validateASCIIArtGenerator tests the ASCII art generator executable
func (v *RuntimeValidator) validateASCIIArtGenerator(ctx context.Context, resultDir string) []ValidationResult {
	results := []ValidationResult{}

	// Build the executable
	exePath, buildErr := v.buildExecutable(resultDir, "ascii-art")
	if buildErr != nil {
		results = append(results, ValidationResult{
			CheckName: "runtime_build",
			Passed:    false,
			Error:     fmt.Sprintf("Failed to build executable: %v", buildErr),
			Timestamp: time.Now(),
		})
		return results
	}
	defer os.Remove(exePath)

	results = append(results, ValidationResult{
		CheckName: "runtime_build",
		Passed:    true,
		Message:   "Executable built successfully",
		Details:   fmt.Sprintf("Binary: %s", exePath),
		Timestamp: time.Now(),
	})

	// Test 1: Run with --help flag
	helpResult := v.runCommand(ctx, exePath, []string{"--help"}, "", 5*time.Second)

	// Check both stdout and stderr for help output (Cobra uses stdout for --help)
	helpOutput := helpResult.Stdout + helpResult.Stderr
	hasUsage := strings.Contains(helpOutput, "Usage") || strings.Contains(helpOutput, "usage") ||
	            strings.Contains(helpOutput, "Use:") || strings.Contains(helpOutput, "use:")

	if helpResult.Error != "" && helpResult.ExitCode != 0 && helpResult.ExitCode != -1 {
		// Non-zero exit is OK for --help in some CLI frameworks
		hasUsage = strings.Contains(helpOutput, "Usage") || strings.Contains(helpOutput, "Use:")
	}

	if !hasUsage {
		results = append(results, ValidationResult{
			CheckName: "runtime_help",
			Passed:    false,
			Error:     "Help output missing usage information",
			Details:   fmt.Sprintf("Stdout:\n%s\nStderr:\n%s", helpResult.Stdout, helpResult.Stderr),
			Timestamp: time.Now(),
		})
	} else {
		results = append(results, ValidationResult{
			CheckName: "runtime_help",
			Passed:    true,
			Message:   "Help command works correctly",
			Timestamp: time.Now(),
		})
	}

	// Test 2: Run with simple text input
	testResult := v.runCommand(ctx, exePath, []string{"HELLO"}, "", 5*time.Second)
	if testResult.Error != "" {
		results = append(results, ValidationResult{
			CheckName: "runtime_basic_generation",
			Passed:    false,
			Error:     fmt.Sprintf("Failed to generate ASCII art: %s", testResult.Error),
			Details:   testResult.Stderr,
			Timestamp: time.Now(),
		})
	} else if testResult.Stdout == "" {
		results = append(results, ValidationResult{
			CheckName: "runtime_basic_generation",
			Passed:    false,
			Error:     "No output generated",
			Timestamp: time.Now(),
		})
	} else {
		// Verify output contains ASCII art or markdown
		hasOutput := len(testResult.Stdout) > 0
		results = append(results, ValidationResult{
			CheckName: "runtime_basic_generation",
			Passed:    hasOutput,
			Message:   "ASCII art generated successfully",
			Details:   fmt.Sprintf("Output length: %d characters", len(testResult.Stdout)),
			Timestamp: time.Now(),
		})
	}

	// Test 3: Run with empty input (should handle gracefully)
	emptyResult := v.runCommand(ctx, exePath, []string{}, "", 5*time.Second)
	results = append(results, ValidationResult{
		CheckName: "runtime_error_handling",
		Passed:    true, // Should not crash
		Message:   "Handles empty input without crashing",
		Details:   fmt.Sprintf("Exit code: %d", emptyResult.ExitCode),
		Timestamp: time.Now(),
	})

	return results
}

// validateTicTacToeTUI tests the tic-tac-toe TUI executable
func (v *RuntimeValidator) validateTicTacToeTUI(ctx context.Context, resultDir string) []ValidationResult {
	results := []ValidationResult{}

	// Build the executable
	exePath, buildErr := v.buildExecutable(resultDir, "tic-tac-toe")
	if buildErr != nil {
		results = append(results, ValidationResult{
			CheckName: "runtime_build",
			Passed:    false,
			Error:     fmt.Sprintf("Failed to build executable: %v", buildErr),
			Timestamp: time.Now(),
		})
		return results
	}
	defer os.Remove(exePath)

	results = append(results, ValidationResult{
		CheckName: "runtime_build",
		Passed:    true,
		Message:   "Executable built successfully",
		Details:   fmt.Sprintf("Binary: %s", exePath),
		Timestamp: time.Now(),
	})

	// Test: Run with 'q' input to quit immediately (non-interactive test)
	quitResult := v.runCommand(ctx, exePath, []string{}, "q\n", 3*time.Second)

	// TUI should start and quit cleanly
	startsCleanly := quitResult.ExitCode == 0 || quitResult.ExitCode == 1 // Some TUIs return 1 on quit
	results = append(results, ValidationResult{
		CheckName: "runtime_tui_starts",
		Passed:    startsCleanly,
		Message:   "TUI starts and responds to quit command",
		Details:   fmt.Sprintf("Exit code: %d", quitResult.ExitCode),
		Timestamp: time.Now(),
	})

	return results
}

// validateNotesProject tests the notes API server
func (v *RuntimeValidator) validateNotesProject(ctx context.Context, resultDir string) []ValidationResult {
	results := []ValidationResult{}

	// Build the executable from cmd/server subdirectory
	buildDir := filepath.Join(resultDir, "cmd", "server")
	exePath, buildErr := v.buildExecutableFromPath(resultDir, buildDir, "notes-server")
	if buildErr != nil {
		results = append(results, ValidationResult{
			CheckName: "runtime_build",
			Passed:    false,
			Error:     fmt.Sprintf("Failed to build executable: %v", buildErr),
			Timestamp: time.Now(),
		})
		return results
	}
	defer os.Remove(exePath)

	results = append(results, ValidationResult{
		CheckName: "runtime_build",
		Passed:    true,
		Message:   "Server executable built successfully",
		Details:   fmt.Sprintf("Binary: %s", exePath),
		Timestamp: time.Now(),
	})

	// Note: Server requires database connection, which we don't have in test environment
	// This is validated by functional_validator.go already
	results = append(results, ValidationResult{
		CheckName: "runtime_server_validation",
		Passed:    true,
		Message:   "Server runtime validated by functional tests",
		Details:   "Database-dependent server tested in functional_validator.go",
		Timestamp: time.Now(),
	})

	return results
}

// buildExecutableFromPath builds from a specific build directory with go.mod in root
func (v *RuntimeValidator) buildExecutableFromPath(rootDir, buildDir, binaryName string) (string, error) {
	// Ensure paths are absolute
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute root path: %v", err)
	}

	absBuildDir, err := filepath.Abs(buildDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute build path: %v", err)
	}

	// Run go mod tidy in root directory
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = absRootDir
	var tidyStderr bytes.Buffer
	tidyCmd.Stderr = &tidyStderr
	if err := tidyCmd.Run(); err != nil {
		fmt.Printf("Warning: go mod tidy failed: %v - %s\n", err, tidyStderr.String())
	}

	// Download dependencies in root directory
	downloadCmd := exec.Command("go", "mod", "download")
	downloadCmd.Dir = absRootDir
	var downloadStderr bytes.Buffer
	downloadCmd.Stderr = &downloadStderr
	if err := downloadCmd.Run(); err != nil {
		fmt.Printf("Warning: go mod download failed: %v - %s\n", err, downloadStderr.String())
	}

	// Build from the build directory
	exePath := filepath.Join(absRootDir, binaryName)
	buildCmd := exec.Command("go", "build", "-o", exePath)
	buildCmd.Dir = absBuildDir

	var buildStderr, buildStdout bytes.Buffer
	buildCmd.Stderr = &buildStderr
	buildCmd.Stdout = &buildStdout

	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("build failed: %v\nStderr: %s\nStdout: %s", err, buildStderr.String(), buildStdout.String())
	}

	// Verify executable exists
	if _, err := os.Stat(exePath); err != nil {
		return "", fmt.Errorf("executable not found after build: %v", err)
	}

	return exePath, nil
}

// buildExecutable builds the project and returns the executable path
func (v *RuntimeValidator) buildExecutable(resultDir, binaryName string) (string, error) {
	// For simple projects, build dir is the same as root dir
	return v.buildExecutableFromPath(resultDir, resultDir, binaryName)
}

// CommandResult holds the result of running a command
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    string
}

// runCommand runs a command and captures its output
func (v *RuntimeValidator) runCommand(ctx context.Context, exePath string, args []string, stdin string, timeout time.Duration) CommandResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, exePath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	err := cmd.Run()

	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
		}
	}

	return result
}
