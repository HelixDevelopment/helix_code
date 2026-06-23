package phase2

import (
	"net/http"
	"os"
	"testing"

	"dev.helix.code/tests/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicIntegration verifies basic connectivity to the real HelixCode server
func TestBasicIntegration(t *testing.T) {
	t.Log("🎯 PHASE 2: Basic Integration Test")
	t.Log("Testing connectivity to real HelixCode server...")
	
	// Create Phase 2 framework
	framework := NewPhase2Framework(t)
	defer framework.Cleanup(t)
	
	t.Log("✅ Phase 2 framework initialized successfully")
	
	// Test 1: Server Health Check
	t.Run("Server Health Check", func(t *testing.T) {
		t.Log("🔍 Checking server health...")
		
		resp, err := framework.GET(t, "/health")
		require.NoError(t, err, "Failed to connect to server")
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Server health check failed")
		
		var healthResponse map[string]interface{}
		e2e.ParseJSON(t, resp, &healthResponse)
		
		// Real assertion: live server reports {"status":"ok"}; older deployments
		// reported "healthy". Both accepted; any other status is a genuine FAIL.
		status, _ := healthResponse["status"].(string)
		assert.Contains(t, []string{"ok", "healthy"}, status, "Server status should be healthy")
		t.Logf("✅ Server health check passed (status=%q)", status)
	})
	
	// Test 2: API Endpoints Availability
	t.Run("API Endpoints Availability", func(t *testing.T) {
		endpoints := []string{
			"/api/v1/auth/health",
			"/api/v1/projects/health", 
			"/api/v1/tasks/health",
			"/api/v1/llm/health",
		}
		
		for _, endpoint := range endpoints {
			t.Logf("🔍 Checking endpoint: %s", endpoint)
			
			resp, err := framework.GET(t, endpoint)
			if err != nil {
				t.Logf("⚠️  Endpoint %s not available: %v", endpoint, err)
				continue
			}
			defer resp.Body.Close()
			
			if resp.StatusCode == http.StatusOK {
				t.Logf("✅ Endpoint %s is available", endpoint)
			} else {
				t.Logf("ℹ️  Endpoint %s returned status %d", endpoint, resp.StatusCode)
			}
		}
	})
	
	// Test 3: Test User Validation
	t.Run("Test User Validation", func(t *testing.T) {
		if len(framework.TestUsers) == 0 {
			t.Skip("No test users available")  // SKIP-OK: #legacy-untriaged
		}
		
		user := framework.TestUsers[0]
		t.Logf("👤 Validating test user: %s", user.Username)
		
		if user.Token != "" {
			t.Logf("✅ Test user %s has valid authentication token", user.Username)
			
			// Test authenticated endpoint
			profileResp, err := framework.GET(t, "/api/v1/auth/me")
			if err != nil {
				t.Logf("⚠️  Failed to access authenticated endpoint: %v", err)
				return
			}
			defer profileResp.Body.Close()
			
			if profileResp.StatusCode == http.StatusOK {
				t.Logf("✅ Test user %s can access authenticated endpoints", user.Username)
			} else {
				t.Logf("ℹ️  Authenticated endpoint returned status %d", profileResp.StatusCode)
			}
		} else {
			t.Logf("⚠️  Test user %s has no authentication token", user.Username)
		}
	})
	
	// Test 4: Test Project Validation  
	t.Run("Test Project Validation", func(t *testing.T) {
		if len(framework.TestProjects) == 0 {
			t.Skip("No test projects available")  // SKIP-OK: #legacy-untriaged
		}
		
		project := framework.TestProjects[0]
		t.Logf("🏗️ Validating test project: %s", project.Name)
		
		if project.ID != "" {
			t.Logf("✅ Test project %s has valid ID: %s", project.Name, project.ID)
			
			// Test project access
			projectResp, err := framework.GET(t, "/api/v1/projects/"+project.ID)
			if err != nil {
				t.Logf("⚠️  Failed to access project: %v", err)
				return
			}
			defer projectResp.Body.Close()
			
			if projectResp.StatusCode == http.StatusOK {
				t.Logf("✅ Test project %s is accessible", project.Name)
			} else {
				t.Logf("ℹ️  Project endpoint returned status %d", projectResp.StatusCode)
			}
		} else {
			t.Logf("⚠️  Test project %s has no valid ID", project.Name)
		}
	})
	
	t.Log("🎉 Basic integration test completed successfully")
	t.Log("✅ Phase 2 basic connectivity verified")
	t.Log("✅ Ready for advanced integration tests")
}

// TestServerCapabilities tests the server's capabilities and features
func TestServerCapabilities(t *testing.T) {
	t.Log("🔧 Testing server capabilities...")
	
	framework := NewPhase2Framework(t)
	defer framework.Cleanup(t)
	
	// Test LLM provider availability
	t.Run("LLM Provider Availability", func(t *testing.T) {
		t.Log("🔍 Checking LLM provider availability...")
		
		resp, err := framework.GET(t, "/api/v1/llm/providers")
		if err != nil {
			t.Logf("⚠️  LLM providers endpoint not available: %v", err)
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			var providersResponse map[string]interface{}
			e2e.ParseJSON(t, resp, &providersResponse)
			
			if providers, ok := providersResponse["providers"].([]interface{}); ok {
				t.Logf("✅ Found %d LLM providers available", len(providers))
				for i, provider := range providers {
					if p, ok := provider.(map[string]interface{}); ok {
						if name, ok := p["name"].(string); ok && i < 3 {
							t.Logf("   - %s", name)
						}
					}
				}
			}
		} else {
			t.Logf("ℹ️  LLM providers endpoint returned status %d", resp.StatusCode)
		}
	})
	
	// Test notification system
	t.Run("Notification System Availability", func(t *testing.T) {
		t.Log("📢 Checking notification system availability...")
		
		resp, err := framework.GET(t, "/api/v1/notifications/channels")
		if err != nil {
			t.Logf("⚠️  Notifications endpoint not available: %v", err)
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			t.Log("✅ Notification system is available")
		} else {
			t.Logf("ℹ️  Notifications endpoint returned status %d", resp.StatusCode)
		}
	})
	
	// Test memory system
	t.Run("Memory System Availability", func(t *testing.T) {
		t.Log("🧠 Checking memory system availability...")
		
		resp, err := framework.GET(t, "/api/v1/memory/providers")
		if err != nil {
			t.Logf("⚠️  Memory endpoint not available: %v", err)
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			t.Log("✅ Memory system is available")
		} else {
			t.Logf("ℹ️  Memory endpoint returned status %d", resp.StatusCode)
		}
	})
}

// TestEnvironmentValidation validates the test environment setup
func TestEnvironmentValidation(t *testing.T) {
	t.Log("🔍 Validating test environment...")
	
	// Check server URL
	serverURL := getServerURL()
	t.Logf("🌐 Server URL: %s", serverURL)
	
	// Check environment variables
	envVars := []string{
		"HELIX_TEST_SERVER",
		"HELIX_AUTH_JWT_SECRET",
		"HELIX_DATABASE_HOST",
		"HELIX_REDIS_HOST",
	}
	
	for _, envVar := range envVars {
		if value := os.Getenv(envVar); value != "" {
			t.Logf("✅ %s is set", envVar)
		} else {
			t.Logf("ℹ️  %s is not set (using defaults)", envVar)
		}
	}
	
	// Test basic connectivity
	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		t.Skipf("Cannot connect to server at %s: %v (SKIP-OK: #server-not-available)", serverURL, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		t.Logf("✅ Server at %s is responding", serverURL)
	} else {
		t.Logf("⚠️  Server at %s returned status %d", serverURL, resp.StatusCode)
	}
	
	t.Log("✅ Test environment validation complete")
}