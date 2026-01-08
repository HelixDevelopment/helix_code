# Persistence Package

The `persistence` package provides file-based state management for the HelixCode platform. It implements a robust persistence system that saves and loads application state including sessions, conversations, and focus chains with support for multiple serialization formats, automatic saving, backup/restore functionality, and atomic write operations.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Types Reference](#types-reference)
- [Storage Formats](#storage-formats)
- [API Reference](#api-reference)
- [Usage Examples](#usage-examples)
- [Configuration](#configuration)
- [Error Handling](#error-handling)
- [Performance Considerations](#performance-considerations)
- [Best Practices](#best-practices)
- [Integration with Other Packages](#integration-with-other-packages)
- [Testing](#testing)

## Overview

The persistence package addresses several key requirements for the HelixCode platform:

- **State Preservation**: Save application state across restarts
- **Data Integrity**: Atomic writes prevent corruption during saves
- **Flexibility**: Multiple serialization formats (JSON, compressed JSON)
- **Automation**: Background auto-save functionality
- **Recovery**: Backup and restore capabilities
- **Observability**: Callbacks for monitoring save/load operations

### Key Capabilities

| Feature | Description |
|---------|-------------|
| Multi-format serialization | JSON, compact JSON, gzip-compressed JSON |
| Atomic file writes | Write-to-temp then rename pattern |
| Auto-save | Configurable background periodic saves |
| Backup/Restore | Full data directory backup and restoration |
| Event callbacks | Hooks for save, load, and error events |
| Thread safety | Read-write mutex for concurrent access |
| Graceful degradation | Individual failures don't abort operations |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              PERSISTENCE LAYER                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                              Store                                   │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │    │
│  │  │ SessionMgr  │  │ MemoryMgr   │  │  FocusMgr   │  │ TemplateMgr│  │    │
│  │  │  Reference  │  │  Reference  │  │  Reference  │  │  Reference │  │    │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └─────┬──────┘  │    │
│  │         │                │                │               │         │    │
│  │         ▼                ▼                ▼               ▼         │    │
│  │  ┌─────────────────────────────────────────────────────────────┐   │    │
│  │  │                       Serializer                             │   │    │
│  │  │  ┌───────────────┐  ┌───────────────┐  ┌─────────────────┐  │   │    │
│  │  │  │ JSONSerializer│  │CompactJSON    │  │ JSONGzipSerializer│  │   │    │
│  │  │  │  (.json)      │  │ Serializer    │  │   (.json.gz)    │  │   │    │
│  │  │  └───────────────┘  └───────────────┘  └─────────────────┘  │   │    │
│  │  └─────────────────────────────────────────────────────────────┘   │    │
│  │                                 │                                   │    │
│  │                                 ▼                                   │    │
│  │  ┌─────────────────────────────────────────────────────────────┐   │    │
│  │  │                    Atomic File Writer                        │   │    │
│  │  │            (write to .tmp → rename to final)                 │   │    │
│  │  └─────────────────────────────────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         File System                                  │    │
│  │                                                                      │    │
│  │    {basePath}/                                                       │    │
│  │    ├── sessions/                                                     │    │
│  │    │   ├── session-abc123.json                                      │    │
│  │    │   └── session-def456.json                                      │    │
│  │    ├── conversations/                                                │    │
│  │    │   ├── conv-ghi789.json                                         │    │
│  │    │   └── conv-jkl012.json                                         │    │
│  │    └── focus/                                                        │    │
│  │        ├── chain-mno345.json                                        │    │
│  │        └── chain-pqr678.json                                        │    │
│  │                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                            CALLBACK SYSTEM                                   │
│                                                                              │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                   │
│  │ SaveCallback│     │LoadCallback │     │ErrorCallback│                   │
│  │ (metadata)  │     │ (metadata)  │     │   (error)   │                   │
│  └─────────────┘     └─────────────┘     └─────────────┘                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Manager     │────▶│   Export()   │────▶│   Snapshot   │
│  (session/   │     │              │     │   Struct     │
│   memory/    │     └──────────────┘     └──────┬───────┘
│   focus)     │                                 │
└──────────────┘                                 ▼
                                          ┌──────────────┐
                                          │ Serialize()  │
                                          │              │
                                          └──────┬───────┘
                                                 │
                                                 ▼
                                          ┌──────────────┐
                                          │ writeAtomic()│
                                          │ (.tmp→final) │
                                          └──────┬───────┘
                                                 │
                                                 ▼
                                          ┌──────────────┐
                                          │  File System │
                                          │  (.json/.gz) │
                                          └──────────────┘

SAVE FLOW                                 LOAD FLOW (reverse)

┌──────────────┐                          ┌──────────────┐
│  File System │◀─────────────────────────│  File System │
│              │                          │              │
└──────────────┘                          └──────┬───────┘
                                                 │
                                                 ▼
                                          ┌──────────────┐
                                          │ ReadFile()   │
                                          │              │
                                          └──────┬───────┘
                                                 │
                                                 ▼
                                          ┌──────────────┐
                                          │ Deserialize()│
                                          │              │
                                          └──────┬───────┘
                                                 │
                                                 ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Manager     │◀────│   Import()   │◀────│   Snapshot   │
│              │     │              │     │   Struct     │
└──────────────┘     └──────────────┘     └──────────────┘
```

## Types Reference

### Core Types

#### Store

The central persistence manager that coordinates all save/load operations.

```go
type Store struct {
    basePath         string            // Base directory for storage
    autoSaveEnabled  bool              // Whether auto-save is enabled
    autoSaveInterval time.Duration     // Auto-save interval
    serializer       Serializer        // Serialization strategy
    sessionMgr       *session.Manager  // Session manager reference
    memoryMgr        *memory.Manager   // Memory manager reference
    focusMgr         *focus.Manager    // Focus manager reference
    templateMgr      *template.Manager // Template manager reference
    mu               sync.RWMutex      // Thread-safety
    stopAutoSave     chan struct{}     // Signal to stop auto-save
    lastSaveTime     time.Time         // Last save timestamp
    onSave           []SaveCallback    // Callbacks on save
    onLoad           []LoadCallback    // Callbacks on load
    onError          []ErrorCallback   // Callbacks on error
}
```

#### Serializer Interface

Defines the contract for serialization strategies.

```go
type Serializer interface {
    Serialize(v interface{}) ([]byte, error)
    Deserialize(data []byte, v interface{}) error
    Format() Format
    Extension() string
}
```

#### Format

Enumeration of supported serialization formats.

```go
type Format string

const (
    FormatJSON     Format = "json"    // Standard JSON format
    FormatJSONGzip Format = "json.gz" // Gzip-compressed JSON
    FormatBinary   Format = "bin"     // Binary format (future)
)
```

### Callback Types

```go
// SaveCallback is called after successful save operations
type SaveCallback func(*SaveMetadata)

// LoadCallback is called after successful load operations
type LoadCallback func(*LoadMetadata)

// ErrorCallback is called when errors occur (especially in auto-save)
type ErrorCallback func(error)
```

### Metadata Types

#### SaveMetadata

Information provided to save callbacks.

```go
type SaveMetadata struct {
    Path      string    // Save path (base directory)
    Format    Format    // Serialization format used
    Size      int64     // Total size in bytes saved
    Timestamp time.Time // Time of save operation
    Items     int       // Number of items saved
}
```

#### LoadMetadata

Information provided to load callbacks.

```go
type LoadMetadata struct {
    Path      string    // Load path (base directory)
    Format    Format    // Serialization format detected
    Size      int64     // Total size in bytes loaded
    Timestamp time.Time // Time of load operation
    Items     int       // Number of items loaded
}
```

### Serializer Implementations

#### JSONSerializer

Standard JSON serialization with optional indentation.

```go
type JSONSerializer struct {
    indent bool  // Whether to indent output
}
```

#### JSONGzipSerializer

Gzip-compressed JSON for reduced storage size.

```go
type JSONGzipSerializer struct {
    indent bool  // Whether to indent before compression
}
```

## Storage Formats

### JSON Format (Default)

- **Extension**: `.json`
- **Characteristics**: Human-readable, easy debugging
- **Use case**: Development, small datasets
- **Example**:

```json
{
  "session": {
    "id": "sess-abc123",
    "project_id": "project1",
    "name": "Development Session",
    "mode": "planning"
  },
  "focus_chain": null
}
```

### Compact JSON Format

- **Extension**: `.json`
- **Characteristics**: No whitespace, smaller files
- **Use case**: Production with moderate data size
- **Example**:

```json
{"session":{"id":"sess-abc123","project_id":"project1","name":"Development Session","mode":"planning"},"focus_chain":null}
```

### Gzip-Compressed JSON Format

- **Extension**: `.json.gz`
- **Characteristics**: Binary, smallest files
- **Use case**: Large datasets, bandwidth-constrained environments
- **Compression ratio**: Typically 60-80% reduction

### Format Detection

The package can automatically detect file formats:

```go
format, err := persistence.DetectFormat(data)
// Checks for gzip magic bytes (0x1f, 0x8b)
// Falls back to JSON validation
// Defaults to binary for unknown formats
```

## API Reference

### Constructor

#### NewStore

Creates a new persistence store with the specified base path.

```go
func NewStore(basePath string) (*Store, error)
```

**Parameters:**
- `basePath`: Root directory for all persisted data

**Returns:**
- `*Store`: The configured store instance
- `error`: If directory creation fails

**Behavior:**
- Creates the base directory if it doesn't exist
- Initializes with JSON serializer (default)
- Auto-save disabled by default

### Manager Configuration

#### SetSessionManager

```go
func (s *Store) SetSessionManager(mgr *session.Manager)
```

Associates a session manager for persistence. Sessions will be saved to `{basePath}/sessions/`.

#### SetMemoryManager

```go
func (s *Store) SetMemoryManager(mgr *memory.Manager)
```

Associates a memory manager for persistence. Conversations will be saved to `{basePath}/conversations/`.

#### SetFocusManager

```go
func (s *Store) SetFocusManager(mgr *focus.Manager)
```

Associates a focus manager for persistence. Focus chains will be saved to `{basePath}/focus/`.

#### SetTemplateManager

```go
func (s *Store) SetTemplateManager(mgr *template.Manager)
```

Associates a template manager for persistence.

#### SetSerializer

```go
func (s *Store) SetSerializer(serializer Serializer)
```

Sets the serialization strategy. Available serializers:
- `NewJSONSerializer()` - Indented JSON
- `NewCompactJSONSerializer()` - Compact JSON
- `NewJSONGzipSerializer()` - Compressed JSON

### Save/Load Operations

#### SaveAll

```go
func (s *Store) SaveAll() error
```

Saves all application state (sessions, conversations, focus chains).

**Behavior:**
- Acquires write lock
- Exports and serializes each manager's data
- Uses atomic writes (temp file + rename)
- Triggers save callbacks
- Updates `lastSaveTime`

#### LoadAll

```go
func (s *Store) LoadAll() error
```

Loads all application state from disk.

**Behavior:**
- Acquires write lock
- Reads and deserializes files from each subdirectory
- Imports data into respective managers
- Triggers load callbacks
- Silently skips missing directories

#### Save (Backward Compatibility)

```go
func (s *Store) Save() error
```

Alias for `SaveAll()`.

#### Load (Backward Compatibility)

```go
func (s *Store) Load() error
```

Alias for `LoadAll()`.

### Auto-Save

#### EnableAutoSave

```go
func (s *Store) EnableAutoSave(interval time.Duration)
```

Enables automatic periodic saving.

**Parameters:**
- `interval`: Time between save operations

**Behavior:**
- Starts a background goroutine
- Saves at specified intervals
- Triggers error callbacks on failures
- No-op if already enabled

#### DisableAutoSave

```go
func (s *Store) DisableAutoSave()
```

Stops automatic saving.

**Behavior:**
- Signals the auto-save goroutine to stop
- No-op if not enabled

### Backup/Restore

#### Backup

```go
func (s *Store) Backup(backupPath string) error
```

Creates a complete backup of all persisted data.

**Parameters:**
- `backupPath`: Destination directory for backup

**Behavior:**
- Creates backup directory
- Recursively copies all subdirectories
- Preserves directory structure

#### Restore

```go
func (s *Store) Restore(backupPath string) error
```

Restores data from a backup.

**Parameters:**
- `backupPath`: Source directory containing backup

**Behavior:**
- Removes existing data directories
- Copies from backup location
- Does NOT reload into managers (call `LoadAll()` after)

### Maintenance

#### Clear

```go
func (s *Store) Clear() error
```

Removes all persisted data.

**Behavior:**
- Deletes sessions, conversations, and focus directories
- Does NOT clear manager state (only disk storage)

#### GetLastSaveTime

```go
func (s *Store) GetLastSaveTime() time.Time
```

Returns the timestamp of the last successful save.

### Callbacks

#### OnSave

```go
func (s *Store) OnSave(callback SaveCallback)
```

Registers a callback for save events.

#### OnLoad

```go
func (s *Store) OnLoad(callback LoadCallback)
```

Registers a callback for load events.

#### OnError

```go
func (s *Store) OnError(callback ErrorCallback)
```

Registers a callback for error events (primarily auto-save errors).

### Utility Functions

#### Validate

```go
func Validate(data []byte, format Format) error
```

Validates that data matches the expected format.

#### DetectFormat

```go
func DetectFormat(data []byte) (Format, error)
```

Attempts to detect the format of data bytes.

## Usage Examples

### Basic Setup

```go
import (
    "time"
    "dev.helix.code/internal/persistence"
    "dev.helix.code/internal/session"
    "dev.helix.code/internal/memory"
    "dev.helix.code/internal/focus"
)

func main() {
    // Create store
    store, err := persistence.NewStore("/var/lib/helixcode/data")
    if err != nil {
        log.Fatal(err)
    }

    // Create managers
    sessionMgr := session.NewManager()
    memoryMgr := memory.NewManager()
    focusMgr := focus.NewManager()

    // Connect managers to store
    store.SetSessionManager(sessionMgr)
    store.SetMemoryManager(memoryMgr)
    store.SetFocusManager(focusMgr)

    // Load existing data
    if err := store.LoadAll(); err != nil {
        log.Printf("Warning: failed to load data: %v", err)
    }

    // Application runs...

    // Save before exit
    if err := store.SaveAll(); err != nil {
        log.Printf("Error saving data: %v", err)
    }
}
```

### Using Auto-Save

```go
func setupAutoSave(store *persistence.Store) {
    // Enable auto-save every 5 minutes
    store.EnableAutoSave(5 * time.Minute)

    // Register error handler
    store.OnError(func(err error) {
        log.Printf("Auto-save failed: %v", err)
        // Could trigger alerts, retry logic, etc.
    })
}

func shutdown(store *persistence.Store) {
    // Disable auto-save before final save
    store.DisableAutoSave()

    // Final save
    if err := store.SaveAll(); err != nil {
        log.Printf("Final save failed: %v", err)
    }
}
```

### Using Compressed Storage

```go
func setupCompressedStorage() (*persistence.Store, error) {
    store, err := persistence.NewStore("/var/lib/helixcode/data")
    if err != nil {
        return nil, err
    }

    // Use gzip compression for smaller files
    store.SetSerializer(persistence.NewJSONGzipSerializer())

    return store, nil
}
```

### Implementing Backup Strategy

```go
func dailyBackup(store *persistence.Store) error {
    // Create timestamped backup directory
    backupDir := fmt.Sprintf("/var/backups/helixcode/%s",
        time.Now().Format("2006-01-02"))

    // Perform backup
    if err := store.Backup(backupDir); err != nil {
        return fmt.Errorf("backup failed: %w", err)
    }

    log.Printf("Backup completed to %s", backupDir)
    return nil
}

func restoreFromBackup(store *persistence.Store, backupDate string) error {
    backupDir := fmt.Sprintf("/var/backups/helixcode/%s", backupDate)

    // Restore files
    if err := store.Restore(backupDir); err != nil {
        return fmt.Errorf("restore failed: %w", err)
    }

    // Reload into managers
    if err := store.LoadAll(); err != nil {
        return fmt.Errorf("load after restore failed: %w", err)
    }

    return nil
}
```

### Monitoring with Callbacks

```go
func setupMonitoring(store *persistence.Store) {
    store.OnSave(func(meta *persistence.SaveMetadata) {
        log.Printf("Saved %d items (%d bytes) in %s format",
            meta.Items, meta.Size, meta.Format)

        // Emit metrics
        metrics.SaveOperations.Inc()
        metrics.SavedBytes.Add(float64(meta.Size))
        metrics.SavedItems.Add(float64(meta.Items))
    })

    store.OnLoad(func(meta *persistence.LoadMetadata) {
        log.Printf("Loaded %d items (%d bytes)",
            meta.Items, meta.Size)

        metrics.LoadOperations.Inc()
    })

    store.OnError(func(err error) {
        log.Printf("Persistence error: %v", err)
        metrics.PersistenceErrors.Inc()

        // Alert on critical errors
        if isCI {
            alerting.SendAlert("Persistence failure", err.Error())
        }
    })
}
```

### Format Detection and Migration

```go
func migrateToCompressedFormat(dataDir string) error {
    // Read existing files
    entries, err := os.ReadDir(filepath.Join(dataDir, "sessions"))
    if err != nil {
        return err
    }

    gzipSerializer := persistence.NewJSONGzipSerializer()
    jsonSerializer := persistence.NewJSONSerializer()

    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }

        path := filepath.Join(dataDir, "sessions", entry.Name())
        data, err := os.ReadFile(path)
        if err != nil {
            continue
        }

        // Detect current format
        format, err := persistence.DetectFormat(data)
        if err != nil || format == persistence.FormatJSONGzip {
            continue // Already compressed or unknown
        }

        // Deserialize
        var snapshot session.SessionSnapshot
        if err := jsonSerializer.Deserialize(data, &snapshot); err != nil {
            continue
        }

        // Re-serialize as gzip
        compressed, err := gzipSerializer.Serialize(snapshot)
        if err != nil {
            continue
        }

        // Write new file
        newPath := strings.TrimSuffix(path, ".json") + ".json.gz"
        if err := os.WriteFile(newPath, compressed, 0644); err != nil {
            continue
        }

        // Remove old file
        os.Remove(path)

        log.Printf("Migrated %s -> %s (%.1f%% reduction)",
            entry.Name(),
            filepath.Base(newPath),
            100*(1-float64(len(compressed))/float64(len(data))))
    }

    return nil
}
```

## Configuration

### YAML Configuration

```yaml
persistence:
  # Base directory for all persistence data
  path: "/var/lib/helixcode/data"

  # Serialization format: "json", "compact", "gzip"
  format: "json"

  # Auto-save settings
  auto_save:
    enabled: true
    interval: 5m

  # Backup settings
  backup:
    enabled: true
    path: "/var/backups/helixcode"
    retention_days: 30
```

### Environment Variables

```bash
# Override persistence path
HELIX_PERSISTENCE_PATH=/custom/data/path

# Override auto-save interval
HELIX_PERSISTENCE_AUTOSAVE_INTERVAL=10m

# Disable auto-save
HELIX_PERSISTENCE_AUTOSAVE_ENABLED=false
```

### Programmatic Configuration

```go
func configurePersistence(cfg *config.Config) (*persistence.Store, error) {
    store, err := persistence.NewStore(cfg.Persistence.Path)
    if err != nil {
        return nil, err
    }

    // Configure serializer
    switch cfg.Persistence.Format {
    case "gzip":
        store.SetSerializer(persistence.NewJSONGzipSerializer())
    case "compact":
        store.SetSerializer(persistence.NewCompactJSONSerializer())
    default:
        store.SetSerializer(persistence.NewJSONSerializer())
    }

    // Configure auto-save
    if cfg.Persistence.AutoSave.Enabled {
        store.EnableAutoSave(cfg.Persistence.AutoSave.Interval)
    }

    return store, nil
}
```

## Error Handling

### Error Types and Patterns

```go
// Directory creation failure
store, err := persistence.NewStore("/invalid/path")
if err != nil {
    // "failed to create base path: permission denied"
}

// Save errors wrap underlying issues
err := store.SaveAll()
if err != nil {
    // "failed to save sessions: ..."
    // "failed to save conversations: ..."
    // "failed to save focus chains: ..."
}

// Load errors wrap underlying issues
err := store.LoadAll()
if err != nil {
    // "failed to load sessions: ..."
    // etc.
}
```

### Graceful Degradation

The package handles individual item failures gracefully:

```go
// In saveSessions(), individual session failures are logged but don't
// abort the entire operation
for _, sess := range sessions {
    snapshot, err := s.sessionMgr.Export(sess.ID)
    if err != nil {
        continue  // Skip this session, continue with others
    }

    data, err := s.serializer.Serialize(snapshot)
    if err != nil {
        continue  // Skip serialization failures
    }

    if err := writeAtomic(filename, data); err != nil {
        continue  // Skip write failures
    }
}
```

### Error Callback Pattern

```go
store.OnError(func(err error) {
    // Log the error
    log.Printf("Persistence error: %v", err)

    // Categorize and handle
    switch {
    case os.IsPermission(err):
        // Permission issues - may need admin intervention
        alerting.Critical("Persistence permission denied")
    case os.IsNotExist(err):
        // Missing directories - attempt recreation
        store.ensureDirectories()
    default:
        // Unknown error - log and continue
        alerting.Warn("Persistence error: " + err.Error())
    }
})
```

## Performance Considerations

### File I/O Optimization

1. **Atomic Writes**: Uses write-to-temp-then-rename pattern
   - Prevents partial writes on crashes
   - Slight overhead but ensures data integrity

2. **Directory Structure**: Separate directories per data type
   - Enables parallel I/O operations
   - Reduces file listing overhead

3. **Gzip Compression**: For large datasets
   - Reduces disk I/O at cost of CPU
   - 60-80% size reduction typical

### Concurrency

```go
// Multiple concurrent reads are allowed
store.OnLoad(...)  // Uses RLock
store.GetLastSaveTime()  // Uses RLock

// Writes are exclusive
store.SaveAll()  // Uses Lock
store.LoadAll()  // Uses Lock
store.Clear()    // Uses Lock
```

### Memory Usage

- Snapshots are created during export (temporary memory spike)
- Serialization buffers are allocated per operation
- Consider chunked processing for very large datasets

### Recommendations

| Scenario | Recommendation |
|----------|----------------|
| < 100 sessions | JSON format, 5-minute auto-save |
| 100-1000 sessions | Compact JSON, 5-minute auto-save |
| > 1000 sessions | Gzip format, 10-minute auto-save |
| High availability | Shorter auto-save interval (1-2 minutes) |
| Limited disk space | Gzip format |
| Debugging needed | JSON format (human-readable) |

## Best Practices

### 1. Always Handle Startup Failures

```go
store, err := persistence.NewStore(dataPath)
if err != nil {
    // Fall back to default path or fail gracefully
    log.Printf("Using fallback data path: %v", err)
    store, err = persistence.NewStore("/tmp/helixcode")
}
```

### 2. Save Before Shutdown

```go
func gracefulShutdown(store *persistence.Store) {
    // Stop auto-save first
    store.DisableAutoSave()

    // Final save with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    done := make(chan error, 1)
    go func() {
        done <- store.SaveAll()
    }()

    select {
    case err := <-done:
        if err != nil {
            log.Printf("Final save failed: %v", err)
        }
    case <-ctx.Done():
        log.Printf("Save timed out during shutdown")
    }
}
```

### 3. Implement Backup Rotation

```go
func rotateBackups(backupPath string, retentionDays int) error {
    cutoff := time.Now().AddDate(0, 0, -retentionDays)

    entries, _ := os.ReadDir(backupPath)
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }

        // Parse date from directory name
        date, err := time.Parse("2006-01-02", entry.Name())
        if err != nil {
            continue
        }

        if date.Before(cutoff) {
            os.RemoveAll(filepath.Join(backupPath, entry.Name()))
        }
    }
    return nil
}
```

### 4. Monitor Persistence Health

```go
func healthCheck(store *persistence.Store) error {
    // Check last save time
    lastSave := store.GetLastSaveTime()
    if time.Since(lastSave) > 15*time.Minute {
        return fmt.Errorf("no save in %v", time.Since(lastSave))
    }

    // Verify write capability
    testFile := filepath.Join(store.basePath, ".healthcheck")
    if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
        return fmt.Errorf("write test failed: %w", err)
    }
    os.Remove(testFile)

    return nil
}
```

### 5. Use Appropriate Serialization

```go
// Development: Use readable JSON
if env == "development" {
    store.SetSerializer(persistence.NewJSONSerializer())
}

// Production: Use compression
if env == "production" {
    store.SetSerializer(persistence.NewJSONGzipSerializer())
}
```

## Integration with Other Packages

### Session Package

The persistence package integrates with `internal/session` through the `SessionSnapshot` type:

```go
// session.SessionSnapshot contains:
type SessionSnapshot struct {
    Session    *Session             `json:"session"`
    FocusChain *focus.ChainSnapshot `json:"focus_chain,omitempty"`
}

// Integration via manager methods:
sessionMgr.Export(sessionID) -> *SessionSnapshot
sessionMgr.Import(snapshot)  -> error
```

### Memory Package

Integration with `internal/memory` for conversation persistence:

```go
// memory.ConversationSnapshot contains:
type ConversationSnapshot struct {
    Conversation *Conversation `json:"conversation"`
    ExportedAt   time.Time     `json:"exported_at"`
}

// Integration via manager methods:
memoryMgr.Export(convID) -> *ConversationSnapshot
memoryMgr.Import(snapshot) -> error
```

### Focus Package

Integration with `internal/focus` for focus chain persistence:

```go
// focus.ChainSnapshot contains:
type ChainSnapshot struct {
    Chain      *Chain
    ExportedAt time.Time
}

// Integration via manager methods:
focusMgr.ExportChain(chainID) -> *ChainSnapshot
focusMgr.ImportChain(snapshot, setActive) -> error
```

### Template Package

Integration with `internal/template` for template persistence:

```go
store.SetTemplateManager(templateMgr)
```

### Dependency Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         persistence                              │
│                            Store                                 │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                │ depends on
                                ▼
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│    session    │     │    memory     │     │    focus      │
│    Manager    │     │    Manager    │     │    Manager    │
│               │     │               │     │               │
│  - Export()   │     │  - Export()   │     │  - ExportChain│
│  - Import()   │     │  - Import()   │     │  - ImportChain│
│  - GetAll()   │     │  - GetAll()   │     │  - GetAllChain│
└───────────────┘     └───────────────┘     └───────────────┘
```

## Testing

### Running Tests

```bash
# Run all persistence tests
go test -v ./internal/persistence/...

# Run with coverage
go test -cover ./internal/persistence/...

# Run specific test
go test -v ./internal/persistence -run TestStore

# Run with race detection
go test -race ./internal/persistence/...
```

### Test Categories

| Test Suite | Description |
|------------|-------------|
| `TestStore` | Store creation and configuration |
| `TestSaveSessions` | Session save/load |
| `TestSaveConversations` | Conversation save/load |
| `TestSaveFocus` | Focus chain save/load |
| `TestAutoSave` | Auto-save functionality |
| `TestBackupRestore` | Backup and restore |
| `TestClear` | Data clearing |
| `TestCallbacks` | Event callbacks |
| `TestSerializers` | Serialization formats |
| `TestFormatValidation` | Format detection/validation |
| `TestConcurrency` | Thread safety |
| `TestEdgeCases` | Edge cases and error handling |
| `TestIntegration` | Full workflow tests |

### Example Test

```go
func TestCustomScenario(t *testing.T) {
    tmpDir := t.TempDir()
    store, err := persistence.NewStore(tmpDir)
    require.NoError(t, err)

    // Setup managers
    sessionMgr := session.NewManager()
    store.SetSessionManager(sessionMgr)

    // Create test data
    sess, _ := sessionMgr.Create("project1", "Test", "Description", session.ModePlanning)

    // Test save/load cycle
    require.NoError(t, store.SaveAll())

    // Verify file exists
    sessionFile := filepath.Join(tmpDir, "sessions", sess.ID+".json")
    _, err = os.Stat(sessionFile)
    assert.NoError(t, err)

    // Load into fresh manager
    newMgr := session.NewManager()
    store.SetSessionManager(newMgr)
    require.NoError(t, store.LoadAll())

    // Verify loaded data
    loaded, err := newMgr.Get(sess.ID)
    assert.NoError(t, err)
    assert.Equal(t, "project1", loaded.ProjectID)
}
```
