package core

import (
	"net/http"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST-E2E-004: Project Creation and Management
func TestProjectCreation(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Step 1: Register and login
	registrationData := map[string]interface{}{
		"username": "projecttest",
		"email":    "project@test.com",
		"password": "ProjectPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "projecttest",
		"password": "ProjectPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	// Step 2: Create new project
	projectData := map[string]interface{}{
		"name":        "Test Project",
		"description": "A test project for E2E testing",
		"type":        "go",
		"template":    "basic",
	}

	createResp, err := framework.POST(t, "/api/v1/projects", projectData)
	require.NoError(t, err)
	defer createResp.Body.Close()
	e2e.AssertStatus(t, createResp, http.StatusCreated)

	var createResponse map[string]interface{}
	e2e.ParseJSON(t, createResp, &createResponse)

	// Verify project creation
	assert.Contains(t, createResponse, "project_id")
	assert.Contains(t, createResponse, "name")
	assert.Equal(t, "Test Project", createResponse["name"])
	assert.Equal(t, "A test project for E2E testing", createResponse["description"])

	projectID := createResponse["project_id"].(string)

	// Step 3: Verify project exists
	getResp, err := framework.GET(t, "/api/v1/projects/"+projectID)
	require.NoError(t, err)
	defer getResp.Body.Close()
	e2e.AssertStatus(t, getResp, http.StatusOK)

	var getResponse map[string]interface{}
	e2e.ParseJSON(t, getResp, &getResponse)

	assert.Equal(t, projectID, getResponse["project_id"])
	assert.Equal(t, "Test Project", getResponse["name"])

	// Step 4: List user's projects
	listResp, err := framework.GET(t, "/api/v1/projects")
	require.NoError(t, err)
	defer listResp.Body.Close()
	e2e.AssertStatus(t, listResp, http.StatusOK)

	var listResponse map[string]interface{}
	e2e.ParseJSON(t, listResp, &listResponse)

	assert.Contains(t, listResponse, "projects")
	projects := listResponse["projects"].([]interface{})
	assert.Greater(t, len(projects), 0)

	// Verify our project is in the list
	found := false
	for _, p := range projects {
		project := p.(map[string]interface{})
		if project["project_id"] == projectID {
			found = true
			assert.Equal(t, "Test Project", project["name"])
			break
		}
	}
	assert.True(t, found, "Created project should be in the list")
}

// TEST-E2E-005: Project File Operations
func TestProjectFileOperations(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register, login, and create project
	registrationData := map[string]interface{}{
		"username": "filetest",
		"email":    "file@test.com",
		"password": "FilePass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "filetest",
		"password": "FilePass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	projectData := map[string]interface{}{
		"name":        "File Test Project",
		"description": "Testing file operations",
		"type":        "go",
	}

	projectResp, err := framework.POST(t, "/api/v1/projects", projectData)
	require.NoError(t, err)
	defer projectResp.Body.Close()
	e2e.AssertStatus(t, projectResp, http.StatusCreated)

	var projectResponse map[string]interface{}
	e2e.ParseJSON(t, projectResp, &projectResponse)
	projectID := projectResponse["project_id"].(string)

	// Step 1: Create a new file
	fileData := map[string]interface{}{
		"path":    "main.go",
		"content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}",
	}

	createFileResp, err := framework.POST(t, "/api/v1/projects/"+projectID+"/files", fileData)
	require.NoError(t, err)
	defer createFileResp.Body.Close()
	e2e.AssertStatus(t, createFileResp, http.StatusCreated)

	var createFileResponse map[string]interface{}
	e2e.ParseJSON(t, createFileResp, &createFileResponse)

	assert.Contains(t, createFileResponse, "file_id")
	assert.Equal(t, "main.go", createFileResponse["path"])

	// Step 2: Read file content
	readFileResp, err := framework.GET(t, "/api/v1/projects/"+projectID+"/files/main.go")
	require.NoError(t, err)
	defer readFileResp.Body.Close()
	e2e.AssertStatus(t, readFileResp, http.StatusOK)

	var readFileResponse map[string]interface{}
	e2e.ParseJSON(t, readFileResp, &readFileResponse)

	assert.Equal(t, "main.go", readFileResponse["path"])
	assert.Contains(t, readFileResponse["content"], "Hello, World!")

	// Step 3: Update file content
	updateFileData := map[string]interface{}{
		"path":    "main.go",
		"content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello, Updated World!\")\n    fmt.Println(\"Testing file updates\")\n}",
	}

	updateFileResp, err := framework.PUT(t, "/api/v1/projects/"+projectID+"/files/main.go", updateFileData)
	require.NoError(t, err)
	defer updateFileResp.Body.Close()
	e2e.AssertStatus(t, updateFileResp, http.StatusOK)

	// Verify update
	verifyResp, err := framework.GET(t, "/api/v1/projects/"+projectID+"/files/main.go")
	require.NoError(t, err)
	defer verifyResp.Body.Close()
	e2e.AssertStatus(t, verifyResp, http.StatusOK)

	var verifyResponse map[string]interface{}
	e2e.ParseJSON(t, verifyResp, &verifyResponse)

	assert.Contains(t, verifyResponse["content"], "Updated World!")
	assert.Contains(t, verifyResponse["content"], "Testing file updates")

	// Step 4: Create directory structure
	dirData := map[string]interface{}{
		"path":     "src/utils",
		"is_dir":   true,
		"content":  "",
	}

	createDirResp, err := framework.POST(t, "/api/v1/projects/"+projectID+"/files", dirData)
	require.NoError(t, err)
	defer createDirResp.Body.Close()
	e2e.AssertStatus(t, createDirResp, http.StatusCreated)

	// Step 5: List project files
	listFilesResp, err := framework.GET(t, "/api/v1/projects/"+projectID+"/files")
	require.NoError(t, err)
	defer listFilesResp.Body.Close()
	e2e.AssertStatus(t, listFilesResp, http.StatusOK)

	var listFilesResponse map[string]interface{}
	e2e.ParseJSON(t, listFilesResp, &listFilesResponse)

	assert.Contains(t, listFilesResponse, "files")
	files := listFilesResponse["files"].([]interface{})
	assert.Greater(t, len(files), 0)

	// Verify main.go exists in listing
	found := false
	for _, f := range files {
		file := f.(map[string]interface{})
		if file["path"] == "main.go" {
			found = true
			assert.Equal(t, false, file["is_dir"])
			break
		}
	}
	assert.True(t, found, "main.go should be in file listing")
}

// TEST-E2E-006: Project Collaboration
func TestProjectCollaboration(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Step 1: Create owner user and project
	ownerRegistration := map[string]interface{}{
		"username": "projectowner",
		"email":    "owner@test.com",
		"password": "OwnerPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", ownerRegistration)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "projectowner",
		"password": "OwnerPass123!",
	}

	loginResp, err := framework.POST(t, "/api/v1/auth/login", loginData)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	e2e.AssertStatus(t, loginResp, http.StatusOK)

	var loginResponse map[string]interface{}
	e2e.ParseJSON(t, loginResp, &loginResponse)

	token := loginResponse["token"].(string)
	framework.TestUser.Token = token

	projectData := map[string]interface{}{
		"name":        "Collaborative Project",
		"description": "A project for collaboration testing",
		"type":        "go",
	}

	projectResp, err := framework.POST(t, "/api/v1/projects", projectData)
	require.NoError(t, err)
	defer projectResp.Body.Close()
	e2e.AssertStatus(t, projectResp, http.StatusCreated)

	var projectResponse map[string]interface{}
	e2e.ParseJSON(t, projectResp, &projectResponse)
	projectID := projectResponse["project_id"].(string)

	// Step 2: Create collaborator user
	collabRegistration := map[string]interface{}{
		"username": "collaborator",
		"email":    "collab@test.com",
		"password": "CollabPass123!",
	}

	collabRegResp, err := framework.POST(t, "/api/v1/auth/register", collabRegistration)
	require.NoError(t, err)
	defer collabRegResp.Body.Close()
	e2e.AssertStatus(t, collabRegResp, http.StatusCreated)

	// Step 3: Add collaborator to project
	collabData := map[string]interface{}{
		"username": "collaborator",
		"role":     "editor",
	}

	addCollabResp, err := framework.POST(t, "/api/v1/projects/"+projectID+"/collaborators", collabData)
	require.NoError(t, err)
	defer addCollabResp.Body.Close()
	e2e.AssertStatus(t, addCollabResp, http.StatusOK)

	// Step 4: Login as collaborator
	collabLoginData := map[string]interface{}{
		"username": "collaborator",
		"password": "CollabPass123!",
	}

	collabLoginResp, err := framework.POST(t, "/api/v1/auth/login", collabLoginData)
	require.NoError(t, err)
	defer collabLoginResp.Body.Close()
	e2e.AssertStatus(t, collabLoginResp, http.StatusOK)

	var collabLoginResponse map[string]interface{}
	e2e.ParseJSON(t, collabLoginResp, &collabLoginResponse)

	collabToken := collabLoginResponse["token"].(string)
	framework.TestUser.Token = collabToken

	// Step 5: Collaborator accesses project
	accessResp, err := framework.GET(t, "/api/v1/projects/"+projectID)
	require.NoError(t, err)
	defer accessResp.Body.Close()
	e2e.AssertStatus(t, accessResp, http.StatusOK)

	// Step 6: Collaborator adds file
	fileData := map[string]interface{}{
		"path":    "collab_file.txt",
		"content": "Added by collaborator",
	}

	fileResp, err := framework.POST(t, "/api/v1/projects/"+projectID+"/files", fileData)
	require.NoError(t, err)
	defer fileResp.Body.Close()
	e2e.AssertStatus(t, fileResp, http.StatusCreated)

	// Step 7: Verify collaboration history
	collabHistoryResp, err := framework.GET(t, "/api/v1/projects/"+projectID+"/collaborators")
	require.NoError(t, err)
	defer collabHistoryResp.Body.Close()
	e2e.AssertStatus(t, collabHistoryResp, http.StatusOK)

	var historyResponse map[string]interface{}
	e2e.ParseJSON(t, collabHistoryResp, &historyResponse)

	assert.Contains(t, historyResponse, "collaborators")
	collaborators := historyResponse["collaborators"].([]interface{})
	assert.Greater(t, len(collaborators), 0)
}