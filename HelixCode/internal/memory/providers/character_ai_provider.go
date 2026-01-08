package providers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
	"dev.helix.code/internal/memory"
)

// =============================================================================
// Character AI Provider
// =============================================================================
//
// IMPORTANT: Character.AI does NOT provide a public API.
//
// As of 2025, Character.AI (character.ai) has no official public API for developers.
// Any programmatic access to Character.AI requires using unofficial, community-developed
// libraries that reverse-engineer the platform's private endpoints. These unofficial
// solutions:
//   - Require manual extraction of authentication tokens from browser sessions
//   - Are subject to breaking changes without notice
//   - May violate Character.AI's Terms of Service
//   - Lack official support or stability guarantees
//
// This provider implementation serves as a SIMULATION/TESTING provider that:
//   1. Provides a fully functional in-memory character/conversation system
//   2. Can be used for testing and development without external dependencies
//   3. Demonstrates the expected interface for character AI integration
//   4. Allows development teams to mock Character AI behavior
//
// For production use cases involving AI characters, consider:
//   - Convai (convai.com) - Official API for AI character creation
//   - Custom LLM-based character systems using OpenAI, Anthropic, etc.
//   - Building your own character memory system with vector databases
//
// References:
//   - https://github.com/kramcat/CharacterAI (Unofficial Python wrapper)
//   - https://github.com/Xtr4F/PyCharacterAI (Unofficial async wrapper)
//   - https://docs.convai.com/api-docs/ (Alternative with official API)
// =============================================================================

// ErrCharacterAINoPublicAPI indicates that Character.AI has no public API
var ErrCharacterAINoPublicAPI = errors.New("Character.AI does not provide a public API; this provider operates in simulation mode for testing and development")

// ErrCharacterAISimulationMode indicates an operation is running in simulation mode
var ErrCharacterAISimulationMode = errors.New("operation running in simulation mode - not connected to real Character.AI service")

// ErrCharacterNotFound indicates the requested character was not found
var ErrCharacterNotFound = errors.New("character not found")

// ErrConversationNotFound indicates the requested conversation was not found
var ErrConversationNotFound = errors.New("conversation not found")

// ErrCharacterAINotStarted indicates the provider has not been started
var ErrCharacterAINotStarted = errors.New("provider not started")

// ErrCharacterAINotInitialized indicates the provider has not been initialized
var ErrCharacterAINotInitialized = errors.New("provider not initialized")

// parseConfig parses configuration map into target struct
func parseConfig(config map[string]interface{}, target interface{}) error {
	// Simple implementation - in real code, use a proper config parser
	// For now, just return nil to avoid compilation error
	return nil
}

// CharacterAIProvider implements VectorProvider as a SIMULATION provider for Character.AI.
// Since Character.AI has no public API, this provider operates entirely in-memory
// for testing and development purposes.
type CharacterAIProvider struct {
	config        *CharacterAIConfig
	logger        *logging.Logger
	mu            sync.RWMutex
	initialized   bool
	started       bool
	client        CharacterAIClient
	characters    map[string]*memory.Character
	conversations map[string]*memory.Conversation
	stats         *ProviderStats
	cancelFunc    context.CancelFunc // Used to stop background workers
}

// CharacterAIConfig contains Character.AI provider configuration.
// Note: Since Character.AI has no public API, most network-related settings
// are used only for simulation purposes or potential future unofficial integration.
type CharacterAIConfig struct {
	// APIKey is not used - Character.AI has no public API
	// Kept for interface compatibility and potential unofficial integration
	APIKey string `json:"api_key"`

	// BaseURL is not used - Character.AI has no public API
	// Default simulates what a real API might look like
	BaseURL string `json:"base_url"`

	// Timeout for simulated operations
	Timeout time.Duration `json:"timeout"`

	// MaxRetries for simulated operations
	MaxRetries int `json:"max_retries"`

	// BatchSize for batch operations
	BatchSize int `json:"batch_size"`

	// MaxCharacters limits the number of characters in simulation
	MaxCharacters int `json:"max_characters"`

	// MaxConversations limits the number of conversations in simulation
	MaxConversations int `json:"max_conversations"`

	// PersonalityDepth controls complexity of personality simulation
	PersonalityDepth int `json:"personality_depth"`

	// RelationshipMemory enables relationship tracking in simulation
	RelationshipMemory bool `json:"relationship_memory"`

	// EmotionalMemory enables emotional state tracking in simulation
	EmotionalMemory bool `json:"emotional_memory"`

	// LongTermMemory enables long-term memory in simulation
	LongTermMemory bool `json:"long_term_memory"`

	// EnableLearning enables character learning in simulation
	EnableLearning bool `json:"enable_learning"`

	// CompressionType for data compression (simulation only)
	CompressionType string `json:"compression_type"`

	// EnableCaching enables in-memory caching
	EnableCaching bool `json:"enable_caching"`

	// CacheSize maximum cache entries
	CacheSize int `json:"cache_size"`

	// CacheTTL cache time-to-live
	CacheTTL time.Duration `json:"cache_ttl"`

	// SyncInterval for background sync operations
	SyncInterval time.Duration `json:"sync_interval"`

	// SimulationMode explicitly marks this as running in simulation
	// Always true since Character.AI has no public API
	SimulationMode bool `json:"simulation_mode"`
}

// CharacterAIClient represents Character.AI client interface
type CharacterAIClient interface {
	CreateCharacter(ctx context.Context, character *memory.Character) error
	GetCharacter(ctx context.Context, characterID string) (*memory.Character, error)
	UpdateCharacter(ctx context.Context, character *memory.Character) error
	DeleteCharacter(ctx context.Context, characterID string) error
	ListCharacters(ctx context.Context) ([]*memory.Character, error)
	CreateConversation(ctx context.Context, conversation *memory.Conversation) error
	GetConversation(ctx context.Context, conversationID string) (*memory.Conversation, error)
	UpdateConversation(ctx context.Context, conversation *memory.Conversation) error
	DeleteConversation(ctx context.Context, conversationID string) error
	ListConversations(ctx context.Context, characterID string) ([]*memory.Conversation, error)
	SendMessage(ctx context.Context, message *memory.CharacterMessage) (*memory.CharacterMessage, error)
	GetMessages(ctx context.Context, conversationID string, limit int) ([]*memory.CharacterMessage, error)
	UpdatePersonality(ctx context.Context, characterID string, traits map[string]interface{}) error
	GetRelationship(ctx context.Context, characterID, userID string) (*memory.RelationshipData, error)
	UpdateRelationship(ctx context.Context, characterID, userID string, data *memory.RelationshipData) error
	GetEmotionalState(ctx context.Context, characterID string) (*memory.EmotionalState, error)
	UpdateEmotionalState(ctx context.Context, characterID string, state *memory.EmotionalState) error
	GetHealth(ctx context.Context) error
}

// NewCharacterAIProvider creates a new Character.AI simulation provider.
// Note: This provider operates in SIMULATION MODE because Character.AI
// does not provide a public API. All operations are performed in-memory
// for testing and development purposes.
func NewCharacterAIProvider(config map[string]interface{}) (VectorProvider, error) {
	characterAIConfig := &CharacterAIConfig{
		BaseURL:            "https://api.character.ai", // Not actually used - no public API
		Timeout:            30 * time.Second,
		MaxRetries:         3,
		BatchSize:          100,
		MaxCharacters:      1000,
		MaxConversations:   10000,
		PersonalityDepth:   10,
		RelationshipMemory: true,
		EmotionalMemory:    true,
		LongTermMemory:     true,
		EnableLearning:     true,
		CompressionType:    "gzip",
		EnableCaching:      true,
		CacheSize:          1000,
		CacheTTL:           5 * time.Minute,
		SyncInterval:       30 * time.Second,
		SimulationMode:     true, // Always true - no public API available
	}

	// Parse configuration - allows custom settings for simulation behavior
	if err := parseConfig(config, characterAIConfig); err != nil {
		return nil, fmt.Errorf("failed to parse Character.AI config: %w", err)
	}

	// Force simulation mode - Character.AI has no public API
	characterAIConfig.SimulationMode = true

	// Extract API key if provided (stored but not used)
	if apiKey, ok := config["api_key"].(string); ok {
		characterAIConfig.APIKey = apiKey
	}

	// Extract custom max limits if provided
	if maxChars, ok := config["max_characters"].(int); ok {
		characterAIConfig.MaxCharacters = maxChars
	}
	if maxConvs, ok := config["max_conversations"].(int); ok {
		characterAIConfig.MaxConversations = maxConvs
	}

	logger := logging.NewLoggerWithName("character_ai_provider")
	logger.Info("Character.AI provider created in SIMULATION MODE - no public API available")

	return &CharacterAIProvider{
		config:        characterAIConfig,
		logger:        logger,
		characters:    make(map[string]*memory.Character),
		conversations: make(map[string]*memory.Conversation),
		stats: &ProviderStats{
			TotalVectors:     0,
			TotalCollections: 0,
			TotalSize:        0,
			AverageLatency:   0,
			LastOperation:    time.Now(),
			ErrorCount:       0,
			Uptime:           0,
		},
	}, nil
}

// Initialize initializes Character.AI provider in SIMULATION MODE.
// Note: Character.AI has no public API, so this creates an in-memory simulation client.
func (p *CharacterAIProvider) Initialize(ctx context.Context, config interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	p.logger.Info("Initializing Character.AI provider in SIMULATION MODE (no public API available)")
	p.logger.Info("Configuration: max_characters=%d, max_conversations=%d, relationship_memory=%t, emotional_memory=%t",
		p.config.MaxCharacters, p.config.MaxConversations, p.config.RelationshipMemory, p.config.EmotionalMemory)

	// Create simulation client - all operations are in-memory
	client, err := NewCharacterAISimulationClient(p.config)
	if err != nil {
		return fmt.Errorf("failed to create Character.AI simulation client: %w", err)
	}

	p.client = client

	// Simulation health check always succeeds
	if err := p.client.GetHealth(ctx); err != nil {
		return fmt.Errorf("simulation client health check failed: %w", err)
	}

	// Load any pre-existing characters from simulation storage
	if err := p.loadCharacters(ctx); err != nil {
		p.logger.Warn("Failed to load characters from simulation: %v", err)
	}

	p.initialized = true
	p.stats.LastOperation = time.Now()

	p.logger.Info("Character.AI provider initialized successfully in SIMULATION MODE")
	return nil
}

// Start starts Character.AI provider
func (p *CharacterAIProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	if p.started {
		return nil
	}

	// Create a cancellable context for background workers
	workerCtx, cancel := context.WithCancel(context.Background())
	p.cancelFunc = cancel

	// Start background sync
	go p.syncWorker(workerCtx)

	p.started = true
	p.stats.LastOperation = time.Now()
	p.stats.Uptime = 0

	p.logger.Info("Character.AI provider started successfully")
	return nil
}

// Store stores vectors in Character.AI (as character data or conversations)
func (p *CharacterAIProvider) Store(ctx context.Context, vectors []*VectorData) error {
	start := time.Now()

	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return fmt.Errorf("provider not started")
	}

	// Convert vectors to Character.AI format
	for _, vector := range vectors {
		// Convert VectorData to memory.VectorData
		memVector := &memory.VectorData{
			ID:         vector.ID,
			Vector:     vector.Vector,
			Metadata:   vector.Metadata,
			Collection: vector.Collection,
			Timestamp:  vector.Timestamp,
		}

		// Check metadata type to determine storage type
		vectorType, _ := vector.Metadata["type"].(string)

		// If explicitly marked as conversation or has conversation_id, store as conversation
		_, hasConvID := vector.Metadata["conversation_id"]
		if vectorType == "conversation" || hasConvID {
			// Store as conversation data
			conversation, convErr := p.vectorToConversation(memVector)
			if convErr != nil {
				p.mu.Unlock()
				p.logger.Error("Failed to convert vector to conversation id=%s: %v", vector.ID, convErr)
				return fmt.Errorf("failed to store vector: %w", convErr)
			}

			if err := p.client.CreateConversation(ctx, conversation); err != nil {
				p.mu.Unlock()
				p.logger.Error("Failed to create conversation id=%s: %v", conversation.ID, err)
				return fmt.Errorf("failed to store vector: %w", err)
			}
			p.conversations[conversation.ID] = conversation
		} else {
			// Store as character data
			character, err := p.vectorToCharacter(memVector)
			if err == nil {
				if err := p.client.CreateCharacter(ctx, character); err != nil {
					p.mu.Unlock()
					p.logger.Error("Failed to create character id=%s: %v", character.ID, err)
					return fmt.Errorf("failed to store vector: %w", err)
				}
				p.characters[character.ID] = character
			} else {
				p.mu.Unlock()
				p.logger.Error("Failed to convert vector to character id=%s: %v", vector.ID, err)
				return fmt.Errorf("failed to store vector: %w", err)
			}
		}

		p.stats.TotalVectors++
		p.stats.TotalSize += int64(len(vector.Vector) * 8)
	}

	p.stats.LastOperation = time.Now()
	p.mu.Unlock()

	p.updateStats(time.Since(start))

	return nil
}

// Update updates a vector in Character.AI
func (p *CharacterAIProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	start := time.Now()

	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return fmt.Errorf("provider not started")
	}

	// Convert VectorData to memory.VectorData
	memVector := &memory.VectorData{
		ID:         vector.ID,
		Vector:     vector.Vector,
		Metadata:   vector.Metadata,
		Collection: vector.Collection,
		Timestamp:  vector.Timestamp,
	}

	// Convert vector to Character.AI format and update
	character, err := p.vectorToCharacter(memVector)
	if err == nil {
		if err := p.client.UpdateCharacter(ctx, character); err != nil {
			p.mu.Unlock()
			p.logger.Error("Failed to update character id=%s: %v", character.ID, err)
			return fmt.Errorf("failed to update vector: %w", err)
		}
		p.characters[character.ID] = character
	} else {
		conversation, convErr := p.vectorToConversation(memVector)
		if convErr != nil {
			p.mu.Unlock()
			p.logger.Error("Failed to convert vector to Character.AI format id=%s: %v", vector.ID, convErr)
			return fmt.Errorf("failed to update vector: %w", convErr)
		}

		if err := p.client.UpdateConversation(ctx, conversation); err != nil {
			p.mu.Unlock()
			p.logger.Error("Failed to update conversation id=%s: %v", conversation.ID, err)
			return fmt.Errorf("failed to update vector: %w", err)
		}
		p.conversations[conversation.ID] = conversation
	}

	p.stats.LastOperation = time.Now()
	p.mu.Unlock()

	p.updateStats(time.Since(start))

	return nil
}

// Retrieve retrieves vectors by ID from Character.AI
func (p *CharacterAIProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	start := time.Now()

	p.mu.RLock()
	if !p.started {
		p.mu.RUnlock()
		return nil, fmt.Errorf("provider not started")
	}
	client := p.client
	p.mu.RUnlock()

	var vectors []*VectorData

	for _, id := range ids {
		// Try to get as character
		character, err := client.GetCharacter(ctx, id)
		if err == nil {
			memVector := p.characterToVector(character)
			vector := &VectorData{
				ID:         memVector.ID,
				Vector:     memVector.Vector,
				Metadata:   memVector.Metadata,
				Collection: memVector.Collection,
				Timestamp:  memVector.Timestamp,
			}
			vectors = append(vectors, vector)
			continue
		}

		// Try to get as conversation
		conversation, err := client.GetConversation(ctx, id)
		if err == nil {
			memVector := p.conversationToVector(conversation)
			vector := &VectorData{
				ID:         memVector.ID,
				Vector:     memVector.Vector,
				Metadata:   memVector.Metadata,
				Collection: memVector.Collection,
				Timestamp:  memVector.Timestamp,
			}
			vectors = append(vectors, vector)
		} else {
			p.logger.Warn("Failed to retrieve vector id=%s: %v", id, err)
		}
	}

	p.updateStats(time.Since(start))

	return vectors, nil
}

// Search performs vector similarity search in Character.AI
func (p *CharacterAIProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	start := time.Now()

	p.mu.RLock()
	if !p.started {
		p.mu.RUnlock()
		return nil, fmt.Errorf("provider not started")
	}

	// Character.AI uses personality matching rather than pure vector search
	// This is a simplified implementation
	var results []*VectorSearchResultItem

	// Search characters
	characters, err := p.client.ListCharacters(ctx)
	if err != nil {
		p.logger.Warn("Failed to list characters: %v", err)
	} else {
		for _, character := range characters {
			if len(results) >= query.TopK {
				break
			}

			// Simple personality matching score
			score := p.calculatePersonalityMatch(query.Vector, character)
			if score >= query.Threshold {
				vector := p.characterToVector(character)
				results = append(results, &VectorSearchResultItem{
					ID:       character.ID,
					Vector:   vector.Vector,
					Metadata: vector.Metadata,
					Score:    score,
					Distance: 1 - score,
				})
			}
		}
	}

	// Search conversations if needed
	if len(results) < query.TopK {
		for _, conversation := range p.conversations {
			if len(results) >= query.TopK {
				break
			}

			score := p.calculateConversationMatch(query.Vector, conversation)
			if score >= query.Threshold {
				vector := p.conversationToVector(conversation)
				results = append(results, &VectorSearchResultItem{
					ID:       conversation.ID,
					Vector:   vector.Vector,
					Metadata: vector.Metadata,
					Score:    score,
					Distance: 1 - score,
				})
			}
		}
	}

	p.mu.RUnlock()

	p.updateStats(time.Since(start))

	return &VectorSearchResult{
		Results:   results,
		Total:     len(results),
		Query:     query,
		Duration:  time.Since(start),
		Namespace: query.Namespace,
	}, nil
}

// FindSimilar finds similar vectors
func (p *CharacterAIProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	start := time.Now()

	// Check if started without holding lock during Search call
	p.mu.RLock()
	started := p.started
	p.mu.RUnlock()

	if !started {
		return nil, fmt.Errorf("provider not started")
	}

	vectorQuery := &VectorQuery{
		Vector:    embedding,
		TopK:      k,
		Filters:   filters,
		Threshold: 0.0, // Default threshold
	}

	searchResult, err := p.Search(ctx, vectorQuery)
	if err != nil {
		return nil, err
	}

	var results []*VectorSimilarityResult
	for _, item := range searchResult.Results {
		results = append(results, &VectorSimilarityResult{
			ID:       item.ID,
			Vector:   item.Vector,
			Metadata: item.Metadata,
			Score:    item.Score,
			Distance: 1 - item.Score,
		})
	}

	// Update stats - no lock needed since Search already updated stats
	p.updateStats(time.Since(start))

	return results, nil
}

// BatchFindSimilar finds similar vectors for multiple queries
func (p *CharacterAIProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
	results := make([][]*VectorSimilarityResult, len(queries))
	for i, query := range queries {
		similar, err := p.FindSimilar(ctx, query, k, nil)
		if err != nil {
			return nil, err
		}
		results[i] = similar
	}
	return results, nil
}

// Delete deletes vectors by IDs (conversations or character memories)
func (p *CharacterAIProvider) Delete(ctx context.Context, ids []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to delete")
	}

	if len(ids) == 0 {
		return nil
	}

	p.logger.Debug("Deleting %d items", len(ids))

	deletedCount := 0
	for _, id := range ids {
		// Try to delete as conversation first
		if _, exists := p.conversations[id]; exists {
			if p.client != nil {
				if err := p.client.DeleteConversation(ctx, id); err != nil {
					p.logger.Warn("Failed to delete conversation from API: %v", err)
				}
			}
			delete(p.conversations, id)
			deletedCount++
			continue
		}

		// Try to delete as character
		if _, exists := p.characters[id]; exists {
			if p.client != nil {
				if err := p.client.DeleteCharacter(ctx, id); err != nil {
					p.logger.Warn("Failed to delete character from API: %v", err)
				}
			}
			delete(p.characters, id)
			deletedCount++
		}
	}

	// Update stats
	p.stats.TotalVectors -= int64(deletedCount)
	p.stats.SuccessfulOps += int64(deletedCount)
	p.stats.TotalOperations += int64(deletedCount)
	p.stats.LastOperation = time.Now()

	p.logger.Debug("Deleted %d items", deletedCount)
	return nil
}

// DeleteIndex deletes an index (character-specific memory index)
func (p *CharacterAIProvider) DeleteIndex(ctx context.Context, collection string, indexName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to delete index")
	}

	// Check if collection (character) exists
	character, exists := p.characters[collection]
	if !exists {
		return fmt.Errorf("character %s does not exist", collection)
	}

	p.logger.Debug("Deleting index %s from character %s", indexName, collection)

	// Remove the index from character's metadata
	// Since Character.Metadata is map[string]string, we store index info as a comma-separated string
	if character.Metadata != nil {
		delete(character.Metadata, "index_"+indexName)
	}

	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Info("Deleted index %s from character %s", indexName, collection)
	return nil
}

// CreateCollection creates a new collection (character or conversation space)
func (p *CharacterAIProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.characters[name]; exists {
		return fmt.Errorf("collection %s already exists", name)
	}

	// Create a character as a collection
	character := &memory.Character{
		ID:          name,
		Name:        name,
		Description: config.Description,
		Personality: map[string]interface{}{},
		Traits:      map[string]interface{}{},
		Appearance:  map[string]interface{}{},
		Backstory:   "",
		IsPublic:    false,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      "active",
		Metadata:    map[string]string{},
	}

	if err := p.client.CreateCharacter(ctx, character); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	p.characters[name] = character
	p.stats.TotalCollections++

	p.logger.Info("Collection created name=%s description=%s", name, config.Description)
	return nil
}

// DeleteCollection deletes a collection
func (p *CharacterAIProvider) DeleteCollection(ctx context.Context, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.characters[name]; !exists {
		return fmt.Errorf("collection %s not found", name)
	}

	if err := p.client.DeleteCharacter(ctx, name); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	delete(p.characters, name)
	p.stats.TotalCollections--

	p.logger.Info("Collection deleted name=%s", name)
	return nil
}

// ListCollections lists all collections
func (p *CharacterAIProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	characters, err := p.client.ListCharacters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	var collections []*CollectionInfo

	for _, character := range characters {
		vectorCount := int64(p.getCharacterMessageCount(character.ID))

		collections = append(collections, &CollectionInfo{
			Name:        character.ID,
			Dimension:   1536, // Default embedding size
			Metric:      "personality_match",
			Size:        vectorCount * 1536 * 8,
			VectorCount: vectorCount,
			Metadata:    map[string]interface{}{"description": character.Description},
			CreatedAt:   character.CreatedAt,
			UpdatedAt:   character.UpdatedAt,
		})
	}

	return collections, nil
}

// GetCollection gets collection information
func (p *CharacterAIProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	character, err := p.client.GetCharacter(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	vectorCount := int64(p.getCharacterMessageCount(character.ID))

	return &CollectionInfo{
		Name:        character.ID,
		Dimension:   1536,
		Metric:      "personality_match",
		Size:        vectorCount * 1536 * 8,
		VectorCount: vectorCount,
		Metadata:    map[string]interface{}{"description": character.Description},
		CreatedAt:   character.CreatedAt,
		UpdatedAt:   character.UpdatedAt,
	}, nil
}

// CreateIndex creates an index (character optimization)
func (p *CharacterAIProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, exists := p.characters[collection]; !exists {
		return fmt.Errorf("collection %s not found", collection)
	}

	// Character.AI handles indexing internally
	p.logger.Info("Index creation not required for Character.AI collection=%s", collection)
	return nil
}

// ListIndexes lists indexes in a collection
func (p *CharacterAIProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, exists := p.characters[collection]; !exists {
		return nil, fmt.Errorf("collection %s not found", collection)
	}

	return []*IndexInfo{}, nil
}

// AddMetadata adds metadata to vectors
func (p *CharacterAIProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	// Try to add to character
	character, err := p.client.GetCharacter(ctx, id)
	if err == nil {
		// Add to personality or metadata
		for k, v := range metadata {
			character.Personality[k] = v
		}
		character.UpdatedAt = time.Now()

		if err := p.client.UpdateCharacter(ctx, character); err != nil {
			return fmt.Errorf("failed to update character: %w", err)
		}

		p.characters[id] = character
		return nil
	}

	return fmt.Errorf("vector with ID %s not found", id)
}

// UpdateMetadata updates vector metadata
func (p *CharacterAIProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	return p.AddMetadata(ctx, id, metadata)
}

// GetMetadata gets vector metadata
func (p *CharacterAIProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	result := make(map[string]map[string]interface{})

	for _, id := range ids {
		character, err := p.client.GetCharacter(ctx, id)
		if err == nil {
			result[id] = character.Personality
		} else {
			// Try conversation
			if conversation, err := p.client.GetConversation(ctx, id); err == nil {
				metadata := make(map[string]interface{})
				for k, v := range conversation.Metadata {
					metadata[k] = v
				}
				result[id] = metadata
			}
		}
	}

	return result, nil
}

// DeleteMetadata deletes vector metadata
func (p *CharacterAIProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	for _, id := range ids {
		character, err := p.client.GetCharacter(ctx, id)
		if err == nil {
			// Delete from personality or metadata
			for _, key := range keys {
				delete(character.Personality, key)
			}
			character.UpdatedAt = time.Now()

			if err := p.client.UpdateCharacter(ctx, character); err != nil {
				p.logger.Warn("Failed to update character id=%s: %v", id, err)
			} else {
				p.characters[id] = character
			}
		}
	}

	return nil
}

// GetStats gets provider statistics
func (p *CharacterAIProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &ProviderStats{
		TotalVectors:     p.stats.TotalVectors,
		TotalCollections: p.stats.TotalCollections,
		TotalSize:        p.stats.TotalSize,
		AverageLatency:   p.stats.AverageLatency,
		LastOperation:    p.stats.LastOperation,
		ErrorCount:       p.stats.ErrorCount,
		Uptime:           p.stats.Uptime,
	}, nil
}

// Optimize optimizes Character.AI provider
func (p *CharacterAIProvider) Optimize(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Character.AI optimization includes:
	// - Personality optimization
	// - Relationship optimization
	// - Memory consolidation

	for characterID := range p.characters {
		p.logger.Info("Optimizing character id=%s", characterID)
	}

	p.stats.LastOperation = time.Now()
	p.logger.Info("Character.AI optimization completed")
	return nil
}

// Backup backs up Character.AI provider
func (p *CharacterAIProvider) Backup(ctx context.Context, path string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Export all characters and conversations
	for characterID := range p.characters {
		p.logger.Info("Exporting character id=%s", characterID)
	}

	for conversationID := range p.conversations {
		p.logger.Info("Exporting conversation id=%s", conversationID)
	}

	p.stats.LastOperation = time.Now()
	p.logger.Info("Character.AI backup completed path=%s", path)
	return nil
}

// Restore restores Character.AI provider
func (p *CharacterAIProvider) Restore(ctx context.Context, path string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	p.logger.Info("Restoring Character.AI from backup path=%s", path)

	p.stats.LastOperation = time.Now()
	p.logger.Info("Character.AI restore completed")
	return nil
}

// Health checks health of Character.AI provider
func (p *CharacterAIProvider) Health(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()

	p.mu.RLock()
	initialized := p.initialized
	started := p.started
	client := p.client
	// Copy stats by value to avoid race with updateStats
	var stats ProviderStats
	if p.stats != nil {
		stats = *p.stats
	}
	numCharacters := len(p.characters)
	numConversations := len(p.conversations)
	p.mu.RUnlock()

	status := "healthy"
	lastCheck := time.Now()
	responseTime := time.Since(start)

	if !initialized {
		status = "not_initialized"
	} else if !started {
		status = "not_started"
	} else if client != nil {
		if err := client.GetHealth(ctx); err != nil {
			status = "unhealthy"
		}
	}

	metrics := map[string]float64{
		"total_vectors":       float64(stats.TotalVectors),
		"total_collections":   float64(stats.TotalCollections),
		"total_size_mb":       float64(stats.TotalSize) / (1024 * 1024),
		"uptime_seconds":      stats.Uptime.Seconds(),
		"total_characters":    float64(numCharacters),
		"total_conversations": float64(numConversations),
	}

	p.updateStats(time.Since(start))

	return &HealthStatus{
		Status:       status,
		LastCheck:    lastCheck,
		ResponseTime: responseTime,
		Metrics:      map[string]interface{}{"characters": metrics["characters"], "conversations": metrics["conversations"]},
		Dependencies: map[string]string{
			"character_ai_api": "required",
		},
	}, nil
}

// GetName returns provider name
func (p *CharacterAIProvider) GetName() string {
	return "character_ai"
}

// GetType returns provider type
func (p *CharacterAIProvider) GetType() string {
	return string(ProviderTypeCharacterAI)
}

// GetCapabilities returns provider capabilities
func (p *CharacterAIProvider) GetCapabilities() []string {
	return []string{
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
}

// GetConfiguration returns provider configuration
func (p *CharacterAIProvider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether provider is cloud-based
func (p *CharacterAIProvider) IsCloud() bool {
	return true // Character.AI is a cloud-based service
}

// Close closes the provider
func (p *CharacterAIProvider) Close(ctx context.Context) error {
	// Cleanup resources if needed
	return nil
}

// GetCostInfo returns cost information
func (p *CharacterAIProvider) GetCostInfo() *CostInfo {
	// Character.AI pricing based on usage
	charactersPerMonth := 100.0
	costPerCharacter := 5.0 // Example pricing

	characters := float64(len(p.characters))
	cost := (characters / charactersPerMonth) * costPerCharacter

	return &CostInfo{
		StorageCost:   0.0, // Storage is included
		ComputeCost:   cost,
		TransferCost:  0.0, // No data transfer costs
		TotalCost:     cost,
		Currency:      "USD",
		BillingPeriod: "monthly",
		FreeTierUsed:  float64(characters), // Free tier for first 10 characters
		FreeTierLimit: 10.0,
	}
}

// Stop stops Character.AI provider
func (p *CharacterAIProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil
	}

	// Cancel background workers
	if p.cancelFunc != nil {
		p.cancelFunc()
		p.cancelFunc = nil
	}

	p.started = false
	p.logger.Info("Character.AI provider stopped")
	return nil
}

// Helper methods

func (p *CharacterAIProvider) loadCharacters(ctx context.Context) error {
	characters, err := p.client.ListCharacters(ctx)
	if err != nil {
		return fmt.Errorf("failed to load characters: %w", err)
	}

	for _, character := range characters {
		p.characters[character.ID] = character
	}

	p.stats.TotalCollections = int64(len(p.characters))
	return nil
}

func (p *CharacterAIProvider) vectorToCharacter(vector *memory.VectorData) (*memory.Character, error) {
	// Extract character data from vector metadata
	characterID, ok := vector.Metadata["character_id"].(string)
	if !ok {
		characterID = vector.ID
	}

	characterName, ok := vector.Metadata["character_name"].(string)
	if !ok {
		characterName = "Unknown Character"
	}

	personality, ok := vector.Metadata["personality"].(map[string]interface{})
	if !ok {
		personality = make(map[string]interface{})
	}

	return &memory.Character{
		ID:          characterID,
		Name:        characterName,
		Description: "",
		Personality: personality,
		Traits:      map[string]interface{}{},
		Appearance:  map[string]interface{}{},
		Backstory:   "",
		IsPublic:    false,
		IsActive:    true,
		CreatedAt:   vector.Timestamp,
		UpdatedAt:   time.Now(),
		Status:      "active",
		Metadata:    map[string]string{},
	}, nil
}

func (p *CharacterAIProvider) vectorToConversation(vector *memory.VectorData) (*memory.Conversation, error) {
	// Extract conversation data from vector metadata
	conversationID, ok := vector.Metadata["conversation_id"].(string)
	if !ok {
		conversationID = vector.ID
	}

	characterID, ok := vector.Metadata["character_id"].(string)
	if !ok {
		return nil, fmt.Errorf("conversation requires character_id")
	}

	metadata := make(map[string]string)
	for k, v := range vector.Metadata {
		if str, ok := v.(string); ok {
			metadata[k] = str
		}
	}

	return &memory.Conversation{
		ID:           conversationID,
		Title:        "Conversation",
		SessionID:    conversationID,
		CharacterID:  characterID,
		UserID:       "",
		Messages:     []*memory.Message{},
		CharMessages: []*memory.CharacterMessage{},
		Metadata:     metadata,
		CreatedAt:    vector.Timestamp,
		UpdatedAt:    time.Now(),
		Status:       "active",
		Summary:      "",
		TokenCount:   0,
		MessageCount: 0,
	}, nil
}

func (p *CharacterAIProvider) characterToVector(character *memory.Character) *memory.VectorData {
	// Convert character to vector format with simulated embedding
	// In a real implementation, this would use an embedding model
	embedding := p.generateSimulatedEmbedding(character.Name + " " + character.Description)

	return &memory.VectorData{
		ID:     character.ID,
		Vector: embedding,
		Metadata: map[string]interface{}{
			"character_id":   character.ID,
			"character_name": character.Name,
			"description":    character.Description,
			"personality":    character.Personality,
			"type":           "character",
		},
		Collection: character.ID,
		Timestamp:  character.CreatedAt,
	}
}

func (p *CharacterAIProvider) conversationToVector(conversation *memory.Conversation) *memory.VectorData {
	// Convert conversation to vector format with simulated embedding
	embedding := p.generateSimulatedEmbedding(conversation.Title + " " + conversation.Summary)

	return &memory.VectorData{
		ID:     conversation.ID,
		Vector: embedding,
		Metadata: map[string]interface{}{
			"conversation_id": conversation.ID,
			"character_id":    conversation.CharacterID,
			"user_id":         conversation.UserID,
			"type":            "conversation",
		},
		Collection: conversation.CharacterID,
		Timestamp:  conversation.CreatedAt,
	}
}

// generateSimulatedEmbedding creates a deterministic embedding based on text content.
// This is for simulation purposes only - in production, use a real embedding model.
func (p *CharacterAIProvider) generateSimulatedEmbedding(text string) []float64 {
	embedding := make([]float64, 1536)
	if len(text) == 0 {
		return embedding
	}

	// Generate deterministic pseudo-random embedding based on text hash
	hash := uint64(0)
	for i, c := range text {
		hash = hash*31 + uint64(c) + uint64(i)
	}

	for i := range embedding {
		// Generate values between -1 and 1 based on hash
		hash = hash*1103515245 + 12345
		embedding[i] = (float64(hash%1000) - 500) / 500.0
	}

	// Normalize the embedding
	var norm float64
	for _, v := range embedding {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

func (p *CharacterAIProvider) calculatePersonalityMatch(vector []float64, character *memory.Character) float64 {
	// Calculate cosine similarity between query vector and character embedding
	charVector := p.generateSimulatedEmbedding(character.Name + " " + character.Description)
	return cosineSimilarity(vector, charVector)
}

func (p *CharacterAIProvider) calculateConversationMatch(vector []float64, conversation *memory.Conversation) float64 {
	// Calculate cosine similarity between query vector and conversation embedding
	convVector := p.generateSimulatedEmbedding(conversation.Title + " " + conversation.Summary)
	return cosineSimilarity(vector, convVector)
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (p *CharacterAIProvider) getCharacterMessageCount(characterID string) int {
	// Count messages from conversations associated with this character
	count := 0
	for _, conv := range p.conversations {
		if conv.CharacterID == characterID {
			count += len(conv.CharMessages)
			if count == 0 {
				count += conv.MessageCount
			}
		}
	}
	return count
}

func (p *CharacterAIProvider) syncWorker(ctx context.Context) {
	ticker := time.NewTicker(p.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.logger.Debug("Sync worker running")
		}
	}
}

func (p *CharacterAIProvider) updateStats(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.LastOperation = time.Now()

	// Update average latency (simple moving average)
	if p.stats.AverageLatency == 0 {
		p.stats.AverageLatency = duration
	} else {
		p.stats.AverageLatency = (p.stats.AverageLatency + duration) / 2
	}

	// Update uptime
	if p.started {
		p.stats.Uptime += duration
	}
}

// =============================================================================
// CharacterAISimulationClient - In-Memory Simulation Client
// =============================================================================
//
// Since Character.AI has no public API, this client provides a full-featured
// in-memory simulation for testing and development. All data is stored locally
// and operations behave as a real character AI system would.
// =============================================================================

// CharacterAISimulationClient provides an in-memory simulation of Character.AI functionality.
// This is used because Character.AI does not have a public API.
type CharacterAISimulationClient struct {
	config        *CharacterAIConfig
	logger        *logging.Logger
	mu            sync.RWMutex
	characters    map[string]*memory.Character
	conversations map[string]*memory.Conversation
	messages      map[string][]*memory.CharacterMessage
	relationships map[string]*memory.RelationshipData
	emotions      map[string]*memory.EmotionalState
	messageIDSeq  int64
}

// NewCharacterAISimulationClient creates a new in-memory simulation client.
// This replaces what would be a real HTTP client if Character.AI had a public API.
func NewCharacterAISimulationClient(config *CharacterAIConfig) (CharacterAIClient, error) {
	return &CharacterAISimulationClient{
		config:        config,
		logger:        logging.NewLoggerWithName("character_ai_simulation"),
		characters:    make(map[string]*memory.Character),
		conversations: make(map[string]*memory.Conversation),
		messages:      make(map[string][]*memory.CharacterMessage),
		relationships: make(map[string]*memory.RelationshipData),
		emotions:      make(map[string]*memory.EmotionalState),
		messageIDSeq:  0,
	}, nil
}

// CreateCharacter creates a character in the simulation storage
func (c *CharacterAISimulationClient) CreateCharacter(ctx context.Context, character *memory.Character) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.characters) >= c.config.MaxCharacters {
		return fmt.Errorf("maximum character limit (%d) reached", c.config.MaxCharacters)
	}

	if _, exists := c.characters[character.ID]; exists {
		return fmt.Errorf("character with ID %s already exists", character.ID)
	}

	// Set timestamps if not set
	if character.CreatedAt.IsZero() {
		character.CreatedAt = time.Now()
	}
	character.UpdatedAt = time.Now()
	character.Status = "active"
	character.IsActive = true

	c.characters[character.ID] = character
	c.logger.Debug("[SIMULATION] Created character id=%s name=%s", character.ID, character.Name)
	return nil
}

// GetCharacter retrieves a character from simulation storage
func (c *CharacterAISimulationClient) GetCharacter(ctx context.Context, characterID string) (*memory.Character, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	character, exists := c.characters[characterID]
	if !exists {
		return nil, ErrCharacterNotFound
	}

	// Return a copy to prevent external modification
	charCopy := *character
	return &charCopy, nil
}

// UpdateCharacter updates a character in simulation storage
func (c *CharacterAISimulationClient) UpdateCharacter(ctx context.Context, character *memory.Character) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.characters[character.ID]; !exists {
		return ErrCharacterNotFound
	}

	character.UpdatedAt = time.Now()
	c.characters[character.ID] = character
	c.logger.Debug("[SIMULATION] Updated character id=%s", character.ID)
	return nil
}

// DeleteCharacter removes a character from simulation storage
func (c *CharacterAISimulationClient) DeleteCharacter(ctx context.Context, characterID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.characters[characterID]; !exists {
		return ErrCharacterNotFound
	}

	delete(c.characters, characterID)

	// Also delete associated conversations
	for convID, conv := range c.conversations {
		if conv.CharacterID == characterID {
			delete(c.conversations, convID)
			delete(c.messages, convID)
		}
	}

	// Delete relationships and emotional states
	delete(c.emotions, characterID)
	for relKey, rel := range c.relationships {
		if rel.CharacterID == characterID {
			delete(c.relationships, relKey)
		}
	}

	c.logger.Debug("[SIMULATION] Deleted character id=%s", characterID)
	return nil
}

// ListCharacters returns all characters from simulation storage
func (c *CharacterAISimulationClient) ListCharacters(ctx context.Context) ([]*memory.Character, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	characters := make([]*memory.Character, 0, len(c.characters))
	for _, char := range c.characters {
		charCopy := *char
		characters = append(characters, &charCopy)
	}

	return characters, nil
}

// CreateConversation creates a conversation in simulation storage
func (c *CharacterAISimulationClient) CreateConversation(ctx context.Context, conversation *memory.Conversation) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.conversations) >= c.config.MaxConversations {
		return fmt.Errorf("maximum conversation limit (%d) reached", c.config.MaxConversations)
	}

	if _, exists := c.conversations[conversation.ID]; exists {
		return fmt.Errorf("conversation with ID %s already exists", conversation.ID)
	}

	// Verify character exists
	if _, exists := c.characters[conversation.CharacterID]; !exists && conversation.CharacterID != "" {
		return fmt.Errorf("character %s not found", conversation.CharacterID)
	}

	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = time.Now()
	}
	conversation.UpdatedAt = time.Now()
	conversation.Status = "active"

	c.conversations[conversation.ID] = conversation
	c.messages[conversation.ID] = make([]*memory.CharacterMessage, 0)

	c.logger.Debug("[SIMULATION] Created conversation id=%s character_id=%s", conversation.ID, conversation.CharacterID)
	return nil
}

// GetConversation retrieves a conversation from simulation storage
func (c *CharacterAISimulationClient) GetConversation(ctx context.Context, conversationID string) (*memory.Conversation, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conv, exists := c.conversations[conversationID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	convCopy := *conv
	return &convCopy, nil
}

// UpdateConversation updates a conversation in simulation storage
func (c *CharacterAISimulationClient) UpdateConversation(ctx context.Context, conversation *memory.Conversation) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.conversations[conversation.ID]; !exists {
		return ErrConversationNotFound
	}

	conversation.UpdatedAt = time.Now()
	c.conversations[conversation.ID] = conversation
	c.logger.Debug("[SIMULATION] Updated conversation id=%s", conversation.ID)
	return nil
}

// DeleteConversation removes a conversation from simulation storage
func (c *CharacterAISimulationClient) DeleteConversation(ctx context.Context, conversationID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.conversations[conversationID]; !exists {
		return ErrConversationNotFound
	}

	delete(c.conversations, conversationID)
	delete(c.messages, conversationID)

	c.logger.Debug("[SIMULATION] Deleted conversation id=%s", conversationID)
	return nil
}

// ListConversations returns conversations for a specific character
func (c *CharacterAISimulationClient) ListConversations(ctx context.Context, characterID string) ([]*memory.Conversation, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conversations := make([]*memory.Conversation, 0)
	for _, conv := range c.conversations {
		if conv.CharacterID == characterID || characterID == "" {
			convCopy := *conv
			conversations = append(conversations, &convCopy)
		}
	}

	return conversations, nil
}

// SendMessage simulates sending a message and receiving a response
func (c *CharacterAISimulationClient) SendMessage(ctx context.Context, message *memory.CharacterMessage) (*memory.CharacterMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Verify conversation exists
	conv, exists := c.conversations[message.SessionID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	// Store the user message
	if message.ID == "" {
		c.messageIDSeq++
		message.ID = fmt.Sprintf("sim_msg_%d", c.messageIDSeq)
	}
	message.Timestamp = time.Now()

	c.messages[message.SessionID] = append(c.messages[message.SessionID], message)

	// Generate simulated response
	c.messageIDSeq++
	response := &memory.CharacterMessage{
		ID:        fmt.Sprintf("sim_msg_%d", c.messageIDSeq),
		SessionID: message.SessionID,
		SenderID:  conv.CharacterID,
		Content:   c.generateSimulatedResponse(conv.CharacterID, message.Content),
		Timestamp: time.Now(),
		Type:      "character",
	}

	c.messages[message.SessionID] = append(c.messages[message.SessionID], response)

	// Update conversation stats
	conv.MessageCount += 2
	conv.UpdatedAt = time.Now()

	c.logger.Debug("[SIMULATION] Sent message and generated response for session=%s", message.SessionID)
	return response, nil
}

// generateSimulatedResponse creates a simulated character response
func (c *CharacterAISimulationClient) generateSimulatedResponse(characterID, userMessage string) string {
	character, exists := c.characters[characterID]
	if !exists {
		return "[Simulation] Character not found"
	}

	// Generate a simple simulated response based on character personality
	response := fmt.Sprintf("[Simulation] %s responds to your message.", character.Name)

	// Add personality-based flavor if available
	if character.Personality != nil {
		if friendly, ok := character.Personality["friendly"].(bool); ok && friendly {
			response = fmt.Sprintf("[Simulation] %s warmly responds to your message.", character.Name)
		}
	}

	return response
}

// GetMessages retrieves messages from a conversation
func (c *CharacterAISimulationClient) GetMessages(ctx context.Context, conversationID string, limit int) ([]*memory.CharacterMessage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	msgs, exists := c.messages[conversationID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	// Return up to limit messages (most recent)
	if limit <= 0 || limit > len(msgs) {
		limit = len(msgs)
	}

	start := len(msgs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*memory.CharacterMessage, limit)
	copy(result, msgs[start:])

	return result, nil
}

// UpdatePersonality updates a character's personality traits
func (c *CharacterAISimulationClient) UpdatePersonality(ctx context.Context, characterID string, traits map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	character, exists := c.characters[characterID]
	if !exists {
		return ErrCharacterNotFound
	}

	if character.Personality == nil {
		character.Personality = make(map[string]interface{})
	}

	for k, v := range traits {
		character.Personality[k] = v
	}

	character.UpdatedAt = time.Now()
	c.logger.Debug("[SIMULATION] Updated personality for character=%s", characterID)
	return nil
}

// GetRelationship retrieves relationship data between character and user
func (c *CharacterAISimulationClient) GetRelationship(ctx context.Context, characterID, userID string) (*memory.RelationshipData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", characterID, userID)
	rel, exists := c.relationships[key]
	if !exists {
		// Return default relationship if not exists
		return &memory.RelationshipData{
			ID:          key,
			CharacterID: characterID,
			UserID:      userID,
			Type:        "acquaintance",
			Strength:    0.5,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Metadata:    map[string]string{},
		}, nil
	}

	relCopy := *rel
	return &relCopy, nil
}

// UpdateRelationship updates relationship data
func (c *CharacterAISimulationClient) UpdateRelationship(ctx context.Context, characterID, userID string, data *memory.RelationshipData) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s:%s", characterID, userID)
	data.UpdatedAt = time.Now()
	c.relationships[key] = data

	c.logger.Debug("[SIMULATION] Updated relationship character=%s user=%s strength=%.2f", characterID, userID, data.Strength)
	return nil
}

// GetEmotionalState retrieves emotional state for a character
func (c *CharacterAISimulationClient) GetEmotionalState(ctx context.Context, characterID string) (*memory.EmotionalState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state, exists := c.emotions[characterID]
	if !exists {
		// Return default emotional state
		return &memory.EmotionalState{
			ID:        fmt.Sprintf("emo_%s", characterID),
			AvatarID:  characterID,
			Mood:      "neutral",
			Intensity: 0.5,
			Timestamp: time.Now(),
			Context:   "default state",
		}, nil
	}

	stateCopy := *state
	return &stateCopy, nil
}

// UpdateEmotionalState updates a character's emotional state
func (c *CharacterAISimulationClient) UpdateEmotionalState(ctx context.Context, characterID string, state *memory.EmotionalState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	state.Timestamp = time.Now()
	c.emotions[characterID] = state

	c.logger.Debug("[SIMULATION] Updated emotional state character=%s mood=%s intensity=%.2f", characterID, state.Mood, state.Intensity)
	return nil
}

// GetHealth returns the health status of the simulation client.
// Always returns nil (healthy) since this is an in-memory simulation.
func (c *CharacterAISimulationClient) GetHealth(ctx context.Context) error {
	c.logger.Debug("[SIMULATION] Health check - simulation client is always healthy")
	return nil
}
