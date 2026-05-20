//go:build integration

package cache

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/redis"
)

// redisConfigFromEnv builds a RedisConfig from CACHE_TEST_REDIS_* (or
// REDIS_*) env vars. Returns nil when no host is configured so the
// test can skip-OK per CONST §11.4.3 (real infra, skip when absent).
func redisConfigFromEnv() *config.RedisConfig {
	host := os.Getenv("CACHE_TEST_REDIS_HOST")
	if host == "" {
		host = os.Getenv("REDIS_HOST")
	}
	if host == "" {
		return nil
	}
	port := 6379
	if p := os.Getenv("CACHE_TEST_REDIS_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	return &config.RedisConfig{Enabled: true, Host: host, Port: port}
}

// TestIntegration_RedisL3_RealBackend exercises the L3 tier against a
// REAL Redis: a 3-tier cache write must land in Redis, a read after
// clearing L1+L2 must fetch it back from Redis and re-warm the upper
// tiers, and an Invalidate must scrub Redis too (coherence on real
// infra). SKIP-OK: requires a real Redis (CONST §11.4.3) — set
// CACHE_TEST_REDIS_HOST (or REDIS_HOST) to run.
func TestIntegration_RedisL3_RealBackend(t *testing.T) {
	cfg := redisConfigFromEnv()
	if cfg == nil {
		t.Skip("SKIP-OK: no real Redis configured (set CACHE_TEST_REDIS_HOST) — CONST §11.4.3")
	}
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("SKIP-OK: real Redis unreachable at %s:%d: %v — CONST §11.4.3", cfg.Host, cfg.Port, err)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	key := "helixcache:integration:p4t02:" + strconv.FormatInt(time.Now().UnixNano(), 10)

	l1 := NewMemoryTier(MemoryTierConfig{MaxItems: 16})
	l2 := NewDiskTier(DiskTierConfig{Dir: t.TempDir()})
	l3 := NewRedisTier(RedisTierConfig{Backend: NewRedisBackend(client), TTL: time.Minute})
	if !l3.Available() {
		t.Fatal("real Redis L3 tier reported unavailable despite a live client")
	}
	mt := New(Config{Tiers: []Tier{l1, l2, l3}})
	defer func() { _ = mt.Invalidate(ctx, key) }()

	// Write through all three tiers.
	if err := mt.Set(ctx, key, []byte("real-redis-value"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Confirm the value is genuinely in real Redis.
	if v, err := l3.Get(ctx, key); err != nil || string(v) != "real-redis-value" {
		t.Fatalf("real Redis L3 Get: got (%q,%v), want real-redis-value", v, err)
	}

	// Clear the faster tiers — a read must now travel to real Redis.
	_ = l1.Delete(ctx, key)
	_ = l2.Delete(ctx, key)
	val, ok := mt.Get(ctx, key)
	if !ok || string(val) != "real-redis-value" {
		t.Fatalf("read-through to real Redis: got (%q,%v), want real-redis-value", val, ok)
	}
	// Populate-upward re-warmed L1 from the real-Redis hit.
	if v, err := l1.Get(ctx, key); err != nil || string(v) != "real-redis-value" {
		t.Errorf("populate-upward from real Redis did not warm L1: (%q,%v)", v, err)
	}

	// Coherence on real infra: Invalidate must scrub Redis.
	if err := mt.Invalidate(ctx, key); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	if _, err := l3.Get(ctx, key); err == nil {
		t.Fatal("COHERENCE BUG: key still present in real Redis after Invalidate")
	}
}
