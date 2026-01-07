package context

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

func TestNewContextManager(t *testing.T) {
	config := &config.ContextConfig{}
	manager := NewContextManager(config)

	if manager == nil {
		t.Fatal("NewContextManager returned nil")
	}

	if len(manager.items) != 0 {
		t.Error("New manager should have no items")
	}

	if len(manager.sessions) != 0 {
		t.Error("New manager should have no sessions")
	}

	if len(manager.projects) != 0 {
		t.Error("New manager should have no projects")
	}

	if manager.global == nil {
		t.Error("New manager should have global context")
	}
}

func TestContextItemIsExpired(t *testing.T) {
	item := &ContextItem{
		ID:        "test-item",
		Timestamp: time.Now(),
	}

	// No TTL - not expired
	if item.IsExpired() {
		t.Error("Item without TTL should not be expired")
	}

	// Past TTL - expired
	ttl := 1 * time.Hour
	item.TTL = &ttl
	item.Timestamp = time.Now().Add(-2 * time.Hour) // 2 hours ago

	if !item.IsExpired() {
		t.Error("Item with expired TTL should be expired")
	}

	// Future TTL - not expired
	item.Timestamp = time.Now()
	if item.IsExpired() {
		t.Error("Item with valid TTL should not be expired")
	}
}

func TestContextManagerStoreRetrieve(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	item := &ContextItem{
		ID:   "test-item",
		Type: ContextTypeGlobal,
		Key:  "test-key",
		Value: map[string]interface{}{
			"data": "test value",
		},
		Metadata: map[string]interface{}{
			"source": "test",
		},
	}

	ctx := context.Background()

	// Store item
	err := manager.Store(ctx, item)
	if err != nil {
		t.Fatalf("Failed to store item: %v", err)
	}

	// Retrieve item
	retrieved, err := manager.Retrieve(ctx, "test-item")
	if err != nil {
		t.Fatalf("Failed to retrieve item: %v", err)
	}

	if retrieved.ID != "test-item" {
		t.Errorf("Expected ID 'test-item', got '%s'", retrieved.ID)
	}

	if retrieved.Key != "test-key" {
		t.Errorf("Expected key 'test-key', got '%s'", retrieved.Key)
	}

	if retrieved.Type != ContextTypeGlobal {
		t.Errorf("Expected type Global, got %v", retrieved.Type)
	}
}

func TestContextManagerStoreRetrieveExpired(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	ttl := 1 * time.Second
	item := &ContextItem{
		ID:        "expired-item",
		Type:      ContextTypeGlobal,
		Key:       "expired-key",
		Timestamp: time.Now().Add(-2 * time.Second), // Already expired
		TTL:       &ttl,
	}

	ctx := context.Background()

	// Store expired item
	err := manager.Store(ctx, item)
	if err != nil {
		t.Fatalf("Failed to store expired item: %v", err)
	}

	// Try to retrieve expired item
	_, err = manager.Retrieve(ctx, "expired-item")
	if err == nil {
		t.Error("Expected error when retrieving expired item")
	}
}

func TestContextManagerSearch(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	// Store multiple items
	items := []*ContextItem{
		{
			ID:   "item1",
			Type: ContextTypeGlobal,
			Key:  "key1",
		},
		{
			ID:   "item2",
			Type: ContextTypeSession,
			Key:  "key2",
		},
		{
			ID:   "item3",
			Type: ContextTypeGlobal,
			Key:  "key3",
		},
	}

	ctx := context.Background()
	for _, item := range items {
		manager.Store(ctx, item)
	}

	// Search for global items
	results, err := manager.Search(ctx, "", ContextTypeGlobal)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 global items, got %d", len(results))
	}

	// Search for session items
	results, err = manager.Search(ctx, "", ContextTypeSession)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 session item, got %d", len(results))
	}
}

func TestContextManagerDelete(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	item := &ContextItem{
		ID:   "delete-test",
		Type: ContextTypeGlobal,
		Key:  "delete-key",
	}

	ctx := context.Background()

	// Store item
	manager.Store(ctx, item)

	// Verify it exists
	_, err := manager.Retrieve(ctx, "delete-test")
	if err != nil {
		t.Fatalf("Item should exist before deletion: %v", err)
	}

	// Delete item
	err = manager.Delete(ctx, "delete-test")
	if err != nil {
		t.Fatalf("Failed to delete item: %v", err)
	}

	// Verify it's gone
	_, err = manager.Retrieve(ctx, "delete-test")
	if err == nil {
		t.Error("Item should not exist after deletion")
	}
}

func TestContextManagerClear(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	// Store some items
	items := []*ContextItem{
		{ID: "clear1", Type: ContextTypeGlobal, Key: "key1"},
		{ID: "clear2", Type: ContextTypeSession, Key: "key2"},
		{ID: "clear3", Type: ContextTypeProject, Key: "key3"},
	}

	ctx := context.Background()
	for _, item := range items {
		manager.Store(ctx, item)
	}

	// Clear all
	err := manager.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	// Verify all are gone
	for _, item := range items {
		_, err := manager.Retrieve(ctx, item.ID)
		if err == nil {
			t.Errorf("Item %s should not exist after clear", item.ID)
		}
	}
}

func TestContextManagerSessionContext(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	sessionID := "test-session"

	// Store session item
	item := &ContextItem{
		ID:   "session-item",
		Type: ContextTypeSession,
		Key:  "session-key",
		Metadata: map[string]interface{}{
			"session_id": sessionID,
		},
	}

	ctx := context.Background()
	manager.Store(ctx, item)

	// Get session context
	session, err := manager.GetSessionContext(sessionID)
	if err != nil {
		t.Fatalf("Failed to get session context: %v", err)
	}

	if session.sessionID != sessionID {
		t.Errorf("Expected session ID '%s', got '%s'", sessionID, session.sessionID)
	}

	// Check item exists in session
	retrieved, exists := session.Retrieve("session-item")
	if !exists {
		t.Error("Item should exist in session context")
	}

	if retrieved.Key != "session-key" {
		t.Errorf("Expected key 'session-key', got '%s'", retrieved.Key)
	}
}

func TestContextManagerProjectContext(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	projectID := "test-project"

	// Store project item
	item := &ContextItem{
		ID:   "project-item",
		Type: ContextTypeProject,
		Key:  "project-key",
		Metadata: map[string]interface{}{
			"project_id": projectID,
		},
	}

	ctx := context.Background()
	manager.Store(ctx, item)

	// Get project context
	project, err := manager.GetProjectContext(projectID)
	if err != nil {
		t.Fatalf("Failed to get project context: %v", err)
	}

	if project.projectID != projectID {
		t.Errorf("Expected project ID '%s', got '%s'", projectID, project.projectID)
	}

	// Check item exists in project
	retrieved, exists := project.Retrieve("project-item")
	if !exists {
		t.Error("Item should exist in project context")
	}

	if retrieved.Key != "project-key" {
		t.Errorf("Expected key 'project-key', got '%s'", retrieved.Key)
	}
}

func TestContextManagerGlobalContext(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	global := manager.GetGlobalContext()
	if global == nil {
		t.Fatal("Global context should not be nil")
	}

	// Store global item
	item := &ContextItem{
		ID:   "global-item",
		Type: ContextTypeGlobal,
		Key:  "global-key",
	}

	ctx := context.Background()
	manager.Store(ctx, item)

	// Check item exists in global context
	retrieved, exists := global.Retrieve("global-item")
	if !exists {
		t.Error("Item should exist in global context")
	}

	if retrieved.Key != "global-key" {
		t.Errorf("Expected key 'global-key', got '%s'", retrieved.Key)
	}
}

func TestSessionContext(t *testing.T) {
	session := NewSessionContext("test-session")

	if session.sessionID != "test-session" {
		t.Errorf("Expected session ID 'test-session', got '%s'", session.sessionID)
	}

	// Test empty session
	if session.Size() != 0 {
		t.Errorf("Expected size 0, got %d", session.Size())
	}

	// Store item
	item := &ContextItem{ID: "test", Key: "key"}
	session.Store(item)

	if session.Size() != 1 {
		t.Errorf("Expected size 1 after store, got %d", session.Size())
	}

	// Retrieve item
	retrieved, exists := session.Retrieve("test")
	if !exists {
		t.Error("Item should exist")
	}

	if retrieved.Key != "key" {
		t.Errorf("Expected key 'key', got '%s'", retrieved.Key)
	}

	// Delete item
	session.Delete("test")

	if session.Size() != 0 {
		t.Errorf("Expected size 0 after delete, got %d", session.Size())
	}

	// Clear session
	session.Store(item)
	session.Clear()

	if session.Size() != 0 {
		t.Error("Session should be empty after clear")
	}
}

func TestProjectContext(t *testing.T) {
	project := NewProjectContext("test-project")

	if project.projectID != "test-project" {
		t.Errorf("Expected project ID 'test-project', got '%s'", project.projectID)
	}

	// Test empty project
	if project.Size() != 0 {
		t.Errorf("Expected size 0, got %d", project.Size())
	}

	// Store item
	item := &ContextItem{ID: "test", Key: "key"}
	project.Store(item)

	if project.Size() != 1 {
		t.Errorf("Expected size 1 after store, got %d", project.Size())
	}

	// Retrieve item
	retrieved, exists := project.Retrieve("test")
	if !exists {
		t.Error("Item should exist")
	}

	if retrieved.Key != "key" {
		t.Errorf("Expected key 'key', got '%s'", retrieved.Key)
	}

	// Delete item
	project.Delete("test")

	if project.Size() != 0 {
		t.Errorf("Expected size 0 after delete, got %d", project.Size())
	}

	// Clear project
	project.Store(item)
	project.Clear()

	if project.Size() != 0 {
		t.Error("Project should be empty after clear")
	}
}

func TestGlobalContext(t *testing.T) {
	global := NewGlobalContext()

	// Test empty global
	if global.Size() != 0 {
		t.Errorf("Expected size 0, got %d", global.Size())
	}

	// Store item
	item := &ContextItem{ID: "test", Key: "key"}
	global.Store(item)

	if global.Size() != 1 {
		t.Errorf("Expected size 1 after store, got %d", global.Size())
	}

	// Retrieve item
	retrieved, exists := global.Retrieve("test")
	if !exists {
		t.Error("Item should exist")
	}

	if retrieved.Key != "key" {
		t.Errorf("Expected key 'key', got '%s'", retrieved.Key)
	}

	// Delete item
	global.Delete("test")

	if global.Size() != 0 {
		t.Errorf("Expected size 0 after delete, got %d", global.Size())
	}

	// Clear global
	global.Store(item)
	global.Clear()

	if global.Size() != 0 {
		t.Error("Global should be empty after clear")
	}
}

func TestContextManagerStatistics(t *testing.T) {
	manager := NewContextManager(&config.ContextConfig{})

	// Store items of different types
	items := []*ContextItem{
		{ID: "global1", Type: ContextTypeGlobal, Key: "key1"},
		{ID: "global2", Type: ContextTypeGlobal, Key: "key2"},
		{ID: "session1", Type: ContextTypeSession, Key: "key3", Metadata: map[string]interface{}{"session_id": "session1"}},
		{ID: "project1", Type: ContextTypeProject, Key: "key4", Metadata: map[string]interface{}{"project_id": "project1"}},
	}

	ctx := context.Background()
	for _, item := range items {
		manager.Store(ctx, item)
	}

	stats := manager.GetStatistics()

	if stats["total_items"] != 4 {
		t.Errorf("Expected 4 total items, got %v", stats["total_items"])
	}

	if stats["total_sessions"] != 1 {
		t.Errorf("Expected 1 total session, got %v", stats["total_sessions"])
	}

	if stats["total_projects"] != 1 {
		t.Errorf("Expected 1 total project, got %v", stats["total_projects"])
	}

	itemsByType := stats["items_by_type"].(map[string]int)
	if itemsByType["global"] != 2 {
		t.Errorf("Expected 2 global items, got %d", itemsByType["global"])
	}

	if itemsByType["session"] != 1 {
		t.Errorf("Expected 1 session item, got %d", itemsByType["session"])
	}

	if itemsByType["project"] != 1 {
		t.Errorf("Expected 1 project item, got %d", itemsByType["project"])
	}
}

func TestGlobalManager(t *testing.T) {
	// Initialize global manager
	config := &config.ContextConfig{}
	InitializeGlobalManager(config)

	manager := GetGlobalManager()
	if manager == nil {
		t.Fatal("Global manager not initialized")
	}

	// Test global functions
	item := &ContextItem{
		ID:   "global-test",
		Type: ContextTypeGlobal,
		Key:  "global-key",
	}

	ctx := context.Background()

	err := StoreGlobal(ctx, item)
	if err != nil {
		t.Fatalf("Failed to store globally: %v", err)
	}

	retrieved, err := RetrieveGlobal(ctx, "global-test")
	if err != nil {
		t.Fatalf("Failed to retrieve globally: %v", err)
	}

	if retrieved.Key != "global-key" {
		t.Errorf("Expected key 'global-key', got '%s'", retrieved.Key)
	}

	results, err := SearchGlobal(ctx, "", ContextTypeGlobal)
	if err != nil {
		t.Fatalf("Failed to search globally: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one search result")
	}
}

func BenchmarkContextManagerStore(b *testing.B) {
	manager := NewContextManager(&config.ContextConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := &ContextItem{
			ID:   fmt.Sprintf("bench-item-%d", i),
			Type: ContextTypeGlobal,
			Key:  fmt.Sprintf("bench-key-%d", i),
		}
		ctx := context.Background()
		manager.Store(ctx, item)
	}
}

func BenchmarkContextManagerRetrieve(b *testing.B) {
	manager := NewContextManager(&config.ContextConfig{})

	// Pre-populate
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		item := &ContextItem{
			ID:   fmt.Sprintf("item-%d", i),
			Type: ContextTypeGlobal,
			Key:  fmt.Sprintf("key-%d", i),
		}
		manager.Store(ctx, item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("item-%d", i%1000)
		manager.Retrieve(ctx, id)
	}
}

func BenchmarkContextManagerSearch(b *testing.B) {
	manager := NewContextManager(&config.ContextConfig{})

	// Pre-populate
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		item := &ContextItem{
			ID:   fmt.Sprintf("search-item-%d", i),
			Type: ContextTypeGlobal,
			Key:  fmt.Sprintf("search-key-%d", i),
		}
		manager.Store(ctx, item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Search(ctx, "", ContextTypeGlobal)
	}
}

func BenchmarkSessionContextStore(b *testing.B) {
	session := NewSessionContext("bench-session")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := &ContextItem{
			ID:  fmt.Sprintf("session-item-%d", i),
			Key: fmt.Sprintf("session-key-%d", i),
		}
		session.Store(item)
	}
}

func BenchmarkGlobalContextStore(b *testing.B) {
	global := NewGlobalContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := &ContextItem{
			ID:  fmt.Sprintf("global-item-%d", i),
			Key: fmt.Sprintf("global-key-%d", i),
		}
		global.Store(item)
	}
}
