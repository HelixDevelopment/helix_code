package integration

import (
	"testing"

	"dev.helix.code/internal/memory/providers"
)

func TestMem0ProviderIntegration(t *testing.T) {
	config := map[string]interface{}{
		"base_url": "https://api.mem0.ai",
		"api_key":  "test-key",
		"user_id":  "test-user",
		"agent_id": "test-agent",
		"run_id":   "test-run",
	}

	provider, err := providers.NewMem0Provider(config)
	if err != nil {
		t.Fatalf("Failed to create Mem0 provider: %v", err)
	}

	// Test basic properties
	if provider.GetName() != "Mem0" {
		t.Errorf("Expected name 'Mem0', got '%s'", provider.GetName())
	}

	if provider.GetType() != "mem0" {
		t.Errorf("Expected type 'mem0', got '%s'", provider.GetType())
	}

	// Test capabilities
	caps := provider.GetCapabilities()
	if len(caps) == 0 {
		t.Error("Expected some capabilities")
	}

	// Verify expected capabilities
	expectedCaps := []string{"memory_storage", "memory_retrieval", "memory_search"}
	for _, expected := range expectedCaps {
		found := false
		for _, cap := range caps {
			if cap == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability '%s' not found", expected)
		}
	}

	// Test cloud detection
	if !provider.IsCloud() {
		t.Error("Expected Mem0 provider to be cloud-based")
	}

	// Test configuration retrieval
	retrievedConfig := provider.GetConfiguration()
	if retrievedConfig == nil {
		t.Error("Expected configuration to be retrievable")
	}

	// Test cost info
	costInfo := provider.GetCostInfo()
	if costInfo == nil {
		t.Error("Expected cost info to be available")
	}
	if costInfo.Currency != "USD" {
		t.Errorf("Expected currency 'USD', got '%s'", costInfo.Currency)
	}
}

func TestZepProviderIntegration(t *testing.T) {
	config := map[string]interface{}{
		"base_url": "https://api.getzep.com",
		"api_key":  "test-key",
	}

	provider, err := providers.NewZepProvider(config)
	if err != nil {
		t.Fatalf("Failed to create Zep provider: %v", err)
	}

	if provider.GetName() != "Zep" {
		t.Errorf("Expected name 'Zep', got '%s'", provider.GetName())
	}

	if string(provider.GetType()) != "zep" {
		t.Errorf("Expected type 'zep', got '%s'", string(provider.GetType()))
	}
}

func TestMemontoProviderIntegration(t *testing.T) {
	config := map[string]interface{}{
		"base_url": "https://api.memonto.ai/v1",
		"api_key":  "test-key",
		"user_id":  "test-user",
	}

	provider, err := providers.NewMemontoProvider(config)
	if err != nil {
		t.Fatalf("Failed to create Memonto provider: %v", err)
	}

	// Test basic properties
	if provider.GetName() != "Memonto" {
		t.Errorf("Expected name 'Memonto', got '%s'", provider.GetName())
	}

	if provider.GetType() != "memonto" {
		t.Errorf("Expected type 'memonto', got '%s'", provider.GetType())
	}

	// Test capabilities
	caps := provider.GetCapabilities()
	if len(caps) == 0 {
		t.Error("Expected some capabilities")
	}

	// Verify expected capabilities for knowledge graph provider
	expectedCaps := []string{"memory_storage", "knowledge_graph", "ontology_management"}
	for _, expected := range expectedCaps {
		found := false
		for _, cap := range caps {
			if cap == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability '%s' not found", expected)
		}
	}

	// Test cloud detection
	if !provider.IsCloud() {
		t.Error("Expected Memonto provider to be cloud-based")
	}

	// Test configuration retrieval
	retrievedConfig := provider.GetConfiguration()
	if retrievedConfig == nil {
		t.Error("Expected configuration to be retrievable")
	}

	// Test cost info
	costInfo := provider.GetCostInfo()
	if costInfo == nil {
		t.Error("Expected cost info to be available")
	}
	if costInfo.Currency != "USD" {
		t.Errorf("Expected currency 'USD', got '%s'", costInfo.Currency)
	}
}

func TestBaseAIProviderIntegration(t *testing.T) {
	config := map[string]interface{}{
		"base_url": "https://api.langbase.com/v1",
		"api_key":  "test-key",
	}

	provider, err := providers.NewBaseAIProvider(config)
	if err != nil {
		t.Fatalf("Failed to create BaseAI provider: %v", err)
	}

	// Test basic properties
	if provider.GetName() != "BaseAI" {
		t.Errorf("Expected name 'BaseAI', got '%s'", provider.GetName())
	}

	if provider.GetType() != "baseai" {
		t.Errorf("Expected type 'baseai', got '%s'", provider.GetType())
	}

	// Test capabilities
	caps := provider.GetCapabilities()
	if len(caps) == 0 {
		t.Error("Expected some capabilities")
	}

	// Verify expected capabilities
	expectedCaps := []string{"memory_storage", "memory_retrieval", "memory_search"}
	for _, expected := range expectedCaps {
		found := false
		for _, cap := range caps {
			if cap == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability '%s' not found", expected)
		}
	}

	// Test cloud detection
	if !provider.IsCloud() {
		t.Error("Expected BaseAI provider to be cloud-based")
	}

	// Test configuration retrieval
	retrievedConfig := provider.GetConfiguration()
	if retrievedConfig == nil {
		t.Error("Expected configuration to be retrievable")
	}

	// Test cost info
	costInfo := provider.GetCostInfo()
	if costInfo == nil {
		t.Error("Expected cost info to be available")
	}
	if costInfo.Currency != "USD" {
		t.Errorf("Expected currency 'USD', got '%s'", costInfo.Currency)
	}
}
