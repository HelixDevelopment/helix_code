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
	// Returns the live pointer by design: Delegate hands the creating caller a
	// usable handle to observe the task's later AssignTask state. The read
	// getters GetTask/ListTasks snapshot because they hand the pointer to
	// ARBITRARY later callers while concurrent writers exist; the creating
	// caller holds the only reference here. A caller sharing this handle across
	// goroutines AND reading it concurrently with AssignTask must GetTask() a
	// snapshot instead.
	return task, nil
}

func (d *TaskDelegator) ListTasks() []*TaskSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make([]*TaskSpec, 0, len(d.tasks))
	for _, t := range d.tasks {
		cp := *t
		result = append(result, &cp)
	}
	return result
}

// GetTask returns a snapshot copy of the stored task. Returning the LIVE
// stored *TaskSpec would let the caller read task.AssignedTo while a
// concurrent AssignTask writes it under d.mu — a data race the caller
// cannot guard because it never sees d.mu. The value-copy detaches the
// returned task from the stored one.
func (d *TaskDelegator) GetTask(id string) (*TaskSpec, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	task, ok := d.tasks[id]
	if !ok {
		return nil, errors.New(tr(context.Background(), "internal_roocode_delegator_task_not_found", map[string]any{"ID": id}))
	}
	cp := *task
	return &cp, nil
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
