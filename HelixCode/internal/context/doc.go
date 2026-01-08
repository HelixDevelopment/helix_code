// Package context provides context building and management for AI conversations
// in HelixCode.
//
// # Overview
//
// The context package implements a comprehensive system for building, managing,
// and storing conversation context across different scopes. It provides a fluent
// API for building conversations and a hierarchical context management system
// with session, project, and global scopes.
//
// # Architecture
//
// The package is organized around two main subsystems:
//
// Context Building:
//   - Builder: Fluent API for constructing AI conversation context
//   - Integration with the memory package for message management
//
// Context Management:
//   - ContextManager: Central manager for context storage and retrieval
//   - SessionContext: Per-session context storage
//   - ProjectContext: Per-project context storage
//   - GlobalContext: Application-wide context storage
//
// # Context Builder
//
// The Builder provides a fluent API for constructing conversations:
//
//	// Create a new builder
//	builder := context.NewBuilder()
//
//	// Configure the conversation
//	builder.SetSystemRole("You are a helpful coding assistant")
//	builder.AddUserMessage("Help me write a function")
//	builder.AddAssistantMessage("Sure! What should the function do?")
//
//	// Add metadata
//	builder.SetMetadata("title", "Code Implementation")
//	builder.SetMetadata("language", "Go")
//
//	// Build the conversation
//	conv := builder.Build()
//
// # Creating Builder from Conversation
//
// Create a builder from an existing conversation:
//
//	builder := context.FromConversation(existingConv)
//	builder.AddUserMessage("Follow-up question")
//	newConv := builder.Build()
//
// # Text Export
//
// Export conversation to plain text:
//
//	text := builder.ToText()
//	// Output:
//	// [system] You are a helpful assistant
//	//
//	// [user] Hello
//	//
//	// [assistant] Hi there!
//
// # Context Manager
//
// The ContextManager provides hierarchical context storage:
//
//	// Create manager with configuration
//	manager := context.NewContextManager(&config.ContextConfig{
//	    Enabled:         true,
//	    MaxSize:         10000,
//	    RetentionPeriod: 24 * time.Hour,
//	})
//
//	// Start the manager (enables cleanup routines)
//	err := manager.Start(ctx)
//
// # Storing Context
//
// Store context items with different scopes:
//
//	// Create a context item
//	item := &context.ContextItem{
//	    ID:       uuid.New().String(),
//	    Type:     context.ContextTypeSession,
//	    Key:      "current_task",
//	    Value:    taskData,
//	    Metadata: map[string]interface{}{"session_id": sessionID},
//	    Priority: 1,
//	}
//
//	// Store the item
//	err := manager.Store(ctx, item)
//
// # Retrieving Context
//
// Retrieve stored context items:
//
//	// Retrieve by ID
//	item, err := manager.Retrieve(ctx, itemID)
//
//	// Search by pattern and type
//	items, err := manager.Search(ctx, "current_*", context.ContextTypeSession)
//
// # Context Types
//
// The package supports four context types:
//
//	const (
//	    ContextTypeFile    // File-based context (attached files)
//	    ContextTypeSession // Session-scoped context
//	    ContextTypeProject // Project-scoped context
//	    ContextTypeGlobal  // Application-wide context
//	)
//
// # Session Context
//
// Work with session-specific context:
//
//	// Get session context
//	sessionCtx, err := manager.GetSessionContext(sessionID)
//
//	// Store in session
//	sessionCtx.Store(item)
//
//	// Retrieve from session
//	item, exists := sessionCtx.Retrieve(itemID)
//
//	// Clear session context
//	sessionCtx.Clear()
//
// # Project Context
//
// Work with project-specific context:
//
//	// Get project context
//	projectCtx, err := manager.GetProjectContext(projectID)
//
//	// Store in project
//	projectCtx.Store(item)
//
//	// Retrieve from project
//	item, exists := projectCtx.Retrieve(itemID)
//
// # Global Context
//
// Work with application-wide context:
//
//	// Get global context
//	globalCtx := manager.GetGlobalContext()
//
//	// Store globally
//	globalCtx.Store(item)
//
// # Context Item TTL
//
// Context items support time-to-live (TTL):
//
//	ttl := 30 * time.Minute
//	item := &context.ContextItem{
//	    ID:    "temp-item",
//	    Type:  context.ContextTypeSession,
//	    Key:   "temp_data",
//	    Value: data,
//	    TTL:   &ttl,
//	}
//
//	// Item will be automatically cleaned up after TTL expires
//
// # Global Manager
//
// Use the global context manager for convenience:
//
//	// Initialize global manager
//	context.InitializeGlobalManager(cfg)
//
//	// Get global manager
//	manager := context.GetGlobalManager()
//
//	// Use convenience functions
//	err := context.StoreGlobal(ctx, item)
//	item, err := context.RetrieveGlobal(ctx, id)
//	items, err := context.SearchGlobal(ctx, pattern, contextType)
//
// # Statistics
//
// Get context manager statistics:
//
//	stats := manager.GetStatistics()
//	// stats["total_items"]: Total items stored
//	// stats["total_sessions"]: Number of active sessions
//	// stats["total_projects"]: Number of active projects
//	// stats["global_items"]: Items in global context
//	// stats["items_by_type"]: Breakdown by context type
//
// # Builder Utilities
//
// Additional builder utilities:
//
//	// Get message count
//	count := builder.MessageCount()
//
//	// Clone builder for variations
//	clone := builder.Clone()
//
//	// Clear builder for reuse
//	builder.Clear()
//
//	// Get metadata
//	value := builder.GetMetadata("key")
//
// # Integration with Memory
//
// The builder integrates with the memory package:
//
//	import (
//	    "dev.helix.code/internal/context"
//	    "dev.helix.code/internal/memory"
//	)
//
//	// Build context
//	builder := context.NewBuilder()
//	builder.SetSystemRole("Expert assistant")
//	builder.AddUserMessage("Question")
//
//	// Create conversation
//	conv := builder.Build()
//
//	// Conv is a *memory.Conversation with all messages and metadata
//
// # Thread Safety
//
// All types in this package are safe for concurrent use:
//
//   - Builder uses sync.RWMutex for thread-safe operations
//   - ContextManager uses sync.RWMutex for all operations
//   - SessionContext and ProjectContext use sync.RWMutex
//   - GlobalContext uses sync.RWMutex
//
// # Cleanup and Lifecycle
//
// The ContextManager handles cleanup automatically:
//
//	// Start manager (starts cleanup goroutine)
//	err := manager.Start(ctx)
//
//	// Cleanup runs every 5 minutes, removing expired items
//
//	// Stop manager gracefully
//	manager.Stop()
//
// # Best Practices
//
//  1. Set system role first when building conversations
//  2. Use metadata to store contextual information
//  3. Clone builders to create variations
//  4. Use appropriate context types for proper scoping
//  5. Set TTL on temporary context items
//  6. Call Stop() on the manager during shutdown
//  7. Use the global manager for simple applications
//
// # Related Packages
//
//   - internal/memory: Message and conversation management
//   - internal/session: Session management
//   - internal/config: Configuration for context settings
package context
