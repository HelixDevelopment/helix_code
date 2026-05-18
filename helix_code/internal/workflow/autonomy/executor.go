package autonomy

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ActionHandler runs the real side-effect for an Action. Handlers MUST be
// registered via ActionExecutor.RegisterHandler before executeAction will
// dispatch the corresponding ActionType. The handler returns the Output that
// will be surfaced on the ActionResult and any error encountered; the
// executor wraps the return into an ActionResult and records metrics.
//
// Round-31 §11.4 anti-bluff sweep (2026-05-18): the previous executeAction
// fabricated Success=true with a hardcoded placeholder string regardless of
// action type — a CRITICAL anti-bluff defect that certified every action as
// PASS. ActionHandler is the injection point that lets callers supply the
// real side-effect (file edit, command exec, network call, …) rather than
// a fixed-PASS lie.
type ActionHandler func(ctx context.Context, action *Action) (output string, err error)

// ActionExecutor executes actions with proper permission checks
type ActionExecutor struct {
	mu          sync.RWMutex
	permManager *PermissionManager
	retryEngine *RetryEngine
	metrics     *Metrics
	handlers    map[ActionType]ActionHandler
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
		metrics:  NewMetrics(),
		handlers: make(map[ActionType]ActionHandler),
	}
}

// RegisterHandler registers an ActionHandler for a specific ActionType. The
// handler will be invoked by executeAction whenever an Action of that type
// is executed. Registering a handler for an already-registered type
// silently replaces the previous handler (last-write-wins) so callers can
// swap implementations at runtime; this is intentional and matches the
// existing Metrics.RecordExecution pattern. handler MUST NOT be nil — a
// nil handler is rejected with ErrActionHandlerNotRegistered so the caller
// is never silently left with a placeholder dispatch.
func (a *ActionExecutor) RegisterHandler(actionType ActionType, handler ActionHandler) error {
	if handler == nil {
		return fmt.Errorf("%w: cannot register nil handler for action type %q",
			ErrActionHandlerNotRegistered, actionType)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handlers[actionType] = handler
	return nil
}

// UnregisterHandler removes the handler for the given ActionType. After
// removal, executeAction for that ActionType will return
// ErrActionHandlerNotRegistered until a fresh handler is registered.
func (a *ActionExecutor) UnregisterHandler(actionType ActionType) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.handlers, actionType)
}

// HasHandler reports whether a handler is currently registered for the
// given ActionType.
func (a *ActionExecutor) HasHandler(actionType ActionType) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.handlers[actionType]
	return ok
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

// executeAction performs the actual action execution by dispatching to the
// ActionHandler registered for the action's Type via RegisterHandler.
//
// Round-31 §11.4 anti-bluff sweep (2026-05-18): the previous implementation
// returned Success=true with a hardcoded placeholder Output ("Executed:
// <description>", "Context loaded successfully", "Debug retry executed",
// etc.) regardless of action type — a CRITICAL bluff (Article XI §11.9 /
// CONST-035 / CONST-050(A)) that certified every action as PASS independent
// of whether any real side-effect occurred. The new dispatch returns
// ErrActionHandlerNotRegistered (wrapped with the action type) when no
// handler is registered, so the caller can never confuse a missing
// implementation with a successful run.
func (a *ActionExecutor) executeAction(ctx context.Context, action *Action) *ActionResult {
	if action == nil {
		return &ActionResult{
			Success: false,
			Error:   fmt.Errorf("%w: cannot execute nil action", ErrActionFailed),
		}
	}

	a.mu.RLock()
	handler, ok := a.handlers[action.Type]
	a.mu.RUnlock()

	if !ok || handler == nil {
		err := fmt.Errorf("%w: action type %q", ErrActionHandlerNotRegistered, action.Type)
		return &ActionResult{
			Action:  action,
			Success: false,
			Error:   err,
			Output:  "",
		}
	}

	output, err := handler(ctx, action)
	return &ActionResult{
		Action:  action,
		Success: err == nil,
		Output:  output,
		Error:   err,
	}
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
