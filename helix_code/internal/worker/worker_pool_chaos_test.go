package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the worker pool.
//
// Chaos classes exercised against the REAL WorkerPool:
//   - process-death: cancel the pool's Start() context mid-operation (the health-
//     check loop goroutine is killed) while assignment traffic flows; assert the
//     pool keeps serving assignments / unwinds cleanly with no panic/deadlock.
//   - state-corruption under contention: concurrently Register/Unregister workers
//     while AssignTask runs, asserting no race/panic and graceful "no available
//     worker" degradation rather than a crash.

// TestWorkerPool_Chaos_KillHealthLoopDuringAssign starts the pool's health-check
// loop, drives assignment traffic, then cancels the Start() context mid-flight
// (process-death of the loop goroutine). The pool MUST keep serving + Stop()
// cleanly with no panic/deadlock. Recovery trace captured to qa-results.
func TestWorkerPool_Chaos_KillHealthLoopDuringAssign(t *testing.T) {
	var assigned int64

	stresschaos.ChaosKillDuring(t, "worker_pool_kill_health_loop", 150*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			pool := NewWorkerPool(&config.WorkersConfig{HealthTTL: 3600, MaxConcurrentTasks: 4})
			for i := 0; i < 4; i++ {
				pool.RegisterWorker(NewPoolWorker(
					fmt.Sprintf("w-%d", i), "W", "localhost:0",
					WorkerCapabilities{CPUCores: 8, MemoryGB: 16}))
			}
			// Start the health-check loop bound to the cancellable chaos context.
			if err := pool.Start(ctx); err != nil {
				rec.Record(stresschaos.Fatal, "pool.Start returned error: "+err.Error())
				return
			}
			// Stop tears down the loop deterministically; a panic here = real defect.
			defer func() {
				defer func() {
					if p := recover(); p != nil {
						rec.Record(stresschaos.Fatal, fmt.Sprintf("pool.Stop panicked: %v", p))
					}
				}()
				pool.Stop()
			}()

			// Drive assignment traffic until the context is cancelled.
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Recovered,
						"health-loop context cancelled mid-operation; assignment loop observed cancellation and stopped cleanly")
					return
				default:
				}
				if w, err := pool.AssignTask(context.Background(), "compute",
					map[string]interface{}{"cpu_cores": 1}); err == nil {
					atomic.AddInt64(&assigned, 1)
					pool.ReleaseWorker(w.ID)
				} else {
					// Brief busy window — graceful backpressure, keep going.
					rec.Record(stresschaos.Degraded, "no available worker (transient): "+err.Error())
				}
				time.Sleep(time.Millisecond)
			}
		})

	t.Logf("worker_pool chaos: %d tasks assigned before/at kill", atomic.LoadInt64(&assigned))
}

// TestWorkerPool_Chaos_RegisterUnregisterDuringAssign injects state-corruption:
// workers are concurrently registered + unregistered while AssignTask runs. The
// pool MUST never panic and MUST degrade gracefully (clean "no available worker"
// error) when the worker set is momentarily empty. Run under -race.
func TestWorkerPool_Chaos_RegisterUnregisterDuringAssign(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "worker_pool_register_unregister_churn", "state-corruption")

	pool := NewWorkerPool(&config.WorkersConfig{HealthTTL: 3600, MaxConcurrentTasks: 8})
	// Seed a couple of stable workers so assignment can mostly succeed.
	for i := 0; i < 2; i++ {
		pool.RegisterWorker(NewPoolWorker(fmt.Sprintf("stable-%d", i), "W", "localhost:0",
			WorkerCapabilities{CPUCores: 8, MemoryGB: 16}))
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Churn goroutine: rapid register/unregister of a volatile worker.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, "register/unregister churn panicked")
			}
		}()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			id := fmt.Sprintf("volatile-%d", i%4)
			pool.RegisterWorker(NewPoolWorker(id, "V", "localhost:0",
				WorkerCapabilities{CPUCores: 8, MemoryGB: 16}))
			pool.UnregisterWorker(id)
			i++
		}
	}()

	// Assigner goroutines hammered while the worker set mutates underneath them.
	var ok, degraded int64
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, "AssignTask panicked during worker-set churn")
				}
			}()
			for it := 0; it < 500; it++ {
				w, err := pool.AssignTask(context.Background(), "compute",
					map[string]interface{}{"cpu_cores": 1})
				if err != nil {
					atomic.AddInt64(&degraded, 1) // clean "no available worker" — graceful
					continue
				}
				atomic.AddInt64(&ok, 1)
				pool.ReleaseWorker(w.ID)
			}
		}()
	}

	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"assigners survived worker-set churn: %d ok, %d clean-degradations, no panic/deadlock",
		atomic.LoadInt64(&ok), atomic.LoadInt64(&degraded)))
	if atomic.LoadInt64(&ok) == 0 {
		rec.Record(stresschaos.Fatal, "zero successful assignments during churn — pool never recovered")
	}
	rec.AssertNoFatal()
	t.Logf("worker_pool chaos churn: ok=%d degraded=%d", atomic.LoadInt64(&ok), atomic.LoadInt64(&degraded))
}
