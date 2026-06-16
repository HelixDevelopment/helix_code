package task

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/database"
	"dev.helix.code/internal/redis"
	"github.com/google/uuid"
)

// redModeEnabled reports whether the §11.4.115 RED polarity is active
// (RED_MODE=1). Default (unset/0) drives the real fixed code as the standing
// GREEN regression guard.
func redModeEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

// TestGetTaskWithCache_SnapshotRace proves (RED) / guards (GREEN) the
// snapshot-getter data race in (*TaskManager).GetTaskWithCache.
//
// DEFECT (§11.4.6, reproduced with `go test -race`): GetTaskWithCache reads
// tm.tasks[id] under RLock but returns the LIVE stored *Task pointer. A caller
// holding that pointer reads task fields (Status, UpdatedAt, Data, ...) with no
// lock, while AssignTask / CompleteTask / FailTask concurrently mutate the very
// same *Task under tm.mu.Lock(). Two goroutines touch the same memory, at least
// one writing, with no shared synchronisation on the returned object → a data
// race. The -race detector trips on the read of task.Status / task.UpdatedAt in
// the reader goroutine vs the write in the writer goroutine.
//
// FIX: GetTaskWithCache returns a deep-enough COPY (snapshot) of the task so the
// caller never shares mutable memory with the manager's writers.
//
// §11.4.115 polarity:
//   RED_MODE=1  — drive the OLD live-pointer behaviour via getTaskLivePointer
//                 (a faithful pre-fix stand-in); the -race trip IS the
//                 reproduction (test fails under -race on the broken behaviour).
//   default(0)  — drive the REAL fixed GetTaskWithCache; the returned snapshot
//                 must be safe to read concurrently with writers (no race) AND
//                 must NOT alias the stored task.
func TestGetTaskWithCache_SnapshotRace(t *testing.T) {
	tm := newSnapshotRaceManager(t)

	task, err := tm.CreateTask(
		TaskTypeBuilding,
		map[string]interface{}{"k": "v"},
		PriorityNormal,
		CriticalityNormal,
		nil,
	)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	id := task.ID

	redMode := redModeEnabled()

	ctx := context.Background()
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer: continuously mutate the stored task under the manager lock,
	// exactly as the real status-transition methods do.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			tm.mu.Lock()
			stored := tm.tasks[id]
			stored.Status = TaskStatusRunning
			stored.UpdatedAt = time.Now()
			stored.Data["counter"] = time.Now().UnixNano()
			tm.mu.Unlock()
		}
	}()

	// Reader: obtain a task via the getter and read its fields with no lock,
	// like a real consumer of the returned value.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			var got *Task
			if redMode {
				got = tm.getTaskLivePointer(id) // faithful pre-fix behaviour
			} else {
				got, _ = tm.GetTaskWithCache(ctx, id) // real fixed behaviour
			}
			if got == nil {
				continue
			}
			// Unsynchronised reads of the returned object's mutable fields.
			_ = got.Status
			_ = got.UpdatedAt
			for range got.Data {
			}
		}
	}()

	// Let the goroutines interleave, then stop.
	time.Sleep(40 * time.Millisecond)
	close(stop)
	wg.Wait()

	if !redMode {
		// GREEN extra assertion: the snapshot must NOT alias the stored task,
		// otherwise mutating the stored Data map would corrupt the caller's view.
		snap, _ := tm.GetTaskWithCache(ctx, id)
		tm.mu.Lock()
		stored := tm.tasks[id]
		tm.mu.Unlock()
		if snap == stored {
			t.Fatal("GetTaskWithCache returned the live stored *Task pointer; expected a snapshot copy")
		}
		// Mutating the stored Data map must not leak into the returned snapshot.
		tm.mu.Lock()
		stored.Data["leak-probe"] = true
		tm.mu.Unlock()
		if _, leaked := snap.Data["leak-probe"]; leaked {
			t.Fatal("snapshot Data map aliases the stored task's Data map (shallow copy)")
		}
	}
}

// getTaskLivePointer reproduces the PRE-FIX GetTaskWithCache lookup: it returns
// the live stored *Task pointer (no snapshot). Used ONLY by the RED_MODE=1
// reproduction so the historical defect is captured on a faithful stand-in.
func (tm *TaskManager) getTaskLivePointer(id uuid.UUID) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tasks[id]
}

func newSnapshotRaceManager(t *testing.T) *TaskManager {
	t.Helper()
	mockDB := database.NewMockDatabase()
	mockDB.MockExecSuccess(1)
	return NewTaskManager(mockDB, &redis.Client{})
}
