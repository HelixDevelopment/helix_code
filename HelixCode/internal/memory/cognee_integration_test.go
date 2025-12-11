package memory

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/logging"
)

func TestNewCogneeIntegration(t *testing.T) {
	cfg := &config.CogneeConfig{
		Mode: "local",
	}
	logger := logging.NewLogger(logging.INFO)
	
	integration := NewCogneeIntegration(cfg, logger)
	
	if integration == nil {
		t.Fatal("NewCogneeIntegration should return a non-nil instance")
	}
	
	if integration.config != cfg {
		t.Error("Config should be set correctly")
	}
	
	if integration.logger != logger {
		t.Error("Logger should be set correctly")
	}
	
	if integration.isRunning {
		t.Error("isRunning should be false initially")
	}
	
	if integration.ctx == nil || integration.cancel == nil {
		t.Error("Context and cancel function should be initialized")
	}
}

func TestCogneeIntegrationInitialize(t *testing.T) {
	cfg := &config.CogneeConfig{
		Mode: "remote",
		RemoteAPI: &config.CogneeRemoteAPIConfig{
			ServiceEndpoint: "http://localhost:8080",
			APIKey:          "test-key",
			Timeout:         30 * time.Second,
		},
	}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test successful initialization
	err := integration.Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize should succeed: %v", err)
	}
	
	if !integration.isRunning {
		t.Error("isRunning should be true after initialization")
	}
	
	if integration.client == nil {
		t.Error("Client should be initialized when RemoteAPI config is provided")
	}
	
	if integration.client.baseURL != cfg.RemoteAPI.ServiceEndpoint {
		t.Error("Client baseURL should be set correctly")
	}
	
	if integration.client.apiKey != cfg.RemoteAPI.APIKey {
		t.Error("Client API key should be set correctly")
	}
	
	// Test double initialization
	err = integration.Initialize(ctx, cfg)
	if err == nil {
		t.Error("Second initialization should fail")
	}
	
	// Test initialization with nil config
	integration2 := NewCogneeIntegration(cfg, logger)
	err = integration2.Initialize(ctx, nil)
	if err == nil {
		t.Error("Initialization with nil config should fail")
	}
	
	// Test initialization without RemoteAPI config
	localCfg := &config.CogneeConfig{Mode: "local"}
	integration3 := NewCogneeIntegration(localCfg, logger)
	err = integration3.Initialize(ctx, localCfg)
	if err != nil {
		t.Errorf("Initialization without RemoteAPI should succeed: %v", err)
	}
	
	if integration3.client != nil {
		t.Error("Client should be nil when RemoteAPI config is not provided")
	}
}

func TestCogneeIntegrationShutdown(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test shutdown without initialization
	err := integration.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown without initialization should not fail: %v", err)
	}
	
	// Test shutdown after initialization
	integration.Initialize(ctx, cfg)
	err = integration.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown should succeed: %v", err)
	}
	
	if integration.isRunning {
		t.Error("isRunning should be false after shutdown")
	}
	
	// Test shutdown after already shut down
	err = integration.Shutdown(ctx)
	if err != nil {
		t.Errorf("Second shutdown should not fail: %v", err)
	}
}

func TestCogneeIntegrationStoreMemory(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test storing memory without initialization
	memory := NewMemoryItem("test-id", "test content", "test", 0.5, time.Now())
	err := integration.StoreMemory(ctx, memory)
	if err == nil {
		t.Error("StoreMemory without initialization should fail")
	}
	
	// Test storing memory after initialization
	integration.Initialize(ctx, cfg)
	err = integration.StoreMemory(ctx, memory)
	if err != nil {
		t.Errorf("StoreMemory should succeed: %v", err)
	}
	
	// Test storing nil memory
	err = integration.StoreMemory(ctx, nil)
	if err == nil {
		t.Error("StoreMemory with nil memory should fail")
	}
}

func TestCogneeIntegrationRetrieveMemory(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	query := NewRetrievalQuery("test query", "semantic", 10)
	
	// Test retrieving memory without initialization
	result, err := integration.RetrieveMemory(ctx, query)
	if err == nil {
		t.Error("RetrieveMemory without initialization should fail")
	}
	
	// Test retrieving memory after initialization
	integration.Initialize(ctx, cfg)
	result, err = integration.RetrieveMemory(ctx, query)
	if err != nil {
		t.Errorf("RetrieveMemory should succeed: %v", err)
	}
	
	if result == nil {
		t.Error("Result should not be nil")
	}
	
	if result.Query != query {
		t.Error("Result query should match input query")
	}
	
	if result.Results == nil {
		t.Error("Results should not be nil")
	}
	
	if result.Total != 0 {
		t.Error("Total should be 0 for placeholder implementation")
	}
	
	// Test retrieving with nil query
	_, err = integration.RetrieveMemory(ctx, nil)
	if err == nil {
		t.Error("RetrieveMemory with nil query should fail")
	}
}

func TestCogneeIntegrationGetContext(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test getting context without initialization
	conversation, err := integration.GetContext(ctx, "openai", "gpt-4", "session-123")
	if err == nil {
		t.Error("GetContext without initialization should fail")
	}
	
	// Test getting context after initialization
	integration.Initialize(ctx, cfg)
	conversation, err = integration.GetContext(ctx, "openai", "gpt-4", "session-123")
	if err != nil {
		t.Errorf("GetContext should succeed: %v", err)
	}
	
	if conversation == nil {
		t.Error("Conversation should not be nil")
	}
	
	expectedTitle := "Context for openai/gpt-4"
	if conversation.Title != expectedTitle {
		t.Errorf("Expected title %s, got %s", expectedTitle, conversation.Title)
	}
	
	if conversation.Metadata["session"] != "session-123" {
		t.Error("Session metadata should be set correctly")
	}
	
	if conversation.Metadata["provider"] != "openai" {
		t.Error("Provider metadata should be set correctly")
	}
	
	if conversation.Metadata["model"] != "gpt-4" {
		t.Error("Model metadata should be set correctly")
	}
}

func TestCogneeIntegrationGetSystemInfo(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test getting system info without initialization
	info, err := integration.GetSystemInfo(ctx)
	if err == nil {
		t.Error("GetSystemInfo without initialization should fail")
	}
	
	// Test getting system info after initialization
	integration.Initialize(ctx, cfg)
	info, err = integration.GetSystemInfo(ctx)
	if err != nil {
		t.Errorf("GetSystemInfo should succeed: %v", err)
	}
	
	if info == nil {
		t.Error("SystemInfo should not be nil")
	}
	
	if info.Component != "cognee" {
		t.Error("Component should be 'cognee'")
	}
	
	if info.Version != "1.0.0" {
		t.Error("Version should be '1.0.0'")
	}
	
	if info.Status != "healthy" {
		t.Error("Status should be 'healthy'")
	}
}

func TestCogneeIntegrationGetOptimizationRecommendations(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test getting recommendations without initialization
	recommendations, err := integration.GetOptimizationRecommendations(ctx)
	if err == nil {
		t.Error("GetOptimizationRecommendations without initialization should fail")
	}
	
	// Test getting recommendations after initialization
	integration.Initialize(ctx, cfg)
	recommendations, err = integration.GetOptimizationRecommendations(ctx)
	if err != nil {
		t.Errorf("GetOptimizationRecommendations should succeed: %v", err)
	}
	
	if recommendations == nil {
		t.Error("Recommendations should not be nil")
	}
	
	if len(recommendations) == 0 {
		t.Error("Should have at least one recommendation")
	}
	
	rec := recommendations[0]
	if rec.Type != "memory" {
		t.Error("First recommendation type should be 'memory'")
	}
	
	if rec.Priority != "high" {
		t.Error("First recommendation priority should be 'high'")
	}
	
	if rec.Impact != 0.8 {
		t.Error("First recommendation impact should be 0.8")
	}
}

func TestCogneeIntegrationApplyOptimizations(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	recommendations := []*OptimizationRecommendation{
		NewOptimizationRecommendation("memory", "Increase memory", "high", 0.8),
	}
	
	// Test applying optimizations without initialization
	err := integration.ApplyOptimizations(ctx, recommendations)
	if err == nil {
		t.Error("ApplyOptimizations without initialization should fail")
	}
	
	// Test applying optimizations after initialization
	integration.Initialize(ctx, cfg)
	err = integration.ApplyOptimizations(ctx, recommendations)
	if err != nil {
		t.Errorf("ApplyOptimizations should succeed: %v", err)
	}
	
	// Test applying nil recommendations
	err = integration.ApplyOptimizations(ctx, nil)
	if err == nil {
		t.Error("ApplyOptimizations with nil recommendations should fail")
	}
	
	// Test applying empty recommendations
	err = integration.ApplyOptimizations(ctx, []*OptimizationRecommendation{})
	if err != nil {
		t.Errorf("ApplyOptimizations with empty recommendations should handle gracefully: %v", err)
	}
}

func TestCogneeIntegrationHealthCheck(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test health check without initialization
	status, err := integration.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck without initialization should not return error: %v", err)
	}
	
	if status == nil {
		t.Error("HealthStatus should not be nil")
	}
	
	if status.Status != "down" {
		t.Error("Status should be 'down' when not initialized")
	}
	
	// Test health check after initialization
	integration.Initialize(ctx, cfg)
	status, err = integration.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck should succeed: %v", err)
	}
	
	if status.Status != "healthy" {
		t.Error("Status should be 'healthy' when initialized")
	}
}

func TestCogneeIntegrationIsRunning(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test initial state
	if integration.IsRunning() {
		t.Error("IsRunning should be false initially")
	}
	
	// Test after initialization
	integration.Initialize(ctx, cfg)
	if !integration.IsRunning() {
		t.Error("IsRunning should be true after initialization")
	}
	
	// Test after shutdown
	integration.Shutdown(ctx)
	if integration.IsRunning() {
		t.Error("IsRunning should be false after shutdown")
	}
}

func TestCogneeIntegrationGetConfig(t *testing.T) {
	cfg := &config.CogneeConfig{
		Mode: "remote",
		RemoteAPI: &config.CogneeRemoteAPIConfig{
			ServiceEndpoint: "http://test.com",
		},
	}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test getting config before initialization
	retrievedConfig := integration.GetConfig()
	if retrievedConfig != cfg {
		t.Error("Config should be accessible before initialization")
	}
	
	// Test getting config after initialization
	integration.Initialize(ctx, cfg)
	retrievedConfig = integration.GetConfig()
	if retrievedConfig != cfg {
		t.Error("Config should be accessible after initialization")
	}
	
	// Test getting config after shutdown
	integration.Shutdown(ctx)
	retrievedConfig = integration.GetConfig()
	if retrievedConfig != cfg {
		t.Error("Config should still be accessible after shutdown")
	}
}

func TestCogneeIntegrationConcurrency(t *testing.T) {
	cfg := &config.CogneeConfig{}
	logger := logging.NewLogger(logging.INFO)
	integration := NewCogneeIntegration(cfg, logger)
	ctx := context.Background()
	
	// Test concurrent access to IsRunning and GetConfig
	done := make(chan bool, 10)
	for i := 0; i < 5; i++ {
		go func() {
			integration.IsRunning()
			done <- true
		}()
		go func() {
			integration.GetConfig()
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Test concurrent operations after initialization
	integration.Initialize(ctx, cfg)
	done = make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			integration.IsRunning()
			done <- true
		}()
		go func() {
			integration.GetConfig()
			done <- true
		}()
	}
	
	for i := 0; i < 20; i++ {
		<-done
	}
}