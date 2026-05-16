// Package task provides the task abstraction for agent work units.
//
// Task is the fundamental unit of work that agents execute. Each task has a type,
// priority, status, and can depend on other tasks. Tasks track their execution
// lifecycle including assignment, progress, completion, and failure states.
//
// # Task Types
//
// The package defines several task types that categorize work:
//   - TaskTypePlanning: Strategic planning and architecture decisions
//   - TaskTypeAnalysis: Code analysis and understanding
//   - TaskTypeCodeGeneration: Creating new code
//   - TaskTypeCodeEdit: Modifying existing code
//   - TaskTypeRefactoring: Restructuring code without changing behavior
//   - TaskTypeTesting: Writing and running tests
//   - TaskTypeDebugging: Finding and fixing bugs
//   - TaskTypeReview: Code review and quality checks
//   - TaskTypeDocumentation: Writing documentation
//   - TaskTypeResearch: Investigating technologies or approaches
//
// # Task Lifecycle
//
// Tasks progress through defined statuses:
//   - StatusPending: Task created but not ready
//   - StatusReady: All dependencies satisfied
//   - StatusAssigned: Assigned to an agent
//   - StatusInProgress: Being actively worked on
//   - StatusBlocked: Waiting on external dependency
//   - StatusCompleted: Successfully finished
//   - StatusFailed: Failed with error
//   - StatusCancelled: Cancelled before completion
//
// # Usage
//
//	task := task.NewTask(task.TaskTypeCodeGeneration, "Implement feature", "Add login button", task.PriorityHigh)
//	task.Start("coding-agent-1")
//	// ... agent performs work ...
//	task.Complete(map[string]interface{}{"code": generatedCode})
//
// # Results and Artifacts
//
// Task execution produces Result objects containing output data, artifacts
// (generated files), metrics (tokens used, lines changed), and confidence scores.
package task
