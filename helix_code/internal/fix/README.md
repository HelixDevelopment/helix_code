# Fix Package

The `fix` package provides comprehensive automated security issue resolution and code fixing capabilities for the HelixCode platform.

## Overview

The fix package enables automated detection and remediation of security vulnerabilities in project code. It integrates tightly with the security package to perform comprehensive scans, apply fixes automatically where possible, validate that fixes were successful, and track issues that require manual intervention.

Key features include:
- Automated security vulnerability detection and fixing
- Configurable critical-only or all-issues processing modes
- Post-fix validation with security score tracking
- Detailed result reporting with fix statistics
- Concurrent fix operation support
- Go file discovery and traversal utilities

## Architecture

The fix package follows a scan-fix-validate workflow:

```
Project Path
     |
     v
[Security Scan] --> Issues Found
     |
     v
[Process Issues] --> Fixed / Failed / Manual / Skipped
     |
     v
[Validate Fixes] --> Post-fix Security Score
     |
     v
[FixResult]
```

The package integrates with:
- `internal/security`: For vulnerability scanning and validation
- `internal/logging`: For progress reporting and diagnostics

## Key Types

### FixResult

Represents the comprehensive result of a security fix operation:

```go
type FixResult struct {
    TotalIssues   int                  `json:"total_issues"`   // Total issues found
    FixedIssues   int                  `json:"fixed_issues"`   // Successfully fixed
    FailedFixes   int                  `json:"failed_fixes"`   // Failed to fix automatically
    ManualFixes   int                  `json:"manual_fixes"`   // Requires manual intervention
    SkippedIssues int                  `json:"skipped_issues"` // Skipped (non-critical in critical-only mode)
    StartTime     time.Time            `json:"start_time"`     // Operation start time
    EndTime       time.Time            `json:"end_time"`       // Operation end time
    Success       bool                 `json:"success"`        // Overall success status
    Validation    *FixValidationResult `json:"validation,omitempty"` // Post-fix validation
}
```

**Success Criteria**: A fix operation is considered successful when:
- At least one issue was fixed (`FixedIssues > 0`), AND
- No critical issues remain after validation (`Validation.RemainingCriticalIssues == 0`)

### FixValidationResult

Contains validation results after applying fixes:

```go
type FixValidationResult struct {
    ScanResult              *security.FeatureScanResult `json:"scan_result"`
    RemainingCriticalIssues int                         `json:"remaining_critical_issues"`
    RemainingHighIssues     int                         `json:"remaining_high_issues"`
    FixesValidated          bool                        `json:"fixes_validated"`
}
```

The `ScanResult` field contains detailed information from the post-fix security scan, including the updated security score.

## Core Functions

### FixAllCriticalSecurityIssues

The primary function for comprehensive security issue resolution:

```go
func FixAllCriticalSecurityIssues(projectPath string, criticalOnly bool) (*FixResult, error)
```

**Parameters**:
- `projectPath`: Absolute path to the project directory to scan and fix
- `criticalOnly`: When `true`, only critical vulnerabilities are fixed; non-critical issues are skipped

**Returns**:
- `*FixResult`: Comprehensive results including fix counts and validation
- `error`: Error if initialization or scanning fails

### processSecurityIssues

Internal function that processes individual security issues:

```go
func processSecurityIssues(projectPath string, issues []interface{}, criticalOnly bool) (int, int, int, int)
```

Returns counts for: fixed, failed, manual, and skipped issues.

### validateFixes

Internal function that validates fixes were successful:

```go
func validateFixes(projectPath string) (*FixValidationResult, error)
```

Performs a post-fix security scan to verify improvements.

### findGoFiles

Helper function to discover Go source files:

```go
func findGoFiles(projectPath string) ([]string, error)
```

Recursively walks the project directory to find all `.go` files.

## Usage Examples

### Basic Security Fix Operation

```go
import "dev.helix.code/internal/fix"

func main() {
    projectPath := "/path/to/project"

    // Fix only critical security issues
    result, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
    if err != nil {
        log.Fatalf("Fix operation failed: %v", err)
    }

    // Report results
    log.Printf("Total Issues: %d", result.TotalIssues)
    log.Printf("Fixed: %d", result.FixedIssues)
    log.Printf("Failed: %d", result.FailedFixes)
    log.Printf("Manual Required: %d", result.ManualFixes)
    log.Printf("Skipped: %d", result.SkippedIssues)
    log.Printf("Overall Success: %t", result.Success)
}
```

### Fix All Issues (Not Just Critical)

```go
// Fix all security issues regardless of severity
result, err := fix.FixAllCriticalSecurityIssues(projectPath, false)
if err != nil {
    log.Fatal(err)
}

if result.Success {
    log.Println("All issues resolved successfully")
} else {
    log.Printf("Some issues remain - manual fixes needed: %d", result.ManualFixes)
}
```

### Checking Post-Fix Security Score

```go
result, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
if err != nil {
    log.Fatal(err)
}

if result.Validation != nil {
    log.Printf("Post-fix Security Score: %d", result.Validation.ScanResult.SecurityScore)
    log.Printf("Remaining Critical Issues: %d", result.Validation.RemainingCriticalIssues)
    log.Printf("Remaining High Issues: %d", result.Validation.RemainingHighIssues)
    log.Printf("Fixes Validated: %t", result.Validation.FixesValidated)
}
```

### Calculating Fix Duration

```go
result, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
if err != nil {
    log.Fatal(err)
}

duration := result.EndTime.Sub(result.StartTime)
log.Printf("Fix operation completed in %s", duration)
```

### Concurrent Fix Operations

```go
// The package supports concurrent fix operations on different projects
projects := []string{
    "/path/to/project1",
    "/path/to/project2",
    "/path/to/project3",
}

var wg sync.WaitGroup
results := make([]*fix.FixResult, len(projects))

for i, project := range projects {
    wg.Add(1)
    go func(idx int, path string) {
        defer wg.Done()
        result, err := fix.FixAllCriticalSecurityIssues(path, true)
        if err != nil {
            log.Printf("Error fixing %s: %v", path, err)
            return
        }
        results[idx] = result
    }(i, project)
}

wg.Wait()

// Process results
for i, result := range results {
    if result != nil {
        log.Printf("Project %s: Fixed %d/%d issues",
            projects[i], result.FixedIssues, result.TotalIssues)
    }
}
```

## Configuration Options

Configure fix settings in `config/config.yaml`:

```yaml
fix:
  # Enable automatic fixing (when false, only reports issues)
  auto_fix: false

  # Minimum confidence level to apply a fix (0.0 - 1.0)
  min_confidence: 0.8

  # Create backups before applying fixes
  backup: true

  # Backup directory (relative to project or absolute)
  backup_dir: ".fix_backups"

  # Maximum number of fixes per run (0 = unlimited)
  max_fixes: 0

  # File patterns to exclude from fixing
  exclude_patterns:
    - "vendor/**"
    - "node_modules/**"
    - "**/*_test.go"
```

## Issue Categories

The package handles various types of security issues:

### Severity Levels

1. **Critical**: Must be fixed immediately
   - SQL injection vulnerabilities
   - Command injection
   - Remote code execution
   - Authentication bypasses

2. **High**: Should be fixed soon
   - Cross-site scripting (XSS)
   - Insecure deserialization
   - Path traversal
   - Sensitive data exposure

3. **Medium**: Fix when possible
   - Weak cryptography
   - Information disclosure
   - Missing security headers
   - Improper error handling

4. **Low**: Consider fixing
   - Outdated dependencies
   - Code quality issues
   - Documentation gaps
   - Minor configuration issues

### Processing Behavior

When `criticalOnly=true`:
- Critical issues: Attempted to fix
- High/Medium/Low issues: Skipped (counted in `SkippedIssues`)

When `criticalOnly=false`:
- All issues: Attempted to fix
- No issues skipped based on severity

## Best Practices

### Before Running Fixes

1. **Backup Your Project**: While the package can create backups, ensure you have version control or external backups.

2. **Run in Test Environment**: Test fixes on a non-production copy first.

3. **Review Fix Plans**: For large projects, review the scan results before applying fixes.

```go
// Review without fixing (if auto_fix is false in config)
result, _ := fix.FixAllCriticalSecurityIssues(projectPath, true)
log.Printf("Would fix %d issues", result.TotalIssues)
```

### After Running Fixes

1. **Review Changes**: Check the fixes applied to ensure they don't break functionality.

2. **Run Tests**: Execute your test suite after applying fixes.

3. **Check Validation Results**: Review the post-fix security score.

```go
result, _ := fix.FixAllCriticalSecurityIssues(projectPath, true)

if result.Validation != nil && result.Validation.ScanResult != nil {
    if result.Validation.ScanResult.SecurityScore < 80 {
        log.Println("Warning: Security score still below 80")
    }
}
```

### Handling Manual Fixes

Some issues cannot be automatically fixed and require manual intervention:

```go
result, _ := fix.FixAllCriticalSecurityIssues(projectPath, true)

if result.ManualFixes > 0 {
    log.Printf("Manual fixes required: %d", result.ManualFixes)
    // Review the security scan report for details
    // Each manual fix will have documentation on how to resolve
}
```

## Integration Patterns

### CI/CD Pipeline Integration

```go
func ciSecurityCheck(projectPath string) error {
    result, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
    if err != nil {
        return fmt.Errorf("security fix failed: %w", err)
    }

    // Fail CI if critical issues remain
    if result.Validation != nil && result.Validation.RemainingCriticalIssues > 0 {
        return fmt.Errorf("critical security issues remain: %d",
            result.Validation.RemainingCriticalIssues)
    }

    // Warn but don't fail for high issues
    if result.Validation != nil && result.Validation.RemainingHighIssues > 0 {
        log.Printf("Warning: %d high-severity issues remain",
            result.Validation.RemainingHighIssues)
    }

    return nil
}
```

### With Security Package

```go
import (
    "dev.helix.code/internal/fix"
    "dev.helix.code/internal/security"
)

func securityWorkflow(projectPath string) error {
    // Initialize security manager (required before fixing)
    if err := security.InitGlobalSecurityManager(); err != nil {
        return fmt.Errorf("security init failed: %w", err)
    }

    // Perform initial scan
    scanResult, err := security.GetGlobalSecurityManager().ScanFeature("pre_fix_scan")
    if err != nil {
        return err
    }

    log.Printf("Initial security score: %d", scanResult.SecurityScore)

    // Apply fixes
    result, err := fix.FixAllCriticalSecurityIssues(projectPath, false)
    if err != nil {
        return err
    }

    // Compare scores
    if result.Validation != nil && result.Validation.ScanResult != nil {
        improvement := result.Validation.ScanResult.SecurityScore - scanResult.SecurityScore
        log.Printf("Security score improved by %d points", improvement)
    }

    return nil
}
```

### With Logging Package

```go
import (
    "dev.helix.code/internal/fix"
    "dev.helix.code/internal/logging"
)

func fixWithLogging(projectPath string) (*fix.FixResult, error) {
    logger := logging.DefaultLogger()

    logger.Info("Starting security fix operation")
    logger.Info("Project: %s", projectPath)

    result, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
    if err != nil {
        logger.Error("Fix operation failed: %v", err)
        return nil, err
    }

    logger.Info("Fix operation completed:")
    logger.Info("  Total Issues: %d", result.TotalIssues)
    logger.Info("  Fixed: %d", result.FixedIssues)
    logger.Info("  Failed: %d", result.FailedFixes)

    if result.FailedFixes > 0 {
        logger.Warn("Some fixes failed - review required")
    }

    if result.Success {
        logger.Info("All critical issues resolved")
    }

    return result, nil
}
```

## Error Handling

The package returns errors for critical failures:

```go
result, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "security manager"):
        log.Println("Security manager not initialized")
    case strings.Contains(err.Error(), "security scan"):
        log.Println("Security scan failed")
    case strings.Contains(err.Error(), "validation scan"):
        log.Println("Post-fix validation failed")
    default:
        log.Printf("Unexpected error: %v", err)
    }
}
```

Individual fix failures are tracked in `FailedFixes` rather than causing the overall operation to fail.

## Testing

```bash
# Run all fix package tests
go test -v ./internal/fix/...

# Run with coverage
go test -cover ./internal/fix/...

# Run with race detector
go test -race ./internal/fix/...
```

### Test Utilities

```go
func TestFixOperation(t *testing.T) {
    // Ensure security manager is initialized
    require.NoError(t, security.InitGlobalSecurityManager())

    // Create temporary project
    tmpDir := t.TempDir()

    // Create test files
    mainFile := filepath.Join(tmpDir, "main.go")
    require.NoError(t, os.WriteFile(mainFile, []byte("package main"), 0644))

    // Run fix operation
    result, err := fix.FixAllCriticalSecurityIssues(tmpDir, true)

    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.NotNil(t, result.Validation)
    assert.False(t, result.StartTime.IsZero())
    assert.False(t, result.EndTime.IsZero())
}
```

## Performance Considerations

- **Scan Time**: Depends on project size and number of files
- **Fix Time**: Varies based on issue complexity
- **Validation Time**: Additional scan after fixes
- **Concurrent Operations**: Supported but may impact performance on large codebases

For large projects, consider:
- Running fixes incrementally by directory
- Using `criticalOnly=true` for faster initial passes
- Scheduling comprehensive scans during off-peak hours

## Related Packages

- `internal/security`: Core security scanning and validation
- `internal/logging`: Progress reporting and diagnostics
- `internal/workflow`: Can trigger fix operations as workflow steps
- `internal/task`: Can wrap fix operations as tasks
