package core

import (
	"net/http"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST-E2E-001: User Registration Flow
func TestUserRegistration(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Test data
	registrationData := map[string]interface{}{
		"username": "testuser",
		"email":    "test@example.com",
		"password": "SecurePassword123!",
	}

	// Step 1: Register new user
	resp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert successful registration
	e2e.AssertStatus(t, resp, http.StatusCreated)

	// Parse response
	var registrationResponse map[string]interface{}
	e2e.ParseJSON(t, resp, &registrationResponse)

	// Verify response contains expected fields
	assert.Contains(t, registrationResponse, "user_id")
	assert.Contains(t, registrationResponse, "message")
	assert.Equal(t, "User registered successfully", registrationResponse["message"])

	// Step 2: Verify user can login with new credentials
	loginData := map[string]interface{}{
		"username": "testuser",
		"password": "SecurePassword123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()

	// Assert successful login
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	// Parse login response
	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	// Verify login response contains token
	assert.Contains(t, loginResponse, "token")
	assert.Contains(t, loginResponse, "expires_in")

	// Step 3: Test token-based access
	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Test authenticated endpoint
	profileResp, err := framework.GET(t, "/api/v1/auth/profile")
	require.NoError(t, err)
	defer profileResp.Body.Close()

	e2e.AssertStatus(t, profileResp, http.StatusOK)

	// Verify profile data
	var profileResponse map[string]interface{}
	e2e.ParseJSON(t, profileResp, &profileResponse)

	assert.Equal(t, "testuser", profileResponse["username"])
	assert.Equal(t, "test@example.com", profileResponse["email"])
}