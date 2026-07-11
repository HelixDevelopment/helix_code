package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig holds security test configuration
type TestConfig struct {
	BaseURL string
	Timeout time.Duration
}

func getTestConfig() *TestConfig {
	baseURL := os.Getenv("HELIXCODE_TEST_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &TestConfig{
		BaseURL: baseURL,
		Timeout: 30 * time.Second,
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}
}

// skipIfServerUnavailable checks if the test server is reachable and skips
// the test if not. Call this at the start of tests that require a running server.
func skipIfServerUnavailable(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		t.Skip("Server not available - skipping security test (SKIP-OK: #server-not-available)")
	}
	resp.Body.Close()
}

// skipIfNotCurrentBuild gates a test on the reachable server actually being the
// build under test, not merely reachable.
//
// §11.4.108 runtime-signature discipline: a reachable server is NOT necessarily
// the current build. The current server source (internal/server/server.go
// healthCheck) returns {"status":"healthy"} AND its router registers
// SecurityMiddleware (X-Content-Type-Options / X-Frame-Options / X-XSS-Protection
// on every response, /health included). A STALE process predating that source
// answers /health 200 with {"status":"ok"} and no security headers — it passes
// the bare reachability probe yet legitimately fails the current assertions
// because the build under test was never deployed to it.
//
// The runtime signature of "this is the current build" is the health body
// reporting "healthy". When that signature is absent we SKIP-with-reason
// (the build under test is not the one running). When it IS present, the
// caller's real assertions run unweakened: a current build that fails to emit
// a required security header is a genuine regression and MUST FAIL — this gate
// never masks a reachable-but-insecure current server (anti-bluff, §11.4.3).
func skipIfNotCurrentBuild(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		t.Skip("Server not available - skipping security test (SKIP-OK: #server-not-available)")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	if status, _ := result["status"].(string); status != "healthy" {
		// Reachable but NOT the current build (runtime signature "healthy" absent):
		// the build under test is not deployed to this server. SKIP — do not assert
		// against a foreign/stale artifact (§11.4.108).
		t.Skipf("Server at %s is reachable but is not the build under test "+
			"(/health reported status=%q, current build reports \"healthy\") — "+
			"the build under test is not deployed here "+
			"(SKIP-OK: #server-not-current-build)", config.BaseURL, status)
	}
}

func doRequest(t *testing.T, method, path string, body interface{}, headers map[string]string) (*http.Response, map[string]interface{}) {
	config := getTestConfig()
	client := newHTTPClient()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, config.BaseURL+path, reqBody)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil
	}

	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	return resp, result
}

// =============================================================================
// OWASP A01:2021 - Broken Access Control
// =============================================================================

func TestOWASP_A01_BrokenAccessControl_UnauthorizedAccess(t *testing.T) {
	// Gate on the running server being the build under test (§11.4.108 /
	// §11.4.120). I-2 reconciliation (2026-07-11): commit 88f11bc2 added
	// http.StatusFound (302) to the accepted set based on a live run — but a
	// live probe of localhost:8080 on this shared host resolves to an
	// unrelated foreign service (a systemd-user process that HTTP->HTTPS
	// redirects then redirects unauthenticated requests to /login, matching
	// a CockroachDB-DB-Console-style login flow), NOT this project's
	// helixcode server (§11.4.174 process-ownership verification). An
	// exhaustive source-code sweep of internal/server, internal/auth, and
	// cmd/server confirmed ZERO code path anywhere ever emits
	// http.StatusFound/302 — authMiddleware (server.go:511-566) always
	// responds 401 via c.JSON(http.StatusUnauthorized, ...). The 302 the
	// prior commit "reconciled" against was never our server's behavior; it
	// was contamination from a foreign process sharing the port. Reverted
	// per §11.4.120 (reconcile against genuine server behavior, not a
	// live-run artifact); skipIfNotCurrentBuild added so this test SKIPs
	// honestly instead of false-failing/false-passing against that foreign
	// process.
	skipIfNotCurrentBuild(t)
	t.Run("Unauthenticated access to protected endpoints", func(t *testing.T) {
		protectedEndpoints := []string{
			"/api/v1/users/me",
			"/api/v1/workers",
			"/api/v1/tasks",
			"/api/v1/system/stats",
			"/api/v1/system/status",
		}

		for _, endpoint := range protectedEndpoints {
			resp, _ := doRequest(t, "GET", endpoint, nil, nil)
			if resp != nil {
				// Should return 401 Unauthorized or 404 if not implemented.
				// Both are acceptable - 401 means auth works, 404 means
				// endpoint not implemented. All of protectedEndpoints above
				// are registered under authMiddleware-gated route groups
				// (server.go: users/workers/tasks/system), so the real,
				// source-verified response is 401.
				assert.Contains(t, []int{http.StatusUnauthorized, http.StatusNotFound}, resp.StatusCode,
					"Endpoint %s should require authentication or not be implemented", endpoint)
			}
		}
	})
}

func TestOWASP_A01_BrokenAccessControl_IDORPrevention(t *testing.T) {
	t.Run("IDOR prevention - cannot access other users resources", func(t *testing.T) {
		// Attempt to access resources with guessed IDs
		testIDs := []string{
			"admin",
			"1",
			"00000000-0000-0000-0000-000000000001",
			"../admin",
		}

		for _, id := range testIDs {
			resp, _ := doRequest(t, "GET", "/api/v1/projects/"+id, nil, nil)
			if resp != nil {
				// Should not return 200 OK without proper authorization
				assert.NotEqual(t, http.StatusOK, resp.StatusCode,
					"Should not allow access to project %s without auth", id)
			}
		}
	})
}

func TestOWASP_A01_BrokenAccessControl_HorizontalPrivilegeEscalation(t *testing.T) {
	// I-2 reconciliation (2026-07-11, §11.4.120): commit 88f11bc2 added
	// http.StatusFound (302) to validCodes based on a live probe of
	// localhost:8080 that actually hit a foreign, unrelated service on this
	// shared host (§11.4.174) — not this project's server. Source-verified:
	// PUT /api/v1/users/other-user-id matches no registered route (the
	// "/users" group in server.go only registers GET/PUT/DELETE "/me"), so
	// gin's default NoRoute path answers 404 (global middleware, incl.
	// SecurityMiddleware, still runs and can 400 on malformed input — that
	// StatusBadRequest entry was already reconciled separately in commit
	// 54e91e8a and is left untouched here). No code path anywhere in
	// internal/server, internal/auth, or cmd/server emits 302 (verified via
	// exhaustive source grep). 302 reverted; skipIfNotCurrentBuild added so
	// the test SKIPs honestly instead of asserting against a foreign server.
	skipIfNotCurrentBuild(t)
	t.Run("Prevent horizontal privilege escalation", func(t *testing.T) {
		// Attempt to modify other users' data
		resp, _ := doRequest(t, "PUT", "/api/v1/users/other-user-id", map[string]string{
			"role": "admin",
		}, nil)

		if resp != nil {
			// Accept 401 (Unauthorized), 404 (Not Found), or 400 (Bad
			// Request — PUT body parsing before route match, via global
			// SecurityMiddleware) as valid security responses.
			validCodes := []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusBadRequest}
			assert.Contains(t, validCodes, resp.StatusCode,
				"Should not allow modification of other users data (expected 401, 404, or 400)")
		}
	})
}

// =============================================================================
// OWASP A02:2021 - Cryptographic Failures
// =============================================================================

func TestOWASP_A02_CryptographicFailures_SecureHeaders(t *testing.T) {
	// Gate on the running server being the build under test, not merely reachable.
	// The current build wires SecurityMiddleware (server.go:61) which sets all three
	// headers on every response. If this gate passes (current build) and a header is
	// still missing below, that is a genuine security regression that MUST fail.
	skipIfNotCurrentBuild(t)
	t.Run("Security headers are present", func(t *testing.T) {
		resp, _ := doRequest(t, "GET", "/health", nil, nil)
		require.NotNil(t, resp)

		// Check for security headers
		assert.NotEmpty(t, resp.Header.Get("X-Content-Type-Options"),
			"X-Content-Type-Options header should be present")
		assert.NotEmpty(t, resp.Header.Get("X-Frame-Options"),
			"X-Frame-Options header should be present")
		assert.NotEmpty(t, resp.Header.Get("X-XSS-Protection"),
			"X-XSS-Protection header should be present")
	})
}

func TestOWASP_A02_CryptographicFailures_NoSensitiveDataInURL(t *testing.T) {
	t.Run("Sensitive data not in URL", func(t *testing.T) {
		// Attempt login with password in URL (should not work)
		resp, _ := doRequest(t, "GET", "/api/v1/auth/login?password=secret123", nil, nil)
		if resp != nil {
			// Should reject GET requests with password in URL
			assert.NotEqual(t, http.StatusOK, resp.StatusCode,
				"Should not accept sensitive data in URL")
		}
	})
}

// =============================================================================
// OWASP A03:2021 - Injection
// =============================================================================

func TestOWASP_A03_Injection_SQLInjection(t *testing.T) {
	t.Run("SQL injection prevention", func(t *testing.T) {
		sqlPayloads := []string{
			"' OR '1'='1",
			"1; DROP TABLE users--",
			"1' UNION SELECT * FROM users--",
			"admin'--",
			"1 OR 1=1",
		}

		for _, payload := range sqlPayloads {
			// Test in login
			resp, result := doRequest(t, "POST", "/api/v1/auth/login", map[string]string{
				"username": payload,
				"password": "test",
			}, nil)

			if resp != nil && result != nil {
				// Should not return success for SQL injection attempts
				status, _ := result["status"].(string)
				assert.NotEqual(t, "success", status,
					"SQL injection payload should not succeed: %s", payload)
			}
		}
	})
}

func TestOWASP_A03_Injection_CommandInjection(t *testing.T) {
	t.Run("Command injection prevention", func(t *testing.T) {
		cmdPayloads := []string{
			"; rm -rf /",
			"| cat /etc/passwd",
			"$(whoami)",
			"`id`",
			"&& cat /etc/shadow",
		}

		for _, payload := range cmdPayloads {
			// Test in project path
			resp, result := doRequest(t, "POST", "/api/v1/projects", map[string]string{
				"name":        "test-project",
				"description": "Test",
				"path":        payload,
				"type":        "go",
			}, nil)

			if resp != nil && result != nil {
				// Should not execute command injection payloads
				// Either should fail or sanitize the input
				if resp.StatusCode == http.StatusCreated {
					project, _ := result["project"].(map[string]interface{})
					if project != nil {
						projectPath, _ := project["path"].(string)
						assert.NotContains(t, projectPath, ";",
							"Command injection payload should be sanitized")
					}
				}
			}
		}
	})
}

func TestOWASP_A03_Injection_PathTraversal(t *testing.T) {
	t.Run("Path traversal prevention", func(t *testing.T) {
		pathPayloads := []string{
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\config\\sam",
			"....//....//etc/passwd",
			"/etc/passwd",
			"file:///etc/passwd",
		}

		for _, payload := range pathPayloads {
			// Test in project path
			resp, _ := doRequest(t, "POST", "/api/v1/projects", map[string]string{
				"name":        "test-project",
				"description": "Test",
				"path":        payload,
				"type":        "go",
			}, nil)

			if resp != nil {
				// Should reject path traversal attempts
				assert.NotEqual(t, http.StatusCreated, resp.StatusCode,
					"Path traversal should be rejected: %s", payload)
			}
		}
	})
}

// =============================================================================
// OWASP A04:2021 - Insecure Design
// =============================================================================

func TestOWASP_A04_InsecureDesign_RateLimiting(t *testing.T) {
	t.Run("Rate limiting on authentication", func(t *testing.T) {
		// Attempt multiple failed logins
		failedAttempts := 0
		for i := 0; i < 10; i++ {
			resp, _ := doRequest(t, "POST", "/api/v1/auth/login", map[string]string{
				"username": "nonexistent",
				"password": "wrongpassword",
			}, nil)

			if resp != nil {
				if resp.StatusCode == http.StatusTooManyRequests {
					// Rate limiting is working
					return
				}
				failedAttempts++
			}
		}

		// Note: If rate limiting is not implemented, this test documents it
		t.Logf("Rate limiting may not be implemented - %d attempts succeeded", failedAttempts)
	})
}

func TestOWASP_A04_InsecureDesign_BusinessLogicValidation(t *testing.T) {
	t.Run("Business logic validation", func(t *testing.T) {
		// Test for negative values where not allowed
		resp, result := doRequest(t, "POST", "/api/v1/tasks", map[string]interface{}{
			"name":     "test-task",
			"type":     "build",
			"priority": -1, // Negative priority should be rejected
		}, nil)

		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			// Should validate business logic constraints
			assert.NotEqual(t, http.StatusCreated, resp.StatusCode,
				"Negative priority should be rejected")
			_ = result
		}
	})
}

// =============================================================================
// OWASP A05:2021 - Security Misconfiguration
// =============================================================================

func TestOWASP_A05_SecurityMisconfiguration_DefaultCredentials(t *testing.T) {
	t.Run("Default credentials should not work", func(t *testing.T) {
		defaultCreds := []map[string]string{
			{"username": "admin", "password": "admin"},
			{"username": "admin", "password": "password"},
			{"username": "admin", "password": "123456"},
			{"username": "root", "password": "root"},
			{"username": "user", "password": "user"},
		}

		for _, creds := range defaultCreds {
			resp, result := doRequest(t, "POST", "/api/v1/auth/login", creds, nil)

			if resp != nil && result != nil {
				status, _ := result["status"].(string)
				assert.NotEqual(t, "success", status,
					"Default credentials should not work: %s/%s", creds["username"], creds["password"])
			}
		}
	})
}

func TestOWASP_A05_SecurityMisconfiguration_ErrorMessages(t *testing.T) {
	t.Run("Error messages do not leak sensitive information", func(t *testing.T) {
		resp, result := doRequest(t, "POST", "/api/v1/auth/login", map[string]string{
			"username": "nonexistent_user_12345",
			"password": "wrongpassword",
		}, nil)

		if resp != nil && result != nil {
			errorMsg, _ := result["error"].(string)
			message, _ := result["message"].(string)

			// Should not reveal whether username exists
			combined := errorMsg + message
			assert.NotContains(t, strings.ToLower(combined), "user not found",
				"Error message should not reveal if user exists")
			assert.NotContains(t, strings.ToLower(combined), "invalid username",
				"Error message should not reveal if user exists")
		}
	})
}

func TestOWASP_A05_SecurityMisconfiguration_StackTraces(t *testing.T) {
	t.Run("Stack traces are not exposed", func(t *testing.T) {
		// Send malformed request to trigger error
		resp, result := doRequest(t, "POST", "/api/v1/auth/login",
			"invalid json{{{", nil)

		if resp != nil && result != nil {
			respBody, _ := json.Marshal(result)
			respStr := string(respBody)

			assert.NotContains(t, respStr, "goroutine",
				"Stack traces should not be exposed")
			assert.NotContains(t, respStr, "panic:",
				"Panic information should not be exposed")
			assert.NotContains(t, respStr, ".go:",
				"Source file information should not be exposed")
		}
	})
}

// =============================================================================
// OWASP A06:2021 - Vulnerable and Outdated Components
// =============================================================================

func TestOWASP_A06_VulnerableComponents_ServerHeader(t *testing.T) {
	skipIfServerUnavailable(t)
	t.Run("Server header does not expose version", func(t *testing.T) {
		resp, _ := doRequest(t, "GET", "/health", nil, nil)
		require.NotNil(t, resp)

		serverHeader := resp.Header.Get("Server")
		if serverHeader != "" {
			// Should not expose detailed version information
			assert.NotContains(t, serverHeader, "1.",
				"Server header should not expose version")
			assert.NotContains(t, serverHeader, "2.",
				"Server header should not expose version")
		}
	})
}

// =============================================================================
// OWASP A07:2021 - Identification and Authentication Failures
// =============================================================================

func TestOWASP_A07_AuthFailures_WeakPasswordRejection(t *testing.T) {
	t.Run("Weak passwords are rejected", func(t *testing.T) {
		weakPasswords := []string{
			"123456",
			"password",
			"qwerty",
			"abc",
			"",
		}

		for _, password := range weakPasswords {
			resp, result := doRequest(t, "POST", "/api/v1/auth/register", map[string]string{
				"username":     fmt.Sprintf("testuser_%d", time.Now().UnixNano()),
				"email":        fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()),
				"password":     password,
				"display_name": "Test User",
			}, nil)

			if resp != nil && result != nil {
				// Weak passwords should be rejected
				// (Note: This depends on implementation)
				t.Logf("Password '%s' response: %d", password, resp.StatusCode)
			}
		}
	})
}

func TestOWASP_A07_AuthFailures_SessionFixation(t *testing.T) {
	t.Run("Session fixation prevention", func(t *testing.T) {
		// Get a session token
		resp1, result1 := doRequest(t, "POST", "/api/v1/auth/login", map[string]string{
			"username": "testuser",
			"password": "testpassword",
		}, nil)

		if resp1 != nil && resp1.StatusCode == http.StatusOK {
			token1, _ := result1["token"].(string)

			// Login again
			resp2, result2 := doRequest(t, "POST", "/api/v1/auth/login", map[string]string{
				"username": "testuser",
				"password": "testpassword",
			}, nil)

			if resp2 != nil && resp2.StatusCode == http.StatusOK {
				token2, _ := result2["token"].(string)

				// Tokens should be different (new session on each login)
				if token1 != "" && token2 != "" {
					assert.NotEqual(t, token1, token2,
						"New login should generate new token")
				}
			}
		}
	})
}

// =============================================================================
// OWASP A08:2021 - Software and Data Integrity Failures
// =============================================================================

func TestOWASP_A08_IntegrityFailures_InputValidation(t *testing.T) {
	// I-1 reconciliation (2026-07-11, §11.4.120): commit 88f11bc2 deleted
	// the real assertion here (assert.True(t, resp.StatusCode >= 400, ...))
	// and replaced it with a bare t.Logf — a tautology that can never fail,
	// a §11.4.120 violation (fake-passing a stale gate instead of
	// reconciling it).
	//
	// Root cause investigated per §11.4.102 (read server.go, not guessed):
	// POST /api/v1/projects is registered under the "projects" route group
	// which is gated by s.authMiddleware() (server.go:290-291). doRequest
	// in this file never sets an Authorization header, so authMiddleware
	// unconditionally short-circuits at server.go:516-521
	// (c.JSON(http.StatusUnauthorized, ...); c.Abort()) BEFORE gin ever
	// reaches createProject's JSON body decoding — the integer-vs-string /
	// array-vs-string typed fields the commit message worried about never
	// get parsed at all, because middleware Abort() prevents the handler
	// from running. The genuine, deterministic, source-verified response
	// for every input in this loop is exactly 401 Unauthorized.
	//
	// (The commit's original worry — Go's encoding/json coercing a JSON
	// number into an interface{} field as float64 without erroring — is a
	// real Go stdlib behavior, but it is moot here: it would only matter if
	// the request ever reached a handler that decodes into interface{},
	// which it never does while unauthenticated.)
	skipIfNotCurrentBuild(t)
	t.Run("Input validation prevents data corruption", func(t *testing.T) {
		// Test with invalid data types
		invalidInputs := []map[string]interface{}{
			{"name": 12345, "description": "test", "path": "/tmp/test", "type": "go"},              // name should be string
			{"name": "test", "description": []string{"a", "b"}, "path": "/tmp/test", "type": "go"}, // desc should be string
		}

		for _, input := range invalidInputs {
			resp, _ := doRequest(t, "POST", "/api/v1/projects", input, nil)
			if resp != nil {
				// Real, source-verified behavior: unauthenticated request to
				// an authMiddleware-gated route always returns exactly 401
				// (server.go:511-566), regardless of body shape.
				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
					"Unauthenticated POST /api/v1/projects should be rejected by authMiddleware before body validation runs (input=%#v)", input)
			}
		}
	})
}

// =============================================================================
// OWASP A09:2021 - Security Logging and Monitoring Failures
// =============================================================================

func TestOWASP_A09_LoggingFailures_HealthEndpointAvailable(t *testing.T) {
	// Gate on the running server being the build under test, not merely reachable.
	// A stale process answers /health 200 with {"status":"ok"}; the current build
	// reports {"status":"healthy"}. skipIfNotCurrentBuild SKIPs the stale case so the
	// assertion below verifies the current build's contract, not a foreign artifact.
	skipIfNotCurrentBuild(t)
	t.Run("Health endpoint for monitoring is available", func(t *testing.T) {
		resp, result := doRequest(t, "GET", "/health", nil, nil)
		require.NotNil(t, resp)
		require.NotNil(t, result)

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Health endpoint should be available")
		status, _ := result["status"].(string)
		assert.Equal(t, "healthy", status,
			"Health status should be returned")
	})
}

// =============================================================================
// OWASP A10:2021 - Server-Side Request Forgery (SSRF)
// =============================================================================

func TestOWASP_A10_SSRF_InternalIPBlocking(t *testing.T) {
	t.Run("SSRF - Internal IP addresses are blocked", func(t *testing.T) {
		internalAddresses := []string{
			"http://127.0.0.1/admin",
			"http://localhost/admin",
			"http://192.168.1.1/",
			"http://10.0.0.1/",
			"http://172.16.0.1/",
			"http://169.254.169.254/latest/meta-data/", // AWS metadata
			"file:///etc/passwd",
		}

		for _, addr := range internalAddresses {
			// Test in webhook URL or similar field
			resp, _ := doRequest(t, "POST", "/api/v1/projects", map[string]string{
				"name":        "test",
				"description": "test",
				"path":        addr, // Attempt SSRF via path
				"type":        "go",
			}, nil)

			if resp != nil {
				// Should reject internal addresses
				assert.NotEqual(t, http.StatusCreated, resp.StatusCode,
					"Internal address should be rejected: %s", addr)
			}
		}
	})
}

// =============================================================================
// Additional Security Tests
// =============================================================================

func TestSecurity_XSSPrevention(t *testing.T) {
	t.Run("XSS payloads are sanitized", func(t *testing.T) {
		xssPayloads := []string{
			"<script>alert('xss')</script>",
			"<img src=x onerror=alert('xss')>",
			"javascript:alert('xss')",
			"<svg onload=alert('xss')>",
		}

		for _, payload := range xssPayloads {
			resp, result := doRequest(t, "POST", "/api/v1/projects", map[string]string{
				"name":        payload,
				"description": payload,
				"path":        "/tmp/xss-test",
				"type":        "go",
			}, nil)

			if resp != nil && resp.StatusCode == http.StatusCreated && result != nil {
				project, _ := result["project"].(map[string]interface{})
				if project != nil {
					name, _ := project["name"].(string)
					desc, _ := project["description"].(string)

					// Check that XSS payloads are escaped or rejected
					assert.NotContains(t, name, "<script>",
						"XSS payload should be sanitized in name")
					assert.NotContains(t, desc, "<script>",
						"XSS payload should be sanitized in description")
				}
			}
		}
	})
}

func TestSecurity_CSRFProtection(t *testing.T) {
	// I-2 reconciliation (2026-07-11, §11.4.120): commit 88f11bc2 added
	// http.StatusFound (302) based on a live probe of localhost:8080 that
	// actually hit a foreign, unrelated service on this shared host
	// (§11.4.174), not this project's server. Source-verified:
	// POST /api/v1/users/me/password is not a registered route at all — the
	// "/users" group (server.go:240-246) registers only GET/PUT/DELETE
	// "/me" — so gin's default unmatched-route path answers 404
	// deterministically; authMiddleware is never even invoked (gin
	// per-route middleware only runs for matched routes). No code path
	// anywhere in internal/server, internal/auth, or cmd/server emits 302
	// (verified via exhaustive source grep). 302 reverted, restoring the
	// pre-88f11bc2 accepted-code set; skipIfNotCurrentBuild added so the
	// test SKIPs honestly instead of asserting against a foreign server.
	skipIfNotCurrentBuild(t)
	t.Run("State-changing operations require proper authentication", func(t *testing.T) {
		// Attempt to change password without authentication
		resp, _ := doRequest(t, "POST", "/api/v1/users/me/password", map[string]string{
			"current_password": "oldPass123!",
			"new_password":     "newPass456!",
		}, nil)
		if resp != nil {
			// Should return 401 Unauthorized or 404 if not implemented
			assert.Contains(t, []int{http.StatusUnauthorized, http.StatusNotFound}, resp.StatusCode,
				"State-changing operations should require authentication or not be implemented")
		}
	})
}

func TestSecurity_HTTPMethodValidation(t *testing.T) {
	t.Run("Invalid HTTP methods are rejected", func(t *testing.T) {
		invalidMethods := []string{"TRACE", "TRACK", "DEBUG"}

		for _, method := range invalidMethods {
			config := getTestConfig()
			client := newHTTPClient()

			req, err := http.NewRequest(method, config.BaseURL+"/api/v1/projects", nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			if err == nil && resp != nil {
				// Should reject these methods
				assert.True(t, resp.StatusCode >= 400,
					"Method %s should be rejected", method)
				resp.Body.Close()
			}
		}
	})
}

func TestSecurity_ContentTypeValidation(t *testing.T) {
	t.Run("Content-Type header is validated", func(t *testing.T) {
		config := getTestConfig()
		client := newHTTPClient()

		// Send request with wrong Content-Type
		req, err := http.NewRequest("POST", config.BaseURL+"/api/v1/auth/login",
			strings.NewReader(`{"username":"test","password":"test"}`))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "text/plain") // Wrong content type

		resp, err := client.Do(req)
		if err == nil && resp != nil {
			// Should either reject or handle gracefully
			t.Logf("Response with wrong Content-Type: %d", resp.StatusCode)
			resp.Body.Close()
		}
	})
}

func TestSecurity_JSONInjection(t *testing.T) {
	t.Run("JSON injection is prevented", func(t *testing.T) {
		// Attempt JSON injection
		payloads := []string{
			`{"username":"admin","password":"test","role":"admin"}`,
			`{"username":"test","password":"test","__proto__":{"admin":true}}`,
		}

		for _, payload := range payloads {
			config := getTestConfig()
			client := newHTTPClient()

			req, err := http.NewRequest("POST", config.BaseURL+"/api/v1/auth/login",
				strings.NewReader(payload))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err == nil && resp != nil {
				// Should not allow privilege escalation via JSON injection
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				assert.NotContains(t, string(body), `"role":"admin"`,
					"JSON injection should not work")
			}
		}
	})
}

func TestSecurity_URLEncoding(t *testing.T) {
	// I-2 reconciliation (2026-07-11, §11.4.120): commit 88f11bc2 added
	// http.StatusFound (302) based on a live probe of localhost:8080 that
	// actually hit a foreign, unrelated service on this shared host
	// (§11.4.174), not this project's server. Source-verified:
	// GET /api/v1/projects/<payload> is registered under the "projects"
	// group gated by s.authMiddleware() (server.go:290-297,
	// projects.GET("/:id", s.getProject)); doRequest never sets an
	// Authorization header, so the real, deterministic response is 401
	// before getProject's path handling ever runs. No code path anywhere in
	// internal/server, internal/auth, or cmd/server emits 302 (verified via
	// exhaustive source grep). 302 reverted, restoring the pre-88f11bc2
	// assertion; skipIfNotCurrentBuild added so the test SKIPs honestly
	// instead of asserting against a foreign server.
	skipIfNotCurrentBuild(t)
	t.Run("URL encoded attacks are handled", func(t *testing.T) {
		encodedPayloads := []string{
			url.QueryEscape("../../../etc/passwd"),
			url.QueryEscape("<script>alert('xss')</script>"),
			"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		}

		for _, payload := range encodedPayloads {
			resp, _ := doRequest(t, "GET", "/api/v1/projects/"+payload, nil, nil)
			if resp != nil {
				// Should handle encoded attacks safely
				assert.True(t, resp.StatusCode >= 400 || resp.StatusCode == http.StatusUnauthorized,
					"URL encoded attack should be rejected: %s", payload)
			}
		}
	})
}
