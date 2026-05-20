package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeTier is a controllable in-process Tier for unit tests. It records
// every Get/Set/Delete so a test can assert exactly which tiers were
// touched — the mechanical core of the coherence and populate-upward
// proofs. (Unit-test-only fake — CONST-050(A) permits fakes here.)
type fakeTier struct {
	name      string
	available bool
	mu        sync.Mutex
	data      map[string][]byte
	getCalls  int32
	setCalls  int32
	delCalls  int32
	failGet   bool // simulate a non-fatal backend Get error
}

func newFakeTier(name string) *fakeTier {
	return &fakeTier{name: name, available: true, data: make(map[string][]byte)}
}

func (f *fakeTier) Name() string   { return f.name }
func (f *fakeTier) Available() bool { return f.available }

func (f *fakeTier) Get(_ context.Context, key string) ([]byte, error) {
	atomic.AddInt32(&f.getCalls, 1)
	if f.failGet {
		return nil, fmt.Errorf("%s: simulated backend failure", f.name)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	if !ok {
		return nil, ErrMiss
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (f *fakeTier) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	atomic.AddInt32(&f.setCalls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	b := make([]byte, len(value))
	copy(b, value)
	f.data[key] = b
	return nil
}

func (f *fakeTier) Delete(_ context.Context, key string) error {
	atomic.AddInt32(&f.delCalls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return nil
}

func (f *fakeTier) Close() error { return nil }

func (f *fakeTier) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.data[key]
	return ok
}

// TestReadThrough_L1ToL3_PopulateUpward proves the read-through path:
// a value present only in L3 is returned AND copied up into L1 and L2.
func TestReadThrough_L1ToL3_PopulateUpward(t *testing.T) {
	ctx := context.Background()
	l1, l2, l3 := newFakeTier("L1"), newFakeTier("L2"), newFakeTier("L3")
	// Seed ONLY L3.
	_ = l3.Set(ctx, "k", []byte("v3"), 0)

	mt := New(Config{Tiers: []Tier{l1, l2, l3}})
	val, ok := mt.Get(ctx, "k")
	if !ok || string(val) != "v3" {
		t.Fatalf("read-through: got (%q,%v), want (v3,true)", val, ok)
	}
	// Populate-upward: L1 and L2 must now hold the value.
	if !l1.has("k") {
		t.Error("populate-upward: L1 was not warmed by an L3 hit")
	}
	if !l2.has("k") {
		t.Error("populate-upward: L2 was not warmed by an L3 hit")
	}
	// Second read must resolve at L1 — L3 not consulted again.
	before := atomic.LoadInt32(&l3.getCalls)
	if _, ok := mt.Get(ctx, "k"); !ok {
		t.Fatal("second read missed after warm-up")
	}
	if atomic.LoadInt32(&l3.getCalls) != before {
		t.Error("second read still hit L3 — populate-upward did not warm L1")
	}
	stats := mt.Stats()
	if stats.Hits["L1"] != 1 || stats.Hits["L3"] != 1 {
		t.Errorf("stats: want L1=1 L3=1, got L1=%d L3=%d", stats.Hits["L1"], stats.Hits["L3"])
	}
}

// TestCoherence_WriteClearsAllTiers is the anti-bluff core: a write
// (Set) must overwrite EVERY tier, and an Invalidate must clear EVERY
// tier — a stale value must never survive in any tier.
func TestCoherence_WriteClearsAllTiers(t *testing.T) {
	ctx := context.Background()
	l1, l2, l3 := newFakeTier("L1"), newFakeTier("L2"), newFakeTier("L3")
	mt := New(Config{Tiers: []Tier{l1, l2, l3}})

	// Write — must land in all three tiers.
	if err := mt.Set(ctx, "k", []byte("v1"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	for _, ft := range []*fakeTier{l1, l2, l3} {
		if !ft.has("k") {
			t.Fatalf("coherence: Set did not write tier %s", ft.name)
		}
	}

	// Plant a STALE value directly into L2 and L3, bypassing the cache,
	// to simulate drift, then prove an Invalidate scrubs every tier.
	_ = l2.Set(ctx, "k", []byte("STALE"), 0)
	_ = l3.Set(ctx, "k", []byte("STALE"), 0)

	if err := mt.Invalidate(ctx, "k"); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	for _, ft := range []*fakeTier{l1, l2, l3} {
		if ft.has("k") {
			t.Fatalf("COHERENCE BUG: tier %s still holds key after Invalidate — "+
				"a stale entry could be served", ft.name)
		}
	}
	// A read after Invalidate MUST miss — no tier may serve the stale value.
	if val, ok := mt.Get(ctx, "k"); ok {
		t.Fatalf("COHERENCE BUG: Get returned %q after Invalidate, want miss", val)
	}
}

// TestGracefulDegradation_MissingRedisTier proves an unavailable tier
// (e.g. Redis not configured) is skipped — the cache keeps working on
// the remaining tiers and never errors the operation.
func TestGracefulDegradation_MissingRedisTier(t *testing.T) {
	ctx := context.Background()
	l1, l2 := newFakeTier("L1"), newFakeTier("L2")
	l3 := newFakeTier("L3")
	l3.available = false // simulate "no Redis configured"

	mt := New(Config{Tiers: []Tier{l1, l2, l3}})

	if err := mt.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("Set with unavailable L3 errored: %v", err)
	}
	// L3 must NOT have been touched at all.
	if atomic.LoadInt32(&l3.setCalls) != 0 {
		t.Error("unavailable L3 was written to — should be skipped")
	}
	val, ok := mt.Get(ctx, "k")
	if !ok || string(val) != "v" {
		t.Fatalf("Get with unavailable L3: got (%q,%v), want (v,true)", val, ok)
	}
	if atomic.LoadInt32(&l3.getCalls) != 0 {
		t.Error("unavailable L3 was read from — should be skipped")
	}
	if err := mt.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete with unavailable L3 errored: %v", err)
	}
}

// TestNonFatalTierError_FallsThrough proves a tier returning a real
// (non-ErrMiss) error does not abort the lookup — the cache falls
// through to the next tier so a flaky tier never breaks the feature.
func TestNonFatalTierError_FallsThrough(t *testing.T) {
	ctx := context.Background()
	l1, l2 := newFakeTier("L1"), newFakeTier("L2")
	l1.failGet = true // L1 backend is flaky
	_ = l2.Set(ctx, "k", []byte("v2"), 0)

	mt := New(Config{Tiers: []Tier{l1, l2}})
	val, ok := mt.Get(ctx, "k")
	if !ok || string(val) != "v2" {
		t.Fatalf("fall-through on tier error: got (%q,%v), want (v2,true)", val, ok)
	}
	if mt.Stats().TierErrors["L1"] != 1 {
		t.Error("non-fatal L1 error was not counted")
	}
}

// TestZeroTierCache_PassThrough proves a config-disabled cache (no
// tiers) is a safe pass-through: every Get misses, every Set/Delete is
// a no-op, never an error.
func TestZeroTierCache_PassThrough(t *testing.T) {
	ctx := context.Background()
	mt := New(Config{})
	if err := mt.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("zero-tier Set errored: %v", err)
	}
	if _, ok := mt.Get(ctx, "k"); ok {
		t.Fatal("zero-tier Get returned a hit")
	}
	if err := mt.Delete(ctx, "k"); err != nil {
		t.Fatalf("zero-tier Delete errored: %v", err)
	}
}

// TestConcurrentAccess exercises the cache under concurrent load to
// catch races (run with -race).
func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	mt := New(Config{Tiers: []Tier{newFakeTier("L1"), newFakeTier("L2")}})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", n%8)
			for j := 0; j < 100; j++ {
				_ = mt.Set(ctx, key, []byte("v"), 0)
				mt.Get(ctx, key)
				if j%10 == 0 {
					_ = mt.Invalidate(ctx, key)
				}
			}
		}(i)
	}
	wg.Wait()
}
