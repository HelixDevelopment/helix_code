package cache

import (
	"context"
	"testing"
)

// payload is a representative cached value size (a parsed-symbol /
// embedding-vector blob).
var payload = make([]byte, 4096)

// BenchmarkTier_L1_Memory measures a warm L1 read — the fast path the
// 3-tier cache exists to hit (P4-T02 anti-bluff: per-tier latency).
func BenchmarkTier_L1_Memory(b *testing.B) {
	ctx := context.Background()
	t := NewMemoryTier(MemoryTierConfig{MaxItems: 1024})
	_ = t.Set(ctx, "k", payload, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := t.Get(ctx, "k"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTier_L2_Disk measures a warm L2 read (one os.ReadFile).
func BenchmarkTier_L2_Disk(b *testing.B) {
	ctx := context.Background()
	t := NewDiskTier(DiskTierConfig{Dir: b.TempDir()})
	_ = t.Set(ctx, "k", payload, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := t.Get(ctx, "k"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMultiTier_WarmRead measures the end-to-end 3-tier read with
// the value warm in L1 — the common case after the cache is primed.
func BenchmarkMultiTier_WarmRead(b *testing.B) {
	ctx := context.Background()
	mt := New(Config{Tiers: []Tier{
		NewMemoryTier(MemoryTierConfig{MaxItems: 1024}),
		NewDiskTier(DiskTierConfig{Dir: b.TempDir()}),
	}})
	_ = mt.Set(ctx, "k", payload, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := mt.Get(ctx, "k"); !ok {
			b.Fatal("warm read missed")
		}
	}
}

// BenchmarkMultiTier_ColdL2_PopulateUpward measures a read that misses
// L1, hits L2, and warms L1 — proving the populate-upward cost.
func BenchmarkMultiTier_ColdL2_PopulateUpward(b *testing.B) {
	ctx := context.Background()
	l1 := NewMemoryTier(MemoryTierConfig{MaxItems: 1024})
	l2 := NewDiskTier(DiskTierConfig{Dir: b.TempDir()})
	mt := New(Config{Tiers: []Tier{l1, l2}})
	_ = l2.Set(ctx, "k", payload, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l1.Delete(ctx, "k") // force the L1 miss each iteration
		if _, ok := mt.Get(ctx, "k"); !ok {
			b.Fatal("L2 read missed")
		}
	}
}
