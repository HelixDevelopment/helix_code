//go:build nogui

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLIAppCreation(t *testing.T) {
	app := NewCLIApp()

	assert.NotNil(t, app)
	assert.NotNil(t, app.securityManager)
	assert.NotNil(t, app.diagnosticsLog)
}

func TestCLISecurityManager(t *testing.T) {
	sm := NewAuroraSecurityManager()

	assert.NotNil(t, sm)
	assert.True(t, sm.encryptionEnabled)
	assert.NotNil(t, sm.accessControl)
	assert.NotNil(t, sm.auditLog)

	// Test adding audit entry
	sm.AddAuditEntry("test_action", "test_user", "test details", "info")

	entries := sm.GetAuditLog()
	assert.Len(t, entries, 1)
	assert.Equal(t, "test_action", entries[0].Action)
	assert.Equal(t, "test_user", entries[0].User)
	assert.Equal(t, "test details", entries[0].Details)
	assert.Equal(t, "info", entries[0].Severity)
}

func TestCLITaskManager(t *testing.T) {
	tm := NewCLITaskManager(nil)

	assert.NotNil(t, tm)
	assert.Empty(t, tm.GetAllTasks())

	// Test stats
	total, completed, running := tm.GetStats()
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, completed)
	assert.Equal(t, 0, running)

	// Test creating a task
	ctx := context.Background()
	task, err := tm.CreateTask(ctx, "building", "Test task", "high")
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "building", task.Type)
	assert.Equal(t, "Test task", task.Description)
	assert.Equal(t, "high", task.Priority)
	assert.Equal(t, "pending", task.Status)

	// Verify task was added
	total, _, _ = tm.GetStats()
	assert.Equal(t, 1, total)

	// Test cancel task
	err = tm.CancelTask(ctx, task.ID)
	assert.NoError(t, err)

	total, _, _ = tm.GetStats()
	assert.Equal(t, 0, total)

	// Test cancel non-existent task
	err = tm.CancelTask(ctx, "non-existent")
	assert.Error(t, err)
}

func TestCLIWorkerManager(t *testing.T) {
	wm := NewCLIWorkerManager(nil)

	assert.NotNil(t, wm)
	assert.Empty(t, wm.GetWorkers())

	// Test adding a worker
	worker := &CLIWorker{
		ID:      "test-worker",
		Host:    "192.168.1.100",
		Port:    "22",
		User:    "deploy",
		Status:  "pending",
		Healthy: false,
	}
	err := wm.AddWorker(worker)
	assert.NoError(t, err)

	workers := wm.GetWorkers()
	assert.Len(t, workers, 1)
	assert.Equal(t, "test-worker", workers[0].ID)

	// Test removing a worker
	err = wm.RemoveWorker("test-worker")
	assert.NoError(t, err)
	assert.Empty(t, wm.GetWorkers())

	// Test removing non-existent worker
	err = wm.RemoveWorker("non-existent")
	assert.Error(t, err)
}

func TestCLITask(t *testing.T) {
	task := CLITask{
		ID:          "test-id",
		Type:        "building",
		Description: "Test task",
		Status:      "pending",
		Priority:    "high",
	}

	assert.Equal(t, "test-id", task.ID)
	assert.Equal(t, "building", task.Type)
	assert.Equal(t, "Test task", task.Description)
	assert.Equal(t, "pending", task.Status)
	assert.Equal(t, "high", task.Priority)
}

func TestCLIWorker(t *testing.T) {
	worker := CLIWorker{
		ID:      "worker-1",
		Host:    "192.168.1.100",
		Port:    "22",
		User:    "deploy",
		Status:  "active",
		Healthy: true,
	}

	assert.Equal(t, "worker-1", worker.ID)
	assert.Equal(t, "192.168.1.100", worker.Host)
	assert.Equal(t, "22", worker.Port)
	assert.Equal(t, "deploy", worker.User)
	assert.Equal(t, "active", worker.Status)
	assert.True(t, worker.Healthy)
}

func TestAuditLogEntry(t *testing.T) {
	sm := NewAuroraSecurityManager()

	// Add multiple entries
	sm.AddAuditEntry("action1", "user1", "details1", "info")
	sm.AddAuditEntry("action2", "user2", "details2", "warning")
	sm.AddAuditEntry("action3", "user3", "details3", "error")

	entries := sm.GetAuditLog()
	assert.Len(t, entries, 3)

	assert.Equal(t, "action1", entries[0].Action)
	assert.Equal(t, "action2", entries[1].Action)
	assert.Equal(t, "action3", entries[2].Action)

	assert.Equal(t, "info", entries[0].Severity)
	assert.Equal(t, "warning", entries[1].Severity)
	assert.Equal(t, "error", entries[2].Severity)
}

func TestAccessControl(t *testing.T) {
	sm := NewAuroraSecurityManager()

	// Check default roles
	assert.Contains(t, sm.accessControl, "admin")
	assert.Contains(t, sm.accessControl, "developer")
	assert.Contains(t, sm.accessControl, "viewer")

	// Check admin permissions
	adminPerms := sm.accessControl["admin"]
	assert.Contains(t, adminPerms, "read")
	assert.Contains(t, adminPerms, "write")
	assert.Contains(t, adminPerms, "execute")
	assert.Contains(t, adminPerms, "admin")

	// Check developer permissions
	devPerms := sm.accessControl["developer"]
	assert.Contains(t, devPerms, "read")
	assert.Contains(t, devPerms, "write")
	assert.Contains(t, devPerms, "execute")
	assert.NotContains(t, devPerms, "admin")

	// Check viewer permissions
	viewerPerms := sm.accessControl["viewer"]
	assert.Contains(t, viewerPerms, "read")
	assert.NotContains(t, viewerPerms, "write")
}
