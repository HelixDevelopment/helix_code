package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/agent/task"
	"github.com/google/uuid"
)

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID           string
	Name         string
	AgentType    AgentType
	RequiredCaps []Capability
	Input        map[string]interface{}
	DependsOn    []string // IDs of steps that must complete first
	Optional     bool     // If true, workflow continues even if this step fails
}

// Workflow represents a multi-step process executed by multiple agents
type Workflow struct {
	ID          string
	Name        string
	Description string
	Steps       []*WorkflowStep
	Status      WorkflowStatus
	Results     map[string]*task.Result // step ID -> result
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
	mu          sync.RWMutex
}

// WorkflowStatus represents the current status of a workflow
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
)

// NewWorkflow creates a new workflow
func NewWorkflow(name, description string) *Workflow {
	return &Workflow{
		ID:          GenerateWorkflowID(),
		Name:        name,
		Description: description,
		Steps:       make([]*WorkflowStep, 0),
		Status:      WorkflowStatusPending,
		Results:     make(map[string]*task.Result),
		CreatedAt:   time.Now(),
	}
}

// AddStep adds a step to the workflow
func (w *Workflow) AddStep(step *WorkflowStep) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Steps = append(w.Steps, step)
}

// Start marks the workflow as started
func (w *Workflow) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.StartedAt = &now
	w.Status = WorkflowStatusRunning
}

// Complete marks the workflow as completed
func (w *Workflow) Complete() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.CompletedAt = &now
	w.Status = WorkflowStatusCompleted
}

// Fail marks the workflow as failed
func (w *Workflow) Fail() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.CompletedAt = &now
	w.Status = WorkflowStatusFailed
}

// Cancel marks the workflow as cancelled
func (w *Workflow) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.CompletedAt = &now
	w.Status = WorkflowStatusCancelled
}

// SetStepResult records the result of a workflow step
func (w *Workflow) SetStepResult(stepID string, result *task.Result) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Results[stepID] = result
}

// GetStepResult retrieves the result of a workflow step
func (w *Workflow) GetStepResult(stepID string) (*task.Result, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result, ok := w.Results[stepID]
	return result, ok
}

// IsStepReady checks if a step's dependencies are satisfied
func (w *Workflow) IsStepReady(step *WorkflowStep) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Check if all dependencies are completed successfully
	for _, depID := range step.DependsOn {
		result, ok := w.Results[depID]
		if !ok {
			return false // Dependency not yet executed
		}
		if !result.Success {
			// If dependency failed, check if it was optional
			isOptional := false
			for _, s := range w.Steps {
				if s.ID == depID && s.Optional {
					isOptional = true
					break
				}
			}
			if !isOptional {
				return false // Required dependency failed
			}
			// Optional dependency failed, continue to next dependency
		}
	}
	return true
}

// GetReadySteps returns all steps that are ready to execute
func (w *Workflow) GetReadySteps() []*WorkflowStep {
	w.mu.RLock()
	defer w.mu.RUnlock()

	ready := make([]*WorkflowStep, 0)
	for _, step := range w.Steps {
		// Skip if already executed
		if _, executed := w.Results[step.ID]; executed {
			continue
		}
		// Check if dependencies are satisfied
		if w.IsStepReady(step) {
			ready = append(ready, step)
		}
	}
	return ready
}

// WorkflowExecutor executes workflows using the coordinator
type WorkflowExecutor struct {
	coordinator *Coordinator
	workflows   map[string]*Workflow
	mu          sync.RWMutex
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(coordinator *Coordinator) *WorkflowExecutor {
	return &WorkflowExecutor{
		coordinator: coordinator,
		workflows:   make(map[string]*Workflow),
	}
}

// Execute executes a workflow
func (we *WorkflowExecutor) Execute(ctx context.Context, workflow *Workflow) error {
	// Register workflow
	we.mu.Lock()
	we.workflows[workflow.ID] = workflow
	we.mu.Unlock()

	// Start workflow
	workflow.Start()

	// Execute steps in dependency order
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			workflow.Cancel()
			return ctx.Err()
		default:
		}

		// Get ready steps
		readySteps := workflow.GetReadySteps()
		if len(readySteps) == 0 {
			// No more steps ready - check if workflow is complete
			workflow.mu.RLock()
			totalSteps := len(workflow.Steps)
			completedSteps := len(workflow.Results)
			workflow.mu.RUnlock()

			if completedSteps == totalSteps {
				// All steps completed
				workflow.Complete()
				return nil
			}

			// Check if we're stuck (no ready steps but not complete)
			// This happens when all remaining steps depend on failed required steps
			allStepsDone := true
			for _, step := range workflow.Steps {
				if _, executed := workflow.Results[step.ID]; !executed {
					allStepsDone = false
					break
				}
			}

			if !allStepsDone {
				// We have unexecuted steps but none are ready - workflow is stuck
				workflow.Fail()
				return fmt.Errorf("workflow stuck: remaining steps have unsatisfied dependencies")
			}

			// All steps are done
			workflow.Complete()
			return nil
		}

		// Execute ready steps in parallel
		var wg sync.WaitGroup
		errChan := make(chan error, len(readySteps))

		for _, step := range readySteps {
			wg.Add(1)
			go func(s *WorkflowStep) {
				defer wg.Done()
				if err := we.executeStep(ctx, workflow, s); err != nil {
					errChan <- err
				}
			}(step)
		}

		wg.Wait()
		close(errChan)

		// Check for errors from non-optional steps
		for err := range errChan {
			if err != nil {
				workflow.Fail()
				return err
			}
		}

		// Continue to next iteration to process newly ready steps
	}
}

// executeStep executes a single workflow step.
//
// Round-36 §11.4 anti-bluff fix (CONST-035 / Article XI §11.9, Pattern A1
// parameter-discard): the previous implementation read only
// step.RequiredCaps[0] when the WorkflowStep.RequiredCaps field is a
// SLICE — meaning a step declaring RequiredCaps=[CodeGen, Testing,
// Review] was satisfied by any agent with CodeGen alone. The downstream
// task DID propagate the full RequiredCapabilities list (lines below),
// so the workflow operator's intent ("the step needs ALL of these") was
// silently downgraded to "ANY of these" at agent-selection time. Real
// workflows could schedule code-review steps onto agents that lack
// review capability — the actual execution would then fail with a
// less-informative error or, worse, fabricate a review because Agent
// implementations rarely re-check their capabilities. The fix is to
// require the candidate agent to satisfy EVERY entry in
// step.RequiredCaps, not just the first.
func (we *WorkflowExecutor) executeStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error {
	// Find suitable agent
	var agent Agent
	var err error

	if len(step.RequiredCaps) > 0 {
		// Find by capability — pick the first agent that satisfies ALL
		// required capabilities. Anchor primary search on the first cap
		// (cheap candidate set), then intersect against the remainder.
		candidates := we.coordinator.registry.GetByCapability(step.RequiredCaps[0])
		var matched []Agent
		for _, candidate := range candidates {
			if agentHasAllCapabilities(candidate, step.RequiredCaps) {
				matched = append(matched, candidate)
			}
		}
		if len(matched) == 0 {
			capsStr := formatCapabilities(step.RequiredCaps)
			if step.Optional {
				// Create a failed result for optional step
				result := &task.Result{
					TaskID:    step.ID,
					AgentID:   "none",
					Success:   false,
					Error:     fmt.Sprintf("no agent found with all required capabilities %s", capsStr),
					Timestamp: time.Now(),
				}
				workflow.SetStepResult(step.ID, result)
				return nil
			}
			return fmt.Errorf("no agent found with all required capabilities %s for step %s", capsStr, step.ID)
		}
		agent = matched[0]
	} else {
		// Find by type
		agents := we.coordinator.registry.GetByType(step.AgentType)
		if len(agents) == 0 {
			if step.Optional {
				result := &task.Result{
					TaskID:    step.ID,
					AgentID:   "none",
					Success:   false,
					Error:     fmt.Sprintf("no agent found of type %s", step.AgentType),
					Timestamp: time.Now(),
				}
				workflow.SetStepResult(step.ID, result)
				return nil
			}
			return fmt.Errorf("no agent found of type %s for step %s", step.AgentType, step.ID)
		}
		agent = agents[0]
	}

	// Prepare input by merging step input with outputs from dependencies
	input := make(map[string]interface{})
	for k, v := range step.Input {
		input[k] = v
	}

	// Add outputs from dependency steps
	for _, depID := range step.DependsOn {
		if result, ok := workflow.GetStepResult(depID); ok && result.Success {
			// Merge dependency output into input
			for k, v := range result.Output {
				// Use dependency output if not already specified in step input
				if _, exists := input[k]; !exists {
					input[k] = v
				}
			}
		}
	}

	// Create task for this step
	t := task.NewTask(
		task.TaskType(step.Name), // Use step name as task type
		step.Name,
		fmt.Sprintf("Workflow step: %s", step.Name),
		task.PriorityNormal,
	)
	t.Input = input
	t.RequiredCapabilities = make([]string, len(step.RequiredCaps))
	for i, cap := range step.RequiredCaps {
		t.RequiredCapabilities[i] = string(cap)
	}

	// Execute task
	result, err := agent.Execute(ctx, t)
	if err != nil {
		if step.Optional {
			// Record failure but continue
			result = &task.Result{
				TaskID:    step.ID,
				AgentID:   agent.ID(),
				Success:   false,
				Error:     err.Error(),
				Timestamp: time.Now(),
			}
			workflow.SetStepResult(step.ID, result)
			return nil
		}
		return fmt.Errorf("step %s failed: %w", step.ID, err)
	}

	// Record result
	workflow.SetStepResult(step.ID, result)
	return nil
}

// GetWorkflow retrieves a workflow by ID
func (we *WorkflowExecutor) GetWorkflow(id string) (*Workflow, error) {
	we.mu.RLock()
	defer we.mu.RUnlock()

	workflow, ok := we.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", id)
	}
	return workflow, nil
}

// ListWorkflows returns all workflows
func (we *WorkflowExecutor) ListWorkflows() []*Workflow {
	we.mu.RLock()
	defer we.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(we.workflows))
	for _, w := range we.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

// GenerateWorkflowID generates a unique workflow ID
func GenerateWorkflowID() string {
	// Use UUID for better uniqueness guarantee
	return fmt.Sprintf("workflow-%s", uuid.New().String())
}

// agentHasAllCapabilities reports whether the given agent declares every
// capability in required.
//
// Helper for executeStep — see the round-36 doc-comment there for the
// §11.4 anti-bluff rationale behind tightening RequiredCaps from
// "any-of" to "all-of" semantics.
func agentHasAllCapabilities(agent Agent, required []Capability) bool {
	if agent == nil || len(required) == 0 {
		return true
	}
	declared := agent.Capabilities()
	have := make(map[Capability]bool, len(declared))
	for _, c := range declared {
		have[c] = true
	}
	for _, req := range required {
		if !have[req] {
			return false
		}
	}
	return true
}

// formatCapabilities renders a []Capability for inclusion in error
// messages. Used by executeStep when reporting "no agent satisfies all
// of these caps".
func formatCapabilities(caps []Capability) string {
	if len(caps) == 0 {
		return "[]"
	}
	parts := make([]string, len(caps))
	for i, c := range caps {
		parts[i] = string(c)
	}
	return "[" + joinStrings(parts, ", ") + "]"
}

// joinStrings is a tiny strings.Join shim. We avoid importing strings
// here because the rest of workflow.go does not use it; a single shim
// keeps the import surface stable.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += sep + p
	}
	return out
}
