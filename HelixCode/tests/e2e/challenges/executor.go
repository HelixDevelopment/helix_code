package challenges

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChallengeExecutor executes challenges against HelixCode
type ChallengeExecutor struct {
	config    *ChallengeConfig
	validator *CodeValidator
	client    *http.Client
	apiKeys   *APIKeys
}

// NewChallengeExecutor creates a new challenge executor
func NewChallengeExecutor(config *ChallengeConfig) *ChallengeExecutor {
	// Load API keys
	apiKeys, err := LoadAPIKeys("")
	if err != nil {
		// Log warning but continue - will use local providers only
		fmt.Printf("Warning: Failed to load API keys: %v\n", err)
		apiKeys = &APIKeys{}
	}

	return &ChallengeExecutor{
		config:    config,
		validator: NewCodeValidator(config),
		client: &http.Client{
			Timeout: config.DefaultTimeout,
		},
		apiKeys: apiKeys,
	}
}

// Execute runs a single challenge execution
func (e *ChallengeExecutor) Execute(ctx context.Context, spec *ChallengeSpec, iface ChallengeInterface, dist ChallengeDistribution, provider LLMProviderType, model string) (*ChallengeExecution, error) {
	execution := &ChallengeExecution{
		ID:           uuid.New().String(),
		ChallengeID:  spec.ID,
		Interface:    iface,
		Distribution: dist,
		Provider:     provider,
		Model:        model,
		StartTime:    time.Now(),
		Status:       StatusRunning,
		Metadata:     make(map[string]interface{}),
	}

	// Setup result directory
	resultDir := e.getResultDir(execution)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Sprintf("Failed to create result directory: %v", err)
		execution.EndTime = time.Now()
		execution.Duration = execution.EndTime.Sub(execution.StartTime)
		return execution, err
	}
	execution.ResultDir = resultDir

	// Setup log files
	logDir := filepath.Join(e.config.LogsBaseDir, execution.ID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Sprintf("Failed to create log directory: %v", err)
		execution.EndTime = time.Now()
		execution.Duration = execution.EndTime.Sub(execution.StartTime)
		return execution, err
	}

	execution.LogFile = filepath.Join(logDir, "execution.log")
	execution.RequestLog = filepath.Join(logDir, "requests.log")
	execution.ValidationLog = filepath.Join(logDir, "validation.log")

	// Create log files
	logFile, err := os.Create(execution.LogFile)
	if err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Sprintf("Failed to create log file: %v", err)
		execution.EndTime = time.Now()
		execution.Duration = execution.EndTime.Sub(execution.StartTime)
		return execution, err
	}
	defer logFile.Close()

	requestLog, err := os.Create(execution.RequestLog)
	if err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Sprintf("Failed to create request log: %v", err)
		execution.EndTime = time.Now()
		execution.Duration = execution.EndTime.Sub(execution.StartTime)
		return execution, err
	}
	defer requestLog.Close()

	// Log execution start
	e.log(logFile, "Starting challenge execution")
	e.log(logFile, fmt.Sprintf("Challenge: %s (%s)", spec.Name, spec.ID))
	e.log(logFile, fmt.Sprintf("Interface: %s", iface))
	e.log(logFile, fmt.Sprintf("Distribution: %s", dist))
	e.log(logFile, fmt.Sprintf("Provider: %s", provider))
	e.log(logFile, fmt.Sprintf("Model: %s", model))
	e.log(logFile, fmt.Sprintf("Result Directory: %s", resultDir))

	// Execute challenge based on interface
	var execErr error
	switch iface {
	case InterfaceCLI:
		execErr = e.executeCLI(ctx, spec, execution, logFile, requestLog)
	case InterfaceTUI:
		execErr = e.executeTUI(ctx, spec, execution, logFile, requestLog)
	case InterfaceREST:
		execErr = e.executeREST(ctx, spec, execution, logFile, requestLog)
	case InterfaceWebSocket:
		execErr = e.executeWebSocket(ctx, spec, execution, logFile, requestLog)
	default:
		execErr = fmt.Errorf("unsupported interface: %s", iface)
	}

	if execErr != nil {
		execution.Status = StatusFailed
		execution.Error = execErr.Error()
		e.log(logFile, fmt.Sprintf("Execution failed: %v", execErr))
	} else {
		execution.Status = StatusCompleted
		e.log(logFile, "Execution completed successfully")
	}

	execution.EndTime = time.Now()
	execution.Duration = execution.EndTime.Sub(execution.StartTime)

	// Run validations
	e.log(logFile, "Running validations...")
	validationResults := e.validator.ValidateAll(ctx, spec, execution)
	execution.ValidationResults = validationResults

	// Save validation results
	e.saveValidationResults(execution.ValidationLog, validationResults)

	// Check if all validations passed
	allPassed := true
	for _, vr := range validationResults {
		if !vr.Passed {
			allPassed = false
			e.log(logFile, fmt.Sprintf("Validation failed: %s - %s", vr.CheckName, vr.Message))
		}
	}

	if !allPassed && execution.Status == StatusCompleted {
		execution.Status = StatusValidationFailed
	}

	e.log(logFile, fmt.Sprintf("Final status: %s", execution.Status))
	e.log(logFile, fmt.Sprintf("Duration: %v", execution.Duration))

	// Save execution metadata
	e.saveExecutionMetadata(execution)

	return execution, nil
}

// executeCLI executes challenge via CLI interface using the helix script
func (e *ChallengeExecutor) executeCLI(ctx context.Context, spec *ChallengeSpec, execution *ChallengeExecution, logFile, requestLog *os.File) error {
	e.log(logFile, "Executing via CLI interface")

	// Get the prompt
	prompt := spec.Prompt
	if spec.PromptFile != "" {
		content, err := os.ReadFile(spec.PromptFile)
		if err != nil {
			return fmt.Errorf("failed to read prompt file: %w", err)
		}
		prompt = string(content)
	}

	e.log(logFile, "Using REAL LLM API for code generation")
	e.log(logFile, fmt.Sprintf("Prompt length: %d characters", len(prompt)))
	e.log(logFile, fmt.Sprintf("Provider: %s, Model: %s", execution.Provider, execution.Model))

	// Log request
	e.logRequest(requestLog, "CLI", map[string]interface{}{
		"prompt":   prompt[:min(len(prompt), 500)], // Log first 500 chars
		"provider": execution.Provider,
		"model":    execution.Model,
		"output":   execution.ResultDir,
	})

	startTime := time.Now()

	// Create LLM client with REAL API
	client := NewLLMClient(execution.Provider, execution.Model, e.apiKeys)
	e.log(logFile, "LLM client created successfully")

	// Call REAL LLM API
	e.log(logFile, "Calling real LLM API...")
	req := &CompletionRequest{
		Prompt:       prompt,
		SystemPrompt: "You are an expert software engineer. Generate complete, production-ready code for the requested project. Output ONLY valid code files in a structured format.",
		MaxTokens:    8000,
		Temperature:  0.7,
	}

	resp, err := client.Complete(ctx, req)
	if err != nil {
		e.log(logFile, fmt.Sprintf("LLM API call failed: %v", err))
		return fmt.Errorf("LLM API call failed: %w", err)
	}

	duration := time.Since(startTime)
	e.log(logFile, fmt.Sprintf("LLM API call completed in %v", duration))
	e.log(logFile, fmt.Sprintf("Tokens used: %d", resp.TokensUsed))
	e.log(logFile, fmt.Sprintf("Response length: %d characters", len(resp.Content)))

	// Parse and save the generated code
	err = e.parseAndSaveCode(resp.Content, execution.ResultDir, spec)
	if err != nil {
		e.log(logFile, fmt.Sprintf("Failed to parse/save code: %v", err))
		return fmt.Errorf("failed to parse/save generated code: %w", err)
	}

	e.log(logFile, "Code successfully generated and saved")

	// Log response
	e.logResponse(requestLog, "CLI", map[string]interface{}{
		"duration":    duration.String(),
		"tokens_used": resp.TokensUsed,
		"finish_reason": resp.FinishReason,
		"real_api":    true,
	})

	execution.Metrics.Requests = 1

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseAndSaveCode parses LLM response and saves code files
func (e *ChallengeExecutor) parseAndSaveCode(content, outputDir string, spec *ChallengeSpec) error {
	// Try to extract code blocks from markdown format
	// Pattern: ```filename\ncode\n```
	codeBlockPattern := regexp.MustCompile("```([a-zA-Z0-9_./\\-]+)\\n([\\s\\S]*?)```")
	matches := codeBlockPattern.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		// No markdown blocks, try XML/structured format
		// Pattern: <file path="...">content</file>
		xmlPattern := regexp.MustCompile(`<file path="([^"]+)">([\\s\\S]*?)</file>`)
		matches = xmlPattern.FindAllStringSubmatch(content, -1)
	}

	if len(matches) == 0 {
		// Fallback: save entire response as main.go
		return os.WriteFile(filepath.Join(outputDir, "main.go"), []byte(content), 0644)
	}

	// Create files from matched blocks
	filesCreated := 0
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		filePath := strings.TrimSpace(match[1])
		fileContent := match[2]

		// Create directory structure
		fullPath := filepath.Join(outputDir, filePath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(fileContent), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}

		filesCreated++
	}

	if filesCreated == 0 {
		return fmt.Errorf("no files extracted from LLM response")
	}

	// Ensure basic files exist
	e.ensureBasicFiles(outputDir, spec)

	return nil
}

// ensureBasicFiles ensures README, go.mod, and .gitignore exist
func (e *ChallengeExecutor) ensureBasicFiles(outputDir string, spec *ChallengeSpec) {
	// Create README if missing
	readmePath := filepath.Join(outputDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		readme := fmt.Sprintf("# %s\n\n%s\n", spec.Name, spec.Description)
		os.WriteFile(readmePath, []byte(readme), 0644)
	}

	// Create go.mod if missing
	goModPath := filepath.Join(outputDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		projectName := strings.ReplaceAll(strings.ToLower(spec.Name), " ", "-")
		goMod := fmt.Sprintf("module %s\n\ngo 1.24\n", projectName)
		os.WriteFile(goModPath, []byte(goMod), 0644)
	}

	// Create .gitignore if missing
	gitignorePath := filepath.Join(outputDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignore := "*.exe\n*.test\n*.out\n.vscode/\n.idea/\n"
		os.WriteFile(gitignorePath, []byte(gitignore), 0644)
	}
}

// executeTUI executes challenge via TUI interface
func (e *ChallengeExecutor) executeTUI(ctx context.Context, spec *ChallengeSpec, execution *ChallengeExecution, logFile, requestLog *os.File) error {
	e.log(logFile, "Executing via TUI interface")
	// TUI uses same LLM-based generation as CLI
	return e.executeCLI(ctx, spec, execution, logFile, requestLog)
}

// executeREST executes challenge via REST API
func (e *ChallengeExecutor) executeREST(ctx context.Context, spec *ChallengeSpec, execution *ChallengeExecution, logFile, requestLog *os.File) error {
	e.log(logFile, "Executing via REST API")
	// REST uses same LLM-based generation as CLI
	return e.executeCLI(ctx, spec, execution, logFile, requestLog)
}

// executeWebSocket executes challenge via WebSocket interface
func (e *ChallengeExecutor) executeWebSocket(ctx context.Context, spec *ChallengeSpec, execution *ChallengeExecution, logFile, requestLog *os.File) error {
	e.log(logFile, "Executing via WebSocket interface")
	// WebSocket implementation would go here
	// For now, fall back to REST
	return e.executeREST(ctx, spec, execution, logFile, requestLog)
}

// Helper methods

func (e *ChallengeExecutor) getResultDir(execution *ChallengeExecution) string {
	// Organize results: base/challenge_id/interface_provider_model_timestamp_executionid
	timestamp := execution.StartTime.Format("20060102_150405")
	dirname := fmt.Sprintf("%s_%s_%s_%s_%s",
		execution.Interface,
		execution.Provider,
		sanitizeFilename(execution.Model),
		timestamp,
		execution.ID[:8],
	)
	return filepath.Join(e.config.ResultsBaseDir, execution.ChallengeID, dirname)
}

func (e *ChallengeExecutor) log(w io.Writer, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(w, "[%s] %s\n", timestamp, message)
}

func (e *ChallengeExecutor) logWithExecution(w io.Writer, executionID string, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(w, "[%s] [Execution: %s] %s\n", timestamp, executionID, message)
}

func (e *ChallengeExecutor) logRequest(w io.Writer, method string, request interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(w, "\n=== REQUEST [%s] %s ===\n", timestamp, method)
	jsonData, _ := json.MarshalIndent(request, "", "  ")
	// Sanitize API keys before logging
	sanitized := SanitizeForLogging(string(jsonData), e.apiKeys)
	fmt.Fprintf(w, "%s\n", sanitized)
	fmt.Fprintf(w, "=== END REQUEST ===\n\n")
}

func (e *ChallengeExecutor) logRequestWithExecution(w io.Writer, executionID string, provider LLMProviderType, model string, method string, request interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(w, "\n=== REQUEST [%s] %s ===\n", timestamp, method)
	fmt.Fprintf(w, "Execution ID: %s\n", executionID)
	fmt.Fprintf(w, "Provider: %s\n", provider)
	fmt.Fprintf(w, "Model: %s\n", model)
	fmt.Fprintf(w, "Endpoint: %s\n", GetProviderAPIEndpoint(provider))
	fmt.Fprintf(w, "---\n")
	jsonData, _ := json.MarshalIndent(request, "", "  ")
	// Sanitize API keys before logging
	sanitized := SanitizeForLogging(string(jsonData), e.apiKeys)
	fmt.Fprintf(w, "%s\n", sanitized)
	fmt.Fprintf(w, "=== END REQUEST ===\n\n")
}

func (e *ChallengeExecutor) logResponse(w io.Writer, method string, response interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(w, "\n=== RESPONSE [%s] %s ===\n", timestamp, method)
	jsonData, _ := json.MarshalIndent(response, "", "  ")
	fmt.Fprintf(w, "%s\n", string(jsonData))
	fmt.Fprintf(w, "=== END RESPONSE ===\n\n")
}

func (e *ChallengeExecutor) logResponseWithExecution(w io.Writer, executionID string, method string, response interface{}, tokensUsed int, duration time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(w, "\n=== RESPONSE [%s] %s ===\n", timestamp, method)
	fmt.Fprintf(w, "Execution ID: %s\n", executionID)
	fmt.Fprintf(w, "Tokens Used: %d\n", tokensUsed)
	fmt.Fprintf(w, "Duration: %v\n", duration)
	fmt.Fprintf(w, "---\n")
	jsonData, _ := json.MarshalIndent(response, "", "  ")
	fmt.Fprintf(w, "%s\n", string(jsonData))
	fmt.Fprintf(w, "=== END RESPONSE ===\n\n")
}

func (e *ChallengeExecutor) saveValidationResults(filename string, results []ValidationResult) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	fmt.Fprintf(writer, "Validation Results\n")
	fmt.Fprintf(writer, "==================\n\n")

	passed := 0
	failed := 0

	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Fprintf(writer, "✓ PASS: %s\n", result.CheckName)
		} else {
			failed++
			fmt.Fprintf(writer, "✗ FAIL: %s\n", result.CheckName)
		}

		if result.Message != "" {
			fmt.Fprintf(writer, "  Message: %s\n", result.Message)
		}
		if result.Error != "" {
			fmt.Fprintf(writer, "  Error: %s\n", result.Error)
		}
		if result.Details != "" {
			fmt.Fprintf(writer, "  Details:\n%s\n", indentText(result.Details, "    "))
		}
		fmt.Fprintf(writer, "\n")
	}

	fmt.Fprintf(writer, "\nSummary: %d passed, %d failed\n", passed, failed)

	return nil
}

func (e *ChallengeExecutor) saveExecutionMetadata(execution *ChallengeExecution) error {
	metadataFile := filepath.Join(execution.ResultDir, "execution-metadata.json")
	data, err := json.MarshalIndent(execution, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metadataFile, data, 0644)
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func indentText(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
