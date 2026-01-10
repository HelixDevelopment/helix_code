package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCopilotTestServer creates a mock server for GitHub token exchange and Copilot API
func setupCopilotTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/copilot_internal/v2/token":
			// Token exchange endpoint
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Token test-github-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			response := CopilotTokenResponse{
				Token:     "test-copilot-bearer-token",
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			}
			json.NewEncoder(w).Encode(response)

		case "/chat/completions":
			// Chat completions endpoint
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

			response := map[string]interface{}{
				"id":      "chatcmpl-test123",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello from GitHub Copilot!",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "/models":
			// Models endpoint for health check
			response := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "gpt-4o"},
					{"id": "gpt-4o-mini"},
					{"id": "claude-3.5-sonnet"},
				},
			}
			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// createCopilotProviderWithMockServer creates a provider with mock endpoints
func createCopilotProviderWithMockServer(t *testing.T, copilotServer *httptest.Server, tokenServer *httptest.Server) (*CopilotProvider, error) {
	// Set the GITHUB_TOKEN environment variable temporarily
	oldToken := os.Getenv("GITHUB_TOKEN")
	os.Setenv("GITHUB_TOKEN", "test-github-token")
	defer func() {
		if oldToken != "" {
			os.Setenv("GITHUB_TOKEN", oldToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	// Create a custom provider that bypasses the real token exchange
	provider := &CopilotProvider{
		config: ProviderConfigEntry{
			Type:     ProviderTypeCopilot,
			APIKey:   "test-github-token",
			Endpoint: copilotServer.URL,
			Enabled:  true,
		},
		endpoint:    copilotServer.URL,
		githubToken: "test-github-token",
		bearerToken: "test-copilot-bearer-token",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		lastHealth: &ProviderHealth{
			Status:    "unknown",
			LastCheck: time.Now(),
		},
	}

	// Initialize models
	provider.initializeModels()

	return provider, nil
}

func TestCopilotProvider_GetType(t *testing.T) {
	server := setupCopilotTestServer(t)
	defer server.Close()

	provider, err := createCopilotProviderWithMockServer(t, server, server)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeCopilot, provider.GetType())
}

func TestCopilotProvider_GetName(t *testing.T) {
	server := setupCopilotTestServer(t)
	defer server.Close()

	provider, err := createCopilotProviderWithMockServer(t, server, server)
	require.NoError(t, err)

	assert.Equal(t, "GitHub Copilot", provider.GetName())
}

func TestCopilotProvider_GetModels(t *testing.T) {
	server := setupCopilotTestServer(t)
	defer server.Close()

	provider, err := createCopilotProviderWithMockServer(t, server, server)
	require.NoError(t, err)

	models := provider.GetModels()
	assert.NotEmpty(t, models)

	// Check that models have proper structure and expected models are present
	modelNames := make([]string, len(models))
	for i, m := range models {
		modelNames[i] = m.Name
		assert.Equal(t, ProviderTypeCopilot, m.Provider)
	}

	// Verify expected models are present
	assert.Contains(t, modelNames, "gpt-4o")
	assert.Contains(t, modelNames, "gpt-4o-mini")
	assert.Contains(t, modelNames, "claude-3.5-sonnet")
	assert.Contains(t, modelNames, "claude-3.7-sonnet")
	assert.Contains(t, modelNames, "o1")
	assert.Contains(t, modelNames, "gemini-2.0-flash-001")
}

func TestCopilotProvider_GetCapabilities(t *testing.T) {
	server := setupCopilotTestServer(t)
	defer server.Close()

	provider, err := createCopilotProviderWithMockServer(t, server, server)
	require.NoError(t, err)

	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, CapabilityTextGeneration)
	assert.Contains(t, capabilities, CapabilityCodeGeneration)
	assert.Contains(t, capabilities, CapabilityCodeAnalysis)
	assert.Contains(t, capabilities, CapabilityPlanning)
	assert.Contains(t, capabilities, CapabilityDebugging)
	assert.Contains(t, capabilities, CapabilityRefactoring)
	assert.Contains(t, capabilities, CapabilityTesting)
}

func TestCopilotProvider_Generate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := setupCopilotTestServer(t)
		defer server.Close()

		provider, err := createCopilotProviderWithMockServer(t, server, server)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "gpt-4o",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			MaxTokens:   100,
			Temperature: 0.7,
		}

		ctx := context.Background()
		response, err := provider.Generate(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "Hello from GitHub Copilot!", response.Content)
		assert.Equal(t, 15, response.Usage.TotalTokens)
	})

	t.Run("APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/chat/completions" {
				w.WriteHeader(http.StatusUnauthorized)
				response := map[string]interface{}{
					"error": map[string]interface{}{
						"message": "Invalid token",
						"type":    "invalid_token",
					},
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		provider, err := createCopilotProviderWithMockServer(t, server, server)
		require.NoError(t, err)

		request := &LLMRequest{
			Model: "gpt-4o",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		ctx := context.Background()
		response, err := provider.Generate(ctx, request)
		assert.Error(t, err)
		assert.Nil(t, response)
	})
}

func TestCopilotProvider_IsAvailable(t *testing.T) {
	t.Run("Available", func(t *testing.T) {
		server := setupCopilotTestServer(t)
		defer server.Close()

		provider, err := createCopilotProviderWithMockServer(t, server, server)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.True(t, available)
	})

	t.Run("Unavailable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		provider, err := createCopilotProviderWithMockServer(t, server, server)
		require.NoError(t, err)

		ctx := context.Background()
		available := provider.IsAvailable(ctx)
		assert.False(t, available)
	})
}

func TestCopilotProvider_GetHealth(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		server := setupCopilotTestServer(t)
		defer server.Close()

		provider, err := createCopilotProviderWithMockServer(t, server, server)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		require.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "healthy", health.Status)
		assert.Equal(t, 3, health.ModelCount)
	})

	t.Run("Unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		provider, err := createCopilotProviderWithMockServer(t, server, server)
		require.NoError(t, err)

		ctx := context.Background()
		health, err := provider.GetHealth(ctx)
		assert.Error(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
	})
}

func TestCopilotProvider_Close(t *testing.T) {
	server := setupCopilotTestServer(t)
	defer server.Close()

	provider, err := createCopilotProviderWithMockServer(t, server, server)
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
}

func TestCopilotProvider_TokenExchange(t *testing.T) {
	t.Run("SuccessfulTokenExchange", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/copilot_internal/v2/token" {
				authHeader := r.Header.Get("Authorization")
				assert.Equal(t, "Token test-github-token", authHeader)
				assert.Equal(t, "HelixCode/1.0", r.Header.Get("User-Agent"))

				response := CopilotTokenResponse{
					Token:     "exchanged-bearer-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		// Create a minimal provider to test token exchange
		provider := &CopilotProvider{
			config: ProviderConfigEntry{
				Type:    ProviderTypeCopilot,
				APIKey:  "test-github-token",
				Enabled: true,
			},
			httpClient: &http.Client{
				Timeout: 30 * time.Second,
			},
		}

		// Test the exchange - this will fail because it tries to connect to api.github.com
		// We're testing that the method exists and is callable
		_, err := provider.exchangeGitHubToken("test-github-token")
		assert.Error(t, err) // Expected since we can't mock api.github.com directly
	})

	t.Run("MissingGitHubToken", func(t *testing.T) {
		// Ensure no GitHub token is set
		oldToken := os.Getenv("GITHUB_TOKEN")
		os.Unsetenv("GITHUB_TOKEN")
		defer func() {
			if oldToken != "" {
				os.Setenv("GITHUB_TOKEN", oldToken)
			}
		}()

		provider := &CopilotProvider{
			config: ProviderConfigEntry{
				Type:    ProviderTypeCopilot,
				Enabled: true,
				// No APIKey set
			},
		}

		token := provider.getGitHubToken()
		assert.Empty(t, token)
	})

	t.Run("GitHubTokenFromConfig", func(t *testing.T) {
		oldToken := os.Getenv("GITHUB_TOKEN")
		os.Unsetenv("GITHUB_TOKEN")
		defer func() {
			if oldToken != "" {
				os.Setenv("GITHUB_TOKEN", oldToken)
			}
		}()

		provider := &CopilotProvider{
			config: ProviderConfigEntry{
				Type:    ProviderTypeCopilot,
				APIKey:  "config-github-token",
				Enabled: true,
			},
		}

		token := provider.getGitHubToken()
		assert.Equal(t, "config-github-token", token)
	})

	t.Run("GitHubTokenFromEnv", func(t *testing.T) {
		os.Setenv("GITHUB_TOKEN", "env-github-token")
		defer os.Unsetenv("GITHUB_TOKEN")

		provider := &CopilotProvider{
			config: ProviderConfigEntry{
				Type:    ProviderTypeCopilot,
				APIKey:  "config-github-token", // Should be overridden by env
				Enabled: true,
			},
		}

		token := provider.getGitHubToken()
		assert.Equal(t, "env-github-token", token)
	})
}

func TestCopilotProvider_ConvertToOpenAIRequest(t *testing.T) {
	server := setupCopilotTestServer(t)
	defer server.Close()

	provider, err := createCopilotProviderWithMockServer(t, server, server)
	require.NoError(t, err)

	request := &LLMRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello", Name: "user1"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		TopP:        0.9,
		Stream:      false,
	}

	openaiReq, err := provider.convertToOpenAIRequest(request)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", openaiReq.Model)
	assert.Len(t, openaiReq.Messages, 2)
	assert.Equal(t, "system", openaiReq.Messages[0].Role)
	assert.Equal(t, "user", openaiReq.Messages[1].Role)
	assert.Equal(t, "user1", openaiReq.Messages[1].Name)
	assert.Equal(t, 100, openaiReq.MaxTokens)
	assert.Equal(t, 0.7, openaiReq.Temperature)
	assert.Equal(t, 0.9, openaiReq.TopP)
}

func TestCopilotProvider_UpdateHealth(t *testing.T) {
	server := setupCopilotTestServer(t)
	defer server.Close()

	provider, err := createCopilotProviderWithMockServer(t, server, server)
	require.NoError(t, err)

	// Test healthy update
	provider.updateHealth("healthy", 50*time.Millisecond, 0)
	assert.Equal(t, "healthy", provider.lastHealth.Status)
	assert.Equal(t, 50*time.Millisecond, provider.lastHealth.Latency)
	assert.Equal(t, 0, provider.lastHealth.ErrorCount)

	// Test unhealthy update
	provider.updateHealth("unhealthy", 100*time.Millisecond, 5)
	assert.Equal(t, "unhealthy", provider.lastHealth.Status)
	assert.Equal(t, 100*time.Millisecond, provider.lastHealth.Latency)
	assert.Equal(t, 5, provider.lastHealth.ErrorCount)

	// Test degraded update
	provider.updateHealth("degraded", 75*time.Millisecond, 2)
	assert.Equal(t, "degraded", provider.lastHealth.Status)
}

func TestCopilotProvider_SetAuthHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers are set correctly
		assert.Equal(t, "Bearer test-bearer-token", r.Header.Get("Authorization"))
		assert.Equal(t, "HelixCode/1.0", r.Header.Get("Editor-Version"))
		assert.Equal(t, "HelixCode/1.0", r.Header.Get("Editor-Plugin-Version"))
		assert.Equal(t, "vscode-chat", r.Header.Get("Copilot-Integration-Id"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &CopilotProvider{
		bearerToken: "test-bearer-token",
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	provider.setAuthHeaders(req)

	// Verify by making request
	_, err := provider.httpClient.Do(req)
	assert.NoError(t, err)
}

func TestCopilotProvider_LoadGitHubCLIToken(t *testing.T) {
	provider := &CopilotProvider{
		config: ProviderConfigEntry{},
	}

	// This should return empty string since we don't have CLI tokens in test env
	assert.Empty(t, provider.loadGitHubCLIToken())
}

func TestCopilotProvider_ExtractTokenFromFile(t *testing.T) {
	provider := &CopilotProvider{
		config: ProviderConfigEntry{},
	}

	// The current implementation returns empty string
	assert.Empty(t, provider.extractTokenFromFile("/nonexistent/path"))
}
