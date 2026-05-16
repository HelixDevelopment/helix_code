# Autonomy Package

The `autonomy` package provides a comprehensive five-level spectrum of AI autonomy modes for HelixCode, enabling users to control how much the AI can do independently while balancing automation with user oversight.

## Overview

The autonomy system allows fine-grained control over AI operations through:
- Five distinct autonomy levels (None, Basic, Basic Plus, Semi Auto, Full Auto)
- Permission-based action execution with guardrails
- Temporary mode escalation with automatic reversion
- Comprehensive metrics tracking
- Thread-safe concurrent operations

## Key Types and Interfaces

### AutonomyMode

Defines the five levels of AI autonomy:

```go
const (
    ModeNone      AutonomyMode = "none"       // Level 1: Complete manual control
    ModeBasic     AutonomyMode = "basic"      // Level 2: Basic automation
    ModeBasicPlus AutonomyMode = "basic_plus" // Level 3: Smart semi-automation
    ModeSemiAuto  AutonomyMode = "semi_auto"  // Level 4: Automated with approval (DEFAULT)
    ModeFullAuto  AutonomyMode = "full_auto"  // Level 5: Fully autonomous
)
```

### AutonomyController

The main orchestrator for autonomy operations:

```go
type AutonomyController struct {
    modeManager *ModeManager
    permManager *PermissionManager
    executor    *ActionExecutor
    escalator   *EscalationEngine
    guardrails  *GuardrailsChecker
    config      *Config
    metrics     *Metrics
}
```

### Action and ActionType

Represents operations requiring permission:

```go
type Action struct {
    Type        ActionType
    Description string
    Risk        RiskLevel
    Context     *ActionContext
    Metadata    map[string]interface{}
}

const (
    ActionLoadContext  ActionType = "load_context"
    ActionApplyChange  ActionType = "apply_change"
    ActionExecuteCmd   ActionType = "execute_command"
    ActionDebugRetry   ActionType = "debug_retry"
    ActionFileDelete   ActionType = "file_delete"
    ActionBulkEdit     ActionType = "bulk_edit"
    ActionNetworkCall  ActionType = "network_call"
    ActionSystemChange ActionType = "system_change"
)
```

### RiskLevel

Categorizes the risk associated with actions:

```go
const (
    RiskNone     RiskLevel = "none"     // No risk
    RiskLow      RiskLevel = "low"      // Easily reversible
    RiskMedium   RiskLevel = "medium"   // May need effort to reverse
    RiskHigh     RiskLevel = "high"     // Difficult to reverse
    RiskCritical RiskLevel = "critical" // Potentially destructive
)
```

## Usage Examples

### Creating an Autonomy Controller

```go
// With default configuration
controller, err := autonomy.NewAutonomyController(nil)
if err != nil {
    log.Fatal(err)
}

// With custom configuration
config := autonomy.NewDefaultConfig()
config.DefaultMode = autonomy.ModeSemiAuto
config.EnableGuardrails = true
config.RiskThreshold = autonomy.RiskMedium

controller, err := autonomy.NewAutonomyController(config)
```

### Mode Management

```go
// Get current mode
mode := controller.GetCurrentMode()
fmt.Printf("Current mode: %s (Level %d)\n", mode, mode.Level())

// Change mode
err := controller.SetModeWithReason(ctx, autonomy.ModeFullAuto, "debugging critical issue")
if err != nil {
    log.Printf("Mode change denied: %v", err)
}

// Get capabilities for current mode
caps := controller.GetCapabilities()
fmt.Printf("Auto-apply: %v, Auto-execute: %v\n", caps.AutoApply, caps.AutoExecute)
```

### Executing Actions

```go
// Create an action
action := autonomy.NewAction(
    autonomy.ActionApplyChange,
    "Apply code changes to main.go",
    autonomy.RiskMedium,
)
action.Context = &autonomy.ActionContext{
    FilesAffected: []string{"main.go", "utils.go"},
    Reversible:    true,
}

// Execute with automatic permission checks
result, err := controller.ExecuteAction(ctx, action)
if err != nil {
    log.Printf("Action failed: %v", err)
} else if result.Success {
    fmt.Printf("Action completed: %s\n", result.Output)
}
```

### Permission Checking

```go
// Check permission before execution
perm, err := controller.RequestPermission(ctx, action)
if err != nil {
    log.Fatal(err)
}

if !perm.Granted {
    fmt.Printf("Permission denied: %s\n", perm.Reason)
    return
}

if perm.RequiresConfirm {
    // Request user confirmation
    fmt.Printf("Confirm: %s\n", perm.ConfirmPrompt)
}
```

### Mode Escalation

```go
// Request temporary escalation (e.g., for debugging)
err := controller.RequestEscalation(ctx, "debugging critical issue", 30*time.Minute)
if err != nil {
    log.Fatal(err)
}

// Mode automatically reverts after duration
// Or manually de-escalate
err = controller.DeEscalate(ctx)
```

### Custom Guardrails

```go
// Add a custom guardrail rule
controller.AddGuardrailRule(autonomy.GuardrailRule{
    Name:        "no_production_delete",
    Description: "Prevent deletions in production directories",
    Severity:    autonomy.RiskCritical,
    Enabled:     true,
    Check: func(ctx context.Context, action *autonomy.Action) (bool, string) {
        if action.Type == autonomy.ActionFileDelete {
            for _, file := range action.Context.FilesAffected {
                if strings.Contains(file, "/production/") {
                    return false, "cannot delete production files"
                }
            }
        }
        return true, ""
    },
})

// Disable/enable specific rules
controller.DisableGuardrailRule("no_system_file_delete")
controller.EnableGuardrailRule("no_system_file_delete")
```

### Metrics and Monitoring

```go
// Get performance metrics
metrics := controller.GetMetrics()
stats := metrics.GetStats()

fmt.Printf("Actions executed: %d\n", stats.ActionsExecuted)
fmt.Printf("Success rate: %.2f%%\n", stats.SuccessRate()*100)
fmt.Printf("Approval rate: %.2f%%\n", stats.ApprovalRate()*100)
fmt.Printf("Average execution time: %v\n", stats.AverageExecuteTime)

// Get mode history
history := controller.GetModeHistory()
for _, change := range history.Changes {
    fmt.Printf("%s: %s -> %s (%s)\n",
        change.Timestamp.Format(time.RFC3339),
        change.From, change.To, change.Reason)
}
```

## Configuration Options

### Config Structure

```go
type Config struct {
    // Mode settings
    DefaultMode     AutonomyMode  // Default: ModeSemiAuto
    AllowModeSwitch bool          // Allow mode changes
    PersistMode     bool          // Persist mode to disk
    SessionScoped   bool          // Mode only for current session

    // Escalation settings
    AllowEscalation   bool          // Allow temporary escalation
    AutoDeEscalate    bool          // Auto-revert after task
    EscalationTimeout time.Duration // Max escalation duration

    // Safety settings
    EnableGuardrails bool      // Enable safety guardrails
    RiskThreshold    RiskLevel // Max allowed risk level
    RequireReason    bool      // Require reason for risky ops

    // Confirmation settings
    ConfirmRisky  bool // Confirm risky operations
    ConfirmBulk   bool // Confirm bulk operations
    BulkThreshold int  // Files threshold for bulk

    // Auto-debug settings
    DebugEnabled    bool          // Enable auto-retry
    MaxRetries      int           // Max retry attempts
    RetryDelay      time.Duration // Delay between retries
    LearnFromErrors bool          // Learn from failures
}
```

## Integration Patterns

### With Workflow Engine

```go
// In workflow step execution
func executeStep(ctx context.Context, step *WorkflowStep, controller *autonomy.AutonomyController) error {
    action := autonomy.NewAction(
        autonomy.ActionApplyChange,
        step.Description,
        determineRisk(step),
    )

    result, err := controller.ExecuteAction(ctx, action)
    if err != nil {
        return fmt.Errorf("step %s failed: %w", step.ID, err)
    }

    return nil
}
```

### With Agent System

```go
// Agent respects autonomy settings
func (agent *CodingAgent) Execute(ctx context.Context, task *Task) error {
    caps := agent.autonomyController.GetCapabilities()

    if !caps.AutoApply {
        // Request confirmation before applying changes
        perm, _ := agent.autonomyController.RequestPermission(ctx, action)
        if perm.RequiresConfirm {
            // Wait for user confirmation
        }
    }

    return agent.autonomyController.ApplyChange(ctx, change)
}
```

## Default Guardrails

The package includes several built-in guardrail rules:

| Rule Name | Description | Severity |
|-----------|-------------|----------|
| `no_system_file_delete` | Prevents deletion of system files (/etc, /sys, etc.) | Critical |
| `no_bulk_unreviewed` | Prevents bulk changes affecting >10 files | High |
| `no_destructive_commands` | Blocks dangerous shell commands (rm -rf /, etc.) | Critical |
| `no_uncontrolled_network` | Requires explicit approval for network operations | Medium |
| `require_reversible_changes` | Ensures high-risk changes are reversible | Medium |
| `limit_iteration_depth` | Prevents infinite iteration loops (max 50) | Medium |
| `no_credential_exposure` | Prevents exposure of credentials | Critical |

## Thread Safety

All components in the autonomy package are thread-safe:

```go
// Multiple goroutines can safely use the controller
for i := 0; i < 10; i++ {
    go func(id int) {
        action := autonomy.NewAction(
            autonomy.ActionLoadContext,
            fmt.Sprintf("Task %d", id),
            autonomy.RiskNone,
        )
        result, _ := controller.ExecuteAction(ctx, action)
        fmt.Printf("Task %d: %v\n", id, result.Success)
    }(i)
}
```

## Graceful Shutdown

```go
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := controller.Shutdown(ctx); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}()
```

## Error Handling

The package provides structured errors with context:

```go
var (
    ErrInvalidMode        = errors.New("invalid autonomy mode")
    ErrModeSwitchDenied   = errors.New("mode switch not allowed")
    ErrPermissionDenied   = errors.New("permission denied")
    ErrConfirmationFailed = errors.New("user confirmation failed")
    ErrGuardrailViolation = errors.New("guardrail violation")
    ErrRetryExhausted     = errors.New("retry attempts exhausted")
    ErrEscalationDenied   = errors.New("escalation request denied")
)
```

## References

Based on patterns from:
- Plandex autonomy modes (plan_config.go)
- Agentic framework patterns (AutoGPT, BabyAGI)
- Human-in-the-loop ML design principles
- AI safety and alignment research
