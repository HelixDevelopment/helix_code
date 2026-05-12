package workflow

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/project"
)

// ProjectManager interface to support both Manager and DatabaseManager
type ProjectManager interface {
	GetProject(ctx context.Context, id string) (*project.Project, error)
	ListProjects(ctx context.Context, ownerID string) ([]*project.Project, error)
	CreateProject(ctx context.Context, name, description, path, projectType string) (*project.Project, error)
}

// LLMProvider interface for LLM operations
type LLMProvider interface {
	Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error)
	IsAvailable(ctx context.Context) bool
}

// ExecutorConfig holds executor configuration
type ExecutorConfig struct {
	MaxConcurrentSteps int
	StepTimeout        time.Duration
	EnableLLM          bool
	EnableMetrics      bool
}

// DefaultExecutorConfig returns default configuration
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxConcurrentSteps: 4,
		StepTimeout:        10 * time.Minute,
		EnableLLM:          true,
		EnableMetrics:      true,
	}
}

// ExecutionMetrics tracks workflow execution metrics
type ExecutionMetrics struct {
	mu               sync.RWMutex
	WorkflowsStarted int64
	WorkflowsSuccess int64
	WorkflowsFailed  int64
	StepsExecuted    int64
	StepsFailed      int64
	TotalDuration    time.Duration
	LLMCalls         int64
	LLMTokensUsed    int64
}

// Executor handles workflow execution
type Executor struct {
	projectManager ProjectManager
	llmProvider    LLMProvider
	config         *ExecutorConfig
	metrics        *ExecutionMetrics
	mu             sync.RWMutex
	activeFlows    map[string]*Workflow
}

// NewExecutor creates a new workflow executor
func NewExecutor(projectManager ProjectManager) *Executor {
	return &Executor{
		projectManager: projectManager,
		config:         DefaultExecutorConfig(),
		metrics:        &ExecutionMetrics{},
		activeFlows:    make(map[string]*Workflow),
	}
}

// NewExecutorWithLLM creates an executor with LLM support
func NewExecutorWithLLM(projectManager ProjectManager, llmProvider LLMProvider, config *ExecutorConfig) *Executor {
	if config == nil {
		config = DefaultExecutorConfig()
	}
	return &Executor{
		projectManager: projectManager,
		llmProvider:    llmProvider,
		config:         config,
		metrics:        &ExecutionMetrics{},
		activeFlows:    make(map[string]*Workflow),
	}
}

// SetLLMProvider sets the LLM provider for the executor
func (e *Executor) SetLLMProvider(provider LLMProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.llmProvider = provider
}

// GetMetrics returns execution metrics
func (e *Executor) GetMetrics() *ExecutionMetrics {
	return e.metrics
}

// GetActiveWorkflows returns currently running workflows
func (e *Executor) GetActiveWorkflows() []*Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(e.activeFlows))
	for _, w := range e.activeFlows {
		workflows = append(workflows, w)
	}
	return workflows
}

// GetWorkflow returns a workflow by ID
func (e *Executor) GetWorkflow(id string) (*Workflow, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	w, ok := e.activeFlows[id]
	return w, ok
}

// CancelWorkflow cancels a running workflow
func (e *Executor) CancelWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	w, ok := e.activeFlows[id]
	if !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}

	w.SetStatus(WorkflowStatusFailed)
	return nil
}

// ExecutePlanningWorkflow executes a planning workflow
func (e *Executor) ExecutePlanningWorkflow(ctx context.Context, projectID string) (*Workflow, error) {
	proj, err := e.projectManager.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	workflow := &Workflow{
		ID:          fmt.Sprintf("planning_%s_%d", projectID, time.Now().UnixNano()),
		Name:        "Project Architecture Planning",
		Description: "Generate system architecture and design for project",
		Mode:        "planning",
		Steps:       e.createPlanningSteps(proj),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      WorkflowStatusPending,
	}

	// Execute workflow
	go e.executeWorkflow(ctx, workflow, proj)

	return workflow, nil
}

// ExecuteBuildingWorkflow executes a building workflow
func (e *Executor) ExecuteBuildingWorkflow(ctx context.Context, projectID string) (*Workflow, error) {
	proj, err := e.projectManager.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	workflow := &Workflow{
		ID:          fmt.Sprintf("building_%s_%d", projectID, time.Now().UnixNano()),
		Name:        "Project Build",
		Description: "Build and compile project",
		Mode:        "building",
		Steps:       e.createBuildingSteps(proj),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      WorkflowStatusPending,
	}

	// Execute workflow
	go e.executeWorkflow(ctx, workflow, proj)

	return workflow, nil
}

// ExecuteTestingWorkflow executes a testing workflow
func (e *Executor) ExecuteTestingWorkflow(ctx context.Context, projectID string) (*Workflow, error) {
	proj, err := e.projectManager.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	workflow := &Workflow{
		ID:          fmt.Sprintf("testing_%s_%d", projectID, time.Now().UnixNano()),
		Name:        "Project Testing",
		Description: "Run comprehensive test suite",
		Mode:        "testing",
		Steps:       e.createTestingSteps(proj),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      WorkflowStatusPending,
	}

	// Execute workflow
	go e.executeWorkflow(ctx, workflow, proj)

	return workflow, nil
}

// ExecuteRefactoringWorkflow executes a refactoring workflow
func (e *Executor) ExecuteRefactoringWorkflow(ctx context.Context, projectID string) (*Workflow, error) {
	proj, err := e.projectManager.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	workflow := &Workflow{
		ID:          fmt.Sprintf("refactoring_%s_%d", projectID, time.Now().UnixNano()),
		Name:        "Code Refactoring",
		Description: "Refactor and improve code quality",
		Mode:        "refactoring",
		Steps:       e.createRefactoringSteps(proj),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      WorkflowStatusPending,
	}

	// Execute workflow
	go e.executeWorkflow(ctx, workflow, proj)

	return workflow, nil
}

// executeWorkflow executes a workflow.
//
// All writes to workflow.Status, workflow.UpdatedAt, and per-step
// Status/Error go through the workflow's mutex so concurrent readers
// (tests, status polling) do not race.
func (e *Executor) executeWorkflow(ctx context.Context, workflow *Workflow, proj *project.Project) {
	workflow.SetStatus(WorkflowStatusRunning)

	for i := range workflow.Steps {
		step := &workflow.Steps[i]

		// Check if all dependencies are completed
		if !e.areDependenciesCompleted(workflow, step) {
			workflow.setStepStatus(i, StepStatusSkipped, "")
			continue
		}

		workflow.setStepStatus(i, StepStatusRunning, "")

		// Execute step. executeStep reads/writes only fields of `step`
		// that the executor goroutine owns at this point (no concurrent
		// reader observes Step internals before completion), so passing
		// the bare pointer is safe.
		result, err := e.executeStep(ctx, step, proj)
		if err != nil {
			workflow.setStepStatus(i, StepStatusFailed, err.Error())
			workflow.SetStatus(WorkflowStatusFailed)
			return
		}

		// Step completed. The `result` check below preserves prior
		// behaviour (re-asserting completed status when non-empty).
		_ = result
		workflow.setStepStatus(i, StepStatusCompleted, "")
	}

	workflow.SetStatus(WorkflowStatusCompleted)
}

// executeStep executes a single workflow step
func (e *Executor) executeStep(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	switch step.Action {
	case StepActionAnalyzeCode:
		return e.executeAnalysisStep(ctx, step, proj)
	case StepActionGenerateCode:
		return e.executeGenerationStep(ctx, step, proj)
	case StepActionExecuteCommand:
		return e.executeCommandStep(ctx, step, proj)
	case StepActionRunTests:
		return e.executeTestStep(ctx, step, proj)
	case StepActionLintCode:
		return e.executeLintStep(ctx, step, proj)
	case StepActionBuildProject:
		return e.executeBuildStep(ctx, step, proj)
	default:
		return "", fmt.Errorf("unknown step action: %s", step.Action)
	}
}

// executeAnalysisStep executes an analysis step using LLM
func (e *Executor) executeAnalysisStep(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	// Gather project context
	projectContext, err := e.gatherProjectContext(proj)
	if err != nil {
		return "", fmt.Errorf("failed to gather project context: %w", err)
	}

	// If LLM is available, use it for analysis
	if e.llmProvider != nil && e.config.EnableLLM && e.llmProvider.IsAvailable(ctx) {
		return e.performLLMAnalysis(ctx, step, proj, projectContext)
	}

	// Fallback to static analysis
	return e.performStaticAnalysis(ctx, step, proj, projectContext)
}

// gatherProjectContext collects relevant context from the project
func (e *Executor) gatherProjectContext(proj *project.Project) (*ProjectContext, error) {
	ctx := &ProjectContext{
		ProjectPath:  proj.Path,
		ProjectType:  proj.Type,
		Files:        make([]FileInfo, 0),
		Dependencies: make([]string, 0),
	}

	// Walk project directory to collect file info
	err := filepath.WalkDir(proj.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip hidden directories and common non-code directories
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check for relevant source files
		ext := filepath.Ext(path)
		if isSourceFile(ext) {
			relPath, _ := filepath.Rel(proj.Path, path)
			info, _ := d.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			ctx.Files = append(ctx.Files, FileInfo{
				Path:    relPath,
				Size:    size,
				Type:    ext,
				IsEntry: isEntryPoint(relPath, proj.Type),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Detect dependencies
	ctx.Dependencies = e.detectDependencies(proj)

	return ctx, nil
}

// ProjectContext holds analysis context
type ProjectContext struct {
	ProjectPath  string
	ProjectType  string
	Files        []FileInfo
	Dependencies []string
	EntryPoints  []string
}

// FileInfo holds file metadata
type FileInfo struct {
	Path    string
	Size    int64
	Type    string
	IsEntry bool
}

func isSourceFile(ext string) bool {
	sourceExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".java": true, ".kt": true, ".rs": true, ".c": true, ".cpp": true, ".h": true,
		".rb": true, ".php": true, ".swift": true, ".scala": true, ".cs": true,
	}
	return sourceExts[ext]
}

func isEntryPoint(path, projectType string) bool {
	switch projectType {
	case "go":
		return strings.HasSuffix(path, "main.go") || strings.Contains(path, "cmd/")
	case "node":
		return path == "index.js" || path == "index.ts" || path == "app.js" || path == "server.js"
	case "python":
		return path == "main.py" || path == "__main__.py" || path == "app.py"
	case "rust":
		return strings.HasSuffix(path, "main.rs") || strings.HasSuffix(path, "lib.rs")
	default:
		return false
	}
}

func (e *Executor) detectDependencies(proj *project.Project) []string {
	deps := make([]string, 0)

	// Check for dependency files
	depFiles := map[string]string{
		"go.mod":           "go",
		"package.json":     "node",
		"requirements.txt": "python",
		"Cargo.toml":       "rust",
		"pom.xml":          "java",
	}

	for file, lang := range depFiles {
		depPath := filepath.Join(proj.Path, file)
		if _, err := os.Stat(depPath); err == nil {
			deps = append(deps, fmt.Sprintf("%s (%s)", file, lang))
		}
	}

	return deps
}

// performLLMAnalysis uses LLM for code analysis
func (e *Executor) performLLMAnalysis(ctx context.Context, step *Step, proj *project.Project, projectCtx *ProjectContext) (string, error) {
	// Build analysis prompt
	prompt := e.buildAnalysisPrompt(step, proj, projectCtx)

	systemPrompt := `You are an expert software architect and code analyst.
Analyze the provided codebase context and provide actionable insights.
Focus on architecture, patterns, potential issues, and improvement suggestions.
Be specific and reference actual files when possible.`

	// Create LLM request with Messages
	request := &llm.LLMRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.2, // Low temperature for analytical tasks
	}

	// Execute LLM call
	response, err := e.llmProvider.Generate(ctx, request)
	if err != nil {
		return "", fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Update metrics
	e.metrics.mu.Lock()
	e.metrics.LLMCalls++
	e.metrics.LLMTokensUsed += int64(response.Usage.TotalTokens)
	e.metrics.mu.Unlock()

	return response.Content, nil
}

func (e *Executor) buildAnalysisPrompt(step *Step, proj *project.Project, projectCtx *ProjectContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Code Analysis Request: %s\n\n", step.Name))
	sb.WriteString(fmt.Sprintf("## Project Information\n"))
	sb.WriteString(fmt.Sprintf("- Type: %s\n", proj.Type))
	sb.WriteString(fmt.Sprintf("- Path: %s\n", proj.Path))
	sb.WriteString(fmt.Sprintf("- Description: %s\n\n", proj.Description))

	sb.WriteString("## Files in Project\n")
	for _, f := range projectCtx.Files {
		entry := ""
		if f.IsEntry {
			entry = " (entry point)"
		}
		sb.WriteString(fmt.Sprintf("- %s (%d bytes)%s\n", f.Path, f.Size, entry))
	}

	sb.WriteString("\n## Dependencies\n")
	for _, d := range projectCtx.Dependencies {
		sb.WriteString(fmt.Sprintf("- %s\n", d))
	}

	sb.WriteString(fmt.Sprintf("\n## Analysis Task\n%s\n", step.Description))
	sb.WriteString("\nProvide a comprehensive analysis addressing the task above.")

	return sb.String()
}

// performStaticAnalysis performs analysis without LLM
func (e *Executor) performStaticAnalysis(ctx context.Context, step *Step, proj *project.Project, projectCtx *ProjectContext) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Static Analysis Report: %s\n\n", step.Name))
	sb.WriteString(fmt.Sprintf("Project: %s (%s)\n\n", proj.Name, proj.Type))

	sb.WriteString("## Project Structure\n")
	sb.WriteString(fmt.Sprintf("- Total source files: %d\n", len(projectCtx.Files)))

	// Count by type
	typeCount := make(map[string]int)
	for _, f := range projectCtx.Files {
		typeCount[f.Type]++
	}
	for ext, count := range typeCount {
		sb.WriteString(fmt.Sprintf("- %s files: %d\n", ext, count))
	}

	sb.WriteString("\n## Entry Points\n")
	for _, f := range projectCtx.Files {
		if f.IsEntry {
			sb.WriteString(fmt.Sprintf("- %s\n", f.Path))
		}
	}

	sb.WriteString("\n## Dependencies\n")
	for _, d := range projectCtx.Dependencies {
		sb.WriteString(fmt.Sprintf("- %s\n", d))
	}

	sb.WriteString("\n## Recommendations\n")
	sb.WriteString("- Enable LLM analysis for deeper insights\n")
	sb.WriteString("- Review entry points for optimization opportunities\n")

	return sb.String(), nil
}

// executeGenerationStep executes a code generation step using LLM
func (e *Executor) executeGenerationStep(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	// If LLM is available, use it for generation
	if e.llmProvider != nil && e.config.EnableLLM && e.llmProvider.IsAvailable(ctx) {
		return e.performLLMGeneration(ctx, step, proj)
	}

	// Fallback: Generate template-based code
	return e.generateTemplateCode(ctx, step, proj)
}

// performLLMGeneration uses LLM for code generation
func (e *Executor) performLLMGeneration(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	// Build generation prompt
	prompt := e.buildGenerationPrompt(step, proj)

	systemPrompt := `You are an expert software developer.
Generate clean, well-documented, and production-ready code.
Follow best practices for the project type and language.
Include comments explaining complex logic.
Provide complete, runnable code without placeholders.`

	// Create LLM request with Messages
	request := &llm.LLMRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   8192,
		Temperature: 0.3, // Slightly higher for generation creativity
	}

	// Execute LLM call
	response, err := e.llmProvider.Generate(ctx, request)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// Update metrics
	e.metrics.mu.Lock()
	e.metrics.LLMCalls++
	e.metrics.LLMTokensUsed += int64(response.Usage.TotalTokens)
	e.metrics.mu.Unlock()

	return response.Content, nil
}

func (e *Executor) buildGenerationPrompt(step *Step, proj *project.Project) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Code Generation Request: %s\n\n", step.Name))
	sb.WriteString(fmt.Sprintf("## Project Context\n"))
	sb.WriteString(fmt.Sprintf("- Language/Type: %s\n", proj.Type))
	sb.WriteString(fmt.Sprintf("- Project Path: %s\n", proj.Path))
	sb.WriteString(fmt.Sprintf("- Description: %s\n\n", proj.Description))

	sb.WriteString(fmt.Sprintf("## Generation Task\n%s\n\n", step.Description))
	sb.WriteString("Generate the requested code following these guidelines:\n")
	sb.WriteString("1. Use idiomatic patterns for the language\n")
	sb.WriteString("2. Include proper error handling\n")
	sb.WriteString("3. Add documentation comments\n")
	sb.WriteString("4. Follow project conventions\n")

	return sb.String()
}

// generateTemplateCode generates template-based code without LLM
func (e *Executor) generateTemplateCode(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("// Generated code for: %s\n", step.Name))
	sb.WriteString(fmt.Sprintf("// Project: %s\n", proj.Name))
	sb.WriteString(fmt.Sprintf("// Task: %s\n\n", step.Description))

	switch proj.Type {
	case "go":
		sb.WriteString(e.generateGoTemplate(step))
	case "node", "typescript":
		sb.WriteString(e.generateNodeTemplate(step))
	case "python":
		sb.WriteString(e.generatePythonTemplate(step))
	case "rust":
		sb.WriteString(e.generateRustTemplate(step))
	default:
		sb.WriteString("// Enable LLM for code generation in this language\n")
	}

	return sb.String(), nil
}

func (e *Executor) generateGoTemplate(step *Step) string {
	return `package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// Task: ` + step.Description + `
// Generated by HelixCode Workflow Engine
// Enable LLM provider for AI-powered implementation

func main() {
	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(ctx context.Context) error {
	// TODO: Implement the following task:
	// ` + step.Description + `

	fmt.Println("Task implementation pending")
	fmt.Println("Configure an LLM provider in HelixCode for AI-powered code generation")

	return nil
}
`
}

func (e *Executor) generateNodeTemplate(step *Step) string {
	return `/**
 * Task: ` + step.Description + `
 * Generated by HelixCode Workflow Engine
 * Enable LLM provider for AI-powered implementation
 */

const process = require('process');

// Graceful shutdown handler
let isShuttingDown = false;

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

function shutdown() {
    if (isShuttingDown) return;
    isShuttingDown = true;
    console.log('Shutting down...');
    process.exit(0);
}

async function main() {
    try {
        // TODO: Implement the following task:
        // ` + step.Description + `

        console.log('Task implementation pending');
        console.log('Configure an LLM provider in HelixCode for AI-powered code generation');

    } catch (error) {
        console.error('Error:', error.message);
        process.exit(1);
    }
}

main();
`
}

func (e *Executor) generatePythonTemplate(step *Step) string {
	return `#!/usr/bin/env python3
"""
Task: ` + step.Description + `
Generated by HelixCode Workflow Engine
Enable LLM provider for AI-powered implementation
"""

import signal
import sys
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Graceful shutdown handler
def signal_handler(signum, frame):
    logger.info("Shutting down...")
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)


def main():
    """
    TODO: Implement the following task:
    ` + step.Description + `
    """
    try:
        logger.info("Task implementation pending")
        logger.info("Configure an LLM provider in HelixCode for AI-powered code generation")

    except Exception as e:
        logger.error(f"Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
`
}

func (e *Executor) generateRustTemplate(step *Step) string {
	return `//! Task: ` + step.Description + `
//! Generated by HelixCode Workflow Engine
//! Enable LLM provider for AI-powered implementation

use std::process;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

fn main() {
    // Setup graceful shutdown
    let running = Arc::new(AtomicBool::new(true));
    let r = running.clone();

    ctrlc::set_handler(move || {
        println!("Shutting down...");
        r.store(false, Ordering::SeqCst);
    }).expect("Error setting Ctrl-C handler");

    if let Err(e) = run() {
        eprintln!("Error: {}", e);
        process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    // TODO: Implement the following task:
    // ` + step.Description + `

    println!("Task implementation pending");
    println!("Configure an LLM provider in HelixCode for AI-powered code generation");

    Ok(())
}
`
}

// executeCommandStep executes a command execution step with security validation
func (e *Executor) executeCommandStep(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	command := step.Description

	// Validate command is not empty
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// Security check: validate command against dangerous patterns
	if isDangerousCommand(command) {
		return "", fmt.Errorf("command blocked: potentially dangerous operation detected")
	}

	// Execute command in project directory
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = proj.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command execution failed: %v\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// isDangerousCommand checks if a command contains dangerous patterns
// This is a security measure to prevent command injection attacks
func isDangerousCommand(cmd string) bool {
	// Normalize: trim whitespace and convert to lowercase for comparison
	normalizedCmd := strings.TrimSpace(strings.ToLower(cmd))

	// Dangerous command prefixes (destructive file/system operations)
	dangerousPrefixes := []string{
		"rm ", "rm\t", // Remove files
		"dd ",           // Low-level disk operations
		"mkfs", "mkfs.", // Format filesystems
		"fdisk",         // Partition editing
		"parted",        // Partition manipulation
		"wipefs",        // Wipe filesystem signatures
		"shred",         // Secure deletion
		"mv /", "mv ~/", // Moving from root/home
		"chmod 777 /",        // Dangerous permissions on root
		"chown -r",           // Recursive ownership change
		"kill -9", "killall", // Process termination
		"pkill",                                  // Process killing
		"shutdown", "reboot", "poweroff", "halt", // System control
		"systemctl stop", "systemctl disable", // Service control
		"init 0", "init 6", // Runlevel changes
		"> /dev/", ">/dev/", // Direct device writes
	}

	// Dangerous patterns anywhere in command
	dangerousPatterns := []string{
		"rm -rf /", "rm -fr /", "rm -r /", "rm -f /", // Root deletion
		"rm -rf ~", "rm -fr ~", "rm -r ~", // Home deletion
		"rm -rf /*", "rm -rf ~/*", // Wildcard deletion
		"rm -rf .", "rm -rf ..", // Current/parent dir deletion
		"--no-preserve-root",      // Bypass rm safety
		"| sh", "| bash", "| zsh", // Piped shell execution
		"|sh", "|bash", "|zsh", // No space variant
		"`rm", "$(rm", // Command substitution with rm
		"; rm", "&& rm", "|| rm", // Chained rm commands
		"eval ", "exec ", // Dynamic execution
		"/dev/sda", "/dev/nvme", "/dev/hd", // Raw disk access
		":(){ :|:& };:", // Fork bomb
	}

	// Check prefixes (command starts with dangerous prefix)
	for _, prefix := range dangerousPrefixes {
		if strings.HasPrefix(normalizedCmd, prefix) {
			return true
		}
	}

	// Check patterns (dangerous pattern anywhere in command)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(normalizedCmd, pattern) {
			return true
		}
	}

	return false
}

// executeTestStep executes a test execution step
func (e *Executor) executeTestStep(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	// Execute test command based on project type
	var cmd *exec.Cmd

	switch proj.Type {
	case "go":
		cmd = exec.CommandContext(ctx, "go", "test", "./...")
	case "node":
		cmd = exec.CommandContext(ctx, "npm", "test")
	case "python":
		cmd = exec.CommandContext(ctx, "python", "-m", "pytest")
	case "rust":
		cmd = exec.CommandContext(ctx, "cargo", "test")
	default:
		return "", fmt.Errorf("unsupported project type for testing: %s", proj.Type)
	}

	cmd.Dir = proj.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("test execution failed: %v\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// executeLintStep executes a linting step
func (e *Executor) executeLintStep(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	// Execute lint command based on project type
	var cmd *exec.Cmd

	switch proj.Type {
	case "go":
		cmd = exec.CommandContext(ctx, "gofmt", "-l", ".")
	case "node":
		cmd = exec.CommandContext(ctx, "npm", "run", "lint")
	case "python":
		cmd = exec.CommandContext(ctx, "flake8", ".")
	case "rust":
		cmd = exec.CommandContext(ctx, "cargo", "clippy")
	default:
		return "", fmt.Errorf("unsupported project type for linting: %s", proj.Type)
	}

	cmd.Dir = proj.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lint execution failed: %v\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// executeBuildStep executes a build step
func (e *Executor) executeBuildStep(ctx context.Context, step *Step, proj *project.Project) (string, error) {
	// Execute build command based on project type
	var cmd *exec.Cmd

	switch proj.Type {
	case "go":
		cmd = exec.CommandContext(ctx, "go", "build")
	case "node":
		cmd = exec.CommandContext(ctx, "npm", "run", "build")
	case "python":
		cmd = exec.CommandContext(ctx, "python", "setup.py", "build")
	case "rust":
		cmd = exec.CommandContext(ctx, "cargo", "build")
	default:
		return "", fmt.Errorf("unsupported project type for building: %s", proj.Type)
	}

	cmd.Dir = proj.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build execution failed: %v\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// areDependenciesCompleted checks if all step dependencies are completed
func (e *Executor) areDependenciesCompleted(workflow *Workflow, step *Step) bool {
	for _, depID := range step.Dependencies {
		depCompleted := false
		for _, s := range workflow.Steps {
			if s.ID == depID && s.Status == StepStatusCompleted {
				depCompleted = true
				break
			}
		}
		if !depCompleted {
			return false
		}
	}
	return true
}

// createPlanningSteps creates steps for planning workflow
func (e *Executor) createPlanningSteps(proj *project.Project) []Step {
	return []Step{
		{
			ID:          "analyze_requirements",
			Name:        "Analyze Requirements",
			Description: "Analyze project requirements and constraints",
			Type:        StepTypeAnalysis,
			Action:      StepActionAnalyzeCode,
			Status:      StepStatusPending,
		},
		{
			ID:           "generate_architecture",
			Name:         "Generate Architecture",
			Description:  "Generate system architecture and design",
			Type:         StepTypeGeneration,
			Action:       StepActionGenerateCode,
			Dependencies: []string{"analyze_requirements"},
			Status:       StepStatusPending,
		},
	}
}

// createBuildingSteps creates steps for building workflow
func (e *Executor) createBuildingSteps(proj *project.Project) []Step {
	return []Step{
		{
			ID:          "setup_environment",
			Name:        "Setup Environment",
			Description: "Setup build environment and dependencies",
			Type:        StepTypeExecution,
			Action:      StepActionExecuteCommand,
			Status:      StepStatusPending,
		},
		{
			ID:           "compile_code",
			Name:         "Compile Code",
			Description:  proj.Metadata.BuildCommand,
			Type:         StepTypeExecution,
			Action:       StepActionBuildProject,
			Dependencies: []string{"setup_environment"},
			Status:       StepStatusPending,
		},
	}
}

// createTestingSteps creates steps for testing workflow
func (e *Executor) createTestingSteps(proj *project.Project) []Step {
	return []Step{
		{
			ID:          "unit_tests",
			Name:        "Unit Tests",
			Description: "Run unit tests",
			Type:        StepTypeExecution,
			Action:      StepActionRunTests,
			Status:      StepStatusPending,
		},
		{
			ID:           "integration_tests",
			Name:         "Integration Tests",
			Description:  "Run integration tests",
			Type:         StepTypeExecution,
			Action:       StepActionRunTests,
			Dependencies: []string{"unit_tests"},
			Status:       StepStatusPending,
		},
	}
}

// createRefactoringSteps creates steps for refactoring workflow
func (e *Executor) createRefactoringSteps(proj *project.Project) []Step {
	return []Step{
		{
			ID:          "analyze_codebase",
			Name:        "Analyze Codebase",
			Description: "Analyze codebase for refactoring opportunities",
			Type:        StepTypeAnalysis,
			Action:      StepActionAnalyzeCode,
			Status:      StepStatusPending,
		},
		{
			ID:           "refactor_code",
			Name:         "Refactor Code",
			Description:  "Perform code refactoring",
			Type:         StepTypeGeneration,
			Action:       StepActionGenerateCode,
			Dependencies: []string{"analyze_codebase"},
			Status:       StepStatusPending,
		},
	}
}
