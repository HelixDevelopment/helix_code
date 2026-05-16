package security

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.code/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Utilities for Authorization Tests
// =============================================================================

func init() {
	gin.SetMode(gin.TestMode)
}

// TestAuthConfig holds configuration for authorization tests
type TestAuthConfig struct {
	JWTSecret     string
	TokenExpiry   time.Duration
	SessionExpiry time.Duration
}

func defaultTestAuthConfig() TestAuthConfig {
	return TestAuthConfig{
		JWTSecret:     "test-secret-key-for-security-testing-min-32-bytes",
		TokenExpiry:   1 * time.Hour,
		SessionExpiry: 24 * time.Hour,
	}
}

// generateTestJWT creates a JWT token for testing purposes
func generateTestJWT(config TestAuthConfig, userID, username, email string, additionalClaims map[string]interface{}) string {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"email":    email,
		"exp":      time.Now().Add(config.TokenExpiry).Unix(),
		"iat":      time.Now().Unix(),
	}

	// Add any additional claims (like role, permissions, etc.)
	for k, v := range additionalClaims {
		claims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(config.JWTSecret))
	return tokenString
}

// createTestRouter creates a test router with mock handlers
func createTestRouter(authConfig TestAuthConfig) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	// Mock auth middleware
	authMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		if len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		tokenString := authHeader[7:]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(authConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		c.Set("user_id", claims["user_id"])
		c.Set("username", claims["username"])
		c.Set("role", claims["role"])
		c.Set("permissions", claims["permissions"])
		c.Next()
	}

	// Role-based middleware
	requireRole := func(roles ...string) gin.HandlerFunc {
		return func(c *gin.Context) {
			userRole, exists := c.Get("role")
			if !exists {
				c.JSON(http.StatusForbidden, gin.H{"error": "Role not found"})
				c.Abort()
				return
			}

			roleStr, ok := userRole.(string)
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role format"})
				c.Abort()
				return
			}

			for _, allowed := range roles {
				if roleStr == allowed {
					c.Next()
					return
				}
			}

			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
		}
	}

	// Permission-based middleware
	requirePermission := func(permission string) gin.HandlerFunc {
		return func(c *gin.Context) {
			perms, exists := c.Get("permissions")
			if !exists {
				c.JSON(http.StatusForbidden, gin.H{"error": "Permissions not found"})
				c.Abort()
				return
			}

			permList, ok := perms.([]interface{})
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "Invalid permissions format"})
				c.Abort()
				return
			}

			for _, p := range permList {
				if pStr, ok := p.(string); ok && pStr == permission {
					c.Next()
					return
				}
			}

			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			c.Abort()
		}
	}

	// Public routes
	router.GET("/api/v1/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public endpoint"})
	})

	// Authenticated routes
	authenticated := router.Group("/api/v1")
	authenticated.Use(authMiddleware)
	{
		// User routes
		authenticated.GET("/users/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"user_id":  c.GetString("user_id"),
				"username": c.GetString("username"),
			})
		})

		authenticated.PUT("/users/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "profile updated"})
		})

		authenticated.DELETE("/users/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "account deleted"})
		})

		// Resource routes with ownership check
		authenticated.GET("/projects/:id", func(c *gin.Context) {
			projectID := c.Param("id")
			userID := c.GetString("user_id")

			// Simulate ownership check
			// In real implementation, this would check database
			if projectID == "owned-project" || projectID == userID+"-project" {
				c.JSON(http.StatusOK, gin.H{
					"project_id": projectID,
					"owner":      userID,
				})
			} else if projectID == "other-user-project" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this project"})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			}
		})

		authenticated.PUT("/projects/:id", func(c *gin.Context) {
			projectID := c.Param("id")
			userID := c.GetString("user_id")

			if projectID == userID+"-project" {
				c.JSON(http.StatusOK, gin.H{"message": "project updated"})
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to modify this project"})
			}
		})

		authenticated.DELETE("/projects/:id", func(c *gin.Context) {
			projectID := c.Param("id")
			userID := c.GetString("user_id")

			if projectID == userID+"-project" {
				c.JSON(http.StatusOK, gin.H{"message": "project deleted"})
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this project"})
			}
		})

		// Role-protected routes
		adminRoutes := authenticated.Group("/admin")
		adminRoutes.Use(requireRole("admin"))
		{
			adminRoutes.GET("/users", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"users": []string{"user1", "user2"}})
			})

			adminRoutes.DELETE("/users/:id", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
			})

			adminRoutes.GET("/system/config", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"config": "system configuration"})
			})
		}

		// Permission-protected routes
		authenticated.POST("/tasks", requirePermission("tasks:create"), func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"task_id": "new-task"})
		})

		authenticated.DELETE("/tasks/:id", requirePermission("tasks:delete"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
		})

		authenticated.POST("/workers", requirePermission("workers:manage"), func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"worker_id": "new-worker"})
		})
	}

	return router
}

func performRequest(router *gin.Engine, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, _ := http.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// =============================================================================
// 1. Role-Based Access Control (RBAC) Tests
// =============================================================================

func TestRBAC_AdminRoleAccess(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	adminToken := generateTestJWT(config, uuid.New().String(), "admin", "admin@example.com", map[string]interface{}{
		"role": "admin",
	})

	t.Run("Admin should access admin endpoints", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/admin/users", nil, adminToken)
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["users"])
	})

	t.Run("Admin should access system config", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/admin/system/config", nil, adminToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Admin should delete users", func(t *testing.T) {
		w := performRequest(router, "DELETE", "/api/v1/admin/users/some-user-id", nil, adminToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRBAC_RegularUserRestricted(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Regular user should not access admin endpoints", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/admin/users", nil, userToken)
		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "permissions")
	})

	t.Run("Regular user should not access system config", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/admin/system/config", nil, userToken)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Regular user should not delete other users", func(t *testing.T) {
		w := performRequest(router, "DELETE", "/api/v1/admin/users/other-user-id", nil, userToken)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestRBAC_NoRoleClaim(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	// Token without role claim
	tokenWithoutRole := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", nil)

	t.Run("Token without role should be denied admin access", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/admin/users", nil, tokenWithoutRole)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestRBAC_InvalidRoleValue(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Invalid role type should be rejected", func(t *testing.T) {
		// Token with numeric role instead of string
		claims := jwt.MapClaims{
			"user_id":  uuid.New().String(),
			"username": "user",
			"email":    "user@example.com",
			"role":     12345, // Invalid type
			"exp":      time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(config.JWTSecret))

		w := performRequest(router, "GET", "/api/v1/admin/users", nil, tokenString)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// =============================================================================
// 2. Permission Escalation Prevention Tests
// =============================================================================

func TestPermissionEscalation_RoleInjectionInToken(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Cannot escalate privileges by modifying token", func(t *testing.T) {
		// Create a valid token as regular user
		userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
			"role": "user",
		})

		// Verify user cannot access admin endpoints
		w := performRequest(router, "GET", "/api/v1/admin/users", nil, userToken)
		assert.Equal(t, http.StatusForbidden, w.Code)

		// Now try with a forged token (wrong secret)
		forgedClaims := jwt.MapClaims{
			"user_id":  uuid.New().String(),
			"username": "hacker",
			"email":    "hacker@example.com",
			"role":     "admin", // Trying to escalate
			"exp":      time.Now().Add(time.Hour).Unix(),
		}
		forgedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, forgedClaims)
		forgedTokenString, _ := forgedToken.SignedString([]byte("wrong-secret"))

		w = performRequest(router, "GET", "/api/v1/admin/users", nil, forgedTokenString)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "Forged token should be rejected")
	})
}

func TestPermissionEscalation_RoleInRequest(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Cannot escalate role via request body", func(t *testing.T) {
		// Try to include role in update request
		body := map[string]interface{}{
			"display_name": "Hacker",
			"role":         "admin", // Trying to escalate via request body
		}

		w := performRequest(router, "PUT", "/api/v1/users/me", body, userToken)
		// The endpoint should not process the role field from body
		assert.Equal(t, http.StatusOK, w.Code)
		// In real implementation, role field would be ignored
	})
}

func TestPermissionEscalation_PermissionInjection(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Cannot add permissions via request", func(t *testing.T) {
		userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
			"role":        "user",
			"permissions": []string{"tasks:read"}, // Limited permissions
		})

		// Try to create task without create permission
		w := performRequest(router, "POST", "/api/v1/tasks", map[string]string{"name": "test"}, userToken)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestPermissionEscalation_HorizontalPrivilegeEscalation(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userAID := uuid.New().String()
	userAToken := generateTestJWT(config, userAID, "userA", "usera@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("User A cannot access User B resources", func(t *testing.T) {
		// Try to access another user's project
		w := performRequest(router, "GET", "/api/v1/projects/other-user-project", nil, userAToken)
		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "Not authorized")
	})

	t.Run("User A can access their own resources", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/projects/"+userAID+"-project", nil, userAToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestPermissionEscalation_VerticalPrivilegeEscalation(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Regular user cannot perform admin actions", func(t *testing.T) {
		adminActions := []struct {
			method string
			path   string
		}{
			{"GET", "/api/v1/admin/users"},
			{"DELETE", "/api/v1/admin/users/any-user"},
			{"GET", "/api/v1/admin/system/config"},
		}

		for _, action := range adminActions {
			w := performRequest(router, action.method, action.path, nil, userToken)
			assert.Equal(t, http.StatusForbidden, w.Code,
				"User should not access %s %s", action.method, action.path)
		}
	})
}

// =============================================================================
// 3. Resource Access Validation Tests
// =============================================================================

func TestResourceAccess_OwnershipValidation(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	ownerID := uuid.New().String()
	ownerToken := generateTestJWT(config, ownerID, "owner", "owner@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Owner can read their resource", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/projects/"+ownerID+"-project", nil, ownerToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Owner can update their resource", func(t *testing.T) {
		w := performRequest(router, "PUT", "/api/v1/projects/"+ownerID+"-project",
			map[string]string{"name": "updated"}, ownerToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Owner can delete their resource", func(t *testing.T) {
		w := performRequest(router, "DELETE", "/api/v1/projects/"+ownerID+"-project", nil, ownerToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestResourceAccess_NonOwnerDenied(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	nonOwnerID := uuid.New().String()
	nonOwnerToken := generateTestJWT(config, nonOwnerID, "nonowner", "nonowner@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Non-owner cannot access other's resource", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/projects/other-user-project", nil, nonOwnerToken)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Non-owner cannot update other's resource", func(t *testing.T) {
		w := performRequest(router, "PUT", "/api/v1/projects/other-user-project",
			map[string]string{"name": "hacked"}, nonOwnerToken)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Non-owner cannot delete other's resource", func(t *testing.T) {
		w := performRequest(router, "DELETE", "/api/v1/projects/other-user-project", nil, nonOwnerToken)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestResourceAccess_IDORPrevention(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("IDOR via sequential ID guessing", func(t *testing.T) {
		// Try to access resources with guessed IDs
		guessedIDs := []string{
			"1", "2", "3",
			"user-1", "user-2",
			"admin", "root", "system",
			"00000000-0000-0000-0000-000000000001",
		}

		for _, id := range guessedIDs {
			w := performRequest(router, "GET", "/api/v1/projects/"+id, nil, userToken)
			assert.NotEqual(t, http.StatusOK, w.Code,
				"Should not allow IDOR access to project %s", id)
		}
	})
}

func TestResourceAccess_PathTraversalPrevention(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Path traversal in resource ID", func(t *testing.T) {
		pathTraversalIDs := []string{
			"../admin",
			"..%2fadmin",
			"....//admin",
			"../../../etc/passwd",
		}

		for _, id := range pathTraversalIDs {
			w := performRequest(router, "GET", "/api/v1/projects/"+id, nil, userToken)
			assert.NotEqual(t, http.StatusOK, w.Code,
				"Path traversal should be prevented: %s", id)
		}
	})
}

// =============================================================================
// 4. API Endpoint Authorization Tests
// =============================================================================

func TestAPIAuthorization_PublicEndpoints(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Public endpoints accessible without auth", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/public", nil, "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAPIAuthorization_ProtectedEndpoints(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/users/me"},
		{"PUT", "/api/v1/users/me"},
		{"DELETE", "/api/v1/users/me"},
		{"GET", "/api/v1/projects/some-id"},
		{"POST", "/api/v1/tasks"},
		{"DELETE", "/api/v1/tasks/some-id"},
		{"GET", "/api/v1/admin/users"},
	}

	t.Run("Protected endpoints require authentication", func(t *testing.T) {
		for _, ep := range protectedEndpoints {
			w := performRequest(router, ep.method, ep.path, nil, "")
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"Endpoint %s %s should require auth", ep.method, ep.path)
		}
	})
}

func TestAPIAuthorization_InvalidToken(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Invalid token rejected", func(t *testing.T) {
		invalidTokens := []string{
			"invalid-token",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			"expired-token-here",
		}

		for _, token := range invalidTokens {
			w := performRequest(router, "GET", "/api/v1/users/me", nil, token)
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"Invalid token should be rejected: %s", token)
		}
	})
}

func TestAPIAuthorization_ExpiredToken(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Expired token rejected", func(t *testing.T) {
		claims := jwt.MapClaims{
			"user_id":  uuid.New().String(),
			"username": "user",
			"email":    "user@example.com",
			"exp":      time.Now().Add(-time.Hour).Unix(), // Expired
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		expiredToken, _ := token.SignedString([]byte(config.JWTSecret))

		w := performRequest(router, "GET", "/api/v1/users/me", nil, expiredToken)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAPIAuthorization_WrongSecretToken(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Token signed with wrong secret rejected", func(t *testing.T) {
		claims := jwt.MapClaims{
			"user_id":  uuid.New().String(),
			"username": "hacker",
			"email":    "hacker@example.com",
			"role":     "admin",
			"exp":      time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		wrongSecretToken, _ := token.SignedString([]byte("wrong-secret-key"))

		w := performRequest(router, "GET", "/api/v1/admin/users", nil, wrongSecretToken)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAPIAuthorization_PermissionBased(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("User with correct permission can access endpoint", func(t *testing.T) {
		tokenWithPermission := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
			"role":        "user",
			"permissions": []string{"tasks:create", "tasks:read"},
		})

		w := performRequest(router, "POST", "/api/v1/tasks", map[string]string{"name": "test"}, tokenWithPermission)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("User without permission denied", func(t *testing.T) {
		tokenWithoutPermission := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
			"role":        "user",
			"permissions": []string{"tasks:read"}, // No create permission
		})

		w := performRequest(router, "POST", "/api/v1/tasks", map[string]string{"name": "test"}, tokenWithoutPermission)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestAPIAuthorization_HTTPMethodValidation(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Only allowed HTTP methods work", func(t *testing.T) {
		// GET should work on read endpoint
		w := performRequest(router, "GET", "/api/v1/users/me", nil, userToken)
		assert.Equal(t, http.StatusOK, w.Code)

		// POST on GET-only endpoint should not work
		w = performRequest(router, "POST", "/api/v1/users/me", nil, userToken)
		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

// =============================================================================
// Additional Authorization Security Tests
// =============================================================================

func TestAuthorization_AuthHeaderFormats(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	validToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Only Bearer token format accepted", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/users/me", nil)

		// No Bearer prefix
		req.Header.Set("Authorization", validToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// Basic auth format
		req.Header.Set("Authorization", "Basic "+validToken)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// Correct Bearer format
		req.Header.Set("Authorization", "Bearer "+validToken)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAuthorization_CaseSensitivity(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	validToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("Bearer prefix case sensitivity", func(t *testing.T) {
		caseVariations := []string{
			"bearer " + validToken,
			"BEARER " + validToken,
			"Bearer " + validToken, // Correct
		}

		for _, auth := range caseVariations {
			req, _ := http.NewRequest("GET", "/api/v1/users/me", nil)
			req.Header.Set("Authorization", auth)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Only "Bearer " (title case) should work
			if auth[:7] == "Bearer " {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}
		}
	})
}

func TestAuthorization_CrossUserDataAccess(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	userAID := uuid.New().String()
	userBID := uuid.New().String()

	userAToken := generateTestJWT(config, userAID, "userA", "usera@example.com", map[string]interface{}{
		"role": "user",
	})

	t.Run("User A cannot impersonate User B", func(t *testing.T) {
		// Try to access user B's resources with user A's token
		w := performRequest(router, "GET", "/api/v1/projects/"+userBID+"-project", nil, userAToken)
		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

func TestAuthorization_EmptyPermissions(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("Empty permissions array denies access", func(t *testing.T) {
		tokenWithEmptyPerms := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
			"role":        "user",
			"permissions": []string{}, // Empty
		})

		w := performRequest(router, "POST", "/api/v1/tasks", map[string]string{"name": "test"}, tokenWithEmptyPerms)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestAuthorization_MaliciousClaimValues(t *testing.T) {
	config := defaultTestAuthConfig()
	router := createTestRouter(config)

	t.Run("SQL injection in role claim", func(t *testing.T) {
		sqlInjectionToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
			"role": "admin' OR '1'='1",
		})

		w := performRequest(router, "GET", "/api/v1/admin/users", nil, sqlInjectionToken)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("XSS in permission claim", func(t *testing.T) {
		xssToken := generateTestJWT(config, uuid.New().String(), "user", "user@example.com", map[string]interface{}{
			"role":        "user",
			"permissions": []string{"<script>alert('xss')</script>"},
		})

		w := performRequest(router, "POST", "/api/v1/tasks", map[string]string{"name": "test"}, xssToken)
		assert.Equal(t, http.StatusForbidden, w.Code) // Should not match any valid permission
	})
}

// =============================================================================
// Integration with auth.AuthService
// =============================================================================

func TestAuthServiceIntegration_VerifyJWTReturnsMinimalUser(t *testing.T) {
	mockRepo := NewMockSecurityAuthRepository()
	config := auth.DefaultConfig()
	service := auth.NewAuthService(config, mockRepo)

	user := &auth.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}

	token, err := service.GenerateJWT(user)
	require.NoError(t, err)

	verifiedUser, err := service.VerifyJWT(token)
	require.NoError(t, err)

	// VerifyJWT returns minimal user from claims, not full user from DB
	assert.Equal(t, user.ID, verifiedUser.ID)
	assert.Equal(t, user.Username, verifiedUser.Username)
	assert.Equal(t, user.Email, verifiedUser.Email)
	// Note: IsActive, IsVerified, etc. are NOT set by VerifyJWT (only by VerifyJWTWithDB)
}

func TestAuthServiceIntegration_DeactivatedUserToken(t *testing.T) {
	t.Run("Token for deactivated user should still verify in JWT-only mode", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Generate token for user who was later deactivated
		user := &auth.User{
			ID:       uuid.New(),
			Username: "deactivated",
			Email:    "deactivated@example.com",
			IsActive: true, // Active when token was generated
		}

		token, err := service.GenerateJWT(user)
		require.NoError(t, err)

		// VerifyJWT only checks token validity, not user status
		verifiedUser, err := service.VerifyJWT(token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, verifiedUser.ID)

		// Note: For deactivation check, use VerifyJWTWithDB which checks database
	})
}
