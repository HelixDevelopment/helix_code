package challenges

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

	e.log(logFile, "Using mock generator for testing")
	e.log(logFile, fmt.Sprintf("Prompt length: %d characters", len(prompt)))

	// Log request
	e.logRequest(requestLog, "CLI", map[string]interface{}{
		"prompt":   prompt[:min(len(prompt), 500)], // Log first 500 chars
		"provider": execution.Provider,
		"model":    execution.Model,
		"output":   execution.ResultDir,
	})

	startTime := time.Now()

	// Use mock generator for now (TODO: integrate with real HelixCode)
	mockGen := NewMockGenerator()

	var err error
	switch spec.ID {
	case "notes-project-001":
		e.log(logFile, "Generating mock Notes project...")
		err = mockGen.GenerateNotesProject(ctx, execution.ResultDir)
	case "tic-tac-toe-tui-001":
		e.log(logFile, "Generating mock Tic-Tac-Toe TUI game...")
		err = mockGen.GenerateTicTacToeGame(ctx, execution.ResultDir)
	case "ascii-art-generator-001":
		e.log(logFile, "Generating mock ASCII Art Generator...")
		err = mockGen.GenerateASCIIArtGenerator(ctx, execution.ResultDir)
	default:
		err = fmt.Errorf("mock generator not implemented for challenge: %s", spec.ID)
	}

	duration := time.Since(startTime)
	e.log(logFile, fmt.Sprintf("Generation completed in %v", duration))

	if err != nil {
		e.log(logFile, fmt.Sprintf("Generation failed: %v", err))
		return fmt.Errorf("mock generation failed: %w", err)
	}

	// Log response
	e.logResponse(requestLog, "CLI", map[string]interface{}{
		"duration":  duration.String(),
		"mock_mode": true,
		"generated": true,
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

// executeTUI executes challenge via TUI interface
func (e *ChallengeExecutor) executeTUI(ctx context.Context, spec *ChallengeSpec, execution *ChallengeExecution, logFile, requestLog *os.File) error {
	e.log(logFile, "Executing via TUI interface")
	// TUI is interactive, so we'll simulate it via CLI with special flags
	return e.executeCLI(ctx, spec, execution, logFile, requestLog)
}

// executeREST executes challenge via REST API
func (e *ChallengeExecutor) executeREST(ctx context.Context, spec *ChallengeSpec, execution *ChallengeExecution, logFile, requestLog *os.File) error {
	e.log(logFile, "Executing via REST API")

	// Get the prompt
	prompt := spec.Prompt
	if spec.PromptFile != "" {
		content, err := os.ReadFile(spec.PromptFile)
		if err != nil {
			return fmt.Errorf("failed to read prompt file: %w", err)
		}
		prompt = string(content)
	}

	// Prepare API request
	apiURL := fmt.Sprintf("http://%s:%d/api/v1/generate", e.config.HelixCodeHost, e.config.HelixCodePort)

	requestBody := map[string]interface{}{
		"prompt":     prompt,
		"provider":   execution.Provider,
		"model":      execution.Model,
		"output_dir": execution.ResultDir,
		"language":   spec.Language,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	e.log(logFile, fmt.Sprintf("POST %s", apiURL))
	e.logRequest(requestLog, "REST", requestBody)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.config.HelixCodeAuth != "" {
		req.Header.Set("Authorization", e.config.HelixCodeAuth)
	}

	// Send request
	startTime := time.Now()
	resp, err := e.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		e.log(logFile, fmt.Sprintf("Request failed: %v", err))
		return fmt.Errorf("REST API request failed: %w", err)
	}
	defer resp.Body.Close()

	e.log(logFile, fmt.Sprintf("Response status: %d (took %v)", resp.StatusCode, duration))

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Log response
	e.logResponse(requestLog, "REST", map[string]interface{}{
		"status_code":   resp.StatusCode,
		"duration":      duration.String(),
		"response_size": len(responseBody),
		"response_body": string(responseBody),
	})

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-OK status: %d - %s", resp.StatusCode, string(responseBody))
	}

	execution.Metrics.Requests = 1

	return nil
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
