package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/config"
)

// ContextType represents the type of context
type ContextType string

const (
	// ContextTypeFile represents file-based context
	ContextTypeFile ContextType = "file"
	// ContextTypeSession represents session-based context
	ContextTypeSession ContextType = "session"
	// ContextTypeProject represents project-based context
	ContextTypeProject ContextType = "project"
	// ContextTypeGlobal represents global context
	ContextTypeGlobal ContextType = "global"
)

// ContextItem represents a single context item
type ContextItem struct {
	ID        string                 `json:"id"`
	Type      ContextType            `json:"type"`
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	TTL       *time.Duration         `json:"ttl,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Priority  int                    `json:"priority"`
}

// IsExpired checks if the context item has expired
func (ci *ContextItem) IsExpired() bool {
	if ci.TTL == nil {
		return false
	}
	return time.Since(ci.Timestamp) > *ci.TTL
}

// ContextManager manages context across different scopes
type ContextManager struct {
	items    map[string]*ContextItem
	sessions map[string]*SessionContext
	projects map[string]*ProjectContext
	global   *GlobalContext
	config   *config.ContextConfig
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// NewContextManager creates a new context manager
func NewContextManager(config *config.ContextConfig) *ContextManager {
	return &ContextManager{
		items:    make(map[string]*ContextItem),
		sessions: make(map[string]*SessionContext),
		projects: make(map[string]*ProjectContext),
		global:   NewGlobalContext(),
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Store stores a context item
func (cm *ContextManager) Store(ctx context.Context, item *ContextItem) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	item.Timestamp = time.Now()
	cm.items[item.ID] = item

	// Store in appropriate context based on type
	switch item.Type {
	case ContextTypeSession:
		if sessionID, ok := item.Metadata["session_id"].(string); ok {
			session := cm.getOrCreateSession(sessionID)
			session.Store(item)
		}
	case ContextTypeProject:
		if projectID, ok := item.Metadata["project_id"].(string); ok {
			project := cm.getOrCreateProject(projectID)
			project.Store(item)
		}
	case ContextTypeGlobal:
		cm.global.Store(item)
	}

	return nil
}

// Retrieve retrieves a context item by ID
func (cm *ContextManager) Retrieve(ctx context.Context, id string) (*ContextItem, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	item, exists := cm.items[id]
	if !exists {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_context_item_not_found", map[string]any{"ID": id}))
	}

	if item.IsExpired() {
		// Remove expired item
		go cm.Delete(context.Background(), id)
		return nil, fmt.Errorf("%s", tr(ctx, "internal_context_item_expired", map[string]any{"ID": id}))
	}

	return item, nil
}

// Search searches for context items by key pattern
func (cm *ContextManager) Search(ctx context.Context, keyPattern string, contextType ContextType) ([]*ContextItem, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	results := make([]*ContextItem, 0)

	for _, item := range cm.items {
		if item.Type != contextType {
			continue
		}

		// Simple pattern matching (could be enhanced with regex)
		if keyPattern == "" || item.Key == keyPattern {
			if !item.IsExpired() {
				results = append(results, item)
			}
		}
	}

	return results, nil
}

// Delete deletes a context item
func (cm *ContextManager) Delete(ctx context.Context, id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	item, exists := cm.items[id]
	if !exists {
		return fmt.Errorf("%s", tr(ctx, "internal_context_item_not_found", map[string]any{"ID": id}))
	}

	delete(cm.items, id)

	// Remove from specific context
	switch item.Type {
	case ContextTypeSession:
		if sessionID, ok := item.Metadata["session_id"].(string); ok {
			if session, exists := cm.sessions[sessionID]; exists {
				session.Delete(id)
			}
		}
	case ContextTypeProject:
		if projectID, ok := item.Metadata["project_id"].(string); ok {
			if project, exists := cm.projects[projectID]; exists {
				project.Delete(id)
			}
		}
	case ContextTypeGlobal:
		cm.global.Delete(id)
	}

	return nil
}

// Clear clears all context items
func (cm *ContextManager) Clear(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.items = make(map[string]*ContextItem)
	cm.sessions = make(map[string]*SessionContext)
	cm.projects = make(map[string]*ProjectContext)
	cm.global = NewGlobalContext()

	return nil
}

// GetSessionContext gets context for a specific session
func (cm *ContextManager) GetSessionContext(sessionID string) (*SessionContext, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	session, exists := cm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("%s", tr(context.Background(), "internal_context_session_not_found", map[string]any{"SessionID": sessionID}))
	}

	return session, nil
}

// GetProjectContext gets context for a specific project
func (cm *ContextManager) GetProjectContext(projectID string) (*ProjectContext, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	project, exists := cm.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("%s", tr(context.Background(), "internal_context_project_not_found", map[string]any{"ProjectID": projectID}))
	}

	return project, nil
}

// GetGlobalContext gets the global context
func (cm *ContextManager) GetGlobalContext() *GlobalContext {
	return cm.global
}

// getOrCreateSession gets or creates a session context
func (cm *ContextManager) getOrCreateSession(sessionID string) *SessionContext {
	session, exists := cm.sessions[sessionID]
	if !exists {
		session = NewSessionContext(sessionID)
		cm.sessions[sessionID] = session
	}
	return session
}

// getOrCreateProject gets or creates a project context
func (cm *ContextManager) getOrCreateProject(projectID string) *ProjectContext {
	project, exists := cm.projects[projectID]
	if !exists {
		project = NewProjectContext(projectID)
		cm.projects[projectID] = project
	}
	return project
}

// Start starts the context manager
func (cm *ContextManager) Start(ctx context.Context) error {
	cm.wg.Add(1)
	go cm.cleanupRoutine(ctx)
	return nil
}

// Stop stops the context manager
func (cm *ContextManager) Stop() {
	close(cm.stopChan)
	cm.wg.Wait()
}

// cleanupRoutine runs periodic cleanup of expired items
func (cm *ContextManager) cleanupRoutine(ctx context.Context) {
	defer cm.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopChan:
			return
		case <-ticker.C:
			cm.cleanupExpiredItems()
		}
	}
}

// cleanupExpiredItems removes expired context items
func (cm *ContextManager) cleanupExpiredItems() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, item := range cm.items {
		if item.IsExpired() {
			delete(cm.items, id)

			// Remove from specific context
			switch item.Type {
			case ContextTypeSession:
				if sessionID, ok := item.Metadata["session_id"].(string); ok {
					if session, exists := cm.sessions[sessionID]; exists {
						session.Delete(id)
					}
				}
			case ContextTypeProject:
				if projectID, ok := item.Metadata["project_id"].(string); ok {
					if project, exists := cm.projects[projectID]; exists {
						project.Delete(id)
					}
				}
			case ContextTypeGlobal:
				cm.global.Delete(id)
			}
		}
	}
}

// GetStatistics returns context manager statistics
func (cm *ContextManager) GetStatistics() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_items":    len(cm.items),
		"total_sessions": len(cm.sessions),
		"total_projects": len(cm.projects),
		"global_items":   cm.global.Size(),
	}

	// Count items by type
	itemsByType := make(map[string]int)
	for _, item := range cm.items {
		itemsByType[string(item.Type)]++
	}
	stats["items_by_type"] = itemsByType

	return stats
}

// SessionContext represents context for a specific session
type SessionContext struct {
	sessionID string
	items     map[string]*ContextItem
	mu        sync.RWMutex
}

// NewSessionContext creates a new session context
func NewSessionContext(sessionID string) *SessionContext {
	return &SessionContext{
		sessionID: sessionID,
		items:     make(map[string]*ContextItem),
	}
}

// Store stores an item in the session context
func (sc *SessionContext) Store(item *ContextItem) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.items[item.ID] = item
}

// Retrieve retrieves an item from the session context
func (sc *SessionContext) Retrieve(id string) (*ContextItem, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	item, exists := sc.items[id]
	return item, exists
}

// Delete deletes an item from the session context
func (sc *SessionContext) Delete(id string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.items, id)
}

// Size returns the number of items in the session context
func (sc *SessionContext) Size() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.items)
}

// Clear clears all items from the session context
func (sc *SessionContext) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.items = make(map[string]*ContextItem)
}

// ProjectContext represents context for a specific project
type ProjectContext struct {
	projectID string
	items     map[string]*ContextItem
	mu        sync.RWMutex
}

// NewProjectContext creates a new project context
func NewProjectContext(projectID string) *ProjectContext {
	return &ProjectContext{
		projectID: projectID,
		items:     make(map[string]*ContextItem),
	}
}

// Store stores an item in the project context
func (pc *ProjectContext) Store(item *ContextItem) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.items[item.ID] = item
}

// Retrieve retrieves an item from the project context
func (pc *ProjectContext) Retrieve(id string) (*ContextItem, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	item, exists := pc.items[id]
	return item, exists
}

// Delete deletes an item from the project context
func (pc *ProjectContext) Delete(id string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.items, id)
}

// Size returns the number of items in the project context
func (pc *ProjectContext) Size() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.items)
}

// Clear clears all items from the project context
func (pc *ProjectContext) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.items = make(map[string]*ContextItem)
}

// GlobalContext represents global context shared across all sessions
type GlobalContext struct {
	items map[string]*ContextItem
	mu    sync.RWMutex
}

// NewGlobalContext creates a new global context
func NewGlobalContext() *GlobalContext {
	return &GlobalContext{
		items: make(map[string]*ContextItem),
	}
}

// Store stores an item in the global context
func (gc *GlobalContext) Store(item *ContextItem) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.items[item.ID] = item
}

// Retrieve retrieves an item from the global context
func (gc *GlobalContext) Retrieve(id string) (*ContextItem, bool) {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	item, exists := gc.items[id]
	return item, exists
}

// Delete deletes an item from the global context
func (gc *GlobalContext) Delete(id string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	delete(gc.items, id)
}

// Size returns the number of items in the global context
func (gc *GlobalContext) Size() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return len(gc.items)
}

// Clear clears all items from the global context
func (gc *GlobalContext) Clear() {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.items = make(map[string]*ContextItem)
}

// Global context manager instance
var globalManager *ContextManager

// GetGlobalManager returns the global context manager
func GetGlobalManager() *ContextManager {
	return globalManager
}

// SetGlobalManager sets the global context manager
func SetGlobalManager(manager *ContextManager) {
	globalManager = manager
}

// InitializeGlobalManager initializes the global context manager
func InitializeGlobalManager(config *config.ContextConfig) {
	globalManager = NewContextManager(config)
}

// StoreGlobal stores an item in the global context
func StoreGlobal(ctx context.Context, item *ContextItem) error {
	if globalManager == nil {
		return fmt.Errorf("%s", tr(ctx, "internal_context_global_manager_not_initialized", nil))
	}
	return globalManager.Store(ctx, item)
}

// RetrieveGlobal retrieves an item from the global context
func RetrieveGlobal(ctx context.Context, id string) (*ContextItem, error) {
	if globalManager == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_context_global_manager_not_initialized", nil))
	}
	return globalManager.Retrieve(ctx, id)
}

// SearchGlobal searches for items in the global context
func SearchGlobal(ctx context.Context, keyPattern string, contextType ContextType) ([]*ContextItem, error) {
	if globalManager == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_context_global_manager_not_initialized", nil))
	}
	return globalManager.Search(ctx, keyPattern, contextType)
}
