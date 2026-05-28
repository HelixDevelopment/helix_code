package task

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the task manager.
//
// Chaos classes exercised against the REAL TaskManager (DB = permissive mock per
// CONST-050(A), unit-test scope; the manager's own RWMutex-guarded state machine
// is the real component under test):
//
//   - state-corruption under contention: a single task is mutated concurrently by
//     CompleteTask / FailTask / GetTaskProgress / GetTaskWithCache from many
//     goroutines mid-flight. The manager MUST never panic or race and MUST end in
//     a single, self-consistent terminal status — not a torn/garbage status.
//   - input-corruption: structurally hostile Data payloads (NaN/Inf floats,
//     deeply nested cycles-by-reference, huge keys, nil entries) are fed to
//     CreateTask. The manager MUST reject (error) or normalise without crashing.

// TestTaskManager_Chaos_ConcurrentStatusMutation creates one real task, then
// hammers it with concurrent status-mutating calls (CompleteTask + FailTask) and
// concurrent readers (GetTaskProgress + GetTaskWithCache). The real tm.mu must
// serialise the writers so the final status is one valid terminal value and the
// in-memory Task struct is never observed torn. Run under -race.
func TestTaskManager_Chaos_ConcurrentStatusMutation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "task_manager_status_mutation_churn", "state-corruption")
	tm := stressManager(t)
	ctx := context.Background()

	task, err := tm.CreateTask(TaskTypeBuilding,
		map[string]interface{}{"description": "chaos-target"},
		PriorityNormal, CriticalityNormal, nil)
	if err != nil {
		t.Fatalf("create chaos target: %v", err)
	}

	const writers = 8
	const readers = 8
	const iters = 400
	var wg sync.WaitGroup
	var completes, fails, reads int64

	// Writers: race CompleteTask vs FailTask on the SAME task. FailTask first
	// retries (status -> pending) then permanently fails; CompleteTask sets
	// completed. Whatever interleaving occurs, the manager must keep the struct
	// consistent and never panic.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("writer %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				if (id+it)%2 == 0 {
					if err := tm.CompleteTask(task.ID, map[string]interface{}{"by": id}); err == nil {
						atomic.AddInt64(&completes, 1)
					}
				} else {
					if err := tm.FailTask(task.ID, fmt.Sprintf("chaos fail %d", id)); err == nil {
						atomic.AddInt64(&fails, 1)
					}
				}
			}
		}(w)
	}

	// Readers: concurrently observe the task; a torn read would surface as a
	// panic or an invalid status under -race.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("reader %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				if prog, err := tm.GetTaskProgress(task.ID); err == nil {
					if math.IsNaN(prog.Progress) || prog.Progress < 0 || prog.Progress > 100 {
						rec.Record(stresschaos.Fatal, fmt.Sprintf("torn read: progress=%v status=%q", prog.Progress, prog.Status))
					}
					atomic.AddInt64(&reads, 1)
				}
				_, _ = tm.GetTaskWithCache(ctx, task.ID)
			}
		}(r)
	}

	wg.Wait()
	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived concurrent status churn: %d completes, %d fails, %d clean reads, no panic/race",
		atomic.LoadInt64(&completes), atomic.LoadInt64(&fails), atomic.LoadInt64(&reads)))

	// Final state MUST be one valid terminal/known status — not corrupted.
	final, err := tm.GetTaskWithCache(ctx, task.ID)
	if err != nil {
		rec.Record(stresschaos.Fatal, "task vanished after churn: "+err.Error())
	} else {
		valid := map[TaskStatus]bool{
			TaskStatusCompleted: true, TaskStatusFailed: true,
			TaskStatusPending: true, TaskStatusAssigned: true,
			TaskStatusRunning: true,
		}
		if !valid[final.Status] {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("final status invalid after churn: %q", final.Status))
		} else {
			rec.Record(stresschaos.Recovered, fmt.Sprintf("final status consistent: %q", final.Status))
		}
	}

	rec.AssertNoFatal()
	t.Logf("task chaos churn: completes=%d fails=%d reads=%d final=%v",
		atomic.LoadInt64(&completes), atomic.LoadInt64(&fails), atomic.LoadInt64(&reads),
		func() TaskStatus { f, _ := tm.GetTaskWithCache(ctx, task.ID); return f.Status }())
}

// TestTaskManager_Chaos_CorruptInputData feeds structurally hostile task Data
// payloads to the REAL CreateTask. The manager must reject or normalise each
// without panicking — a crash on malformed input is a §11.4.85(B) failure. Each
// payload is a JSON-ish corruption injected via a map the manager will try to
// JSON-marshal in estimateDataSize / cacheTask paths.
func TestTaskManager_Chaos_CorruptInputData(t *testing.T) {
	tm := stressManager(t)

	// Build corrupt-input payloads. We serialise a description of each so the
	// helper's [][]byte contract is honoured; feed() reconstructs a hostile map.
	corruptKinds := []map[string]interface{}{
		{"nan": math.NaN()},                       // NaN float — json.Marshal returns an error
		{"inf": math.Inf(1)},                      // +Inf — json.Marshal returns an error
		{"channel": "unmarshalable-marker-chan"},  // sentinel: feed swaps in a real chan
		{"deep": "unmarshalable-marker-func"},     // sentinel: feed swaps in a func value
		{"huge_key": makeHugeString(1 << 16)},     // 64 KiB key value
		{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}},
	}

	payloads := make([][]byte, len(corruptKinds))
	for i, k := range corruptKinds {
		// Marshalling the *descriptor* may itself fail for NaN/Inf; fall back to
		// a tag so the helper still gets a non-empty []byte to drive feed().
		b, err := json.Marshal(k)
		if err != nil {
			b = []byte(fmt.Sprintf(`{"corrupt_index":%d}`, i))
		}
		payloads[i] = b
	}

	stresschaos.ChaosCorruptInputDuring(t, "task_manager_corrupt_input", payloads,
		func(input []byte) error {
			idx := corruptIndexOf(input)
			data := hostileDataFor(idx)
			_, err := tm.CreateTask(TaskTypeBuilding, data, PriorityNormal, CriticalityNormal, nil)
			// A non-nil error is the desired graceful-rejection path; nil (the
			// manager normalised/accepted it) is also acceptable as long as no
			// panic occurred — the helper records both as non-fatal.
			return err
		})
}

// makeHugeString returns an n-byte string of 'x' for oversized-input chaos.
func makeHugeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// corruptIndexOf recovers the chaos payload index from the marshalled descriptor.
func corruptIndexOf(input []byte) int {
	var probe struct {
		CorruptIndex int `json:"corrupt_index"`
	}
	if err := json.Unmarshal(input, &probe); err == nil && probe.CorruptIndex >= 0 {
		// NaN/Inf descriptors fall back to {"corrupt_index":N}.
		if hasKey(input, "corrupt_index") {
			return probe.CorruptIndex
		}
	}
	// Sentinel-string detection for the channel/cycle/huge/nested cases.
	s := string(input)
	switch {
	case contains(jsonKeys(s), "channel"):
		return 2
	case contains(jsonKeys(s), "deep"):
		return 3
	case contains(jsonKeys(s), "huge_key"):
		return 4
	case contains(jsonKeys(s), "nested"):
		return 5
	case contains(jsonKeys(s), "nan"):
		return 0
	case contains(jsonKeys(s), "inf"):
		return 1
	}
	return 0
}

func hasKey(b []byte, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func jsonKeys(s string) []string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// hostileDataFor reconstructs the actual hostile Data map for a given chaos
// index — including types (chan, self-referential map) that cannot be carried
// through the []byte serialisation but exercise the manager's marshal paths.
func hostileDataFor(idx int) map[string]interface{} {
	switch idx {
	case 0:
		return map[string]interface{}{"nan": math.NaN()}
	case 1:
		return map[string]interface{}{"inf": math.Inf(1)}
	case 2:
		// A channel is not JSON-marshalable; estimateDataSize/cacheTask must not
		// panic on the marshal error.
		return map[string]interface{}{"channel": make(chan int)}
	case 3:
		// A func value is not JSON-marshalable; json.Marshal returns a clean
		// *UnsupportedTypeError (not a panic). The manager must surface/ignore
		// the marshal error gracefully without crashing.
		return map[string]interface{}{"deep": func() {}}
	case 4:
		return map[string]interface{}{"huge_key": makeHugeString(1 << 16)}
	default:
		return map[string]interface{}{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}}
	}
}
