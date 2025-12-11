package phase2

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
)

// Phase2TestFramework extends the E2E framework for real server integration
type Phase2TestFramework struct {
	*e2e.E2ETestFramework
	ServerURL    string
	TestUsers    []TestUser
	TestProjects []TestProject
}

// TestUser represents a test user for Phase 2
type TestUser struct {
	Username string
	Email    string
	Password string
	Role     string
	Token    string
	ID       string
}

// TestProject represents a test project for Phase 2
type TestProject struct {
	ID          string
	Name        string
	Description string
	Type        string
	Owner       string
}

// NewPhase2Framework creates a new Phase 2 test framework
func NewPhase2Framework(t *testing.T) *Phase2TestFramework {
	serverURL := getServerURL()
	
	// Create base framework with real server URL
	baseFramework := &e2e.E2ETestFramework{
		Server:     nil, // Will be set to real server
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    serverURL,
		TestUser:   nil,
	}
	
	framework := &Phase2TestFramework{
		E2ETestFramework: baseFramework,
		ServerURL:        serverURL,
		TestUsers:        []TestUser{},
		TestProjects:     []TestProject{},
	}
	
	// Setup test environment
	framework.setupTestEnvironment(t)
	
	return framework
}

// getServerURL returns the server URL for Phase 2 testing
func getServerURL() string {
	if url := os.Getenv("HELIX_TEST_SERVER"); url != "" {
		return url
	}
	return "http://localhost:8080"
}

// setupTestEnvironment prepares the test environment
func (f *Phase2TestFramework) setupTestEnvironment(t *testing.T) {
	t.Log("🚀 Setting up Phase 2 test environment...")
	
	// Wait for server to be ready
	f.waitForServerReady(t, 60*time.Second)
	
	// Verify server health
	f.verifyServerHealth(t)
	
	// Setup test data
	f.setupTestData(t)
	
	t.Log("✅ Phase 2 test environment ready")
}

// waitForServerReady waits for the real server to be ready
func (f *Phase2TestFramework) waitForServerReady(t *testing.T, timeout time.Duration) {
	t.Logf("⏳ Waiting for server at %s to be ready...", f.ServerURL)
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	for {
		select {
		case <-ctx.Done():
			t.Fatal("❌ Timeout waiting for server to be ready")
		default:
			resp, err := f.HTTPClient.Get(f.ServerURL + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				t.Log("✅ Server is ready")
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// verifyServerHealth verifies the server is healthy
func (f *Phase2TestFramework) verifyServerHealth(t *testing.T) {
	t.Log("🔍 Verifying server health...")
	
	resp, err := f.GET(t, "/health")
	if err != nil {
		t.Fatalf("❌ Failed to check server health: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("❌ Server health check failed: status %d", resp.StatusCode)
	}
	
	var healthResponse map[string]interface{}
	e2e.ParseJSON(t, resp, &healthResponse)
	
	if status, ok := healthResponse["status"].(string); ok && status == "healthy" {
		t.Log("✅ Server is healthy")
	} else {
		t.Fatalf("❌ Server health check failed: %v", healthResponse)
	}
}

// setupTestData creates test data for Phase 2
func (f *Phase2TestFramework) setupTestData(t *testing.T) {
	t.Log("📊 Setting up test data...")
	
	// Create test users
	f.CreateTestUsers(t)
	
	// Create test projects
	f.CreateTestProjects(t)
	
	t.Log("✅ Test data setup complete")
}

// CreateTestUsers creates test users for Phase 2
func (f *Phase2TestFramework) CreateTestUsers(t *testing.T) {
	t.Log("👥 Creating test users...")
	
	testUsers := []TestUser{
		{
			Username: "phase2_user",
			Email:    "phase2@helixcode.com",
			Password: "Phase2Pass123!",
			Role:     "user",
		},
		{
			Username: "phase2_admin",
			Email:    "admin@helixcode.com",
			Password: "Admin2Pass123!",
			Role:     "admin",
		},
	}
	
	for i, user := range testUsers {
		t.Logf("📝 Creating user %d: %s", i+1, user.Username)
		
		registrationData := map[string]interface{}{
			"username": user.Username,
			"email":    user.Email,
			"password": user.Password,
			"role":     user.Role,
		}
		
		resp, err := f.POST(t, "/api/v1/auth/register", registrationData)
		if err != nil {
			t.Logf("⚠️  Failed to create user %s: %v (may already exist)", user.Username, err)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusCreated {
			t.Logf("✅ User %s created successfully", user.Username)
			
			// Login to get token
			loginData := map[string]interface{}{
				"username": user.Username,
				"password": user.Password,
			}
			
			loginResp, err := f.POST(t, "/api/v1/auth/login", loginData)
			if err != nil {
				t.Logf("⚠️  Failed to login user %s: %v", user.Username, err)
				continue
			}
			defer loginResp.Body.Close()
			
			if loginResp.StatusCode == http.StatusOK {
				var loginResponse map[string]interface{}
				e2e.ParseJSON(t, loginResp, &loginResponse)
				
				if token, ok := loginResponse["token"].(string); ok {
					user.Token = token
					t.Logf("✅ User %s logged in successfully", user.Username)
				}
			}
		} else if resp.StatusCode == http.StatusConflict {
			t.Logf("ℹ️  User %s already exists, attempting login", user.Username)
			
			// Try to login existing user
			loginData := map[string]interface{}{
				"username": user.Username,
				"password": user.Password,
			}
			
			loginResp, err := f.POST(t, "/api/v1/auth/login", loginData)
			if err != nil {
				t.Logf("⚠️  Failed to login existing user %s: %v", user.Username, err)
				continue
			}
			defer loginResp.Body.Close()
			
			if loginResp.StatusCode == http.StatusOK {
				var loginResponse map[string]interface{}
				e2e.ParseJSON(t, loginResp, &loginResponse)
				
				if token, ok := loginResponse["token"].(string); ok {
					user.Token = token
					t.Logf("✅ Existing user %s logged in successfully", user.Username)
				}
			}
		} else {
			t.Logf("⚠️  Unexpected response for user %s: status %d", user.Username, resp.StatusCode)
		}
		
		f.TestUsers = append(f.TestUsers, user)
	}
}

// CreateTestProjects creates test projects for Phase 2
func (f *Phase2TestFramework) CreateTestProjects(t *testing.T) {
	t.Log("🏗️ Creating test projects...")
	
	if len(f.TestUsers) == 0 {
		t.Log("⚠️  No test users available, skipping project creation")
		return
	}
	
	// Use the first test user for project creation
	user := f.TestUsers[0]
	if user.Token == "" {
		t.Log("⚠️  Test user not authenticated, skipping project creation")
		return
	}
	
	// Set authentication token
	f.TestUser = &e2e.TestUser{
		Token: user.Token,
	}
	
	testProjects := []TestProject{
		{
			Name:        "Phase 2 Test Project",
			Description: "Project for Phase 2 integration testing",
			Type:        "go",
			Owner:       user.Username,
		},
		{
			Name:        "Phase 2 Demo Project",
			Description: "Demonstration project for real server testing",
			Type:        "python",
			Owner:       user.Username,
		},
	}
	
	for i, project := range testProjects {
		t.Logf("🏗️ Creating project %d: %s", i+1, project.Name)
		
		projectData := map[string]interface{}{
			"name":        project.Name,
			"description": project.Description,
			"type":        project.Type,
		}
		
		resp, err := f.POST(t, "/api/v1/projects", projectData)
		if err != nil {
			t.Logf("⚠️  Failed to create project %s: %v", project.Name, err)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusCreated {
			var projectResponse map[string]interface{}
			e2e.ParseJSON(t, resp, &projectResponse)
			
			if projectID, ok := projectResponse["project_id"].(string); ok {
				project.ID = projectID
				t.Logf("✅ Project %s created successfully with ID: %s", project.Name, projectID)
				f.TestProjects = append(f.TestProjects, project)
			}
		} else if resp.StatusCode == http.StatusConflict {
			t.Logf("ℹ️  Project %s already exists", project.Name)
		} else {
			t.Logf("⚠️  Unexpected response for project %s: status %d", project.Name, resp.StatusCode)
		}
	}
}

// Cleanup cleans up Phase 2 test resources
func (f *Phase2TestFramework) Cleanup(t *testing.T) {
	t.Log("🧹 Cleaning up Phase 2 test resources...")
	
	// Clean up test data
	f.cleanupTestData(t)
	
	// Call base framework cleanup
	if f.E2ETestFramework != nil {
		f.E2ETestFramework.Cleanup(t)
	}
	
	t.Log("✅ Phase 2 cleanup complete")
}

// cleanupTestData removes test data created during Phase 2
func (f *Phase2TestFramework) cleanupTestData(t *testing.T) {
	t.Log("🗑️ Cleaning up test data...")
	
	// Logout all test users
	for _, user := range f.TestUsers {
		if user.Token != "" {
			t.Logf("🚪 Logging out user %s", user.Username)
			logoutResp, err := f.POST(t, "/api/v1/auth/logout", nil)
			if err != nil {
				t.Logf("⚠️  Failed to logout user %s: %v", user.Username, err)
				continue
			}
			logoutResp.Body.Close()
			
			if logoutResp.StatusCode == http.StatusOK {
				t.Logf("✅ User %s logged out successfully", user.Username)
			}
		}
	}
	
	// Note: Actual cleanup of database records would require admin privileges
	// For now, we just logout users to invalidate tokens
}

// GetTestUser returns a test user by username
func (f *Phase2TestFramework) GetTestUser(username string) *TestUser {
	for _, user := range f.TestUsers {
		if user.Username == username {
			return &user
		}
	}
	return nil
}

// GetTestProject returns a test project by name
func (f *Phase2TestFramework) GetTestProject(name string) *TestProject {
	for _, project := range f.TestProjects {
		if project.Name == name {
			return &project
		}
	}
	return nil
}