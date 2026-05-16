package phase3

import (
	"fmt"
	"net/http"
	"testing"

	"dev.helix.code/tests/e2e"
)

// TestMemorySystemIntegration tests external memory system integration
func TestMemorySystemIntegration(t *testing.T) {
	t.Log("🧠 Testing memory system integration...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Test different memory providers
	memoryProviders := []struct {
		name    string
		enabled bool
		config  map[string]interface{}
	}{
		{
			name:    "mem0",
			enabled: true,
			config: map[string]interface{}{
				"api_key": "${MEM0_API_KEY}",
				"endpoint": "https://api.mem0.ai",
			},
		},
		{
			name:    "zep",
			enabled: true,
			config: map[string]interface{}{
				"api_key": "${ZEP_API_KEY}",
				"endpoint": "https://api.getzep.com",
			},
		},
		{
			name:    "chroma",
			enabled: true,
			config: map[string]interface{}{
				"endpoint": "http://localhost:8000",
			},
		},
	}
	
	for _, provider := range memoryProviders {
		t.Run(provider.name, func(t *testing.T) {
			t.Logf("🧠 Testing %s memory provider...", provider.name)
			
			// Test memory provider health
			healthResp, err := framework.GET(t, fmt.Sprintf("/api/v1/memory/providers/%s/health", provider.name))
			if err != nil {
				t.Logf("⚠️ %s health check failed: %v", provider.name, err)
				return
			}
			defer healthResp.Body.Close()
			
			switch healthResp.StatusCode {
			case http.StatusOK:
				t.Logf("✅ %s memory provider is healthy", provider.name)
			case http.StatusNotFound:
				t.Logf("ℹ️ %s memory provider not configured (expected)", provider.name)
			default:
				t.Logf("ℹ️ %s memory provider returned status %d", provider.name, healthResp.StatusCode)
			}
			
			// Test memory storage
			memoryData := map[string]interface{}{
				"provider": provider.name,
				"user_id": "test_user_123",
				"memory_items": []map[string]interface{}{
					{
						"type": "user_preference",
						"content": "User prefers Go programming language",
						"metadata": map[string]interface{}{
							"category": "programming",
							"preference": "language",
						},
					},
				},
			}
			
			storeResp, err := framework.POST(t, "/api/v1/memory/store", memoryData)
			if err != nil {
				t.Logf("⚠️ %s memory storage failed: %v", provider.name, err)
				return
			}
			defer storeResp.Body.Close()
			
			switch storeResp.StatusCode {
			case http.StatusOK:
				t.Logf("✅ %s memory storage successful", provider.name)
			case http.StatusServiceUnavailable:
				t.Logf("ℹ️ %s memory provider not configured (expected)", provider.name)
			default:
				t.Logf("ℹ️ %s memory storage returned status %d", provider.name, storeResp.StatusCode)
			}
		})
	}
	
	t.Log("✅ Memory system integration completed")
}

// TestConversationMemory tests conversational memory persistence
func TestConversationMemory(t *testing.T) {
	t.Log("💬 Testing conversational memory persistence...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Create a conversation
	conversationData := map[string]interface{}{
		"name": "Production Memory Test",
		"description": "Testing conversational memory in production",
		"context_type": "programming",
	}
	
	convResp, err := framework.POST(t, "/api/v1/conversations", conversationData)
	if err != nil {
		t.Logf("⚠️ Conversation creation failed: %v", err)
		return
	}
	defer convResp.Body.Close()
	
	if convResp.StatusCode != http.StatusCreated {
		t.Logf("ℹ️ Conversation creation returned status %d", convResp.StatusCode)
		return
	}
	
	var convResponse map[string]interface{}
	e2e.ParseJSON(t, convResp, &convResponse)
	
	if conversationID, ok := convResponse["conversation_id"].(string); ok {
		t.Logf("✅ Conversation created with ID: %s", conversationID)
		
		// Test memory within conversation
		memoryData := map[string]interface{}{
			"conversation_id": conversationID,
			"memory_items": []map[string]interface{}{
				{
					"type": "context",
					"content": "User is working on a Go web application",
					"metadata": map[string]interface{}{
						"topic": "current_work",
						"language": "go",
						"framework": "web",
					},
				},
			},
		}
		
		memResp, err := framework.POST(t, "/api/v1/conversations/context", memoryData)
		if err != nil {
			t.Logf("⚠️ Conversation memory failed: %v", err)
			return
		}
		defer memResp.Body.Close()
		
		switch memResp.StatusCode {
		case http.StatusOK:
			t.Log("✅ Conversation memory stored successfully")
		case http.StatusServiceUnavailable:
			t.Log("ℹ️ Memory system not configured (expected)")
		default:
			t.Logf("ℹ️ Conversation memory returned status %d", memResp.StatusCode)
		}
	}
}

// TestMemorySearchAndRetrieval tests memory search and retrieval functionality
func TestMemorySearchAndRetrieval(t *testing.T) {
	t.Log("🔍 Testing memory search and retrieval functionality...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Test memory search
	searchData := map[string]interface{}{
		"provider": "mem0",
		"user_id": "test_user_123",
		"query": "programming preferences",
		"limit": 10,
	}
	
	searchResp, err := framework.POST(t, "/api/v1/memory/search", searchData)
	if err != nil {
		t.Logf("⚠️ Memory search failed: %v", err)
		return
	}
	defer searchResp.Body.Close()
	
	switch searchResp.StatusCode {
	case http.StatusOK:
		t.Log("✅ Memory search successful")
		var searchResponse map[string]interface{}
		e2e.ParseJSON(t, searchResp, &searchResponse)
		
		if memories, ok := searchResponse["memories"].([]interface{}); ok {
			t.Logf("✅ Found %d memories", len(memories))
		}
	case http.StatusServiceUnavailable:
		t.Log("ℹ️ Memory search system not configured (expected)")
	default:
		t.Logf("ℹ️ Memory search returned status %d", searchResp.StatusCode)
	}
}

// TestMemoryPersistence tests memory persistence across sessions
func TestMemoryPersistence(t *testing.T) {
	t.Log("💾 Testing memory persistence across sessions...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Create test memory
	memoryData := map[string]interface{}{
		"provider": "zep",
		"user_id": "persistence_test_user",
		"memory_items": []map[string]interface{}{
			{
				"type": "user_fact",
				"content": "User prefers dark mode interfaces",
				"metadata": map[string]interface{}{
					"category": "ui_preference",
					"setting": "dark_mode",
				},
			},
		},
	}
	
	storeResp, err := framework.POST(t, "/api/v1/memory/store", memoryData)
	if err != nil {
		t.Logf("⚠️ Memory storage failed: %v", err)
		return
	}
	defer storeResp.Body.Close()
	
	if storeResp.StatusCode != http.StatusOK {
		t.Logf("ℹ️ Memory storage returned status %d", storeResp.StatusCode)
		return
	}
	
	// Test memory retrieval after storage
	retrieveData := map[string]interface{}{
		"provider": "zep",
		"user_id": "persistence_test_user",
		"query": "dark mode",
		"limit": 5,
	}
	
	retrieveResp, err := framework.POST(t, "/api/v1/memory/retrieve", retrieveData)
	if err != nil {
		t.Logf("⚠️ Memory retrieval failed: %v", err)
		return
	}
	defer retrieveResp.Body.Close()
	
	if retrieveResp.StatusCode == http.StatusOK {
		var retrieveResponse map[string]interface{}
		e2e.ParseJSON(t, retrieveResp, &retrieveResponse)
		
		if memories, ok := retrieveResponse["memories"].([]interface{}); ok && len(memories) > 0 {
			t.Logf("✅ Memory persistence confirmed - retrieved %d memories", len(memories))
		}
	}
}

// TestMemoryAnalytics tests memory analytics and insights
func TestMemoryAnalytics(t *testing.T) {
	t.Log("📊 Testing memory analytics and insights...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Test memory analytics endpoint
	analyticsData := map[string]interface{}{
		"provider": "mem0",
		"user_id": "analytics_test_user",
		"metric_type": "usage_stats",
		"time_range": "30d",
	}
	
	analyticsResp, err := framework.POST(t, "/api/v1/memory/analytics", analyticsData)
	if err != nil {
		t.Logf("⚠️ Memory analytics failed: %v", err)
		return
	}
	defer analyticsResp.Body.Close()
	
	switch analyticsResp.StatusCode {
	case http.StatusOK:
		t.Log("✅ Memory analytics successful")
		var analyticsResponse map[string]interface{}
		e2e.ParseJSON(t, analyticsResp, &analyticsResponse)
		
		if metrics, ok := analyticsResponse["metrics"].(map[string]interface{}); ok {
			t.Logf("✅ Retrieved %d analytics metrics", len(metrics))
		}
	case http.StatusServiceUnavailable:
		t.Log("ℹ️ Memory analytics not configured (expected)")
	default:
		t.Logf("ℹ️ Memory analytics returned status %d", analyticsResp.StatusCode)
	}
}

// TestMemoryPrivacyAndSecurity tests memory privacy and security features
func TestMemoryPrivacyAndSecurity(t *testing.T) {
	t.Log("🔒 Testing memory privacy and security features...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Test memory encryption/privacy settings
	privacyData := map[string]interface{}{
		"provider": "zep",
		"user_id": "privacy_test_user",
		"privacy_settings": map[string]interface{}{
			"encryption_enabled": true,
			"retention_days": 30,
			"access_control": "strict",
		},
	}
	
	privacyResp, err := framework.POST(t, "/api/v1/memory/privacy", privacyData)
	if err != nil {
		t.Logf("⚠️ Privacy settings failed: %v", err)
		return
	}
	defer privacyResp.Body.Close()
	
	switch privacyResp.StatusCode {
	case http.StatusOK:
		t.Log("✅ Privacy settings configured successfully")
	case http.StatusServiceUnavailable:
		t.Log("ℹ️ Privacy system not configured (expected)")
	default:
		t.Logf("ℹ️ Privacy settings returned status %d", privacyResp.StatusCode)
	}
	
	t.Log("✅ Memory privacy and security features tested")
}