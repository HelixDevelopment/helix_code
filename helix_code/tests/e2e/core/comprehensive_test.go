package core

import (
	"fmt"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
)

// ComprehensiveTest verifies that all E2E test infrastructure is working
func TestComprehensiveE2E(t *testing.T) {
	fmt.Println("🚀 Starting Comprehensive E2E Test Suite")
	
	// Initialize the test framework
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	// Wait for server to be ready
	e2e.WaitForServer(t, framework, 30*time.Second)
	
	// Test basic functionality
	testBasicRegistration(t, framework)
	testBasicLogin(t, framework)
	testBasicProjectCreation(t, framework)
	
	fmt.Println("✅ Comprehensive E2E Test Suite completed successfully")
}

func testBasicRegistration(t *testing.T, framework *e2e.E2ETestFramework) {
	fmt.Println("📝 Testing basic user registration...")
	
	registrationData := map[string]interface{}{
		"username": "comprehensive_test_user",
		"email":    "comprehensive@test.com",
		"password": "ComprehensiveTest123!",
	}

	resp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	e2e.ParseJSON(t, resp, &response)
	
	if _, ok := response["user_id"]; !ok {
		t.Error("Response should contain user_id")
	}
	
	fmt.Println("✅ Basic registration test passed")
}

func testBasicLogin(t *testing.T, framework *e2e.E2ETestFramework) {
	fmt.Println("🔐 Testing basic user login...")
	
	loginData := map[string]interface{}{
		"username": "comprehensive_test_user",
		"password": "ComprehensiveTest123!",
	}

	resp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	e2e.ParseJSON(t, resp, &response)
	
	if _, ok := response["token"]; !ok {
		t.Error("Response should contain token")
	}
	
	// Set token for subsequent requests
	if token, ok := response["token"].(string); ok {
		framework.TestUser.Token = token
	}
	
	fmt.Println("✅ Basic login test passed")
}

func testBasicProjectCreation(t *testing.T, framework *e2e.E2ETestFramework) {
	fmt.Println("🏗️ Testing basic project creation...")
	
	projectData := map[string]interface{}{
		"name":        "Comprehensive Test Project",
		"description": "Project created by comprehensive E2E test",
		"type":        "go",
	}

	resp, err := framework.POST(t, "/api/v1/projects", projectData)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	e2e.ParseJSON(t, resp, &response)
	
	if _, ok := response["project_id"]; !ok {
		t.Error("Response should contain project_id")
	}
	
	fmt.Println("✅ Basic project creation test passed")
}