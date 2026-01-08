// Package mocks provides mock implementations of internal interfaces for testing.
//
// The mocks package contains mock implementations of key HelixCode interfaces
// including vector providers, memory managers, API key managers, and conversation
// managers. These mocks are built on testify/mock and provide both recording
// of method calls and configurable return values for comprehensive testing.
//
// # Key Mock Types
//
// MockVectorProvider implements the VectorProvider interface:
//
//	provider := mocks.NewMockVectorProvider(t)
//
//	// Configure expectations
//	provider.On("Store", mock.Anything, mock.Anything).Return(nil)
//	provider.On("Search", mock.Anything, mock.Anything).Return(searchResult, nil)
//
//	// Use in tests
//	err := provider.Store(ctx, vectors)
//
//	// Verify expectations
//	provider.AssertExpectations(t)
//
// # MockVectorProvider Operations
//
// The mock vector provider supports all vector operations:
//
//	// Storage operations
//	provider.Store(ctx, vectors)
//	provider.Retrieve(ctx, ids)
//
//	// Search operations
//	provider.Search(ctx, query)
//	provider.FindSimilar(ctx, embedding, k, filters)
//
//	// Collection management
//	provider.CreateCollection(ctx, name, config)
//	provider.DeleteCollection(ctx, name)
//	provider.ListCollections(ctx)
//	provider.GetCollection(ctx, name)
//
//	// Index management
//	provider.CreateIndex(ctx, collection, config)
//	provider.DeleteIndex(ctx, collection, name)
//	provider.ListIndexes(ctx, collection)
//
//	// Metadata operations
//	provider.AddMetadata(ctx, id, metadata)
//	provider.UpdateMetadata(ctx, id, metadata)
//	provider.GetMetadata(ctx, ids)
//	provider.DeleteMetadata(ctx, ids, keys)
//
//	// Lifecycle operations
//	provider.Initialize(ctx, config)
//	provider.Start(ctx)
//	provider.Stop(ctx)
//	provider.Health(ctx)
//
// # Test Data Helpers
//
// The mock provider includes test data helpers:
//
//	// Add test vectors directly
//	provider.AddTestData(vectors)
//
//	// Clear all test data
//	provider.ClearTestData()
//
//	// Get stored vectors for verification
//	vectors := provider.GetStoredVectors("collection")
//
//	// Set health status
//	provider.SetHealth(true) // or false for unhealthy
//
// # Creating Test Data
//
// Helper functions create test data:
//
//	// Create single test vector
//	vector := mocks.CreateTestVector("id-1", "collection", 1536)
//
//	// Create multiple test vectors
//	vectors := mocks.CreateTestVectors(10, "collection", 1536)
//
//	// Create test memory/message
//	message := mocks.CreateTestMemory("id-1", "user", "Hello world")
//
//	// Create conversation message
//	msg := mocks.CreateTestConversationMessage("id-1", "assistant", "Response")
//
// # MockVectorProviderManager
//
// Manages multiple vector providers:
//
//	manager := mocks.NewMockVectorProviderManager(t)
//
//	manager.On("Store", mock.Anything, mock.Anything).Return(nil)
//	manager.On("GetActiveProvider").Return("chroma")
//	manager.On("SetActiveProvider", mock.Anything, "pinecone").Return(nil)
//
// # MockAPIKeyManager
//
// Manages API keys for testing:
//
//	keyManager := mocks.NewMockAPIKeyManager(t)
//
//	keyManager.On("GetAPIKey", "openai").Return("sk-test-key", nil)
//	keyManager.On("SetAPIKey", "openai", mock.Anything).Return(nil)
//	keyManager.On("RotateAPIKey", "openai").Return(nil)
//
// # MockMemoryManager
//
// Memory storage mock:
//
//	memoryManager := mocks.NewMockMemoryManager(t)
//
//	memoryManager.On("Initialize", mock.Anything, mock.Anything).Return(nil)
//	memoryManager.On("Store", mock.Anything, mock.Anything).Return(nil)
//	memoryManager.On("Search", mock.Anything, mock.Anything).Return(result, nil)
//
// # MockConversationManager
//
// Conversation history mock:
//
//	convManager := mocks.NewMockConversationManager(t)
//
//	convManager.On("AddMessage", mock.Anything, "session-1", mock.Anything).Return(nil)
//	convManager.On("GetSummary", mock.Anything, "session-1").Return(summary, nil)
//	convManager.On("GetContextWindow", mock.Anything, "session-1", 10).Return(messages, nil)
//
// # Test Suite
//
// MockSuite provides a complete test setup:
//
//	type MyTestSuite struct {
//	    mocks.MockSuite
//	}
//
//	func (s *MyTestSuite) TestFeature() {
//	    // s.mockProvider is already set up
//	    // s.mockProviderManager is already set up
//	    // s.ctx is ready to use
//
//	    result, err := myFunction(s.ctx, s.mockProvider)
//	    s.NoError(err)
//	}
//
//	func TestMySuite(t *testing.T) {
//	    suite.Run(t, new(MyTestSuite))
//	}
//
// # Creating Mock Providers with Data
//
// Create pre-populated mock providers:
//
//	// Create provider with 100 test vectors
//	provider := mocks.CreateMockVectorProvider(t, 100)
//
//	// Vectors are added to "test_collection" with 1536 dimensions
//	vectors := provider.GetStoredVectors("test_collection")
//
// # Internal State
//
// Mock providers maintain internal state:
//
//	// MockVectorProvider tracks:
//	// - store: map[string][]*VectorData (by collection)
//	// - collections: map[string]*CollectionConfig
//	// - indices: map[string]*IndexInfo
//	// - stats: *ProviderStats
//	// - healthy: bool
//	// - initialized: bool
//	// - started: bool
//
// # Thread Safety
//
// All mock implementations use mutex protection for thread safety,
// allowing concurrent test execution.
//
// # Integration with testify
//
// Mocks are built on testify/mock:
//
//	// Set up expectations
//	provider.On("Method", args...).Return(returnValues...)
//
//	// Match any argument
//	provider.On("Store", mock.Anything, mock.Anything).Return(nil)
//
//	// Match specific argument
//	provider.On("GetCollection", mock.Anything, "my-collection").Return(info, nil)
//
//	// Verify all expectations were met
//	provider.AssertExpectations(t)
//
//	// Check specific call was made
//	provider.AssertCalled(t, "Store", mock.Anything, mock.Anything)
//
// # Example Test
//
//	func TestVectorStorage(t *testing.T) {
//	    // Create mock
//	    provider := mocks.NewMockVectorProvider(t)
//
//	    // Set up expectations
//	    provider.On("Store", mock.Anything, mock.Anything).Return(nil)
//
//	    // Create test data
//	    vectors := mocks.CreateTestVectors(5, "test", 1536)
//
//	    // Execute
//	    ctx := context.Background()
//	    err := provider.Store(ctx, vectors)
//
//	    // Verify
//	    assert.NoError(t, err)
//	    stored := provider.GetStoredVectors("test")
//	    assert.Len(t, stored, 5)
//
//	    provider.AssertExpectations(t)
//	}
package mocks
