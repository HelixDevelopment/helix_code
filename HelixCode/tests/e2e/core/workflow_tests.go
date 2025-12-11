package core

import (
	"net/http"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST-E2E-007: Task Creation and Execution
func TestTaskCreationExecution(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register, login, and create project
	registrationData := map[string]interface{}{
		"username": "tasktest",
		"email":    "task@test.com",
		"password": "TaskPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "tasktest",
		"password": "TaskPass123!",
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
		"name":        "Task Test Project",
		"description": "Testing task operations",
		"type":        "go",
	}

	projectResp, err := framework.POST(t, "/api/v1/projects", projectData)
	require.NoError(t, err)
	defer projectResp.Body.Close()
	e2e.AssertStatus(t, projectResp, http.StatusCreated)

	var projectResponse map[string]interface{}
	e2e.ParseJSON(t, projectResp, &projectResponse)
	projectID := projectResponse["project_id"].(string)

	// Step 1: Create a task
	taskData := map[string]interface{}{
		"title":       "Build Go Application",
		"description": "Build a simple Go application with dependencies",
		"type":        "build",
		"project_id":  projectID,
		"priority":    "high",
	}

	taskResp, err := framework.POST(t, "/api/v1/tasks", taskData)
	require.NoError(t, err)
	defer taskResp.Body.Close()
	e2e.AssertStatus(t, taskResp, http.StatusCreated)

	var taskResponse map[string]interface{}
	e2e.ParseJSON(t, taskResp, &taskResponse)

	assert.Contains(t, taskResponse, "task_id")
	assert.Equal(t, "Build Go Application", taskResponse["title"])
	assert.Equal(t, "pending", taskResponse["status"])

	taskID := taskResponse["task_id"].(string)

	// Step 2: Monitor task status
	statusResp, err := framework.GET(t, "/api/v1/tasks/"+taskID)
	require.NoError(t, err)
	defer statusResp.Body.Close()
	e2e.AssertStatus(t, statusResp, http.StatusOK)

	var statusResponse map[string]interface{}
	e2e.ParseJSON(t, statusResp, &statusResponse)

	assert.Equal(t, taskID, statusResponse["task_id"])
	assert.Contains(t, []string{"pending", "running", "completed", "failed"}, statusResponse["status"])

	// Step 3: Create dependent task
	dependentTaskData := map[string]interface{}{
		"title":       "Run Tests",
		"description": "Run unit tests after build completes",
		"type":        "test",
		"project_id":  projectID,
		"priority":    "medium",
		"dependencies": []string{taskID},
	}

	depTaskResp, err := framework.POST(t, "/api/v1/tasks", dependentTaskData)
	require.NoError(t, err)
	depTaskResp.Body.Close()
	e2e.AssertStatus(t, depTaskResp, http.StatusCreated)

	// Step 4: List tasks for project
	listTasksResp, err := framework.GET(t, "/api/v1/projects/"+projectID+"/tasks")
	require.NoError(t, err)
	defer listTasksResp.Body.Close()
	e2e.AssertStatus(t, listTasksResp, http.StatusOK)

	var listTasksResponse map[string]interface{}
	e2e.ParseJSON(t, listTasksResp, &listTasksResponse)

	assert.Contains(t, listTasksResponse, "tasks")
	tasks := listTasksResponse["tasks"].([]interface{})
	assert.GreaterOrEqual(t, len(tasks), 2)
}

// TEST-E2E-008: Workflow Automation
func TestWorkflowAutomation(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register, login, and create project
	registrationData := map[string]interface{}{
		"username": "workflowtest",
		"email":    "workflow@test.com",
		"password": "WorkflowPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "workflowtest",
		"password": "WorkflowPass123!",
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
		"name":        "Workflow Test Project",
		"description": "Testing workflow automation",
		"type":        "go",
	}

	projectResp, err := framework.POST(t, "/api/v1/projects", projectData)
	require.NoError(t, err)
	defer projectResp.Body.Close()
	e2e.AssertStatus(t, projectResp, http.StatusCreated)

	var projectResponse map[string]interface{}
	e2e.ParseJSON(t, projectResp, &projectResponse)
	projectID := projectResponse["project_id"].(string)

	// Step 1: Create workflow definition
	workflowData := map[string]interface{}{
		"name":        "CI/CD Pipeline",
		"description": "Automated build and test pipeline",
		"project_id":  projectID,
		"steps": []map[string]interface{}{
			{
				"name":   "Build",
				"type":   "build",
				"config": map[string]interface{}{
					"command": "go build",
				},
			},
			{
				"name":   "Test",
				"type":   "test",
				"depends_on": []string{"Build"},
				"config": map[string]interface{}{
					"command": "go test",
				},
			},
			{
				"name":   "Deploy",
				"type":   "deploy",
				"depends_on": []string{"Test"},
				"config": map[string]interface{}{
					"target": "staging",
				},
			},
		},
	}

	workflowResp, err := framework.POST(t, "/api/v1/workflows", workflowData)
	require.NoError(t, err)
	defer workflowResp.Body.Close()
	e2e.AssertStatus(t, workflowResp, http.StatusCreated)

	var workflowResponse map[string]interface{}
	e2e.ParseJSON(t, workflowResp, &workflowResponse)

	assert.Contains(t, workflowResponse, "workflow_id")
	assert.Contains(t, workflowResponse, "steps")

	workflowID := workflowResponse["workflow_id"].(string)

	// Step 2: Trigger workflow execution
	triggerData := map[string]interface{}{
		"trigger": "manual",
		"parameters": map[string]interface{}{
			"branch": "main",
			"environment": "testing",
		},
	}

	triggerResp, err := framework.POST(t, "/api/v1/workflows/"+workflowID+"/execute", triggerData)
	require.NoError(t, err)
	defer triggerResp.Body.Close()
	e2e.AssertStatus(t, triggerResp, http.StatusAccepted)

	var triggerResponse map[string]interface{}
	e2e.ParseJSON(t, triggerResp, &triggerResponse)

	assert.Contains(t, triggerResponse, "execution_id")
	assert.Contains(t, triggerResponse, "status")

	executionID := triggerResponse["execution_id"].(string)

	// Step 3: Monitor workflow execution
	execResp, err := framework.GET(t, "/api/v1/workflows/executions/"+executionID)
	require.NoError(t, err)
	defer execResp.Body.Close()
	e2e.AssertStatus(t, execResp, http.StatusOK)

	var execResponse map[string]interface{}
	e2e.ParseJSON(t, execResp, &execResponse)

	assert.Equal(t, executionID, execResponse["execution_id"])
	assert.Contains(t, []string{"pending", "running", "completed", "failed"}, execResponse["status"])

	// Step 4: List workflow executions
	listExecResp, err := framework.GET(t, "/api/v1/workflows/"+workflowID+"/executions")
	require.NoError(t, err)
	defer listExecResp.Body.Close()
	e2e.AssertStatus(t, listExecResp, http.StatusOK)

	var listExecResponse map[string]interface{}
	e2e.ParseJSON(t, listExecResp, &listExecResponse)

	assert.Contains(t, listExecResponse, "executions")
	executions := listExecResponse["executions"].([]interface{})
	assert.Greater(t, len(executions), 0)
}

// TEST-E2E-009: Task Checkpointing and Recovery
func TestTaskCheckpointingRecovery(t *testing.T) {
	framework := e2e.NewE2ETestFramework(t)
	defer framework.Cleanup(t)

	e2e.WaitForServer(t, framework, 30*time.Second)

	// Setup: Register, login, and create project
	registrationData := map[string]interface{}{
		"username": "checkpointtest",
		"email":    "checkpoint@test.com",
		"password": "CheckpointPass123!",
	}

	regResp, err := framework.POST(t, "/api/v1/auth/register", registrationData)
	require.NoError(t, err)
	defer regResp.Body.Close()
	e2e.AssertStatus(t, regResp, http.StatusCreated)

	loginData := map[string]interface{}{
		"username": "checkpointtest",
		"password": "CheckpointPass123!",
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
		"name":        "Checkpoint Test Project",
		"description": "Testing task checkpointing",
		"type":        "go",
	}

	projectResp, err := framework.POST(t, "/api/v1/projects", projectData)
	require.NoError(t, err)
	defer projectResp.Body.Close()
	e2e.AssertStatus(t, projectResp, http.StatusCreated)

	var projectResponse map[string]interface{}
	e2e.ParseJSON(t, projectResp, &projectResponse)
	projectID := projectResponse["project_id"].(string)

	// Step 1: Create long-running task with checkpointing
	taskData := map[string]interface{}{
		"title":       "Data Processing Task",
		"description": "Process large dataset with checkpoints",
		"type":        "data_processing",
		"project_id":  projectID,
		"priority":    "high",
		"checkpoint_enabled": true,
		"checkpoint_interval": 60,
		"config": map[string]interface{}{
			"dataset_size": "large",
			"batch_size":   1000,
		},
	}

	taskResp, err := framework.POST(t, "/api/v1/tasks", taskData)
	require.NoError(t, err)
	defer taskResp.Body.Close()
	e2e.AssertStatus(t, taskResp, http.StatusCreated)

	var taskResponse map[string]interface{}
	e2e.ParseJSON(t, taskResp, &taskResponse)

	taskID := taskResponse["task_id"].(string)

	// Step 2: Check task checkpoints
	checkpointsResp, err := framework.GET(t, "/api/v1/tasks/"+taskID+"/checkpoints")
	require.NoError(t, err)
	defer checkpointsResp.Body.Close()
	e2e.AssertStatus(t, checkpointsResp, http.StatusOK)

	var checkpointsResponse map[string]interface{}
	e2e.ParseJSON(t, checkpointsResp, &checkpointsResponse)

	assert.Contains(t, checkpointsResponse, "checkpoints")
	checkpoints := checkpointsResponse["checkpoints"].([]interface{})
	assert.GreaterOrEqual(t, len(checkpoints), 0)

	// Step 3: Simulate task failure and recovery
	simulateFailureData := map[string]interface{}{
		"action": "simulate_failure",
		"reason": "network_timeout",
	}

	failureResp, err := framework.POST(t, "/api/v1/tasks/"+taskID+"/fail", simulateFailureData)
	require.NoError(t, err)
	defer failureResp.Body.Close()
	e2e.AssertStatus(t, failureResp, http.StatusOK)

	// Step 4: Recover task from last checkpoint
	recoverData := map[string]interface{}{
		"action": "recover",
		"from_checkpoint": true,
	}

	recoverResp, err := framework.POST(t, "/api/v1/tasks/"+taskID+"/recover", recoverData)
	require.NoError(t, err)
	defer recoverResp.Body.Close()
	e2e.AssertStatus(t, recoverResp, http.StatusAccepted)

	var recoverResponse map[string]interface{}
	e2e.ParseJSON(t, recoverResp, &recoverResponse)

	assert.Contains(t, recoverResponse, "recovery_id")
	assert.Equal(t, "recovering", recoverResponse["status"])

	// Step 5: Verify task status after recovery
	statusResp, err := framework.GET(t, "/api/v1/tasks/"+taskID)
	require.NoError(t, err)
	defer statusResp.Body.Close()
	e2e.AssertStatus(t, statusResp, http.StatusOK)

	var statusResponse map[string]interface{}
	e2e.ParseJSON(t, statusResp, &statusResponse)

	assert.Equal(t, taskID, statusResponse["task_id"])
	assert.Contains(t, []string{"recovering", "pending", "running"}, statusResponse["status"])
	assert.Contains(t, statusResponse, "last_checkpoint")
}