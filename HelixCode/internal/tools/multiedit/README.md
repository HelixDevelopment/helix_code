# Multiedit Package

The `multiedit` package provides atomic multi-file editing capabilities with transactional semantics for HelixCode. It ensures that complex code modifications across multiple files either succeed completely or can be safely rolled back.

## Overview

This package enables:
- Atomic multi-file editing with ACID-like transaction semantics
- Automatic backup creation before modifications
- Rollback capability for failed operations
- Conflict detection and resolution
- Preview mode for reviewing changes before commit
- Git-aware operations with optional auto-commit
- Unified diff generation for change visualization

## Key Types

### MultiEditor

The main coordinator for multi-file edit operations.

```go
type MultiEditor struct {
    txManager      *TransactionManager
    conflictResolver *ConflictResolver
    executor       *EditExecutor
    config         *MultiEditConfig
    mu             sync.RWMutex
}

type MultiEditConfig struct {
    BackupEnabled   bool          // Create backups before editing
    BackupDir       string        // Backup storage directory
    MaxFileSize     int64         // Maximum file size to edit
    Timeout         time.Duration // Transaction timeout
    GitAware        bool          // Integrate with git
    AutoCommit      bool          // Auto-commit on success
    PreviewByDefault bool         // Always preview first
    MaxFilesPerTx   int           // Max files per transaction
}
```

### TransactionManager

Manages the lifecycle of edit transactions.

```go
type TransactionManager struct {
    mu           sync.RWMutex
    transactions map[string]*EditTransaction
    maxDuration  time.Duration
}
```

### EditTransaction

Represents a multi-file edit operation.

```go
type EditTransaction struct {
    ID          string
    State       TransactionState
    Files       []*FileEdit
    CreatedAt   time.Time
    UpdatedAt   time.Time
    Options     EditOptions
    Metadata    map[string]interface{}
    backupPaths map[string]string // file path -> backup path
    mu          sync.RWMutex
}
```

### TransactionState

Defines the states a transaction can be in.

```go
type TransactionState int

const (
    StatePending     TransactionState = iota // Initial state
    StatePreview                             // Changes being previewed
    StateReady                               // Ready to commit
    StateCommitting                          // Commit in progress
    StateCommitted                           // Successfully committed
    StateRollingBack                         // Rollback in progress
    StateRolledBack                          // Successfully rolled back
    StateAborted                             // Aborted by user/timeout
    StateFailed                              // Failed with error
)
```

### FileEdit

Represents a single file edit operation.

```go
type FileEdit struct {
    FilePath   string        // File to edit
    TargetPath string        // New path (for rename operations)
    Operation  EditOperation // create, update, delete, rename
    OldContent []byte        // Original content
    NewContent []byte        // New content
    Checksum   string        // SHA256 of original content
    Applied    bool          // Whether edit has been applied
    Error      error         // Any error during application
}
```

### EditOperation

Types of file operations supported.

```go
type EditOperation int

const (
    OpCreate EditOperation = iota // Create new file
    OpUpdate                      // Modify existing file
    OpDelete                      // Delete file
    OpRename                      // Rename/move file
)
```

### EditOptions

Configuration for edit behavior.

```go
type EditOptions struct {
    DryRun         bool           // Preview without applying
    ConflictPolicy ConflictPolicy // How to handle conflicts
    BackupEnabled  bool           // Create backups
    GitAware       bool           // Git integration
    PreCommitHooks bool           // Run pre-commit hooks
    MaxFileSize    int64          // Max file size
    AllowedPaths   []string       // Allowed path patterns
    DeniedPaths    []string       // Denied path patterns
}
```

### ConflictPolicy

Defines how conflicts are handled.

```go
type ConflictPolicy int

const (
    ConflictPolicyAbort     ConflictPolicy = iota // Abort on conflict
    ConflictPolicySkip                            // Skip conflicting files
    ConflictPolicyOverwrite                       // Overwrite conflicts
    ConflictPolicyAsk                             // Ask user for each
)
```

### ConflictResolver

Detects and resolves edit conflicts.

```go
type ConflictResolver struct {
    gitIntegration *GitIntegration
    detectGit      bool
}

type Conflict struct {
    Type        ConflictType
    FilePath    string
    Expected    string // Expected checksum
    Actual      string // Actual checksum
    Description string
    Resolution  *ConflictResolution
}

type ConflictType int

const (
    ConflictModified    ConflictType = iota // File modified since read
    ConflictDeleted                         // File deleted since read
    ConflictMoved                           // File moved since read
    ConflictPermissions                     // Permission changed
)
```

## Usage Examples

### Basic Multi-File Edit

```go
package main

import (
    "context"
    "fmt"

    "dev.helix.code/internal/tools/multiedit"
)

func main() {
    // Create multi-editor
    editor := multiedit.NewMultiEditor(&multiedit.MultiEditConfig{
        BackupEnabled: true,
        BackupDir:     ".helix/backups",
        Timeout:       5 * time.Minute,
        GitAware:      true,
    })

    ctx := context.Background()

    // Begin transaction
    tx, err := editor.Begin(ctx, &multiedit.EditOptions{
        BackupEnabled:  true,
        ConflictPolicy: multiedit.ConflictPolicyAbort,
    })
    if err != nil {
        panic(err)
    }

    // Add file edits
    err = editor.AddEdit(tx, &multiedit.FileEdit{
        FilePath:   "src/main.go",
        Operation:  multiedit.OpUpdate,
        NewContent: []byte("package main\n\nfunc main() {\n    // Updated\n}\n"),
    })
    if err != nil {
        editor.Abort(tx)
        panic(err)
    }

    err = editor.AddEdit(tx, &multiedit.FileEdit{
        FilePath:   "src/utils.go",
        Operation:  multiedit.OpCreate,
        NewContent: []byte("package main\n\n// Utility functions\n"),
    })
    if err != nil {
        editor.Abort(tx)
        panic(err)
    }

    // Commit the transaction
    result, err := editor.Commit(ctx, tx)
    if err != nil {
        // Transaction automatically rolls back on error
        panic(err)
    }

    fmt.Printf("Transaction %s committed successfully\n", result.TransactionID)
    fmt.Printf("Files modified: %d\n", len(result.FilesModified))
}
```

### Preview Mode

```go
// Begin transaction with preview
tx, err := editor.Begin(ctx, &multiedit.EditOptions{
    DryRun: true,
})

// Add edits
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:   "src/handler.go",
    Operation:  multiedit.OpUpdate,
    NewContent: []byte("// Updated handler code"),
})

// Preview changes without applying
preview, err := editor.Preview(ctx, tx)

fmt.Printf("Preview for transaction %s:\n", preview.TransactionID)
for _, change := range preview.Changes {
    fmt.Printf("  [%s] %s\n", change.Operation, change.FilePath)
    fmt.Printf("  Diff:\n%s\n", change.UnifiedDiff)
}

// User reviews and approves
if userApproves {
    // Transition to ready and commit
    err = editor.MarkReady(tx)
    result, err := editor.Commit(ctx, tx)
} else {
    editor.Abort(tx)
}
```

### Conflict Detection and Resolution

```go
// Begin transaction
tx, err := editor.Begin(ctx, &multiedit.EditOptions{
    ConflictPolicy: multiedit.ConflictPolicyAsk,
})

// Read current content and compute checksum
content, _ := os.ReadFile("src/config.go")
checksum := multiedit.CalculateChecksum(content)

// Add edit with checksum for conflict detection
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:   "src/config.go",
    Operation:  multiedit.OpUpdate,
    OldContent: content,
    NewContent: []byte("// Updated config"),
    Checksum:   checksum,
})

// Check for conflicts before committing
conflicts, err := editor.DetectConflicts(ctx, tx)

if len(conflicts) > 0 {
    for _, conflict := range conflicts {
        fmt.Printf("Conflict in %s: %s\n", conflict.FilePath, conflict.Description)

        // Resolve conflict based on user input
        switch userChoice {
        case "keep-theirs":
            editor.ResolveConflict(conflict, multiedit.StrategyTheirs)
        case "keep-ours":
            editor.ResolveConflict(conflict, multiedit.StrategyOurs)
        case "abort":
            editor.Abort(tx)
            return
        }
    }
}

// Commit with resolved conflicts
result, err := editor.Commit(ctx, tx)
```

### File Operations

```go
// Create new file
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:   "src/new_module/handler.go",
    Operation:  multiedit.OpCreate,
    NewContent: []byte("package new_module\n"),
})

// Update existing file
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:   "src/main.go",
    Operation:  multiedit.OpUpdate,
    NewContent: []byte("// Updated main.go"),
})

// Delete file
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:  "src/deprecated.go",
    Operation: multiedit.OpDelete,
})

// Rename/move file
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:   "src/old_name.go",
    TargetPath: "src/new_name.go",
    Operation:  multiedit.OpRename,
})
```

### Rollback

```go
// Begin transaction
tx, err := editor.Begin(ctx, nil)

// Add edits
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:   "src/main.go",
    Operation:  multiedit.OpUpdate,
    NewContent: []byte("// New content"),
})

// Start commit
result, err := editor.Commit(ctx, tx)
if err != nil {
    fmt.Printf("Commit failed: %v\n", err)
    // Transaction is already rolled back
    return
}

// Later, if we need to manually rollback
if needsRollback {
    err = editor.Rollback(ctx, tx)
    if err != nil {
        fmt.Printf("Rollback failed: %v\n", err)
    }
}
```

### Transaction Management

```go
// Get transaction by ID
tx, err := editor.GetTransaction(txID)

// List all active transactions
transactions := editor.ListTransactions()
for _, tx := range transactions {
    fmt.Printf("TX %s: %s (created: %s)\n",
        tx.ID, tx.State, tx.CreatedAt)
}

// Cleanup old transactions
cleaned := editor.Cleanup(24 * time.Hour)
fmt.Printf("Cleaned up %d old transactions\n", cleaned)

// Get transaction status
status := editor.GetStatus(tx)
fmt.Printf("State: %s\n", status.State)
fmt.Printf("Files: %d\n", status.FileCount)
fmt.Printf("Duration: %s\n", status.Duration)
```

### Batch Operations

```go
// Batch edit multiple files at once
edits := []*multiedit.FileEdit{
    {
        FilePath:   "src/a.go",
        Operation:  multiedit.OpUpdate,
        NewContent: []byte("// Updated A"),
    },
    {
        FilePath:   "src/b.go",
        Operation:  multiedit.OpUpdate,
        NewContent: []byte("// Updated B"),
    },
    {
        FilePath:   "src/c.go",
        Operation:  multiedit.OpCreate,
        NewContent: []byte("// New C"),
    },
}

// Execute as single atomic operation
result, err := editor.ExecuteBatch(ctx, edits, &multiedit.EditOptions{
    BackupEnabled: true,
})
```

### Git Integration

```go
// Create git-aware editor
editor := multiedit.NewMultiEditor(&multiedit.MultiEditConfig{
    GitAware:   true,
    AutoCommit: true,
})

tx, err := editor.Begin(ctx, &multiedit.EditOptions{
    GitAware:       true,
    PreCommitHooks: true,
})

// Add edits
editor.AddEdit(tx, &multiedit.FileEdit{
    FilePath:   "src/feature.go",
    Operation:  multiedit.OpCreate,
    NewContent: []byte("// New feature"),
})

// Commit transaction with git commit
result, err := editor.Commit(ctx, tx)

fmt.Printf("Git commit: %s\n", result.GitCommitHash)
fmt.Printf("Message: %s\n", result.GitCommitMessage)
```

## Configuration Options

### MultiEditConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `BackupEnabled` | bool | true | Create backups before editing |
| `BackupDir` | string | .helix/backups | Backup storage directory |
| `MaxFileSize` | int64 | 10MB | Maximum file size to edit |
| `Timeout` | time.Duration | 5m | Transaction timeout |
| `GitAware` | bool | false | Enable git integration |
| `AutoCommit` | bool | false | Auto-commit on success |
| `PreviewByDefault` | bool | false | Always preview first |
| `MaxFilesPerTx` | int | 100 | Max files per transaction |

### EditOptions

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `DryRun` | bool | false | Preview without applying |
| `ConflictPolicy` | ConflictPolicy | Abort | Conflict handling strategy |
| `BackupEnabled` | bool | true | Create backups |
| `GitAware` | bool | false | Git integration |
| `PreCommitHooks` | bool | false | Run pre-commit hooks |
| `MaxFileSize` | int64 | 10MB | Max file size |
| `AllowedPaths` | []string | [] | Allowed path patterns |
| `DeniedPaths` | []string | [] | Denied path patterns |

## Transaction State Machine

```
                    +------------+
                    |  Pending   |
                    +-----+------+
                          |
                    +-----v------+
               +--->|  Preview   |<---+
               |    +-----+------+    |
               |          |           |
               |    +-----v------+    |
               |    |   Ready    |----+
               |    +-----+------+
               |          |
        +------+    +-----v------+
        |Abort |<---|Committing  |
        +------+    +-----+------+
                          |
              +-----------+-----------+
              |                       |
        +-----v------+         +------v-----+
        | Committed  |         |   Failed   |
        +------------+         +-----+------+
                                     |
                               +-----v------+
                               |RollingBack |
                               +-----+------+
                                     |
                               +-----v------+
                               |RolledBack  |
                               +------------+
```

## Security Considerations

1. **Backup Creation**: Always enable backups for production systems to allow recovery from mistakes.

2. **Path Validation**: Configure `AllowedPaths` and `DeniedPaths` to restrict which files can be modified.

3. **Checksum Verification**: Use checksums to detect unexpected file modifications and prevent data loss.

4. **Transaction Timeouts**: Set appropriate timeouts to prevent orphaned transactions from holding resources.

5. **Git Integration**: When using git-aware mode, ensure proper authentication and permissions are configured.

6. **Atomic Operations**: The package uses atomic file operations to prevent partial writes and data corruption.

## Error Types

```go
var (
    ErrTransactionNotFound = errors.New("transaction not found")
    ErrInvalidState        = errors.New("invalid transaction state")
    ErrConflictDetected    = errors.New("conflict detected")
    ErrBackupFailed        = errors.New("backup failed")
    ErrRollbackFailed      = errors.New("rollback failed")
    ErrFileNotFound        = errors.New("file not found")
    ErrChecksumMismatch    = errors.New("checksum mismatch")
    ErrPermissionDenied    = errors.New("permission denied")
    ErrTransactionTimeout  = errors.New("transaction timeout")
    ErrInvalidPath         = errors.New("invalid file path")
)
```

## Best Practices

1. **Always use transactions** for multi-file operations to ensure atomicity.

2. **Preview changes** before committing for critical operations.

3. **Enable backups** to allow recovery from unexpected issues.

4. **Set appropriate timeouts** to prevent long-running transactions.

5. **Use checksums** for conflict detection when editing files that may be modified externally.

6. **Handle errors appropriately** - check for specific error types to provide meaningful feedback.

7. **Clean up old transactions** periodically to free resources.
