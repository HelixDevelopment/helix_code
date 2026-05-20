package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestMemoryTier_LRUEviction proves the L1 tier evicts the least
// recently used entry once over capacity.
func TestMemoryTier_LRUEviction(t *testing.T) {
	ctx := context.Background()
	mt := NewMemoryTier(MemoryTierConfig{MaxItems: 3})
	for _, k := range []string{"a", "b", "c"} {
		_ = mt.Set(ctx, k, []byte(k), 0)
	}
	// Touch "a" so it is most-recently-used; insert "d" -> "b" evicted.
	if _, err := mt.Get(ctx, "a"); err != nil {
		t.Fatalf("Get a: %v", err)
	}
	_ = mt.Set(ctx, "d", []byte("d"), 0)
	if mt.Len() != 3 {
		t.Fatalf("MaxItems not enforced: len=%d", mt.Len())
	}
	if _, err := mt.Get(ctx, "b"); !errors.Is(err, ErrMiss) {
		t.Error("LRU: expected 'b' to be evicted")
	}
	if _, err := mt.Get(ctx, "a"); err != nil {
		t.Error("LRU: 'a' was wrongly evicted despite recent use")
	}
}

// TestMemoryTier_TTLExpiry proves an expired L1 entry reports a miss.
func TestMemoryTier_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	mt := NewMemoryTier(MemoryTierConfig{MaxItems: 8})
	_ = mt.Set(ctx, "k", []byte("v"), 20*time.Millisecond)
	if _, err := mt.Get(ctx, "k"); err != nil {
		t.Fatalf("fresh entry should hit: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err := mt.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Error("expired L1 entry should report ErrMiss")
	}
}

// TestDiskTier_PersistAcrossInstances proves the L2 tier survives a
// process restart: a second DiskTier over the same directory reads the
// entry the first one wrote.
func TestDiskTier_PersistAcrossInstances(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	d1 := NewDiskTier(DiskTierConfig{Dir: dir})
	if !d1.Available() {
		t.Fatal("disk tier should be available with a valid dir")
	}
	if err := d1.Set(ctx, "ctx-key", []byte("persisted-context"), time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}
	_ = d1.Close()

	// Fresh instance over the same dir == simulated process restart.
	d2 := NewDiskTier(DiskTierConfig{Dir: dir})
	val, err := d2.Get(ctx, "ctx-key")
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if string(val) != "persisted-context" {
		t.Fatalf("disk persistence: got %q, want persisted-context", val)
	}
}

// TestDiskTier_TTLExpiry proves an expired on-disk entry is not served.
func TestDiskTier_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	d := NewDiskTier(DiskTierConfig{Dir: t.TempDir()})
	_ = d.Set(ctx, "k", []byte("v"), 20*time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	if _, err := d.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Error("expired disk entry should report ErrMiss")
	}
}

// TestDiskTier_UnavailableDegrades proves a DiskTier with an empty dir
// is unavailable and its operations are safe no-ops.
func TestDiskTier_UnavailableDegrades(t *testing.T) {
	ctx := context.Background()
	d := NewDiskTier(DiskTierConfig{Dir: ""})
	if d.Available() {
		t.Fatal("disk tier with empty dir should be unavailable")
	}
	if err := d.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Errorf("unavailable disk Set should no-op, got %v", err)
	}
	if _, err := d.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Error("unavailable disk Get should report ErrMiss")
	}
}

// TestRedisTier_NilBackendUnavailable proves the L3 tier with no
// backend is unavailable — the cache then runs L1+L2 only.
func TestRedisTier_NilBackendUnavailable(t *testing.T) {
	ctx := context.Background()
	r := NewRedisTier(RedisTierConfig{Backend: nil})
	if r.Available() {
		t.Fatal("redis tier with nil backend should be unavailable")
	}
	if err := r.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Errorf("unavailable redis Set should no-op, got %v", err)
	}
}

// TestBuilder_DisabledIsPassThrough proves a config-disabled
// MultiTierConfig builds a zero-tier pass-through cache.
func TestBuilder_DisabledIsPassThrough(t *testing.T) {
	mt := MultiTierConfig{Enabled: false}.Build(nil, "repomap")
	if len(mt.TierNames()) != 0 {
		t.Fatalf("disabled config built %d tiers, want 0", len(mt.TierNames()))
	}
}

// TestBuilder_EnabledTierComposition proves the builder wires exactly
// the tiers the config enables.
func TestBuilder_EnabledTierComposition(t *testing.T) {
	cfg := MultiTierConfig{
		Enabled:     true,
		DiskEnabled: true,
		DiskDir:     t.TempDir(),
		// RedisEnabled false -> no L3.
	}
	mt := cfg.Build(nil, "embeddings")
	names := mt.TierNames()
	if len(names) != 2 || names[0] != "L1-memory" || names[1] != "L2-disk" {
		t.Fatalf("builder tier composition: got %v, want [L1-memory L2-disk]", names)
	}
}

// fakeRedisBackend is an in-process RedisBackend for the L3 unit path
// (CONST-050(A): fakes permitted in unit tests).
type fakeRedisBackend struct {
	enabled bool
	data    map[string][]byte
}

func (f *fakeRedisBackend) Enabled() bool { return f.enabled }
func (f *fakeRedisBackend) Get(_ context.Context, k string) ([]byte, error) {
	v, ok := f.data[k]
	if !ok {
		return nil, ErrMiss
	}
	return v, nil
}
func (f *fakeRedisBackend) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	f.data[k] = v
	return nil
}
func (f *fakeRedisBackend) Delete(_ context.Context, k string) error {
	delete(f.data, k)
	return nil
}

// TestRedisTier_FakeBackendRoundTrip proves the L3 tier round-trips
// through a RedisBackend without a real Redis (the real-Redis path is
// covered by the integration test).
func TestRedisTier_FakeBackendRoundTrip(t *testing.T) {
	ctx := context.Background()
	b := &fakeRedisBackend{enabled: true, data: map[string][]byte{}}
	r := NewRedisTier(RedisTierConfig{Backend: b})
	if !r.Available() {
		t.Fatal("redis tier with enabled backend should be available")
	}
	if err := r.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := r.Get(ctx, "k")
	if err != nil || string(val) != "v" {
		t.Fatalf("round-trip: got (%q,%v), want (v,nil)", val, err)
	}
	if err := r.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Error("Get after Delete should report ErrMiss")
	}
}
