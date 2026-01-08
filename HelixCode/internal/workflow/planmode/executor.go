package planmode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/google/uuid"
)

// ExecutionResult contains the result of plan execution
type ExecutionResult struct {
	ID           string
	PlanID       string
	Success      bool
	Steps        []*StepResult
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	FilesChanged []string
	Errors       []error
	Metrics      *ExecutionMetrics
}

// ExecutionProgress tracks execution progress
type ExecutionProgress struct {
	ExecutionID        string
	CurrentStep        int
	TotalSteps         int
	CompletedSteps     int
	FailedSteps        int
	SkippedSteps       int
	ElapsedTime        time.Duration
	EstimatedRemaining time.Duration
	Status             string
}

// ExecutionMetrics contains execution metrics
type ExecutionMetrics struct {
	StepsCompleted   int
	StepsFailed      int
	FilesModified    int
	FilesCreated     int
	FilesDeleted     int
	LinesChanged     int
	CommandsExecuted int
	Errors           int
	Warnings         int
}

// Executor executes plans
type Executor interface {
	// Execute executes a plan
	Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error)

	// ExecuteStep executes a single step
	ExecuteStep(ctx context.Context, step *PlanStep) (*StepResult, error)

	// Pause pauses execution
	Pause(executionID string) error

	// Resume resumes execution
	Resume(executionID string) error

	// Cancel cancels execution
	Cancel(executionID string) error

	// GetProgress returns execution progress
	GetProgress(executionID string) (*ExecutionProgress, error)
}

// DefaultExecutor implements Executor
type DefaultExecutor struct {
	executions      sync.Map // map[string]*activeExecution
	progressTracker *ProgressTracker
	workspaceRoot   string
	llmProvider     llm.Provider
}

// activeExecution tracks an active execution
type activeExecution struct {
	result   *ExecutionResult
	progress *ExecutionProgress
	ctx      context.Context
	cancel   context.CancelFunc
	paused   bool
	mu       sync.RWMutex
}

// NewDefaultExecutor creates a new default executor
func NewDefaultExecutor(workspaceRoot string) *DefaultExecutor {
	return &DefaultExecutor{
		progressTracker: NewProgressTracker(),
		workspaceRoot:   workspaceRoot,
	}
}

// NewDefaultExecutorWithLLM creates a new default executor with LLM support
func NewDefaultExecutorWithLLM(workspaceRoot string, provider llm.Provider) *DefaultExecutor {
	return &DefaultExecutor{
		progressTracker: NewProgressTracker(),
		workspaceRoot:   workspaceRoot,
		llmProvider:     provider,
	}
}

// SetLLMProvider sets the LLM provider for the executor
func (e *DefaultExecutor) SetLLMProvider(provider llm.Provider) {
	e.llmProvider = provider
}

// Execute executes a plan
func (e *DefaultExecutor) Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error) {
	executionID := uuid.New().String()
	execCtx, cancel := context.WithCancel(ctx)

	result := &ExecutionResult{
		ID:        executionID,
		PlanID:    plan.ID,
		StartTime: time.Now(),
		Steps:     make([]*StepResult, 0),
		Errors:    make([]error, 0),
		Metrics:   &ExecutionMetrics{},
	}

	progress := &ExecutionProgress{
		ExecutionID: executionID,
		TotalSteps:  len(plan.Steps),
		Status:      "Starting execution",
	}

	active := &activeExecution{
		result:   result,
		progress: progress,
		ctx:      execCtx,
		cancel:   cancel,
		paused:   false,
	}

	e.executions.Store(executionID, active)
	defer e.executions.Delete(executionID)

	// Execute steps
	for i, step := range plan.Steps {
		// Check for cancellation
		select {
		case <-execCtx.Done():
			result.Success = false
			result.Errors = append(result.Errors, fmt.Errorf("execution cancelled"))
			e.finalizeExecution(result)
			return result, fmt.Errorf("execution cancelled")
		default:
		}

		// Check if paused
		active.mu.RLock()
		for active.paused {
			active.mu.RUnlock()
			time.Sleep(100 * time.Millisecond)
			active.mu.RLock()
		}
		active.mu.RUnlock()

		// Check dependencies
		if !e.areDependenciesSatisfied(plan, step, result) {
			step.Status = StepSkipped
			progress.SkippedSteps++
			progress.Status = fmt.Sprintf("Skipped: %s (dependencies not met)", step.Title)
			continue
		}

		// Update progress
		progress.CurrentStep = i + 1
		progress.Status = fmt.Sprintf("Executing: %s", step.Title)
		step.Status = StepInProgress

		// Execute step
		stepResult, err := e.ExecuteStep(execCtx, step)
		if err != nil {
			result.Errors = append(result.Errors, err)
			result.Metrics.Errors++
			step.Status = StepFailed
			progress.FailedSteps++
		} else if stepResult.Success {
			step.Status = StepCompleted
			progress.CompletedSteps++
			result.Metrics.StepsCompleted++
		} else {
			step.Status = StepFailed
			progress.FailedSteps++
			result.Metrics.StepsFailed++
		}

		step.Result = stepResult
		result.Steps = append(result.Steps, stepResult)

		// Track files changed
		if stepResult.FilesChanged != nil {
			result.FilesChanged = append(result.FilesChanged, stepResult.FilesChanged...)
			result.Metrics.FilesModified += len(stepResult.FilesChanged)
		}

		// Update elapsed time
		progress.ElapsedTime = time.Since(result.StartTime)

		// Estimate remaining time
		if progress.CompletedSteps > 0 {
			avgTimePerStep := progress.ElapsedTime / time.Duration(progress.CompletedSteps)
			remainingSteps := progress.TotalSteps - progress.CurrentStep
			progress.EstimatedRemaining = avgTimePerStep * time.Duration(remainingSteps)
		}
	}

	e.finalizeExecution(result)
	return result, nil
}

// ExecuteStep executes a single step
func (e *DefaultExecutor) ExecuteStep(ctx context.Context, step *PlanStep) (*StepResult, error) {
	startTime := time.Now()
	result := &StepResult{
		Success: false,
		Metrics: make(map[string]interface{}),
	}

	switch step.Type {
	case StepTypeFileOperation:
		err := e.executeFileOperation(ctx, step, result)
		result.Success = err == nil
		result.Error = err

	case StepTypeShellCommand:
		err := e.executeShellCommand(ctx, step, result)
		result.Success = err == nil
		result.Error = err

	case StepTypeCodeGeneration:
		err := e.executeCodeGeneration(ctx, step, result)
		result.Success = err == nil
		result.Error = err

	case StepTypeCodeAnalysis:
		err := e.executeCodeAnalysis(ctx, step, result)
		result.Success = err == nil
		result.Error = err

	case StepTypeValidation:
		err := e.executeValidation(ctx, step, result)
		result.Success = err == nil
		result.Error = err

	case StepTypeTesting:
		err := e.executeTesting(ctx, step, result)
		result.Success = err == nil
		result.Error = err

	default:
		result.Error = fmt.Errorf("unknown step type: %s", step.Type)
	}

	result.Duration = time.Since(startTime)
	return result, result.Error
}

// Pause pauses execution
func (e *DefaultExecutor) Pause(executionID string) error {
	val, ok := e.executions.Load(executionID)
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	active := val.(*activeExecution)
	active.mu.Lock()
	defer active.mu.Unlock()

	if active.paused {
		return fmt.Errorf("execution already paused")
	}

	active.paused = true
	active.progress.Status = "Paused"
	return nil
}

// Resume resumes execution
func (e *DefaultExecutor) Resume(executionID string) error {
	val, ok := e.executions.Load(executionID)
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	active := val.(*activeExecution)
	active.mu.Lock()
	defer active.mu.Unlock()

	if !active.paused {
		return fmt.Errorf("execution not paused")
	}

	active.paused = false
	active.progress.Status = "Resumed"
	return nil
}

// Cancel cancels execution
func (e *DefaultExecutor) Cancel(executionID string) error {
	val, ok := e.executions.Load(executionID)
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	active := val.(*activeExecution)
	active.cancel()
	active.progress.Status = "Cancelled"
	return nil
}

// GetProgress returns execution progress
func (e *DefaultExecutor) GetProgress(executionID string) (*ExecutionProgress, error) {
	val, ok := e.executions.Load(executionID)
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	active := val.(*activeExecution)
	active.mu.RLock()
	defer active.mu.RUnlock()

	// Return a copy
	progress := &ExecutionProgress{
		ExecutionID:        active.progress.ExecutionID,
		CurrentStep:        active.progress.CurrentStep,
		TotalSteps:         active.progress.TotalSteps,
		CompletedSteps:     active.progress.CompletedSteps,
		FailedSteps:        active.progress.FailedSteps,
		SkippedSteps:       active.progress.SkippedSteps,
		ElapsedTime:        active.progress.ElapsedTime,
		EstimatedRemaining: active.progress.EstimatedRemaining,
		Status:             active.progress.Status,
	}

	return progress, nil
}

// areDependenciesSatisfied checks if step dependencies are satisfied
func (e *DefaultExecutor) areDependenciesSatisfied(plan *Plan, step *PlanStep, result *ExecutionResult) bool {
	if len(step.Dependencies) == 0 {
		return true
	}

	for _, depID := range step.Dependencies {
		found := false
		for _, s := range plan.Steps {
			if s.ID == depID {
				if s.Status != StepCompleted {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// finalizeExecution finalizes an execution result
func (e *DefaultExecutor) finalizeExecution(result *ExecutionResult) {
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = len(result.Errors) == 0 && result.Metrics.StepsFailed == 0
}

// Step execution implementations

func (e *DefaultExecutor) executeFileOperation(ctx context.Context, step *PlanStep, result *StepResult) error {
	// Parse the file operation from step.Action
	// Expected format: "operation:path[:content]" where operation is create, write, delete, move, copy
	// Examples:
	//   "create:/path/to/file.go"
	//   "write:/path/to/file.go:content here"
	//   "delete:/path/to/file.go"
	//   "move:/src/path:/dst/path"
	//   "copy:/src/path:/dst/path"

	parts := strings.SplitN(step.Action, ":", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid file operation format: expected 'operation:path', got '%s'", step.Action)
	}

	operation := strings.TrimSpace(strings.ToLower(parts[0]))
	targetPath := strings.TrimSpace(parts[1])

	// Resolve path relative to workspace root
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(e.workspaceRoot, targetPath)
	}

	// Validate path is within workspace (security check)
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	absWorkspace, err := filepath.Abs(e.workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace: %w", err)
	}

	if !strings.HasPrefix(absPath, absWorkspace) {
		return fmt.Errorf("path '%s' is outside workspace", targetPath)
	}

	switch operation {
	case "create", "write":
		// Get content from parts[2] or step.Description
		content := ""
		if len(parts) >= 3 {
			content = parts[2]
		} else {
			content = step.Description
		}

		// Create parent directory if needed
		parentDir := filepath.Dir(absPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", parentDir, err)
		}

		// Write the file
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file '%s': %w", absPath, err)
		}

		result.Output = fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), absPath)
		result.FilesChanged = []string{absPath}

	case "delete":
		// Check if file exists
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				result.Output = fmt.Sprintf("File '%s' does not exist, skipping delete", absPath)
				return nil
			}
			return fmt.Errorf("failed to stat file '%s': %w", absPath, err)
		}

		// Delete file or directory
		if info.IsDir() {
			if err := os.RemoveAll(absPath); err != nil {
				return fmt.Errorf("failed to delete directory '%s': %w", absPath, err)
			}
		} else {
			if err := os.Remove(absPath); err != nil {
				return fmt.Errorf("failed to delete file '%s': %w", absPath, err)
			}
		}

		result.Output = fmt.Sprintf("Successfully deleted %s", absPath)
		result.FilesChanged = []string{absPath}

	case "move", "rename":
		if len(parts) < 3 {
			return fmt.Errorf("move operation requires source and destination paths")
		}
		dstPath := strings.TrimSpace(parts[2])
		if !filepath.IsAbs(dstPath) {
			dstPath = filepath.Join(e.workspaceRoot, dstPath)
		}

		absDst, err := filepath.Abs(dstPath)
		if err != nil {
			return fmt.Errorf("failed to resolve destination path: %w", err)
		}

		// Validate destination is within workspace
		if !strings.HasPrefix(absDst, absWorkspace) {
			return fmt.Errorf("destination path '%s' is outside workspace", dstPath)
		}

		// Create parent directory for destination if needed
		dstParent := filepath.Dir(absDst)
		if err := os.MkdirAll(dstParent, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		if err := os.Rename(absPath, absDst); err != nil {
			return fmt.Errorf("failed to move '%s' to '%s': %w", absPath, absDst, err)
		}

		result.Output = fmt.Sprintf("Successfully moved %s to %s", absPath, absDst)
		result.FilesChanged = []string{absPath, absDst}

	case "copy":
		if len(parts) < 3 {
			return fmt.Errorf("copy operation requires source and destination paths")
		}
		dstPath := strings.TrimSpace(parts[2])
		if !filepath.IsAbs(dstPath) {
			dstPath = filepath.Join(e.workspaceRoot, dstPath)
		}

		absDst, err := filepath.Abs(dstPath)
		if err != nil {
			return fmt.Errorf("failed to resolve destination path: %w", err)
		}

		// Validate destination is within workspace
		if !strings.HasPrefix(absDst, absWorkspace) {
			return fmt.Errorf("destination path '%s' is outside workspace", dstPath)
		}

		// Read source file
		content, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to read source file '%s': %w", absPath, err)
		}

		// Create parent directory for destination if needed
		dstParent := filepath.Dir(absDst)
		if err := os.MkdirAll(dstParent, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		// Write destination file
		srcInfo, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("failed to stat source file: %w", err)
		}

		if err := os.WriteFile(absDst, content, srcInfo.Mode()); err != nil {
			return fmt.Errorf("failed to write destination file '%s': %w", absDst, err)
		}

		result.Output = fmt.Sprintf("Successfully copied %s to %s (%d bytes)", absPath, absDst, len(content))
		result.FilesChanged = []string{absDst}

	case "mkdir":
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", absPath, err)
		}
		result.Output = fmt.Sprintf("Successfully created directory %s", absPath)
		result.FilesChanged = []string{absPath}

	default:
		return fmt.Errorf("unknown file operation: %s", operation)
	}

	return nil
}

func (e *DefaultExecutor) executeShellCommand(ctx context.Context, step *PlanStep, result *StepResult) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", step.Action)
	cmd.Dir = e.workspaceRoot

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

func (e *DefaultExecutor) executeCodeGeneration(ctx context.Context, step *PlanStep, result *StepResult) error {
	// Check if LLM provider is available
	if e.llmProvider == nil {
		result.Output = fmt.Sprintf("Code generation skipped (no LLM provider): %s", step.Description)
		return nil
	}

	// Check if LLM is available
	if !e.llmProvider.IsAvailable(ctx) {
		result.Output = fmt.Sprintf("Code generation skipped (LLM unavailable): %s", step.Description)
		return nil
	}

	// Build the code generation prompt
	systemPrompt := `You are an expert software developer. Generate clean, well-documented, production-ready code.
Follow best practices for the language and project type.
Include comments explaining complex logic.
Provide complete, runnable code without placeholders.
Return only the code, wrapped in appropriate markdown code blocks.`

	userPrompt := fmt.Sprintf(`Generate code for the following task:

Task: %s

Description: %s

Workspace: %s

Please generate the complete code implementation.`, step.Title, step.Description, e.workspaceRoot)

	// Create LLM request
	request := &llm.LLMRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   8192,
		Temperature: 0.3,
	}

	// Execute LLM call
	response, err := e.llmProvider.Generate(ctx, request)
	if err != nil {
		return fmt.Errorf("LLM code generation failed: %w", err)
	}

	// Extract generated code
	generatedCode := response.Content

	// If step.Action specifies a file path, write the code to that file
	if step.Action != "" && strings.HasPrefix(step.Action, "file:") {
		filePath := strings.TrimPrefix(step.Action, "file:")
		filePath = strings.TrimSpace(filePath)

		// Resolve path relative to workspace
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(e.workspaceRoot, filePath)
		}

		// Validate path is within workspace
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		}

		absWorkspace, err := filepath.Abs(e.workspaceRoot)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace: %w", err)
		}

		if !strings.HasPrefix(absPath, absWorkspace) {
			return fmt.Errorf("file path '%s' is outside workspace", filePath)
		}

		// Extract code from markdown code blocks if present
		code := extractCodeFromMarkdown(generatedCode)

		// Create parent directory if needed
		parentDir := filepath.Dir(absPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Write the code to file
		if err := os.WriteFile(absPath, []byte(code), 0644); err != nil {
			return fmt.Errorf("failed to write generated code to file: %w", err)
		}

		result.Output = fmt.Sprintf("Generated code written to %s (%d bytes)", absPath, len(code))
		result.FilesChanged = []string{absPath}
	} else {
		result.Output = generatedCode
	}

	// Record metrics
	result.Metrics["tokens_used"] = response.Usage.TotalTokens
	result.Metrics["prompt_tokens"] = response.Usage.PromptTokens
	result.Metrics["completion_tokens"] = response.Usage.CompletionTokens

	return nil
}

// extractCodeFromMarkdown extracts code from markdown code blocks
func extractCodeFromMarkdown(content string) string {
	// Check if content contains markdown code blocks
	if !strings.Contains(content, "```") {
		return content
	}

	// Find all code blocks
	var codeBlocks []string
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	var currentBlock strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				codeBlocks = append(codeBlocks, currentBlock.String())
				currentBlock.Reset()
				inCodeBlock = false
			} else {
				// Start of code block
				inCodeBlock = true
			}
		} else if inCodeBlock {
			if currentBlock.Len() > 0 {
				currentBlock.WriteString("\n")
			}
			currentBlock.WriteString(line)
		}
	}

	// If we have code blocks, return them joined
	if len(codeBlocks) > 0 {
		return strings.Join(codeBlocks, "\n\n")
	}

	// No code blocks found, return original content
	return content
}

func (e *DefaultExecutor) executeCodeAnalysis(ctx context.Context, step *PlanStep, result *StepResult) error {
	// Determine target files/directories to analyze
	targetPath := step.Action
	if targetPath == "" {
		targetPath = e.workspaceRoot
	}

	// Resolve path relative to workspace
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(e.workspaceRoot, targetPath)
	}

	// Collect file information for analysis
	var analysisOutput strings.Builder
	var filesAnalyzed []string
	var totalLines int
	var totalFiles int

	// Walk the target path and collect file stats
	err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip hidden directories and common non-code directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a source file
		ext := filepath.Ext(path)
		if !isSourceFile(ext) {
			return nil
		}

		// Count lines in the file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lineCount := len(strings.Split(string(content), "\n"))
		totalLines += lineCount
		totalFiles++

		relPath, _ := filepath.Rel(e.workspaceRoot, path)
		filesAnalyzed = append(filesAnalyzed, relPath)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Build static analysis summary
	analysisOutput.WriteString("# Code Analysis Report\n\n")
	analysisOutput.WriteString(fmt.Sprintf("## Summary\n"))
	analysisOutput.WriteString(fmt.Sprintf("- Target: %s\n", targetPath))
	analysisOutput.WriteString(fmt.Sprintf("- Total Files: %d\n", totalFiles))
	analysisOutput.WriteString(fmt.Sprintf("- Total Lines: %d\n", totalLines))
	analysisOutput.WriteString(fmt.Sprintf("- Analysis Task: %s\n\n", step.Description))

	// Group files by extension
	extCount := make(map[string]int)
	for _, f := range filesAnalyzed {
		ext := filepath.Ext(f)
		extCount[ext]++
	}

	analysisOutput.WriteString("## File Types\n")
	for ext, count := range extCount {
		analysisOutput.WriteString(fmt.Sprintf("- %s: %d files\n", ext, count))
	}
	analysisOutput.WriteString("\n")

	// If LLM is available, perform deeper analysis
	if e.llmProvider != nil && e.llmProvider.IsAvailable(ctx) {
		// Read sample files for LLM analysis (limit to avoid context overflow)
		var sampleContent strings.Builder
		maxSampleFiles := 5
		sampledCount := 0

		for _, relPath := range filesAnalyzed {
			if sampledCount >= maxSampleFiles {
				break
			}

			absPath := filepath.Join(e.workspaceRoot, relPath)
			content, err := os.ReadFile(absPath)
			if err != nil || len(content) > 10000 {
				continue
			}

			sampleContent.WriteString(fmt.Sprintf("\n--- %s ---\n", relPath))
			sampleContent.WriteString(string(content))
			sampledCount++
		}

		// Perform LLM analysis
		systemPrompt := `You are an expert code analyst. Analyze the provided code and provide insights on:
1. Code quality and organization
2. Potential bugs or issues
3. Architecture and design patterns
4. Suggestions for improvement
5. Security considerations

Be specific and actionable in your recommendations.`

		userPrompt := fmt.Sprintf(`Analyze the following code for: %s

Files analyzed: %d
Total lines: %d

Sample code:
%s

Please provide a comprehensive analysis.`, step.Description, totalFiles, totalLines, sampleContent.String())

		request := &llm.LLMRequest{
			Messages: []llm.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
			MaxTokens:   4096,
			Temperature: 0.2,
		}

		response, err := e.llmProvider.Generate(ctx, request)
		if err == nil {
			analysisOutput.WriteString("## LLM Analysis\n\n")
			analysisOutput.WriteString(response.Content)
			analysisOutput.WriteString("\n")

			result.Metrics["llm_tokens_used"] = response.Usage.TotalTokens
		}
	} else {
		analysisOutput.WriteString("## Notes\n")
		analysisOutput.WriteString("- LLM analysis not available. Configure an LLM provider for deeper analysis.\n")
	}

	// Try to run language-specific static analysis tools
	toolResults := e.runStaticAnalysisTools(ctx, targetPath)
	if toolResults != "" {
		analysisOutput.WriteString("\n## Static Analysis Tool Results\n")
		analysisOutput.WriteString(toolResults)
	}

	result.Output = analysisOutput.String()
	result.Metrics["files_analyzed"] = totalFiles
	result.Metrics["total_lines"] = totalLines

	return nil
}

// isSourceFile checks if a file extension indicates a source code file
func isSourceFile(ext string) bool {
	sourceExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".java": true, ".kt": true, ".rs": true, ".c": true, ".cpp": true, ".h": true,
		".hpp": true, ".rb": true, ".php": true, ".swift": true, ".scala": true, ".cs": true,
		".vue": true, ".svelte": true, ".sh": true, ".bash": true, ".yaml": true, ".yml": true,
		".json": true, ".xml": true, ".sql": true, ".graphql": true, ".proto": true,
	}
	return sourceExts[ext]
}

// runStaticAnalysisTools runs available static analysis tools
func (e *DefaultExecutor) runStaticAnalysisTools(ctx context.Context, targetPath string) string {
	var results strings.Builder

	// Detect project type and run appropriate tools
	projectType := detectProjectType(targetPath)

	switch projectType {
	case "go":
		// Run go vet
		cmd := exec.CommandContext(ctx, "go", "vet", "./...")
		cmd.Dir = targetPath
		output, err := cmd.CombinedOutput()
		if err == nil || len(output) > 0 {
			results.WriteString("### Go Vet\n```\n")
			if len(output) == 0 {
				results.WriteString("No issues found\n")
			} else {
				results.WriteString(string(output))
			}
			results.WriteString("```\n\n")
		}

		// Run golangci-lint if available
		cmd = exec.CommandContext(ctx, "golangci-lint", "run", "--fast")
		cmd.Dir = targetPath
		output, err = cmd.CombinedOutput()
		if err == nil || len(output) > 0 {
			results.WriteString("### GolangCI-Lint\n```\n")
			results.WriteString(string(output))
			results.WriteString("```\n\n")
		}

	case "node":
		// Run eslint if available
		cmd := exec.CommandContext(ctx, "npx", "eslint", ".", "--format", "compact")
		cmd.Dir = targetPath
		output, _ := cmd.CombinedOutput()
		if len(output) > 0 {
			results.WriteString("### ESLint\n```\n")
			results.WriteString(string(output))
			results.WriteString("```\n\n")
		}

	case "python":
		// Run flake8 if available
		cmd := exec.CommandContext(ctx, "flake8", ".")
		cmd.Dir = targetPath
		output, _ := cmd.CombinedOutput()
		if len(output) > 0 {
			results.WriteString("### Flake8\n```\n")
			results.WriteString(string(output))
			results.WriteString("```\n\n")
		}

		// Run pylint if available
		cmd = exec.CommandContext(ctx, "pylint", "--output-format=text", ".")
		cmd.Dir = targetPath
		output, _ = cmd.CombinedOutput()
		if len(output) > 0 {
			results.WriteString("### Pylint\n```\n")
			results.WriteString(string(output))
			results.WriteString("```\n\n")
		}

	case "rust":
		// Run cargo clippy
		cmd := exec.CommandContext(ctx, "cargo", "clippy", "--", "-W", "clippy::all")
		cmd.Dir = targetPath
		output, _ := cmd.CombinedOutput()
		if len(output) > 0 {
			results.WriteString("### Cargo Clippy\n```\n")
			results.WriteString(string(output))
			results.WriteString("```\n\n")
		}
	}

	return results.String()
}

// detectProjectType detects the type of project based on marker files
func detectProjectType(path string) string {
	markers := map[string]string{
		"go.mod":          "go",
		"package.json":    "node",
		"requirements.txt": "python",
		"setup.py":        "python",
		"pyproject.toml":  "python",
		"Cargo.toml":      "rust",
		"pom.xml":         "java",
		"build.gradle":    "java",
	}

	for file, projectType := range markers {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			return projectType
		}
	}

	return "unknown"
}

func (e *DefaultExecutor) executeValidation(ctx context.Context, step *PlanStep, result *StepResult) error {
	// Execute validation checks based on step configuration
	// step.Action can be:
	//   - "syntax" - syntax validation
	//   - "build" - build validation (compile check)
	//   - "lint" - lint validation
	//   - "format" - format validation
	//   - "deps" - dependency validation
	//   - custom command

	validationType := step.Action
	if validationType == "" {
		validationType = "build" // Default to build validation
	}

	var validationOutput strings.Builder
	validationOutput.WriteString(fmt.Sprintf("# Validation Report: %s\n\n", step.Description))

	projectType := detectProjectType(e.workspaceRoot)
	validationPassed := true
	var validationErrors []string

	switch validationType {
	case "syntax", "build":
		// Run build/syntax check based on project type
		switch projectType {
		case "go":
			// Go syntax/build check
			cmd := exec.CommandContext(ctx, "go", "build", "-o", "/dev/null", "./...")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Go build failed: %s", string(output)))
			} else {
				validationOutput.WriteString("## Go Build: PASSED\n")
			}

		case "node":
			// TypeScript/JavaScript syntax check
			// Try tsc first for TypeScript projects
			if _, err := os.Stat(filepath.Join(e.workspaceRoot, "tsconfig.json")); err == nil {
				cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
				cmd.Dir = e.workspaceRoot
				output, err := cmd.CombinedOutput()
				if err != nil {
					validationPassed = false
					validationErrors = append(validationErrors, fmt.Sprintf("TypeScript check failed: %s", string(output)))
				} else {
					validationOutput.WriteString("## TypeScript Check: PASSED\n")
				}
			} else {
				// Try npm run build
				cmd := exec.CommandContext(ctx, "npm", "run", "build", "--if-present")
				cmd.Dir = e.workspaceRoot
				output, err := cmd.CombinedOutput()
				if err != nil {
					validationPassed = false
					validationErrors = append(validationErrors, fmt.Sprintf("Build failed: %s", string(output)))
				} else {
					validationOutput.WriteString("## Build: PASSED\n")
					validationOutput.WriteString(string(output))
				}
			}

		case "python":
			// Python syntax check using py_compile
			cmd := exec.CommandContext(ctx, "python", "-m", "py_compile")
			cmd.Dir = e.workspaceRoot

			// Find Python files and compile them
			var pyFiles []string
			filepath.Walk(e.workspaceRoot, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if filepath.Ext(path) == ".py" {
					relPath, _ := filepath.Rel(e.workspaceRoot, path)
					// Skip virtual environments and cache
					if !strings.Contains(relPath, "venv") && !strings.Contains(relPath, "__pycache__") {
						pyFiles = append(pyFiles, path)
					}
				}
				return nil
			})

			for _, pyFile := range pyFiles {
				cmd := exec.CommandContext(ctx, "python", "-m", "py_compile", pyFile)
				output, err := cmd.CombinedOutput()
				if err != nil {
					validationPassed = false
					relPath, _ := filepath.Rel(e.workspaceRoot, pyFile)
					validationErrors = append(validationErrors, fmt.Sprintf("Syntax error in %s: %s", relPath, string(output)))
				}
			}
			if validationPassed {
				validationOutput.WriteString(fmt.Sprintf("## Python Syntax Check: PASSED (%d files)\n", len(pyFiles)))
			}

		case "rust":
			// Rust build check
			cmd := exec.CommandContext(ctx, "cargo", "check")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Cargo check failed: %s", string(output)))
			} else {
				validationOutput.WriteString("## Cargo Check: PASSED\n")
			}

		default:
			validationOutput.WriteString("## Build Validation: SKIPPED (unknown project type)\n")
		}

	case "lint":
		// Run linting based on project type
		toolResults := e.runStaticAnalysisTools(ctx, e.workspaceRoot)
		validationOutput.WriteString("## Lint Results\n")
		validationOutput.WriteString(toolResults)

	case "format":
		// Check code formatting
		switch projectType {
		case "go":
			// Check gofmt
			cmd := exec.CommandContext(ctx, "gofmt", "-l", ".")
			cmd.Dir = e.workspaceRoot
			output, _ := cmd.CombinedOutput()
			if len(strings.TrimSpace(string(output))) > 0 {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Files need formatting:\n%s", string(output)))
			} else {
				validationOutput.WriteString("## Go Format: PASSED\n")
			}

		case "node":
			// Check prettier if available
			cmd := exec.CommandContext(ctx, "npx", "prettier", "--check", ".")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Format check failed: %s", string(output)))
			} else {
				validationOutput.WriteString("## Prettier Check: PASSED\n")
			}

		case "python":
			// Check black formatting
			cmd := exec.CommandContext(ctx, "black", "--check", ".")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Format check failed: %s", string(output)))
			} else {
				validationOutput.WriteString("## Black Format: PASSED\n")
			}

		case "rust":
			// Check rustfmt
			cmd := exec.CommandContext(ctx, "cargo", "fmt", "--", "--check")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Format check failed: %s", string(output)))
			} else {
				validationOutput.WriteString("## Rust Format: PASSED\n")
			}
		}

	case "deps":
		// Validate dependencies
		switch projectType {
		case "go":
			// Check go mod
			cmd := exec.CommandContext(ctx, "go", "mod", "verify")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Go mod verify failed: %s", string(output)))
			} else {
				validationOutput.WriteString("## Go Mod Verify: PASSED\n")
			}

			// Check for missing/unused dependencies
			cmd = exec.CommandContext(ctx, "go", "mod", "tidy", "-v")
			cmd.Dir = e.workspaceRoot
			output, _ = cmd.CombinedOutput()
			if len(strings.TrimSpace(string(output))) > 0 {
				validationOutput.WriteString(fmt.Sprintf("### Module Changes:\n%s\n", string(output)))
			}

		case "node":
			// Check npm dependencies
			cmd := exec.CommandContext(ctx, "npm", "ls", "--all")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				// npm ls returns non-zero for peer dependency issues, which may not be fatal
				validationOutput.WriteString(fmt.Sprintf("### NPM Dependencies (with warnings):\n%s\n", string(output)))
			} else {
				validationOutput.WriteString("## NPM Dependencies: OK\n")
			}

		case "python":
			// Check pip requirements
			if _, err := os.Stat(filepath.Join(e.workspaceRoot, "requirements.txt")); err == nil {
				cmd := exec.CommandContext(ctx, "pip", "check")
				cmd.Dir = e.workspaceRoot
				output, err := cmd.CombinedOutput()
				if err != nil {
					validationPassed = false
					validationErrors = append(validationErrors, fmt.Sprintf("Pip check failed: %s", string(output)))
				} else {
					validationOutput.WriteString("## Pip Check: PASSED\n")
				}
			}

		case "rust":
			// Check cargo dependencies
			cmd := exec.CommandContext(ctx, "cargo", "verify-project")
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				validationPassed = false
				validationErrors = append(validationErrors, fmt.Sprintf("Cargo verify-project failed: %s", string(output)))
			} else {
				validationOutput.WriteString("## Cargo Verify: PASSED\n")
			}
		}

	default:
		// Run as custom command
		cmd := exec.CommandContext(ctx, "sh", "-c", validationType)
		cmd.Dir = e.workspaceRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			validationPassed = false
			validationErrors = append(validationErrors, fmt.Sprintf("Custom validation failed: %s", string(output)))
		} else {
			validationOutput.WriteString(fmt.Sprintf("## Custom Validation: PASSED\n%s\n", string(output)))
		}
	}

	// Add error summary
	if !validationPassed {
		validationOutput.WriteString("\n## Validation FAILED\n\n### Errors:\n")
		for _, errMsg := range validationErrors {
			validationOutput.WriteString(fmt.Sprintf("- %s\n", errMsg))
		}
		result.Output = validationOutput.String()
		result.Metrics["validation_passed"] = false
		result.Metrics["error_count"] = len(validationErrors)
		return fmt.Errorf("validation failed with %d error(s)", len(validationErrors))
	}

	validationOutput.WriteString("\n## Validation PASSED\n")
	result.Output = validationOutput.String()
	result.Metrics["validation_passed"] = true

	return nil
}

func (e *DefaultExecutor) executeTesting(ctx context.Context, step *PlanStep, result *StepResult) error {
	// Execute tests based on step configuration
	// step.Action can specify:
	//   - specific test pattern (e.g., "TestSomething")
	//   - test directory/file
	//   - "all" for all tests
	//   - "coverage" for tests with coverage
	//   - "integration" for integration tests
	//   - "unit" for unit tests only

	testMode := step.Action
	if testMode == "" {
		testMode = "all" // Default to running all tests
	}

	var testOutput strings.Builder
	testOutput.WriteString(fmt.Sprintf("# Test Execution Report: %s\n\n", step.Description))

	projectType := detectProjectType(e.workspaceRoot)
	var testsPassed bool
	var testResults TestResults

	switch projectType {
	case "go":
		testsPassed, testResults = e.runGoTests(ctx, testMode)

	case "node":
		testsPassed, testResults = e.runNodeTests(ctx, testMode)

	case "python":
		testsPassed, testResults = e.runPythonTests(ctx, testMode)

	case "rust":
		testsPassed, testResults = e.runRustTests(ctx, testMode)

	default:
		testOutput.WriteString("## Warning: Unknown project type\n")
		testOutput.WriteString("Cannot determine test command. Please specify a custom test command in the step action.\n")

		// Try to run step action as custom command if provided
		if testMode != "all" && testMode != "" {
			cmd := exec.CommandContext(ctx, "sh", "-c", testMode)
			cmd.Dir = e.workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				testOutput.WriteString(fmt.Sprintf("## Test Command Output:\n```\n%s\n```\n", string(output)))
				testOutput.WriteString(fmt.Sprintf("\n## Tests FAILED: %v\n", err))
				result.Output = testOutput.String()
				return fmt.Errorf("tests failed: %w", err)
			}
			testOutput.WriteString(fmt.Sprintf("## Test Command Output:\n```\n%s\n```\n", string(output)))
			testOutput.WriteString("\n## Tests PASSED\n")
			result.Output = testOutput.String()
			return nil
		}

		result.Output = testOutput.String()
		return nil
	}

	// Build test report
	testOutput.WriteString(fmt.Sprintf("## Test Summary\n"))
	testOutput.WriteString(fmt.Sprintf("- Project Type: %s\n", projectType))
	testOutput.WriteString(fmt.Sprintf("- Test Mode: %s\n", testMode))
	testOutput.WriteString(fmt.Sprintf("- Total Tests: %d\n", testResults.Total))
	testOutput.WriteString(fmt.Sprintf("- Passed: %d\n", testResults.Passed))
	testOutput.WriteString(fmt.Sprintf("- Failed: %d\n", testResults.Failed))
	testOutput.WriteString(fmt.Sprintf("- Skipped: %d\n", testResults.Skipped))
	if testResults.Coverage > 0 {
		testOutput.WriteString(fmt.Sprintf("- Coverage: %.1f%%\n", testResults.Coverage))
	}
	testOutput.WriteString(fmt.Sprintf("- Duration: %s\n", testResults.Duration))
	testOutput.WriteString("\n")

	// Add test output
	testOutput.WriteString("## Test Output\n```\n")
	testOutput.WriteString(testResults.Output)
	testOutput.WriteString("\n```\n")

	// Record metrics
	result.Metrics["tests_total"] = testResults.Total
	result.Metrics["tests_passed"] = testResults.Passed
	result.Metrics["tests_failed"] = testResults.Failed
	result.Metrics["tests_skipped"] = testResults.Skipped
	result.Metrics["test_coverage"] = testResults.Coverage
	result.Metrics["test_duration_ms"] = testResults.Duration.Milliseconds()

	if !testsPassed {
		testOutput.WriteString("\n## Tests FAILED\n")
		if len(testResults.FailedTests) > 0 {
			testOutput.WriteString("### Failed Tests:\n")
			for _, t := range testResults.FailedTests {
				testOutput.WriteString(fmt.Sprintf("- %s\n", t))
			}
		}
		result.Output = testOutput.String()
		return fmt.Errorf("tests failed: %d test(s) failed", testResults.Failed)
	}

	testOutput.WriteString("\n## Tests PASSED\n")
	result.Output = testOutput.String()
	return nil
}

// TestResults holds test execution results
type TestResults struct {
	Total       int
	Passed      int
	Failed      int
	Skipped     int
	Coverage    float64
	Duration    time.Duration
	Output      string
	FailedTests []string
}

// runGoTests runs Go tests
func (e *DefaultExecutor) runGoTests(ctx context.Context, mode string) (bool, TestResults) {
	results := TestResults{}
	startTime := time.Now()

	var args []string
	switch mode {
	case "coverage":
		args = []string{"test", "-v", "-cover", "-coverprofile=coverage.out", "./..."}
	case "unit":
		args = []string{"test", "-v", "-short", "./..."}
	case "integration":
		args = []string{"test", "-v", "-run", "Integration", "./..."}
	case "all":
		args = []string{"test", "-v", "./..."}
	default:
		// Assume mode is a test pattern or package
		args = []string{"test", "-v", "-run", mode, "./..."}
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = e.workspaceRoot
	output, err := cmd.CombinedOutput()
	results.Output = string(output)
	results.Duration = time.Since(startTime)

	// Parse Go test output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "--- PASS:") {
			results.Passed++
			results.Total++
		} else if strings.Contains(line, "--- FAIL:") {
			results.Failed++
			results.Total++
			// Extract test name
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				testName := strings.TrimSpace(strings.TrimPrefix(parts[0], "--- FAIL"))
				results.FailedTests = append(results.FailedTests, testName)
			}
		} else if strings.Contains(line, "--- SKIP:") {
			results.Skipped++
			results.Total++
		} else if strings.Contains(line, "coverage:") && strings.Contains(line, "%") {
			// Extract coverage percentage
			parts := strings.Split(line, "coverage:")
			if len(parts) > 1 {
				coverStr := strings.TrimSpace(parts[1])
				coverStr = strings.Split(coverStr, "%")[0]
				var coverage float64
				fmt.Sscanf(coverStr, "%f", &coverage)
				if coverage > results.Coverage {
					results.Coverage = coverage
				}
			}
		}
	}

	return err == nil, results
}

// runNodeTests runs Node.js tests
func (e *DefaultExecutor) runNodeTests(ctx context.Context, mode string) (bool, TestResults) {
	results := TestResults{}
	startTime := time.Now()

	var args []string
	switch mode {
	case "coverage":
		args = []string{"test", "--", "--coverage"}
	case "unit":
		args = []string{"test", "--", "--testPathPattern=unit"}
	case "integration":
		args = []string{"test", "--", "--testPathPattern=integration"}
	case "all":
		args = []string{"test"}
	default:
		args = []string{"test", "--", "--testNamePattern=" + mode}
	}

	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = e.workspaceRoot
	output, err := cmd.CombinedOutput()
	results.Output = string(output)
	results.Duration = time.Since(startTime)

	// Parse Jest/Mocha output (basic parsing)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Tests:") {
			// Parse Jest summary line like "Tests: 5 passed, 1 failed, 6 total"
			if strings.Contains(line, "passed") {
				parts := strings.Split(line, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if strings.Contains(part, "passed") {
						fmt.Sscanf(part, "%d passed", &results.Passed)
					} else if strings.Contains(part, "failed") {
						fmt.Sscanf(part, "%d failed", &results.Failed)
					} else if strings.Contains(part, "skipped") {
						fmt.Sscanf(part, "%d skipped", &results.Skipped)
					} else if strings.Contains(part, "total") {
						fmt.Sscanf(part, "%d total", &results.Total)
					}
				}
			}
		} else if strings.Contains(line, "FAIL ") {
			// Extract failed test
			testName := strings.TrimPrefix(strings.TrimSpace(line), "FAIL ")
			results.FailedTests = append(results.FailedTests, testName)
		} else if strings.Contains(line, "All files") && strings.Contains(line, "|") {
			// Parse coverage line
			parts := strings.Split(line, "|")
			if len(parts) >= 2 {
				coverStr := strings.TrimSpace(parts[1])
				fmt.Sscanf(coverStr, "%f", &results.Coverage)
			}
		}
	}

	return err == nil, results
}

// runPythonTests runs Python tests
func (e *DefaultExecutor) runPythonTests(ctx context.Context, mode string) (bool, TestResults) {
	results := TestResults{}
	startTime := time.Now()

	var args []string
	switch mode {
	case "coverage":
		args = []string{"-m", "pytest", "-v", "--cov=.", "--cov-report=term-missing"}
	case "unit":
		args = []string{"-m", "pytest", "-v", "-m", "not integration"}
	case "integration":
		args = []string{"-m", "pytest", "-v", "-m", "integration"}
	case "all":
		args = []string{"-m", "pytest", "-v"}
	default:
		args = []string{"-m", "pytest", "-v", "-k", mode}
	}

	cmd := exec.CommandContext(ctx, "python", args...)
	cmd.Dir = e.workspaceRoot
	output, err := cmd.CombinedOutput()
	results.Output = string(output)
	results.Duration = time.Since(startTime)

	// Parse pytest output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Parse summary line like "5 passed, 1 failed, 2 skipped in 1.23s"
		if strings.Contains(line, "passed") || strings.Contains(line, "failed") {
			if strings.Contains(line, " in ") {
				parts := strings.Split(line, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if strings.Contains(part, "passed") {
						fmt.Sscanf(part, "%d passed", &results.Passed)
					} else if strings.Contains(part, "failed") {
						fmt.Sscanf(part, "%d failed", &results.Failed)
					} else if strings.Contains(part, "skipped") {
						fmt.Sscanf(part, "%d skipped", &results.Skipped)
					}
				}
				results.Total = results.Passed + results.Failed + results.Skipped
			}
		} else if strings.Contains(line, "FAILED ") {
			// Extract failed test name
			testName := strings.TrimPrefix(strings.TrimSpace(line), "FAILED ")
			results.FailedTests = append(results.FailedTests, testName)
		} else if strings.Contains(line, "TOTAL") && strings.Contains(line, "%") {
			// Parse coverage
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.HasSuffix(part, "%") && i > 0 {
					coverStr := strings.TrimSuffix(part, "%")
					fmt.Sscanf(coverStr, "%f", &results.Coverage)
					break
				}
			}
		}
	}

	return err == nil, results
}

// runRustTests runs Rust tests
func (e *DefaultExecutor) runRustTests(ctx context.Context, mode string) (bool, TestResults) {
	results := TestResults{}
	startTime := time.Now()

	var args []string
	switch mode {
	case "coverage":
		// For Rust coverage, we'd use cargo-tarpaulin or llvm-cov
		args = []string{"test", "--", "--test-threads=1"}
	case "unit":
		args = []string{"test", "--lib"}
	case "integration":
		args = []string{"test", "--test", "*"}
	case "all":
		args = []string{"test"}
	default:
		args = []string{"test", mode}
	}

	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = e.workspaceRoot
	output, err := cmd.CombinedOutput()
	results.Output = string(output)
	results.Duration = time.Since(startTime)

	// Parse cargo test output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Parse summary line like "test result: ok. 5 passed; 1 failed; 2 ignored; 0 measured; 0 filtered out"
		if strings.Contains(line, "test result:") {
			if strings.Contains(line, "passed") {
				parts := strings.Split(line, ";")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if strings.Contains(part, "passed") {
						fmt.Sscanf(part, "%d passed", &results.Passed)
					} else if strings.Contains(part, "failed") {
						fmt.Sscanf(part, "%d failed", &results.Failed)
					} else if strings.Contains(part, "ignored") {
						fmt.Sscanf(part, "%d ignored", &results.Skipped)
					}
				}
				results.Total = results.Passed + results.Failed + results.Skipped
			}
		} else if strings.Contains(line, "test ") && strings.Contains(line, "... FAILED") {
			// Extract failed test name
			parts := strings.Split(line, "...")
			if len(parts) > 0 {
				testName := strings.TrimPrefix(strings.TrimSpace(parts[0]), "test ")
				results.FailedTests = append(results.FailedTests, testName)
			}
		}
	}

	return err == nil, results
}

// ProgressTracker tracks execution progress
type ProgressTracker struct {
	mu        sync.RWMutex
	callbacks map[string][]ProgressCallback
}

// ProgressCallback is called when progress updates
type ProgressCallback func(*ExecutionProgress)

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		callbacks: make(map[string][]ProgressCallback),
	}
}

// RegisterCallback registers a progress callback
func (pt *ProgressTracker) RegisterCallback(executionID string, callback ProgressCallback) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, exists := pt.callbacks[executionID]; !exists {
		pt.callbacks[executionID] = make([]ProgressCallback, 0)
	}

	pt.callbacks[executionID] = append(pt.callbacks[executionID], callback)
}

// NotifyProgress notifies all callbacks for an execution
func (pt *ProgressTracker) NotifyProgress(progress *ExecutionProgress) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if callbacks, exists := pt.callbacks[progress.ExecutionID]; exists {
		for _, callback := range callbacks {
			callback(progress)
		}
	}
}

// ClearCallbacks clears callbacks for an execution
func (pt *ProgressTracker) ClearCallbacks(executionID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	delete(pt.callbacks, executionID)
}
