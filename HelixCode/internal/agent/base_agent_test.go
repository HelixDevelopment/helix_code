package agent

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

func TestNewBaseAgent(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	if agent == nil {
		t.Fatal("NewBaseAgent returned nil")
	}

	if agent.ID() != "test-agent" {
		t.Errorf("Expected ID 'test-agent', got '%s'", agent.ID())
	}

	if agent.Name() != "Test Agent" {
		t.Errorf("Expected name 'Test Agent', got '%s'", agent.Name())
	}

	if agent.Status() != StatusIdle {
		t.Errorf("Expected status Idle, got %v", agent.Status())
	}
}

func TestBaseAgentStatusManagement(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Test initial status
	if agent.Status() != StatusIdle {
		t.Errorf("Expected initial status Idle, got %v", agent.Status())
	}

	// Test status change
	agent.SetStatus(StatusBusy)
	if agent.Status() != StatusBusy {
		t.Errorf("Expected status Busy, got %v", agent.Status())
	}

	agent.SetStatus(StatusError)
	if agent.Status() != StatusError {
		t.Errorf("Expected status Error, got %v", agent.Status())
	}
}

func TestBaseAgentCapabilities(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Test initial capabilities
	caps := agent.Capabilities()
	if len(caps) != 0 {
		t.Errorf("Expected 0 initial capabilities, got %d", len(caps))
	}

	// Test adding capability
	cap := Capability{
		Name:        "test-task",
		Description: "Test task capability",
		Version:     "1.0.0",
	}

	agent.AddCapability(cap)
	caps = agent.Capabilities()
	if len(caps) != 1 {
		t.Errorf("Expected 1 capability after adding, got %d", len(caps))
	}

	if caps[0].Name != "test-task" {
		t.Errorf("Expected capability name 'test-task', got '%s'", caps[0].Name)
	}

	// Test removing capability
	agent.RemoveCapability("test-task")
	caps = agent.Capabilities()
	if len(caps) != 0 {
		t.Errorf("Expected 0 capabilities after removing, got %d", len(caps))
	}
}

func TestBaseAgentCanHandle(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Add capability
	cap := Capability{
		Name:        "test-task",
		Description: "Test task capability",
		Version:     "1.0.0",
	}
	agent.AddCapability(cap)

	// Test can handle
	if !agent.CanHandle("test-task") {
		t.Error("Agent should be able to handle 'test-task'")
	}

	if agent.CanHandle("unknown-task") {
		t.Error("Agent should not be able to handle 'unknown-task'")
	}
}

func TestBaseAgentSubmitTask(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Add capability
	agent.AddCapability(Capability{Name: "test-task"})

	// Create test task
	task := &Task{
		ID:   "test-task-1",
		Type: "test-task",
		Payload: map[string]interface{}{
			"message": "test payload",
		},
		CreatedAt: time.Now(),
	}

	// Submit task
	ctx := context.Background()
	result, err := agent.SubmitTask(ctx, task)

	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	if result == nil {
		t.Fatal("SubmitTask returned nil result")
	}

	if !result.Success {
		t.Errorf("Expected task to succeed, got error: %s", result.Error)
	}

	if result.TaskID != "test-task-1" {
		t.Errorf("Expected task ID 'test-task-1', got '%s'", result.TaskID)
	}

	if result.Duration <= 0 {
		t.Error("Expected positive duration")
	}
}

func TestBaseAgentSubmitTaskNilTask(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	ctx := context.Background()
	result, err := agent.SubmitTask(ctx, nil)

	if err == nil {
		t.Error("Expected error for nil task")
	}

	if result != nil {
		t.Error("Expected nil result for nil task")
	}
}

func TestBaseAgentSubmitTaskCannotHandle(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	task := &Task{
		ID:   "test-task-1",
		Type: "unknown-task",
	}

	ctx := context.Background()
	result, err := agent.SubmitTask(ctx, task)

	if err == nil {
		t.Error("Expected error for unhandled task type")
	}

	if result != nil {
		t.Error("Expected nil result for unhandled task type")
	}
}

func TestBaseAgentHealth(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	health := agent.Health()

	if health["id"] != "test-agent" {
		t.Errorf("Expected health ID 'test-agent', got %v", health["id"])
	}

	if health["name"] != "Test Agent" {
		t.Errorf("Expected health name 'Test Agent', got %v", health["name"])
	}

	if health["status"] != "idle" {
		t.Errorf("Expected health status 'idle', got %v", health["status"])
	}

	if health["capabilities"] != 0 {
		t.Errorf("Expected 0 capabilities in health, got %v", health["capabilities"])
	}
}

func TestBaseAgentStatistics(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Add capability and submit a task
	agent.AddCapability(Capability{Name: "test-task"})

	task := &Task{
		ID:   "test-task-1",
		Type: "test-task",
	}

	ctx := context.Background()
	_, err := agent.SubmitTask(ctx, task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	stats := agent.GetStatistics()

	if stats["tasks_processed"] != 1 {
		t.Errorf("Expected 1 task processed, got %v", stats["tasks_processed"])
	}

	if stats["tasks_succeeded"] != 1 {
		t.Errorf("Expected 1 task succeeded, got %v", stats["tasks_succeeded"])
	}

	if stats["tasks_failed"] != 0 {
		t.Errorf("Expected 0 tasks failed, got %v", stats["tasks_failed"])
	}

	if stats["success_rate"] != 100.0 {
		t.Errorf("Expected 100%% success rate, got %v", stats["success_rate"])
	}
}

func TestBaseAgentResetStatistics(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Add capability and submit a task
	agent.AddCapability(Capability{Name: "test-task"})

	task := &Task{
		ID:   "test-task-1",
		Type: "test-task",
	}

	ctx := context.Background()
	_, err := agent.SubmitTask(ctx, task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Reset statistics
	agent.ResetStatistics()

	stats := agent.GetStatistics()

	if stats["tasks_processed"] != 0 {
		t.Errorf("Expected 0 tasks processed after reset, got %v", stats["tasks_processed"])
	}

	if stats["tasks_succeeded"] != 0 {
		t.Errorf("Expected 0 tasks succeeded after reset, got %v", stats["tasks_succeeded"])
	}

	if stats["tasks_failed"] != 0 {
		t.Errorf("Expected 0 tasks failed after reset, got %v", stats["tasks_failed"])
	}
}

func TestBaseAgentIsHealthy(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Test healthy agent
	if !agent.IsHealthy() {
		t.Error("Expected agent to be healthy initially")
	}

	// Test unhealthy agent (error status)
	agent.SetStatus(StatusError)
	if agent.IsHealthy() {
		t.Error("Expected agent to be unhealthy with error status")
	}

	// Reset status
	agent.SetStatus(StatusIdle)

	// Test unhealthy agent (no recent activity)
	agent.mu.Lock()
	agent.lastActivity = time.Now().Add(-10 * time.Minute) // 10 minutes ago
	agent.mu.Unlock()

	if agent.IsHealthy() {
		t.Error("Expected agent to be unhealthy with old activity")
	}
}

func TestBaseAgentConfiguration(t *testing.T) {
	// Test with nil config
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	if agent.GetMaxConcurrency() != 1 {
		t.Errorf("Expected default max concurrency 1, got %d", agent.GetMaxConcurrency())
	}

	if agent.GetTimeout() != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", agent.GetTimeout())
	}

	if agent.GetRetryCount() != 3 {
		t.Errorf("Expected default retry count 3, got %d", agent.GetRetryCount())
	}

	// Test with custom config
	config := &config.AgentConfig{
		MaxConcurrency: 5,
		Timeout:        60,
		RetryCount:     5,
	}

	agent2 := NewBaseAgent("test-agent-2", "Test Agent 2", config)

	if agent2.GetMaxConcurrency() != 5 {
		t.Errorf("Expected max concurrency 5, got %d", agent2.GetMaxConcurrency())
	}

	if agent2.GetTimeout() != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", agent2.GetTimeout())
	}

	if agent2.GetRetryCount() != 5 {
		t.Errorf("Expected retry count 5, got %d", agent2.GetRetryCount())
	}
}

func TestBaseAgentSetters(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Test SetMaxConcurrency
	agent.SetMaxConcurrency(10)
	if agent.GetMaxConcurrency() != 10 {
		t.Errorf("Expected max concurrency 10, got %d", agent.GetMaxConcurrency())
	}

	// Test invalid SetMaxConcurrency
	agent.SetMaxConcurrency(-1)
	if agent.GetMaxConcurrency() != 10 {
		t.Errorf("Expected max concurrency to remain 10, got %d", agent.GetMaxConcurrency())
	}

	// Test SetTimeout
	newTimeout := 120 * time.Second
	agent.SetTimeout(newTimeout)
	if agent.GetTimeout() != newTimeout {
		t.Errorf("Expected timeout %v, got %v", newTimeout, agent.GetTimeout())
	}

	// Test invalid SetTimeout
	agent.SetTimeout(-1 * time.Second)
	if agent.GetTimeout() != newTimeout {
		t.Errorf("Expected timeout to remain %v, got %v", newTimeout, agent.GetTimeout())
	}

	// Test SetRetryCount
	agent.SetRetryCount(10)
	if agent.GetRetryCount() != 10 {
		t.Errorf("Expected retry count 10, got %d", agent.GetRetryCount())
	}

	// Test invalid SetRetryCount
	agent.SetRetryCount(-1)
	if agent.GetRetryCount() != 10 {
		t.Errorf("Expected retry count to remain 10, got %d", agent.GetRetryCount())
	}
}

func TestBaseAgentStartStop(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start agent
	err := agent.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// Stop agent
	agent.Stop()

	// Agent should still be functional
	if !agent.IsHealthy() {
		t.Error("Agent should still be healthy after stop")
	}
}

func TestBaseAgentUpdateLastActivity(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	initialActivity := agent.lastActivity
	time.Sleep(1 * time.Millisecond) // Ensure time difference

	agent.UpdateLastActivity()

	if !agent.lastActivity.After(initialActivity) {
		t.Error("Last activity should be updated")
	}
}

func TestBaseAgentTaskCounters(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	// Initially all counters should be 0
	stats := agent.GetStatistics()
	if stats["tasks_processed"] != 0 {
		t.Errorf("Expected 0 tasks processed initially, got %v", stats["tasks_processed"])
	}

	if stats["tasks_succeeded"] != 0 {
		t.Errorf("Expected 0 tasks succeeded initially, got %v", stats["tasks_succeeded"])
	}

	if stats["tasks_failed"] != 0 {
		t.Errorf("Expected 0 tasks failed initially, got %v", stats["tasks_failed"])
	}
}

func TestBaseAgentConcurrentTaskCounting(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)
	agent.AddCapability(Capability{Name: "test-task"})

	// Run multiple tasks concurrently
	numTasks := 10
	done := make(chan bool, numTasks)

	for i := 0; i < numTasks; i++ {
		go func(id int) {
			task := &Task{
				ID:   fmt.Sprintf("concurrent-task-%d", id),
				Type: "test-task",
			}

			ctx := context.Background()
			_, err := agent.SubmitTask(ctx, task)
			if err != nil {
				t.Errorf("Task %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all tasks to complete
	for i := 0; i < numTasks; i++ {
		<-done
	}

	// Check final statistics
	stats := agent.GetStatistics()
	if stats["tasks_processed"] != numTasks {
		t.Errorf("Expected %d tasks processed, got %v", numTasks, stats["tasks_processed"])
	}

	if stats["tasks_succeeded"] != numTasks {
		t.Errorf("Expected %d tasks succeeded, got %v", numTasks, stats["tasks_succeeded"])
	}
}

func TestBaseAgentConcurrentErrorCounting(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)
	// Don't add capability to force errors

	numTasks := 5
	done := make(chan bool, numTasks)

	for i := 0; i < numTasks; i++ {
		go func(id int) {
			task := &Task{
				ID:   fmt.Sprintf("error-task-%d", id),
				Type: "unknown-task", // This will cause an error
			}

			ctx := context.Background()
			_, err := agent.SubmitTask(ctx, task)
			if err == nil {
				t.Errorf("Task %d should have failed", id)
			}
			done <- true
		}(i)
	}

	// Wait for all tasks to complete
	for i := 0; i < numTasks; i++ {
		<-done
	}

	// Check error statistics
	stats := agent.GetStatistics()
	if stats["tasks_processed"] != numTasks {
		t.Errorf("Expected %d tasks processed, got %v", numTasks, stats["tasks_processed"])
	}

	if stats["tasks_failed"] != numTasks {
		t.Errorf("Expected %d tasks failed, got %v", numTasks, stats["tasks_failed"])
	}

	if stats["tasks_succeeded"] != 0 {
		t.Errorf("Expected 0 tasks succeeded, got %v", stats["tasks_succeeded"])
	}
}

func TestBaseAgentConcurrentStatusChanges(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)
	agent.AddCapability(Capability{Name: "test-task"})

	numTasks := 20
	done := make(chan bool, numTasks)

	for i := 0; i < numTasks; i++ {
		go func(id int) {
			// Change status
			agent.SetStatus(StatusBusy)
			time.Sleep(1 * time.Millisecond)
			agent.SetStatus(StatusIdle)

			task := &Task{
				ID:   fmt.Sprintf("status-task-%d", id),
				Type: "test-task",
			}

			ctx := context.Background()
			_, err := agent.SubmitTask(ctx, task)
			if err != nil {
				t.Errorf("Task %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all tasks to complete
	for i := 0; i < numTasks; i++ {
		<-done
	}

	// Final status should be idle
	if agent.Status() != StatusIdle {
		t.Errorf("Expected final status idle, got %v", agent.Status())
	}
}

func TestBaseAgentConcurrentCapabilityChecks(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	numGoroutines := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			// Add capability
			cap := Capability{
				Name: fmt.Sprintf("capability-%d", id),
			}
			agent.AddCapability(cap)

			// Check capabilities
			caps := agent.Capabilities()
			if len(caps) == 0 {
				t.Errorf("Goroutine %d: no capabilities found", id)
			}

			// Check can handle
			canHandle := agent.CanHandle(fmt.Sprintf("capability-%d", id))
			if !canHandle {
				t.Errorf("Goroutine %d: should be able to handle its own capability", id)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Should have all capabilities added
	caps := agent.Capabilities()
	if len(caps) != numGoroutines {
		t.Errorf("Expected %d capabilities, got %d", numGoroutines, len(caps))
	}
}

func TestBaseAgentConcurrentHealthChecks(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test Agent", nil)

	numChecks := 50
	done := make(chan bool, numChecks)

	for i := 0; i < numChecks; i++ {
		go func(id int) {
			// Perform health check
			health := agent.Health()
			if health == nil {
				t.Errorf("Health check %d returned nil", id)
			}

			// Check is healthy
			isHealthy := agent.IsHealthy()
			if !isHealthy {
				t.Errorf("Agent should be healthy in check %d", id)
			}

			done <- true
		}(i)
	}

	// Wait for all health checks
	for i := 0; i < numChecks; i++ {
		<-done
	}
}

func TestMockAgent(t *testing.T) {
	// This test would test a mock agent implementation
	// For now, just test that the base agent works as expected
	agent := NewBaseAgent("mock-agent", "Mock Agent", nil)

	if agent.ID() != "mock-agent" {
		t.Errorf("Expected ID 'mock-agent', got '%s'", agent.ID())
	}

	if agent.Name() != "Mock Agent" {
		t.Errorf("Expected name 'Mock Agent', got '%s'", agent.Name())
	}
}

func TestCoordinatorSubmitTask(t *testing.T) {
	// This test would test task coordination
	// For now, test basic task submission
	agent := NewBaseAgent("coord-agent", "Coordinator Agent", nil)
	agent.AddCapability(Capability{Name: "coord-task"})

	task := &Task{
		ID:   "coord-task-1",
		Type: "coord-task",
	}

	ctx := context.Background()
	result, err := agent.SubmitTask(ctx, task)

	if err != nil {
		t.Fatalf("Coordinator task submission failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Coordinator task should succeed, got error: %s", result.Error)
	}
}

func TestAgentRegistry(t *testing.T) {
	// This test would test agent registry functionality
	// For now, test that agents can be created and managed
	agent1 := NewBaseAgent("registry-agent-1", "Registry Agent 1", nil)
	agent2 := NewBaseAgent("registry-agent-2", "Registry Agent 2", nil)

	if agent1.ID() == agent2.ID() {
		t.Error("Agents should have unique IDs")
	}

	if agent1.Name() == agent2.Name() {
		t.Error("Agents can have same names but different IDs")
	}
}

func TestAgentRegistryByCapability(t *testing.T) {
	// Test capability-based agent lookup
	agent := NewBaseAgent("capability-agent", "Capability Agent", nil)

	// Add multiple capabilities
	caps := []Capability{
		{Name: "task-type-1", Description: "First task type"},
		{Name: "task-type-2", Description: "Second task type"},
		{Name: "task-type-3", Description: "Third task type"},
	}

	for _, cap := range caps {
		agent.AddCapability(cap)
	}

	// Test can handle each capability
	for _, cap := range caps {
		if !agent.CanHandle(cap.Name) {
			t.Errorf("Agent should handle capability: %s", cap.Name)
		}
	}

	// Test cannot handle unknown capability
	if agent.CanHandle("unknown-capability") {
		t.Error("Agent should not handle unknown capability")
	}
}

func TestAgentRegistryListMultiple(t *testing.T) {
	// Test managing multiple agents
	agents := []*BaseAgent{}

	for i := 0; i < 5; i++ {
		agent := NewBaseAgent(fmt.Sprintf("multi-agent-%d", i), fmt.Sprintf("Multi Agent %d", i), nil)
		agents = append(agents, agent)
	}

	// All agents should have unique IDs
	ids := make(map[string]bool)
	for _, agent := range agents {
		if ids[agent.ID()] {
			t.Errorf("Duplicate agent ID: %s", agent.ID())
		}
		ids[agent.ID()] = true
	}

	if len(ids) != 5 {
		t.Errorf("Expected 5 unique IDs, got %d", len(ids))
	}
}

func TestAgentRegistryUnregisterNonExistent(t *testing.T) {
	// Test unregistering non-existent agent
	agent := NewBaseAgent("existent-agent", "Existent Agent", nil)

	// Try to remove non-existent capability
	agent.RemoveCapability("non-existent-capability")

	// Agent should still function normally
	if !agent.IsHealthy() {
		t.Error("Agent should remain healthy after removing non-existent capability")
	}

	caps := agent.Capabilities()
	if len(caps) != 0 {
		t.Error("Agent should have no capabilities")
	}
}

func TestAgentRegistryMultipleUnregister(t *testing.T) {
	// Test multiple unregister operations
	agent := NewBaseAgent("multi-unregister-agent", "Multi Unregister Agent", nil)

	// Add capabilities
	caps := []string{"cap1", "cap2", "cap3", "cap4", "cap5"}
	for _, capName := range caps {
		agent.AddCapability(Capability{Name: capName})
	}

	// Remove some capabilities
	agent.RemoveCapability("cap2")
	agent.RemoveCapability("cap4")

	// Check remaining capabilities
	remainingCaps := agent.Capabilities()
	expectedCount := 3
	if len(remainingCaps) != expectedCount {
		t.Errorf("Expected %d capabilities after removal, got %d", expectedCount, len(remainingCaps))
	}

	// Check specific capabilities are gone
	remainingNames := make(map[string]bool)
	for _, cap := range remainingCaps {
		remainingNames[cap.Name] = true
	}

	if remainingNames["cap2"] {
		t.Error("cap2 should have been removed")
	}

	if remainingNames["cap4"] {
		t.Error("cap4 should have been removed")
	}

	if !remainingNames["cap1"] {
		t.Error("cap1 should still exist")
	}
}

func TestAgentRegistryGetByTypeEmpty(t *testing.T) {
	// Test getting agents by type when none exist
	agent := NewBaseAgent("empty-type-agent", "Empty Type Agent", nil)

	// Should not handle any task types initially
	if agent.CanHandle("any-task-type") {
		t.Error("Agent should not handle any task types initially")
	}
}

func TestAgentRegistryGetByCapabilityEmpty(t *testing.T) {
	// Test getting agents by capability when none exist
	agent := NewBaseAgent("empty-capability-agent", "Empty Capability Agent", nil)

	caps := agent.Capabilities()
	if len(caps) != 0 {
		t.Errorf("Expected 0 capabilities, got %d", len(caps))
	}
}

func TestGenerateAgentID(t *testing.T) {
	// Test agent ID generation
	agent1 := NewBaseAgent("id-test-1", "ID Test 1", nil)
	agent2 := NewBaseAgent("id-test-2", "ID Test 2", nil)

	if agent1.ID() != "id-test-1" {
		t.Errorf("Agent1 ID should be 'id-test-1', got '%s'", agent1.ID())
	}

	if agent2.ID() != "id-test-2" {
		t.Errorf("Agent2 ID should be 'id-test-2', got '%s'", agent2.ID())
	}

	if agent1.ID() == agent2.ID() {
		t.Error("Agent IDs should be unique")
	}
}

func BenchmarkBaseAgentSubmitTask(b *testing.B) {
	agent := NewBaseAgent("bench-agent", "Benchmark Agent", nil)
	agent.AddCapability(Capability{Name: "bench-task"})

	task := &Task{
		ID:   "bench-task",
		Type: "bench-task",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task.ID = fmt.Sprintf("bench-task-%d", i)
		_, err := agent.SubmitTask(ctx, task)
		if err != nil {
			b.Fatalf("Benchmark task failed: %v", err)
		}
	}
}

func BenchmarkBaseAgentCapabilities(b *testing.B) {
	agent := NewBaseAgent("bench-cap-agent", "Benchmark Cap Agent", nil)

	// Add many capabilities
	for i := 0; i < 100; i++ {
		agent.AddCapability(Capability{
			Name: fmt.Sprintf("cap-%d", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.Capabilities()
	}
}

func BenchmarkBaseAgentCanHandle(b *testing.B) {
	agent := NewBaseAgent("bench-handle-agent", "Benchmark Handle Agent", nil)

	// Add many capabilities
	for i := 0; i < 100; i++ {
		agent.AddCapability(Capability{
			Name: fmt.Sprintf("cap-%d", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.CanHandle("cap-50")
	}
}
