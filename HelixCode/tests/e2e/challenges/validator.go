package challenges

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CodeValidator validates generated code
type CodeValidator struct {
	config *ChallengeConfig
}

// NewCodeValidator creates a new code validator
func NewCodeValidator(config *ChallengeConfig) *CodeValidator {
	return &CodeValidator{config: config}
}

// ValidateAll runs all validation checks on generated code
func (v *CodeValidator) ValidateAll(ctx context.Context, spec *ChallengeSpec, execution *ChallengeExecution) []ValidationResult {
	results := []ValidationResult{}

	// Check if result directory exists and has content
	results = append(results, v.validateDirectory(execution.ResultDir))

	// Check for placeholder code
	results = append(results, v.validateNoPlaceholders(execution.ResultDir, spec.Language))

	// Check for empty functions/classes
	results = append(results, v.validateNoEmptyImplementations(execution.ResultDir, spec.Language))

	// Check required files
	if spec.Requirements.HasReadme {
		results = append(results, v.validateFileExists(execution.ResultDir, "README.md"))
	}
	if spec.Requirements.HasDockerfile {
		results = append(results, v.validateFileExists(execution.ResultDir, "Dockerfile"))
	}

	// Check compilation
	if spec.Requirements.CompilationCheck && v.config.ValidateCompilation {
		results = append(results, v.validateCompilation(ctx, execution.ResultDir, spec.Language))
	}

	// Check tests
	if spec.Requirements.TestsPass && v.config.ValidateTests {
		results = append(results, v.validateTests(ctx, execution.ResultDir, spec.Language))
	}

	// Check if app runs
	if spec.Requirements.RunCheck && v.config.ValidateRun {
		results = append(results, v.validateRuns(ctx, execution.ResultDir, spec.Language))
	}

	// Validate use cases and documentation
	useCaseValidator := NewUseCaseValidator(v.config)
	results = append(results, useCaseValidator.ValidateUseCases(ctx, spec, execution.ResultDir)...)
	results = append(results, useCaseValidator.ValidateCommonSenseFeatures(ctx, spec, execution.ResultDir)...)

	// Functional validation (only if basic validations passed)
	basicValidationsPassed := true
	for _, r := range results {
		if !r.Passed && (r.CheckName == "compilation" || r.CheckName == "tests_pass") {
			basicValidationsPassed = false
			break
		}
	}

	if basicValidationsPassed {
		functionalValidator := NewFunctionalValidator(v.config)
		results = append(results, functionalValidator.ValidateFunctional(ctx, spec, execution.ResultDir)...)
	}

	// Runtime validation (only if all critical validations passed)
	if basicValidationsPassed && spec.Requirements.RunCheck {
		runtimeValidator := NewRuntimeValidator(v.config)
		results = append(results, runtimeValidator.ValidateRuntime(ctx, spec, execution.ResultDir)...)
	}

	// Count metrics
	metrics := v.calculateMetrics(execution.ResultDir, spec.Language)
	execution.Metrics = metrics

	return results
}

// validateDirectory checks if the result directory exists and has content
func (v *CodeValidator) validateDirectory(resultDir string) ValidationResult {
	result := ValidationResult{
		CheckName: "directory_exists",
		Timestamp: time.Now(),
	}

	info, err := os.Stat(resultDir)
	if err != nil {
		if os.IsNotExist(err) {
			result.Passed = false
			result.Error = "Result directory does not exist"
			return result
		}
		result.Passed = false
		result.Error = fmt.Sprintf("Failed to check directory: %v", err)
		return result
	}

	if !info.IsDir() {
		result.Passed = false
		result.Error = "Result path is not a directory"
		return result
	}

	// Check if directory has any files
	entries, err := os.ReadDir(resultDir)
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Failed to read directory: %v", err)
		return result
	}

	if len(entries) == 0 {
		result.Passed = false
		result.Message = "Directory is empty"
		return result
	}

	result.Passed = true
	result.Message = fmt.Sprintf("Directory exists with %d entries", len(entries))
	return result
}

// validateNoPlaceholders checks for TODO, FIXME, placeholder comments
func (v *CodeValidator) validateNoPlaceholders(resultDir, language string) ValidationResult {
	result := ValidationResult{
		CheckName: "no_placeholders",
		Timestamp: time.Now(),
	}

	placeholderPatterns := []string{
		`TODO[:\s]`,
		`FIXME[:\s]`,
		`XXX[:\s]`,
		`PLACEHOLDER`,
		`NOT\s+IMPLEMENTED`,
		`IMPLEMENT\s+ME`,
		`FILL\s+IN`,
		`YOUR\s+CODE\s+HERE`,
	}

	placeholders := []string{}
	err := filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip common directories
			if strings.Contains(path, "node_modules") || strings.Contains(path, "vendor") ||
				strings.Contains(path, ".git") || strings.Contains(path, "__pycache__") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check only source code files
		if !isSourceFile(path, language) {
			return nil
		}

		found, err := findPatternsInFile(path, placeholderPatterns)
		if err != nil {
			return err
		}

		placeholders = append(placeholders, found...)
		return nil
	})

	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Failed to check for placeholders: %v", err)
		return result
	}

	if len(placeholders) > 0 {
		result.Passed = false
		result.Message = fmt.Sprintf("Found %d placeholders/TODOs", len(placeholders))
		result.Details = strings.Join(placeholders, "\n")
		return result
	}

	result.Passed = true
	result.Message = "No placeholders found"
	return result
}

// validateNoEmptyImplementations checks for empty functions, classes, etc.
func (v *CodeValidator) validateNoEmptyImplementations(resultDir, language string) ValidationResult {
	result := ValidationResult{
		CheckName: "no_empty_implementations",
		Timestamp: time.Now(),
	}

	emptyPatterns := getEmptyPatterns(language)
	if len(emptyPatterns) == 0 {
		result.Passed = true
		result.Message = "Language not supported for empty implementation check"
		return result
	}

	emptyFuncs := []string{}
	err := filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.Contains(path, "node_modules") || strings.Contains(path, "vendor") ||
				strings.Contains(path, ".git") {
				return filepath.SkipDir
			}
			return nil
		}

		if !isSourceFile(path, language) {
			return nil
		}

		found, err := findPatternsInFile(path, emptyPatterns)
		if err != nil {
			return err
		}

		emptyFuncs = append(emptyFuncs, found...)
		return nil
	})

	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Failed to check for empty implementations: %v", err)
		return result
	}

	if len(emptyFuncs) > 0 {
		result.Passed = false
		result.Message = fmt.Sprintf("Found %d empty implementations", len(emptyFuncs))
		result.Details = strings.Join(emptyFuncs, "\n")
		return result
	}

	result.Passed = true
	result.Message = "No empty implementations found"
	return result
}

// validateFileExists checks if a required file exists
func (v *CodeValidator) validateFileExists(resultDir, filename string) ValidationResult {
	result := ValidationResult{
		CheckName: fmt.Sprintf("file_exists_%s", filename),
		Timestamp: time.Now(),
	}

	filePath := filepath.Join(resultDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		result.Passed = false
		result.Message = fmt.Sprintf("Required file %s not found", filename)
		return result
	}

	result.Passed = true
	result.Message = fmt.Sprintf("File %s exists", filename)
	return result
}

// validateCompilation checks if the code compiles
func (v *CodeValidator) validateCompilation(ctx context.Context, resultDir, language string) ValidationResult {
	result := ValidationResult{
		CheckName: "compilation",
		Timestamp: time.Now(),
	}

	startTime := time.Now()
	cmd, err := getCompileCommand(resultDir, language)
	if err != nil {
		result.Passed = false
		result.Error = err.Error()
		return result
	}

	if cmd == nil {
		result.Passed = true
		result.Message = "Compilation not applicable for this language"
		return result
	}

	// Enforce a 30-second timeout for compilation
	compileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd = exec.CommandContext(compileCtx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = resultDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	compilationTime := time.Since(startTime)

	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Compilation failed: %v", err)
		result.Details = fmt.Sprintf("STDOUT:\n%s\n\nSTDERR:\n%s", stdout.String(), stderr.String())
		return result
	}

	result.Passed = true
	result.Message = fmt.Sprintf("Compilation successful (took %v)", compilationTime)
	return result
}

// validateTests checks if tests pass
func (v *CodeValidator) validateTests(ctx context.Context, resultDir, language string) ValidationResult {
	result := ValidationResult{
		CheckName: "tests_pass",
		Timestamp: time.Now(),
	}

	startTime := time.Now()
	cmd, err := getTestCommand(resultDir, language)
	if err != nil {
		result.Passed = false
		result.Error = err.Error()
		return result
	}

	if cmd == nil {
		result.Passed = true
		result.Message = "Tests not found or not applicable"
		return result
	}

	// Enforce a 30-second timeout for tests
	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd = exec.CommandContext(testCtx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = resultDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	testTime := time.Since(startTime)

	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Tests failed: %v", err)
		result.Details = fmt.Sprintf("STDOUT:\n%s\n\nSTDERR:\n%s", stdout.String(), stderr.String())
		return result
	}

	result.Passed = true
	result.Message = fmt.Sprintf("Tests passed (took %v)", testTime)
	return result
}

// validateRuns checks if the application runs
func (v *CodeValidator) validateRuns(ctx context.Context, resultDir, language string) ValidationResult {
	result := ValidationResult{
		CheckName: "runs_successfully",
		Timestamp: time.Now(),
	}

	cmd, err := getRunCommand(resultDir, language)
	if err != nil {
		result.Passed = false
		result.Error = err.Error()
		return result
	}

	if cmd == nil {
		result.Passed = true
		result.Message = "Run check not applicable"
		return result
	}

	// Run with timeout
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmdWithCtx := exec.CommandContext(runCtx, cmd.Path, cmd.Args[1:]...)
	cmdWithCtx.Dir = cmd.Dir

	var stdout, stderr bytes.Buffer
	cmdWithCtx.Stdout = &stdout
	cmdWithCtx.Stderr = &stderr

	err = cmdWithCtx.Start()
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Failed to start: %v", err)
		return result
	}

	// Wait a bit to see if it crashes immediately
	time.Sleep(2 * time.Second)

	// Check if still running
	if cmdWithCtx.Process != nil {
		// Kill the process
		cmdWithCtx.Process.Kill()
		// Wait with a timeout to avoid hanging on child processes
		done := make(chan error, 1)
		go func() { done <- cmdWithCtx.Wait() }()
		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			// Timed out waiting for process cleanup
		}

		result.Passed = true
		result.Message = "Application started successfully"
		return result
	}

	result.Passed = false
	result.Error = "Application exited immediately"
	result.Details = fmt.Sprintf("STDOUT:\n%s\n\nSTDERR:\n%s", stdout.String(), stderr.String())
	return result
}

// calculateMetrics calculates code metrics
func (v *CodeValidator) calculateMetrics(resultDir, language string) ExecutionMetrics {
	metrics := ExecutionMetrics{}

	filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if strings.Contains(path, "node_modules") || strings.Contains(path, "vendor") {
			return filepath.SkipDir
		}

		metrics.FilesGenerated++

		if isSourceFile(path, language) {
			loc, err := countLOC(path)
			if err == nil {
				metrics.LinesOfCode += loc
			}
		}

		return nil
	})

	return metrics
}

// Helper functions

func isSourceFile(path, language string) bool {
	ext := filepath.Ext(path)
	switch language {
	case "go":
		return ext == ".go"
	case "python":
		return ext == ".py"
	case "javascript", "typescript":
		return ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx"
	case "java":
		return ext == ".java"
	case "rust":
		return ext == ".rs"
	case "c", "cpp":
		return ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".hpp"
	default:
		return false
	}
}

func findPatternsInFile(path string, patterns []string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, pattern := range patterns {
			matched, _ := regexp.MatchString(pattern, line)
			if matched {
				relPath := path
				matches = append(matches, fmt.Sprintf("%s:%d: %s", relPath, lineNum, strings.TrimSpace(line)))
				break
			}
		}
	}

	return matches, scanner.Err()
}

func getEmptyPatterns(language string) []string {
	switch language {
	case "go":
		return []string{
			`func\s+\w+\([^)]*\)\s*\([^)]*\)\s*\{\s*\}`, // Empty function with return
			`func\s+\w+\([^)]*\)\s*\{\s*\}`,             // Empty function
		}
	case "python":
		return []string{
			`def\s+\w+\([^)]*\):\s*pass`,
		}
	case "javascript", "typescript":
		return []string{
			`function\s+\w+\([^)]*\)\s*\{\s*\}`,
			`\w+\([^)]*\)\s*\{\s*\}`,
		}
	case "java":
		return []string{
			`(public|private|protected)?\s*(static)?\s*\w+\s+\w+\([^)]*\)\s*\{\s*\}`,
		}
	default:
		return []string{}
	}
}

func getCompileCommand(resultDir, language string) (*exec.Cmd, error) {
	switch language {
	case "go":
		// Check if go.mod exists
		if _, err := os.Stat(filepath.Join(resultDir, "go.mod")); err == nil {
			cmd := exec.Command("go", "build", "-tags", "nogui", "./...")
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, fmt.Errorf("no go.mod found")

	case "java":
		// Look for pom.xml or build.gradle
		if _, err := os.Stat(filepath.Join(resultDir, "pom.xml")); err == nil {
			cmd := exec.Command("mvn", "compile")
			cmd.Dir = resultDir
			return cmd, nil
		}
		if _, err := os.Stat(filepath.Join(resultDir, "build.gradle")); err == nil {
			cmd := exec.Command("gradle", "build")
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, fmt.Errorf("no build configuration found")

	case "rust":
		if _, err := os.Stat(filepath.Join(resultDir, "Cargo.toml")); err == nil {
			cmd := exec.Command("cargo", "build")
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, fmt.Errorf("no Cargo.toml found")

	case "typescript":
		if _, err := os.Stat(filepath.Join(resultDir, "tsconfig.json")); err == nil {
			cmd := exec.Command("tsc")
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, nil // TypeScript is optional

	default:
		return nil, nil // Compilation not needed
	}
}

func getTestCommand(resultDir, language string) (*exec.Cmd, error) {
	switch language {
	case "go":
		cmd := exec.Command("go", "test", "-v", "./...")
		cmd.Dir = resultDir
		return cmd, nil

	case "python":
		// Try pytest first, then unittest
		if _, err := exec.LookPath("pytest"); err == nil {
			cmd := exec.Command("pytest", "-v")
			cmd.Dir = resultDir
			return cmd, nil
		}
		cmd := exec.Command("python", "-m", "unittest", "discover")
		cmd.Dir = resultDir
		return cmd, nil

	case "javascript", "typescript":
		// Check package.json for test script
		if _, err := os.Stat(filepath.Join(resultDir, "package.json")); err == nil {
			cmd := exec.Command("npm", "test")
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, nil

	case "java":
		if _, err := os.Stat(filepath.Join(resultDir, "pom.xml")); err == nil {
			cmd := exec.Command("mvn", "test")
			cmd.Dir = resultDir
			return cmd, nil
		}
		if _, err := os.Stat(filepath.Join(resultDir, "build.gradle")); err == nil {
			cmd := exec.Command("gradle", "test")
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, nil

	case "rust":
		cmd := exec.Command("cargo", "test")
		cmd.Dir = resultDir
		return cmd, nil

	default:
		return nil, nil
	}
}

func getRunCommand(resultDir, language string) (*exec.Cmd, error) {
	switch language {
	case "go":
		// Find main package
		mainPath := findMainPackage(resultDir)
		if mainPath != "" {
			cmd := exec.Command("go", "run", mainPath)
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, fmt.Errorf("no main package found")

	case "python":
		// Look for main.py or __main__.py
		for _, name := range []string{"main.py", "__main__.py", "app.py"} {
			if _, err := os.Stat(filepath.Join(resultDir, name)); err == nil {
				cmd := exec.Command("python", name)
				cmd.Dir = resultDir
				return cmd, nil
			}
		}
		return nil, fmt.Errorf("no entry point found")

	case "javascript":
		// Look for package.json start script
		if _, err := os.Stat(filepath.Join(resultDir, "package.json")); err == nil {
			cmd := exec.Command("npm", "start")
			cmd.Dir = resultDir
			return cmd, nil
		}
		return nil, nil

	default:
		return nil, nil
	}
}

func findMainPackage(dir string) string {
	var mainPath string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || mainPath != "" {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".go" {
			content, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(content), "package main") {
				mainPath = path
			}
		}
		return nil
	})
	return mainPath
}

func countLOC(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Count non-empty, non-comment lines
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") {
			count++
		}
	}

	return count, scanner.Err()
}
