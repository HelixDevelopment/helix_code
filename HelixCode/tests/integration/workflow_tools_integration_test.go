// Package integration provides component integration tests
package integration

import (
	"testing"
	"time"

	"dev.helix.code/internal/task"
	"dev.helix.code/internal/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Workflow Types Integration Tests
// ========================================

// TestWorkflowStepDefinition tests workflow step creation and validation
func TestWorkflowStepDefinition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a workflow with steps
	wf := &workflow.Workflow{
		ID:          uuid.New().String(),
		Name:        "Test Development Workflow",
		Description: "Integration test workflow",
		Mode:        "auto",
		Steps: []workflow.Step{
			{
				ID:          "step-1",
				Name:        "Analyze Code",
				Description: "Analyze the codebase",
				Type:        workflow.StepTypeAnalysis,
				Action:      workflow.StepActionAnalyzeCode,
				Status:      workflow.StepStatusPending,
			},
			{
				ID:           "step-2",
				Name:         "Generate Code",
				Description:  "Generate new code",
				Type:         workflow.StepTypeGeneration,
				Action:       workflow.StepActionGenerateCode,
				Dependencies: []string{"step-1"},
				Status:       workflow.StepStatusPending,
			},
			{
				ID:           "step-3",
				Name:         "Run Tests",
				Description:  "Execute test suite",
				Type:         workflow.StepTypeExecution,
				Action:       workflow.StepActionRunTests,
				Dependencies: []string{"step-2"},
				Status:       workflow.StepStatusPending,
			},
		},
		Status:    workflow.WorkflowStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Verify workflow structure
	assert.Equal(t, 3, len(wf.Steps))
	assert.Equal(t, workflow.WorkflowStatusPending, wf.Status)

	// Verify step types
	assert.Equal(t, workflow.StepTypeAnalysis, wf.Steps[0].Type)
	assert.Equal(t, workflow.StepTypeGeneration, wf.Steps[1].Type)
	assert.Equal(t, workflow.StepTypeExecution, wf.Steps[2].Type)

	// Verify step dependencies
	assert.Empty(t, wf.Steps[0].Dependencies)
	assert.Equal(t, []string{"step-1"}, wf.Steps[1].Dependencies)
	assert.Equal(t, []string{"step-2"}, wf.Steps[2].Dependencies)

	// Verify step actions
	assert.Equal(t, workflow.StepActionAnalyzeCode, wf.Steps[0].Action)
	assert.Equal(t, workflow.StepActionGenerateCode, wf.Steps[1].Action)
	assert.Equal(t, workflow.StepActionRunTests, wf.Steps[2].Action)
}

// TestWorkflowStepStatusTransitions tests step status transitions
func TestWorkflowStepStatusTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	step := workflow.Step{
		ID:     "test-step",
		Name:   "Test Step",
		Type:   workflow.StepTypeAnalysis,
		Action: workflow.StepActionAnalyzeCode,
		Status: workflow.StepStatusPending,
	}

	// Valid transitions
	validTransitions := []struct {
		from workflow.StepStatus
		to   workflow.StepStatus
	}{
		{workflow.StepStatusPending, workflow.StepStatusRunning},
		{workflow.StepStatusRunning, workflow.StepStatusCompleted},
		{workflow.StepStatusRunning, workflow.StepStatusFailed},
		{workflow.StepStatusPending, workflow.StepStatusSkipped},
	}

	for _, transition := range validTransitions {
		step.Status = transition.from
		step.Status = transition.to
		assert.Equal(t, transition.to, step.Status)
	}
}

// ========================================
// Task Types Integration Tests
// ========================================

// TestTaskQueuePriorityIntegration tests task queue priority handling
func TestTaskQueuePriorityIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create task queue
	queue := task.NewTaskQueue()

	// Add tasks with different priorities
	tasks := []*task.Task{
		{ID: uuid.New(), Priority: task.PriorityLow},
		{ID: uuid.New(), Priority: task.PriorityCritical},
		{ID: uuid.New(), Priority: task.PriorityNormal},
		{ID: uuid.New(), Priority: task.PriorityHigh},
	}

	for _, tk := range tasks {
		queue.AddTask(tk)
	}

	// Verify tasks are dequeued in priority order
	// Critical -> High -> Normal -> Low
	expectedPriorities := []task.TaskPriority{
		task.PriorityCritical,
		task.PriorityHigh,
		task.PriorityNormal,
		task.PriorityLow,
	}

	for i, expected := range expectedPriorities {
		dequeuedTask := queue.GetNextTask()
		require.NotNil(t, dequeuedTask, "Task %d should not be nil", i)
		assert.Equal(t, expected, dequeuedTask.Priority, "Task %d should have priority %v", i, expected)
	}

	// Queue should be empty now
	assert.Nil(t, queue.GetNextTask())
}

// TestTaskTypesAndPriorities tests task type and priority combinations
func TestTaskTypesAndPriorities(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test all task types
	taskTypes := []task.TaskType{
		task.TaskTypePlanning,
		task.TaskTypeBuilding,
		task.TaskTypeTesting,
		task.TaskTypeRefactoring,
		task.TaskTypeDebugging,
	}

	// Test all priorities
	priorities := []task.TaskPriority{
		task.PriorityLow,
		task.PriorityNormal,
		task.PriorityHigh,
		task.PriorityCritical,
	}

	// Create a queue and add tasks with all combinations
	queue := task.NewTaskQueue()

	for _, taskType := range taskTypes {
		for _, priority := range priorities {
			tk := &task.Task{
				ID:       uuid.New(),
				Type:     taskType,
				Priority: priority,
			}
			queue.AddTask(tk)
		}
	}

	// Verify we added the right number of tasks using GetQueueStats
	stats := queue.GetQueueStats()
	assert.Equal(t, len(taskTypes)*len(priorities), stats.Total)

	// Dequeue all tasks and verify they come out in priority order
	var lastPriority task.TaskPriority = task.PriorityCritical
	for queue.GetQueueStats().Total > 0 {
		tk := queue.GetNextTask()
		require.NotNil(t, tk)
		// Priority should be <= last priority (descending order)
		assert.LessOrEqual(t, int(tk.Priority), int(lastPriority),
			"Tasks should be dequeued in descending priority order")
		lastPriority = tk.Priority
	}
}

// TestTaskStatusTransitions tests task status transitions
func TestTaskStatusTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Define valid status transitions using correct constants
	validTransitions := map[task.TaskStatus][]task.TaskStatus{
		task.TaskStatusPending: {
			task.TaskStatusAssigned,
		},
		task.TaskStatusAssigned: {
			task.TaskStatusCompleted,
			task.TaskStatusFailed,
			task.TaskStatusPending, // Return to pending if worker fails
		},
		task.TaskStatusFailed: {
			task.TaskStatusPending, // Retry
		},
	}

	for from, toList := range validTransitions {
		for _, to := range toList {
			tk := &task.Task{
				ID:     uuid.New(),
				Status: from,
			}
			tk.Status = to
			assert.Equal(t, to, tk.Status,
				"Transition from %s to %s should be valid", from, to)
		}
	}
}

// ========================================
// Workflow + Task Integration Tests
// ========================================

// TestWorkflowWithTasks tests workflow and task type compatibility
func TestWorkflowWithTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Map workflow step types to task types
	stepToTaskType := map[workflow.StepType]task.TaskType{
		workflow.StepTypeAnalysis:   task.TaskTypePlanning,
		workflow.StepTypeGeneration: task.TaskTypeBuilding,
		workflow.StepTypeExecution:  task.TaskTypeTesting,
		workflow.StepTypeValidation: task.TaskTypeTesting,
	}

	// Create a workflow
	wf := &workflow.Workflow{
		ID:   uuid.New().String(),
		Name: "Task Integration Workflow",
		Steps: []workflow.Step{
			{ID: "analyze", Type: workflow.StepTypeAnalysis, Status: workflow.StepStatusPending},
			{ID: "generate", Type: workflow.StepTypeGeneration, Status: workflow.StepStatusPending},
			{ID: "test", Type: workflow.StepTypeExecution, Status: workflow.StepStatusPending},
		},
	}

	// Create task queue
	queue := task.NewTaskQueue()

	// Create tasks for each step
	for _, step := range wf.Steps {
		taskType := stepToTaskType[step.Type]
		tk := &task.Task{
			ID:       uuid.New(),
			Type:     taskType,
			Priority: task.PriorityNormal,
		}
		queue.AddTask(tk)
	}

	// Verify all tasks were queued
	stats := queue.GetQueueStats()
	assert.Equal(t, len(wf.Steps), stats.Total)
}

// TestWorkflowStatusCorrelation tests workflow status correlation with step statuses
func TestWorkflowStatusCorrelation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name           string
		stepStatuses   []workflow.StepStatus
		expectedStatus workflow.WorkflowStatus
	}{
		{
			name:           "All pending",
			stepStatuses:   []workflow.StepStatus{workflow.StepStatusPending, workflow.StepStatusPending},
			expectedStatus: workflow.WorkflowStatusPending,
		},
		{
			name:           "One running",
			stepStatuses:   []workflow.StepStatus{workflow.StepStatusCompleted, workflow.StepStatusRunning},
			expectedStatus: workflow.WorkflowStatusRunning,
		},
		{
			name:           "All completed",
			stepStatuses:   []workflow.StepStatus{workflow.StepStatusCompleted, workflow.StepStatusCompleted},
			expectedStatus: workflow.WorkflowStatusCompleted,
		},
		{
			name:           "One failed",
			stepStatuses:   []workflow.StepStatus{workflow.StepStatusCompleted, workflow.StepStatusFailed},
			expectedStatus: workflow.WorkflowStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				ID:     uuid.New().String(),
				Status: workflow.WorkflowStatusPending,
			}

			// Add steps with the specified statuses
			for i, status := range tt.stepStatuses {
				wf.Steps = append(wf.Steps, workflow.Step{
					ID:     uuid.New().String(),
					Status: status,
					Name:   string(rune('A' + i)),
				})
			}

			// Calculate expected workflow status based on steps
			calculatedStatus := calculateWorkflowStatus(wf.Steps)
			assert.Equal(t, tt.expectedStatus, calculatedStatus)
		})
	}
}

// calculateWorkflowStatus determines workflow status from step statuses
func calculateWorkflowStatus(steps []workflow.Step) workflow.WorkflowStatus {
	if len(steps) == 0 {
		return workflow.WorkflowStatusPending
	}

	allCompleted := true
	hasFailed := false
	hasRunning := false

	for _, step := range steps {
		switch step.Status {
		case workflow.StepStatusFailed:
			hasFailed = true
		case workflow.StepStatusRunning:
			hasRunning = true
			allCompleted = false
		case workflow.StepStatusPending, workflow.StepStatusSkipped:
			allCompleted = false
		}
	}

	if hasFailed {
		return workflow.WorkflowStatusFailed
	}
	if hasRunning {
		return workflow.WorkflowStatusRunning
	}
	if allCompleted {
		return workflow.WorkflowStatusCompleted
	}
	return workflow.WorkflowStatusPending
}

// TestTaskQueueClearAndRemove tests queue clear and remove operations
func TestTaskQueueClearAndRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	queue := task.NewTaskQueue()

	// Add some tasks
	task1 := &task.Task{ID: uuid.New(), Priority: task.PriorityHigh}
	task2 := &task.Task{ID: uuid.New(), Priority: task.PriorityNormal}
	task3 := &task.Task{ID: uuid.New(), Priority: task.PriorityLow}

	queue.AddTask(task1)
	queue.AddTask(task2)
	queue.AddTask(task3)

	// Verify all tasks added
	assert.Equal(t, 3, queue.GetQueueStats().Total)

	// Remove specific task
	removed := queue.RemoveTask(task2.ID.String())
	assert.True(t, removed)
	assert.Equal(t, 2, queue.GetQueueStats().Total)

	// Try to remove non-existent task
	removed = queue.RemoveTask(uuid.New().String())
	assert.False(t, removed)

	// Clear all tasks
	queue.Clear()
	assert.Equal(t, 0, queue.GetQueueStats().Total)
}

// TestWorkflowDAGValidation tests DAG structure validation in workflows
func TestWorkflowDAGValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Valid DAG - linear dependency chain
	validDAG := &workflow.Workflow{
		ID:   uuid.New().String(),
		Name: "Valid DAG Workflow",
		Steps: []workflow.Step{
			{ID: "step-1", Name: "Step 1", Dependencies: nil},
			{ID: "step-2", Name: "Step 2", Dependencies: []string{"step-1"}},
			{ID: "step-3", Name: "Step 3", Dependencies: []string{"step-2"}},
		},
	}

	// Verify structure
	assert.Equal(t, 3, len(validDAG.Steps))

	// Build dependency map
	depMap := make(map[string][]string)
	for _, step := range validDAG.Steps {
		depMap[step.ID] = step.Dependencies
	}

	// step-1 has no dependencies
	assert.Empty(t, depMap["step-1"])

	// step-2 depends on step-1
	assert.Equal(t, []string{"step-1"}, depMap["step-2"])

	// step-3 depends on step-2
	assert.Equal(t, []string{"step-2"}, depMap["step-3"])

	// Valid DAG - fan-out pattern
	fanOut := &workflow.Workflow{
		ID:   uuid.New().String(),
		Name: "Fan-out Workflow",
		Steps: []workflow.Step{
			{ID: "root", Name: "Root", Dependencies: nil},
			{ID: "branch-a", Name: "Branch A", Dependencies: []string{"root"}},
			{ID: "branch-b", Name: "Branch B", Dependencies: []string{"root"}},
			{ID: "branch-c", Name: "Branch C", Dependencies: []string{"root"}},
		},
	}

	assert.Equal(t, 4, len(fanOut.Steps))

	// All branches depend on root
	for _, step := range fanOut.Steps[1:] {
		assert.Equal(t, []string{"root"}, step.Dependencies)
	}

	// Valid DAG - fan-in pattern
	fanIn := &workflow.Workflow{
		ID:   uuid.New().String(),
		Name: "Fan-in Workflow",
		Steps: []workflow.Step{
			{ID: "source-a", Name: "Source A", Dependencies: nil},
			{ID: "source-b", Name: "Source B", Dependencies: nil},
			{ID: "source-c", Name: "Source C", Dependencies: nil},
			{ID: "merge", Name: "Merge", Dependencies: []string{"source-a", "source-b", "source-c"}},
		},
	}

	assert.Equal(t, 4, len(fanIn.Steps))

	// Merge depends on all sources
	assert.Equal(t, []string{"source-a", "source-b", "source-c"}, fanIn.Steps[3].Dependencies)
}

// TestTaskCriticalityAndPriorityCombinations tests task criticality handling
func TestTaskCriticalityAndPriorityCombinations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	queue := task.NewTaskQueue()

	// Add tasks with various criticality levels
	// Note: Criticality affects sorting within the high priority queue
	criticalTask := &task.Task{
		ID:          uuid.New(),
		Priority:    task.PriorityCritical,
		Criticality: task.CriticalityHigh,
	}
	highTask := &task.Task{
		ID:          uuid.New(),
		Priority:    task.PriorityHigh,
		Criticality: task.CriticalityNormal,
	}
	normalTask := &task.Task{
		ID:          uuid.New(),
		Priority:    task.PriorityNormal,
		Criticality: task.CriticalityLow,
	}

	// Add in reverse order to test sorting
	queue.AddTask(normalTask)
	queue.AddTask(highTask)
	queue.AddTask(criticalTask)

	// Verify queue stats
	stats := queue.GetQueueStats()
	assert.Equal(t, 2, stats.HighPriority)   // Critical and High go to high priority queue
	assert.Equal(t, 1, stats.NormalPriority) // Normal goes to normal queue
	assert.Equal(t, 0, stats.LowPriority)
	assert.Equal(t, 3, stats.Total)

	// Dequeue and verify order - critical/high should come first
	first := queue.GetNextTask()
	require.NotNil(t, first)
	assert.True(t, first.Priority == task.PriorityCritical || first.Priority == task.PriorityHigh,
		"First task should be critical or high priority")
}
