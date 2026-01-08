# Filesystem Package

The `filesystem` package provides secure, controlled file system operations for HelixCode. It implements comprehensive path validation, workspace restrictions, and atomic write operations to ensure safe AI-driven file manipulation.

## Overview

This package enables:
- Secure file reading with configurable size limits and encoding detection
- Atomic file writing with backup creation and permission preservation
- Pattern-based file searching with glob and regex support
- Line-based and range-based file editing
- Path validation with workspace boundary enforcement
- Symlink handling and directory traversal prevention

## Key Types

### FileSystem

The main coordinator for file system operations.

```go
type FileSystem struct {
    reader     *FileReader
    writer     *FileWriter
    searcher   *FileSearcher
    editor     *FileEditor
    config     *FileSystemConfig
    mu         sync.RWMutex
}
```

### FileSystemConfig

Configuration for file system operations.

```go
type FileSystemConfig struct {
    WorkspaceRoot    string        // Root directory for operations
    AllowedPaths     []string      // Additional allowed paths
    DeniedPaths      []string      // Explicitly denied paths
    MaxFileSize      int64         // Maximum readable file size
    MaxSearchResults int           // Limit search results
    FollowSymlinks   bool          // Whether to follow symlinks
    CreateBackups    bool          // Create backups before writes
    BackupDir        string        // Directory for backups
    DefaultEncoding  string        // Default file encoding
    Timeout          time.Duration // Operation timeout
}
```

### FileReader

Handles file reading operations.

```go
type FileReader struct {
    config     *ReaderConfig
    cache      *FileCache
    mu         sync.RWMutex
}

type ReaderConfig struct {
    MaxSize          int64
    DefaultEncoding  string
    DetectEncoding   bool
    ChunkSize        int
    CacheEnabled     bool
    CacheTTL         time.Duration
}
```

### FileWriter

Handles file writing with atomic operations.

```go
type FileWriter struct {
    config     *WriterConfig
    backupMgr  *BackupManager
    mu         sync.RWMutex
}

type WriterConfig struct {
    CreateBackups     bool
    BackupDir         string
    PreservePerms     bool
    DefaultPerms      os.FileMode
    AtomicWrites      bool
    SyncAfterWrite    bool
}
```

### FileSearcher

Provides file and content search capabilities.

```go
type FileSearcher struct {
    config     *SearchConfig
    mu         sync.RWMutex
}

type SearchConfig struct {
    MaxResults       int
    MaxDepth         int
    IncludeHidden    bool
    FollowSymlinks   bool
    IgnorePatterns   []string
    FileTypes        []string
    CaseSensitive    bool
}
```

### FileEditor

Enables in-place file modifications.

```go
type FileEditor struct {
    reader     *FileReader
    writer     *FileWriter
    mu         sync.RWMutex
}
```

## Usage Examples

### Reading Files

```go
package main

import (
    "context"
    "fmt"

    "dev.helix.code/internal/tools/filesystem"
)

func main() {
    config := &filesystem.FileSystemConfig{
        WorkspaceRoot: "/home/user/project",
        MaxFileSize:   10 * 1024 * 1024, // 10MB
    }

    fs, err := filesystem.NewFileSystem(config)
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    // Read entire file
    content, err := fs.ReadFile(ctx, "src/main.go")
    if err != nil {
        panic(err)
    }
    fmt.Println(string(content))

    // Read with line range
    lines, err := fs.ReadLines(ctx, "src/main.go", 10, 20)
    if err != nil {
        panic(err)
    }
    for i, line := range lines {
        fmt.Printf("%d: %s\n", 10+i, line)
    }

    // Read file info
    info, err := fs.GetFileInfo(ctx, "src/main.go")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Size: %d, Modified: %s\n", info.Size, info.ModTime)
}
```

### Writing Files

```go
// Write new file
err := fs.WriteFile(ctx, "src/new_file.go", []byte("package main\n"))

// Write with specific permissions
err = fs.WriteFileWithPerms(ctx, "scripts/run.sh", []byte("#!/bin/bash\necho hello\n"), 0755)

// Atomic write (writes to temp file, then renames)
err = fs.AtomicWrite(ctx, "config/settings.yaml", []byte("key: value\n"))

// Append to file
err = fs.AppendFile(ctx, "logs/app.log", []byte("New log entry\n"))

// Create backup before modifying
backupPath, err := fs.CreateBackup(ctx, "important.txt")
fmt.Printf("Backup created at: %s\n", backupPath)
```

### Searching Files

```go
// Search by glob pattern
files, err := fs.Glob(ctx, "**/*.go")
for _, f := range files {
    fmt.Println(f)
}

// Search with exclusions
files, err = fs.Search(ctx, &filesystem.SearchQuery{
    Pattern:        "*.js",
    ExcludePattern: []string{"node_modules/**", "dist/**"},
    MaxResults:     100,
})

// Search file contents
results, err := fs.SearchContent(ctx, &filesystem.ContentSearchQuery{
    Pattern:       "func.*Error",
    IsRegex:       true,
    FilePattern:   "*.go",
    CaseSensitive: false,
    ContextLines:  2,
})

for _, result := range results {
    fmt.Printf("%s:%d: %s\n", result.FilePath, result.LineNumber, result.Line)
}

// Find files by type
files, err = fs.FindByType(ctx, &filesystem.TypeQuery{
    Types:     []string{".go", ".py", ".js"},
    MinSize:   1024,
    MaxSize:   1024 * 1024,
    ModifiedAfter: time.Now().Add(-24 * time.Hour),
})
```

### Editing Files

```go
// Replace text in file
err := fs.Replace(ctx, "src/main.go", &filesystem.ReplaceOptions{
    Old:       "oldFunction",
    New:       "newFunction",
    All:       true,
    UseRegex:  false,
})

// Edit specific lines
err = fs.EditLines(ctx, "src/main.go", &filesystem.LineEdit{
    StartLine: 10,
    EndLine:   15,
    NewContent: []string{
        "// Updated comment",
        "func newImplementation() {",
        "    return nil",
        "}",
    },
})

// Insert at line
err = fs.InsertAt(ctx, "src/main.go", 5, "// Inserted comment\n")

// Delete lines
err = fs.DeleteLines(ctx, "src/main.go", 20, 25)

// Apply multiple edits atomically
err = fs.ApplyEdits(ctx, "src/main.go", []filesystem.Edit{
    {Type: filesystem.EditReplace, Line: 10, Old: "foo", New: "bar"},
    {Type: filesystem.EditInsert, Line: 20, Content: "// new line"},
    {Type: filesystem.EditDelete, Line: 30},
})
```

### Directory Operations

```go
// Create directory
err := fs.CreateDir(ctx, "src/new_package")

// Create nested directories
err = fs.CreateDirAll(ctx, "src/deeply/nested/package")

// List directory contents
entries, err := fs.ListDir(ctx, "src")
for _, entry := range entries {
    if entry.IsDir {
        fmt.Printf("[DIR] %s\n", entry.Name)
    } else {
        fmt.Printf("[FILE] %s (%d bytes)\n", entry.Name, entry.Size)
    }
}

// Remove directory (must be empty)
err = fs.RemoveDir(ctx, "src/old_package")

// Remove directory recursively
err = fs.RemoveDirAll(ctx, "src/deprecated")

// Copy directory
err = fs.CopyDir(ctx, "src/template", "src/new_module")
```

### Path Validation

```go
// Check if path is within workspace
valid := fs.IsValidPath(ctx, "/home/user/project/src/main.go")

// Resolve path relative to workspace
absPath, err := fs.ResolvePath(ctx, "./src/main.go")

// Check path safety (no traversal, valid characters)
err = fs.ValidatePath(ctx, "../../../etc/passwd")
// Returns: ErrPathTraversal

// Get relative path from workspace root
relPath, err := fs.RelativePath(ctx, "/home/user/project/src/main.go")
// Returns: "src/main.go"
```

## Configuration Options

### FileSystemConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WorkspaceRoot` | string | cwd | Root directory for all operations |
| `AllowedPaths` | []string | [] | Additional paths allowed outside workspace |
| `DeniedPaths` | []string | [] | Paths explicitly denied |
| `MaxFileSize` | int64 | 10MB | Maximum file size for reading |
| `MaxSearchResults` | int | 1000 | Limit on search results |
| `FollowSymlinks` | bool | false | Follow symbolic links |
| `CreateBackups` | bool | true | Create backups before writes |
| `BackupDir` | string | .helix/backups | Backup storage directory |
| `DefaultEncoding` | string | utf-8 | Default file encoding |
| `Timeout` | time.Duration | 30s | Operation timeout |

### ReaderConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `MaxSize` | int64 | 10MB | Maximum readable file size |
| `DefaultEncoding` | string | utf-8 | Default encoding |
| `DetectEncoding` | bool | true | Auto-detect file encoding |
| `ChunkSize` | int | 64KB | Buffer size for streaming |
| `CacheEnabled` | bool | true | Enable file content caching |
| `CacheTTL` | time.Duration | 5m | Cache entry TTL |

### WriterConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `CreateBackups` | bool | true | Create backups before modifying |
| `BackupDir` | string | .helix/backups | Backup directory |
| `PreservePerms` | bool | true | Preserve original permissions |
| `DefaultPerms` | os.FileMode | 0644 | Default file permissions |
| `AtomicWrites` | bool | true | Use atomic write operations |
| `SyncAfterWrite` | bool | false | Fsync after writing |

### SearchConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `MaxResults` | int | 1000 | Maximum search results |
| `MaxDepth` | int | 100 | Maximum directory depth |
| `IncludeHidden` | bool | false | Include hidden files |
| `FollowSymlinks` | bool | false | Follow symbolic links |
| `IgnorePatterns` | []string | [] | Patterns to ignore |
| `FileTypes` | []string | [] | File extensions to include |
| `CaseSensitive` | bool | true | Case-sensitive matching |

## Security Considerations

1. **Workspace Boundaries**: All operations are restricted to the configured workspace root. Paths outside the workspace are rejected unless explicitly allowed.

2. **Path Traversal Prevention**: The package validates all paths to prevent directory traversal attacks using `..` or symlinks.

3. **Symlink Handling**: Symlinks are not followed by default. When enabled, they're validated to ensure they don't escape the workspace.

4. **Denied Paths**: Sensitive paths can be explicitly denied:
   ```go
   config := &filesystem.FileSystemConfig{
       DeniedPaths: []string{
           "**/.env*",
           "**/credentials*",
           "**/*.key",
           "**/.ssh/**",
       },
   }
   ```

5. **File Size Limits**: Prevents memory exhaustion by limiting maximum file sizes.

6. **Permission Preservation**: Original file permissions are preserved by default when modifying files.

7. **Atomic Writes**: Prevents partial writes and data corruption by using temp files with atomic rename.

8. **Backup Creation**: Automatic backup creation before destructive operations allows recovery.

## Error Types

```go
var (
    ErrPathTraversal     = errors.New("path traversal detected")
    ErrOutsideWorkspace  = errors.New("path outside workspace")
    ErrPathDenied        = errors.New("path explicitly denied")
    ErrFileTooLarge      = errors.New("file exceeds size limit")
    ErrFileNotFound      = errors.New("file not found")
    ErrNotAFile          = errors.New("path is not a file")
    ErrNotADirectory     = errors.New("path is not a directory")
    ErrPermissionDenied  = errors.New("permission denied")
    ErrInvalidEncoding   = errors.New("invalid file encoding")
    ErrBackupFailed      = errors.New("failed to create backup")
    ErrSymlinkLoop       = errors.New("symlink loop detected")
)
```

## Best Practices

1. **Always set a workspace root** to contain operations within a safe boundary.

2. **Enable backups** for production systems to allow recovery from mistakes.

3. **Use atomic writes** for critical files to prevent corruption.

4. **Set appropriate size limits** based on your memory constraints.

5. **Deny sensitive paths** explicitly rather than relying on workspace boundaries alone.

6. **Handle errors appropriately** - check for specific error types to provide meaningful feedback.
