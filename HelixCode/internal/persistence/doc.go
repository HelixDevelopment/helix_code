// Package persistence provides file-based state management for HelixCode.
//
// This package implements a persistent storage system that saves and loads
// application state including sessions, conversations, and focus chains.
// It supports multiple serialization formats, automatic saving, and
// backup/restore functionality.
//
// # Architecture
//
// The persistence system is built around several components:
//
//   - Store: Central manager for all persistence operations
//   - Serializer: Interface for data serialization (JSON, compressed JSON)
//   - SaveCallback/LoadCallback: Hooks for save/load events
//   - ErrorCallback: Error handling hooks
//
// # Supported Data Types
//
// The store manages persistence for:
//
//   - Sessions: User session state from the session package
//   - Conversations: Chat history from the memory package
//   - Focus Chains: Context focus state from the focus package
//
// # Basic Usage
//
// Creating and using the persistence store:
//
//	store, err := persistence.NewStore("/path/to/data")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Connect managers
//	store.SetSessionManager(sessionMgr)
//	store.SetMemoryManager(memoryMgr)
//	store.SetFocusManager(focusMgr)
//
//	// Load existing data
//	err = store.LoadAll()
//
//	// Save current state
//	err = store.SaveAll()
//
// # Auto-Save
//
// Enable automatic periodic saving:
//
//	store.EnableAutoSave(5 * time.Minute)
//	defer store.DisableAutoSave()
//
// The auto-save runs in a background goroutine and saves all state
// at the configured interval. Errors during auto-save trigger the
// registered error callbacks.
//
// # Serialization Formats
//
// The package supports multiple serialization strategies:
//
//	// JSON format (default, human-readable)
//	store.SetSerializer(persistence.NewJSONSerializer())
//
//	// Compact JSON (smaller files)
//	store.SetSerializer(persistence.NewCompactJSONSerializer())
//
//	// Compressed JSON (gzip, smallest files)
//	store.SetSerializer(persistence.NewJSONGzipSerializer())
//
// Available formats:
//
//	FormatJSON     - Standard JSON (.json)
//	FormatJSONGzip - Gzip-compressed JSON (.json.gz)
//	FormatBinary   - Reserved for future binary format (.bin)
//
// # Directory Structure
//
// Data is organized in subdirectories:
//
//	{basePath}/
//	├── sessions/       # Session snapshots
//	│   ├── session1.json
//	│   └── session2.json
//	├── conversations/  # Conversation history
//	│   ├── conv1.json
//	│   └── conv2.json
//	└── focus/          # Focus chain state
//	    ├── chain1.json
//	    └── chain2.json
//
// # Callbacks
//
// Register callbacks for persistence events:
//
//	store.OnSave(func(metadata *persistence.SaveMetadata) {
//	    log.Printf("Saved %d items (%d bytes)", metadata.Items, metadata.Size)
//	})
//
//	store.OnLoad(func(metadata *persistence.LoadMetadata) {
//	    log.Printf("Loaded %d items", metadata.Items)
//	})
//
//	store.OnError(func(err error) {
//	    log.Printf("Persistence error: %v", err)
//	})
//
// # Metadata
//
// Save and load operations provide metadata:
//
//	type SaveMetadata struct {
//	    Path      string    // Save directory
//	    Format    Format    // Serialization format
//	    Size      int64     // Total bytes saved
//	    Timestamp time.Time // Save time
//	    Items     int       // Number of items
//	}
//
//	type LoadMetadata struct {
//	    Path      string    // Load directory
//	    Format    Format    // Serialization format
//	    Size      int64     // Total bytes loaded
//	    Timestamp time.Time // Load time
//	    Items     int       // Number of items
//	}
//
// # Backup and Restore
//
// Create backups and restore from them:
//
//	// Create backup
//	err := store.Backup("/path/to/backup")
//
//	// Restore from backup
//	err = store.Restore("/path/to/backup")
//
// Backups are complete copies of the data directory structure.
//
// # Clear Data
//
// Remove all persisted data:
//
//	err := store.Clear()
//
// This removes all subdirectories (sessions, conversations, focus).
//
// # Atomic Writes
//
// File writes use atomic operations (write to temp file, then rename)
// to prevent data corruption during saves. This ensures that even if
// a save is interrupted, the previous data remains intact.
//
// # Format Detection
//
// The package can detect file formats automatically:
//
//	format, err := persistence.DetectFormat(data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	switch format {
//	case persistence.FormatJSON:
//	    // Handle JSON
//	case persistence.FormatJSONGzip:
//	    // Handle compressed
//	}
//
// # Validation
//
// Validate data before deserialization:
//
//	err := persistence.Validate(data, persistence.FormatJSON)
//	if err != nil {
//	    log.Printf("Invalid data: %v", err)
//	}
//
// # Thread Safety
//
// The Store uses read-write mutex for thread-safe operations:
//   - Multiple concurrent reads are allowed
//   - Writes are exclusive
//   - Auto-save runs in a separate goroutine with proper synchronization
//
// # Error Handling
//
// The package handles errors gracefully:
//   - Individual item failures don't abort entire save/load
//   - Errors are logged and callbacks are triggered
//   - Missing directories are created automatically
//   - Missing backup sources are handled silently
package persistence
