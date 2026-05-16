# Confirmation Package

The `confirmation` package provides a security confirmation system for HelixCode that protects against dangerous or destructive operations. It implements a flexible policy-based approach to require user confirmation before executing potentially harmful actions.

## Overview

This package enables:
- Detection and classification of dangerous operations
- Policy-based confirmation requirements with configurable risk levels
- Multiple confirmation modes (always, interactive, auto-approve)
- Comprehensive audit logging of all confirmation decisions
- Integration with HelixCode's tool execution pipeline

## Key Types

### Confirmer

The main interface for requesting confirmations.

```go
type Confirmer interface {
    // Confirm requests confirmation for an operation
    Confirm(ctx context.Context, request *ConfirmationRequest) (*ConfirmationResult, error)

    // SetPolicy updates the confirmation policy
    SetPolicy(policy *ConfirmationPolicy)

    // GetPolicy returns the current policy
    GetPolicy() *ConfirmationPolicy
}
```

### ConfirmationRequest

Describes an operation requiring confirmation.

```go
type ConfirmationRequest struct {
    Operation    string            // Operation type (e.g., "file_delete", "shell_exec")
    Resource     string            // Target resource (file path, URL, etc.)
    Description  string            // Human-readable description
    RiskLevel    RiskLevel         // Assessed risk level
    Reversible   bool              // Whether operation can be undone
    Details      map[string]string // Additional context
    Timeout      time.Duration     // Confirmation timeout
    RequestID    string            // Unique request identifier
    Timestamp    time.Time         // Request timestamp
}
```

### ConfirmationResult

The outcome of a confirmation request.

```go
type ConfirmationResult struct {
    Approved    bool              // Whether operation was approved
    Reason      string            // Reason for decision
    ApprovedBy  string            // Who approved (user, policy, auto)
    Timestamp   time.Time         // Decision timestamp
    Conditions  []string          // Any conditions attached to approval
    RequestID   string            // Original request ID
}
```

### ConfirmationPolicy

Defines when confirmation is required.

```go
type ConfirmationPolicy struct {
    Mode             ConfirmationMode    // always, interactive, auto
    RiskThreshold    RiskLevel           // Minimum risk requiring confirmation
    AllowedPaths     []string            // Paths that skip confirmation
    DeniedPaths      []string            // Paths that always require confirmation
    AllowedCommands  []string            // Commands that skip confirmation
    DeniedCommands   []string            // Commands always requiring confirmation
    AutoApproveAfter time.Duration       // Auto-approve timeout (interactive mode)
    MaxRiskAutoApprove RiskLevel         // Max risk level for auto-approval
    RequireReason    bool                // Require reason for approval
}
```

### RiskLevel

Categorizes operation risk.

```go
type RiskLevel int

const (
    RiskNone     RiskLevel = iota // No risk - informational operations
    RiskLow                       // Low risk - easily reversible
    RiskMedium                    // Medium risk - may require manual recovery
    RiskHigh                      // High risk - significant impact
    RiskCritical                  // Critical - potentially irreversible damage
)
```

### DangerDetector

Analyzes operations to determine their risk level.

```go
type DangerDetector struct {
    rules       []DangerRule
    customRules []DangerRule
}

type DangerRule struct {
    Name        string
    Pattern     *regexp.Regexp
    RiskLevel   RiskLevel
    Description string
    Category    string
}
```

### AuditLogger

Records all confirmation events for compliance and debugging.

```go
type AuditLogger struct {
    entries   []AuditEntry
    maxSize   int
    outputDir string
    mu        sync.RWMutex
}

type AuditEntry struct {
    Timestamp   time.Time
    RequestID   string
    Operation   string
    Resource    string
    RiskLevel   RiskLevel
    Decision    string // "approved", "denied", "timeout"
    ApprovedBy  string
    Reason      string
    Duration    time.Duration
}
```

## Usage Examples

### Basic Confirmation

```go
package main

import (
    "context"
    "fmt"
    "time"

    "dev.helix.code/internal/tools/confirmation"
)

func main() {
    // Create confirmer with default policy
    policy := &confirmation.ConfirmationPolicy{
        Mode:          confirmation.ModeInteractive,
        RiskThreshold: confirmation.RiskMedium,
    }

    confirmer := confirmation.NewConfirmer(policy)
    ctx := context.Background()

    // Request confirmation for file deletion
    request := &confirmation.ConfirmationRequest{
        Operation:   "file_delete",
        Resource:    "/home/user/important-file.txt",
        Description: "Delete important configuration file",
        RiskLevel:   confirmation.RiskHigh,
        Reversible:  false,
        Timeout:     30 * time.Second,
    }

    result, err := confirmer.Confirm(ctx, request)
    if err != nil {
        panic(err)
    }

    if result.Approved {
        fmt.Println("Operation approved, proceeding...")
    } else {
        fmt.Printf("Operation denied: %s\n", result.Reason)
    }
}
```

### Danger Detection

```go
// Create danger detector
detector := confirmation.NewDangerDetector()

// Analyze a shell command
analysis := detector.Analyze(&confirmation.OperationContext{
    Type:     "shell_command",
    Command:  "rm -rf /var/log/*",
    WorkDir:  "/home/user",
})

fmt.Printf("Risk Level: %s\n", analysis.RiskLevel)
fmt.Printf("Reasons: %v\n", analysis.Reasons)
// Output:
// Risk Level: critical
// Reasons: [recursive deletion, wildcard pattern, system directory]

// Analyze file operation
analysis = detector.Analyze(&confirmation.OperationContext{
    Type:     "file_write",
    FilePath: "/etc/passwd",
})

fmt.Printf("Risk Level: %s\n", analysis.RiskLevel)
// Output: Risk Level: critical
```

### Policy Configuration

```go
// Strict policy - always require confirmation
strictPolicy := &confirmation.ConfirmationPolicy{
    Mode:          confirmation.ModeAlways,
    RiskThreshold: confirmation.RiskNone,
    RequireReason: true,
}

// Permissive policy with path-based exceptions
permissivePolicy := &confirmation.ConfirmationPolicy{
    Mode:          confirmation.ModeInteractive,
    RiskThreshold: confirmation.RiskHigh,
    AllowedPaths: []string{
        "/tmp/*",
        "/home/user/scratch/*",
    },
    DeniedPaths: []string{
        "/etc/*",
        "/usr/*",
        "~/.ssh/*",
    },
    AutoApproveAfter:   10 * time.Second,
    MaxRiskAutoApprove: confirmation.RiskLow,
}

// Auto-approve policy for CI/CD environments
autoPolicy := &confirmation.ConfirmationPolicy{
    Mode:             confirmation.ModeAuto,
    MaxRiskAutoApprove: confirmation.RiskMedium,
    DeniedCommands: []string{
        "rm -rf /",
        "dd if=/dev/zero",
        ":(){:|:&};:",
    },
}
```

### Audit Logging

```go
// Create audit logger
auditLogger := confirmation.NewAuditLogger(&confirmation.AuditConfig{
    OutputDir: "/var/log/helix/confirmations",
    MaxSize:   10000,
    Retention: 30 * 24 * time.Hour,
})

// Attach to confirmer
confirmer.SetAuditLogger(auditLogger)

// All confirmations are now logged...

// Query audit log
entries := auditLogger.Query(&confirmation.AuditQuery{
    StartTime: time.Now().Add(-24 * time.Hour),
    EndTime:   time.Now(),
    RiskLevel: confirmation.RiskHigh,
    Decision:  "denied",
})

for _, entry := range entries {
    fmt.Printf("[%s] %s on %s: %s (%s)\n",
        entry.Timestamp.Format(time.RFC3339),
        entry.Operation,
        entry.Resource,
        entry.Decision,
        entry.Reason)
}

// Export audit log
err := auditLogger.Export("/tmp/audit-report.json")
```

### Custom Danger Rules

```go
detector := confirmation.NewDangerDetector()

// Add custom danger rule
detector.AddRule(&confirmation.DangerRule{
    Name:        "production_database",
    Pattern:     regexp.MustCompile(`(?i)(drop|delete|truncate).*production`),
    RiskLevel:   confirmation.RiskCritical,
    Description: "Destructive operation on production database",
    Category:    "database",
})

// Add path-based rule
detector.AddRule(&confirmation.DangerRule{
    Name:        "kubernetes_config",
    Pattern:     regexp.MustCompile(`.*\.kube/config.*`),
    RiskLevel:   confirmation.RiskHigh,
    Description: "Modification of Kubernetes configuration",
    Category:    "infrastructure",
})
```

### Integration with Tool Execution

```go
type SecureToolExecutor struct {
    confirmer *confirmation.Confirmer
    detector  *confirmation.DangerDetector
    executor  ToolExecutor
}

func (s *SecureToolExecutor) Execute(ctx context.Context, tool Tool, args map[string]interface{}) (interface{}, error) {
    // Analyze operation risk
    analysis := s.detector.Analyze(&confirmation.OperationContext{
        Type:    tool.Name(),
        Args:    args,
        WorkDir: args["workdir"].(string),
    })

    // Request confirmation if needed
    if analysis.RequiresConfirmation {
        request := &confirmation.ConfirmationRequest{
            Operation:   tool.Name(),
            Resource:    analysis.Resource,
            Description: analysis.Description,
            RiskLevel:   analysis.RiskLevel,
            Reversible:  analysis.Reversible,
            Details:     analysis.Details,
        }

        result, err := s.confirmer.Confirm(ctx, request)
        if err != nil {
            return nil, err
        }

        if !result.Approved {
            return nil, fmt.Errorf("operation denied: %s", result.Reason)
        }
    }

    return s.executor.Execute(ctx, tool, args)
}
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Mode` | ConfirmationMode | Interactive | Confirmation behavior mode |
| `RiskThreshold` | RiskLevel | Medium | Minimum risk requiring confirmation |
| `AllowedPaths` | []string | [] | Paths that skip confirmation |
| `DeniedPaths` | []string | [] | Paths that always require confirmation |
| `AllowedCommands` | []string | [] | Commands that skip confirmation |
| `DeniedCommands` | []string | [] | Commands always requiring confirmation |
| `AutoApproveAfter` | time.Duration | 0 | Auto-approve timeout |
| `MaxRiskAutoApprove` | RiskLevel | Low | Max risk for auto-approval |
| `RequireReason` | bool | false | Require reason for approval |

## Confirmation Modes

| Mode | Description |
|------|-------------|
| `ModeAlways` | Always require explicit confirmation |
| `ModeInteractive` | Require confirmation based on risk threshold |
| `ModeAuto` | Auto-approve up to MaxRiskAutoApprove level |
| `ModeDisabled` | Disable confirmation (not recommended) |

## Built-in Danger Rules

The package includes detection rules for common dangerous patterns:

### Shell Commands
- Recursive deletion (`rm -rf`)
- Force flags (`-f`, `--force`)
- System directories (`/etc`, `/usr`, `/var`)
- Disk operations (`dd`, `mkfs`, `fdisk`)
- Network tools (`nc`, `netcat` with listeners)

### File Operations
- Sensitive files (`.ssh/`, `.gnupg/`, credentials)
- System configuration (`/etc/passwd`, `/etc/shadow`)
- Application configs (`.env`, `config.yaml`)

### Git Operations
- Force push (`--force`, `-f`)
- Branch deletion
- History rewriting (`rebase`, `reset --hard`)

## Security Considerations

1. **Default Deny**: Configure denied paths/commands explicitly for sensitive resources.

2. **Audit Retention**: Maintain audit logs for compliance and incident investigation.

3. **Timeout Handling**: Set appropriate timeouts to prevent indefinite blocking.

4. **Auto-Approve Limits**: Never auto-approve critical risk operations.

5. **Path Validation**: The package validates paths to prevent directory traversal attacks.

6. **Logging Sensitivity**: Audit logs may contain sensitive paths - secure log storage appropriately.

## Error Types

```go
var (
    ErrConfirmationDenied  = errors.New("confirmation denied by user")
    ErrConfirmationTimeout = errors.New("confirmation request timed out")
    ErrPolicyViolation     = errors.New("operation violates confirmation policy")
    ErrInvalidRequest      = errors.New("invalid confirmation request")
    ErrAuditFailed         = errors.New("failed to write audit log")
)
```
