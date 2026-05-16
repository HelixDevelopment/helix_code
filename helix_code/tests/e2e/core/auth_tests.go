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

// TEST-E2E-002: User Login/Logout Flow  
func TestUserLoginLogout(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// First register a user for testing
	registrationData := map[string]interface{}{
		"username": "logintest",
		"email":    "login@test.com",
		"password": "TestPassword123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	// Step 1: Login with valid credentials
	loginData := map[string]interface{}{
		"username": "logintest",
		"password": "TestPassword123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()

	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	require.NotEmpty(t, token)

	// Step 2: Test token-based access
	framework.TestUser.Token = token

	// Access protected resource
	protectedResp, err := framework.GET(t, "/api/v1/auth/me")
	require.NoError(t, err)
	defer protectedResp.Body.Close()

	e2e.AssertStatus(t, protectedResp, http.StatusOK)

	// Step 3: Logout (invalidate token)
	logoutResp, err := framework.POST(t, "/api/v1/auth/logout", nil)
	require.NoError(t, err)
	defer logoutResp.Body.Close()

	e2e.AssertStatus(t, logoutResp, http.StatusOK)

	// Step 4: Verify token is invalidated
	invalidResp, err := framework.GET(t, "/api/v1/auth/me")
	require.NoError(t, err)
	defer invalidResp.Body.Close()

	// Should return unauthorized
	e2e.AssertStatus(t, invalidResp, http.StatusUnauthorized)
}

// TEST-E2E-003: Role-Based Access Control
func TestRoleBasedAccess(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Create users with different roles
	users := []struct {
		username string
		email    string
		password string
		role     string
	}{
		{"admin", "admin@test.com", "AdminPass123!", "admin"},
		{"user", "user@test.com", "UserPass123!", "user"},
		{"guest", "guest@test.com", "GuestPass123!", "guest"},
	}

	var tokens = make(map[string]string)

	// Register all users
	for _, user := range users {
		registrationData := map[string]interface{}{
			"username": user.username,
			"email":    user.email,
			"password": user.password,
			"role":     user.role,
		}

		regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
		require.NoError(t, err)
		defer regResp.Body.Close()
		e2e.AssertStatus(t, regResp, http.StatusCreated)

		// Login and get token
		loginData := map[string]interface{}{
			"username": user.username,
			"password": user.password,
		}

		loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
		require.NoError(t, err)
		defer loginResp.Body.Close()
		e2e.AssertStatus(t, loginResp, http.StatusOK)

		var loginResponse map[string]interface{}
		e2e.ParseJSON(t, loginResp, &loginResponse)

		tokens[user.role] = loginResponse["token"].(string)
	}

	// Test admin access
	framework.TestUser.Token = tokens["admin"]
	adminResp, err := framework.GET(t, "/api/v1/admin/users")
	require.NoError(t, err)
	defer adminResp.Body.Close()
	e2e.AssertStatus(t, adminResp, http.StatusOK)

	// Test user access (should be forbidden for admin endpoints)
	framework.TestUser.Token = tokens["user"]
	userAdminResp, err := framework.GET(t, "/api/v1/admin/users")
	require.NoError(t, err)
	defer userAdminResp.Body.Close()
	e2e.AssertStatus(t, userAdminResp, http.StatusForbidden)

	// Test guest access (should be forbidden for most endpoints)
	framework.TestUser.Token = tokens["guest"]
	guestProjectResp, err := framework.POST(t, "/api/v1/projects", map[string]interface{}{
		"name": "Guest Project",
	})
	require.NoError(t, err)
	defer guestProjectResp.Body.Close()
	e2e.AssertStatus(t, guestProjectResp, http.StatusForbidden)
}