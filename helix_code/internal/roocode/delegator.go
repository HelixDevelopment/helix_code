package roocode

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type TaskDelegator struct {
	mu    sync.Mutex
	tasks map[string]*TaskSpec
}

func NewTaskDelegator() *TaskDelegator {
	return &TaskDelegator{tasks: make(map[string]*TaskSpec)}
}

func (d *TaskDelegator) Delegate(ctx context.Context, title, description string, priority int) (*TaskSpec, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	task := &TaskSpec{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Priority:    priority,
	}
	d.tasks[task.ID] = task
	return task, nil
}

func (d *TaskDelegator) ListTasks() []*TaskSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	var result []*TaskSpec
	for _, t := range d.tasks {
		result = append(result, t)
	}
	return result
}

func (d *TaskDelegator) GetTask(id string) (*TaskSpec, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	task, ok := d.tasks[id]
	if !ok {
		return nil, errors.New(tr(context.Background(), "internal_roocode_delegator_task_not_found", map[string]any{"ID": id}))
	}
	return task, nil
}

func (d *TaskDelegator) AssignTask(id, subagentID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	task, ok := d.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}
	task.AssignedTo = subagentID
	return nil
}
