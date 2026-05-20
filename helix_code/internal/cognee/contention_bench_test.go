package cognee

import (
	"fmt"
	"testing"
	"time"
)

// contention_bench_test.go — Speed programme Phase 4 / P4-T04.
//
// These benchmarks exist to PROVE (or disprove) the B23 audit hypothesis that
// CogneeService's ServiceCache mutex is a real lock-contention bottleneck
// ("mutex density suggests serialised graph ops"). The audit marked B23
// "UNCONFIRMED" — these benchmarks, run under -mutexprofile/-blockprofile,
// supply the captured runtime evidence required by §11.4.6 (no-guessing)
// before any locking-strategy change is made.
//
// Mocks/fakes are NOT used — this is a unit benchmark exercising the real
// ServiceCache type (CONST-050(A): unit-test scope).

func newBenchCache() *ServiceCache {
	c := &ServiceCache{
		memories:    make(map[string]*CogneeMemory),
		searches:    make(map[string]*SearchMemoryResponse),
		datasets:    make(map[string]*Dataset),
		maxItems:    1000,
		ttl:         time.Hour,
		lastCleanup: time.Now(),
	}
	for i := 0; i < 256; i++ {
		c.memories[fmt.Sprintf("mem-%d", i)] = &CogneeMemory{ID: fmt.Sprintf("mem-%d", i)}
	}
	return c
}

// BenchmarkServiceCache_ConcurrentRead hammers the cache read path
// (getCachedMemory equivalent) from many goroutines. The mutex profile
// captured during this run is the BEFORE/AFTER evidence for B23.
func BenchmarkServiceCache_ConcurrentRead(b *testing.B) {
	c := newBenchCache()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.mu.RLock()
			_ = c.memories[fmt.Sprintf("mem-%d", i&0xff)]
			c.mu.RUnlock()
			i++
		}
	})
}

// BenchmarkServiceCache_MixedReadWrite mixes cache reads with writes so the
// RWMutex sees writer contention — the worst case the B23 hypothesis posits.
func BenchmarkServiceCache_MixedReadWrite(b *testing.B) {
	c := newBenchCache()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			k := fmt.Sprintf("mem-%d", i&0xff)
			if i&0x0f == 0 {
				c.mu.Lock()
				c.memories[k] = &CogneeMemory{ID: k}
				c.mu.Unlock()
			} else {
				c.mu.RLock()
				_ = c.memories[k]
				c.mu.RUnlock()
			}
			i++
		}
	})
}
