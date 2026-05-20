package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// contention_bench_test.go — Speed programme Phase 4 / P4-T04.
//
// These benchmarks exist to PROVE (or disprove) the B22 audit hypothesis that
// MemoryManager's read-mux is a real lock-contention bottleneck under
// concurrent agent turns. The audit marked B22 "UNCONFIRMED" — these
// benchmarks, run under -mutexprofile/-blockprofile/-cpuprofile, supply the
// captured runtime evidence required by §11.4.6 (no-guessing) before any
// locking-strategy change is made.
//
// Mocks/fakes are NOT used here — this is a unit benchmark exercising the real
// MemoryManager + real InMemoryProvider (CONST-050(A): unit-test scope).

func newBenchManager(b *testing.B) *MemoryManager {
	b.Helper()
	mm := NewMemoryManager(&MemoryConfig{Provider: "in-memory"})
	p, err := NewInMemoryProvider(nil)
	if err != nil {
		b.Fatalf("NewInMemoryProvider: %v", err)
	}
	if err := mm.RegisterProvider("in-memory", p); err != nil {
		b.Fatalf("RegisterProvider: %v", err)
	}
	// Pre-populate so reads exercise the map under the lock.
	ctx := context.Background()
	for i := 0; i < 256; i++ {
		if err := mm.Store(ctx, fmt.Sprintf("key-%d", i), i); err != nil {
			b.Fatalf("Store: %v", err)
		}
	}
	return mm
}

// BenchmarkMemoryManager_ConcurrentRetrieve hammers the read path
// (GetDefaultProvider -> GetProvider -> provider.Retrieve) from many
// goroutines. This is the path B22 claims is contended; the mutex profile
// captured during this run is the BEFORE/AFTER evidence for the task.
func BenchmarkMemoryManager_ConcurrentRetrieve(b *testing.B) {
	mm := newBenchManager(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := mm.Retrieve(ctx, fmt.Sprintf("key-%d", i&0xff)); err != nil {
				b.Errorf("Retrieve: %v", err)
				return
			}
			i++
		}
	})
}

// BenchmarkMemoryManager_MixedReadWrite mixes Store (write) with Retrieve
// (read) so the read-mux sees writer contention — the worst case for the
// double-RLock pattern in GetDefaultProvider.
func BenchmarkMemoryManager_MixedReadWrite(b *testing.B) {
	mm := newBenchManager(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			k := fmt.Sprintf("key-%d", i&0xff)
			if i&0x0f == 0 {
				if err := mm.Store(ctx, k, i); err != nil {
					b.Errorf("Store: %v", err)
					return
				}
			} else {
				if _, err := mm.Retrieve(ctx, k); err != nil {
					b.Errorf("Retrieve: %v", err)
					return
				}
			}
			i++
		}
	})
}

// BenchmarkMemoryManager_GetDefaultProvider isolates the GetDefaultProvider
// call so the mutex profile attributes contention precisely to the
// manager-level read-mux (the double-RLock site).
func BenchmarkMemoryManager_GetDefaultProvider(b *testing.B) {
	mm := newBenchManager(b)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := mm.GetDefaultProvider(); err != nil {
				b.Errorf("GetDefaultProvider: %v", err)
				return
			}
		}
	})
}

// TestMemoryManager_GetDefaultProviderNoReentrantDeadlock verifies that
// GetDefaultProvider does not deadlock when a writer races with concurrent
// readers. Go's sync.RWMutex.RLock is NOT reentrant: if GetDefaultProvider
// holds an RLock and then re-enters via GetProvider while a writer's Lock()
// is queued, the inner RLock blocks behind the writer behind the outer
// RLock -> deadlock. This test reproduces that race window.
func TestMemoryManager_GetDefaultProviderNoReentrantDeadlock(t *testing.T) {
	mm := NewMemoryManager(&MemoryConfig{Provider: "in-memory"})
	p, err := NewInMemoryProvider(nil)
	if err != nil {
		t.Fatalf("NewInMemoryProvider: %v", err)
	}
	if err := mm.RegisterProvider("in-memory", p); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	const readers = 64
	const writers = 8
	const iterations = 2000
	var readerWG, writerWG sync.WaitGroup
	done := make(chan struct{})

	// Writers continuously contend the write lock until `done` is closed.
	for w := 0; w < writers; w++ {
		writerWG.Add(1)
		go func(id int) {
			defer writerWG.Done()
			name := fmt.Sprintf("writer-%d", id)
			for {
				select {
				case <-done:
					return
				default:
					_ = mm.RegisterProvider(name, p)
					_ = mm.UnregisterProvider(name)
				}
			}
		}(w)
	}

	// Readers hammer GetDefaultProvider — the double-RLock path — for a
	// fixed iteration count, then exit on their own.
	for r := 0; r < readers; r++ {
		readerWG.Add(1)
		go func() {
			defer readerWG.Done()
			for i := 0; i < iterations; i++ {
				if _, err := mm.GetDefaultProvider(); err != nil {
					t.Errorf("GetDefaultProvider: %v", err)
					return
				}
			}
		}()
	}

	// Wait for all readers to finish their fixed work — if the double-RLock
	// path deadlocks, this never returns and the test times out (FAIL).
	readerWG.Wait()
	// Readers done: stop the infinite writers and join them.
	close(done)
	writerWG.Wait()
}
