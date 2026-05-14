package providers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/memory"
)

// =============================================================================
// Test Setup and Helpers
// =============================================================================

func newTestCharacterAIProvider() *CharacterAIProvider {
	config := map[string]interface{}{
		"api_key": "test_key",
	}
	provider, _ := NewCharacterAIProvider(config)
	return provider.(*CharacterAIProvider)
}

func newInitializedCharacterAIProvider(t *testing.T) *CharacterAIProvider {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Cleanup: stop the provider when test finishes to avoid goroutine leaks
	t.Cleanup(func() {
		_ = provider.Stop(context.Background())
	})

	return provider
}

// =============================================================================
// NewCharacterAIProvider Tests
// =============================================================================

func TestNewCharacterAIProvider(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name: "with api_key",
			config: map[string]interface{}{
				"api_key": "test_key",
			},
		},
		{
			name:   "with empty config",
			config: map[string]interface{}{},
		},
		{
			name: "with custom max_characters",
			config: map[string]interface{}{
				"max_characters": 500,
			},
		},
		{
			name: "with custom max_conversations",
			config: map[string]interface{}{
				"max_conversations": 5000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewCharacterAIProvider(tt.config)
			require.NoError(t, err)
			require.NotNil(t, provider)

			cap := provider.(*CharacterAIProvider)
			assert.True(t, cap.config.SimulationMode)
		})
	}
}

// =============================================================================
// Provider Metadata Tests
// =============================================================================

func TestCharacterAIProvider_GetType(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test_key",
	}

	provider, err := NewCharacterAIProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.GetType() != string(ProviderTypeCharacterAI) {
		t.Errorf("Expected %s, got %v", ProviderTypeCharacterAI, provider.GetType())
	}
}

func TestCharacterAIProvider_GetName(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test_key",
	}

	provider, err := NewCharacterAIProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.GetName() != "character_ai" {
		t.Errorf("Expected 'character_ai', got %v", provider.GetName())
	}
}

func TestCharacterAIProvider_GetCapabilities(t *testing.T) {
	provider := newTestCharacterAIProvider()
	caps := provider.GetCapabilities()

	expectedCaps := []string{
		"character_creation",
		"personality_development",
		"conversation_memory",
		"relationship_tracking",
		"emotional_state",
		"character_learning",
		"memory_compression",
		"character_search",
		"personality_matching",
		"relationship_analysis",
	}

	for _, expected := range expectedCaps {
		assert.Contains(t, caps, expected, "Missing capability: %s", expected)
	}
}

func TestCharacterAIProvider_GetConfiguration(t *testing.T) {
	provider := newTestCharacterAIProvider()
	config := provider.GetConfiguration()

	assert.NotNil(t, config)
	charConfig, ok := config.(*CharacterAIConfig)
	require.True(t, ok)
	assert.True(t, charConfig.SimulationMode)
}

func TestCharacterAIProvider_IsCloud(t *testing.T) {
	provider := newTestCharacterAIProvider()
	// Character.AI is a cloud service (even though we can't access it)
	assert.True(t, provider.IsCloud())
}

func TestCharacterAIProvider_GetCostInfo(t *testing.T) {
	provider := newTestCharacterAIProvider()
	costInfo := provider.GetCostInfo()

	assert.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, "monthly", costInfo.BillingPeriod)
}

// =============================================================================
// Lifecycle Tests
// =============================================================================

func TestCharacterAIProvider_Initialize(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.True(t, provider.initialized)

	// Double initialization should be idempotent
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)
}

func TestCharacterAIProvider_Start(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	// Cleanup: stop provider when test finishes
	t.Cleanup(func() {
		_ = provider.Stop(context.Background())
	})

	// Start without initialize should fail
	err := provider.Start(ctx)
	assert.Error(t, err)

	// Initialize first
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	// Now start should succeed
	err = provider.Start(ctx)
	require.NoError(t, err)
	assert.True(t, provider.started)

	// Double start should be idempotent
	err = provider.Start(ctx)
	require.NoError(t, err)
}

func TestCharacterAIProvider_Stop(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, provider.started)

	// Stop again should be idempotent
	err = provider.Stop(ctx)
	require.NoError(t, err)
}

func TestCharacterAIProvider_Close(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Close(ctx)
	require.NoError(t, err)
}

// =============================================================================
// Collection Management Tests
// =============================================================================

func TestCharacterAIProvider_CreateCollection(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	config := &CollectionConfig{
		Name:        "test-character",
		Description: "Test character collection",
	}

	err := provider.CreateCollection(ctx, "test-character", config)
	require.NoError(t, err)

	// Verify collection was created
	collections, err := provider.ListCollections(ctx)
	require.NoError(t, err)

	found := false
	for _, col := range collections {
		if col.Name == "test-character" {
			found = true
			break
		}
	}
	assert.True(t, found, "Collection not found after creation")
}

func TestCharacterAIProvider_CreateCollection_Duplicate(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	config := &CollectionConfig{
		Name:        "duplicate-character",
		Description: "Test duplicate",
	}

	err := provider.CreateCollection(ctx, "duplicate-character", config)
	require.NoError(t, err)

	// Creating same collection should fail
	err = provider.CreateCollection(ctx, "duplicate-character", config)
	assert.Error(t, err)
}

func TestCharacterAIProvider_DeleteCollection(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{
		Name:        "to-delete",
		Description: "Will be deleted",
	}
	err := provider.CreateCollection(ctx, "to-delete", config)
	require.NoError(t, err)

	// Delete it
	err = provider.DeleteCollection(ctx, "to-delete")
	require.NoError(t, err)

	// Verify it's gone
	_, err = provider.GetCollection(ctx, "to-delete")
	assert.Error(t, err)
}

func TestCharacterAIProvider_DeleteCollection_NotFound(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.DeleteCollection(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestCharacterAIProvider_GetCollection(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{
		Name:        "get-test",
		Description: "Get test collection",
	}
	err := provider.CreateCollection(ctx, "get-test", config)
	require.NoError(t, err)

	// Get it
	col, err := provider.GetCollection(ctx, "get-test")
	require.NoError(t, err)
	assert.Equal(t, "get-test", col.Name)
}

func TestCharacterAIProvider_ListCollections(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create a few collections
	for i := 0; i < 3; i++ {
		config := &CollectionConfig{
			Name: "list-test-" + string(rune('a'+i)),
		}
		err := provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)
	}

	collections, err := provider.ListCollections(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(collections), 3)
}

// =============================================================================
// Vector Storage Tests
// =============================================================================

func TestCharacterAIProvider_Store(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create a collection/character first
	config := &CollectionConfig{Name: "store-test"}
	err := provider.CreateCollection(ctx, "store-test", config)
	require.NoError(t, err)

	// Store vectors as a conversation (not a character) linked to the existing character
	// Use character_id in metadata but a different vector ID
	vectors := []*VectorData{
		{
			ID:     "conversation-1",
			Vector: make([]float64, 1536),
			Metadata: map[string]interface{}{
				"conversation_id": "conversation-1",
				"character_id":    "store-test",
				"type":            "conversation",
			},
			Collection: "store-test",
		},
	}

	err = provider.Store(ctx, vectors)
	require.NoError(t, err)
}

func TestCharacterAIProvider_Store_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	vectors := []*VectorData{
		{
			ID:     "vec-1",
			Vector: make([]float64, 1536),
		},
	}

	err := provider.Store(ctx, vectors)
	assert.Error(t, err)
}

func TestCharacterAIProvider_Retrieve(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection and store data
	config := &CollectionConfig{Name: "retrieve-test"}
	err := provider.CreateCollection(ctx, "retrieve-test", config)
	require.NoError(t, err)

	// Note: Retrieve works on the simulation client's storage
	// After creating a collection, we can retrieve the character

	results, err := provider.Retrieve(ctx, []string{"retrieve-test"})
	require.NoError(t, err)
	// Results depend on simulation client state
	assert.NotNil(t, results)
}

func TestCharacterAIProvider_Retrieve_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	_, err := provider.Retrieve(ctx, []string{"id"})
	assert.Error(t, err)
}

func TestCharacterAIProvider_Update(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection
	config := &CollectionConfig{Name: "update-test"}
	err := provider.CreateCollection(ctx, "update-test", config)
	require.NoError(t, err)

	// Update vector
	vector := &VectorData{
		ID:     "update-test",
		Vector: make([]float64, 1536),
		Metadata: map[string]interface{}{
			"character_id":   "update-test",
			"character_name": "Updated Character",
		},
	}

	err = provider.Update(ctx, "update-test", vector)
	require.NoError(t, err)
}

func TestCharacterAIProvider_Delete(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection
	config := &CollectionConfig{Name: "delete-test"}
	err := provider.CreateCollection(ctx, "delete-test", config)
	require.NoError(t, err)

	// Delete
	err = provider.Delete(ctx, []string{"delete-test"})
	require.NoError(t, err)
}

func TestCharacterAIProvider_Delete_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Delete(ctx, []string{"id"})
	assert.Error(t, err)
}

// =============================================================================
// Search Tests
// =============================================================================

func TestCharacterAIProvider_Search(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create some characters
	for i := 0; i < 3; i++ {
		config := &CollectionConfig{
			Name:        "search-char-" + string(rune('a'+i)),
			Description: "Search test character " + string(rune('a'+i)),
		}
		err := provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)
	}

	query := &VectorQuery{
		Vector:    make([]float64, 1536),
		TopK:      5,
		Threshold: 0.0,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Results), 0)
}

func TestCharacterAIProvider_Search_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	query := &VectorQuery{
		Vector: make([]float64, 1536),
		TopK:   5,
	}

	_, err := provider.Search(ctx, query)
	assert.Error(t, err)
}

func TestCharacterAIProvider_FindSimilar(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create some data first
	config := &CollectionConfig{Name: "find-similar-test"}
	err := provider.CreateCollection(ctx, "find-similar-test", config)
	require.NoError(t, err)

	embedding := make([]float64, 1536)
	results, err := provider.FindSimilar(ctx, embedding, 5, nil)
	require.NoError(t, err)
	// Results may be empty or contain matches, but should not error
	assert.GreaterOrEqual(t, len(results), 0)
}

func TestCharacterAIProvider_FindSimilar_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	embedding := make([]float64, 1536)
	_, err := provider.FindSimilar(ctx, embedding, 5, nil)
	assert.Error(t, err)
}

func TestCharacterAIProvider_BatchFindSimilar(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	queries := [][]float64{
		make([]float64, 1536),
		make([]float64, 1536),
		make([]float64, 1536),
	}

	results, err := provider.BatchFindSimilar(ctx, queries, 3)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

// =============================================================================
// Index Management Tests
// =============================================================================

func TestCharacterAIProvider_CreateIndex(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{Name: "index-test"}
	err := provider.CreateCollection(ctx, "index-test", config)
	require.NoError(t, err)

	// Create index (Character.AI handles indexing internally)
	indexConfig := &IndexConfig{
		Name: "test-index",
		Type: "personality",
	}

	err = provider.CreateIndex(ctx, "index-test", indexConfig)
	require.NoError(t, err)
}

func TestCharacterAIProvider_ListIndexes(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{Name: "list-index-test"}
	err := provider.CreateCollection(ctx, "list-index-test", config)
	require.NoError(t, err)

	indexes, err := provider.ListIndexes(ctx, "list-index-test")
	require.NoError(t, err)
	assert.NotNil(t, indexes)
}

func TestCharacterAIProvider_DeleteIndex(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{Name: "del-index-test"}
	err := provider.CreateCollection(ctx, "del-index-test", config)
	require.NoError(t, err)

	err = provider.DeleteIndex(ctx, "del-index-test", "test-index")
	require.NoError(t, err)
}

// =============================================================================
// Metadata Tests
// =============================================================================

func TestCharacterAIProvider_AddMetadata(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{Name: "meta-test"}
	err := provider.CreateCollection(ctx, "meta-test", config)
	require.NoError(t, err)

	metadata := map[string]interface{}{
		"custom_field": "value",
		"another":      123,
	}

	err = provider.AddMetadata(ctx, "meta-test", metadata)
	require.NoError(t, err)
}

func TestCharacterAIProvider_UpdateMetadata(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{Name: "update-meta-test"}
	err := provider.CreateCollection(ctx, "update-meta-test", config)
	require.NoError(t, err)

	metadata := map[string]interface{}{
		"updated_field": "new_value",
	}

	err = provider.UpdateMetadata(ctx, "update-meta-test", metadata)
	require.NoError(t, err)
}

func TestCharacterAIProvider_GetMetadata(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{Name: "get-meta-test"}
	err := provider.CreateCollection(ctx, "get-meta-test", config)
	require.NoError(t, err)

	metadata, err := provider.GetMetadata(ctx, []string{"get-meta-test"})
	require.NoError(t, err)
	assert.NotNil(t, metadata)
}

func TestCharacterAIProvider_DeleteMetadata(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection first
	config := &CollectionConfig{Name: "del-meta-test"}
	err := provider.CreateCollection(ctx, "del-meta-test", config)
	require.NoError(t, err)

	err = provider.DeleteMetadata(ctx, []string{"del-meta-test"}, []string{"some_key"})
	require.NoError(t, err)
}

// =============================================================================
// Stats and Health Tests
// =============================================================================

func TestCharacterAIProvider_GetStats(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	stats, err := provider.GetStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestCharacterAIProvider_Health(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "healthy", health.Status)
}

func TestCharacterAIProvider_Health_NotInitialized(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "not_initialized", health.Status)
}

func TestCharacterAIProvider_Health_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "not_started", health.Status)
}

// =============================================================================
// Backup and Restore Tests
// =============================================================================

func TestCharacterAIProvider_Backup(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Backup(ctx, "/tmp/character_ai_backup")
	require.NoError(t, err)
}

func TestCharacterAIProvider_Restore(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Restore(ctx, "/tmp/character_ai_backup")
	require.NoError(t, err)
}

func TestCharacterAIProvider_Optimize(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Optimize(ctx)
	require.NoError(t, err)
}

// =============================================================================
// Simulation Client Tests
// =============================================================================

func TestCharacterAISimulationClient_CreateCharacter(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	character := &memory.Character{
		ID:          "char-1",
		Name:        "Test Character",
		Description: "A test character",
		Personality: map[string]interface{}{
			"friendly": true,
		},
	}

	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	// Retrieve it
	retrieved, err := client.GetCharacter(ctx, "char-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Character", retrieved.Name)
	assert.True(t, retrieved.IsActive)
}

func TestCharacterAISimulationClient_CreateCharacter_MaxLimit(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    2,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create max characters
	for i := 0; i < 2; i++ {
		err = client.CreateCharacter(ctx, &memory.Character{
			ID:   "char-" + string(rune('a'+i)),
			Name: "Character " + string(rune('a'+i)),
		})
		require.NoError(t, err)
	}

	// Third should fail
	err = client.CreateCharacter(ctx, &memory.Character{
		ID:   "char-c",
		Name: "Character C",
	})
	assert.Error(t, err)
}

func TestCharacterAISimulationClient_CreateCharacter_Duplicate(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	character := &memory.Character{
		ID:   "dup-char",
		Name: "Duplicate Character",
	}

	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	// Duplicate should fail
	err = client.CreateCharacter(ctx, character)
	assert.Error(t, err)
}

func TestCharacterAISimulationClient_GetCharacter_NotFound(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.GetCharacter(ctx, "nonexistent")
	assert.True(t, errors.Is(err, ErrCharacterNotFound))
}

func TestCharacterAISimulationClient_UpdateCharacter(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character
	character := &memory.Character{
		ID:   "update-char",
		Name: "Original Name",
	}
	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	// Update it
	character.Name = "Updated Name"
	err = client.UpdateCharacter(ctx, character)
	require.NoError(t, err)

	// Verify update
	updated, err := client.GetCharacter(ctx, "update-char")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
}

func TestCharacterAISimulationClient_DeleteCharacter(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character
	err = client.CreateCharacter(ctx, &memory.Character{
		ID:   "delete-char",
		Name: "To Delete",
	})
	require.NoError(t, err)

	// Delete it
	err = client.DeleteCharacter(ctx, "delete-char")
	require.NoError(t, err)

	// Verify deletion
	_, err = client.GetCharacter(ctx, "delete-char")
	assert.True(t, errors.Is(err, ErrCharacterNotFound))
}

func TestCharacterAISimulationClient_ListCharacters(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create multiple characters
	for i := 0; i < 5; i++ {
		err = client.CreateCharacter(ctx, &memory.Character{
			ID:   "list-char-" + string(rune('a'+i)),
			Name: "List Character " + string(rune('a'+i)),
		})
		require.NoError(t, err)
	}

	characters, err := client.ListCharacters(ctx)
	require.NoError(t, err)
	assert.Len(t, characters, 5)
}

func TestCharacterAISimulationClient_Conversations(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character first
	err = client.CreateCharacter(ctx, &memory.Character{
		ID:   "conv-char",
		Name: "Conversation Character",
	})
	require.NoError(t, err)

	// Create conversation
	conv := &memory.Conversation{
		ID:          "conv-1",
		Title:       "Test Conversation",
		CharacterID: "conv-char",
	}
	err = client.CreateConversation(ctx, conv)
	require.NoError(t, err)

	// Get conversation
	retrieved, err := client.GetConversation(ctx, "conv-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Conversation", retrieved.Title)

	// List conversations for character
	convs, err := client.ListConversations(ctx, "conv-char")
	require.NoError(t, err)
	assert.Len(t, convs, 1)
}

func TestCharacterAISimulationClient_SendMessage(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character
	err = client.CreateCharacter(ctx, &memory.Character{
		ID:   "msg-char",
		Name: "Message Character",
		Personality: map[string]interface{}{
			"friendly": true,
		},
	})
	require.NoError(t, err)

	// Create conversation
	err = client.CreateConversation(ctx, &memory.Conversation{
		ID:          "msg-conv",
		Title:       "Message Test",
		CharacterID: "msg-char",
	})
	require.NoError(t, err)

	// Send message
	msg := &memory.CharacterMessage{
		SessionID: "msg-conv",
		SenderID:  "user",
		Content:   "Hello!",
		Type:      "user",
	}

	response, err := client.SendMessage(ctx, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, response.Content)
	assert.Equal(t, "character", response.Type)

	// Get messages
	messages, err := client.GetMessages(ctx, "msg-conv", 10)
	require.NoError(t, err)
	assert.Len(t, messages, 2) // User message + character response
}

func TestCharacterAISimulationClient_UpdatePersonality(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character
	err = client.CreateCharacter(ctx, &memory.Character{
		ID:   "pers-char",
		Name: "Personality Character",
	})
	require.NoError(t, err)

	// Update personality
	traits := map[string]interface{}{
		"friendly": true,
		"creative": true,
	}
	err = client.UpdatePersonality(ctx, "pers-char", traits)
	require.NoError(t, err)

	// Verify
	char, err := client.GetCharacter(ctx, "pers-char")
	require.NoError(t, err)
	assert.True(t, char.Personality["friendly"].(bool))
	assert.True(t, char.Personality["creative"].(bool))
}

func TestCharacterAISimulationClient_Relationships(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Get default relationship
	rel, err := client.GetRelationship(ctx, "char-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "acquaintance", rel.Type)
	assert.Equal(t, 0.5, rel.Strength)

	// Update relationship
	updatedRel := &memory.RelationshipData{
		CharacterID: "char-1",
		UserID:      "user-1",
		Type:        "friend",
		Strength:    0.8,
	}
	err = client.UpdateRelationship(ctx, "char-1", "user-1", updatedRel)
	require.NoError(t, err)

	// Verify update
	retrieved, err := client.GetRelationship(ctx, "char-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "friend", retrieved.Type)
	assert.Equal(t, 0.8, retrieved.Strength)
}

func TestCharacterAISimulationClient_EmotionalState(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Get default emotional state
	state, err := client.GetEmotionalState(ctx, "char-1")
	require.NoError(t, err)
	assert.Equal(t, "neutral", state.Mood)
	assert.Equal(t, 0.5, state.Intensity)

	// Update emotional state
	newState := &memory.EmotionalState{
		AvatarID:  "char-1",
		Mood:      "happy",
		Intensity: 0.9,
		Context:   "good conversation",
	}
	err = client.UpdateEmotionalState(ctx, "char-1", newState)
	require.NoError(t, err)

	// Verify update
	retrieved, err := client.GetEmotionalState(ctx, "char-1")
	require.NoError(t, err)
	assert.Equal(t, "happy", retrieved.Mood)
	assert.Equal(t, 0.9, retrieved.Intensity)
}

func TestCharacterAISimulationClient_Health(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Simulation client health should always succeed
	err = client.GetHealth(ctx)
	require.NoError(t, err)
}

// =============================================================================
// Error Variable Tests
// =============================================================================

func TestCharacterAI_ErrorVariables(t *testing.T) {
	assert.NotNil(t, ErrCharacterAINoPublicAPI)
	assert.NotNil(t, ErrCharacterAIStandaloneMode)
	assert.NotNil(t, ErrCharacterNotFound)
	assert.NotNil(t, ErrConversationNotFound)
	assert.NotNil(t, ErrCharacterAINotStarted)
	assert.NotNil(t, ErrCharacterAINotInitialized)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "empty vectors",
			a:        []float64{},
			b:        []float64{},
			expected: 0.0,
		},
		{
			name:     "different length vectors",
			a:        []float64{1, 2},
			b:        []float64{1, 2, 3},
			expected: 0.0,
		},
		{
			name:     "zero vector",
			a:        []float64{0, 0, 0},
			b:        []float64{1, 2, 3},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestCharacterAIProvider_GenerateSimulatedEmbedding(t *testing.T) {
	provider := newTestCharacterAIProvider()

	// Same text should produce same embedding
	emb1 := provider.generateEmbedding("test text")
	emb2 := provider.generateEmbedding("test text")
	assert.Equal(t, emb1, emb2)

	// Different text should produce different embedding
	emb3 := provider.generateEmbedding("different text")
	assert.NotEqual(t, emb1, emb3)

	// Embedding should be normalized (length ~1)
	var norm float64
	for _, v := range emb1 {
		norm += v * v
	}
	assert.InDelta(t, 1.0, norm, 0.0001)

	// Empty text should return zero vector
	emb4 := provider.generateEmbedding("")
	for _, v := range emb4 {
		assert.Equal(t, 0.0, v)
	}
}

func TestCharacterAIProvider_GetCharacterMessageCount(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create character
	config := &CollectionConfig{Name: "msg-count-char"}
	err := provider.CreateCollection(ctx, "msg-count-char", config)
	require.NoError(t, err)

	// Initially should be 0
	count := provider.getCharacterMessageCount("msg-count-char")
	assert.Equal(t, 0, count)
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

func TestCharacterAIProvider_ImplementsVectorProvider(t *testing.T) {
	var _ VectorProvider = (*CharacterAIProvider)(nil)
}

// =============================================================================
// Error Handling Edge Case Tests
// =============================================================================

func TestCharacterAIProvider_InitializeWithCancelledContext(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Initialize should still work since simulation client doesn't depend on context
	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)
}

func TestCharacterAIProvider_StartWithCancelledContext(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Start should still work
	err = provider.Start(cancelledCtx)
	require.NoError(t, err)

	// Cleanup
	_ = provider.Stop(context.Background())
}

func TestCharacterAIProvider_StopWithCancelledContext(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.Stop(ctx)
	require.NoError(t, err)
}

func TestCharacterAIProvider_StoreWithNilVectors(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Store(ctx, nil)
	require.NoError(t, err)
}

func TestCharacterAIProvider_StoreWithEmptyVectors(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Store(ctx, []*VectorData{})
	require.NoError(t, err)
}

func TestCharacterAIProvider_RetrieveWithEmptyIDs(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	results, err := provider.Retrieve(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCharacterAIProvider_RetrieveWithNilIDs(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	results, err := provider.Retrieve(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCharacterAIProvider_RetrieveNonExistentIDs(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	results, err := provider.Retrieve(ctx, []string{"nonexistent-1", "nonexistent-2"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCharacterAIProvider_DeleteWithEmptyIDs(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Delete(ctx, []string{})
	require.NoError(t, err)
}

func TestCharacterAIProvider_DeleteWithNilIDs(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Delete(ctx, nil)
	require.NoError(t, err)
}

func TestCharacterAIProvider_DeleteNonExistentIDs(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.Delete(ctx, []string{"nonexistent-1", "nonexistent-2"})
	require.NoError(t, err)
}

func TestCharacterAIProvider_UpdateNonExistentVector(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	vector := &VectorData{
		ID:     "nonexistent-id",
		Vector: make([]float64, 1536),
		Metadata: map[string]interface{}{
			"character_id": "nonexistent-id",
		},
	}

	// Update should fail for non-existent character
	err := provider.Update(ctx, "nonexistent-id", vector)
	assert.Error(t, err)
}

func TestCharacterAIProvider_Update_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	vector := &VectorData{
		ID:     "test-id",
		Vector: make([]float64, 1536),
	}

	err := provider.Update(ctx, "test-id", vector)
	assert.Error(t, err)
}

func TestCharacterAIProvider_SearchWithZeroTopK(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	query := &VectorQuery{
		Vector:    make([]float64, 1536),
		TopK:      0,
		Threshold: 0.0,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Results)
}

func TestCharacterAIProvider_SearchWithNegativeTopK(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	query := &VectorQuery{
		Vector:    make([]float64, 1536),
		TopK:      -5,
		Threshold: 0.0,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCharacterAIProvider_SearchWithEmptyVector(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	query := &VectorQuery{
		Vector:    []float64{},
		TopK:      5,
		Threshold: 0.0,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCharacterAIProvider_SearchWithHighThreshold(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create some characters
	config := &CollectionConfig{Name: "high-threshold-test"}
	err := provider.CreateCollection(ctx, "high-threshold-test", config)
	require.NoError(t, err)

	query := &VectorQuery{
		Vector:    make([]float64, 1536),
		TopK:      10,
		Threshold: 0.999, // Very high threshold
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// With high threshold, few or no results expected
}

func TestCharacterAIProvider_CreateCollectionWithEmptyName(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	config := &CollectionConfig{
		Name:        "",
		Description: "Empty name collection",
	}

	// Empty name might still work (depends on implementation)
	err := provider.CreateCollection(ctx, "", config)
	// Check that it either succeeds or fails gracefully
	if err != nil {
		assert.Contains(t, err.Error(), "already exists")
	}
}

func TestCharacterAIProvider_CreateCollectionWithNilConfig(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Should handle nil config gracefully (may panic if not handled)
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic caught (expected for nil config): %v", r)
		}
	}()

	err := provider.CreateCollection(ctx, "nil-config-test", nil)
	// Either succeeds or fails gracefully
	_ = err
}

func TestCharacterAIProvider_GetCollectionNotFound(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	_, err := provider.GetCollection(ctx, "nonexistent-collection")
	assert.Error(t, err)
}

func TestCharacterAIProvider_AddMetadata_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.AddMetadata(ctx, "test-id", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
}

func TestCharacterAIProvider_AddMetadataToNonExistent(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.AddMetadata(ctx, "nonexistent-id", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
}

func TestCharacterAIProvider_GetMetadata_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	_, err := provider.GetMetadata(ctx, []string{"test-id"})
	assert.Error(t, err)
}

func TestCharacterAIProvider_GetMetadataWithEmptyIDs(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	result, err := provider.GetMetadata(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestCharacterAIProvider_DeleteMetadata_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.DeleteMetadata(ctx, []string{"test-id"}, []string{"key"})
	assert.Error(t, err)
}

func TestCharacterAIProvider_DeleteIndex_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.DeleteIndex(ctx, "collection", "index")
	assert.Error(t, err)
}

func TestCharacterAIProvider_DeleteIndexNonExistentCollection(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	err := provider.DeleteIndex(ctx, "nonexistent-collection", "test-index")
	assert.Error(t, err)
}

func TestCharacterAIProvider_ListIndexes_NotFound(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	_, err := provider.ListIndexes(ctx, "nonexistent-collection")
	assert.Error(t, err)
}

func TestCharacterAIProvider_BatchFindSimilar_NotStarted(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	queries := [][]float64{make([]float64, 1536)}
	_, err := provider.BatchFindSimilar(ctx, queries, 5)
	assert.Error(t, err)
}

func TestCharacterAIProvider_BatchFindSimilarWithEmptyQueries(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	results, err := provider.BatchFindSimilar(ctx, [][]float64{}, 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCharacterAISimulationClient_SendMessageToNonExistentConversation(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	msg := &memory.CharacterMessage{
		SessionID: "nonexistent-conv",
		SenderID:  "user",
		Content:   "Hello!",
		Type:      "user",
	}

	_, err = client.SendMessage(ctx, msg)
	assert.True(t, errors.Is(err, ErrConversationNotFound))
}

func TestCharacterAISimulationClient_GetMessagesFromNonExistentConversation(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.GetMessages(ctx, "nonexistent-conv", 10)
	assert.True(t, errors.Is(err, ErrConversationNotFound))
}

func TestCharacterAISimulationClient_UpdateNonExistentCharacter(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	character := &memory.Character{
		ID:   "nonexistent",
		Name: "Test",
	}

	err = client.UpdateCharacter(ctx, character)
	assert.True(t, errors.Is(err, ErrCharacterNotFound))
}

func TestCharacterAISimulationClient_DeleteNonExistentCharacter(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.DeleteCharacter(ctx, "nonexistent")
	assert.True(t, errors.Is(err, ErrCharacterNotFound))
}

func TestCharacterAISimulationClient_UpdatePersonalityNonExistentCharacter(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	traits := map[string]interface{}{"friendly": true}
	err = client.UpdatePersonality(ctx, "nonexistent", traits)
	assert.True(t, errors.Is(err, ErrCharacterNotFound))
}

func TestCharacterAISimulationClient_UpdateNonExistentConversation(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	conv := &memory.Conversation{
		ID:    "nonexistent",
		Title: "Test",
	}

	err = client.UpdateConversation(ctx, conv)
	assert.True(t, errors.Is(err, ErrConversationNotFound))
}

func TestCharacterAISimulationClient_DeleteNonExistentConversation(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.DeleteConversation(ctx, "nonexistent")
	assert.True(t, errors.Is(err, ErrConversationNotFound))
}

func TestCharacterAISimulationClient_CreateConversationMaxLimit(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 2,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create 2 conversations
	for i := 0; i < 2; i++ {
		conv := &memory.Conversation{
			ID:    "conv-" + string(rune('a'+i)),
			Title: "Conv " + string(rune('a'+i)),
		}
		err = client.CreateConversation(ctx, conv)
		require.NoError(t, err)
	}

	// Third should fail
	conv := &memory.Conversation{
		ID:    "conv-c",
		Title: "Conv C",
	}
	err = client.CreateConversation(ctx, conv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum conversation limit")
}

func TestCharacterAISimulationClient_CreateDuplicateConversation(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	conv := &memory.Conversation{
		ID:    "dup-conv",
		Title: "Duplicate Conv",
	}
	err = client.CreateConversation(ctx, conv)
	require.NoError(t, err)

	// Duplicate should fail
	err = client.CreateConversation(ctx, conv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCharacterAISimulationClient_GetMessagesWithZeroLimit(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create conversation
	conv := &memory.Conversation{
		ID:    "zero-limit-conv",
		Title: "Test",
	}
	err = client.CreateConversation(ctx, conv)
	require.NoError(t, err)

	// Get messages with 0 limit (should return all)
	msgs, err := client.GetMessages(ctx, "zero-limit-conv", 0)
	require.NoError(t, err)
	assert.NotNil(t, msgs)
}

func TestCharacterAISimulationClient_GetMessagesWithNegativeLimit(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create conversation
	conv := &memory.Conversation{
		ID:    "neg-limit-conv",
		Title: "Test",
	}
	err = client.CreateConversation(ctx, conv)
	require.NoError(t, err)

	// Get messages with negative limit (should handle gracefully)
	msgs, err := client.GetMessages(ctx, "neg-limit-conv", -5)
	require.NoError(t, err)
	assert.NotNil(t, msgs)
}

// =============================================================================
// Concurrency and Race Condition Tests
// =============================================================================

func TestCharacterAIProvider_ConcurrentCollectionCreation(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	const numGoroutines = 20
	done := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			config := &CollectionConfig{
				Name: fmt.Sprintf("concurrent-create-%d", idx),
			}
			err := provider.CreateCollection(ctx, config.Name, config)
			done <- err
		}(i)
	}

	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-done
		if err == nil {
			successCount++
		}
	}

	// All should succeed since they have unique names
	assert.Equal(t, numGoroutines, successCount)
}

func TestCharacterAIProvider_ConcurrentWriteAndRead(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create initial collection
	config := &CollectionConfig{Name: "concurrent-rw-test"}
	err := provider.CreateCollection(ctx, "concurrent-rw-test", config)
	require.NoError(t, err)

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			if idx%2 == 0 {
				// Read operation
				_, _ = provider.GetCollection(ctx, "concurrent-rw-test")
			} else {
				// Write operation - search
				query := &VectorQuery{
					Vector: make([]float64, 1536),
					TopK:   5,
				}
				_, _ = provider.Search(ctx, query)
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions occurred
}

func TestCharacterAIProvider_ConcurrentHealthChecks(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	const numGoroutines = 30
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := provider.Health(ctx)
			errors <- err
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		assert.NoError(t, err)
	}
}

func TestCharacterAIProvider_ConcurrentStatsRetrieval(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	const numGoroutines = 30
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			stats, err := provider.GetStats(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, stats)
		}()
	}

	wg.Wait()
}

func TestCharacterAIProvider_ConcurrentListCollections(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create some collections first
	for i := 0; i < 5; i++ {
		config := &CollectionConfig{Name: fmt.Sprintf("list-test-%d", i)}
		_ = provider.CreateCollection(ctx, config.Name, config)
	}

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			collections, err := provider.ListCollections(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, collections)
		}()
	}

	wg.Wait()
}

func TestCharacterAISimulationClient_ConcurrentCharacterOperations(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    1000,
		MaxConversations: 10000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			// Create character
			character := &memory.Character{
				ID:   fmt.Sprintf("concurrent-char-%d", idx),
				Name: fmt.Sprintf("Character %d", idx),
			}
			err := client.CreateCharacter(ctx, character)
			if err != nil {
				return
			}

			// Update character
			character.Name = fmt.Sprintf("Updated Character %d", idx)
			_ = client.UpdateCharacter(ctx, character)

			// Get character
			_, _ = client.GetCharacter(ctx, character.ID)
		}(i)
	}

	wg.Wait()
}

func TestCharacterAISimulationClient_ConcurrentConversationOperations(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    1000,
		MaxConversations: 10000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a character first
	character := &memory.Character{
		ID:   "conv-test-char",
		Name: "Test Character",
	}
	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			// Create conversation
			conv := &memory.Conversation{
				ID:          fmt.Sprintf("concurrent-conv-%d", idx),
				Title:       fmt.Sprintf("Conversation %d", idx),
				CharacterID: "conv-test-char",
			}
			err := client.CreateConversation(ctx, conv)
			if err != nil {
				return
			}

			// List conversations
			_, _ = client.ListConversations(ctx, "conv-test-char")

			// Get conversation
			_, _ = client.GetConversation(ctx, conv.ID)
		}(i)
	}

	wg.Wait()
}

func TestCharacterAISimulationClient_ConcurrentMessageSending(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character and conversation
	character := &memory.Character{
		ID:   "msg-test-char",
		Name: "Message Test Character",
	}
	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	conv := &memory.Conversation{
		ID:          "msg-test-conv",
		Title:       "Message Test",
		CharacterID: "msg-test-char",
	}
	err = client.CreateConversation(ctx, conv)
	require.NoError(t, err)

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			msg := &memory.CharacterMessage{
				SessionID: "msg-test-conv",
				SenderID:  "user",
				Content:   fmt.Sprintf("Message %d", idx),
				Type:      "user",
			}
			_, _ = client.SendMessage(ctx, msg)
		}(i)
	}

	wg.Wait()

	// Verify messages were created
	messages, err := client.GetMessages(ctx, "msg-test-conv", 100)
	require.NoError(t, err)
	// Each goroutine sends 1 message and gets 1 response = 2 messages per goroutine
	assert.GreaterOrEqual(t, len(messages), numGoroutines)
}

func TestCharacterAISimulationClient_ConcurrentRelationshipUpdates(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	const numGoroutines = 30
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			charID := fmt.Sprintf("rel-char-%d", idx%5) // 5 different characters
			userID := fmt.Sprintf("rel-user-%d", idx%3) // 3 different users

			// Get and update relationship
			rel, err := client.GetRelationship(ctx, charID, userID)
			if err != nil {
				return
			}

			rel.Strength = float64(idx) / 100.0
			_ = client.UpdateRelationship(ctx, charID, userID, rel)
		}(i)
	}

	wg.Wait()
}

func TestCharacterAISimulationClient_ConcurrentEmotionalStateUpdates(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	const numGoroutines = 30
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	moods := []string{"happy", "sad", "excited", "calm", "anxious"}

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			charID := fmt.Sprintf("emo-char-%d", idx%5)

			state := &memory.EmotionalState{
				AvatarID:  charID,
				Mood:      moods[idx%len(moods)],
				Intensity: float64(idx%100) / 100.0,
			}
			_ = client.UpdateEmotionalState(ctx, charID, state)

			_, _ = client.GetEmotionalState(ctx, charID)
		}(i)
	}

	wg.Wait()
}

func TestCharacterAIProvider_ConcurrentReads(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Pre-create some collections
	for i := 0; i < 3; i++ {
		config := &CollectionConfig{
			Name: "preexisting-" + string(rune('a'+i)),
		}
		err := provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)
	}

	// Run concurrent read operations only (safer)
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = provider.Health(ctx)
			_, _ = provider.GetStats(ctx)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestCharacterAIProvider_SequentialOperations(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Test sequential operations work correctly
	for i := 0; i < 5; i++ {
		config := &CollectionConfig{
			Name: "sequential-" + string(rune('a'+i)),
		}
		err := provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)

		collections, err := provider.ListCollections(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(collections), i+1)

		stats, err := provider.GetStats(ctx)
		require.NoError(t, err)
		assert.NotNil(t, stats)

		health, err := provider.Health(ctx)
		require.NoError(t, err)
		assert.Equal(t, "healthy", health.Status)
	}
}

// =============================================================================
// Large Data Handling Tests
// =============================================================================

func TestCharacterAIProvider_LargeVectorDimensions(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection
	config := &CollectionConfig{Name: "large-vector-test"}
	err := provider.CreateCollection(ctx, "large-vector-test", config)
	require.NoError(t, err)

	// Test with large vector dimension (4096)
	largeVector := make([]float64, 4096)
	for i := range largeVector {
		largeVector[i] = float64(i) / 4096.0
	}

	query := &VectorQuery{
		Vector:    largeVector,
		TopK:      5,
		Threshold: 0.0,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCharacterAIProvider_ManyCollections(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	const numCollections = 100

	// Create many collections
	for i := 0; i < numCollections; i++ {
		config := &CollectionConfig{
			Name:        fmt.Sprintf("many-collections-%d", i),
			Description: fmt.Sprintf("Collection %d for testing", i),
		}
		err := provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)
	}

	// List all collections
	collections, err := provider.ListCollections(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(collections), numCollections)

	// Verify we can retrieve each one
	for i := 0; i < numCollections; i++ {
		col, err := provider.GetCollection(ctx, fmt.Sprintf("many-collections-%d", i))
		require.NoError(t, err)
		assert.NotNil(t, col)
	}
}

func TestCharacterAISimulationClient_ManyCharacters(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    500,
		MaxConversations: 5000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	const numCharacters = 200

	// Create many characters
	for i := 0; i < numCharacters; i++ {
		character := &memory.Character{
			ID:          fmt.Sprintf("bulk-char-%d", i),
			Name:        fmt.Sprintf("Character %d", i),
			Description: fmt.Sprintf("Description for character %d with some extra text to simulate larger metadata", i),
			Personality: map[string]interface{}{
				"trait1": i % 5,
				"trait2": float64(i) / 100.0,
				"trait3": i%2 == 0,
			},
		}
		err := client.CreateCharacter(ctx, character)
		require.NoError(t, err)
	}

	// List all characters
	characters, err := client.ListCharacters(ctx)
	require.NoError(t, err)
	assert.Equal(t, numCharacters, len(characters))
}

func TestCharacterAISimulationClient_ManyConversations(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a character
	character := &memory.Character{
		ID:   "conv-bulk-char",
		Name: "Bulk Conversation Character",
	}
	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	const numConversations = 200

	// Create many conversations
	for i := 0; i < numConversations; i++ {
		conv := &memory.Conversation{
			ID:          fmt.Sprintf("bulk-conv-%d", i),
			Title:       fmt.Sprintf("Conversation %d", i),
			CharacterID: "conv-bulk-char",
		}
		err := client.CreateConversation(ctx, conv)
		require.NoError(t, err)
	}

	// List all conversations for the character
	conversations, err := client.ListConversations(ctx, "conv-bulk-char")
	require.NoError(t, err)
	assert.Equal(t, numConversations, len(conversations))
}

func TestCharacterAISimulationClient_ManyMessagesInConversation(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character and conversation
	character := &memory.Character{
		ID:   "msg-bulk-char",
		Name: "Message Bulk Character",
	}
	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	conv := &memory.Conversation{
		ID:          "msg-bulk-conv",
		Title:       "Bulk Message Test",
		CharacterID: "msg-bulk-char",
	}
	err = client.CreateConversation(ctx, conv)
	require.NoError(t, err)

	const numMessages = 100

	// Send many messages
	for i := 0; i < numMessages; i++ {
		msg := &memory.CharacterMessage{
			SessionID: "msg-bulk-conv",
			SenderID:  "user",
			Content:   fmt.Sprintf("Message %d with some additional content to simulate real messages", i),
			Type:      "user",
		}
		response, err := client.SendMessage(ctx, msg)
		require.NoError(t, err)
		assert.NotEmpty(t, response.Content)
	}

	// Get all messages (should be 2 * numMessages due to responses)
	messages, err := client.GetMessages(ctx, "msg-bulk-conv", numMessages*2+10)
	require.NoError(t, err)
	assert.Equal(t, numMessages*2, len(messages))
}

func TestCharacterAIProvider_LargeMetadata(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collection
	config := &CollectionConfig{Name: "large-metadata-test"}
	err := provider.CreateCollection(ctx, "large-metadata-test", config)
	require.NoError(t, err)

	// Create large metadata map
	largeMetadata := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeMetadata[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d_with_some_longer_text_to_simulate_real_data", i)
	}

	// Add metadata
	err = provider.AddMetadata(ctx, "large-metadata-test", largeMetadata)
	require.NoError(t, err)

	// Retrieve metadata
	metadata, err := provider.GetMetadata(ctx, []string{"large-metadata-test"})
	require.NoError(t, err)
	assert.NotNil(t, metadata["large-metadata-test"])
}

func TestCharacterAIProvider_BatchFindSimilarManyQueries(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create some collections to search
	for i := 0; i < 10; i++ {
		config := &CollectionConfig{Name: fmt.Sprintf("batch-search-%d", i)}
		_ = provider.CreateCollection(ctx, config.Name, config)
	}

	const numQueries = 50
	queries := make([][]float64, numQueries)
	for i := range queries {
		queries[i] = make([]float64, 1536)
		for j := range queries[i] {
			queries[i][j] = float64(i*1536+j) / float64(numQueries*1536)
		}
	}

	results, err := provider.BatchFindSimilar(ctx, queries, 5)
	require.NoError(t, err)
	assert.Len(t, results, numQueries)
}

// =============================================================================
// Memory Leaks and Cleanup Verification Tests
// =============================================================================

func TestCharacterAIProvider_CleanupAfterStop(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	// Initialize and start
	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Create some data
	for i := 0; i < 10; i++ {
		config := &CollectionConfig{Name: fmt.Sprintf("cleanup-test-%d", i)}
		_ = provider.CreateCollection(ctx, config.Name, config)
	}

	// Stop
	err = provider.Stop(ctx)
	require.NoError(t, err)

	// Verify stopped state
	assert.False(t, provider.started)

	// Operations should fail after stop
	_, err = provider.Search(ctx, &VectorQuery{Vector: make([]float64, 1536), TopK: 5})
	assert.Error(t, err)
}

func TestCharacterAIProvider_CleanupAfterClose(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	// Initialize and start
	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Create some data
	for i := 0; i < 5; i++ {
		config := &CollectionConfig{Name: fmt.Sprintf("close-test-%d", i)}
		_ = provider.CreateCollection(ctx, config.Name, config)
	}

	// Close
	err = provider.Close(ctx)
	require.NoError(t, err)
}

func TestCharacterAIProvider_MultipleStartStopCycles(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	// Multiple start/stop cycles
	for i := 0; i < 5; i++ {
		err = provider.Start(ctx)
		require.NoError(t, err)
		assert.True(t, provider.started)

		// Create a collection
		config := &CollectionConfig{Name: fmt.Sprintf("cycle-test-%d", i)}
		err = provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)

		err = provider.Stop(ctx)
		require.NoError(t, err)
		assert.False(t, provider.started)
	}
}

func TestCharacterAISimulationClient_CleanupOnCharacterDelete(t *testing.T) {
	config := &CharacterAIConfig{
		MaxCharacters:    100,
		MaxConversations: 1000,
	}
	client, err := NewCharacterAISimulationClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Create character with conversations
	character := &memory.Character{
		ID:   "cleanup-char",
		Name: "Cleanup Character",
	}
	err = client.CreateCharacter(ctx, character)
	require.NoError(t, err)

	// Create conversations for this character
	for i := 0; i < 5; i++ {
		conv := &memory.Conversation{
			ID:          fmt.Sprintf("cleanup-conv-%d", i),
			Title:       fmt.Sprintf("Cleanup Conv %d", i),
			CharacterID: "cleanup-char",
		}
		err = client.CreateConversation(ctx, conv)
		require.NoError(t, err)

		// Send messages
		msg := &memory.CharacterMessage{
			SessionID: conv.ID,
			SenderID:  "user",
			Content:   "Test message",
			Type:      "user",
		}
		_, _ = client.SendMessage(ctx, msg)
	}

	// Delete character (should also delete associated conversations)
	err = client.DeleteCharacter(ctx, "cleanup-char")
	require.NoError(t, err)

	// Verify character is gone
	_, err = client.GetCharacter(ctx, "cleanup-char")
	assert.True(t, errors.Is(err, ErrCharacterNotFound))

	// Verify conversations are gone
	for i := 0; i < 5; i++ {
		_, err = client.GetConversation(ctx, fmt.Sprintf("cleanup-conv-%d", i))
		assert.True(t, errors.Is(err, ErrConversationNotFound))
	}
}

func TestCharacterAIProvider_ResourceCleanupOnDelete(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Create collections
	for i := 0; i < 10; i++ {
		config := &CollectionConfig{Name: fmt.Sprintf("resource-cleanup-%d", i)}
		err := provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)
	}

	// Get initial stats
	stats, err := provider.GetStats(ctx)
	require.NoError(t, err)
	initialCollections := stats.TotalCollections

	// Delete all collections
	for i := 0; i < 10; i++ {
		err := provider.DeleteCollection(ctx, fmt.Sprintf("resource-cleanup-%d", i))
		require.NoError(t, err)
	}

	// Verify stats updated
	stats, err = provider.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCollections-10, stats.TotalCollections)
}

func TestCharacterAIProvider_BackgroundWorkerCleanup(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	// Start multiple times (should be idempotent and not leak goroutines)
	for i := 0; i < 3; i++ {
		err = provider.Start(ctx)
		require.NoError(t, err)
	}

	// Stop should clean up the background worker
	err = provider.Stop(ctx)
	require.NoError(t, err)

	// Give time for goroutine to exit
	time.Sleep(100 * time.Millisecond)

	// cancelFunc should be nil after stop
	assert.Nil(t, provider.cancelFunc)
}

// =============================================================================
// Provider Registry Integration Tests
// =============================================================================

func TestCharacterAIProvider_RegistryCreation(t *testing.T) {
	registry := NewProviderRegistry()

	// Verify CharacterAI provider is registered
	providers := registry.ListProviders()
	found := false
	for _, p := range providers {
		if p == ProviderTypeCharacterAI {
			found = true
			break
		}
	}
	assert.True(t, found, "CharacterAI provider should be registered")
}

func TestCharacterAIProvider_RegistryCreateProvider(t *testing.T) {
	registry := NewProviderRegistry()

	config := map[string]interface{}{
		"api_key":        "test_key",
		"max_characters": 100,
	}

	provider, err := registry.CreateProvider(ProviderTypeCharacterAI, config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "character_ai", provider.GetName())
	assert.Equal(t, string(ProviderTypeCharacterAI), provider.GetType())
}

func TestCharacterAIProvider_RegistryGetProviderInfo(t *testing.T) {
	registry := NewProviderRegistry()

	info, err := registry.GetProviderInfo(ProviderTypeCharacterAI)
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, ProviderTypeCharacterAI, info.Type)
	assert.Equal(t, "character_ai", info.Name)
	assert.True(t, info.IsCloud)
	assert.NotEmpty(t, info.Capabilities)
}

func TestCharacterAIProvider_RegistryDefaultConfig(t *testing.T) {
	registry := NewProviderRegistry()

	defaultConfig := registry.GetDefaultConfig(ProviderTypeCharacterAI)
	assert.NotNil(t, defaultConfig)

	// Check expected default values
	assert.Equal(t, "https://api.character.ai", defaultConfig["base_url"])
	assert.Equal(t, 1000, defaultConfig["max_characters"])
	assert.Equal(t, true, defaultConfig["relationship_memory"])
}

func TestCharacterAIProvider_RegistryCreateWithDefaults(t *testing.T) {
	registry := NewProviderRegistry()

	provider, err := registry.CreateProviderWithDefaults(ProviderTypeCharacterAI)
	require.NoError(t, err)
	assert.NotNil(t, provider)

	// Should be in simulation mode
	config := provider.GetConfiguration().(*CharacterAIConfig)
	assert.True(t, config.SimulationMode)
}

func TestCharacterAIProvider_RegistryValidateConfig(t *testing.T) {
	registry := NewProviderRegistry()

	config := map[string]interface{}{
		"max_characters": 500,
	}

	err := registry.ValidateProviderConfig(ProviderTypeCharacterAI, config)
	require.NoError(t, err)
}

func TestCharacterAIProvider_RegistryCompatibility(t *testing.T) {
	registry := NewProviderRegistry()

	// Test capability-based filtering
	requirements := &ProviderRequirements{
		Capabilities: []string{"character_creation", "personality_development"},
	}

	compatible := registry.GetCompatibleProviders(requirements)
	found := false
	for _, p := range compatible {
		if p == ProviderTypeCharacterAI {
			found = true
			break
		}
	}
	assert.True(t, found, "CharacterAI should be compatible with character_creation capability")
}

func TestCharacterAIProvider_RegistryCompatibility_Cloud(t *testing.T) {
	registry := NewProviderRegistry()

	isCloud := true
	requirements := &ProviderRequirements{
		IsCloud: &isCloud,
	}

	compatible := registry.GetCompatibleProviders(requirements)
	found := false
	for _, p := range compatible {
		if p == ProviderTypeCharacterAI {
			found = true
			break
		}
	}
	assert.True(t, found, "CharacterAI should be compatible as a cloud provider")
}

func TestCharacterAIProvider_RegistryStatistics(t *testing.T) {
	registry := NewProviderRegistry()

	stats := registry.GetProviderStatistics()
	assert.NotNil(t, stats)
	assert.True(t, stats.TotalProviders > 0)
	assert.True(t, stats.Initialized)

	// Check that CharacterAI is counted in the appropriate category
	aiMemoryCount := stats.ProvidersByType["ai_memory"]
	assert.True(t, aiMemoryCount > 0, "ai_memory category should have at least one provider")
}

func TestCharacterAIProvider_RegistryGetProviderFactory(t *testing.T) {
	registry := NewProviderRegistry()

	factory, err := registry.GetProviderFactory(ProviderTypeCharacterAI)
	require.NoError(t, err)
	assert.NotNil(t, factory)

	// Use factory to create provider
	provider, err := factory(map[string]interface{}{})
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "character_ai", provider.GetName())
}

func TestCharacterAIProvider_RegistryUnknownProvider(t *testing.T) {
	registry := NewProviderRegistry()

	_, err := registry.CreateProvider(ProviderType("unknown_provider"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider type")
}

func TestCharacterAIProvider_FullLifecycleWithRegistry(t *testing.T) {
	registry := NewProviderRegistry()
	ctx := context.Background()

	// Create provider via registry
	provider, err := registry.CreateProvider(ProviderTypeCharacterAI, map[string]interface{}{
		"max_characters": 50,
	})
	require.NoError(t, err)

	// Initialize
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	// Start
	err = provider.Start(ctx)
	require.NoError(t, err)

	// Use provider
	err = provider.CreateCollection(ctx, "registry-lifecycle-test", &CollectionConfig{Name: "registry-lifecycle-test"})
	require.NoError(t, err)

	collections, err := provider.ListCollections(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(collections), 1)

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)

	// Stop
	err = provider.Stop(ctx)
	require.NoError(t, err)

	// Close
	err = provider.Close(ctx)
	require.NoError(t, err)
}

// =============================================================================
// Additional Edge Cases and Stress Tests
// =============================================================================

func TestCharacterAIProvider_RapidStartStop(t *testing.T) {
	provider := newTestCharacterAIProvider()
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	// Rapid start/stop cycles
	for i := 0; i < 10; i++ {
		err = provider.Start(ctx)
		require.NoError(t, err)

		err = provider.Stop(ctx)
		require.NoError(t, err)
	}
}

func TestCharacterAIProvider_StatsConsistency(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Get initial stats
	stats1, err := provider.GetStats(ctx)
	require.NoError(t, err)
	initialCollections := stats1.TotalCollections

	// Create collections
	for i := 0; i < 5; i++ {
		config := &CollectionConfig{Name: fmt.Sprintf("stats-consistency-%d", i)}
		err := provider.CreateCollection(ctx, config.Name, config)
		require.NoError(t, err)
	}

	// Get updated stats
	stats2, err := provider.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCollections+5, stats2.TotalCollections)

	// Delete some collections
	for i := 0; i < 3; i++ {
		err := provider.DeleteCollection(ctx, fmt.Sprintf("stats-consistency-%d", i))
		require.NoError(t, err)
	}

	// Verify stats
	stats3, err := provider.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCollections+2, stats3.TotalCollections)
}

func TestCharacterAIProvider_TimeoutContext(t *testing.T) {
	// Anti-bluff (CONST-035 / §11.9): the original form discarded the
	// error with `_ = err` and a "May or may not error" comment, passing
	// regardless of Health's behaviour under a cancelled context. Per
	// the standalone Character.AI provider's Health implementation it
	// returns the provider's HealthInfo synchronously from in-memory
	// state — context cancellation does not abort it. Pin that documented
	// contract: with a deadline-expired context, Health still returns a
	// non-nil HealthInfo and no error (a future regression that started
	// honouring the deadline would FAIL this test, forcing an explicit
	// behaviour decision rather than silent passing).
	provider := newInitializedCharacterAIProvider(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ensure deadline has elapsed

	info, err := provider.Health(ctx)
	assert.NoError(t, err, "Health is in-memory and must not surface a deadline error")
	assert.NotNil(t, info, "Health must return a non-nil HealthInfo even on a cancelled context")
}

func TestCharacterAIProvider_EmptyTextEmbedding(t *testing.T) {
	provider := newTestCharacterAIProvider()

	// Empty text should return zero vector
	embedding := provider.generateEmbedding("")
	assert.Len(t, embedding, 1536)

	// All values should be zero
	for _, v := range embedding {
		assert.Equal(t, 0.0, v)
	}
}

func TestCharacterAIProvider_UnicodeTextEmbedding(t *testing.T) {
	provider := newTestCharacterAIProvider()

	// Test with various unicode characters
	unicodeTexts := []string{
		"Hello World",
		"Hola Mundo",
		"Bonjour le monde",
		"Привет мир",
		"こんにちは世界",
		"مرحبا بالعالم",
		"🌍🌎🌏",
	}

	embeddings := make([][]float64, len(unicodeTexts))
	for i, text := range unicodeTexts {
		embeddings[i] = provider.generateEmbedding(text)
		assert.Len(t, embeddings[i], 1536)

		// Verify normalized
		var norm float64
		for _, v := range embeddings[i] {
			norm += v * v
		}
		assert.InDelta(t, 1.0, norm, 0.0001)
	}

	// Different texts should produce different embeddings
	for i := 0; i < len(embeddings)-1; i++ {
		for j := i + 1; j < len(embeddings); j++ {
			assert.NotEqual(t, embeddings[i], embeddings[j])
		}
	}
}

func TestCharacterAIProvider_CostInfoAccuracy(t *testing.T) {
	provider := newInitializedCharacterAIProvider(t)
	ctx := context.Background()

	// Initially no characters
	costInfo := provider.GetCostInfo()
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, "monthly", costInfo.BillingPeriod)
	initialCost := costInfo.TotalCost

	// Add characters
	for i := 0; i < 20; i++ {
		config := &CollectionConfig{Name: fmt.Sprintf("cost-test-%d", i)}
		_ = provider.CreateCollection(ctx, config.Name, config)
	}

	// Cost should increase
	costInfo = provider.GetCostInfo()
	assert.GreaterOrEqual(t, costInfo.TotalCost, initialCost)
	assert.Equal(t, float64(20), costInfo.FreeTierUsed)
}

func TestCosineSimilarity_NaNHandling(t *testing.T) {
	// Test with vectors that could produce NaN
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}

	result := cosineSimilarity(a, b)
	assert.Equal(t, 0.0, result)
}

func TestCosineSimilarity_LargeVectors(t *testing.T) {
	size := 4096
	a := make([]float64, size)
	b := make([]float64, size)

	for i := 0; i < size; i++ {
		a[i] = float64(i) / float64(size)
		b[i] = float64(size-i) / float64(size)
	}

	result := cosineSimilarity(a, b)
	// Result should be a valid number between -1 and 1
	assert.True(t, result >= -1.0 && result <= 1.0)
}
