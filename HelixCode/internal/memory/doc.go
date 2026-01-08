// Package memory provides long-term memory and conversation management for the HelixCode platform.
//
// The memory package handles conversation persistence, message history, and integration with
// multiple memory providers for AI agents. It supports both local conversation management
// and external vector database integration for semantic search and retrieval.
//
// # Core Features
//
//   - Conversation Management: Create, store, and retrieve conversation histories
//   - Message Handling: Track messages with roles, timestamps, and metadata
//   - Token Tracking: Estimate and manage token counts for LLM context limits
//   - Multi-Provider Support: Integrate with Mem0, Zep, Memonto, Pinecone, and more
//   - Vector Storage: Store and search embeddings for semantic retrieval
//
// # Message Roles
//
// Messages support standard conversation roles:
//
//   - RoleUser: Messages from the user
//   - RoleAssistant: AI assistant responses
//   - RoleSystem: System instructions and context
//   - RoleTool: Tool/function execution results
//
// # Basic Conversation Usage
//
// Creating and managing conversations:
//
//	manager := memory.NewManager()
//
//	// Create a conversation
//	conv, err := manager.CreateConversation("Project Planning")
//
//	// Add messages
//	userMsg := memory.NewUserMessage("Help me plan the project")
//	manager.AddMessage(conv.ID, userMsg)
//
//	assistantMsg := memory.NewAssistantMessage("I'll help you create a plan...")
//	manager.AddMessage(conv.ID, assistantMsg)
//
//	// Retrieve conversation
//	conv, err := manager.GetConversation(convID)
//	messages := conv.GetMessages()
//
// # Active Conversation
//
// Set and use the active conversation for convenience:
//
//	manager.SetActive(conv.ID)
//
//	// Add to active conversation
//	manager.AddMessageToActive(memory.NewUserMessage("Continue please"))
//
//	// Get active conversation
//	active := manager.GetActive()
//
// # Message Creation
//
// Create messages with the appropriate role:
//
//	// User message
//	userMsg := memory.NewUserMessage("What's the weather?")
//
//	// Assistant message
//	assistantMsg := memory.NewAssistantMessage("The weather is sunny.")
//
//	// System message
//	systemMsg := memory.NewSystemMessage("You are a helpful assistant.")
//
//	// Generic message with any role
//	msg := memory.NewMessage(memory.RoleTool, "Function result: success")
//
// # Conversation Operations
//
// Search and manage conversations:
//
//	// Search conversations
//	results := manager.Search("project planning")
//
//	// Search messages across all conversations
//	messages := manager.SearchMessages("weather")
//
//	// Get recent conversations
//	recent := manager.GetRecent(10)
//
//	// Clear conversation messages
//	manager.ClearConversation(convID)
//
//	// Delete conversation
//	manager.DeleteConversation(convID)
//
// # Token and Message Limits
//
// Configure limits to manage memory usage:
//
//	manager.SetMaxMessages(1000)      // Max messages per conversation
//	manager.SetMaxTokens(100000)      // Max tokens per conversation
//	manager.SetMaxConversations(100)  // Max conversations to keep
//
//	// Trim old conversations
//	removed := manager.TrimConversations()
//
// # Conversation Statistics
//
// Get statistics about conversations:
//
//	stats := manager.GetStatistics()
//	fmt.Printf("Conversations: %d\n", stats.TotalConversations)
//	fmt.Printf("Messages: %d\n", stats.TotalMessages)
//	fmt.Printf("Tokens: %d\n", stats.TotalTokens)
//
//	// Per-conversation statistics
//	convStats := conv.GetStatistics()
//	fmt.Printf("By role: %v\n", convStats.ByRole)
//
// # Export and Import
//
// Export conversations for backup or transfer:
//
//	// Export
//	snapshot, err := manager.Export(convID)
//
//	// Import
//	err := manager.Import(snapshot)
//
// # Version Conflict Resolution
//
// Handle concurrent updates with version control:
//
//	// Update with version check
//	resolution, err := manager.UpdateConversationWithVersion(id, updated, expectedVersion)
//	if !resolution.Resolved {
//	    // Handle conflict
//	    resolution, _ = manager.ResolveConflict(id, incoming, "merge")
//	}
//
// # Memory Callbacks
//
// Register callbacks for lifecycle events:
//
//	manager.OnCreate(func(conv *memory.Conversation) {
//	    log.Printf("Created: %s", conv.Title)
//	})
//
//	manager.OnMessage(func(conv *memory.Conversation, msg *memory.Message) {
//	    log.Printf("Message added to %s", conv.Title)
//	})
//
//	manager.OnDelete(func(conv *memory.Conversation) {
//	    log.Printf("Deleted: %s", conv.Title)
//	})
//
// # Vector Provider Interface
//
// For advanced memory systems, implement the VectorProvider interface:
//
//	type VectorProvider interface {
//	    Store(ctx context.Context, vectors []*VectorData) error
//	    Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error)
//	    FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error)
//	    CreateCollection(ctx context.Context, name string, config *CollectionConfig) error
//	    // ... additional methods
//	}
//
// # Specialized Memory Types
//
// The package includes types for advanced AI memory systems:
//
//   - Character/Avatar: AI personas with personality traits
//   - Crew/Agent/Task: Multi-agent orchestration (CrewAI)
//   - MemoryBlock/WorkingMemory: Hierarchical memory (MemGPT)
//   - Embedding: Vector representations for semantic search
//
// # Provider Types
//
// Supported memory provider types:
//
//   - Vector DBs: Pinecone, Weaviate, Qdrant, ChromaDB, FAISS, Milvus
//   - Memory Systems: Mem0, Zep, Memonto, MemGPT
//   - AI Platforms: CrewAI, Character.AI, Anima
//
// # Thread Safety
//
// The Manager is thread-safe for concurrent access. All operations use
// appropriate locking to prevent race conditions.
package memory
