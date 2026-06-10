package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.code/internal/logging"
	zep "github.com/getzep/zep-go/v3"
	zepclient "github.com/getzep/zep-go/v3/client"
	"github.com/getzep/zep-go/v3/option"
)

// ZepProvider implements memory operations using Zep.ai
// Zep is a long-term memory service for AI assistants and agents.
// It uses a knowledge graph approach with Users, Threads, and Facts.
//
// Mapping to VectorProvider interface:
// - Collection -> User (each user has their own graph)
// - Vector -> Facts/Edges in the knowledge graph
// - Metadata -> User metadata or fact attributes
// - Index -> Not applicable (Zep handles indexing automatically)
type ZepProvider struct {
	config  map[string]interface{}
	client  *zepclient.Client
	logger  *logging.Logger
	userID  string
	apiKey  string
	baseURL string

	// Track collections (users) we've created for management
	collections   map[string]*CollectionInfo
	collectionsMu sync.RWMutex

	// Track metadata by ID for operations
	metadataCache   map[string]map[string]interface{}
	metadataCacheMu sync.RWMutex
}

// NewZepProvider creates a new Zep provider instance
func NewZepProvider(config map[string]interface{}) (*ZepProvider, error) {
	provider := &ZepProvider{
		config:        config,
		logger:        logging.NewLoggerWithName("zep_provider"),
		collections:   make(map[string]*CollectionInfo),
		metadataCache: make(map[string]map[string]interface{}),
	}

	// Extract configuration
	if apiKey, ok := config["api_key"].(string); ok {
		provider.apiKey = apiKey
	}

	if baseURL, ok := config["base_url"].(string); ok {
		provider.baseURL = baseURL
	}

	if userID, ok := config["user_id"].(string); ok {
		provider.userID = userID
	}

	// Initialize client
	clientOptions := []option.RequestOption{
		option.WithAPIKey(provider.apiKey),
	}

	if provider.baseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(provider.baseURL))
	}

	provider.client = zepclient.NewClient(clientOptions...)

	return provider, nil
}

// GetType returns the provider type
func (p *ZepProvider) GetType() string {
	return string(ProviderTypeZep)
}

// GetName returns the provider name
func (p *ZepProvider) GetName() string {
	return "Zep"
}

// GetCapabilities returns provider capabilities
func (p *ZepProvider) GetCapabilities() []string {
	return []string{
		"memory_storage",
		"memory_retrieval",
		"memory_search",
		"context_management",
		"graph_memory",
		"knowledge_graph",
		"user_management",
		"thread_management",
	}
}

// GetConfiguration returns provider configuration
func (p *ZepProvider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether this is a cloud provider
func (p *ZepProvider) IsCloud() bool {
	return p.baseURL == "" || contains(p.baseURL, "zep.ai") || contains(p.baseURL, "getzep.com")
}

// Store stores memory data in Zep
func (p *ZepProvider) Store(ctx context.Context, data []*VectorData) error {
	if len(data) == 0 {
		return nil
	}

	// Create user if not exists
	if p.userID != "" {
		_, err := p.client.User.Add(ctx, &zep.CreateUserRequest{
			UserID: p.userID,
		})
		if err != nil {
			p.logger.Warn("Failed to create user: %v", err)
		}
	}

	// Create thread for storing messages
	threadID := generateThreadID()

	_, err := p.client.Thread.Create(ctx, &zep.CreateThreadRequest{
		ThreadID: threadID,
		UserID:   p.userID,
	})
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}

	// Convert data to messages
	var messages []*zep.Message
	for _, item := range data {
		content := item.Metadata["content"]
		if contentStr, ok := content.(string); ok {
			messages = append(messages, &zep.Message{
				Role:    "user",
				Content: contentStr,
			})
		}
	}

	if len(messages) > 0 {
		_, err = p.client.Thread.AddMessages(ctx, threadID, &zep.AddThreadMessagesRequest{
			Messages: messages,
		})
		if err != nil {
			return fmt.Errorf("failed to add messages: %w", err)
		}
	}

	return nil
}

// Search searches for memory in Zep
func (p *ZepProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	// Use graph search for Zep
	searchResults, err := p.client.Graph.Search(ctx, &zep.GraphSearchQuery{
		UserID: zep.String(p.userID),
		Query:  query.Text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search graph: %w", err)
	}

	// Convert results
	results := []*VectorSearchResultItem{}
	for _, edge := range searchResults.Edges {
		results = append(results, &VectorSearchResultItem{
			ID: edge.UUID,
			Metadata: map[string]interface{}{
				"fact":     edge.Fact,
				"type":     "edge",
				"valid_at": edge.ValidAt,
			},
			Score: 1.0, // Zep doesn't provide scores in this format
		})
	}

	for _, node := range searchResults.Nodes {
		results = append(results, &VectorSearchResultItem{
			ID: node.UUID,
			Metadata: map[string]interface{}{
				"name":    node.Name,
				"type":    "node",
				"summary": node.Summary,
			},
			Score: 1.0,
		})
	}

	return &VectorSearchResult{
		Results: results,
	}, nil
}

// Retrieve retrieves vectors by IDs from Zep.
// IDs can be either edge UUIDs (for graph edges/facts) or node UUIDs (for graph nodes).
// Each retrieved item is converted to VectorData with metadata containing the entity details.
func (p *ZepProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	if len(ids) == 0 {
		return []*VectorData{}, nil
	}

	results := make([]*VectorData, 0, len(ids))

	for _, id := range ids {
		if id == "" {
			continue
		}

		// Try to retrieve as an edge (fact) first
		edge, err := p.client.Graph.Edge.Get(ctx, id)
		if err == nil && edge != nil {
			metadata := map[string]interface{}{
				"type":             "edge",
				"fact":             edge.GetFact(),
				"name":             edge.GetName(),
				"source_node_uuid": edge.GetSourceNodeUUID(),
				"target_node_uuid": edge.GetTargetNodeUUID(),
			}
			if edge.GetValidAt() != nil {
				metadata["valid_at"] = *edge.GetValidAt()
			}
			if edge.GetInvalidAt() != nil {
				metadata["invalid_at"] = *edge.GetInvalidAt()
			}
			if createdAt := edge.GetCreatedAt(); createdAt != "" {
				metadata["created_at"] = createdAt
			}
			if edge.GetAttributes() != nil {
				for k, v := range edge.GetAttributes() {
					metadata[k] = v
				}
			}

			// Use the fact content as the text representation
			content := edge.GetFact()
			if content == "" {
				content = edge.GetName()
			}
			metadata["content"] = content

			results = append(results, &VectorData{
				ID:       id,
				Metadata: metadata,
			})
			continue
		}

		// Try to retrieve as a node
		node, err := p.client.Graph.Node.Get(ctx, id)
		if err == nil && node != nil {
			metadata := map[string]interface{}{
				"type":    "node",
				"name":    node.GetName(),
				"summary": node.GetSummary(),
				"labels":  node.GetLabels(),
			}
			if createdAt := node.GetCreatedAt(); createdAt != "" {
				metadata["created_at"] = createdAt
			}
			if node.GetAttributes() != nil {
				for k, v := range node.GetAttributes() {
					metadata[k] = v
				}
			}

			// Use the node summary or name as content
			content := node.GetSummary()
			if content == "" {
				content = node.GetName()
			}
			metadata["content"] = content

			results = append(results, &VectorData{
				ID:       id,
				Metadata: metadata,
			})
			continue
		}

		// ID not found as edge or node - log but don't fail
		p.logger.Debug("Entity with ID '%s' not found in Zep graph", id)
	}

	return results, nil
}

// Update updates an entity in Zep.
// For user IDs, updates user metadata. For edge UUIDs, Zep doesn't support direct edge updates,
// so the edge is deleted and re-created with new data if a "fact" is provided in metadata.
// For node UUIDs, nodes cannot be directly updated in Zep.
func (p *ZepProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if vector == nil {
		return fmt.Errorf("vector data cannot be nil")
	}

	// Check if it's a user ID (collection)
	isUserID := !strings.Contains(id, "-") || p.isKnownUser(id)
	if isUserID {
		// Update user metadata
		return p.addUserMetadata(ctx, id, vector.Metadata)
	}

	// For edge UUIDs, Zep doesn't support partial updates
	// We need to delete and recreate if we want to "update" a fact
	if fact, ok := vector.Metadata["fact"].(string); ok && fact != "" {
		// First, get the existing edge to preserve source/target info
		existingEdge, err := p.client.Graph.Edge.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get existing edge for update: %w", err)
		}

		// Delete the old edge
		_, err = p.client.Graph.Edge.Delete(ctx, id)
		if err != nil {
			p.logger.Warn("Failed to delete old edge during update: %v", err)
		}

		// Determine user ID for the new edge
		userID := p.userID
		if uid, ok := vector.Metadata["user_id"].(string); ok && uid != "" {
			userID = uid
		}

		// Extract edge properties
		sourceName := "Entity"
		targetName := "Entity"
		factName := "RELATES_TO"

		if sn, ok := vector.Metadata["source_node"].(string); ok {
			sourceName = sn
		} else if existingEdge != nil && existingEdge.GetSourceNodeUUID() != "" {
			// Try to get the source node name from existing edge
			sourceNode, err := p.client.Graph.Node.Get(ctx, existingEdge.GetSourceNodeUUID())
			if err == nil && sourceNode != nil {
				sourceName = sourceNode.GetName()
			}
		}

		if tn, ok := vector.Metadata["target_node"].(string); ok {
			targetName = tn
		} else if existingEdge != nil && existingEdge.GetTargetNodeUUID() != "" {
			// Try to get the target node name from existing edge
			targetNode, err := p.client.Graph.Node.Get(ctx, existingEdge.GetTargetNodeUUID())
			if err == nil && targetNode != nil {
				targetName = targetNode.GetName()
			}
		}

		if fn, ok := vector.Metadata["fact_name"].(string); ok {
			factName = fn
		} else if existingEdge != nil && existingEdge.GetName() != "" {
			factName = existingEdge.GetName()
		}

		// Create new fact triple with updated content
		req := &zep.AddTripleRequest{
			UserID:         zep.String(userID),
			Fact:           fact,
			FactName:       factName,
			SourceNodeName: zep.String(sourceName),
			TargetNodeName: zep.String(targetName),
		}

		_, err = p.client.Graph.AddFactTriple(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to create updated fact: %w", err)
		}

		p.logger.Debug("Updated edge '%s' by replacing with new fact", id)
		return nil
	}

	// For node UUIDs without a fact - nodes cannot be directly modified in Zep
	// Log a warning and update local metadata cache
	p.logger.Warn("Direct node updates not supported in Zep - storing metadata locally")
	p.metadataCacheMu.Lock()
	if p.metadataCache[id] == nil {
		p.metadataCache[id] = make(map[string]interface{})
	}
	for k, v := range vector.Metadata {
		p.metadataCache[id][k] = v
	}
	p.metadataCacheMu.Unlock()

	return nil
}

// Delete deletes entities from Zep by their IDs.
// For user IDs, this deletes the entire user and their knowledge graph (use with caution).
// For edge UUIDs, deletes the specific edge/fact.
// For node UUIDs, Zep doesn't support direct node deletion - nodes are garbage collected
// when no edges reference them.
func (p *ZepProvider) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	var lastErr error
	deletedCount := 0

	for _, id := range ids {
		if id == "" {
			continue
		}

		// Check if it's a user ID (collection)
		isUserID := !strings.Contains(id, "-") || p.isKnownUser(id)
		if isUserID {
			// Delete the user - this removes the entire knowledge graph
			// This is a destructive operation
			_, err := p.client.User.Delete(ctx, id)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					p.logger.Debug("User '%s' not found, may already be deleted", id)
				} else {
					p.logger.Warn("Failed to delete user '%s': %v", id, err)
					lastErr = err
				}
			} else {
				// Remove from local tracking
				p.collectionsMu.Lock()
				delete(p.collections, id)
				p.collectionsMu.Unlock()
				deletedCount++
				p.logger.Info("Deleted user/collection '%s'", id)
			}
			continue
		}

		// Try to delete as an edge (fact)
		_, err := p.client.Graph.Edge.Delete(ctx, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				p.logger.Debug("Edge '%s' not found, may already be deleted", id)
			} else {
				// Might be a node UUID - nodes can't be directly deleted in Zep
				// They are garbage collected when no edges reference them
				p.logger.Debug("Could not delete '%s' as edge: %v - if this is a node, it will be garbage collected when orphaned", id, err)
			}
		} else {
			deletedCount++
			p.logger.Debug("Deleted edge '%s'", id)
		}

		// Clean up local metadata cache
		p.metadataCacheMu.Lock()
		delete(p.metadataCache, id)
		p.metadataCacheMu.Unlock()
	}

	if deletedCount > 0 {
		p.logger.Info("Deleted %d entities from Zep", deletedCount)
	}

	return lastErr
}

// FindSimilar finds similar vectors in Zep
func (p *ZepProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	// Use graph search for similarity
	searchResults, err := p.client.Graph.Search(ctx, &zep.GraphSearchQuery{
		UserID: zep.String(p.userID),
		Query:  fmt.Sprintf("embedding:%v", embedding),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search graph: %w", err)
	}

	results := []*VectorSimilarityResult{}
	for _, edge := range searchResults.Edges {
		results = append(results, &VectorSimilarityResult{
			ID:       edge.UUID,
			Score:    1.0,
			Metadata: map[string]interface{}{"fact": edge.Fact},
		})
		if len(results) >= k {
			break
		}
	}

	return results, nil
}

// BatchFindSimilar finds similar vectors for multiple queries in Zep
func (p *ZepProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
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

// CreateCollection creates a collection in Zep by creating a new User.
// In Zep's architecture, each User has their own knowledge graph, which
// conceptually maps to a "collection" of memories and facts.
func (p *ZepProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	if name == "" {
		return fmt.Errorf("collection name (user ID) cannot be empty")
	}

	// Create the user in Zep
	createReq := &zep.CreateUserRequest{
		UserID: name,
	}

	// Add metadata from config if provided
	if config != nil && config.Properties != nil {
		createReq.Metadata = config.Properties
	}

	user, err := p.client.User.Add(ctx, createReq)
	if err != nil {
		// Check if user already exists (this is not necessarily an error)
		if strings.Contains(err.Error(), "already exists") {
			p.logger.Info("User/collection '%s' already exists in Zep", name)
			// Still track it locally
		} else {
			return fmt.Errorf("failed to create Zep user/collection '%s': %w", name, err)
		}
	}

	// Track the collection locally
	p.collectionsMu.Lock()
	defer p.collectionsMu.Unlock()

	collInfo := &CollectionInfo{
		Name:      name,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	if config != nil {
		collInfo.Dimension = config.Dimension
		collInfo.Metric = config.Metric
		collInfo.Config = config
	}

	if user != nil && user.GetUUID() != nil {
		if collInfo.Metadata == nil {
			collInfo.Metadata = make(map[string]interface{})
		}
		collInfo.Metadata["zep_uuid"] = *user.GetUUID()
	}

	p.collections[name] = collInfo

	p.logger.Info("Created Zep collection (user): %s", name)
	return nil
}

// DeleteCollection deletes a collection in Zep by deleting the corresponding User.
// WARNING: This will permanently delete all data associated with the user including
// all threads, messages, and the user's knowledge graph.
func (p *ZepProvider) DeleteCollection(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("collection name (user ID) cannot be empty")
	}

	// Delete the user in Zep
	_, err := p.client.User.Delete(ctx, name)
	if err != nil {
		// Check if user doesn't exist
		if strings.Contains(err.Error(), "not found") {
			p.logger.Warn("User/collection '%s' not found in Zep, may already be deleted", name)
		} else {
			return fmt.Errorf("failed to delete Zep user/collection '%s': %w", name, err)
		}
	}

	// Remove from local tracking
	p.collectionsMu.Lock()
	delete(p.collections, name)
	p.collectionsMu.Unlock()

	p.logger.Info("Deleted Zep collection (user): %s", name)
	return nil
}

// ListCollections lists collections in Zep by listing all Users.
// Each User in Zep corresponds to a collection with its own knowledge graph.
func (p *ZepProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	// List all users from Zep
	listReq := &zep.UserListOrderedRequest{}
	usersResp, err := p.client.User.ListOrdered(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list Zep users/collections: %w", err)
	}

	var collections []*CollectionInfo

	if usersResp != nil && usersResp.Users != nil {
		for _, user := range usersResp.Users {
			if user == nil {
				continue
			}

			collInfo := &CollectionInfo{
				Name:   safeString(user.GetUserID()),
				Status: "active",
			}

			if user.GetCreatedAt() != nil {
				if t, err := time.Parse(time.RFC3339, *user.GetCreatedAt()); err == nil {
					collInfo.CreatedAt = t
				}
			}

			if user.GetUUID() != nil {
				if collInfo.Metadata == nil {
					collInfo.Metadata = make(map[string]interface{})
				}
				collInfo.Metadata["zep_uuid"] = *user.GetUUID()
			}

			if user.GetMetadata() != nil {
				if collInfo.Metadata == nil {
					collInfo.Metadata = make(map[string]interface{})
				}
				for k, v := range user.GetMetadata() {
					collInfo.Metadata[k] = v
				}
			}

			collections = append(collections, collInfo)

			// Update local cache
			p.collectionsMu.Lock()
			p.collections[collInfo.Name] = collInfo
			p.collectionsMu.Unlock()
		}
	}

	return collections, nil
}

// GetCollection gets collection info in Zep by retrieving User details.
// Returns information about the user and their knowledge graph.
func (p *ZepProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("collection name (user ID) cannot be empty")
	}

	// First check local cache
	p.collectionsMu.RLock()
	if cached, ok := p.collections[name]; ok {
		p.collectionsMu.RUnlock()
		return cached, nil
	}
	p.collectionsMu.RUnlock()

	// Fetch from Zep
	user, err := p.client.User.Get(ctx, name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("collection (user) '%s' not found in Zep", name)
		}
		return nil, fmt.Errorf("failed to get Zep user/collection '%s': %w", name, err)
	}

	if user == nil {
		return nil, fmt.Errorf("collection (user) '%s' not found in Zep", name)
	}

	collInfo := &CollectionInfo{
		Name:   safeString(user.GetUserID()),
		Status: "active",
	}

	if user.GetCreatedAt() != nil {
		if t, err := time.Parse(time.RFC3339, *user.GetCreatedAt()); err == nil {
			collInfo.CreatedAt = t
		}
	}

	if user.GetUpdatedAt() != nil {
		if t, err := time.Parse(time.RFC3339, *user.GetUpdatedAt()); err == nil {
			collInfo.UpdatedAt = t
		}
	}

	collInfo.Metadata = make(map[string]interface{})
	if user.GetUUID() != nil {
		collInfo.Metadata["zep_uuid"] = *user.GetUUID()
	}
	if user.GetEmail() != nil {
		collInfo.Metadata["email"] = *user.GetEmail()
	}
	if user.GetFirstName() != nil {
		collInfo.Metadata["first_name"] = *user.GetFirstName()
	}
	if user.GetLastName() != nil {
		collInfo.Metadata["last_name"] = *user.GetLastName()
	}
	if user.GetMetadata() != nil {
		for k, v := range user.GetMetadata() {
			collInfo.Metadata[k] = v
		}
	}

	// Cache the result
	p.collectionsMu.Lock()
	p.collections[name] = collInfo
	p.collectionsMu.Unlock()

	return collInfo, nil
}

// ErrZepIndexNotSupported is returned when index operations are attempted.
// Zep automatically manages indexing internally and does not expose index management APIs.
var ErrZepIndexNotSupported = fmt.Errorf("zep does not support manual index management: " +
	"Zep automatically handles indexing internally using its knowledge graph architecture. " +
	"No action is required - your data is automatically indexed for semantic search. " +
	"Consider using Graph.Search for semantic queries or Thread operations for context retrieval")

// CreateIndex is not supported in Zep.
// Zep automatically handles indexing internally using its knowledge graph architecture.
// The graph structure with nodes and edges is automatically maintained and optimized
// for semantic search without requiring manual index creation.
func (p *ZepProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	p.logger.Info("CreateIndex called but Zep handles indexing automatically - no action needed")
	return ErrZepIndexNotSupported
}

// DeleteIndex is not supported in Zep.
// Zep automatically manages its internal indexing structures.
// Users cannot create or delete indexes manually.
func (p *ZepProvider) DeleteIndex(ctx context.Context, collection, name string) error {
	p.logger.Info("DeleteIndex called but Zep handles indexing automatically - no action needed")
	return ErrZepIndexNotSupported
}

// ListIndexes returns an empty list for Zep.
// Zep does not expose index information as it manages indexing internally.
// The knowledge graph structure serves as the implicit "index" for semantic search.
func (p *ZepProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	// Return a single synthetic index entry to indicate Zep's automatic indexing
	return []*IndexInfo{
		{
			Name:      "zep_knowledge_graph",
			Type:      "automatic",
			State:     "active",
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"description": "Zep automatically indexes data using its knowledge graph architecture",
				"managed_by":  "zep_internal",
			},
		},
	}, nil
}

// AddMetadata adds metadata to an entity in Zep.
// The 'id' can be either a user ID (for user metadata) or a fact/edge UUID.
// For user metadata, it updates the user's metadata field.
// For facts, it adds a new fact triple to the user's graph.
func (p *ZepProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if len(metadata) == 0 {
		return nil // Nothing to add
	}

	// Determine if this is a user ID or a fact UUID
	// User IDs typically don't have dashes like UUIDs
	isUserID := !strings.Contains(id, "-") || p.isKnownUser(id)

	if isUserID {
		// Update user metadata
		return p.addUserMetadata(ctx, id, metadata)
	}

	// For non-user IDs, store metadata locally and associate with graph facts
	// This could be an edge UUID or a custom ID
	p.metadataCacheMu.Lock()
	if p.metadataCache[id] == nil {
		p.metadataCache[id] = make(map[string]interface{})
	}
	for k, v := range metadata {
		p.metadataCache[id][k] = v
	}
	p.metadataCacheMu.Unlock()

	// If there's a "fact" key in metadata, add it as a fact triple
	if fact, ok := metadata["fact"].(string); ok {
		userID := p.userID
		if uid, ok := metadata["user_id"].(string); ok {
			userID = uid
		}
		if userID != "" {
			return p.addFactTriple(ctx, userID, id, fact, metadata)
		}
	}

	return nil
}

// addUserMetadata updates the metadata for a Zep user
func (p *ZepProvider) addUserMetadata(ctx context.Context, userID string, metadata map[string]interface{}) error {
	// Get existing user to merge metadata
	user, err := p.client.User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user '%s' for metadata update: %w", userID, err)
	}

	// Merge existing metadata with new metadata
	existingMeta := user.GetMetadata()
	if existingMeta == nil {
		existingMeta = make(map[string]interface{})
	}
	for k, v := range metadata {
		existingMeta[k] = v
	}

	// Update the user
	updateReq := &zep.UpdateUserRequest{
		Metadata: existingMeta,
	}
	_, err = p.client.User.Update(ctx, userID, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update metadata for user '%s': %w", userID, err)
	}

	p.logger.Debug("Added metadata to user '%s'", userID)
	return nil
}

// addFactTriple adds a fact triple to the user's graph
func (p *ZepProvider) addFactTriple(ctx context.Context, userID, factID, fact string, metadata map[string]interface{}) error {
	// Extract source and target nodes from metadata if available
	sourceName := "Entity"
	targetName := "Entity"
	factName := "RELATES_TO"

	if sn, ok := metadata["source_node"].(string); ok {
		sourceName = sn
	}
	if tn, ok := metadata["target_node"].(string); ok {
		targetName = tn
	}
	if fn, ok := metadata["fact_name"].(string); ok {
		factName = fn
	}

	req := &zep.AddTripleRequest{
		UserID:         zep.String(userID),
		Fact:           fact,
		FactName:       factName,
		FactUUID:       zep.String(factID),
		SourceNodeName: zep.String(sourceName),
		TargetNodeName: zep.String(targetName),
	}

	_, err := p.client.Graph.AddFactTriple(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add fact triple: %w", err)
	}

	return nil
}

// isKnownUser checks if an ID is a known user ID
func (p *ZepProvider) isKnownUser(id string) bool {
	p.collectionsMu.RLock()
	defer p.collectionsMu.RUnlock()
	_, exists := p.collections[id]
	return exists
}

// UpdateMetadata updates metadata for an entity in Zep.
// This is similar to AddMetadata but replaces existing values.
func (p *ZepProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if len(metadata) == 0 {
		return nil
	}

	// For users, this works the same as AddMetadata (upsert behavior)
	isUserID := !strings.Contains(id, "-") || p.isKnownUser(id)
	if isUserID {
		return p.addUserMetadata(ctx, id, metadata)
	}

	// For other IDs, update the local cache
	p.metadataCacheMu.Lock()
	if p.metadataCache[id] == nil {
		p.metadataCache[id] = make(map[string]interface{})
	}
	for k, v := range metadata {
		p.metadataCache[id][k] = v
	}
	p.metadataCacheMu.Unlock()

	return nil
}

// GetMetadata gets metadata for entities in Zep.
// For user IDs, retrieves user metadata. For other IDs, retrieves from local cache
// or attempts to fetch edge/node details from the graph.
func (p *ZepProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})

	for _, id := range ids {
		if id == "" {
			continue
		}

		// Check if it's a user ID
		isUserID := !strings.Contains(id, "-") || p.isKnownUser(id)
		if isUserID {
			user, err := p.client.User.Get(ctx, id)
			if err == nil && user != nil {
				meta := make(map[string]interface{})
				if user.GetMetadata() != nil {
					for k, v := range user.GetMetadata() {
						meta[k] = v
					}
				}
				if user.GetUUID() != nil {
					meta["zep_uuid"] = *user.GetUUID()
				}
				if user.GetEmail() != nil {
					meta["email"] = *user.GetEmail()
				}
				result[id] = meta
			}
			continue
		}

		// Try to get from local cache first
		p.metadataCacheMu.RLock()
		if cached, ok := p.metadataCache[id]; ok {
			result[id] = cached
			p.metadataCacheMu.RUnlock()
			continue
		}
		p.metadataCacheMu.RUnlock()

		// Try to fetch as an edge UUID
		edge, err := p.client.Graph.Edge.Get(ctx, id)
		if err == nil && edge != nil {
			meta := make(map[string]interface{})
			meta["fact"] = edge.GetFact()
			meta["name"] = edge.GetName()
			meta["source_node_uuid"] = edge.GetSourceNodeUUID()
			meta["target_node_uuid"] = edge.GetTargetNodeUUID()
			if edge.GetValidAt() != nil {
				meta["valid_at"] = *edge.GetValidAt()
			}
			if edge.GetAttributes() != nil {
				for k, v := range edge.GetAttributes() {
					meta[k] = v
				}
			}
			result[id] = meta
			continue
		}

		// Try to fetch as a node UUID
		node, err := p.client.Graph.Node.Get(ctx, id)
		if err == nil && node != nil {
			meta := make(map[string]interface{})
			meta["name"] = node.GetName()
			meta["summary"] = node.GetSummary()
			meta["labels"] = node.GetLabels()
			if node.GetAttributes() != nil {
				for k, v := range node.GetAttributes() {
					meta[k] = v
				}
			}
			result[id] = meta
		}
	}

	return result, nil
}

// DeleteMetadata deletes specific metadata keys from entities in Zep.
// For user metadata, it removes the specified keys from the user's metadata.
// Note: Zep does not support deleting individual edge attributes - edges must be deleted entirely.
func (p *ZepProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	if len(ids) == 0 || len(keys) == 0 {
		return nil
	}

	for _, id := range ids {
		if id == "" {
			continue
		}

		// Check if it's a user ID
		isUserID := !strings.Contains(id, "-") || p.isKnownUser(id)
		if isUserID {
			// Get existing user metadata
			user, err := p.client.User.Get(ctx, id)
			if err != nil {
				p.logger.Warn("Failed to get user '%s' for metadata deletion: %v", id, err)
				continue
			}

			// Remove specified keys from metadata
			existingMeta := user.GetMetadata()
			if existingMeta == nil {
				continue
			}

			for _, key := range keys {
				delete(existingMeta, key)
			}

			// Update the user with modified metadata
			updateReq := &zep.UpdateUserRequest{
				Metadata: existingMeta,
			}
			_, err = p.client.User.Update(ctx, id, updateReq)
			if err != nil {
				p.logger.Warn("Failed to delete metadata keys from user '%s': %v", id, err)
			}
			continue
		}

		// For edge UUIDs, we can only delete the entire edge, not individual attributes
		// Check if it looks like a UUID (edge/fact ID)
		if strings.Contains(id, "-") {
			// Try to delete the edge
			_, err := p.client.Graph.Edge.Delete(ctx, id)
			if err != nil {
				p.logger.Warn("Failed to delete edge '%s': %v (Zep does not support partial metadata deletion for edges)", id, err)
			} else {
				p.logger.Debug("Deleted edge '%s' (Zep requires deleting entire edge)", id)
			}
		}

		// Clean up local cache
		p.metadataCacheMu.Lock()
		if meta, ok := p.metadataCache[id]; ok {
			for _, key := range keys {
				delete(meta, key)
			}
			if len(meta) == 0 {
				delete(p.metadataCache, id)
			}
		}
		p.metadataCacheMu.Unlock()
	}

	return nil
}

// Optimize optimizes the Zep provider by warming user graphs for low-latency search.
// This hints Zep to pre-load user data into memory for faster subsequent queries.
// If a specific userID is configured, it warms that user's graph.
// Otherwise, it attempts to warm all known users' graphs.
func (p *ZepProvider) Optimize(ctx context.Context) error {
	if p.userID != "" {
		// Warm the configured user's graph
		_, err := p.client.User.Warm(ctx, p.userID)
		if err != nil {
			p.logger.Warn("Failed to warm user graph for '%s': %v", p.userID, err)
			// Don't return error - warming is a hint, not a requirement
		} else {
			p.logger.Info("Warmed user graph for '%s' for low-latency search", p.userID)
		}
		return nil
	}

	// Warm all known users' graphs
	p.collectionsMu.RLock()
	userIDs := make([]string, 0, len(p.collections))
	for userID := range p.collections {
		userIDs = append(userIDs, userID)
	}
	p.collectionsMu.RUnlock()

	warmed := 0
	for _, userID := range userIDs {
		_, err := p.client.User.Warm(ctx, userID)
		if err != nil {
			p.logger.Warn("Failed to warm user graph for '%s': %v", userID, err)
		} else {
			warmed++
		}
	}

	if warmed > 0 {
		p.logger.Info("Warmed %d user graphs for low-latency search", warmed)
	} else {
		p.logger.Info("No user graphs to warm (Zep Cloud manages optimization automatically)")
	}

	return nil
}

// ErrZepBackupNotSupported is returned when backup/restore operations are attempted.
var ErrZepBackupNotSupported = fmt.Errorf("zep cloud does not support direct backup/restore operations: " +
	"Data is automatically persisted and replicated by Zep Cloud infrastructure. " +
	"For data export, use the Graph API to retrieve edges/nodes and serialize them. " +
	"For self-hosted Zep, backup the underlying PostgreSQL database directly")

// Backup is not directly supported by Zep Cloud API.
// Zep Cloud automatically handles data persistence and replication.
// For data export purposes, this method exports graph data to a JSON file.
func (p *ZepProvider) Backup(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("backup path cannot be empty")
	}

	// For Zep, we can export the graph data as a form of "backup"
	// This exports edges and nodes for the configured user
	if p.userID == "" {
		return fmt.Errorf("no user_id configured - cannot determine which graph to backup. " +
			"Zep Cloud manages data persistence automatically. " +
			"For data export, configure a user_id or use the Zep Cloud dashboard")
	}

	// Get edges for the user
	edges, err := p.client.Graph.Edge.GetByUserID(ctx, p.userID, &zep.GraphEdgesRequest{})
	if err != nil {
		return fmt.Errorf("failed to get edges for backup: %w", err)
	}

	// Get nodes for the user
	nodes, err := p.client.Graph.Node.GetByUserID(ctx, p.userID, &zep.GraphNodesRequest{})
	if err != nil {
		return fmt.Errorf("failed to get nodes for backup: %w", err)
	}

	// Create backup data structure
	backupData := map[string]interface{}{
		"user_id":     p.userID,
		"backup_time": time.Now().UTC().Format(time.RFC3339),
		"provider":    "zep",
		"edges":       edges,
		"nodes":       nodes,
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(backupData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize backup data: %w", err)
	}

	// Persist backup to the supplied path. The historical comment
	// "in a real implementation, you'd use os.WriteFile / for now we
	// just log" was a §11.4 PASS-bluff (Article XI §11.9 / CONST-035):
	// the function signature promised a file backup, the API contract
	// described durable export, and the implementation produced
	// nothing on disk. The cache write below preserves the
	// in-memory shortcut path but is no longer the only persistence.
	if path != "" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			return fmt.Errorf("zep backup: write %s: %w", path, err)
		}
	}
	p.logger.Info("Created backup for user '%s' (%d edges, %d nodes) - data size: %d bytes",
		p.userID, len(edges), len(nodes), len(data))
	p.logger.Info("Backup data written to: %s", path)
	p.logger.Warn("Note: Zep Cloud automatically persists data. This export is for portability only.")

	// Store backup data in metadata cache for retrieval
	p.metadataCacheMu.Lock()
	p.metadataCache["_last_backup"] = map[string]interface{}{
		"path":       path,
		"size":       len(data),
		"edges":      len(edges),
		"nodes":      len(nodes),
		"created_at": time.Now().UTC(),
		"data":       string(data),
	}
	p.metadataCacheMu.Unlock()

	return nil
}

// Restore is not directly supported by Zep Cloud API.
// Zep Cloud automatically handles data persistence.
// For data import from a backup file, use the Graph API to add edges/nodes.
func (p *ZepProvider) Restore(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("restore path cannot be empty")
	}

	// Check if we have backup data in cache (from a recent Backup call)
	p.metadataCacheMu.RLock()
	backupMeta, hasBackup := p.metadataCache["_last_backup"]
	p.metadataCacheMu.RUnlock()

	if !hasBackup {
		return fmt.Errorf("restore not supported: Zep Cloud manages data persistence automatically. " +
			"To import data, use Graph.AddFactTriple to add facts or Thread.AddMessages to add conversations. " +
			"For bulk import, use the Zep Cloud dashboard or API directly")
	}

	// If we have cached backup data, we can attempt to restore facts
	dataStr, ok := backupMeta["data"].(string)
	if !ok || dataStr == "" {
		return fmt.Errorf("no backup data available to restore")
	}

	// Parse backup data
	var backupData map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &backupData); err != nil {
		return fmt.Errorf("failed to parse backup data: %w", err)
	}

	p.logger.Info("Restore operation: Zep Cloud persists data automatically")
	p.logger.Info("Backup contains %v edges and %v nodes from user %v",
		backupMeta["edges"], backupMeta["nodes"], backupData["user_id"])
	p.logger.Warn("Note: Full restore requires using Graph.AddFactTriple for each edge. " +
		"This is typically done through data migration scripts")

	return ErrZepBackupNotSupported
}

// Initialize initializes the Zep provider
func (p *ZepProvider) Initialize(ctx context.Context, config interface{}) error {
	// Already initialized in NewZepProvider
	return nil
}

// Start starts the Zep provider
func (p *ZepProvider) Start(ctx context.Context) error {
	// Zep client is already started
	return nil
}

// Stop stops the Zep provider
func (p *ZepProvider) Stop(ctx context.Context) error {
	// Close client if needed
	return p.Close(ctx)
}

// GetCostInfo returns cost information for Zep
func (p *ZepProvider) GetCostInfo() *CostInfo {
	return &CostInfo{
		Currency:      "USD",
		ComputeCost:   0.0,
		TransferCost:  0.0,
		StorageCost:   0.0,
		TotalCost:     0.0,
		BillingPeriod: "monthly",
		FreeTierUsed:  0.0,
		FreeTierLimit: 0.0,
	}
}

// GetStats returns provider statistics
func (p *ZepProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	// Get user info as basic stats
	stats := &ProviderStats{
		Name:             "Zep",
		Type:             "zep",
		Status:           "active",
		TotalOperations:  0,
		SuccessfulOps:    0,
		FailedOps:        0,
		AverageLatency:   0,
		TotalVectors:     0,
		TotalCollections: 0,
		TotalSize:        0,
		LastHealthCheck:  time.Now(),
	}

	return stats, nil
}

// Health checks provider health
func (p *ZepProvider) Health(ctx context.Context) (*HealthStatus, error) {
	// Simple health check by trying to get user info
	_, err := p.client.User.Get(ctx, p.userID)
	if err != nil {
		return &HealthStatus{
			Status:    "unhealthy",
			Message:   err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	return &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
	}, nil
}

// Close closes the provider
func (p *ZepProvider) Close(ctx context.Context) error {
	// Cleanup if needed
	return nil
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// threadIDCounter guarantees uniqueness when generateThreadID is called
// multiple times within the same nanosecond (HXC-046: on fast hardware two
// back-to-back time.Now().UnixNano() reads can return the identical value).
var threadIDCounter atomic.Uint64

func generateThreadID() string {
	return fmt.Sprintf("thread-%d-%d", time.Now().UnixNano(), threadIDCounter.Add(1))
}

// safeString returns the string value or empty string if pointer is nil
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
