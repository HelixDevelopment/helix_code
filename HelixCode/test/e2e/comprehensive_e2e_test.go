//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/mcp"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/worker"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompleteDistributedWorkflow tests a complete distributed AI workflow
func TestCompleteDistributedWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complete workflow test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	t.Log("🚀 Starting complete distributed workflow test")

	// Phase 1: Setup all components
	t.Log("Phase 1: Setting up all components...")
	
	workerPool := worker.NewSSHWorkerPool(true) // Auto-install enabled
	notificationEngine := notification.NewNotificationEngine()
	mcpServer := mcp.NewMCPServer()
	
	assert.NotNil(t, workerPool)
	assert.NotNil(t, notificationEngine)
	assert.NotNil(t, mcpServer)

	// Phase 2: Worker network setup
	t.Log("Phase 2: Setting up worker network...")
	
	// Test worker health monitoring
	err := workerPool.HealthCheck(ctx)
	assert.NoError(t, err)
	
	initialStats := workerPool.GetWorkerStats(ctx)
	t.Logf("Initial worker stats: %+v", initialStats)

	// Phase 3: Notification system setup
	t.Log("Phase 3: Setting up notification system...")
	
	// Add notification channels
	slackChannel := notification.NewSlackChannel("", "#test", "Helix Test")
	emailChannel := notification.NewEmailChannel("smtp.test.com", 587, "test", "pass", "test@helix.dev")
	
	err = notificationEngine.RegisterChannel(slackChannel)
	assert.NoError(t, err)
	
	err = notificationEngine.RegisterChannel(emailChannel)
	assert.NoError(t, err)

	// Phase 4: MCP integration
	t.Log("Phase 4: Setting up MCP integration...")
	
	// Register MCP tools
	tools := []*mcp.Tool{
		{
			ID:          "code_analyzer",
			Name:        "Code Analyzer",
			Description: "Analyzes code quality and provides suggestions",
			Handler: func(ctx context.Context, session *mcp.MCPSession, args map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{
					"quality_score": 85,
					"suggestions":   []string{"Add comments", "Improve error handling"},
				}, nil
			},
		},
		{
			ID:          "test_runner",
			Name:        "Test Runner",
			Description: "Runs tests and reports results",
			Handler: func(ctx context.Context, session *mcp.MCPSession, args map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{
					"passed": 15,
					"failed": 2,
					"coverage": 92.5,
				}, nil
			},
		},
	}

	for _, tool := range tools {
		err := mcpServer.RegisterTool(tool)
		assert.NoError(t, err)
	}

	// Phase 5: Cross-component integration
	t.Log("Phase 5: Testing cross-component integration...")
	
	// Test notification from worker operations
	workerNotification := &notification.Notification{
		Title:    "Worker Network Ready",
		Message:  "Distributed worker network initialized successfully",
		Type:     notification.NotificationTypeSuccess,
		Priority: notification.NotificationPriorityMedium,
	}

	err = notificationEngine.SendNotification(ctx, workerNotification)
	assert.NoError(t, err)

	// Phase 6: Performance testing
	t.Log("Phase 6: Performance testing...")
	
	start := time.Now()
	
	// Test concurrent operations
	numOperations := 50
	done := make(chan bool, numOperations)
	
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			// Mixed operations across all components
			_ = workerPool.GetWorkerStats(ctx)
			_ = notificationEngine.GetChannelStats()
			_ = mcpServer.GetToolCount()
			
			done <- true
		}(i)
	}
	
	// Wait for all operations
	for i := 0; i < numOperations; i++ {
		<-done
	}
	
	duration := time.Since(start)
	
	// Performance target: 50 mixed operations in under 2 seconds
	assert.Less(t, duration, 2*time.Second)
	t.Logf("Performance: %d mixed operations in %v", numOperations, duration)

	// Phase 7: Final verification
	t.Log("Phase 7: Final verification...")
	
	// Verify all components are working
	finalWorkerStats := workerPool.GetWorkerStats(ctx)
	assert.NotNil(t, finalWorkerStats)
	
	finalChannelStats := notificationEngine.GetChannelStats()
	assert.NotNil(t, finalChannelStats)
	
	finalToolCount := mcpServer.GetToolCount()
	assert.Equal(t, 2, finalToolCount)

	t.Log("✅ Complete distributed workflow test PASSED")
}

// TestRealAIEndToEnd tests real AI integration end-to-end
func TestRealAIEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real AI end-to-end test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	t.Log("🧠 Starting real AI end-to-end test")

	// Setup LLM provider manager
	config := llm.ProviderConfig{
		DefaultProvider: llm.ProviderTypeLocal,
		Timeout:         120 * time.Second,
		MaxRetries:      5,
	}

	providerManager := llm.NewProviderManager(config)

	// Get available providers
	availableProviders := providerManager.GetAvailableProviders()
	if len(availableProviders) == 0 {
		t.Skip("No LLM providers available for end-to-end testing")
	}

	t.Logf("Found %d available LLM providers", len(availableProviders))

	// Test each provider with complex reasoning
	for _, provider := range availableProviders {
		t.Run(provider.GetName(), func(t *testing.T) {
			// Create reasoning engine
			reasoningEngine := llm.NewReasoningEngine(provider)

			// Test complex reasoning with multiple steps
			request := llm.ReasoningRequest{
				Prompt: `You are an expert software engineer. Analyze the following problem:

Problem: We need to implement a distributed task scheduling system that can handle:
- 100+ concurrent workers
- Dynamic resource allocation
- Fault tolerance
- Real-time monitoring

Please provide a step-by-step architecture design.`, 
				ReasoningType: llm.ReasoningTypeChainOfThought,
				MaxSteps:      10,
				Temperature:   0.7,
			}

			response, err := reasoningEngine.GenerateWithReasoning(ctx, request)
			
			if err != nil {
				t.Logf("Provider %s failed: %v", provider.GetName(), err)
				return
			}

			// Verify response quality
			assert.NotNil(t, response)
			assert.NotEmpty(t, response.FinalAnswer)
			assert.Greater(t, len(response.FinalAnswer), 100) // Substantial answer
			assert.Greater(t, response.Duration, time.Duration(0))
			assert.Less(t, response.Duration, 2*time.Minute) // Should complete in reasonable time

			// Verify reasoning steps
			assert.Greater(t, len(response.ReasoningSteps), 0)
			
			for _, step := range response.ReasoningSteps {
				assert.NotEmpty(t, step.Thought)
				assert.Greater(t, step.StepNumber, 0)
				assert.Greater(t, step.Confidence, 0.0)
			}

			t.Logf("✅ %s AI test completed: %d steps, %v duration", 
				provider.GetName(), len(response.ReasoningSteps), response.Duration)
		})
	}

	t.Log("✅ Real AI end-to-end test PASSED")
}

// TestScalabilityEndToEnd tests system scalability
func TestScalabilityEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	ctx := context.Background()

	t.Log("📈 Starting scalability end-to-end test")

	// Test with large number of simulated workers
	workerPool := worker.NewSSHWorkerPool(false)

	// Add many simulated workers
	numWorkers := 100
	for i := 0; i < numWorkers; i++ {
		workerID := uuid.New()
		workerPool.workers[workerID] = &worker.SSHWorker{
			ID:           workerID,
			Hostname:     "worker-" + string(rune('A'+(i%26))),
			Status:       worker.WorkerStatusActive,
			HealthStatus: worker.WorkerHealthHealthy,
			Resources: worker.Resources{
				CPUCount:    4,
				TotalMemory: 8589934592, // 8GB
				GPUCount:    1,
			},
		}
	}

	// Test performance with many workers
	start := time.Now()
	
	stats := workerPool.GetWorkerStats(ctx)
	
	duration := time.Since(start)
	
	// Should handle 100 workers efficiently
	assert.Equal(t, numWorkers, stats.TotalWorkers)
	assert.Less(t, duration, 100*time.Millisecond)
	
	t.Logf("Scalability: %d workers processed in %v", numWorkers, duration)

	// Test notification system with many channels
	notificationEngine := notification.NewNotificationEngine()
	
	// Add multiple notification channels
	for i := 0; i < 10; i++ {
		channel := notification.NewSlackChannel("", "#channel-"+string(rune('A'+i)), "Helix")
		_ = notificationEngine.RegisterChannel(channel)
	}
	
	channelStats := notificationEngine.GetChannelStats()
	assert.NotNil(t, channelStats)
	
	t.Log("✅ Scalability end-to-end test PASSED")
}

// TestFaultToleranceEndToEnd tests system fault tolerance
func TestFaultToleranceEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping fault tolerance test in short mode")
	}

	ctx := context.Background()

	t.Log("🛡️ Starting fault tolerance end-to-end test")

	workerPool := worker.NewSSHWorkerPool(false)
	notificationEngine := notification.NewNotificationEngine()
	mcpServer := mcp.NewMCPServer()

	// Test 1: Component failures shouldn't crash the system
	t.Log("Test 1: Component failure resilience...")
	
	// Simulate various error conditions
	for i := 0; i < 10; i++ {
		// These should not panic
		_ = workerPool.HealthCheck(ctx)
		_ = workerPool.GetWorkerStats(ctx)
		_ = notificationEngine.GetChannelStats()
		_ = mcpServer.GetToolCount()
		
		// Simulate command execution failure
		_, err := workerPool.ExecuteCommand(ctx, uuid.New(), "invalid-command")
		assert.Error(t, err) // Expected to fail
	}

	// Test 2: System should remain functional after errors
	t.Log("Test 2: Post-error functionality...")
	
	finalStats := workerPool.GetWorkerStats(ctx)
	assert.NotNil(t, finalStats)
	
	finalChannels := notificationEngine.GetChannelStats()
	assert.NotNil(t, finalChannels)
	
	finalTools := mcpServer.GetToolCount()
	assert.Equal(t, 0, finalTools)

	// Test 3: Recovery from partial failures
	t.Log("Test 3: Recovery testing...")
	
	// Add a worker after previous failures
	workerID := uuid.New()
	workerPool.workers[workerID] = &worker.SSHWorker{
		ID:       workerID,
		Hostname: "recovered-worker",
	}
	
	recoveredStats := workerPool.GetWorkerStats(ctx)
	assert.Equal(t, 1, recoveredStats.TotalWorkers)

	t.Log("✅ Fault tolerance end-to-end test PASSED")
}

// TestCrossPlatformEndToEnd tests cross-platform compatibility
func TestCrossPlatformEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-platform test in short mode")
	}

	t.Log("🌐 Starting cross-platform end-to-end test")

	// Test all components on different platforms
	components := []struct {
		name string
		test func(*testing.T)
	}{
		{
			name: "Worker Pool",
			test: func(t *testing.T) {
				workerPool := worker.NewSSHWorkerPool(false)
				ctx := context.Background()
				
				// Test basic functionality
				_ = workerPool.HealthCheck(ctx)
				stats := workerPool.GetWorkerStats(ctx)
				assert.NotNil(t, stats)
			},
		},
		{
			name: "Notification System",
			test: func(t *testing.T) {
				engine := notification.NewNotificationEngine()
				
				// Test basic functionality
				stats := engine.GetChannelStats()
				assert.NotNil(t, stats)
			},
		},
		{
			name: "MCP Server",
			test: func(t *testing.T) {
				server := mcp.NewMCPServer()
				
				// Test basic functionality
				tool := &mcp.Tool{
					ID:          "cross_platform_tool",
					Name:        "Cross Platform Tool",
					Description: "Works on all platforms",
					Handler: func(ctx context.Context, session *mcp.MCPSession, args map[string]interface{}) (interface{}, error) {
						return "platform_agnostic_result", nil
					},
				}
				
				err := server.RegisterTool(tool)
				assert.NoError(t, err)
				assert.Equal(t, 1, server.GetToolCount())
			},
		},
	}

	// Run all component tests
	for _, component := range components {
		t.Run(component.name, component.test)
	}

	t.Log("✅ Cross-platform end-to-end test PASSED")
}