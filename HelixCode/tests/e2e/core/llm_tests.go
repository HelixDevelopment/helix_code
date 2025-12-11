package core

import (
	"net/http"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST-E2E-010: LLM Provider Integration
func TestLLMProviderIntegration(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register and login
	registrationData := map[string]interface{}{
		"username": "llmtest",
		"email":    "llm@test.com",
		"password": "LLMPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "llmtest",
		"password": "LLMPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Step 1: List available LLM providers
	providersResp, err := framework.GET(t, "/api/v1/llm/providers")
	require.NoError(t, err)
	defer providersResp.Body.Close()
	e2e.AssertStatus(t, providersResp, http.StatusOK)

	var providersResponse map[string]interface{}
	e2e.ParseJSON(t, providersResp, &providersResponse)

	assert.Contains(t, providersResponse, "providers")
	providers := providersResponse["providers"].([]interface{})
	assert.Greater(t, len(providers), 0)

	// Verify common providers are available
	providerNames := make([]string, 0)
	for _, p := range providers {
		provider := p.(map[string]interface{})
		providerNames = append(providerNames, provider["name"].(string))
	}

	// Check for at least some providers
	assert.Contains(t, providerNames, "openai")

	// Step 2: Test LLM generation with different providers
	testPrompts := []struct {
		provider string
		model    string
		prompt   string
	}{
		{"openai", "gpt-3.5-turbo", "Write a simple Go function that adds two numbers"},
		{"anthropic", "claude-3-haiku", "Explain what a REST API is in one sentence"},
		{"local", "llama-3-8b", "What is the capital of France?"},
	}

	for _, test := range testPrompts {
		generationData := map[string]interface{}{
			"provider": test.provider,
			"model":    test.model,
			"prompt":   test.prompt,
			"max_tokens": 150,
			"temperature": 0.7,
		}

		genResp, err := framework.POST(t, "/api/v1/llm/generate", generationData)
		require.NoError(t, err)
		defer genResp.Body.Close()

		// Some providers might not be configured, so we accept both success and service unavailable
		if genResp.StatusCode == http.StatusOK {
			var genResponse map[string]interface{}
			e2e.ParseJSON(t, genResp, &genResponse)

			assert.Contains(t, genResponse, "response")
			assert.Contains(t, genResponse, "provider")
			assert.Contains(t, genResponse, "model")
			assert.Equal(t, test.provider, genResponse["provider"])
		} else if genResp.StatusCode == http.StatusServiceUnavailable {
			// Provider not configured, which is acceptable
			t.Logf("Provider %s not configured, skipping test", test.provider)
		}
	}

	// Step 3: Test LLM streaming
	streamData := map[string]interface{}{
		"provider": "openai",
		"model":    "gpt-3.5-turbo",
		"prompt":   "Count from 1 to 5 slowly",
		"stream":   true,
		"max_tokens": 50,
	}

	streamResp, err := framework.POST(t, "/api/v1/llm/generate", streamData)
	require.NoError(t, err)
	defer streamResp.Body.Close()

	// Streaming might not be supported by all setups
	if streamResp.StatusCode == http.StatusOK {
		// Verify streaming response format
		assert.Contains(t, streamResp.Header.Get("Content-Type"), "text/event-stream")
	} else if streamResp.StatusCode == http.StatusServiceUnavailable {
		t.Log("Streaming not available, skipping test")
	}
}

// TEST-E2E-011: LLM Model Management
func TestLLMModelManagement(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register and login
	registrationData := map[string]interface{}{
		"username": "modeltest",
		"email":    "model@test.com",
		"password": "ModelPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "modeltest",
		"password": "ModelPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Step 1: List available models for specific provider
	modelsResp, err := framework.GET(t, "/api/v1/llm/providers/openai/models")
	require.NoError(t, err)
	defer modelsResp.Body.Close()
	e2e.AssertStatus(t, modelsResp, http.StatusOK)

	var modelsResponse map[string]interface{}
	e2e.ParseJSON(t, modelsResp, &modelsResponse)

	assert.Contains(t, modelsResponse, "models")
	models := modelsResponse["models"].([]interface{})
	assert.Greater(t, len(models), 0)

	// Verify expected models are available
	modelIDs := make([]string, 0)
	for _, m := range models {
		model := m.(map[string]interface{})
		modelIDs = append(modelIDs, model["id"].(string))
	}

	// Check for common models
	assert.Contains(t, modelIDs, "gpt-3.5-turbo")

	// Step 2: Test model capabilities
	capabilitiesResp, err := framework.GET(t, "/api/v1/llm/models/gpt-3.5-turbo/capabilities")
	require.NoError(t, err)
	defer capabilitiesResp.Body.Close()
	e2e.AssertStatus(t, capabilitiesResp, http.StatusOK)

	var capabilitiesResponse map[string]interface{}
	e2e.ParseJSON(t, capabilitiesResp, &capabilitiesResponse)

	assert.Contains(t, capabilitiesResponse, "capabilities")
	assert.Contains(t, capabilitiesResponse, "max_tokens")
	assert.Contains(t, capabilitiesResponse, "supports_streaming")

	// Step 3: Test model health check
	healthResp, err := framework.GET(t, "/api/v1/llm/models/gpt-3.5-turbo/health")
	require.NoError(t, err)
	defer healthResp.Body.Close()

	// Health check might fail if provider not configured
	if healthResp.StatusCode == http.StatusOK {
		var healthResponse map[string]interface{}
		e2e.ParseJSON(t, healthResp, &healthResponse)

		assert.Contains(t, healthResponse, "status")
		assert.Contains(t, []string{"healthy", "unhealthy", "unknown"}, healthResponse["status"])
	}

	// Step 4: Test model usage statistics
	statsResp, err := framework.GET(t, "/api/v1/llm/usage")
	require.NoError(t, err)
	defer statsResp.Body.Close()
	e2e.AssertStatus(t, statsResp, http.StatusOK)

	var statsResponse map[string]interface{}
	e2e.ParseJSON(t, statsResp, &statsResponse)

	assert.Contains(t, statsResponse, "usage_stats")
	assert.Contains(t, statsResponse, "total_requests")
	assert.Contains(t, statsResponse, "total_tokens")
}

// TEST-E2E-012: LLM Context and Memory Management
func TestLLMContextMemory(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register and login
	registrationData := map[string]interface{}{
		"username": "memorytest",
		"email":    "memory@test.com",
		"password": "MemoryPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "memorytest",
		"password": "MemoryPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Step 1: Create conversation context
	conversationData := map[string]interface{}{
		"name":        "Go Programming Help",
		"description": "Conversation about Go programming concepts",
		"context_type": "programming",
	}

	convResp, err := framework.POST(t, "/api/v1/conversations", conversationData)
	require.NoError(t, err)
	defer convResp.Body.Close()
	e2e.AssertStatus(t, convResp, http.StatusCreated)

	var convResponse map[string]interface{}
	e2e.ParseJSON(t, convResp, &convResponse)

	assert.Contains(t, convResponse, "conversation_id")
	conversationID := convResponse["conversation_id"].(string)

	// Step 2: Add context to conversation
	contextData := map[string]interface{}{
		"conversation_id": conversationID,
		"context_items": []map[string]interface{}{
			{
				"type": "file",
				"content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello World\")\n}",
				"path": "main.go",
			},
			{
				"type": "instruction",
				"content": "Help me understand Go structs and interfaces",
			},
		},
	}

	contextResp, err := framework.POST(t, "/api/v1/conversations/context", contextData)
	require.NoError(t, err)
	defer contextResp.Body.Close()
	e2e.AssertStatus(t, contextResp, http.StatusOK)

	// Step 3: Generate response with context
	genWithContextData := map[string]interface{}{
		"conversation_id": conversationID,
		"provider":        "openai",
		"model":           "gpt-3.5-turbo",
		"prompt":          "Can you explain how to create a struct in Go?",
		"include_context": true,
	}

	genResp, err := framework.POST(t, "/api/v1/llm/generate", genWithContextData)
	require.NoError(t, err)
	defer genResp.Body.Close()

	if genResp.StatusCode == http.StatusOK {
		var genResponse map[string]interface{}
		e2e.ParseJSON(t, genResp, &genResponse)

		assert.Contains(t, genResponse, "response")
		assert.Contains(t, genResponse, "context_used")
	}

	// Step 4: Test memory persistence
	memoryData := map[string]interface{}{
		"conversation_id": conversationID,
		"memory_items": []map[string]interface{}{
			{
				"type": "user_preference",
				"key":   "programming_level",
				"value": "beginner",
			},
			{
				"type": "project_info",
				"key":   "project_type",
				"value": "go_cli",
			},
		},
	}

	memoryResp, err := framework.POST(t, "/api/v1/memory/store", memoryData)
	require.NoError(t, err)
	defer memoryResp.Body.Close()
	e2e.AssertStatus(t, memoryResp, http.StatusOK)

	// Step 5: Retrieve stored memory
	retrieveMemoryData := map[string]interface{}{
		"conversation_id": conversationID,
		"memory_types":    []string{"user_preference", "project_info"},
	}

	retrieveResp, err := framework.POST(t, "/api/v1/memory/retrieve", retrieveMemoryData)
	require.NoError(t, err)
	defer retrieveResp.Body.Close()
	e2e.AssertStatus(t, retrieveResp, http.StatusOK)

	var retrieveResponse map[string]interface{}
	e2e.ParseJSON(t, retrieveResp, &retrieveResponse)

	assert.Contains(t, retrieveResponse, "memory_items")
	memoryItems := retrieveResponse["memory_items"].([]interface{})
	assert.Greater(t, len(memoryItems), 0)

	// Verify programming level is stored
	found := false
	for _, item := range memoryItems {
		memoryItem := item.(map[string]interface{})
		if memoryItem["key"] == "programming_level" {
			found = true
			assert.Equal(t, "beginner", memoryItem["value"])
			break
		}
	}
	assert.True(t, found, "Programming level should be stored in memory")

	// Step 6: Test conversation history
	historyResp, err := framework.GET(t, "/api/v1/conversations/"+conversationID+"/history")
	require.NoError(t, err)
	defer historyResp.Body.Close()
	e2e.AssertStatus(t, historyResp, http.StatusOK)

	var historyResponse map[string]interface{}
	e2e.ParseJSON(t, historyResp, &historyResponse)

	assert.Contains(t, historyResponse, "messages")
	assert.Contains(t, historyResponse, "conversation_id")
	assert.Equal(t, conversationID, historyResponse["conversation_id"])
}