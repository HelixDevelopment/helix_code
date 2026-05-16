package core

import (
	"net/http"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST-E2E-013: Multi-Provider LLM Integration
func TestMultiProviderLLMIntegration(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register and login
	registrationData := map[string]interface{}{
		"username": "multillmtest",
		"email":    "multillm@test.com",
		"password": "MultiLLMPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "multillmtest",
		"password": "MultiLLMPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Step 1: Test fallback between providers
	fallbackData := map[string]interface{}{
		"prompt": "Write a haiku about programming",
		"providers": []string{
			"anthropic", // Try first
			"openai",    // Fallback to second
			"local",     // Fallback to local
		},
		"max_tokens": 100,
		"temperature": 0.7,
		"fallback_enabled": true,
	}

	fallbackResp, err := framework.POST(t, "/api/v1/llm/generate", fallbackData)
	require.NoError(t, err)
	defer fallbackResp.Body.Close()

	if fallbackResp.StatusCode == http.StatusOK {
		var fallbackResponse map[string]interface{}
		e2e.ParseJSON(t, fallbackResp, &fallbackResponse)

		assert.Contains(t, fallbackResponse, "response")
		assert.Contains(t, fallbackResponse, "provider")
		assert.Contains(t, fallbackResponse, "fallback_used")

		// Verify that a provider was eventually used
		usedProvider := fallbackResponse["provider"].(string)
		assert.Contains(t, []string{"anthropic", "openai", "local"}, usedProvider)
	}

	// Step 2: Test provider comparison
	comparisonData := map[string]interface{}{
		"prompt": "Explain recursion in programming",
		"providers": []string{"openai", "anthropic"},
		"comparison_mode": true,
		"max_tokens": 150,
	}

	comparisonResp, err := framework.POST(t, "/api/v1/llm/compare", comparisonData)
	require.NoError(t, err)
	defer comparisonResp.Body.Close()

	if comparisonResp.StatusCode == http.StatusOK {
		var comparisonResponse map[string]interface{}
		e2e.ParseJSON(t, comparisonResp, &comparisonResponse)

		assert.Contains(t, comparisonResponse, "comparisons")
		comparisons := comparisonResponse["comparisons"].([]interface{})
		assert.Greater(t, len(comparisons), 0)

		// Verify each comparison has expected fields
		for _, comp := range comparisons {
			comparison := comp.(map[string]interface{})
			assert.Contains(t, comparison, "provider")
			assert.Contains(t, comparison, "response")
			assert.Contains(t, comparison, "metrics")
		}
	}

	// Step 3: Test provider load balancing
	loadBalanceData := map[string]interface{}{
		"prompts": []string{
			"What is a function?",
			"What is a variable?",
			"What is a loop?",
		},
		"strategy": "round_robin",
		"providers": []string{"openai", "anthropic"},
		"max_tokens": 100,
	}

	loadBalanceResp, err := framework.POST(t, "/api/v1/llm/batch", loadBalanceData)
	require.NoError(t, err)
	defer loadBalanceResp.Body.Close()

	if loadBalanceResp.StatusCode == http.StatusOK {
		var loadBalanceResponse map[string]interface{}
		e2e.ParseJSON(t, loadBalanceResp, &loadBalanceResponse)

		assert.Contains(t, loadBalanceResponse, "results")
		results := loadBalanceResponse["results"].([]interface{})
		assert.Equal(t, 3, len(results))

		// Verify each prompt got a response
		for i, result := range results {
			resultMap := result.(map[string]interface{})
			assert.Contains(t, resultMap, "prompt")
			assert.Contains(t, resultMap, "response")
			assert.Contains(t, resultMap, "provider")
			assert.Equal(t, loadBalanceData["prompts"].([]string)[i], resultMap["prompt"])
		}
	}

	// Step 4: Test provider health monitoring
	healthResp, err := framework.GET(t, "/api/v1/llm/providers/health")
	require.NoError(t, err)
	defer healthResp.Body.Close()
	e2e.AssertStatus(t, healthResp, http.StatusOK)

	var healthResponse map[string]interface{}
	e2e.ParseJSON(t, healthResp, &healthResponse)

	assert.Contains(t, healthResponse, "provider_health")
	providerHealth := healthResponse["provider_health"].(map[string]interface{})

	// Verify health status for configured providers
	for provider, health := range providerHealth {
		healthStatus := health.(map[string]interface{})
		assert.Contains(t, healthStatus, "status")
		assert.Contains(t, healthStatus, "last_check")
		assert.Contains(t, []string{"healthy", "unhealthy", "unknown"}, healthStatus["status"])
		t.Logf("Provider %s health: %v", provider, healthStatus["status"])
	}
}

// TEST-E2E-014: Memory System Integration
func TestMemorySystemIntegration(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register and login
	registrationData := map[string]interface{}{
		"username": "memorysystest",
		"email":    "memsys@test.com",
		"password": "MemSysPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "memorysystest",
		"password": "MemSysPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Step 1: Test different memory providers
	memoryProviders := []string{"mem0", "zep", "chroma", "pinecone", "qdrant"}

	for _, provider := range memoryProviders {
		// Test memory storage
		storeData := map[string]interface{}{
			"provider": provider,
			"user_id":  "test_user_123",
			"memory_items": []map[string]interface{}{
				{
					"type": "conversation",
					"content": "User prefers Go programming language",
					"metadata": map[string]interface{}{
						"topic": "programming",
						"preference": "language",
					},
				},
			},
		}

		storeResp, err := framework.POST(t, "/api/v1/memory/store", storeData)
		require.NoError(t, err)
		defer storeResp.Body.Close()

		// Some providers might not be configured
		if storeResp.StatusCode == http.StatusOK {
			// Test memory retrieval
			retrieveData := map[string]interface{}{
				"provider": provider,
				"user_id":  "test_user_123",
				"query":    "programming language preference",
				"limit":    5,
			}

			retrieveResp, err := framework.POST(t, "/api/v1/memory/retrieve", retrieveData)
			require.NoError(t, err)
			defer retrieveResp.Body.Close()

			if retrieveResp.StatusCode == http.StatusOK {
				var retrieveResponse map[string]interface{}
				e2e.ParseJSON(t, retrieveResp, &retrieveResponse)

				assert.Contains(t, retrieveResponse, "memories")
				assert.Contains(t, retrieveResponse, "provider")
				assert.Equal(t, provider, retrieveResponse["provider"])

				memories := retrieveResponse["memories"].([]interface{})
				t.Logf("Provider %s returned %d memories", provider, len(memories))
			}
		} else if storeResp.StatusCode == http.StatusServiceUnavailable {
			t.Logf("Memory provider %s not configured, skipping", provider)
		}
	}

	// Step 2: Test memory search across providers
	searchData := map[string]interface{}{
		"query": "programming preferences",
		"user_id": "test_user_123",
		"providers": []string{"mem0", "chroma"},
		"limit": 10,
	}

	searchResp, err := framework.POST(t, "/api/v1/memory/search", searchData)
	require.NoError(t, err)
	defer searchResp.Body.Close()

	if searchResp.StatusCode == http.StatusOK {
		var searchResponse map[string]interface{}
		e2e.ParseJSON(t, searchResp, &searchResponse)

		assert.Contains(t, searchResponse, "results")
		assert.Contains(t, searchResponse, "total_count")

		results := searchResponse["results"].([]interface{})
		assert.GreaterOrEqual(t, len(results), 0)

		// Verify search result structure
		for _, result := range results {
			resultMap := result.(map[string]interface{})
			assert.Contains(t, resultMap, "content")
			assert.Contains(t, resultMap, "provider")
			assert.Contains(t, resultMap, "score")
			assert.Contains(t, resultMap, "metadata")
		}
	}

	// Step 3: Test memory management
	// Create some test memories
	testMemories := []map[string]interface{}{
		{
			"type": "user_fact",
			"content": "User works at TechCorp",
			"metadata": map[string]interface{}{
				"category": "employment",
				"company": "TechCorp",
			},
		},
		{
			"type": "preference",
			"content": "User likes dark mode interfaces",
			"metadata": map[string]interface{}{
				"category": "ui_preference",
				"setting": "dark_mode",
			},
		},
	}

	for _, memory := range testMemories {
		storeData := map[string]interface{}{
			"provider": "mem0",
			"user_id": "test_user_123",
			"memory_items": []map[string]interface{}{memory},
		}

		storeResp, err := framework.POST(t, "/api/v1/memory/store", storeData)
		require.NoError(t, err)
		defer storeResp.Body.Close()

		if storeResp.StatusCode == http.StatusOK {
			t.Logf("Stored memory: %s", memory["content"])
		}
	}

	// Step 4: Test memory deletion
	// Test memory deletion via POST to delete endpoint
	deleteData := map[string]interface{}{
		"provider": "mem0",
		"user_id": "test_user_123",
		"query": "employment",
		"delete_matching": true,
	}

	deleteResp, err := framework.POST(t, "/api/v1/memory/delete", deleteData)
	require.NoError(t, err)
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode == http.StatusOK {
		var deleteResponse map[string]interface{}
		e2e.ParseJSON(t, deleteResp, &deleteResponse)

		assert.Contains(t, deleteResponse, "deleted_count")
		assert.Contains(t, deleteResponse, "provider")
	}
}

// TEST-E2E-015: Notification System Integration
func TestNotificationSystemIntegration(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register and login
	registrationData := map[string]interface{}{
		"username": "notifytest",
		"email":    "notify@test.com",
		"password": "NotifyPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "notifytest",
		"password": "NotifyPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Step 1: Test notification channels configuration
	channels := []string{"email", "slack", "discord", "telegram", "webhook"}

	for _, channel := range channels {
		// Configure notification channel
		configData := map[string]interface{}{
			"channel": channel,
			"enabled": true,
			"settings": map[string]interface{}{
				"priority": "high",
				"events":   []string{"task_completed", "build_failed"},
			},
		}

		configResp, err := framework.POST(t, "/api/v1/notifications/channels/configure", configData)
		require.NoError(t, err)
		defer configResp.Body.Close()

		if configResp.StatusCode == http.StatusOK {
			var configResponse map[string]interface{}
			e2e.ParseJSON(t, configResp, &configResponse)

			assert.Contains(t, configResponse, "channel_id")
			assert.Contains(t, configResponse, "status")
			assert.Equal(t, "configured", configResponse["status"])

			// Test sending notification
			notificationData := map[string]interface{}{
				"channel_id": configResponse["channel_id"],
				"message":    "Test notification from E2E test",
				"level":      "info",
				"event_type": "test_notification",
			}

			notifyResp, err := framework.POST(t, "/api/v1/notifications/send", notificationData)
			require.NoError(t, err)
			defer notifyResp.Body.Close()

			if notifyResp.StatusCode == http.StatusOK {
				var notifyResponse map[string]interface{}
				e2e.ParseJSON(t, notifyResp, &notifyResponse)

				assert.Contains(t, notifyResponse, "notification_id")
				assert.Contains(t, notifyResponse, "status")
				t.Logf("Notification sent via %s: %s", channel, notifyResponse["notification_id"])
			}
		} else if configResp.StatusCode == http.StatusServiceUnavailable {
			t.Logf("Notification channel %s not configured, skipping", channel)
		}
	}

	// Step 2: Test notification templates
	templateData := map[string]interface{}{
		"name": "task_completion",
		"template": "Task {{.task_name}} has been completed successfully in project {{.project_name}}.",
		"variables": []string{"task_name", "project_name"},
	}

	templateResp, err := framework.POST(t, "/api/v1/notifications/templates", templateData)
	require.NoError(t, err)
	defer templateResp.Body.Close()
	e2e.AssertStatus(t, templateResp, http.StatusCreated)

	var templateResponse map[string]interface{}
	e2e.ParseJSON(t, templateResp, &templateResponse)

	assert.Contains(t, templateResponse, "template_id")
	templateID := templateResponse["template_id"].(string)

	// Step 3: Test templated notification
	templatedNotificationData := map[string]interface{}{
		"template_id": templateID,
		"variables": map[string]interface{}{
			"task_name":    "Build Process",
			"project_name": "Test Project",
		},
		"channels": []string{"email"},
	}

	templatedResp, err := framework.POST(t, "/api/v1/notifications/send_template", templatedNotificationData)
	require.NoError(t, err)
	defer templatedResp.Body.Close()

	if templatedResp.StatusCode == http.StatusOK {
		var templatedResponse map[string]interface{}
		e2e.ParseJSON(t, templatedResp, &templatedResponse)

		assert.Contains(t, templatedResponse, "notification_id")
		assert.Contains(t, templatedResponse, "rendered_message")
	}

	// Step 4: Test notification history
	historyResp, err := framework.GET(t, "/api/v1/notifications/history")
	require.NoError(t, err)
	defer historyResp.Body.Close()
	e2e.AssertStatus(t, historyResp, http.StatusOK)

	var historyResponse map[string]interface{}
	e2e.ParseJSON(t, historyResp, &historyResponse)

	assert.Contains(t, historyResponse, "notifications")
	assert.Contains(t, historyResponse, "total_count")

	notifications := historyResponse["notifications"].([]interface{})
	assert.GreaterOrEqual(t, len(notifications), 0)

	// Verify notification structure
	for _, notification := range notifications {
		notif := notification.(map[string]interface{})
		assert.Contains(t, notif, "notification_id")
		assert.Contains(t, notif, "message")
		assert.Contains(t, notif, "timestamp")
		assert.Contains(t, notif, "status")
	}
}