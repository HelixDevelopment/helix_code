package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ========================================
// WorkerIsolationManager Tests
// ========================================

func TestNewWorkerIsolationManager(t *testing.T) {
	manager := NewWorkerIsolationManager()

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.sandboxes)
	assert.Len(t, manager.sandboxes, 0)
}

func TestWorkerSandbox_Structure(t *testing.T) {
	id := uuid.New()
	workerID := uuid.New()
	now := time.Now()

	sandbox := WorkerSandbox{
		ID:            id,
		WorkerID:      workerID,
		Directory:     "/tmp/helix-sandbox-test",
		User:          "helix-test",
		Group:         "helix-test",
		MaxMemory:     4 * 1024 * 1024 * 1024, // 4GB
		MaxCPU:        4.0,
		MaxProcesses:  100,
		NetworkAccess: false,
		CreatedAt:     now,
		LastUsed:      now,
	}

	assert.Equal(t, id, sandbox.ID)
	assert.Equal(t, workerID, sandbox.WorkerID)
	assert.Equal(t, "/tmp/helix-sandbox-test", sandbox.Directory)
	assert.Equal(t, "helix-test", sandbox.User)
	assert.Equal(t, int64(4*1024*1024*1024), sandbox.MaxMemory)
	assert.Equal(t, 4.0, sandbox.MaxCPU)
	assert.Equal(t, 100, sandbox.MaxProcesses)
	assert.False(t, sandbox.NetworkAccess)
}

func TestWorkerIsolationManager_GetSandbox_NotFound(t *testing.T) {
	manager := NewWorkerIsolationManager()

	sandbox, err := manager.GetSandbox(uuid.New())
	assert.Nil(t, sandbox)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox not found")
}

func TestWorkerIsolationManager_ListSandboxes_Empty(t *testing.T) {
	manager := NewWorkerIsolationManager()

	sandboxes := manager.ListSandboxes()
	assert.NotNil(t, sandboxes)
	assert.Equal(t, 0, len(sandboxes))
}

func TestWorkerIsolationManager_ListSandboxes_WithSandboxes(t *testing.T) {
	manager := NewWorkerIsolationManager()

	// Add sandboxes directly to the map for testing
	sandbox1 := &WorkerSandbox{
		ID:        uuid.New(),
		WorkerID:  uuid.New(),
		Directory: "/tmp/sandbox1",
		CreatedAt: time.Now(),
	}
	sandbox2 := &WorkerSandbox{
		ID:        uuid.New(),
		WorkerID:  uuid.New(),
		Directory: "/tmp/sandbox2",
		CreatedAt: time.Now(),
	}

	manager.mutex.Lock()
	manager.sandboxes[sandbox1.ID] = sandbox1
	manager.sandboxes[sandbox2.ID] = sandbox2
	manager.mutex.Unlock()

	sandboxes := manager.ListSandboxes()
	assert.Equal(t, 2, len(sandboxes))
}

func TestWorkerIsolationManager_GetSandbox_Found(t *testing.T) {
	manager := NewWorkerIsolationManager()

	sandbox := &WorkerSandbox{
		ID:        uuid.New(),
		WorkerID:  uuid.New(),
		Directory: "/tmp/test-sandbox",
		CreatedAt: time.Now(),
	}

	manager.mutex.Lock()
	manager.sandboxes[sandbox.ID] = sandbox
	manager.mutex.Unlock()

	retrieved, err := manager.GetSandbox(sandbox.ID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, sandbox.ID, retrieved.ID)
	assert.Equal(t, sandbox.Directory, retrieved.Directory)
}

func TestWorkerIsolationManager_CleanupExpiredSandboxes(t *testing.T) {
	manager := NewWorkerIsolationManager()

	// Add an expired sandbox (last used over 1 hour ago)
	expiredSandbox := &WorkerSandbox{
		ID:        uuid.New(),
		WorkerID:  uuid.New(),
		Directory: "/tmp/expired-sandbox",
		LastUsed:  time.Now().Add(-2 * time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	// Add a recent sandbox
	recentSandbox := &WorkerSandbox{
		ID:        uuid.New(),
		WorkerID:  uuid.New(),
		Directory: "/tmp/recent-sandbox",
		LastUsed:  time.Now(),
		CreatedAt: time.Now(),
	}

	manager.mutex.Lock()
	manager.sandboxes[expiredSandbox.ID] = expiredSandbox
	manager.sandboxes[recentSandbox.ID] = recentSandbox
	manager.mutex.Unlock()

	ctx := context.Background()

	// Clean up sandboxes older than 1 hour (async cleanup)
	manager.CleanupExpiredSandboxes(ctx, 1*time.Hour)

	// Give async cleanup a moment to complete
	time.Sleep(100 * time.Millisecond)

	// Recent sandbox should still be retrievable
	retrieved, err := manager.GetSandbox(recentSandbox.ID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
}

func TestWorkerIsolationManager_CleanupExpiredSandboxes_NoneExpired(t *testing.T) {
	manager := NewWorkerIsolationManager()

	// Add recent sandboxes
	sandbox1 := &WorkerSandbox{
		ID:        uuid.New(),
		LastUsed:  time.Now(),
		CreatedAt: time.Now(),
	}
	sandbox2 := &WorkerSandbox{
		ID:        uuid.New(),
		LastUsed:  time.Now().Add(-30 * time.Minute),
		CreatedAt: time.Now().Add(-30 * time.Minute),
	}

	manager.mutex.Lock()
	manager.sandboxes[sandbox1.ID] = sandbox1
	manager.sandboxes[sandbox2.ID] = sandbox2
	manager.mutex.Unlock()

	ctx := context.Background()

	// Clean up sandboxes older than 1 hour
	manager.CleanupExpiredSandboxes(ctx, 1*time.Hour)

	// Give async cleanup a moment to complete
	time.Sleep(100 * time.Millisecond)

	// Both sandboxes should still exist since they're not expired
	assert.Equal(t, 2, len(manager.ListSandboxes()))
}

func TestWorkerIsolationManager_CleanupSandbox_NotFound(t *testing.T) {
	manager := NewWorkerIsolationManager()
	ctx := context.Background()

	// Try to clean up non-existent sandbox
	err := manager.CleanupSandbox(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWorkerIsolationManager_Concurrent(t *testing.T) {
	manager := NewWorkerIsolationManager()

	// Test concurrent access
	done := make(chan bool, 10)

	// Concurrent writers
	for i := 0; i < 5; i++ {
		go func(idx int) {
			sandbox := &WorkerSandbox{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				LastUsed:  time.Now(),
			}
			manager.mutex.Lock()
			manager.sandboxes[sandbox.ID] = sandbox
			manager.mutex.Unlock()
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			_ = manager.ListSandboxes()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 5 sandboxes
	assert.Len(t, manager.ListSandboxes(), 5)
}
