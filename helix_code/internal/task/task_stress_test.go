package task

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
	"github.com/google/uuid"
)

// §11.4.85 stress coverage for the task manager + queue.
//
// The unit under stress is the REAL TaskManager's RWMutex-guarded tasks map,
// the REAL TaskQueue's mutex-guarded priority slices, and the real CreateTask /
// GetTaskWithCache / GetTaskProgress / CompleteTask machinery (no fakes for the
// concurrency surface). The DB is a testify mock with Exec/QueryRow stubbed for
// success — permissible under CONST-050(A) because these *_test.go files run
// without the integration build tag (unit-test scope). The mock makes the real
// storeTaskInDB / updateTaskInDB SQL paths execute end-to-end so the manager's
// genuine create→persist→cache→read flow is exercised, not bypassed.
//
// Sustained Create/Get/Update load (N>=100) + N>=10 concurrent Create+Get
// producers, capturing latency + concurrency evidence under -race.

// stressManager builds a real TaskManager backed by a permissive mock DB (the
// package's existing MockDatabase()/MockRedis() helpers) so the real
// persistence + cache-graceful-degradation code paths run end-to-end. The
// disabled Redis client (IsEnabled()==false) drives the real no-op cache branch.
func stressManager(t *testing.T) *TaskManager {
	t.Helper()
	return NewTaskManager(MockDatabase(), MockRedis())
}

// TestTaskManager_Stress_SustainedCreateGetUpdate drives the real
// CreateTask -> GetTaskWithCache -> CompleteTask lifecycle under sustained load
// (N>=100), recording per-call latency. Every iteration creates a real task
// (real persistence path through the mock DB), reads it back, and completes it,
// asserting a non-zero processed count so the run proves real work happened.
func TestTaskManager_Stress_SustainedCreateGetUpdate(t *testing.T) {
	tm := stressManager(t)
	ctx := context.Background()

	var processed int64
	stresschaos.RunSustainedLoad(t, "task_manager_sustained_create_get_update",
		stresschaos.SustainedConfig{N: 1500, MaxErrorRate: 0.0},
		func(i int) error {
			task, err := tm.CreateTask(TaskTypeBuilding,
				map[string]interface{}{"description": fmt.Sprintf("stress-%d", i)},
				PriorityNormal, CriticalityNormal, nil)
			if err != nil {
				return fmt.Errorf("create: %w", err)
			}
			got, err := tm.GetTaskWithCache(ctx, task.ID)
			if err != nil {
				return fmt.Errorf("get: %w", err)
			}
			if got.ID != task.ID {
				return fmt.Errorf("get returned wrong task: want %s got %s", task.ID, got.ID)
			}
			if _, err := tm.GetTaskProgress(task.ID); err != nil {
				return fmt.Errorf("progress: %w", err)
			}
			if err := tm.CompleteTask(task.ID, map[string]interface{}{"ok": true}); err != nil {
				return fmt.Errorf("complete: %w", err)
			}
			atomic.AddInt64(&processed, 1)
			return nil
		})

	if atomic.LoadInt64(&processed) == 0 {
		t.Fatal("task manager processed zero tasks under sustained load — not real work")
	}
	t.Logf("task_manager sustained: %d tasks created+read+completed", atomic.LoadInt64(&processed))
}

// TestTaskManager_Stress_ConcurrentCreateGet hammers CreateTask + GetTaskWithCache
// + GetTaskProgress from N>=10 concurrent goroutines, asserting no deadlock, no
// goroutine leak, and no data race (run under -race) in the manager's shared
// tasks map + queue state. Each goroutine creates its own tasks and reads them
// back, so the RWMutex is exercised under genuine read/write contention.
func TestTaskManager_Stress_ConcurrentCreateGet(t *testing.T) {
	tm := stressManager(t)
	ctx := context.Background()

	var created int64
	stresschaos.RunConcurrent(t, "task_manager_concurrent_create_get",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 120, Timeout: 25 * time.Second},
		func(g, it int) error {
			task, err := tm.CreateTask(TaskTypePlanning,
				map[string]interface{}{"g": g, "it": it},
				PriorityHigh, CriticalityHigh, nil)
			if err != nil {
				return fmt.Errorf("create: %w", err)
			}
			atomic.AddInt64(&created, 1)
			// Read it back through the real cache-then-map path under contention.
			if _, err := tm.GetTaskWithCache(ctx, task.ID); err != nil {
				return fmt.Errorf("get: %w", err)
			}
			// Concurrent read of a definitely-missing task widens the RLock surface
			// and exercises the not-found error path.
			_, _ = tm.GetTaskProgress(uuid.New())
			return nil
		})

	if atomic.LoadInt64(&created) == 0 {
		t.Fatal("task manager created zero tasks under concurrent load")
	}
	t.Logf("task_manager concurrent: %d tasks created+read", atomic.LoadInt64(&created))
}
