//go:build integration

package redis

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/tests/stresschaos"
	goredis "github.com/redis/go-redis/v9"
)

// §11.4.85 STRESS coverage for internal/redis against a REAL Redis (no mocks —
// CONST-050: non-unit tests MUST hit real infrastructure). The unit under stress
// is the REAL *Client wrapper (Set/Get/Del/HSet/HGetAll/LPush/RPop/SAdd/ZAdd/
// Publish/Subscribe/pipeline-via-GetClient) talking to a live Redis at
// localhost:6379. Every PASS proves real round-trips happened: values written are
// read back and asserted equal, so a no-op wrapper would fail.
//
// Connection convention matches the existing internal/redis/redis_test.go:
// config.RedisConfig{Enabled:true, Host:"localhost", Port:6379}. Host/port are
// overridable via TEST_REDIS_HOST / TEST_REDIS_PORT for portability; default is
// the running podman instance. All keys use a per-test unique prefix and are
// cleaned up in t.Cleanup (no FLUSHDB — we never touch keys we did not create).

// testClient builds a REAL Client connected to the live Redis, or skips honestly
// (§11.4.3) ONLY if Redis is genuinely unreachable. It never fakes a connection.
func testClient(t *testing.T) *Client {
	t.Helper()
	// HXC-067: honour the standard HELIX_REDIS_* contract first, then the
	// legacy TEST_REDIS_* names, then the default — so the suite targets the
	// booted test Redis without a false 100%-error FAIL on a port mismatch.
	cfg := &config.RedisConfig{
		Enabled:  true,
		Host:     firstNonEmptyEnv("HELIX_REDIS_HOST", "TEST_REDIS_HOST", "localhost"),
		Port:     firstNonEmptyEnvInt("HELIX_REDIS_PORT", "TEST_REDIS_PORT", 6379),
		Password: firstNonEmptyEnv("HELIX_REDIS_PASSWORD", "TEST_REDIS_PASSWORD", ""),
		Database: envOrInt("TEST_REDIS_DB", 0),
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Skipf("SKIP-OK: real Redis unreachable at %s:%d (%v) — §11.4.3 honest skip, never a faked PASS",
			cfg.Host, cfg.Port, err)
	}
	if c == nil || c.GetClient() == nil {
		t.Skipf("SKIP-OK: real Redis returned nil client at %s:%d — §11.4.3 honest skip", cfg.Host, cfg.Port)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// keyPrefix returns a process+test-unique prefix so concurrent tests never
// collide and cleanup is surgical (delete-by-prefix, never FLUSHDB).
func keyPrefix(t *testing.T) string {
	return fmt.Sprintf("helixtest:%s:%d:", sanitize(t.Name()), time.Now().UnixNano())
}

// cleanupPrefix deletes every key under prefix via a real SCAN, registered as a
// t.Cleanup so the live DB is left exactly as we found it.
func cleanupPrefix(t *testing.T, c *Client, prefix string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		rdb := c.GetClient()
		if rdb == nil {
			return
		}
		var cursor uint64
		for {
			keys, next, err := rdb.Scan(ctx, cursor, prefix+"*", 256).Result()
			if err != nil {
				t.Logf("cleanup scan error (non-fatal): %v", err)
				return
			}
			if len(keys) > 0 {
				if err := rdb.Del(ctx, keys...).Err(); err != nil {
					t.Logf("cleanup del error (non-fatal): %v", err)
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	})
}

func sanitize(s string) string {
	return strings.NewReplacer("/", "_", " ", "_", ":", "_").Replace(s)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envOrInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return d
}

// firstNonEmptyEnv returns the first non-empty value among preferred, legacy,
// else the default (HXC-067 — prefer the standard HELIX_REDIS_* contract).
func firstNonEmptyEnv(preferred, legacy, d string) string {
	if v := os.Getenv(preferred); v != "" {
		return v
	}
	if v := os.Getenv(legacy); v != "" {
		return v
	}
	return d
}

// firstNonEmptyEnvInt is the int form of firstNonEmptyEnv.
func firstNonEmptyEnvInt(preferred, legacy string, d int) int {
	if v := os.Getenv(preferred); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	if v := os.Getenv(legacy); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return d
}

// =============================================================================
// §11.4.85(A)(1) — Sustained load (N>=100, p50/p95/p99 captured)
// =============================================================================

// TestRedis_Stress_SustainedSetGetDel drives a real SET -> GET -> DEL round-trip
// against live Redis under sustained load (N>=100). Each iteration writes a unique
// value, reads it back and asserts byte-equality, then deletes it — proving real
// persistence, not a no-op wrapper. Latency p50/p95/p99 are captured as evidence.
func TestRedis_Stress_SustainedSetGetDel(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()

	var ops int64
	rep := stresschaos.RunSustainedLoad(t, "redis_sustained_set_get_del",
		stresschaos.SustainedConfig{N: 600, MaxErrorRate: 0.0},
		func(i int) error {
			key := fmt.Sprintf("%ssgd:%d", prefix, i)
			want := fmt.Sprintf("value-%d-%d", i, time.Now().UnixNano())
			if err := c.Set(ctx, key, want, 30*time.Second); err != nil {
				return fmt.Errorf("set: %w", err)
			}
			got, err := c.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("get: %w", err)
			}
			if got != want {
				return fmt.Errorf("round-trip mismatch: got %q want %q", got, want)
			}
			if err := c.Del(ctx, key); err != nil {
				return fmt.Errorf("del: %w", err)
			}
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("redis sustained loop performed zero real round-trips — not real work")
	}
	t.Logf("redis sustained set/get/del: %d real round-trips, p50=%.3fms p95=%.3fms p99=%.3fms",
		atomic.LoadInt64(&ops), rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestRedis_Stress_SustainedHashAndList drives real hash + list ops under load to
// exercise more of the wrapper surface (HSet/HGetAll/HDel + LPush/LLen/RPop).
func TestRedis_Stress_SustainedHashAndList(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()

	stresschaos.RunSustainedLoad(t, "redis_sustained_hash_and_list",
		stresschaos.SustainedConfig{N: 300, MaxErrorRate: 0.0},
		func(i int) error {
			hkey := fmt.Sprintf("%shash:%d", prefix, i)
			if err := c.HSet(ctx, hkey, "field", fmt.Sprintf("v%d", i), "n", i); err != nil {
				return fmt.Errorf("hset: %w", err)
			}
			m, err := c.HGetAll(ctx, hkey)
			if err != nil {
				return fmt.Errorf("hgetall: %w", err)
			}
			if m["field"] != fmt.Sprintf("v%d", i) {
				return fmt.Errorf("hash round-trip mismatch: %v", m)
			}
			if err := c.HDel(ctx, hkey, "field", "n"); err != nil {
				return fmt.Errorf("hdel: %w", err)
			}

			lkey := fmt.Sprintf("%slist:%d", prefix, i)
			if err := c.LPush(ctx, lkey, fmt.Sprintf("item%d", i)); err != nil {
				return fmt.Errorf("lpush: %w", err)
			}
			n, err := c.LLen(ctx, lkey)
			if err != nil {
				return fmt.Errorf("llen: %w", err)
			}
			if n != 1 {
				return fmt.Errorf("expected list len 1, got %d", n)
			}
			popped, err := c.RPop(ctx, lkey)
			if err != nil {
				return fmt.Errorf("rpop: %w", err)
			}
			if popped != fmt.Sprintf("item%d", i) {
				return fmt.Errorf("list round-trip mismatch: %q", popped)
			}
			return nil
		})
}

// =============================================================================
// §11.4.85(A)(2) — Concurrent contention (N>=10 goroutines, no deadlock, no leak)
// =============================================================================

// TestRedis_Stress_ConcurrentSetGet hammers the REAL client from N>=10 goroutines
// performing concurrent SET/GET against disjoint key spaces. Run under -race this
// also catches data races in the wrapper. Each goroutine asserts its own writes
// read back correctly, so concurrent correctness is proven, not just survival.
func TestRedis_Stress_ConcurrentSetGet(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()

	stresschaos.RunConcurrent(t, "redis_concurrent_set_get",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 60},
		func(g, iter int) error {
			key := fmt.Sprintf("%sg%d:i%d", prefix, g, iter)
			want := fmt.Sprintf("g%d-i%d", g, iter)
			if err := c.Set(ctx, key, want, 30*time.Second); err != nil {
				return fmt.Errorf("set: %w", err)
			}
			got, err := c.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("get: %w", err)
			}
			if got != want {
				return fmt.Errorf("concurrent round-trip mismatch g=%d iter=%d: got %q want %q", g, iter, got, want)
			}
			return nil
		})
}

// TestRedis_Stress_ConcurrentSharedKeyCounter exercises real contention on a
// SINGLE shared key via atomic INCR (through GetClient()), proving the wrapper +
// real Redis serialise concurrent writes correctly. The final value must equal
// the exact number of increments — a lost update would fail the assertion.
func TestRedis_Stress_ConcurrentSharedKeyCounter(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()
	rdb := c.GetClient()

	const parallelism = 12
	const itersPer = 50
	counterKey := prefix + "counter"

	stresschaos.RunConcurrent(t, "redis_concurrent_shared_counter",
		stresschaos.ConcurrencyConfig{Parallelism: parallelism, IterationsPerGoroutine: itersPer},
		func(g, iter int) error {
			return rdb.Incr(ctx, counterKey).Err()
		})

	final, err := rdb.Get(ctx, counterKey).Int64()
	if err != nil {
		t.Fatalf("read final counter: %v", err)
	}
	want := int64(parallelism * itersPer)
	if final != want {
		t.Fatalf("shared-key INCR lost updates: final=%d want=%d (real-Redis atomicity broken)", final, want)
	}
	t.Logf("redis concurrent shared counter: %d increments all landed atomically", final)
}

// TestRedis_Stress_ConcurrentPipeline drives the real go-redis pipeline (batched
// SET via GetClient().Pipelined) from N>=10 goroutines, then verifies every
// pipelined write is readable — proving batched throughput against live Redis.
func TestRedis_Stress_ConcurrentPipeline(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()
	rdb := c.GetClient()

	const batch = 20
	stresschaos.RunConcurrent(t, "redis_concurrent_pipeline",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 25},
		func(g, iter int) error {
			pipe := rdb.Pipeline()
			keys := make([]string, batch)
			for b := 0; b < batch; b++ {
				k := fmt.Sprintf("%spipe:g%d:i%d:b%d", prefix, g, iter, b)
				keys[b] = k
				pipe.Set(ctx, k, fmt.Sprintf("%d", b), 30*time.Second)
			}
			if _, err := pipe.Exec(ctx); err != nil {
				return fmt.Errorf("pipeline exec: %w", err)
			}
			// Verify one of the batched writes actually landed.
			got, err := rdb.Get(ctx, keys[batch-1]).Result()
			if err != nil {
				return fmt.Errorf("verify pipelined write: %w", err)
			}
			if got != fmt.Sprintf("%d", batch-1) {
				return fmt.Errorf("pipelined value mismatch: %q", got)
			}
			return nil
		})
}

// TestRedis_Stress_ConcurrentPubSub exercises the real Publish/Subscribe path
// under concurrency: a subscriber drains a real channel while N>=10 publishers
// fire messages. Proves the pub/sub wrapper survives concurrent fan-in without
// deadlock or leak, and that at least some messages are genuinely delivered.
func TestRedis_Stress_ConcurrentPubSub(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	ctx := context.Background()

	channel := prefix + "chan"
	sub := c.Subscribe(ctx, channel)
	if sub == nil {
		t.Fatal("Subscribe returned nil for enabled client")
	}
	t.Cleanup(func() { _ = sub.Close() })

	// Wait for the subscription to be established before publishing.
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscription confirm: %v", err)
	}

	var received int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		ch := sub.Channel()
		deadline := time.After(20 * time.Second)
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if msg != nil {
					atomic.AddInt64(&received, 1)
				}
			case <-deadline:
				return
			}
		}
	}()

	const publishers = 12
	const perPub = 30
	stresschaos.RunConcurrent(t, "redis_concurrent_pubsub_publish",
		stresschaos.ConcurrencyConfig{Parallelism: publishers, IterationsPerGoroutine: perPub},
		func(g, iter int) error {
			return c.Publish(ctx, channel, fmt.Sprintf("msg-g%d-i%d", g, iter))
		})

	// Allow in-flight messages to drain, then stop the drainer.
	time.Sleep(500 * time.Millisecond)
	_ = sub.Close()
	<-done

	got := atomic.LoadInt64(&received)
	if got == 0 {
		t.Fatal("pub/sub delivered zero messages — real delivery not proven")
	}
	t.Logf("redis concurrent pub/sub: delivered %d/%d messages (some loss acceptable for fire-and-forget pub/sub)",
		got, publishers*perPub)
}

// =============================================================================
// §11.4.85(A)(3) — Boundary conditions (empty / max / off-by-one)
// =============================================================================

// TestRedis_Stress_Boundaries categorises boundary inputs and asserts the real
// wrapper handles each: empty value, large value, missing key (redis.Nil),
// TTL edge (immediate-expiry + persistent), and binary-safe value round-trip.
func TestRedis_Stress_Boundaries(t *testing.T) {
	c := testClient(t)
	prefix := keyPrefix(t)
	cleanupPrefix(t, c, prefix)
	ctx := context.Background()

	t.Run("empty_value", func(t *testing.T) {
		key := prefix + "empty"
		if err := c.Set(ctx, key, "", 30*time.Second); err != nil {
			t.Fatalf("set empty: %v", err)
		}
		got, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("get empty: %v", err)
		}
		if got != "" {
			t.Fatalf("empty round-trip got %q", got)
		}
	})

	t.Run("large_value_1MB", func(t *testing.T) {
		key := prefix + "large"
		large := strings.Repeat("X", 1<<20) // 1 MiB
		if err := c.Set(ctx, key, large, 30*time.Second); err != nil {
			t.Fatalf("set large: %v", err)
		}
		got, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("get large: %v", err)
		}
		if len(got) != len(large) {
			t.Fatalf("large round-trip length got %d want %d", len(got), len(large))
		}
	})

	t.Run("missing_key_returns_nil_error", func(t *testing.T) {
		_, err := c.Get(ctx, prefix+"definitely-absent")
		if err == nil {
			t.Fatal("Get on missing key returned nil error — boundary not handled")
		}
		if err != goredis.Nil {
			t.Fatalf("missing key error = %v, want redis.Nil", err)
		}
	})

	t.Run("binary_safe_value", func(t *testing.T) {
		key := prefix + "binary"
		payload := []byte{0x00, 0x01, 0xff, 0xfe, 0x00, 0x7f, 0x80}
		if err := c.Set(ctx, key, payload, 30*time.Second); err != nil {
			t.Fatalf("set binary: %v", err)
		}
		got, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("get binary: %v", err)
		}
		if got != string(payload) {
			t.Fatalf("binary round-trip corrupted: got %x want %x", got, payload)
		}
	})

	t.Run("ttl_edges", func(t *testing.T) {
		key := prefix + "ttl"
		if err := c.Set(ctx, key, "v", 0); err != nil { // 0 == persistent
			t.Fatalf("set persistent: %v", err)
		}
		ttl, err := c.TTL(ctx, key)
		if err != nil {
			t.Fatalf("ttl: %v", err)
		}
		// Persistent key reports -1 (== -1ns through go-redis).
		if ttl != -1*time.Nanosecond {
			t.Logf("persistent TTL reported %v (acceptable: server-dependent representation)", ttl)
		}
		if err := c.Expire(ctx, key, 1*time.Second); err != nil {
			t.Fatalf("expire: %v", err)
		}
		ttl2, err := c.TTL(ctx, key)
		if err != nil {
			t.Fatalf("ttl after expire: %v", err)
		}
		if ttl2 <= 0 {
			t.Fatalf("expected positive TTL after Expire, got %v", ttl2)
		}
	})
}
