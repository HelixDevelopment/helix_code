package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NOTE (P1-F19-T05): the previous in-tree AskUserTool stub that returned a
// UserResponse struct without ever prompting the user was deleted. The real
// ask_user tool now lives at internal/tools/askuser/ and is wired into the
// registry by cmd/cli/main.go (so it has access to os.Stdin/os.Stdout). See
// CONST-035 §11.9 — a non-prompting stub registered as "ask_user" was a
// bluff because every call appeared to succeed without ever blocking on the
// human operator.

// TaskTrackerTool implements task tracking
type TaskTrackerTool struct {
	registry *ToolRegistry
	tasks    map[string]*Task
	mu       sync.RWMutex
}

func (t *TaskTrackerTool) Name() string { return "task_tracker" }

func (t *TaskTrackerTool) Description() string {
	return "Track and manage tasks during execution"
}

func (t *TaskTrackerTool) Category() ToolCategory {
	return CategoryInteractive
}

func (t *TaskTrackerTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: create, update, list, get, complete",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Task ID (for update, get, complete)",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Task title (for create)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Task description (for create/update)",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Task status: pending, in_progress, completed, failed",
			},
			"progress": map[string]interface{}{
				"type":        "integer",
				"description": "Progress percentage (0-100)",
			},
		},
		Required:    []string{"action"},
		Description: "Track and manage tasks during execution",
	}
}

func (t *TaskTrackerTool) Validate(params map[string]interface{}) error {
	if _, ok := params["action"]; !ok {
		return fmt.Errorf("action is required")
	}

	action := params["action"].(string)
	validActions := map[string]bool{
		"create":   true,
		"update":   true,
		"list":     true,
		"get":      true,
		"complete": true,
	}

	if !validActions[action] {
		return fmt.Errorf("invalid action: %s", action)
	}

	return nil
}

func (t *TaskTrackerTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if t.tasks == nil {
		t.tasks = make(map[string]*Task)
	}

	action := params["action"].(string)

	switch action {
	case "create":
		return t.createTask(params)
	case "update":
		return t.updateTask(params)
	case "list":
		return t.listTasks()
	case "get":
		return t.getTask(params)
	case "complete":
		return t.completeTask(params)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *TaskTrackerTool) createTask(params map[string]interface{}) (*Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	task := &Task{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if title, ok := params["title"].(string); ok {
		task.Title = title
	}

	if desc, ok := params["description"].(string); ok {
		task.Description = desc
	}

	t.tasks[task.ID] = task
	return task, nil
}

func (t *TaskTrackerTool) updateTask(params map[string]interface{}) (*Task, error) {
	taskID, ok := params["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("task_id is required for update")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	task, exists := t.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if status, ok := params["status"].(string); ok {
		task.Status = TaskStatus(status)
	}

	if progress, ok := params["progress"].(int); ok {
		task.Progress = progress
	}

	if desc, ok := params["description"].(string); ok {
		task.Description = desc
	}

	task.UpdatedAt = time.Now()
	return task, nil
}

func (t *TaskTrackerTool) listTasks() ([]*Task, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tasks := make([]*Task, 0, len(t.tasks))
	for _, task := range t.tasks {
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (t *TaskTrackerTool) getTask(params map[string]interface{}) (*Task, error) {
	taskID, ok := params["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("task_id is required for get")
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	task, exists := t.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

func (t *TaskTrackerTool) completeTask(params map[string]interface{}) (*Task, error) {
	taskID, ok := params["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("task_id is required for complete")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	task, exists := t.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = TaskStatusCompleted
	task.Progress = 100
	task.CompletedAt = time.Now()
	task.UpdatedAt = time.Now()

	return task, nil
}

// Task represents a tracked task
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"` // 0-100
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)
