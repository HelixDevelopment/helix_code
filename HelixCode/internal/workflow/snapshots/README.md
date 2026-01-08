# Snapshots Package

The `snapshots` package provides git-based workspace snapshot functionality for HelixCode, implementing a checkpoint system that allows users to save, compare, and restore workspace states at different points in time.

## Overview

The snapshots package enables:
- Workspace snapshot creation using git stash
- Comparison between any two snapshots with detailed diffs
- Safe restoration to specific snapshots with conflict detection
- Rich metadata tracking (timestamp, task, description, file statistics)
- Filtering and querying snapshots
- Automatic backup creation before destructive operations

## Key Types and Interfaces

### Snapshot

Represents a workspace snapshot with metadata:

```go
type Snapshot struct {
    ID          string         // Unique identifier (helix-snapshot-{uuid})
    StashRef    string         // Git stash reference (stash@{n})
    StashIndex  int            // Index in stash list
    CreatedAt   time.Time      // Creation timestamp
    Description string         // Human-readable description
    TaskID      string         // Associated task ID (optional)
    Status      SnapshotStatus // active, archived, corrupted
    Metadata    *Metadata      // Detailed metadata
    Tags        []string       // Custom tags
    FileCount   int            // Number of files in snapshot
    Size        int64          // Approximate size in bytes
}
```

### Metadata

Contains detailed snapshot information:

```go
type Metadata struct {
    WorkingDirectory string   // Repository path
    Branch           string   // Git branch name
    CommitHash       string   // Current commit hash

    FilesAdded     []string // Files added
    FilesModified  []string // Files modified
    FilesDeleted   []string // Files deleted
    UntrackedFiles []string // Untracked files

    TaskDescription string            // Task description
    TaskStep        int               // Task step number
    HelixVersion    string            // HelixCode version
    Custom          map[string]string // Custom metadata
}
```

### Manager

The main orchestrator for snapshot operations:

```go
type Manager struct {
    repoPath      string
    metadataStore *MetadataStore
}
```

### Comparison

Represents differences between two snapshots:

```go
type Comparison struct {
    From       *Snapshot   // Source snapshot
    To         *Snapshot   // Target snapshot
    Summary    *Summary    // High-level statistics
    FileDiffs  []*FileDiff // Per-file differences
    Statistics *Statistics // Detailed statistics
}
```

## Usage Examples

### Creating a Snapshot Manager

```go
manager, err := snapshots.NewManager("/path/to/git/repo")
if err != nil {
    log.Fatalf("Failed to create manager: %v", err)
}
```

### Creating Snapshots

```go
// Basic snapshot
snapshot, err := manager.CreateSnapshot(ctx, &snapshots.CreateOptions{
    Description: "Before refactoring auth module",
})

// Snapshot with full options
snapshot, err := manager.CreateSnapshot(ctx, &snapshots.CreateOptions{
    Description:      "Pre-migration checkpoint",
    TaskID:           "task-123",
    Tags:             []string{"stable", "pre-release"},
    IncludeUntracked: true,
    Metadata: map[string]string{
        "developer": "john",
        "sprint":    "2024-01",
    },
    AutoGenerate: false, // Set true to auto-generate description
})

if err != nil {
    log.Printf("Failed to create snapshot: %v", err)
}
fmt.Printf("Created snapshot: %s\n", snapshot.ID)
```

### Listing and Filtering Snapshots

```go
// List all snapshots
allSnapshots, err := manager.ListSnapshots(ctx, nil)

// Filter by task ID
taskSnapshots, err := manager.ListSnapshots(ctx, &snapshots.Filter{
    TaskID: "task-123",
})

// Filter by tags and date range
filteredSnapshots, err := manager.ListSnapshots(ctx, &snapshots.Filter{
    Tags:     []string{"stable"},
    Status:   snapshots.StatusActive,
    FromDate: time.Now().AddDate(0, -1, 0), // Last month
    ToDate:   time.Now(),
    Limit:    10,
})

for _, s := range filteredSnapshots {
    fmt.Printf("%s: %s (%d files)\n", s.ID, s.Description, s.FileCount)
}
```

### Comparing Snapshots

```go
// Compare two snapshots
comparison, err := manager.CompareSnapshots(ctx, snapshot1.ID, snapshot2.ID)
if err != nil {
    log.Fatal(err)
}

// Access summary statistics
fmt.Printf("Files added: %d\n", comparison.Summary.FilesAdded)
fmt.Printf("Files modified: %d\n", comparison.Summary.FilesModified)
fmt.Printf("Files deleted: %d\n", comparison.Summary.FilesDeleted)
fmt.Printf("Lines added: %d\n", comparison.Summary.LinesAdded)
fmt.Printf("Lines deleted: %d\n", comparison.Summary.LinesDeleted)
fmt.Printf("Time elapsed: %v\n", comparison.Summary.TimeElapsed)

// Iterate through file diffs
for _, diff := range comparison.FileDiffs {
    fmt.Printf("%s: %s (+%d/-%d)\n",
        diff.Path, diff.Status, diff.LinesAdded, diff.LinesDeleted)

    // Full unified diff is available in diff.Diff
}
```

### Restoring Snapshots

```go
// Preview restore (dry run)
preview, err := manager.PreviewRestore(ctx, snapshot.ID)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Would restore %d files\n", len(preview.FilesRestored))

// Validate restore is safe
err = manager.ValidateRestore(ctx, snapshot.ID)
if err != nil {
    log.Printf("Restore validation failed: %v", err)
}

// Restore with automatic backup
result, err := manager.RestoreSnapshot(ctx, snapshot.ID, &snapshots.RestoreOptions{
    CreateBackup: true,  // Create backup before restore
    DryRun:       false, // Actually perform restore
    Force:        false, // Require clean working tree
    KeepIndex:    false, // Restore index as well
})

if !result.Success {
    log.Printf("Restore failed: %v", result.Errors)
    if len(result.ConflictFiles) > 0 {
        fmt.Printf("Conflicts in: %v\n", result.ConflictFiles)
    }
    return
}

fmt.Printf("Restored %d files in %v\n", len(result.FilesRestored), result.Duration)
if result.BackupSnapshot != nil {
    fmt.Printf("Backup created: %s\n", result.BackupSnapshot.ID)
}
```

### Checking for Conflicts

```go
// Predict potential conflicts before restore
conflicts, err := manager.GetRestoreConflicts(ctx, snapshot.ID)
if err != nil {
    log.Fatal(err)
}

if len(conflicts) > 0 {
    fmt.Printf("Potential conflicts in %d files:\n", len(conflicts))
    for _, file := range conflicts {
        fmt.Printf("  - %s\n", file)
    }
}
```

### Working with Snapshot Contents

```go
// Get list of files in snapshot
files, err := manager.GetSnapshotFiles(ctx, snapshot.ID)
for _, file := range files {
    fmt.Printf("  %s\n", file)
}

// Get content of specific file from snapshot
content, err := manager.GetFileContent(ctx, snapshot.ID, "main.go")
fmt.Printf("File content:\n%s\n", content)

// Get diff statistics
stat, err := manager.GetDiffStat(ctx, snapshot.ID)
fmt.Printf("Diff stat:\n%s\n", stat)

// Get snapshot size
size, err := manager.GetSnapshotSize(ctx, snapshot.ID)
fmt.Printf("Snapshot size: %d bytes\n", size)
```

### Deleting Snapshots

```go
err := manager.DeleteSnapshot(ctx, snapshot.ID)
if err != nil {
    log.Printf("Failed to delete snapshot: %v", err)
}
```

### Rollback After Failed Restore

```go
result, err := manager.RestoreSnapshot(ctx, targetID, opts)
if !result.Success && result.BackupSnapshot != nil {
    // Rollback to the backup
    err := manager.RollbackRestore(ctx, result.BackupSnapshot.ID)
    if err != nil {
        log.Printf("Rollback failed: %v", err)
    }
}
```

## Configuration Options

### CreateOptions

```go
type CreateOptions struct {
    Description      string            // Human-readable description
    TaskID           string            // Associated task ID
    Tags             []string          // Custom tags for filtering
    IncludeUntracked bool              // Include untracked files
    Metadata         map[string]string // Custom key-value metadata
    AutoGenerate     bool              // Auto-generate description from changes
}
```

### RestoreOptions

```go
type RestoreOptions struct {
    CreateBackup bool // Create backup before restore (default: true)
    DryRun       bool // Preview without applying changes
    Force        bool // Force restore even with uncommitted changes
    KeepIndex    bool // Keep current staged changes
}
```

### Filter

```go
type Filter struct {
    TaskID   string         // Filter by task ID
    Tags     []string       // Filter by tags (any match)
    Status   SnapshotStatus // Filter by status
    FromDate time.Time      // Created after this date
    ToDate   time.Time      // Created before this date
    Limit    int            // Maximum results to return
}
```

## Integration Patterns

### With Task System

```go
// Create checkpoint before task execution
func executeTask(ctx context.Context, taskID string, manager *snapshots.Manager) error {
    // Create pre-task snapshot
    snapshot, err := manager.CreateSnapshot(ctx, &snapshots.CreateOptions{
        Description: "Pre-task checkpoint",
        TaskID:      taskID,
        Tags:        []string{"auto", "pre-task"},
    })
    if err != nil {
        return fmt.Errorf("failed to create checkpoint: %w", err)
    }

    // Execute task...
    err = doTaskWork(ctx)

    if err != nil {
        // Restore on failure
        _, restoreErr := manager.RestoreSnapshot(ctx, snapshot.ID, &snapshots.RestoreOptions{
            CreateBackup: false,
            Force:        true,
        })
        if restoreErr != nil {
            return fmt.Errorf("task failed and restore failed: %w", restoreErr)
        }
        return fmt.Errorf("task failed, restored to checkpoint: %w", err)
    }

    return nil
}
```

### With Workflow Engine

```go
// Checkpoint at each workflow step
func executeWorkflowStep(ctx context.Context, step *Step, manager *snapshots.Manager) error {
    snapshot, _ := manager.CreateSnapshot(ctx, &snapshots.CreateOptions{
        Description: fmt.Sprintf("Step %d: %s", step.Number, step.Name),
        TaskID:      step.WorkflowID,
        Metadata: map[string]string{
            "step_number": strconv.Itoa(step.Number),
            "step_type":   step.Type,
        },
    })

    // Store snapshot ID for potential rollback
    step.CheckpointID = snapshot.ID

    return executeStep(ctx, step)
}
```

### With Autonomy System

```go
// Create snapshot before risky operations
func executeRiskyAction(ctx context.Context, action *autonomy.Action) error {
    if action.IsRisky() {
        _, err := snapshotManager.CreateSnapshot(ctx, &snapshots.CreateOptions{
            Description: fmt.Sprintf("Pre-action: %s", action.Description),
            Tags:        []string{"auto", "risky-action"},
        })
        if err != nil {
            return fmt.Errorf("failed to create safety checkpoint: %w", err)
        }
    }

    return executeAction(ctx, action)
}
```

## Architecture

### Storage Design

- **Snapshots**: Stored using git stash with tagged names (`helix-snapshot-{uuid}: {description}`)
- **Metadata**: Persisted to `.helix.snapshots.json` (should be gitignored)
- **Efficiency**: Leverages Git's object storage for deduplication and compression

### Thread Safety

The `MetadataStore` uses mutex locks for thread-safe operations:

```go
type MetadataStore struct {
    repoPath string
    mu       sync.RWMutex
    cache    map[string]*Snapshot
}
```

### File Structure

```
repo/
  .helix.snapshots.json  # Metadata storage (gitignored)
  .git/
    refs/stash           # Git stash storage
```

## Safety Features

1. **Clean Working Tree Verification**: By default, restore requires a clean working tree
2. **Conflict Detection**: Detects and reports merge conflicts during restore
3. **Automatic Backup**: Creates backup snapshot before destructive operations
4. **Snapshot Verification**: Validates snapshot exists in git stash before operations
5. **Atomic Metadata Updates**: Uses temp file + rename for safe metadata writes

## Limitations

- Requires a git repository (initialized with `git init`)
- Snapshot IDs must be unique across the repository
- Large binary files may slow down operations
- Restore conflicts require manual resolution
- Stash-based storage means snapshots can be affected by `git stash` commands

## Error Handling

Common error scenarios and handling:

```go
snapshot, err := manager.CreateSnapshot(ctx, opts)
if err != nil {
    if strings.Contains(err.Error(), "no changes") {
        // No local changes to snapshot
        log.Println("Nothing to snapshot")
    } else if strings.Contains(err.Error(), "not a git repository") {
        // Path is not a git repo
        log.Println("Not a git repository")
    } else {
        log.Printf("Unexpected error: %v", err)
    }
}
```

## Performance Considerations

- Snapshots share objects with the repository (Git deduplication)
- Only changed files are stored
- Git's compression is applied automatically
- Metadata file is kept separate for fast querying without git operations
- List operations filter in-memory after loading metadata
