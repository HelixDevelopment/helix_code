package workflow

import (
	"sync"
	"time"
)

// Workflow represents a development workflow.
//
// Status, UpdatedAt, and the per-step Status / Error fields are mutated by
// the executor goroutine (see (*Executor).executeWorkflow). Callers from
// other goroutines (tests, status pollers, API handlers) MUST go through
// the GetStatus / GetUpdatedAt / GetStepStatuses accessors below — direct
// field reads race with the executor.
type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Mode        string         `json:"mode"`
	Steps       []Step         `json:"steps"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Status      WorkflowStatus `json:"status"`

	// mu guards Status, UpdatedAt, and the Status/Error fields of each
	// element of Steps while the executor goroutine is running.
	mu sync.RWMutex `json:"-"`
}

// GetStatus returns the current workflow status under the workflow lock.
// Use this instead of reading Workflow.Status directly when the workflow
// may be running.
func (w *Workflow) GetStatus() WorkflowStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status
}

// SetStatus atomically updates the workflow status and UpdatedAt.
func (w *Workflow) SetStatus(s WorkflowStatus) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = s
	w.UpdatedAt = time.Now()
}

// GetUpdatedAt returns UpdatedAt under the workflow lock.
func (w *Workflow) GetUpdatedAt() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.UpdatedAt
}

// touchUpdatedAt is an internal helper used by the executor to refresh
// UpdatedAt without changing Status.
func (w *Workflow) touchUpdatedAt() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.UpdatedAt = time.Now()
}

// setStepStatus updates the status (and optional error) of the step at
// index i under the workflow lock.
func (w *Workflow) setStepStatus(i int, s StepStatus, errMsg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Steps[i].Status = s
	if errMsg != "" {
		w.Steps[i].Error = errMsg
	}
	w.UpdatedAt = time.Now()
}

// getStepStatus returns the status of the step at index i under the
// workflow lock.
func (w *Workflow) getStepStatus(i int) StepStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Steps[i].Status
}

// Step represents a workflow step
type Step struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Type         StepType   `json:"type"`
	Action       StepAction `json:"action"`
	Dependencies []string   `json:"dependencies"`
	Status       StepStatus `json:"status"`
	Error        string     `json:"error,omitempty"`
}

// StepType represents the type of workflow step
type StepType string

const (
	StepTypeAnalysis   StepType = "analysis"
	StepTypeGeneration StepType = "generation"
	StepTypeExecution  StepType = "execution"
	StepTypeValidation StepType = "validation"
)

// StepAction represents the action to perform in a step
type StepAction string

const (
	StepActionAnalyzeCode    StepAction = "analyze_code"
	StepActionGenerateCode   StepAction = "generate_code"
	StepActionExecuteCommand StepAction = "execute_command"
	StepActionRunTests       StepAction = "run_tests"
	StepActionLintCode       StepAction = "lint_code"
	StepActionBuildProject   StepAction = "build_project"
)

// StepStatus represents the status of a workflow step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// WorkflowStatus represents the overall workflow status
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
)
