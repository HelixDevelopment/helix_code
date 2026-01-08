package autonomy

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ActionExecutor executes actions with proper permission checks
type ActionExecutor struct {
	mu          sync.RWMutex
	permManager *PermissionManager
	retryEngine *RetryEngine
	metrics     *Metrics
}

// RetryEngine handles automatic retries
type RetryEngine struct {
	maxRetries int
	delay      time.Duration
}

// NewActionExecutor creates a new action executor
func NewActionExecutor(permManager *PermissionManager) *ActionExecutor {
	return &ActionExecutor{
		permManager: permManager,
		retryEngine: &RetryEngine{
			maxRetries: 3,
			delay:      2 * time.Second,
		},
		metrics: NewMetrics(),
	}
}

// Execute runs an action with permission checking
func (a *ActionExecutor) Execute(ctx context.Context, action *Action) (*ActionResult, error) {
	start := time.Now()

	// Check permission
	perm, err := a.permManager.Check(ctx, action)
	if err != nil {
		return &ActionResult{
			Success:  false,
			Action:   action,
			Error:    err,
			Duration: time.Since(start),
		}, err
	}

	if !perm.Granted {
		return &ActionResult{
			Success:  false,
			Action:   action,
			Error:    ErrPermissionDenied,
			Duration: time.Since(start),
		}, ErrPermissionDenied
	}

	// Request confirmation if required
	if perm.RequiresConfirm {
		confirmed, err := a.permManager.RequestConfirmation(ctx, action)
		if err != nil || !confirmed {
			return &ActionResult{
				Success:   false,
				Action:    action,
				Error:     ErrConfirmationFailed,
				Confirmed: confirmed,
				Duration:  time.Since(start),
			}, ErrConfirmationFailed
		}
	}

	// Execute the action
	result := a.executeAction(ctx, action)
	result.Duration = time.Since(start)

	// Record metrics
	a.metrics.RecordExecution(result)

	return result, result.Error
}

// ExecuteWithRetry executes with automatic retry on failure
func (a *ActionExecutor) ExecuteWithRetry(ctx context.Context, action *Action, maxRetries int) (*ActionResult, error) {
	var lastResult *ActionResult
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := a.Execute(ctx, action)
		if err == nil && result.Success {
			result.Retries = attempt
			return result, nil
		}

		lastResult = result
		lastErr = err

		// Don't retry on permission or confirmation errors
		if err == ErrPermissionDenied || err == ErrConfirmationFailed {
			break
		}

		// Wait before retry
		if attempt < maxRetries {
			select {
			case <-time.After(a.retryEngine.delay):
			case <-ctx.Done():
				return lastResult, ctx.Err()
			}
		}
	}

	if lastResult != nil {
		lastResult.Retries = maxRetries
	}

	return lastResult, fmt.Errorf("%w: %v", ErrRetryExhausted, lastErr)
}

// CanExecuteAutomatically checks if action can run without confirmation
func (a *ActionExecutor) CanExecuteAutomatically(action *Action) bool {
	perm, err := a.permManager.Check(context.Background(), action)
	if err != nil || !perm.Granted {
		return false
	}

	return !perm.RequiresConfirm
}

// executeAction performs the actual action execution
func (a *ActionExecutor) executeAction(ctx context.Context, action *Action) *ActionResult {
	// This is a simplified implementation
	// In production, this would dispatch to actual action handlers

	result := &ActionResult{
		Action:  action,
		Success: true,
		Output:  fmt.Sprintf("Executed: %s", action.Description),
	}

	// Simulate different action types
	switch action.Type {
	case ActionLoadContext:
		result.Output = "Context loaded successfully"

	case ActionApplyChange:
		if action.Context != nil && len(action.Context.FilesAffected) > 0 {
			result.Output = fmt.Sprintf("Applied changes to %d files", len(action.Context.FilesAffected))
		}

	case ActionExecuteCmd:
		if action.Context != nil && action.Context.CommandToRun != "" {
			result.Output = fmt.Sprintf("Executed: %s", action.Context.CommandToRun)
		}

	case ActionDebugRetry:
		result.Output = "Debug retry executed"

	default:
		result.Output = fmt.Sprintf("Action %s completed", action.Type)
	}

	return result
}

// LoadContext automatically loads relevant context
func (a *ActionExecutor) LoadContext(ctx context.Context, task string) error {
	action := NewAction(ActionLoadContext, fmt.Sprintf("Load context for: %s", task), RiskNone)

	result, err := a.Execute(ctx, action)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("failed to load context: %v", result.Error)
	}

	return nil
}

// ApplyChange automatically applies code changes
func (a *ActionExecutor) ApplyChange(ctx context.Context, change *CodeChange) error {
	action := NewAction(ActionApplyChange, change.Description, RiskLow)
	action.Context = &ActionContext{
		FilesAffected: []string{change.FilePath},
		Reversible:    change.Reversible,
	}

	result, err := a.Execute(ctx, action)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("failed to apply change: %v", result.Error)
	}

	return nil
}

// ExecuteCommand runs a command with safety checks
func (a *ActionExecutor) ExecuteCommand(ctx context.Context, cmd string) (*ActionResult, error) {
	// Determine risk level based on command
	risk := RiskLow
	if containsDangerous(cmd) {
		risk = RiskHigh
	}

	action := NewAction(ActionExecuteCmd, fmt.Sprintf("Execute: %s", cmd), risk)
	action.Context = &ActionContext{
		CommandToRun: cmd,
	}

	return a.Execute(ctx, action)
}

// GetMetrics returns execution metrics
func (a *ActionExecutor) GetMetrics() *Metrics {
	return a.metrics
}

// containsDangerous checks if command contains dangerous patterns
func containsDangerous(cmd string) bool {
	// Normalize: trim whitespace and convert to lowercase for comparison
	normalizedCmd := strings.TrimSpace(strings.ToLower(cmd))

	// Dangerous command prefixes (destructive file/system operations)
	dangerousPrefixes := []string{
		"rm ", "rm\t", "rm\n", // Remove files
		"dd ",           // Low-level disk operations
		"mkfs", "mkfs.", // Format filesystems
		"fdisk",         // Partition editing
		"parted",        // Partition manipulation
		"wipefs",        // Wipe filesystem signatures
		"shred",         // Secure deletion
		"mv /", "mv ~/", // Moving from root/home
		"chmod -r 777", "chmod 777 /", // Dangerous permissions
		"chown -r",           // Recursive ownership change
		"kill -9", "killall", // Process termination
		"pkill",                                  // Process killing
		"shutdown", "reboot", "poweroff", "halt", // System control
		"systemctl stop", "systemctl disable", // Service control
		"init 0", "init 6", // Runlevel changes
		"> /dev/", ">/dev/", // Direct device writes
		"wget -o-|", "curl|", // Piped downloads (potential malware)
		":(){ :|:& };:", // Fork bomb
	}

	// Dangerous patterns anywhere in command
	dangerousPatterns := []string{
		"rm -rf /", "rm -fr /", "rm -r /", "rm -f /", // Root deletion
		"rm -rf ~", "rm -fr ~", "rm -r ~", // Home deletion
		"rm -rf /*", "rm -rf ~/*", // Wildcard deletion
		"rm -rf .", "rm -rf ..", // Current/parent dir deletion
		"--no-preserve-root",               // Bypass rm safety
		"| sh", "| bash", "| zsh", "| ksh", // Piped shell execution
		"|sh", "|bash", "|zsh", "|ksh", // No space variant
		"`rm", "$(rm", // Command substitution with rm
		"; rm", "&& rm", "|| rm", // Chained rm commands
		"eval ", "exec ", // Dynamic execution
		"/dev/sda", "/dev/nvme", "/dev/hd", // Raw disk access
		"format c:", "del /f /s /q", // Windows variants
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

	// Check for shell metacharacter exploitation
	if containsShellExploit(normalizedCmd) {
		return true
	}

	return false
}

// containsShellExploit checks for common shell exploitation patterns
func containsShellExploit(cmd string) bool {
	// Check for command injection via backticks or $()
	backtickCount := strings.Count(cmd, "`")
	if backtickCount > 0 && backtickCount%2 == 0 {
		// Has matched backticks - potential command substitution
		inner := extractBetween(cmd, "`", "`")
		if containsDangerous(inner) {
			return true
		}
	}

	// Check for $() command substitution
	if strings.Contains(cmd, "$(") && strings.Contains(cmd, ")") {
		// Extract content between $( and )
		start := strings.Index(cmd, "$(")
		if start >= 0 {
			depth := 1
			for i := start + 2; i < len(cmd); i++ {
				if cmd[i] == '(' {
					depth++
				} else if cmd[i] == ')' {
					depth--
					if depth == 0 {
						inner := cmd[start+2 : i]
						if containsDangerous(inner) {
							return true
						}
						break
					}
				}
			}
		}
	}

	return false
}

// extractBetween extracts content between first occurrence of start and end delimiters
func extractBetween(s, start, end string) string {
	startIdx := strings.Index(s, start)
	if startIdx < 0 {
		return ""
	}
	s = s[startIdx+len(start):]
	endIdx := strings.Index(s, end)
	if endIdx < 0 {
		return ""
	}
	return s[:endIdx]
}

// SetRetryConfig configures retry behavior
func (a *ActionExecutor) SetRetryConfig(maxRetries int, delay time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.retryEngine.maxRetries = maxRetries
	a.retryEngine.delay = delay
}
