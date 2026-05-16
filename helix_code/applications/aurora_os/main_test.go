//go:build !nogui

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuroraAppCreation(t *testing.T) {
	// Test that we can create the basic structs without panicking
	// Note: Full GUI testing would require a display environment

	aurora := &AuroraIntegration{
		nativeServices: make(map[string]interface{}),
	}

	assert.NotNil(t, aurora)
	assert.NotNil(t, aurora.nativeServices)
}

func TestAuroraSystemMonitor(t *testing.T) {
	monitor := &AuroraSystemMonitor{
		networkStats: make(map[string]interface{}),
	}

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.networkStats)
}

func TestAuroraSecurityManager(t *testing.T) {
	security := NewAuroraSecurityManager()

	assert.NotNil(t, security)
	assert.NotNil(t, security.accessControl)
	assert.NotNil(t, security.auditLog)
	assert.True(t, security.encryptionEnabled)
	assert.Equal(t, "AES-256-GCM", security.encryptionAlgo)
}

func TestAuroraSecurityManagerAuditLog(t *testing.T) {
	security := NewAuroraSecurityManager()

	// Add audit entry
	security.AddAuditEntry("test_action", "test_user", "test details", "info")

	entries := security.GetAuditLog()
	assert.Len(t, entries, 1)
	assert.Equal(t, "test_action", entries[0].Action)
	assert.Equal(t, "test_user", entries[0].User)
	assert.Equal(t, "test details", entries[0].Details)
	assert.Equal(t, "info", entries[0].Severity)
}

func TestAuroraTaskManager(t *testing.T) {
	tm := NewAuroraTaskManager(nil)

	assert.NotNil(t, tm)
	assert.Empty(t, tm.GetAllTasks())

	stats := tm.GetStats()
	assert.Equal(t, 0, stats.TotalTasks)
	assert.Equal(t, 0, stats.CompletedTasks)
	assert.Equal(t, 0, stats.RunningTasks)
	assert.Equal(t, 0, stats.PendingTasks)
}

func TestAuroraWorkerManager(t *testing.T) {
	wm := NewAuroraWorkerManager(nil)

	assert.NotNil(t, wm)
	assert.Empty(t, wm.GetWorkers())
}

func TestUIWorker(t *testing.T) {
	worker := &UIWorker{
		ID:      "test-worker",
		Host:    "192.168.1.100",
		Port:    "22",
		User:    "deploy",
		Status:  "active",
		Healthy: true,
	}

	assert.Equal(t, "test-worker", worker.ID)
	assert.Equal(t, "192.168.1.100", worker.Host)
	assert.True(t, worker.Healthy)
}

func TestUITask(t *testing.T) {
	task := &UITask{
		ID:          "test-task",
		Type:        "building",
		Description: "Test task",
		Status:      "pending",
		Priority:    "high",
	}

	assert.Equal(t, "test-task", task.ID)
	assert.Equal(t, "building", task.Type)
	assert.Equal(t, "pending", task.Status)
}

func TestThemeManager(t *testing.T) {
	tm := NewThemeManager()

	assert.NotNil(t, tm)
	assert.NotNil(t, tm.GetCurrentTheme())

	themes := tm.GetAvailableThemes()
	assert.NotEmpty(t, themes)
}

func TestThemeManagerSetTheme(t *testing.T) {
	tm := NewThemeManager()

	// Test setting different themes
	success := tm.SetTheme("dark")
	assert.True(t, success)
	assert.Equal(t, "Dark", tm.GetCurrentTheme().Name)

	success = tm.SetTheme("light")
	assert.True(t, success)
	assert.Equal(t, "Light", tm.GetCurrentTheme().Name)

	// Test invalid theme
	success = tm.SetTheme("nonexistent")
	assert.False(t, success)
}

func TestThemeManagerGetColor(t *testing.T) {
	tm := NewThemeManager()
	tm.SetTheme("aurora")

	primary := tm.GetColor("primary")
	assert.NotEmpty(t, primary)
	assert.True(t, primary[0] == '#')
}
